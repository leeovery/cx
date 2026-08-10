package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// The shared notice-band primitive + single-slot arbiter.
//
// Inline notices follow one convention: a `▌` left-bar accent line directly under
// the title separator, above the section header (full-width), the section header +
// list shifting down. The notice slot holds AT MOST ONE band — a transient flash
// takes the slot temporarily, replacing any persistent band (the no-tags signpost,
// the command-pending banner) for its duration, then the persistent band returns. A
// persistent (violet info) band and a transient flash never display at once — the
// transient flash wins while shown.
//
// This file owns the band ROLE vocabulary, the shared render primitive, and the
// Sessions-page arbiter that funnels both notice sources through a single slot.
// The per-band tint, on-band text token, message wording, and (for flashes) the
// ⚠/✓ glyph + bold/dim are selected by the consuming call sites; this file supplies
// the structural slot and the height recompute that goes with it.

// noticeBarGlyph is the left-bar accent glyph — a solid block pinned far-left in
// the band's role colour (accent.attention / state.positive / accent.primary). Under
// NO_COLOR the glyph + its position survive; only the colour drops.
const noticeBarGlyph = "▌"

// flashWarningGlyph / flashSuccessGlyph are the flash status glyphs. They follow
// the message on the warning / success flash band so the two variants stay
// distinguishable by GLYPH alone — never colour-only — which is what keeps the
// NO_COLOR carve-out legible: the ⚠ / ✓ survives even when the tint and bar colour
// drop. The persistent info band (the signpost) carries NO status glyph.
const (
	flashWarningGlyph = "⚠"
	flashSuccessGlyph = "✓"
)

// noticeBandRole is one of the notice-band role variants. The role selects the
// left-bar colour token and the band's tint:
//
//   - bandWarning → accent.attention (transient / warning flash, bg.attention tint)
//   - bandSuccess → state.positive   (transient / success flash, bg.attention tint)
//   - bandInfo    → accent.primary (persistent info band, bg.selection tint)
//   - bandCommand → accent.primary (persistent command-pending banner, bg.selection tint)
//
// The two INFO bands (the signpost's bandInfo and the command-pending bandCommand)
// are the SAME info-message element: an identical violet `▌` left-bar on the SAME
// bg.selection tint. They share one render base (renderNoticeBand) so their bar +
// tint can never drift; bandCommand layers a `▸` caret + an orange command chip on
// top (renderCommandBand). The flash roles keep their own bg.attention/glyph treatment.
type noticeBandRole int

const (
	// bandWarning is the transient warning flash role — an accent.attention left-bar.
	bandWarning noticeBandRole = iota
	// bandSuccess is the transient success flash role — a state.positive left-bar.
	bandSuccess
	// bandInfo is the persistent info band role — an accent.primary left-bar on
	// the subtle bg.selection tint (the SAME tint as the command-pending band:
	// the signpost and the command banner are one info-message element).
	bandInfo
	// bandCommand is the persistent command-pending banner role — an
	// accent.primary left-bar on the bg.selection tint, identical to bandInfo's base;
	// it layers a `▸` caret + an orange command chip on top (renderCommandBand).
	bandCommand
)

// commandBandCaret is the `▸` lead glyph that follows the `▌` left-bar on the
// command-pending banner, just before the fixed text. It survives the NO_COLOR
// carve-out (its position/glyph carry the banner's intent colourlessly).
const commandBandCaret = "▸"

// commandBandText is the fixed banner wording, sourced once here
// as the single source of truth. The joined pending command renders beside it in an
// accent.attention chip (renderCommandBand).
const commandBandText = "Pick a project to run"

// barToken returns the role token whose foreground paints the role's left-bar. No
// literal hex survives here — the colour is sourced from the closed token
// vocabulary so a re-theme moves the band bars with every other element.
func (r noticeBandRole) barToken(th theme.Theme) theme.Token {
	switch r {
	case bandWarning:
		return th.AccentAttention
	case bandSuccess:
		return th.StatePositive
	default: // bandInfo, bandCommand
		return th.AccentPrimary
	}
}

