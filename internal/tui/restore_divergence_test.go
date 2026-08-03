package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// This file is the behavioural half of §11.4's named verification, driven
// through the PRODUCTION swap path.
//
// restore_test.go's TestRestoreTerminalBackground_AnchoredToStartupHex proved the
// ANCHOR — that RestoreTerminalBackground compares against the retained startup
// canvas — by moving activeTheme directly, because when it was written no
// mid-session swap existed to move it any other way. The two cases §11.4 actually
// names are DIVERGENCES, and neither was reachable then:
//
//   - A theme COMMITTED mid-session, so the active theme is one the user chose
//     but the startup window never painted.
//   - A quit with an UNCOMMITTED PREVIEW active — Ctrl-C with the panel open
//     (§9.7). The active theme is the previewed one, which the user never
//     persisted and which a naive implementation would compare against. §11.4
//     calls this the likelier mistake of the two, and the only path on which a
//     colour the user never chose can be left stuck in the terminal after Portal
//     exits.
//
// Model.ApplyTheme (§11.1) made both reachable, so every swap below goes through
// it — never by assigning activeTheme, and never by writing startupCanvasHex,
// which must be produced by construction alone (§8.4). That is also what makes
// these cases say anything about Phases 8-9: the panel drives this exact entry
// point, so the mechanism proven here is the mechanism it uses.
//
// The panel does not exist until Phase 8, so "uncommitted preview" is modelled as
// what an arrow keypress does and nothing else — a swap, with no commit, no prefs
// write and no panel state.
//
// NO_COLOR gets no divergence case, and could not have a useful one. The startup
// hex is captured as normal from the selected member (§9.10), but the exit path
// returns on m.colourless before it compares anything — so a swapped NO_COLOR
// case would assert zero bytes under the anchored comparison AND under the naive
// one, distinguishing nothing. restore_test.go's
// TestNoColor_HexCapturedAndSetBackIsANoOp already pins the carve-out itself.

const (
	// nordSlug names the third built-in — the one neither half of the shipped
	// adaptive pair, so it is the theme these cases preview or commit TO.
	nordSlug = "nord"
	// nordCanvas is that built-in's canvas (§7.4's Nord port), restated so the
	// assertions below name a value a reader can check against the embedded file
	// rather than one derived from the same place they read it.
	nordCanvas = "#2E3440"
)

// testNordTheme loads the Nord built-in through the production loader, pinning
// its canvas so an edit to the embedded file fails here rather than silently
// leaving these cases comparing a value nothing names.
func testNordTheme(t *testing.T) theme.Theme {
	t.Helper()
	th := testBuiltinTheme(t, nordSlug)
	if got := th.Canvas.Value; got != nordCanvas {
		t.Fatalf("nord built-in canvas = %q, want %q", got, nordCanvas)
	}
	return th
}

// startupModel builds the production-shaped model every case here diverges FROM:
// a CONSTANT nomination of `startup`, so the §2.6 gate is resolved at
// construction and the startup canvas hex is captured there (§8.4), with one full
// frame rendered under it.
//
// The rendered frame is not decoration. It is the frame that, in production, set
// the terminal's default background to `startup`'s canvas — which is what makes
// that canvas the one the exit path must restore FROM, however many themes are
// applied afterwards.
//
// The hex is asserted to have come from CONSTRUCTION, not from the test: nothing
// below writes startupCanvasHex, so a regression that starts writing it on swap
// has nowhere to hide.
func startupModel(t *testing.T, startup theme.Theme) Model {
	t.Helper()
	m := newSwapFrameModel(t, startup, false)
	if got := m.startupCanvasHex; got != startup.Canvas.Value {
		t.Fatalf("startupCanvasHex = %q at construction, want the startup canvas %q — these cases rest on the hex being captured by the gate, not set by the test", got, startup.Canvas.Value)
	}
	return m
}

// withCapturedOriginal returns a COPY of m carrying `original` as the terminal's
// captured OSC 11 reply.
//
// It sets the field directly because three of the four echo shapes below (upper
// case, a trailing alpha pair, a missing '#') are shapes a TERMINAL sends and
// that the tea.BackgroundColorMsg path cannot produce — Update renders every
// reply as lower-case #rrggbb. The one case whose original must arrive exactly as
// production captures it — the genuine terminal background of
// TestRestoreBackground_ActiveCanvasEqualsOriginalStillSetsBack — is routed
// through Update instead.
//
// The copy is what lets one post-swap model serve several cases without them
// leaking into each other: Model is a value, and the exit path takes it by value
// too.
func withCapturedOriginal(m Model, original string) Model {
	m.originalBg = original
	return m
}

