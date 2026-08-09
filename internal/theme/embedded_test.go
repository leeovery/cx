package theme_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// This file is §7.6's build-time guarantee, and it is deliberately TWO HALVES
// carried in one place.
//
// There is no runtime fallback to hardcoded values beneath the built-in
// fallback — no compiled-in last-resort palette, no "if all else fails, paint
// these". Making the built-ins FILES moved their parse failures from compile
// time to load time, and §7.6's answer to that is not a runtime crutch: the
// situation is made impossible at build time instead.
//
//  1. Every embedded built-in parses and validates against §6.1's rule.
//  2. Every fallback slug AND the shipped default pair RESOLVE within that set
//     (§8.3, §8.5).
//
// One half alone is not enough, and the reason is specific. Validating the
// files proves the FILES are good, but the fallback is hardcoded slug constants
// resolving INTO that set — rename a built-in file in a later PR, or typo a
// constant, and every embedded theme still validates while every fallback path
// becomes unresolvable. Half one stays green throughout, and nothing else in
// the feature would catch it.
//
// NOTHING HERE NAMES A THEME. Half one enumerates BuiltinSlugs(); half two goes
// through the production constants, whose values live in builtins.go. So a
// built-in added by a later PR is enrolled with no test edit, and
// TestEmbeddedValidity_EnumeratesRatherThanNames is the structural pin on that
// rather than a convention.
//
// VALIDATION IS DELIBERATELY NOT STARTUP-EAGER. Nothing walks the embedded set
// at init, because this test already proves the set at build time and
// re-proving it on every launch buys nothing on a cold path this feature
// otherwise adds no cost to. Task 2.1's no-init guard
// (TestThemePackage_HasNoInitFunction, leaf_guard_test.go) is the structural
// half of that claim; this file is the half that makes it safe to hold.
//
// The escalation §7.6 does specify — the one-line
// `built-in theme <slug> is missing or invalid — this binary is broken` — is
// raised where a fallback is NEEDED, which is Phase 5, not here. internal/theme
// reports a broken embedded file as an ordinary rejection (asserted in-package
// by TestEmbeddedParseFailureIsAnOrdinaryError), and carries no fatal path at
// all (asserted below).

// canonicalHexValue is §4.3's stored form: a '#' and exactly six UPPER-CASE hex
// digits. The parser canonicalises case on the way in, so a shipped value that
// does not match this has not been through it.
var canonicalHexValue = regexp.MustCompile(`^#[0-9A-F]{6}$`)

// embeddedTestFile is this file, read back so the enrolment guard below can
// scan the source it is written in.
const embeddedTestFile = "embedded_test.go"

// embeddedLoader is the production loader shape — reserved slugs derived from
// the embedded set — wired to a discarding event seam.
//
// The seam is silent because this file DIAGNOSES the shipped set rather than
// using a theme, so the §12.3 trail has nothing to record. The production shape
// is used anyway, rather than a bare Loader, because it is the shape Phase 5's
// fallback will hold when it resolves these same slugs.
func embeddedLoader() theme.Loader {
	return theme.NewSilentLoader()
}

// TestEveryEmbeddedThemeIsValid is §7.6's FIRST half: every theme Portal ships
// parses and validates against §6.1's rule — all 19 tokens present, every value
// syntactically well-formed.
//
// The failure names the slug and §6.2's reason WITH its detail, because the
// person who reads it is the maintainer who just added or edited a .theme file
// and needs to be told which file and which line, not merely that the suite is
// red.
//
// The value check is upper-case canonical rather than merely hex-shaped, which
// is a stronger assertion than validity alone: §4.3's canonicalisation is what
// lets §11.3's background diffing and §11.4's retained startup canvas hex
// compare hex strings at all, so a shipped value reaching a renderer in the
// file's own case would break a comparison no floor test would notice.
func TestEveryEmbeddedThemeIsValid(t *testing.T) {
	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("the embedded set is empty — this half of §7.6's guarantee would pass vacuously")
	}

	loader := embeddedLoader()
	wantTokens := len(theme.TokenNames())

	for _, slug := range slugs {
		t.Run(slug, func(t *testing.T) {
			result, rejection, found := loader.LoadBuiltin(slug)

			if !found {
				t.Fatalf("built-in %q is enumerated but resolves to nothing", slug)
			}
			if rejection != nil {
				t.Fatalf("built-in %q is invalid: %s — %s", slug, rejection.Reason, rejection.Detail)
			}

			tokens := result.Theme.All()
			if len(tokens) != wantTokens {
				t.Fatalf("built-in %q yielded %d tokens, want the closed vocabulary's %d", slug, len(tokens), wantTokens)
			}

			for _, tok := range tokens {
				if tok.Value == "" {
					t.Errorf("built-in %q leaves %s unpopulated, want all %d tokens carrying a value", slug, tok.Name, wantTokens)
					continue
				}
				if !canonicalHexValue.MatchString(tok.Value) {
					t.Errorf("built-in %q token %s = %q, want the §4.3 upper-case canonical #RRGGBB", slug, tok.Name, tok.Value)
				}
			}
		})
	}
}

