package tui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/tmux"
)

// In a non-test file so a drift on Client breaks the build before any caller
// is affected.
var _ previewAttachTmux = (*tmux.Client)(nil)

// previewAttachTmux is the tmux seam for the Enter pre-select pipeline, one
// method per pre-connector call. Production wiring is *tmux.Client.
type previewAttachTmux interface {
	HasSessionProbe(name string) (bool, error)
	SelectWindow(session string, window int) error
	SelectPane(session string, window, pane int) error
}

// Signals that Enter must abandon the connector handoff and take the
// session-killed-externally bail path. Session is empty when the defensive
// empty-session guard fires before any tmux call.
type previewAttachBailMsg struct {
	Session string
}

// The success terminal of the preview-Enter pipeline. The model handler
// records the name and returns tea.Quit so the TUI shuts down before the
// connector runs: running switch-client inside the live tea.Cmd goroutine left
// an orphan portal process event-looping with no UI.
type previewAttachSelectedMsg struct {
	Session string
}

// previewAttachPipeline composes the three pre-select tmux calls. It does not
// own the connector handoff — cmd/open.go performs that post-TUI.
type previewAttachPipeline struct {
	tmux   previewAttachTmux
	logger *slog.Logger
}

// NewPreviewAttachPipeline is the production constructor. logger must be
// non-nil. The seam interface stays unexported; production callers pass
// concrete types that satisfy it structurally.
func NewPreviewAttachPipeline(t previewAttachTmux, logger *slog.Logger) PreviewAttacher {
	return &previewAttachPipeline{tmux: t, logger: logger}
}

// Run returns a tea.Cmd that executes the three pre-select tmux calls in order
// and resolves to either previewAttachBailMsg (the session vanished) or
// previewAttachSelectedMsg. The select calls are best-effort: an error logs a
// WARN and proceeds. Blocking tmux calls are fine here — the sequence is
// sub-millisecond and runs off the UI thread.
func (p *previewAttachPipeline) Run(session string, window, pane int) tea.Cmd {
	return func() tea.Msg {
		// Defensive guard for callers that forgot to gate on a non-empty
		// captured session.
		if session == "" {
			return previewAttachBailMsg{Session: ""}
		}

		present, err := p.tmux.HasSessionProbe(session)
		if !present {
			// present=false dominates: a genuine non-zero tmux exit and the
			// defensive (false, nil) both bail.
			return previewAttachBailMsg{Session: session}
		}
		if err != nil {
			// Present, but an OS-layer fault — warn and proceed.
			p.logger.Warn("has-session probe OS-layer error", "session", session, "error", err)
		}

		// Window/pane indices have no closed attr key, so they are dropped.
		if err := p.tmux.SelectWindow(session, window); err != nil {
			p.logger.Warn("select-window failed", "session", session, "error", err)
		}

		if err := p.tmux.SelectPane(session, window, pane); err != nil {
			p.logger.Warn("select-pane failed", "session", session, "error", err)
		}

		return previewAttachSelectedMsg{Session: session}
	}
}
