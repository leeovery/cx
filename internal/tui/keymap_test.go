package tui

import "testing"

func TestSessionsKeymap(t *testing.T) {
	entries := sessionsKeymap()

	t.Run("it enumerates exactly the Sessions bindings in the reference help order", func(t *testing.T) {
		want := []keymapEntry{
			{Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"},
			{Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"},
			{Key: "⏎", HelpKey: "⏎", Action: "attach", HelpAction: "Open / attach session", Core: true},
			{Key: "/", Action: "filter", HelpAction: "Filter sessions", Core: true},
			{Key: "␣", HelpKey: "␣", Action: "preview", HelpAction: "Preview scrollback", Core: true},
			{Key: "s", Action: "switch view", HelpAction: "Switch view — flat / project / tag", Core: true},
			{Key: "n", Action: "new in cwd", HelpAction: "New session in cwd"},
			{Key: "r", Action: "rename", HelpAction: "Rename session"},
			{Key: "k", Action: "kill", HelpAction: "Kill session", Destructive: true},
			{Key: "q", Action: "quit", HelpAction: "Quit"},
			{Key: "x", Action: "projects", HelpAction: "Switch to Projects", Core: true},
			{Key: "t", Action: "theme", HelpAction: "Theme picker", Core: true},
			{Key: "m", Action: "multi", HelpAction: "Multi-select mode", Core: true},
			{Key: "?", Action: "help", HelpAction: "Show this help", Core: true, RightAligned: true},
		}
		if len(entries) != len(want) {
			t.Fatalf("descriptor has %d entries, want %d: %+v", len(entries), len(want), entries)
		}
		for i, w := range want {
			if entries[i] != w {
				t.Errorf("entry[%d] = %+v, want %+v", i, entries[i], w)
			}
		}
	})

	t.Run("it marks the core-footer keys as Core and the rest as help-only", func(t *testing.T) {
		core := map[string]bool{}
		for _, e := range entries {
			core[e.Key] = e.Core
		}
		wantCore := []string{"⏎", "/", "␣", "s", "x", "t", "m", "?"}
		for _, k := range wantCore {
			if !core[k] {
				t.Errorf("key %q should be Core (footer), got Core=false", k)
			}
		}
		wantHelpOnly := []string{"↑↓", "n", "r", "k", "q", "^↑/↓"}
		for _, k := range wantHelpOnly {
			if core[k] {
				t.Errorf("key %q should be help-only (Core=false), got Core=true", k)
			}
		}
	})

	t.Run("it marks only ? help as right-aligned", func(t *testing.T) {
		for _, e := range entries {
			wantRight := e.Key == "?"
			if e.RightAligned != wantRight {
				t.Errorf("entry %q RightAligned = %v, want %v", e.Key, e.RightAligned, wantRight)
			}
		}
	})

	t.Run("it carries the Core relative order the footer reads", func(t *testing.T) {
		var coreKeys []string
		for _, e := range entries {
			if e.Core {
				coreKeys = append(coreKeys, e.Key)
			}
		}
		wantCoreOrder := []string{"⏎", "/", "␣", "s", "x", "t", "m", "?"}
		if len(coreKeys) != len(wantCoreOrder) {
			t.Fatalf("Core entries = %v, want %v", coreKeys, wantCoreOrder)
		}
		for i, k := range wantCoreOrder {
			if coreKeys[i] != k {
				t.Errorf("Core entry %d = %q, want %q (the row is pinned)", i, coreKeys[i], k)
			}
		}
	})

	t.Run("the help body keeps the slashed nav via HelpKey while page reads Key directly", func(t *testing.T) {
		wantHelpKey := map[string]string{"↑↓": "↑/↓", "⏎": "⏎", "␣": "␣"}
		for _, e := range entries {
			if want, ok := wantHelpKey[e.Key]; ok {
				if e.HelpKey != want {
					t.Errorf("%s HelpKey = %q, want %q", e.Key, e.HelpKey, want)
				}
				continue
			}
			if e.HelpKey != "" {
				t.Errorf("key %q must have NO HelpKey override (got %q)", e.Key, e.HelpKey)
			}
		}
	})

	t.Run("it has no uppercase or vim-alias key in the descriptor", func(t *testing.T) {
		banned := map[string]bool{
			"h": true, "j": true, "l": true, "g": true, "G": true,
			"K": true, "N": true, "R": true, "Q": true, "S": true, "X": true,
			"pgup": true, "pgdown": true, "home": true, "end": true,
		}
		for _, e := range entries {
			if banned[e.Key] {
				t.Errorf("descriptor contains banned key %q (no vim/uppercase/page-jump aliases)", e.Key)
			}
		}
	})
}

func TestNavKeymapEntries(t *testing.T) {
	shared := navKeymapEntries()

	t.Run("it declares the non-core nav and page rows in that order", func(t *testing.T) {
		want := []keymapEntry{
			{Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"},
			{Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"},
		}
		if len(shared) != len(want) {
			t.Fatalf("shared nav entries = %d, want %d: %+v", len(shared), len(want), shared)
		}
		for i, w := range want {
			if shared[i] != w {
				t.Errorf("entry[%d] = %+v, want %+v", i, shared[i], w)
			}
		}
	})

	t.Run("every list descriptor leads with the shared pair", func(t *testing.T) {
		descriptors := map[string][]keymapEntry{
			"sessionsKeymap":   sessionsKeymap(),
			"projectsKeymap":   projectsKeymap(),
			"themePanelKeymap": themePanelKeymap(),
		}
		for name, entries := range descriptors {
			if len(entries) < len(shared) {
				t.Fatalf("%s has %d entries, want at least %d", name, len(entries), len(shared))
			}
			for i, w := range shared {
				if entries[i] != w {
					t.Errorf("%s entry[%d] = %+v, want the shared %+v", name, i, entries[i], w)
				}
			}
		}
	})
}
