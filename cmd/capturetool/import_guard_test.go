package main

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

const capturePkg = "github.com/leeovery/portal/internal/capture"

const portalMainPkg = "github.com/leeovery/portal"

const captureToolPkg = "github.com/leeovery/portal/cmd/capturetool"

func TestPortalBinaryDoesNotImportCapture(t *testing.T) {
	if slices.Contains(deps(t, portalMainPkg), capturePkg) {
		t.Fatalf("portal binary (%s) transitively imports %s — harness code must stay out of production", portalMainPkg, capturePkg)
	}
}

func TestCaptureToolDoesImportCapture(t *testing.T) {
	if !slices.Contains(deps(t, captureToolPkg), capturePkg) {
		t.Fatalf("capture tool (%s) does NOT import %s — the import guard is vacuous", captureToolPkg, capturePkg)
	}
}

// deps enumerates pkg's transitive dependencies with resolution anchored at
// the module root, so the listing does not depend on where the test binary
// runs.
func deps(t *testing.T, pkg string) []string {
	t.Helper()
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	return sourceguardtest.PackageDeps(t, pkg, sourceguardtest.InDir(root))
}
