package resolver

import (
	"os"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The guard is deliberately precise: it forbids the emission surface — a
// log/slog import or a component binding — not every reference to internal/log,
// which the resolver legitimately uses for a non-emitting exec helper.
func TestResolver_DoesNotEmitLogs(t *testing.T) {
	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		for _, imp := range source.File.Imports {
			if strings.Trim(imp.Path.Value, `"`) == "log/slog" {
				t.Errorf("%s imports \"log/slog\" — internal/resolver must not emit logs; the resolve decision line is emitted only in cmd/open.go", source.Path)
			}
		}

		src, err := os.ReadFile(source.Path)
		if err != nil {
			t.Fatalf("read %s: %v", source.Path, err)
		}
		if strings.Contains(string(src), "log.For(") {
			t.Errorf("%s binds a log component via log.For — internal/resolver must not emit logs; the resolve decision line is emitted only in cmd/open.go", source.Path)
		}
	}
}
