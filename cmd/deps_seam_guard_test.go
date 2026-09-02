package cmd

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

// Every package-level *Deps seam outlives the test that installs it, so an
// install without its paired restore answers for every later test in the
// package — and cmd's TestMain poison is the only other line of defence. The
// staging helpers are the only place the install and the restore are written
// together; a bare assignment anywhere else can drop the restore, or register it
// somewhere a neighbouring cleanup runs between.
//
// The seam set is not listed here. It is read out of the package's own
// production sources, so a seam declared tomorrow is guarded the day it appears
// rather than the day someone remembers to extend this list.
const depsSeamHelperFile = "testhelpers_test.go"

func TestSeamsInstalledOnlyThroughTheStagingHelpers(t *testing.T) {
	paths, err := sourceguardtest.PackageGoFiles(".", true)
	if err != nil {
		t.Fatalf("enumerate the cmd package sources: %v", err)
	}
	runSeamAssignmentGuard(t, paths, declaredSeams(t), depsSeamHelperFile)
}

// runSeamAssignmentGuard reports every assignment to one of seams made by a test
// source outside helperFile, and fails outright when it scanned nothing: a guard
// that enumerated no files would otherwise pass by having stopped looking.
//
// It takes the TestingT subset rather than *testing.T so its own failure paths
// are exercisable.
func runSeamAssignmentGuard(t harnesstest.TestingT, paths []string, seams []string, helperFile string) {
	t.Helper()
	var scanPaths []string
	for _, path := range paths {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") && base != helperFile {
			scanPaths = append(scanPaths, path)
		}
	}
	for _, source := range sourceguardtest.ParseSources(t, scanPaths) {
		base := filepath.Base(source.Path)
		for _, found := range seamAssignments(source.File, seams) {
			t.Errorf("%s:%d assigns the package-level %s seam directly; install it through with%s%s in %s so the restore cannot be dropped",
				base, source.Fset.Position(found.pos).Line, found.name,
				strings.ToUpper(found.name[:1]), found.name[1:], helperFile)
		}
	}
}

type seamAssignment struct {
	name string
	pos  token.Pos
}

// seamAssignments returns every assignment in file whose target is one of the
// named seam identifiers, in source order.
func seamAssignments(file *ast.File, seams []string) []seamAssignment {
	var found []seamAssignment
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, target := range assign.Lhs {
			ident, isIdent := target.(*ast.Ident)
			if isIdent && slices.Contains(seams, ident.Name) {
				found = append(found, seamAssignment{name: ident.Name, pos: ident.Pos()})
			}
		}
		return true
	})
	return found
}

// declaredSeams returns the identifiers of the package-level `var xDeps *XDeps`
// test seams the cmd production sources declare, in name order. It is fatal for
// the set to be empty: the guard's whole subject would otherwise vanish.
func declaredSeams(t *testing.T) []string {
	t.Helper()
	var seams []string
	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		for _, decl := range source.File.Decls {
			gen, isGen := decl.(*ast.GenDecl)
			if !isGen || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, isValue := spec.(*ast.ValueSpec)
				if !isValue {
					continue
				}
				star, isStar := value.Type.(*ast.StarExpr)
				if !isStar {
					continue
				}
				pointee, isIdent := star.X.(*ast.Ident)
				if !isIdent || !strings.HasSuffix(pointee.Name, "Deps") {
					continue
				}
				for _, name := range value.Names {
					if strings.HasSuffix(name.Name, "Deps") {
						seams = append(seams, name.Name)
					}
				}
			}
		}
	}
	if len(seams) == 0 {
		t.Fatal("no package-level *Deps seams found in the cmd production sources; the guard has lost its subject")
	}
	slices.Sort(seams)
	return seams
}

func TestSeamGuard(t *testing.T) {
	t.Run("it covers every declared seam identifier", func(t *testing.T) {
		declared := declaredSeams(t)

		var staged []string
		for _, sc := range seamStagingCases() {
			staged = append(staged, sc.name)
		}
		slices.Sort(staged)

		if !slices.Equal(declared, staged) {
			t.Errorf("the declared seams %v are not the seams staged through a helper %v; every seam needs an install helper, and the guard's coverage is the staging table", declared, staged)
		}
	})

	t.Run("it fails a test file assigning a seam outside its helper", func(t *testing.T) {
		// Demonstrated per identifier: one seam slipping out of the guard's
		// vocabulary is exactly the regression this test exists to catch.
		for _, seam := range declaredSeams(t) {
			t.Run(seam, func(t *testing.T) {
				path := writeSeamFixture(t, "offender_test.go", fmt.Sprintf(
					"package cmd\n\nfunc TestOffender() {\n\t%s = nil\n}\n", seam))

				failed, msg := captureSeamGuardFailure(func(sub harnesstest.TestingT) {
					runSeamAssignmentGuard(sub, []string{path}, declaredSeams(t), depsSeamHelperFile)
				})

				if !failed {
					t.Fatalf("the guard passed a test file assigning %s directly", seam)
				}
				if !strings.Contains(msg, seam) {
					t.Errorf("the guard's complaint %q does not name the %s seam it flagged", msg, seam)
				}
			})
		}
	})

	t.Run("it passes a test file installing every seam through its helper", func(t *testing.T) {
		var body strings.Builder
		body.WriteString("package cmd\n\nfunc TestWellBehaved() {\n")
		for _, seam := range declaredSeams(t) {
			fmt.Fprintf(&body, "\twith%s%s(t, %s%sDeps{})\n",
				strings.ToUpper(seam[:1]), seam[1:], strings.ToUpper(seam[:1]), strings.TrimSuffix(seam[1:], "Deps"))
		}
		body.WriteString("}\n")
		path := writeSeamFixture(t, "wellbehaved_test.go", body.String())

		failed, msg := captureSeamGuardFailure(func(sub harnesstest.TestingT) {
			runSeamAssignmentGuard(sub, []string{path}, declaredSeams(t), depsSeamHelperFile)
		})
		if failed {
			t.Errorf("the guard flagged a file that installs every seam through its helper: %s", msg)
		}
	})

	t.Run("it fatals when the guard scans no files", func(t *testing.T) {
		for name, paths := range map[string][]string{
			"no paths at all":           nil,
			"no test source among them": {"root.go", "open.go"},
			"only the helper file":      {depsSeamHelperFile},
		} {
			t.Run(name, func(t *testing.T) {
				failed, msg := captureSeamGuardFailure(func(sub harnesstest.TestingT) {
					runSeamAssignmentGuard(sub, paths, declaredSeams(t), depsSeamHelperFile)
				})
				if !failed {
					t.Fatal("the guard passed having scanned nothing")
				}
				if !strings.Contains(msg, "stopped looking") {
					t.Errorf("the guard's complaint %q does not say it scanned nothing", msg)
				}
			})
		}
	})
}

// writeSeamFixture stages a source fixture the guard can be pointed at, named so
// the guard's own filename rule applies to it.
func writeSeamFixture(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("stage the guard fixture %s: %v", name, err)
	}
	return path
}

// captureSeamGuardFailure runs fn against the shared stand-in, which absorbs
// the abort a Fatalf stands for, and reports whether the guard complained and
// with what.
func captureSeamGuardFailure(fn func(harnesstest.TestingT)) (failed bool, msg string) {
	rec := &harnesstest.Recorder{}
	rec.Run(func() { fn(rec) })
	return rec.Failed(), rec.Report()
}
