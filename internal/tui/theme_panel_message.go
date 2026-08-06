package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// §9.1's MESSAGE SLOT: the single row (or two) directly above the panel's vertical
// keymap footer, and the two contenders that can occupy it.
//
// IT IS A SINGLE-SLOT ARBITER WITH EXACTLY TWO CONTENDERS, AND THEIR EXCLUSION IS
// ASSERTED RATHER THAN RANKED. The contenders are §9.2's slot-from-constant
// confirm and §9.13's failed-commit line, and §9.1 is explicit that they "can never
// be live at once because a confirm resolves before any write happens": the confirm
// gates the write, so by the time a write can fail the confirm has already
// resolved. There is therefore NO precedence rule to invent here — the state is
// unreachable by construction, and the value below is shaped so that installing one
// contender structurally clears the other.
//
// THE MESSAGE CARRIES NO FREE TEXT. Each contender renders its own pinned §14A
// copy from the constants below, so no call site can paraphrase it and the copy
// stays single-sourced — the same discipline spawn.UnsupportedNoopMessage applies
// to the burst's user-facing strings. The only datum a contender carries is the
// confirm's slug, and that is an identity rather than a sentence.
//
// THE SLOT IS NOT RESERVED WHEN EMPTY (§9.1): it appears and the list shrinks, the
// same way the main screen's notice band recomputes list height. Its height is
// MEASURED off the rendered block (themePanelMessageHeight), so a message that
// wraps shrinks the list by two rows rather than one.

const (
	// themePanelConfirmFormat is §14A's slot-from-constant confirm, VERBATIM —
	// `clear constant <slug>?  y / n`, with TWO spaces before the `y` exactly as
	// pinned.
	//
	// It is one format string rather than a prefix/suffix pair so the copy reads as
	// §14A writes it, and so the frame's own width can be derived from it
	// (themePanelConfirmFixedWidth) rather than restated as a literal.
	themePanelConfirmFormat = "clear constant %s?  y / n"

	// themePanelCommitFailedMessage is §14A's failed-commit line, VERBATIM. It is
	// GLYPH-BACKED per §9.13 and Portal's convention — the `⚠` is part of the
	// string, so the report survives the NO_COLOR carve-out where the
	// accent.attention hue does not.
	themePanelCommitFailedMessage = flashWarningGlyph + " couldn't save theme"
)

// themePanelMessageKind names which of §9.1's two contenders holds the slot.
// themeMessageNone is the zero value and the empty slot, which is what makes an
// unset panel's slot cost nothing without a second flag to keep in step.
type themePanelMessageKind int

const (
	// themeMessageNone is the empty slot: no contender, no row.
	themeMessageNone themePanelMessageKind = iota
	// themeMessageConfirm is §9.2's slot-from-constant confirm, naming the constant
	// that will be cleared.
	themeMessageConfirm
	// themeMessageCommitFailed is §9.13's failed-commit line.
	themeMessageCommitFailed
)

// themePanelMessage is the slot's value: WHICH contender is live, and the one
// datum the confirm needs.
//
// ONE Kind FIELD IS THE EXCLUSION. Two contenders cannot be live at once because
// there is only one field to say which is, and every writer below assigns the WHOLE
// value rather than mutating a field — so a confirm's slug cannot survive into a
// failed-commit line as residue.
type themePanelMessage struct {
	// Kind is the live contender, themeMessageNone for an empty slot.
	Kind themePanelMessageKind
	// Slug is the persisted CONSTANT being cleared — the confirm's datum, unset for
	// every other kind. It is the RAW persisted key rather than the slug the panel
	// resolved: see Model.raiseThemePanelConfirm.
	Slug string
}

// raiseThemePanelConfirm raises §9.2's slot-from-constant confirm, naming the
// constant it will clear.
//
// IT READS THE RAW KEYS, NOT THE NOMINATION. m.themeKeys.Theme is what is
// PERSISTED, and under §8.5's fallback that is not what is on screen: the persisted
// slug may be the very one that failed to load, with a built-in fallback rendering
// in its place. The confirm has to name what is being CLEARED, so it must name the
// persisted string even when nothing loaded from it — naming the resolution would
// ask the user to confirm clearing a theme they never set.
//
// Installing the whole value is what clears the other contender (see
// themePanelMessage).
func (m *Model) raiseThemePanelConfirm() {
	m.setThemePanelMessage(themePanelMessage{Kind: themeMessageConfirm, Slug: m.themeKeys.Theme})
}

// raiseThemePanelCommitFailed raises §9.13's failed-commit line, clearing whatever
// the slot held.
func (m *Model) raiseThemePanelCommitFailed() {
	m.setThemePanelMessage(themePanelMessage{Kind: themeMessageCommitFailed})
}

// clearThemePanelMessage empties the slot, which costs the panel no row at all
// (§9.1's unreserved-when-empty rule).
func (m *Model) clearThemePanelMessage() {
	m.setThemePanelMessage(themePanelMessage{})
}

