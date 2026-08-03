package com.ecommerce.identity.security;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.hamcrest.Matchers.containsString;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.jayway.jsonpath.JsonPath;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.jwk.JWKSet;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import java.time.Instant;
import java.util.Date;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.security.oauth2.core.DelegatingOAuth2TokenValidator;
import org.springframework.security.oauth2.core.OAuth2TokenValidator;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.security.oauth2.jwt.JwtClaimValidator;
import org.springframework.security.oauth2.jwt.JwtValidators;
import org.springframework.security.oauth2.jwt.NimbusJwtDecoder;
import org.springframework.security.oauth2.jwt.JwtException;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.springframework.jdbc.core.JdbcTemplate;

@AutoConfigureMockMvc
class JwksApiTest extends PostgresIntegrationTestSupport {
    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @Autowired
    SigningKeyMaterial signingKeyMaterial;

    @BeforeEach
    void resetDatabase() {
        jdbcTemplate.update("delete from user_roles");
        jdbcTemplate.update("delete from identity_profiles");
        jdbcTemplate.update("delete from identity_users");
        jdbcTemplate.update("delete from roles");
        jdbcTemplate.update("insert into roles (id, code) values (1, 'CUSTOMER')");
    }

    @Test
    void publishesOnlyThePublicSigningKeyAndVerifiesLoginTokens() throws Exception {
        String token = registerAndLogin();

        MvcResult result = mockMvc.perform(get("/.well-known/jwks.json"))
                .andExpect(status().isOk())
                .andExpect(header().string("Cache-Control", containsString("max-age=300")))
                .andExpect(header().string("Cache-Control", containsString("public")))
                .andReturn();

        JWKSet jwkSet = JWKSet.parse(result.getResponse().getContentAsString());
        RSAKey jwk = (RSAKey) jwkSet.getKeyByKeyId(SignedJWT.parse(token).getHeader().getKeyID());
        Map<String, Object> jwkJson = jwk.toJSONObject();
        assertThat(jwk.getKeyType().getValue()).isEqualTo("RSA");
        assertThat(jwk.getKeyUse().getValue()).isEqualTo("sig");
        assertThat(jwk.getAlgorithm().getName()).isEqualTo("RS256");
        assertThat(jwk.getKeyID()).isEqualTo("identity-test-key");
        assertThat(jwk.getModulus().toString()).isNotBlank();
        assertThat(jwk.getPublicExponent().toString()).isNotBlank();
        assertThat(jwkJson).doesNotContainKeys("d", "p", "q", "dp", "dq", "qi");

        NimbusJwtDecoder decoder = decoder(jwk);
        Jwt verified = decoder.decode(token);
        assertThat(verified.getSubject()).isNotBlank();
        assertThat(verified.getClaimAsStringList("roles")).containsExactly("CUSTOMER");

        assertThatThrownBy(() -> decoder.decode(tamperSignature(token))).isInstanceOf(JwtException.class);
        assertThatThrownBy(() -> decoder.decode(signedToken("wrong-issuer", "ecommerce-api", Instant.now().plusSeconds(60))))
                .isInstanceOf(JwtException.class);
        assertThatThrownBy(() -> decoder.decode(signedToken("identity-test", "wrong-audience", Instant.now().plusSeconds(60))))
                .isInstanceOf(JwtException.class);
        assertThatThrownBy(() -> decoder.decode(signedToken("identity-test", "ecommerce-api", Instant.now().minusSeconds(60))))
                .isInstanceOf(JwtException.class);
    }

    private String registerAndLogin() throws Exception {
        String email = "jwks-" + UUID.randomUUID() + "@example.com";
        String password = "correct horse battery staple";
        mockMvc.perform(post("/register")
                        .contentType("application/json")
                        .content("{\"email\":\"%s\",\"password\":\"%s\",\"full_name\":\"JWKS User\"}".formatted(email, password)))
                .andExpect(status().isCreated());
        MvcResult login = mockMvc.perform(post("/login")
                        .contentType("application/json")
                        .content("{\"email\":\"%s\",\"password\":\"%s\"}".formatted(email, password)))
                .andExpect(status().isOk())
                .andReturn();
        return JsonPath.read(login.getResponse().getContentAsString(), "$.access_token");
    }

    private NimbusJwtDecoder decoder(RSAKey jwk) throws Exception {
        NimbusJwtDecoder decoder = NimbusJwtDecoder.withPublicKey(jwk.toRSAPublicKey()).build();
        OAuth2TokenValidator<Jwt> audience = new JwtClaimValidator<List<String>>("aud", value -> value.contains("ecommerce-api"));
        decoder.setJwtValidator(new DelegatingOAuth2TokenValidator<>(
                JwtValidators.createDefaultWithIssuer("identity-test"), audience));
        return decoder;
    }

    private String signedToken(String issuer, String audience, Instant expiration) throws Exception {
        SignedJWT token = new SignedJWT(
                new JWSHeader.Builder(JWSAlgorithm.RS256).keyID("identity-test-key").build(),
                new JWTClaimsSet.Builder()
                        .subject(UUID.randomUUID().toString())
                        .issuer(issuer)
                        .audience(audience)
                        .issueTime(Date.from(Instant.now()))
                        .expirationTime(Date.from(expiration))
                        .claim("roles", List.of("CUSTOMER"))
                        .build());
        token.sign(new RSASSASigner(signingKeyMaterial.privateKey()));
        return token.serialize();
    }

    private String tamperSignature(String token) {
        int signatureStart = token.lastIndexOf('.') + 1;
        char firstSignatureCharacter = token.charAt(signatureStart);
        char replacement = firstSignatureCharacter == 'A' ? 'B' : 'A';
        return token.substring(0, signatureStart) + replacement + token.substring(signatureStart + 1);
    }
}
