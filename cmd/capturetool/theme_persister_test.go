package main

// Tests in this file set env vars, so they MUST NOT use t.Parallel.

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/tui"
)

// TestCapturetool_NoThemePersister pins §8.9's harness half: a capture commits
// nowhere. Every fixture leaves the theme-commit seam unwired, so a commit taken
// during a capture has nothing to write with — exactly as the mode persister
// already behaves, and the reason the harness reads no real config at all.
//
// The enumeration is over EVERY registered fixture rather than a sampled one, so
// a fixture added later inherits the guarantee instead of quietly opting out. It
// covers capturetool itself because resolveModel takes the fixture's Deps
// verbatim, overriding only the theme and the NO_COLOR flag.
func TestCapturetool_NoThemePersister(t *testing.T) {
	names := capture.FixtureNames()
	if len(names) == 0 {
		t.Fatal("no fixtures are registered; the enumeration below would be vacuous")
	}

	for _, name := range names {
		if name == capture.ContrastValidationFixture {
			// The swatch is a standalone tea.Model resolved by resolveProgram, not
			// a tui.Build fixture: it holds no seam set at all, so there is nothing
			// here for it to wire.
			continue
		}
		t.Run(name, func(t *testing.T) {
			fx, err := capture.FixtureByName(name)
			if err != nil {
				t.Fatalf("FixtureByName(%s): %v", name, err)
			}

			deps := fx.Deps()
			if deps.ThemePersister != nil {
				t.Errorf("fixture %s wires a ThemePersister (%#v); a commit during a capture must write nowhere", name, deps.ThemePersister)
			}
			// The established precedent, asserted alongside so the two persisters
			// cannot diverge on the harness path.
			if deps.ModePersister != nil {
				t.Errorf("fixture %s wires a ModePersister (%#v); an `s`-toggle during a capture must write nowhere", name, deps.ModePersister)
			}
		})
	}
}

// TestCapturetool_WritesNoPrefsFile is the runtime half: a rendered capture,
// driven through the keys that persist in production, leaves no prefs.json
// behind.
//
// It is worth having beside the structural assertion above because the two fail
// differently. Nil seams prove the model cannot write; this proves nothing ELSE
// in the harness does either — no config path is resolved, nothing is created on
// the way to a frame. Both config env vars are pointed into an empty temp dir, so
// any write at all is visible as a file appearing in it.
func TestCapturetool_WritesNoPrefsFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PORTAL_PREFS_FILE", filepath.Join(dir, "prefs.json"))

	m, err := resolveModel("sessions-flat", builtinForTest(t, defaultThemeSlug))
	if err != nil {
		t.Fatalf("resolveModel(sessions-flat): %v", err)
	}

	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	// `s` is the one key that persists today (the grouping-mode toggle); the
	// theme panel's keys are Phase 9. Pressing it is what makes "wrote nothing"
	// an observation rather than an assumption.
	for range 3 {
		model, _ = model.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	}
	if content := model.(tui.Model).View().Content; content == "" {
		t.Fatal("the capture model painted nothing; the no-write assertion would be vacuous")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read the isolated config dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a capture wrote %d entr(y/ies) into the config dir: %v — the harness reads and writes no real config", len(entries), entries)
	}
}
