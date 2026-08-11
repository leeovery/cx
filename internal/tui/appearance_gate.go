package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// appearanceDetectTimeout is the upper bound on how long the first real paint
// waits for the OSC 11 BackgroundColorMsg reply before falling through to the
// dark fallback — the detect-or-timeout first-paint gate.
//
// Chosen value: 50ms. Terminals
// that answer OSC 11 do so in single-digit ms, so 50ms gives a comfortable
// margin for the answer to win the race on a real terminal — the correct canvas
// lands on frame one. It also stays invisible against the multi-hundred-ms cold
// bootstrap (the loading path gates the same way) and well under the ~100ms
// "instant" perception threshold, so a non-responding terminal's brief blank
// wait is never perceived as a flash. Smaller (e.g. 10ms) risks racing a slow
// terminal's answer and flipping to the dark fallback prematurely; larger (e.g.
// 200ms) starts to approach the perceptible-pause threshold.
const appearanceDetectTimeout = 50 * time.Millisecond

// appearanceTimeoutMsg is the detect-or-timeout deadline message. It is emitted
// by the tea.Tick armed in Init (adaptive pair only) after
// appearanceDetectTimeout. When it wins the race against the OSC 11
// BackgroundColorMsg, the gate resolves to the dark fallback. It is
// ignored once the gate is already resolved (the loser of the race never
// re-resolves — no flip).
type appearanceTimeoutMsg struct{}

// appearanceGate is the reusable detect-or-timeout first-paint mechanism. It owns
// the resolved canvas appearance and the "may the real canvas
// paint yet?" flag, so a page that gates its first paint (the foundation Sessions
// screen and the cold-path loading page) shares one resolution
// path rather than re-implementing the race.
//
// The contract is single-resolution: once armed, whichever of the OSC 11 reply
// or the timeout fires FIRST resolves the mode and flips resolved to true; every
// later signal is ignored, so the canvas is painted exactly once and never flips.
//
// Its SHAPE is decided by the theme setting's shape. Only an ADAPTIVE
// nomination has a light/dark question to answer, so only an adaptive gate is
// armable; a CONSTANT nomination constructs the gate already resolved and
// unarmable — detection and the timeout wait are skipped entirely, which is the
// real startup win that skipping them buys. The NO_COLOR carve-out does the
// same via colourless: under NO_COLOR there is no canvas to select, so the gate
// is resolved and unarmable — like a constant, but for a different reason (no hue
// at all, rather than a chosen palette).
type appearanceGate struct {
	// appearance is the resolved light/dark canvas the owned canvas is painted
	// for. An unresolved adaptive gate already carries the no-answer fallback,
	// because that is theme.Member's zero value; it is simply not painted until
	// the gate resolves.
	//
	// On a gate that was never armed — a constant nomination, a nomination-less
	// model — the value is that standing fallback and NOTHING ELSE: no question was
	// asked, so it is not detection-derived. What the terminal reported is a
	// separate fact, held as themeState.reply; themeState.inForceMode is the answer
	// in force, whichever of the two established it.
	appearance theme.Member
	// pending reports whether the detect-or-timeout window is OPEN (the first real
	// paint must wait). It is named negatively on purpose: the zero value (false)
	// means "not pending" = resolved, so a zero-value gate (a struct-literal test
	// model) paints immediately. arm() opens the window (pending=true) on an
	// adaptive gate; the OSC 11 reply or the timeout closes it. resolved() is the
	// positive read used everywhere else.
	pending bool
	// pinned marks a gate with NOTHING TO DETECT: a constant nomination (for which
	// detection is never consulted), a model constructed with no nomination at all
	// (nothing to select between), or the test/capture WithCanvasMode override. A
	// pinned gate is never armed (arm is a no-op), so its answer survives and no
	// timeout tick is issued.
	pinned bool
	// colourless marks the NO_COLOR carve-out. A colourless gate is never
	// armed (arm is a no-op) and is constructed already resolved, so detection and
	// the first-paint wait are skipped entirely — there is no canvas to select. It
	// is independent of appearance: under NO_COLOR no canvas is painted at all, so
	// the resolved answer is irrelevant (the render path reads m.colourless to
	// suppress the paint).
	colourless bool
}

