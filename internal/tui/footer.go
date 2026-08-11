package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

const (
	// Upper one-eighth block: the rule lands at the top edge of its cell,
	// opening room below it before the key row (the header rule mirrors it low).
	footerRuleGlyph      = "▔"
	footerKeyLabelGap    = " "
	footerEntrySeparator = " · "
)

const footerEllipsis = "…"

// Entries are a parameter, not a sessionsKeymap() call: the footer must be
// filtered in lockstep with `?` help through the one call-site filter, which
// drops a blocked `t` or `m`.
func renderSessionsFooter(entries []keymapEntry, width int, th theme.Theme, colourless bool) string {
	return renderCondensedFooter(entries, width, th, colourless)
}

func renderProjectsFooter(entries []keymapEntry, width int, th theme.Theme, colourless bool) string {
	return renderCondensedFooter(entries, width, th, colourless)
}

func renderCommandPendingFooter(width int, th theme.Theme, colourless bool) string {
	return renderFilterFooter(commandPendingFooterEntries(th), width, th, colourless)
}

func commandPendingFooterEntries(th theme.Theme) []filterFooterEntry {
	descriptor := commandPendingKeymap()
	entries := make([]filterFooterEntry, 0, len(descriptor))
	for _, e := range descriptor {
		entries = append(entries, filterFooterEntry{
			Key:   []keyGlyph{{helpKeyGlyph(e), th.AccentKey}},
			Label: e.Action,
		})
	}
	return entries
}

const (
	multiSelectNavGlyph     = "↑↓"
	multiSelectNavLabel     = "navigate"
	multiSelectToggleGlyph  = "m"
	multiSelectToggleLabel  = "toggle"
	multiSelectPreviewGlyph = "␣" // U+2423
	multiSelectPreviewLabel = "preview"
	multiSelectOpenGlyph    = "⏎" // U+23CE
	multiSelectOpenLabel    = "open"
	multiSelectCancelGlyph  = "esc"
	multiSelectCancelLabel  = "cancel"
)

const multiSelectFooterText = multiSelectNavGlyph + footerKeyLabelGap + multiSelectNavLabel +
	footerEntrySeparator + multiSelectToggleGlyph + footerKeyLabelGap + multiSelectToggleLabel +
	footerEntrySeparator + multiSelectPreviewGlyph + footerKeyLabelGap + multiSelectPreviewLabel +
	footerEntrySeparator + multiSelectOpenGlyph + footerKeyLabelGap + multiSelectOpenLabel +
	footerEntrySeparator + multiSelectCancelGlyph + footerKeyLabelGap + multiSelectCancelLabel

func multiSelectFooterEntries(th theme.Theme) []filterFooterEntry {
	return []filterFooterEntry{
		{Key: []keyGlyph{{multiSelectNavGlyph, th.AccentKey}}, Label: multiSelectNavLabel},
		{Key: []keyGlyph{{multiSelectToggleGlyph, th.AccentKey}}, Label: multiSelectToggleLabel},
		{Key: []keyGlyph{{multiSelectPreviewGlyph, th.AccentKey}}, Label: multiSelectPreviewLabel},
		{Key: []keyGlyph{{multiSelectOpenGlyph, th.AccentKey}}, Label: multiSelectOpenLabel},
		{Key: []keyGlyph{{multiSelectCancelGlyph, th.AccentKey}}, Label: multiSelectCancelLabel},
	}
}

// The multi-select footer carries no `? help` anchor, so it cannot route
// through renderFilterFooter/filterFooterRow (those hardcode one).
func renderMultiSelectFooter(width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	rule := footerTopRule(w, th, colourless)
	left, leftWidth := fitFilterCluster(multiSelectFooterEntries(th), w, th, colourless)
	row := assembleRightAnchoredRow(left, leftWidth, "", 0, w, th, colourless)
	return lipgloss.JoinVertical(lipgloss.Left, rule, row)
}

// With no right anchor the full width is the budget.
func fitFilterCluster(entries []filterFooterEntry, w int, th theme.Theme, colourless bool) (string, int) {
	sep := renderFooterDetail(footerEntrySeparator, th, colourless)
	ellipsis := renderFooterDetail(footerEllipsis, th, colourless)
	renderCluster := func(n int) (string, int) {
		cluster := renderFilterCluster(entries[:n], th, colourless)
		return cluster, lipgloss.Width(cluster)
	}
	return fitClusterToWidth(len(entries), w, renderCluster, sep, ellipsis)
}

