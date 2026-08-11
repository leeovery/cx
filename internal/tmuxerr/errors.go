package tmuxerr

import "errors"

// ErrNoSuchSession is wrapped by per-session tmux operations when tmux reports
// the addressed session does not exist. internal/tmux re-exports it as
// tmux.ErrNoSuchSession; the two symbols are identity-equal.
var ErrNoSuchSession = errors.New("no such session")
