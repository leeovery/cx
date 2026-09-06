package tmux_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

const (
	tmuxImportPath = "github.com/leeovery/portal/internal/tmux"
	cmdImportPath  = "github.com/leeovery/portal/cmd"
)

// targetComposingPackages returns the directory of every package that can
// compose a tmux `-t` argument, sorted so a scan's findings come out in a
// deterministic order.
func targetComposingPackages(t *testing.T) []string {
	t.Helper()
	return slices.Sorted(maps.Values(targetComposingPackageDirs(t)))
}

// targetComposingPackageDirs maps the import path of every package that can
// compose a tmux `-t` argument to its directory: those importing internal/tmux,
// plus internal/tmux itself, which holds the vocabulary. The set is derived from
// the import graph rather than listed, so a package that starts addressing tmux
// joins the scan by construction instead of by someone remembering to add it.
func targetComposingPackageDirs(t *testing.T) map[string]string {
	t.Helper()

	listing, err := modulePackageListing()
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := importersOfTmux(listing)
	if err != nil {
		t.Fatal(err)
	}
	return dirs
}

// modulePackageListing reads the module's packages once per test binary: the
// listing is the same for every caller, and `go list` over the whole module is
// the expensive part of this guard.
//
// The scan parses every non-test source in a package whatever tag gates it, so
// the importer set is resolved with the integration tag in force too: a package
// reaching tmux only from a tagged file composes targets the scan would
// otherwise read from a directory it never visited.
var modulePackageListing = sync.OnceValues(func() (string, error) {
	out, err := exec.Command("go", "list",
		"-tags", "integration",
		"-f", "{{.ImportPath}}\t{{.Dir}}\t{{join .Imports \" \"}}",
		"github.com/leeovery/portal/...",
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go list the module's packages: %w\n%s", err, out)
	}
	return string(out), nil
})

// importersOfTmux reads a `go list` listing of "import path, directory, imports"
// lines into the import path → directory map of the packages addressing tmux.
// Resolving none is an error rather than an empty map: a scan that has stopped
// looking would otherwise report a clean repository forever, which is the
// failure the derivation exists to prevent.
func importersOfTmux(listing string) (map[string]string, error) {
	dirs := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(listing), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			continue
		}
		importPath, dir, imports := fields[0], fields[1], strings.Fields(fields[2])
		if importPath == tmuxImportPath || slices.Contains(imports, tmuxImportPath) {
			dirs[importPath] = dir
		}
	}
	if len(dirs) == 0 {
		return nil, errors.New("no package importing internal/tmux was found, so the guard would scan nothing")
	}
	return dirs, nil
}

// exactTargetHelpers is the whole vocabulary a composed target may be drawn
// from. Their bare siblings (PaneTarget, windowTarget) are absent on purpose:
// tmux prefix-matches an unpinned session name, so a target built from one
// reaches a live stranger whose name merely starts the same way.
var exactTargetHelpers = map[string]bool{
	"SessionTargetExact": true,
	"CoordTargetExact":   true,
	"windowTargetExact":  true,
	"PaneTargetExact":    true,
}

// passThroughTargetParams names the parameters and named results that hold an
// already-composed target. A parameter's provenance is its caller's, which this
// scan checks wherever that caller sits in one of the derived packages; a named
// result's provenance is the declaring body itself, which the scan does not
// check.
// The allow-list is deliberately narrow rather than "any parameter": a function
// taking a session *name* must still pin it, which is what makes a bare
// `-t name` a finding rather than a pass-through. Widening it is the point at
// which someone has to justify a new unexamined target.
//
// It does double duty. It exempts these parameters where they are spent on an
// argv, and it is also how a function that takes an already-composed target is
// recognised, so its call sites are held to the same rule. Recognition reads function
// declarations alone, so a parameter declared by a function value is exempt
// where it is spent without that value's call sites being checked.
var passThroughTargetParams = map[string]bool{
	"target": true,
	"paneID": true,
}

// TestTmuxTargetsAreComposedThroughTheExactnessVocabulary is the standing guard:
// the exactness rule has been rediscovered a call site at a time, so it is
// enforced here rather than left to whoever writes the next one.
func TestTmuxTargetsAreComposedThroughTheExactnessVocabulary(t *testing.T) {
	findings := scanBareTargets(t, targetComposingPackages(t))
	for _, finding := range findings {
		t.Errorf("%s: %s", finding.pos, finding.detail)
	}
}

