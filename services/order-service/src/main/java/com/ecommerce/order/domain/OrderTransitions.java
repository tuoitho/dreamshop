package com.ecommerce.order.domain;

import java.util.EnumMap;
import java.util.EnumSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Order lifecycle rules shared by customer, administrator and system transitions.
 * System-driven steps (stock reservation, payment, COD auto-confirmation) are applied by
 * their own workflows; this class answers which steps an administrator may take and when
 * a customer may still cancel.
 */
public final class OrderTransitions {

    private static final Map<OrderStatus, List<OrderStatus>> ADMIN_TARGETS = new EnumMap<>(OrderStatus.class);
    private static final Set<OrderStatus> CUSTOMER_CANCELLABLE = EnumSet.of(
            OrderStatus.PENDING_PAYMENT, OrderStatus.PENDING_CONFIRMATION, OrderStatus.CONFIRMED);
    private static final Set<OrderStatus> REASON_REQUIRED = EnumSet.of(
            OrderStatus.CANCELLED, OrderStatus.DELIVERY_FAILED);

    static {
        ADMIN_TARGETS.put(OrderStatus.PENDING_CONFIRMATION, List.of(OrderStatus.CANCELLED));
        ADMIN_TARGETS.put(OrderStatus.CONFIRMED, List.of(OrderStatus.PREPARING, OrderStatus.CANCELLED));
        ADMIN_TARGETS.put(OrderStatus.PREPARING, List.of(OrderStatus.SHIPPING, OrderStatus.CANCELLED));
        ADMIN_TARGETS.put(OrderStatus.SHIPPING, List.of(OrderStatus.DELIVERED, OrderStatus.DELIVERY_FAILED));
        ADMIN_TARGETS.put(OrderStatus.DELIVERY_FAILED, List.of(OrderStatus.SHIPPING, OrderStatus.CANCELLED));
    }

    private OrderTransitions() {
    }

    public static List<OrderStatus> adminTargets(OrderStatus from) {
        return ADMIN_TARGETS.getOrDefault(from, List.of());
    }

    public static boolean customerCanCancel(OrderStatus from) {
        return CUSTOMER_CANCELLABLE.contains(from);
    }

    public static boolean requiresReason(OrderStatus target) {
        return REASON_REQUIRED.contains(target);
    }
}
