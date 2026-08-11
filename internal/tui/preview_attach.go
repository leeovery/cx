package tui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/tmux"
)

// In a non-test file so a seam drift breaks the production build.
var _ previewAttachTmux = (*tmux.Client)(nil)

type previewAttachTmux interface {
	HasSessionProbe(name string) (bool, error)
	SelectWindow(session string, window int) error
	SelectPane(session string, window, pane int) error
}

// Enter must abandon the connector handoff. Session is empty when the defensive
// empty-session guard fires before any tmux call.
type previewAttachBailMsg struct {
	Session string
}

// The handler must record the name and tea.Quit before the connector runs:
// switch-client inside the live tea.Cmd goroutine left an orphan portal process
// event-looping with no UI.
type previewAttachSelectedMsg struct {
	Session string
}

// Does not own the connector handoff — cmd/open.go performs that post-TUI.
type previewAttachPipeline struct {
	tmux   previewAttachTmux
	logger *slog.Logger
}

// logger must be non-nil.
func NewPreviewAttachPipeline(t previewAttachTmux, logger *slog.Logger) PreviewAttacher {
	return &previewAttachPipeline{tmux: t, logger: logger}
}

// Blocking tmux calls are fine here — the sequence is sub-millisecond and runs
// off the UI thread. The select calls are best-effort.
func (p *previewAttachPipeline) Run(session string, window, pane int) tea.Cmd {
	return func() tea.Msg {
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
