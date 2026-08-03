package com.ecommerce.identity.security;

import com.nimbusds.jose.jwk.RSAKey;
import java.security.interfaces.RSAPrivateKey;
import java.security.interfaces.RSAPublicKey;

public record SigningKeyMaterial(RSAPublicKey publicKey, RSAPrivateKey privateKey, RSAKey publicJwk) {
}
