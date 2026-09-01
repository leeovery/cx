package nanoid_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

const nanoidPkg = "github.com/leeovery/portal/internal/nanoid"

// The id vocabulary is shared by packages that must not import each other, so
// it can only ever depend on the standard library.
func TestNanoIDPackage_DependsOnTheStandardLibraryAlone(t *testing.T) {
	for _, dep := range sourceguardtest.PackageDeps(t, nanoidPkg) {
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
