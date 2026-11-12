package com.ecommerce.order.application;

import com.ecommerce.contracts.order.v1.CreateOrderResponse;
import com.ecommerce.contracts.order.v1.ShippingAddressSnapshot;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.common.TraceIdProvider;
import com.ecommerce.order.config.OrderProperties;
import com.ecommerce.order.config.RabbitMqConfig;
import com.ecommerce.order.domain.ActorType;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.domain.ShippingPolicy;
import com.ecommerce.order.persistence.IdempotencyRecordEntity;
import com.ecommerce.order.persistence.IdempotencyRecordId;
import com.ecommerce.order.persistence.IdempotencyRecordRepository;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderItemEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.persistence.OrderStatusHistoryEntity;
import com.ecommerce.order.persistence.OrderStatusHistoryRepository;
import com.ecommerce.order.persistence.OutboxEventEntity;
import com.ecommerce.order.persistence.OutboxEventRepository;
import io.grpc.Status;
import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import tools.jackson.databind.json.JsonMapper;

@Service
public class OrderTransactionStepService {
    private static final JsonMapper JSON = JsonMapper.builder().build();

    private final OrderRepository orderRepository;
    private final OrderStatusHistoryRepository statusHistoryRepository;
    private final IdempotencyRecordRepository idempotencyRepository;
    private final OutboxEventRepository outboxRepository;
    private final IdempotencyService idempotencyService;
    private final OutboxRelayScheduler outboxRelayScheduler;
    private final OrderProperties properties;
    private final JdbcTemplate jdbcTemplate;

    public record Tx1Outcome(boolean claimed, UUID orderId, Instant paymentExpiresAt) {
    }

    public OrderTransactionStepService(
            OrderRepository orderRepository,
            OrderStatusHistoryRepository statusHistoryRepository,
            IdempotencyRecordRepository idempotencyRepository,
            OutboxEventRepository outboxRepository,
            IdempotencyService idempotencyService,
            OutboxRelayScheduler outboxRelayScheduler,
            OrderProperties properties,
            JdbcTemplate jdbcTemplate) {
        this.orderRepository = orderRepository;
        this.statusHistoryRepository = statusHistoryRepository;
        this.idempotencyRepository = idempotencyRepository;
        this.outboxRepository = outboxRepository;
        this.idempotencyService = idempotencyService;
        this.outboxRelayScheduler = outboxRelayScheduler;
        this.properties = properties;
        this.jdbcTemplate = jdbcTemplate;
    }