// assertNothingWritten asserts the exit path writes NOTHING for the model as it
// stands, naming in the failure WHY nothing was owed.
//
// The exit path has two silent returns and a swap can reach both, so the reason is
// a parameter rather than a sentence baked into one message: a case asserting the
// echo guard and a case asserting the no-capture return would otherwise fail with
// each other's contract.
func assertNothingWritten(t *testing.T, m Model, why string) {
	t.Helper()
	var b strings.Builder
	RestoreTerminalBackground(&b, m)
	if got := b.String(); got != "" {
		t.Errorf("captured original %q (startup canvas %s, active canvas %s): wrote %q, want nothing — %s",
			m.OriginalBackground(), m.startupCanvasHex, m.activeTheme.Canvas.Value, escSeq(got), why)
	}
}

// assertSkipped asserts the canvas-echo guard fires for the original m carries:
// the terminal echoed the STARTUP canvas back, and setting it back would re-paint
// the canvas after Bubble Tea's OSC 111 reset and leave it stuck.
func assertSkipped(t *testing.T, m Model) {
	t.Helper()
	assertNothingWritten(t, m, "an echo of the STARTUP canvas must be skipped whatever theme is active")
}

// assertSetBack asserts the exit path writes exactly the OSC 11 SET of the
// original m carries — the answer for every original that is not an echo of the
// STARTUP canvas, however it compares against the ACTIVE theme's.
//
// The expectation is derived from the captured original rather than restated, so
// what is asserted is the contract itself: set back what the terminal reported.
func assertSetBack(t *testing.T, m Model) {
	t.Helper()
	var b strings.Builder
	RestoreTerminalBackground(&b, m)
	want := ansi.SetBackgroundColor(m.OriginalBackground())
	if got := b.String(); got != want {
		t.Errorf("captured original %q (startup canvas %s, active canvas %s): wrote %q, want %q — the comparison is anchored to the STARTUP canvas, so anything else is a genuine original and must be set back",
			m.OriginalBackground(), m.startupCanvasHex, m.activeTheme.Canvas.Value, escSeq(got), escSeq(want))
	}
}

// TestRestoreBackground_CommittedThemeDivergence is §11.4's first divergence
// case: a theme committed mid-session, so the active theme is one the startup
// window never painted. Startup is tokyo-night (#0B0C14); tokyo-night-day
// (#E1E2E7) is committed over it through ApplyTheme.
//
// Both directions are asserted, and a comparison re-derived from the active theme
// fails both — measured, not assumed. They are not equally consequential:
//
//   - The echo assertion is where the visible harm lives. A naive implementation
//     compares the echoed #0b0c14 against the now-active #E1E2E7, sees a
//     difference, and WRITES — re-painting the startup canvas AFTER Bubble Tea's
//     OSC 111 reset, which is the Ghostty leak the guard was added for. It sticks
//     on any terminal that honours an OSC 11 set, reset-honouring ones included,
//     because the write lands after the program has already returned.
//   - The set-back assertion catches the same naive comparison from the other
//     side: it would skip a write the anchored one makes. Its harm is bounded —
//     an original equal to the ACTIVE canvas cannot be a race echo (the query is
//     issued once, before the swap, §11.3), so it is a genuine background that
//     coincides with that canvas, and what stays on screen matches it either way.
//     The implementation is wrong regardless, and #E1E2E7 is the value a naive one
//     is written against.
func TestRestoreBackground_CommittedThemeDivergence(t *testing.T) {
	startup, committed := testDarkTheme(t), testLightTheme(t)
	m := startupModel(t, startup)

	m.ApplyTheme(committed)
	if got := m.activeTheme.Canvas.Value; got != testLightThemeCanvas {
		t.Fatalf("active canvas = %q after the commit, want %q — without the divergence there is nothing to test, only restore_test.go's skip/emit pair repeated", got, testLightThemeCanvas)
	}

	t.Run("an echo of the startup canvas is still skipped", func(t *testing.T) {
		assertSkipped(t, withCapturedOriginal(m, strings.ToLower(testDarkThemeCanvas)))
	})

	t.Run("the committed theme's canvas is a genuine original and is set back", func(t *testing.T) {
		assertSetBack(t, withCapturedOriginal(m, testLightThemeCanvas))
	})

	t.Run("the startup hex did not move with the commit", func(t *testing.T) {
		if got := m.startupCanvasHex; got != testDarkThemeCanvas {
			t.Errorf("startupCanvasHex = %q after committing %s, want %q — it is frozen at gate resolution (§11.4)",
				got, themeLabel(committed), testDarkThemeCanvas)
		}
	})
}

