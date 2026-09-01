package tmux

import (
	"errors"
	"fmt"
	"slices"
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

// tmux words one absence two ways — "no such session" and "can't find session" —
// and which one a command emits belongs to that command's own lookup path, not
// to the kind of target it takes: show-environment says the first, list-panes
// and kill-session the second. Both report the addressed session not existing,
// so both must reach ErrNoSuchSession. "can't find window" is deliberately
// absent — a missing window inside a live session is not a session absence.
// Case-sensitive on purpose: tmux emits the lowercase forms, and a loose match
// would absorb unrelated phrasings from tools layered on top.
var noSuchSessionStderrSubstrs = []string{"no such session", "can't find session"}

// The multi-%w wrap is required: it keeps both the sentinel and the original
// *CommandError reachable on the same error value.
func wrapNoSuchSession(err error) error {
	if err == nil {
		return nil
	}
	var cmdErr *CommandError
	if errors.As(err, &cmdErr) && reportsNoSuchSession(cmdErr.Stderr) {
		return fmt.Errorf("%w: %w", ErrNoSuchSession, err)
	}
	return err
}

func reportsNoSuchSession(stderr string) bool {
	return slices.ContainsFunc(noSuchSessionStderrSubstrs, func(substr string) bool {
		return strings.Contains(stderr, substr)
	})
}

// ErrUnaddressableSessionName reports a session name that Portal's exact-match
// target form cannot express; discriminate with errors.Is. It is identity-equal
// to tmuxerr.ErrUnaddressableSessionName, which lives in a leaf package so
// internal/state can classify against it without an import cycle.
var ErrUnaddressableSessionName = tmuxerr.ErrUnaddressableSessionName

// targetSeparator divides the session component of a tmux target from its window
// and pane components. tmux offers no escape for one inside a session name, so a
// name carrying it cannot be addressed exactly at all — quoting, backslashes and
// a trailing separator were measured and all fail identically.
const targetSeparator = ":"

// ValidateSessionName reports whether a session name can be addressed by the
// exact-match target every per-session operation composes. The returned error
// wraps ErrUnaddressableSessionName and names the offending character, so a
// caller can surface the refusal verbatim.
func ValidateSessionName(name string) error {
	if strings.Contains(name, targetSeparator) {
		return fmt.Errorf("%w: %q contains %q, which tmux reserves as a target separator",
			ErrUnaddressableSessionName, name, targetSeparator)
	}
	return nil
}

// wrapSessionTargetErr classifies a failed per-session operation. The name check
// must come first: tmux answers an unaddressable name with the same "no such
// session" stderr a vanished one produces, and wrapNoSuchSession would hand the
// capture loop a live session dressed as natural churn. An operation classifies
// inside its own message wrap, so the sentinel and tmux's own words both stay
// reachable on the error its caller receives.
func wrapSessionTargetErr(session string, err error) error {
	if err == nil {
		return nil
	}
	if nameErr := ValidateSessionName(session); nameErr != nil {
		return fmt.Errorf("%w: %w", nameErr, err)
	}
	return wrapNoSuchSession(err)
}
