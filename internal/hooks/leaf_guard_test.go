package hooks_test

import (
	"go/parser"
	"go/token"
	"maps"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

const (
	modulePrefix = "github.com/leeovery/portal/"
	hooksPkgPath = "internal/hooks"
)

// The store is a JSON persistence leaf on the hydrate hot path, and the edge it
// would create is one-way and permanent: anything it imports can never import
// it back. internal/state — which owns the pane-token option and the capture
// column carrying the token — is the package most likely to want this store,
// and an import of the session or tmux tree here locks it out for good.
var hooksMayImport = map[string]bool{
	"internal/fileutil": true,
	"internal/log":      true,
	"internal/nanoid":   true,
	"internal/storelog": true,
}

func TestHooksPackage_ImportsOnlyLeaves(t *testing.T) {
	t.Run("its own sources import no other internal package", func(t *testing.T) {
		paths, err := sourceguardtest.PackageGoFiles(".", false)
		if err != nil {
			t.Fatalf("enumerate package sources: %v", err)
		}

		for _, path := range paths {
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly|parser.SkipObjectResolution)
			if parseErr != nil {
				t.Fatalf("parse %s: %v", path, parseErr)
			}
			for _, spec := range file.Imports {
				imported, uerr := strconv.Unquote(spec.Path.Value)
				if uerr != nil {
					t.Fatalf("%s: unquote import path %s: %v", path, spec.Path.Value, uerr)
				}
				internalPath, isInternal := strings.CutPrefix(imported, modulePrefix)
				if isInternal && !hooksMayImport[internalPath] {
					t.Errorf("%s imports %s — internal/hooks may import only %v", path, imported, sortedKeys(hooksMayImport))
				}
			}
		}
	})

	t.Run("it drags in no session, tmux or state tree transitively", func(t *testing.T) {
		out, err := exec.Command("go", "list", "-deps", modulePrefix+hooksPkgPath).CombinedOutput()
		if err != nil {
			t.Fatalf("go list -deps internal/hooks: %v\n%s", err, out)
		}

		for dep := range strings.FieldsSeq(string(out)) {
			internalPath, isInternal := strings.CutPrefix(dep, modulePrefix)
			if internalPath == hooksPkgPath {
				continue
			}
			if isInternal && !hooksMayImport[internalPath] {
				t.Errorf("internal/hooks transitively depends on %s — it must reach no further than %v", dep, sortedKeys(hooksMayImport))
			}
		}
	})
}

func sortedKeys(set map[string]bool) []string {
	return slices.Sorted(maps.Keys(set))
}
