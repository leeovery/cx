package tui

import (
	"reflect"

	"github.com/leeovery/portal/internal/theme"
)

// ThemeEnumerator is the seam through which the theme panel gets its rows — the
// TmuxEnumerator / ScrollbackReader idiom the preview page already uses, applied
// to §9.4's union.
//
// It returns the FINISHED UNION, never a directory listing (§13.3). Open takes
// the raw persisted theme keys and hands back the complete row set — every
// built-in, every `.theme` file, and every persisted slug resolving to neither —
// already deduped one slug, one row, and already carrying each row's single §6.2
// reason. internal/theme owns that assembly, which keeps three decisions
// consistent: `theme: enumerated`'s count and rejected count are computable where
// they are emitted (§12.3), THIS PACKAGE NEVER BECOMES A FOURTH EMITTER of the
// `theme` component (§8.9 closes that set at three — loader, translation,
// persister), and the panel does no merging of its own.
//
// The themes DIRECTORY is deliberately absent from the signature, exactly as the
// state directory is absent from ScrollbackReader's: the production adapter
// closes over the resolved path (cmd/config.go owns that chain), so the panel
// holds no path policy and a fixture needs none. A fixture fakes a Union
// wholesale — it is an ordinary value — which is the only way internal/capture
// renders an invalid-theme row at all, its no-real-config import guard forbidding
// it a real themes directory.
//
// Reassemble is the re-derivation entry point, and it is a SEPARATE METHOD rather
// than a second Open because it must not re-read the directory: §9.2's
// post-commit recompute and §5.8's `Esc` re-resolution both re-derive from the
// enumeration Open retained, with changed persisted keys and no fresh I/O.
//
// Resolve is the RE-RESOLUTION of the persisted setting against that same
// retained enumeration, and it is what makes §5.8's "the panel's parse supersedes
// the construction-time parse" real: the panel opens on the theme the enumeration
// says is live rather than on the one construction happened to load, so a
// mid-session edit — including one that INVALIDATES the active theme — takes
// effect on open. It resolves against the retained parse and NEVER the
// filesystem, because a read here would produce a third parse of the same slug
// that can disagree with the row the user is looking at (§8.4). Its error is
// §7.6's fatal and the panel DEGRADES on it rather than escalating; see
// Model.applyThemePanelResolution, which states that policy once for all three
// panel call sites.
type ThemeEnumerator interface {
	Open(keys theme.RawKeys) (theme.Enumeration, theme.Union)
	Reassemble(e theme.Enumeration, keys theme.RawKeys) theme.Union
	Resolve(e theme.Enumeration, s theme.Setting) (theme.Resolution, error)
}

// liveThemeEnumerator reports whether e is a seam that can actually be CALLED, as
// against either shape of "nothing was wired".
//
// The nil INTERFACE is the ordinary unwired state and needs no explanation. The
// TYPED nil is the one worth the reflection: a `(*adapter)(nil)` boxed into this
// interface satisfies `!= nil`, so a plain nil check would store it and the model
// would hold a seam that looks live and panics on its first call — which, for a
// key bound on both pages, means the panic lands on an ordinary keypress rather
// than at the wiring site that caused it.
//
// The nil-able kinds are enumerated rather than blanket-calling IsNil, which
// panics on the kinds that cannot be nil (a struct value seam being the normal
// production and fixture shape). See WithThemeEnumerator, the single caller.
func liveThemeEnumerator(e ThemeEnumerator) bool {
	if e == nil {
		return false
	}
	switch v := reflect.ValueOf(e); v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return !v.IsNil()
	default:
		return true
	}
}
