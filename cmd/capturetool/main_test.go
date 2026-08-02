package main

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// TestResolveModel verifies the capture tool resolves a fixture name into a real
// production tui.Model via the shared tui.Build constructor — without opening a
// tmux server or touching config (the fixture is fully in-memory).
func TestResolveModel(t *testing.T) {
	t.Run("known fixture builds a sessions-page model", func(t *testing.T) {
		m, err := resolveModel("sessions-flat", "dark")
		if err != nil {
			t.Fatalf("resolveModel(sessions-flat): %v", err)
		}
		if m.ActivePage() != tui.PageSessions {
			t.Errorf("ActivePage() = %d, want PageSessions", m.ActivePage())
		}
	})

	t.Run("unknown fixture is an error that lists the available fixtures", func(t *testing.T) {
		_, err := resolveModel("nope", "dark")
		if err == nil {
			t.Fatal("resolveModel(nope) returned nil error, want error")
		}
		if !strings.Contains(err.Error(), "sessions-flat") {
			t.Errorf("error %q does not list the available fixtures", err.Error())
		}
	})

	t.Run("empty fixture name is an error", func(t *testing.T) {
		if _, err := resolveModel("", "dark"); err == nil {
			t.Fatal("resolveModel(\"\") returned nil error, want error")
		}
	})

	t.Run("invalid appearance is an error", func(t *testing.T) {
		if _, err := resolveModel("sessions-flat", "purple"); err == nil {
			t.Fatal("resolveModel with appearance=purple returned nil error, want error")
		}
	})
}

// TestResolveAppearanceSlug verifies the --appearance flag maps to the built-in
// slug whose palette the harness pins as a CONSTANT nomination (§13.3). A
// constant skips the gate entirely — no OSC 11 detection, no first-paint wait —
// which is what keeps a capture byte-deterministic.
//
// The flag survives exactly one more task: 3-4 replaces it with --theme, which
// takes a slug or a path directly.
func TestResolveAppearanceSlug(t *testing.T) {
	t.Run("dark names the shipped dark built-in", func(t *testing.T) {
		got, err := resolveAppearanceSlug("dark")
		if err != nil {
			t.Fatalf("resolveAppearanceSlug(dark): %v", err)
		}
		if got != theme.DefaultDarkSlug {
			t.Errorf("resolveAppearanceSlug(dark) = %q, want %q", got, theme.DefaultDarkSlug)
		}
	})

	t.Run("light names the shipped light built-in", func(t *testing.T) {
		got, err := resolveAppearanceSlug("light")
		if err != nil {
			t.Fatalf("resolveAppearanceSlug(light): %v", err)
		}
		if got != theme.DefaultLightSlug {
			t.Errorf("resolveAppearanceSlug(light) = %q, want %q", got, theme.DefaultLightSlug)
		}
	})

	t.Run("invalid value is an error", func(t *testing.T) {
		if _, err := resolveAppearanceSlug("purple"); err == nil {
			t.Fatal("resolveAppearanceSlug(purple) returned nil error, want error")
		}
	})
}

// TestResolveTheme_DefaultsToTokyoNight pins the built-in --theme resolves to
// when the flag is omitted.
//
// It is the shipped dark default (§13.3), and every capture taken without the
// flag depends on it — a moved default silently re-points every such capture at
// a palette nobody asked for.
func TestResolveTheme_DefaultsToTokyoNight(t *testing.T) {
	if defaultThemeSlug != "tokyo-night" {
		t.Fatalf("defaultThemeSlug = %q, want tokyo-night (the shipped dark default)", defaultThemeSlug)
	}

	got, err := resolveTheme(newThemeLoader(), defaultThemeSlug)
	if err != nil {
		t.Fatalf("resolveTheme(%s): %v", defaultThemeSlug, err)
	}

	want, rejection, found := newThemeLoader().LoadBuiltin("tokyo-night")
	if !found || rejection != nil {
		t.Fatalf("LoadBuiltin(tokyo-night) found=%v rejection=%v", found, rejection)
	}
	if got != want.Theme {
		t.Errorf("resolveTheme(%s) did not return the embedded tokyo-night palette", defaultThemeSlug)
	}
}

