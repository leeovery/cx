package tui

import (
	"bytes"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

// Task spectrum-tui-design-9-3 consolidation gate. SessionDelegate.canvasBg /
// tokenStyle (session_item.go) were a fork of the header.go leaf canvas-paint
// pair (headerCanvasBg / headerStyle) — the same "role-token foreground over
// Background(canvas), bare under NO_COLOR" rule already exported as the single
// canonical source (and that loadingStyle/loadingFg + rowBg/rowToken already
// delegate to). These tests prove the post-consolidation delegate render is
// byte-identical to the PRE-refactor source:
//   - the helper-level probes pin canvasBg ≡ headerCanvasBg and tokenStyle ≡
//     headerStyle(...).Inherit(base) (so the delegate names delegate to the single
//     header source, never re-implement);
//   - preCanvasBg / preTokenStyle reproduce the ORIGINAL inline-logic bodies
//     verbatim, and the header-row + session-row goldens are rendered from those
//     originals, so any drift in the leaf-paint composition or the NO_COLOR
//     carve-out after the consolidation is caught byte-for-byte across light,
//     dark, and NO_COLOR (selected + unselected).
//
// No t.Parallel() — the shared canvas helpers make parallelism unsafe.

// preCanvasBg reproduces the ORIGINAL SessionDelegate.canvasBg inline logic
// verbatim — the golden the consolidation must preserve.
func preCanvasBg(th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Background(th.Canvas.Color())
}

// preTokenStyle reproduces the ORIGINAL SessionDelegate.tokenStyle inline logic
// verbatim (base + token foreground over canvas, base unchanged under NO_COLOR) —
// the golden the consolidation must preserve.
func preTokenStyle(base lipgloss.Style, fg theme.Token, th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return base
	}
	return base.
		Foreground(fg.Color()).
		Background(th.Canvas.Color())
}

// TestSessionCanvasBg_DelegatesToHeaderCanvasBg asserts the consolidated
// canvasBg renders a probe string byte-identically to headerCanvasBg (the single
// header.go source) AND to the original inline canvasBg logic, across both modes ×
// colourless on/off. Pins that canvasBg no longer re-implements the leaf paint.
func TestSessionCanvasBg_DelegatesToHeaderCanvasBg(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		for _, colourless := range []bool{false, true} {
			d := SessionDelegate{Theme: th, Colourless: colourless}
			want := headerCanvasBg(th, colourless).Render("  ")
			if got := d.canvasBg().Render("  "); got != want {
				t.Errorf("canvasBg(th=%v col=%v) = %q, want headerCanvasBg %q",
					themeLabel(th), colourless, got, want)
			}
			pre := preCanvasBg(th, colourless).Render("  ")
			if want != pre {
				t.Errorf("headerCanvasBg(th=%v col=%v) = %q drifts from pre-refactor canvasBg %q",
					themeLabel(th), colourless, want, pre)
			}
		}
	}
}

// TestSessionTokenStyle_DelegatesToHeaderStyle asserts the consolidated
// tokenStyle renders a probe string byte-identically to headerStyle(...).Inherit(base)
// (the single header.go source composited with the caller-supplied base) AND to
// the original inline tokenStyle logic, across a representative token × both modes ×
// colourless on/off × an empty base and a Bold base. Pins that tokenStyle no longer
// re-implements the leaf token-over-canvas paint.
func TestSessionTokenStyle_DelegatesToHeaderStyle(t *testing.T) {
	roles := []string{"text.muted", "text.subtle", "text.primary", "state.positive"}
	bases := map[string]lipgloss.Style{
		"empty": lipgloss.Style{},
		"bold":  lipgloss.NewStyle().Bold(true),
	}
	for baseName, base := range bases {
		for _, role := range roles {
			for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
				tok := tokenNamed(t, th, role)
				for _, colourless := range []bool{false, true} {
					d := SessionDelegate{Theme: th, Colourless: colourless}
					want := headerStyle(tok, th, colourless).Inherit(base).Render("probe")
					if got := d.tokenStyle(base, tok).Render("probe"); got != want {
						t.Errorf("tokenStyle(base=%s tok=%s th=%v col=%v) = %q, want headerStyle.Inherit %q",
							baseName, tok.Name, themeLabel(th), colourless, got, want)
					}
					pre := preTokenStyle(base, tok, th, colourless).Render("probe")
					if want != pre {
						t.Errorf("headerStyle(base=%s tok=%s th=%v col=%v) = %q drifts from pre-refactor tokenStyle %q",
							baseName, tok.Name, themeLabel(th), colourless, want, pre)
					}
				}
			}
		}
	}
}

// TestSessionDelegateRender_ByteIdenticalAcrossConsolidation asserts the full
// delegate render of a HeaderItem (the row that routes through canvasBg/tokenStyle)
// and of a grouped session row (selected + unselected) is byte-for-byte identical
// across the consolidation, in light, dark, and NO_COLOR. The goldens in
// sessionStyleGoldens were CAPTURED from the PRE-refactor source (forked
// canvasBg/tokenStyle bodies, via a throwaway generator run — see the task notes),
// so a drift in the leaf-paint composition or the NO_COLOR carve-out after the
// consolidation is caught verbatim.
func TestSessionDelegateRender_ByteIdenticalAcrossConsolidation(t *testing.T) {
	const w = 80
	modeNames := map[theme.Theme]string{testDarkTheme(t): "dark", testLightTheme(t): "light"}

	header := HeaderItem{Heading: "Portal", Count: 2, Key: "/p/portal"}
	sessions := []list.Item{
		SessionItem{Session: tmux.Session{Name: "dev", Windows: 3, Attached: true}, GroupKey: "/p/portal", GroupHeading: "Portal"},
		SessionItem{Session: tmux.Session{Name: "work", Windows: 1, Attached: false}, GroupKey: "/p/portal", GroupHeading: "Portal"},
	}

	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		for _, colourless := range []bool{false, true} {
			d := SessionDelegate{Theme: th, Colourless: colourless}
			modeName := modeNames[th]

			hm := list.New([]list.Item{header}, d, w, 10)
			var hb bytes.Buffer
			d.Render(&hb, hm, 0, header)
			assertSessionGolden(t, "header", modeName, colourless, hb.String())

			sm := list.New(sessions, d, w, 10)
			var sel, unsel bytes.Buffer
			d.Render(&sel, sm, 0, sessions[0])   // index 0 == selected
			d.Render(&unsel, sm, 1, sessions[1]) // index 1 != selected
			assertSessionGolden(t, "session-selected", modeName, colourless, sel.String())
			assertSessionGolden(t, "session-unselected", modeName, colourless, unsel.String())
		}
	}
}

