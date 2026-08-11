package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// Answering terminals reply to OSC 11 in single-digit ms; staying under the
// ~100ms perception threshold keeps a silent terminal's wait from reading as a flash.
const appearanceDetectTimeout = 50 * time.Millisecond

type appearanceTimeoutMsg struct{}

type appearanceGate struct {
	appearance theme.Member
	// Negative on purpose: the zero value means resolved, so a zero-value
	// gate paints immediately.
	pending    bool
	pinned     bool
	colourless bool
}

func (g appearanceGate) resolved() bool {
	return !g.pending
}

// Even the armable adaptive gate is returned resolved, so a directly
// constructed model paints immediately; production opens the window via arm().
func newNominationGate(n theme.Nomination) appearanceGate {
	if n.IsConstant() || n == (theme.Nomination{}) {
		return appearanceGate{pinned: true}
	}
	return appearanceGate{}
}

func newColourlessGate() appearanceGate {
	return appearanceGate{colourless: true}
}

func (g *appearanceGate) arm() {
	if g.pinned || g.colourless {
		return
	}
	g.pending = true
}

func (g appearanceGate) timeoutCmd() tea.Cmd {
	if g.resolved() {
		return nil
	}
	return tea.Tick(appearanceDetectTimeout, func(time.Time) tea.Msg {
		return appearanceTimeoutMsg{}
	})
}

func (g *appearanceGate) resolveDark() bool {
	return g.resolve(theme.MemberDark)
}

// First call only: a late reply or timeout that lost the race never flips the
// painted canvas.
func (g *appearanceGate) resolve(appearance theme.Member) bool {
	if g.resolved() {
		return false
	}
	g.appearance = appearance
	g.pending = false
	return true
}