// tintToken returns the surface token whose background fills the band's row.
// The transient flashes (warning AND success) sit on the single co-tuned
// bg.attention tint — no invented success-specific tint; the bar colour + ✓
// glyph carry the success distinction. The TWO persistent info bands —
// the signpost's bandInfo AND the command-pending bandCommand — share the
// SAME violet-anchored bg.selection surface, because they are one info-message
// element (the signpost and the command banner must look identical at the base:
// same `▌` bar, same tint). This single shared mapping is the regression guard
// that keeps the two info bands from drifting apart. The pairing is
// closed-vocabulary only (no invented token, no literal hex).
func (r noticeBandRole) tintToken(th theme.Theme) theme.Token {
	switch r {
	case bandInfo, bandCommand:
		return th.BgSelection
	default: // bandWarning, bandSuccess
		return th.BgAttention
	}
}

// statusGlyph returns the status glyph that follows the bar on the role's band, or
// "" for the persistent info band (which carries no status glyph). The glyph is
// what keeps the warning / success variants distinguishable without relying on
// colour, and is what survives the NO_COLOR carve-out.
func (r noticeBandRole) statusGlyph() string {
	switch r {
	case bandWarning:
		return flashWarningGlyph
	case bandSuccess:
		return flashSuccessGlyph
	default: // bandInfo
		return ""
	}
}

// noticeBandTintStyle returns the band's BACKGROUND-fill style: Background(tint)
// for the role's surface token (bg.attention for the flashes, bg.selection for the
// info bands), or a bare style under the NO_COLOR carve-out so the band
// renders on the terminal's native bg. Every band cell (bar, glyph, message, the
// gaps, and the right pad) is painted through this so the whole row is one
// uniform tint with no terminal-bg island.
func noticeBandTintStyle(tint theme.Token, th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Background(tint.Color())
}

// noticeBandFgStyle returns the on-band FOREGROUND style: the supplied role token
// over the band's tint background — the bar (role colour) and the on-band text /
// glyph (onBandText) leg both paint through this so their cells carry the same
// tint as the gaps. Under NO_COLOR it is a bare style (no hue, no tint).
func noticeBandFgStyle(fg, tint theme.Token, th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Foreground(fg.Color()).
		Background(tint.Color())
}

// bandBase is the info-band base treatment shared by EVERY band render path: the
// role's resolved tint token plus the two pre-rendered cells that paint that tint —
// the `▌` left-bar (in the role colour) and a single tint-painted gap cell. Both
// renderNoticeBand (the signpost and the flashes) and
// renderCommandBand (the command-pending banner) derive their bar + tint from
// here, so the two info bands (signpost, command) can never diverge in bar glyph,
// bar colour, or tint — there is exactly ONE place that assembles them.
type bandBase struct {
	tint theme.Token // the role's surface tint (bg.selection for the info bands)
	bar  string      // the `▌` left-bar cell, painted in the role colour on the tint
	gap  string      // a single tint-painted space cell
}

// newBandBase builds the shared info-band base for a role: the tint token + the
// tint-painted `▌` bar and gap cells. The NO_COLOR carve-out is honoured by
// the underlying style helpers, so under colourless the bar/gap carry no SGR.
func newBandBase(role noticeBandRole, th theme.Theme, colourless bool) bandBase {
	tint := role.tintToken(th)
	return bandBase{
		tint: tint,
		bar:  noticeBandFgStyle(role.barToken(th), tint, th, colourless).Render(noticeBarGlyph),
		gap:  noticeBandTintStyle(tint, th, colourless).Render(" "),
	}
}

