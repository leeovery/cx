//go:build integration

// This file builds the portal CLI, so it lives in the integration lane: the
// unit lane compiles no portal binary.

package portalbintest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

func TestStagePortalBinary(t *testing.T) {
	priorPATH := os.Getenv("PATH")

	binDir := portalbintest.StagePortalBinary(t)

	if binDir == "" {
		t.Fatalf("StagePortalBinary returned empty binDir")
	}

	binary := filepath.Join(binDir, "portal")
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("expected portal binary at %s: %v", binary, err)
	}

	// The prepend order matters: a system-installed portal must not shadow the
	// freshly built one.
	gotPATH := os.Getenv("PATH")
	wantPrefix := binDir + string(os.PathListSeparator)
	if !strings.HasPrefix(gotPATH, wantPrefix) {
		t.Fatalf("PATH not prepended with binDir; got %q, want prefix %q", gotPATH, wantPrefix)
	}
	if !strings.HasSuffix(gotPATH, priorPATH) {
		t.Fatalf("PATH does not retain prior PATH as suffix; got %q, want suffix %q", gotPATH, priorPATH)
	}

	resolved, err := exec.LookPath("portal")
	if err != nil {
		t.Fatalf("exec.LookPath(\"portal\") after StagePortalBinary: %v", err)
	}
	// EvalSymlinks both sides: a symlinked $TMPDIR would otherwise false-negative.
	resolvedReal, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", resolved, err)
	}
	binDirReal, err := filepath.EvalSymlinks(binDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", binDir, err)
	}
	if filepath.Dir(resolvedReal) != binDirReal {
		t.Fatalf("portal resolved outside staged binDir; got %s, want under %s", resolvedReal, binDirReal)
	}
}
