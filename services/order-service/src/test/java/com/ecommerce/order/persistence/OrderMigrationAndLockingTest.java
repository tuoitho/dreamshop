package com.ecommerce.order.persistence;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.order.domain.ActorType;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.support.TransactionTemplate;

class OrderMigrationAndLockingTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    JdbcTemplate jdbcTemplate;

    @Autowired
    OrderRepository orderRepository;

    @Autowired
    TransactionTemplate transactionTemplate;

    @Autowired
    OrderStatusHistoryRepository statusHistoryRepository;

    @BeforeEach
    void cleanTables() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
    }

    @Test
    void createsFlywaySchemaAndSupportsPessimisticForUpdateLocking() {
        List<String> tables = jdbcTemplate.queryForList(
                "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'",
                String.class);
        assertThat(tables).contains("orders", "order_items", "idempotency_records", "outbox_events", "processed_events");

        UUID orderId = UUID.randomUUID();
        UUID userId = UUID.randomUUID();

        transactionTemplate.executeWithoutResult(status -> {
            OrderEntity order = OrderEntity.builder()
                    .id(orderId)
                    .orderCode("ORD-20261105-LOCK01")
                    .userId(userId)
                    .idempotencyKey("idem-lock-1")
                    .status(OrderStatus.PENDING_STOCK)
                    .paymentMethod(PaymentMethod.COD)
                    .paymentStatus(PaymentStatus.UNPAID)
                    .recipientName("Nguyen Van A")
                    .recipientPhone("0901234567")
                    .provinceCode("79")
                    .provinceName("TP. Ho Chi Minh")
                    .wardCode("26734")
                    .wardName("Phuong Ben Nghe")
                    .addressLine("1 Le Duan")
                    .addressLabel("Nha rieng")
                    .currency("VND")
                    .subtotalMinor(350_000L)
                    .shippingFeeMinor(0L)
                    .totalAmountMinor(350_000L)
                    .payUrl("")
                    .paymentTransId("")
                    .note("")
                    .cancelReason("")
                    .createdAt(Instant.now())
                    .updatedAt(Instant.now())
                    .build();
            order.addItem(OrderItemEntity.builder()
                    .id(UUID.randomUUID())
                    .cartItemId("ci-1")
                    .productId("prod-1")
                    .skuId("prod-1-v1")
                    .productVersion(3L)
                    .productName("Ao thun")
                    .brand("BrandX")
                    .categoryName("Thoi trang")
                    .variantLabel("Den / L")
                    .variantColorHex("#000000")
                    .imageUrl("https://cdn.example/1.jpg")
                    .imageEmoji("")
                    .imageHue("")
                    .unitPriceMinor(350_000L)
                    .quantity(1)
                    .lineTotalMinor(350_000L)
                    .build());
            orderRepository.saveAndFlush(order);
        });

        OrderEntity locked = transactionTemplate.execute(status -> {
            OrderEntity found = orderRepository.findByIdForUpdate(orderId).orElseThrow();
            found.setStatus(OrderStatus.CONFIRMED);
            found.setUpdatedAt(Instant.now());
            return orderRepository.saveAndFlush(found);
        });

        assertThat(locked).isNotNull();
        assertThat(locked.getStatus()).isEqualTo(OrderStatus.CONFIRMED);
        assertThat(locked.getVersion()).isGreaterThanOrEqualTo(1L);
    }

    @Test
    void persistsFulfillmentFieldsAndStatusHistory() {
        UUID orderId = UUID.randomUUID();
        Instant autoConfirmAt = Instant.parse("2026-11-12T08:00:00Z");
        orderRepository.saveAndFlush(OrderEntity.builder()
                .id(orderId)
                .orderCode("ORD-20261112-DELIV1")
                .userId(UUID.randomUUID())
                .idempotencyKey("idem-deliv-1")
                .status(OrderStatus.DELIVERED)
                .paymentMethod(PaymentMethod.COD)
                .paymentStatus(PaymentStatus.PAID)
                .recipientName("Nguyen Van A")
                .recipientPhone("0901234567")
                .provinceCode("79")
                .provinceName("TP. Ho Chi Minh")
                .wardCode("26734")
                .wardName("Phuong Ben Nghe")
                .addressLine("1 Le Duan")
                .subtotalMinor(100_000L)
                .shippingFeeMinor(0L)
                .totalAmountMinor(100_000L)
                .autoConfirmAt(autoConfirmAt)
                .shippingCarrier("GHN")
                .trackingNumber("T1")
                .createdAt(Instant.now())
                .updatedAt(Instant.now())
                .build());
        UUID adminId = UUID.randomUUID();
        statusHistoryRepository.saveAndFlush(OrderStatusHistoryEntity.builder()
                .id(UUID.randomUUID())
                .orderId(orderId)
                .fromStatus(OrderStatus.SHIPPING)
                .toStatus(OrderStatus.DELIVERED)
                .actorType(ActorType.ADMIN)
                .actorId(adminId)
                .shippingCarrier("GHN")
                .trackingNumber("T1")
                .createdAt(Instant.now())
                .build());

        OrderEntity reloaded = orderRepository.findById(orderId).orElseThrow();
        assertThat(reloaded.getStatus()).isEqualTo(OrderStatus.DELIVERED);
        assertThat(reloaded.getAutoConfirmAt()).isEqualTo(autoConfirmAt);
        assertThat(reloaded.getShippingCarrier()).isEqualTo("GHN");
        assertThat(reloaded.getTrackingNumber()).isEqualTo("T1");

        List<OrderStatusHistoryEntity> history = statusHistoryRepository.findByOrderIdOrderByCreatedAtAsc(orderId);
        assertThat(history).singleElement().satisfies(entry -> {
            assertThat(entry.getFromStatus()).isEqualTo(OrderStatus.SHIPPING);
            assertThat(entry.getToStatus()).isEqualTo(OrderStatus.DELIVERED);
            assertThat(entry.getActorType()).isEqualTo(ActorType.ADMIN);
            assertThat(entry.getActorId()).isEqualTo(adminId);
            assertThat(entry.getReason()).isEmpty();
        });
    }

    @Test
    void persistsRefundStatusesAuditColumnsAndSupportsBackfillUpdate() {
        UUID refundedId = UUID.randomUUID();
        UUID failedId = UUID.randomUUID();
        UUID adminId = UUID.randomUUID();
        Instant refundedAt = Instant.parse("2026-11-25T05:00:00Z");

        orderRepository.saveAndFlush(OrderEntity.builder()
                .id(refundedId)
                .orderCode("ORD-20261125-REF01")
                .userId(UUID.randomUUID())
                .idempotencyKey("idem-ref-1")
                .status(OrderStatus.CANCELLED)
                .paymentMethod(PaymentMethod.MOMO)
                .paymentStatus(PaymentStatus.REFUNDED)
                .recipientName("Nguyen Van B")
                .recipientPhone("0901234567")
                .provinceCode("79")
                .provinceName("HCM")
                .wardCode("26734")
                .wardName("Ben Nghe")
                .addressLine("1 Le Duan")
                .subtotalMinor(150_000L)
                .shippingFeeMinor(0L)
                .totalAmountMinor(150_000L)
                .refundTransId("momo-ref-123")
                .refundLastError("")
                .refundedAt(refundedAt)
                .refundActorType("ADMIN")
                .refundActorId(adminId)
                .refundReason("MoMo recovered")
                .createdAt(Instant.now())
                .updatedAt(Instant.now())
                .build());

        orderRepository.saveAndFlush(OrderEntity.builder()
                .id(failedId)
                .orderCode("ORD-20261125-REF02")
                .userId(UUID.randomUUID())
                .idempotencyKey("idem-ref-2")
                .status(OrderStatus.CANCELLED)
                .paymentMethod(PaymentMethod.MOMO)
                .paymentStatus(PaymentStatus.REFUND_FAILED)
                .recipientName("Nguyen Van C")
                .recipientPhone("0901234567")
                .provinceCode("79")
                .provinceName("HCM")
                .wardCode("26734")
                .wardName("Ben Nghe")
                .addressLine("2 Le Duan")
                .subtotalMinor(150_000L)
                .shippingFeeMinor(0L)
                .totalAmountMinor(150_000L)
                .refundLastError("gateway 503")
                .createdAt(Instant.now())
                .updatedAt(Instant.now())
                .build());

        OrderEntity reloadedRefunded = orderRepository.findById(refundedId).orElseThrow();
        assertThat(reloadedRefunded.getPaymentStatus()).isEqualTo(PaymentStatus.REFUNDED);
        assertThat(reloadedRefunded.getRefundTransId()).isEqualTo("momo-ref-123");
        assertThat(reloadedRefunded.getRefundedAt()).isEqualTo(refundedAt);
        assertThat(reloadedRefunded.getRefundActorType()).isEqualTo("ADMIN");
        assertThat(reloadedRefunded.getRefundActorId()).isEqualTo(adminId);
        assertThat(reloadedRefunded.getRefundReason()).isEqualTo("MoMo recovered");

        OrderEntity reloadedFailed = orderRepository.findById(failedId).orElseThrow();
        assertThat(reloadedFailed.getPaymentStatus()).isEqualTo(PaymentStatus.REFUND_FAILED);
        assertThat(reloadedFailed.getRefundLastError()).isEqualTo("gateway 503");

        // Verify V3 backfill SQL semantics on a CANCELLED + MOMO + REFUND_REQUIRED row.
        UUID legacyId = UUID.randomUUID();
        orderRepository.saveAndFlush(OrderEntity.builder()
                .id(legacyId)
                .orderCode("ORD-20261125-LEG01")
                .userId(UUID.randomUUID())
                .idempotencyKey("idem-leg-1")
                .status(OrderStatus.CANCELLED)
                .paymentMethod(PaymentMethod.MOMO)
                .paymentStatus(PaymentStatus.REFUND_REQUIRED)
                .recipientName("Legacy")
                .recipientPhone("0901234567")
                .provinceCode("79")
                .provinceName("HCM")
                .wardCode("26734")
                .wardName("Ben Nghe")
                .addressLine("3 Le Duan")
                .subtotalMinor(100_000L)
                .shippingFeeMinor(0L)
                .totalAmountMinor(100_000L)
                .createdAt(Instant.now())
                .updatedAt(Instant.now())
                .build());

        int updated = jdbcTemplate.update("""
                UPDATE orders
                SET payment_status = 'REFUND_FAILED',
                    refund_last_error = 'Đơn cần hoàn tiền phát sinh trước khi bật hoàn tiền tự động',
                    updated_at = now()
                WHERE status = 'CANCELLED'
                  AND payment_method = 'MOMO'
                  AND payment_status = 'REFUND_REQUIRED'
                """);
        assertThat(updated).isEqualTo(1);
        OrderEntity backfilled = orderRepository.findById(legacyId).orElseThrow();
        assertThat(backfilled.getPaymentStatus()).isEqualTo(PaymentStatus.REFUND_FAILED);
        assertThat(backfilled.getRefundLastError()).isNotBlank();
    }
}
