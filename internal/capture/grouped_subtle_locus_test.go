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
	"github.com/leeovery/portal/internal/themetest"
	"github.com/leeovery/portal/internal/tui"
)

const groupedNordFixture = "sessions-by-project"

var groupCountPattern = regexp.MustCompile(`··· \d+`)

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

func renderFixtureFrame(t *testing.T, fixture string, palette theme.Theme, width, height int) string {
	t.Helper()

	fx, err := capture.FixtureByName(fixture)
	if err != nil {
		t.Fatalf("FixtureByName(%s): %v", fixture, err)
	}
	deps := fx.Deps(palette)

	m := tui.Build(deps)
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	for _, msg := range flattenCmd(m.Init()) {
		model, _ = model.Update(msg)
	}
	return model.(tui.Model).View().Content
}

func styledRunOpening(t *testing.T, fg, bg theme.Token, text string) string {
	t.Helper()
	rendered := lipgloss.NewStyle().Foreground(fg.Color()).Background(bg.Color()).Render(text)
	reset := strings.LastIndex(rendered, "\x1b")
	if reset <= 0 {
		t.Fatalf("could not locate the trailing reset in the rendered run %q", rendered)
	}
	return rendered[:reset]
}

func TestGroupedRender_CarriesTextSubtleCountLocus(t *testing.T) {
	nord := themetest.Builtin(t, "nord")

	for _, tok := range nord.All() {
		if tok.Name != "text.subtle" && tok.Value == nord.TextSubtle.Value {
			t.Fatalf("precondition broken: nord's %s shares text.subtle's value %s, so the locus is not attributable", tok.Name, tok.Value)
		}
	}

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
