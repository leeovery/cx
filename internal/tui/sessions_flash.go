package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

// The flash state primitives (setFlash, clearFlash, flashText, flashGen) live
// on Model; this file holds the tick-clear plumbing.

// flashKind is the styling variant of an active flash. The zero value is
// flashWarning, so an unparameterised setFlash stays the warning band.
type flashKind int

const (
	flashWarning flashKind = iota
	flashSuccess
)

// flashOrigin exists for one reason: the theme signals outrank the filter line
// in the notice band's order. The flash has to carry that discrimination —
// inferring it from the message text would re-order a signal on a copy edit
// and would grant the tier to any flash that happened to mention a theme.
type flashOrigin int

const (
	// flashOriginDefault is the zero value, so the construction-time seed keeps
	// today's order without naming an origin.
	flashOriginDefault flashOrigin = iota
	flashOriginTheme
)

const flashAutoClearDuration = 3 * time.Second

// Gen is the flashGen captured when the tick was scheduled; the Update handler
// compares it against the live value so a superseded flash's tick cannot
// early-clear the current one.
type flashTickMsg struct {
	Gen uint64
}

// The gen is captured by value at call time, so later setFlash calls bump the
// live gen without affecting a pending tick.
func flashTickCmd(gen uint64) tea.Cmd {
	return tea.Tick(flashAutoClearDuration, func(time.Time) tea.Msg {
		return flashTickMsg{Gen: gen}
	})
}

// isActionableKey reports whether a key press should clear an active flash,
// including while filter input is focused. The zero-zero shape (no Code, no
// Text) is treated as non-actionable — a guard against library-emitted no-op
// key events; every real keystroke satisfies one of the two conditions.
func isActionableKey(msg tea.KeyPressMsg) bool {
	return msg.Code != 0 || msg.Text != ""
}

// Literal quote bytes, never %q, so output is byte-exact regardless of name
// content. Callers must not modify the returned string.
func formatSessionGoneFlash(name string) string {
	return fmt.Sprintf(`session "%s" no longer exists`, name)
}
