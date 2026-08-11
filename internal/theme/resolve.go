package theme

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ResolveByName resolves one nominated slug to a theme, or to one reason it is
// not usable. The charset check runs before any path is composed — the slug
// comes from a hand-editable file or a command line and is used verbatim as a
// path component — and its failure is `bad name`, never `not found`. The
// embedded set is consulted before the directory, which is the no-shadowing
// property on this path.
func (l Loader) ResolveByName(slug, themesDir string) (Result, *Rejection) {
	return l.resolveNamed(slug, func(s string) (Result, *Rejection) {
		return l.loadFromThemesDir(s, themesDir)
	})
}

// slugLoader is the ladder's last rung as a value: where a slug the embedded set
// declined loads from. A parameter so the charset check and the embedded-set
// precedence are stated once for both sources.
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

// No ReadDir here: a directory scan on the cold path would be paid by every
// process, panel or not. The empty directory is `not found` before any join,
// which would otherwise yield a path relative to the working directory. A
// directory denying access stats perfectly well, so that state surfaces one
// syscall later, in narrowReadFailure.
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
// actually was. A denied lookup is the unusable-directory state arriving one
// syscall late, reported against the directory so it dedups with the
// enumeration's sighting — a denial specifically, since an over-long slug also
// fails both syscalls against a perfectly healthy directory. Anything else,
// including a dangling symlink, stays `unreadable` with the OS error verbatim.
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

// Reported against the directory, never the composed file path: the seam dedups
// on path+reason, so this read and the panel's enumeration collapse into a
// single line only while both name the directory.
func (l Loader) reportDirectoryUnusable(themesDir string, rejection *Rejection) *Rejection {
	l.events.DirectoryUnusable(themesDir, rejection)
	return rejection
}

func notFound() *Rejection {
	return &Rejection{Reason: ReasonNotFound}
}
