package theme

// DirEnumerator is the theme panel's seam onto ONE themes directory: the row
// model the panel lists, and the resolution behind the theme it applies.
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
type DirEnumerator struct {
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
func (e DirEnumerator) Open(keys RawKeys) (Enumeration, Union) {
	return e.assembler().Open(e.Dir, keys)
}

// Reassemble re-derives the union from an already-retained enumeration and the
// current persisted keys, with NO fresh read — the post-commit recompute and the
// `Esc` re-resolution both re-run it against the same parse.
func (e DirEnumerator) Reassemble(enumeration Enumeration, keys RawKeys) Union {
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
// It takes no directory and MUST NOT CONSULT Dir: a read here would be a third
// parse of the same slug, neither construction's nor the panel's.
func (e DirEnumerator) Resolve(enumeration Enumeration, setting Setting) (Resolution, error) {
	return e.Loader.ResolveNominationFrom(enumeration, setting)
}

// ResolveSlot runs that same per-slot resolution for ONE slot against the same
// retained enumeration, at the commit-time cadence: the newly-live opposite slot
// on a constant → adaptive conversion, and the one `theme: loaded` that fires
// outside construction.
//
// It takes no directory and reads none, for Resolve's reason exactly.
func (e DirEnumerator) ResolveSlot(enumeration Enumeration, slot Slot, slug string) (SlotResolution, error) {
	return e.Loader.ResolveSlot(enumeration, slot, slug)
}

// assembler is the row-model half of the pair, over this seam's loader — built
// per call because it is a value over that one loader and holds nothing else.
func (e DirEnumerator) assembler() Assembler {
	return Assembler{Loader: e.Loader}
}
