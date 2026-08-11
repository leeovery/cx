package theme

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ResolveByName resolves one nominated slug to a theme, or to one reason why
// it is not usable: charset check, then the embedded set, then the themes
// directory. The charset check runs before any path is composed — the slug
// comes from a hand-editable file or a command line and is used verbatim as a
// path component on a route that never enumerates — and a charset failure is
// `bad name`, never `not found`. The embedded set winning over the directory
// is the no-shadowing property on this path. themesDir is injected, never
// resolved here.
func (l Loader) ResolveByName(slug, themesDir string) (Result, *Rejection) {
	return l.resolveNamed(slug, func(s string) (Result, *Rejection) {
		return l.loadFromThemesDir(s, themesDir)
	})
}

// slugLoader is the ladder's third rung as a value: given a slug the embedded
// set declined, what does it load to? A parameter so the charset check and
// embedded-set precedence are stated once for both sources — the
// construction-time directory read and the panel's retained enumeration.
type slugLoader func(slug string) (Result, *Rejection)

func (l Loader) resolveNamed(slug string, source slugLoader) (Result, *Rejection) {
	if !ValidSlug(slug) {
		return Result{}, badName(BadNameSlug)
	}

	if result, rejection, found := l.LoadBuiltin(slug); found {
		return result, rejection
	}

	return source(slug)
}

// Exactly one file is read and no directory is ever listed: a ReadDir here
// would pay a full directory scan on the cold path for a process that may
// never open the theme panel. The directory is judged before the path is
// composed — an empty string is `not found` with nothing composed, since a
// join onto it would yield a path relative to the working directory; an
// absent directory is `not found` too; an unusable one is `unreadable`. A
// directory denying access stats perfectly well, so that state surfaces one
// syscall later in narrowReadFailure.
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

	path := filepath.Join(themesDir, slug+FileExtension)
	result, rejection := l.LoadFile(path)
	if rejection != nil {
		return Result{}, l.narrowReadFailure(themesDir, path, rejection)
	}
	return result, nil
}

// narrowReadFailure decides which state a failed read of the composed path
// actually was. A free name is `not found` (drawn here, by the resolver that
// composed the path, since LoadFile producing it would send a user to look for
// a file it was just handed). A denied lookup is the unusable-directory state
// arriving one syscall late, reported against the directory so it dedups with
// the enumeration's sighting — a denial specifically, since an over-long slug
// also fails both syscalls against a perfectly healthy directory. Anything
// else, including a dangling symlink, stays `unreadable` with the OS error
// verbatim.
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

// The record is reported against the directory, never the composed file path:
// the seam dedups on path+reason, so this read and the panel's enumeration
// collapse into one line only while both name the directory.
func (l Loader) reportDirectoryUnusable(themesDir string, rejection *Rejection) *Rejection {
	l.events.DirectoryUnusable(themesDir, rejection)
	return rejection
}

// No detail: no file was read and no contents judged — the whole fact is that
// nothing answers to the name.
func notFound() *Rejection {
	return &Rejection{Reason: ReasonNotFound}
}
