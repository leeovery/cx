package sourceguardtest_test

import (
	"go/ast"
	"go/build/constraint"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

const sourceGuardTestPkg = "github.com/leeovery/portal/internal/sourceguardtest"

// The guard is written with the primitives it polices, which is the ordinary
// bootstrapping shape: a dependency this package should not have arrives in
// this suite's own dependency set too, so the assertion below judges it rather
// than being blinded to it.
func TestSourceGuardTestPackage(t *testing.T) {
	// The source guards across this module are unit-lane tests because the
	// scanning primitives they share reach no further than the standard
	// library and the stand-in they report through. Anything else here — a
	// third-party walker, or an internal package of the tree these guards
	// read — would drag its own dependencies, and any build tag among them,
	// into every one of those guards at once.
	t.Run("it confines internal/sourceguardtest to the standard library", func(t *testing.T) {
		// portalbintest is admitted for the root resolution the repo-wide scan
		// is anchored at, and it is admissible for the same reason the rule
		// exists: it is stdlib-only and untagged, so it drags neither a
		// dependency nor a lane onto the guards built here.
		sourceguardtest.AssertDepsWithin(t, sourceGuardTestPkg, []string{
			"github.com/leeovery/portal/internal/harnesstest",
			"github.com/leeovery/portal/internal/portalbintest",
		}, sourceguardtest.ForbiddingThirdParty())
	})

	// go list resolves a package under the default build tags, so a dependency
	// behind a tag on one of these sources sits outside every dependency
	// reading taken of this package — and a primitive behind one carries the
	// tag onto whatever guard reaches for it.
	t.Run("it carries no build tag, so the guards built on it stay in the unit lane", func(t *testing.T) {
		for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
			if line, tagged := buildConstraintLine(source.File); tagged {
				t.Errorf("%s carries the build constraint %s — a tag here gates every guard built on these primitives out of the unit lane", source.Path, line)
			}
		}
	})
}

// buildConstraintLine reports the first //go:build line preceding the package
// clause, which is where a build constraint on the file is stated.
func buildConstraintLine(file *ast.File) (string, bool) {
	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}
		for _, c := range group.List {
			if _, err := constraint.Parse(c.Text); err == nil {
				return c.Text, true
			}
		}
	}
	return "", false
}
