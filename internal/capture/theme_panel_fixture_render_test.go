package capture_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

var panelEntryRefusalCopy = []string{
	"terminal too narrow for the theme picker",
	"terminal too short for the theme picker",
}

func panelFixtureFrame(t *testing.T, fixture string, palette theme.Theme) string {
	t.Helper()
	return panelFrameAt(t, fixture, palette, harnessWidth, harnessHeight)
}

func registeredPanelFixtureNames() []string {
	var names []string
	for _, name := range capture.FixtureNames() {
		if strings.HasPrefix(name, panelFixtureNamePrefix) {
			names = append(names, name)
		}
	}
	return names
}

func stageDecoyConfig(t *testing.T) (configDir string) {
	t.Helper()

	dir := t.TempDir()
	themes := filepath.Join(dir, "portal", "themes")
	if err := os.MkdirAll(themes, 0o755); err != nil {
		t.Fatalf("stage the decoy themes directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themes, decoySlug+theme.FileExtension), themetest.Body(), 0o644); err != nil {
		t.Fatalf("stage the decoy theme: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PORTAL_THEMES_DIR", themes)
	t.Setenv("PORTAL_PREFS_FILE", filepath.Join(dir, "prefs.json"))
	return dir
}

const decoySlug = "decoy-drop-in"

func requireConfigUntouched(t *testing.T, configDir, frame string) {
	t.Helper()

	if strings.Contains(ansi.Strip(frame), decoySlug) {
		t.Error("the panel lists the decoy drop-in; the fixture reached the real themes directory")
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Fatalf("read the isolated config dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "portal" {
		t.Errorf("the config dir holds %v, want only the staged portal/ tree — the fixture wrote config", entries)
	}
}

func TestPanelFixture_NoConfigAccess(t *testing.T) {
	for _, name := range registeredPanelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			configDir := stageDecoyConfig(t)
			requireConfigUntouched(t, configDir, panelFixtureFrame(t, name, themetest.Builtin(t, theme.DefaultDarkSlug)))
		})
	}
}

func TestPanelFixture_AdaptivePairBadges(t *testing.T) {
	rows := panelRows(t, panelFixtureFrame(t, "theme-panel-adaptive-pair", themetest.Builtin(t, "nord")))

	t.Run("the light slot's row carries ● light", func(t *testing.T) {
		if got, want := rows["tokyo-night-day"].badge, "● light"; got != want {
			t.Errorf("the tokyo-night-day row's badge = %q, want %q", got, want)
		}
	})

	t.Run("the dark slot's row carries ● dark", func(t *testing.T) {
		if got, want := rows["nord"].badge, "● dark"; got != want {
			t.Errorf("the nord row's badge = %q, want %q", got, want)
		}
	})

	t.Run("the cursor is on the in-force dark slot's row", func(t *testing.T) {
		if !rows["nord"].cursor {
			t.Error("the cursor is not on nord; the cursor belongs on the theme actually rendering, which under a pair with no gate is the dark slot's")
		}
		if rows["tokyo-night-day"].cursor {
			t.Error("the cursor is on tokyo-night-day as well; only ONE row may carry the cursor bar")
		}
	})

	t.Run("the unassigned rows carry no badge", func(t *testing.T) {
		for _, label := range []string{"catppuccin-latte", "tokyo-night"} {
			if got := rows[label].badge; got != "" {
				t.Errorf("the %s row carries badge %q; `●` marks what is SET, and nothing sets it", label, got)
			}
		}
	})
}

func TestPanelFixture_ConstantWhilePreviewing(t *testing.T) {
	rows := panelRows(t, panelFixtureFrame(t, "theme-panel-constant-previewing", themetest.Builtin(t, theme.DefaultLightSlug)))

	t.Run("the constant's row carries a bare ●", func(t *testing.T) {
		if got, want := rows["nord"].badge, "●"; got != want {
			t.Errorf("the nord row's badge = %q, want the bare %q", got, want)
		}
	})

	t.Run("no slot badge appears on any list row", func(t *testing.T) {
		for _, label := range panelUnionSlugs() {
			if badge := rows[label].badge; badge != "" && badge != "●" {
				t.Errorf("the %s row carries badge %q; under a constant the slots are not read at all, so the only badge available is the bare ●", label, badge)
			}
		}
	})

	t.Run("the cursor sits on a different row from the marked one", func(t *testing.T) {
		if !rows["tokyo-night-day"].cursor {
			t.Error("the cursor is not on tokyo-night-day; the whole point of this frame is a cursor on a row other than the marked one")
		}
		if rows["nord"].cursor {
			t.Error("the cursor is on the marked row nord, so this frame shows nothing the adaptive-pair frame does not")
		}
	})
}

