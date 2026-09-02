package restoretest_test

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
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
// Findings are named relative to root, so a guard's complaint reads as a
// repository path rather than as one machine's checkout.
//
// Build tags are not honoured by the walk itself: a guard decides through
// include which lane's files it polices, so the unit lane can police both.
func scanGuardTestFiles(
	t harnesstest.TestingT,
	root string,
	include func(*ast.File) bool,
	collect func(*token.FileSet, *ast.File) []string,
) (scanned int, findings []string) {
	t.Helper()

	paths, err := sourceguardtest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate the .go files under %s: %v", root, err)
		return 0, nil
	}

	var testPaths []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			testPaths = append(testPaths, path)
		}
	}

	for _, source := range sourceguardtest.ParseSources(t, testPaths) {
		if !include(source.File) {
			continue
		}
		scanned++
		for _, finding := range collect(source.Fset, source.File) {
			findings = append(findings, strings.TrimPrefix(finding, root+string(filepath.Separator)))
		}
	}
	return scanned, findings
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
