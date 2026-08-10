package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

// colourlessTestModel builds a production-shaped Sessions model in the colourless
// (NO_COLOR) carve-out path, sized for rendering and loaded with the
// deterministic flat session set through the production applySessions path. It
// drives the same Build chokepoint cmd/open.go uses, with NoColor set — so the
// model resolves the colourless render path exactly as production does.
func colourlessTestModel(t *testing.T, w, h int) Model {
	t.Helper()
	sessions := []tmux.Session{
		{Name: "alpha", Windows: 3, Attached: true},
		{Name: "bravo", Windows: 1, Attached: false},
		{Name: "charlie", Windows: 2, Attached: false},
	}
	m := Build(Deps{Lister: fakeLister{}, NoColor: true})
	m.termWidth = w
	m.termHeight = h
	m.applySessions(sessions)
	return m
}

// frameHasAnyBackgroundSGR reports whether the rendered frame ever activates an
// explicit background-colour SGR (a truecolor 48;… background, or a named 40-47 /
// 100-107 background). Under NO_COLOR the canvas is suppressed in BOTH layers, so
// no background SGR may ever become active — every cell renders on the terminal's
// native background.
func frameHasAnyBackgroundSGR(t *testing.T, frame string) bool {
	t.Helper()
	return frameEverActivates(t, frame, sgrBackgroundActive)
}

// frameHasAnyForegroundSGR reports whether the rendered frame ever activates an
// explicit foreground-colour SGR (an extended 38;2;… / 38;5;… foreground, or a
// named 30-37 / 90-97 foreground). It is the foreground counterpart of
// frameHasAnyBackgroundSGR — together the two cover both halves of the NO_COLOR
// carve-out, so no token's value can reach the writer in either slot.
//
// Extended-colour runs are consumed WHOLE (via production's
// consumeExtendedColorRun, the same primitive sgrBackgroundActive uses) so a
// background channel value that happens to equal a named foreground code can
// never be misread as a foreground change.
func frameHasAnyForegroundSGR(t *testing.T, frame string) bool {
	t.Helper()
	return frameEverActivates(t, frame, sgrForegroundActive)
}

// frameEverActivates walks the frame's SGR sequences in order, folding each into
// the running "is this slot explicitly coloured?" flag with fold, and reports
// whether the flag was ever raised.
//
// The walk is shared by the fg and bg scanners so the two halves of the NO_COLOR
// carve-out are checked by the identical traversal — only the fold differs.
// Scanning in order (rather than substring-matching) is what makes a later reset
// count: a colour that is set and then cleared still means a token reached the
// writer, and it is the RAISE that is reported, never the final state.
func frameEverActivates(t *testing.T, frame string, fold func(bool, []string) bool) bool {
	t.Helper()
	parser := ansi.NewParser()
	src := []byte(frame)
	state := byte(0)
	active := false
	for len(src) > 0 {
		seq, _, n, newState := ansi.DecodeSequence(src, state, parser)
		s := string(seq)
		if strings.HasPrefix(s, "\x1b[") && strings.HasSuffix(s, "m") {
			active = fold(active, sgrParamsList(s))
			if active {
				return true
			}
		}
		state = newState
		src = src[n:]
	}
	return false
}

// sgrForegroundActive folds an SGR sequence's parameters into the running "is an
// explicit foreground active?" flag — the mirror of production's
// sgrBackgroundActive, with the fg and bg roles swapped.
func sgrForegroundActive(active bool, params []string) bool {
	for i := 0; i < len(params); i++ {
		switch p := params[i]; {
		case p == "" || p == "0" || p == "39":
			active = false
		case p == "38":
			active = true
			i = consumeExtendedColorRun(params, i)
		case p == "48":
			i = consumeExtendedColorRun(params, i)
		case isNamedForeground(p):
			active = true
		}
	}
	return active
}

func isNamedForeground(p string) bool {
	switch p {
	case "30", "31", "32", "33", "34", "35", "36", "37",
		"90", "91", "92", "93", "94", "95", "96", "97":
		return true
	}
	return false
}

// TestColourless_SingleFlagFromDeps asserts the colourless decision is a single
// inheritable flag on the model, set from Deps.NoColor at the Build chokepoint —
// not re-derived per surface. Every canvas-dependent surface reads THIS flag.
func TestColourless_SingleFlagFromDeps(t *testing.T) {
	t.Run("NoColor sets the colourless flag", func(t *testing.T) {
		m := Build(Deps{Lister: fakeLister{}, NoColor: true})
		if !m.colourless {
			t.Errorf("Build(NoColor:true).colourless = false, want true")
		}
	})

	t.Run("without NoColor the flag is off", func(t *testing.T) {
		m := Build(Deps{Lister: fakeLister{}})
		if m.colourless {
			t.Errorf("Build(NoColor:false).colourless = true, want false")
		}
	})
}

