package boundaries

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckAllowsE2EReadingHttpSnapshotsAndRejectsSourceImports(t *testing.T) {
	root := t.TempDir()

	writeFile := func(rel, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("services/bff-service/go.mod", "module ecommerce.local/bff-service\n")
	writeFile("contracts/http-snapshots/bff-storefront.json", `{"domain":"bff-storefront","snapshots":{}}`)
	writeFile("e2e/tests/ok.spec.ts", `
import * as fs from "node:fs"
const raw = fs.readFileSync("../contracts/http-snapshots/bff-storefront.json", "utf-8")
export const ok = raw
`)

	var out bytes.Buffer
	if code := Check(root, &out); code != 0 {
		t.Fatalf("expected clean e2e snapshot read to pass, got %d:\n%s", code, out.String())
	}

	writeFile("e2e/tests/bad.spec.ts", `import { x } from "../../ecommerce-fe/src/lib/api"`)
	out.Reset()
	if code := Check(root, &out); code == 0 {
		t.Fatalf("expected forbidden e2e frontend source import to fail, got pass:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "e2e imports service, contract code, or frontend source") {
		t.Fatalf("unexpected violation output:\n%s", out.String())
	}
}