func TestBareTargetGuard_ScansEveryPackageAddressingTmux(t *testing.T) {
	t.Run("it derives the scanned package set from the packages importing internal/tmux", func(t *testing.T) {
		byPath := targetComposingPackageDirs(t)

		for _, want := range []string{cmdImportPath, tmuxImportPath, "github.com/leeovery/portal/internal/restore"} {
			if _, ok := byPath[want]; !ok {
				t.Errorf("derived package set %v holds no entry for %s", slices.Sorted(maps.Keys(byPath)), want)
			}
		}
	})

	t.Run("it flags a bare target composed in cmd", func(t *testing.T) {
		dirs := targetComposingPackages(t)
		cmdDir := targetComposingPackageDirs(t)[cmdImportPath]
		probed := stagePackageWithProbe(t, cmdDir, `package cmd

func probeRun(args ...string) {}

func probeAddressSession(name string) { probeRun("has-session", "-t", name) }
`)
		cmdAt := slices.Index(dirs, cmdDir)
		if cmdAt < 0 {
			t.Fatalf("the derived package set %v holds no cmd directory to stage a probe over", dirs)
		}
		dirs[cmdAt] = probed

		findings := scanBareTargets(t, dirs)
		if len(findings) != 1 {
			t.Fatalf("scan of the derived set with one bare target staged in cmd found %d findings, want 1: %v",
				len(findings), findings)
		}
		if !strings.Contains(findings[0].pos, probeFileName) {
			t.Errorf("finding %v does not name the staged probe", findings[0])
		}
	})

	t.Run("it flags a bare target spent through a non-method helper in cmd", func(t *testing.T) {
		dirs := targetComposingPackages(t)
		cmdDir := targetComposingPackageDirs(t)[cmdImportPath]
		probed := stagePackageWithProbe(t, cmdDir, `package cmd

func probeRun(args ...string) {}

func probeAddressSession(target string) { probeRun("has-session", "-t", target) }

func probeSpendSession(name string) { probeAddressSession(name + ":") }
`)
		cmdAt := slices.Index(dirs, cmdDir)
		if cmdAt < 0 {
			t.Fatalf("the derived package set %v holds no cmd directory to stage a probe over", dirs)
		}
		dirs[cmdAt] = probed

		findings := scanBareTargets(t, dirs)
		if len(findings) != 1 {
			t.Fatalf("scan of the derived set with one helper-spent bare target staged in cmd found %d findings, want 1: %v",
				len(findings), findings)
		}
		if !strings.Contains(findings[0].pos, probeFileName) {
			t.Errorf("finding %v does not name the staged probe", findings[0])
		}
	})
}

const probeFileName = "zz_probe.go"