// TestRestoreBackground_UncommittedPreviewDivergence is §11.4's second and
// likelier divergence case: Ctrl-C with the panel open, so the active theme is
// one the user PREVIEWED and never persisted.
//
// The panel arrives in Phase 8; until then an arrow-preview is exactly a swap
// with no commit, so the run below is three ApplyTheme calls and nothing else —
// no panel state, no prefs write, no commit path. It arrows onto the light
// built-in, back onto the startup theme (a real swap — the active theme there is
// the light one) and finally onto Nord, then quits there. Passing back THROUGH
// the startup theme mid-run is deliberate: a hex re-derived on swap would be
// momentarily correct at that step and wrong at the last, so it cannot hide
// behind the coincidence.
//
// The set-back assertion is the divergence §11.4 names — at exit the active theme
// is one the user never persisted, and a naive comparison against that Nord canvas
// skips a write the anchored one makes. As in the committed case the two
// assertions catch it from opposite sides, and it is again the ECHO one carrying
// the visible harm.
func TestRestoreBackground_UncommittedPreviewDivergence(t *testing.T) {
	startup := testDarkTheme(t)
	previewed := testNordTheme(t)
	m := startupModel(t, startup)

	for i, th := range []theme.Theme{testLightTheme(t), startup, previewed} {
		m.ApplyTheme(th)
		if got := m.startupCanvasHex; got != testDarkThemeCanvas {
			t.Fatalf("startupCanvasHex = %q after preview %d of 3 (%s), want %q — a preview commits nothing and moves nothing",
				got, i+1, themeLabel(th), testDarkThemeCanvas)
		}
	}
	if got := m.activeTheme.Canvas.Value; got != nordCanvas {
		t.Fatalf("active canvas = %q after the preview run, want %q — the run must END on the previewed theme, or there is no divergence to test", got, nordCanvas)
	}

	t.Run("the previewed canvas the user never chose is set back", func(t *testing.T) {
		assertSetBack(t, withCapturedOriginal(m, nordCanvas))
	})

	t.Run("an echo of the startup canvas is still skipped", func(t *testing.T) {
		assertSkipped(t, withCapturedOriginal(m, strings.ToLower(testDarkThemeCanvas)))
	})
}

// TestRestoreBackground_ActiveCanvasEqualsOriginalStillSetsBack pins the case a
// naive implementation looks most defensible in: the terminal's GENUINE original
// happens to be the canvas of the theme now active. A naive comparison sees
// original == active canvas and skips; the anchored one compares against the
// startup canvas, finds no match, and sets back.
//
// It is not folded into a table because the setup is what carries the meaning:
// the original arrives the way production captures one — an OSC 11 reply routed
// through Update, before any swap, since the query is issued once from Init and a
// later switch issues no new query (§11.3). Every other case here hands the field
// a shape a terminal sends; this one is the wire value itself.
//
// The rule it pins is the same rule the two divergence cases above pin, and a
// naive implementation fails those too — measured, not assumed. What is
// different here is the framing: this is the one shape where the naive answer is
// visually INDISTINGUISHABLE from the correct one, because the canvas it leaves
// standing IS the terminal's own background. A terminal honouring OSC 111 resets
// to that background; one ignoring it keeps the active canvas, which equals it.
// So this is where a re-derived comparison is likeliest to be written and least
// likely to be caught by looking at a terminal.
func TestRestoreBackground_ActiveCanvasEqualsOriginalStillSetsBack(t *testing.T) {
	startup := testDarkTheme(t)
	landedOn := testNordTheme(t)
	m := startupModel(t, startup)

	// The user's terminal really is Nord-coloured, and says so.
	updated, _ := m.Update(tea.BackgroundColorMsg{Color: landedOn.Canvas.Color()})
	m = updated.(Model)
	if got := m.OriginalBackground(); !strings.EqualFold(got, nordCanvas) {
		t.Fatalf("captured original = %q, want %s (modulo case) — the reply must be captured, or this case asserts nothing", got, nordCanvas)
	}

	m.ApplyTheme(landedOn)

	if got, want := m.activeTheme.Canvas.Value, m.OriginalBackground(); !strings.EqualFold(got, want) {
		t.Fatalf("active canvas %q and captured original %q differ; this case only exists while they coincide", got, want)
	}
	if strings.EqualFold(m.startupCanvasHex, m.OriginalBackground()) {
		t.Fatalf("startup canvas %q equals the captured original; then the guard should SKIP and the case is inverted", m.startupCanvasHex)
	}
	assertSetBack(t, m)
}

