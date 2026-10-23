package boundaries

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var excludedDirs = map[string]bool{
	"target":       true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	".git":         true,
	"bin":          true,
	"obj":          true,
}

var (
	goImportPattern       = regexp.MustCompile(`ecommerce\.local/[A-Za-z0-9._-]+`)
	javaImportPattern     = regexp.MustCompile(`^\s*import\s+(static\s+)?com\.ecommerce\.([a-z0-9_]+)`)
	pomArtifactPattern    = regexp.MustCompile(`<artifactId>([a-z0-9-]+-service)</artifactId>`)
	kongCallPattern       = regexp.MustCompile(`(localhost|127\.0\.0\.1|kong|host\.docker\.internal):8000\b`)
	contractGoPattern     = regexp.MustCompile(`ecommerce\.local/[A-Za-z0-9._-]+-service`)
	frontendImportPattern = regexp.MustCompile(`(from\s+|import\s*\(\s*|require\s*\(\s*)['"][^'"]*(services/[a-z0-9-]+-service|contracts/(go|java|proto))[^'"]*['"]`)
)

// Check scans the repository rooted at root and writes human-readable output to out.
// It returns 0 when all boundary rules pass and 1 when violations or errors occur.
func Check(root string, out io.Writer) int {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(out, "failed to resolve root %q: %v\n", root, err)
		return 1
	}

	servicesDir := filepath.Join(absRoot, "services")
	entries, err := os.ReadDir(servicesDir)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(out, "failed to read services directory: %v\n", err)
		return 1
	}

	var services []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), "-service") {
			services = append(services, entry.Name())
		}
	}
	sort.Strings(services)

	databases := make([]string, 0, len(services))
	for _, svc := range services {
		databases = append(databases, strings.TrimSuffix(svc, "-service")+"_db")
	}

	var violations []string
	addMatches := func(files []string, pattern *regexp.Regexp, isViolation func(submatches []string) bool, rule string) {
		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				continue
			}
			scanner := bufio.NewScanner(f)
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 4*1024*1024)
			lineNo := 0
			for scanner.Scan() {
				lineNo++
				line := scanner.Text()
				matches := pattern.FindAllStringSubmatch(line, -1)
				for _, m := range matches {
					if isViolation(m) {
						rel, err := filepath.Rel(absRoot, file)
						if err != nil {
							rel = file
						}
						rel = filepath.ToSlash(rel)
						violations = append(violations, fmt.Sprintf("%s:%d: %s: %s", rel, lineNo, rule, m[0]))
					}
				}
			}
			_ = f.Close()
		}
	}

	for _, service := range services {
		serviceDir := filepath.Join(servicesDir, service)
		ownName := strings.TrimSuffix(service, "-service")
		ownDB := ownName + "_db"

		var foreignDBs []string
		for _, db := range databases {
			if db != ownDB {
				foreignDBs = append(foreignDBs, regexp.QuoteMeta(db))
			}
		}

		goFiles := getSourceFiles(serviceDir, []string{".go", ".mod"})
		addMatches(goFiles, goImportPattern, func(m []string) bool {
			return m[0] != "ecommerce.local/"+service && m[0] != "ecommerce.local/contracts"
		}, "cross-service Go import")

		javaFiles := getSourceFiles(serviceDir, []string{".java"})
		addMatches(javaFiles, javaImportPattern, func(m []string) bool {
			pkg := m[2]
			return pkg != ownName && pkg != "contracts"
		}, "cross-service Java import")

		var pomFiles []string
		for _, f := range getSourceFiles(serviceDir, []string{".xml"}) {
			if filepath.Base(f) == "pom.xml" {
				pomFiles = append(pomFiles, f)
			}
		}
		addMatches(pomFiles, pomArtifactPattern, func(m []string) bool {
			return m[1] != service
		}, "cross-service Maven dependency")

		if len(foreignDBs) > 0 {
			configFiles := getSourceFiles(serviceDir, []string{
				".go", ".java", ".sql", ".yml", ".yaml", ".properties", ".json", ".env", ".ps1", ".sh",
			})
			dbPattern := regexp.MustCompile(`\b(` + strings.Join(foreignDBs, "|") + `)\b`)
			addMatches(configFiles, dbPattern, func(_ []string) bool {
				return true
			}, "foreign service database reference")
		}

		runtimeSourceFiles := getSourceFiles(serviceDir, []string{
			".go", ".java", ".yml", ".yaml", ".properties", ".env",
		})
		addMatches(runtimeSourceFiles, kongCallPattern, func(_ []string) bool {
			return true
		}, "internal service call through Kong")
	}

	contractsGo := getSourceFiles(filepath.Join(absRoot, "contracts", "go"), []string{".go", ".mod"})
	addMatches(contractsGo, contractGoPattern, func(_ []string) bool {
		return true
	}, "contract module imports service code")

	contractsJava := getSourceFiles(filepath.Join(absRoot, "contracts", "java", "src"), []string{".java"})
	addMatches(contractsJava, javaImportPattern, func(m []string) bool {
		return m[2] != "contracts"
	}, "contract module imports service code")

	frontendFiles := getSourceFiles(filepath.Join(absRoot, "ecommerce-fe", "src"), []string{".ts", ".tsx", ".js", ".jsx"})
	addMatches(frontendFiles, frontendImportPattern, func(_ []string) bool {
		return true
	}, "frontend imports backend source")

	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintln(out, v)
		}
		fmt.Fprintf(out, "Boundary check failed: %d violation(s). See docs/steering/service-boundaries.md.\n", len(violations))
		return 1
	}

	fmt.Fprintf(out, "Boundary check passed for %d services, contracts, and ecommerce-fe.\n", len(services))
	return 0
}

func getSourceFiles(dir string, extensions []string) []string {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}
	extSet := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		extSet[strings.ToLower(ext)] = true
	}

	var files []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && excludedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if extSet[ext] {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}
