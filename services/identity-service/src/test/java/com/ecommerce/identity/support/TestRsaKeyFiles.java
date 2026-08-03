package com.ecommerce.identity.support;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.util.Base64;

public record TestRsaKeyFiles(Path privateKeyPath, Path publicKeyPath, KeyPair keyPair) {
    public static TestRsaKeyFiles create(int bits) {
        try {
            KeyPairGenerator generator = KeyPairGenerator.getInstance("RSA");
            generator.initialize(bits);
            KeyPair keyPair = generator.generateKeyPair();
            Path directory = Files.createTempDirectory("identity-rsa-");
            Path privateKeyPath = directory.resolve("private.pem");
            Path publicKeyPath = directory.resolve("public.pem");
            Files.writeString(privateKeyPath, pem("PRIVATE KEY", keyPair.getPrivate().getEncoded()));
            Files.writeString(publicKeyPath, pem("PUBLIC KEY", keyPair.getPublic().getEncoded()));
            return new TestRsaKeyFiles(privateKeyPath, publicKeyPath, keyPair);
        } catch (Exception exception) {
            throw new IllegalStateException("Unable to create test RSA keys", exception);
        }
    }

    private static String pem(String type, byte[] encoded) {
        String base64 = Base64.getMimeEncoder(64, "\n".getBytes(StandardCharsets.US_ASCII))
                .encodeToString(encoded);
        return "-----BEGIN " + type + "-----\n" + base64 + "\n-----END " + type + "-----\n";
    }
}
