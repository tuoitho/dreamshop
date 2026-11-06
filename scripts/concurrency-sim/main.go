package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ecommerce.local/scripts/internal/repo"
)

const (
	defaultMomoPartnerCode = "MOMO_LOCAL"
	defaultMomoAccessKey   = "local-access-key"
	defaultMomoSecretKey   = "local-secret-key-change-me"
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("concurrency-sim", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		scenario        string
		baseURL         string
		adminToken      string
		customerToken   string
		adminEmail      string
		adminPass       string
		customerEmail   string
		customerPass    string
		momoPartnerCode string
		momoAccessKey   string
		momoSecretKey   string
		verifySQL       bool
		pgContainer     string
		jsonOutput      bool
		settleTimeout   time.Duration
	)

	dotenv := map[string]string{}
	if root, err := repo.Root(); err == nil {
		dotenv = loadDotenvFallback(filepath.Join(root, ".env"))
	}

	fs.StringVar(&scenario, "scenario", "all", "scenario to run: all|flash-sale-oversell|snapshot-mismatch-race|idempotency-storm|momo-callback-race")
	fs.StringVar(&baseURL, "base-url", "http://localhost:8000/api/v1", "public API base URL")
	fs.StringVar(&adminToken, "admin-token", "", "Bearer JWT for admin user")
	fs.StringVar(&customerToken, "customer-token", "", "Bearer JWT for customer user")
	fs.StringVar(&adminEmail, "admin-email", "", "admin login email (used if -admin-token is empty)")
	fs.StringVar(&adminPass, "admin-password", "", "admin login password")
	fs.StringVar(&customerEmail, "customer-email", "", "customer login email (used if -customer-token is empty)")
	fs.StringVar(&customerPass, "customer-password", "", "customer login password")
	fs.StringVar(&momoPartnerCode, "momo-partner-code", resolveEnv(dotenv, "MOMO_PARTNER_CODE", defaultMomoPartnerCode), "MoMo partnerCode")
	fs.StringVar(&momoAccessKey, "momo-access-key", resolveEnv(dotenv, "MOMO_ACCESS_KEY", defaultMomoAccessKey), "MoMo accessKey")
	fs.StringVar(&momoSecretKey, "momo-secret-key", resolveEnv(dotenv, "MOMO_SECRET_KEY", defaultMomoSecretKey), "MoMo secretKey")
	fs.BoolVar(&verifySQL, "verify-sql", false, "verify cross-database SQL invariants via docker exec psql")
	fs.StringVar(&pgContainer, "pg-container", "ecommerce-postgres", "PostgreSQL Docker container name for -verify-sql")
	fs.BoolVar(&jsonOutput, "json", false, "emit JSON report")
	fs.DurationVar(&settleTimeout, "settle-timeout", 10*time.Second, "max time to wait for an async order to reach a terminal state")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute+4*settleTimeout)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second}

	if adminToken == "" && adminEmail != "" {
		tok, err := loginForToken(ctx, client, baseURL, adminEmail, adminPass)
		if err != nil {
			fmt.Fprintf(errOut, "admin login failed: %v\n", err)
			return 1
		}
		adminToken = tok
	}
	if customerToken == "" && customerEmail != "" {
		tok, err := loginForToken(ctx, client, baseURL, customerEmail, customerPass)
		if err != nil {
			fmt.Fprintf(errOut, "customer login failed: %v\n", err)
			return 1
		}
		customerToken = tok
	}

	if adminToken == "" || customerToken == "" {
		fmt.Fprintln(errOut, "both admin and customer credentials/tokens are required to run live scenarios")
		return 2
	}

	shipping := ShippingPayload{
		RecipientName: "Concurrency Runner",
		Phone:         "0901234567",
		ProvinceCode:  "79",
		ProvinceName:  "TP. Hồ Chí Minh",
		WardCode:      "26740",
		WardName:      "Phường Bến Nghé",
		AddressLine:   "1 Lê Duẩn",
	}

	var results []ScenarioResult
	runOne := func(name string) error {
		switch name {
		case "flash-sale-oversell":
			prod, _, err := seedTestProductAndStock(ctx, client, baseURL, adminToken, "FlashSim", 1200000, 5)
			if err != nil {
				return err
			}
			r, err := RunFlashSaleOversell(ctx, FlashSaleConfig{
				BaseURL:        baseURL,
				HTTPClient:     client,
				AdminToken:     adminToken,
				CustomerTokens: []string{customerToken},
				Product:        prod,
				SettleTimeout:  settleTimeout,
				InitialStock:   5,
				Buyers:         50,
				Shipping:       shipping,
			})
			if err != nil {
				return err
			}
			results = append(results, r)
		case "snapshot-mismatch-race":
			prod, updateBody, err := seedTestProductAndStock(ctx, client, baseURL, adminToken, "SnapSim", 10000000, 100)
			if err != nil {
				return err
			}
			if variants, ok := updateBody["variants"].([]map[string]any); ok && len(variants) > 0 {
				variants[0]["price"] = int64(12000000)
				variants[0]["originalPrice"] = int64(12000000)
			}
			r, err := RunSnapshotMismatchRace(ctx, SnapshotRaceConfig{
				BaseURL:            baseURL,
				HTTPClient:         client,
				AdminToken:         adminToken,
				CustomerToken:      customerToken,
				Product:            prod,
				NewPrice:           12000000,
				AdminUpdatePayload: updateBody,
				Buyers:             20,
				Shipping:           shipping,
			})
			if err != nil {
				return err
			}
			results = append(results, r)
		case "idempotency-storm":
			prod, _, err := seedTestProductAndStock(ctx, client, baseURL, adminToken, "IdemSim", 1500000, 50)
			if err != nil {
				return err
			}
			r, err := RunIdempotencyStorm(ctx, IdempotencyStormConfig{
				BaseURL:               baseURL,
				HTTPClient:            client,
				CustomerToken:         customerToken,
				Product:               prod,
				SamePayloadWorkers:    20,
				MutatedPayloadWorkers: 5,
				Shipping:              shipping,
			})
			if err != nil {
				return err
			}
			results = append(results, r)
		case "momo-callback-race":
			prod, _, err := seedTestProductAndStock(ctx, client, baseURL, adminToken, "MomoSim", 2500000, 20)
			if err != nil {
				return err
			}
			r, err := RunMomoCallbackRace(ctx, MomoCallbackRaceConfig{
				BaseURL:         baseURL,
				HTTPClient:      client,
				CustomerToken:   customerToken,
				PartnerCode:     momoPartnerCode,
				AccessKey:       momoAccessKey,
				SecretKey:       momoSecretKey,
				ValidRedirects:  10,
				ValidIPNs:       10,
				ForgedCallbacks: 5,
				CancelRequests:  5,
				Product:         prod,
				SettleTimeout:   settleTimeout,
				Shipping:        shipping,
			})
			if err != nil {
				return err
			}
			results = append(results, r)
		default:
			return fmt.Errorf("unknown scenario %q", name)
		}
		return nil
	}

	targets := []string{scenario}
	if scenario == "all" {
		targets = []string{
			"flash-sale-oversell",
			"snapshot-mismatch-race",
			"idempotency-storm",
			"momo-callback-race",
		}
	}

	for _, target := range targets {
		if err := runOne(target); err != nil {
			fmt.Fprintf(errOut, "scenario %s setup/exec error: %v\n", target, err)
			return 1
		}
	}

	if verifySQL {
		results = append(results, checkDatabaseInvariants(ctx, pgContainer))
	}

	if jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
	} else {
		printSummaryTable(out, results)
	}

	for _, r := range results {
		if !r.Passed {
			return 1
		}
	}
	return 0
}

