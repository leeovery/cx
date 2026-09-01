package restoretest_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The type no test may compose itself, and the constructors that compose it
// with Exe pinned.
const (
	orchestratorPkg  = "restore"
	orchestratorType = "Orchestrator"

	stagedConstructor = "restoretest.NewRestoreOrchestrator"
	fakeConstructor   = "restoretest.NewFakeExeOrchestrator"
)

// TestNoTestComposesABareRestoreOrchestrator is the standing guard over the one
// field whose omission is silent. restore.Orchestrator.Exe is optional and falls
// back to os.Executable(); under `go test` that resolves to the test binary, so a
// pane armed by an orchestrator with no Exe respawns into the suite itself, which
// stops flag parsing at the leading `state` positional, re-runs its own tests
// inside the tmux pane and exits 0. The symptom is a session that quietly
// disappeared — no error, no failure, nothing in the log.
//
// The rule is therefore not "set Exe" but "never compose the struct": a rule
// about a field that may legitimately be absent cannot be checked, while a rule
// about the literal can. Both constructors pin Exe, so routing through either
// makes the omission unrepresentable.
func TestNoTestComposesABareRestoreOrchestrator(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	scanned, findings, err := scanTestOrchestratorLiterals(root)
	if err != nil {
		t.Fatalf("scan for bare %s.%s literals: %v", orchestratorPkg, orchestratorType, err)
	}
	if len(findings) == 0 {
		return
	}
	t.Fatalf("%d bare %s.%s literal(s) in test files (of %d scanned):\n  %s\n"+
		"  Exe is opt-in on the struct and a forgotten one is silent: the pane respawns into\n"+
		"  the test binary and the session vanishes with no error. Build it through %s\n"+
		"  (a staged binary) or %s (no live pane to arm).",
		len(findings), orchestratorPkg, orchestratorType, scanned,
		strings.Join(findings, "\n  "), stagedConstructor, fakeConstructor)
}

func TestOrchestratorLiteralGuard_FlagsATestComposingTheStructItself(t *testing.T) {
	t.Run("it fails a test file composing a bare restore.Orchestrator literal", func(t *testing.T) {
		root := writeGuardFixture(t, "fixture_test.go", `package fixture

import "github.com/leeovery/portal/internal/restore"

func drive() {
	o := &restore.Orchestrator{StateDir: "/tmp"}
	_ = o
	_ = restore.Orchestrator{}
}
`)

		scanned, findings, err := scanTestOrchestratorLiterals(root)
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if scanned != 1 {
			t.Fatalf("scanned %d files, want 1", scanned)
		}
		if len(findings) != 2 {
			t.Fatalf("scan found %d literals, want 2: %v", len(findings), findings)
		}
	})

	t.Run("it passes a test file routing through the constructors", func(t *testing.T) {
		root := writeGuardFixture(t, "fixture_test.go", `package fixture

import "github.com/leeovery/portal/internal/restoretest"

func drive(t *testingT, client *client) {
	o := restoretest.NewRestoreOrchestrator(t, client, "/state", "/bin")
	p := restoretest.NewFakeExeOrchestrator(t, client, "/state", nil)
	_, _ = o, p
}
`)

		_, findings, err := scanTestOrchestratorLiterals(root)
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("scan flagged %v, want nothing", findings)
		}
	})

	// The rule is scoped to tests: restoretest's own constructors compose the
	// struct, and production wiring composes it too.
	t.Run("it ignores a production file composing the struct", func(t *testing.T) {
		root := writeGuardFixture(t, "production.go", `package fixture

import "github.com/leeovery/portal/internal/restore"

var o = &restore.Orchestrator{}
`)
		writeGuardFile(t, root, "keeps_scanning_test.go", "package fixture\n")

		scanned, findings, err := scanTestOrchestratorLiterals(root)
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if scanned != 1 || len(findings) != 0 {
			t.Errorf("scan of a production literal = %d scanned / %v, want 1 scanned / nothing", scanned, findings)
		}
	})
}

// A guard that stopped finding test sources would otherwise report a clean tree
// forever.
func TestOrchestratorLiteralGuard_ErrorsWhenItEnumeratesNoTestFiles(t *testing.T) {
	t.Run("it fatals when the orchestrator guard scans no files", func(t *testing.T) {
		if _, _, err := scanTestOrchestratorLiterals(t.TempDir()); err == nil {
			t.Fatal("scan of a directory holding no sources succeeded, want an error")
		}

		root := writeGuardFixture(t, "production.go", "package fixture\n")
		if _, _, err := scanTestOrchestratorLiterals(root); err == nil {
			t.Fatal("scan of a tree holding no _test.go succeeded, want an error")
		}
	})
}

// scanTestOrchestratorLiterals reports every composite literal of the
// orchestrator type in a _test.go under root, as "<file>:<line>". It reads the
// AST rather than the text, so a mention of the type inside a string — this
// guard's own fixtures — is not a finding.
func scanTestOrchestratorLiterals(root string) (scanned int, findings []string, err error) {
	paths, err := sourceguardtest.GoSourceFiles(root)
	if err != nil {
		return 0, nil, err
	}

	fset := token.NewFileSet()
	for _, path := range paths {
		if !strings.HasSuffix(path, "_test.go") {
			continue
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return 0, nil, relErr
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return 0, nil, fmt.Errorf("read %s: %w", rel, readErr)
		}
		// Build tags are ignored on purpose: the integration-tagged files are
		// most of the subject, and the unit lane must still police them.
		file, parseErr := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
		if parseErr != nil {
			return 0, nil, fmt.Errorf("parse %s: %w", rel, parseErr)
		}
		scanned++
		findings = append(findings, orchestratorLiteralsIn(fset, file)...)
	}
	if scanned == 0 {
		return 0, nil, errors.New("no _test.go was enumerated, so the guard would pass by having stopped looking")
	}
	return scanned, findings, nil
}

func orchestratorLiteralsIn(fset *token.FileSet, file *ast.File) []string {
	var findings []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, isLit := n.(*ast.CompositeLit)
		if !isLit || !isOrchestratorType(lit.Type) {
			return true
		}
		findings = append(findings, fset.Position(lit.Pos()).String())
		return true
	})
	return findings
}

func isOrchestratorType(expr ast.Expr) bool {
	sel, isSel := expr.(*ast.SelectorExpr)
	if !isSel || sel.Sel.Name != orchestratorType {
		return false
	}
	pkg, isIdent := sel.X.(*ast.Ident)
	return isIdent && pkg.Name == orchestratorPkg
}

func writeGuardFixture(t *testing.T, name, src string) string {
	t.Helper()
	root := t.TempDir()
	writeGuardFile(t, root, name, src)
	return root
}

func writeGuardFile(t *testing.T, root, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}