// TestColourless_SkipsDetectionAndFirstPaintWait asserts that under NO_COLOR the
// appearance gate resolves IMMEDIATELY: there is no canvas to select, so the
// model is resolved at construction (no blank-frame wait) and Init arms NO
// detect-or-timeout tick.
//
// The OSC 11 QUERY is deliberately not part of that: §8.8 issues it under every
// shape, NO_COLOR included, because restore.go and §9.3's conversion need the
// reply independently of detection. TestGate_QueryIssuedRegardlessOfSettingShape
// covers it — and covers it correctly, by matching the query cmd's own message
// type, which is the unexported request marker rather than the
// tea.BackgroundColorMsg RESPONSE the program runtime later synthesises.
func TestColourless_SkipsDetectionAndFirstPaintWait(t *testing.T) {
	m := Build(Deps{Lister: fakeLister{}, NoColor: true, Theme: testBuiltinPair(t)})

	if !m.modeResolved() {
		t.Errorf("colourless model is unresolved; want immediate resolution (no canvas to select, no first-paint wait)")
	}

	for _, msg := range initCmds(t, m.Init()) {
		if _, ok := msg.(appearanceTimeoutMsg); ok {
			t.Errorf("colourless Init armed a detect-or-timeout tick; want none (detection skipped)")
		}
	}
}

// TestColourless_ViewSetsNoBackgroundColor asserts View() does NOT set the
// tea.View BackgroundColor under NO_COLOR (no OSC 11 set — Portal imposes no hues
// and does not recolour the terminal default).
func TestColourless_ViewSetsNoBackgroundColor(t *testing.T) {
	m := colourlessTestModel(t, 90, 24)
	v := m.View()
	if v.BackgroundColor != nil {
		t.Errorf("colourless View.BackgroundColor = %v, want nil (no OSC 11 canvas set)", v.BackgroundColor)
	}
}

// TestColourless_FillEmitsNoCanvasBackground asserts the rendered frame carries
// NO background-colour SGR under NO_COLOR — both the outer full-terminal fill and
// the leaf styles suppress the canvas bg, so every cell renders on the terminal's
// native background.
func TestColourless_FillEmitsNoCanvasBackground(t *testing.T) {
	m := colourlessTestModel(t, 90, 24)
	frame := m.View().Content
	if frameHasAnyBackgroundSGR(t, frame) {
		t.Errorf("colourless frame emits a background-colour SGR; want none (native bg, no painted canvas)")
	}
	// And specifically: neither the dark nor the light canvas sequence appears.
	if seq := canvasSeq(t, testDarkTheme(t)); strings.Contains(frame, seq) {
		t.Errorf("colourless frame contains the dark canvas background sequence %q", seq)
	}
	if seq := canvasSeq(t, testLightTheme(t)); strings.Contains(frame, seq) {
		t.Errorf("colourless frame contains the light canvas background sequence %q", seq)
	}
}

// TestColourless_NoTokenReachesTheWriter asserts the OTHER half of the carve-out:
// no token's value reaches the writer at all under NO_COLOR — not as a background
// (the canvas), and not as a foreground either. A theme is a set of token values,
// so the colourless path is only honest if every one of them is dropped before the
// frame is written; a colourless branch that still emitted a token's hue as a
// foreground would leave Portal imposing colour on a terminal that asked for none.
func TestColourless_NoTokenReachesTheWriter(t *testing.T) {
	frame := colourlessTestModel(t, 90, 24).View().Content

	if frameHasAnyForegroundSGR(t, frame) {
		t.Errorf("colourless frame emits a foreground-colour SGR; want none (no token may reach the writer)")
	}
	if frameHasAnyBackgroundSGR(t, frame) {
		t.Errorf("colourless frame emits a background-colour SGR; want none (no token may reach the writer)")
	}
}

// TestColourless_StateStaysGlyphDistinct asserts state stays glyph-distinct
// without colour: the attached ● marker and the session names are all present in
// the colourless frame, and the selector cursor glyph marks the selected row — so
// state is carried by glyph + position, never by hue alone (§2.2).
func TestColourless_StateStaysGlyphDistinct(t *testing.T) {
	m := colourlessTestModel(t, 90, 24)
	frame := m.View().Content

	for _, want := range []string{"● attached", "alpha", "bravo", "charlie"} {
		if !strings.Contains(frame, want) {
			t.Errorf("colourless frame missing %q (state must stay glyph-distinct without colour)", want)
		}
	}
	// The selector cursor glyph sits at the selected row. The ▌ selector bar must
	// be present (the selector is a glyph, not a hue) — under NO_COLOR the violet
	// drops but the bar glyph stays, keeping the selection glyph-distinct (§2.2).
	if !strings.Contains(frame, "▌") {
		t.Errorf("colourless frame missing the selector bar glyph (state via glyph, not colour)")
	}
}

