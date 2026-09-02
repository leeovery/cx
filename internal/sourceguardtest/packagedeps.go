package sourceguardtest

import (
	"os/exec"
	"strings"
)

// TestingT is the subset of *testing.T the fatal-on-failure primitives depend
// on, so their own failure paths can be unit-tested without aborting the
// harness.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// DepsOption adjusts how a dependency assertion resolves and judges its
// package.
type DepsOption func(*depsConfig)

// InDir runs the enumeration from dir, so a guard can name its package
// relatively, or anchor resolution somewhere other than the test binary's own
// working directory.
func InDir(dir string) DepsOption {
	return func(cfg *depsConfig) { cfg.dir = dir }
}

// ForbiddingThirdParty widens an assertion to judge dependencies belonging to
// other modules as well as to the package's own, so that an empty allowlist
// states "the standard library alone" rather than "nothing else from this
// module".
func ForbiddingThirdParty() DepsOption {
	return func(cfg *depsConfig) { cfg.thirdPartyForbidden = true }
}

type depsConfig struct {
	dir                 string
	thirdPartyForbidden bool
}

func newDepsConfig(opts []DepsOption) depsConfig {
	var cfg depsConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// dep is one row of a transitive dependency listing. Module is the path of the
// module providing the package, and is empty for the standard library — which
// is how a dependency's origin is told apart without any hardcoded prefix.
type dep struct {
	Path   string
	Module string
}

const depFormat = "{{.ImportPath}}\t{{with .Module}}{{.Path}}{{end}}"

// listDeps is the enumeration seam: the shapes go list cannot be made to
// produce on demand — an empty set among them — are reachable only by
// swapping it.
var listDeps = func(dir, pkg string) ([]dep, error) {
	cmd := exec.Command("go", "list", "-deps", "-f", depFormat, pkg)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &listError{output: string(out), err: err}
	}
	return parseDeps(string(out)), nil
}

func parseDeps(out string) []dep {
	var deps []dep
	for line := range strings.SplitSeq(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		path, module, _ := strings.Cut(line, "\t")
		deps = append(deps, dep{Path: path, Module: module})
	}
	return deps
}

type listError struct {
	output string
	err    error
}

func (e *listError) Error() string {
	return e.err.Error() + "\n" + e.output
}

// PackageDeps returns pkg's transitive dependency list — the import paths
// `go list -deps` reports, pkg itself included, in that command's order. The
// argument is ordinarily an import path, so a guard resolves the same set
// regardless of the test binary's working directory; InDir moves that
// resolution to a chosen directory for a caller that needs one. A package go
// list cannot resolve, and a set that comes back empty, are both fatal: a
// guard must fail rather than pass over nothing.
func PackageDeps(t TestingT, pkg string, opts ...DepsOption) []string {
	t.Helper()

	var paths []string
	for _, d := range packageDeps(t, pkg, opts) {
		paths = append(paths, d.Path)
	}
	return paths
}

func packageDeps(t TestingT, pkg string, opts []DepsOption) []dep {
	t.Helper()

	deps, err := listDeps(newDepsConfig(opts).dir, pkg)
	if err != nil {
		t.Fatalf("go list -deps %s: %v", pkg, err)
		return nil
	}
	if len(deps) == 0 {
		t.Fatalf("go list -deps %s resolved no dependencies at all — a guard over this set would pass vacuously", pkg)
		return nil
	}
	return deps
}
