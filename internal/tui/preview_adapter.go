package tui

import (
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

const previewTailLines = 1000

// stateDir is fixed at TUI startup so preview reads the daemon's write directory.
type scrollbackReaderAdapter struct {
	stateDir string
	n        int
}

func NewProductionScrollbackReader(stateDir string) ScrollbackReader {
	return scrollbackReaderAdapter{stateDir: stateDir, n: previewTailLines}
}

// The return shapes documented on ScrollbackReader.Tail flow through unchanged —
// this adapter adds no policy of its own.
func (a scrollbackReaderAdapter) Tail(paneKey string) ([]byte, error) {
	path := state.ScrollbackFile(a.stateDir, paneKey)
	return state.TailScrollback(path, a.n)
}

// In a non-test file so a seam regression breaks the production build.
var (
	_ TmuxEnumerator   = (*tmux.Client)(nil)
	_ ScrollbackReader = scrollbackReaderAdapter{}
)