// TestRestoreBackground_StartupHexSurvivesSwaps walks a long arrowing run over
// every built-in — including a return to the startup theme — and asserts the
// retained hex never moves, pairing each step with the exit-path behaviour it
// anchors.
//
// apply_theme_test.go's TestApplyTheme_DoesNotMoveStartupCanvasHex asserts the
// same non-movement from the SWAP side, over synthetic palettes and the field
// alone. This asserts it from the RESTORE side, over the built-ins the panel
// enumerates, with the observable consequence attached at every step: a hex that
// drifted onto the active theme would stop skipping the echo the moment it moved.
func TestRestoreBackground_StartupHexSurvivesSwaps(t *testing.T) {
	startup := testDarkTheme(t)
	light, nord := testLightTheme(t), testNordTheme(t)
	m := startupModel(t, startup)

	echo := strings.ToLower(testDarkThemeCanvas)
	for i, th := range []theme.Theme{light, nord, startup, nord, light} {
		m.ApplyTheme(th)

		if got := m.startupCanvasHex; got != testDarkThemeCanvas {
			t.Fatalf("startupCanvasHex = %q after swap %d (%s), want %q unchanged", got, i+1, themeLabel(th), testDarkThemeCanvas)
		}
		assertSkipped(t, withCapturedOriginal(m, echo))
	}
}

// TestRestoreBackground_EchoGuardShapesAfterSwap keeps the four reply shapes
// covered through the swap path. A terminal echoes the canvas back in whatever
// shape it likes while §4.3 canonicalises the retained hex to upper-case
// #RRGGBB, so all four must still resolve to the same RGB triple and skip — after
// any number of swaps, since the guard reads a value no swap touches.
func TestRestoreBackground_EchoGuardShapesAfterSwap(t *testing.T) {
	startup := testDarkTheme(t)
	m := startupModel(t, startup)
	m.ApplyTheme(testLightTheme(t))
	m.ApplyTheme(testNordTheme(t))

	lower := strings.ToLower(testDarkThemeCanvas)
	for _, tc := range []struct {
		name     string
		original string
	}{
		{"lower case", lower},
		{"upper case, as the retained hex is canonicalised", testDarkThemeCanvas},
		{"trailing alpha", lower + "ff"},
		{"no leading hash", strings.TrimPrefix(lower, "#")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertSkipped(t, withCapturedOriginal(m, tc.original))
		})
	}
}

// TestRestoreBackground_NonHexReplyAfterSwapStillSetsBack keeps the fall-through
// covered through the swap path: a terminal answering OSC 11 in the `rgb:` form
// cannot be compared against the retained hex, so the guard declines to match and
// the set-back is emitted unchanged — never worse than before the guard existed,
// and no more affected by a swap than the hex shapes are.
//
// The other fall-through — an empty capture — gets its own post-swap case below
// rather than a claim here, because a swap can decide whether the original is
// empty in a way the `rgb:` shape cannot.
func TestRestoreBackground_NonHexReplyAfterSwapStillSetsBack(t *testing.T) {
	const rgbReply = "rgb:0b0b/0c0c/1414" // the startup canvas, in the shape sameHexColour cannot read

	startup := testDarkTheme(t)
	m := startupModel(t, startup)
	m.ApplyTheme(testNordTheme(t))

	assertSetBack(t, withCapturedOriginal(m, rgbReply))
}

// TestRestoreBackground_EmptyCaptureAfterSwapWritesNothing keeps the no-capture
// return covered through the swap path: a model that never received an OSC 11
// reply still writes nothing once a theme has been previewed over it.
//
// This is NOT background_restore_test.go's TestRestoreTerminalBackground_EmptyWritesNothing
// repeated. That one holds an un-swapped model, and the exit path is not the only
// thing deciding whether the original is empty — the SWAP path can populate it. An
// ApplyTheme that also refreshed the captured original from the palette it is
// handed (the plausible "keep the capture current" edit) would leave this model
// reporting the PREVIEWED canvas as the terminal's original, and the exit path
// would then write #2E3440 after Bubble Tea's OSC 111 reset: §11.4's named harm
// exactly, a colour the user never chose left stuck after Portal exits. The
// anchored comparison cannot catch that one — the value it is handed is a lie
// before it ever reaches the guard.
//
// So no original is handed in after the swap, deliberately. Passing one would
// overwrite whatever the swap wrote and the regression would have nowhere to show.
func TestRestoreBackground_EmptyCaptureAfterSwapWritesNothing(t *testing.T) {
	m := startupModel(t, testDarkTheme(t))
	if got := m.OriginalBackground(); got != "" {
		t.Fatalf("probe setup: captured original = %q before any reply, want empty — this case only says anything while nothing has been captured", got)
	}

	m.ApplyTheme(testNordTheme(t))
	if got := m.activeTheme.Canvas.Value; got != nordCanvas {
		t.Fatalf("active canvas = %q after the preview, want %q — without the swap this is the un-swapped case repeated", got, nordCanvas)
	}

	assertNothingWritten(t, m, "nothing was ever captured, so there is nothing to set back — and a swap must not invent one")
}
