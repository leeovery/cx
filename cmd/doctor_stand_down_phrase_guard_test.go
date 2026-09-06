package cmd

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// missingPhrases reports the declared reasons a surface vocabulary renders
// nothing for. It is the whole rule the coverage guard applies, named so the
// guard's own failure mode is testable: a reason declared without a phrase.
func missingPhrases(reasons []skipReason, m map[skipReason]string) []skipReason {
	var missing []skipReason
	for _, reason := range reasons {
		if phrase, ok := m[reason]; !ok || phrase == "" {
			missing = append(missing, reason)
		}
	}
	return missing
}

// undeclaredKeys reports the vocabulary's keys that name no declared reason —
// the leftover a retired reason would strand.
func undeclaredKeys(m map[skipReason]string, reasons []skipReason) []skipReason {
	var extra []skipReason
	for key := range m {
		if !slices.Contains(reasons, key) {
			extra = append(extra, key)
		}
	}
	slices.Sort(extra)
	return extra
}

// Every stand-down reason reaches the user through two vocabularies, and
// nothing at runtime words one they leave out. This guard is the whole
// enforcement: an omission is a test failure here, rather than an empty phrase
// on the command whose purpose is explaining what happened.
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

	t.Run("it fails the coverage guard when a declared reason is absent from a phrase map", func(t *testing.T) {
		const unmapped skipReason = "new-reason-with-no-phrase"
		declared := append(slices.Clone(skipReasons), unmapped)

		for name, vocabulary := range map[string]map[skipReason]string{
			"skippedPrunePhrases": skippedPrunePhrases,
			"notEvaluableDetails": notEvaluableDetails,
		} {
			if missing := missingPhrases(declared, vocabulary); !slices.Equal(missing, []skipReason{unmapped}) {
				t.Errorf("missingPhrases(+%q, %s) = %v, want exactly [%q]; the rule would not catch the omission",
					unmapped, name, missing, unmapped)
			}

			// Nothing renders an unmapped reason: this guard is the whole
			// enforcement, so a reason left out of a map must not reach a
			// surface wearing its own slug as copy.
			if phrase := phraseFor(vocabulary, unmapped); phrase != "" {
				t.Errorf("phraseFor(%s, %q) = %q, want nothing; the guard is the only enforcement", name, unmapped, phrase)
			}
		}
	})
}

// The coverage guard above ranges over skipReasons, so it is only as complete
// as that slice. This one reads the declarations themselves, keyed on the
// skipReason type rather than on the names chosen for them: the type is what
// makes a const a reason, so a const the compiler accepts as one and the slice
// omits is exactly what stays invisible to every check that ranges over the
// set. Membership is what the type cannot hold, and it is what this guard still
// adds over it.
func TestSkipReasonsEnumeratesEveryDeclaredConst(t *testing.T) {
	sources := sourceguardtest.ParsePackageSources(t, ".", false)
	declared := declaredReasonConsts(sources)
	enumerated := enumeratedReasons(t, sources)

	if len(declared) == 0 {
		t.Fatalf("no const declared with the %s type in the cmd package; the guard is scanning nothing", reasonTypeName)
	}

	t.Run("it enumerates every const declared with the reason type", func(t *testing.T) {
		if missing := unenumerated(declared, enumerated); len(missing) > 0 {
			t.Errorf("skipReasons omits the declared reason(s) %v; every declared reason must be enumerable", missing)
		}
		if extra := unenumerated(enumerated, declared); len(extra) > 0 {
			t.Errorf("skipReasons enumerates %v, which no const declares", extra)
		}
		if len(enumerated) != len(skipReasons) {
			t.Errorf("skipReasons literal names %d reasons but the value holds %d", len(enumerated), len(skipReasons))
		}
	})

	// The type admits a const named anything, so the rule must catch one the
	// slice leaves out however it is named.
	t.Run("it fails a reason absent from the enumerable set", func(t *testing.T) {
		synthetic := parseSyntheticSource(t, `package cmd

const offConvention skipReason = "off-convention"

var skipReasons = []skipReason{skipReasonRestoring}
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
	reasonTypeName   = "skipReason"
	reasonNamePrefix = "skipReason"
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

// enumeratedReasons returns the identifiers the skipReasons slice literal
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
			if name.Name != "skipReasons" {
				continue
			}
			found = true
			enumerated = append(enumerated, sliceElementNames(t, fset, value.Values[i])...)
		}
	})
	if !found {
		t.Fatal("no skipReasons slice found; the guard is scanning nothing")
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

// A phrase both vocabularies use must be written once and composed into each,
// because a guard that binds map keys cannot see a map value re-authored beside
// it: re-wording one copy would leave the other surface printing the old words
// for the same condition. The rule is over the declarations themselves — the
// values agreeing at runtime is what a shared const produces, not evidence of
// one.
func TestStandDownVocabulariesShareNoInlineLiteral(t *testing.T) {
	vocabularies := []string{"skippedPrunePhrases", "notEvaluableDetails"}
	literals := map[string][]string{}
	found := map[string]bool{}
	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		for _, decl := range source.File.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range value.Names {
					if !slices.Contains(vocabularies, name.Name) {
						continue
					}
					found[name.Name] = true
					literals[name.Name] = mapValueLiterals(t, source.Fset, value.Values[i])
				}
			}
		}
	}

	for _, name := range vocabularies {
		if !found[name] {
			t.Fatalf("no %s map literal found in the cmd package; the guard is scanning nothing", name)
		}
	}

	runtime := map[string]map[skipReason]string{
		"skippedPrunePhrases": skippedPrunePhrases,
		"notEvaluableDetails": notEvaluableDetails,
	}
	for i, name := range vocabularies {
		sibling := vocabularies[1-i]
		for _, phrase := range literals[name] {
			for _, rendered := range runtime[sibling] {
				if rendered == phrase {
					t.Errorf("%s authors %q inline while %s renders the same phrase; lift it into a const both compose from",
						name, phrase, sibling)
				}
			}
		}
	}
}

// mapValueLiterals returns the bare string literals a map composite literal
// authors as values. Any other expression — an identifier, or a concatenation —
// is skipped, so a phrase assembled from parts is outside the rule's reach.
func mapValueLiterals(t *testing.T, fset *token.FileSet, expr ast.Expr) []string {
	t.Helper()
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		t.Fatalf("%s: vocabulary is not a composite literal; the guard cannot read its values", fset.Position(expr.Pos()))
	}
	var values []string
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			t.Fatalf("%s: vocabulary element is not a key/value pair", fset.Position(elt.Pos()))
		}
		basic, ok := kv.Value.(*ast.BasicLit)
		if !ok || basic.Kind != token.STRING {
			continue
		}
		phrase, err := strconv.Unquote(basic.Value)
		if err != nil {
			t.Fatalf("%s: vocabulary value %s is not a quoted string", fset.Position(basic.Pos()), basic.Value)
		}
		values = append(values, phrase)
	}
	return values
}
