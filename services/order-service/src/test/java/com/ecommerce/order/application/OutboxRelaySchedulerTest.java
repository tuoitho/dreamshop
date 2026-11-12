package com.ecommerce.order.application;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.order.config.RabbitMqConfig;
import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.amqp.rabbit.core.RabbitAdmin;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.support.TransactionTemplate;

class OutboxRelaySchedulerTest extends PostgresRabbitIntegrationTestSupport {

    @Autowired
    OutboxRelayScheduler relay;

    @Autowired
    OrderTransactionStepService txStepService;

    @Autowired
    TransactionTemplate transactionTemplate;

    @Autowired
    RabbitAdmin rabbitAdmin;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void cleanTables() {
        jdbcTemplate.execute("TRUNCATE TABLE order_items, orders, idempotency_records, outbox_events, processed_events CASCADE");
    }

    @Test
    void unroutableEventStaysPendingUntilAConsumerQueueIsBound() {
        String routingKey = "order.test.unbound." + UUID.randomUUID();
        UUID orderId = UUID.randomUUID();
        transactionTemplate.executeWithoutResult(status ->
                txStepService.insertCompensationOutbox("TestEvent", routingKey, orderId, "TEST", Instant.now()));

        relay.flushPendingOutbox();

        Map<String, Object> pending = outboxRow(orderId);
        assertThat(pending.get("published_at")).isNull();
        assertThat((String) pending.get("last_error")).isNotBlank();

        Queue queue = new Queue("probe.unbound." + UUID.randomUUID(), false, false, false);
        rabbitAdmin.declareQueue(queue);
        rabbitAdmin.declareBinding(BindingBuilder.bind(queue)
                .to(new TopicExchange(RabbitMqConfig.EXCHANGE_NAME, true, false))
                .with(routingKey));
        jdbcTemplate.update("UPDATE outbox_events SET next_attempt_at = now() WHERE aggregate_id = ?", orderId.toString());

        relay.flushPendingOutbox();

        assertThat(outboxRow(orderId).get("published_at")).isNotNull();
        rabbitAdmin.deleteQueue(queue.getName());
    }

    private Map<String, Object> outboxRow(UUID orderId) {
        return jdbcTemplate.queryForMap(
                "SELECT published_at, last_error FROM outbox_events WHERE aggregate_id = ?",
                orderId.toString());
    }
}
