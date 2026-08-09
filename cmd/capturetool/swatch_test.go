package main

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/themetest"
)

// TestResolveProgramContrastValidation verifies the capture tool resolves the
// contrast-validation fixture into a runnable swatch tea.Model, driven by
// --theme. The swatch is the labelled-tint validation surface the human eyeball
// gate judges a theme's pinned tints on — deliberately NOT the production
// tui.Model, which is what lets it take a whole palette before the render layer
// does. It is resolved as a tea.Model so vhs can drive it.
func TestResolveProgramContrastValidation(t *testing.T) {
	t.Run("a built-in slug pins the swatch", func(t *testing.T) {
		m, err := resolveProgram(capture.ContrastValidationFixture, "tokyo-night", io.Discard)
		if err != nil {
			t.Fatalf("resolveProgram(contrast-validation, tokyo-night): %v", err)
		}
		if m == nil {
			t.Fatal("resolveProgram returned a nil model")
		}

		// The slug has to reach the RENDER, not merely the resolver. Both halves
		// are pinned elsewhere — resolveTheme returns the embedded palette, and
		// the swatch renders whatever palette it is handed — but nothing else
		// pins the join, and a wrong-variable slip at the call site is silent:
		// the tool would exit 0 having captured a frame of some other theme, at
		// a gate whose entire job is judging colours the tests cannot (§13.3).
		//
		// The expectation is sourced from the loader rather than hardcoded, so a
		// deliberate re-tint of tokyo-night's canvas moves the assertion with it
		// (mirroring TestResolveTheme_DefaultsToTokyoNight).
		want := themetest.Builtin(t, "tokyo-night")
		title := fmt.Sprintf("CONTRAST VALIDATION — canvas %s", want.Canvas.Value)
		if content := m.View().Content; !strings.Contains(content, title) {
			t.Errorf("the swatch does not render tokyo-night's palette: no %q in its view\n--- view ---\n%s", title, content)
		}
	})

	t.Run("an explicit path pins the swatch too", func(t *testing.T) {
		// The swatch is the surface a light theme's pinned tints are signed off
		// on (§7.5, §13.5), so a drop-in author must be able to point it at their
		// own file — the whole reason --theme takes a path (§13.3).
		path := themetest.Write(t, t.TempDir(), "mytheme.theme", themetest.WithValue(themetest.Lines(), "canvas", "#1a2b3c"))

		m, err := resolveProgram(capture.ContrastValidationFixture, path, io.Discard)
		if err != nil {
			t.Fatalf("resolveProgram(contrast-validation, %q): %v", path, err)
		}

		const title = "CONTRAST VALIDATION — canvas #1A2B3C"
		if content := m.View().Content; !strings.Contains(content, title) {
			t.Errorf("the swatch does not render the file's palette: no %q in its view\n--- view ---\n%s", title, content)
		}
	})

	t.Run("an unknown theme is an error", func(t *testing.T) {
		m, err := resolveProgram(capture.ContrastValidationFixture, "not-a-theme", io.Discard)
		if err == nil {
			t.Fatal("resolveProgram(contrast-validation, not-a-theme) returned nil error, want error")
		}
		if m != nil {
			t.Error("resolveProgram returned a model alongside its error — nothing must render")
		}
	})
}

// TestResolveProgramSessionsFixture verifies the sessions-flat fixture still
// resolves through the same dispatch (a tui.Model is a tea.Model), so the two
// branches share one entry point and one theme.
func TestResolveProgramSessionsFixture(t *testing.T) {
	m, err := resolveProgram("sessions-flat", defaultThemeSlug, io.Discard)
	if err != nil {
		t.Fatalf("resolveProgram(sessions-flat, %s): %v", defaultThemeSlug, err)
	}
	if m == nil {
		t.Fatal("resolveProgram returned a nil model")
	}
}
