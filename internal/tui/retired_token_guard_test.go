package tui_test

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

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

type retiredNameExemption struct {
	// Package-qualified (`<package dir>/<file>`), not a bare base name: the scan
	// spans two packages, so a base name would let an exemption granted for one
	// silence a same-named file the other gains later.
	file string
	// name scopes the exemption to one retired name; self is the deliberate
	// whole-file form, so an entry that simply forgets a name is not silently
	// a blanket exemption.
	name   string
	self   bool
	reason string
}

var retiredNameExemptions = []retiredNameExemption{
	// Every retired name, because this file declares the table itself. Scoped by
	// filename alone — an empty name would blanket-exempt any file added here.
	{
		file:   filepath.Join("tui", "retired_token_guard_test.go"),
		self:   true,
		reason: "declares the retired table this guard matches against",
	},
	{
		file:   filepath.Join("tui", "active_theme_test.go"),
		name:   "border.footer",
		reason: "pins that the retired footer-rule shade renders nowhere, so the rule reads from the one border token",
	},
}

func exemptRetiredName(file, name string) bool {
	for _, e := range retiredNameExemptions {
		if e.file == file && (e.self || e.name == name) {
			return true
		}
	}
	return false
}

var renderLayerPackageDirs = []string{filepath.Join("..", "tui"), filepath.Join("..", "capture")}

func TestNoRetiredTokenName(t *testing.T) {
	for _, dir := range renderLayerPackageDirs {
		for _, source := range sourceguardtest.ParsePackageSources(t, dir, true) {
			path, file := source.Path, source.File
			qualified := filepath.Join(filepath.Base(dir), filepath.Base(path))
			t.Run(qualified, func(t *testing.T) {
				report := func(pos token.Pos, where, text string) {
					for retired, current := range retiredTokenNames {
						if !strings.Contains(text, retired) || exemptRetiredName(qualified, retired) {
							continue
						}
						t.Errorf("%s:%d names the retired token %q in a %s; the role is %q now", path, source.Position(pos).Line, retired, where, current)
					}
				}

				for _, group := range file.Comments {
					for _, c := range group.List {
						report(c.Pos(), "comment", c.Text)
					}
				}

				// String literals too: a failure message naming a dead token
				// teaches the vocabulary off a red test run, which is where a
				// maintainer reads it most attentively.
				ast.Inspect(file, func(n ast.Node) bool {
					lit, ok := n.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true
					}
					report(lit.Pos(), "string literal", lit.Value)
					return true
				})
			})
		}
	}
}
