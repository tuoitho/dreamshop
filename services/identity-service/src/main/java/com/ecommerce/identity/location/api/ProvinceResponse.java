package com.ecommerce.identity.location.api;

/**
 * Code-name pair returned by {@code GET /locations/provinces}.
 */
public record ProvinceResponse(String code, String name) {
}