func TestPanelFixture_CursorSeedDoesNotApplyATheme(t *testing.T) {
	pinned := themetest.Builtin(t, "nord")
	seeded := themetest.Builtin(t, theme.DefaultLightSlug)

	frame := panelFixtureFrame(t, "theme-panel-constant-previewing", pinned)

	if !panelRows(t, frame)["tokyo-night-day"].cursor {
		t.Fatal("the cursor is not on the seeded row, so this asserts nothing about what the seed does")
	}
	if !strings.Contains(frame, bgSeq(t, pinned.Canvas)) {
		t.Errorf("the frame does not carry `--theme`'s canvas %s; the palette must follow the flag", pinned.Canvas.Value)
	}
	if strings.Contains(frame, bgSeq(t, seeded.Canvas)) {
		t.Errorf("the frame carries the SEEDED row's canvas %s; the cursor seed is placement only and must apply no theme", seeded.Canvas.Value)
	}
}

func TestPanelFixture_PaletteFollowsTheThemeFlag(t *testing.T) {
	first, second := themetest.Builtin(t, "nord"), themetest.Builtin(t, theme.DefaultLightSlug)
	if first.Canvas.Value == second.Canvas.Value {
		t.Fatalf("the two palettes share a canvas (%s); the diff below would be unobservable", first.Canvas.Value)
	}

	for _, fixture := range capturePanelFixtureNames() {
		t.Run(fixture, func(t *testing.T) {
			a := panelFixtureFrame(t, fixture, first)
			b := panelFixtureFrame(t, fixture, second)

			if a == b {
				t.Fatal("the two renders are byte-identical, so the palette does not follow `--theme` at all")
			}
			for _, tc := range []struct {
				name    string
				frame   string
				carries theme.Theme
				lacks   theme.Theme
			}{
				{"under the first palette", a, first, second},
				{"under the second palette", b, second, first},
			} {
				if !strings.Contains(tc.frame, bgSeq(t, tc.carries.Canvas)) {
					t.Errorf("%s: the frame does not carry canvas %s", tc.name, tc.carries.Canvas.Value)
				}
				if strings.Contains(tc.frame, bgSeq(t, tc.lacks.Canvas)) {
					t.Errorf("%s: the frame carries the OTHER palette's canvas %s", tc.name, tc.lacks.Canvas.Value)
				}
			}

			if got, want := labelSet(panelRows(t, a)), labelSet(panelRows(t, b)); !slices.Equal(got, want) {
				t.Errorf("the panel lists %v under one palette and %v under the other; the row set is fixture data and must not follow the flag", got, want)
			}
		})
	}
}

func labelSet(rows map[string]panelRow) []string {
	labels := make([]string, 0, len(rows))
	for label := range rows {
		labels = append(labels, label)
	}
	slices.Sort(labels)
	return labels
}

func capturePanelFixtureNames() []string {
	return []string{
		panelFixtureNamePrefix + "adaptive-pair",
		panelFixtureNamePrefix + "constant-previewing",
	}
}

func panelUnionSlugs() []string {
	return []string{"catppuccin-latte", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug}
}

func TestPanelFixture_Registered(t *testing.T) {
	guarded := make([]string, 0, len(capture.FixtureNames()))
	for _, fx := range guardedFixtures(t) {
		guarded = append(guarded, fx.Name())
	}

	for _, name := range registeredPanelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			if _, err := capture.FixtureByName(name); err != nil {
				t.Errorf("FixtureByName(%s): %v — the fixture is not resolvable", name, err)
			}
			if !slices.Contains(capture.FixtureNames(), name) {
				t.Errorf("FixtureNames() %v omits %s — an unenumerated fixture is never driven by the guard", capture.FixtureNames(), name)
			}
			if !slices.Contains(guarded, name) {
				t.Errorf("%s is not in the guarded set %v; the guard enumerates, so a fixture it never sees is uncovered while reading as covered", name, guarded)
			}
		})
	}
}

func TestPanelFixture_PanelIsCompositedUnderTheGuard(t *testing.T) {
	a, b := syntheticPalettes(t)

	for _, name := range capturePanelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			fx, err := capture.FixtureByName(name)
			if err != nil {
				t.Fatalf("FixtureByName(%s): %v", name, err)
			}

			before, _ := fx.RenderSwapRender(a, b, harnessWidth, harnessHeight)
			visible := ansi.Strip(before)

			if !strings.Contains(visible, panelBorder) {
				t.Errorf("the A-frame at %d×%d carries no panel left border; the entry gate refused, so RAISE the pinned render size rather than weakening this:\n%s", harnessWidth, harnessHeight, visible)
			}
			if !strings.Contains(visible, "Themes") {
				t.Errorf("the A-frame at %d×%d carries no `Themes` header; the panel did not open:\n%s", harnessWidth, harnessHeight, visible)
			}
			for _, refusal := range panelEntryRefusalCopy {
				if strings.Contains(visible, refusal) {
					t.Errorf("the A-frame carries the blocked-entry flash %q; the pinned render size is below task 8-11's floor", refusal)
				}
			}
		})
	}
}
