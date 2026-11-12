package com.ecommerce.order.application;

import com.ecommerce.contracts.order.v1.OrderDetail;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.config.RabbitMqConfig;
import com.ecommerce.order.domain.ActorType;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.OrderTransitions;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.persistence.OrderStatusHistoryRepository;
import io.grpc.Status;
import java.time.Instant;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/** Order state changes requested after checkout: cancellation and fulfillment updates. */
@Service
public class OrderLifecycleService {
    static final int MAX_REASON_LENGTH = 128;

    private final OrderRepository orderRepository;
    private final OrderStatusHistoryRepository statusHistoryRepository;
    private final OrderTransactionStepService txStepService;

    public OrderLifecycleService(
            OrderRepository orderRepository,
            OrderStatusHistoryRepository statusHistoryRepository,
            OrderTransactionStepService txStepService) {
        this.orderRepository = orderRepository;
        this.statusHistoryRepository = statusHistoryRepository;
        this.txStepService = txStepService;
    }

    @Transactional
    public OrderDetail cancelByCustomer(UUID userId, UUID orderId, String reason) {
        String cleanReason = trimToLimit(reason, MAX_REASON_LENGTH, "reason");
        OrderEntity order = orderRepository.findByIdForUpdate(orderId)
                .filter(o -> o.getUserId().equals(userId))
                .orElseThrow(() -> new DomainException(Status.Code.NOT_FOUND, "ORDER_NOT_FOUND", "Order not found"));

        if (order.getStatus() != OrderStatus.CANCELLED) {
            if (!OrderTransitions.customerCanCancel(order.getStatus())) {
                throw new DomainException(
                        Status.Code.FAILED_PRECONDITION,
                        "ORDER_NOT_CANCELLABLE",
                        "Order can no longer be cancelled");
            }
            applyCancellation(
                    order,
                    ActorType.CUSTOMER,
                    userId,
                    cleanReason.isEmpty() ? "USER_CANCELLED" : cleanReason,
                    Instant.now());
        }
        return OrderProtoMapper.toOrderDetail(order, statusHistoryRepository.findByOrderIdOrderByCreatedAtAsc(orderId));
    }

    /**
     * Cancels a locked order and queues the compensation its previous status requires:
     * before confirmation the stock is still only reserved; after confirmation it was
     * deducted and must be restocked, and a captured payment must be refunded.
     */
    void applyCancellation(OrderEntity order, ActorType actor, UUID actorId, String reason, Instant now) {
        OrderStatus previous = order.getStatus();
        boolean paid = order.getPaymentStatus() == PaymentStatus.PAID;
        order.setStatus(OrderStatus.CANCELLED);
        order.setCancelReason(reason);
        order.setPayUrl("");
        if (paid) {
            order.setPaymentStatus(PaymentStatus.REFUND_REQUIRED);
        }
        order.setUpdatedAt(now);
        orderRepository.saveAndFlush(order);
        txStepService.recordStatusChange(order, previous, actor, actorId, reason, now);

        switch (previous) {
            case PENDING_PAYMENT -> {
                txStepService.insertCompensationOutbox(
                        "ReservationReleaseRequested", RabbitMqConfig.RK_RESERVATION_RELEASE_REQUESTED, order.getId(), reason, now);
                txStepService.insertCompensationOutbox(
                        "PaymentCancelRequested", RabbitMqConfig.RK_PAYMENT_CANCEL_REQUESTED, order.getId(), reason, now);
            }
            case PENDING_CONFIRMATION -> txStepService.insertCompensationOutbox(
                    "ReservationReleaseRequested", RabbitMqConfig.RK_RESERVATION_RELEASE_REQUESTED, order.getId(), reason, now);
            default -> {
                txStepService.insertCompensationOutbox(
                        "StockRestockRequested", RabbitMqConfig.RK_STOCK_RESTOCK_REQUESTED, order.getId(), reason, now);
                if (paid) {
                    txStepService.insertCompensationOutbox(
                            "RefundRequested", RabbitMqConfig.RK_PAYMENT_REFUND_REQUESTED, order.getId(), reason, now);
                }
            }
        }
        txStepService.registerOutboxWakeupAfterCommit();
    }

    static String trimToLimit(String value, int maxLength, String field) {
        String clean = value == null ? "" : value.trim();
        if (clean.length() > maxLength) {
            throw new DomainException(
                    Status.Code.INVALID_ARGUMENT,
                    "INVALID_PARAMETER",
                    field + " must be at most " + maxLength + " characters");
        }
        return clean;
    }
}