// TestFallbackSlugsResolveWithinEmbeddedSet is §7.6's SECOND half: every slug
// §8.5 falls back to is one the embedded set actually holds.
//
// This is a RESOLUTION assertion and is deliberately distinct from the validity
// one above. The two fail independently and for different reasons: renaming
// nord.theme breaks validity for nothing and resolution for nothing, while
// renaming the file DefaultDarkSlug names leaves every embedded theme valid and
// makes two of the three slots below unresolvable. Only this half sees that.
//
// All three of §8.5's slots are stated rather than the two distinct values,
// because the table is per-slot and mode-matched: the constant `theme` slot
// falling to the DARK default is its own decision, not a consequence of the
// other two, and a future fourth slot has an obvious home here.
func TestFallbackSlugsResolveWithinEmbeddedSet(t *testing.T) {
	slots := []struct {
		slot string
		slug string
	}{
		{slot: "theme_dark", slug: theme.DefaultDarkSlug},
		{slot: "theme_light", slug: theme.DefaultLightSlug},
		{slot: "theme (constant)", slug: theme.DefaultDarkSlug},
	}

	loader := embeddedLoader()
	for _, slot := range slots {
		t.Run(slot.slot, func(t *testing.T) {
			_, rejection, found := loader.LoadBuiltin(slot.slug)

			if !found {
				t.Fatalf("§8.5's fallback for %s is %q, which resolves to no built-in — every %s fallback is unresolvable", slot.slot, slot.slug, slot.slot)
			}
			if rejection != nil {
				t.Fatalf("§8.5's fallback for %s is %q, which is invalid: %s", slot.slot, slot.slug, rejection)
			}
		})
	}
}

// TestShippedDefaultPairResolves is the same resolution assertion for §8.3's
// shipped adaptive pair — the nomination a brand-new user launches with.
//
// It is its own carrier rather than a duplicate of the fallback test because
// the two roles are separate decisions that merely happen to hold the same two
// values, and §8.3's "the adaptive pair degrades to a constant dark default"
// argument rests on that coincidence. Adopting a single fixed fallback would
// leave this test green and the argument dead, which is why builtins.go's
// constants carry the warning rather than this file.
//
// The pair is also asserted to BE a pair. Typing DefaultLightSlug to the dark
// slug's value leaves both members resolving, both themes valid and every
// assertion above green, while adaptivity is silently a no-op — a light
// terminal would get the dark theme with nothing anywhere reporting it.
func TestShippedDefaultPairResolves(t *testing.T) {
	pair := []struct {
		key  string
		slug string
	}{
		{key: "theme_dark", slug: theme.DefaultDarkSlug},
		{key: "theme_light", slug: theme.DefaultLightSlug},
	}

	loader := embeddedLoader()
	for _, member := range pair {
		t.Run(member.key, func(t *testing.T) {
			_, rejection, found := loader.LoadBuiltin(member.slug)

			if !found {
				t.Fatalf("§8.3 ships %s = %q, which resolves to no built-in", member.key, member.slug)
			}
			if rejection != nil {
				t.Fatalf("§8.3 ships %s = %q, which is invalid: %s", member.key, member.slug, rejection)
			}
		})
	}

	if theme.DefaultDarkSlug == theme.DefaultLightSlug {
		t.Errorf("§8.3's shipped pair nominates %q for both slots — the adaptive default is a constant one", theme.DefaultDarkSlug)
	}
}

// TestEmbeddedSetIsNonEmpty refuses the one failure mode a loop over an
// enumerated set can silently have: measuring nothing and reporting success.
//
// Both halves of §7.6 are loops over BuiltinSlugs(), so an empty embedded set
// passes each of them vacuously — which is precisely the state a binary must
// never ship in. Stated as its own named test so the guarantee is a fact of the
// suite rather than a fatal buried in a helper.
func TestEmbeddedSetIsNonEmpty(t *testing.T) {
	if slugs := theme.BuiltinSlugs(); len(slugs) == 0 {
		t.Fatal("no theme is embedded — §7.6's validity and resolution halves would both pass against nothing")
	}
}

