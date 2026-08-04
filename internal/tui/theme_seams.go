package tui

import "github.com/leeovery/portal/internal/theme"

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
// THE SHAPE IS NOT CLOSED AT TWO METHODS: task 8-8 extends it with a third,
// Resolve — the open-time re-resolution of the persisted setting against that
// same retained enumeration. An implementation designed as a final pair would
// have to be reopened there.
type ThemeEnumerator interface {
	Open(keys theme.RawKeys) (theme.Enumeration, theme.Union)
	Reassemble(e theme.Enumeration, keys theme.RawKeys) theme.Union
}
