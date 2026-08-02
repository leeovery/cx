package capture

import "testing"

// TestFixtureColourless_ReadsDepsNoColor pins the discriminator §13.4's
// swap-and-diff guard excludes colourless fixtures on.
//
// The exclusion has to be structural: a colourless render carries no theme hexes
// at all, so there is nothing to diff, and a hand-maintained name list would go
// stale the moment a colourless fixture is added. Colourless therefore reads the
// fixture's own Deps rather than answering from a list.
//
// It is an in-package test because the interesting case does not exist among the
// registered fixtures: none is colourless today, so the accessor is exercised over
// a fixture whose flag is set directly. Asserting only over the registered set
// would check nothing but the false branch.
func TestFixtureColourless_ReadsDepsNoColor(t *testing.T) {
	t.Run("false when the fixture sets no NO_COLOR flag", func(t *testing.T) {
		fx := &Fixture{Lister: &fakeLister{}, projectStore: &fakeProjectStore{}}
		if fx.Deps().NoColor {
			t.Fatal("probe setup: Deps().NoColor is true on an unflagged fixture")
		}
		if fx.Colourless() {
			t.Error("Colourless() = true on an unflagged fixture, want false")
		}
	})

	t.Run("true when the fixture carries the NO_COLOR flag", func(t *testing.T) {
		fx := &Fixture{Lister: &fakeLister{}, projectStore: &fakeProjectStore{}, noColor: true}
		if !fx.Deps().NoColor {
			t.Fatal("probe setup: Deps() does not carry the fixture's NO_COLOR flag through to the model seam set")
		}
		if !fx.Colourless() {
			t.Error("Colourless() = false on a NO_COLOR fixture, want true — the guard's exclusion would miss it")
		}
	})

	t.Run("every registered fixture agrees with its own Deps", func(t *testing.T) {
		for _, name := range FixtureNames() {
			if name == ContrastValidationFixture {
				continue
			}
			fx, err := FixtureByName(name)
			if err != nil {
				t.Fatalf("FixtureByName(%s): %v", name, err)
			}
			if got, want := fx.Colourless(), fx.Deps().NoColor; got != want {
				t.Errorf("%s: Colourless() = %t, Deps().NoColor = %t", name, got, want)
			}
		}
	})
}
