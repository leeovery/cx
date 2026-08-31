package portaltest_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The calls and the env var the rule is expressed in.
const (
	isolateCall  = "portaltest.IsolateStateForTest"
	serverCall   = "tmuxtest.New"
	guardCall    = "portaltest.RegisterStateDirTeardownGuard"
	stateDirEnv  = "PORTAL_STATE_DIR"
	guardPIDCall = "portaltest.RegisterStateDirTeardownGuardWithPIDSource"
)

// fixtureCalls records what one test file does, as the rule below reads it.
type fixtureCalls struct {
	NamesStateDir  bool
	StartsServer   bool
	Isolates       bool
	RegistersGuard bool
}

// qualifies reports whether the file is subject to the rule: it drives a live
// tmux server against a state directory, whether it named that directory itself
// or took one from the isolation helper.
func (c fixtureCalls) qualifies() bool {
	return c.StartsServer && (c.NamesStateDir || c.Isolates)
}

func (c fixtureCalls) covered() bool {
	return c.Isolates && c.RegistersGuard
}

// TestTeardownGuardCoversEveryServerHostingFixture polices the rule that a test
// file pairing a state directory with a live tmux server must isolate that
// directory and register the teardown guard over it. Such a server can host
// writers into the state dir — a saver daemon's SIGHUP flush, a session-closed
// hook subprocess, a hydrate helper — and those outlive kill-server, so without
// the guard the TempDir RemoveAll races them and fails the test with "directory
// not empty" after its assertions have already passed. Without the isolation the
// fixture is writing somewhere it does not own at all.
//
// The state directory is a trigger in its own right, alongside the isolation
// call, because a trigger keyed only on the helper can fire on files that
// already opted in — and skipping that call is exactly the defect worth
// catching.
//
// The pairing is judged per file, which bounds what the rule can reach: a
// fixture that stages the state dir in a shared arrange and starts the server in
// the file that calls it is invisible here, and carries its registration in that
// arrange instead. The scoping buys legibility, and the value is mostly in
// catching a newly added file, which arranges both calls together.
func TestTeardownGuardCoversEveryServerHostingFixture(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	paths, err := sourceguardtest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate .go files: %v", err)
	}

	files := make(map[string]fixtureCalls)
	for _, path := range paths {
		if !strings.HasSuffix(path, "_test.go") {
			continue
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatalf("relativise %s: %v", path, relErr)
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", rel, readErr)
		}
		calls, scanErr := fixtureCallsIn(string(src))
		if scanErr != nil {
			t.Fatalf("scan %s: %v", rel, scanErr)
		}
		files[rel] = calls
	}

	if msg := coverageFailure(auditFixtureCoverage(files)); msg != "" {
		t.Fatal(msg)
	}
}

// fixtureCallsIn scans one file's source. Build tags are ignored on purpose: the
// integration-tagged files are the whole subject, and the unit lane must still
// police them.
func fixtureCallsIn(src string) (fixtureCalls, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, parser.SkipObjectResolution)
	if err != nil {
		return fixtureCalls{}, err
	}

	var calls fixtureCalls
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BasicLit:
			if node.Kind == token.STRING && strings.Contains(node.Value, stateDirEnv) {
				calls.NamesStateDir = true
			}
		case *ast.CallExpr:
			switch selectorName(node) {
			case serverCall:
				calls.StartsServer = true
			case isolateCall:
				calls.Isolates = true
			case guardCall, guardPIDCall:
				calls.RegistersGuard = true
			}
		}
		return true
	})
	return calls, nil
}

// selectorName renders a call as <package>.<func>, or "" when it is not one.
func selectorName(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return pkg.Name + "." + sel.Sel.Name
}

// auditFixtureCoverage returns how many files the rule applied to and the sorted
// names of those failing it.
func auditFixtureCoverage(files map[string]fixtureCalls) (scanned int, uncovered []string) {
	for name, calls := range files {
		if !calls.qualifies() {
			continue
		}
		scanned++
		if !calls.covered() {
			uncovered = append(uncovered, name)
		}
	}
	sort.Strings(uncovered)
	return scanned, uncovered
}

// coverageFailure renders the guard's failure, or "" when the tree passes.
func coverageFailure(scanned int, uncovered []string) string {
	if scanned == 0 {
		return fmt.Sprintf("no file pairs %s with either %s or %s; the guard has stopped looking rather than passed",
			serverCall, stateDirEnv, isolateCall)
	}
	if len(uncovered) == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d state-dir fixtures start a tmux server without calling both %s and %s:\n  %s\n"+
		"  a server can host writers into that state dir past kill-server, so the TempDir\n"+
		"  RemoveAll races them; isolate the dir and register the guard, both before %s",
		len(uncovered), scanned, isolateCall, guardCall, strings.Join(uncovered, "\n  "), serverCall)
}
