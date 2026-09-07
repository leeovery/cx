package theme_test

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
	"github.com/leeovery/portal/internal/theme"
)

var canonicalHexValue = regexp.MustCompile(`^#[0-9A-F]{6}$`)

const embeddedTestFile = "embedded_test.go"

func embeddedLoader() theme.Loader {
	return theme.NewSilentLoader()
}

func TestEveryEmbeddedThemeIsValid(t *testing.T) {
	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("the embedded set is empty — this half of the built-in guarantee would pass vacuously")
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

			for _, tok := range result.Theme.All() {
				if tok.Value == "" {
					t.Errorf("built-in %q leaves %s unpopulated, want all %d tokens carrying a value", slug, tok.Name, wantTokens)
					continue
				}
				if !canonicalHexValue.MatchString(tok.Value) {
					t.Errorf("built-in %q token %s = %q, want the upper-case canonical #RRGGBB", slug, tok.Name, tok.Value)
				}
			}
		})
	}
}

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
				t.Fatalf("the shipped fallback for %s is %q, which resolves to no built-in — every %s fallback is unresolvable", slot.slot, slot.slug, slot.slot)
			}
			if rejection != nil {
				t.Fatalf("the shipped fallback for %s is %q, which is invalid: %s", slot.slot, slot.slug, rejection)
			}
		})
	}
}

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
				t.Fatalf("the shipped pair sets %s = %q, which resolves to no built-in", member.key, member.slug)
			}
			if rejection != nil {
				t.Fatalf("the shipped pair sets %s = %q, which is invalid: %s", member.key, member.slug, rejection)
			}
		})
	}

	if theme.DefaultDarkSlug == theme.DefaultLightSlug {
		t.Errorf("the shipped pair nominates %q for both slots — the adaptive default is a constant one", theme.DefaultDarkSlug)
	}
}

func TestEmbeddedSetIsNonEmpty(t *testing.T) {
	if slugs := theme.BuiltinSlugs(); len(slugs) == 0 {
		t.Fatal("no theme is embedded — the validity and resolution halves would both pass against nothing")
	}
}

func TestEmbeddedValidity_EnumeratesRatherThanNames(t *testing.T) {
	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("the embedded set is empty — the scan below would have nothing to look for")
	}

	source := sourceguardtest.PackageSource(t, ".", embeddedTestFile)

	scanned := 0
	ast.Inspect(source.File, func(n ast.Node) bool {
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
				t.Errorf("%s:%d names the built-in %q — this file enumerates the embedded set and reaches its defaults through builtins.go's constants, so a theme added by a later PR is enrolled automatically", embeddedTestFile, source.Position(lit.Pos()).Line, slug)
			}
		}
		return true
	})

	if scanned == 0 {
		t.Fatalf("%s yielded no string literals to scan — the guard passed without looking at anything", embeddedTestFile)
	}
}

const fatalCopyOwner = "broken_builtin.go"

func TestEmbeddedRejection_HasNoFatalPathInThePackage(t *testing.T) {
	const fatalCopy = "this binary is broken"

	owners := 0

	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		ast.Inspect(source.File, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "panic" && !inNilSeamConstructor(source.File, node.Pos()) {
					t.Errorf("%s:%d panics — a broken embedded file is an ordinary *Rejection here, and a broken fallback is an ordinary returned error", source.Path, source.Fset.Position(node.Pos()).Line)
				}
			case *ast.SelectorExpr:
				pkg, ok := node.X.(*ast.Ident)
				if !ok {
					return true
				}
				if pkg.Name == "os" && node.Sel.Name == "Exit" {
					t.Errorf("%s:%d calls os.Exit — internal/theme returns errors; main.go owns the single exit", source.Path, source.Fset.Position(node.Pos()).Line)
				}
				if pkg.Name == "log" && strings.HasPrefix(node.Sel.Name, "Fatal") {
					t.Errorf("%s:%d calls log.%s — internal/theme returns errors and never terminates a process", source.Path, source.Fset.Position(node.Pos()).Line, node.Sel.Name)
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
				if filepath.Base(source.Path) != fatalCopyOwner {
					t.Errorf("%s:%d carries the fatal startup copy — the sentence is single-sourced in %s and raised only where a fallback is needed, never where a file is read", source.Path, source.Fset.Position(node.Pos()).Line, fatalCopyOwner)
					return true
				}
				owners++
			}
			return true
		})
	}

	if owners != 1 {
		t.Errorf("%s declares the fatal copy %d times, want exactly 1 — the sentence is single-sourced", fatalCopyOwner, owners)
	}
}

const nilSeamConstructor = "NewLoader"

func inNilSeamConstructor(file *ast.File, pos token.Pos) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if pos >= fn.Body.Pos() && pos <= fn.Body.End() {
			return fn.Name.Name == nilSeamConstructor
		}
	}
	return false
}
