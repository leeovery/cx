package portaltest_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The three calls the rule is expressed in, as <package>.<func> selectors.
const (
	isolateCall = "portaltest.IsolateStateForTest"
	serverCall  = "tmuxtest.New"
	guardCall   = "portaltest.RegisterStateDirTeardownGuard"
)

// TestTeardownGuardCoversEveryServerHostingFixture polices the rule that a test
// file pairing an isolated state dir with a live tmux server must also register
// the teardown guard. Such a server can host writers into that state dir — a
// saver daemon's SIGHUP flush, a session-closed hook subprocess, a hydrate
// helper — and those outlive kill-server, so without the guard the TempDir
// RemoveAll races them and fails the test with "directory not empty" after its
// assertions have already passed.
//
// The pairing is judged per file, which bounds what the rule can reach: a
// fixture that isolates the state dir in a shared arrange and starts the
// server in the file that calls it is invisible here, and carries its
// registration in that arrange instead. The scoping buys legibility, and the
// value is mostly in catching a newly added file, which arranges both calls
// together.
func TestTeardownGuardCoversEveryServerHostingFixture(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	paths, err := sourceguardtest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate .go files: %v", err)
	}

	var scanned int
	var missing []string
	for _, path := range paths {
		if !strings.HasSuffix(path, "_test.go") {
			continue
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatalf("relativise %s: %v", path, relErr)
		}

		// Parsing ignores build tags on purpose: the integration-tagged files
		// are the whole subject, and the unit lane must still police them.
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", rel, parseErr)
		}

		calls := selectorCalls(file)
		if !calls[isolateCall] || !calls[serverCall] {
			continue
		}
		scanned++
		if !calls[guardCall] {
			missing = append(missing, rel)
		}
	}

	if scanned == 0 {
		t.Fatalf("no file pairs %s with %s; the guard has stopped looking rather than passed",
			isolateCall, serverCall)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("%d of %d state-isolated fixtures start a tmux server without calling %s:\n  %s\n"+
			"  a server can host writers into the isolated state dir past kill-server, so the\n"+
			"  TempDir RemoveAll races them; register the guard after %s and before %s",
			len(missing), scanned, guardCall, strings.Join(missing, "\n  "), isolateCall, serverCall)
	}
}

// selectorCalls reports which <package>.<func> calls the file makes.
func selectorCalls(file *ast.File) map[string]bool {
	found := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		found[pkg.Name+"."+sel.Sel.Name] = true
		return true
	})
	return found
}
