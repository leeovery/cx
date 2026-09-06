package hooksweep

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The caller-side coverage guard ranges over Reasons, so it is only as
// complete as that slice. This one reads the declarations themselves, keyed on the
// Reason type rather than on the names chosen for them: the type is what
// makes a const a reason, so a const the compiler accepts as one and the slice
// omits is exactly what stays invisible to every check that ranges over the
// set. Membership is what the type cannot hold, and it is what this guard still
// adds over it.
func TestReasonsEnumeratesEveryDeclaredConst(t *testing.T) {
	sources := sourceguardtest.ParsePackageSources(t, ".", false)
	declared := declaredReasonConsts(sources)
	enumerated := enumeratedReasons(t, sources)

	if len(declared) == 0 {
		t.Fatalf("no const declared with the %s type in this package; the guard is scanning nothing", reasonTypeName)
	}

	t.Run("it enumerates every const declared with the reason type", func(t *testing.T) {
		if missing := unenumerated(declared, enumerated); len(missing) > 0 {
			t.Errorf("Reasons omits the declared reason(s) %v; every declared reason must be enumerable", missing)
		}
		if extra := unenumerated(enumerated, declared); len(extra) > 0 {
			t.Errorf("Reasons enumerates %v, which no const declares", extra)
		}
		if len(enumerated) != len(Reasons) {
			t.Errorf("Reasons literal names %d reasons but the value holds %d", len(enumerated), len(Reasons))
		}
	})

	// The type admits a const named anything, so the rule must catch one the
	// slice leaves out however it is named.
	t.Run("it fails a reason absent from the enumerable set", func(t *testing.T) {
		synthetic := parseSyntheticSource(t, `package hooksweep

const offConvention Reason = "off-convention"

var Reasons = []Reason{ReasonRestoring}
`)
		missing := unenumerated(declaredReasonConsts(synthetic), enumeratedReasons(t, synthetic))
		if !slices.Equal(missing, []string{"offConvention"}) {
			t.Errorf("unenumerated = %v, want exactly [offConvention]; the rule would not catch the omission", missing)
		}
	})
}

// reasonTypeName is the type whose consts are the reason vocabulary, and
// reasonNamePrefix the convention its members are named by.
const (
	reasonTypeName   = "Reason"
	reasonNamePrefix = "Reason"
)

// declaredReasonConsts returns the names of every const the package declares as
// a reason: one carrying the reason type, or one named by the convention. The
// type is the rule — it is what the compiler accepts as a reason and it reaches
// a const named anything — and the name is the residual it cannot see: an
// untyped const in the block is a plain string to the compiler, and passing it
// as a reason still compiles.
func declaredReasonConsts(sources []sourceguardtest.ParsedSource) []string {
	var declared []string
	forEachValueSpec(sources, func(gen *ast.GenDecl, _ *token.FileSet, value *ast.ValueSpec) {
		if gen.Tok != token.CONST {
			return
		}
		typed := isReasonType(value.Type)
		for _, name := range value.Names {
			if typed || strings.HasPrefix(name.Name, reasonNamePrefix) {
				declared = append(declared, name.Name)
			}
		}
	})
	slices.Sort(declared)
	return declared
}

// enumeratedReasons returns the identifiers the Reasons slice literal
// lists. A package with no such slice is fatal: the rule would otherwise pass
// having compared nothing.
func enumeratedReasons(t *testing.T, sources []sourceguardtest.ParsedSource) []string {
	t.Helper()

	var enumerated []string
	found := false
	forEachValueSpec(sources, func(gen *ast.GenDecl, fset *token.FileSet, value *ast.ValueSpec) {
		if gen.Tok != token.VAR {
			return
		}
		for i, name := range value.Names {
			if name.Name != "Reasons" {
				continue
			}
			found = true
			enumerated = append(enumerated, sliceElementNames(t, fset, value.Values[i])...)
		}
	})
	if !found {
		t.Fatal("no Reasons slice found; the guard is scanning nothing")
	}
	slices.Sort(enumerated)
	return enumerated
}

// unenumerated reports the names in first that second does not hold.
func unenumerated(first, second []string) []string {
	var missing []string
	for _, name := range first {
		if !slices.Contains(second, name) {
			missing = append(missing, name)
		}
	}
	return missing
}

func isReasonType(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == reasonTypeName
}

// forEachValueSpec visits every const and var spec the sources declare.
func forEachValueSpec(sources []sourceguardtest.ParsedSource, visit func(*ast.GenDecl, *token.FileSet, *ast.ValueSpec)) {
	for _, source := range sources {
		for _, decl := range source.File.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				if value, ok := spec.(*ast.ValueSpec); ok {
					visit(gen, source.Fset, value)
				}
			}
		}
	}
}

// parseSyntheticSource parses source authored by the test, so a rule's own
// failure mode is exercised against a declaration the package does not hold.
func parseSyntheticSource(t *testing.T, source string) []sourceguardtest.ParsedSource {
	t.Helper()

	path := filepath.Join(t.TempDir(), "synthetic.go")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write synthetic source: %v", err)
	}
	return sourceguardtest.ParseSources(t, []string{path})
}

// sliceElementNames returns the identifiers a composite slice literal lists, so
// the guard compares names with the declarations rather than re-deriving values.
func sliceElementNames(t *testing.T, fset *token.FileSet, expr ast.Expr) []string {
	t.Helper()
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("%s: Reasons is not a composite literal; the guard cannot read its members", fset.Position(expr.Pos()))
	}
	names := make([]string, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		ident, ok := elt.(*ast.Ident)
		if !ok {
			t.Fatalf("%s: Reasons element is not an identifier; every member must name a declared const",
				fset.Position(elt.Pos()))
		}
		names = append(names, ident.Name)
	}
	return names
}
