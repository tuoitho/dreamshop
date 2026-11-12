package com.ecommerce.order.application;

import com.ecommerce.order.domain.ActorType;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Confirms COD orders whose confirmation delay has elapsed. Rows are claimed with
 * {@code FOR UPDATE SKIP LOCKED}, so concurrent instances and a customer cancelling at
 * the same moment never confirm the same order twice.
 */
@Component
public class CodAutoConfirmScheduler {
    private static final int BATCH_SIZE = 50;

    private final JdbcTemplate jdbcTemplate;
    private final TransactionTemplate transactionTemplate;
    private final OrderRepository orderRepository;
    private final OrderTransactionStepService txStepService;

    public CodAutoConfirmScheduler(
            JdbcTemplate jdbcTemplate,
            TransactionTemplate transactionTemplate,
            OrderRepository orderRepository,
            OrderTransactionStepService txStepService) {
        this.jdbcTemplate = jdbcTemplate;
        this.transactionTemplate = transactionTemplate;
        this.orderRepository = orderRepository;
        this.txStepService = txStepService;
    }

    @Scheduled(fixedDelayString = "${order.cod.auto-confirm-interval-ms:5000}")
    public void confirmDueOrders() {
        transactionTemplate.executeWithoutResult(status -> {
            List<UUID> dueIds = jdbcTemplate.query(
                    """
                    SELECT id FROM orders
                    WHERE status = 'PENDING_CONFIRMATION'
                      AND auto_confirm_at <= ?
                    ORDER BY auto_confirm_at ASC
                    LIMIT ?
                    FOR UPDATE SKIP LOCKED
                    """,
                    (rs, rowNum) -> UUID.fromString(rs.getString("id")),
                    Timestamp.from(Instant.now()),
                    BATCH_SIZE);

            if (dueIds.isEmpty()) {
                return;
            }

            Instant now = Instant.now();
            for (UUID id : dueIds) {
                OrderEntity order = orderRepository.findById(id).orElse(null);
                if (order == null || order.getStatus() != OrderStatus.PENDING_CONFIRMATION) {
                    continue;
                }
                order.setStatus(OrderStatus.CONFIRMED);
                order.setUpdatedAt(now);
                orderRepository.saveAndFlush(order);
                txStepService.recordStatusChange(order, OrderStatus.PENDING_CONFIRMATION, ActorType.SYSTEM, null, "", now);
                txStepService.insertOrderConfirmedOutbox(order, now);
            }
            txStepService.registerOutboxWakeupAfterCommit();
        });
    }
}
