package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/spawn"
)

// TerminalDetector resolves the host terminal's identity for the picker's
// async detection lifecycle. Production passes a *spawn.Detector built once at
// TUI construction.
type TerminalDetector interface {
	Detect() spawn.Identity
}

// Carries the resolved identity back onto the event loop. Its arm resolves the
// identity to a Resolution and caches both, so the later burst and banner read
// them without re-detecting.
type terminalDetectedMsg struct {
	identity spawn.Identity
}

// WithTerminalDetector wires the async detection seam. Nil-tolerant: a nil
// detector leaves detection permanently unwired.
func WithTerminalDetector(d TerminalDetector) Option {
	return func(m *Model) {
		m.detector = d
	}
}

// WithResolve wires the config-aware identity→adapter/resolution seam — the
// same resolver the multi-target open burst uses, loaded once from
// terminals.json at construction.
func WithResolve(fn spawn.AdapterResolver) Option {
	return func(m *Model) {
		m.resolve = fn
	}
}

// WithInitialDetection seeds the detection cache at construction — the
// capture-harness entry point for the otherwise async lifecycle. A nil
// identity is a no-op.
//
// It caches both halves of one resolve so detectAdapter and detectResolution
// stay in lockstep and a later dispatchBurst cannot disagree with the gate.
// Seeding the Resolution (not just IsNull) is what makes DetectUnsupported
// true for a recognised-but-undriven terminal, so the banner renders from the
// first frame.
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

// Returns the detection command exactly once, guarded by the detectDispatched
// latch, and nil when detection is unwired, already dispatched, or the model
// is off the Sessions page. Detect() runs on Bubble Tea's command goroutine,
// so detection never blocks Update and is never part of the first-paint
// appearance gate.
//
// Pointer receiver: the latch mutation must persist onto the model Update
// returns.
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

// DetectDispatched reports whether the detection command has been dispatched,
// for testing.
func (m Model) DetectDispatched() bool { return m.detectDispatched }

// DetectResolved reports whether the detected identity has been resolved and
// cached, for testing.
func (m Model) DetectResolved() bool { return m.detectResolved }

// DetectedIdentity returns the cached identity, for testing. Zero until
// DetectResolved is true.
func (m Model) DetectedIdentity() spawn.Identity { return m.detectIdentity }

// DetectedResolution returns the cached resolution, for testing. Empty until
// DetectResolved is true.
func (m Model) DetectedResolution() spawn.Resolution { return m.detectResolution }

// DetectUnsupported reports whether this terminal cannot spawn host windows:
// true once detection resolved and the resolution is unsupported. IsNull()
// alone is not the test — a recognised-but-undriven terminal is non-NULL yet
// unsupported. False while detection is in flight.
func (m Model) DetectUnsupported() bool {
	return m.detectResolved && m.detectResolution == spawn.ResolutionUnsupported
}
