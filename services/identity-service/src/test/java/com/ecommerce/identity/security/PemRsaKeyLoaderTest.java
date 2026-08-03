package com.ecommerce.identity.security;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.ecommerce.identity.common.config.IdentityJwtProperties;
import com.ecommerce.identity.support.TestRsaKeyFiles;
import java.nio.file.Path;
import java.time.Duration;
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
}