    @Transactional
    public Tx1Outcome beginOrderTx1(
            UUID userId,
            String idempotencyKey,
            String fingerprint,
            String orderCode,
            PaymentMethod paymentMethod,
            ShippingAddressSnapshot address,
            String note,
            List<OrderItemEntity> items,
            long subtotalMinor,
            long shippingFeeMinor,
            long totalAmountMinor) {
        Instant now = Instant.now();
        Duration idemTtl = properties.idempotencyTtl() != null ? properties.idempotencyTtl() : Duration.ofHours(24);
        Duration payTtl = properties.paymentTtl() != null ? properties.paymentTtl() : Duration.ofMinutes(15);

        jdbcTemplate.update(
                """
                DELETE FROM idempotency_records r
                WHERE r.user_id = ?
                  AND r.idempotency_key = ?
                  AND r.expires_at <= ?
                  AND NOT EXISTS (
                      SELECT 1 FROM orders o
                      WHERE o.user_id = r.user_id
                        AND o.idempotency_key = r.idempotency_key
                  )
                """,
                userId,
                idempotencyKey,
                java.sql.Timestamp.from(now));

        UUID orderId = UUID.randomUUID();
        int inserted = jdbcTemplate.update(
                """
                INSERT INTO idempotency_records (
                    user_id, idempotency_key, request_fingerprint, status,
                    order_id, failure_code, failure_message, expires_at, created_at, updated_at
                )
                VALUES (?, ?, ?, ?, ?, '', '', ?, ?, ?)
                ON CONFLICT (user_id, idempotency_key) DO NOTHING
                """,
                userId,
                idempotencyKey,
                fingerprint,
                IdempotencyService.STATUS_IN_PROGRESS,
                orderId,
                java.sql.Timestamp.from(now.plus(idemTtl)),
                java.sql.Timestamp.from(now),
                java.sql.Timestamp.from(now));

        if (inserted == 0) {
            return new Tx1Outcome(false, null, null);
        }

        Instant paymentExpiresAt = paymentMethod == PaymentMethod.MOMO ? now.plus(payTtl) : null;
        OrderEntity order = OrderEntity.builder()
                .id(orderId)
                .orderCode(orderCode)
                .userId(userId)
                .idempotencyKey(idempotencyKey)
                .status(OrderStatus.PENDING_STOCK)
                .paymentMethod(paymentMethod)
                .paymentStatus(paymentMethod == PaymentMethod.COD ? PaymentStatus.UNPAID : PaymentStatus.PENDING)
                .recipientName(address.getRecipientName().trim())
                .recipientPhone(address.getPhone().trim())
                .provinceCode(address.getProvinceCode().trim())
                .provinceName(address.getProvinceName().trim())
                .wardCode(address.getWardCode().trim())
                .wardName(address.getWardName().trim())
                .addressLine(address.getAddressLine().trim())
                .addressLabel(address.getLabel().trim())
                .currency(ShippingPolicy.CURRENCY)
                .subtotalMinor(subtotalMinor)
                .shippingFeeMinor(shippingFeeMinor)
                .totalAmountMinor(totalAmountMinor)
                .payUrl("")
                .paymentTransId("")
                .paymentExpiresAt(paymentExpiresAt)
                .note(note == null ? "" : note.trim())
                .cancelReason("")
                .createdAt(now)
                .updatedAt(now)
                .build();

        for (OrderItemEntity item : items) {
            if (item.getId() == null) {
                item.setId(UUID.randomUUID());
            }
            order.addItem(item);
        }
        orderRepository.saveAndFlush(order);
        recordStatusChange(order, null, ActorType.CUSTOMER, userId, "", now);
        return new Tx1Outcome(true, orderId, paymentExpiresAt);
    }

    /**
     * Moves a COD order whose stock is reserved to PENDING_CONFIRMATION. The order is
     * confirmed later by {@link CodAutoConfirmScheduler}; OrderPlaced lets Cart clear the
     * ordered lines right away.
     */
    @Transactional
    public CreateOrderResponse markPendingConfirmation(UUID orderId, UUID userId, String idempotencyKey) {
        OrderEntity order = orderRepository.findByIdForUpdate(orderId)
                .orElseThrow(() -> new DomainException(Status.Code.NOT_FOUND, "ORDER_NOT_FOUND", "Order not found"));
        if (order.getStatus() != OrderStatus.PENDING_STOCK) {
            throw new DomainException(
                    Status.Code.FAILED_PRECONDITION,
                    "ORDER_ALREADY_CANCELLED",
                    "Order is no longer awaiting stock confirmation");
        }

        Instant now = Instant.now();
        Duration delay = properties.cod() != null && properties.cod().autoConfirmDelay() != null
                ? properties.cod().autoConfirmDelay()
                : Duration.ofMinutes(1);
        order.setStatus(OrderStatus.PENDING_CONFIRMATION);
        order.setPaymentStatus(PaymentStatus.UNPAID);
        order.setAutoConfirmAt(now.plus(delay));
        order.setUpdatedAt(now);
        orderRepository.saveAndFlush(order);
        recordStatusChange(order, OrderStatus.PENDING_STOCK, ActorType.SYSTEM, null, "", now);

        insertOrderPlacedOutbox(order, now);

        CreateOrderResponse response = OrderProtoMapper.toCreateOrderResponse(order);
        markIdempotencyCompleted(userId, idempotencyKey, orderId, response, now);
        registerOutboxWakeupAfterCommit();
        return response;
    }

