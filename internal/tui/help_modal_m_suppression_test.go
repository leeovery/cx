package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/spawn"
)

const multiSelectHelpLabel = "Multi-select mode"

const multiSelectFooterLabel = "m multi"

func keymapHasMKey(entries []keymapEntry) bool {
	for _, e := range entries {
		if e.Key == "m" {
			return true
		}
	}
	return false
}

func TestSessionsHelpKeymap_UnsupportedNotInMultiSelect_OmitsM(t *testing.T) {
	tests := []struct {
		name     string
		identity spawn.Identity
	}{
		{"named undriven (com.apple.Terminal)", appleTerminalIdentity()},
		{"NULL remote/mosh (empty identity)", spawn.Identity{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := unsupportedResolvedModel(t, tc.identity)
			if !m.DetectUnsupported() {
				t.Fatalf("precondition: %s must resolve unsupported", tc.name)
			}
			if m.multiSelectMode {
				t.Fatal("precondition: must not be in multi-select mode")
			}

			if keymapHasMKey(m.sessionsHelpKeymap()) {
				t.Error("sessionsHelpKeymap() must OMIT the m entry when unsupported and not in multi-select")
			}

			body := ansi.Strip(helpModalBody(m.sessionsHelpKeymap(), testDarkTheme(t), false))
			if strings.Contains(body, multiSelectHelpLabel) {
				t.Errorf("rendered help body must omit %q when m is blocked:\n%s", multiSelectHelpLabel, body)
			}
		})
	}
}

func TestSessionsHelpKeymap_Supported_ListsM(t *testing.T) {
	m := unsupportedResolvedModel(t, ghosttyIdentity())
	if m.DetectUnsupported() {
		t.Fatal("precondition: ghostty must resolve native (supported)")
	}

	if !keymapHasMKey(m.sessionsHelpKeymap()) {
		t.Error("sessionsHelpKeymap() must LIST the m entry on a supported terminal (filter inert)")
	}

	body := ansi.Strip(helpModalBody(m.sessionsHelpKeymap(), testDarkTheme(t), false))
	if !strings.Contains(body, multiSelectHelpLabel) {
		t.Errorf("rendered help body must list %q on a supported terminal:\n%s", multiSelectHelpLabel, body)
	}
}

func TestSessionsHelpKeymap_UnsupportedInMultiSelect_ListsM(t *testing.T) {
	tests := []struct {
		name     string
		identity spawn.Identity
	}{
		{"named undriven (com.apple.Terminal)", appleTerminalIdentity()},
		{"NULL remote/mosh (empty identity)", spawn.Identity{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := unsupportedResolvedModel(t, tc.identity)
			if !m.DetectUnsupported() {
				t.Fatalf("precondition: %s must resolve unsupported", tc.name)
			}
			m.multiSelectMode = true

			if !keymapHasMKey(m.sessionsHelpKeymap()) {
				t.Error("sessionsHelpKeymap() must LIST m while in multi-select mode — the help never hides a working row-toggle")
			}

			body := ansi.Strip(helpModalBody(m.sessionsHelpKeymap(), testDarkTheme(t), false))
			if !strings.Contains(body, multiSelectHelpLabel) {
				t.Errorf("rendered help body must list %q while in multi-select mode:\n%s", multiSelectHelpLabel, body)
			}
		})
	}
}

func TestSessionsFooter_ListsMultiOnlyWhenFunctional(t *testing.T) {
	tests := []struct {
		name      string
		identity  spawn.Identity
		wantMulti bool
	}{
		{"supported (ghostty → native)", ghosttyIdentity(), true},
		{"named undriven (com.apple.Terminal)", appleTerminalIdentity(), false},
		{"NULL remote/mosh (empty identity)", spawn.Identity{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := unsupportedResolvedModel(t, tc.identity)
			if supported := !m.DetectUnsupported(); supported != tc.wantMulti {
				t.Fatalf("precondition: %s must resolve supported=%v", tc.name, tc.wantMulti)
			}
			if m.multiSelectMode {
				t.Fatal("precondition: must not be in multi-select mode")
			}

			footer := ansi.Strip(renderSessionsFooter(m.sessionsHelpKeymap(), referenceFooterWidth, m.themeState.active, m.colourless))
			if got := strings.Contains(footer, multiSelectFooterLabel); got != tc.wantMulti {
				t.Errorf("the condensed Sessions footer carries %q = %v, want %v (§14.1 lists it, §14.3 filters it where blocked):\n%s", multiSelectFooterLabel, got, tc.wantMulti, footer)
			}
		})
	}
}

func TestSessionsKeymap_StaticConstantUnaffectedByFilter(t *testing.T) {
	if !keymapHasMKey(sessionsKeymap()) {
		t.Error("sessionsKeymap() must remain a pure static constant that always lists m — the filter belongs at the call site only")
	}
}