// renderNoticeBand renders the shared notice band: a far-left `▌` left-bar in
// the role colour, then — for the flashes — a `⚠`/`✓` status glyph, then the
// message in the supplied on-band text token, all on the role's tint and padded to
// exactly width cells so each line spans the full row like the section header it
// sits above (single-line when the message fits, wrapping to multi-line otherwise
// — see below). The flash bands (warning / success) fill the row with the
// bg.attention tint; the persistent info band (the signpost) carries no status glyph
// and sits on the bg.selection tint — the SAME tint as the command-pending banner,
// since the two are one info-message element. This is the shared base both
// info bands render through (renderCommandBand layers its caret + chip on top), so
// the bar + tint can never diverge between them.
//
// The message WRAPS to the available content width (= width − the prefix width: the
// `▌` bar + its gap + (for flashes) the `⚠`/`✓` glyph + its gap). When the message
// is longer than that width the band returns a MULTI-LINE string (lines joined with
// "\n"); when it fits it stays a single line. The `▌` bar repeats on EVERY wrapped
// line (in the role colour) so the bar spans the band's full height, and
// continuation lines indent their text under line 1's message start (the `⚠`/`✓`
// glyph appears only on line 1). Wrapping is on word boundaries; a word longer than
// the available width is hard-broken. Every line is padded to exactly width cells
// with the role's tint (noticeBandPadRight) so the tint spans all wrapped lines with
// no terminal-bg island on any line.
//
// onBandText is the text token the caller selects for the message (text.on-attention
// for the flashes, text.on-selection for the signpost). The role selects the tint
// and the status glyph (role.tintToken / role.statusGlyph).
//
// Under the NO_COLOR carve-out the bar colour, the on-band text hue, and the tint
// all drop; the `▌` bar, its position, the `⚠`/`✓` glyph (line 1), and the message
// text survive on the terminal's native fg/bg so the band's STATE stays legible
// colourlessly — glyph-distinct, never colour-only. The bar survives
// on every wrapped line.
func renderNoticeBand(role noticeBandRole, message string, onBandText theme.Token, width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	base := newBandBase(role, th, colourless)
	tint := base.tint
	gap := base.gap
	bar := base.bar
	fg := noticeBandFgStyle(onBandText, tint, th, colourless)

	// Prefix laid out before the message on line 1: the `▌` bar + gap, plus (for the
	// flashes) the status glyph + gap. The bar (1) + gap (1) [+ glyph (1) + gap (1)]
	// fixes the cell width the message starts at — the available content width and
	// the continuation-line indent both derive from it so the wrapped text lines up
	// under line 1's message start.
	glyph := role.statusGlyph()
	prefixWidth := lipgloss.Width(noticeBarGlyph) + 1 // bar + its gap
	if glyph != "" {
		prefixWidth += lipgloss.Width(glyph) + 1 // glyph + its gap
	}

	// Wrap the message to the width remaining after the prefix on word boundaries,
	// hard-breaking only a word longer than the available width (ansi.Wrap breaks a
	// word that does not fit). A non-positive available width (pathologically narrow
	// band) degrades to a 1-cell column so the bar still renders.
	avail := max(w-prefixWidth, 1)
	wrapped := strings.Split(ansi.Wrap(message, avail, ""), "\n")

	// Continuation lines lay the bar + gap (2 cells), then pad the REMAINING prefix
	// cells (prefixWidth − bar − gap = the glyph + gap slot, when present) so the
	// wrapped text aligns under line 1's message start; the glyph itself is line 1
	// only. With no glyph (prefixWidth == 2) the pad is empty and continuation text
	// already aligns under line 1's message.
	barGapWidth := lipgloss.Width(noticeBarGlyph) + 1
	var contIndent string
	if prefixWidth > barGapWidth {
		contIndent = noticeBandTintStyle(tint, th, colourless).Render(strings.Repeat(" ", prefixWidth-barGapWidth))
	}

	lines := make([]string, 0, len(wrapped))
	for i, text := range wrapped {
		segs := []string{bar, gap}
		if i == 0 {
			if glyph != "" {
				segs = append(segs, fg.Render(glyph), gap)
			}
		} else if contIndent != "" {
			segs = append(segs, contIndent)
		}
		segs = append(segs, fg.Render(text))

		row := lipgloss.JoinHorizontal(lipgloss.Top, segs...)
		lines = append(lines, noticeBandPadRight(row, lipgloss.Width(row), w, tint, th, colourless))
	}

	return strings.Join(lines, "\n")
}

