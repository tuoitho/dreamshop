package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"ecommerce.local/scripts/internal/repo"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("generate-identity-dev-keys", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var root string
	fs.StringVar(&root, "root", "", "repository root directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if root == "" {
		r, err := repo.Root()
		if err != nil {
			fmt.Fprintf(errOut, "error locating repository root: %v\n", err)
			return 1
		}
		root = r
	}

	keyDir := filepath.Join(root, "deployments", "secrets", "identity")
	privatePath := filepath.Join(keyDir, "private.pem")
	publicPath := filepath.Join(keyDir, "public.pem")

	privExists := fileExists(privatePath)
	pubExists := fileExists(publicPath)

	if privExists != pubExists {
		fmt.Fprintln(errOut, "Identity key pair is incomplete; remove the remaining file deliberately and rerun.")
		return 1
	}
	if privExists && pubExists {
		fmt.Fprintln(out, "Identity development key pair already exists.")
		return 0
	}

	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		fmt.Fprintf(errOut, "failed to create key directory: %v\n", err)
		return 1
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Fprintf(errOut, "failed to generate RSA key: %v\n", err)
		return 1
	}

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		fmt.Fprintf(errOut, "failed to marshal PKCS#8 private key: %v\n", err)
		return 1
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	spkiBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		fmt.Fprintf(errOut, "failed to marshal PKIX public key: %v\n", err)
		return 1
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: spkiBytes,
	})

	if err := os.WriteFile(privatePath, privPEM, 0o600); err != nil {
		fmt.Fprintf(errOut, "failed to write private key: %v\n", err)
		return 1
	}
	if err := os.WriteFile(publicPath, pubPEM, 0o644); err != nil {
		fmt.Fprintf(errOut, "failed to write public key: %v\n", err)
		return 1
	}

	fmt.Fprintln(out, "Created untracked Identity development key pair.")
	return 0
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
