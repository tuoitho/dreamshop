package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
}

func readRepoFile(t *testing.T, root string, rel ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{root}, rel...)...)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(rel...), err)
	}
	return string(content)
}

func hasTrimmedLine(content, expected string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == expected {
			return true
		}
	}
	return false
}

func TestBackendDockerfilesAndRootDockerignore(t *testing.T) {
	root := repoRoot(t)

	dockerignore := readRepoFile(t, root, ".dockerignore")
	for _, requiredIgnore := range []string{
		".git",
		"**/node_modules",
		"**/target",
		"ecommerce-fe/dist",
		".env",
		".env.*",
		"deployments/secrets",
	} {
		if !hasTrimmedLine(dockerignore, requiredIgnore) {
			t.Errorf(".dockerignore missing line %q", requiredIgnore)
		}
	}

	gitignore := readRepoFile(t, root, ".gitignore")
	if !hasTrimmedLine(gitignore, ".env.production") {
		t.Error(".gitignore must ignore .env.production")
	}

	goServices := []struct {
		name  string
		ports []string
	}{
		{name: "bff-service", ports: []string{"8080"}},
		{name: "catalog-service", ports: []string{"8081", "50051"}},
		{name: "inventory-service", ports: []string{"8082", "50052"}},
		{name: "cart-service", ports: []string{"8084", "50053"}},
	}

	for _, svc := range goServices {
		dockerfile := readRepoFile(t, root, "services", svc.name, "Dockerfile")
		if !strings.Contains(dockerfile, "ENV GOWORK=off") {
			t.Errorf("%s Dockerfile must set ENV GOWORK=off", svc.name)
		}
		if strings.Contains(dockerfile, "COPY go.work") {
			t.Errorf("%s Dockerfile must not copy go.work", svc.name)
		}
		if strings.Contains(dockerfile, "COPY services ./services") {
			t.Errorf("%s Dockerfile must not copy all services", svc.name)
		}
		if !strings.Contains(dockerfile, "USER appuser") {
			t.Errorf("%s Dockerfile must run as non-root appuser", svc.name)
		}
		for _, port := range svc.ports {
			if !strings.Contains(dockerfile, port) {
				t.Errorf("%s Dockerfile must expose port %s", svc.name, port)
			}
		}
	}

	for _, svc := range []string{"identity-service", "order-service"} {
		dockerfile := readRepoFile(t, root, "services", svc, "Dockerfile")
		if strings.Contains(dockerfile, "COPY services ./services") {
			t.Errorf("%s Dockerfile must not copy all services", svc)
		}
		expectedSrc := "services/" + svc + "/src"
		if !strings.Contains(dockerfile, expectedSrc) {
			t.Errorf("%s Dockerfile must copy scoped source %s", svc, expectedSrc)
		}
		if !strings.Contains(dockerfile, "USER appuser") {
			t.Errorf("%s Dockerfile must run as non-root appuser", svc)
		}
	}

	orderAppYml := readRepoFile(t, root, "services", "order-service", "src", "main", "resources", "application.yml")
	if !strings.Contains(orderAppYml, "${SERVER_PORT:8083}") {
		t.Error("order-service application.yml must bind server.port via ${SERVER_PORT:8083}")
	}
}

func TestGoServicesBuildStandaloneWithGoWorkOff(t *testing.T) {
	root := repoRoot(t)
	for _, svc := range []string{"bff-service", "catalog-service", "inventory-service", "cart-service"} {
		cmd := exec.Command("go", "list", "./cmd/server")
		cmd.Dir = filepath.Join(root, "services", svc)
		cmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("%s failed standalone module resolution with GOWORK=off: %v\n%s", svc, err, string(out))
		}
	}
}

func TestFrontendDockerfileAndNginxConfig(t *testing.T) {
	root := repoRoot(t)

	feIgnore := readRepoFile(t, root, "ecommerce-fe", ".dockerignore")
	for _, entry := range []string{"node_modules", "dist", ".env", ".env.*", ".opencode", ".agents"} {
		if !hasTrimmedLine(feIgnore, entry) {
			t.Errorf("ecommerce-fe/.dockerignore missing %q", entry)
		}
	}

	feDockerfile := readRepoFile(t, root, "ecommerce-fe", "Dockerfile")
	for _, expected := range []string{
		"FROM node:22.12-alpine",
		"ARG VITE_API_BASE_URL=/api/v1",
		"ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}",
		"npm ci",
		"npm run build",
		"FROM nginxinc/nginx-unprivileged:1.27-alpine",
		"/etc/nginx/conf.d/default.conf",
		"EXPOSE 8080",
	} {
		if !strings.Contains(feDockerfile, expected) {
			t.Errorf("ecommerce-fe/Dockerfile missing %q", expected)
		}
	}

	nginxConf := readRepoFile(t, root, "ecommerce-fe", "nginx.conf")
	for _, expected := range []string{
		"listen 8080;",
		"location = /healthz",
		"return 200",
		"try_files $uri $uri/ /index.html;",
	} {
		if !strings.Contains(nginxConf, expected) {
			t.Errorf("ecommerce-fe/nginx.conf missing %q", expected)
		}
	}
}
