package sourceguardtest

import (
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
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
	rec := &harnesstest.Recorder{}

	rec.Run(func() { AssertDepsWithin(rec, hooksPkg, []string{fileutilPkg}) })

	for _, want := range []string{logPkg, nanoidPkg, "internal/storelog"} {
		if !errored(rec, want) {
			t.Errorf("AssertDepsWithin did not report %s as outside the allowlist; reported %v", want, rec.Errors)
		}
	}
	if len(rec.Fatals) != 0 {
		t.Errorf("AssertDepsWithin fatalled on an allowlist whose entry is present: %v", rec.Fatals)
	}
}

func TestAssertDepsWithin_FatalsOnAnEmptyDepSet(t *testing.T) {
	defer stubListDeps(t, nil, nil)()
	rec := &harnesstest.Recorder{}

	rec.Run(func() { AssertDepsWithin(rec, hooksPkg, []string{fileutilPkg}) })

	if len(rec.Fatals) != 1 {
		t.Fatalf("AssertDepsWithin reported %d fatals on an empty dependency set, want 1 — the guard would pass over nothing: %v", len(rec.Fatals), rec.Fatals)
	}
}

func TestAssertDepsWithin_FatalsWhenTheSetDoesNotHoldThePackageItself(t *testing.T) {
	defer stubListDeps(t, []dep{{Path: "strings"}}, nil)()
	rec := &harnesstest.Recorder{}

	rec.Run(func() { AssertDepsWithin(rec, hooksPkg, nil) })

	if len(rec.Fatals) != 1 {
		t.Fatalf("AssertDepsWithin reported %d fatals over a set that does not hold the package under test, want 1 — it was asserting about something else: %v", len(rec.Fatals), rec.Fatals)
	}
}

func TestAssertDepsWithin_FatalsWhenNoAllowlistedInternalDepIsPresent(t *testing.T) {
	rec := &harnesstest.Recorder{}

	rec.Run(func() { AssertDepsWithin(rec, nanoidPkg, []string{fileutilPkg}) })

	if len(rec.Fatals) != 1 {
		t.Fatalf("AssertDepsWithin reported %d fatals when its allowlist named no dependency the package actually has, want 1; errors %v", len(rec.Fatals), rec.Errors)
	}
	if !strings.Contains(rec.Fatals[0], fileutilPkg) {
		t.Errorf("fatal message %q does not name the allowlist it could not see", rec.Fatals[0])
	}
}

func TestAssertDepsWithin_LeavesAnotherModuleAloneByDefault(t *testing.T) {
	rec := &harnesstest.Recorder{}

	rec.Run(func() { AssertDepsWithin(rec, themePkg, []string{logPkg}) })

	if rec.Failed() {
		t.Errorf("AssertDepsWithin faulted a dependency outside the package's own module: %s", rec.Report())
	}
}

func TestAssertDepsWithin_ForbiddingThirdPartyReportsAnotherModule(t *testing.T) {
	rec := &harnesstest.Recorder{}

	rec.Run(func() { AssertDepsWithin(rec, themePkg, []string{logPkg}, ForbiddingThirdParty()) })

	if !errored(rec, "lipgloss") {
		t.Errorf("ForbiddingThirdParty did not report the third-party dependency; reported %v", rec.Errors)
	}
}

func TestAssertDepsWithin_PassesForAStdlibOnlyPackageWithAnEmptyAllowlist(t *testing.T) {
	rec := &harnesstest.Recorder{}

	rec.Run(func() { AssertDepsWithin(rec, nanoidPkg, nil, ForbiddingThirdParty()) })

	if rec.Failed() {
		t.Errorf("AssertDepsWithin faulted a stdlib-only package: %s", rec.Report())
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

// errored reports whether any of the recorded complaints carries substring.
func errored(rec *harnesstest.Recorder, substring string) bool {
	return slices.ContainsFunc(rec.Errors, func(msg string) bool { return strings.Contains(msg, substring) })
}
