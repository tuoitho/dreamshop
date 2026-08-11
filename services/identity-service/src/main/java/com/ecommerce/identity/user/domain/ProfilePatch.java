package com.ecommerce.identity.user.domain;

import java.util.Objects;

/**
 * Three-state patch for one optional profile field: keep the current value,
 * clear it to null, or replace it with a provided value.
 *
 * @param action KEEP preserves the current value, CLEAR nulls it, SET replaces it
 * @param value  replacement value, only meaningful for SET
 * @param <T>    field value type (already parsed/converted)
 */
public record ProfilePatch<T>(Action action, T value) {
    public enum Action {
        KEEP,
        CLEAR,
        SET
    }

    public static <T> ProfilePatch<T> keep() {
        return new ProfilePatch<>(Action.KEEP, null);
    }

    public static <T> ProfilePatch<T> clear() {
        return new ProfilePatch<>(Action.CLEAR, null);
    }

    public static <T> ProfilePatch<T> set(T value) {
        return new ProfilePatch<>(Action.SET, Objects.requireNonNull(value, "value"));
    }

    public T applyTo(T current) {
        return switch (action) {
            case KEEP -> current;
            case CLEAR -> null;
            case SET -> value;
        };
    }
}
