package capture_test

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// This file guards the §7.4 outstanding visual gate: `text.subtle` is the ONE
// Nord value with no locus on any captured Nord frame, because it paints the
// group `··· N` counts and the loading page's pending steps and neither appears
// on a flat Sessions screen. It is also an INVENTION (interpolated on nord3's hue
// and saturation, 3.18 against Nord's own mid-dark canvas — 0.18 of headroom over
// the 3.00 band floor), so the numbers cannot answer whether it reads as
// de-emphasised but still readable. Only a human eye on a grouped Nord frame can.
//
// What this test does is make that gate impossible to take on a frame where the
// token is invisible: it renders the SAME fixture the grouped Nord tape drives,
// under the SAME theme, through the SAME production constructor, and proves every
// `··· N` count on the frame is painted in nord's text.subtle over nord's canvas.
// Without it a reviewer could sign off a locus-free frame and the gate would have
// judged nothing at all.

// groupedNordFixture is the fixture the grouped Nord capture is taken from, and
// therefore the one this assertion must render.
//
// By-Project is the right surface rather than By-Tag: its group headings are
// project names and its counts include a group of 2 alongside the pinned Unknown
// catch-all, so the frame carries several count loci. The tape
// (testdata/vhs/sessions-by-project-nord.tape) names this same fixture — the two
// must not drift, which is why the name is a constant here.
const groupedNordFixture = "sessions-by-project"

// groupCountPattern matches a rendered group count — the §5.1 `··· N` tally
// beside a group heading — in the frame's VISIBLE text.
//
// It reads the counts off the frame rather than off the fixture's session data so
// the assertion needs no second copy of the grouping rules; what has to be proven
// is that whatever counts the frame drew are painted in text.subtle. The three
// middle dots cannot collide with the footer's single `·` separators or the
// delegate's `…` truncation ellipsis.
var groupCountPattern = regexp.MustCompile(`··· \d+`)

// nordBuiltin loads the Nord built-in — the palette the grouped capture pins —
// failing on anything but a clean parse (§7.6 makes a rejection here a build-time
// impossibility, so it is Fatal rather than a fallback).
func nordBuiltin(t *testing.T) theme.Theme {
	t.Helper()
	const slug = "nord"
	loaded, rejection, found := theme.NewLoader(nil).LoadBuiltin(slug)
	if !found {
		t.Fatalf("built-in %q not found in the embedded set", slug)
	}
	if rejection != nil {
		t.Fatalf("built-in %q was rejected: %s", slug, rejection.Reason)
	}
	return loaded.Theme
}

// flattenCmd executes cmd and returns every leaf message it produces, draining
// tea.BatchMsg recursively.
//
// Model.Init batches the OSC 11 background query alongside the session and
// project loads, so its result is a batch rather than a single message; a frame
// assertion needs the leaves. It mirrors internal/tui's flattenInitMsgs, which
// lives in that package's own external test binary and so cannot be shared.
func flattenCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for _, c := range batch {
		out = append(out, flattenCmd(c)...)
	}
	return out
}

// renderFixtureFrame builds the named fixture on the given palette through the
// production tui.Build constructor — exactly what capturetool does — drives its
// real Init → Update flow, and returns the painted frame.
//
// The CONSTANT nomination is what capturetool passes (§13.3), so the gate is
// already resolved and the frame is the un-gated one the tape screenshots rather
// than the neutral held blank.
func renderFixtureFrame(t *testing.T, fixture string, palette theme.Theme, width, height int) string {
	t.Helper()

	fx, err := capture.FixtureByName(fixture)
	if err != nil {
		t.Fatalf("FixtureByName(%s): %v", fixture, err)
	}
	deps := fx.Deps()
	deps.Theme = theme.ConstantNomination(palette)

	m := tui.Build(deps)
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	for _, msg := range flattenCmd(m.Init()) {
		model, _ = model.Update(msg)
	}
	return model.(tui.Model).View().Content
}

// styledRunOpening returns the opening SGR sequence plus the text of a run
// painted fg-on-bg — everything lipgloss emits for that run except its trailing
// reset.
//
// The reset is dropped because the frame's outer canvas fill REWRITES it: a run
// closed by lipgloss with a bare `ESC[m` comes back out of fillCanvas closed with
// `ESC[0;48;2;r;g;bm`, so it re-establishes the canvas for the rest of the line.
// Everything before it — the exact foreground and background params, in
// lipgloss's own ordering, immediately followed by the run's exact text — is
// untouched, and is what identifies the run.
//
// It builds the expectation straight from the two tokens rather than through the
// delegate's own style helper. An expectation routed through the code under test
// moves with it and would stay equal under any mutation of the pairing; built
// independently, a count that stopped being text.subtle, or one painted on a
// canvas other than the theme's own, is a failure.
func styledRunOpening(t *testing.T, fg, bg theme.Token, text string) string {
	t.Helper()
	rendered := lipgloss.NewStyle().Foreground(fg.Color()).Background(bg.Color()).Render(text)
	reset := strings.LastIndex(rendered, "\x1b")
	if reset <= 0 {
		t.Fatalf("could not locate the trailing reset in the rendered run %q", rendered)
	}
	return rendered[:reset]
}

// TestGroupedRender_CarriesTextSubtleCountLocus asserts the grouped Nord frame
// carries `··· N` group counts painted in nord's text.subtle over nord's own
// canvas — the locus the §7.4 visual gate is taken on.
func TestGroupedRender_CarriesTextSubtleCountLocus(t *testing.T) {
	nord := nordBuiltin(t)

	// Precondition: text.subtle must be distinguishable from every other role on
	// the frame, or "this run is text.subtle" would be undecidable from its
	// colour. Nord's ramp is barrelled at the ends and its two middle steps are
	// both inventions, so this is worth stating rather than assuming.
	for _, tok := range nord.All() {
		if tok.Name != "text.subtle" && tok.Value == nord.TextSubtle.Value {
			t.Fatalf("precondition broken: nord's %s shares text.subtle's value %s, so the locus is not attributable", tok.Name, tok.Value)
		}
	}

	// 1280×800 at the tape's 16px JetBrains Mono is roughly this cell grid; the
	// assertion is size-insensitive, and a frame this tall keeps every group on
	// the first page.
	frame := renderFixtureFrame(t, groupedNordFixture, nord, 120, 40)

	counts := groupCountPattern.FindAllString(ansi.Strip(frame), -1)
	if len(counts) == 0 {
		t.Fatalf("the %s frame carries no `··· N` group count at all — there is no text.subtle locus to gate on:\n%s", groupedNordFixture, ansi.Strip(frame))
	}

	for _, count := range counts {
		want := styledRunOpening(t, nord.TextSubtle, nord.Canvas, count)
		if !strings.Contains(frame, want) {
			t.Errorf("the %q count on the %s frame is not painted in nord's text.subtle (%s) over its canvas (%s)\nwant the run %q\n--- frame ---\n%s",
				count, groupedNordFixture, nord.TextSubtle.Value, nord.Canvas.Value, want, frame)
		}
	}
}
