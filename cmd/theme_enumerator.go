package cmd

import (
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// themeEnumerator is the production tui.ThemeEnumerator (§13.3): the panel's
// seam onto §9.4's union, closing over the process's theme.Loader and the
// resolved themes directory.
//
// It closes over the DIRECTORY for the reason the scrollback reader closes over
// the state directory: path resolution is cmd's (themesDirPath owns that chain),
// so the seam's methods take only the persisted keys, the panel holds no path
// policy, and a fixture needs none — which is what lets internal/capture fake the
// whole seam under its no-real-config import guard.
//
// It closes over the LOADER because a Loader owns the `theme` component's
// per-process dedup state: the panel's enumeration and the construction-time
// by-name read hit the same conditions (§5.5), so they must share one dedup scope
// or a broken file WARNs twice per launch. Handing this the loader TUI
// construction already built is what makes that structural rather than remembered.
//
// It adds NO policy of its own. The union assembly, the ordering, the reasons and
// §12.3's `theme: enumerated` all live in internal/theme, which is what keeps
// internal/tui from becoming a fourth package emitting the component (§8.9 closes
// that set at three).
type themeEnumerator struct {
	loader    theme.Loader
	themesDir string
}

// Satisfying the seam is the whole point of the type, so a signature drift is a
// compile error here rather than a nil enumerator at the wiring site.
var _ tui.ThemeEnumerator = themeEnumerator{}

// newThemeEnumerator binds the process's theme loader to the resolved themes
// directory, ONCE, at TUI construction.
//
// It resolves the path and reads NOTHING: §5.7 keeps the cold path free of the
// N-file sweep, and §5.8 puts the read on every panel open instead. A path that
// cannot be resolved at all degrades to the empty directory rather than blocking
// the launch — themesDirPath already answers "" on failure, so the discarded error
// IS the degradation, exactly as the construction-time resolution treats it: the
// embedded set is reachable with no path at all, so the panel still opens and
// still lists every built-in.
func newThemeEnumerator(loader theme.Loader) themeEnumerator {
	themesDir, _ := themesDirPath()
	return themeEnumerator{loader: loader, themesDir: themesDir}
}

// Open is §5.8's per-open read: one directory enumeration, producing the
// enumeration the panel retains for its lifetime and the finished union it lists.
func (e themeEnumerator) Open(keys theme.RawKeys) (theme.Enumeration, theme.Union) {
	return e.loader.Open(e.themesDir, keys)
}

// Reassemble re-derives the union from an already-retained enumeration and the
// current persisted keys, with NO fresh read — §9.2's post-commit recompute and
// §5.8's `Esc` re-resolution both re-run it against the same parse.
func (e themeEnumerator) Reassemble(enumeration theme.Enumeration, keys theme.RawKeys) theme.Union {
	return e.loader.Reassemble(enumeration, keys)
}
