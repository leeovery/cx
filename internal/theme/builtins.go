package theme

import (
	"embed"
	"slices"
	"strings"
)

// DefaultDarkSlug and DefaultLightSlug name the two built-ins Portal resolves
// when nothing else answers. They serve BOTH roles, and it is the same two
// values in each:
//
//   - §8.3's SHIPPED ADAPTIVE PAIR — `theme_dark = tokyo-night`,
//     `theme_light = tokyo-night-day` — already nominated, so a brand-new user
//     gets whichever matches their terminal with nothing to configure.
//   - §8.5's PER-SLOT MODE-MATCHED FALLBACK — an unloadable `theme_dark` falls
//     to DefaultDarkSlug, an unloadable `theme_light` to DefaultLightSlug, and
//     an unloadable constant `theme` to DefaultDarkSlug.
//
// That the two roles hold the same values is load-bearing rather than
// incidental. §8.3's second reason for shipping a pair — "it degrades to the
// alternative", a constant dark default — is true ONLY because an unresolvable
// slot lands on the theme the shipped default already nominates. So CHANGING
// THESE VALUES, OR ADOPTING A SINGLE FIXED FALLBACK, SILENTLY INVALIDATES
// §8.3's degrades-to-a-constant-dark-default argument: nothing stops compiling,
// no floor moves, and the reasoning simply stops holding.
//
// They are PRODUCTION constants rather than test-file literals because Phase 5
// consumes them on both paths, and because §7.6's build-time guarantee is
// precisely the claim that these two values resolve WITHIN the embedded set —
// a renamed built-in file or a typo here leaves every embedded theme valid
// while every fallback path becomes unresolvable, which is why embedded_test.go
// asserts resolution separately from validity.
const (
	DefaultDarkSlug  = "tokyo-night"
	DefaultLightSlug = "tokyo-night-day"
)

// builtinDir is the embedded set's directory within builtinFS. It is also the
// prefix of every name in that filesystem, since go:embed keeps the pattern's
// own path.
const builtinDir = "builtins"

// builtinFS holds Portal's built-in themes as the .theme FILES they are (§7.1).
//
// A built-in is not a Go struct: it is a file, embedded here and parsed by the
// SAME loader as a stranger's drop-in. One code path, one format, one validity
// rule — so "copy a built-in and edit it" is a first-class workflow, adding a
// theme is adding a file, and a format bug is the maintainer's problem on day
// one rather than a user's on day ninety.
//
// The pattern is a single directive over every .theme file in the directory, so
// a new built-in is embedded, enumerated and reserved without a line of Go
// changing. It embeds nothing else — a stray README or a half-written .txt in
// the directory is not a theme and never becomes one.
//
// Embedding is NOT config discovery: nothing here resolves a path, reads an env
// var or touches the filesystem at runtime, which is what keeps the built-in
// lookup reachable from internal/capture under its no-real-config import guard
// (§7.1, §13.3).
//
//go:embed builtins/*.theme
var builtinFS embed.FS

// BuiltinBytes returns the embedded source of the built-in named slug, and
// whether there is one.
//
// The bytes are the FILE's bytes, comments and trailing newline included — they
// are what `portal theme export` writes (§12.1) and what the loader parses, so
// a user copying a built-in gets the attribution header rather than a
// re-serialisation stripped of it.
//
// Each call hands back a FRESH slice: the embedded data is an immutable string
// and a string-to-[]byte conversion always copies, so a caller may mutate what
// it is given without reaching the embedded set or the next caller's read.
//
// The slug is used to build a name within the embedded filesystem, and needs no
// validation to be safe there: embed matches names EXACTLY and rejects any path
// carrying a `..` segment, so a hostile or malformed slug can only ever miss.
// A miss is an ordinary not-found — never an error — because "not a built-in"
// is a routine answer on the by-name resolution path, where the themes
// directory is the next place to look.
func BuiltinBytes(slug string) ([]byte, bool) {
	data, err := builtinFS.ReadFile(builtinDir + "/" + slug + themeExtension)
	if err != nil {
		return nil, false
	}
	return data, true
}

