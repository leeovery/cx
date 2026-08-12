package tui_test

import (
	"go/parser"
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
		file:   "retired_token_guard_test.go",
		self:   true,
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

func exemptRetiredName(file, name string) bool {
	for _, e := range retiredNameExemptions {
		if e.file == file && (e.self || e.name == name) {
			return true
		}
	}
	return false
}

var renderLayerPackageDirs = []string{".", filepath.Join("..", "capture")}

func TestNoRetiredTokenNameInComments(t *testing.T) {
	for _, dir := range renderLayerPackageDirs {
		matches, err := sourceguardtest.PackageGoFiles(dir, true)
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