// stagePackageWithProbe copies src's production sources into a temp directory
// and adds probe alongside them, so the scan reads the package's real sources
// with one authored offender among them. The copy is what keeps the repository
// untouched: nothing is ever written into src.
func stagePackageWithProbe(t *testing.T, src, probe string) string {
	t.Helper()
	paths, err := sourceguardtest.PackageGoFiles(src, false)
	if err != nil {
		t.Fatalf("enumerate %s: %v", src, err)
	}
	dir := t.TempDir()
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(path)), body, 0o600); err != nil {
			t.Fatalf("stage %s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, probeFileName), []byte(probe), 0o600); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	return dir
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

		findings := scanBareTargets(t, []string{dir})
		if len(findings) != 5 {
			t.Fatalf("scan found %d bare targets, want 5: %v", len(findings), findings)
		}
	})

	t.Run("it flags a bare target composed in a non-method helper", func(t *testing.T) {
		dir := writeFixturePackage(t, `package fixture

type Client struct{}

func (c *Client) SelectPane(target string) { run("select-pane", "-t", target) }

func run(args ...string) {}

func addressSession(target string) { run("has-session", "-t", target) }

func spendSession(name string) { addressSession(name + ":") }
`)

		findings := scanBareTargets(t, []string{dir})
		if len(findings) != 1 {
			t.Fatalf("scan of a bare target spent through a non-method helper found %d findings, want 1: %v",
				len(findings), findings)
		}
		if findings[0].detail != "the target passed to addressSession is composed by hand — "+routeItThrough {
			t.Errorf("finding %v does not name the helper the bare target was passed to", findings[0])
		}
	})

	t.Run("it flags a target split across the end of an argv", func(t *testing.T) {
		dir := writeFixturePackage(t, `package fixture

type Client struct{}

func (c *Client) SelectPane(target string) { run("select-pane", "-t", target) }

func run(args ...string) {}

func addressSession(name string) {
	args := []string{"kill-session", "-t"}
	run(append(args, name)...)
}
`)

		findings := scanBareTargets(t, []string{dir})
		if len(findings) != 1 {
			t.Fatalf("scan of an argv ending in \"-t\" found %d findings, want 1: %v", len(findings), findings)
		}
		if findings[0].detail != `"-t" ends its argv, so its target is composed apart from the flag` {
			t.Errorf("finding %v does not report the target as composed apart from its flag", findings[0])
		}
	})

	t.Run("it flags every position two same-named target takers declare", func(t *testing.T) {
		dir := writeFixturePackage(t, `package fixture

type Client struct{}

func (c *Client) stampPaneToken(session, paneKey, target, token string) {}

func stampPaneToken(paneID string) {}

func run(args ...string) {}

func addressSession(c *Client, target, name string) {
	stampPaneToken(name + ":")
	c.stampPaneToken(target, "key", name+":", "token")
}
`)

		findings := scanBareTargets(t, []string{dir})
		if len(findings) != 2 {
			t.Fatalf("scan of two same-named takers each handed a bare target found %d findings, want 2: %v",
				len(findings), findings)
		}
	})

	t.Run("it flags a hand-composed target assigned to a local of a pass-through name", func(t *testing.T) {
		dir := writeFixturePackage(t, `package fixture

type Client struct{}

func (c *Client) SelectPane(target string) { run("select-pane", "-t", target) }

func run(args ...string) {}

func addressSession(name string) {
	target := name + ":"
	run("has-session", "-t", target)
}
`)

		findings := scanBareTargets(t, []string{dir})
		if len(findings) != 1 {
			t.Fatalf("scan of a hand-composed target laundered through a local found %d findings, want 1: %v",
				len(findings), findings)
		}
		if findings[0].detail != `the target after "-t" is composed by hand — `+routeItThrough {
			t.Errorf("finding %v does not report the target as composed by hand", findings[0])
		}
	})

	t.Run("it passes a target held by a local of the vocabulary's own name", func(t *testing.T) {
		dir := writeFixturePackage(t, `package fixture

type Client struct{}

func (c *Client) SelectPane(target string) { run("select-pane", "-t", target) }

func run(args ...string) {}

func resolveCurrentPane() (string, string, error) { return "", "", nil }

var addressSession = &struct{ RunE func() error }{
	RunE: func() error {
		key, target, err := resolveCurrentPane()
		if err != nil {
			return err
		}
		_ = key
		run("has-session", "-t", target)
		return nil
	},
}
`)

		findings := scanBareTargets(t, []string{dir})
		if len(findings) != 0 {
			t.Errorf("scan flagged %v, want nothing", findings)
		}
	})

	t.Run("it passes a package composing every target through the vocabulary", func(t *testing.T) {
		dir := writeFixturePackage(t, `package fixture

type Client struct{}

func (c *Client) SplitWindow(target, cwd string) error { return nil }

func (c *Client) SelectPane(target string) { run("select-pane", "-t", target) }

func run(args ...string) {}

func CoordTargetExact(session string) string { return "=" + session + ":" }

func addressSession(c *Client, name string) {
	run("has-session", "-t", CoordTargetExact(name))
	target := CoordTargetExact(name)
	run("kill-session", "-t", target)
	args := []string{"split-window", "-t", target}
	run(args...)
	_ = c.SplitWindow(target, "/tmp")
	_ = c.SplitWindow(CoordTargetExact(name), "/tmp")
}
`)

		findings := scanBareTargets(t, []string{dir})
		if len(findings) != 0 {
			t.Errorf("scan flagged %v, want nothing", findings)
		}
	})
}

// The same reasoning one rule up: an import scan resolving no importer is a
// derivation that has stopped looking, not a repository that has stopped
// composing targets.
func TestBareTargetGuard_ErrorsWhenTheImportScanResolvesNothing(t *testing.T) {
	listing := "github.com/leeovery/portal/internal/alias\t/repo/internal/alias\tfmt os\n"

	if _, err := importersOfTmux(listing); err == nil {
		t.Fatal("an import scan resolving no importer of internal/tmux succeeded, want an error")
	}
}

