package portalbintest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

func TestProjectRoot(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("ProjectRoot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected go.mod under %s: %v", root, err)
	}
	// A stray go.mod in a parent directory would pass the check above, so
	// confirm the module path too.
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	want := "module github.com/leeovery/portal"
	if !strings.Contains(string(data), want) {
		t.Errorf("go.mod at %s does not declare %q; got:\n%s", root, want, data)
	}
}

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
