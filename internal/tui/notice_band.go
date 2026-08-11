package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// The notice slot holds at most one band: a transient flash takes it
// temporarily, replacing any persistent band for its duration.

const noticeBarGlyph = "▌"

// The status glyphs keep the warning/success variants distinguishable by glyph
// alone, which is what survives NO_COLOR. The persistent info band carries
// none.
const (
	flashWarningGlyph = "⚠"
	flashSuccessGlyph = "✓"
)

// noticeBandRole selects the band's left-bar colour and tint. The two info
// bands (signpost and command-pending) are the same element at the base — same
// bar, same tint — and bandCommand only layers a caret + chip on top.
type noticeBandRole int

const (
	bandWarning noticeBandRole = iota
	bandSuccess
	bandInfo
	bandCommand
)

const commandBandCaret = "▸"

const commandBandText = "Pick a project to run"

func (r noticeBandRole) barToken(th theme.Theme) theme.Token {
	switch r {
	case bandWarning:
		return th.AccentAttention
	case bandSuccess:
		return th.StatePositive
	default:
		return th.AccentPrimary
	}
}

// Both flashes share one tint — the bar colour and ✓/⚠ glyph carry the
// distinction — and both info bands share another, because the signpost and
// the command banner must look identical at the base.
func (r noticeBandRole) tintToken(th theme.Theme) theme.Token {
	switch r {
	case bandInfo, bandCommand:
		return th.BgSelection
	default:
		return th.BgAttention
	}
}

func (r noticeBandRole) statusGlyph() string {
	switch r {
	case bandWarning:
		return flashWarningGlyph
	case bandSuccess:
		return flashSuccessGlyph
	default:
		return ""
	}
}

// Every band cell paints through this so the whole row is one uniform tint
// with no terminal-bg island.
func noticeBandTintStyle(tint theme.Token, th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Background(tint.Color())
}

func noticeBandFgStyle(fg, tint theme.Token, th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Foreground(fg.Color()).
		Background(tint.Color())
}

// bandBase is the shared base every band render derives from, so the two info
// bands cannot diverge in bar glyph, bar colour, or tint.
type bandBase struct {
	tint theme.Token
	bar  string
	gap  string
}

func newBandBase(role noticeBandRole, th theme.Theme, colourless bool) bandBase {
	tint := role.tintToken(th)
	return bandBase{
		tint: tint,
		bar:  noticeBandFgStyle(role.barToken(th), tint, th, colourless).Render(noticeBarGlyph),
		gap:  noticeBandTintStyle(tint, th, colourless).Render(" "),
	}
}

