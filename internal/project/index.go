package project

// Index maps a directory's canonical key to the Project stored there,
// canonicalising each stored path once instead of paying an EvalSymlinks syscall
// per project per lookup.
//
// It is a derived cache: rebuild it whenever the project set changes or lookups
// go stale. The zero value is not usable; construct via NewIndex.
type Index struct {
	byKey map[string]Project
}

// NewIndex builds an Index from projects. An empty slice yields a usable Index
// whose Match always reports not-found. Two projects reducing to the same
// canonical key collide last-write-wins.
func NewIndex(projects []Project) Index {
	byKey := make(map[string]Project, len(projects))
	for _, p := range projects {
		byKey[CanonicalDirKey(p.Path)] = p
	}
	return Index{byKey: byKey}
}

// Match finds the Project whose directory matches dirPath. The returned key is
// CanonicalDirKey(dirPath) whether or not a project matched, so a caller needing
// the canonical key reuses this computation rather than paying a second
// EvalSymlinks.
//
// An empty dirPath canonicalises to the working directory, which no real project
// key collides with, so it reports not-found.
func (idx Index) Match(dirPath string) (Project, string, bool) {
	key := CanonicalDirKey(dirPath)
	p, ok := idx.byKey[key]
	return p, key, ok
}