// TestColourless_StructureMatchesColouredFrame asserts the colourless frame keeps
// the foundation Sessions structure: the same one-row-per-delegate layout, full
// terminal width per line, and exactly termH rows — only the canvas paint differs
// from the coloured path.
func TestColourless_StructureMatchesColouredFrame(t *testing.T) {
	const w, h = 90, 24
	m := colourlessTestModel(t, w, h)
	frame := m.View().Content

	if got := lipgloss.Height(frame); got != h {
		t.Errorf("colourless frame height = %d, want exactly %d (filled to termH)", got, h)
	}
	for i, line := range strings.Split(frame, "\n") {
		if lw := lipgloss.Width(line); lw != w {
			t.Errorf("colourless line %d width = %d, want exactly %d (padded to termW)", i, lw, w)
		}
	}
	if !strings.Contains(frame, "Sessions") {
		t.Errorf("colourless frame missing the 'Sessions' section header")
	}
}

// TestColourless_NavigationParity asserts behaviour parity: navigation under
// NO_COLOR is identical to the coloured path. Moving the cursor down selects the
// next session exactly as it does with colour.
func TestColourless_NavigationParity(t *testing.T) {
	m := colourlessTestModel(t, 90, 24)
	si, ok := m.selectedSessionItem()
	if !ok || si.Session.Name != "alpha" {
		t.Fatalf("initial selection = %q (ok=%v), want alpha", si.Session.Name, ok)
	}
	moved, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	si2, ok := moved.(Model).selectedSessionItem()
	if !ok || si2.Session.Name != "bravo" {
		t.Errorf("after down, selection = %q (ok=%v), want bravo (nav identical under NO_COLOR)", si2.Session.Name, ok)
	}
}

// TestColourless_FilterParity asserts filter behaviour parity under NO_COLOR:
// applying a filter narrows the list exactly as it does with colour. The same
// query is applied to BOTH the colourless and the coloured model and the
// resulting applied result set must be identical and genuinely narrowed —
// NO_COLOR changes only rendering, never the filter engine.
func TestColourless_FilterParity(t *testing.T) {
	colourless := colourlessTestModel(t, 90, 24)
	coloured := newCanvasTestModel(t, 90, 24, theme.MemberDark)

	// "charl" is a contiguous prefix of charlie only, so the applied filter
	// genuinely narrows the list to a single row.
	colourless.SetSessionListFilter("charl")
	coloured.SetSessionListFilter("charl")

	if !colourless.sessionList.IsFiltered() {
		t.Fatalf("colourless filter did not apply")
	}
	// The applied result set must be identical to the coloured path (behaviour
	// parity — the visible filtered rows must match exactly).
	cl, co := visibleSessionNames(colourless), visibleSessionNames(coloured)
	if !equalStrings(cl, co) {
		t.Errorf("colourless filtered rows %v != coloured filtered rows %v (filter parity broken)", cl, co)
	}
	if !equalStrings(cl, []string{"charlie"}) {
		t.Errorf("colourless applied filter rows = %v, want [charlie] (filter must narrow under NO_COLOR)", cl)
	}
	// And the narrowed render reflects the applied filter.
	frame := colourless.View().Content
	if !strings.Contains(frame, "charlie") {
		t.Errorf("colourless filtered frame missing the matched row 'charlie'")
	}
	if strings.Contains(frame, "alpha") || strings.Contains(frame, "bravo") {
		t.Errorf("colourless filtered frame still shows a non-matching row (filter parity broken)")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestColourless_ColouredPathUnaffected asserts the coloured path is unchanged by
// the NO_COLOR additive carve-out: a non-colourless model still paints the canvas
// (the canvas background sequence is present and View sets the OSC 11 bg).
func TestColourless_ColouredPathUnaffected(t *testing.T) {
	const w, h = 90, 24
	m := newCanvasTestModel(t, w, h, theme.MemberDark)
	v := m.View()
	if v.BackgroundColor == nil {
		t.Errorf("coloured View.BackgroundColor = nil, want the canvas colour set (coloured path must still paint)")
	}
	if seq := canvasSeq(t, testDarkTheme(t)); !strings.Contains(v.Content, seq) {
		t.Errorf("coloured frame missing the dark canvas background sequence %q (coloured path must still paint)", seq)
	}
}
