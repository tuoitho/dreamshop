package com.ecommerce.order.domain;

public enum OrderStatus {
    PENDING_STOCK,
    PENDING_PAYMENT,
    PENDING_CONFIRMATION,
    CONFIRMED,
    PREPARING,
    SHIPPING,
    DELIVERED,
    DELIVERY_FAILED,
    CANCELLED
}
