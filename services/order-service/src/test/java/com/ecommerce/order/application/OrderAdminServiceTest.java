package com.ecommerce.order.application;

import static com.ecommerce.order.support.OrderFixtures.seedOrder;
import static com.ecommerce.order.support.StatusHistoryQueries.historyOf;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

import com.ecommerce.contracts.order.v1.AdminGetOrderResponse;
import com.ecommerce.contracts.order.v1.AdminListOrdersResponse;
import com.ecommerce.contracts.order.v1.AdminRetryRefundResponse;
import com.ecommerce.contracts.order.v1.AdminUpdateOrderStatusResponse;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.integration.AuthorizationGrpcAdapter;
import com.ecommerce.order.integration.PaymentGrpcAdapter;
import io.grpc.Status;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.util.UUID;
import org.assertj.core.api.ThrowableAssert.ThrowingCallable;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.bean.override.mockito.MockitoBean;

class OrderAdminServiceTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    OrderAdminQueryService queryService;

    @Autowired
    OrderLifecycleService lifecycleService;

    @Autowired
    OrderRepository orderRepository;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @MockitoBean
    AuthorizationGrpcAdapter authorizationAdapter;

    @MockitoBean
    PaymentGrpcAdapter paymentGrpcAdapter;

    private final UUID adminId = UUID.randomUUID();

    @BeforeEach
    void setUp() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
        reset(authorizationAdapter, paymentGrpcAdapter);
        when(authorizationAdapter.hasPermission(eq(adminId), eq("order.admin.view"))).thenReturn(true);
        when(authorizationAdapter.hasPermission(eq(adminId), eq("order.admin.update"))).thenReturn(true);
    }

    @Test
    void missingPermissionsAreRejected() {
        UUID orderId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-PERM1",
                OrderStatus.CONFIRMED, PaymentMethod.COD, PaymentStatus.UNPAID);
        String stranger = UUID.randomUUID().toString();
        when(authorizationAdapter.hasPermission(any(UUID.class), eq("order.view"))).thenReturn(true);

        assertReason(() -> queryService.listOrders(stranger, "", "", 1, 20), "ORDER_PERMISSION_DENIED");
        assertReason(() -> queryService.getOrder(stranger, orderId.toString()), "ORDER_PERMISSION_DENIED");
        assertReason(() -> update(stranger, orderId, "CONFIRMED", "PREPARING", ""), "ORDER_PERMISSION_DENIED");
    }

    @Test
    void listFiltersByStatusAndCodePrefixAndCountsEveryStatus() {
        seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-LIST1", OrderStatus.CONFIRMED, PaymentMethod.COD, PaymentStatus.UNPAID);
        seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-LIST2", OrderStatus.CONFIRMED, PaymentMethod.COD, PaymentStatus.UNPAID);
        seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-LIST3", OrderStatus.SHIPPING, PaymentMethod.COD, PaymentStatus.UNPAID);
        seedOrder(orderRepository, UUID.randomUUID(), "XYZ-1", OrderStatus.SHIPPING, PaymentMethod.COD, PaymentStatus.UNPAID);

        AdminListOrdersResponse confirmed = queryService.listOrders(adminId.toString(), "CONFIRMED", "ord-2026", 1, 20);

        assertThat(confirmed.getItemsList()).extracting(o -> o.getOrderCode())
                .containsExactlyInAnyOrder("ORD-20261112-LIST1", "ORD-20261112-LIST2");
        assertThat(confirmed.getTotalItems()).isEqualTo(2);
        assertThat(confirmed.getStatusCountsMap())
                .containsEntry("CONFIRMED", 2L)
                .containsEntry("SHIPPING", 1L)
                .containsEntry("DELIVERED", 0L)
                .hasSize(OrderStatus.values().length);
        assertThat(confirmed.getItems(0).getStatusHistoryList()).isEmpty();

        AdminListOrdersResponse all = queryService.listOrders(adminId.toString(), "", "", 0, 0);
        assertThat(all.getTotalItems()).isEqualTo(4);
        assertThat(all.getPage()).isEqualTo(1);
        assertThat(all.getPageSize()).isEqualTo(20);

        assertReason(() -> queryService.listOrders(adminId.toString(), "SHIPPED", "", 1, 20), "INVALID_PARAMETER");
    }

    @Test
    void getReturnsHistoryAndAllowedNextStatuses() {
        UUID orderId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-GET01",
                OrderStatus.CONFIRMED, PaymentMethod.COD, PaymentStatus.UNPAID);

        AdminGetOrderResponse response = queryService.getOrder(adminId.toString(), orderId.toString());

        assertThat(response.getOrder().getOrderCode()).isEqualTo("ORD-20261112-GET01");
        assertThat(response.getAllowedNextStatusesList()).containsExactly("PREPARING", "CANCELLED");
        assertReason(() -> queryService.getOrder(adminId.toString(), UUID.randomUUID().toString()), "ORDER_NOT_FOUND");
    }

    @Test
    void codOrderWalksFulfillmentFlowAndIsPaidOnDelivery() {
        UUID orderId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-FLOW1",
                OrderStatus.CONFIRMED, PaymentMethod.COD, PaymentStatus.UNPAID);

        update(adminId.toString(), orderId, "CONFIRMED", "PREPARING", "");
        AdminUpdateOrderStatusResponse shipping = lifecycleService.updateStatusByAdmin(
                adminId.toString(), orderId.toString(), "PREPARING", "SHIPPING", "", " GHN ", " T1 ");
        assertThat(shipping.getAllowedNextStatusesList()).containsExactly("DELIVERED", "DELIVERY_FAILED");
        AdminUpdateOrderStatusResponse delivered = update(adminId.toString(), orderId, "SHIPPING", "DELIVERED", "");

        assertThat(delivered.getOrder().getStatus()).isEqualTo("DELIVERED");
        assertThat(delivered.getOrder().getPaymentStatus()).isEqualTo("PAID");
        assertThat(delivered.getOrder().getShippingCarrier()).isEqualTo("GHN");
        assertThat(delivered.getOrder().getTrackingNumber()).isEqualTo("T1");
        assertThat(delivered.getOrder().getStatusHistoryList()).hasSize(3);
        assertThat(delivered.getAllowedNextStatusesList()).isEmpty();
        assertThat(historyOf(jdbcTemplate, orderId)).containsExactly(
                "CONFIRMED>PREPARING:ADMIN", "PREPARING>SHIPPING:ADMIN", "SHIPPING>DELIVERED:ADMIN");
        assertThat(jdbcTemplate.queryForObject(
                "SELECT tracking_number FROM order_status_history WHERE order_id = ? AND to_status = 'SHIPPING'",
                String.class, orderId)).isEqualTo("T1");
        assertThat(jdbcTemplate.queryForObject(
                "SELECT shipping_carrier || '|' || tracking_number FROM order_status_history WHERE order_id = ? AND to_status = 'DELIVERED'",
                String.class, orderId)).isEqualTo("|");
        assertThat(jdbcTemplate.queryForObject(
                "SELECT COUNT(*) FROM order_status_history WHERE order_id = ? AND actor_id = ?",
                Integer.class, orderId, adminId)).isEqualTo(3);
        assertThat(jdbcTemplate.queryForList(
                "SELECT routing_key FROM outbox_events WHERE aggregate_id = ? ORDER BY created_at",
                String.class, orderId.toString())).containsExactly("order.delivered");
        String deliveredSummary = jdbcTemplate.queryForObject(
                """
                SELECT payload->>'eventType'
                    || '|' || (payload->'data'->>'orderCode')
                    || '|' || (payload->'data'->'items'->0->>'variantLabel')
                    || '|' || CASE WHEN COALESCE(payload->'data'->>'deliveredAt', '') <> '' THEN 'OK' ELSE '' END
                FROM outbox_events
                WHERE aggregate_id = ? AND routing_key = 'order.delivered'
                """,
                String.class, orderId.toString());
        assertThat(deliveredSummary).isEqualTo("OrderDelivered|ORD-20261112-FLOW1|Std|OK");
    }

    @Test
    void redeliveryKeepsShippingDetailsUnlessNewOnesAreGiven() {
        UUID orderId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-RESHIP",
                OrderStatus.PREPARING, PaymentMethod.COD, PaymentStatus.UNPAID);
        lifecycleService.updateStatusByAdmin(adminId.toString(), orderId.toString(), "PREPARING", "SHIPPING", "", "GHN", "T1");
        update(adminId.toString(), orderId, "SHIPPING", "DELIVERY_FAILED", "Khong lien lac duoc");

        AdminUpdateOrderStatusResponse retried = update(adminId.toString(), orderId, "DELIVERY_FAILED", "SHIPPING", "");

        assertThat(retried.getOrder().getShippingCarrier()).isEqualTo("GHN");
        assertThat(retried.getOrder().getTrackingNumber()).isEqualTo("T1");
        update(adminId.toString(), orderId, "SHIPPING", "DELIVERY_FAILED", "Van khong lien lac duoc");
        AdminUpdateOrderStatusResponse replaced = lifecycleService.updateStatusByAdmin(
                adminId.toString(), orderId.toString(), "DELIVERY_FAILED", "SHIPPING", "", "GHTK", "T2");
        assertThat(replaced.getOrder().getShippingCarrier()).isEqualTo("GHTK");
        assertThat(replaced.getOrder().getTrackingNumber()).isEqualTo("T2");
    }

    @Test
    void staleExpectedStatusIsAConflictAndChangesNothing() {
        UUID orderId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-STALE",
                OrderStatus.PREPARING, PaymentMethod.COD, PaymentStatus.UNPAID);

        assertReason(() -> update(adminId.toString(), orderId, "CONFIRMED", "PREPARING", ""), "ORDER_STATUS_CONFLICT");

        assertThat(orderRepository.findById(orderId).orElseThrow().getStatus()).isEqualTo(OrderStatus.PREPARING);
        assertThat(historyOf(jdbcTemplate, orderId)).isEmpty();
    }

    @Test
    void invalidTransitionsAndMissingReasonsAreRejected() {
        UUID confirmedId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-BAD01",
                OrderStatus.CONFIRMED, PaymentMethod.COD, PaymentStatus.UNPAID);
        UUID shippingId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-BAD02",
                OrderStatus.SHIPPING, PaymentMethod.COD, PaymentStatus.UNPAID);

        assertReason(() -> update(adminId.toString(), confirmedId, "CONFIRMED", "DELIVERED", ""), "ORDER_INVALID_TRANSITION");
        assertReason(() -> update(adminId.toString(), shippingId, "SHIPPING", "DELIVERY_FAILED", "  "), "ORDER_REASON_REQUIRED");
        assertReason(() -> update(adminId.toString(), shippingId, "SHIPPING", "DELIVERY_FAILED", "x".repeat(129)), "INVALID_PARAMETER");
        assertReason(() -> lifecycleService.updateStatusByAdmin(adminId.toString(), shippingId.toString(),
                "SHIPPING", "DELIVERED", "", "x".repeat(101), ""), "INVALID_PARAMETER");
        assertReason(() -> update(adminId.toString(), shippingId, "", "DELIVERED", ""), "INVALID_PARAMETER");
        assertReason(() -> update(adminId.toString(), shippingId, "SHIPPING", "LOST", ""), "INVALID_PARAMETER");

        AdminUpdateOrderStatusResponse failed = update(adminId.toString(), shippingId, "SHIPPING", "DELIVERY_FAILED", "Khong lien lac duoc");
        assertThat(failed.getOrder().getStatus()).isEqualTo("DELIVERY_FAILED");
        assertThat(failed.getOrder().getPaymentStatus()).isEqualTo("UNPAID");
    }

    @Test
    void adminCancellingPaidOrderRestocksAndRequestsRefund() {
        UUID orderId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261112-ACAN1",
                OrderStatus.PREPARING, PaymentMethod.MOMO, PaymentStatus.PAID);

        AdminUpdateOrderStatusResponse cancelled = update(adminId.toString(), orderId, "PREPARING", "CANCELLED", "Het hang");

        OrderEntity order = orderRepository.findById(orderId).orElseThrow();
        assertThat(cancelled.getOrder().getStatus()).isEqualTo("CANCELLED");
        assertThat(order.getPaymentStatus()).isEqualTo(PaymentStatus.REFUND_REQUIRED);
        assertThat(order.getCancelReason()).isEqualTo("Het hang");
        assertThat(jdbcTemplate.queryForList(
                "SELECT routing_key FROM outbox_events WHERE aggregate_id = ? ORDER BY routing_key",
                String.class, orderId.toString()))
                .containsExactly("order.payment.refund_requested", "order.stock.restock_requested");
        assertThat(historyOf(jdbcTemplate, orderId)).containsExactly("PREPARING>CANCELLED:ADMIN");
    }

    @Test
    void adminRetryRefundRequiresPermissionAndRefundFailedStatus() {
        UUID refundRequiredId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261125-RREQ",
                OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUND_REQUIRED);
        UUID paidId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261125-RPAID",
                OrderStatus.CONFIRMED, PaymentMethod.MOMO, PaymentStatus.PAID);
        UUID codId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261125-RCOD",
                OrderStatus.CANCELLED, PaymentMethod.COD, PaymentStatus.UNPAID);
        UUID refundedId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261125-RDONE",
                OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUNDED);
        UUID refundFailedId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261125-RFAIL",
                OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUND_FAILED);

        String stranger = UUID.randomUUID().toString();
        assertReason(() -> lifecycleService.retryRefundByAdmin(stranger, refundFailedId.toString(), "retry"),
                "ORDER_PERMISSION_DENIED");
        assertReason(() -> lifecycleService.retryRefundByAdmin(adminId.toString(), refundRequiredId.toString(), "retry"),
                "REFUND_AUTO_RETRY_IN_PROGRESS");
        assertReason(() -> lifecycleService.retryRefundByAdmin(adminId.toString(), paidId.toString(), "retry"),
                "ORDER_REFUND_NOT_ELIGIBLE");
        assertReason(() -> lifecycleService.retryRefundByAdmin(adminId.toString(), codId.toString(), "retry"),
                "ORDER_REFUND_NOT_ELIGIBLE");
        assertReason(() -> lifecycleService.retryRefundByAdmin(adminId.toString(), refundedId.toString(), "retry"),
                "ORDER_REFUND_NOT_ELIGIBLE");

        verifyNoInteractions(paymentGrpcAdapter);
    }

    @Test
    void adminRetryRefundSuccessUpdatesOrderToRefundedWithAdminAudit() {
        UUID orderId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261125-ROK",
                OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUND_FAILED);
        OrderEntity seeded = orderRepository.findById(orderId).orElseThrow();
        seeded.setRefundLastError("old error");
        orderRepository.saveAndFlush(seeded);

        when(paymentGrpcAdapter.retryRefund(eq(orderId), eq("MoMo recovered")))
                .thenReturn(new PaymentGrpcAdapter.RetryRefundOutcome("REFUNDED", "momo-ref-1", "Successful."));

        int historyBefore = historyOf(jdbcTemplate, orderId).size();
        AdminRetryRefundResponse resp = lifecycleService.retryRefundByAdmin(
                adminId.toString(), orderId.toString(), "  MoMo recovered  ");

        assertThat(resp.getOrder().getPaymentStatus()).isEqualTo("REFUNDED");
        assertThat(resp.getOrder().getRefundTransId()).isEqualTo("momo-ref-1");
        assertThat(resp.getOrder().getRefundActorType()).isEqualTo("ADMIN");
        assertThat(resp.getOrder().getRefundReason()).isEqualTo("MoMo recovered");
        assertThat(resp.getOrder().getRefundLastError()).isEmpty();
        assertThat(resp.getOrder().getRefundedAt()).isNotBlank();

        OrderEntity updated = orderRepository.findById(orderId).orElseThrow();
        assertThat(updated.getPaymentStatus()).isEqualTo(PaymentStatus.REFUNDED);
        assertThat(updated.getRefundTransId()).isEqualTo("momo-ref-1");
        assertThat(updated.getRefundActorType()).isEqualTo("ADMIN");
        assertThat(updated.getRefundActorId()).isEqualTo(adminId);
        assertThat(updated.getRefundReason()).isEqualTo("MoMo recovered");
        assertThat(updated.getRefundLastError()).isEmpty();
        assertThat(historyOf(jdbcTemplate, orderId)).hasSize(historyBefore);
    }

    @Test
    void adminRetryRefundGatewayFailurePersistsLatestErrorAndRethrows() {
        UUID orderId = seedOrder(orderRepository, UUID.randomUUID(), "ORD-20261125-RERR",
                OrderStatus.CANCELLED, PaymentMethod.MOMO, PaymentStatus.REFUND_FAILED);

        when(paymentGrpcAdapter.retryRefund(eq(orderId), eq("retry now")))
                .thenThrow(new DomainException(
                        Status.Code.UNAVAILABLE, "PAYMENT_GATEWAY_UNAVAILABLE", "MoMo 503 outage"));

        assertReason(() -> lifecycleService.retryRefundByAdmin(adminId.toString(), orderId.toString(), "retry now"),
                "PAYMENT_GATEWAY_UNAVAILABLE");

        OrderEntity reloaded = orderRepository.findById(orderId).orElseThrow();
        assertThat(reloaded.getPaymentStatus()).isEqualTo(PaymentStatus.REFUND_FAILED);
        assertThat(reloaded.getRefundLastError()).isEqualTo("MoMo 503 outage");
    }

    private AdminUpdateOrderStatusResponse update(String userId, UUID orderId, String expected, String target, String reason) {
        return lifecycleService.updateStatusByAdmin(userId, orderId.toString(), expected, target, reason, "", "");
    }

    private static void assertReason(ThrowingCallable call, String reason) {
        assertThatThrownBy(call)
                .isInstanceOf(DomainException.class)
                .extracting(e -> ((DomainException) e).reason())
                .isEqualTo(reason);
    }
}
