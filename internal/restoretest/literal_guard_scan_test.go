package restoretest_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// restorePkg is the package qualifier a guarded type is written under at a call
// site.
const restorePkg = "restore"

// scanGuardTestFiles parses every _test.go under root that include accepts and
// returns what collect finds in them, along with how many files were scanned —
// a caller treats a zero count as its own failure, since a guard that has
// stopped finding sources reports a clean tree forever.
//
// Build tags are not honoured by the walk itself: a guard decides through
// include which lane's files it polices, so the unit lane can police both.
func scanGuardTestFiles(
	root string,
	include func(*ast.File) bool,
	collect func(*token.FileSet, *ast.File) []string,
) (scanned int, findings []string, err error) {
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
		file, parseErr := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution|parser.ParseComments)
		if parseErr != nil {
			return 0, nil, fmt.Errorf("parse %s: %w", rel, parseErr)
		}
		if !include(file) {
			continue
		}
		scanned++
		findings = append(findings, collect(fset, file)...)
	}
	return scanned, findings, nil
}

// isRestorePkgType reports whether expr names the given type of the restore
// package, as written at a call site: restore.<typeName>.
func isRestorePkgType(expr ast.Expr, typeName string) bool {
	sel, isSel := expr.(*ast.SelectorExpr)
	if !isSel || sel.Sel.Name != typeName {
		return false
	}
	pkg, isIdent := sel.X.(*ast.Ident)
	return isIdent && pkg.Name == restorePkg
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
