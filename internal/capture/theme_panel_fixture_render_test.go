package capture_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/theme"
)

// The FRAME-level half of §13.3's panel fixtures: what the two setting-state
// captures actually render.
//
// §9.14 identifies the panel's slot half as the part with NO PRIOR ART ANYWHERE —
// assigning a theme to a light/dark slot from inside a picker was found in no
// surveyed tool, so `d`/`l` and the `● dark` / `● light` / `● both` vocabulary
// have no established shape to borrow, and these frames plus the Paper artboards
// are the only reference that exists. That is why the badges are asserted here
// rather than left to the eye at the visual gate: the gate judges COLOUR, and a
// badge on the wrong row is not a colour question.
//
// These assertions read the RENDERED frame rather than model internals, because
// internal/tui exports no panel accessor — and because the frame is what the
// tapes screenshot, so a divergence between the two would be invisible.

// panelBorder is §9.1's left-border-only glyph, the one column that marks where
// the slide-over starts on every row of the frame. It is a literal here because
// internal/tui keeps its own copy unexported; the assertions below are what
// notice if the two ever disagree (the panel would then be unfindable).
const panelBorder = "│"

// panelEntryRefusalCopy is §14A's pinned blocked-entry copy for the two render
// floor dimensions. A frame carrying either is a frame where `t` REFUSED, which
// on a fixture means the render size never cleared task 8-11's entry gate.
var panelEntryRefusalCopy = []string{
	"terminal too narrow for the theme picker",
	"terminal too short for the theme picker",
}

// panelRow is one parsed row of the rendered slide-over: whether the cursor bar
// is on it, its label, and the badge text to its right.
type panelRow struct {
	cursor bool
	label  string
	badge  string
}

