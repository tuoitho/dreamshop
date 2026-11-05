package com.ecommerce.order.domain;

public final class ShippingPolicy {
    public static final String CURRENCY = "VND";
    public static final long FREE_SHIPPING_THRESHOLD_MINOR = 299_000L;
    public static final long STANDARD_SHIPPING_FEE_MINOR = 25_000L;

    private ShippingPolicy() {
    }

    public static long computeShippingFeeMinor(long subtotalMinor) {
        return subtotalMinor >= FREE_SHIPPING_THRESHOLD_MINOR ? 0L : STANDARD_SHIPPING_FEE_MINOR;
    }

    public static long computeTotalMinor(long subtotalMinor, long shippingFeeMinor) {
        return Math.addExact(subtotalMinor, shippingFeeMinor);
    }
}
