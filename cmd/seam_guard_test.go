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

// Every package-level seam outlives the test that installs it, so an install
// without its paired restore answers for every later test in the package — and
// cmd's TestMain poison is the only other line of defence. The staging helpers
// are the only place the install and the restore are written together; a bare
// assignment anywhere else can drop the restore, or register it somewhere a
// neighbouring cleanup runs between.
//
// The seam set is not listed here. It is read out of the package's own
// production sources, so a seam declared tomorrow is guarded the day it appears
// rather than the day someone remembers to extend this list.
const (
	seamHelperFile = "testhelpers_test.go"

	// funcSeamHelper stages the whole function-var family, whose members differ
	// only in their signature — which the helper's type parameter carries.
	funcSeamHelper = "withFuncSeam"

	// testMainFunc installs seams package-wide and has no *testing.T to restore
	// into, so the staging helpers cannot serve it and the guard leaves it be.
	testMainFunc = "TestMain"
)

func TestSeamsInstalledOnlyThroughTheStagingHelpers(t *testing.T) {
	paths, err := sourceguardtest.PackageGoFiles(".", true)
	if err != nil {
		t.Fatalf("enumerate the cmd package sources: %v", err)
	}
	runSeamAssignmentGuard(t, paths, declaredSeams(t), seamHelperFile)
}

// runSeamAssignmentGuard reports every assignment to one of seams made by a test
// source outside helperFile, and fails outright when it scanned nothing: a guard
// that enumerated no files would otherwise pass by having stopped looking.
//
// It takes the TestingT subset rather than *testing.T so its own failure paths
// are exercisable.
func runSeamAssignmentGuard(t harnesstest.TestingT, paths []string, seams []seamDecl, helperFile string) {
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
			t.Errorf("%s:%d assigns the package-level %s seam directly; install it through %s in %s so the restore cannot be dropped",
				base, source.Fset.Position(found.pos).Line, found.name, found.helper, helperFile)
		}
	}
}

// seamDecl names one package-level seam alongside the staging helper a test has
// to install it through.
type seamDecl struct {
	name   string
	helper string
}

type seamAssignment struct {
	seamDecl
	pos token.Pos
}

// seamAssignments returns every assignment in file whose target is one of the
// named seams, in source order. Assignments inside TestMain are left out: that
// route has no *testing.T, so it cannot reach a staging helper.
func seamAssignments(file *ast.File, seams []seamDecl) []seamAssignment {
	var found []seamAssignment
	for _, decl := range file.Decls {
		if fn, isFunc := decl.(*ast.FuncDecl); isFunc && fn.Name.Name == testMainFunc {
			continue
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, target := range assign.Lhs {
				ident, isIdent := target.(*ast.Ident)
				if !isIdent {
					continue
				}
				if i := slices.IndexFunc(seams, func(s seamDecl) bool { return s.name == ident.Name }); i >= 0 {
					found = append(found, seamAssignment{seamDecl: seams[i], pos: ident.Pos()})
				}
			}
			return true
		})
	}
	return found
}

// declaredSeams returns both families of package-level test seam the cmd
// production sources declare, in name order.
func declaredSeams(t *testing.T) []seamDecl {
	t.Helper()
	seams := append(declaredDepsSeams(t), declaredFuncSeams(t)...)
	slices.SortFunc(seams, func(a, b seamDecl) int { return strings.Compare(a.name, b.name) })
	return seams
}

// declaredDepsSeams returns the `var xDeps *XDeps` seams, each paired with its
// own named staging helper.
func declaredDepsSeams(t *testing.T) []seamDecl {
	t.Helper()
	return requireSeams(t, depsSeamDecls(sourceguardtest.ParsePackageSources(t, ".", false)), "*Deps")
}

// declaredFuncSeams returns the package-level function-var seams, all staged
// through the one generic helper.
func declaredFuncSeams(t *testing.T) []seamDecl {
	t.Helper()
	return requireSeams(t, funcSeamDecls(sourceguardtest.ParsePackageSources(t, ".", false)), "function-var")
}

// requireSeams sorts an arm's seams by name and fails outright when the arm came
// back empty: its whole subject would otherwise have vanished unnoticed.
func requireSeams(t *testing.T, seams []seamDecl, kind string) []seamDecl {
	t.Helper()
	if len(seams) == 0 {
		t.Fatalf("no package-level %s seams found in the cmd production sources; the guard has lost that arm's subject", kind)
	}
	slices.SortFunc(seams, func(a, b seamDecl) int { return strings.Compare(a.name, b.name) })
	return seams
}

