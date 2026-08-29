package nanoid_test

import (
	"os/exec"
	"strings"
	"testing"
)

const nanoidPkg = "github.com/leeovery/portal/internal/nanoid"

// The id vocabulary is shared by packages that must not import each other, so
// it can only ever depend on the standard library.
func TestNanoIDPackage_DependsOnTheStandardLibraryAlone(t *testing.T) {
	for _, dep := range packageDeps(t, nanoidPkg) {
		if dep == nanoidPkg {
			continue
		}
		// A standard-library import path has no dot in its first segment;
		// every other origin — this module included — is a domain name.
		if root, _, _ := strings.Cut(dep, "/"); strings.Contains(root, ".") {
			t.Errorf("internal/nanoid depends on %s — the id vocabulary is a stdlib-only leaf so any package can reach it", dep)
		}
	}
}

func packageDeps(t *testing.T, pkg string) []string {
	t.Helper()

	out, err := exec.Command("go", "list", "-deps", pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
	}
	return strings.Fields(string(out))
}
