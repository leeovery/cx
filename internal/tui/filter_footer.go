package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

type keyGlyph struct {
	Text string
	Tok  theme.Token
}

// BrowseResults tags the entry structurally so the no-matches footer drops it
// by flag, not by display copy — rewording the label cannot re-admit it.
type filterFooterEntry struct {
	Key           []keyGlyph
	Label         string
	BrowseResults bool
}

func filteringFooterEntries(th theme.Theme) []filterFooterEntry {
	return []filterFooterEntry{
		{Key: []keyGlyph{{"type", th.AccentAttention}}, Label: "to filter"},
		{Key: []keyGlyph{
			{"↵", th.AccentKey},
			{" / ", th.TextMuted},
			{"↓", th.AccentKey},
		}, Label: "browse results", BrowseResults: true},
		{Key: []keyGlyph{{"esc", th.TextMuted}}, Label: "clear"},
	}
}

func dropBrowseResults(src []filterFooterEntry) []filterFooterEntry {
	entries := make([]filterFooterEntry, 0, len(src))
	for _, e := range src {
		if e.BrowseResults {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

func filterAppliedFooterEntries(th theme.Theme) []filterFooterEntry {
	return []filterFooterEntry{
		{Key: []keyGlyph{{"↵", th.AccentKey}}, Label: "attach"},
		{Key: []keyGlyph{{"↑↓", th.AccentKey}}, Label: "navigate"},
		{Key: []keyGlyph{{"esc", th.AccentAttention}}, Label: "clear filter"},
	}
}

// Enter on Projects creates a session rather than attaching, so the Sessions
// "attach" copy must not leak here.
func projectsFilterAppliedFooterEntries(th theme.Theme) []filterFooterEntry {
	return []filterFooterEntry{
		{Key: []keyGlyph{{"↵", th.AccentKey}}, Label: "new session"},
		{Key: []keyGlyph{{"↑↓", th.AccentKey}}, Label: "navigate"},
		{Key: []keyGlyph{{"esc", th.AccentAttention}}, Label: "clear filter"},
	}
}

func renderFilteringFooter(width int, th theme.Theme, colourless bool) string {
	return renderFilterFooter(filteringFooterEntries(th), width, th, colourless)
}

func renderFilterAppliedFooter(width int, th theme.Theme, colourless bool) string {
	return renderFilterFooter(filterAppliedFooterEntries(th), width, th, colourless)
}

func renderProjectsFilterAppliedFooter(width int, th theme.Theme, colourless bool) string {
	return renderFilterFooter(projectsFilterAppliedFooterEntries(th), width, th, colourless)
}

func renderFilterFooter(entries []filterFooterEntry, width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	rule := footerTopRule(w, th, colourless)
	row := filterFooterRow(entries, w, th, colourless)
	return lipgloss.JoinVertical(lipgloss.Left, rule, row)
}

// The `? help` anchor is sourced from the sessionsKeymap descriptor so its
// glyph/label/colour cannot drift from the standard footer's.
func filterFooterRow(entries []filterFooterEntry, w int, th theme.Theme, colourless bool) string {
	// The anchor renders first so the left cluster fits around the space it reserves.
	_, helpEntry := splitFooterEntries(sessionsKeymap())
	rightSeg := ""
	rightWidth := 0
	if helpEntry != nil {
		rightSeg = renderFooterEntry(*helpEntry, th.AccentPrimary, th, colourless)
		rightWidth = lipgloss.Width(rightSeg)
	}

	// The budget reserves the anchor plus one spacer cell, as fitLeftCluster does:
	// unfitted, the whole cluster vanishes at once for every width between its own
	// and cluster+1+anchor, leaving a filtering user no keymap at all.
	budget := w
	if rightWidth > 0 {
		budget = max(w-rightWidth-1, 0)
	}
	left, leftWidth := fitFilterCluster(entries, budget, th, colourless)

	return assembleRightAnchoredRow(left, leftWidth, rightSeg, rightWidth, w, th, colourless)
}

func renderFilterCluster(entries []filterFooterEntry, th theme.Theme, colourless bool) string {
	if len(entries) == 0 {
		return ""
	}
	segs := make([]string, 0, len(entries)*2-1)
	for i, e := range entries {
		if i > 0 {
			segs = append(segs, renderFooterDetail(footerEntrySeparator, th, colourless))
		}
		key := renderKeyGlyphs(e.Key, th, colourless)
		gap := headerCanvasBg(th, colourless).Render(footerKeyLabelGap)
		label := headerStyle(th.TextMuted, th, colourless).Render(e.Label)
		segs = append(segs, lipgloss.JoinHorizontal(lipgloss.Top, key, gap, label))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, segs...)
}

func renderKeyGlyphs(glyphs []keyGlyph, th theme.Theme, colourless bool) string {
	runs := make([]string, 0, len(glyphs))
	for _, g := range glyphs {
		runs = append(runs, headerStyle(g.Tok, th, colourless).Render(g.Text))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, runs...)
}
