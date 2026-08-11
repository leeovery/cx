package tui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

// darkBg / lightBg are deterministic OSC 11 BackgroundColorMsg payloads. The
// dark value is near-black (luminance < 0.5 → IsDark true); the light value is
// near-white (luminance ≥ 0.5 → IsDark false).
var (
	darkBg  = tea.BackgroundColorMsg{Color: color.RGBA{R: 0x0b, G: 0x0c, B: 0x14, A: 0xff}}
	lightBg = tea.BackgroundColorMsg{Color: color.RGBA{R: 0xe1, G: 0xe2, B: 0xe7, A: 0xff}}
)

// detectModel builds a Sessions model for the given loaded nomination through
// the PRODUCTION chokepoint (Build, which opens the detect-or-timeout window),
// sized for rendering with the deterministic flat session set ingested through
// the production path.
//
// The nomination's SHAPE decides what the gate does: an adaptive pair holds the
// first paint on detection-or-timeout, a constant stays resolved (arm is a no-op).
func detectModel(t *testing.T, n theme.Nomination) Model {
	t.Helper()
	sessions := []tmux.Session{
		{Name: "alpha", Windows: 3, Attached: true},
		{Name: "bravo", Windows: 1, Attached: false},
		{Name: "charlie", Windows: 2, Attached: false},
	}
	m := Build(Deps{Lister: fakeLister{}, Theme: n})
	m.termWidth = 90
	m.termHeight = 24
	m.applySessions(sessions)
	return m
}

// blankView is the neutral pre-resolution frame: a full-terminal blank canvas
// with no real content (no "Sessions" title row painted). The detect-or-timeout
// gate holds the first real paint until the mode resolves so Portal never paints
// one canvas then flips to the other.
func assertBlankFrame(t *testing.T, m Model) {
	t.Helper()
	if m.modeResolved() {
		t.Fatalf("modeResolved = true, want false (model must be unresolved for a blank frame)")
	}
	if got := m.View().Content; strings.Contains(got, "Sessions") {
		t.Errorf("pre-resolution View painted real content (found 'Sessions' title); want a neutral blank frame")
	}
}

// assertPaintedCanvas asserts an ADAPTIVE model resolved to the given answer and
// painted the member that answer names. It is adaptive-only by construction: a
// constant derives no answer, so its answer in force stays at the dark zero value
// whatever palette it paints (assertActiveTheme is that path's assertion).
func assertPaintedCanvas(t *testing.T, m Model, appearance theme.Member) {
	t.Helper()
	if !m.modeResolved() {
		t.Fatalf("modeResolved = false, want true (the canvas must be resolved before the real paint)")
	}
	if m.themeState.inForceMode() != appearance {
		t.Errorf("inForceMode() = %v, want %v", m.themeState.inForceMode(), appearance)
	}
	view := m.View().Content
	if !strings.Contains(view, "Sessions") {
		t.Errorf("resolved View did not paint the real content (no 'Sessions' title)")
	}
	if seq := canvasSeq(t, themeForAppearance(t, appearance)); !strings.Contains(view, seq) {
		t.Errorf("resolved View does not contain the %v canvas background sequence %q", appearance, seq)
	}
}

// themeForAppearance returns the built-in the gate's answer selects out of the
// shipped pair — the test-side mirror of theme.Nomination.Select.
func themeForAppearance(t *testing.T, appearance theme.Member) theme.Theme {
	t.Helper()
	if appearance == theme.MemberLight {
		return testLightTheme(t)
	}
	return testDarkTheme(t)
}

// TestUnresolvedGateCarriesDarkFallback pins the load-bearing zero value: a
// gate nobody resolved — a bare struct, an armed and still-open adaptive gate,
// and the model built over one — already carries the dark member Portal falls
// back to when no answer arrives, so a timeout resolves to the answer that was
// standing all along.
func TestUnresolvedGateCarriesDarkFallback(t *testing.T) {
	var bare appearanceGate
	if bare.appearance != theme.MemberDark {
		t.Errorf("zero appearanceGate answer = %v, want %v", bare.appearance, theme.MemberDark)
	}

	g := newNominationGate(testBuiltinPair(t))
	g.arm()

	if g.resolved() {
		t.Fatalf("an armed adaptive gate reports resolved, want an open window")
	}
	if g.appearance != theme.MemberDark {
		t.Errorf("unresolved gate answer = %v, want %v (the no-answer fallback)", g.appearance, theme.MemberDark)
	}

	m := detectModel(t, testBuiltinPair(t))
	if m.themeState.inForceMode() != theme.MemberDark {
		t.Errorf("unresolved model inForceMode() = %v, want %v", m.themeState.inForceMode(), theme.MemberDark)
	}
}

// TestAdaptiveDetectsDark: an adaptive pair + a dark BackgroundColorMsg resolves
// the answer in force to Dark, marks resolved, and paints the dark canvas.
// Before the message the frame is the neutral blank (no pre-resolution real
// paint).
func TestAdaptiveDetectsDark(t *testing.T) {
	m := detectModel(t, testBuiltinPair(t))
	assertBlankFrame(t, m)

	updated, _ := m.Update(darkBg)
	assertPaintedCanvas(t, updated.(Model), theme.MemberDark)
}

