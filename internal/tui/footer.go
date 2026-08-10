package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// The condensed Sessions footer: a SINGLE row of the Core keymap keys on
// the left (↑↓ navigate · ⏎ attach · / filter · ␣ preview · s switch
// view · x projects) with a right-aligned `? help` hint pinned to the content
// width, over a 1px footer top rule. It replaces the former manual
// three-column keymap footer for Sessions; the help-only keys
// (n/r/k/q/Ctrl+↑/↓) live in the ? help modal, not the footer.
//
// The footer renders FROM the keymap descriptor (sessionsKeymap), with the
// entries filtered to Core, so the footer and the ? help modal never drift
// from a second hand-authored binding list.
//
// Every cell carries the owned canvas background (leaf .Background(canvas)),
// mirroring header.go / section_header.go, so the right-aligned spacer gap is
// canvas-painted with no terminal-bg island. Under the NO_COLOR carve-out
// every hue and the canvas drop; the glyphs stay structurally distinct on the
// terminal's native fg/bg.

const (
	// footerRuleGlyph is the 1px footer top-rule glyph, drawn in the `border` token — the SAME token as the title rule (the token layer
	// consolidated the two border roles into one).
	// It uses the UPPER one-eighth block so the rule lands at the TOP edge of its
	// cell, opening breathing room BELOW it before the key row (the header rule uses
	// the LOWER block ▁ to sit low; the footer rule mirrors it high so the border is
	// not crowded against the keybindings). The footer rule and the header rule are
	// the SAME token and differ only in glyph weight — there is no second
	// border role, so there is no distinct footer hue left to conflate.
	footerRuleGlyph = "▔"
	// footerKeyLabelGap is the single space between a key glyph and its label
	// (e.g. "↑↓ navigate"). Canvas-painted so the gap is not a terminal-bg island.
	// Used by renderFilterCluster (whose multi-glyph key cluster is a distinct shape
	// from the single-key renderKeyHint helper). renderKeyHint paints the same single
	// canvas space inline.
	footerKeyLabelGap = " "
	// footerEntrySeparator is the " · " dot separator between footer entries in the
	// left cluster (the dot-separated condensed row). Rendered in text.muted
	// so it reads as quiet chrome between the brighter key glyphs.
	footerEntrySeparator = " · "
)

// footerEllipsis is the overflow marker the narrow-degrade path appends to the
// left cluster when one or more lower-priority entries are dropped.
const footerEllipsis = "…"

// renderSessionsFooter renders the condensed Sessions footer
// (`⏎ attach · / filter · ␣ preview · s switch view · x projects · t theme ·
// m multi`, right-aligned `? help`) for the given descriptor entries, content
// width and resolved canvas mode (and the NO_COLOR carve-out). It is the single
// render entry point so the composed-view render (viewSessionList) and the
// height-budget computation (applySessionListSize) resolve the footer against the
// SAME entries/width/mode and agree on its height exactly.
//
// IT TAKES ITS ENTRIES AS A PARAMETER rather than calling sessionsKeymap itself
// The footer is filtered in lockstep with `?` help through the ONE
// call-site filter (Model.sessionsHelpKeymap), which drops a blocked `t`
// or a blocked `m` (unsupported terminal) — so a footer sourcing the static
// descriptor for itself is exactly how the two surfaces come to disagree about one
// key. Both the render and the budget pass the same filtered slice.
//
// The footer is two rows: the 1px footer top rule, then the condensed key
// row (Core keys left, right-aligned ? help). Below the width at which the full
// left cluster + a spacer + the ? help no longer fit, lower-priority Core entries
// drop from the RIGHT (with an ellipsis marker) so the row degrades gracefully on
// ONE line — it never wraps to a second line (which would steal a list row) and
// the ? help right anchor is never dropped while it fits.
func renderSessionsFooter(entries []keymapEntry, width int, th theme.Theme, colourless bool) string {
	return renderCondensedFooter(entries, width, th, colourless)
}

