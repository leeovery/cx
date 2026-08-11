package theme

// DirThemeSource is the theme panel's seam onto one themes directory. It closes
// over the directory — path resolution belongs to the caller that owns the
// config chain — and adds no policy of its own.
type DirThemeSource struct {
	Loader Loader

	// An empty Dir behaves as an absent directory: the panel still opens and
	// lists every built-in.
	Dir string
}

func (e DirThemeSource) Open(keys RawKeys) (Enumeration, Union) {
	return e.assembler().Open(e.Dir, keys)
}

func (e DirThemeSource) Reassemble(enumeration Enumeration, keys RawKeys) Union {
	return e.assembler().Reassemble(enumeration, keys)
}

// Resolve re-runs construction's per-slot resolution against the retained
// enumeration, so badges, the applied theme and the cursor row derive from one
// answer. It must not consult Dir: a read here would be another parse of the
// same slug, free to disagree.
func (e DirThemeSource) Resolve(enumeration Enumeration, keys RawKeys) (Resolution, error) {
	setting, _ := ResolveSetting(keys)
	return e.Loader.ResolveNominationFrom(enumeration, setting)
}

// LoadSlot runs the same resolution for one slot against the retained
// enumeration and records the load; a slot the user never set loads the shipped
// default rather than an empty slug. Only the error is returned: the palette
// belongs to the nomination Resolve hands back, and a second copy of it would be
// free to disagree. It reads no directory, for Resolve's reason.
func (e DirThemeSource) LoadSlot(enumeration Enumeration, slot Slot, keys RawKeys) error {
	_, err := e.Loader.ResolveSlot(enumeration, slot, SlugForSlot(keys, slot))
	return err
}

func (e DirThemeSource) assembler() Assembler {
	return Assembler{Loader: e.Loader}
}