// commandChipPadX is the command chip's horizontal padding (cells each side)
// inside its tinted box — so the chip reads `│ npm run dev │` compactly, matching
// the reference's compact orange command chip.
const commandChipPadX = 1

// renderCommandBand renders the command-pending banner: the info-band BASE (the
// bandCommand role's `▌` violet left-bar on the bg.selection tint, sourced from
// the SAME newBandBase used by the signpost so the bar + tint cannot
// diverge between the two info bands) WITH a `▸` violet caret + the fixed `Pick
// a project to run` text (text.on-selection) + the joined pending command in an
// accent.attention chip layered on, padded to exactly width cells so the band
// occupies the full row like the section header it sits above.
//
// The chip is a small box treatment: the joined command in accent.attention on a
// bg.attention surface fill with one cell of horizontal padding each side, so it
// reads as a distinct orange chip ON the violet-tinted band. Both tints are
// existing surface tokens (no invented token, no literal hex).
//
// Under the NO_COLOR carve-out the bar/caret/text/chip colours and both tints drop;
// the `▌` bar, the `▸` caret, the text, and the chip command survive on the
// terminal's native fg/bg, so the chip degrades to a colourless padded box still
// distinguishable by position, never colour-only.
func renderCommandBand(command []string, width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	base := newBandBase(bandCommand, th, colourless)

	caret := noticeBandFgStyle(bandCommand.barToken(th), base.tint, th, colourless).Render(commandBandCaret)
	text := noticeBandFgStyle(th.TextOnSelection, base.tint, th, colourless).Render(commandBandText)
	chip := renderCommandChip(strings.Join(command, " "), th, colourless)

	row := lipgloss.JoinHorizontal(lipgloss.Top, base.bar, base.gap, caret, base.gap, text, base.gap, chip)
	return noticeBandPadRight(row, lipgloss.Width(row), w, base.tint, th, colourless)
}

// renderCommandChip renders the command chip: the joined command in
// accent.attention on a bg.attention surface fill (the orange-anchored subtle
// tint) with commandChipPadX cells of horizontal padding each side, so it reads
// as a distinct orange box ON the violet-tinted band. Under NO_COLOR all
// colours + the fill drop, leaving a padded colourless box distinguishable by
// position.
func renderCommandChip(command string, th theme.Theme, colourless bool) string {
	if colourless {
		pad := strings.Repeat(" ", commandChipPadX)
		return pad + command + pad
	}
	chip := lipgloss.NewStyle().
		Foreground(th.AccentAttention.Color()).
		Background(th.BgAttention.Color()).
		Padding(0, commandChipPadX).
		Render(command)
	return chip
}

// noticeBandPadRight pads the assembled band row out to exactly w cells with
// tint-painted spaces, so the band carries its tint on every cell to the right
// edge with no terminal-bg island. A row already at/over w is returned unchanged
// (the band is clamped to w by construction at the call site). It binds the band's
// tint fill and delegates the pad geometry to the shared padRightWithStyle (the
// same core headerPadRight routes through, which pads with the canvas instead).
func noticeBandPadRight(seg string, segWidth, w int, tint theme.Token, th theme.Theme, colourless bool) string {
	return padRightWithStyle(seg, segWidth, w, noticeBandTintStyle(tint, th, colourless))
}

