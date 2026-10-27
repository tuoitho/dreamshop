package config

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type kongFullConfig struct {
	FormatVersion string              `yaml:"_format_version"`
	Services      []kongServiceConfig `yaml:"services"`
	Plugins       []kongPluginConfig  `yaml:"plugins"`
}

type kongServiceConfig struct {
	Name    string             `yaml:"name"`
	URL     string             `yaml:"url"`
	Plugins []kongPluginConfig `yaml:"plugins"`
	Routes  []kongRouteConfig  `yaml:"routes"`
}

type kongRouteConfig struct {
	Name      string             `yaml:"name"`
	Paths     []string           `yaml:"paths"`
	Methods   []string           `yaml:"methods"`
	StripPath *bool              `yaml:"strip_path"`
	Plugins   []kongPluginConfig `yaml:"plugins"`
}

type kongPluginConfig struct {
	Name   string `yaml:"name"`
	Config struct {
		JWKSURI  string   `yaml:"jwks_uri"`
		Issuer   string   `yaml:"issuer"`
		Audience string   `yaml:"audience"`
		Origins  []string `yaml:"origins"`
		Methods  []string `yaml:"methods"`
		Remove   struct {
			Headers []string `yaml:"headers"`
		} `yaml:"remove"`
	} `yaml:"config"`
}

func hasPlugin(plugins []kongPluginConfig, name string) (kongPluginConfig, bool) {
	for _, p := range plugins {
		if p.Name == name {
			return p, true
		}
	}
	return kongPluginConfig{}, false
}

func removesCallerHeaders(plugins []kongPluginConfig) bool {
	p, ok := hasPlugin(plugins, "request-transformer")
	if !ok {
		return false
	}
	var hasUserID, hasUserRoles bool
	for _, h := range p.Config.Remove.Headers {
		switch strings.ToLower(h) {
		case "x-user-id":
			hasUserID = true
		case "x-user-roles":
			hasUserRoles = true
		}
	}
	return hasUserID && hasUserRoles
}

