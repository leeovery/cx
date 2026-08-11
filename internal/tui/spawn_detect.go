package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/spawn"
)

type TerminalDetector interface {
	Detect() spawn.Identity
}

type terminalDetectedMsg struct {
	identity spawn.Identity
}

func WithTerminalDetector(d TerminalDetector) Option {
	return func(m *Model) {
		m.detector = d
	}
}

func WithResolve(fn spawn.AdapterResolver) Option {
	return func(m *Model) {
		m.resolve = fn
	}
}

// Caches both halves of one resolve so detectAdapter and detectResolution stay
// in lockstep. Seeding the Resolution (not just IsNull) is what makes
// DetectUnsupported true for a recognised-but-undriven terminal.
func WithInitialDetection(id *spawn.Identity) Option {
	return func(m *Model) {
		if id == nil {
			return
		}
		adapter, resolution := spawn.ResolveAdapter(*id)
		m.detectIdentity = *id
		m.detectAdapter = adapter
		m.detectResolution = resolution
		m.detectResolved = true
	}
}

// Detect() runs on Bubble Tea's command goroutine, so detection never blocks
// Update and is never part of the first-paint appearance gate. Pointer receiver:
// the latch mutation must persist onto the model Update returns.
func (m *Model) maybeDispatchDetectionCmd() tea.Cmd {
	if m.detector == nil || m.detectDispatched || m.activePage != PageSessions {
		return nil
	}
	m.detectDispatched = true
	detector := m.detector
	return func() tea.Msg {
		return terminalDetectedMsg{identity: detector.Detect()}
	}
}

func (m Model) DetectDispatched() bool { return m.detectDispatched }

func (m Model) DetectResolved() bool { return m.detectResolved }

func (m Model) DetectedIdentity() spawn.Identity { return m.detectIdentity }

func (m Model) DetectedResolution() spawn.Resolution { return m.detectResolution }

// IsNull() alone is not the test — a recognised-but-undriven terminal is non-NULL
// yet unsupported. False while detection is in flight.
func (m Model) DetectUnsupported() bool {
	return m.detectResolved && m.detectResolution == spawn.ResolutionUnsupported
}
