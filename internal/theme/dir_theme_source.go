package theme

// DirThemeSource is the theme panel's seam onto ONE themes directory: it
// assembles the finished union the panel lists, and it resolves the persisted
// setting against that retained enumeration — the theme the panel applies, the
// badges it marks and the commit-time per-slot load.
//
// It closes over the DIRECTORY, so its methods take only the persisted keys and
// the enumeration the panel retained. Path resolution belongs to the caller that
// owns the config chain, which is what keeps path policy out of the render layer
// and lets a fixture answer the same four methods with no filesystem at all.
//
// It is EXPORTED so every caller that drives the panel against a real directory
// reaches the same four methods. A hand-written re-implementation of this
// delegation is what lets a seam and the production wiring drift apart while both
// still compile.
//
// It adds NO policy of its own. The enumeration, the union assembly, the
// ordering, the reasons and the `theme` events all belong to the two values it
// composes, so nothing downstream of this seam becomes a second place any of
// them is decided.
type DirThemeSource struct {
	// Loader parses and resolves for both halves of the seam. Holding ONE
	// instance is what keeps a launch to one `theme` dedup scope: the panel's
	// enumeration and the by-name read that resolved the theme at construction
	// meet the same directory conditions, so a second loader would report one
	// misconfiguration twice.
	Loader Loader

	// Dir is the resolved themes directory, re-read by every Open. The empty
	// string is the unresolvable-path degradation and behaves as an absent
	// directory: the embedded set needs no path, so the panel still opens and
	// still lists every built-in.
	Dir string
}

// Open is the per-open read: one directory enumeration, producing the
// enumeration the panel retains for its lifetime and the finished union it
// lists.
func (e DirThemeSource) Open(keys RawKeys) (Enumeration, Union) {
	return e.assembler().Open(e.Dir, keys)
}

// Reassemble re-derives the union from an already-retained enumeration and the
// current persisted keys, with NO fresh read — the post-commit recompute and the
// `Esc` re-resolution both re-run it against the same parse.
func (e DirThemeSource) Reassemble(enumeration Enumeration, keys RawKeys) Union {
	return e.assembler().Reassemble(enumeration, keys)
}

// Resolve re-runs construction's own per-slot resolution and mode-matched
// fallback against the retained enumeration — the SAME rules, reading the panel's
// parse instead of the directory, which supersedes what construction loaded.
//
// It is where construction's resolution rules reach the panel: the badge table,
// the theme applied on open and the row the cursor lands on all derive from this
// ONE answer, so none of the three can be the stale one.
//
// It takes the persisted keys and collapses them here, as Reassemble does: the
// tiebreak and the shipped-default substitution are this package's rules, so a
// caller cannot resolve against a setting derived differently from the one it
// lists and marks.
//
// It takes no directory and MUST NOT CONSULT Dir: a read here would be a third
// parse of the same slug, neither construction's nor the panel's.
func (e DirThemeSource) Resolve(enumeration Enumeration, keys RawKeys) (Resolution, error) {
	setting, _ := ResolveSetting(keys)
	return e.Loader.ResolveNominationFrom(enumeration, setting)
}

// ResolveSlot runs that same per-slot resolution for ONE slot against the same
// retained enumeration, at the commit-time cadence: the newly-live opposite slot
// on a constant → adaptive conversion, and the one `theme: loaded` that fires
// outside construction.
//
// The slug is taken from the same collapse Resolve runs, through Setting.Slug, so
// a slot the user never set resolves the shipped default rather than an empty
// slug reported as a fallback of a name nobody chose.
//
// It takes no directory and reads none, for Resolve's reason exactly.
func (e DirThemeSource) ResolveSlot(enumeration Enumeration, slot Slot, keys RawKeys) (SlotResolution, error) {
	setting, _ := ResolveSetting(keys)
	return e.Loader.ResolveSlot(enumeration, slot, setting.Slug(slot))
}

// assembler is the row-model half of the pair, over this seam's loader — built
// per call because it is a value over that one loader and holds nothing else.
func (e DirThemeSource) assembler() Assembler {
	return Assembler{Loader: e.Loader}
}
