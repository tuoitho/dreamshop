package com.ecommerce.order.application;

import com.ecommerce.order.config.RabbitMqConfig;
import com.ecommerce.order.domain.ActorType;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;
import tools.jackson.databind.json.JsonMapper;

@Component
public class PaymentEventConsumer {
    private static final Logger log = LoggerFactory.getLogger(PaymentEventConsumer.class);
    private static final JsonMapper JSON = JsonMapper.builder().build();

    private final OrderRepository orderRepository;
    private final OrderTransactionStepService txStepService;
    private final JdbcTemplate jdbcTemplate;

    public PaymentEventConsumer(
            OrderRepository orderRepository,
            OrderTransactionStepService txStepService,
            JdbcTemplate jdbcTemplate) {
        this.orderRepository = orderRepository;
        this.txStepService = txStepService;
        this.jdbcTemplate = jdbcTemplate;
    }

    @RabbitListener(queues = RabbitMqConfig.PAYMENT_EVENTS_QUEUE)
    @Transactional
    public void onRabbitMessage(Message message) {
        String routingKey = message.getMessageProperties() != null
                ? message.getMessageProperties().getReceivedRoutingKey()
                : "";
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        handleMessage(routingKey, body);
    }

    @Transactional
    @SuppressWarnings("unchecked")
    public void handleMessage(String routingKey, String jsonPayload) {
        Map<String, Object> envelope = JSON.readValue(jsonPayload, Map.class);
        String eventId = stringValue(envelope.get("eventId"));
        String eventType = stringValue(envelope.get("eventType"));
        String aggregateId = stringValue(envelope.get("aggregateId"));

        Map<String, Object> data = envelope.get("data") instanceof Map<?, ?> m
                ? (Map<String, Object>) m
                : Map.of();

        String orderIdRaw = stringValue(data.get("orderId"));
        if (orderIdRaw.isEmpty()) {
            orderIdRaw = aggregateId;
        }
        if (eventId.isEmpty() || orderIdRaw.isEmpty()) {
            log.warn("Ignoring malformed payment event without eventId or orderId: rk={}", routingKey);
            return;
        }

        int inserted = jdbcTemplate.update(
                """
                INSERT INTO processed_events (event_id, event_type, processed_at)
                VALUES (?, ?, now())
                ON CONFLICT (event_id) DO NOTHING
                """,
                eventId,
                eventType.isEmpty() ? routingKey : eventType);
        if (inserted == 0) {
            return;
        }

        UUID orderId = UUID.fromString(orderIdRaw);
        OrderEntity order = orderRepository.findByIdForUpdate(orderId).orElse(null);
        if (order == null) {
            return;
        }

        Instant now = Instant.now();
        String momoTransId = stringValue(data.get("momoTransId"));

        if (RabbitMqConfig.RK_PAYMENT_SUCCEEDED.equals(routingKey) || "PaymentSucceeded".equals(eventType)) {
            if (order.getStatus() == OrderStatus.PENDING_PAYMENT || order.getStatus() == OrderStatus.PENDING_STOCK) {
                OrderStatus previous = order.getStatus();
                order.setStatus(OrderStatus.CONFIRMED);
                order.setPaymentStatus(PaymentStatus.PAID);
                if (!momoTransId.isEmpty()) {
                    order.setPaymentTransId(momoTransId);
                }
                order.setUpdatedAt(now);
                orderRepository.saveAndFlush(order);
                txStepService.recordStatusChange(order, previous, ActorType.SYSTEM, null, "", now);

                txStepService.insertOrderConfirmedOutbox(order, now);
                txStepService.registerOutboxWakeupAfterCommit();
            } else if (order.getStatus() == OrderStatus.CANCELLED) {
                order.setPaymentStatus(PaymentStatus.REFUND_REQUIRED);
                if (!momoTransId.isEmpty()) {
                    order.setPaymentTransId(momoTransId);
                }
                order.setUpdatedAt(now);
                orderRepository.saveAndFlush(order);

                txStepService.insertCompensationOutbox(
                        "RefundRequested",
                        RabbitMqConfig.RK_PAYMENT_REFUND_REQUESTED,
                        order.getId(),
                        "LATE_PAYMENT_ON_CANCELLED_ORDER",
                        now);
                txStepService.registerOutboxWakeupAfterCommit();
            }
        } else if (RabbitMqConfig.RK_PAYMENT_FAILED.equals(routingKey) || "PaymentFailed".equals(eventType)) {
            if (order.getStatus() == OrderStatus.PENDING_PAYMENT) {
                order.setStatus(OrderStatus.CANCELLED);
                order.setPaymentStatus(PaymentStatus.FAILED);
                order.setCancelReason("PAYMENT_FAILED");
                order.setPayUrl("");
                order.setUpdatedAt(now);
                orderRepository.saveAndFlush(order);
                txStepService.recordStatusChange(
                        order, OrderStatus.PENDING_PAYMENT, ActorType.SYSTEM, null, "PAYMENT_FAILED", now);

                txStepService.insertCompensationOutbox(
                        "ReservationReleaseRequested",
                        RabbitMqConfig.RK_RESERVATION_RELEASE_REQUESTED,
                        order.getId(),
                        "PAYMENT_FAILED",
                        now);
                txStepService.registerOutboxWakeupAfterCommit();
            }
        }
    }

    private static String stringValue(Object obj) {
        return obj == null ? "" : String.valueOf(obj).trim();
    }
}
