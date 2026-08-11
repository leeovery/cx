package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

const (
	nordSlug   = "nord"
	nordCanvas = "#2E3440"
)

func testNordTheme(t *testing.T) theme.Theme {
	t.Helper()
	th := themetest.Builtin(t, nordSlug)
	if got := th.Canvas.Value; got != nordCanvas {
		t.Fatalf("nord built-in canvas = %q, want %q", got, nordCanvas)
	}
	return th
}

func startupModel(t *testing.T, startup theme.Theme) Model {
	t.Helper()
	m := newSwapFrameModel(t, startup, false)
	if got := m.themeState.startupCanvasHex; got != startup.Canvas.Value {
		t.Fatalf("startupCanvasHex = %q at construction, want the startup canvas %q — these cases rest on the hex being captured by the gate, not set by the test", got, startup.Canvas.Value)
	}
	return m
}

func withCapturedOriginal(m Model, original string) Model {
	m.originalBg = original
	return m
}

func assertNothingWritten(t *testing.T, m Model, why string) {
	t.Helper()
	var b strings.Builder
	RestoreTerminalBackground(&b, m)
	if got := b.String(); got != "" {
		t.Errorf("captured original %q (startup canvas %s, active canvas %s): wrote %q, want nothing — %s",
			m.OriginalBackground(), m.themeState.startupCanvasHex, m.themeState.active.Canvas.Value, escSeq(got), why)
	}
}

func assertSkipped(t *testing.T, m Model) {
	t.Helper()
	assertNothingWritten(t, m, "an echo of the STARTUP canvas must be skipped whatever theme is active")
}

func assertSetBack(t *testing.T, m Model) {
	t.Helper()
	var b strings.Builder
	RestoreTerminalBackground(&b, m)
	want := ansi.SetBackgroundColor(m.OriginalBackground())
	if got := b.String(); got != want {
		t.Errorf("captured original %q (startup canvas %s, active canvas %s): wrote %q, want %q — the comparison is anchored to the STARTUP canvas, so anything else is a genuine original and must be set back",
			m.OriginalBackground(), m.themeState.startupCanvasHex, m.themeState.active.Canvas.Value, escSeq(got), escSeq(want))
	}
}

func TestRestoreBackground_CommittedThemeDivergence(t *testing.T) {
	startup, committed := testDarkTheme(t), testLightTheme(t)
	m := startupModel(t, startup)

	m.ApplyTheme(committed)
	if got := m.themeState.active.Canvas.Value; got != testLightThemeCanvas {
		t.Fatalf("active canvas = %q after the commit, want %q — without the divergence there is nothing to test, only restore_test.go's skip/emit pair repeated", got, testLightThemeCanvas)
	}

	t.Run("an echo of the startup canvas is still skipped", func(t *testing.T) {
		assertSkipped(t, withCapturedOriginal(m, strings.ToLower(testDarkThemeCanvas)))
	})

	t.Run("the committed theme's canvas is a genuine original and is set back", func(t *testing.T) {
		assertSetBack(t, withCapturedOriginal(m, testLightThemeCanvas))
	})

	t.Run("the startup hex did not move with the commit", func(t *testing.T) {
		if got := m.themeState.startupCanvasHex; got != testDarkThemeCanvas {
			t.Errorf("startupCanvasHex = %q after committing %s, want %q — it is frozen at gate resolution",
				got, themeLabel(committed), testDarkThemeCanvas)
		}
	})
}

func TestRestoreBackground_UncommittedPreviewDivergence(t *testing.T) {
	startup := testDarkTheme(t)
	previewed := testNordTheme(t)
	m := startupModel(t, startup)

	for i, th := range []theme.Theme{testLightTheme(t), startup, previewed} {
		m.ApplyTheme(th)
		if got := m.themeState.startupCanvasHex; got != testDarkThemeCanvas {
			t.Fatalf("startupCanvasHex = %q after preview %d of 3 (%s), want %q — a preview commits nothing and moves nothing",
				got, i+1, themeLabel(th), testDarkThemeCanvas)
		}
	}
	if got := m.themeState.active.Canvas.Value; got != nordCanvas {
		t.Fatalf("active canvas = %q after the preview run, want %q — the run must END on the previewed theme, or there is no divergence to test", got, nordCanvas)
	}

	t.Run("the previewed canvas the user never chose is set back", func(t *testing.T) {
		assertSetBack(t, withCapturedOriginal(m, nordCanvas))
	})

	t.Run("an echo of the startup canvas is still skipped", func(t *testing.T) {
		assertSkipped(t, withCapturedOriginal(m, strings.ToLower(testDarkThemeCanvas)))
	})
}

func TestRestoreBackground_ActiveCanvasEqualsOriginalStillSetsBack(t *testing.T) {
	startup := testDarkTheme(t)
	landedOn := testNordTheme(t)
	m := startupModel(t, startup)

	updated, _ := m.Update(tea.BackgroundColorMsg{Color: landedOn.Canvas.Color()})
	m = updated.(Model)
	if got := m.OriginalBackground(); !strings.EqualFold(got, nordCanvas) {
		t.Fatalf("captured original = %q, want %s (modulo case) — the reply must be captured, or this case asserts nothing", got, nordCanvas)
	}

	m.ApplyTheme(landedOn)

	if got, want := m.themeState.active.Canvas.Value, m.OriginalBackground(); !strings.EqualFold(got, want) {
		t.Fatalf("active canvas %q and captured original %q differ; this case only exists while they coincide", got, want)
	}
	if strings.EqualFold(m.themeState.startupCanvasHex, m.OriginalBackground()) {
		t.Fatalf("startup canvas %q equals the captured original; then the guard should SKIP and the case is inverted", m.themeState.startupCanvasHex)
	}
	assertSetBack(t, m)
}

func TestRestoreBackground_StartupHexSurvivesSwaps(t *testing.T) {
	startup := testDarkTheme(t)
	light, nord := testLightTheme(t), testNordTheme(t)
	m := startupModel(t, startup)

	echo := strings.ToLower(testDarkThemeCanvas)
	for i, th := range []theme.Theme{light, nord, startup, nord, light} {
		m.ApplyTheme(th)

		if got := m.themeState.startupCanvasHex; got != testDarkThemeCanvas {
			t.Fatalf("startupCanvasHex = %q after swap %d (%s), want %q unchanged", got, i+1, themeLabel(th), testDarkThemeCanvas)
		}
		assertSkipped(t, withCapturedOriginal(m, echo))
	}
}

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

func TestRestoreBackground_NonHexReplyAfterSwapStillSetsBack(t *testing.T) {
	const rgbReply = "rgb:0b0b/0c0c/1414"

	startup := testDarkTheme(t)
	m := startupModel(t, startup)
	m.ApplyTheme(testNordTheme(t))

	assertSetBack(t, withCapturedOriginal(m, rgbReply))
}

func TestRestoreBackground_EmptyCaptureAfterSwapWritesNothing(t *testing.T) {
	m := startupModel(t, testDarkTheme(t))
	if got := m.OriginalBackground(); got != "" {
		t.Fatalf("probe setup: captured original = %q before any reply, want empty — this case only says anything while nothing has been captured", got)
	}

	m.ApplyTheme(testNordTheme(t))
	if got := m.themeState.active.Canvas.Value; got != nordCanvas {
		t.Fatalf("active canvas = %q after the preview, want %q — without the swap this is the un-swapped case repeated", got, nordCanvas)
	}

	assertNothingWritten(t, m, "nothing was ever captured, so there is nothing to set back — and a swap must not invent one")
}
