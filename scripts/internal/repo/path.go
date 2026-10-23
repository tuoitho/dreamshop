package repo

import (
	"os"
	"path/filepath"
	"strings"
)

// EnsureGoBinInPath prepends $HOME/go/bin to PATH if the directory exists and is not already in PATH.
func EnsureGoBinInPath() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	goBin := filepath.Join(home, "go", "bin")
	info, err := os.Stat(goBin)
	if err != nil || !info.IsDir() {
		return
	}
	currentPath := os.Getenv("PATH")
	for _, entry := range filepath.SplitList(currentPath) {
		if strings.EqualFold(filepath.Clean(entry), filepath.Clean(goBin)) {
			return
		}
	}
	_ = os.Setenv("PATH", goBin+string(os.PathListSeparator)+currentPath)
}
