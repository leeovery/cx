package tui

import (
	"strings"
	"testing"
)

// The descriptor-governed keymap rule panel-scope gate. These tests lock the theme panel's
// keymap scope as a COMPLETE six-entry descriptor — the four commits marked Core (what the
// vertical footer renders) plus arrows and paging as non-core (what makes the scope complete
// for the dispatch guard) — and prove the scope does not leak into either
// main-screen footer or either page's help body.
//
// Pure data, no rendering (mirrors TestSessionsKeymap / TestProjectsKeymap); the
// footer that consumes it is gated by theme_panel_footer_test.go.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// TestThemePanelKeymap_CarriesAllSixKeys locks the descriptor-governed keymap's panel scope:
// ALL SIX keys the panel dispatches — `↑↓`, `^↑/↓`, `⏎`, `d`, `l`, `esc` — in the declared
// order, with the pinned copy on the four commits.
//
// Completeness is the point rather than a nicety: `keymap_dispatch_guard_test`'s
// contract is descriptor↔dispatch PARITY (which extends to this scope), so a
// scope authored as just the four visible keys is precisely what BREAKS the guard.
// The scope also carries NO RightAligned entry (a vertical footer has no right
// anchor) and NO `?` entry (`?` does nothing inside the panel — there is no
// panel help modal).
func TestThemePanelKeymap_CarriesAllSixKeys(t *testing.T) {
	entries := themePanelKeymap()

	t.Run("it carries all six panel keys", func(t *testing.T) {
		want := []keymapEntry{
			{Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"},
			{Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"},
			{Key: "⏎", Action: "set theme", HelpAction: "Set as the theme", Core: true},
			{Key: "d", Action: "set as dark", HelpAction: "Assign to the dark slot", Core: true},
			{Key: "l", Action: "set as light", HelpAction: "Assign to the light slot", Core: true},
			{Key: "esc", Action: "close", HelpAction: "Close the theme picker", Core: true},
		}
		if len(entries) != len(want) {
			t.Fatalf("panel scope has %d entries, want %d: %+v", len(entries), len(want), entries)
		}
		for i, w := range want {
			if entries[i] != w {
				t.Errorf("entry[%d] = %+v, want %+v", i, entries[i], w)
			}
		}
	})

	t.Run("it pins no right anchor and no ? entry", func(t *testing.T) {
		for _, e := range entries {
			if e.RightAligned {
				t.Errorf("entry %q is RightAligned — a vertical footer has no right anchor", e.Key)
			}
			if e.Key == "?" {
				t.Errorf("panel scope carries a %q entry — §9.12: ? does nothing inside the panel", e.Key)
			}
		}
	})
}

// TestThemePanelKeymap_CoreIsTheFourCommits pins the descriptor-governed keymap's Core split
// from the other side: Core is EXACTLY the four commit/close keys the vertical footer
// renders, and arrows and paging are non-core.
//
// It is the invariant the footer's four pinned rows rest on. Marking a fifth entry
// Core would silently grow the pinned copy's four-row footer (and the panel's height floor
// with it); un-marking one would drop a commit the user has no other affordance
// for, since there is no panel help modal to recover it from.
func TestThemePanelKeymap_CoreIsTheFourCommits(t *testing.T) {
	core := map[string]bool{}
	for _, e := range themePanelKeymap() {
		core[e.Key] = e.Core
	}

	t.Run("it marks exactly the four commits core", func(t *testing.T) {
		for _, k := range []string{"⏎", "d", "l", "esc"} {
			if !core[k] {
				t.Errorf("key %q should be Core (the footer renders it), got Core=false", k)
			}
		}
		for _, k := range []string{"↑↓", "^↑/↓"} {
			if core[k] {
				t.Errorf("key %q should be non-core (§14.1: arrows in a list are a given), got Core=true", k)
			}
		}
	})
}

// TestThemePanelKeymap_DoesNotLeakIntoPageSurfaces proves containment: the panel
// scope reaches the panel's own vertical footer and NOTHING else. Neither
// main-screen footer nor either page's help body gains an entry (the panel
// is a scope beside the page descriptors, not an addition to them).
//
// The two page descriptors are pinned entry-by-entry by TestSessionsKeymap and
// TestProjectsKeymap, which this task leaves untouched; what those cannot see is
// the RENDERED direction, so this test walks the four page surfaces the scope
// could leak into and asserts none of the pinned copy's panel copy appears in any of them.
func TestThemePanelKeymap_DoesNotLeakIntoPageSurfaces(t *testing.T) {
	th := testDarkTheme(t)
	panelCore := coreEntriesOf(themePanelKeymap())

	pageDescriptors := map[string][]keymapEntry{
		"sessionsKeymap": sessionsKeymap(),
		"projectsKeymap": projectsKeymap(),
	}
	pageSurfaces := map[string]string{
		"Sessions footer":    renderSessionsFooter(sessionsKeymap(), referenceFooterWidth, th, false),
		"Projects footer":    renderProjectsFooter(projectsKeymap(), referenceFooterWidth, th, false),
		"Sessions help body": helpModalBody(sessionsKeymap(), th, false),
		"Projects help body": helpModalBody(projectsKeymap(), th, false),
	}

	t.Run("it leaves the page keymaps untouched", func(t *testing.T) {
		for name, entries := range pageDescriptors {
			for _, page := range entries {
				for _, panel := range panelCore {
					if page.Action == panel.Action || page.HelpAction == panel.HelpAction {
						t.Errorf("%s carries the panel entry %+v — the panel scope is a scope beside the page descriptors, not an addition to them", name, page)
					}
				}
			}
		}
	})

	t.Run("no panel copy reaches either footer or either help body", func(t *testing.T) {
		// The pinned copy's three commit strings verbatim, plus each Core entry's help phrase
		// and its rendered `<glyph> <label>` row — the forms the copy could arrive in.
		needles := []string{"set theme", "set as dark", "set as light"}
		for _, e := range panelCore {
			needles = append(needles, e.HelpAction, helpKeyGlyph(e)+" "+e.Action)
		}
		for name, surface := range pageSurfaces {
			visible := footerVisible(surface)
			for _, needle := range needles {
				if strings.Contains(visible, needle) {
					t.Errorf("%s carries the panel-scope copy %q:\n%s", name, needle, visible)
				}
			}
		}
	})

	t.Run("the panel footer is the scope's only consumer", func(t *testing.T) {
		// The positive half of the same claim: the copy the page surfaces must NOT
		// carry is exactly what the panel's own footer DOES carry, so the assertions
		// above are containment rather than a check against a string nothing renders.
		panelFooter := footerVisible(renderThemePanelFooter(themePanelKeymap(), themePanelFooterTestWidth, th, false))
		for _, e := range panelCore {
			if !strings.Contains(panelFooter, e.Action) {
				t.Errorf("panel footer does not carry %q, so the containment assertions guard nothing:\n%s", e.Action, panelFooter)
			}
		}
	})
}

// TestThemeConfirmKeymap_DoesNotLeakIntoPageSurfaces: its scope does not leak.
//
// The picker idiom's nested confirm scope is a SECOND scope beneath the panel's, not a longer
// first one, so its containment is a claim of its own: `y confirm` / `n cancel`
// reach the panel's SUBSTITUTED footer and nothing else. Neither main-screen footer
// nor either page's help body gains an entry, and — the half the panel scope's own
// test cannot make — neither does the panel's STANDING footer, which is the surface
// the confirm's rows temporarily replace rather than join.
//
// It also pins the two structural absences the descriptor-governed keymap rule's reasoning
// gives the panel scope and which apply here verbatim: no RightAligned entry, because a
// VERTICAL footer has no right anchor, and no `?` entry, because `?` does nothing inside the
// panel.
func TestThemeConfirmKeymap_DoesNotLeakIntoPageSurfaces(t *testing.T) {
	th := testDarkTheme(t)
	confirm := themePanelConfirmKeymap()

	t.Run("it pins no right anchor and no ? entry", func(t *testing.T) {
		for _, e := range confirm {
			if e.RightAligned {
				t.Errorf("entry %q is RightAligned — a vertical footer has no right anchor", e.Key)
			}
			if e.Key == "?" {
				t.Errorf("confirm scope carries a %q entry — §9.12: ? does nothing inside the panel", e.Key)
			}
			if !e.Core {
				t.Errorf("entry %q is non-core — both confirm keys are what the substituted footer renders", e.Key)
			}
		}
	})

	t.Run("no confirm copy reaches a page surface or the standing footer", func(t *testing.T) {
		surfaces := map[string]string{
			"Sessions footer":       renderSessionsFooter(sessionsKeymap(), referenceFooterWidth, th, false),
			"Projects footer":       renderProjectsFooter(projectsKeymap(), referenceFooterWidth, th, false),
			"Sessions help body":    helpModalBody(sessionsKeymap(), th, false),
			"Projects help body":    helpModalBody(projectsKeymap(), th, false),
			"panel standing footer": renderThemePanelFooter(themePanelKeymap(), themePanelFooterTestWidth, th, false),
		}
		needles := []string{}
		for _, e := range confirm {
			needles = append(needles, e.HelpAction, helpKeyGlyph(e)+" "+e.Action)
		}
		for name, surface := range surfaces {
			// The surfaces are read in their `<glyph> <label>` form, with the vertical
			// footer's fixed key column collapsed — so a leak into a padded row reads the
			// same as a leak into a horizontal one and neither can hide behind whitespace.
			visible := themePanelFooterCopy(footerVisible(surface))
			for _, needle := range needles {
				if strings.Contains(visible, needle) {
					t.Errorf("%s carries the confirm-scope copy %q:\n%s", name, needle, visible)
				}
			}
		}
	})

	t.Run("the substituted footer is the scope's only consumer", func(t *testing.T) {
		// The positive half: the copy every surface above must NOT carry is exactly
		// what the panel's footer DOES carry once the scope is substituted into it, so
		// the containment above guards something that renders.
		substituted := themePanelFooterCopy(footerVisible(renderThemePanelFooter(confirm, themePanelFooterTestWidth, th, false)))
		for _, e := range confirm {
			if want := helpKeyGlyph(e) + " " + e.Action; !strings.Contains(substituted, want) {
				t.Errorf("the substituted footer does not carry %q, so the containment assertions guard nothing:\n%s", want, substituted)
			}
		}
	})
}
