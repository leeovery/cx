package tmux_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The packages that compose a tmux `-t` argument. internal/tmux holds the
// vocabulary; internal/session composes an exec chain the client never runs,
// and internal/restore composes the skeleton target it hands the client.
var targetComposingPackages = []string{".", "../session", "../restore"}

// exactTargetHelpers is the whole vocabulary a composed target may be drawn
// from. Their bare siblings (PaneTarget, windowTarget) are absent on purpose:
// tmux prefix-matches an unpinned session name, so a target built from one
// reaches a live stranger whose name merely starts the same way.
var exactTargetHelpers = map[string]bool{
	"ExactSessionTarget": true,
	"ExactCoordTarget":   true,
	"windowTargetExact":  true,
	"PaneTargetExact":    true,
}

// passThroughTargetParams names the parameters that arrive already composed, so
// their provenance is the caller's, which this scan checks wherever that caller
// sits in one of the packages above.
// The allow-list is deliberately narrow rather than "any parameter": a method
// taking a session *name* must still pin it, which is what makes a bare
// `-t name` a finding rather than a pass-through. Widening it is the point at
// which someone has to justify a new unexamined target.
//
// It does double duty. It exempts these parameters where they are spent on an
// argv, and it is also how a method that takes an already-composed target is
// recognised, so its call sites are held to the same rule.
var passThroughTargetParams = map[string]bool{
	"target": true,
	"paneID": true,
}

// TestTmuxTargetsAreComposedThroughTheExactnessVocabulary is the standing guard:
// the exactness rule has been rediscovered a call site at a time, so it is
// enforced here rather than left to whoever writes the next one.
func TestTmuxTargetsAreComposedThroughTheExactnessVocabulary(t *testing.T) {
	findings, err := scanBareTargets(targetComposingPackages)
	if err != nil {
		t.Fatalf("scan for bare targets: %v", err)
	}
	for _, finding := range findings {
		t.Errorf("%s: %s", finding.pos, finding.detail)
	}
}

func TestBareTargetGuard_FlagsAPackageComposingABareTarget(t *testing.T) {
	t.Run("it fails a package composing a bare -t target", func(t *testing.T) {
		dir := writeFixturePackage(t, `package fixture

import "fmt"

type Client struct{}

func (c *Client) SplitWindow(target, cwd string) error { return nil }

func run(args ...string) {}

var stagedArgs = []string{"kill-session", "-t", "some-name"}

func addressSession(c *Client, name string) {
	bare := name + "-2"
	run("has-session", "-t", bare)
	run("kill-session", "-t", fmt.Sprintf("%s:", name))
	args := []string{"new-window", "-t", name + ":"}
	run(args...)
	_ = c.SplitWindow(name+":", "/tmp")
}
`)

		findings, err := scanBareTargets([]string{dir})
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if len(findings) != 5 {
			t.Fatalf("scan found %d bare targets, want 5: %v", len(findings), findings)
		}
	})

	t.Run("it passes a package composing every target through the vocabulary", func(t *testing.T) {
		dir := writeFixturePackage(t, `package fixture

type Client struct{}

func (c *Client) SplitWindow(target, cwd string) error { return nil }

func (c *Client) SendKeys(target string) { run("send-keys", "-t", target) }

func run(args ...string) {}

func ExactCoordTarget(session string) string { return "=" + session + ":" }

func addressSession(c *Client, name string) {
	run("has-session", "-t", ExactCoordTarget(name))
	target := ExactCoordTarget(name)
	run("kill-session", "-t", target)
	args := []string{"split-window", "-t", target}
	run(args...)
	_ = c.SplitWindow(target, "/tmp")
	_ = c.SplitWindow(ExactCoordTarget(name), "/tmp")
}
`)

		findings, err := scanBareTargets([]string{dir})
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("scan flagged %v, want nothing", findings)
		}
	})
}

