package tui_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

// retiredTokenNames maps every token name the vocabulary retired to the name that
// now carries that role. It is one declared table so the rename has a single
// statement in the render layer, and it is what the guard below reads.
//
// These names are dead: no Theme field, no .theme key and no loader case answers
// to them. A comment that still uses one teaches a vocabulary a drop-in theme
// cannot be written against, and nothing else catches that — the compiler sees a
// comment.
var retiredTokenNames = map[string]string{
	"accent.violet":     "accent.primary",
	"accent.blue":       "accent.key",
	"accent.cyan":       "accent.mode",
	"accent.orange":     "accent.attention",
	"state.green":       "state.positive",
	"state.red":         "state.destructive",
	"text.strong":       "text.secondary",
	"text.muted-bright": "text.tertiary",
	"text.detail":       "text.muted",
	"text.dim":          "text.subtle",
	"bg.warning":        "bg.attention",
	"bg.track":          "bg.subtle",
	"text.on-warning":   "text.on-attention",
	"border.separator":  "border",
	"border.footer":     "border",
}

// retiredNameExemption is one file allowed to name one retired token, with the
// reason that reference is correct in place.
type retiredNameExemption struct {
	file   string
	name   string
	reason string
}

// retiredNameExemptions is the closed set of comments that name a retired token
// deliberately: guards whose whole subject is that the name — and the shade it
// used to carry — no longer renders. Naming it is the point, so substituting the
// current name would destroy the claim. Every other occurrence is stale prose.
var retiredNameExemptions = []retiredNameExemption{
	{
		file:   "retired_token_guard_test.go",
		reason: "declares the retired table this guard matches against",
	},
	{
		file:   "active_theme_test.go",
		name:   "border.footer",
		reason: "pins that the retired footer-rule shade renders nowhere, so the rule reads from the one border token",
	},
	{
		file:   "help_modal_frame_test.go",
		name:   "border.separator",
		reason: "records the two frame roles that were consolidated into the one border token",
	},
	{
		file:   "help_modal_frame_test.go",
		name:   "border.footer",
		reason: "records the two frame roles that were consolidated into the one border token",
	},
}

// exemptRetiredName reports whether file may name the retired token.
func exemptRetiredName(file, name string) bool {
	for _, e := range retiredNameExemptions {
		if e.file == file && (e.name == "" || e.name == name) {
			return true
		}
	}
	return false
}

// renderLayerPackageDirs are the package directories the guard covers: the render
// layer and the fixture harness that renders through it. Both describe the same
// surfaces in the same vocabulary, so a retired name is equally misleading in
// either.
var renderLayerPackageDirs = []string{".", filepath.Join("..", "capture")}

// TestNoRetiredTokenNameInComments fails when a comment in the render layer names
// a token the vocabulary retired, reporting the file, the line, the dead name and
// the name that carries the role now.
//
// It reads the parsed comments rather than the raw file text, so it judges only
// the prose a maintainer learns the vocabulary from.
func TestNoRetiredTokenNameInComments(t *testing.T) {
	for _, dir := range renderLayerPackageDirs {
		matches, err := portalbintest.PackageGoFiles(dir, true)
		if err != nil {
			t.Fatalf("enumerate the %s package sources: %v", dir, err)
		}
		for _, path := range matches {
			name := filepath.Base(path)
			t.Run(filepath.Join(filepath.Base(dir), name), func(t *testing.T) {
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
				if err != nil {
					t.Fatalf("parse %s: %v", path, err)
				}
				for _, group := range file.Comments {
					for _, c := range group.List {
						for retired, current := range retiredTokenNames {
							if !strings.Contains(c.Text, retired) || exemptRetiredName(name, retired) {
								continue
							}
							t.Errorf("%s:%d names the retired token %q; the role is %q now", path, fset.Position(c.Pos()).Line, retired, current)
						}
					}
				}
			})
		}
	}
}
