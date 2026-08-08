package cmd

import (
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// themeEnumerator is the production tui.ThemeEnumerator: the panel's seam onto
// the theme union, closing over the process's theme.Loader and the resolved
// themes directory.
//
// It closes over the DIRECTORY for the reason the scrollback reader closes over
// the state directory: path resolution is cmd's (themesDirPath owns that chain),
// so the seam's methods take only the persisted keys, the panel holds no path
// policy, and a fixture needs none — which is what lets internal/capture fake the
// whole seam under its no-real-config import guard.
//
// It closes over the LOADER because a Loader owns the `theme` component's
// per-process dedup state: the panel's enumeration and the construction-time
// by-name read hit the same directory conditions, so they must share one dedup
// scope or a broken file WARNs twice per launch. Handing this the loader TUI
// construction already built is what makes that structural rather than remembered.
//
// It adds NO policy of its own. The union assembly, the ordering, the reasons and
// the `theme: enumerated` event all live in internal/theme, which is what keeps
// internal/tui from becoming another package emitting the component.
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
// It resolves the path and reads NOTHING: the cold path stays free of the N-file
// sweep, and the read happens on every panel open instead. A path that
// cannot be resolved at all degrades to the empty directory rather than blocking
// the launch — themesDirPath already answers "" on failure, so the discarded error
// IS the degradation, exactly as the construction-time resolution treats it: the
// embedded set is reachable with no path at all, so the panel still opens and
// still lists every built-in.
func newThemeEnumerator(loader theme.Loader) themeEnumerator {
	themesDir, _ := themesDirPath()
	return themeEnumerator{loader: loader, themesDir: themesDir}
}

// Open is the per-open read: one directory enumeration, producing the enumeration
// the panel retains for its lifetime and the finished union it lists.
func (e themeEnumerator) Open(keys theme.RawKeys) (theme.Enumeration, theme.Union) {
	return e.loader.Open(e.themesDir, keys)
}

// Reassemble re-derives the union from an already-retained enumeration and the
// current persisted keys, with NO fresh read — the post-commit recompute and the
// `Esc` re-resolution both re-run it against the same parse.
func (e themeEnumerator) Reassemble(enumeration theme.Enumeration, keys theme.RawKeys) theme.Union {
	return e.loader.Reassemble(enumeration, keys)
}

// Resolve re-runs construction's own per-slot resolution and mode-matched
// fallback against the retained enumeration — the SAME rules, reading the panel's
// parse instead of the directory, which supersedes what construction loaded.
//
// It is where construction's resolution rules reach the panel: the badge table,
// the theme applied on open and the row the cursor lands on all derive from this
// ONE answer, so none of the three can be the stale one.
//
// It takes no directory and reads none. The adapter still closes over one for
// Open, but THIS METHOD MUST NOT CONSULT IT: a read here would be a third parse
// of the same slug, neither construction's nor the panel's.
func (e themeEnumerator) Resolve(enumeration theme.Enumeration, setting theme.Setting) (theme.Resolution, error) {
	return e.loader.ResolveNominationFrom(enumeration, setting)
}

// ResolveSlot runs that same per-slot resolution for ONE slot against the same
// retained enumeration, at the commit-time cadence: the newly-live opposite slot
// on a constant → adaptive conversion, and the one `theme: loaded` that fires
// outside construction.
//
// It takes no directory and reads none, for Resolve's reason exactly.
func (e themeEnumerator) ResolveSlot(enumeration theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error) {
	return e.loader.ResolveSlot(enumeration, slot, slug)
}
