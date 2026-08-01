package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/capture"
)

// TestResolveProgramContrastValidation verifies the capture tool resolves the
// contrast-validation fixture into a runnable swatch tea.Model, driven by
// --theme. The swatch is the labelled-tint validation surface the human eyeball
// gate judges a theme's pinned tints on — deliberately NOT the production
// tui.Model, which is what lets it take a whole palette before the render layer
// does. It is resolved as a tea.Model so vhs can drive it.
func TestResolveProgramContrastValidation(t *testing.T) {
	t.Run("a built-in slug pins the swatch", func(t *testing.T) {
		m, err := resolveProgram(capture.ContrastValidationFixture, "dark", "tokyo-night")
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
		want, rejection, found := newThemeLoader().LoadBuiltin("tokyo-night")
		if !found || rejection != nil {
			t.Fatalf("LoadBuiltin(tokyo-night) found=%v rejection=%v", found, rejection)
		}
		title := fmt.Sprintf("CONTRAST VALIDATION — canvas %s", want.Theme.Canvas.Value)
		if content := m.View().Content; !strings.Contains(content, title) {
			t.Errorf("the swatch does not render tokyo-night's palette: no %q in its view\n--- view ---\n%s", title, content)
		}
	})

	t.Run("an unknown theme is an error", func(t *testing.T) {
		m, err := resolveProgram(capture.ContrastValidationFixture, "dark", "not-a-theme")
		if err == nil {
			t.Fatal("resolveProgram(contrast-validation, not-a-theme) returned nil error, want error")
		}
		if m != nil {
			t.Error("resolveProgram returned a model alongside its error — nothing must render")
		}
	})

	t.Run("the swatch ignores --appearance", func(t *testing.T) {
		// --appearance drives the tui.Build path only: a theme carries its own
		// canvas, so there is no mode left for the swatch to pin.
		if _, err := resolveProgram(capture.ContrastValidationFixture, "purple", "tokyo-night"); err != nil {
			t.Fatalf("resolveProgram(contrast-validation, appearance=purple): %v", err)
		}
	})
}

// TestResolveProgramSessionsFixture verifies the existing sessions-flat fixture
// still resolves through the same dispatch (a tui.Model is a tea.Model) via
// --appearance, so the swatch's move to --theme is additive and does not regress
// the production capture path.
func TestResolveProgramSessionsFixture(t *testing.T) {
	m, err := resolveProgram("sessions-flat", "dark", defaultThemeSlug)
	if err != nil {
		t.Fatalf("resolveProgram(sessions-flat, dark): %v", err)
	}
	if m == nil {
		t.Fatal("resolveProgram returned a nil model")
	}
}
