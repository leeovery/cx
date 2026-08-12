package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
)

type recordingCreator struct {
	dir     string
	command []string
}

func (c *recordingCreator) CreateFromDir(dir string, command []string) (string, error) {
	c.dir = dir
	c.command = command
	return "myapp-abc123", nil
}

func newCommandPendingTestModel(t *testing.T, w, h int, projects []project.Project, command []string) Model {
	t.Helper()
	m := New(fakeLister{}).WithCommand(command)
	m.termWidth = w
	m.termHeight = h
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	return m
}

func TestCommandBand_VioletBarCaretTextAndOrangeChip(t *testing.T) {
	const w = 80
	band := renderCommandBand([]string{"npm", "run", "dev"}, w, testDarkTheme(t), false)

	stripped := ansi.Strip(band)
	if !strings.HasPrefix(stripped, noticeBarGlyph) {
		t.Errorf("command band does not start with the %q left-bar: %q", noticeBarGlyph, stripped)
	}
	if !strings.Contains(stripped, commandBandCaret) {
		t.Errorf("command band missing the %q caret: %q", commandBandCaret, stripped)
	}
	if !strings.Contains(stripped, commandBandText) {
		t.Errorf("command band missing the fixed text %q: %q", commandBandText, stripped)
	}
	if !strings.Contains(stripped, "npm run dev") {
		t.Errorf("command band missing the joined command %q: %q", "npm run dev", stripped)
	}

	violetSeq := tokenFgSeq(t, testDarkTheme(t).AccentPrimary)
	if !strings.Contains(band, violetSeq) {
		t.Errorf("command band missing the accent.primary bar foreground sequence %q:\n%s", violetSeq, band)
	}
	orangeSeq := tokenFgSeq(t, testDarkTheme(t).AccentAttention)
	if !strings.Contains(band, orangeSeq) {
		t.Errorf("command band missing the accent.attention chip foreground sequence %q:\n%s", orangeSeq, band)
	}
}

func TestCommandBand_JoinsCommandSlice(t *testing.T) {
	const w = 90
	band := renderCommandBand([]string{"go", "test", "./..."}, w, testDarkTheme(t), false)
	stripped := ansi.Strip(band)
	if !strings.Contains(stripped, "go test ./...") {
		t.Errorf("command band must join the slice on spaces: %q", stripped)
	}
}

func TestCommandBand_FixedTextConstant(t *testing.T) {
	const want = "Pick a project to run"
	if commandBandText != want {
		t.Errorf("commandBandText = %q, want the spec-exact wording %q", commandBandText, want)
	}
}

func TestCommandBand_Tinted(t *testing.T) {
	const w = 80
	band := renderCommandBand([]string{"npm", "run", "dev"}, w, testDarkTheme(t), false)
	tintSeq := tokenBgSeq(t, testDarkTheme(t).BgSelection)
	if !strings.Contains(band, tintSeq) {
		t.Errorf("command band missing the bg.selection tint %q:\n%s", tintSeq, band)
	}
	if got := lipgloss.Width(band); got != w {
		t.Errorf("command band width = %d, want %d (full width)", got, w)
	}
}

func TestCommandBand_NoColorKeepsBarCaretAndChip(t *testing.T) {
	const w = 80
	band := renderCommandBand([]string{"npm", "run", "dev"}, w, testDarkTheme(t), true)

	stripped := ansi.Strip(band)
	if !strings.HasPrefix(stripped, noticeBarGlyph) {
		t.Errorf("NO_COLOR command band must keep the far-left %q bar: %q", noticeBarGlyph, stripped)
	}
	if !strings.Contains(stripped, commandBandCaret) {
		t.Errorf("NO_COLOR command band must keep the %q caret: %q", commandBandCaret, stripped)
	}
	if !strings.Contains(stripped, commandBandText) {
		t.Errorf("NO_COLOR command band must keep the text %q: %q", commandBandText, stripped)
	}
	if !strings.Contains(stripped, "npm run dev") {
		t.Errorf("NO_COLOR command band must keep the chip command %q: %q", "npm run dev", stripped)
	}
	if band != stripped {
		t.Errorf("NO_COLOR command band must carry no SGR colour sequences; got raw %q", band)
	}
}

func TestCommandBandRole_BarAndTintTokens(t *testing.T) {
	if got := bandCommand.barToken(testDarkTheme(t)).Name; got != testDarkTheme(t).AccentPrimary.Name {
		t.Errorf("bandCommand bar token = %q, want accent.primary", got)
	}
	if got := bandCommand.tintToken(testDarkTheme(t)).Name; got != testDarkTheme(t).BgSelection.Name {
		t.Errorf("bandCommand tint token = %q, want bg.selection", got)
	}
}