// A guard that stopped finding sources would otherwise report a clean scan
// forever.
func TestBareTargetGuard_ErrorsWhenItEnumeratesNoFiles(t *testing.T) {
	if _, err := scanBareTargets([]string{t.TempDir()}); err == nil {
		t.Fatal("scan of a directory holding no sources succeeded, want an error")
	}
}

// The same reasoning one rule down: the argument rule is only as wide as the
// method set it derives, so an empty set is a scan that has stopped looking
// rather than a clean one.
func TestBareTargetGuard_ErrorsWhenItFindsNoTargetTakingMethods(t *testing.T) {
	dir := writeFixturePackage(t, `package fixture

func run(args ...string) {}

func addressSession(session string) { run("has-session", session) }
`)

	if _, err := scanBareTargets([]string{dir}); err == nil {
		t.Fatal("scan of a package declaring no target-taking method succeeded, want an error")
	}
}

func writeFixturePackage(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture package: %v", err)
	}
	return dir
}

type bareTargetFinding struct {
	pos    string
	detail string
}

func (f bareTargetFinding) String() string { return f.pos + ": " + f.detail }

const routeItThrough = "route it through ExactSessionTarget, ExactCoordTarget, windowTargetExact or PaneTargetExact"

// scanBareTargets reports every place in dirs where a tmux target the exactness
// vocabulary did not produce is spent: composed into an argv after a literal
// "-t", or passed to a method that takes an already-composed target. The second
// rule is what reaches a target composed in one package and flagged with "-t" in
// another, which is the shape a hand-built "<session>:" takes on its way to
// SplitWindow.
func scanBareTargets(dirs []string) ([]bareTargetFinding, error) {
	fset := token.NewFileSet()
	var files []*ast.File
	for _, dir := range dirs {
		paths, err := sourceguardtest.PackageGoFiles(dir, false)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		}
	}

	methods := targetTakingMethods(files)
	if len(methods) == 0 {
		return nil, errors.New("no method taking an already-composed target was found, so nothing would be checked at a call site")
	}

	var findings []bareTargetFinding
	for _, file := range files {
		findings = append(findings, scanFileForBareTargets(fset, file, methods)...)
	}
	return findings, nil
}

// targetTakingMethods records, per method name, the argument positions holding
// an already-composed target. Every method is read, not only the tmux client's:
// a parameter named for a target is one wherever it is declared, and a wider
// map only ever checks more call sites.
func targetTakingMethods(files []*ast.File) map[string][]int {
	methods := map[string][]int{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Recv == nil {
				continue
			}
			if positions := targetParamPositions(fn); len(positions) > 0 {
				methods[fn.Name.Name] = positions
			}
		}
	}
	return methods
}

func targetParamPositions(fn *ast.FuncDecl) []int {
	var positions []int
	pos := 0
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if passThroughTargetParams[name.Name] {
				positions = append(positions, pos)
			}
			pos++
		}
	}
	return positions
}

func scanFileForBareTargets(fset *token.FileSet, file *ast.File, methods map[string][]int) []bareTargetFinding {
	var findings []bareTargetFinding
	check := func(pos token.Pos, call *ast.CallExpr, elems []ast.Expr) {
		bound := map[string]bool{}
		if decl := enclosingFunc(file, pos); decl != nil {
			bound = boundTargets(decl)
		}
		findings = append(findings, bareTargetsIn(fset, elems, bound)...)
		if call != nil {
			findings = append(findings, bareTargetArguments(fset, call, methods, bound)...)
		}
	}

	sourceguardtest.ForEachFuncCall(file, func(_ string, call *ast.CallExpr) bool {
		check(call.Pos(), call, call.Args)
		return true
	})
	// ForEachFuncCall descends into function declarations, and a composed argv
	// slice is not a call in the first place. Both remaining shapes are read
	// here rather than skipped: a guard that declines to look at a node class is
	// the failure it exists to prevent.
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			check(node.Pos(), nil, node.Elts)
		case *ast.CallExpr:
			if enclosingFunc(file, node.Pos()) == nil {
				check(node.Pos(), node, node.Args)
			}
		}
		return true
	})
	return findings
}

