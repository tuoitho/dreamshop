package main

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateIdentityDevKeysCreatesValidKeyPair(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer

	code := run([]string{"-root", root}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Created untracked Identity development key pair.") {
		t.Fatalf("unexpected stdout: %s", out.String())
	}

	privBytes, err := os.ReadFile(filepath.Join(root, "deployments", "secrets", "identity", "private.pem"))
	if err != nil {
		t.Fatalf("read private.pem: %v", err)
	}
	privBlock, _ := pem.Decode(privBytes)
	if privBlock == nil || privBlock.Type != "PRIVATE KEY" {
		t.Fatalf("expected PRIVATE KEY PEM block, got %v", privBlock)
	}
	parsedPriv, err := x509.ParsePKCS8PrivateKey(privBlock.Bytes)
	if err != nil {
		t.Fatalf("parse PKCS8 private key: %v", err)
	}
	rsaPriv, ok := parsedPriv.(*rsa.PrivateKey)
	if !ok || rsaPriv.N.BitLen() != 2048 {
		t.Fatalf("expected 2048-bit RSA private key, got %T", parsedPriv)
	}

	pubBytes, err := os.ReadFile(filepath.Join(root, "deployments", "secrets", "identity", "public.pem"))
	if err != nil {
		t.Fatalf("read public.pem: %v", err)
	}
	pubBlock, _ := pem.Decode(pubBytes)
	if pubBlock == nil || pubBlock.Type != "PUBLIC KEY" {
		t.Fatalf("expected PUBLIC KEY PEM block, got %v", pubBlock)
	}
	parsedPub, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		t.Fatalf("parse PKIX public key: %v", err)
	}
	rsaPub, ok := parsedPub.(*rsa.PublicKey)
	if !ok || !rsaPub.Equal(&rsaPriv.PublicKey) {
		t.Fatalf("public key does not match private key")
	}

	// Second run is idempotent
	out.Reset()
	code = run([]string{"-root", root}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit 0 on existing keys, got %d", code)
	}
	if !strings.Contains(out.String(), "Identity development key pair already exists.") {
		t.Fatalf("unexpected stdout on second run: %s", out.String())
	}
}

func TestGenerateIdentityDevKeysFailsOnIncompletePair(t *testing.T) {
	root := t.TempDir()
	keyDir := filepath.Join(root, "deployments", "secrets", "identity")
	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keyDir, "private.pem"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := run([]string{"-root", root}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit 1 on incomplete key pair, got %d", code)
	}
	if !strings.Contains(errOut.String(), "Identity key pair is incomplete; remove the remaining file deliberately and rerun.") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}