// A guard that stopped finding sources would otherwise report a clean scan
// forever.
func TestBareTargetGuard_FatalsWhenItEnumeratesNoFiles(t *testing.T) {
	rec := &harnesstest.Recorder{}

	rec.Run(func() { scanBareTargets(rec, []string{t.TempDir()}) })

	if len(rec.Fatals) != 1 {
		t.Fatalf("scan of a directory holding no sources reported %d fatals, want 1: %v", len(rec.Fatals), rec.Fatals)
	}
	if !strings.Contains(rec.Fatals[0], "no .go files") {
		t.Errorf("fatal message %q does not say the directory held no sources", rec.Fatals[0])
	}
}

// The same reasoning one rule down: the argument rule is only as wide as the
// function set it derives, so an empty set is a scan that has stopped looking
// rather than a clean one.
func TestBareTargetGuard_FatalsWhenItFindsNoTargetTakingFuncs(t *testing.T) {
	dir := writeFixturePackage(t, `package fixture

func run(args ...string) {}

func addressSession(session string) { run("has-session", session) }
`)

	rec := &harnesstest.Recorder{}

	rec.Run(func() { scanBareTargets(rec, []string{dir}) })

	if len(rec.Fatals) != 1 {
		t.Fatalf("scan of a package declaring no target-taking function reported %d fatals, want 1: %v", len(rec.Fatals), rec.Fatals)
	}
	if !strings.Contains(rec.Fatals[0], "already-composed target") {
		t.Errorf("fatal message %q does not say the function set was empty", rec.Fatals[0])
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

const routeItThrough = "route it through SessionTargetExact, CoordTargetExact, windowTargetExact or PaneTargetExact"

// scanBareTargets reports every place in dirs where a tmux target the exactness
// vocabulary did not produce is spent: composed into an argv after a literal
// "-t", or passed to a function that takes an already-composed target. The second
// rule is what reaches a target composed in one package and flagged with "-t" in
// another, which is the shape a hand-built "<session>:" takes on its way to
// SplitWindow.
func scanBareTargets(t harnesstest.TestingT, dirs []string) []bareTargetFinding {
	t.Helper()

	var sources []sourceguardtest.ParsedSource
	for _, dir := range dirs {
		sources = append(sources, sourceguardtest.ParsePackageSources(t, dir, false)...)
	}
	if len(sources) == 0 {
		return nil
	}

	files := make([]*ast.File, 0, len(sources))
	for _, source := range sources {
		files = append(files, source.File)
	}

	takers := targetTakingFuncs(files)
	if len(takers) == 0 {
		t.Fatalf("no function taking an already-composed target was found, so nothing would be checked at a call site")
		return nil
	}

	var findings []bareTargetFinding
	for _, source := range sources {
		findings = append(findings, scanFileForBareTargets(source.Fset, source.File, takers)...)
	}
	return findings
}

// targetTakingFuncs records, per function name, the argument positions holding
// an already-composed target. Methods and plain functions alike are read, and
// not only the tmux client's: a parameter named for a target is one wherever it
// is declared, and a wider map only ever checks more call sites. Reading both narrows the gap
// between what the rule exempts and what it recognises — the parameter is exempt
// where it is spent in any function, so a function declaring it is checked at its
// call sites too, or a bare target reaches tmux through a helper and is never
// seen.
//
// A call site is matched on the callee's name alone, so two declarations sharing
// a name — a method and a plain function, say — are one entry, and their
// positions are unioned rather than overwritten: keeping only the last read
// would drop the other's positions out of the rule while leaving its parameter
// exempt, which is the asymmetry above by another route. The pooling is why an
// unrelated function sharing a taker's name has its own arguments checked at
// those positions; give the two operations distinct names rather than widening
// the vocabulary to absorb the report.
func targetTakingFuncs(files []*ast.File) map[string][]int {
	takers := map[string][]int{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc {
				continue
			}
			if positions := targetParamPositions(fn); len(positions) > 0 {
				takers[fn.Name.Name] = unionPositions(takers[fn.Name.Name], positions)
			}
		}
	}
	return takers
}

// unionPositions merges two position sets, deduped and ascending, so a name's
// findings come out in a deterministic order whatever order the declarations
// were read in.
func unionPositions(a, b []int) []int {
	merged := slices.Concat(a, b)
	slices.Sort(merged)
	return slices.Compact(merged)
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

func scanFileForBareTargets(fset *token.FileSet, file *ast.File, takers map[string][]int) []bareTargetFinding {
	var findings []bareTargetFinding
	check := func(pos token.Pos, call *ast.CallExpr, elems []ast.Expr) {
		bound := map[string]bool{}
		if decl := enclosingDecl(file, pos); decl != nil {
			bound = boundTargets(decl)
		}
		findings = append(findings, bareTargetsIn(fset, elems, bound)...)
		if call != nil {
			findings = append(findings, bareTargetArguments(fset, call, takers, bound)...)
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

// bareTargetArguments reports each argument handed to a target-taking function
// that the vocabulary did not produce. The callee is read through CalleeName, so
// a plain function call reaches the rule as much as a method call does.
func bareTargetArguments(fset *token.FileSet, call *ast.CallExpr, takers map[string][]int, bound map[string]bool) []bareTargetFinding {
	callee := sourceguardtest.CalleeName(call)
	if callee == "" {
		return nil
	}
	var findings []bareTargetFinding
	for _, pos := range takers[callee] {
		if pos >= len(call.Args) || targetIsExact(call.Args[pos], bound) {
			continue
		}
		findings = append(findings, bareTargetFinding{
			pos:    fset.Position(call.Args[pos].Pos()).String(),
			detail: "the target passed to " + callee + " is composed by hand — " + routeItThrough,
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

// boundTargets names the identifiers the declaration may spend as a target: the
// pass-through names its signature declares, those its closures declare, and the
// locals it assigns that the rule reads as already-composed. A declaration of any
// kind is read, because a command body written as a function literal inside a
// package-level var declares its pass-throughs there rather than in a signature.
func boundTargets(decl ast.Decl) map[string]bool {
	bound := map[string]bool{}
	if fn, isFunc := decl.(*ast.FuncDecl); isFunc {
		bindSignature(bound, fn.Type)
	}
	ast.Inspect(decl, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncLit:
			bindSignature(bound, node.Type)
		case *ast.AssignStmt:
			bindAssignedTargets(bound, node)
		}
		return true
	})
	return bound
}

// A named result carries the same word as a parameter would and is declared in
// the same signature, so it is read the same way: a function returning a
// composed target names it in its own signature rather than in its caller's.
func bindSignature(bound map[string]bool, sig *ast.FuncType) {
	bindFields(bound, sig.Params)
	bindFields(bound, sig.Results)
}

func bindFields(bound map[string]bool, fields *ast.FieldList) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			if passThroughTargetParams[name.Name] {
				bound[name.Name] = true
			}
		}
	}
}

// bindAssignedTargets binds the identifiers an assignment declares that the rule
// reads as already-composed. Where the two sides pair up positionally, that is
// an identifier assigned straight from the vocabulary, whatever it is named.
// Where they do not — a multi-valued result, whose provenance no expression on
// the right states — a pass-through name is taken at its word, as it is in a
// signature.
//
// The name is deliberately not read on a paired assignment: there the right-hand
// side says what the target was composed from, so honouring the name would
// launder a hand-composed one through a rename.
func bindAssignedTargets(bound map[string]bool, assign *ast.AssignStmt) {
	paired := len(assign.Lhs) == len(assign.Rhs)
	for i, lhs := range assign.Lhs {
		ident, isIdent := lhs.(*ast.Ident)
		if !isIdent {
			continue
		}
		if paired && isExactTargetCall(assign.Rhs[i]) || !paired && passThroughTargetParams[ident.Name] {
			bound[ident.Name] = true
		}
	}
}

func isExactTargetCall(expr ast.Expr) bool {
	call, isCall := expr.(*ast.CallExpr)
	if !isCall {
		return false
	}
	return exactTargetHelpers[sourceguardtest.CalleeName(call)]
}

// enclosingDecl returns the top-level declaration holding pos, whatever kind it
// is; enclosingFunc answers the narrower question of whether a call has already
// been visited by the function walk.
func enclosingDecl(file *ast.File, pos token.Pos) ast.Decl {
	for _, decl := range file.Decls {
		if decl.Pos() <= pos && pos < decl.End() {
			return decl
		}
	}
	return nil
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
