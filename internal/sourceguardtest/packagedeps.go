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

// PackageDeps returns pkg's transitive dependency list — the import paths
// `go list -deps` reports, pkg itself included, in that command's order. The
// argument is an import path rather than a directory so a guard resolves the
// same set regardless of the test binary's working directory. A package go list
// cannot resolve is fatal: a leaf guard must fail rather than pass over an
// empty set.
func PackageDeps(t TestingT, pkg string) []string {
	t.Helper()

	out, err := exec.Command("go", "list", "-deps", pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
		return nil
	}
	return strings.Fields(string(out))
}
