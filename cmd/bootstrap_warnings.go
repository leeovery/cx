package cmd

import (
	"io"
	"sync"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/tui"
	"github.com/leeovery/portal/internal/warning"
)

// BootstrapWarningsSink accumulates the orchestrator's soft bootstrap warnings
// until a consumer drains them. Every operation is safe for concurrent use:
// warnings are added on the main goroutine but Bubble Tea may drain from
// another.
type BootstrapWarningsSink struct {
	mu       sync.Mutex
	warnings []bootstrap.Warning
}

// Add appends a single warning to the sink.
func (s *BootstrapWarningsSink) Add(w bootstrap.Warning) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.warnings = append(s.warnings, w)
}

// Drain returns every buffered warning and clears the sink atomically, so
// concurrent callers receive disjoint slices. An empty sink returns nil.
func (s *BootstrapWarningsSink) Drain() []bootstrap.Warning {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.warnings
	s.warnings = nil
	return out
}

// EmitTo drains the sink and writes every warning's lines to w in
// orchestrator-observation order. It delegates to warning.WriteLines so the CLI
// and TUI paths produce byte-identical output.
func (s *BootstrapWarningsSink) EmitTo(w io.Writer) {
	warning.WriteLines(w, s.Drain())
}

var bootstrapWarnings = &BootstrapWarningsSink{}

// Called between building the model and starting Bubble Tea, so the warnings
// ride the first BootstrapCompleteMsg and reach stderr only once the loading
// page has dismissed.
func stageBootstrapWarningsOnModel(m *tui.Model) {
	pending := bootstrapWarnings.Drain()
	if len(pending) == 0 {
		return
	}
	m.SetPendingBootstrapWarnings(pending)
}
