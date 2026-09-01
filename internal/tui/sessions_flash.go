package tui

import (
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/leeovery/portal/internal/tmux"
)

// The zero value is flashWarning, so an unparameterised setFlash stays a warning.
type flashKind int

const (
	flashWarning flashKind = iota
	flashSuccess
)

// Carried by the flash so a theme signal's precedence tier is granted at set time
// rather than inferred from message text, which a copy edit would silently move a
// signal out of. What the tier does and does not arbitrate is on flashSlotClaim.
type flashOrigin int

const (
	flashOriginDefault flashOrigin = iota
	flashOriginTheme
)

const flashAutoClearDuration = 3 * time.Second

// Gen is the flashGen captured when the tick was scheduled, compared against the
// live value so a superseded flash's tick cannot early-clear the current one.
type flashTickMsg struct {
	Gen uint64
}

func flashTickCmd(gen uint64) tea.Cmd {
	return tea.Tick(flashAutoClearDuration, func(time.Time) tea.Msg {
		return flashTickMsg{Gen: gen}
	})
}

// The zero-zero shape (no Code, no Text) is non-actionable — a guard against
// library-emitted no-op key events; every real keystroke satisfies one condition.
func isActionableKey(msg tea.KeyPressMsg) bool {
	return msg.Code != 0 || msg.Text != ""
}

// Literal quote bytes, never %q, so output is byte-exact.
func formatSessionGoneFlash(name string) string {
	return fmt.Sprintf(`session "%s" no longer exists`, name)
}

// The band's own voice for each rename refusal. Neither carries a ⚠ glyph — the
// warning band prepends one.
const (
	renameSeparatorRefusedFlash = `":" isn't allowed in a session name — tmux reads it as a separator`
	renameIDPrefixRefusedFlash  = `"$" isn't allowed at the start of a session name — tmux reads it as a session ID`
)

// renameRefusalFlash picks the band's wording from the rule the validator
// reported, so the refusal names the character the user actually typed.
func renameRefusalFlash(err error) string {
	if errors.Is(err, tmux.ErrSessionNameIDPrefix) {
		return renameIDPrefixRefusedFlash
	}
	return renameSeparatorRefusedFlash
}
