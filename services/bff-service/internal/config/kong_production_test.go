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
		JWKSURI string   `yaml:"jwks_uri"`
		Issuer  string   `yaml:"issuer"`
		Audience string  `yaml:"audience"`
		Origins []string `yaml:"origins"`
		Remove  struct {
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

func TestProductionKongLocksDownPublicRoutesAndServesSPA(t *testing.T) {
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

	expectedAuthRoutes := map[string]struct {
		url    string
		method string
	}{
		"~/api/v1/auth/register$":              {url: "http://identity-service:8080/register", method: "POST"},
		"~/api/v1/auth/login$":                 {url: "http://identity-service:8080/login", method: "POST"},
		"~/api/v1/auth/refresh$":               {url: "http://identity-service:8080/refresh", method: "POST"},
		"~/api/v1/auth/logout$":                {url: "http://identity-service:8080/logout", method: "POST"},
		`~/api/v1/auth/\.well-known/jwks\.json$`: {url: "http://identity-service:8080/.well-known/jwks.json", method: "GET"},
	}
	seenAuthRoutes := make(map[string]bool)

	expectedProtectedRoutes := map[string]struct {
		url     string
		path    string
		methods []string
	}{
		"identity-users-protected":       {url: "http://identity-service:8080/users", path: "/api/v1/users", methods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
		"identity-locations-protected":   {url: "http://identity-service:8080/locations", path: "/api/v1/locations", methods: []string{"GET"}},
		"identity-admin-users-protected": {url: "http://identity-service:8080/admin/users", path: "/api/v1/admin/users", methods: []string{"GET", "PATCH", "PUT", "DELETE"}},
		"identity-admin-rbac-protected":  {url: "http://identity-service:8080/admin/rbac", path: "/api/v1/admin/rbac", methods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
		"cart-api":                       {url: "http://bff-service:8080", path: "/api/v1/cart", methods: []string{"GET", "DELETE"}},
		"cart-items-api":                 {url: "http://bff-service:8080/items", path: "/api/v1/cart/items", methods: []string{"POST", "PATCH", "DELETE"}},
	}
	seenProtectedRoutes := make(map[string]bool)

	var storefrontFound, frontendFound bool

	for _, svc := range doc.Services {
		svcRemovesHeaders := removesCallerHeaders(svc.Plugins)
		_, svcHasCallerCtx := hasPlugin(svc.Plugins, "caller-context")

		for _, route := range svc.Routes {
			if len(route.Methods) == 0 {
				t.Errorf("route %q (service %q) must declare explicit methods so Kong 3.8 match_weight cannot be shadowed by frontend-spa", route.Name, svc.Name)
			}

			for _, p := range route.Paths {
				if p == "/api/v1" || p == "/api/v1/auth" || p == "/api/v1/storefront" || p == "/api/v1/admin" || strings.HasPrefix(p, "/api/v1/admin/products") {
					t.Errorf("unsafe broad or unauthenticated admin route path %q on route %q", p, route.Name)
				}
			}

			_, routeHasCallerCtx := hasPlugin(route.Plugins, "caller-context")
			routeRemovesHeaders := removesCallerHeaders(route.Plugins)
			if !svcHasCallerCtx && !routeHasCallerCtx && !svcRemovesHeaders && !routeRemovesHeaders {
				t.Errorf("unauthenticated route %q (service %q) must strip X-User-Id and X-User-Roles via request-transformer", route.Name, svc.Name)
			}

			if len(route.Paths) == 1 {
				if exp, isAuth := expectedAuthRoutes[route.Paths[0]]; isAuth {
					seenAuthRoutes[route.Paths[0]] = true
					if svc.URL != exp.url {
						t.Errorf("auth route %q upstream = %q, want %q", route.Paths[0], svc.URL, exp.url)
					}
					if !slices.Equal(route.Methods, []string{exp.method}) {
						t.Errorf("auth route %q methods = %v, want [%s]", route.Paths[0], route.Methods, exp.method)
					}
					if route.StripPath == nil || !*route.StripPath {
						t.Errorf("auth route %q must set strip_path: true", route.Paths[0])
					}
				}
			}

			if exp, isProtected := expectedProtectedRoutes[route.Name]; isProtected {
				seenProtectedRoutes[route.Name] = true
				if svc.URL != exp.url {
					t.Errorf("protected route %q upstream = %q, want %q", route.Name, svc.URL, exp.url)
				}
				if !slices.Equal(route.Paths, []string{exp.path}) {
					t.Errorf("protected route %q paths = %v, want [%s]", route.Name, route.Paths, exp.path)
				}
				if !slices.Equal(route.Methods, exp.methods) {
					t.Errorf("protected route %q methods = %v, want %v", route.Name, route.Methods, exp.methods)
				}
				if route.StripPath == nil || !*route.StripPath {
					t.Errorf("protected route %q must set strip_path: true", route.Name)
				}
				cc, ok := hasPlugin(route.Plugins, "caller-context")
				if !ok || cc.Config.JWKSURI != "http://identity-service:8080/.well-known/jwks.json" || cc.Config.Issuer != "http://identity-service:8080" || cc.Config.Audience != "ecommerce-api" {
					t.Errorf("protected route %q caller-context = %#v (present=%t)", route.Name, cc.Config, ok)
				}
			}

			if route.Name == "storefront-api" {
				storefrontFound = true
				if svc.URL != "http://bff-service:8080/products" {
					t.Errorf("storefront-api upstream = %q, want http://bff-service:8080/products", svc.URL)
				}
				if !slices.Equal(route.Paths, []string{"/api/v1/storefront/products"}) {
					t.Errorf("storefront-api paths = %v, want [/api/v1/storefront/products]", route.Paths)
				}
				if !slices.Equal(route.Methods, []string{"GET"}) {
					t.Errorf("storefront-api methods = %v, want [GET]", route.Methods)
				}
				if route.StripPath == nil || !*route.StripPath {
					t.Error("storefront-api must set strip_path: true")
				}
			}

			if route.Name == "frontend-spa" {
				frontendFound = true
				if svc.URL != "http://ecommerce-fe:8080" {
					t.Errorf("frontend-spa upstream = %q, want http://ecommerce-fe:8080", svc.URL)
				}
				if !slices.Equal(route.Paths, []string{"/"}) {
					t.Errorf("frontend-spa paths = %v, want [/]", route.Paths)
				}
				if !slices.Equal(route.Methods, []string{"GET", "HEAD"}) {
					t.Errorf("frontend-spa methods = %v, want [GET HEAD]", route.Methods)
				}
				if route.StripPath == nil || *route.StripPath {
					t.Error("frontend-spa must set strip_path: false")
				}
			}
		}
	}

	for path := range expectedAuthRoutes {
		if !seenAuthRoutes[path] {
			t.Errorf("missing expected public auth route %q", path)
		}
	}
	for name := range expectedProtectedRoutes {
		if !seenProtectedRoutes[name] {
			t.Errorf("missing expected protected route %q", name)
		}
	}
	if !storefrontFound {
		t.Error("missing hardened storefront-api route")
	}
	if !frontendFound {
		t.Error("missing frontend-spa route for ecommerce-fe")
	}
}
