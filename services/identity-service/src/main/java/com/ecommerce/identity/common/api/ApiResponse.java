package com.ecommerce.identity.common.api;

public record ApiResponse<T>(T data, String message) {
}