func TestProductionKongConsolidatesServicesWithStripPathFalseAndServesSPA(t *testing.T) {
	root := repoRoot(t)
	raw := readRepoFile(t, root, "deployments", "kong", "kong-production.yaml")

	var doc kongFullConfig
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("parse kong-production.yaml: %v", err)
	}
	if doc.FormatVersion != "3.0" {
		t.Fatalf("_format_version = %q, want 3.0", doc.FormatVersion)
	}
	if strings.Contains(raw, "frontend.example.com") {
		t.Fatal("kong-production.yaml must not contain placeholder frontend.example.com CORS origin")
	}

	if len(doc.Services) != 3 {
		t.Fatalf("kong-production.yaml services count = %d, want 3 (identity-service, bff-service, ecommerce-fe)", len(doc.Services))
	}

	expectedServices := map[string]string{
		"identity-service": "http://identity-service:8080",
		"bff-service":      "http://bff-service:8080",
		"ecommerce-fe":     "http://ecommerce-fe:8080",
	}

	type routeExpectation struct {
		service   string
		paths     []string
		methods   []string
		protected bool
	}
	expectedRoutes := map[string]routeExpectation{
		"identity-auth-public": {
			service: "identity-service",
			paths: []string{
				"~/api/v1/auth/register$",
				"~/api/v1/auth/login$",
				"~/api/v1/auth/refresh$",
				"~/api/v1/auth/logout$",
			},
			methods:   []string{"POST"},
			protected: false,
		},
		"identity-auth-jwks": {
			service:   "identity-service",
			paths:     []string{`~/api/v1/auth/\.well-known/jwks\.json$`},
			methods:   []string{"GET"},
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
			methods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
			protected: true,
		},
		"storefront-api": {
			service:   "bff-service",
			paths:     []string{"/api/v1/storefront/products"},
			methods:   []string{"GET"},
			protected: false,
		},
		"cart-api": {
			service:   "bff-service",
			paths:     []string{"/api/v1/cart"},
			methods:   []string{"GET", "POST", "PATCH", "DELETE"},
			protected: true,
		},
		"favorites-api": {
			service:   "bff-service",
			paths:     []string{"/api/v1/favorites"},
			methods:   []string{"GET", "POST", "DELETE"},
			protected: true,
		},
		"admin-products-api": {
			service:   "bff-service",
			paths:     []string{"/api/v1/admin/products"},
			methods:   []string{"POST"},
			protected: true,
		},
		"frontend-spa": {
			service:   "ecommerce-fe",
			paths:     []string{"/"},
			methods:   []string{"GET", "HEAD"},
			protected: false,
		},
	}

	seenServices := make(map[string]bool)
	seenRoutes := make(map[string]bool)

	for _, svc := range doc.Services {
		wantURL, ok := expectedServices[svc.Name]
		if !ok {
			t.Errorf("unexpected service %q in kong-production.yaml", svc.Name)
			continue
		}
		seenServices[svc.Name] = true
		if svc.URL != wantURL {
			t.Errorf("service %q url = %q, want %q", svc.Name, svc.URL, wantURL)
		}

		if (svc.Name == "identity-service" || svc.Name == "bff-service") && removesCallerHeaders(svc.Plugins) {
			t.Errorf("service %q must not attach service-level request-transformer (priority 801 runs after caller-context 1001 and strips verified caller headers)", svc.Name)
		}

		svcRemovesHeaders := removesCallerHeaders(svc.Plugins)

		for _, route := range svc.Routes {
			if len(route.Methods) == 0 {
				t.Errorf("route %q (service %q) must declare explicit methods so Kong 3.8 match_weight cannot be shadowed by frontend-spa", route.Name, svc.Name)
			}
			if route.StripPath == nil || *route.StripPath {
				t.Errorf("route %q (service %q) must set strip_path: false", route.Name, svc.Name)
			}
			for _, p := range route.Paths {
				if p == "/api/v1" || p == "/api/v1/auth" || p == "/api/v1/storefront" || p == "/api/v1/admin" {
					t.Errorf("unsafe broad route path %q on route %q", p, route.Name)
				}
			}

			exp, known := expectedRoutes[route.Name]
			if !known {
				t.Errorf("unexpected route %q under service %q", route.Name, svc.Name)
				continue
			}
			seenRoutes[route.Name] = true
			if svc.Name != exp.service {
				t.Errorf("route %q attached to service %q, want %q", route.Name, svc.Name, exp.service)
			}
			if !slices.Equal(route.Paths, exp.paths) {
				t.Errorf("route %q paths = %v, want %v", route.Name, route.Paths, exp.paths)
			}
			if !slices.Equal(route.Methods, exp.methods) {
				t.Errorf("route %q methods = %v, want %v", route.Name, route.Methods, exp.methods)
			}

			if exp.protected {
				cc, ok := hasPlugin(route.Plugins, "caller-context")
				if !ok || cc.Config.JWKSURI != "http://identity-service:8080/.well-known/jwks.json" || cc.Config.Issuer != "http://identity-service:8080" || cc.Config.Audience != "ecommerce-api" {
					t.Errorf("protected route %q caller-context = %#v (present=%t)", route.Name, cc.Config, ok)
				}
				if removesCallerHeaders(route.Plugins) {
					t.Errorf("protected route %q must not attach request-transformer alongside caller-context", route.Name)
				}
			} else {
				if !svcRemovesHeaders && !removesCallerHeaders(route.Plugins) {
					t.Errorf("unauthenticated route %q (service %q) must strip X-User-Id and X-User-Roles via request-transformer", route.Name, svc.Name)
				}
			}
		}
	}

	for name := range expectedServices {
		if !seenServices[name] {
			t.Errorf("missing expected service %q", name)
		}
	}
	for name := range expectedRoutes {
		if !seenRoutes[name] {
			t.Errorf("missing expected route %q", name)
		}
	}
}
