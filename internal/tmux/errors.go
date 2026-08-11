package tmux

import (
	"errors"
	"fmt"
	"strings"

	"github.com/leeovery/portal/internal/tmuxerr"
)

// ErrNoSuchSession is wrapped into the error of a per-session tmux operation
// whose stderr reports the addressed session does not exist; discriminate with
// errors.Is. Layers above internal/tmux must not substring-match tmux stderr
// themselves — tmux's phrasing is not a stable contract. It is identity-equal to
// tmuxerr.ErrNoSuchSession, which lives in a leaf package so internal/state can
// classify against it without an import cycle.
var ErrNoSuchSession = tmuxerr.ErrNoSuchSession

// ErrEmptyPaneList reports that a pane enumeration succeeded against a session
// that exists but listed no panes — an unusual transient (e.g. a pane
// mid-respawn), observably distinct from ErrNoSuchSession.
var ErrEmptyPaneList = errors.New("empty pane list")

// ErrPanePIDParse reports a pane enumeration whose first line is not a base-10
// pane pid.
var ErrPanePIDParse = errors.New("pane pid parse")

// Case-sensitive on purpose: tmux emits the lowercase form, and a loose match
// would absorb unrelated phrasings from tools layered on top.
const noSuchSessionStderrSubstr = "no such session"

// The multi-%w wrap is required: it keeps both the sentinel and the original
// *CommandError reachable on the same error value.
func wrapNoSuchSession(err error) error {
	if err == nil {
		return nil
	}
	var cmdErr *CommandError
	if errors.As(err, &cmdErr) && strings.Contains(cmdErr.Stderr, noSuchSessionStderrSubstr) {
		return fmt.Errorf("%w: %w", ErrNoSuchSession, err)
	}
	return err
}
