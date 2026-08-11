package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// The model holds two DIFFERENT light/dark facts, and these cases pin the
// difference: what the TERMINAL said (themeState.reply, an optional value that
// only an OSC 11 arrival fills) and the answer IN FORCE (themeState.inForceMode,
// the one value that selects the active member and names the slot painting the
// screen). A launch that never asked the question has the first and not the
// second, which is the whole reason the mid-session conversion can answer at all.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// TestTerminalReply_NoAnswerShapedReplyIsAnArrival pins the shape of the reply a
// terminal that answers nothing sends: a nil colour. It is still an ARRIVAL —
// classified dark, carrying no hex — which is why the arrival is tracked
// separately from a non-empty retained background.
func TestTerminalReply_NoAnswerShapedReplyIsAnArrival(t *testing.T) {
	m := New(fakeLister{})

	updated, _ := m.Update(tea.BackgroundColorMsg{Color: nil})
	after := updated.(Model)

	if !after.themeState.reply.arrived {
		t.Error("reply.arrived = false after a no-answer-shaped reply, want true — a nil colour is still an answer")
	}
	if got := after.themeState.reply.member; got != theme.MemberDark {
		t.Errorf("reply.member = %v, want %v", got, theme.MemberDark)
	}
	if got := after.OriginalBackground(); got != "" {
		t.Errorf("OriginalBackground() = %q, want empty — a nil colour carries no hex", got)
	}
}

// TestTerminalReply_UnansweredIsNotAnArrival pins the other half: with no reply
// at all the optional is empty, and the answer it stands in for is the standing
// dark fallback.
func TestTerminalReply_UnansweredIsNotAnArrival(t *testing.T) {
	m := New(fakeLister{})

	if m.themeState.reply.arrived {
		t.Error("reply.arrived = true before any reply, want false")
	}
	if got := m.themeState.reply.answer(); got != theme.MemberDark {
		t.Errorf("reply.answer() = %v with no reply, want %v", got, theme.MemberDark)
	}
}

// TestInForceMode_ConversionOnAPinnedGateTakesTheRetainedReply drives the case
// the two facts exist for: a constant launch, whose gate was never armed, in a
// LIGHT terminal. The conversion must land on the light slot from the reply
// already in hand, and the gate must keep its own standing fallback — it resolved
// once and a conversion is not a second resolution.
func TestInForceMode_ConversionOnAPinnedGateTakesTheRetainedReply(t *testing.T) {
	dir := newConversionThemesDir(t)
	m, _, _ := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
	m = deliverBackgroundReply(t, m, lightBg)
	if !m.themeState.gate.pinned {
		t.Fatal("fixture: the constant launch armed its gate, so the never-armed case is not being driven")
	}
	m = openConversionPanel(t, m)

	m, _ = convertToSlot(t, m, "nord", slotDarkPress)

	if got := m.themeState.inForceMode(); got != theme.MemberLight {
		t.Errorf("inForceMode() = %v after converting in a light terminal, want %v", got, theme.MemberLight)
	}
	if got := m.themeState.gate.appearance; got != theme.MemberDark {
		t.Errorf("the conversion moved the gate's own answer to %v, want the untouched %v", got, theme.MemberDark)
	}
}

// TestInForceMode_LateReplyIsRecordedButNeverReThemes pins the resolve-once rule
// at its sharpest point: the timeout resolved the gate first. The reply is still
// recorded as what the terminal said — restore-on-exit and a later conversion both
// need it — but the answer in force and the painted palette do not move.
func TestInForceMode_LateReplyIsRecordedButNeverReThemes(t *testing.T) {
	m := detectModel(t, testBuiltinPair(t))
	timedOut, _ := m.Update(appearanceTimeoutMsg{})
	resolved := timedOut.(Model)
	active := resolved.themeState.active

	late, _ := resolved.Update(lightBg)
	after := late.(Model)

	if got := after.themeState.reply.answer(); got != theme.MemberLight {
		t.Errorf("reply.answer() = %v after a late light reply, want %v — the reply is still recorded", got, theme.MemberLight)
	}
	if got := after.themeState.inForceMode(); got != theme.MemberDark {
		t.Errorf("inForceMode() = %v after a late reply, want the already-resolved %v", got, theme.MemberDark)
	}
	if after.themeState.active != active {
		t.Errorf("a late reply re-themed to %s, want the untouched %s", themeLabel(after.themeState.active), themeLabel(active))
	}
}
