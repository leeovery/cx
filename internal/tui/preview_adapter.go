package tui

import (
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

const previewTailLines = 1000

// stateDir is closed over once at TUI startup so preview reads the same
// directory the daemon and bootstrap write to.
type scrollbackReaderAdapter struct {
	stateDir string
	n        int
}

// NewProductionScrollbackReader returns the production ScrollbackReader
// closing over stateDir, with the tail-line floor fixed at previewTailLines.
// Preview never resolves stateDir on its own.
func NewProductionScrollbackReader(stateDir string) ScrollbackReader {
	return scrollbackReaderAdapter{stateDir: stateDir, n: previewTailLines}
}

// Tail resolves the on-disk path and reads the last n newline-terminated
// lines. The three return shapes documented on ScrollbackReader.Tail flow
// through unchanged — this adapter adds no policy of its own.
func (a scrollbackReaderAdapter) Tail(paneKey string) ([]byte, error) {
	path := state.ScrollbackFile(a.stateDir, paneKey)
	return state.TailScrollback(path, a.n)
}

// In a non-test file so a seam regression breaks the production build.
var (
	_ TmuxEnumerator   = (*tmux.Client)(nil)
	_ ScrollbackReader = scrollbackReaderAdapter{}
)
