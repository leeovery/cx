package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const daemonRunFuncName = "defaultDaemonRun"

const acquireDaemonLockIdent = "acquireDaemonLock"

// writePIDFileIdent is matched by name only, not by full selector, so both a
// bare WritePIDFile call and the state.WritePIDFile shape are tolerated.
const writePIDFileIdent = "WritePIDFile"

// stateDaemonSourcePath is hard-coded so a refactor that moves
// defaultDaemonRun elsewhere fails fast rather than silently finding nothing.
var stateDaemonSourcePath = filepath.Join("state_daemon.go")

func TestDaemonAcquireLockOrdering_WritePIDFollowsAcquire(t *testing.T) {
	src, err := os.ReadFile(stateDaemonSourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, stateDaemonSourcePath, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}

	fn := findFuncDecl(file, daemonRunFuncName)
	if fn == nil {
		t.Fatalf("function %q not found in %s — has it been renamed or moved?",
			daemonRunFuncName, stateDaemonSourcePath)
	}
	if fn.Body == nil {
		t.Fatalf("function %q has nil Body — cannot inspect statements", daemonRunFuncName)
	}

	acquireIdx := -1
	for i, stmt := range fn.Body.List {
		if isAssignCallTo(stmt, acquireDaemonLockIdent) {
			acquireIdx = i
			break
		}
	}
	if acquireIdx < 0 {
		t.Fatalf("no AssignStmt calling %q found in %s body",
			acquireDaemonLockIdent, daemonRunFuncName)
	}

	if acquireIdx+2 >= len(fn.Body.List) {
		t.Fatalf("body has insufficient statements after %q (idx=%d, len=%d) — "+
			"expected err-guard at i+1 and WritePIDFile if-stmt at i+2",
			acquireDaemonLockIdent, acquireIdx, len(fn.Body.List))
	}

	errGuard, ok := fn.Body.List[acquireIdx+1].(*ast.IfStmt)
	if !ok {
		got := fn.Body.List[acquireIdx+1]
		t.Fatalf("statement at index %d after %q is not an *ast.IfStmt; "+
			"got %T at line %d — the err-guard for the acquire call must be the "+
			"immediately-following statement",
			acquireIdx+1, acquireDaemonLockIdent, got, fset.Position(got.Pos()).Line)
	}
	if !ifStmtIsErrGuard(errGuard) {
		t.Fatalf("statement at index %d (line %d) is an *ast.IfStmt but does not "+
			"match the err-guard shape (`if err != nil { ... return ... }`)",
			acquireIdx+1, fset.Position(errGuard.Pos()).Line)
	}

	writePIDStmt := fn.Body.List[acquireIdx+2]
	writePIDIfStmt, ok := writePIDStmt.(*ast.IfStmt)
	if !ok {
		t.Fatalf("statement at index %d after err-guard is not an *ast.IfStmt; "+
			"got %T at line %d — the spec mandates WritePIDFile is the next "+
			"statement after the acquire err-guard. Did a refactor insert "+
			"intermediate work between acquireDaemonLock's err-guard and WritePIDFile?",
			acquireIdx+2, writePIDStmt, fset.Position(writePIDStmt.Pos()).Line)
	}
	if !ifStmtContainsCallTo(writePIDIfStmt, writePIDFileIdent) {
		t.Fatalf("statement at index %d (line %d) is an *ast.IfStmt but does not "+
			"contain a call to %q. The spec mandates the acquire err-guard be "+
			"immediately followed by WritePIDFile — see specification.md § "+
			"Component C step 4.",
			acquireIdx+2, fset.Position(writePIDIfStmt.Pos()).Line, writePIDFileIdent)
	}
}

func TestAcquireDaemonLock_SingleProductionCallSite(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read cmd dir: %v", err)
	}

	fset := token.NewFileSet()
	count := 0
	var locations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if callTargetMatches(call, acquireDaemonLockIdent) ||
				callSelectorMatches(call, "state", "AcquireDaemonLock") {
				count++
				locations = append(locations,
					name+":"+positionString(fset, call.Pos()))
			}
			return true
		})
	}

	if count != 1 {
		t.Errorf("expected exactly 1 production call site to acquireDaemonLock / "+
			"state.AcquireDaemonLock in cmd/; got %d at: %v\n\n"+
			"Spec § Component C step 4 mandates a single production call site "+
			"inside defaultDaemonRun. A second caller would bypass the "+
			"acquire+WritePIDFile adjacency check enforced by "+
			"TestDaemonAcquireLockOrdering_WritePIDFollowsAcquire and "+
			"re-introduce the race Component C closes.",
			count, locations)
	}
}

func findFuncDecl(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name != nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// isAssignCallTo reports whether stmt is `<lhs...> := <ident>(<args...>)`.
func isAssignCallTo(stmt ast.Stmt, name string) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok {
		return false
	}
	if len(assign.Rhs) != 1 {
		return false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	return callTargetMatches(call, name)
}

// callTargetMatches matches a bare ident only; selector expressions go through
// callSelectorMatches.
func callTargetMatches(call *ast.CallExpr, name string) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == name
}

func callSelectorMatches(call *ast.CallExpr, pkg, sel string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return x.Name == pkg && selector.Sel != nil && selector.Sel.Name == sel
}

// ifStmtIsErrGuard matches `if err != nil { ... return ... }` structurally,
// tolerating any content between the open brace and the return.
func ifStmtIsErrGuard(ifStmt *ast.IfStmt) bool {
	bin, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	leftIdent, leftOk := bin.X.(*ast.Ident)
	rightIdent, rightOk := bin.Y.(*ast.Ident)
	if !leftOk || !rightOk {
		return false
	}
	if leftIdent.Name == "" || rightIdent.Name != "nil" {
		return false
	}
	if ifStmt.Body == nil {
		return false
	}
	hasReturn := false
	ast.Inspect(ifStmt.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.ReturnStmt); ok {
			hasReturn = true
			return false
		}
		return true
	})
	return hasReturn
}

// ifStmtContainsCallTo searches init, cond and body for a call to a bare ident
// with the given name or a selector ending in it.
func ifStmtContainsCallTo(ifStmt *ast.IfStmt, name string) bool {
	found := false
	visit := func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if fun.Name == name {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			if fun.Sel != nil && fun.Sel.Name == name {
				found = true
				return false
			}
		}
		return true
	}
	if ifStmt.Init != nil {
		ast.Inspect(ifStmt.Init, visit)
	}
	if !found && ifStmt.Cond != nil {
		ast.Inspect(ifStmt.Cond, visit)
	}
	if !found && ifStmt.Body != nil {
		ast.Inspect(ifStmt.Body, visit)
	}
	return found
}

func positionString(fset *token.FileSet, pos token.Pos) string {
	p := fset.Position(pos)
	return strings.TrimPrefix(p.String(), p.Filename+":")
}
