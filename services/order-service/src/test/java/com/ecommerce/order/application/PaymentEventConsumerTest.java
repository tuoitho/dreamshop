package com.ecommerce.order.application;

import static com.ecommerce.order.support.StatusHistoryQueries.historyOf;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.when;

import com.ecommerce.contracts.order.v1.OrderDetail;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.integration.AuthorizationGrpcAdapter;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderItemEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.bean.override.mockito.MockitoBean;

class PaymentEventConsumerTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    PaymentEventConsumer paymentEventConsumer;

    @Autowired
    OrderSagaOrchestrator orchestrator;

    @Autowired
    OrderRepository orderRepository;

    @Autowired
    OrderQueryService orderQueryService;

    @Autowired
    OrderAdminQueryService orderAdminQueryService;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @MockitoBean
    AuthorizationGrpcAdapter authorizationAdapter;

    @BeforeEach
    void cleanTables() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
        reset(authorizationAdapter);
        when(authorizationAdapter.hasPermission(any(UUID.class), eq("order.cancel"))).thenReturn(true);
    }

    @Test
    void paymentSucceededConfirmsPendingOrderIdempotentlyAndIgnoresLateFailure() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(userId, "ORD-20261105-PAY001", OrderStatus.PENDING_PAYMENT, PaymentMethod.MOMO, PaymentStatus.PENDING);

        String succeededJson = """
                {
                  "eventId": "evt-succ-1",
                  "eventType": "PaymentSucceeded",
                  "eventVersion": 1,
                  "aggregateId": "%s",
                  "correlationId": "trace-1",
                  "data": {
                    "orderId": "%s",
                    "momoTransId": "momo-tx-777",
                    "amountMinor": 250000
                  }
                }
                """.formatted(orderId, orderId);

        paymentEventConsumer.handleMessage("payment.succeeded", succeededJson);
        // Replay duplicate eventId
        paymentEventConsumer.handleMessage("payment.succeeded", succeededJson);

        OrderEntity updated = orderRepository.findById(orderId).orElseThrow();
        assertThat(updated.getStatus()).isEqualTo(OrderStatus.CONFIRMED);
        assertThat(updated.getPaymentStatus()).isEqualTo(PaymentStatus.PAID);
        assertThat(updated.getPaymentTransId()).isEqualTo("momo-tx-777");

        Integer confirmedOutbox = jdbcTemplate.queryForObject(
                "SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND routing_key = 'order.confirmed'",
                Integer.class,
                orderId.toString());
        assertThat(confirmedOutbox).isEqualTo(1);
        assertThat(historyOf(jdbcTemplate, orderId)).containsExactly("PENDING_PAYMENT>CONFIRMED:SYSTEM");

        when(authorizationAdapter.hasPermission(any(UUID.class), eq("order.view"))).thenReturn(true);
        OrderDetail detail = orderQueryService.getOrder(userId.toString(), orderId.toString());
        assertThat(detail.getStatusHistoryList())
                .singleElement()
                .satisfies(change -> {
                    assertThat(change.getFromStatus()).isEqualTo("PENDING_PAYMENT");
                    assertThat(change.getToStatus()).isEqualTo("CONFIRMED");
                    assertThat(change.getActorType()).isEqualTo("SYSTEM");
                    assertThat(change.getCreatedAt()).isNotBlank();
                });

        // Late PaymentFailed after CONFIRMED is a no-op
        String failedJson = """
                {
                  "eventId": "evt-fail-late",
                  "eventType": "PaymentFailed",
                  "eventVersion": 1,
                  "aggregateId": "%s",
                  "data": { "orderId": "%s", "message": "Late fail" }
                }
                """.formatted(orderId, orderId);
        paymentEventConsumer.handleMessage("payment.failed", failedJson);

        OrderEntity stillConfirmed = orderRepository.findById(orderId).orElseThrow();
        assertThat(stillConfirmed.getStatus()).isEqualTo(OrderStatus.CONFIRMED);
        assertThat(stillConfirmed.getPaymentStatus()).isEqualTo(PaymentStatus.PAID);
    }

    @Test
    void paymentFailedCancelsPendingOrderAndRecordsHistory() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(userId, "ORD-20261112-PFAIL1", OrderStatus.PENDING_PAYMENT, PaymentMethod.MOMO, PaymentStatus.PENDING);

        paymentEventConsumer.handleMessage("payment.failed", """
                {
                  "eventId": "evt-fail-1",
                  "eventType": "PaymentFailed",
                  "eventVersion": 1,
                  "aggregateId": "%s",
                  "data": { "orderId": "%s", "message": "Declined" }
                }
                """.formatted(orderId, orderId));

        assertThat(orderRepository.findById(orderId).orElseThrow().getStatus()).isEqualTo(OrderStatus.CANCELLED);
        assertThat(historyOf(jdbcTemplate, orderId)).containsExactly("PENDING_PAYMENT>CANCELLED:SYSTEM");
    }

    @Test
    void latePaymentSucceededOnCancelledOrderMarksRefundRequiredAndEmitsRefundRequested() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(userId, "ORD-20261105-LATE01", OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.PENDING);

        String succeededJson = """
                {
                  "eventId": "evt-late-succ-1",
                  "eventType": "PaymentSucceeded",
                  "eventVersion": 1,
                  "aggregateId": "%s",
                  "data": {
                    "orderId": "%s",
                    "momoTransId": "momo-late-999",
                    "amountMinor": 250000
                  }
                }
                """.formatted(orderId, orderId);

        paymentEventConsumer.handleMessage("payment.succeeded", succeededJson);

        OrderEntity order = orderRepository.findById(orderId).orElseThrow();
        assertThat(order.getStatus()).isEqualTo(OrderStatus.CANCELLED);
        assertThat(order.getPaymentStatus()).isEqualTo(PaymentStatus.REFUND_REQUIRED);
        assertThat(order.getPaymentTransId()).isEqualTo("momo-late-999");

        List<String> routingKeys = jdbcTemplate.queryForList(
                "SELECT routing_key FROM outbox_events WHERE aggregate_id = ?",
                String.class,
                orderId.toString());
        assertThat(routingKeys).containsExactly("order.payment.refund_requested");
    }

    @Test
    void cancelOrderEmitsReleaseAndPaymentCancelOnPendingPaymentAndRejectsShipping() {
        UUID userId = UUID.randomUUID();
        UUID pendingId = seedOrder(userId, "ORD-20261105-CANC01", OrderStatus.PENDING_PAYMENT, PaymentMethod.MOMO, PaymentStatus.PENDING);

        OrderDetail cancelled = orchestrator.cancelOrder(userId.toString(), pendingId.toString(), "Doi y");
        assertThat(cancelled.getStatus()).isEqualTo("CANCELLED");

        List<String> routingKeys = jdbcTemplate.queryForList(
                "SELECT routing_key FROM outbox_events WHERE aggregate_id = ? ORDER BY routing_key ASC",
                String.class,
                pendingId.toString());
        assertThat(routingKeys).containsExactly("order.payment.cancel_requested", "order.reservation.release_requested");

        UUID shippingId = seedOrder(userId, "ORD-20261105-SHIP01", OrderStatus.SHIPPING, PaymentMethod.COD, PaymentStatus.UNPAID);
        assertThatThrownBy(() -> orchestrator.cancelOrder(userId.toString(), shippingId.toString(), "Muon huy"))
                .isInstanceOf(DomainException.class)
                .extracting(e -> ((DomainException) e).reason())
                .isEqualTo("ORDER_NOT_CANCELLABLE");
    }

    @Test
    void paymentRefundedTransitionsRefundRequiredAndRefundFailedToRefunded() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(userId, "ORD-20261125-RF01", OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUND_REQUIRED);

        paymentEventConsumer.handleMessage("payment.refunded", """
                {
                  "eventId": "evt-refunded-1",
                  "eventType": "PaymentRefunded",
                  "eventVersion": 1,
                  "aggregateId": "%s",
                  "data": {
                    "orderId": "%s",
                    "refundTransId": "momo-ref-tx-1",
                    "resultCode": 0,
                    "message": "Successful."
                  }
                }
                """.formatted(orderId, orderId));

        OrderEntity updated = orderRepository.findById(orderId).orElseThrow();
        assertThat(updated.getPaymentStatus()).isEqualTo(PaymentStatus.REFUNDED);
        assertThat(updated.getRefundTransId()).isEqualTo("momo-ref-tx-1");
        assertThat(updated.getRefundActorType()).isEqualTo("SYSTEM");
        assertThat(updated.getRefundedAt()).isNotNull();
        assertThat(updated.getRefundLastError()).isEmpty();

        // Duplicate PaymentRefunded with different eventId must not overwrite ADMIN audit metadata.
        UUID adminId = UUID.randomUUID();
        UUID adminRefundedId = seedOrder(userId, "ORD-20261125-RF02", OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUNDED);
        OrderEntity adminEntity = orderRepository.findById(adminRefundedId).orElseThrow();
        adminEntity.setRefundTransId("momo-ref-admin-1");
        adminEntity.setRefundActorType("ADMIN");
        adminEntity.setRefundActorId(adminId);
        adminEntity.setRefundReason("Manual retry");
        adminEntity.setRefundedAt(Instant.parse("2026-11-25T04:00:00Z"));
        orderRepository.saveAndFlush(adminEntity);

        paymentEventConsumer.handleMessage("payment.refunded", """
                {
                  "eventId": "evt-refunded-dup-admin",
                  "eventType": "PaymentRefunded",
                  "eventVersion": 1,
                  "aggregateId": "%s",
                  "data": {
                    "orderId": "%s",
                    "refundTransId": "momo-ref-admin-1",
                    "resultCode": 0,
                    "message": "Successful."
                  }
                }
                """.formatted(adminRefundedId, adminRefundedId));

        OrderEntity afterDup = orderRepository.findById(adminRefundedId).orElseThrow();
        assertThat(afterDup.getRefundActorType()).isEqualTo("ADMIN");
        assertThat(afterDup.getRefundActorId()).isEqualTo(adminId);
        assertThat(afterDup.getRefundReason()).isEqualTo("Manual retry");
    }

    @Test
    void paymentRefundFailedMarksRefundFailedAndNeverDowngradesRefunded() {
        UUID userId = UUID.randomUUID();
        UUID orderId = seedOrder(userId, "ORD-20261125-RFF1", OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUND_REQUIRED);
        String longErr = "E".repeat(300);

        paymentEventConsumer.handleMessage("payment.refund_failed", """
                {
                  "eventId": "evt-refund-failed-1",
                  "eventType": "PaymentRefundFailed",
                  "eventVersion": 1,
                  "aggregateId": "%s",
                  "data": {
                    "orderId": "%s",
                    "attempts": 3,
                    "resultCode": -1,
                    "message": "%s"
                  }
                }
                """.formatted(orderId, orderId, longErr));

        OrderEntity failed = orderRepository.findById(orderId).orElseThrow();
        assertThat(failed.getPaymentStatus()).isEqualTo(PaymentStatus.REFUND_FAILED);
        assertThat(failed.getRefundLastError()).hasSize(255);

        // Out-of-order PaymentRefundFailed after REFUNDED must never downgrade.
        UUID refundedId = seedOrder(userId, "ORD-20261125-RFF2", OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUNDED);
        paymentEventConsumer.handleMessage("payment.refund_failed", """
                {
                  "eventId": "evt-refund-failed-late",
                  "eventType": "PaymentRefundFailed",
                  "eventVersion": 1,
                  "aggregateId": "%s",
                  "data": {
                    "orderId": "%s",
                    "attempts": 3,
                    "resultCode": -1,
                    "message": "late failure"
                  }
                }
                """.formatted(refundedId, refundedId));

        OrderEntity stillRefunded = orderRepository.findById(refundedId).orElseThrow();
        assertThat(stillRefunded.getPaymentStatus()).isEqualTo(PaymentStatus.REFUNDED);
        assertThat(stillRefunded.getRefundLastError()).isEmpty();
    }

    @Test
    void customerOrderQueryRedactsAdminRefundErrorAndAuditFields() {
        UUID userId = UUID.randomUUID();
        UUID adminId = UUID.randomUUID();
        UUID orderId = seedOrder(userId, "ORD-20261125-RED1", OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUND_FAILED);
        OrderEntity entity = orderRepository.findById(orderId).orElseThrow();
        entity.setRefundLastError("momo gateway 503 internal error");
        entity.setRefundActorType("ADMIN");
        entity.setRefundReason("internal ops note");
        orderRepository.saveAndFlush(entity);

        when(authorizationAdapter.hasPermission(eq(userId), eq("order.view"))).thenReturn(true);
        when(authorizationAdapter.hasPermission(eq(adminId), eq("order.admin.view"))).thenReturn(true);

        OrderDetail customerDetail = orderQueryService.getOrder(userId.toString(), orderId.toString());
        assertThat(customerDetail.getPaymentStatus()).isEqualTo("REFUND_FAILED");
        assertThat(customerDetail.getRefundLastError()).isEmpty();
        assertThat(customerDetail.getRefundActorType()).isEmpty();
        assertThat(customerDetail.getRefundReason()).isEmpty();

        OrderDetail adminDetail = orderAdminQueryService.getOrder(adminId.toString(), orderId.toString()).getOrder();
        assertThat(adminDetail.getPaymentStatus()).isEqualTo("REFUND_FAILED");
        assertThat(adminDetail.getRefundLastError()).isEqualTo("momo gateway 503 internal error");
        assertThat(adminDetail.getRefundActorType()).isEqualTo("ADMIN");
        assertThat(adminDetail.getRefundReason()).isEqualTo("internal ops note");
    }

    private UUID seedOrder(UUID userId, String code, OrderStatus status, PaymentMethod method, PaymentStatus payStatus) {
        UUID orderId = UUID.randomUUID();
        OrderEntity entity = OrderEntity.builder()
                .id(orderId)
                .orderCode(code)
                .userId(userId)
                .idempotencyKey("idem-" + code)
                .status(status)
                .paymentMethod(method)
                .paymentStatus(payStatus)
                .recipientName("Le Van C")
                .recipientPhone("0909998888")
                .provinceCode("79")
                .provinceName("HCM")
                .wardCode("26734")
                .wardName("Ben Nghe")
                .addressLine("2 Hai Trieu")
                .addressLabel("")
                .currency("VND")
                .subtotalMinor(225_000L)
                .shippingFeeMinor(25_000L)
                .totalAmountMinor(250_000L)
                .payUrl(method == PaymentMethod.MOMO ? "https://momo.test/pay/" + code : "")
                .paymentTransId("")
                .paymentExpiresAt(method == PaymentMethod.MOMO ? Instant.now().plusSeconds(900) : null)
                .note("")
                .cancelReason(status == OrderStatus.CANCELLED ? "PAYMENT_EXPIRED" : "")
                .createdAt(Instant.now())
                .updatedAt(Instant.now())
                .build();
        entity.addItem(OrderItemEntity.builder()
                .id(UUID.randomUUID())
                .cartItemId("ci-1")
                .productId("p-1")
                .skuId("p-1-v1")
                .productVersion(1L)
                .productName("Product")
                .brand("Brand")
                .categoryName("Cat")
                .variantLabel("Std")
                .variantColorHex("")
                .imageUrl("")
                .imageEmoji("")
                .imageHue("")
                .unitPriceMinor(225_000L)
                .quantity(1)
                .lineTotalMinor(225_000L)
                .build());
        orderRepository.saveAndFlush(entity);
        return orderId;
    }
}
