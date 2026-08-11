package tui

import "github.com/leeovery/portal/internal/tmux"

// TmuxEnumerator is the seam through which preview reads window-grouped pane
// structure for a session. It mirrors *tmux.Client.ListWindowsAndPanesInSession
// exactly, so production wiring is a direct method-value handoff.
type TmuxEnumerator interface {
	ListWindowsAndPanesInSession(session string) ([]tmux.WindowGroup, error)
}

// ScrollbackReader is the seam through which preview reads the last N lines of
// a pane's saved scrollback, keyed by the canonical paneKey the daemon writes
// with. The state directory is hidden behind the interface — the production
// adapter closes over it at construction, and mocks key on paneKey alone.
//
// Tail returns three shapes the caller maps to distinct renderings:
//
//   - (bytes != nil, nil) — normal content, rendered verbatim.
//   - (nil, nil) — no content available; collapses ENOENT, a zero-byte file,
//     and a file holding only an unterminated partial line.
//   - (nil, err != nil) — OS-level read failure.
//
// The three no-content cases are unified by design: the placeholder/error
// decision lives at the call site, not in the helper.
type ScrollbackReader interface {
	Tail(paneKey string) ([]byte, error)
}
