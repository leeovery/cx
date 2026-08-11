package tui

import "github.com/leeovery/portal/internal/theme"

// A zero Theme fails silently (lipgloss.Color("") is the no-colour sentinel), so
// the model needs a seed. Seeding is not a "use", hence the silent loader.
func defaultDarkTheme() theme.Theme {
	return loadBuiltinTheme(theme.NewSilentLoader(), theme.DefaultDarkSlug)
}

// Deliberately no fallback on a miss: that would mean a binary shipped with a
// broken default, and the per-slot fallback belongs to nomination resolution.
func loadBuiltinTheme(loader theme.Loader, slug string) theme.Theme {
	loaded, _, _ := loader.LoadBuiltin(slug)
	return loaded.Theme
}
