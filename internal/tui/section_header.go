package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// The section header replaces the rendered title line; it does not rewrite the
// list's title value. Its suffix text and inside-tmux decoration are sourced
// from sessionListTitleForMode so the rendered strings stay byte-identical to
// that single title producer.

const (
	// The fixed prefix of every sessionListTitleForMode output — the suffix is
	// the remainder after it, so the two split with no duplicated wording.
	sectionLabel         = "Sessions"
	projectsSectionLabel = "Projects"
	// The `s switch view` hint is the footer's responsibility and deliberately
	// not duplicated here.
	sectionFilterHint     = "/ to filter"
	multiSelectCancelHint = "esc cancel"

	unsupportedLabel    = "unsupported terminal"
	unsupportedDocsHint = "see docs"
	// The OSC 8 link target. The banner never prints this as visible text —
	// link-or-bare-word only.
	unsupportedDocsURL = "https://github.com/leeovery/portal/blob/main/docs/custom-terminals.md"
	// A spaced em-dash before the friendly name, a spaced middot before the
	// bundle id.
	unsupportedIdentityDash   = " — "
	unsupportedIdentityMiddot = " · "

	// Esc clears the abort banner while staying in multi-select mode.
	preflightAbortDismissHint = "esc dismiss"
)

func renderSectionHeader(mode prefs.SessionListMode, insideTmux bool, currentSession string, count, width int, th theme.Theme, colourless bool) string {
	left := sectionLeftCluster(mode, insideTmux, currentSession, count, th, colourless)
	return renderSectionHeaderRow(left, width, th, colourless)
}

func renderProjectsSectionHeader(count, width int, th theme.Theme, colourless bool) string {
	left := projectsLeftCluster(count, th, colourless)
	return renderSectionHeaderRow(left, width, th, colourless)
}

// The multi-select banner replaces the standard section header while the mode
// is active. It carries no `▌` left-bar — it is a section-header variant, not
// a notice band.
func renderMultiSelectHeader(count, width int, th theme.Theme, colourless bool) string {
	left := headerStyle(th.AccentPrimary, th, colourless).Render(strconv.Itoa(count) + " selected")
	hint := headerStyle(th.TextMuted, th, colourless).Render(multiSelectCancelHint)
	return renderRightAnchoredSectionRow(left, hint, width, th, colourless)
}

// The in-burst pending affordance, with no right hint so the empty hint pads
// the right side with canvas. No `▌` left-bar.
func renderOpeningBand(done, total, width int, th theme.Theme, colourless bool) string {
	left := headerStyle(th.AccentPrimary, th, colourless).
		Render(fmt.Sprintf("Opening %d/%d…", done, total))
	return renderRightAnchoredSectionRow(left, "", width, th, colourless)
}

// Named-only: bundleID != "" always holds here, because the gate
// (unsupportedBannerActive) carries the !IsNull() discriminator, so a
// NULL/remote identity renders the standard header instead. No `▌` left-bar.
func renderUnsupportedHeader(name, bundleID string, width int, th theme.Theme, colourless bool) string {
	left := unsupportedLeftCluster(name, bundleID, th, colourless)
	// OSC 8 is orthogonal to colour, so the link is emitted unconditionally and
	// degrades to the bare word where the terminal does not render it.
	hint := headerStyle(th.AccentKey, th, colourless).
		Hyperlink(unsupportedDocsURL).
		Render(unsupportedDocsHint)
	return renderRightAnchoredSectionRow(left, hint, width, th, colourless)
}

// The message is pre-composed by the caller. No `▌` left-bar.
func renderPreflightAbortHeader(message string, width int, th theme.Theme, colourless bool) string {
	left := headerStyle(th.StateDestructive, th, colourless).
		Render(flashWarningGlyph + " " + message)
	hint := headerStyle(th.TextMuted, th, colourless).Render(preflightAbortDismissHint)
	return renderRightAnchoredSectionRow(left, hint, width, th, colourless)
}

