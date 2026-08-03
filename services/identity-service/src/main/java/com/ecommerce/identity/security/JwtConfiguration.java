package com.ecommerce.identity.security;

import com.ecommerce.identity.common.config.IdentityJwtProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.oauth2.jose.jws.SignatureAlgorithm;
import org.springframework.security.oauth2.jwt.JwtEncoder;
import org.springframework.security.oauth2.jwt.NimbusJwtEncoder;

@Configuration
public class JwtConfiguration {
    @Bean
    SigningKeyMaterial signingKeyMaterial(PemRsaKeyLoader loader, IdentityJwtProperties properties) {
        return loader.load(properties);
    }

    @Bean
    JwtEncoder jwtEncoder(SigningKeyMaterial keys, IdentityJwtProperties properties) {
        return NimbusJwtEncoder.withKeyPair(keys.publicKey(), keys.privateKey())
                .algorithm(SignatureAlgorithm.RS256)
                .jwkPostProcessor(jwk -> jwk.keyID(properties.keyId()))
                .build();
    }
}