// depsSeamDecls returns the `var xDeps *XDeps` seams sources declare, each named
// alongside the withXDeps helper that stages it.
func depsSeamDecls(sources []sourceguardtest.ParsedSource) []seamDecl {
	var seams []seamDecl
	forEachPackageVar(sources, func(name *ast.Ident, typ, _ ast.Expr) {
		star, isStar := typ.(*ast.StarExpr)
		if !isStar {
			return
		}
		pointee, isIdent := star.X.(*ast.Ident)
		if !isIdent || !strings.HasSuffix(pointee.Name, "Deps") || !strings.HasSuffix(name.Name, "Deps") {
			return
		}
		seams = append(seams, seamDecl{name: name.Name, helper: "with" + strings.ToUpper(name.Name[:1]) + name.Name[1:]})
	})
	return seams
}

// funcSeamDecls returns the package-level function-var seams sources declare.
//
// The rule is the declaration's own face: a package-level var holds a function
// when it carries an explicit func type, when its initialiser is a function
// literal, when that initialiser names a function the package declares, or when
// it is a qualified name from another package. The last of those is where the
// rule is deliberately generous — a qualified non-function value is
// indistinguishable from a function without type information the guard does not
// have — so a var holding one must state its type, as an interface-typed seam
// already does. A spurious seam costs a helper call; the opposite mistake drops
// a real seam out of the guard's vocabulary, which is the failure this guard
// exists to prevent.
func funcSeamDecls(sources []sourceguardtest.ParsedSource) []seamDecl {
	packageFuncs := map[string]bool{}
	funcTypes := map[string]bool{}
	for _, source := range sources {
		for _, decl := range source.File.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil {
					packageFuncs[decl.Name.Name] = true
				}
			case *ast.GenDecl:
				if decl.Tok != token.TYPE {
					continue
				}
				for _, spec := range decl.Specs {
					typeSpec, isType := spec.(*ast.TypeSpec)
					if !isType {
						continue
					}
					if _, isFunc := typeSpec.Type.(*ast.FuncType); isFunc {
						funcTypes[typeSpec.Name.Name] = true
					}
				}
			}
		}
	}

	var seams []seamDecl
	forEachPackageVar(sources, func(name *ast.Ident, typ, value ast.Expr) {
		if holdsFunc(typ, value, packageFuncs, funcTypes) {
			seams = append(seams, seamDecl{name: name.Name, helper: funcSeamHelper})
		}
	})
	return seams
}

// holdsFunc reports whether a package-level var declaration says on its face
// that the var holds a function. An explicit type settles it, either as a
// literal func type or as the name of a func type the package declares; only an
// absent one is read off the initialiser. The residual is an explicit type
// named from another package (`var seam pkg.FuncType = …`): whether that name
// denotes a function cannot be resolved from this package's AST alone, so such
// a var is not recognised as a seam.
func holdsFunc(typ, value ast.Expr, packageFuncs, funcTypes map[string]bool) bool {
	if typ != nil {
		switch typ := typ.(type) {
		case *ast.FuncType:
			return true
		case *ast.Ident:
			return funcTypes[typ.Name]
		default:
			return false
		}
	}
	switch initialiser := value.(type) {
	case *ast.FuncLit:
		return true
	case *ast.Ident:
		return packageFuncs[initialiser.Name]
	case *ast.SelectorExpr:
		return true
	default:
		return false
	}
}

// forEachPackageVar visits every name declared by a package-level var across
// sources, handing the recogniser the type and initialiser that name was
// declared with — either of which may be absent.
func forEachPackageVar(sources []sourceguardtest.ParsedSource, visit func(name *ast.Ident, typ, value ast.Expr)) {
	for _, source := range sources {
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
				for i, name := range value.Names {
					var initialiser ast.Expr
					if i < len(value.Values) {
						initialiser = value.Values[i]
					}
					visit(name, value.Type, initialiser)
				}
			}
		}
	}
}

// seamNames reduces seams to their identifiers, for the set comparisons the
// coverage assertions make.
func seamNames(seams []seamDecl) []string {
	names := make([]string, 0, len(seams))
	for _, seam := range seams {
		names = append(names, seam.name)
	}
	return names
}