// TestResolveTheme_UnknownSlugIsAnError asserts a slug naming no built-in is a
// hard error carrying the slug, never a silent fall back to some other palette.
//
// Rendering the wrong theme at a visual gate is precisely the failure this tool
// exists to prevent (§13.3), so an unknown slug must render nothing at all.
func TestResolveTheme_UnknownSlugIsAnError(t *testing.T) {
	got, err := resolveTheme(newThemeLoader(), "not-a-theme")
	if err == nil {
		t.Fatal("resolveTheme(not-a-theme) returned nil error, want an error")
	}
	if !strings.Contains(err.Error(), "not-a-theme") {
		t.Errorf("error %q does not name the slug that was asked for", err.Error())
	}
	if got != (theme.Theme{}) {
		t.Error("resolveTheme returned a palette alongside its error")
	}
}

// TestResolveTheme_InvalidThemeIsAnErrorNotAFallback asserts a slug that DOES
// name a built-in whose file fails validation is a hard error carrying the §6.2
// reason — again never a fallback.
//
// §7.6's build-time guarantee makes this unreachable through the shipped set, so
// the rejection arrives through an injected loader rather than by breaking a
// shipped file.
func TestResolveTheme_InvalidThemeIsAnErrorNotAFallback(t *testing.T) {
	rejection := &theme.Rejection{Reason: theme.ReasonMissingTokens, Detail: "missing canvas, border"}

	got, err := resolveTheme(rejectingLoader{rejection: rejection}, "tokyo-night")
	if err == nil {
		t.Fatal("resolveTheme returned nil error for a rejected built-in, want an error rather than a fallback")
	}
	for _, want := range []string{"tokyo-night", string(theme.ReasonMissingTokens), rejection.Detail} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err.Error(), want)
		}
	}
	if got != (theme.Theme{}) {
		t.Error("resolveTheme returned a palette alongside its error")
	}
}

// rejectingLoader is a built-in loader that reports every slug as a known
// built-in whose file the §6.2 ladder refused.
type rejectingLoader struct {
	rejection *theme.Rejection
}

// LoadBuiltin returns the canned rejection, found, and no palette — the shape
// theme.Loader produces for a built-in that does not parse.
func (l rejectingLoader) LoadBuiltin(string) (theme.Result, *theme.Rejection, bool) {
	return theme.Result{}, l.rejection, true
}

// TestCaptureTool_ThemeResolutionIsSilent asserts resolving a theme writes
// NOTHING to the log.
//
// capturetool is §12.3's fifth caller and neither uses nor diagnoses a theme —
// it is an offline renderer whose output is a frame — so the loader is handed
// log.Discard and the `theme` component stays empty on this path. The positive
// control proves the assertion is not vacuous: a component logger emitting
// through the same swapped handler IS captured.
func TestCaptureTool_ThemeResolutionIsSilent(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)

	if _, err := resolveProgram(capture.ContrastValidationFixture, "dark", defaultThemeSlug); err != nil {
		t.Fatalf("resolveProgram(contrast-validation, %s): %v", defaultThemeSlug, err)
	}
	if _, err := resolveProgram(capture.ContrastValidationFixture, "dark", "not-a-theme"); err == nil {
		t.Fatal("resolveProgram with an unknown theme returned nil error")
	}

	if got := sink.Records(); len(got) != 0 {
		t.Errorf("theme resolution emitted %d log records, want 0: %+v", len(got), got)
	}

	theme.NewEventLogger(log.For("theme")).Rejected("some-theme", "", &theme.Rejection{Reason: theme.ReasonBadSyntax})
	if got := sink.Records(); len(got) != 1 {
		t.Fatalf("the positive control emitted %d records, want 1 — the sink is not wired to the theme component", len(got))
	}
}