// setThemePanelMessage installs the slot's whole value AND re-derives the panel's
// layout from it — the single act every writer above performs, so none of them can
// do half of it.
//
// THE RE-SIZE IS NOT COSMETIC. The slot changes the panel's vertical budget in BOTH
// directions at once (themePanelListSize): the message costs its measured rows,
// while §9.2's nested confirm scope hands two footer rows back. renderThemePanel
// sizes its per-frame copy from the message it is handed, so a model left on the
// pre-message budget keeps a `bubbles/list` PerPage the drawn frame does not have —
// and `Ctrl+↑`/`Ctrl+↓` then move a different distance than the screen scrolls, with
// no rendered frame revealing it. That is the class resizeThemePanel re-sizes for,
// and the one the main screen already answers with resyncPageLayouts on every notice
// band raise and clear.
//
// IT IS HERE RATHER THAN AT THE CALL SITES for the same reason: the confirm swallows
// the arrows (§9.2), so the stale page is harmless while IT is live — but §9.13's
// failed-commit line persists with the arrows LIVE, so the fix has to be inherited
// by whoever raises it rather than remembered at each raise.
//
// It re-invokes the SAME function the open and the resize run rather than a second
// copy of the arithmetic, which is what keeps the three in step; the delegate
// re-point it carries is idempotent (a message is not a theme swap).
//
// IT ASSUMES THE PANEL IS OPEN, which every writer above is: the confirm's raise and
// clear are panel-scoped and §9.13's line is raised from a commit, which only fires
// while themePanel.open. A call with the panel CLOSED would size a zero list.Model
// against the closed panel's zero width — unreachable today, and left unguarded
// rather than defended speculatively; task 9-7 owns the one message specified to
// OUTLIVE a keypress, so it is that task's business to keep the assumption true.
func (m *Model) setThemePanelMessage(message themePanelMessage) {
	m.themePanel.message = message
	m.applyThemePanelListStyles()
}

// themePanelFooterScope is the keymap scope the panel's footer renders for the
// slot's current state: §9.2's NESTED CONFIRM SCOPE while the confirm is live, the
// standing panel scope otherwise.
//
// THE CONFIRM CANNOT ADVERTISE THE STANDING FOOTER. It is key-exclusive within the
// panel and resolves on `y`/`n`/`Esc` alone, so `⏎ set theme` / `d set as dark` /
// `l set as light` / `esc close` would list four keys of which NONE would act —
// and §14.3 is firm that advertising a key that will not act is the dead end a
// proactive block exists to prevent.
//
// IT SUBSTITUTES A SCOPE, IT DOES NOT FORK A RENDERER. Both scopes go through
// renderThemePanelFooter and both heights through themePanelFooterHeight, so there
// is no second footer implementation to drift — which is the same drift class the
// descriptor discipline closes one layer up.
func themePanelFooterScope(message themePanelMessage) []keymapEntry {
	if message.Kind == themeMessageConfirm {
		return themePanelConfirmKeymap()
	}
	return themePanelKeymap()
}

// themePanelMessageWrapRows is the slot's height when it WRAPS — §9.1's "at the
// minimum panel width the slot may wrap to two rows", read as the slot's maximum
// rather than as an observation about one string.
//
// The cap is what keeps the vertical budget provably bounded. themePanelBlock can
// only CUT an over-long assembly, and it cuts from the BOTTOM — off the footer,
// `esc close` first — so an unbounded slot is one long message away from taking
// the key that closes a panel the user can no longer read the way out of.
const themePanelMessageWrapRows = 2

// renderThemePanelMessage renders §9.1's message slot: the row (or two) directly
// above the vertical keymap footer, or "" when the slot is empty.
//
// IT IS NOT RESERVED WHEN EMPTY — it appears and the list shrinks, the same way the
// main screen's notice band recomputes list height.
//
// THE TWO DIMENSIONS DEGRADE DIFFERENTLY, ON PURPOSE (§9.1). At the minimum panel
// WIDTH the slot may wrap to two rows — it is not a list delegate, so wrapping costs
// pagination nothing. At the minimum HEIGHT it is TRUNCATED to one line instead,
// because §9.8's floor counts exactly one message row: a two-row message there would
// leave zero list rows or overflow the frame, and truncating degrades the message
// the user is being asked to answer rather than the row they are answering ABOUT.
// wrap carries that decision in from the caller (themePanelMessageWraps), so the
// budget and the block resolve it once and identically.
//
// The wrap is CAPPED at themePanelMessageWrapRows with the overflow truncated into
// the last row, so the slot's cost is bounded whatever it is handed (see that
// constant). A zero or negative inner width takes the truncating path, which is also
// the only width ansi.Wrap declines to act on.
func renderThemePanelMessage(message themePanelMessage, inner int, wrap bool, th theme.Theme, colourless bool) string {
	text, token := themePanelMessageContent(message, inner, th)
	if text == "" {
		return ""
	}
	return headerStyle(token, th, colourless).Render(themePanelMessageText(text, inner, wrap))
}

