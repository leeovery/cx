package theme

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ResolveByName resolves one nominated slug to a theme, or to exactly one §6.2
// reason saying why it is not usable.
//
// It is THE by-name resolver, shared by TUI construction and `portal theme
// export` so the two cannot drift apart about what a slug means.
//
// Step 1 is the charset check (§8.6). The slug arrives from a HAND-EDITABLE file
// or a command line and is about to be used to locate a file BY NAME on a path
// that deliberately never enumerates — so `../something` would be used verbatim
// as a path component. Validating first is what stops that, and it runs before
// any path is composed rather than merely before one is read.
//
// A charset failure is `bad name`, NEVER `not found` (§9.4). Each of §6.2's
// reasons has exactly one condition, and telling a user their file is missing
// when they typed an illegal name sends them looking in the wrong place.
//
// Step 2 is the EMBEDDED SET, and it runs before the themes directory (§8.4). A
// nominated slug that names a built-in resolves to the built-in and never reads
// the themes directory at all. That ordering IS §5.4's no-shadowing safety
// property on this path: construction does not enumerate, so there is no
// collision to DETECT here — and construction is where the fallback resolves,
// which is the exact thing no-shadowing exists to protect. A built-in's own
// rejection rides back unchanged; §7.6's build-time test is what makes it
// unreachable in a correct binary.
//
// Step 3, only then, is the themes directory — see loadFromThemesDir. The
// directory arrives as an INJECTED value and is never resolved here (§8.9's
// division of labour): cmd/config.go's themesDirPath owns that chain, which is
// what keeps the embedded set reachable with no path at all and internal/capture's
// no-real-config import guard satisfiable.
func (l Loader) ResolveByName(slug, themesDir string) (Result, *Rejection) {
	return l.resolveNamed(slug, func(s string) (Result, *Rejection) {
		return l.loadFromThemesDir(s, themesDir)
	})
}

// slugLoader is the THIRD rung of the ladder above, as a value: given a slug the
// embedded set has already declined, what does it load to?
//
// It is a parameter because the ladder has exactly two sources and steps 1 and 2
// must not know which one is beneath them. Construction reads the themes
// directory BY NAME (§5.7); the panel resolves against the enumeration it already
// retained (§5.8), because a directory read there would produce a THIRD parse of
// the same slug that can disagree with the row the user is looking at (§8.4).
// Stating the charset check and the embedded-set precedence ONCE for both is what
// keeps §8.6's validation and §5.4's no-shadowing guarantee true on the panel's
// path without either being restated there.
type slugLoader func(slug string) (Result, *Rejection)

// resolveNamed is §8.4's ladder itself — the charset check, then the embedded
// set, then the pass's own source — and the ONE statement of that order.
//
// Steps 1 and 2 are ResolveByName's own doc comment: the charset check runs
// before any path is composed (§8.6), and a nominated slug naming a built-in
// resolves to the built-in and never reaches the source at all (§8.4), which IS
// §5.4's no-shadowing safety property on a path that never enumerates.
func (l Loader) resolveNamed(slug string, source slugLoader) (Result, *Rejection) {
	if !ValidSlug(slug) {
		return Result{}, badName(BadNameSlug)
	}

	if result, rejection, found := l.LoadBuiltin(slug); found {
		return result, rejection
	}

	return source(slug)
}

// loadFromThemesDir is §8.4's second half: compose `<themesDir>/<slug>.theme`
// and load it through the same §6.2 ladder every other route runs.
//
// EXACTLY ONE FILE IS READ and no directory is ever listed (§5.7). Construction
// loads only the nominated themes by name — one read for a constant, two for an
// adaptive pair — and a ReadDir here would pay the startup scan §5.7 rejects on
// the cold path, in a process that may never open the theme panel at all.
// Enumeration belongs to panel open.
//
// The composed basename is always `<valid-slug>.theme` because step 1 already
// refused anything else, which is what makes the ladder's filename rung
// structurally unreachable from this path.
//
// The directory is judged before the path is composed, and §5.5's three states
// give three different answers. An EMPTY directory string is `not found` with no
// path composed and nothing emitted: it is what task 5-7 hands over when the
// themes-directory path could not be resolved at all, and a join onto it would
// yield a RELATIVE path resolving against the process's working directory. An
// ABSENT directory is `not found` too, and equally silent — the common case, not
// an error. An UNUSABLE one is `unreadable`, because permissions is the actual
// problem rather than the filename, and it is the one state that earns a log
// record: §5.5 has `theme: directory unusable` emitted by the panel's
// enumeration and by this construction-time read alike, through the same
// per-process-deduped seam (§12.3) so the two never double up. Emitting it here
// too is what gives a user who never opens the theme panel a record at all.
//
// The stat catches only what a stat can see, which is a regular file where a
// directory belongs. A directory that denies ACCESS stats perfectly well and
// surfaces one syscall later, at the composed path — narrowReadFailure is where
// that state is recognised, and it reports the identical record.
func (l Loader) loadFromThemesDir(slug, themesDir string) (Result, *Rejection) {
	if themesDir == "" {
		return Result{}, notFound()
	}

	usable, rejection := statThemeDir(themesDir)
	if rejection != nil {
		return Result{}, l.reportDirectoryUnusable(themesDir, rejection)
	}
	if !usable {
		return Result{}, notFound()
	}

	path := filepath.Join(themesDir, slug+themeExtension)
	result, rejection := l.LoadFile(path)
	if rejection != nil {
		return Result{}, l.narrowReadFailure(themesDir, path, rejection)
	}
	return result, nil
}

