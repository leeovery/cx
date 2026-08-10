package tui

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

// These tests pin the §9.1 peek-mode marker, the §9.3 footer nav-hint content
// (derived from the previewKeymap descriptor), and the role token the preview
// chrome resolves. Drift is caught loudly — the spec is the source of truth and
// any change to these literals must be a deliberate spec update.

// TestPreviewFooterCanonicalByteContent pins the §9.1 footer's exact stripped
// content: `←→ window  ⇥ pane  ⏎ attach  ␣ back` — glyphs + labels,
// space-separated (no middots), in descriptor order.
func TestPreviewFooterCanonicalByteContent(t *testing.T) {
	const want = "←→ window  ⇥ pane  ⏎ attach  ␣ back"
	got := stripANSI(composePreviewFooterRow(200, testDarkTheme(t), false))
	if got != want {
		t.Errorf("preview footer = %q, want %q", got, want)
	}
}

// TestPreviewFooterNoMiddots pins the shared footer convention: the §9.1 footer
// is SPACE-separated, never middot-separated (the verbose-bar middots were
// dropped in the §9 restructure).
func TestPreviewFooterNoMiddots(t *testing.T) {
	got := stripANSI(composePreviewFooterRow(200, testDarkTheme(t), false))
	if strings.ContainsRune(got, '·') {
		t.Errorf("preview footer contains a middot U+00B7; want space-separated only: %q", got)
	}
}

// TestPreviewFooterCompactGlyphsOnly pins the narrow-width cascade: when the
// labelled form does not fit, the footer compacts to accent.key glyphs only,
// dropping the labels but keeping every nav-hint glyph present.
func TestPreviewFooterCompactGlyphsOnly(t *testing.T) {
	// A content width too narrow for the labelled form (~38 cells) but wide
	// enough for the full compact glyph form (13 cells) forces the compact path.
	got := stripANSI(composePreviewFooterRow(20, testDarkTheme(t), false))
	const want = "←→  ⇥  ⏎  ␣"
	if got != want {
		t.Errorf("compact preview footer = %q, want %q", got, want)
	}
	for _, label := range []string{"window", "pane", "attach", "back"} {
		if strings.Contains(got, label) {
			t.Errorf("compact preview footer must drop labels; found %q in %q", label, got)
		}
	}
}

func TestPreviewMarkerExactByteContent(t *testing.T) {
	want := "◉ preview"
	if previewMarker != want {
		t.Errorf("previewMarker = %q, want %q", previewMarker, want)
	}
}

// TestPreviewBorderResolvesTheModeAccentToken pins that the §9.1 preview chrome
// border resolves the accent.mode role token off the ACTIVE THEME — never a raw
// hex, and never a token copied at package init (§11.2's named offender, whose
// package-scope copy is gone and whose return TestNoPackageLevelThemeVar blocks).
// The retired explicit border hex `#7B95BD` must not reappear either.
func TestPreviewBorderResolvesTheModeAccentToken(t *testing.T) {
	th := testDarkTheme(t)
	m := newPeekPreviewModel(t, "work", []tmux.WindowGroup{
		{WindowIndex: 0, WindowName: "editor", PaneIndices: []int{0}},
	}, []byte("hello\n"), 80, 24)
	m.th = th
	frame := m.View()

	if seq := tokenFgSeq(t, th.AccentMode); !strings.Contains(frame, seq) {
		t.Errorf("preview frame does not draw its border from accent.mode (SGR %q)", seq)
	}
	if th.AccentMode.Value == "#7B95BD" {
		t.Errorf("accent.mode = %q; the retired explicit border hex must not survive", th.AccentMode.Value)
	}
}