// renderNoticeBand renders the band, wrapping the message to the width left
// after the prefix. A message that fits stays one line; otherwise the result is
// multi-line, with the `▌` bar repeated on every line and continuation lines
// indented under line 1's message start (the status glyph is line 1 only).
// onBandText is the caller-selected message token.
func renderNoticeBand(role noticeBandRole, message string, onBandText theme.Token, width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	base := newBandBase(role, th, colourless)
	tint := base.tint
	gap := base.gap
	bar := base.bar
	fg := noticeBandFgStyle(onBandText, tint, th, colourless)

	glyph := role.statusGlyph()
	prefixWidth := lipgloss.Width(noticeBarGlyph) + 1
	if glyph != "" {
		prefixWidth += lipgloss.Width(glyph) + 1
	}

	// A pathologically narrow band degrades to a 1-cell column so the bar still
	// renders.
	avail := max(w-prefixWidth, 1)
	wrapped := strings.Split(ansi.Wrap(message, avail, ""), "\n")

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

const commandChipPadX = 1

// The command-pending banner: the shared info-band base with a caret, the
// fixed lead text, and the pending command in a chip layered on.
func renderCommandBand(command []string, width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	base := newBandBase(bandCommand, th, colourless)

	caret := noticeBandFgStyle(bandCommand.barToken(th), base.tint, th, colourless).Render(commandBandCaret)
	text := noticeBandFgStyle(th.TextOnSelection, base.tint, th, colourless).Render(commandBandText)
	chip := renderCommandChip(strings.Join(command, " "), th, colourless)

	row := lipgloss.JoinHorizontal(lipgloss.Top, base.bar, base.gap, caret, base.gap, text, base.gap, chip)
	return noticeBandPadRight(row, lipgloss.Width(row), w, base.tint, th, colourless)
}

// Under NO_COLOR the chip degrades to a padded colourless box, still
// distinguishable by position.
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

func noticeBandPadRight(seg string, segWidth, w int, tint theme.Token, th theme.Theme, colourless bool) string {
	return padRightWithStyle(seg, segWidth, w, noticeBandTintStyle(tint, th, colourless))
}

// activeNoticeBand is the Sessions single-slot arbiter: a live flash wins,
// otherwise the no-tags signpost holds the slot unless multi-select or the
// unsupported banner has replaced the section header.
//
// The flash arm deliberately ignores m.multiSelectMode so a spawn-failure
// flash co-renders with the `N selected` banner on the section-header row —
// the user sees the failure and that the retry set is still marked. Do not add
// `&& !m.multiSelectMode` to force a single row.
//
// The filter line has no arm here: it claims the section-header row, a
// different physical row, so the two co-render.
func (m Model) activeNoticeBand() (role noticeBandRole, message string, ok bool) {
	if role, message, live := m.flashSlotClaim(); live {
		return role, message, true
	}
	// unsupportedBannerActive is the same predicate applySectionHeader's
	// claimant reads, so the suppression and the swap cannot drift.
	if m.byTagSignpost && !m.multiSelectMode && !m.unsupportedBannerActive() {
		return bandInfo, byTagSignpostText, true
	}
	// ok=false, so the returned role is a never-rendered placeholder.
	return bandWarning, "", false
}

// activeProjectNoticeBand is the Projects single-slot arbiter. Projects gets
// the flash contender alone — the signpost, multi-select, unsupported and
// burst-progress bands are all Sessions elements with no Projects analogue, so
// do not port one here for symmetry.
//
// The slot is deliberately not scoped to a set of flash origins: closing the
// theme panel discharges an outstanding failed-commit report whether or not a
// flash rendered, so a slot filtered by set-site would destroy the report
// rather than defer it.
func (m Model) activeProjectNoticeBand() (role noticeBandRole, message string, ok bool) {
	if role, message, live := m.flashSlotClaim(); live {
		return role, message, true
	}
	if m.commandPending {
		return bandCommand, commandBandText, true
	}
	return bandWarning, "", false
}

// The bandCommand arm composes its own caret + chip from the model's pending
// command, so it does not consume the returned message.
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

func flashBandRole(kind flashKind) noticeBandRole {
	if kind == flashSuccess {
		return bandSuccess
	}
	return bandWarning
}

func (m Model) flashClaim() (role noticeBandRole, message string, ok bool) {
	if m.flashText == "" {
		return bandWarning, "", false
	}
	return flashBandRole(m.flashKind), m.flashText, true
}

// Reads the origin the flash carries and never the message text: precedence
// keyed on wording would move a signal out of the tier on a copy edit.
func (m Model) themeFlashClaim() (role noticeBandRole, message string, ok bool) {
	if m.flashOrigin != flashOriginTheme {
		return bandWarning, "", false
	}
	return m.flashClaim()
}

// flashSlotClaim holds the flash tier order for both pages, written once so a
// later tier lands in one place.
//
// The filter line's position is between the two arms below: the theme tier
// outranks it, every other flash sits under it. That asymmetry is load-bearing
// — a filter can sit applied-but-unfocused for a whole panel session, and a
// theme signal that cannot claim the slot beneath it is destroyed rather than
// deferred (closing the panel discharges the failed-commit report either way),
// while the proactive NO_COLOR block would produce nothing at all. A contender
// inserted between the arms must not pre-empt the theme tier.
func (m Model) flashSlotClaim() (role noticeBandRole, message string, ok bool) {
	if role, message, live := m.themeFlashClaim(); live {
		return role, message, true
	}
	return m.flashClaim()
}

// text.on-selection is co-tuned for the info band's tint; the flashes carry
// text.on-attention.
func noticeBandOnBandText(role noticeBandRole, th theme.Theme) theme.Token {
	if role == bandInfo {
		return th.TextOnSelection
	}
	return th.TextOnAttention
}

func (m Model) renderActiveNoticeBand() string {
	role, message, ok := m.activeNoticeBand()
	if !ok {
		return ""
	}
	return renderNoticeBand(role, message, noticeBandOnBandText(role, m.themeState.active), m.contentWidth(), m.themeState.active, m.colourless)
}

// Both the composition and the height reserve consume this, so the reserved
// row count is by construction the rendered height. The blank is an explicit
// canvas row rather than a bare "" so the slot's height is deterministic.
func (m Model) renderSessionBandSlot() string {
	band := m.renderActiveNoticeBand()
	if band == "" {
		return ""
	}
	blank := blankCanvasRow(m.contentWidth(), m.themeState.active, m.colourless)
	return lipgloss.JoinVertical(lipgloss.Left, band, blank)
}
