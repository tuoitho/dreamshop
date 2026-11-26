// Reference template: Backend contract snapshot recorder cho Go HTTP handler tests (services/bff-service).
// Chỉ dùng Go standard library (không thêm dependency ngoài vào go.mod).
//
// Nguyên tắc domain-grouped: mỗi nhóm handler BFF ghi vào đúng file domain của mình dưới
// contracts/http-snapshots/ (ví dụ: bff-storefront.json, bff-orders.json, bff-admin.json)
// để độc lập hoàn toàn với identity-service (identity-auth.json, identity-admin.json).
//
// Cách dùng trong services/bff-service/internal/http/*_test.go:
//   snapshotPath := resolveContractSnapshotPath("bff-orders.json")
//   assertOrRecordContractSnapshot(t, snapshotPath, "bff-orders", "orders_list_200",
//       "bff-service", "GET /api/v1/orders", nil, rec)
package http

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type contractSnapshotFile struct {
	Domain    string                           `json:"domain"`
	Snapshots map[string]contractSnapshotEntry `json:"snapshots"`
}

type contractSnapshotEntry struct {
	Service      string `json:"service"` // "bff-service" | "identity-service"
	Endpoint     string `json:"endpoint"`
	Status       int    `json:"status"`
	RequestBody  any    `json:"requestBody,omitempty"`
	ResponseBody any    `json:"responseBody"`
}

// resolveContractSnapshotPath trả về đường dẫn chuẩn tới contracts/http-snapshots/<fileName>
// khi chạy test từ services/bff-service/internal/http/ (4 cấp lên repo root), có hỗ trợ override.
func resolveContractSnapshotPath(fileName string) string {
	if override := os.Getenv("CONTRACT_SNAPSHOT_PATH"); override != "" {
		return override
	}
	return filepath.Clean(filepath.Join("..", "..", "..", "..", "contracts", "http-snapshots", fileName))
}

func assertOrRecordContractSnapshot(
	t *testing.T,
	snapshotFilePath string,
	domain string,
	snapshotKey string,
	service string,
	endpoint string,
	requestBody any,
	recorder *httptest.ResponseRecorder,
) {
	t.Helper()

	var actualBody any
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &actualBody); err != nil {
			t.Fatalf("response body must be valid JSON for %s (%s): %v", snapshotKey, endpoint, err)
		}
	}

	// Chuẩn hóa requestBody qua JSON roundtrip để số int/float có kiểu float64 đồng nhất với json.Unmarshal.
	var normalizedReq any
	if requestBody != nil {
		rawReq, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("marshal requestBody for %s: %v", snapshotKey, err)
		}
		if err := json.Unmarshal(rawReq, &normalizedReq); err != nil {
			t.Fatalf("unmarshal normalized requestBody for %s: %v", snapshotKey, err)
		}
	}

	entry := contractSnapshotEntry{
		Service:      service,
		Endpoint:     endpoint,
		Status:       recorder.Code,
		RequestBody:  normalizedReq,
		ResponseBody: actualBody,
	}

	doc := contractSnapshotFile{
		Domain:    domain,
		Snapshots: map[string]contractSnapshotEntry{},
	}

	if os.Getenv("UPDATE_CONTRACT_SNAPSHOTS") == "1" {
		if _, err := os.Stat(snapshotFilePath); err == nil {
			raw, err := os.ReadFile(snapshotFilePath)
			if err != nil {
				t.Fatalf("read snapshot file %s: %v", snapshotFilePath, err)
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("parse snapshot file %s: %v", snapshotFilePath, err)
			}
			if doc.Domain == "" {
				doc.Domain = domain
			}
			if doc.Snapshots == nil {
				doc.Snapshots = map[string]contractSnapshotEntry{}
			}
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat snapshot file %s: %v", snapshotFilePath, err)
		}
		doc.Snapshots[snapshotKey] = entry
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			t.Fatalf("marshal snapshot file: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(snapshotFilePath), 0o755); err != nil {
			t.Fatalf("mkdir snapshot dir: %v", err)
		}
		if err := os.WriteFile(snapshotFilePath, append(out, '\n'), 0o644); err != nil {
			t.Fatalf("write snapshot file: %v", err)
		}
		return
	}

	raw, err := os.ReadFile(snapshotFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("missing contract snapshot file %s for key %q (run with UPDATE_CONTRACT_SNAPSHOTS=1 to record)", snapshotFilePath, snapshotKey)
		}
		t.Fatalf("read snapshot file %s: %v", snapshotFilePath, err)
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse snapshot file %s: %v", snapshotFilePath, err)
	}

	expected, exists := doc.Snapshots[snapshotKey]
	if !exists {
		t.Fatalf("missing contract snapshot key %q in %s (run with UPDATE_CONTRACT_SNAPSHOTS=1 to record)", snapshotKey, snapshotFilePath)
	}
	if expected.Service != entry.Service {
		t.Fatalf("service mismatch for %s: expected %q, got %q", snapshotKey, expected.Service, entry.Service)
	}
	if expected.Endpoint != entry.Endpoint {
		t.Fatalf("endpoint mismatch for %s: expected %q, got %q", snapshotKey, expected.Endpoint, entry.Endpoint)
	}
	if expected.Status != entry.Status {
		t.Fatalf("HTTP status mismatch for %s: expected %d, got %d", snapshotKey, expected.Status, entry.Status)
	}

	assertContractShapeEqual(t, expected.ResponseBody, entry.ResponseBody, snapshotKey+".responseBody")
	if expected.RequestBody != nil || entry.RequestBody != nil {
		assertContractShapeEqual(t, expected.RequestBody, entry.RequestBody, snapshotKey+".requestBody")
	}
}

func assertContractShapeEqual(t *testing.T, expected any, actual any, path string) {
	t.Helper()
	expectedShape := extractJSONShape(expected)
	actualShape := extractJSONShape(actual)
	if !reflect.DeepEqual(expectedShape, actualShape) {
		t.Fatalf("contract shape drift at %s:\nexpected shape: %#v\nactual shape:   %#v", path, expectedShape, actualShape)
	}
}

// extractJSONShape rút trích cấu trúc khóa và kiểu dữ liệu JSON.
// Với mảng không rỗng, chuẩn hóa về tập shape phần tử duy nhất đã sắp xếp để số lượng phần tử
// thay đổi (vd: 1 item vs 2 items) không gây false drift trong khi sai kiểu/key vẫn bị bắt.
func extractJSONShape(v any) any {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			out[k] = extractJSONShape(child)
		}
		return out
	case []any:
		if len(val) == 0 {
			return []any{}
		}
		seen := make(map[string]any, len(val))
		keys := make([]string, 0, len(val))
		for _, child := range val {
			childShape := extractJSONShape(child)
			canonicalBytes, err := json.Marshal(childShape)
			canonical := string(canonicalBytes)
			if err != nil {
				canonical = reflect.TypeOf(childShape).String()
			}
			if _, ok := seen[canonical]; !ok {
				seen[canonical] = childShape
				keys = append(keys, canonical)
			}
		}
		sort.Strings(keys)
		out := make([]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, seen[k])
		}
		return out
	case string:
		return "string"
	case float64, int, int32, int64, uint32, uint64:
		return "number"
	case bool:
		return "boolean"
	default:
		return reflect.TypeOf(v).String()
	}
}
