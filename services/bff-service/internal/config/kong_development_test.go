package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDevelopmentKongSeparatesProductAndIdentityAdminRoutes(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
	configPath := filepath.Join(root, "deployments", "kong", "kong.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read development Kong config: %v", err)
	}

	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("parse development Kong config: %v", err)
	}
	services, ok := document["services"].([]any)
	if !ok {
		t.Fatal("development Kong config has no services")
	}

	var productService map[string]any
	for _, rawService := range services {
		service, ok := rawService.(map[string]any)
		if ok && service["name"] == "bff-admin-products" {
			productService = service
			break
		}
	}
	if productService == nil {
		t.Fatal("development Kong config is missing bff-admin-products service")
	}
	if productService["url"] != "http://host.docker.internal:8081/products" {
		t.Fatalf("admin products upstream = %v", productService["url"])
	}
	routes, ok := productService["routes"].([]any)
	if !ok || len(routes) != 1 {
		t.Fatalf("admin products routes = %#v", productService["routes"])
	}
	route := routes[0].(map[string]any)
	paths := route["paths"].([]any)
	if route["name"] != "admin-products-api" || len(paths) != 1 || paths[0] != "/api/v1/admin/products" || route["strip_path"] != true {
		t.Fatalf("admin products route = %#v", route)
	}
	plugins, ok := route["plugins"].([]any)
	if !ok || len(plugins) != 1 {
		t.Fatalf("development admin-products-api plugins = %#v, want caller-context", route["plugins"])
	}
	callerPlugin := plugins[0].(map[string]any)
	callerConfig, _ := callerPlugin["config"].(map[string]any)
	if callerPlugin["name"] != "caller-context" ||
		callerConfig["jwks_uri"] != "http://host.docker.internal:8080/.well-known/jwks.json" ||
		callerConfig["issuer"] != "http://localhost:8080" ||
		callerConfig["audience"] != "ecommerce-api" {
		t.Fatalf("development admin-products-api caller-context = %#v", callerPlugin)
	}

	for _, rawService := range services {
		service := rawService.(map[string]any)
		for _, rawRoute := range service["routes"].([]any) {
			route := rawRoute.(map[string]any)
			for _, rawPath := range route["paths"].([]any) {
				if rawPath == "/api/v1/admin" {
					t.Fatalf("route %s shadows all Identity admin APIs", route["name"])
				}
			}
		}
	}
}