// renderProjectsFooter renders the condensed Projects footer
// (`⏎ new session · x sessions · e edit · / filter · t theme`, right-aligned
// `? help`) through the SAME condensed-footer machinery as the Sessions footer —
// replacing the former three-column keymap footer for the Projects page. Same
// two-row shape (the shared 1px footer top rule + one key row), so it is
// height-neutral against the Sessions footer's reserved budget.
//
// Like its Sessions sibling it takes its entries as a parameter: `t theme` is in
// this footer too, so Projects needs the matching call-site filter
// (Model.projectsHelpKeymap) — the block is stated for the Sessions site, but the
// mechanism is the same and the second call site is required.
func renderProjectsFooter(entries []keymapEntry, width int, th theme.Theme, colourless bool) string {
	return renderCondensedFooter(entries, width, th, colourless)
}

// renderCommandPendingFooter renders the command-pending Projects footer:
// `⏎ run here · n run in cwd · esc cancel` (left cluster) + the right-aligned
// `? help` anchor, over the shared 1px footer top rule. The left cluster
// entries are derived from the commandPendingKeymap() descriptor — the single binding
// source — mapped to MV chrome (key glyphs accent.key, labels text.muted,
// the `enter` binding shown as its declarative HelpKey `⏎` glyph). It routes through
// the shared renderFilterFooter machinery so the `? help` anchor + the two-row
// structure stay byte-consistent with the standard / filter footers; only the entries
// differ.
func renderCommandPendingFooter(width int, th theme.Theme, colourless bool) string {
	return renderFilterFooter(commandPendingFooterEntries(th), width, th, colourless)
}

// commandPendingFooterEntries maps the descriptor (commandPendingKeymap) to the
// filter-footer entry shape: each entry's Action becomes the label and its glyph comes
// from helpKeyGlyph (the declarative HelpKey when set — `enter`'s `⏎` — else the terse
// Key), the SAME glyph resolution the descriptor-driven help path uses. This retires
// the former inline `enter→⏎` rewrite, folding the command-pending footer into the
// shared descriptor/entry vocabulary. Every key glyph is accent.key per the MV footer
// convention; labels render in text.muted.
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

// The multi-select mode footer copy — the pinned entry glyphs + labels, fixed by
// the delivered design frame:
//
//	↑↓ navigate · m toggle · ␣ preview · ⏎ open · esc cancel
//
// Sourced once here as named constants (mirroring commandBandText) so the wording
// can't drift from a paraphrase. The glyphs are the codebase canon: nav ↑↓, preview ␣
// (U+2423) and open ⏎ (U+23CE) match the sessionsKeymap Key forms. Unlike the
// standard/filter footers this footer carries NO right-aligned `? help` anchor — the
// delivered frame has none.
const (
	multiSelectNavGlyph     = "↑↓"
	multiSelectNavLabel     = "navigate"
	multiSelectToggleGlyph  = "m"
	multiSelectToggleLabel  = "toggle"
	multiSelectPreviewGlyph = "␣" // U+2423, the sessionsKeymap preview glyph
	multiSelectPreviewLabel = "preview"
	multiSelectOpenGlyph    = "⏎" // U+23CE, the sessionsKeymap enter/attach glyph
	multiSelectOpenLabel    = "open"
	multiSelectCancelGlyph  = "esc"
	multiSelectCancelLabel  = "cancel"
)

// multiSelectFooterText is the pinned mode-footer copy assembled from the
// per-entry constants above (separators the shared footerEntrySeparator ` · `). It is
// the single source of truth the copy-pin test asserts the render against.
const multiSelectFooterText = multiSelectNavGlyph + footerKeyLabelGap + multiSelectNavLabel +
	footerEntrySeparator + multiSelectToggleGlyph + footerKeyLabelGap + multiSelectToggleLabel +
	footerEntrySeparator + multiSelectPreviewGlyph + footerKeyLabelGap + multiSelectPreviewLabel +
	footerEntrySeparator + multiSelectOpenGlyph + footerKeyLabelGap + multiSelectOpenLabel +
	footerEntrySeparator + multiSelectCancelGlyph + footerKeyLabelGap + multiSelectCancelLabel

