package com.ecommerce.identity.security;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.ecommerce.identity.auth.application.AccessTokenResult;
import com.ecommerce.identity.auth.domain.AuthenticationUnavailableException;
import com.ecommerce.identity.common.config.IdentityJwtProperties;
import com.ecommerce.identity.support.TestRsaKeyFiles;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jwt.SignedJWT;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.Set;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.security.oauth2.jwt.JwtEncoder;
import org.springframework.security.oauth2.jwt.JwtEncodingException;
import org.springframework.security.oauth2.jwt.NimbusJwtEncoder;

class JwtTokenServiceTest {
    private static final Instant NOW = Instant.parse("2030-01-01T12:00:00Z");
    private static final Clock CLOCK = Clock.fixed(NOW, ZoneOffset.UTC);

    @Test
    void issuesAStandardsShapedRs256AccessToken() throws Exception {
        TestRsaKeyFiles keys = TestRsaKeyFiles.create(2048);
        IdentityJwtProperties properties = properties(keys);
        JwtEncoder encoder = NimbusJwtEncoder.withKeyPair(
                        (java.security.interfaces.RSAPublicKey) keys.keyPair().getPublic(),
                        (java.security.interfaces.RSAPrivateKey) keys.keyPair().getPrivate())
                .jwkPostProcessor(jwk -> jwk.keyID(properties.keyId()))
                .build();
        JwtTokenService service = new JwtTokenService(encoder, properties, CLOCK);
        UUID subject = UUID.randomUUID();

        AccessTokenResult result = service.issue(subject, Set.of("ADMIN", "CUSTOMER"));
        SignedJWT token = SignedJWT.parse(result.token());

        assertThat(token.getHeader().getAlgorithm()).isEqualTo(JWSAlgorithm.RS256);
        assertThat(token.getHeader().getType().getType()).isEqualTo("JWT");
        assertThat(token.getHeader().getKeyID()).isEqualTo("identity-test-key");
        assertThat(token.verify(new RSASSAVerifier((java.security.interfaces.RSAPublicKey) keys.keyPair().getPublic())))
                .isTrue();
        assertThat(token.getJWTClaimsSet().getSubject()).isEqualTo(subject.toString());
        assertThat(token.getJWTClaimsSet().getStringListClaim("roles")).containsExactly("ADMIN", "CUSTOMER");
        assertThat(token.getJWTClaimsSet().getIssuer()).isEqualTo("identity-test");
        assertThat(token.getJWTClaimsSet().getAudience()).containsExactly("ecommerce-api");
        assertThat(token.getJWTClaimsSet().getIssueTime().toInstant()).isEqualTo(NOW);
        assertThat(token.getJWTClaimsSet().getExpirationTime().toInstant()).isEqualTo(NOW.plusSeconds(900));
        assertThat(token.getJWTClaimsSet().getJWTID()).isNotBlank();
        assertThat(result.expiresInSeconds()).isEqualTo(900);
    }

    @Test
    void hidesJwtEncoderFailuresBehindAuthenticationUnavailableException() {
        TestRsaKeyFiles keys = TestRsaKeyFiles.create(2048);
        JwtEncoder failingEncoder = parameters -> {
            throw new JwtEncodingException("private encoder detail");
        };
        JwtTokenService service = new JwtTokenService(failingEncoder, properties(keys), CLOCK);

        assertThatThrownBy(() -> service.issue(UUID.randomUUID(), Set.of("CUSTOMER")))
                .isInstanceOf(AuthenticationUnavailableException.class)
                .hasMessageNotContaining("private encoder detail");
    }

    private IdentityJwtProperties properties(TestRsaKeyFiles keys) {
        return new IdentityJwtProperties(
                "identity-test", "ecommerce-api", keys.privateKeyPath(), keys.publicKeyPath(),
                "identity-test-key", Duration.ofMinutes(15));
    }
}