// The ⚠ glyph is shared with the warning flash so the two warning surfaces
// stay glyph-consistent.
func unsupportedLeftCluster(name, bundleID string, th theme.Theme, colourless bool) string {
	label := headerStyle(th.AccentAttention, th, colourless).
		Render(flashWarningGlyph + " " + unsupportedLabel)
	identity := headerStyle(th.TextMuted, th, colourless).
		Render(unsupportedIdentityDash + name + unsupportedIdentityMiddot + bundleID)
	return lipgloss.JoinHorizontal(lipgloss.Top, label, identity)
}

func renderSectionHeaderRow(left string, width int, th theme.Theme, colourless bool) string {
	hint := headerStyle(th.TextMuted, th, colourless).Render(sectionFilterHint)
	return renderRightAnchoredSectionRow(left, hint, width, th, colourless)
}

// The shared right-anchor core every section-header variant routes through, so
// their alignment and narrow degrade cannot drift. Always exactly width cells.
func renderRightAnchoredSectionRow(left, hint string, width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	leftWidth := lipgloss.Width(left)
	hintWidth := lipgloss.Width(hint)

	// Drop the hint rather than overflow.
	if leftWidth >= w || leftWidth+1+hintWidth > w {
		return headerPadRight(left, leftWidth, w, th, colourless)
	}

	spacerWidth := w - leftWidth - hintWidth
	spacer := headerCanvasBg(th, colourless).Render(strings.Repeat(" ", spacerWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, hint)
}

func projectsLeftCluster(count int, th theme.Theme, colourless bool) string {
	label := headerStyle(th.StatePositive, th, colourless).Render(projectsSectionLabel)
	gap := headerCanvasBg(th, colourless).Render(" ")
	countRun := headerStyle(th.TextMuted, th, colourless).Render(strconv.Itoa(count))
	return lipgloss.JoinHorizontal(lipgloss.Top, label, gap, countRun)
}

// Flush at the content's left edge — the same column as the header wordmark
// and the row cursor — with no leading indent. The count is a plain run at the
// label's cap-height, distinguished only by colour.
func sectionLeftCluster(mode prefs.SessionListMode, insideTmux bool, currentSession string, count int, th theme.Theme, colourless bool) string {
	label := headerStyle(th.AccentMode, th, colourless).Render(sectionLabel)

	gap := headerCanvasBg(th, colourless).Render(" ")
	countRun := headerStyle(th.StatePositive, th, colourless).Render(strconv.Itoa(count))

	cluster := lipgloss.JoinHorizontal(lipgloss.Top, label, gap, countRun)

	if suffix := sectionModeSuffix(mode, insideTmux, currentSession); suffix != "" {
		suffixRun := headerStyle(th.TextMuted, th, colourless).Render(suffix)
		cluster = lipgloss.JoinHorizontal(lipgloss.Top, cluster, suffixRun)
	}
	return cluster
}

// Rendered both in the live input and in the locked-query header, so the two
// modes read identically aside from the cursor.
const filterPromptPrefix = "/ "

// The list-active locked query, with no cursor (signalling the list is
// filtered) and no tint. It replaces the section header for the FilterApplied
// frame; the input-active frame belongs to the live bubbles/list FilterInput.
func renderFilterQueryHeader(query string, width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	run := headerStyle(th.AccentAttention, th, colourless).Render(filterPromptPrefix + query)
	runWidth := lipgloss.Width(run)
	return headerPadRight(run, runWidth, w, th, colourless)
}

// Derived by stripping the fixed prefix from the title producer rather than
// re-deriving, so the wording stays byte-identical. It carries a single
// leading space so it renders after the count.
func sectionModeSuffix(mode prefs.SessionListMode, insideTmux bool, currentSession string) string {
	title := sessionListTitleForMode(mode, insideTmux, currentSession)
	return strings.TrimPrefix(title, sectionLabel)
}
