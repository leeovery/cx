package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// This file is the net-new gate for the §7 MV filter-input reskin on the
// PROJECTS list. styleFilterInput now restyles BOTH m.sessionList.FilterInput
// and m.projectList.FilterInput through the same shared per-list helper, so the
// two pages can never drift. These assertions mirror the Sessions filter-input
// gate (filtering_reskin_test.go) for the project list: an accent.attention `/
// ` prompt, accent.attention query text, and an accent.attention block cursor
// (blink off) in the coloured branch (Dark/Light), dropping to a bare NoColor
// input under the NO_COLOR carve-out. Colour roles are pinned in exact
// mode-resolved SGR (the §2.9 token core), like the sibling filtering-reskin
// tests — a token swap is caught, not merely a glyph's presence. No
// t.Parallel() (the package-level mock convention and the shared canvas helpers
// make parallelism unsafe across this package's tests).

// TestProjectsFilterInput_ColouredBranchOrange asserts §7: after the Projects-list
// model is constructed and the canvas mode applied, the project FilterInput's `/ `
// prompt, live query text, and block cursor all carry the accent.attention token in
// both Dark and Light, with blink disabled. Pinned in exact mode-resolved SGR.
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

// TestProjectsFilterInput_ColourlessBranchBare asserts the §2.5 NO_COLOR
// carve-out: the project FilterInput keeps the `/ ` prompt but emits NO
// accent.attention SGR — the prompt, query, and cursor render on the terminal's
// native fg, and the cursor is a bare NoColor block with blink disabled.
func TestProjectsFilterInput_ColourlessBranchBare(t *testing.T) {
	m := New(fakeLister{}, WithColourless(true))

	fi := m.projectList.FilterInput
	if fi.Prompt != filterPromptPrefix {
		t.Errorf("colourless project FilterInput.Prompt = %q, want %q", fi.Prompt, filterPromptPrefix)
	}

	styles := fi.Styles()

	// No accent.attention SGR may be emitted by any of the input's styled runs (checked
	// against BOTH mode variants of the token so neither hex can leak through).
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

	// The cursor is a bare block (NoColor) — no foreground colour SGR at all.
	if _, ok := styles.Cursor.Color.(lipgloss.NoColor); !ok {
		t.Errorf("colourless project FilterInput Cursor.Color = %#v, want lipgloss.NoColor{}", styles.Cursor.Color)
	}
	if styles.Cursor.Blink {
		t.Errorf("colourless project FilterInput Cursor.Blink = true, want false")
	}
}
