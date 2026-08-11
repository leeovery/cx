package cmd

import "github.com/leeovery/portal/internal/theme"

// The path-resolution error is discarded deliberately: "" degrades to the
// embedded set alone rather than blocking the launch. The loader must be the
// construction-time instance — a Loader owns the `theme` component's WARN
// dedup, so a fresh one here would WARN twice per launch.
func newThemeSource(loader theme.Loader) theme.DirThemeSource {
	themesDir, _ := themesDirPath()
	return theme.DirThemeSource{Loader: loader, Dir: themesDir}
}
