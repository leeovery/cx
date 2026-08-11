package tui

import (
	"github.com/leeovery/portal/internal/spawn"
)

// Prunes the gone set so survivors stay marked and a second Enter proceeds
// with them rather than re-aborting; stays in multi-select mode.
func (m Model) handlePreflightAbort(msg spawnAbortMsg) Model {
	m.emitPreflightAbort(msg.Gone)

	m.abortBannerText = spawn.GoneMessage(msg.Gone)

	m.goneFlagged = make(map[string]struct{}, len(msg.Gone))
	for _, name := range msg.Gone {
		m.goneFlagged[name] = struct{}{}
		delete(m.selectedSessions, name)
	}

	(&m).resetBurstState()
	(&m).refreshSessionDelegate()
	return m
}

// WithInitialGoneFlagged seeds the pre-flight abort state at construction —
// the capture-harness entry point. A nil/empty slice is a no-op.
func WithInitialGoneFlagged(names []string) Option {
	return func(m *Model) {
		if len(names) == 0 {
			return
		}
		m.abortBannerText = spawn.GoneMessage(names)
		m.goneFlagged = make(map[string]struct{}, len(names))
		for _, name := range names {
			m.goneFlagged[name] = struct{}{}
		}
		m.refreshSessionDelegate()
	}
}

func (m *Model) clearAbortBanner() {
	m.abortBannerText = ""
	m.goneFlagged = nil
	m.refreshSessionDelegate()
}
