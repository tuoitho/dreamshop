package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ecommerce.local/scripts/internal/repo"
)

type launcherFunc func(dir string, env []string, name string, args ...string) int

func main() {
	os.Exit(runWithDeps(
		os.Args[1:],
		os.Stdin,
		os.Stdout,
		os.Stderr,
		os.Environ(),
		defaultLauncher(os.Stdin, os.Stdout, os.Stderr),
	))
}

func defaultLauncher(in io.Reader, out, errOut io.Writer) launcherFunc {
	return func(dir string, env []string, name string, args ...string) int {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdin = in
		cmd.Stdout = out
		cmd.Stderr = errOut
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitErr.ExitCode()
			}
			fmt.Fprintf(errOut, "failed to run %s: %v\n", name, err)
			return 1
		}
		return 0
	}
}

func runWithDeps(args []string, in io.Reader, out, errOut io.Writer, baseEnv []string, launch launcherFunc) int {
	fs := flag.NewFlagSet("start-local", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var service, root string
	var bootstrapEmail, bootstrapFullName string
	var emailFlagSet, fullNameFlagSet bool

	fs.StringVar(&service, "service", "", "service to start locally")
	fs.StringVar(&service, "Service", "", "service to start locally")
	fs.StringVar(&root, "root", "", "repository root directory")
	fs.Func("bootstrap-super-admin-email", "bootstrap SUPER_ADMIN email", func(v string) error {
		bootstrapEmail = strings.TrimSpace(v)
		emailFlagSet = true
		return nil
	})
	fs.Func("BootstrapSuperAdminEmail", "bootstrap SUPER_ADMIN email", func(v string) error {
		bootstrapEmail = strings.TrimSpace(v)
		emailFlagSet = true
		return nil
	})
	fs.Func("bootstrap-super-admin-full-name", "bootstrap SUPER_ADMIN full name", func(v string) error {
		bootstrapFullName = strings.TrimSpace(v)
		fullNameFlagSet = true
		return nil
	})
	fs.Func("BootstrapSuperAdminFullName", "bootstrap SUPER_ADMIN full name", func(v string) error {
		bootstrapFullName = strings.TrimSpace(v)
		fullNameFlagSet = true
		return nil
	})

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if service == "" && fs.NArg() > 0 {
		service = fs.Arg(0)
	}
	if root == "" {
		r, err := repo.Root()
		if err != nil {
			fmt.Fprintf(errOut, "error locating repository root: %v\n", err)
			return 1
		}
		root = r
	}

	if baseEnv == nil {
		baseEnv = os.Environ()
	}
	env := parseEnv(baseEnv)

	switch normalizeService(service) {
	case "identity-service":
		setDefaultEnv(env, "IDENTITY_JWT_PRIVATE_KEY_PATH", filepath.Join(root, "deployments", "secrets", "identity", "private.pem"))
		setDefaultEnv(env, "IDENTITY_JWT_PUBLIC_KEY_PATH", filepath.Join(root, "deployments", "secrets", "identity", "public.pem"))
		setDefaultEnv(env, "SERVER_PORT", "8080")
		setDefaultEnv(env, "MANAGEMENT_SERVER_PORT", "8090")
		setDefaultEnv(env, "IDENTITY_JWT_ISSUER", "http://localhost:8080")
		setDefaultEnv(env, "IDENTITY_SESSION_COOKIE_SECURE", "false")
		setDefaultEnv(env, "IDENTITY_GRPC_HOST", "127.0.0.1")
		setDefaultEnv(env, "IDENTITY_GRPC_PORT", "50054")
		setDefaultEnv(env, "IDENTITY_GRPC_SHUTDOWN_GRACE", "5s")

		javaOpts := getEnv(env, "JDK_JAVA_OPTIONS")
		if !strings.Contains(javaOpts, "-Djava.net.preferIPv4Stack=true") {
			if javaOpts == "" {
				javaOpts = "-Djava.net.preferIPv4Stack=true"
			} else {
				javaOpts = javaOpts + " -Djava.net.preferIPv4Stack=true"
			}
			setEnv(env, "JDK_JAVA_OPTIONS", javaOpts)
		}

		if !emailFlagSet {
			bootstrapEmail = strings.TrimSpace(getEnv(env, "IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL"))
		}
		if !fullNameFlagSet {
			bootstrapFullName = strings.TrimSpace(getEnv(env, "IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME"))
		}
		bootstrapPassword := getEnv(env, "IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD")
		bootstrapRequested := bootstrapEmail != "" || bootstrapFullName != "" || strings.TrimSpace(bootstrapPassword) != ""

		if bootstrapRequested {
			if bootstrapEmail == "" || bootstrapFullName == "" {
				fmt.Fprintln(errOut, "Bootstrap SUPER_ADMIN email and full name must be supplied together.")
				return 1
			}
			if strings.TrimSpace(bootstrapPassword) == "" {
				fmt.Fprint(out, "Bootstrap SUPER_ADMIN password: ")
				reader := bufio.NewReader(in)
				line, _ := reader.ReadString('\n')
				bootstrapPassword = strings.TrimRight(line, "\r\n")
				if len(bootstrapPassword) < 12 || len(bootstrapPassword) > 128 {
					fmt.Fprintln(errOut, "Bootstrap SUPER_ADMIN password must be 12 through 128 characters.")
					return 1
				}
			}
			setEnv(env, "IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL", bootstrapEmail)
			setEnv(env, "IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD", bootstrapPassword)
			setEnv(env, "IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME", bootstrapFullName)
		}

		return launch(root, formatEnv(env), "mvn", "-f", "services/identity-service/pom.xml", "spring-boot:run")

	case "order-service":
		setDefaultEnv(env, "SERVER_PORT", "8083")
		setDefaultEnv(env, "MANAGEMENT_SERVER_PORT", "8093")
		return launch(root, formatEnv(env), "mvn", "-f", "services/order-service/pom.xml", "spring-boot:run")

	case "catalog-service":
		setDefaultEnv(env, "DATABASE_URL", "postgres://ecommerce:change-me-local-only@127.0.0.1:5432/catalog_db?sslmode=disable")
		setDefaultEnv(env, "HTTP_ADDR", ":8085")
		setDefaultEnv(env, "GRPC_ADDR", "127.0.0.1:50051")
		setDefaultEnv(env, "IDENTITY_GRPC_ADDR", "127.0.0.1:50054")
		setDefaultEnv(env, "IDENTITY_GRPC_TIMEOUT", "500ms")

		return launch(filepath.Join(root, "services", "catalog-service"), formatEnv(env), "go", "run", "./cmd/server")

	case "inventory-service":
		setDefaultEnv(env, "DATABASE_URL", "postgres://ecommerce:change-me-local-only@127.0.0.1:5432/inventory_db?sslmode=disable")
		setDefaultEnv(env, "HTTP_ADDR", ":8082")
		setDefaultEnv(env, "GRPC_ADDR", "127.0.0.1:50052")

		return launch(filepath.Join(root, "services", "inventory-service"), formatEnv(env), "go", "run", "./cmd/server")

	case "cart-service":
		setDefaultEnv(env, "DATABASE_URL", "postgres://ecommerce:change-me-local-only@127.0.0.1:5432/cart_db?sslmode=disable")
		setDefaultEnv(env, "HTTP_ADDR", ":8084")
		setDefaultEnv(env, "GRPC_ADDR", "127.0.0.1:50053")
		setDefaultEnv(env, "CATALOG_GRPC_ADDR", "127.0.0.1:50051")
		setDefaultEnv(env, "CATALOG_GRPC_TIMEOUT", "3s")
		setDefaultEnv(env, "INVENTORY_GRPC_ADDR", "127.0.0.1:50052")
		setDefaultEnv(env, "INVENTORY_GRPC_TIMEOUT", "3s")
		setDefaultEnv(env, "IDENTITY_GRPC_ADDR", "127.0.0.1:50054")
		setDefaultEnv(env, "IDENTITY_GRPC_TIMEOUT", "500ms")

		return launch(filepath.Join(root, "services", "cart-service"), formatEnv(env), "go", "run", "./cmd/server")

	case "bff-service":
		setDefaultEnv(env, "BFF_HTTP_ADDR", ":8081")
		setDefaultEnv(env, "BFF_CATALOG_GRPC_ADDR", "127.0.0.1:50051")
		setDefaultEnv(env, "BFF_CATALOG_GRPC_TIMEOUT", "3s")
		setDefaultEnv(env, "BFF_INVENTORY_GRPC_ADDR", "127.0.0.1:50052")
		setDefaultEnv(env, "BFF_INVENTORY_GRPC_TIMEOUT", "3s")
		setDefaultEnv(env, "BFF_CART_GRPC_ADDR", "127.0.0.1:50053")
		setDefaultEnv(env, "BFF_CART_GRPC_TIMEOUT", "3s")

		return launch(filepath.Join(root, "services", "bff-service"), formatEnv(env), "go", "run", "./cmd/server")

	case "ecommerce-fe":
		setDefaultEnv(env, "VITE_API_BASE_URL", "http://localhost:8000/api/v1")

		return launch(root, formatEnv(env), "npm", "--prefix", "ecommerce-fe", "run", "dev")

	default:
		fmt.Fprintf(errOut, "Unknown local service %q. Expected one of: identity-service, order-service, catalog-service, inventory-service, cart-service, bff-service, ecommerce-fe\n", service)
		return 2
	}
}

func normalizeService(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "identity", "identity-service":
		return "identity-service"
	case "order", "order-service":
		return "order-service"
	case "catalog", "catalog-service":
		return "catalog-service"
	case "inventory", "inventory-service":
		return "inventory-service"
	case "cart", "cart-service":
		return "cart-service"
	case "bff", "bff-service":
		return "bff-service"
	case "fe", "frontend", "ecommerce-fe":
		return "ecommerce-fe"
	default:
		return s
	}
}

type envStore struct {
	order []string
	vals  map[string]string
}

func parseEnv(items []string) *envStore {
	s := &envStore{vals: make(map[string]string, len(items))}
	for _, kv := range items {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			continue
		}
		if _, exists := s.vals[k]; !exists {
			s.order = append(s.order, k)
		}
		s.vals[k] = v
	}
	return s
}

func getEnv(s *envStore, key string) string {
	return s.vals[key]
}

func setEnv(s *envStore, key, val string) {
	if _, exists := s.vals[key]; !exists {
		s.order = append(s.order, key)
	}
	s.vals[key] = val
}

func setDefaultEnv(s *envStore, key, defVal string) {
	if strings.TrimSpace(s.vals[key]) == "" {
		setEnv(s, key, defVal)
	}
}

func formatEnv(s *envStore) []string {
	res := make([]string, 0, len(s.order))
	for _, k := range s.order {
		res = append(res, k+"="+s.vals[k])
	}
	return res
}