// multiSelectFooterEntries returns the multi-select mode footer entries in frame
// order. Every key glyph is accent.key and every label text.muted — the standard MV
// footer colour convention. It mirrors filteringFooterEntries' entry-list shape
// so the cluster renders through the shared renderFilterCluster machinery.
func multiSelectFooterEntries(th theme.Theme) []filterFooterEntry {
	return []filterFooterEntry{
		{Key: []keyGlyph{{multiSelectNavGlyph, th.AccentKey}}, Label: multiSelectNavLabel},
		{Key: []keyGlyph{{multiSelectToggleGlyph, th.AccentKey}}, Label: multiSelectToggleLabel},
		{Key: []keyGlyph{{multiSelectPreviewGlyph, th.AccentKey}}, Label: multiSelectPreviewLabel},
		{Key: []keyGlyph{{multiSelectOpenGlyph, th.AccentKey}}, Label: multiSelectOpenLabel},
		{Key: []keyGlyph{{multiSelectCancelGlyph, th.AccentKey}}, Label: multiSelectCancelLabel},
	}
}

// renderMultiSelectFooter renders the multi-select mode footer: the five pinned
// entries as a dot-separated left cluster over the shared 1px footer top rule,
// with NO right-aligned `? help` anchor (the delivered frame has none). It reuses
// renderFilterCluster (via fitFilterCluster) for the cluster body so the dot
// separators, canvas-painted gaps, and NO_COLOR carve-out match the other footers
// byte-for-byte, and hands the width-pad to assembleRightAnchoredRow with an EMPTY
// right segment (the pad-to-width path — no anchor). At a narrow width
// fitFilterCluster drops trailing entries with an ellipsis so the row degrades on ONE
// line without wrapping. Two rows (rule + entry row), height-neutral against the
// reserved sessionFooterHeight budget. It does NOT route through
// renderFilterFooter/filterFooterRow (those hardcode the sessionsKeymap `? help`
// anchor).
func renderMultiSelectFooter(width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	rule := footerTopRule(w, th, colourless)
	left, leftWidth := fitFilterCluster(multiSelectFooterEntries(th), w, th, colourless)
	row := assembleRightAnchoredRow(left, leftWidth, "", 0, w, th, colourless)
	return lipgloss.JoinVertical(lipgloss.Left, rule, row)
}

// fitFilterCluster renders the given filter-footer entries as a dot-separated left
// cluster that fits within w cells, greedily including entries in order and, when the
// full cluster does not fit, dropping trailing entries and appending an ellipsis
// marker so the row degrades on ONE line without wrapping. It owns only the
// per-glyph filterFooterEntry cluster renderer + its budget (the multi-select footer
// has no right anchor, so the full width is the budget), delegating the shared
// try-full-then-greedy-prefix-with-ellipsis loop to fitClusterToWidth — the SAME loop
// fitLeftCluster (the keymap-descriptor footer's fitter) uses, so the two can never
// drift. Returns the rendered cluster and its exact rendered width (always ≤ w).
func fitFilterCluster(entries []filterFooterEntry, w int, th theme.Theme, colourless bool) (string, int) {
	// The multi-select footer has no right anchor, so the full width is the budget.
	sep := renderFooterDetail(footerEntrySeparator, th, colourless)
	ellipsis := renderFooterDetail(footerEllipsis, th, colourless)
	renderCluster := func(n int) (string, int) {
		cluster := renderFilterCluster(entries[:n], th, colourless)
		return cluster, lipgloss.Width(cluster)
	}
	return fitClusterToWidth(len(entries), w, renderCluster, sep, ellipsis)
}

