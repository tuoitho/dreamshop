package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func setupCleanRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "services/cart-service/main.go", "package main\nimport \"ecommerce.local/cart-service/internal/foo\"\nimport \"ecommerce.local/contracts/go/cart/v1\"\n")
	writeTestFile(t, root, "services/cart-service/config.yml", "db: cart_db\n")
	writeTestFile(t, root, "services/order-service/pom.xml", "<project><artifactId>order-service</artifactId></project>\n")
	writeTestFile(t, root, "services/order-service/src/Main.java", "package com.ecommerce.order;\nimport com.ecommerce.order.domain.Order;\nimport com.ecommerce.contracts.order.v1.OrderProto;\n")
	writeTestFile(t, root, "contracts/go/cart.go", "package cartv1\n")
	writeTestFile(t, root, "contracts/java/src/Order.java", "package com.ecommerce.contracts;\nimport com.ecommerce.contracts.common.Base;\n")
	writeTestFile(t, root, "ecommerce-fe/src/App.tsx", "import React from 'react'\n")
	writeTestFile(t, root, "admin-fe/src/App.tsx", "import React from 'react'\n")
	return root
}

func TestCheckBoundariesCleanRepoPasses(t *testing.T) {
	root := setupCleanRepo(t)
	// Excluded directories should be ignored even if they contain matching strings
	writeTestFile(t, root, "services/cart-service/target/bad.go", "import \"ecommerce.local/order-service\"\n")

	var out bytes.Buffer
	code := run([]string{"-root", root}, &out)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "Boundary check passed for 2 services, contracts, ecommerce-fe, admin-fe, and e2e.") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestCheckBoundariesDetectsViolations(t *testing.T) {
	root := setupCleanRepo(t)
	writeTestFile(t, root, "services/cart-service/bad.go", "package main\nimport \"ecommerce.local/order-service/pkg\"\n")
	writeTestFile(t, root, "services/order-service/src/Bad.java", "import com.ecommerce.cart.Cart;\n")
	writeTestFile(t, root, "services/order-service/pom.xml", "<project><artifactId>order-service</artifactId><dependency><artifactId>cart-service</artifactId></dependency></project>\n")
	writeTestFile(t, root, "services/cart-service/application.yml", "url: jdbc:postgresql://localhost/order_db\n")
	writeTestFile(t, root, "contracts/go/bad.go", "import \"ecommerce.local/cart-service/foo\"\n")
	writeTestFile(t, root, "contracts/java/src/Bad.java", "import com.ecommerce.order.Foo;\n")
	writeTestFile(t, root, "ecommerce-fe/src/bad.ts", "import { x } from '../../services/cart-service/internal/foo'\n")
	writeTestFile(t, root, "ecommerce-fe/src/badProto.ts", "import proto from '../../contracts/proto/cart/v1/cart.proto'\n")
	writeTestFile(t, root, "ecommerce-fe/src/badAdmin.ts", "import { x } from '../../admin-fe/src/lib/api'\n")
	writeTestFile(t, root, "admin-fe/src/badService.ts", "import { x } from '../../services/catalog-service/internal/domain/product'\n")
	writeTestFile(t, root, "admin-fe/src/badProto.ts", "import proto from '../../contracts/proto/catalog/v1/catalog.proto'\n")
	writeTestFile(t, root, "admin-fe/src/badStorefront.ts", "import { x } from '../../ecommerce-fe/src/lib/api'\n")
	writeTestFile(t, root, "services/cart-service/internal/kong_call.go", "package internal\nconst url = \"http://localhost:8000/api/v1/catalog\"\n")

	var out bytes.Buffer
	code := run([]string{"-root", root}, &out)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d; output:\n%s", code, out.String())
	}

	got := out.String()
	expectedSubstrings := []string{
		"services/cart-service/bad.go:2: cross-service Go import: ecommerce.local/order-service",
		"services/order-service/src/Bad.java:1: cross-service Java import: import com.ecommerce.cart",
		"services/order-service/pom.xml:1: cross-service Maven dependency: <artifactId>cart-service</artifactId>",
		"services/cart-service/application.yml:1: foreign service database reference: order_db",
		"services/cart-service/internal/kong_call.go:2: internal service call through Kong: localhost:8000",
		"contracts/go/bad.go:1: contract module imports service code: ecommerce.local/cart-service",
		"contracts/java/src/Bad.java:1: contract module imports service code: import com.ecommerce.order",
		"ecommerce-fe/src/bad.ts:1: frontend imports backend source:",
		"ecommerce-fe/src/badProto.ts:1: frontend imports backend source:",
		"ecommerce-fe/src/badAdmin.ts:1: cross-frontend import:",
		"admin-fe/src/badService.ts:1: frontend imports backend source:",
		"admin-fe/src/badProto.ts:1: frontend imports backend source:",
		"admin-fe/src/badStorefront.ts:1: cross-frontend import:",
		"Boundary check failed: 13 violation(s). See docs/steering/service-boundaries.md.",
	}
	for _, want := range expectedSubstrings {
		if !strings.Contains(got, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}