func assertSessionGolden(t *testing.T, frame, th string, colourless bool, got string) {
	t.Helper()
	want, ok := sessionStyleGoldens[sessionStyleGoldenKey{frame, th, colourless}]
	if !ok {
		t.Fatalf("no golden for {%s %s col=%v}", frame, th, colourless)
	}
	if got != want {
		t.Errorf("[%s %s col=%v] delegate render drifted from pre-refactor golden\n got: %q\nwant: %q",
			frame, th, colourless, ansi.Strip(got), ansi.Strip(want))
	}
}

type sessionStyleGoldenKey struct {
	frame      string
	th         string
	colourless bool
}

// sessionStyleGoldens are the EXACT bytes the PRE-refactor delegate render (forked
// canvasBg/tokenStyle bodies) emitted for a Portal HeaderItem and a grouped
// session row (selected + unselected) at width 80, captured from a throwaway
// generator run against the pre-consolidation source. Keyed by
// {frame, mode, colourless}. The post-consolidation render MUST reproduce these
// byte-for-byte.
var sessionStyleGoldens = map[sessionStyleGoldenKey]string{
	{"header", "dark", false}:              "\x1b[48;2;11;12;20m  \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mPortal \x1b[m\x1b[38;2;83;92;134;48;2;11;12;20m··· 2\x1b[m",
	{"session-selected", "dark", false}:    "\x1b[48;2;40;36;58m  \x1b[m\x1b[38;2;187;154;247;48;2;40;36;58m▌\x1b[m\x1b[48;2;40;36;58m \x1b[m\x1b[1;38;2;255;255;255;48;2;40;36;58mdev\x1b[m\x1b[48;2;40;36;58m                                                \x1b[m\x1b[48;2;40;36;58m  \x1b[m\x1b[38;2;169;177;214;48;2;40;36;58m3 windows\x1b[m\x1b[48;2;40;36;58m  \x1b[m\x1b[38;2;158;206;106;48;2;40;36;58m● attached\x1b[m\x1b[48;2;40;36;58m\x1b[m\x1b[48;2;40;36;58m  \x1b[m",
	{"session-unselected", "dark", false}:  "\x1b[48;2;11;12;20m  \x1b[m\x1b[48;2;11;12;20m  \x1b[m\x1b[1;38;2;192;202;245;48;2;11;12;20mwork\x1b[m\x1b[48;2;11;12;20m                                               \x1b[m\x1b[48;2;11;12;20m  \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20m1 window\x1b[m\x1b[48;2;11;12;20m   \x1b[m\x1b[48;2;11;12;20m          \x1b[m\x1b[48;2;11;12;20m  \x1b[m",
	{"header", "dark", true}:               "  Portal ··· 2",
	{"session-selected", "dark", true}:     "  ▌ \x1b[1mdev\x1b[m                                                  3 windows  ● attached  ",
	{"session-unselected", "dark", true}:   "    \x1b[1mwork\x1b[m                                                 1 window               ",
	{"header", "light", false}:             "\x1b[48;2;225;226;231m  \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mPortal \x1b[m\x1b[38;2;118;125;162;48;2;225;226;231m··· 2\x1b[m",
	{"session-selected", "light", false}:   "\x1b[48;2;208;198;240m  \x1b[m\x1b[38;2;138;63;209;48;2;208;198;240m▌\x1b[m\x1b[48;2;208;198;240m \x1b[m\x1b[1;38;2;26;27;46;48;2;208;198;240mdev\x1b[m\x1b[48;2;208;198;240m                                                \x1b[m\x1b[48;2;208;198;240m  \x1b[m\x1b[38;2;63;71;96;48;2;208;198;240m3 windows\x1b[m\x1b[48;2;208;198;240m  \x1b[m\x1b[38;2;59;94;24;48;2;208;198;240m● attached\x1b[m\x1b[48;2;208;198;240m\x1b[m\x1b[48;2;208;198;240m  \x1b[m",
	{"session-unselected", "light", false}: "\x1b[48;2;225;226;231m  \x1b[m\x1b[48;2;225;226;231m  \x1b[m\x1b[1;38;2;46;60;100;48;2;225;226;231mwork\x1b[m\x1b[48;2;225;226;231m                                               \x1b[m\x1b[48;2;225;226;231m  \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231m1 window\x1b[m\x1b[48;2;225;226;231m   \x1b[m\x1b[48;2;225;226;231m          \x1b[m\x1b[48;2;225;226;231m  \x1b[m",
	{"header", "light", true}:              "  Portal ··· 2",
	{"session-selected", "light", true}:    "  ▌ \x1b[1mdev\x1b[m                                                  3 windows  ● attached  ",
	{"session-unselected", "light", true}:  "    \x1b[1mwork\x1b[m                                                 1 window               ",
}