// BuiltinSlugs returns the slug of every built-in, sorted.
//
// The set is DERIVED from the embedded filenames rather than restated as a Go
// list, which is what makes "a PR is add a file" literal (§7.1): a new .theme
// file is enumerated here, reserves its own slug against shadowing (§5.4) and
// is covered by §7.6's build-time validation, with no Go to keep in step. A
// hand-written list would be a second source of truth that silently disagrees
// with the directory the moment someone forgets it.
//
// Sorting is a guarantee OF THIS FUNCTION, not one inherited from the directory
// read: stripping the extension can reorder two names — `a-.theme` sorts before
// `a.theme` while the stems `a` and `a-` sort the other way round — so the
// order is established after the stems exist.
func BuiltinSlugs() []string {
	entries, err := builtinFS.ReadDir(builtinDir)
	if err != nil {
		// Unreachable: go:embed fails the BUILD when its pattern matches no
		// file, so the directory always exists in the embedded filesystem.
		return nil
	}

	slugs := make([]string, 0, len(entries))
	for _, entry := range entries {
		slugs = append(slugs, strings.TrimSuffix(entry.Name(), themeExtension))
	}

	slices.Sort(slugs)
	return slugs
}

// builtinSlugSet returns every embedded slug in the set shape §6.2's rung 2
// looks a candidate's slug up in.
//
// It is DERIVED from BuiltinSlugs — from the embedded files themselves — and is
// never a hand-maintained list, which is what keeps §5.4's guarantee true as the
// set grows: a built-in added by a later PR reserves its own slug with no Go
// edit, where a restated list would pass every test right up until someone
// forgot to extend it and shipped a shadowable theme.
//
// The property that reservation protects, in one sentence: the built-in Portal
// falls back to (§8.5) can never be the user's broken file.
func builtinSlugSet() map[string]struct{} {
	slugs := BuiltinSlugs()
	set := make(map[string]struct{}, len(slugs))
	for _, slug := range slugs {
		set[slug] = struct{}{}
	}
	return set
}

// LoadBuiltin loads the built-in named slug, returning the same shape LoadFile
// returns for a drop-in, whether the slug names a built-in at all, and — if it
// does and its file is broken — why.
//
// The third return is FOUND, not an error, because an unknown slug is not a
// failure of anything: by-name resolution asks the built-in set first and falls
// through to the themes directory, so "not one of ours" is the routine answer
// that sends it there. A rejection means something quite different — the slug IS
// a built-in and its file did not parse, which is what §7.6's build-time test
// exists to make impossible, and which is reported here as an ordinary
// rejection rather than a panic.
//
// It reaches the SAME parse path a file does (§7.1), so the two callers cannot
// be told apart by behaviour. What it deliberately does not run are the ladder's
// two filename rungs: `bad name` and `reserved name` are decided from a
// FILENAME, and a built-in's identity comes from an embedded name Portal itself
// committed — checking a built-in against the reserved set would be asking
// whether it shadows itself. The reserved-slug set and the event seam therefore
// bear on it not at all; the receiver carries only the injected byte source, and
// the method form is what lets by-name resolution reach both loaders through the
// one Loader it holds.
//
// It takes NO DIRECTORY and touches no filesystem, on either outcome. That is
// what keeps the built-in set reachable from internal/capture under its
// no-real-config import guard, and what lets Phase 5's fallback resolve a
// built-in in a process that has resolved no config path at all.
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

// builtinBytes reads a built-in's source through the Loader's injected seam,
// falling back to the embedded set when none was injected — which is every
// production caller (see Loader.BuiltinSource).
//
// The indirection buys the one thing a go:embed'ed set otherwise cannot have: a
// way to stage §7.6's broken binary, where the theme a fallback resolves to is
// missing or invalid. It costs a nil check per built-in read and changes nothing
// a shipped binary does.
func (l Loader) builtinBytes(slug string) ([]byte, bool) {
	if l.BuiltinSource == nil {
		return BuiltinBytes(slug)
	}
	return l.BuiltinSource(slug)
}
