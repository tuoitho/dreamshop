package com.ecommerce.identity.security.api;

import com.ecommerce.identity.security.jwt.SigningKeyMaterial;
import com.nimbusds.jose.jwk.JWKSet;
import java.time.Duration;
import java.util.Map;
import lombok.RequiredArgsConstructor;
import org.springframework.http.CacheControl;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class JwksController {
    private final SigningKeyMaterial keys;

    @GetMapping("/.well-known/jwks.json")
    ResponseEntity<Map<String, Object>> jwks() {
        Map<String, Object> body = new JWKSet(keys.publicJwk().toPublicJWK()).toJSONObject();
        return ResponseEntity.ok()
                .cacheControl(CacheControl.maxAge(Duration.ofMinutes(5)).cachePublic())
                .body(body);
    }
}
