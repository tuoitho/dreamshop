package com.ecommerce.identity.address.domain;

import java.util.Objects;

public record ShippingAddressPatch<T>(Action action, T value) {
    public enum Action {
        KEEP,
        SET,
        CLEAR
    }

    public ShippingAddressPatch {
        Objects.requireNonNull(action, "action");
        if (action == Action.SET) {
            Objects.requireNonNull(value, "value");
        }
    }

    public static <T> ShippingAddressPatch<T> keep() {
        return new ShippingAddressPatch<>(Action.KEEP, null);
    }

    public static <T> ShippingAddressPatch<T> set(T value) {
        return new ShippingAddressPatch<>(Action.SET, value);
    }

    public static <T> ShippingAddressPatch<T> clear() {
        return new ShippingAddressPatch<>(Action.CLEAR, null);
    }
}
