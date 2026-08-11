package main

import (
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

const capturePkg = "github.com/leeovery/portal/internal/capture"

const portalMainPkg = "github.com/leeovery/portal"

func goListDeps(t *testing.T, pkg string) []string {
	t.Helper()
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	cmd := exec.Command("go", "list", "-deps", pkg)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
	}
	return strings.Fields(string(out))
}

func TestPortalBinaryDoesNotImportCapture(t *testing.T) {
	for _, dep := range goListDeps(t, portalMainPkg) {
		if dep == capturePkg {
			t.Fatalf("portal binary (%s) transitively imports %s — harness code must stay out of production", portalMainPkg, capturePkg)
		}
	}
}

func TestCaptureToolDoesImportCapture(t *testing.T) {
	const captureToolPkg = "github.com/leeovery/portal/cmd/capturetool"
	if slices.Contains(goListDeps(t, captureToolPkg), capturePkg) {
		return
	}
	t.Fatalf("capture tool (%s) does NOT import %s — the import guard is vacuous", captureToolPkg, capturePkg)
}
