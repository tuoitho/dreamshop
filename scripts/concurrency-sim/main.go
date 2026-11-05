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
	"strings"
	"time"
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
		jsonOutput      bool
	)

	fs.StringVar(&scenario, "scenario", "all", "scenario to run: all|flash-sale-oversell|snapshot-mismatch-race|idempotency-storm|momo-callback-race")
	fs.StringVar(&baseURL, "base-url", "http://localhost:8000/api/v1", "public API base URL")
	fs.StringVar(&adminToken, "admin-token", "", "Bearer JWT for admin user")
	fs.StringVar(&customerToken, "customer-token", "", "Bearer JWT for customer user")
	fs.StringVar(&adminEmail, "admin-email", "", "admin login email (used if -admin-token is empty)")
	fs.StringVar(&adminPass, "admin-password", "", "admin login password")
	fs.StringVar(&customerEmail, "customer-email", "", "customer login email (used if -customer-token is empty)")
	fs.StringVar(&customerPass, "customer-password", "", "customer login password")
	fs.StringVar(&momoPartnerCode, "momo-partner-code", envOr("MOMO_PARTNER_CODE", "MOMO"), "MoMo partnerCode")
	fs.StringVar(&momoAccessKey, "momo-access-key", envOr("MOMO_ACCESS_KEY", "F8BBA842ECF85"), "MoMo accessKey")
	fs.StringVar(&momoSecretKey, "momo-secret-key", envOr("MOMO_SECRET_KEY", "K951B6PE1waDMi640xX08PD3vg6EkVlz"), "MoMo secretKey")
	fs.BoolVar(&jsonOutput, "json", false, "emit JSON report")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
			updateBody["price"] = int64(12000000)
			if variants, ok := updateBody["variants"].([]map[string]any); ok && len(variants) > 0 {
				variants[0]["price"] = int64(12000000)
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
		"name":          name,
		"brand":         "SimLab",
		"categorySlug":  "dien-thoai",
		"categoryName":  "Điện thoại",
		"price":         price,
		"originalPrice": price,
		"imageEmoji":    "⚡",
		"imageHue":      []int{210, 250},
		"description":   "Concurrency simulation test product",
		"specs":         map[string]string{"Mode": "Sim"},
		"tags":          []string{"sim"},
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

	status, err := putJSON(ctx, client, strings.TrimRight(baseURL, "/")+"/admin/inventory/"+skuID, adminToken, map[string]any{
		"quantity": qty,
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

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
