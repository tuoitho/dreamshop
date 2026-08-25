package com.ecommerce.identity.security.jwt;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.ecommerce.identity.common.config.IdentityJwtProperties;
import com.ecommerce.identity.support.TestRsaKeyFiles;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.interfaces.RSAPrivateCrtKey;
import java.security.interfaces.RSAPublicKey;
import java.security.spec.RSAPrivateKeySpec;
import java.time.Duration;
import java.util.Base64;
import java.util.List;
import org.junit.jupiter.api.Test;

class PemRsaKeyLoaderTest {
    private final PemRsaKeyLoader loader = new PemRsaKeyLoader();

    @Test
    void loadsMatching2048BitPemKeys() {
        TestRsaKeyFiles keys = TestRsaKeyFiles.create(2048);

        SigningKeyMaterial loaded = loader.load(properties(keys.privateKeyPath(), keys.publicKeyPath()));

        assertThat(loaded.publicKey().getModulus()).isEqualTo(loaded.privateKey().getModulus());
        assertThat(loaded.publicJwk().getKeyID()).isEqualTo("identity-test-key");
    }

    @Test
    void rejectsMismatchedKeysWithoutExposingPemContents() {
        TestRsaKeyFiles privateKeys = TestRsaKeyFiles.create(2048);
        TestRsaKeyFiles publicKeys = TestRsaKeyFiles.create(2048);

        assertThatThrownBy(() -> loader.load(properties(privateKeys.privateKeyPath(), publicKeys.publicKeyPath())))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageNotContaining("BEGIN");
    }

    @Test
    void rejectsKeysWithTheSameModulusButDifferentPublicExponent() throws Exception {
        TestRsaKeyFiles keys = TestRsaKeyFiles.create(2048);
        RSAPublicKey publicKey = (RSAPublicKey) keys.keyPair().getPublic();
        RSAPrivateCrtKey privateKey = (RSAPrivateCrtKey) keys.keyPair().getPrivate();
        BigInteger lambda = privateKey.getPrimeP().subtract(BigInteger.ONE)
                .multiply(privateKey.getPrimeQ().subtract(BigInteger.ONE))
                .divide(privateKey.getPrimeP().subtract(BigInteger.ONE)
                        .gcd(privateKey.getPrimeQ().subtract(BigInteger.ONE)));
        BigInteger alternateExponent = List.of(BigInteger.valueOf(3), BigInteger.valueOf(17), BigInteger.valueOf(257))
                .stream()
                .filter(exponent -> !exponent.equals(publicKey.getPublicExponent()))
                .filter(exponent -> exponent.gcd(lambda).equals(BigInteger.ONE))
                .findFirst()
                .orElseThrow();
        Path alternatePrivateKeyPath = keys.privateKeyPath().resolveSibling("alternate-private.pem");
        byte[] encoded = KeyFactory.getInstance("RSA")
                .generatePrivate(new RSAPrivateKeySpec(privateKey.getModulus(), alternateExponent.modInverse(lambda)))
                .getEncoded();
        Files.writeString(alternatePrivateKeyPath, pem("PRIVATE KEY", encoded));

        assertThatThrownBy(() -> loader.load(properties(alternatePrivateKeyPath, keys.publicKeyPath())))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageNotContaining("BEGIN");
    }

    @Test
    void rejects1024BitKeys() {
        TestRsaKeyFiles keys = TestRsaKeyFiles.create(1024);

        assertThatThrownBy(() -> loader.load(properties(keys.privateKeyPath(), keys.publicKeyPath())))
                .isInstanceOf(IllegalStateException.class);
    }

    @Test
    void rejectsMissingKeyFile() {
        TestRsaKeyFiles keys = TestRsaKeyFiles.create(2048);
        Path missing = keys.privateKeyPath().resolveSibling("missing.pem");

        assertThatThrownBy(() -> loader.load(properties(missing, keys.publicKeyPath())))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageNotContaining("BEGIN");
    }

    private IdentityJwtProperties properties(Path privateKeyPath, Path publicKeyPath) {
        return new IdentityJwtProperties(
                "identity-test", "ecommerce-api", privateKeyPath, publicKeyPath,
                "identity-test-key", Duration.ofMinutes(15));
    }

    private String pem(String type, byte[] encoded) {
        String base64 = Base64.getMimeEncoder(64, "\n".getBytes(StandardCharsets.US_ASCII)).encodeToString(encoded);
        return "-----BEGIN " + type + "-----\n" + base64 + "\n-----END " + type + "-----\n";
    }
}
