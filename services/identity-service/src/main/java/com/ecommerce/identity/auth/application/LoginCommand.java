package com.ecommerce.identity.auth.application;

public record LoginCommand(String email, String password) {
}