// activeNoticeBand is the single-slot arbiter for the Sessions page: it
// returns the ONE band that owns the notice slot, if any. The single-slot rule —
// a transient flash wins over any persistent band while shown; otherwise the
// active persistent band occupies the slot:
//
//   - a THEME-ORIGIN flash → the slot, ABOVE the filter line (see below).
//   - flashText != ""  → the transient flash takes the slot; its flashKind selects
//     the warning (bandWarning) or success (bandSuccess) styling.
//   - byTagSignpost     → the persistent no-tags info band (bandInfo),
//     UNLESS multi-select mode is active, which replaces the section header with
//     the `N selected` banner and suppresses the signpost (a flash still wins).
//
// At most one is ever returned, so the two independent band sources collapse to a
// single arbitrated insert (no double-band). The role/message the arbiter returns
// are consumed by viewSessionList's single insertion step; the on-band text token
// is selected at the render site so each band keeps its existing on-band colour.
//
// A THEME-ORIGIN FLASH OUTRANKS THE FILTER LINE, AND ONLY IT DOES. The filter
// line is the one contender above the flash tier that can be live throughout a
// panel's open, use and close — a filter can sit applied-but-unfocused on this list
// the whole time — and two guarantees fail SILENTLY if a theme signal cannot claim
// the slot beneath it:
//
//   - The failed-commit report. Closing the panel DISCHARGES the outstanding
//     failure whether or not a flash rendered, so a report that cannot claim the
//     slot is DESTROYED rather than deferred — which is the silent revert that
//     report exists to close.
//   - The proactive `NO_COLOR` block, which would produce nothing at all:
//     the walkable dead end it exists to prevent, reached by another route.
//
// The asymmetry is the justification: a filter line is a persistent restatement of a
// state the user can already see in their own list, while each theme flash reports a
// one-time event with no other surface. The tier is therefore the FIRST arm here,
// and the ordinary flash keeps its own position below — a later contender inserted
// between them must not be allowed to pre-empt the theme tier.
//
// The filter line itself has NO arm here: it claims the SECTION-HEADER row
// (applySectionHeader), a different physical row, so the two co-render today and the
// ordering above is what keeps that true if either ever moves.
//
// The PROJECTS page has its own arbiter (activeProjectNoticeBand) over its own
// contender set — the transient flash and nothing else. The two are
// deliberately separate functions rather than one page-parameterised arbiter:
// every contender here except the flash is a Sessions element, so a merged one
// would be a page switch wearing a shared name.
func (m Model) activeNoticeBand() (role noticeBandRole, message string, ok bool) {
	// Precedence seam — do NOT collapse to strict single-slot. This flash arm takes
	// the band slot REGARDLESS of m.multiSelectMode, so the spawn-failure /
	// permission flash (setFlash from handleBurstPartialFailure) deliberately
	// CO-RENDERS with the `N selected` multi-select banner across TWO physical rows —
	// this band under the title separator, the banner on the section-header row
	// (applySectionHeader). This is informative by design: the user sees the failure
	// AND that they remain in multi-select with the retry set still marked. Its
	// pre-flight-abort SIBLING in the same flash tier is DIFFERENT by design: that one
	// is a section-header claimant (applySectionHeader's abortBannerText branch) that
	// DOES replace the banner. A later reader must NOT add `&& !m.multiSelectMode`
	// here to force a single row.
	//
	// Both flash tiers carry that exception: the theme panel NESTS over multi-select,
	// so a theme signal raised there co-renders with the banner too.
	if role, message, live := m.themeFlashClaim(); live {
		return role, message, true
	}
	// The filter line's position in the order is HERE, between the two flash tiers:
	// the theme tier above it, every other flash below. It claims the SECTION-HEADER
	// row rather than this slot, so it has no arm of its own.
	if role, message, live := m.flashClaim(); live {
		return role, message, true
	}
	// Multi-select mode replaces the section header with the `N selected` banner and
	// a resolved-unsupported terminal replaces it with the `⚠ unsupported terminal`
	// banner; both own the section-header row, so the persistent By-Tag signpost is
	// suppressed while either does (a transient flash, handled above, still wins the
	// slot). unsupportedBannerActive is the SAME predicate applySectionHeader's own
	// claimant reads, so the suppression and the swap can never drift.
	if m.byTagSignpost && !m.multiSelectMode && !m.unsupportedBannerActive() {
		return bandInfo, byTagSignpostText, true
	}
	// No active band: ok=false, so the role is don't-care (callers gate on ok); the
	// returned bandWarning is an arbitrary placeholder that is never rendered.
	return bandWarning, "", false
}

