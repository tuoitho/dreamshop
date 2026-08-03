package com.ecommerce.identity.security;

import com.ecommerce.identity.common.config.IdentityJwtProperties;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.GeneralSecurityException;
import java.security.KeyFactory;
import java.security.Signature;
import java.security.interfaces.RSAPrivateKey;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.PKCS8EncodedKeySpec;
import java.security.spec.X509EncodedKeySpec;
import java.util.Base64;
import org.springframework.stereotype.Component;

@Component
public class PemRsaKeyLoader {
    private static final byte[] KEY_PAIR_CHALLENGE = "identity-key-pair-check".getBytes(StandardCharsets.US_ASCII);

    public SigningKeyMaterial load(IdentityJwtProperties properties) {
        try {
            RSAPublicKey publicKey = readPublicKey(properties.publicKeyPath());
            RSAPrivateKey privateKey = readPrivateKey(properties.privateKeyPath());
            validateKeyPair(publicKey, privateKey);
            RSAKey publicJwk = new RSAKey.Builder(publicKey)
                    .keyUse(KeyUse.SIGNATURE)
                    .algorithm(JWSAlgorithm.RS256)
                    .keyID(properties.keyId())
                    .build();
            return new SigningKeyMaterial(publicKey, privateKey, publicJwk);
        } catch (IOException | GeneralSecurityException | IllegalArgumentException exception) {
            throw new IllegalStateException("Unable to load identity JWT signing keys", exception);
        }
    }

    private RSAPublicKey readPublicKey(Path path) throws IOException, GeneralSecurityException {
        return (RSAPublicKey) KeyFactory.getInstance("RSA")
                .generatePublic(new X509EncodedKeySpec(readPem(path, "PUBLIC KEY")));
    }

    private RSAPrivateKey readPrivateKey(Path path) throws IOException, GeneralSecurityException {
        return (RSAPrivateKey) KeyFactory.getInstance("RSA")
                .generatePrivate(new PKCS8EncodedKeySpec(readPem(path, "PRIVATE KEY")));
    }

    private byte[] readPem(Path path, String type) throws IOException {
        String pem = Files.readString(path);
        String base64 = pem
                .replace("-----BEGIN " + type + "-----", "")
                .replace("-----END " + type + "-----", "")
                .replaceAll("\\s", "");
        return Base64.getDecoder().decode(base64);
    }

    private void validateKeyPair(RSAPublicKey publicKey, RSAPrivateKey privateKey) {
        if (publicKey.getModulus().bitLength() < 2048 || privateKey.getModulus().bitLength() < 2048) {
            throw new IllegalArgumentException("RSA signing keys must be at least 2048 bits");
        }
        if (!publicKey.getModulus().equals(privateKey.getModulus())) {
            throw new IllegalArgumentException("RSA signing keys do not match");
        }
        try {
            Signature signature = Signature.getInstance("SHA256withRSA");
            signature.initSign(privateKey);
            signature.update(KEY_PAIR_CHALLENGE);
            byte[] challengeSignature = signature.sign();
            signature.initVerify(publicKey);
            signature.update(KEY_PAIR_CHALLENGE);
            if (!signature.verify(challengeSignature)) {
                throw new IllegalArgumentException("RSA signing keys do not match");
            }
        } catch (GeneralSecurityException exception) {
            throw new IllegalArgumentException("RSA signing keys do not match", exception);
        }
    }
}
