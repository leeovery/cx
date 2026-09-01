package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The token `hook set` stamps onto a pane becomes a key in hooks.json, and a
// persisted key is classified by nanoid.IsTokenShaped — which reads the
// pane-token width alone. The general-purpose width that session names and
// spawn ids share is explicitly free to move; if the mint drew on it, moving it
// would reclassify every key already persisted, whereupon the staleness rule
// would decline to judge them and retain them forever. The two widths are equal
// today, so nothing observable separates the generators.
func TestHookSeams_MintsTokensAtThePaneTokenWidth(t *testing.T) {
	const (
		wantGenerator  = "NewPaneTokenGenerator"
		wrongGenerator = "NewGenerator"
	)

	paths, err := sourceguardtest.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate package sources: %v", err)
	}

	found := false
	for _, path := range paths {
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			assign, isAssign := n.(*ast.AssignStmt)
			if !isAssign {
				return true
			}
			for i, lhs := range assign.Lhs {
				target, isSel := lhs.(*ast.SelectorExpr)
				if !isSel || target.Sel.Name != "TokenMinter" || i >= len(assign.Rhs) {
					continue
				}
				found = true

				switch minter := generatorName(assign.Rhs[i]); minter {
				case wantGenerator:
				case wrongGenerator:
					t.Errorf("%s: TokenMinter defaults to the general-purpose %s — a pane token must be minted at the width IsTokenShaped reads, so hooks.json's persisted keys stay judgeable", path, minter)
				default:
					t.Errorf("%s: TokenMinter defaults to %q, want nanoid.%s", path, minter, wantGenerator)
				}
			}
			return true
		})
	}

	if !found {
		t.Fatalf("guard found no assignment to TokenMinter — it can no longer see the mint it exists to pin")
	}
}

// generatorName returns the name of the nanoid generator constructor the
// expression calls, or "" when it calls none.
func generatorName(rhs ast.Expr) string {
	call, isCall := rhs.(*ast.CallExpr)
	if !isCall {
		return ""
	}
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return ""
	}
	if pkg, isIdent := sel.X.(*ast.Ident); !isIdent || pkg.Name != "nanoid" {
		return ""
	}
	return sel.Sel.Name
}
