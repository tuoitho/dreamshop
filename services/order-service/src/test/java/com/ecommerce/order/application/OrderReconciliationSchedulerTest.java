package com.ecommerce.order.application;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.persistence.IdempotencyRecordEntity;
import com.ecommerce.order.persistence.IdempotencyRecordId;
import com.ecommerce.order.persistence.IdempotencyRecordRepository;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;

class OrderReconciliationSchedulerTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    OrderReconciliationScheduler scheduler;

    @Autowired
    OrderRepository orderRepository;

    @Autowired
    IdempotencyRecordRepository idempotencyRepository;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void cleanTables() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
    }

    @Test
    void reconcilesStalePendingStockAndExpiredPendingPaymentOrders() {
        UUID userId = UUID.randomUUID();
        UUID staleStockId = UUID.randomUUID();
        Instant threeMinutesAgo = Instant.now().minusSeconds(180);

        orderRepository.saveAndFlush(OrderEntity.builder()
                .id(staleStockId)
                .orderCode("ORD-20261105-STALE1")
                .userId(userId)
                .idempotencyKey("idem-stale-1")
                .status(OrderStatus.PENDING_STOCK)
                .paymentMethod(PaymentMethod.MOMO)
                .paymentStatus(PaymentStatus.PENDING)
                .recipientName("User")
                .recipientPhone("0900000001")
                .provinceCode("79")
                .provinceName("HCM")
                .wardCode("26734")
                .wardName("Ward")
                .addressLine("1 Street")
                .addressLabel("")
                .currency("VND")
                .subtotalMinor(300_000L)
                .shippingFeeMinor(0L)
                .totalAmountMinor(300_000L)
                .payUrl("")
                .paymentTransId("")
                .paymentExpiresAt(Instant.now().plusSeconds(600))
                .note("")
                .cancelReason("")
                .createdAt(threeMinutesAgo)
                .updatedAt(threeMinutesAgo)
                .build());

        idempotencyRepository.saveAndFlush(IdempotencyRecordEntity.builder()
                .id(new IdempotencyRecordId(userId, "idem-stale-1"))
                .requestFingerprint("fp-stale-1")
                .status("IN_PROGRESS")
                .orderId(staleStockId)
                .failureCode("")
                .failureMessage("")
                .expiresAt(Instant.now().plusSeconds(3600))
                .createdAt(threeMinutesAgo)
                .updatedAt(threeMinutesAgo)
                .build());

        UUID expiredPaymentId = UUID.randomUUID();
        orderRepository.saveAndFlush(OrderEntity.builder()
                .id(expiredPaymentId)
                .orderCode("ORD-20261105-EXP001")
                .userId(userId)
                .idempotencyKey("idem-exp-1")
                .status(OrderStatus.PENDING_PAYMENT)
                .paymentMethod(PaymentMethod.MOMO)
                .paymentStatus(PaymentStatus.PENDING)
                .recipientName("User")
                .recipientPhone("0900000001")
                .provinceCode("79")
                .provinceName("HCM")
                .wardCode("26734")
                .wardName("Ward")
                .addressLine("1 Street")
                .addressLabel("")
                .currency("VND")
                .subtotalMinor(300_000L)
                .shippingFeeMinor(0L)
                .totalAmountMinor(300_000L)
                .payUrl("https://momo.test/pay/exp")
                .paymentTransId("")
                .paymentExpiresAt(Instant.now().minusSeconds(30))
                .note("")
                .cancelReason("")
                .createdAt(Instant.now().minusSeconds(930))
                .updatedAt(Instant.now().minusSeconds(930))
                .build());

        scheduler.reconcileStaleOrders();

        OrderEntity staleUpdated = orderRepository.findById(staleStockId).orElseThrow();
        assertThat(staleUpdated.getStatus()).isEqualTo(OrderStatus.CANCELLED);

        IdempotencyRecordEntity idemUpdated = idempotencyRepository
                .findById(new IdempotencyRecordId(userId, "idem-stale-1"))
                .orElseThrow();
        assertThat(idemUpdated.getStatus()).isEqualTo("FAILED_TERMINAL");

        List<String> staleOutbox = jdbcTemplate.queryForList(
                "SELECT routing_key FROM outbox_events WHERE aggregate_id = ? ORDER BY routing_key ASC",
                String.class,
                staleStockId.toString());
        assertThat(staleOutbox).containsExactly("order.payment.cancel_requested", "order.reservation.release_requested");

        OrderEntity expUpdated = orderRepository.findById(expiredPaymentId).orElseThrow();
        assertThat(expUpdated.getStatus()).isEqualTo(OrderStatus.CANCELLED);
        assertThat(expUpdated.getCancelReason()).isEqualTo("PAYMENT_EXPIRED");

        List<String> expOutbox = jdbcTemplate.queryForList(
                "SELECT routing_key FROM outbox_events WHERE aggregate_id = ? ORDER BY routing_key ASC",
                String.class,
                expiredPaymentId.toString());
        assertThat(expOutbox).containsExactly("order.payment.cancel_requested", "order.reservation.release_requested");
    }
}
