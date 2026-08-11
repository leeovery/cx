package theme

import (
	"embed"
	"slices"
	"strings"
)

// These are both the shipped adaptive pair and the per-slot mode-matched
// fallbacks (an unloadable constant `theme` falls to DefaultDarkSlug). Both must
// name a theme in the embedded set, or every fallback becomes unresolvable.
const (
	DefaultDarkSlug  = "tokyo-night"
	DefaultLightSlug = "tokyo-night-day"
)

const builtinDir = "builtins"

// The built-ins are .theme files parsed by the same loader as a drop-in, so
// adding a theme is adding a file. Embedding reads no path, env var or
// filesystem, so the lookup stays reachable where config discovery is forbidden.
//
//go:embed builtins/*.theme
var builtinFS embed.FS

// BuiltinBytes returns the embedded source of the built-in named slug, and
// whether there is one. The bytes are verbatim, in a fresh slice a caller may
// mutate; a miss is an ordinary not-found, never an error (embed matches exactly
// and rejects `..`, so a hostile slug can only miss).
func BuiltinBytes(slug string) ([]byte, bool) {
	data, err := builtinFS.ReadFile(builtinDir + "/" + slug + FileExtension)
	if err != nil {
		return nil, false
	}
	return data, true
}

// BuiltinSlugs returns the slug of every built-in, sorted. Derived from the
// embedded filenames rather than a restated Go list; the sort runs after
// extension-stripping, because stripping can reorder two names.
func BuiltinSlugs() []string {
	entries, err := builtinFS.ReadDir(builtinDir)
	if err != nil {
		// Unreachable: go:embed fails the build when its pattern matches no file.
		return nil
	}

	slugs := make([]string, 0, len(entries))
	for _, entry := range entries {
		slugs = append(slugs, strings.TrimSuffix(entry.Name(), FileExtension))
	}

	slices.Sort(slugs)
	return slugs
}

func builtinSlugSet() map[string]struct{} {
	slugs := BuiltinSlugs()
	set := make(map[string]struct{}, len(slugs))
	for _, slug := range slugs {
		set[slug] = struct{}{}
	}
	return set
}

// LoadBuiltin loads the built-in named slug. The third return is found, not an
// error: an unknown slug routinely sends by-name resolution on to the themes
// directory. It skips the ladder's filename rungs — checking a built-in against
// the reserved set would be asking whether it shadows itself — and touches no
// filesystem.
func (l Loader) LoadBuiltin(slug string) (Result, *Rejection, bool) {
	data, found := l.builtinBytes(slug)
	if !found {
		return Result{}, nil, false
	}

	built, rejection := parseThemeBytes(data)
	if rejection != nil {
		return Result{}, rejection, true
	}

	return Result{Slug: slug, Theme: built, Source: data}, nil, true
}

func (l Loader) builtinBytes(slug string) ([]byte, bool) {
	if l.BuiltinSource == nil {
		return BuiltinBytes(slug)
	}
	return l.BuiltinSource(slug)
}
