package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Root returns the repository root directory by walking upwards from the
// current working directory (or this source file's location as fallback) until
// go.work or buf.yaml is found.
func Root() (string, error) {
	if wd, err := os.Getwd(); err == nil {
		if r, ok := findRootFrom(wd); ok {
			return r, nil
		}
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		if r, found := findRootFrom(filepath.Dir(file)); found {
			return r, nil
		}
	}
	return "", fmt.Errorf("could not locate repository root")
}

func findRootFrom(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if fileExists(filepath.Join(dir, "go.work")) || fileExists(filepath.Join(dir, "buf.yaml")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
