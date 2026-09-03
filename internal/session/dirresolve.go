package session

import (
	"fmt"
	"strings"

	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/resolver"
	"github.com/leeovery/portal/internal/tmux"
)

// PaneCurrentPathReader reads the active pane's current_path for a named session.
// Deliberately this narrow: with no all-panes method to call, "active pane only"
// is structural rather than a convention.
type PaneCurrentPathReader interface {
	ActivePaneCurrentPath(session string) (string, error)
}

var _ PaneCurrentPathReader = (*tmux.Client)(nil)

// ResolveSessionDir derives a session's directory from its active pane's
// current_path, resolved to a git root and reduced to the project store's
// canonical key. It is the fallback for a session carrying no @portal-dir stamp.
//
// ok==false with a nil error means the session is unresolvable this pass —
// killed mid-resolve, or no readable current_path — which must never abort the
// grouped render. Both arrive the same way, and an empty path is the whole
// signal: the production shape is a reader answering an unmatched target with
// an empty expansion at exit 0, so a session that is gone reaches here as an
// empty path rather than as an error, and the empty check below is what
// catches it. A non-nil error from the reader is therefore an unexpected
// pane-read failure — no server to connect to, say — and is reported as one,
// never absorbed as routine session churn.
func ResolveSessionDir(session string, reader PaneCurrentPathReader, runner resolver.CommandRunner) (string, bool, error) {
	paneCwd, err := reader.ActivePaneCurrentPath(session)
	if err != nil {
		return "", false, fmt.Errorf("failed to read active pane path for session %q: %w", session, err)
	}

	// Guard before ResolveGitRoot, which would os.Stat("") and error.
	if strings.TrimSpace(paneCwd) == "" {
		return "", false, nil
	}

	// ResolveGitRoot errors only when paneCwd is gone from disk — unresolvable
	// this pass, not fatal.
	gitRoot, err := resolver.ResolveGitRoot(paneCwd, runner)
	if err != nil {
		return "", false, nil
	}

	return project.CanonicalDirKey(gitRoot), true, nil
}
