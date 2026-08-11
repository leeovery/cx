package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

func TestProjectsFilterInput_ColouredBranchOrange(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := New(fakeLister{}, WithThemeNomination(theme.ConstantNomination(th)))

		fi := m.projectList.FilterInput
		if fi.Prompt != filterPromptPrefix {
			t.Errorf("[%v] project FilterInput.Prompt = %q, want %q", themeLabel(th), fi.Prompt, filterPromptPrefix)
		}

		orange := tokenFgSeq(t, th.AccentAttention)
		styles := fi.Styles()

		if seq := styles.Focused.Prompt.Render("x"); !strings.Contains(seq, orange) {
			t.Errorf("[%v] project FilterInput Focused.Prompt missing accent.orange SGR %q (got %q)", themeLabel(th), orange, escSeq(seq))
		}
		if seq := styles.Focused.Text.Render("x"); !strings.Contains(seq, orange) {
			t.Errorf("[%v] project FilterInput Focused.Text missing accent.orange SGR %q (got %q)", themeLabel(th), orange, escSeq(seq))
		}
		cursorProbe := lipgloss.NewStyle().Foreground(styles.Cursor.Color).Render("x")
		if !strings.Contains(cursorProbe, orange) {
			t.Errorf("[%v] project FilterInput Cursor.Color missing accent.orange SGR %q (got %q)", themeLabel(th), orange, escSeq(cursorProbe))
		}
		if styles.Cursor.Blink {
			t.Errorf("[%v] project FilterInput Cursor.Blink = true, want false (deterministic block cursor)", themeLabel(th))
		}
	}
}

func TestProjectsFilterInput_ColourlessBranchBare(t *testing.T) {
	m := New(fakeLister{}, WithColourless(true))

	fi := m.projectList.FilterInput
	if fi.Prompt != filterPromptPrefix {
		t.Errorf("colourless project FilterInput.Prompt = %q, want %q", fi.Prompt, filterPromptPrefix)
	}

	styles := fi.Styles()

	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		orange := tokenFgSeq(t, th.AccentAttention)
		runs := map[string]string{
			"Focused.Prompt": styles.Focused.Prompt.Render(filterPromptPrefix),
			"Focused.Text":   styles.Focused.Text.Render("fab"),
			"Cursor.Color":   lipgloss.NewStyle().Foreground(styles.Cursor.Color).Render("x"),
		}
		for name, run := range runs {
			if strings.Contains(run, orange) {
				t.Errorf("colourless project FilterInput %s leaked accent.orange SGR %q (%v): %q", name, orange, themeLabel(th), escSeq(run))
			}
		}
	}

	if _, ok := styles.Cursor.Color.(lipgloss.NoColor); !ok {
		t.Errorf("colourless project FilterInput Cursor.Color = %#v, want lipgloss.NoColor{}", styles.Cursor.Color)
	}
	if styles.Cursor.Blink {
		t.Errorf("colourless project FilterInput Cursor.Blink = true, want false")
	}
}