// themePanelMessageContent is the ARBITER: the live contender's pinned copy and the
// §9.1 role token it renders in, decided in ONE place so a contender cannot get one
// without the other.
//
// NEITHER CONTENDER CARRIES A BAND (§9.1's table). The confirm is `text.secondary`
// on the panel's own canvas; the failed-commit line puts BOTH the `⚠` and the text
// in `accent.attention` — one run, exactly as the pinned directory row does — and
// deliberately takes NO `bg.attention` band: that is a full-width main-screen flash
// treatment and would read as heavy inside a 27–34 column panel.
//
// The empty slot returns no text, which is what renderThemePanelMessage reads as
// "render nothing"; its zero token is never consulted.
func themePanelMessageContent(message themePanelMessage, inner int, th theme.Theme) (text string, token theme.Token) {
	switch message.Kind {
	case themeMessageConfirm:
		return themePanelConfirmText(message.Slug, inner), th.TextSecondary
	case themeMessageCommitFailed:
		return themePanelCommitFailedMessage, th.AccentAttention
	}
	return "", theme.Token{}
}

// themePanelConfirmText composes §14A's confirm around the persisted constant, with
// the slug truncated by §9.5's rule when it does not fit.
//
// §9.5's RULE IS THE ROW-COMPOSITION ONE, applied to the slot: the pinned frame is
// charged first and the one flexing element takes what is left, floored at three
// visible characters plus the ellipsis (themeRowLabelFloor). Truncating the SLUG
// rather than letting the slot truncate the whole line is what keeps `?  y / n` in
// the copy — the tail the user needs to answer the question — however long the
// persisted string is.
//
// THE FLOOR CAN EXCEED THE WIDTH, and that is §9.5's shape rather than a defect:
// below the floor the label simply stops shrinking, and the slot's own
// per-dimension rule (wrap above the height floor, truncate at it) then governs the
// composed line.
func themePanelConfirmText(slug string, inner int) string {
	return fmt.Sprintf(themePanelConfirmFormat, ansi.Truncate(slug, themePanelConfirmSlugBudget(inner), themeRowEllipsis))
}

// themePanelConfirmSlugBudget is the cells §9.5's rule leaves the slug: the inner
// width less the pinned frame's own cost, floored at three visible characters plus
// the ellipsis.
func themePanelConfirmSlugBudget(inner int) int {
	return max(inner-themePanelConfirmFixedWidth(), themeRowLabelFloor)
}

// themePanelConfirmFixedWidth is the pinned frame's cost — §14A's format rendered
// with an EMPTY slug — so the charge is derived from the copy itself and a reworded
// confirm cannot leave a stale literal behind.
func themePanelConfirmFixedWidth() int {
	return lipgloss.Width(fmt.Sprintf(themePanelConfirmFormat, ""))
}

// themePanelMessageText lays a composed message out for the slot — one truncated
// line, or up to themePanelMessageWrapRows wrapped ones with the overflow truncated
// into the last.
func themePanelMessageText(message string, inner int, wrap bool) string {
	width := max(inner, 0)
	if !wrap || width == 0 {
		return ansi.Truncate(message, width, themeRowEllipsis)
	}
	lines := strings.Split(ansi.Wrap(message, width, ""), "\n")
	if len(lines) <= themePanelMessageWrapRows {
		return strings.Join(lines, "\n")
	}
	head := lines[:themePanelMessageWrapRows-1]
	tail := ansi.Truncate(strings.Join(lines[themePanelMessageWrapRows-1:], " "), width, themeRowEllipsis)
	return strings.Join(append(head, tail), "\n")
}

// themePanelMessageHeight is the message slot's MEASURED height — the value
// themePanelListSize subtracts, so a message appearing costs the list exactly the
// rows the slot renders. It measures the real renderer (with a zero theme and the
// colourless path, as themePanelFooterHeight does), so the two cannot drift and a
// WRAPPED message is charged its two rows rather than an assumed one.
func themePanelMessageHeight(message themePanelMessage, inner int, wrap bool) int {
	return blockHeight(renderThemePanelMessage(message, inner, wrap, theme.Theme{}, true))
}

// themePanelMessageWraps reports whether the slot may wrap at the height the panel
// is being rendered at — §9.1's per-dimension rule resolved in ONE place, so the
// budget themePanelListSize reserves and the block renderThemePanel assembles can
// never disagree about how many rows the message costs.
//
// At or below §9.8's floor the slot truncates; above it, it wraps. `at or below`
// rather than `at`, because renderThemePanel is a pure function of the height it is
// handed and a sub-floor height is a shape a fixture can hand it.
//
// THE FLOOR IT COMPARES AGAINST IS THE STANDING SCOPE'S, exactly as
// themePanelFloor's is: see themePanelMinHeight for why the confirm's shorter
// footer never moves it.
func themePanelMessageWraps(p themePanel, height int) bool {
	return height > themePanelMinHeight(themePanelKeymap(), p.union.DirUnusable)
}