// panelRows parses the slide-over out of a rendered frame, keyed by label.
//
// Every panel line begins with the one `border`-coloured column, so the panel's
// own text is everything after the FIRST such glyph on the line. Within it a row
// composes as `[2-cell cursor column][label][pad][badge]` (theme_row.go), so the
// fields are the cursor bar (when present), the label, and the badge's words.
//
// It parses the whole panel rather than only its list, so the header row and the
// footer rows are keyed too — which is what lets an assertion state that the
// `Themes` header is on the frame without a second parser.
func panelRows(t *testing.T, frame string) map[string]panelRow {
	t.Helper()

	rows := make(map[string]panelRow)
	for line := range strings.SplitSeq(ansi.Strip(frame), "\n") {
		_, panel, onPanel := strings.Cut(line, panelBorder)
		if !onPanel {
			continue
		}
		fields := strings.Fields(panel)
		if len(fields) == 0 {
			continue
		}
		row := panelRow{cursor: fields[0] == "▌"}
		if row.cursor {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		rows[fields[0]] = panelRow{cursor: row.cursor, label: fields[0], badge: strings.Join(fields[1:], " ")}
	}
	if len(rows) == 0 {
		t.Fatalf("no panel rows were parsed out of the frame; the slide-over did not render:\n%s", ansi.Strip(frame))
	}
	return rows
}

// panelFixtureFrame renders one panel fixture at the guard's pinned size, driven
// through its own captureKeys by ModelAt — the identical drive a tape performs.
func panelFixtureFrame(t *testing.T, fixture string, palette theme.Theme) string {
	t.Helper()
	return panelFrameAt(t, fixture, palette, harnessWidth, harnessHeight)
}

// TestPanelFixture_AdaptivePairBadges: it renders the adaptive-pair badges.
//
// The frame §9.14 calls the primary reference for the badge vocabulary: two slot
// badges live at once, on different rows, with the cursor on the row the DARK
// slot names — because capturetool runs no light/dark gate, so the standing
// no-answer fallback selects dark (§8.8).
//
// It is unreachable from `--theme` alone, which is exactly why §13.3 separates
// the raw keys from the palette: capturetool always passes the CONSTANT shape,
// and §8.2 makes a non-empty `theme` render a bare `●` with no slot badges.
func TestPanelFixture_AdaptivePairBadges(t *testing.T) {
	// The coherent pairing the fixture's doc comment states: `--theme` names the
	// theme under the cursor, which at open is the dark slot's.
	rows := panelRows(t, panelFixtureFrame(t, "theme-panel-adaptive-pair", builtinPalette(t, "nord")))

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
			t.Error("the cursor is not on nord; §9.2 puts it on the theme actually rendering, which under a pair with no gate is the dark slot's")
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

// TestPanelFixture_ConstantWhilePreviewing: it renders a bare dot while
// previewing another row.
//
// §9.14: "the constant frame completes the panel's specification because the two
// setting states never coexist on screen". It is the picker idiom made visible —
// the `●` is what is PERSISTED, the cursor plus the canvas is what is PREVIEWED,
// and `Esc` would restore the marked one.
//
// §8.2 is what makes the bare `●` correct: a non-empty `theme` wins and the slots
// are NOT READ AT ALL, so there is no slot to qualify the marker with.
func TestPanelFixture_ConstantWhilePreviewing(t *testing.T) {
	// The coherent pairing the fixture's doc comment states: `--theme` names the
	// PREVIEWED theme, deliberately not the marked constant.
	rows := panelRows(t, panelFixtureFrame(t, "theme-panel-constant-previewing", builtinPalette(t, theme.DefaultLightSlug)))

	t.Run("the constant's row carries a bare ●", func(t *testing.T) {
		if got, want := rows["nord"].badge, "●"; got != want {
			t.Errorf("the nord row's badge = %q, want the bare %q", got, want)
		}
	})

	t.Run("no slot badge appears on any list row", func(t *testing.T) {
		for _, label := range panelUnionSlugs() {
			if badge := rows[label].badge; badge != "" && badge != "●" {
				t.Errorf("the %s row carries badge %q; under a constant the slots are not read at all, so the only badge available is the bare ● (§8.2)", label, badge)
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

// TestPanelFixture_CursorSeedDoesNotApplyATheme: it treats the cursor seed as
// placement only.
//
// The seed moves the cursor and NOTHING ELSE. Applying the seeded row's palette
// would make `capturetool --theme <slug|path>` inert on precisely the frames a
// drop-in author most wants to check — the panel's own — which is the one thing
// §13.3 says the flag exists for.
//
// It is driven with a DELIBERATELY INCOHERENT pairing: the fixture seeds its
// cursor onto tokyo-night-day while `--theme` names nord. A coherent frame cannot
// tell the two apart, because the seeded row and the flag would name the same
// palette.
func TestPanelFixture_CursorSeedDoesNotApplyATheme(t *testing.T) {
	pinned := builtinPalette(t, "nord")
	seeded := builtinPalette(t, theme.DefaultLightSlug)

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

// TestPanelFixture_PaletteFollowsTheThemeFlag: it takes its palette from the
// theme flag.
//
// The two inputs are separate by §8.4's own construction — `--theme` pins the
// PALETTE, while the badges and the marked rows derive from the RAW KEYS — so a
// fixture's declared keys must not move when the flag does, and the colours must.
//
// Asserted by rendering ONE fixture under two `--theme` values and diffing, which
// is the only form that can fail for a fixture whose palette is hard-coded
// somewhere inside it: a single render under a single flag agrees with a
// hard-coded palette exactly as often as it agrees with a threaded one.
func TestPanelFixture_PaletteFollowsTheThemeFlag(t *testing.T) {
	first, second := builtinPalette(t, "nord"), builtinPalette(t, theme.DefaultLightSlug)
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

			// The rows are fixture data, not palette data, so they must be identical
			// either side of the swap — which is what makes the colour diff above a
			// statement about the palette rather than about the frame's content.
			if got, want := labelSet(panelRows(t, a)), labelSet(panelRows(t, b)); !slices.Equal(got, want) {
				t.Errorf("the panel lists %v under one palette and %v under the other; the row set is fixture data and must not follow the flag", got, want)
			}
		})
	}
}

// labelSet is a parsed panel's row labels, sorted — the palette-independent half
// of a frame.
func labelSet(rows map[string]panelRow) []string {
	labels := make([]string, 0, len(rows))
	for label := range rows {
		labels = append(labels, label)
	}
	slices.Sort(labels)
	return labels
}

// capturePanelFixtureNames is the pair this task registers.
func capturePanelFixtureNames() []string {
	return []string{"theme-panel-adaptive-pair", "theme-panel-constant-previewing"}
}

// panelUnionSlugs is the §9.4 row set both panel fixtures declare — the three
// built-ins plus one valid drop-in.
//
// Assertions about badges range over THIS rather than over every parsed row,
// because the parser reads the whole slide-over: the vertical keymap footer's own
// rows are `d set as dark` and `l set as light`, which a naive scan for the badge
// vocabulary matches on the footer the panel always renders.
func panelUnionSlugs() []string {
	return []string{"catppuccin-latte", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug}
}

// TestPanelFixture_RegisteredInBothRegistries: it registers in both lists.
//
// FixtureByName's switch and FixtureNames()'s slice are two hand-maintained
// lists, and a fixture present in one and absent from the other is INVISIBLE to
// §13.4's enumerating guard — it exists, it renders, and nothing ever swaps it.
// Absence reads as coverage, which is the shape task 4-3's drift check exists to
// close; this is the same claim stated for the two fixtures by name, so a
// half-registration fails here with the fixture named.
func TestPanelFixture_RegisteredInBothRegistries(t *testing.T) {
	for _, name := range capturePanelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			if _, err := capture.FixtureByName(name); err != nil {
				t.Errorf("FixtureByName(%s): %v — the fixture is not resolvable", name, err)
			}
			if !slices.Contains(capture.FixtureNames(), name) {
				t.Errorf("FixtureNames() %v omits %s — an unenumerated fixture is never driven by the guard", capture.FixtureNames(), name)
			}
		})
	}
}

// TestPanelFixture_UnderTheGuard: it is enumerated by the swap-and-diff guard.
//
// §13.4's guard NEVER NAMES A FIXTURE — it iterates the registry — so the claim
// worth pinning is that these two arrive in the set the guard's assertions range
// over, with no edit to the guard itself.
func TestPanelFixture_UnderTheGuard(t *testing.T) {
	guarded := make([]string, 0, len(capture.FixtureNames()))
	for _, fx := range guardedFixtures(t) {
		guarded = append(guarded, fx.Name())
	}
	for _, name := range capturePanelFixtureNames() {
		if !slices.Contains(guarded, name) {
			t.Errorf("%s is not in the guarded set %v; the guard enumerates, so a fixture it never sees is uncovered while reading as covered", name, guarded)
		}
	}
}

// TestPanelFixture_PanelIsCompositedUnderTheGuard: it composites the panel under
// the guard's pinned size.
//
// PROVEN, NOT ASSUMED. §13.4's guard renders every enumerated fixture at task
// 4-3's single pinned size and drives it through ModelAt, which replays
// captureKeys — so a panel fixture's `t` passes through task 8-13's entry gate
// there exactly as it does under a tape. If that constant failed task 8-11's
// floor (width below the panel minimum, or height below header + footer + one
// list row + one message row), the guard would render a PANEL-LESS frame carrying
// a blocked-`t` flash and EVERY ASSERTION WOULD STILL PASS: the panel's own
// bubbles/list instance would be covered by nothing while reading as covered.
//
// So the A-frame must carry the panel's left border and its `Themes` header. If
// this fails, RAISE task 4-3's pinned size rather than weakening the assertion,
// and record the panel floor among the reasons the value is what it is.
func TestPanelFixture_PanelIsCompositedUnderTheGuard(t *testing.T) {
	a, b := syntheticPalettes()

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

// builtinPalette loads one shipped built-in. §7.6's build-time guarantee makes a
// rejection impossible, so it is a Fatal rather than a fallback.
func builtinPalette(t *testing.T, slug string) theme.Theme {
	t.Helper()
	result, rejection, found := theme.NewLoader(nil).LoadBuiltin(slug)
	if !found {
		t.Fatalf("built-in %q not found in the embedded set", slug)
	}
	if rejection != nil {
		t.Fatalf("built-in %q was rejected: %s", slug, rejection.Reason)
	}
	return result.Theme
}
