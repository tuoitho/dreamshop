package com.ecommerce.order.domain;

public record SnapshotMismatchDetail(String skuId, String code, String message) {
    public String formattedMessage() {
        return "[" + code + "] " + message;
    }
}