// Narrow degrade: try the full cluster, else the widest leading prefix that
// fits with a trailing ` · …`, else the bare ellipsis, else empty. renderCluster
// renders the first n entries and returns the cluster with its rendered width;
// the result is always ≤ budget so the row never wraps to a second line.
func fitClusterToWidth(count, budget int, renderCluster func(n int) (string, int), sep, ellipsis string) (string, int) {
	if full, fullWidth := renderCluster(count); fullWidth <= budget {
		return full, fullWidth
	}

	ellipsisWidth := lipgloss.Width(ellipsis)
	sepWidth := lipgloss.Width(sep)

	best := ""
	bestWidth := 0
	for n := 1; n <= count; n++ {
		cluster, clusterWidth := renderCluster(n)
		candidateWidth := clusterWidth + sepWidth + ellipsisWidth
		if candidateWidth > budget {
			break
		}
		best = lipgloss.JoinHorizontal(lipgloss.Top, cluster, sep, ellipsis)
		bestWidth = candidateWidth
	}
	if best != "" {
		return best, bestWidth
	}

	if ellipsisWidth <= budget {
		return ellipsis, ellipsisWidth
	}
	return "", 0
}

// Single render entry point so a page's render and its height-budget
// computation resolve the footer identically and agree on its height.
func renderCondensedFooter(entries []keymapEntry, width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	rule := footerTopRule(w, th, colourless)
	row := footerKeyRow(entries, w, th, colourless)
	return lipgloss.JoinVertical(lipgloss.Left, rule, row)
}

func footerTopRule(w int, th theme.Theme, colourless bool) string {
	rule := strings.Repeat(footerRuleGlyph, w)
	return headerStyle(th.Border, th, colourless).Render(rule)
}

func footerKeyRow(entries []keymapEntry, w int, th theme.Theme, colourless bool) string {
	core, right := splitFooterEntries(entries)

	// The anchor is rendered first so the left cluster fits around the space it
	// reserves.
	rightSeg := ""
	rightWidth := 0
	if right != nil {
		rightSeg = renderFooterEntry(*right, th.AccentPrimary, th, colourless)
		rightWidth = lipgloss.Width(rightSeg)
	}

	left, leftWidth := fitLeftCluster(core, w, rightWidth, th, colourless)

	return assembleRightAnchoredRow(left, leftWidth, rightSeg, rightWidth, w, th, colourless)
}

// The anchor survives; the left cluster gives way beneath it. `? help` is the
// escape hatch that makes every dropped entry recoverable, so it is never
// dropped while it fits. Rungs, widest first: no anchor → pad the cluster;
// both fit → cluster, spacer, anchor; anchor only → right-aligned alone;
// neither → an empty canvas row.
func assembleRightAnchoredRow(left string, leftWidth int, rightSeg string, rightWidth, w int, th theme.Theme, colourless bool) string {
	if rightSeg == "" {
		return headerPadRight(left, leftWidth, w, th, colourless)
	}
	if leftWidth+1+rightWidth <= w {
		spacerWidth := w - leftWidth - rightWidth
		spacer := headerCanvasBg(th, colourless).Render(strings.Repeat(" ", spacerWidth))
		return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, rightSeg)
	}
	if rightWidth <= w {
		return headerPadLeft(rightSeg, rightWidth, w, th, colourless)
	}
	return headerPadRight("", 0, w, th, colourless)
}

// Non-Core entries are dropped — they live in the `?` help modal.
func splitFooterEntries(entries []keymapEntry) (core []keymapEntry, right *keymapEntry) {
	for i := range entries {
		e := entries[i]
		if !e.Core {
			continue
		}
		if e.RightAligned {
			r := e
			right = &r
			continue
		}
		core = append(core, e)
	}
	return core, right
}

// The budget reserves the right anchor plus one spacer cell.
func fitLeftCluster(core []keymapEntry, w, rightWidth int, th theme.Theme, colourless bool) (string, int) {
	budget := w
	if rightWidth > 0 {
		budget = w - rightWidth - 1
	}
	if budget < 0 {
		budget = 0
	}

	sep := renderFooterDetail(footerEntrySeparator, th, colourless)
	ellipsis := renderFooterDetail(footerEllipsis, th, colourless)
	renderCluster := func(n int) (string, int) {
		cluster := renderFooterCluster(core[:n], th, colourless)
		return cluster, lipgloss.Width(cluster)
	}
	return fitClusterToWidth(len(core), budget, renderCluster, sep, ellipsis)
}

func renderFooterCluster(entries []keymapEntry, th theme.Theme, colourless bool) string {
	if len(entries) == 0 {
		return ""
	}
	segs := make([]string, 0, len(entries)*2-1)
	for i, e := range entries {
		if i > 0 {
			segs = append(segs, renderFooterDetail(footerEntrySeparator, th, colourless))
		}
		segs = append(segs, renderFooterEntry(e, th.AccentKey, th, colourless))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, segs...)
}

func renderFooterEntry(e keymapEntry, keyTok theme.Token, th theme.Theme, colourless bool) string {
	return renderKeyHint(e.Key, e.Action, keyTok, th, colourless)
}

func renderFooterDetail(s string, th theme.Theme, colourless bool) string {
	return headerStyle(th.TextMuted, th, colourless).Render(s)
}
