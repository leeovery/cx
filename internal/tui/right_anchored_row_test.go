package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

const rightAnchoredCanvasWidth = 120

func TestAssembleRightAnchoredRow_WideEmitsClusterSpacerAnchor(t *testing.T) {
	const w = rightAnchoredCanvasWidth
	left := renderFooterDetail("left", testDarkTheme(t), false)
	leftWidth := lipgloss.Width(left)
	rightSeg := renderFooterDetail("? help", testDarkTheme(t), false)
	rightWidth := lipgloss.Width(rightSeg)

	row := assembleRightAnchoredRow(left, leftWidth, rightSeg, rightWidth, w, testDarkTheme(t), false)

	if got := lipgloss.Width(row); got != w {
		t.Errorf("wide row width = %d, want exactly %d", got, w)
	}
	vis := footerVisible(row)
	if !strings.HasPrefix(vis, "left") {
		t.Errorf("wide row must lead with the left cluster:\n%q", vis)
	}
	if !strings.HasSuffix(vis, "? help") {
		t.Errorf("wide row must end with the right anchor:\n%q", vis)
	}
	gap := strings.TrimSuffix(strings.TrimPrefix(vis, "left"), "? help")
	if len(gap) <= 1 {
		t.Errorf("wide row flex spacer too narrow (%d cells): %q", len(gap), vis)
	}
}

func TestAssembleRightAnchoredRow_NarrowDegradeKeepsAnchorDropsCluster(t *testing.T) {
	left := renderFooterDetail("left", testDarkTheme(t), false)
	leftWidth := lipgloss.Width(left)
	rightSeg := renderFooterDetail("? help", testDarkTheme(t), false)
	rightWidth := lipgloss.Width(rightSeg)

	w := leftWidth + rightWidth
	if leftWidth+1+rightWidth <= w {
		t.Fatalf("test setup: width %d is not at/below the degrade boundary", w)
	}

	row := assembleRightAnchoredRow(left, leftWidth, rightSeg, rightWidth, w, testDarkTheme(t), false)

	want := headerPadLeft(rightSeg, rightWidth, w, testDarkTheme(t), false)
	if row != want {
		t.Errorf("narrow-degrade row != headerPadLeft(rightSeg, …):\n got=%q\nwant=%q", row, want)
	}
	if !strings.Contains(footerVisible(row), "? help") {
		t.Errorf("narrow-degrade row must KEEP the ? help anchor (§14.4):\n%q", footerVisible(row))
	}
	if strings.Contains(footerVisible(row), "left") {
		t.Errorf("narrow-degrade row must drop the left cluster beneath the anchor:\n%q", footerVisible(row))
	}
	if got := lipgloss.Width(row); got != w {
		t.Errorf("narrow-degrade row width = %d, want exactly %d", got, w)
	}
}

func TestAssembleRightAnchoredRow_BelowAnchorRendersEmpty(t *testing.T) {
	th := testDarkTheme(t)
	left := renderFooterDetail("left", th, false)
	leftWidth := lipgloss.Width(left)
	rightSeg := renderFooterDetail("? help", th, false)
	rightWidth := lipgloss.Width(rightSeg)

	w := rightWidth - 1
	row := assembleRightAnchoredRow(left, leftWidth, rightSeg, rightWidth, w, th, false)

	if got := strings.TrimSpace(footerVisible(row)); got != "" {
		t.Errorf("below the anchor's width the row must be empty, got %q", got)
	}
	if got := lipgloss.Width(row); got != w {
		t.Errorf("empty row width = %d, want exactly %d", got, w)
	}
	if want := headerPadRight("", 0, w, th, false); row != want {
		t.Errorf("empty row != headerPadRight(\"\", …):\n got=%q\nwant=%q", row, want)
	}
}

func TestAssembleRightAnchoredRow_NoRightAnchorPadsLeft(t *testing.T) {
	const w = rightAnchoredCanvasWidth
	left := renderFooterDetail("left", testDarkTheme(t), false)
	leftWidth := lipgloss.Width(left)

	row := assembleRightAnchoredRow(left, leftWidth, "", 0, w, testDarkTheme(t), false)

	want := headerPadRight(left, leftWidth, w, testDarkTheme(t), false)
	if row != want {
		t.Errorf("no-anchor row != headerPadRight(left, …):\n got=%q\nwant=%q", row, want)
	}
}

func TestFooters_RouteThroughSharedAssembler_NarrowDegradeIdentical(t *testing.T) {
	th := testDarkTheme(t)

	core, helpEntry := splitFooterEntries(sessionsKeymap())
	if helpEntry == nil {
		t.Fatal("sessionsKeymap must carry the right-aligned ? help anchor")
	}
	rightWidth := lipgloss.Width(renderFooterEntry(*helpEntry, th.AccentPrimary, th, false))

	rightSeg := renderFooterEntry(*helpEntry, th.AccentPrimary, th, false)
	clusters := map[string]string{
		"standard":  renderFooterCluster(core, th, false),
		"filtering": renderFilterCluster(filteringFooterEntries(th), th, false),
		"applied":   renderFilterCluster(filterAppliedFooterEntries(th), th, false),
	}
	for name, left := range clusters {
		leftWidth := lipgloss.Width(left)
		w := leftWidth + rightWidth
		if leftWidth+1+rightWidth <= w {
			t.Fatalf("[%s] setup width %d is not at/below the degrade boundary", name, w)
		}

		got := assembleRightAnchoredRow(left, leftWidth, rightSeg, rightWidth, w, th, false)
		want := headerPadLeft(rightSeg, rightWidth, w, th, false)
		if got != want {
			t.Errorf("[%s w=%d] assembler degrade != headerPadLeft(rightSeg, …):\n got=%q\nwant=%q", name, w, got, want)
		}
		if !strings.Contains(footerVisible(got), "? help") {
			t.Errorf("[%s w=%d] assembler degrade must KEEP the ? help anchor (§14.4):\n%q", name, w, footerVisible(got))
		}
	}

	tinyWidth := rightWidth
	renders := map[string]string{
		"standard":  lastLine(renderSessionsFooter(sessionsKeymap(), tinyWidth, th, false)),
		"filtering": lastLine(renderFilteringFooter(tinyWidth, th, false)),
		"applied":   lastLine(renderFilterAppliedFooter(tinyWidth, th, false)),
	}
	for name, row := range renders {
		if got := strings.TrimSpace(footerVisible(row)); got != "? help" {
			t.Errorf("[%s w=%d] the anchor-width render = %q, want the bare %q", name, tinyWidth, got, "? help")
		}
	}
}

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}
