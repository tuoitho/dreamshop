package com.ecommerce.order.application;

import com.ecommerce.order.config.OrderProperties;
import com.ecommerce.order.config.RabbitMqConfig;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.persistence.IdempotencyRecordId;
import com.ecommerce.order.persistence.IdempotencyRecordRepository;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionTemplate;

@Component
public class OrderReconciliationScheduler {
    private static final int BATCH_SIZE = 50;

    private final JdbcTemplate jdbcTemplate;
    private final TransactionTemplate transactionTemplate;
    private final OrderRepository orderRepository;
    private final IdempotencyRecordRepository idempotencyRepository;
    private final OrderTransactionStepService txStepService;
    private final OrderProperties properties;

    public OrderReconciliationScheduler(
            JdbcTemplate jdbcTemplate,
            TransactionTemplate transactionTemplate,
            OrderRepository orderRepository,
            IdempotencyRecordRepository idempotencyRepository,
            OrderTransactionStepService txStepService,
            OrderProperties properties) {
        this.jdbcTemplate = jdbcTemplate;
        this.transactionTemplate = transactionTemplate;
        this.orderRepository = orderRepository;
        this.idempotencyRepository = idempotencyRepository;
        this.txStepService = txStepService;
        this.properties = properties;
    }

    @Scheduled(fixedDelayString = "${order.reconciliation.interval-ms:30000}")
    public void reconcileStaleOrders() {
        reconcileStalePendingStock();
        reconcileExpiredPendingPayment();
    }

    private void reconcileStalePendingStock() {
        Duration timeout = properties.reconciliation() != null && properties.reconciliation().pendingStockTimeout() != null
                ? properties.reconciliation().pendingStockTimeout()
                : Duration.ofMinutes(2);
        Instant cutoff = Instant.now().minus(timeout);

        transactionTemplate.executeWithoutResult(status -> {
            List<UUID> staleIds = jdbcTemplate.query(
                    """
                    SELECT id FROM orders
                    WHERE status = 'PENDING_STOCK'
                      AND created_at <= ?
                    ORDER BY created_at ASC
                    LIMIT ?
                    FOR UPDATE SKIP LOCKED
                    """,
                    (rs, rowNum) -> UUID.fromString(rs.getString("id")),
                    Timestamp.from(cutoff),
                    BATCH_SIZE);

            if (staleIds.isEmpty()) {
                return;
            }

            Instant now = Instant.now();
            for (UUID id : staleIds) {
                OrderEntity order = orderRepository.findById(id).orElse(null);
                if (order == null || order.getStatus() != OrderStatus.PENDING_STOCK) {
                    continue;
                }
                order.setStatus(OrderStatus.CANCELLED);
                if (order.getPaymentMethod() == PaymentMethod.MOMO) {
                    order.setPaymentStatus(PaymentStatus.FAILED);
                }
                order.setCancelReason("PENDING_STOCK_TIMEOUT");
                order.setPayUrl("");
                order.setUpdatedAt(now);
                orderRepository.saveAndFlush(order);

                idempotencyRepository.findById(new IdempotencyRecordId(order.getUserId(), order.getIdempotencyKey()))
                        .ifPresent(rec -> {
                            if (IdempotencyService.STATUS_IN_PROGRESS.equals(rec.getStatus())) {
                                rec.setStatus(IdempotencyService.STATUS_FAILED_TERMINAL);
                                rec.setFailureCode("ORDER_RECONCILED_TIMEOUT");
                                rec.setFailureMessage("Order creation timed out and was reconciled");
                                rec.setUpdatedAt(now);
                                idempotencyRepository.saveAndFlush(rec);
                            }
                        });

                txStepService.insertCompensationOutbox(
                        "ReservationReleaseRequested",
                        RabbitMqConfig.RK_RESERVATION_RELEASE_REQUESTED,
                        order.getId(),
                        "PENDING_STOCK_TIMEOUT",
                        now);
                if (order.getPaymentMethod() == PaymentMethod.MOMO) {
                    txStepService.insertCompensationOutbox(
                            "PaymentCancelRequested",
                            RabbitMqConfig.RK_PAYMENT_CANCEL_REQUESTED,
                            order.getId(),
                            "PENDING_STOCK_TIMEOUT",
                            now);
                }
            }
            txStepService.registerOutboxWakeupAfterCommit();
        });
    }

    private void reconcileExpiredPendingPayment() {
        Instant nowCutoff = Instant.now();
        transactionTemplate.executeWithoutResult(status -> {
            List<UUID> expiredIds = jdbcTemplate.query(
                    """
                    SELECT id FROM orders
                    WHERE status = 'PENDING_PAYMENT'
                      AND payment_expires_at IS NOT NULL
                      AND payment_expires_at <= ?
                    ORDER BY payment_expires_at ASC
                    LIMIT ?
                    FOR UPDATE SKIP LOCKED
                    """,
                    (rs, rowNum) -> UUID.fromString(rs.getString("id")),
                    Timestamp.from(nowCutoff),
                    BATCH_SIZE);

            if (expiredIds.isEmpty()) {
                return;
            }

            Instant now = Instant.now();
            for (UUID id : expiredIds) {
                OrderEntity order = orderRepository.findById(id).orElse(null);
                if (order == null || order.getStatus() != OrderStatus.PENDING_PAYMENT) {
                    continue;
                }
                order.setStatus(OrderStatus.CANCELLED);
                if (order.getPaymentStatus() == PaymentStatus.PENDING) {
                    order.setPaymentStatus(PaymentStatus.FAILED);
                }
                order.setCancelReason("PAYMENT_EXPIRED");
                order.setPayUrl("");
                order.setUpdatedAt(now);
                orderRepository.saveAndFlush(order);

                txStepService.insertCompensationOutbox(
                        "ReservationReleaseRequested",
                        RabbitMqConfig.RK_RESERVATION_RELEASE_REQUESTED,
                        order.getId(),
                        "PAYMENT_EXPIRED",
                        now);
                txStepService.insertCompensationOutbox(
                        "PaymentCancelRequested",
                        RabbitMqConfig.RK_PAYMENT_CANCEL_REQUESTED,
                        order.getId(),
                        "PAYMENT_EXPIRED",
                        now);
            }
            txStepService.registerOutboxWakeupAfterCommit();
        });
    }
}