func TestViewProjectList_CommandPendingBandOverFullChrome(t *testing.T) {
	m := newCommandPendingTestModel(t, 90, 30, sampleProjects(), []string{"npm", "run", "dev"})
	view := m.viewProjectList()
	visible := ansi.Strip(view)

	if !strings.Contains(visible, commandBandText) {
		t.Errorf("command-pending view missing the banner text %q:\n%s", commandBandText, visible)
	}
	if !strings.Contains(visible, "npm run dev") {
		t.Errorf("command-pending view missing the joined command chip %q:\n%s", "npm run dev", visible)
	}
	if !strings.Contains(visible, "Projects") {
		t.Errorf("command-pending view missing the Projects section header (chrome stripped?):\n%s", visible)
	}
	if seq := tokenFgSeq(t, testDarkTheme(t).StatePositive); !strings.Contains(view, seq) {
		t.Errorf("command-pending view missing the state.positive section label role sequence %q", seq)
	}
	if !strings.Contains(visible, sectionFilterHint) {
		t.Errorf("command-pending view missing the %q hint (chrome stripped?):\n%s", sectionFilterHint, visible)
	}
	if !strings.Contains(visible, "P O R T A L") {
		t.Errorf("command-pending view missing the PORTAL wordmark (chrome stripped?):\n%s", visible)
	}
	if strings.Contains(visible, "Select project to run") {
		t.Errorf("command-pending view leaked the legacy plain status line:\n%s", visible)
	}
}

func TestViewProjectList_CommandBandUnderSeparatorAboveSectionHeader(t *testing.T) {
	m := newCommandPendingTestModel(t, 90, 30, sampleProjects(), []string{"npm", "run", "dev"})
	lines := strings.Split(ansi.Strip(m.viewProjectList()), "\n")

	ruleIdx := -1
	for i, l := range lines {
		if strings.Contains(l, strings.Repeat(headerRuleGlyph, 4)) {
			ruleIdx = i
			break
		}
	}
	bandIdx := lineIndexContaining(lines, commandBandText)
	sectionIdx := lineIndexContaining(lines, "Projects")
	if ruleIdx < 0 || bandIdx < 0 || sectionIdx < 0 {
		t.Fatalf("missing a landmark: rule=%d band=%d section=%d\n%s", ruleIdx, bandIdx, sectionIdx, strings.Join(lines, "\n"))
	}
	if bandIdx <= ruleIdx {
		t.Errorf("band index %d must be > separator-rule index %d (band under the separator)", bandIdx, ruleIdx)
	}
	if sectionIdx <= bandIdx {
		t.Errorf("section header index %d must be > band index %d (band above the section header)", sectionIdx, bandIdx)
	}
	if sectionIdx-bandIdx != 2 {
		t.Errorf("section header is %d rows below the band, want 2 (band → blank → section header)", sectionIdx-bandIdx)
	}
	blank := lines[bandIdx+1]
	if strings.TrimSpace(blank) != "" {
		t.Errorf("row between the band and section header must be blank, got %q", blank)
	}
}

func TestProjectBandHeight_TracksRenderedSlot(t *testing.T) {
	withBand := newCommandPendingTestModel(t, 90, 30, sampleProjects(), []string{"npm", "run", "dev"})
	slotHeight := lipgloss.Height(withBand.renderProjectBandSlot())
	if slotHeight < 2 {
		t.Fatalf("command band slot height = %d, want >=2 (band + blank)", slotHeight)
	}
	if got := withBand.projectBandHeight(); got != slotHeight {
		t.Errorf("projectBandHeight = %d, want %d (measured off the rendered slot)", got, slotHeight)
	}

	noBand := newProjectsPageTestModel(t, 90, 30, testDarkTheme(t), sampleProjects())
	if got := noBand.projectBandHeight(); got != 0 {
		t.Errorf("projectBandHeight (not command-pending) = %d, want 0", got)
	}
}

func TestCommandPendingFooter_SwappedCopy(t *testing.T) {
	m := newCommandPendingTestModel(t, 160, 30, sampleProjects(), []string{"npm", "run", "dev"})
	visible := ansi.Strip(m.viewProjectList())

	for _, want := range []string{"run here", "run in cwd", "cancel", "help"} {
		if !strings.Contains(visible, want) {
			t.Errorf("command-pending footer missing the entry %q:\n%s", want, visible)
		}
	}
	if strings.Contains(visible, "quit") {
		t.Errorf("command-pending footer must NOT contain 'quit' (deferred to ? help):\n%s", visible)
	}
	for _, banned := range []string{"new session", "new in cwd"} {
		if strings.Contains(visible, banned) {
			t.Errorf("command-pending footer leaked non-command-pending copy %q:\n%s", banned, visible)
		}
	}
}

