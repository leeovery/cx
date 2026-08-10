package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// This file pins the shared right-anchored footer row assembler
// (assembleRightAnchoredRow) extracted from footerKeyRow and filterFooterRow
// (task 8-7). Both the standard condensed footer and the contextual filter
// footers route their final right-anchor layout (the fit test, the narrow degrade,
// and the canvas flex-spacer join) through this single assembler, so a change to
// the right-anchor degrade rule is made once.
//
// §14.4 INVERTED THE DEGRADE, and these assertions moved with it (§15.1 names §14
// as the amendment). The assembler used to drop the RIGHT ANCHOR first and pad the
// left cluster — so exactly the width at which the footer stopped advertising
// anything was the width at which it also took away `? help`, the escape hatch that
// makes every dropped entry recoverable. It now keeps the anchor and lets the left
// cluster give way beneath it, rendering the anchor alone at its own width and an
// empty row only below that.
//
// The shared-owner guarantee is the load-bearing assertion: both footers must reach
// the SAME rung of that ladder at the same boundary.

// rightAnchoredCanvasWidth is a content width wide enough that both footers
// render their full left cluster + flex spacer + the ? help right anchor with no
// §2.7 truncation.
const rightAnchoredCanvasWidth = 120

// TestAssembleRightAnchoredRow_WideEmitsClusterSpacerAnchor asserts the WIDE
// path: when the right anchor fits beside the left cluster, the assembler emits
// the cluster, a canvas-painted flex spacer, then the right anchor — the row is
// exactly w cells wide and the anchor ends flush at the right edge.
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
	// The gap between cluster and anchor is a flex spacer wider than one cell.
	gap := strings.TrimSuffix(strings.TrimPrefix(vis, "left"), "? help")
	if len(gap) <= 1 {
		t.Errorf("wide row flex spacer too narrow (%d cells): %q", len(gap), vis)
	}
}

// TestAssembleRightAnchoredRow_NarrowDegradeKeepsAnchorDropsCluster asserts the
// §14.4 degrade boundary (leftWidth+1+rightWidth > w): the assembler keeps the
// right anchor and the LEFT CLUSTER gives way beneath it, the anchor rendering
// alone right-aligned via headerPadLeft. This is the inversion — the anchor is
// never dropped while it fits, because it is the escape hatch that makes every
// dropped entry recoverable.
func TestAssembleRightAnchoredRow_NarrowDegradeKeepsAnchorDropsCluster(t *testing.T) {
	left := renderFooterDetail("left", testDarkTheme(t), false)
	leftWidth := lipgloss.Width(left)
	rightSeg := renderFooterDetail("? help", testDarkTheme(t), false)
	rightWidth := lipgloss.Width(rightSeg)

	// A width at/below the degrade boundary: leftWidth+1+rightWidth > w, but wide
	// enough that the anchor alone still fits.
	w := leftWidth + rightWidth // strictly less than leftWidth+1+rightWidth
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

// TestAssembleRightAnchoredRow_BelowAnchorRendersEmpty asserts §14.4's bottom rung:
// below the width at which `? help` alone fits, the row renders EMPTY — exactly w
// canvas cells, no partial anchor. Consistent with §2.7's degrade-never-break, and
// Portal's documented 40-column minimum sits well above it.
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

// TestAssembleRightAnchoredRow_NoRightAnchorPadsLeft asserts the rightSeg=="" arm:
// with no right anchor the assembler pads the left cluster to width regardless of
// the fit test (mirrors the original "no right entry" branch).
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

// TestFooters_RouteThroughSharedAssembler_NarrowDegradeIdentical asserts the
// load-bearing shared-degrade guarantee at the §14.4 boundary: both the standard
// condensed footer AND the contextual filter footers route their final
// right-anchor layout through assembleRightAnchoredRow, so at a width that forces
// the degrade (leftWidth+1+rightWidth > w) BOTH keep the ? help anchor and give up
// the left cluster beneath it, byte-identically, through the SHARED assembler.
//
// NOTE on scope: the filter footers have NO left-cluster fitting (fitLeftCluster
// is footer.go-specific and stays so per the task scope guard), so their left
// cluster can itself exceed w at very narrow widths. Under the pre-§14.4 rule that
// left them overflowing the row; under the inverted rule the cluster is what gives
// way, so the surviving anchor is what both footers agree on.
func TestFooters_RouteThroughSharedAssembler_NarrowDegradeIdentical(t *testing.T) {
	th := testDarkTheme(t)

	// The right anchor is the shared sessionsKeymap ? help, sourced identically by
	// both row functions; its rendered width derives the degrade boundary.
	core, helpEntry := splitFooterEntries(sessionsKeymap())
	if helpEntry == nil {
		t.Fatal("sessionsKeymap must carry the right-aligned ? help anchor")
	}
	rightWidth := lipgloss.Width(renderFooterEntry(*helpEntry, th.AccentPrimary, th, false))

	// Each footer is driven through assembleRightAnchoredRow with its OWN rendered
	// left cluster. To prove the SHARED degrade, feed every footer's left cluster
	// through the assembler at a width strictly below the degrade boundary
	// (leftWidth+1+rightWidth > w) and assert the assembler keeps the anchor and
	// returns exactly headerPadLeft(rightSeg, rightWidth, w, …) — byte-identical
	// degrade regardless of which footer's cluster it is.
	rightSeg := renderFooterEntry(*helpEntry, th.AccentPrimary, th, false)
	clusters := map[string]string{
		"standard":  renderFooterCluster(core, th, false),
		"filtering": renderFilterCluster(filteringFooterEntries(th), th, false),
		"applied":   renderFilterCluster(filterAppliedFooterEntries(th), th, false),
	}
	for name, left := range clusters {
		leftWidth := lipgloss.Width(left)
		w := leftWidth + rightWidth // leftWidth+1+rightWidth > w (no spacer cell)
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

	// And the end-to-end render: at exactly the anchor's width every footer renders
	// the ? help anchor ALONE on its single key row (the degrade is reached through
	// the render path, not just the bare assembler) — the escape hatch survives on
	// all three.
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

// lastLine returns the final \n-separated line of a rendered footer (the key row;
// line 0 is the border top rule).
func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}
