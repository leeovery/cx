package tui

import "github.com/leeovery/portal/internal/theme"

// A zero Theme fails silently (lipgloss.Color("") is the no-colour sentinel), so
// the model needs a seed. Seeding is not a "use", hence the silent loader.
// Deliberately no fallback on a miss: that would mean a binary shipped with a
// broken default, and the per-slot fallback belongs to nomination resolution.
func defaultDarkTheme() theme.Theme {
	loaded, _, _ := theme.NewSilentLoader().LoadBuiltin(theme.DefaultDarkSlug)
	return loaded.Theme
}
