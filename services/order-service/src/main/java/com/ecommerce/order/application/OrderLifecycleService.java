package com.ecommerce.order.application;

import com.ecommerce.contracts.order.v1.AdminRetryRefundResponse;
import com.ecommerce.contracts.order.v1.AdminUpdateOrderStatusResponse;
import com.ecommerce.contracts.order.v1.OrderDetail;
import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.config.RabbitMqConfig;
import com.ecommerce.order.domain.ActorType;
import com.ecommerce.order.domain.OrderStatus;
import com.ecommerce.order.domain.OrderTransitions;
import com.ecommerce.order.domain.PaymentMethod;
import com.ecommerce.order.domain.PaymentStatus;
import com.ecommerce.order.integration.AuthorizationGrpcAdapter;
import com.ecommerce.order.integration.PaymentGrpcAdapter;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderRepository;
import com.ecommerce.order.persistence.OrderStatusHistoryRepository;
import io.grpc.Status;
import java.time.Instant;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

/** Order state changes requested after checkout: cancellation and fulfillment updates. */
@Service
public class OrderLifecycleService {
    static final int MAX_REASON_LENGTH = 128;
    static final int MAX_SHIPPING_FIELD_LENGTH = 100;
    static final String UPDATE_PERMISSION = "order.admin.update";

    private final OrderRepository orderRepository;
    private final OrderStatusHistoryRepository statusHistoryRepository;
    private final OrderTransactionStepService txStepService;
    private final AuthorizationGrpcAdapter authorizationAdapter;
    private final PaymentGrpcAdapter paymentGrpcAdapter;
    private final TransactionTemplate transactionTemplate;
    private final TransactionTemplate readOnlyTx;

