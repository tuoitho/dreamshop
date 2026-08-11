package com.ecommerce.identity.location;

/**
 * Thrown when a location endpoint receives a province code absent from the vendored snapshot.
 * Mapped to {@code 404 NOT_FOUND}.
 */
public class ProvinceNotFoundException extends RuntimeException {
}
