package tui

import (
	"strings"
	"testing"
)

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
				t.Errorf("panel scope carries a %q entry — ? does nothing inside the panel", e.Key)
			}
		}
	})
}

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
				t.Errorf("key %q should be non-core (arrows in a list are a given), got Core=true", k)
			}
		}
	})
}

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
		panelFooter := footerVisible(renderThemePanelFooter(themePanelKeymap(), themePanelFooterTestWidth, th, false))
		for _, e := range panelCore {
			if !strings.Contains(panelFooter, e.Action) {
				t.Errorf("panel footer does not carry %q, so the containment assertions guard nothing:\n%s", e.Action, panelFooter)
			}
		}
	})
}

func TestThemeConfirmKeymap_DoesNotLeakIntoPageSurfaces(t *testing.T) {
	th := testDarkTheme(t)
	confirm := themePanelConfirmKeymap()

	t.Run("it pins no right anchor and no ? entry", func(t *testing.T) {
		for _, e := range confirm {
			if e.RightAligned {
				t.Errorf("entry %q is RightAligned — a vertical footer has no right anchor", e.Key)
			}
			if e.Key == "?" {
				t.Errorf("confirm scope carries a %q entry — ? does nothing inside the panel", e.Key)
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
			visible := themePanelFooterCopy(footerVisible(surface))
			for _, needle := range needles {
				if strings.Contains(visible, needle) {
					t.Errorf("%s carries the confirm-scope copy %q:\n%s", name, needle, visible)
				}
			}
		}
	})

	t.Run("the substituted footer is the scope's only consumer", func(t *testing.T) {
		substituted := themePanelFooterCopy(footerVisible(renderThemePanelFooter(confirm, themePanelFooterTestWidth, th, false)))
		for _, e := range confirm {
			if want := helpKeyGlyph(e) + " " + e.Action; !strings.Contains(substituted, want) {
				t.Errorf("the substituted footer does not carry %q, so the containment assertions guard nothing:\n%s", want, substituted)
			}
		}
	})
}