    public OrderLifecycleService(
            OrderRepository orderRepository,
            OrderStatusHistoryRepository statusHistoryRepository,
            OrderTransactionStepService txStepService,
            AuthorizationGrpcAdapter authorizationAdapter,
            PaymentGrpcAdapter paymentGrpcAdapter,
            PlatformTransactionManager transactionManager) {
        this.orderRepository = orderRepository;
        this.statusHistoryRepository = statusHistoryRepository;
        this.txStepService = txStepService;
        this.authorizationAdapter = authorizationAdapter;
        this.paymentGrpcAdapter = paymentGrpcAdapter;
        this.transactionTemplate = new TransactionTemplate(transactionManager);
        TransactionTemplate ro = new TransactionTemplate(transactionManager);
        ro.setReadOnly(true);
        this.readOnlyTx = ro;
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
     * Applies an administrator fulfillment step. {@code expectedStatus} must match the
     * current status so a stale console cannot overwrite another administrator's change.
     */
    public AdminUpdateOrderStatusResponse updateStatusByAdmin(
            String rawUserId,
            String rawOrderId,
            String rawExpectedStatus,
            String rawTargetStatus,
            String reason,
            String shippingCarrier,
            String trackingNumber) {
        UUID adminId = AdminRequests.requiredUuid(rawUserId, "user_id");
        UUID orderId = AdminRequests.requiredUuid(rawOrderId, "order_id");
        OrderStatus expected = AdminRequests.requiredStatus(rawExpectedStatus, "expected_status");
        OrderStatus target = AdminRequests.requiredStatus(rawTargetStatus, "target_status");
        String cleanReason = trimToLimit(reason, MAX_REASON_LENGTH, "reason");
        String cleanCarrier = trimToLimit(shippingCarrier, MAX_SHIPPING_FIELD_LENGTH, "shipping_carrier");
        String cleanTracking = trimToLimit(trackingNumber, MAX_SHIPPING_FIELD_LENGTH, "tracking_number");
        if (!authorizationAdapter.hasPermission(adminId, UPDATE_PERMISSION)) {
            throw new DomainException(
                    Status.Code.PERMISSION_DENIED, "ORDER_PERMISSION_DENIED", "Missing " + UPDATE_PERMISSION + " permission");
        }

        return transactionTemplate.execute(tx -> {
            OrderEntity order = orderRepository.findByIdForUpdate(orderId)
                    .orElseThrow(() -> new DomainException(Status.Code.NOT_FOUND, "ORDER_NOT_FOUND", "Order not found"));
            OrderStatus current = order.getStatus();
            if (current != expected) {
                throw new DomainException(
                        Status.Code.ABORTED,
                        "ORDER_STATUS_CONFLICT",
                        "Order status changed to " + current.name() + "; reload before updating");
            }
            if (!OrderTransitions.adminTargets(current).contains(target)) {
                throw new DomainException(
                        Status.Code.FAILED_PRECONDITION,
                        "ORDER_INVALID_TRANSITION",
                        "Cannot change order status from " + current.name() + " to " + target.name());
            }
            if (OrderTransitions.requiresReason(target) && cleanReason.isEmpty()) {
                throw new DomainException(
                        Status.Code.INVALID_ARGUMENT,
                        "ORDER_REASON_REQUIRED",
                        "A reason is required for " + target.name());
            }

            Instant now = Instant.now();
            if (target == OrderStatus.CANCELLED) {
                applyCancellation(order, ActorType.ADMIN, adminId, cleanReason, now);
            } else {
                order.setStatus(target);
                // A redelivery may leave the fields blank to keep the current carrier and tracking number.
                if (target == OrderStatus.SHIPPING && !cleanCarrier.isEmpty()) {
                    order.setShippingCarrier(cleanCarrier);
                }
                if (target == OrderStatus.SHIPPING && !cleanTracking.isEmpty()) {
                    order.setTrackingNumber(cleanTracking);
                }
                if (target == OrderStatus.DELIVERED && order.getPaymentMethod() == PaymentMethod.COD) {
                    order.setPaymentStatus(PaymentStatus.PAID);
                }
                order.setUpdatedAt(now);
                orderRepository.saveAndFlush(order);
                txStepService.recordStatusChange(order, current, ActorType.ADMIN, adminId, cleanReason, now);
                if (target == OrderStatus.DELIVERED) {
                    txStepService.insertOrderDeliveredOutbox(order, now);
                    txStepService.registerOutboxWakeupAfterCommit();
                }
            }

            AdminUpdateOrderStatusResponse.Builder builder = AdminUpdateOrderStatusResponse.newBuilder()
                    .setOrder(OrderProtoMapper.toAdminOrderDetail(
                            order, statusHistoryRepository.findByOrderIdOrderByCreatedAtAsc(orderId)));
            OrderTransitions.adminTargets(order.getStatus())
                    .forEach(next -> builder.addAllowedNextStatuses(next.name()));
            return builder.build();
        });
    }

    public AdminRetryRefundResponse retryRefundByAdmin(String rawUserId, String rawOrderId, String reason) {
        UUID adminId = AdminRequests.requiredUuid(rawUserId, "user_id");
        UUID orderId = AdminRequests.requiredUuid(rawOrderId, "order_id");
        String cleanReason = trimToLimit(reason, MAX_REASON_LENGTH, "reason");
        if (!authorizationAdapter.hasPermission(adminId, UPDATE_PERMISSION)) {
            throw new DomainException(
                    Status.Code.PERMISSION_DENIED, "ORDER_PERMISSION_DENIED", "Missing " + UPDATE_PERMISSION + " permission");
        }

        readOnlyTx.executeWithoutResult(tx -> {
            OrderEntity order = orderRepository.findById(orderId)
                    .orElseThrow(() -> new DomainException(Status.Code.NOT_FOUND, "ORDER_NOT_FOUND", "Order not found"));
            if (order.getStatus() != OrderStatus.CANCELLED || order.getPaymentMethod() != PaymentMethod.MOMO) {
                throw new DomainException(
                        Status.Code.FAILED_PRECONDITION,
                        "ORDER_REFUND_NOT_ELIGIBLE",
                        "Only cancelled MoMo orders can be manually refunded");
            }
            if (order.getPaymentStatus() == PaymentStatus.REFUND_REQUIRED) {
                throw new DomainException(
                        Status.Code.FAILED_PRECONDITION,
                        "REFUND_AUTO_RETRY_IN_PROGRESS",
                        "Automatic refund retry is still in progress");
            }
            if (order.getPaymentStatus() != PaymentStatus.REFUND_FAILED) {
                throw new DomainException(
                        Status.Code.FAILED_PRECONDITION,
                        "ORDER_REFUND_NOT_ELIGIBLE",
                        "Manual refund is only allowed after automatic retries fail");
            }
        });

        PaymentGrpcAdapter.RetryRefundOutcome outcome;
        try {
            outcome = paymentGrpcAdapter.retryRefund(orderId, cleanReason);
        } catch (DomainException ex) {
            if ("PAYMENT_GATEWAY_REJECTED".equals(ex.reason()) || "PAYMENT_GATEWAY_UNAVAILABLE".equals(ex.reason())) {
                String latestError = truncate(ex.getMessage(), 255);
                transactionTemplate.executeWithoutResult(tx -> orderRepository.findByIdForUpdate(orderId).ifPresent(o -> {
                    if (o.getPaymentStatus() == PaymentStatus.REFUND_FAILED) {
                        o.setRefundLastError(latestError);
                        o.setUpdatedAt(Instant.now());
                        orderRepository.saveAndFlush(o);
                    }
                }));
            }
            throw ex;
        }

        return transactionTemplate.execute(tx -> {
            OrderEntity order = orderRepository.findByIdForUpdate(orderId)
                    .orElseThrow(() -> new DomainException(Status.Code.NOT_FOUND, "ORDER_NOT_FOUND", "Order not found"));
            Instant now = Instant.now();
            order.setPaymentStatus(PaymentStatus.REFUNDED);
            if (outcome.refundTransId() != null && !outcome.refundTransId().isBlank()) {
                order.setRefundTransId(truncate(outcome.refundTransId().trim(), 64));
            }
            order.setRefundLastError("");
            order.setRefundedAt(now);
            order.setRefundActorType("ADMIN");
            order.setRefundActorId(adminId);
            order.setRefundReason(cleanReason);
            order.setUpdatedAt(now);
            orderRepository.saveAndFlush(order);

            AdminRetryRefundResponse.Builder builder = AdminRetryRefundResponse.newBuilder()
                    .setOrder(OrderProtoMapper.toAdminOrderDetail(
                            order, statusHistoryRepository.findByOrderIdOrderByCreatedAtAsc(orderId)));
            OrderTransitions.adminTargets(order.getStatus())
                    .forEach(next -> builder.addAllowedNextStatuses(next.name()));
            return builder.build();
        });
    }

    private static String truncate(String value, int maxLength) {
        if (value == null || value.length() <= maxLength) {
            return value == null ? "" : value;
        }
        return value.substring(0, maxLength);
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
