package com.ecommerce.identity.security.jwt;

import com.ecommerce.identity.auth.application.AccessTokenResult;
import com.ecommerce.identity.auth.domain.AuthenticationUnavailableException;
import com.ecommerce.identity.common.config.IdentityJwtProperties;
import java.time.Clock;
import java.time.Instant;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import org.springframework.security.oauth2.jose.jws.SignatureAlgorithm;
import org.springframework.security.oauth2.jwt.JwtClaimsSet;
import org.springframework.security.oauth2.jwt.JwtEncoder;
import org.springframework.security.oauth2.jwt.JwtEncoderParameters;
import org.springframework.security.oauth2.jwt.JwtEncodingException;
import org.springframework.security.oauth2.jwt.JwsHeader;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class JwtTokenService {
    private final JwtEncoder jwtEncoder;
    private final IdentityJwtProperties properties;
    private final Clock clock;

    public AccessTokenResult issue(UUID subject, Set<String> roles) {
        Instant issuedAt = clock.instant();
        Instant expiresAt = issuedAt.plus(properties.accessTtl());
        JwsHeader header = JwsHeader.with(SignatureAlgorithm.RS256)
                .type("JWT")
                .keyId(properties.keyId())
                .build();
        JwtClaimsSet claims = JwtClaimsSet.builder()
                .subject(subject.toString())
                .claim("roles", roles.stream().sorted().toList())
                .issuer(properties.issuer())
                .audience(List.of(properties.audience()))
                .issuedAt(issuedAt)
                .expiresAt(expiresAt)
                .id(UUID.randomUUID().toString())
                .build();
        try {
            String token = jwtEncoder.encode(JwtEncoderParameters.from(header, claims)).getTokenValue();
            return new AccessTokenResult(token, properties.accessTtl().toSeconds());
        } catch (JwtEncodingException exception) {
            throw new AuthenticationUnavailableException();
        }
    }
}
