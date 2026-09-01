package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// missingPhrases reports the declared reasons a surface vocabulary renders
// nothing for. It is the whole rule the coverage guard applies, named so the
// guard's own failure mode is testable: a reason declared without a phrase.
func missingPhrases(reasons []string, m map[string]string) []string {
	var missing []string
	for _, reason := range reasons {
		if phrase, ok := m[reason]; !ok || phrase == "" {
			missing = append(missing, reason)
		}
	}
	return missing
}

// undeclaredKeys reports the vocabulary's keys that name no declared reason —
// the leftover a retired reason would strand.
func undeclaredKeys(m map[string]string, reasons []string) []string {
	var extra []string
	for key := range m {
		if !slices.Contains(reasons, key) {
			extra = append(extra, key)
		}
	}
	slices.Sort(extra)
	return extra
}

// Every stand-down reason reaches the user through two vocabularies, and an
// unmapped one falls through to its raw slug — internal words on the command
// whose purpose is explaining what happened. The fall-through stays as the
// runtime net; this guard is what makes the omission a test failure instead.
func TestStandDownPhraseCoverage(t *testing.T) {
	if len(skipReasons) == 0 {
		t.Fatal("skipReasons is empty; the coverage guard would pass having checked nothing")
	}

	t.Run("it renders a phrase for every declared stand-down reason", func(t *testing.T) {
		if missing := missingPhrases(skipReasons, skippedPrunePhrases); len(missing) > 0 {
			t.Errorf("skippedPrunePhrases has no phrase for %v; --fix would print the raw slug", missing)
		}
	})

	t.Run("it renders a not-evaluable detail for every declared stand-down reason", func(t *testing.T) {
		if missing := missingPhrases(skipReasons, notEvaluableDetails); len(missing) > 0 {
			t.Errorf("notEvaluableDetails has no detail for %v; the diagnosis would print the raw slug", missing)
		}
	})

	t.Run("it holds no phrase for a reason that is not declared", func(t *testing.T) {
		if extra := undeclaredKeys(skippedPrunePhrases, skipReasons); len(extra) > 0 {
			t.Errorf("skippedPrunePhrases holds undeclared reasons %v", extra)
		}
		if extra := undeclaredKeys(notEvaluableDetails, skipReasons); len(extra) > 0 {
			t.Errorf("notEvaluableDetails holds undeclared reasons %v", extra)
		}
	})

	t.Run("it fails when a newly declared reason has no phrase", func(t *testing.T) {
		const sixth = "new-reason-with-no-phrase"
		declared := append(slices.Clone(skipReasons), sixth)

		for name, vocabulary := range map[string]map[string]string{
			"skippedPrunePhrases": skippedPrunePhrases,
			"notEvaluableDetails": notEvaluableDetails,
		} {
			if missing := missingPhrases(declared, vocabulary); !slices.Equal(missing, []string{sixth}) {
				t.Errorf("missingPhrases(+%q, %s) = %v, want exactly [%q]; the rule would not catch the omission",
					sixth, name, missing, sixth)
			}
		}

		if phrase := phraseFor(skippedPrunePhrases, sixth); phrase != sixth {
			t.Errorf("phraseFor(skippedPrunePhrases, %q) = %q, want the raw reason as the runtime net", sixth, phrase)
		}
	})
}

// The coverage guard above ranges over skipReasons, so it is only as complete as
// that slice. This one reads the const block itself: a sixth reason declared and
// left out of the slice would otherwise be invisible to every check.
func TestSkipReasonsEnumeratesEveryDeclaredConst(t *testing.T) {
	const prefix = "skipReason"

	paths, err := sourceguardtest.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate cmd package sources: %v", err)
	}

	fset := token.NewFileSet()
	var declared, enumerated []string
	sliceFound := false
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range value.Names {
					switch {
					case gen.Tok == token.CONST && strings.HasPrefix(name.Name, prefix):
						declared = append(declared, name.Name)
					case gen.Tok == token.VAR && name.Name == "skipReasons":
						sliceFound = true
						enumerated = append(enumerated, sliceElementNames(t, fset, value.Values[i])...)
					}
				}
			}
		}
	}

	if len(declared) == 0 {
		t.Fatalf("no %s* const found in the cmd package; the guard is scanning nothing", prefix)
	}
	if !sliceFound {
		t.Fatal("no skipReasons slice found in the cmd package; the guard is scanning nothing")
	}

	slices.Sort(declared)
	slices.Sort(enumerated)
	if !slices.Equal(declared, enumerated) {
		t.Errorf("skipReasons enumerates %v, but the const block declares %v; every declared reason must be enumerable",
			enumerated, declared)
	}
	if len(enumerated) != len(skipReasons) {
		t.Errorf("skipReasons literal names %d reasons but the value holds %d", len(enumerated), len(skipReasons))
	}
}

// sliceElementNames returns the identifiers a composite slice literal lists, so
// the guard compares names with the const block rather than re-deriving values.
func sliceElementNames(t *testing.T, fset *token.FileSet, expr ast.Expr) []string {
	t.Helper()
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("%s: skipReasons is not a composite literal; the guard cannot read its members", fset.Position(expr.Pos()))
	}
	names := make([]string, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		ident, ok := elt.(*ast.Ident)
		if !ok {
			t.Fatalf("%s: skipReasons element is not an identifier; every member must name a declared const",
				fset.Position(elt.Pos()))
		}
		names = append(names, ident.Name)
	}
	return names
}
