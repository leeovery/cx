package tui

import "github.com/leeovery/portal/internal/theme"

// defaultDarkTheme is the shipped dark built-in — the palette every directly
// constructed model and preview SEEDS itself with, before any nomination is
// applied.
//
// The seed exists because a zero Theme is not a loud failure: lipgloss.Color("")
// yields the no-colour sentinel, so an unseeded model renders a silently
// colourless frame with no compile error and no failing assertion anywhere. A
// model built without Build — a struct-literal test model, a preview — is
// therefore themed from the moment it exists, and applying a nomination
// overwrites it.
//
// It resolves NO PATH and reads no config: LoadBuiltin serves the embedded set,
// which is what keeps TUI construction free of config discovery (and keeps the
// offline capture harness reachable under its no-real-config import guard).
func defaultDarkTheme() theme.Theme {
	return loadBuiltinTheme(theme.NewLoader(nil), theme.DefaultDarkSlug)
}

// loadBuiltinTheme loads one built-in by slug, returning the zero Theme if the
// embedded set somehow does not carry it.
//
// There is deliberately NO fallback beneath it. §7.6 makes a broken built-in
// impossible at BUILD time rather than handled at runtime — the embedded set is
// parsed and validated by a unit test, and the two default slugs are asserted to
// resolve within it — so a rejection here would mean a binary shipped with a
// broken default. §8.5's per-slot fallback belongs to the nomination's RESOLUTION
// (Phase 5), not to this seed; inventing an interim fallback here would be a
// second, unspecified resolution policy.
func loadBuiltinTheme(loader theme.Loader, slug string) theme.Theme {
	loaded, _, _ := loader.LoadBuiltin(slug)
	return loaded.Theme
}
