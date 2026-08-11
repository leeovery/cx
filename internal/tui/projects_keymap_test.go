package tui

import "testing"

func TestProjectsKeymap(t *testing.T) {
	entries := projectsKeymap()

	t.Run("it enumerates exactly the Projects bindings nav-first", func(t *testing.T) {
		want := []keymapEntry{
			{Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"},
			{Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"},
			{Key: "⏎", Action: "new session", HelpAction: "New session from project", Core: true},
			{Key: "x", Action: "sessions", HelpAction: "Switch to Sessions", Core: true},
			{Key: "e", Action: "edit", HelpAction: "Edit project", Core: true},
			{Key: "/", Action: "filter", HelpAction: "Filter projects", Core: true},
			{Key: "t", Action: "theme", HelpAction: "Theme picker", Core: true},
			{Key: "d", Action: "delete", HelpAction: "Delete project", Destructive: true},
			{Key: "n", Action: "new in cwd", HelpAction: "New session in cwd"},
			{Key: "q", Action: "quit", HelpAction: "Quit"},
			{Key: "esc", Action: "back", HelpAction: "Back / quit"},
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
		seen := map[string]bool{}
		for _, e := range entries {
			core[e.Key] = e.Core
			seen[e.Key] = true
		}
		wantCore := []string{"⏎", "x", "e", "/", "t", "?"}
		for _, k := range wantCore {
			if !seen[k] {
				t.Errorf("descriptor missing Core key %q", k)
			}
			if !core[k] {
				t.Errorf("key %q should be Core (footer), got Core=false", k)
			}
		}
		wantHelpOnly := []string{"d", "n", "↑↓", "^↑/↓", "q", "esc"}
		for _, k := range wantHelpOnly {
			if !seen[k] {
				t.Errorf("descriptor missing help-only key %q", k)
			}
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

	t.Run("it carries every help-only key so the ? help modal lists the complete keymap", func(t *testing.T) {
		have := map[string]string{}
		for _, e := range entries {
			have[e.Key] = e.Action
		}
		wantHelp := map[string]string{
			"d":    "delete",
			"n":    "new in cwd",
			"↑↓":   "navigate",
			"^↑/↓": "page",
			"q":    "quit",
			"esc":  "back",
		}
		for k, action := range wantHelp {
			got, ok := have[k]
			if !ok {
				t.Errorf("help-only key %q (%s) missing from the descriptor", k, action)
				continue
			}
			if got != action {
				t.Errorf("help-only key %q action = %q, want %q", k, got, action)
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
		wantCoreOrder := []string{"⏎", "x", "e", "/", "t", "?"}
		if len(coreKeys) != len(wantCoreOrder) {
			t.Fatalf("Core entries = %v, want %v", coreKeys, wantCoreOrder)
		}
		for i, k := range wantCoreOrder {
			if coreKeys[i] != k {
				t.Errorf("Core entry %d = %q, want %q (the row is pinned)", i, coreKeys[i], k)
			}
		}
	})

	t.Run("it has no uppercase or vim-alias key in the descriptor", func(t *testing.T) {
		banned := map[string]bool{
			"h": true, "j": true, "k": true, "l": true, "g": true, "G": true,
			"D": true, "E": true, "N": true, "Q": true, "S": true, "X": true,
			"s":    true,
			"pgup": true, "pgdown": true, "home": true, "end": true,
		}
		for _, e := range entries {
			if banned[e.Key] {
				t.Errorf("descriptor contains banned key %q (no s alias / no vim / no uppercase / no page-jump)", e.Key)
			}
		}
	})
}
