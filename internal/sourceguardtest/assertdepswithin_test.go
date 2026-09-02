package sourceguardtest

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

const (
	hooksPkg     = "github.com/leeovery/portal/internal/hooks"
	nanoidPkg    = "github.com/leeovery/portal/internal/nanoid"
	themePkg     = "github.com/leeovery/portal/internal/theme"
	fileutilPkg  = "github.com/leeovery/portal/internal/fileutil"
	logPkg       = "github.com/leeovery/portal/internal/log"
	nanoidPkgDir = "../nanoid"
)

func TestAssertDepsWithin_ReportsEveryDepOutsideTheAllowlist(t *testing.T) {
	stub := &stubT{}

	AssertDepsWithin(stub, hooksPkg, []string{fileutilPkg})

	for _, want := range []string{logPkg, nanoidPkg, "internal/storelog"} {
		if !stub.errored(want) {
			t.Errorf("AssertDepsWithin did not report %s as outside the allowlist; reported %v", want, stub.errors)
		}
	}
	if stub.fataled {
		t.Errorf("AssertDepsWithin fatalled on an allowlist whose entry is present: %q", stub.fatal)
	}
}

func TestAssertDepsWithin_FatalsOnAnEmptyDepSet(t *testing.T) {
	defer stubListDeps(t, nil, nil)()
	stub := &stubT{}

	AssertDepsWithin(stub, hooksPkg, []string{fileutilPkg})

	if !stub.fataled {
		t.Fatal("AssertDepsWithin did not fatal on an empty dependency set — the guard would pass over nothing")
	}
}

func TestAssertDepsWithin_FatalsWhenTheSetDoesNotHoldThePackageItself(t *testing.T) {
	defer stubListDeps(t, []dep{{Path: "strings"}}, nil)()
	stub := &stubT{}

	AssertDepsWithin(stub, hooksPkg, nil)

	if !stub.fataled {
		t.Fatal("AssertDepsWithin did not fatal over a set that does not hold the package under test — it was asserting about something else")
	}
}

func TestAssertDepsWithin_FatalsWhenNoAllowlistedInternalDepIsPresent(t *testing.T) {
	stub := &stubT{}

	AssertDepsWithin(stub, nanoidPkg, []string{fileutilPkg})

	if !stub.fataled {
		t.Fatalf("AssertDepsWithin did not fatal when its allowlist named no dependency the package actually has; errors %v", stub.errors)
	}
	if !strings.Contains(stub.fatal, fileutilPkg) {
		t.Errorf("fatal message %q does not name the allowlist it could not see", stub.fatal)
	}
}

func TestAssertDepsWithin_LeavesAnotherModuleAloneByDefault(t *testing.T) {
	stub := &stubT{}

	AssertDepsWithin(stub, themePkg, []string{logPkg})

	if stub.fataled || len(stub.errors) > 0 {
		t.Errorf("AssertDepsWithin faulted a dependency outside the package's own module: fatal %q, errors %v", stub.fatal, stub.errors)
	}
}

func TestAssertDepsWithin_ForbiddingThirdPartyReportsAnotherModule(t *testing.T) {
	stub := &stubT{}

	AssertDepsWithin(stub, themePkg, []string{logPkg}, ForbiddingThirdParty())

	if !stub.errored("lipgloss") {
		t.Errorf("ForbiddingThirdParty did not report the third-party dependency; reported %v", stub.errors)
	}
}

func TestAssertDepsWithin_PassesForAStdlibOnlyPackageWithAnEmptyAllowlist(t *testing.T) {
	stub := &stubT{}

	AssertDepsWithin(stub, nanoidPkg, nil, ForbiddingThirdParty())

	if stub.fataled || len(stub.errors) > 0 {
		t.Errorf("AssertDepsWithin faulted a stdlib-only package: fatal %q, errors %v", stub.fatal, stub.errors)
	}
}

func TestPackageDeps_ResolvesAPackageRelativeToTheGivenWorkingDirectory(t *testing.T) {
	deps := PackageDeps(t, ".", InDir(nanoidPkgDir))

	if !slices.Contains(deps, nanoidPkg) {
		t.Errorf("PackageDeps(\".\", InDir(%q)) resolved %v, want the set to hold %s", nanoidPkgDir, deps, nanoidPkg)
	}
	if own := PackageDeps(t, "."); slices.Contains(own, nanoidPkg) {
		t.Errorf("PackageDeps(\".\") without a directory resolved %s — the control case is not distinguishing", nanoidPkg)
	}
}

// stubListDeps swaps the enumeration seam for the duration of a case, so the
// shapes go list cannot be made to produce on demand — an empty set among
// them — are still exercised. The returned func restores the real one.
func stubListDeps(t *testing.T, deps []dep, err error) func() {
	t.Helper()
	prior := listDeps
	listDeps = func(_, _ string) ([]dep, error) { return deps, err }
	return func() { listDeps = prior }
}

// stubT stands in for *testing.T so the failure paths are observable. A real
// Fatalf ends the goroutine, which the recorder cannot do, so the code under
// test must return explicitly after fatalling.
type stubT struct {
	errors  []string
	fataled bool
	fatal   string
}

func (s *stubT) Helper() {}

func (s *stubT) Errorf(format string, args ...any) {
	s.errors = append(s.errors, fmt.Sprintf(format, args...))
}

func (s *stubT) Fatalf(format string, args ...any) {
	s.fataled = true
	s.fatal = fmt.Sprintf(format, args...)
}

func (s *stubT) errored(substring string) bool {
	for _, msg := range s.errors {
		if strings.Contains(msg, substring) {
			return true
		}
	}
	return false
}
