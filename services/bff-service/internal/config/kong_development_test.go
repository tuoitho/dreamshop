package config

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDevelopmentKongConsolidatesTwoServicesWithStripPathFalseAndPreservesCORS(t *testing.T) {
	root := repoRoot(t)
	raw := readRepoFile(t, root, "deployments", "kong", "kong.yaml")

	var doc kongFullConfig
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("parse kong.yaml: %v", err)
	}
	if doc.FormatVersion != "3.0" {
		t.Fatalf("_format_version = %q, want 3.0", doc.FormatVersion)
	}
	if len(doc.Services) != 2 {
		t.Fatalf("kong.yaml services count = %d, want 2 (identity-service, bff-service)", len(doc.Services))
	}

	corsPlugin, hasCORS := hasPlugin(doc.Plugins, "cors")
	if !hasCORS || !slices.Contains(corsPlugin.Config.Origins, "http://localhost:5173") || !slices.Contains(corsPlugin.Config.Methods, "OPTIONS") {
		t.Fatalf("kong.yaml global cors plugin = %#v (present=%t), want localhost:5173 with OPTIONS", corsPlugin.Config, hasCORS)
	}

	expectedServices := map[string]string{
		"identity-service": "http://host.docker.internal:8080",
		"bff-service":      "http://host.docker.internal:8081",
	}

	type devRouteExpectation struct {
		service   string
		paths     []string
		protected bool
	}
	expectedRoutes := map[string]devRouteExpectation{
		"identity-auth-public": {
			service:   "identity-service",
			paths:     []string{"/api/v1/auth"},
			protected: false,
		},
		"identity-protected": {
			service: "identity-service",
			paths: []string{
				"/api/v1/users",
				"/api/v1/locations",
				"/api/v1/admin/users",
				"/api/v1/admin/rbac",
			},
			protected: true,
		},
		"storefront-api": {
			service:   "bff-service",
			paths:     []string{"/api/v1/storefront/products"},
			protected: false,
		},
		"cart-api": {
			service:   "bff-service",
			paths:     []string{"/api/v1/cart"},
			protected: true,
		},
		"admin-products-api": {
			service:   "bff-service",
			paths:     []string{"/api/v1/admin/products"},
			protected: true,
		},
	}

	seenRoutes := make(map[string]bool)
	for _, svc := range doc.Services {
		wantURL, ok := expectedServices[svc.Name]
		if !ok {
			t.Fatalf("unexpected service %q in kong.yaml", svc.Name)
		}
		if svc.URL != wantURL {
			t.Errorf("service %q url = %q, want %q", svc.Name, svc.URL, wantURL)
		}
		if removesCallerHeaders(svc.Plugins) {
			t.Errorf("service %q must not attach service-level request-transformer", svc.Name)
		}

		for _, route := range svc.Routes {
			if route.StripPath == nil || *route.StripPath {
				t.Errorf("dev route %q must set strip_path: false", route.Name)
			}
			if len(route.Methods) > 0 && !slices.Contains(route.Methods, "OPTIONS") {
				t.Errorf("dev route %q restricts methods %v without OPTIONS, breaking browser CORS preflight", route.Name, route.Methods)
			}
			for _, p := range route.Paths {
				if p == "/api/v1" || p == "/api/v1/admin" || p == "/api/v1/storefront" {
					t.Errorf("dev route %q uses overly broad path %q", route.Name, p)
				}
			}

			exp, known := expectedRoutes[route.Name]
			if !known {
				t.Errorf("unexpected dev route %q on service %q", route.Name, svc.Name)
				continue
			}
			seenRoutes[route.Name] = true
			if svc.Name != exp.service {
				t.Errorf("dev route %q attached to %q, want %q", route.Name, svc.Name, exp.service)
			}
			if !slices.Equal(route.Paths, exp.paths) {
				t.Errorf("dev route %q paths = %v, want %v", route.Name, route.Paths, exp.paths)
			}
			if exp.protected {
				cc, ok := hasPlugin(route.Plugins, "caller-context")
				if !ok || cc.Config.JWKSURI != "http://host.docker.internal:8080/.well-known/jwks.json" || cc.Config.Issuer != "http://localhost:8080" || cc.Config.Audience != "ecommerce-api" {
					t.Errorf("dev protected route %q caller-context = %#v (present=%t)", route.Name, cc.Config, ok)
				}
				if removesCallerHeaders(route.Plugins) {
					t.Errorf("dev protected route %q must not attach request-transformer alongside caller-context", route.Name)
				}
			} else if !removesCallerHeaders(route.Plugins) {
				t.Errorf("dev unauthenticated route %q must strip X-User-Id and X-User-Roles via request-transformer", route.Name)
			}
		}
	}

	for name := range expectedRoutes {
		if !seenRoutes[name] {
			t.Errorf("missing expected dev route %q", name)
		}
	}
}
