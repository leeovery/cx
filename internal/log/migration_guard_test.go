package log_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

var forbiddenLegacySymbols = []string{
	"state.Component",
	"state.OpenLogger",
	"state.NopLogger",
	"openNoRotateLogger",
}

var excludedFromGuard = map[string]bool{}

func TestNoLegacyLoggerInProductionSource(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if excludedFromGuard[rel] {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content := string(data)
		for _, sym := range forbiddenLegacySymbols {
			if strings.Contains(content, sym) {
				t.Errorf("production source %s references forbidden legacy-logger symbol %q", rel, sym)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk project tree: %v", walkErr)
	}
}
