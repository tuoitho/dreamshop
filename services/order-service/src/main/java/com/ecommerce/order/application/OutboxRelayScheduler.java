package com.ecommerce.order.application;

import com.ecommerce.order.config.RabbitMqConfig;
import jakarta.annotation.PreDestroy;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageDeliveryMode;
import org.springframework.amqp.core.MessageProperties;
import org.springframework.amqp.rabbit.connection.CachingConnectionFactory;
import org.springframework.amqp.rabbit.connection.CorrelationData;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionTemplate;

@Component
public class OutboxRelayScheduler {
    private static final Logger log = LoggerFactory.getLogger(OutboxRelayScheduler.class);
    private static final int BATCH_SIZE = 50;

    private final JdbcTemplate jdbcTemplate;
    private final TransactionTemplate transactionTemplate;
    private final RabbitTemplate rabbitTemplate;
    private final ExecutorService wakeupExecutor = Executors.newSingleThreadExecutor(r -> {
        Thread t = new Thread(r, "order-outbox-wakeup");
        t.setDaemon(true);
        return t;
    });
    private final AtomicBoolean flushQueued = new AtomicBoolean(false);
    private final Object drainLock = new Object();

    public record ClaimedOutboxRow(UUID id, String eventType, String routingKey, String payload) {
    }

    public OutboxRelayScheduler(
            JdbcTemplate jdbcTemplate,
            TransactionTemplate transactionTemplate,
            RabbitTemplate rabbitTemplate) {
        this.jdbcTemplate = jdbcTemplate;
        this.transactionTemplate = transactionTemplate;
        this.rabbitTemplate = rabbitTemplate;
    }

    public void triggerFlush() {
        if (flushQueued.compareAndSet(false, true)) {
            wakeupExecutor.submit(() -> {
                flushQueued.set(false);
                flushPendingOutbox();
            });
        }
    }

    @Scheduled(fixedDelayString = "${order.outbox.poll-interval-ms:1000}")
    public void scheduledSweep() {
        flushPendingOutbox();
    }

    public void flushPendingOutbox() {
        synchronized (drainLock) {
            while (true) {
                List<ClaimedOutboxRow> batch = claimDueBatch();
                if (batch == null || batch.isEmpty()) {
                    return;
                }
                for (ClaimedOutboxRow row : batch) {
                    try {
                        publishWithConfirm(row);
                        markPublished(row.id());
                    } catch (Exception ex) {
                        log.warn("Failed to publish order outbox event id={} routingKey={}: {}",
                                row.id(), row.routingKey(), ex.getMessage());
                        recordFailure(row.id(), ex.getMessage());
                        return;
                    }
                }
                if (batch.size() < BATCH_SIZE) {
                    return;
                }
            }
        }
    }

    private List<ClaimedOutboxRow> claimDueBatch() {
        return transactionTemplate.execute(status -> jdbcTemplate.query(
                """
                WITH due AS (
                    SELECT id
                    FROM outbox_events
                    WHERE published_at IS NULL
                      AND next_attempt_at <= now()
                    ORDER BY next_attempt_at ASC, created_at ASC
                    LIMIT ?
                    FOR UPDATE SKIP LOCKED
                )
                UPDATE outbox_events o
                SET attempts = o.attempts + 1,
                    next_attempt_at = now() + interval '15 seconds'
                FROM due
                WHERE o.id = due.id
                RETURNING o.id, o.event_type, o.routing_key, o.payload::text
                """,
                (rs, rowNum) -> new ClaimedOutboxRow(
                        UUID.fromString(rs.getString("id")),
                        rs.getString("event_type"),
                        rs.getString("routing_key"),
                        rs.getString("payload")),
                BATCH_SIZE));
    }

    private void publishWithConfirm(ClaimedOutboxRow row) throws Exception {
        MessageProperties props = new MessageProperties();
        props.setContentType(MessageProperties.CONTENT_TYPE_JSON);
        props.setDeliveryMode(MessageDeliveryMode.PERSISTENT);
        props.setMessageId(row.id().toString());
        props.setType(row.eventType());
        Message message = new Message(row.payload().getBytes(StandardCharsets.UTF_8), props);

        boolean confirmsEnabled = rabbitTemplate.getConnectionFactory() instanceof CachingConnectionFactory ccf
                && ccf.isPublisherConfirms();
        if (confirmsEnabled) {
            CorrelationData cd = new CorrelationData(row.id().toString());
            rabbitTemplate.send(RabbitMqConfig.EXCHANGE_NAME, row.routingKey(), message, cd);
            CorrelationData.Confirm confirm = cd.getFuture().get(3, TimeUnit.SECONDS);
            if (confirm == null || !confirm.ack()) {
                throw new IllegalStateException("Broker nacked outbox event " + row.id());
            }
            // Published as mandatory: a broker ack still arrives for a message no queue is bound
            // to, so a returned message must stay pending until its consumer has bound a queue.
            if (cd.getReturned() != null) {
                throw new IllegalStateException("No queue bound for routing key " + row.routingKey());
            }
        } else {
            rabbitTemplate.send(RabbitMqConfig.EXCHANGE_NAME, row.routingKey(), message);
        }
    }

    private void markPublished(UUID id) {
        jdbcTemplate.update(
                "UPDATE outbox_events SET published_at = now(), last_error = '' WHERE id = ?",
                id);
    }

    private void recordFailure(UUID id, String errorMsg) {
        String safeMsg = errorMsg == null ? "publish error" : errorMsg;
        jdbcTemplate.update(
                "UPDATE outbox_events SET last_error = ?, next_attempt_at = now() + interval '2 seconds' WHERE id = ? AND published_at IS NULL",
                safeMsg,
                id);
    }

    @PreDestroy
    public void shutdown() {
        wakeupExecutor.shutdownNow();
    }
}
