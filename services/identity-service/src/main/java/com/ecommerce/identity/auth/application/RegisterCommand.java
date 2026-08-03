package com.ecommerce.identity.auth.application;

public record RegisterCommand(String email, String password, String fullName) {
}
