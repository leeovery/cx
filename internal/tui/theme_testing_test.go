package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// sgrParams renders a one-cell probe through style and returns the SGR parameter
// run it opens with — everything between the CSI `[` and the terminating `m`.
//
// The bare run is the shape most frame assertions want: a style carrying a
// foreground, a background and attributes emits them as ONE sequence, so the
// wrapper a single-property style renders is not what lands in composed output —
// the `38;2;...` / `48;2;...` core is. A call site that needs the whole `ESC[…m`
// (a background-only fill does emit it standalone) rebuilds it around this run.
func sgrParams(t *testing.T, style lipgloss.Style) string {
	t.Helper()
	probe := style.Render("x")
	start := strings.IndexByte(probe, '[')
	end := strings.IndexByte(probe, 'm')
	if start < 0 || end <= start {
		t.Fatalf("could not derive an SGR parameter run from %q", probe)
	}
	return probe[start+1 : end]
}

// tokenFgSeq returns the bare `38;2;r;g;b` foreground SGR parameter run a role
// token renders as.
func tokenFgSeq(t *testing.T, tok theme.Token) string {
	t.Helper()
	return sgrParams(t, lipgloss.NewStyle().Foreground(tok.Color()))
}

// tokenBgSeq returns the bare `48;2;r;g;b` background SGR parameter run a role
// token renders as — the background analogue of tokenFgSeq, used to assert a tint
// IS or is NOT painted.
func tokenBgSeq(t *testing.T, tok theme.Token) string {
	t.Helper()
	return sgrParams(t, lipgloss.NewStyle().Background(tok.Color()))
}

// testDarkTheme and testLightTheme load the two embedded built-ins the model's
// transitional theme source selects between — the same two `internal/theme`
// resolves for the dark and light halves of the shipped default.
//
// They exist as ONE pair of helpers rather than an ad-hoc LoadBuiltin at each of
// the several hundred render-assertion sites: a test that needs "the palette the
// dark canvas renders" needs exactly this, and a per-site load would make the
// suite's notion of "the dark theme" re-derivable in several hundred places.
//
// They are FUNCTIONS, not package-level vars, deliberately. §3.4 forbids
// package-level mutable theme state on the render path, and TestNoPackageLevelThemeVar
// guards production against it; keeping the test-side source a function too means
// the suite cannot grow the very shape the guard exists to prevent.
func testDarkTheme(t *testing.T) theme.Theme {
	t.Helper()
	return themetest.DefaultDark(t)
}

func testLightTheme(t *testing.T) theme.Theme {
	t.Helper()
	return themetest.DefaultLight(t)
}

// themeLabel names th for a failure message — "dark", "light", or the theme's
// canvas hex for anything else.
//
// A token carries its VALUE now, so a whole Theme through %v is 19 {name value}
// pairs: ~500 characters of noise on a line whose only job is to say WHICH theme
// failed, where the mode it replaced printed 0 or 1. This says exactly that much.
//
// It identifies a theme by its CANVAS because that is the one token the two
// built-ins can never share — the canvas is what the mode *is*. Anything the
// built-in pair does not claim falls back to that hex rather than to "", so a
// hand-built or future theme still names itself in the message; a zero Theme
// (which several helper-built models legitimately hold) says so outright rather
// than borrowing a built-in's label.
func themeLabel(th theme.Theme) string {
	switch v := th.Canvas.Value; v {
	case "":
		return "zero-theme"
	case builtinCanvasValue(theme.DefaultDarkSlug):
		return "dark"
	case builtinCanvasValue(theme.DefaultLightSlug):
		return "light"
	default:
		return v
	}
}

// builtinCanvasValue returns the canvas value of the embedded built-in named
// slug, or "" if it does not load.
//
// It takes no *testing.T — themeLabel is evaluated inside an argument list, where
// there is no failing to be done — so a broken embedded file degrades to "",
// which themeLabel has already ruled out as a candidate value before it asks.
func builtinCanvasValue(slug string) string {
	loaded, rejection, found := theme.NewLoader(nil).LoadBuiltin(slug)
	if !found || rejection != nil {
		return ""
	}
	return loaded.Theme.Canvas.Value
}

// tokenNamed returns the role token called name off th.
//
// It exists because a token now carries its VALUE, so a table of tokens is a
// table of one theme's values: a test that walks several themes must re-resolve
// each role per theme rather than hold a fixed []theme.Token. Naming the role and
// resolving it here keeps those tables reading as the role list they are, and
// resolves through Theme.All() so a role that stops existing fails loudly rather
// than silently yielding a zero token.
func tokenNamed(t *testing.T, th theme.Theme, name string) theme.Token {
	t.Helper()
	for _, tok := range th.All() {
		if tok.Name == name {
			return tok
		}
	}
	t.Fatalf("theme carries no token named %q", name)
	return theme.Token{}
}

// testConstantFor is the CONSTANT nomination painting the built-in that the given
// canvas answer names.
//
// It is how a test pins a palette from frame one now that there is no light/dark
// appearance to pin: a constant skips the gate entirely (§8.8), so the model is
// resolved at construction and the frame is un-gated — the same property the
// capture harness relies on.
func testConstantFor(t *testing.T, appearance canvasAppearance) theme.Nomination {
	t.Helper()
	return theme.ConstantNomination(themeForAppearance(t, appearance))
}

// appearanceForTheme returns the gate answer that selects th from the built-in
// pair — the inverse of themeForAppearance, for the tests that drive a model
// through WithCanvasMode (which pins the ANSWER, from which the model re-derives
// the palette) while asserting against a palette.
func appearanceForTheme(t *testing.T, th theme.Theme) canvasAppearance {
	t.Helper()
	switch th.Canvas.Value {
	case testLightTheme(t).Canvas.Value:
		return appearanceLightCanvas
	case testDarkTheme(t).Canvas.Value:
		return appearanceDarkCanvas
	}
	t.Fatalf("theme with canvas %q is neither built-in", th.Canvas.Value)
	return appearanceDarkCanvas
}
