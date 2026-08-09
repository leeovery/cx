package tui

// persistent-no-host-terminal-banner-2-3 — Help-Modal m-Suppression at the
// Sessions call site (spec §4 / §7).
//
// These white-box (package tui) tests pin the §4 call-site filter: the `m`
// (multi-select) entry is dropped from the descriptor slice fed to the Sessions
// `?` help modal IFF `DetectUnsupported() && !multiSelectMode` — exactly "`m`
// appears in help iff `m` is functional". sessionsKeymap() itself stays a pure
// static constant (the filter lives at the call site via m.sessionsHelpKeymap()),
// so keymap_dispatch_guard_test — which probes the static descriptor with
// detection unwired — stays green.
//
// The footer case below is NOT the superseded "the footer never lists `m`
// (non-Core)" rule: §14.1 promotes `m` to Core, so the condensed Sessions footer
// DOES render `m multi` on a supported terminal, and §14.3 filters it through the
// SAME call-site slice as help. What this file keeps that the §14 tests do not
// reproduce is the per-terminal-identity matrix — supported, named-undriven, and
// the NULL/remote shape — asserted on every surface the filter reaches.
//
// No t.Parallel: consistent with the rest of the tui test surface (package-level
// mocks + shared canvas helpers).

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/spawn"
)

// multiSelectHelpLabel is the HelpAction label the `?` help body renders for the
// multi-select (`m`) row (sessionsKeymap()). Kept in one place so the render-level
// assertions cannot drift from the descriptor.
const multiSelectHelpLabel = "Multi-select mode"

// multiSelectFooterLabel is the CONDENSED footer label for that same `m` entry —
// §14.3 shortens it to `m multi` for the footer while the help body keeps the long
// form. Kept beside its help counterpart so the two surfaces' labels are asserted
// from one place.
const multiSelectFooterLabel = "m multi"

// keymapHasMKey reports whether the descriptor slice carries the `m` multi-select
// entry (keyed off keymapEntry.Key, a string glyph).
func keymapHasMKey(entries []keymapEntry) bool {
	for _, e := range entries {
		if e.Key == "m" {
			return true
		}
	}
	return false
}

// TestSessionsHelpKeymap_UnsupportedNotInMultiSelect_OmitsM covers §7 case (a):
// on a resolved-unsupported terminal (both the named com.apple.Terminal shape and
// the NULL/remote spawn.Identity{} shape), NOT in multi-select, the descriptor fed
// to the help modal omits `m` and the rendered help body omits the multi-select
// label. The predicate is DetectUnsupported() — identity-blind — so both shapes
// suppress `m`.
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

// TestSessionsHelpKeymap_Supported_ListsM covers §7 case (b): on a supported
// terminal (ghostty → native), DetectUnsupported() is false so the filter is
// inert — sessionsHelpKeymap() lists `m` and the rendered help body carries the
// multi-select label.
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

// TestSessionsHelpKeymap_UnsupportedInMultiSelect_ListsM covers §7 case (c): the
// A1 in-flight-entered state — detection resolves unsupported WHILE multi-select is
// already open. Here `m` is a live row-toggle, so the help never hides it. The
// !multiSelectMode leg of the predicate makes the filter inert. Both the named and
// NULL shapes list `m` in this state.
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
			// A1: multi-select was entered during the async in-flight window and is not
			// ejected when detection later resolves unsupported.
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

// TestSessionsFooter_ListsMultiOnlyWhenFunctional carries the §4 footer case
// forward onto §14's rule, which reverses it: §14.1 promotes `m` to Core so the
// condensed Sessions footer DOES list `m multi`, and §14.3 filters the footer
// through the SAME call-site slice `?` help reads — so the entry appears exactly
// where `m` is functional. Swept across all three host-terminal identities, which
// is the matrix this file contributes over the §14 lockstep tests (the NULL/remote
// shape in particular).
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
			// The fixture must genuinely land on the resolution the case names —
			// otherwise both legs would assert the same state and neither would bite.
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

// TestSessionsKeymap_StaticConstantUnaffectedByFilter guards §4's core constraint:
// the filter lives only in the call-site copy — sessionsKeymap() itself remains a
// pure static constant that always lists `m`, so the descriptor↔dispatch guard
// (which probes the static descriptor with detection unwired) stays green.
func TestSessionsKeymap_StaticConstantUnaffectedByFilter(t *testing.T) {
	if !keymapHasMKey(sessionsKeymap()) {
		t.Error("sessionsKeymap() must remain a pure static constant that always lists m — the filter belongs at the call site only")
	}
}
