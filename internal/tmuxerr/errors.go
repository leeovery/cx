package tmuxerr

import "errors"

// ErrNoSuchSession is wrapped by per-session tmux operations when tmux reports
// the addressed session does not exist. internal/tmux re-exports it as
// tmux.ErrNoSuchSession; the two symbols are identity-equal.
var ErrNoSuchSession = errors.New("no such session")

// ErrUnaddressableSessionName is wrapped by per-session tmux operations whose
// target Portal's exact-match form cannot express: tmux reserves ":" as a target
// separator and offers no escape for one inside a session name. It is deliberately
// distinct from ErrNoSuchSession — tmux answers such a target with the very same
// "no such session" stderr, and reading that as a vanished session drops a live
// session from the capture with nothing to show for it.
var ErrUnaddressableSessionName = errors.New("session name not addressable by exact target")
