package tui

// keymapEntry is one binding in a per-page keymap descriptor. Descriptors
// drive display only (footer = Core entries, help modal = all entries), not
// dispatch — a binding change must also be made in the per-page Update switch.
type keymapEntry struct {
	Key string
	// HelpKey is the help-modal key form; empty falls back to Key.
	HelpKey string
	Action  string
	// HelpAction is the help-modal label; empty falls back to Action.
	HelpAction string
	Core       bool
	// RightAligned pins the entry to the footer's right edge; at most one
	// entry sets it.
	RightAligned bool
	Destructive  bool
}

// Declared once so the arrow glyphs and their help labels cannot diverge
// between the descriptors that open with them.
func navKeymapEntries() []keymapEntry {
	return []keymapEntry{
		{Key: "↑↓", HelpKey: "↑/↓", Action: "navigate", HelpAction: "Move selection"},
		{Key: "^↑/↓", Action: "page", HelpAction: "Next / prev page"},
	}
}

// Stays a pure static function: the `t`/`m` blocked-key filters are applied
// at the call site (Model.sessionsHelpKeymap), never here, so the descriptor
// remains a complete statement of the page's bindings.
func sessionsKeymap() []keymapEntry {
	return append(navKeymapEntries(), []keymapEntry{
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
	}...)
}

// Stays a pure static function: the `t` blocked-key filter is applied at the
// call site (Model.projectsHelpKeymap), never here.
func projectsKeymap() []keymapEntry {
	return append(navKeymapEntries(), []keymapEntry{
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
	}...)
}

// The scope is complete — all six keys the panel dispatches, arrows and
// paging included. Do not trim it to the four Core keys the footer shows.
func themePanelKeymap() []keymapEntry {
	return append(navKeymapEntries(), []keymapEntry{
		{Key: "⏎", Action: "set theme", HelpAction: "Set as the theme", Core: true},
		{Key: "d", Action: "set as dark", HelpAction: "Assign to the dark slot", Core: true},
		{Key: "l", Action: "set as light", HelpAction: "Assign to the light slot", Core: true},
		{Key: "esc", Action: "close", HelpAction: "Close the theme picker", Core: true},
	}...)
}

// A nested scope, not a longer panel scope: while the confirm is live it is
// key-exclusive within the panel, so its footer replaces the standing four
// keys. The uppercase Y/N dispatch is deliberately not restated here.
func themePanelConfirmKeymap() []keymapEntry {
	return []keymapEntry{
		{Key: "y", Action: "confirm", HelpAction: "Clear the constant and set the slot", Core: true},
		{Key: "n", Action: "cancel", HelpAction: "Keep the constant", Core: true},
	}
}

// A footer-copy source, not a help reference — no Core/RightAligned flags.
func commandPendingKeymap() []keymapEntry {
	return []keymapEntry{
		{Key: "enter", HelpKey: "⏎", Action: "run here"},
		{Key: "n", Action: "run in cwd"},
		{Key: "esc", Action: "cancel"},
	}
}

// Pane cycling is Tab, not Ctrl+←/→ — macOS Mission Control hijacks Ctrl+←/→
// for Spaces switching.
func previewKeymap() []keymapEntry {
	return []keymapEntry{
		{Key: "↑/↓", Action: "scroll", HelpAction: "Scroll up / down"},
		{Key: "^↑/↓", Action: "page", HelpAction: "Page up / down"},
		{Key: "Home/End", Action: "top/bottom", HelpAction: "Jump to top / bottom"},
		{Key: "←→", Action: "window", HelpAction: "Prev / next window", Core: true},
		{Key: "⇥", Action: "pane", HelpAction: "Next pane", Core: true},
		{Key: "⏎", Action: "attach", HelpAction: "Attach this pane", Core: true},
		{Key: "␣", Action: "back", HelpAction: "Back to sessions", Core: true},
	}
}