    @Transactional(noRollbackFor = DomainException.class)
    public CreateOrderResponse markPendingPayment(UUID orderId, UUID userId, String idempotencyKey, String payUrl) {
        OrderEntity order = orderRepository.findByIdForUpdate(orderId)
                .orElseThrow(() -> new DomainException(Status.Code.NOT_FOUND, "ORDER_NOT_FOUND", "Order not found"));
        if (order.getStatus() != OrderStatus.PENDING_STOCK) {
            Instant now = Instant.now();
            insertCompensationOutbox(
                    "PaymentCancelRequested",
                    RabbitMqConfig.RK_PAYMENT_CANCEL_REQUESTED,
                    orderId,
                    "ORDER_ALREADY_CANCELLED",
                    now);
            idempotencyRepository.findById(new IdempotencyRecordId(userId, idempotencyKey)).ifPresent(rec -> {
                if (IdempotencyService.STATUS_IN_PROGRESS.equals(rec.getStatus())) {
                    rec.setStatus(IdempotencyService.STATUS_FAILED_TERMINAL);
                    rec.setFailureCode("ORDER_ALREADY_CANCELLED");
                    rec.setFailureMessage("Order was cancelled before payment session completed");
                    rec.setUpdatedAt(now);
                    idempotencyRepository.saveAndFlush(rec);
                }
            });
            registerOutboxWakeupAfterCommit();
            throw new DomainException(
                    Status.Code.FAILED_PRECONDITION,
                    "ORDER_ALREADY_CANCELLED",
                    "Order was cancelled before payment session completed");
        }

        Instant now = Instant.now();
        order.setStatus(OrderStatus.PENDING_PAYMENT);
        order.setPaymentStatus(PaymentStatus.PENDING);
        order.setPayUrl(payUrl);
        order.setUpdatedAt(now);
        orderRepository.saveAndFlush(order);
        recordStatusChange(order, OrderStatus.PENDING_STOCK, ActorType.SYSTEM, null, "", now);

        CreateOrderResponse response = OrderProtoMapper.toCreateOrderResponse(order);
        markIdempotencyCompleted(userId, idempotencyKey, orderId, response, now);
        return response;
    }

    @Transactional
    public void markCancelledOnSagaFailure(
            UUID orderId,
            UUID userId,
            String idempotencyKey,
            String cancelReason,
            String failureCode,
            String failureMessage,
            boolean releaseStock,
            boolean cancelPayment) {
        OrderEntity order = orderRepository.findByIdForUpdate(orderId).orElse(null);
        Instant now = Instant.now();
        if (order != null && order.getStatus() == OrderStatus.PENDING_STOCK) {
            order.setStatus(OrderStatus.CANCELLED);
            if (order.getPaymentMethod() == PaymentMethod.MOMO) {
                order.setPaymentStatus(PaymentStatus.FAILED);
            }
            order.setCancelReason(cancelReason);
            order.setPayUrl("");
            order.setUpdatedAt(now);
            orderRepository.saveAndFlush(order);
            recordStatusChange(order, OrderStatus.PENDING_STOCK, ActorType.SYSTEM, null, cancelReason, now);

            if (releaseStock) {
                insertCompensationOutbox(
                        "ReservationReleaseRequested",
                        RabbitMqConfig.RK_RESERVATION_RELEASE_REQUESTED,
                        order.getId(),
                        cancelReason,
                        now);
            }
            if (cancelPayment) {
                insertCompensationOutbox(
                        "PaymentCancelRequested",
                        RabbitMqConfig.RK_PAYMENT_CANCEL_REQUESTED,
                        order.getId(),
                        cancelReason,
                        now);
            }
            if (releaseStock || cancelPayment) {
                registerOutboxWakeupAfterCommit();
            }
        }

        idempotencyRepository.findById(new IdempotencyRecordId(userId, idempotencyKey)).ifPresent(rec -> {
            if (IdempotencyService.STATUS_IN_PROGRESS.equals(rec.getStatus())) {
                rec.setStatus(IdempotencyService.STATUS_FAILED_TERMINAL);
                rec.setFailureCode(failureCode);
                rec.setFailureMessage(failureMessage == null ? "" : failureMessage);
                rec.setUpdatedAt(now);
                idempotencyRepository.saveAndFlush(rec);
            }
        });
    }