func printSummaryTable(out io.Writer, results []ScenarioResult) {
	fmt.Fprintf(out, "%-26s %-8s %-10s %-10s %-10s %s\n", "SCENARIO", "RESULT", "WORKERS", "P50(ms)", "P95(ms)", "DETAILS")
	fmt.Fprintln(out, strings.Repeat("-", 96))
	for _, r := range results {
		verdict := "PASS"
		if !r.Passed {
			verdict = "FAIL"
		}
		detail := fmt.Sprintf("status=%v errors=%v", r.StatusHistogram, r.ErrorHistogram)
		fmt.Fprintf(out, "%-26s %-8s %-10d %-10.2f %-10.2f %s\n", r.Scenario, verdict, r.Goroutines, r.P50Ms, r.P95Ms, detail)
		for _, note := range r.Notes {
			fmt.Fprintf(out, "  note: %s\n", note)
		}
		for _, v := range r.Violations {
			fmt.Fprintf(out, "  violation: %s\n", v)
		}
	}
}

func loginForToken(ctx context.Context, client *http.Client, baseURL, email, password string) (string, error) {
	raw, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/auth/login", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	var env struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", err
	}
	return env.Data.AccessToken, nil
}

func seedTestProductAndStock(ctx context.Context, client *http.Client, baseURL, adminToken, prefix string, price int64, qty int) (SnapshotProduct, map[string]any, error) {
	suffix := randomHex(4)
	skuID := strings.ToLower(prefix) + "-sku-" + suffix
	name := prefix + " " + suffix
	createBody := map[string]any{
		"name":             name,
		"shortDescription": "Concurrency simulation test product",
		"description":      "Concurrency simulation test product",
		"brand":            "SimLab",
		"categoryId":       "dien-thoai",
		"categoryName":     "Điện thoại",
		"images":           []string{},
		"videos":           []string{},
		"isNew":            false,
		"badge":            "",
		"featured":         false,
		"emoji":            "⚡",
		"imageHue":         []int{210, 250},
		"details":          []string{"Simulation mode"},
		"variants": []map[string]any{
			{
				"skuId":         skuID,
				"label":         "Standard",
				"colorHex":      "#111827",
				"price":         price,
				"originalPrice": price,
			},
		},
	}

	raw, _ := json.Marshal(createBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/admin/products", bytes.NewReader(raw))
	if err != nil {
		return SnapshotProduct{}, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := client.Do(req)
	if err != nil {
		return SnapshotProduct{}, nil, err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return SnapshotProduct{}, nil, fmt.Errorf("create product HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var prodEnv struct {
		Data struct {
			ID      string `json:"id"`
			Version int64  `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &prodEnv); err != nil {
		return SnapshotProduct{}, nil, err
	}

	status, err := putJSON(ctx, client, strings.TrimRight(baseURL, "/")+"/admin/inventory/skus/"+skuID, adminToken, map[string]any{
		"productId": prodEnv.Data.ID,
		"quantity":  qty,
	})
	if err != nil || (status != http.StatusOK && status != http.StatusCreated) {
		return SnapshotProduct{}, nil, fmt.Errorf("set stock HTTP %d err=%v", status, err)
	}

	ver := prodEnv.Data.Version
	if ver <= 0 {
		ver = 1
	}

	return SnapshotProduct{
		ProductID:      prodEnv.Data.ID,
		SKUID:          skuID,
		Name:           name,
		Brand:          "SimLab",
		CategoryName:   "Điện thoại",
		VariantLabel:   "Standard",
		ImageEmoji:     "⚡",
		UnitPrice:      price,
		ProductVersion: ver,
	}, createBody, nil
}

func checkDatabaseInvariants(ctx context.Context, container string) ScenarioResult {
	res := ScenarioResult{
		Scenario:        "sql-invariants",
		Goroutines:      1,
		StatusHistogram: map[int]int{},
		ErrorHistogram:  map[string]int{},
	}

	checks := []struct {
		db    string
		label string
		sql   string
	}{
		{
			db:    "inventory_db",
			label: "negative_or_overflow_stock",
			sql:   "SELECT COUNT(*) FROM stock WHERE quantity < 0 OR reserved < 0 OR reserved > quantity;",
		},
		{
			db:    "order_db",
			label: "confirmed_with_refund_required",
			sql:   "SELECT COUNT(*) FROM orders WHERE status = 'CONFIRMED' AND payment_status = 'REFUND_REQUIRED';",
		},
		{
			db:    "payment_db",
			label: "duplicate_payment_sessions_per_order",
			sql:   "SELECT COUNT(*) FROM (SELECT order_id FROM payments GROUP BY order_id HAVING COUNT(*) > 1) dup;",
		},
	}

	for _, c := range checks {
		cmd := exec.CommandContext(ctx, "docker", "exec", container, "psql", "-U", "ecommerce", "-d", c.db, "-tAc", c.sql)
		raw, err := cmd.CombinedOutput()
		if err != nil {
			res.Violations = append(res.Violations, fmt.Sprintf("%s (%s) query failed: %v (%s)", c.db, c.label, err, strings.TrimSpace(string(raw))))
			continue
		}
		val := strings.TrimSpace(string(raw))
		if val != "0" {
			res.Violations = append(res.Violations, fmt.Sprintf("%s (%s) invariant violated: count=%s", c.db, c.label, val))
		} else {
			res.Notes = append(res.Notes, fmt.Sprintf("%s.%s=0", c.db, c.label))
		}
	}

	res.Passed = len(res.Violations) == 0
	return res
}

func loadDotenvFallback(path string) map[string]string {
	vals := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return vals
	}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if k != "" {
			if _, exists := vals[k]; !exists {
				vals[k] = v
			}
		}
	}
	return vals
}

func resolveEnv(dotenv map[string]string, key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	if v := strings.TrimSpace(dotenv[key]); v != "" {
		return v
	}
	return fallback
}