// fitClusterToWidth is the shared narrow-degrade fitter behind both the standard
// keymap footer (fitLeftCluster) and the per-glyph filter footer (fitFilterCluster).
// Given the entry count, the width budget, a renderCluster closure that renders the
// first n entries (returning the cluster string and its exact rendered width), and the
// pre-rendered separator + ellipsis chrome runs, it returns the widest fitting cluster
// and its rendered width (always ≤ budget). The algorithm is unchanged from the two former
// per-caller copies: try the full cluster first (the common, wide-terminal case), then
// greedily grow a leading prefix appending a `<cluster> · …` separator+ellipsis, then
// fall back to the bare ellipsis, then an empty cluster at extreme narrowness. Only this
// try-full-then-greedy-prefix-with-ellipsis loop is shared — the per-type cluster
// renderers (renderFooterCluster / renderFilterCluster) and each caller's budget
// computation (full width vs right-anchor-reserved) stay caller-owned.
func fitClusterToWidth(count, budget int, renderCluster func(n int) (string, int), sep, ellipsis string) (string, int) {
	// Try the full cluster first (the common, wide-terminal case).
	if full, fullWidth := renderCluster(count); fullWidth <= budget {
		return full, fullWidth
	}

	// Narrow degrade: include as many leading entries as fit, then append an
	// ellipsis marker. Find the largest prefix whose rendered width (with the separator
	// + ellipsis appended) still fits the budget.
	ellipsisWidth := lipgloss.Width(ellipsis)
	sepWidth := lipgloss.Width(sep)

	best := ""
	bestWidth := 0
	for n := 1; n <= count; n++ {
		cluster, clusterWidth := renderCluster(n)
		// Width of "<cluster> · …": the cluster, a separator, then the ellipsis.
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

	// Not even one entry + ellipsis fits: render just the ellipsis if it fits, else an
	// empty cluster (the row degrades to blank canvas / the surviving right anchor at
	// extreme narrowness).
	if ellipsisWidth <= budget {
		return ellipsis, ellipsisWidth
	}
	return "", 0
}

// renderCondensedFooter is the shared condensed-footer renderer for a
// per-page keymap descriptor: the descriptor's Core entries form the dot-separated
// left cluster and the single right-aligned entry (the ? help hint) is pinned to
// the right, over the 1px footer top rule. It is the single render entry
// point so a page's composed-view render and its height-budget computation resolve
// the footer against the SAME width/mode and agree on its height exactly. Both the
// Sessions and Projects footers route through here so the two never drift.
func renderCondensedFooter(entries []keymapEntry, width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	rule := footerTopRule(w, th, colourless)
	row := footerKeyRow(entries, w, th, colourless)
	return lipgloss.JoinVertical(lipgloss.Left, rule, row)
}

// footerTopRule renders the full-width 1px footer top rule above the
// condensed footer row. Under the NO_COLOR carve-out the rule keeps its glyphs but
// drops the colour and the canvas, rendering on the terminal's native fg/bg.
// Mirrors headerSeparatorRule, swapping only the rule glyph — the token is the same.
func footerTopRule(w int, th theme.Theme, colourless bool) string {
	rule := strings.Repeat(footerRuleGlyph, w)
	return headerStyle(th.Border, th, colourless).Render(rule)
}

// footerKeyRow renders the single condensed key row for the given keymap
// descriptor: the Core keymap entries as a dot-separated left cluster, then a
// canvas-painted flex spacer, then the right-aligned ? help. The row is always
// exactly w cells wide. When the full left cluster does not fit beside the ? help,
// lower-priority Core entries are dropped (with an ellipsis) until it fits — the
// ? help anchor survives as long as possible, and the row never wraps.
func footerKeyRow(entries []keymapEntry, w int, th theme.Theme, colourless bool) string {
	core, right := splitFooterEntries(entries)

	// Render the right-aligned ? help hint first — it is the surviving anchor, so
	// the left cluster is fitted around the space it reserves. Its key glyph is the
	// only one in accent.primary (the rest are accent.key).
	rightSeg := ""
	rightWidth := 0
	if right != nil {
		rightSeg = renderFooterEntry(*right, th.AccentPrimary, th, colourless)
		rightWidth = lipgloss.Width(rightSeg)
	}

	left, leftWidth := fitLeftCluster(core, w, rightWidth, th, colourless)

	// The fitLeftCluster contract guarantees leftWidth ≤ w; the shared assembler
	// owns the fit test, the narrow-degrade, and the flex-spacer join.
	return assembleRightAnchoredRow(left, leftWidth, rightSeg, rightWidth, w, th, colourless)
}

// assembleRightAnchoredRow lays out a right-anchored footer row of exactly w
// cells: a left cluster (already rendered, leftWidth cells) and a right anchor
// segment (rightSeg, rightWidth cells) pinned to the row's right edge with a
// canvas-painted flex spacer between them. It is the single owner of this
// right-anchor geometry, shared by the standard condensed footer (footerKeyRow) and
// the contextual filter footers (filterFooterRow) so a change to the degrade rule is
// made once. Callers render their OWN left cluster (the footer-specific
// fitLeftCluster ellipsis logic stays out of here).
//
// THE ANCHOR SURVIVES; THE LEFT CLUSTER GIVES WAY BENEATH IT. `? help` is
// never dropped while it fits: it is right-aligned, and it is the escape hatch that
// makes every dropped entry recoverable — the help modal lists the full keymap
// regardless of footer width, so a user on a narrow terminal loses the reminder,
// not the capability. This INVERTS the former rule, which dropped the anchor first
// and padded the left cluster: exactly the width at which the footer stopped
// advertising anything was the width at which it also took away the surface that
// could tell the user what it had stopped advertising.
//
// Four rungs, widest first:
//
//  1. NO ANCHOR (rightSeg == "") — the multi-select footer's shape. Pad the left
//     cluster to width; there is nothing to anchor.
//  2. BOTH FIT (leftWidth+1+rightWidth ≤ w, at least one spacer cell) — cluster,
//     flex spacer, anchor.
//  3. ONLY THE ANCHOR FITS — render it alone, right-aligned. The caller's fitter
//     has already reserved rightWidth+1 for it (fitLeftCluster), so on the standard
//     footer this is reached only once the cluster has degraded to nothing; the
//     contextual filter footers, which do not fit their cluster at all, reach it as
//     soon as the two collide, and there too the anchor is what is kept.
//  4. NOT EVEN THE ANCHOR — an empty row of exactly w canvas cells. Consistent with
//     the degrade-never-break rule, and Portal's documented 40-column minimum sits
//     well above it.
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

// splitFooterEntries partitions a keymap descriptor into the ordered Core entries
// that form the footer's left cluster and the single right-aligned entry (the ?
// help hint) the footer pins to the right. Non-Core (help-only) entries are
// dropped — they live in the ? help modal, not the footer. The
// right-aligned entry is excluded from the left cluster slice.
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

// fitLeftCluster renders the ordered Core entries as a dot-separated left cluster
// that fits within w cells while leaving room for the reserved right anchor
// (rightWidth, plus one spacer cell). It greedily includes entries in priority
// order (descriptor order — navigate is highest priority, projects lowest) and, if
// the full cluster does not fit, drops trailing entries and appends an ellipsis
// marker so the row truncates on ONE line without wrapping. It owns only the
// keymapEntry cluster renderer + its right-anchor-reserved budget, delegating the
// shared narrow-degrade loop to fitClusterToWidth (the SAME loop fitFilterCluster
// uses). Returns the rendered cluster and its exact rendered width (always ≤ w).
func fitLeftCluster(core []keymapEntry, w, rightWidth int, th theme.Theme, colourless bool) (string, int) {
	// The budget the left cluster may occupy: the full width minus the reserved
	// right anchor and one spacer cell. When there is no right anchor the cluster
	// may use the full width.
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

// renderFooterCluster renders the given Core entries joined by the dot separator
// into a single left-cluster string. Each entry's key glyph is accent.key, its
// label text.muted, and the separators text.muted.
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

// renderFooterEntry renders one keymap entry as "<key> <label>" with the key glyph
// in keyTok (accent.key for left-cluster entries, accent.primary for the ? help
// hint) and the label in text.muted, with a single canvas-painted gap between
// them. It routes through the shared renderKeyHint helper (the single canvas space
// renderKeyHint paints matches footerKeyLabelGap, so the output is byte-identical).
func renderFooterEntry(e keymapEntry, keyTok theme.Token, th theme.Theme, colourless bool) string {
	return renderKeyHint(e.Key, e.Action, keyTok, th, colourless)
}

// renderFooterDetail renders a chrome run (a separator or the ellipsis marker) in
// text.muted over the owned canvas.
func renderFooterDetail(s string, th theme.Theme, colourless bool) string {
	return headerStyle(th.TextMuted, th, colourless).Render(s)
}