    /**
     * Appends a history entry for the order's current status. Call it in the same
     * transaction as the status change so history and state never diverge.
     */
    public void recordStatusChange(
            OrderEntity order,
            OrderStatus fromStatus,
            ActorType actorType,
            UUID actorId,
            String reason,
            Instant at) {
        statusHistoryRepository.save(OrderStatusHistoryEntity.builder()
                .id(UUID.randomUUID())
                .orderId(order.getId())
                .fromStatus(fromStatus)
                .toStatus(order.getStatus())
                .actorType(actorType)
                .actorId(actorId)
                .reason(reason == null ? "" : reason)
                .shippingCarrier(order.getShippingCarrier() == null ? "" : order.getShippingCarrier())
                .trackingNumber(order.getTrackingNumber() == null ? "" : order.getTrackingNumber())
                .createdAt(at)
                .build());
    }

    public void insertOrderConfirmedOutbox(OrderEntity order, Instant now) {
        insertOrderItemsEvent(order, "OrderConfirmed", RabbitMqConfig.RK_ORDER_CONFIRMED, now);
    }

    public void insertOrderPlacedOutbox(OrderEntity order, Instant now) {
        insertOrderItemsEvent(order, "OrderPlaced", RabbitMqConfig.RK_ORDER_PLACED, now);
    }

    private void insertOrderItemsEvent(OrderEntity order, String eventType, String routingKey, Instant now) {
        UUID eventId = UUID.randomUUID();
        List<Map<String, Object>> itemRefs = order.getItems().stream()
                .map(i -> {
                    Map<String, Object> m = new LinkedHashMap<>();
                    m.put("cartItemId", i.getCartItemId());
                    m.put("productId", i.getProductId());
                    m.put("skuId", i.getSkuId());
                    m.put("quantity", i.getQuantity());
                    return m;
                })
                .toList();

        Map<String, Object> data = new LinkedHashMap<>();
        data.put("orderId", order.getId().toString());
        data.put("orderCode", order.getOrderCode());
        data.put("userId", order.getUserId().toString());
        data.put("items", itemRefs);

        saveOutboxEvent(eventId, eventType, routingKey, order.getId().toString(), data, now);
    }

    public void insertCompensationOutbox(String eventType, String routingKey, UUID orderId, String reason, Instant now) {
        UUID eventId = UUID.randomUUID();
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("orderId", orderId.toString());
        data.put("reason", reason);
        saveOutboxEvent(eventId, eventType, routingKey, orderId.toString(), data, now);
    }

    public void registerOutboxWakeupAfterCommit() {
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
                @Override
                public void afterCommit() {
                    outboxRelayScheduler.triggerFlush();
                }
            });
        } else {
            outboxRelayScheduler.triggerFlush();
        }
    }

    private void saveOutboxEvent(
            UUID eventId,
            String eventType,
            String routingKey,
            String aggregateId,
            Map<String, Object> data,
            Instant now) {
        String corrId = TraceIdProvider.current();
        Map<String, Object> envelope = new LinkedHashMap<>();
        envelope.put("eventId", eventId.toString());
        envelope.put("eventType", eventType);
        envelope.put("eventVersion", 1);
        envelope.put("occurredAt", now.toString());
        envelope.put("aggregateId", aggregateId);
        envelope.put("correlationId", corrId);
        envelope.put("data", data);

        outboxRepository.save(OutboxEventEntity.builder()
                .id(eventId)
                .eventType(eventType)
                .routingKey(routingKey)
                .eventVersion(1)
                .aggregateId(aggregateId)
                .correlationId(corrId)
                .payload(JSON.writeValueAsString(envelope))
                .attempts(0)
                .nextAttemptAt(now)
                .lastError("")
                .createdAt(now)
                .build());
    }

    private void markIdempotencyCompleted(
            UUID userId,
            String idempotencyKey,
            UUID orderId,
            CreateOrderResponse response,
            Instant now) {
        IdempotencyRecordEntity record = idempotencyRepository
                .findById(new IdempotencyRecordId(userId, idempotencyKey))
                .orElse(null);
        if (record != null) {
            record.setStatus(IdempotencyService.STATUS_COMPLETED);
            record.setOrderId(orderId);
            record.setResponseJson(idempotencyService.serializeResponse(response));
            record.setUpdatedAt(now);
            idempotencyRepository.saveAndFlush(record);
        }
    }
}