// narrowReadFailure decides which of §5.5's states a failed read of the composed
// path actually was, from the ONE thing that can tell them apart: whether the
// name itself exists.
//
// Three answers, one Lstat:
//
//   - The name is FREE — `not found`. LoadFile cannot draw this line itself,
//     because `not found` is outside §6.2's ladder and producing it from a path
//     would send a user to look for a file that function was just handed. So the
//     resolver that COMPOSED the path draws it, which is the same
//     absent-versus-unusable discrimination §5.5 draws for the directory and
//     §12.1 draws for export: `not found` says check the filename, `unreadable`
//     says check permissions.
//
//   - The LOOKUP ITSELF WAS DENIED — the directory could not be traversed, so
//     this is §5.5's unusable-directory state arriving one syscall later than
//     the directory stat. An unreadable directory STATS PERFECTLY WELL (stat
//     needs no permission on the directory itself), so the stat above cannot see
//     it and the composed path is where it surfaces. It earns the same `theme:
//     directory unusable` record, emitted against THE DIRECTORY — the same
//     identity the panel's enumeration reports it under, which is what lets
//     §12.3's `path`+`reason` dedup keep the two from doubling up.
//
//     A DENIAL specifically, not merely a non-absence. §5.5 defines that record
//     for a directory that is unreadable or is not a directory, and for nothing
//     else. A slug is an identity with NO LENGTH BOUND (§5.2 fixes none
//     deliberately), so a 300-character one fails both syscalls with
//     ENAMETOOLONG against a perfectly healthy directory: blaming it would put a
//     spec-defined WARN on a state the spec defines none for, and would burn the
//     `path`+`reason` dedup slot a real sighting from the panel's enumeration
//     needs in the same process.
//
//   - ANYTHING ELSE — the name resolves and the read still failed (an unreadable
//     file, or a DANGLING SYMLINK), or the lookup failed for a reason that is
//     neither an absence nor a denial. All stay `unreadable` with the OS error
//     verbatim: a dangling link fails to read with exactly the errno an absent
//     file does, and §5.6 puts EVERY read failure under `unreadable`, "not only
//     permissions".
//
// The rejection returned is always LoadFile's own, unaltered but for the
// `not found` substitution — so the detail stays the OS error verbatim (§14A)
// even where the directory is what denied it. Every reason but `unreadable`
// passes straight through, which is what carries the three content reasons back
// exactly as the ladder produced them.
func (l Loader) narrowReadFailure(themesDir, path string, rejection *Rejection) *Rejection {
	if rejection.Reason != ReasonUnreadable {
		return rejection
	}

	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return notFound()
	}
	if errors.Is(err, fs.ErrPermission) {
		return l.reportDirectoryUnusable(themesDir, rejection)
	}
	return rejection
}

// reportDirectoryUnusable emits §5.5's `theme: directory unusable` record and
// hands the rejection straight back, so a caller states the verdict once.
//
// Both of this file's two sightings of an unusable directory route through it
// for one reason: the record is reported against THE DIRECTORY, never against
// the composed file path. §12.3 dedups on `path`+`reason`, so the panel's
// enumeration and this by-name read only collapse into one line while both name
// the directory — a second call site free to pass something else is exactly how
// §5.5's "the two never double up" would quietly stop holding.
func (l Loader) reportDirectoryUnusable(themesDir string, rejection *Rejection) *Rejection {
	l.events.DirectoryUnusable(themesDir, rejection)
	return rejection
}

// notFound builds §6.2's seventh reason — the one deliberately outside the
// rejection ladder, for a nominated slug that names neither a built-in nor a
// file.
//
// It carries no detail, because there is nothing to check: no file was read, no
// contents were judged, and the whole fact is that nothing answers to the name.
func notFound() *Rejection {
	return &Rejection{Reason: ReasonNotFound}
}