// resolved reports whether the first real paint may proceed — the positive read
// of the (negatively-stored) pending flag.
func (g appearanceGate) resolved() bool {
	return !g.pending
}

// newNominationGate builds the gate for the SHAPE of a loaded theme setting.
//
// An ADAPTIVE pair is the only shape with a question to answer, so it is the only
// armable gate. It is constructed RESOLVED to the dark fallback (pending=false)
// so a directly constructed model paints immediately; production opens the
// detect-or-timeout window explicitly via arm() (see Build), which is what holds
// the live picker's first paint until the answer lands.
//
// Every other shape is pinned — resolved and unarmable, painting from frame one
// with no detection and no wait:
//
//   - A CONSTANT nomination, because detection is never consulted for one.
//   - A ZERO nomination, i.e. a model nobody handed a theme. There is nothing to
//     select between, so waiting for an answer would gate the first paint on a
//     question whose outcome cannot change what is painted (New's dark built-in
//     seed).
//
// Its answer is the dark zero value in both pinned cases, and is NOT
// detection-derived — see appearanceGate.appearance.
func newNominationGate(n theme.Nomination) appearanceGate {
	if n.IsConstant() || n == (theme.Nomination{}) {
		return appearanceGate{pinned: true}
	}
	return appearanceGate{}
}

// newColourlessGate builds the gate for the NO_COLOR carve-out. It
// is constructed already resolved (pending=false) and unarmable (colourless=true,
// so arm() is a no-op), so detection and the first-paint wait are skipped — there
// is no canvas to select. The appearance is the dark zero value, which is the
// standing no-answer fallback selecting the active member — which is why the
// theme machinery below the render layer still runs unchanged under NO_COLOR
// (both nominated themes loaded, one selected) even though nothing is painted.
func newColourlessGate() appearanceGate {
	return appearanceGate{colourless: true}
}

// arm opens the detect-or-timeout window on a non-pinned (adaptive) gate: it
// marks the gate pending so View holds the neutral blank frame and Init issues
// the timeout tick, until the OSC 11 reply or the timeout resolves it. It is a
// no-op on a pinned gate (which keeps painting from frame one). Production
// (Build) arms the gate for every construction; only an adaptive nomination
// actually opens a window. The foundation Sessions screen and the loading
// page share this one mechanism.
func (g *appearanceGate) arm() {
	if g.pinned || g.colourless {
		return
	}
	g.pending = true
}

// timeoutCmd is the tea.Cmd that arms the detect-or-timeout deadline tick. It
// returns nil for an already-resolved gate (a constant, or an adaptive gate whose
// window is not open) so no spurious wait is issued; for an open (pending) gate
// it schedules the appearanceTimeoutMsg after appearanceDetectTimeout.
func (g appearanceGate) timeoutCmd() tea.Cmd {
	if g.resolved() {
		return nil
	}
	return tea.Tick(appearanceDetectTimeout, func(time.Time) tea.Msg {
		return appearanceTimeoutMsg{}
	})
}

// resolveDark resolves an open gate to the dark fallback (the no-answer /
// timeout outcome) and reports whether this call performed the resolution.
// It is a no-op (returning false) once already resolved, so a late timeout that
// lost the race never re-resolves — no second resolution, no flip.
func (g *appearanceGate) resolveDark() bool {
	return g.resolve(theme.MemberDark)
}

// resolve is the single-resolution core: it sets the appearance and closes the
// window on the FIRST call only (i.e. while the window is open / pending),
// returning true when it performed the resolution and false otherwise. This is the
// no-flip invariant — the canvas is fixed exactly once per open window, so a late
// OSC 11 reply or a timeout that lost the race never flips the painted canvas.
func (g *appearanceGate) resolve(appearance theme.Member) bool {
	if g.resolved() {
		return false
	}
	g.appearance = appearance
	g.pending = false
	return true
}