// activeProjectNoticeBand is the single-slot arbiter for the PROJECTS page: it
// returns the ONE band that owns that page's notice slot, if any.
//
// Projects has a transient-flash slot of its own because `t` is bound on this page
// and every theme flash is reachable here, while the Sessions arbiter's other
// contenders are all Sessions elements.
//
// The order — a transient flash wins the slot while shown, otherwise the
// command-pending banner holds it:
//
//   - a THEME-ORIGIN flash → the slot, ABOVE the filter line (see below).
//   - flashText != "" → the transient flash takes the slot; its flashKind selects
//     the warning (bandWarning) or success (bandSuccess) styling.
//   - commandPending  → the persistent command-pending banner (bandCommand).
//
// A THEME-ORIGIN FLASH OUTRANKS THE FILTER LINE HERE TOO, and for the same
// two guarantees the Sessions arbiter states — the failed-commit report, which is
// DESTROYED rather than deferred if it cannot claim the slot (closing the panel
// discharges the outstanding failure whether or not a flash rendered), and the
// proactive `NO_COLOR` block, which would otherwise produce nothing at all. `t` is
// bound on this page because theme is a global setting, so every one of those
// signals is reachable here and the rule cannot be Sessions-only. The filter line
// claims this page's section-header row (applyProjectsSectionHeader), a different
// physical row, so the two co-render; the ordering below is what keeps the theme
// tier above it if either ever moves.
//
// PROJECTS GETS THE FLASH CONTENDER ALONE, not the full arbiter. The no-tags
// signpost, the multi-select banner, the unsupported banner and the burst-progress
// band are all Sessions elements with NO Projects analogue, so a later reader must
// not port one here to make the two arbiters look symmetrical. The asymmetry is
// the decision.
//
// It ARBITRATES, never co-renders: the band is one slot, and the Sessions arm's
// deliberate multi-select co-render exception (see activeNoticeBand) has no
// Projects analogue — nothing on this page owns a second row to co-render with.
//
// The slot is deliberately NOT scoped to the entry-blocked flashes: it takes any
// flashText, whatever set it. The `theme not saved — see portal.log` report lands
// in this same slot, and closing the panel DISCHARGES that outstanding
// state whether or not a flash rendered — so a slot that filtered by set-site
// would destroy the report rather than defer it.
//
// The bandCommand arm's message is the banner's fixed lead text; the banner's
// caret + orange command chip are composed by renderCommandBand from the model's
// pending command, so that arm's render does not consume the returned message.
func (m Model) activeProjectNoticeBand() (role noticeBandRole, message string, ok bool) {
	if role, message, live := m.themeFlashClaim(); live {
		return role, message, true
	}
	// The filter line's position in the order is HERE, between the two flash tiers;
	// it owns the section-header row rather than this slot, so it has no arm.
	if role, message, live := m.flashClaim(); live {
		return role, message, true
	}
	if m.commandPending {
		return bandCommand, commandBandText, true
	}
	// No active band: ok=false, so the role is don't-care (callers gate on ok); the
	// returned bandWarning is an arbitrary placeholder that is never rendered.
	return bandWarning, "", false
}

// renderActiveProjectNoticeBand renders the arbitrated Projects-page notice band
// for the model's current width / resolved canvas mode (and the NO_COLOR
// carve-out), or the empty string when no band owns the slot. It is the single
// render entry point the band SLOT (renderProjectBandSlot) composes beneath the
// title separator, and from which the slot's height is measured, so the budget and
// the render agree.
//
// The flash arm routes through the SAME renderNoticeBand primitive and the SAME
// on-band text token the Sessions band uses, so a flash renders byte-identically
// on either page for the same message and width — there is one flash band, shown
// in two places, not two implementations that could drift.
func (m Model) renderActiveProjectNoticeBand() string {
	role, message, ok := m.activeProjectNoticeBand()
	if !ok {
		return ""
	}
	if role == bandCommand {
		return renderCommandBand(m.command, m.contentWidth(), m.themeState.active, m.colourless)
	}
	return renderNoticeBand(role, message, noticeBandOnBandText(role, m.themeState.active), m.contentWidth(), m.themeState.active, m.colourless)
}

