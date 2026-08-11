package tui

import "github.com/leeovery/portal/internal/tmux"

type TmuxEnumerator interface {
	ListWindowsAndPanesInSession(session string) ([]tmux.WindowGroup, error)
}

// Keyed by the canonical paneKey the daemon writes with; the state directory is
// hidden behind the interface. Tail returns three distinct shapes:
//
//   - (bytes, nil) — content, rendered verbatim.
//   - (nil, nil) — no content; collapses ENOENT, a zero-byte file, and a file
//     holding only an unterminated partial line.
//   - (nil, err) — OS-level read failure.
type ScrollbackReader interface {
	Tail(paneKey string) ([]byte, error)
}