// TestEmbeddedValidity_EnumeratesRatherThanNames pins the enrolment property
// the rest of this file rests on: a built-in added by a later PR is covered
// with no test edit, because no test here names a theme.
//
// The check is structural rather than aspirational — the file's own source is
// scanned for a string literal carrying any embedded slug. Half one enumerates
// BuiltinSlugs(), and half two reaches its two themes through the production
// constants of builtins.go, so neither needs a name in this file. A literal
// slug appearing here would mean some assertion had been pinned to one palette
// and would silently stop covering the next one added.
//
// Only string literals are scanned. A slug in prose — this comment names none,
// but a later one might — is documentation, not an enrolment.
func TestEmbeddedValidity_EnumeratesRatherThanNames(t *testing.T) {
	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("the embedded set is empty — the scan below would have nothing to look for")
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, embeddedTestFile, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", embeddedTestFile, err)
	}

	scanned := 0
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}

		value, uerr := strconv.Unquote(lit.Value)
		if uerr != nil {
			return true
		}
		scanned++

		for _, slug := range slugs {
			if strings.Contains(value, slug) {
				t.Errorf("%s:%d names the built-in %q — this file enumerates the embedded set and reaches its defaults through builtins.go's constants, so a theme added by a later PR is enrolled automatically", embeddedTestFile, fset.Position(lit.Pos()).Line, slug)
			}
		}
		return true
	})

	if scanned == 0 {
		t.Fatalf("%s yielded no string literals to scan — the guard passed without looking at anything", embeddedTestFile)
	}
}

// fatalCopyOwner is the ONE production file allowed to carry §14A's fatal
// sentence: the file that single-sources it.
const fatalCopyOwner = "broken_builtin.go"

// TestEmbeddedRejection_HasNoFatalPathInThePackage asserts the other side of
// §7.6's mechanism: internal/theme reports a broken embedded file and terminates
// nothing.
//
// The loader returns an ordinary error for an embedded parse failure — it does
// not panic, does not exit and does not print. The escalation happens where the
// fallback is NEEDED, which under §7.6 is a returned error and never a raised
// one, so the user sees one line rather than a Go stack trace and main.go's
// panic-recovering exit stays the backstop for a genuine programming fault
// rather than the designed route.
//
// This is guarded rather than merely intended because the temptation sits right
// next to the code: a package holding files it knows must be valid invites a
// `panic("broken built-in")` at the one place that discovers otherwise, which
// would turn "your theme is invalid" into a crash on someone's cold start.
//
// THE COPY CHECK IS AN OWNERSHIP CHECK, not a prohibition. When this guard was
// first written the escalation had no home yet and the sentence belonged
// nowhere in the package; Phase 5 gave it one — resolveSlot, where a fallback is
// needed — and single-sourced it in fatalCopyOwner. So the invariant is now the
// stronger of the two: the sentence exists in EXACTLY ONE production file, and
// the PARSE layer (load.go, builtins.go, lex.go, validate.go) still cannot raise
// it. A second copy anywhere is what would let the panel, doctor and the fatal
// path drift apart on the one line a user with a broken binary ever sees.
func TestEmbeddedRejection_HasNoFatalPathInThePackage(t *testing.T) {
	// §14A's sentence, reduced to the fragment no other copy could contain by
	// coincidence.
	const fatalCopy = "this binary is broken"

	owners := 0

	for _, source := range parseThemeSources(t) {
		ast.Inspect(source.File, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "panic" {
					t.Errorf("%s:%d panics — a broken embedded file is an ordinary *Rejection here, and a broken fallback is an ordinary returned error", source.Name, source.Fset.Position(node.Pos()).Line)
				}
			case *ast.SelectorExpr:
				pkg, ok := node.X.(*ast.Ident)
				if !ok {
					return true
				}
				if pkg.Name == "os" && node.Sel.Name == "Exit" {
					t.Errorf("%s:%d calls os.Exit — internal/theme returns errors; main.go owns the single exit", source.Name, source.Fset.Position(node.Pos()).Line)
				}
				if pkg.Name == "log" && strings.HasPrefix(node.Sel.Name, "Fatal") {
					t.Errorf("%s:%d calls log.%s — internal/theme returns errors and never terminates a process", source.Name, source.Fset.Position(node.Pos()).Line, node.Sel.Name)
				}
			case *ast.BasicLit:
				if node.Kind != token.STRING {
					return true
				}
				value, uerr := strconv.Unquote(node.Value)
				if uerr != nil {
					return true
				}
				if !strings.Contains(value, fatalCopy) {
					return true
				}
				if source.Name != fatalCopyOwner {
					t.Errorf("%s:%d carries §14A's fatal startup copy — the sentence is single-sourced in %s and raised only where a fallback is needed, never where a file is read", source.Name, source.Fset.Position(node.Pos()).Line, fatalCopyOwner)
					return true
				}
				owners++
			}
			return true
		})
	}

	if owners != 1 {
		t.Errorf("%s declares §14A's fatal copy %d times, want exactly 1 — the sentence is single-sourced", fatalCopyOwner, owners)
	}
}