func TestCommandPending_DispatchParity(t *testing.T) {
	command := []string{"npm", "run", "dev"}
	projects := []project.Project{{Path: "/code/myapp", Name: "myapp"}}

	build := func() (Model, *recordingCreator) {
		creator := &recordingCreator{}
		m := New(fakeLister{}, WithProjectStore(stubProjectStore{}), WithSessionCreator(creator)).WithCommand(command)
		m.cwd = "/code/cwd"
		m.setProjects(projects)
		m.projectList.SetItems(ProjectsToListItems(projects))
		return m, creator
	}

	t.Run("Enter dispatches run-here from the selected project", func(t *testing.T) {
		m, creator := build()
		_, cmd := m.updateProjectsPage(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("Enter must dispatch a create command in command-pending mode")
		}
		cmd()
		if creator.dir != "/code/myapp" {
			t.Errorf("Enter ran here for dir %q, want the selected project /code/myapp", creator.dir)
		}
		if strings.Join(creator.command, " ") != strings.Join(command, " ") {
			t.Errorf("Enter forwarded command %v, want %v", creator.command, command)
		}
	})

	t.Run("n dispatches run-in-cwd", func(t *testing.T) {
		m, creator := build()
		_, cmd := m.updateProjectsPage(tea.KeyPressMsg{Code: 'n', Text: "n"})
		if cmd == nil {
			t.Fatal("n must dispatch a create-in-cwd command in command-pending mode")
		}
		cmd()
		if creator.dir != "/code/cwd" {
			t.Errorf("n ran in dir %q, want the cwd /code/cwd", creator.dir)
		}
		if strings.Join(creator.command, " ") != strings.Join(command, " ") {
			t.Errorf("n forwarded command %v, want %v", creator.command, command)
		}
	})

	t.Run("Esc dispatches cancel (quit)", func(t *testing.T) {
		m, _ := build()
		_, cmd := m.updateProjectsPage(tea.KeyPressMsg{Code: tea.KeyEscape})
		if cmd == nil {
			t.Fatal("Esc must dispatch quit (cancel) in command-pending mode")
		}
		if msg := cmd(); msg == nil {
			t.Fatal("Esc cancel must produce a quit message")
		}
	})
}

func TestCommandPendingKeymap_Copy(t *testing.T) {
	entries := commandPendingKeymap()
	want := []struct{ key, helpKey, action string }{
		{"enter", "⏎", "run here"},
		{"n", "", "run in cwd"},
		{"esc", "", "cancel"},
	}
	if len(entries) != len(want) {
		t.Fatalf("commandPendingKeymap returned %d entries, want %d", len(entries), len(want))
	}
	for i, w := range want {
		e := entries[i]
		if e.Key != w.key || e.HelpKey != w.helpKey || e.Action != w.action {
			t.Errorf("entry %d = (Key=%q, HelpKey=%q, Action=%q), want (Key=%q, HelpKey=%q, Action=%q)",
				i, e.Key, e.HelpKey, e.Action, w.key, w.helpKey, w.action)
		}
	}
}

func TestCommandPendingFooter_ByteExact(t *testing.T) {
	const wantDark = "\x1b[38;2;41;46;66;48;2;11;12;20m▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔\x1b[m\n\x1b[38;2;122;162;247;48;2;11;12;20m⏎\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mrun here\x1b[m\x1b[38;2;115;122;162;48;2;11;12;20m · \x1b[m\x1b[38;2;122;162;247;48;2;11;12;20mn\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mrun in cwd\x1b[m\x1b[38;2;115;122;162;48;2;11;12;20m · \x1b[m\x1b[38;2;122;162;247;48;2;11;12;20mesc\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mcancel\x1b[m\x1b[48;2;11;12;20m                                                                                                                    \x1b[m\x1b[38;2;187;154;247;48;2;11;12;20m?\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mhelp\x1b[m"
	const wantLight = "\x1b[38;2;201;205;219;48;2;225;226;231m▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔\x1b[m\n\x1b[38;2;45;92;202;48;2;225;226;231m⏎\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mrun here\x1b[m\x1b[38;2;88;96;147;48;2;225;226;231m · \x1b[m\x1b[38;2;45;92;202;48;2;225;226;231mn\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mrun in cwd\x1b[m\x1b[38;2;88;96;147;48;2;225;226;231m · \x1b[m\x1b[38;2;45;92;202;48;2;225;226;231mesc\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mcancel\x1b[m\x1b[48;2;225;226;231m                                                                                                                    \x1b[m\x1b[38;2;138;63;209;48;2;225;226;231m?\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mhelp\x1b[m"
	const wantNoColour = "▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔▔\n⏎ run here · n run in cwd · esc cancel                                                                                                                    ? help"

	tests := []struct {
		name       string
		th         theme.Theme
		colourless bool
		want       string
	}{
		{"dark", testDarkTheme(t), false, wantDark},
		{"light", testLightTheme(t), false, wantLight},
		{"no-colour", testDarkTheme(t), true, wantNoColour},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderCommandPendingFooter(160, tc.th, tc.colourless)
			if got != tc.want {
				t.Errorf("command-pending footer byte mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}
