package sourceguardtest

import "go/ast"

// CalleeName returns the name a call expression names — the identifier of a
// bare call, or the selected name of a method or qualified call, so that
// s.Load() and Load() both report "Load". Any other callee shape, an
// immediately-invoked function literal among them, names nothing and reports
// the empty string.
//
// It is the companion of ForEachFuncCall: a guard matches the visited call's
// name against its own vocabulary rather than re-authoring the unwrap.
func CalleeName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}
