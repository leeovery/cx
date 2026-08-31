package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/leeovery/portal/internal/nanoid"
	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

func TestNewPaneToken(t *testing.T) {
	t.Run("it mints a token-shaped value", func(t *testing.T) {
		for range 200 {
			token, err := session.NewPaneToken()
			if err != nil {
				t.Fatalf("NewPaneToken: %v", err)
			}
			if !nanoid.IsTokenShaped(token) {
				t.Fatalf("NewPaneToken() = %q, which is not token-shaped", token)
			}
		}
	})

	t.Run("it mints a distinct token per call", func(t *testing.T) {
		seen := map[string]struct{}{}
		for range 200 {
			token, err := session.NewPaneToken()
			if err != nil {
				t.Fatalf("NewPaneToken: %v", err)
			}
			if _, dup := seen[token]; dup {
				t.Fatalf("NewPaneToken minted %q twice in 200 calls", token)
			}
			seen[token] = struct{}{}
		}
	})
}

// The pane token's width is hooks.json's key-recognition contract, so the mint
// must draw on the pane-token generator — the one width IsTokenShaped reads —
// rather than the general-purpose generator that session names and spawn ids
// share and that is free to move without reclassifying a persisted key.
func TestNewPaneToken_MintsFromThePaneTokenGenerator(t *testing.T) {
	paths, err := sourceguardtest.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate package sources: %v", err)
	}

	scanned := 0
	for _, path := range paths {
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		scanned++
		sourceguardtest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
			if funcName != "NewPaneToken" {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "NewGenerator" {
				t.Errorf("%s: NewPaneToken mints from the general-purpose generator — a pane token must be minted at the width IsTokenShaped recognises", path)
			}
			return true
		})
	}

	if scanned == 0 {
		t.Fatal("guard scanned no files")
	}
}