func TestSeamGuard(t *testing.T) {
	t.Run("it covers every declared seam identifier", func(t *testing.T) {
		declared := seamNames(declaredDepsSeams(t))

		var staged []string
		for _, sc := range seamStagingCases() {
			staged = append(staged, sc.name)
		}
		slices.Sort(staged)

		if !slices.Equal(declared, staged) {
			t.Errorf("the declared seams %v are not the seams staged through a helper %v; every seam needs an install helper, and the guard's coverage is the staging table", declared, staged)
		}
	})

	t.Run("it flags a direct assignment to a *Deps seam", func(t *testing.T) {
		// Demonstrated per identifier: one seam slipping out of the guard's
		// vocabulary is exactly the regression this test exists to catch.
		for _, seam := range declaredDepsSeams(t) {
			t.Run(seam.name, func(t *testing.T) {
				path := writeSeamFixture(t, "offender_test.go", fmt.Sprintf(
					"package cmd\n\nfunc TestOffender() {\n\t%s = nil\n}\n", seam.name))

				failed, msg := captureSeamGuardFailure(func(sub harnesstest.TestingT) {
					runSeamAssignmentGuard(sub, []string{path}, declaredSeams(t), seamHelperFile)
				})

				if !failed {
					t.Fatalf("the guard passed a test file assigning %s directly", seam.name)
				}
				if !strings.Contains(msg, seam.name) {
					t.Errorf("the guard's complaint %q does not name the %s seam it flagged", msg, seam.name)
				}
			})
		}
	})

	t.Run("it passes a test file installing every seam through its helper", func(t *testing.T) {
		var body strings.Builder
		body.WriteString("package cmd\n\nfunc TestWellBehaved() {\n")
		for _, seam := range declaredDepsSeams(t) {
			fmt.Fprintf(&body, "\t%s(t, %s%s{})\n",
				seam.helper, strings.ToUpper(seam.name[:1]), seam.name[1:])
		}
		for _, seam := range declaredFuncSeams(t) {
			fmt.Fprintf(&body, "\t%s(t, &%s, nil)\n", seam.helper, seam.name)
		}
		body.WriteString("}\n")
		path := writeSeamFixture(t, "wellbehaved_test.go", body.String())

		failed, msg := captureSeamGuardFailure(func(sub harnesstest.TestingT) {
			runSeamAssignmentGuard(sub, []string{path}, declaredSeams(t), seamHelperFile)
		})
		if failed {
			t.Errorf("the guard flagged a file that installs every seam through its helper: %s", msg)
		}
	})

	t.Run("it fatals when the guard scans no files", func(t *testing.T) {
		for name, paths := range map[string][]string{
			"no paths at all":           nil,
			"no test source among them": {"root.go", "open.go"},
			"only the helper file":      {seamHelperFile},
		} {
			t.Run(name, func(t *testing.T) {
				failed, msg := captureSeamGuardFailure(func(sub harnesstest.TestingT) {
					runSeamAssignmentGuard(sub, paths, declaredSeams(t), seamHelperFile)
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

// writeSeamFixture stages a source fixture a guard can be pointed at. The caller
// names it, because the assignment guard's own rules turn on the filename.
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

func TestFuncSeamGuard(t *testing.T) {
	t.Run("it derives the function-var seam set from the production sources", func(t *testing.T) {
		path := writeSeamFixture(t, "staged.go", `package staged

var explicitlyTyped func(int) error = someFunc

type localFn func(int) error

var fromNamedType localFn = someFunc

var fromLiteral = func() {}

var fromPackageFunc = someFunc

var fromQualifiedName = os.Exit

var (
	firstGrouped  = someFunc
	secondGrouped = func() {}
)

var notAFunc io.Writer = os.Stderr

var notAFuncLiteral = "text"

var notAPackageFunc = someValue

var alsoNotASeam *OpenDeps

var someValue = 1

func someFunc(int) error { return nil }

func holder() {
	var localFunc = func() {}
	_ = localFunc
}
`)
		sources := sourceguardtest.ParseSources(t, []string{path})

		want := []string{
			"explicitlyTyped",
			"firstGrouped",
			"fromLiteral",
			"fromNamedType",
			"fromPackageFunc",
			"fromQualifiedName",
			"secondGrouped",
		}
		got := seamNames(funcSeamDecls(sources))
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("the derived function-var seams are %v, want %v", got, want)
		}
	})

	t.Run("it flags a direct assignment to a function-var seam", func(t *testing.T) {
		for _, seam := range declaredFuncSeams(t) {
			t.Run(seam.name, func(t *testing.T) {
				path := writeSeamFixture(t, "offender_test.go", fmt.Sprintf(
					"package cmd\n\nfunc TestOffender() {\n\t%s = nil\n}\n", seam.name))

				failed, msg := captureSeamGuardFailure(func(sub harnesstest.TestingT) {
					runSeamAssignmentGuard(sub, []string{path}, declaredSeams(t), seamHelperFile)
				})

				if !failed {
					t.Fatalf("the guard passed a test file assigning %s directly", seam.name)
				}
				if !strings.Contains(msg, funcSeamHelper) {
					t.Errorf("the guard's complaint %q does not name the %s helper the seam is installed through", msg, funcSeamHelper)
				}
			})
		}
	})

	t.Run("it leaves TestMain's package-wide install alone", func(t *testing.T) {
		seam := declaredFuncSeams(t)[0].name
		path := writeSeamFixture(t, "testmain_test.go", fmt.Sprintf(
			"package cmd\n\nfunc TestMain(m *testing.M) {\n\t%s = nil\n\tos.Exit(m.Run())\n}\n", seam))

		failed, msg := captureSeamGuardFailure(func(sub harnesstest.TestingT) {
			runSeamAssignmentGuard(sub, []string{path}, declaredSeams(t), seamHelperFile)
		})
		if failed {
			t.Errorf("the guard flagged TestMain's package-wide install, which has no *testing.T to restore into: %s", msg)
		}
	})
}
