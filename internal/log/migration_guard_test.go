package log_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

var forbiddenLegacySymbols = []string{
	"state.Component",
	"state.OpenLogger",
	"state.NopLogger",
	"openNoRotateLogger",
}

var excludedFromGuard = map[string]bool{}

func TestNoLegacyLoggerInProductionSource(t *testing.T) {
	root, sources := sourceguardtest.RepoSources(t, sourceguardtest.NonTestSources)
	for _, source := range sources {
		if excludedFromGuard[source.Path] {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(root, source.Path))
		if readErr != nil {
			t.Fatalf("read %s: %v", source.Path, readErr)
		}
		content := string(data)
		for _, sym := range forbiddenLegacySymbols {
			if strings.Contains(content, sym) {
				t.Errorf("production source %s references forbidden legacy-logger symbol %q", source.Path, sym)
			}
		}
	}
}
