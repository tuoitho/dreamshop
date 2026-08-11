package com.ecommerce.identity.location.api;

/**
 * Code-name pair returned by {@code GET /locations/provinces/{provinceCode}/wards}.
 */
public record WardResponse(String code, String name) {
}
