package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

func TestProjectsFooter_CondensedCopyExact(t *testing.T) {
	footer := renderProjectsFooter(projectsKeymap(), referenceFooterWidth, testDarkTheme(t), false)
	lines := strings.Split(footer, "\n")
	if len(lines) != 2 {
		t.Fatalf("condensed Projects footer must be 2 rows (rule + key row), got %d:\n%s", len(lines), footer)
	}
	keyRow := footerVisible(lines[1])

	for _, want := range []string{
		"⏎ new session", "x sessions", "e edit", "/ filter", "t theme", "? help",
	} {
		if !strings.Contains(keyRow, want) {
			t.Errorf("Projects key row missing Core entry %q:\n%s", want, keyRow)
		}
	}

	helpIdx := strings.Index(keyRow, "? help")
	filterIdx := strings.Index(keyRow, "/ filter")
	if helpIdx < 0 || filterIdx < 0 {
		t.Fatalf("expected both '/ filter' and '? help' on the key row:\n%s", keyRow)
	}
	if helpIdx <= filterIdx {
		t.Errorf("? help (idx %d) must be right of / filter (idx %d):\n%s", helpIdx, filterIdx, keyRow)
	}

	if got := lipgloss.Width(lines[1]); got != referenceFooterWidth {
		t.Errorf("Projects key row width = %d, want exactly %d (right-aligned to width)", got, referenceFooterWidth)
	}
	if !strings.HasSuffix(strings.TrimRight(keyRow, " "), "help") {
		t.Errorf("? help must be the trailing (right-most) entry on the row:\n%s", keyRow)
	}
}

func TestProjectsFooter_NoLegacySessionsCopy(t *testing.T) {
	keyRow := footerVisible(renderProjectsFooter(projectsKeymap(), referenceFooterWidth, testDarkTheme(t), false))
	for _, banned := range []string{"attach", "switch view", "preview", "navigate", "projects"} {
		if strings.Contains(keyRow, banned) {
			t.Errorf("Projects footer leaked the Sessions copy %q:\n%s", banned, keyRow)
		}
	}
}

func TestProjectsFooter_TokenColours(t *testing.T) {
	forEachBuiltinTheme(t, func(t *testing.T, th theme.Theme) {
		footer := renderProjectsFooter(projectsKeymap(), referenceFooterWidth, th, false)

		if seq := tokenFgSeq(t, th.AccentKey); !strings.Contains(footer, seq) {
			t.Errorf("Projects footer missing accent.blue key-glyph role sequence %q", seq)
		}
		if seq := tokenFgSeq(t, th.TextMuted); !strings.Contains(footer, seq) {
			t.Errorf("Projects footer missing text.detail label role sequence %q", seq)
		}
		if seq := tokenFgSeq(t, th.AccentPrimary); !strings.Contains(footer, seq) {
			t.Errorf("Projects footer missing accent.violet ? glyph role sequence %q", seq)
		}
		if seq := tokenFgSeq(t, th.Border); !strings.Contains(footer, seq) {
			t.Errorf("Projects footer missing the border top-rule role sequence %q", seq)
		}
	})
}

func TestProjectsFooter_ColourlessDropsHueAndCanvas(t *testing.T) {
	footer := renderProjectsFooter(projectsKeymap(), referenceFooterWidth, testDarkTheme(t), true)
	keyRow := footerVisible(strings.Split(footer, "\n")[1])

	for _, want := range []string{"⏎ new session", "x sessions", "e edit", "/ filter", "? help"} {
		if !strings.Contains(keyRow, want) {
			t.Errorf("colourless Projects footer dropped the copy %q:\n%s", want, keyRow)
		}
	}
	if seq := canvasSeq(t, testDarkTheme(t)); strings.Contains(footer, seq) {
		t.Errorf("colourless Projects footer still paints the canvas background sequence %q", seq)
	}
	for _, tok := range []theme.Token{testDarkTheme(t).AccentKey, testDarkTheme(t).TextMuted, testDarkTheme(t).AccentPrimary} {
		if seq := tokenFgSeq(t, tok); strings.Contains(footer, seq) {
			t.Errorf("colourless Projects footer still emits a foreground role sequence %q", seq)
		}
	}
}

func TestProjectsFooter_FilterAppliedSaysNewSessionNotAttach(t *testing.T) {
	footer := renderProjectsFilterAppliedFooter(referenceFooterWidth, testDarkTheme(t), false)
	keyRow := footerVisible(strings.Split(footer, "\n")[1])
	if !strings.Contains(keyRow, "new session") {
		t.Errorf("Projects list-active footer missing %q:\n%s", "new session", keyRow)
	}
	if strings.Contains(keyRow, "attach") {
		t.Errorf("Projects list-active footer leaks the Sessions %q copy:\n%s", "attach", keyRow)
	}
	for _, want := range []string{"navigate", "clear filter"} {
		if !strings.Contains(keyRow, want) {
			t.Errorf("Projects list-active footer missing %q:\n%s", want, keyRow)
		}
	}
}

func TestProjectsFooter_NarrowDegradeKeepsHelpAnchor(t *testing.T) {
	const narrow = 24
	footer := renderProjectsFooter(projectsKeymap(), narrow, testDarkTheme(t), false)
	for i, line := range strings.Split(footer, "\n") {
		if lw := lipgloss.Width(line); lw > narrow {
			t.Errorf("narrow Projects footer line %d width = %d (overflow, want <= %d):\n%s", i, lw, narrow, ansi.Strip(line))
		}
	}
}
