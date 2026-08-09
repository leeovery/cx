package tui

import "github.com/leeovery/portal/internal/theme"

// ThemeEnumerator is the seam through which the theme panel gets its rows — the
// TmuxEnumerator / ScrollbackReader idiom the preview page already uses, applied
// to the theme union.
//
// It returns the FINISHED UNION, never a directory listing. Open takes
// the raw persisted theme keys and hands back the complete row set — every
// built-in, every `.theme` file, and every persisted slug resolving to neither —
// already deduped one slug, one row, and already carrying each row's single
// reason. internal/theme owns that assembly, which keeps three decisions
// consistent: `theme: enumerated`'s count and rejected count are computable where
// they are emitted, THIS PACKAGE NEVER BECOMES ANOTHER EMITTER of the
// `theme` component (the emitters are the loader, the translation and
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
// than a second Open because it must not re-read the directory: the post-commit
// recompute and the `Esc` re-resolution both re-derive from the
// enumeration Open retained, with changed persisted keys and no fresh I/O.
//
// Resolve is the RE-RESOLUTION of the persisted setting against that same
// retained enumeration, and it is what makes "the panel's parse supersedes
// the construction-time parse" real: the panel opens on the theme the enumeration
// says is live rather than on the one construction happened to load, so a
// mid-session edit — including one that INVALIDATES the active theme — takes
// effect on open. It resolves against the retained parse and NEVER the
// filesystem, because a read here would produce a third parse of the same slug
// that can disagree with the row the user is looking at. Its error is the
// broken-builtin fatal and the panel DEGRADES on it rather than escalating; see
// Model.applyInForceTheme, which states that policy once for all three panel
// call sites.
//
// ResolveSlot is the COMMIT-TIME entry point — the one theme load outside
// construction — and it is a FOURTH METHOD rather than a parameter on Resolve
// because the two answer different questions at different cadences: Resolve
// re-resolves a setting construction already reported and emits no
// `theme: loaded`, while this resolves the ONE slot a constant → adaptive
// conversion just made live and emits the `theme: loaded` line for exactly that
// moment. It shares its rule body with Resolve inside internal/theme (the same
// charset check, the same embedded-set-first ordering, the same per-slot
// fallback, read off the SAME retained parse), so the badge path and the load path
// cannot disagree about a slug — and it reads no directory, for the reason Resolve
// does not.
type ThemeEnumerator interface {
	Open(keys theme.RawKeys) (theme.Enumeration, theme.Union)
	Reassemble(e theme.Enumeration, keys theme.RawKeys) theme.Union
	Resolve(e theme.Enumeration, s theme.Setting) (theme.Resolution, error)
	ResolveSlot(e theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error)
}