// bareTargetsIn reports each "-t" in elems whose following element is neither a
// call to the vocabulary nor an identifier bound to one.
func bareTargetsIn(fset *token.FileSet, elems []ast.Expr, bound map[string]bool) []bareTargetFinding {
	var findings []bareTargetFinding
	for i, elem := range elems {
		if !isStringLit(elem, "-t") {
			continue
		}
		if i+1 == len(elems) {
			findings = append(findings, bareTargetFinding{
				pos:    fset.Position(elem.Pos()).String(),
				detail: `"-t" ends its argv, so its target is composed apart from the flag`,
			})
			continue
		}
		if targetIsExact(elems[i+1], bound) {
			continue
		}
		findings = append(findings, bareTargetFinding{
			pos:    fset.Position(elems[i+1].Pos()).String(),
			detail: `the target after "-t" is composed by hand — ` + routeItThrough,
		})
	}
	return findings
}

// bareTargetArguments reports each argument handed to a target-taking method
// that the vocabulary did not produce.
func bareTargetArguments(fset *token.FileSet, call *ast.CallExpr, methods map[string][]int, bound map[string]bool) []bareTargetFinding {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return nil
	}
	var findings []bareTargetFinding
	for _, pos := range methods[sel.Sel.Name] {
		if pos >= len(call.Args) || targetIsExact(call.Args[pos], bound) {
			continue
		}
		findings = append(findings, bareTargetFinding{
			pos:    fset.Position(call.Args[pos].Pos()).String(),
			detail: "the target passed to " + sel.Sel.Name + " is composed by hand — " + routeItThrough,
		})
	}
	return findings
}

func targetIsExact(target ast.Expr, bound map[string]bool) bool {
	if ident, isIdent := target.(*ast.Ident); isIdent {
		return bound[ident.Name]
	}
	return isExactTargetCall(target)
}

// boundTargets names the identifiers the declaration may spend as a target: its
// pass-through parameters, its closures' pass-through parameters, and its locals
// assigned from the vocabulary.
func boundTargets(decl *ast.FuncDecl) map[string]bool {
	bound := map[string]bool{}
	bindParams(bound, decl.Type)
	ast.Inspect(decl, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncLit:
			bindParams(bound, node.Type)
		case *ast.AssignStmt:
			bindVocabularyAssignments(bound, node)
		}
		return true
	})
	return bound
}

func bindParams(bound map[string]bool, sig *ast.FuncType) {
	for _, field := range sig.Params.List {
		for _, name := range field.Names {
			if passThroughTargetParams[name.Name] {
				bound[name.Name] = true
			}
		}
	}
}

func bindVocabularyAssignments(bound map[string]bool, assign *ast.AssignStmt) {
	if len(assign.Lhs) != len(assign.Rhs) {
		return
	}
	for i, rhs := range assign.Rhs {
		ident, isIdent := assign.Lhs[i].(*ast.Ident)
		if isIdent && isExactTargetCall(rhs) {
			bound[ident.Name] = true
		}
	}
}

func isExactTargetCall(expr ast.Expr) bool {
	call, isCall := expr.(*ast.CallExpr)
	if !isCall {
		return false
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return exactTargetHelpers[fun.Name]
	case *ast.SelectorExpr:
		return exactTargetHelpers[fun.Sel.Name]
	}
	return false
}

func enclosingFunc(file *ast.File, pos token.Pos) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if isFunc && fn.Pos() <= pos && pos < fn.End() {
			return fn
		}
	}
	return nil
}

func isStringLit(expr ast.Expr, want string) bool {
	lit, isLit := expr.(*ast.BasicLit)
	return isLit && lit.Kind == token.STRING && lit.Value == `"`+want+`"`
}