// TestAdaptiveDetectsLight: an adaptive pair + a light BackgroundColorMsg
// resolves the answer in force to Light and paints the light canvas.
func TestAdaptiveDetectsLight(t *testing.T) {
	m := detectModel(t, testBuiltinPair(t))
	assertBlankFrame(t, m)

	updated, _ := m.Update(lightBg)
	assertPaintedCanvas(t, updated.(Model), theme.MemberLight)
}

// TestNoPaintThenFlip: before resolution the View is the neutral blank frame
// (not a painted canvas); after resolution it is the correct canvas; and a later
// message never re-resolves the mode (no second resolution, no flip).
func TestNoPaintThenFlip(t *testing.T) {
	m := detectModel(t, testBuiltinPair(t))
	assertBlankFrame(t, m)

	// OSC 11 answers dark first → resolves dark, paints.
	updated, _ := m.Update(darkBg)
	resolved := updated.(Model)
	assertPaintedCanvas(t, resolved, theme.MemberDark)

	// A late timeout (the loser of the race) must be ignored — the mode is
	// already resolved, so it must not flip to anything.
	after, _ := resolved.Update(appearanceTimeoutMsg{})
	if after.(Model).themeState.inForceMode() != theme.MemberDark {
		t.Errorf("a late timeout flipped the answer in force to %v, want it pinned at the dark canvas (no second resolution)", after.(Model).themeState.inForceMode())
	}

	// And a late, conflicting BackgroundColorMsg (light) must not flip either.
	after2, _ := after.(Model).Update(lightBg)
	if after2.(Model).themeState.inForceMode() != theme.MemberDark {
		t.Errorf("a late light BackgroundColorMsg flipped the answer in force to %v, want it pinned at the dark canvas (no flip)", after2.(Model).themeState.inForceMode())
	}
}

// TestTimeoutFallsBackToDark: the timeout fires before any BackgroundColorMsg, so
// the mode resolves to the dark fallback and paints.
func TestTimeoutFallsBackToDark(t *testing.T) {
	m := detectModel(t, testBuiltinPair(t))
	assertBlankFrame(t, m)

	updated, _ := m.Update(appearanceTimeoutMsg{})
	assertPaintedCanvas(t, updated.(Model), theme.MemberDark)
}

// TestColorFGBGNeverOverridesOSC11: even with COLORFGBG advertising a light
// terminal, an OSC 11 answer of dark wins — COLORFGBG is a weak hint only and
// must never override the OSC 11 reply.
func TestColorFGBGNeverOverridesOSC11(t *testing.T) {
	t.Setenv("COLORFGBG", "0;15") // fg black on bg white → "light" by the weak hint
	m := detectModel(t, testBuiltinPair(t))

	updated, _ := m.Update(darkBg)
	assertPaintedCanvas(t, updated.(Model), theme.MemberDark)
}

// TestMisdetectionLegibleNotBroken: a mis-detected terminal resolves to the
// wrong-but-painted canvas (here light reported for a model that "should" be
// dark) — the canvas still paints (not blank, not crashed); the contrast floor
// holds against whichever canvas is painted (§2.3).
func TestMisdetectionLegibleNotBroken(t *testing.T) {
	m := detectModel(t, testBuiltinPair(t))

	// The terminal mis-reports light. The model paints the light canvas — wrong
	// mode but fully legible, not blank, not crashed.
	updated, _ := m.Update(lightBg)
	resolved := updated.(Model)
	assertPaintedCanvas(t, resolved, theme.MemberLight)
	view := resolved.View().Content
	if strings.TrimSpace(view) == "" {
		t.Errorf("mis-detected canvas rendered blank, want a legible (wrong-mode) screen")
	}
}

// assertNoTimeoutTick drains Init's batched cmds and asserts none of them
// produces an appearanceTimeoutMsg — a CONSTANT nomination must not arm the
// detection timeout (it skips the gate and the wait entirely).
func assertNoTimeoutTick(t *testing.T, m Model) {
	t.Helper()
	for _, msg := range initCmds(t, m.Init()) {
		if _, ok := msg.(appearanceTimeoutMsg); ok {
			t.Errorf("Init armed a detection timeout on a constant nomination, want none (the wait is skipped)")
		}
	}
}

// TestAdaptiveArmsTimeoutTick: under an adaptive pair Init arms the
// detect-or-timeout tick so a non-responding terminal still resolves to the dark
// fallback.
func TestAdaptiveArmsTimeoutTick(t *testing.T) {
	m := detectModel(t, testBuiltinPair(t))
	found := false
	for _, msg := range initCmds(t, m.Init()) {
		if _, ok := msg.(appearanceTimeoutMsg); ok {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Init did not arm the detect-or-timeout tick under an adaptive pair (the no-answer fallback would never fire)")
	}
}
