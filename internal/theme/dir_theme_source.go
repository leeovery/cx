package theme

// DirThemeSource is the theme panel's seam onto one themes directory. It
// closes over the directory — path resolution belongs to the caller that owns
// the config chain — and adds no policy of its own.
type DirThemeSource struct {
	// Holding one Loader keeps a launch to one `theme` dedup scope; a second
	// loader would report one misconfiguration twice.
	Loader Loader

	// Dir empty behaves as an absent directory: the panel still opens and
	// lists every built-in.
	Dir string
}

// Open is the per-open read: one directory enumeration, producing the
// enumeration the panel retains and the union it lists.
func (e DirThemeSource) Open(keys RawKeys) (Enumeration, Union) {
	return e.assembler().Open(e.Dir, keys)
}

// Reassemble re-derives the union from an already-retained enumeration and the
// current persisted keys, with no fresh directory read.
func (e DirThemeSource) Reassemble(enumeration Enumeration, keys RawKeys) Union {
	return e.assembler().Reassemble(enumeration, keys)
}

// Resolve re-runs construction's per-slot resolution against the retained
// enumeration, so badges, the applied theme and the cursor row derive from
// one answer. It must not consult Dir: a read here would be a third parse of
// the same slug.
func (e DirThemeSource) Resolve(enumeration Enumeration, keys RawKeys) (Resolution, error) {
	setting, _ := ResolveSetting(keys)
	return e.Loader.ResolveNominationFrom(enumeration, setting)
}

// ResolveSlot runs the same per-slot resolution for one slot against the
// retained enumeration; a slot the user never set resolves the shipped
// default rather than an empty slug. It reads no directory, for Resolve's
// reason.
func (e DirThemeSource) ResolveSlot(enumeration Enumeration, slot Slot, keys RawKeys) (SlotResolution, error) {
	setting, _ := ResolveSetting(keys)
	return e.Loader.ResolveSlot(enumeration, slot, setting.Slug(slot))
}

func (e DirThemeSource) assembler() Assembler {
	return Assembler{Loader: e.Loader}
}
