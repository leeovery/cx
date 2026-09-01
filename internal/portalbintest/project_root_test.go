package portalbintest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

// ProjectRoot compiles nothing, so it stays in the unit lane, where the
// repo-wide source guards call it.
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
