package tui

import "github.com/leeovery/portal/internal/theme"

// The seed exists because a zero Theme fails silently — lipgloss.Color("") is
// the no-colour sentinel — and takes the silent loader: seeding is not a "use",
// so no `theme` records are written.
func defaultDarkTheme() theme.Theme {
	return loadBuiltinTheme(theme.NewSilentLoader(), theme.DefaultDarkSlug)
}

// Deliberately no fallback on a miss: that would mean a binary shipped with a
// broken default, and the per-slot fallback belongs to nomination resolution.
func loadBuiltinTheme(loader theme.Loader, slug string) theme.Theme {
	loaded, _, _ := loader.LoadBuiltin(slug)
	return loaded.Theme
}
