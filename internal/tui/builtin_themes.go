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
//
// It writes no `theme` records, which is why it takes the silent loader: the
// component records where a theme is USED, and priming a model with the shipped
// palette before any nomination is applied is a seed rather than a use.
func defaultDarkTheme() theme.Theme {
	return loadBuiltinTheme(theme.NewSilentLoader(), theme.DefaultDarkSlug)
}

// loadBuiltinTheme loads one built-in by slug, returning the zero Theme if the
// embedded set somehow does not carry it.
//
// There is deliberately NO fallback beneath it. A broken built-in is
// impossible at BUILD time rather than handled at runtime — the embedded set is
// parsed and validated by a unit test, and the two default slugs are asserted to
// resolve within it — so a rejection here would mean a binary shipped with a
// broken default. The per-slot fallback belongs to the nomination's RESOLUTION,
// not to this seed; inventing an interim fallback here would be a
// second, unspecified resolution policy.
func loadBuiltinTheme(loader theme.Loader, slug string) theme.Theme {
	loaded, _, _ := loader.LoadBuiltin(slug)
	return loaded.Theme
}
