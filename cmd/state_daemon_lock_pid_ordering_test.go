package cmd

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
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
	source := sourceguardtest.PackageSource(t, ".", stateDaemonSourcePath)

	fn := findFuncDecl(source.File, daemonRunFuncName)
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
			acquireIdx+1, acquireDaemonLockIdent, got, source.Position(got.Pos()).Line)
	}
	if !ifStmtIsErrGuard(errGuard) {
		t.Fatalf("statement at index %d (line %d) is an *ast.IfStmt but does not "+
			"match the err-guard shape (`if err != nil { ... return ... }`)",
			acquireIdx+1, source.Position(errGuard.Pos()).Line)
	}

	writePIDStmt := fn.Body.List[acquireIdx+2]
	writePIDIfStmt, ok := writePIDStmt.(*ast.IfStmt)
	if !ok {
		t.Fatalf("statement at index %d after err-guard is not an *ast.IfStmt; "+
			"got %T at line %d — WritePIDFile must be the next "+
			"statement after the acquire err-guard. Did a refactor insert "+
			"intermediate work between acquireDaemonLock's err-guard and WritePIDFile?",
			acquireIdx+2, writePIDStmt, source.Position(writePIDStmt.Pos()).Line)
	}
	if !ifStmtContainsCallTo(writePIDIfStmt, writePIDFileIdent) {
		t.Fatalf("statement at index %d (line %d) is an *ast.IfStmt but does not "+
			"contain a call to %q. The acquire err-guard must be "+
			"immediately followed by WritePIDFile.",
			acquireIdx+2, source.Position(writePIDIfStmt.Pos()).Line, writePIDFileIdent)
	}
}

func TestAcquireDaemonLock_SingleProductionCallSite(t *testing.T) {
	count := 0
	var locations []string
	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		name := source.Path
		ast.Inspect(source.File, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if callTargetMatches(call, acquireDaemonLockIdent) ||
				callSelectorMatches(call, "state", "AcquireDaemonLock") {
				count++
				locations = append(locations,
					name+":"+positionString(source.Fset, call.Pos()))
			}
			return true
		})
	}

	if count != 1 {
		t.Errorf("expected exactly 1 production call site to acquireDaemonLock / "+
			"state.AcquireDaemonLock in cmd/; got %d at: %v\n\n"+
			"There must be exactly one production call site, inside "+
			"defaultDaemonRun. A second caller would bypass the "+
			"acquire+WritePIDFile adjacency check enforced by "+
			"TestDaemonAcquireLockOrdering_WritePIDFollowsAcquire and "+
			"re-introduce the acquire/pid-write race it closes.",
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
		if sourceguardtest.CalleeName(call) == name {
			found = true
			return false
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
