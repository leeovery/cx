package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

const testSessionName = "nvim-editor"

func chl(t *testing.T, w int, name string) string {
	t.Helper()
	return stripANSI(composePreviewHeaderRow(w, 0, 1, 0, 1, name, testDarkTheme(t), false))
}

func TestComposePreviewHeaderRow_NoEmbeddedNewlines(t *testing.T) {
	for _, w := range []int{200, 80, 60, 40, 25, 15, 10, 4, 1, 0} {
		got := composePreviewHeaderRow(w, 0, 1, 0, 1, testSessionName, testDarkTheme(t), false)
		if n := strings.Count(got, "\n"); n != 0 {
			t.Errorf("composePreviewHeaderRow(width=%d) returned %d embedded newline(s); want 0; got=%q", w, n, got)
		}
	}
}

func TestComposePreviewHeaderRow_FitsWithinContentWidth(t *testing.T) {
	for _, w := range []int{200, 80, 60, 40, 25, 18, 13, 12, 11, 8, 4, 2, 1} {
		got := composePreviewHeaderRow(w, 0, 1, 0, 1, testSessionName, testDarkTheme(t), false)
		if width := lipgloss.Width(got); width > w {
			t.Errorf("content width %d: header width = %d, want <= %d; got=%q", w, width, w, stripANSI(got))
		}
	}
}

func TestComposePreviewHeaderRow_Tier1FullAtWideWidth(t *testing.T) {
	got := chl(t, 200, testSessionName)
	for _, want := range []string{previewMarker, testSessionName, "Window 1/1 · Pane 1/1"} {
		if !strings.Contains(got, want) {
			t.Errorf("tier 1 wide: expected substring %q; got=%q", want, got)
		}
	}
	if strings.Contains(got, "…") {
		t.Errorf("tier 1 wide: expected no ellipsis on full-name tier; got=%q", got)
	}
}

func TestComposePreviewHeaderRow_Tier1BoundaryFullFit(t *testing.T) {
	counters := "Window 1/1 · Pane 1/1"
	fullW := lipgloss.Width(previewMarker) + 1 + lipgloss.Width(testSessionName) + 1 + lipgloss.Width(counters)
	got := chl(t, fullW, testSessionName)
	if !strings.Contains(got, testSessionName) {
		t.Errorf("tier 1 boundary w %d: expected full session %q; got=%q", fullW, testSessionName, got)
	}
	if strings.Contains(got, "…") {
		t.Errorf("tier 1 boundary w %d: expected no ellipsis; got=%q", fullW, got)
	}
	if !strings.Contains(got, counters) {
		t.Errorf("tier 1 boundary w %d: expected counters; got=%q", fullW, got)
	}
}

func TestComposePreviewHeaderRow_Tier2TruncatesSessionKeepsCounters(t *testing.T) {
	counters := "Window 1/1 · Pane 1/1"
	fullW := lipgloss.Width(previewMarker) + 1 + lipgloss.Width(testSessionName) + 1 + lipgloss.Width(counters)
	w := fullW - 1
	got := chl(t, w, testSessionName)
	if !strings.Contains(got, "…") {
		t.Errorf("tier 2 w %d: expected truncated session with ellipsis; got=%q", w, got)
	}
	if !strings.Contains(got, counters) {
		t.Errorf("tier 2 w %d: expected counters present; got=%q", w, got)
	}
}

func TestComposePreviewHeaderRow_Tier3DropsCountersKeepsFullSession(t *testing.T) {
	w := lipgloss.Width(previewMarker) + 1 + lipgloss.Width(testSessionName)
	got := chl(t, w, testSessionName)
	if strings.Contains(got, "Window") {
		t.Errorf("tier 3 w %d: expected NO counters segment; got=%q", w, got)
	}
	if !strings.Contains(got, testSessionName) {
		t.Errorf("tier 3 w %d: expected session %q present; got=%q", w, testSessionName, got)
	}
	if strings.Contains(got, "…") {
		t.Errorf("tier 3 w %d: expected no ellipsis on full session; got=%q", w, got)
	}
}

func TestComposePreviewHeaderRow_Tier4TruncatesSessionNoCounters(t *testing.T) {
	w := lipgloss.Width(previewMarker) + 1 + lipgloss.Width(testSessionName) - 1
	got := chl(t, w, testSessionName)
	if strings.Contains(got, "Window") {
		t.Errorf("tier 4 w %d: expected NO counters; got=%q", w, got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("tier 4 w %d: expected truncated session with ellipsis; got=%q", w, got)
	}
	if !strings.Contains(got, previewMarker) {
		t.Errorf("tier 4 w %d: expected marker present; got=%q", w, got)
	}
}

func TestComposePreviewHeaderRow_AlwaysCarriesMarker(t *testing.T) {
	for _, w := range []int{200, 60, 30, 18, 13, 11} {
		got := chl(t, w, testSessionName)
		if !strings.Contains(got, previewMarker) {
			t.Errorf("width %d: header dropped the marker; got=%q", w, got)
		}
	}
}
