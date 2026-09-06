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

	"github.com/leeovery/portal/internal/hooksweep"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

// missingPhrases reports the declared reasons a surface vocabulary renders
// nothing for. It is the whole rule the coverage guard applies, named so the
// guard's own failure mode is testable: a reason declared without a phrase.
func missingPhrases(reasons []hooksweep.Reason, m map[hooksweep.Reason]string) []hooksweep.Reason {
	var missing []hooksweep.Reason
	for _, reason := range reasons {
		if phrase, ok := m[reason]; !ok || phrase == "" {
			missing = append(missing, reason)
		}
	}
	return missing
}

// undeclaredKeys reports the vocabulary's keys that name no declared reason —
// the leftover a retired reason would strand.
func undeclaredKeys(m map[hooksweep.Reason]string, reasons []hooksweep.Reason) []hooksweep.Reason {
	var extra []hooksweep.Reason
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
	if len(hooksweep.Reasons) == 0 {
		t.Fatal("hooksweep.Reasons is empty; the coverage guard would pass having checked nothing")
	}

	t.Run("it renders a phrase for every declared stand-down reason", func(t *testing.T) {
		if missing := missingPhrases(hooksweep.Reasons, skippedPrunePhrases); len(missing) > 0 {
			t.Errorf("skippedPrunePhrases has no phrase for %v; --fix would print the raw slug", missing)
		}
	})

	t.Run("it renders a not-evaluable detail for every declared stand-down reason", func(t *testing.T) {
		if missing := missingPhrases(hooksweep.Reasons, notEvaluableDetails); len(missing) > 0 {
			t.Errorf("notEvaluableDetails has no detail for %v; the diagnosis would print the raw slug", missing)
		}
	})

	t.Run("it holds no phrase for a reason that is not declared", func(t *testing.T) {
		if extra := undeclaredKeys(skippedPrunePhrases, hooksweep.Reasons); len(extra) > 0 {
			t.Errorf("skippedPrunePhrases holds undeclared reasons %v", extra)
		}
		if extra := undeclaredKeys(notEvaluableDetails, hooksweep.Reasons); len(extra) > 0 {
			t.Errorf("notEvaluableDetails holds undeclared reasons %v", extra)
		}
	})

	t.Run("it fails the coverage guard when a declared reason is absent from a phrase map", func(t *testing.T) {
		const unmapped hooksweep.Reason = "new-reason-with-no-phrase"
		declared := append(slices.Clone(hooksweep.Reasons), unmapped)

		for name, vocabulary := range map[string]map[hooksweep.Reason]string{
			"skippedPrunePhrases": skippedPrunePhrases,
			"notEvaluableDetails": notEvaluableDetails,
		} {
			if missing := missingPhrases(declared, vocabulary); !slices.Equal(missing, []hooksweep.Reason{unmapped}) {
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

	runtime := map[string]map[hooksweep.Reason]string{
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

// phraseConstSuffix is the convention naming a const as one of the stand-down
// phrases, so the guard below reads the declarations rather than a list it
// would have to be told to grow.
const phraseConstSuffix = "StandDownPhrase"

// phraseRespelling is one production string literal that spells a phrase a
// const already declares — the second home the guard exists to find.
type phraseRespelling struct {
	Const   string
	Literal string
}

// standDownPhraseConsts returns every declared stand-down phrase by the name
// that declares it.
func standDownPhraseConsts(t *testing.T, sources []sourceguardtest.ParsedSource) map[string]string {
	t.Helper()

	consts := map[string]string{}
	forEachValueSpec(sources, func(gen *ast.GenDecl, fset *token.FileSet, value *ast.ValueSpec) {
		if gen.Tok != token.CONST {
			return
		}
		for i, name := range value.Names {
			if !strings.HasSuffix(name.Name, phraseConstSuffix) {
				continue
			}
			consts[name.Name] = stringLiteralValue(t, fset, value.Values[i])
		}
	})
	return consts
}

// standDownPhraseRespellings reports the string literals that spell a declared
// phrase somewhere other than its declaration. Containment rather than equality
// is the rule: a surface that words a phrase and then adds to it — the
// not-evaluable suffix, a prefix — has written the phrase a second time as
// surely as one that repeats it whole.
func standDownPhraseRespellings(sources []sourceguardtest.ParsedSource, consts map[string]string) []phraseRespelling {
	declarations := map[ast.Expr]bool{}
	forEachValueSpec(sources, func(gen *ast.GenDecl, _ *token.FileSet, value *ast.ValueSpec) {
		if gen.Tok != token.CONST {
			return
		}
		for i, name := range value.Names {
			if strings.HasSuffix(name.Name, phraseConstSuffix) {
				declarations[value.Values[i]] = true
			}
		}
	})

	var found []phraseRespelling
	for _, source := range sources {
		ast.Inspect(source.File, func(node ast.Node) bool {
			basic, ok := node.(*ast.BasicLit)
			if !ok || basic.Kind != token.STRING || declarations[ast.Expr(basic)] {
				return true
			}
			literal, err := strconv.Unquote(basic.Value)
			if err != nil {
				return true
			}
			for name, phrase := range consts {
				if strings.Contains(literal, phrase) {
					found = append(found, phraseRespelling{Const: name, Literal: literal})
				}
			}
			return true
		})
	}
	slices.SortFunc(found, func(a, b phraseRespelling) int { return strings.Compare(a.Literal, b.Literal) })
	return found
}

func stringLiteralValue(t *testing.T, fset *token.FileSet, expr ast.Expr) string {
	t.Helper()

	basic, ok := expr.(*ast.BasicLit)
	if !ok || basic.Kind != token.STRING {
		t.Fatalf("%s: stand-down phrase const is not a string literal", fset.Position(expr.Pos()))
	}
	value, err := strconv.Unquote(basic.Value)
	if err != nil {
		t.Fatalf("%s: stand-down phrase const %s is not a quoted string", fset.Position(basic.Pos()), basic.Value)
	}
	return value
}

// A phrase is declared once and every surface that prints it composes from the
// declaration. A second spelling agrees with the first only until one of them
// is re-worded, and nothing at runtime can tell the two apart — so the rule is
// over the source: no production literal may spell a phrase a const declares.
func TestStandDownPhrasesAreSpelledOnlyInTheirDeclaredHome(t *testing.T) {
	sources := sourceguardtest.ParsePackageSources(t, ".", false)
	consts := standDownPhraseConsts(t, sources)
	if len(consts) == 0 {
		t.Fatalf("no const named *%s in the cmd package; the guard is scanning nothing", phraseConstSuffix)
	}

	t.Run("it finds no phrase spelled outside its declaration", func(t *testing.T) {
		for _, respelling := range standDownPhraseRespellings(sources, consts) {
			t.Errorf("the literal %q spells the phrase %s already declares; compose it from the const instead",
				respelling.Literal, respelling.Const)
		}
	})

	t.Run("it fails the copy guard when a phrase is spelled outside its declared home", func(t *testing.T) {
		synthetic := parseSyntheticSource(t, `package cmd

const lockedStandDownPhrase = "hooks.json is locked"

var elsewhere = map[string]string{"lock": "hooks.json is locked (not evaluable)"}
`)
		want := []phraseRespelling{{Const: "lockedStandDownPhrase", Literal: "hooks.json is locked (not evaluable)"}}
		got := standDownPhraseRespellings(synthetic, standDownPhraseConsts(t, synthetic))
		if !slices.Equal(got, want) {
			t.Errorf("standDownPhraseRespellings = %v, want %v; the rule would not catch the second spelling", got, want)
		}
	})
}
