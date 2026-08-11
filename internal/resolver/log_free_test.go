package resolver

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The guard is deliberately precise: it forbids the emission surface — a
// log/slog import or a component binding — not every reference to internal/log,
// which the resolver legitimately uses for a non-emitting exec helper.
func TestResolver_DoesNotEmitLogs(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}

		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, imp := range af.Imports {
			if strings.Trim(imp.Path.Value, `"`) == "log/slog" {
				t.Errorf("%s imports \"log/slog\" — internal/resolver must not emit logs; the resolve decision line is emitted only in cmd/open.go", f)
			}
		}

		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if strings.Contains(string(src), "log.For(") {
			t.Errorf("%s binds a log component via log.For — internal/resolver must not emit logs; the resolve decision line is emitted only in cmd/open.go", f)
		}
	}
}
