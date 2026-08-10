package cmd

import "github.com/leeovery/portal/internal/theme"

// newThemeSource binds the process's theme loader to the resolved themes
// directory, ONCE, at TUI construction.
//
// It resolves the path and reads NOTHING: the cold path stays free of the N-file
// sweep, and the read happens on every panel open instead. A path that
// cannot be resolved at all degrades to the empty directory rather than blocking
// the launch — themesDirPath already answers "" on failure, so the discarded error
// IS the degradation, exactly as the construction-time resolution treats it: the
// embedded set is reachable with no path at all, so the panel still opens and
// still lists every built-in.
//
// The loader is the one TUI construction already built rather than a fresh one,
// because a Loader owns the `theme` component's per-process dedup state: the
// panel's enumeration and the construction-time by-name read hit the same
// directory conditions, so they must share one dedup scope or a broken file WARNs
// twice per launch.
//
// The adapter itself belongs to internal/theme — resolving the path is all cmd
// contributes here, so wiring the panel composes the shared adapter rather than
// restating its delegation.
func newThemeSource(loader theme.Loader) theme.DirThemeSource {
	themesDir, _ := themesDirPath()
	return theme.DirThemeSource{Loader: loader, Dir: themesDir}
}
