package tui

import (
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/leeovery/portal/internal/warning"
)

type BootstrapWarning = warning.Warning

var flushWarningsToStderr = func(warnings []BootstrapWarning) {
	WriteBootstrapWarnings(os.Stderr, warnings)
}

// WriteBootstrapWarnings ignores write errors — diagnostics must not
// themselves fail the program.
func WriteBootstrapWarnings(w io.Writer, warnings []BootstrapWarning) {
	warning.WriteLines(w, warnings)
}

func SetFlushWarningsToStderrForTest(fn func([]BootstrapWarning)) func() {
	prev := flushWarningsToStderr
	flushWarningsToStderr = fn
	return func() { flushWarningsToStderr = prev }
}

func formatWarningsFlash(warnings []BootstrapWarning) string {
	var lines []string
	for _, w := range warnings {
		lines = append(lines, w.Lines...)
	}
	return strings.Join(lines, "\n")
}

// Bubble Tea v2 has no imperative alt-screen commands, so the stderr write
// stands alone and the lines surface when the alt-screen is torn down on exit.
func (m *Model) flushBufferedWarningsCmd() tea.Cmd {
	if len(m.bufferedWarnings) == 0 {
		return nil
	}
	warnings := m.bufferedWarnings
	m.bufferedWarnings = nil
	return func() tea.Msg {
		flushWarningsToStderr(warnings)
		return nil
	}
}

// The concurrent cold/TUI route surfaces warnings as an in-TUI band — a
// stderr flush mid-run would be lost under the live alt-screen.
func (m *Model) surfaceBufferedWarnings() tea.Cmd {
	if m.progressReceiver == nil {
		return m.flushBufferedWarningsCmd()
	}
	message := formatWarningsFlash(m.bufferedWarnings)
	m.bufferedWarnings = nil
	if message == "" {
		return nil
	}
	m.setFlash(message)
	return flashTickCmd(m.flashGen)
}
