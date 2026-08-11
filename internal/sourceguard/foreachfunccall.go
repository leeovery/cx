package sourceguard

import "go/ast"

// ForEachFuncCall visits every call expression inside a function declaration of
// file, passing the enclosing declaration's name alongside each call. Calls
// nested in arguments, closures and inner function literals are visited too, all
// reported under the top-level declaration holding them. Returning false ends
// the whole walk, not just the current declaration.
//
// A declaration with no body yields nothing rather than panicking, so a caller
// needs no nil check of its own.
func ForEachFuncCall(file *ast.File, visit func(funcName string, call *ast.CallExpr) bool) {
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc {
			continue
		}
		stopped := false
		ast.Inspect(fn, func(n ast.Node) bool {
			if stopped {
				return false
			}
			call, isCall := n.(*ast.CallExpr)
			if !isCall {
				return true
			}
			if !visit(fn.Name.Name, call) {
				stopped = true
				return false
			}
			return true
		})
		if stopped {
			return
		}
	}
}