// flashBandRole maps an inline-flash kind to the shared notice-band role
// that selects its bar colour, tint, and status glyph. flashWarning is the
// default, so an unparameterised setFlash routes to bandWarning.
func flashBandRole(kind flashKind) noticeBandRole {
	if kind == flashSuccess {
		return bandSuccess
	}
	return bandWarning
}

// flashClaim is the transient-flash contender: the band a live flash claims —
// the role its kind selects plus its verbatim message — or ok=false when no flash is
// live. Both pages' arbiters resolve the flash through it, so the one contender they
// share cannot come to mean two things.
func (m Model) flashClaim() (role noticeBandRole, message string, ok bool) {
	if m.flashText == "" {
		return bandWarning, "", false
	}
	return flashBandRole(m.flashKind), m.flashText, true
}

// themeFlashClaim is flashClaim narrowed to the THEME-ORIGIN tier — the flashes
// that claim the notice slot even while the filter line is live. It reads the origin
// the flash carries (setThemeFlash) and NEVER the message text: precedence keyed on
// wording would move a signal out of the tier on a copy edit, and would let any
// unrelated message that happened to mention a theme fall into it.
func (m Model) themeFlashClaim() (role noticeBandRole, message string, ok bool) {
	if m.themeState.flashOrigin != flashOriginTheme {
		return bandWarning, "", false
	}
	return m.flashClaim()
}

// noticeBandOnBandText selects the on-band text token for the arbitrated
// band role. The warning/success flash carries text.on-attention; the persistent
// info signpost carries text.on-selection — the bright white co-tuned for the
// bg.selection tint the info band sits on (the same token the selected
// session-row name uses on that surface), so the message stays legible.
func noticeBandOnBandText(role noticeBandRole, th theme.Theme) theme.Token {
	if role == bandInfo {
		return th.TextOnSelection
	}
	return th.TextOnAttention
}

// renderActiveNoticeBand renders the arbitrated Sessions-page notice band for the
// model's current width / resolved canvas mode (and the NO_COLOR carve-out), or
// the empty string when no band owns the slot. It is the single render entry
// point the band SLOT (renderSessionBandSlot) composes beneath the title
// separator, and from which the slot's height is measured, so the budget and the
// render agree.
func (m Model) renderActiveNoticeBand() string {
	role, message, ok := m.activeNoticeBand()
	if !ok {
		return ""
	}
	return renderNoticeBand(role, message, noticeBandOnBandText(role, m.themeState.active), m.contentWidth(), m.themeState.active, m.colourless)
}

// renderSessionBandSlot renders the FULL Sessions notice slot for the model's
// current width / canvas mode — the arbitrated band PLUS one canvas-painted
// full-width blank row BENEATH it (the band→section-header breathing gap), or the
// empty string when no band owns the slot. The blank sits ONLY below the band: the
// band stays flush under the title separator, the blank separates it from the
// section header (line 0 of listView), so the slot composes as band → blank →
// listView.
//
// This is the SINGLE source of truth for what the slot inserts: both
// viewSessionList (composition) and sessionBandHeight (the height reserve)
// consume it, so the reserved row count is, by construction, exactly the rendered
// height of what is composed — the two can never drift. The blank is an explicit
// canvas-painted full-width row (blankCanvasRow) rather than a bare "" left for
// the outer fill to pad, so the slot's height is deterministic and the row paints
// to the owned canvas with no ragged/zero-width gap (and survives NO_COLOR).
func (m Model) renderSessionBandSlot() string {
	band := m.renderActiveNoticeBand()
	if band == "" {
		return ""
	}
	blank := blankCanvasRow(m.contentWidth(), m.themeState.active, m.colourless)
	return lipgloss.JoinVertical(lipgloss.Left, band, blank)
}
