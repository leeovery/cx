package theme

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Entry is one candidate found in the themes directory, valid or not — a broken
// file is present and named rather than silently absent. Slug is empty exactly
// when the rejection is `bad name`.
type Entry struct {
	Path      string
	Filename  string
	Slug      string
	Theme     Theme
	Rejection *Rejection
}

// Enumerate reads the top level of the themes directory (no recursion) and
// classifies every candidate through LoadFile's rejection ladder. The Rejection
// return is the verdict on the directory itself: absent is silent, unusable
// comes back `unreadable`. Entries are in filename order — the display sort
// belongs to Reassemble — and the directory is read afresh every call, since
// caching would break the copy-edit-see drop-in loop.
func (l Loader) Enumerate(dir string) ([]Entry, *Rejection) {
	dirEntries, rejection := readThemeDir(dir)
	if rejection != nil {
		l.events.DirectoryUnusable(dir, rejection)
		return nil, rejection
	}

	entries := make([]Entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		if !isCandidate(name) {
			continue
		}
		path := filepath.Join(dir, name)
		if resolvesToDirectory(path) {
			continue
		}
		entry := l.classify(path)
		if entry.Rejection != nil {
			l.events.Rejected(entry.Slug, entry.Path, entry.Rejection)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// OpenEnumeration performs one directory read and states what it means, so no
// consumer decides for itself what an absent or unusable directory is — two
// answers to that question is two consumers disagreeing about one directory. An
// unresolved path reads nothing: it is not a directory anything could name.
func (l Loader) OpenEnumeration(dir string) Enumeration {
	if dir == "" {
		return Enumeration{}
	}

	entries, rejection := l.Enumerate(dir)
	return Enumeration{Entries: entries, DirUnusable: rejection != nil, DirPath: dir}
}

// Deliberately looser than what is accepted: a mis-cased extension becomes a
// visible `bad name` rejection instead of being silently invisible on a
// case-insensitive filesystem, and it never contributes a slug, so the looser
// match mints no duplicate.
func isCandidate(name string) bool {
	return strings.EqualFold(filepath.Ext(name), FileExtension)
}

// Stat, not Lstat, so a symlink is judged by its target. A stat failure is
// deliberately not a skip — a dangling symlink must reach the ladder and be
// reported `unreadable` rather than vanish.
func resolvesToDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// A rejected candidate keeps the slug its filename yields — a broken file still
// has to be listed under a name — re-derived through SlugFromFilename because a
// rejected LoadFile returns the zero Result.
func (l Loader) classify(path string) Entry {
	entry := Entry{Path: path, Filename: filepath.Base(path)}

	result, rejection := l.LoadFile(path)
	if rejection != nil {
		entry.Rejection = rejection
		entry.Slug = slugOrEmpty(entry.Filename)
		return entry
	}

	entry.Slug = result.Slug
	entry.Theme = result.Theme
	return entry
}

func slugOrEmpty(filename string) string {
	slug, rejection := SlugFromFilename(filename)
	if rejection != nil {
		return ""
	}
	return slug
}

func readThemeDir(dir string) ([]os.DirEntry, *Rejection) {
	usable, rejection := statThemeDir(dir)
	if !usable {
		return nil, rejection
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, unreadable(err)
	}
	return entries, nil
}

// An absent directory is (false, nil) — silent by decision — and an unusable one
// (false, `unreadable`). Stat, not Lstat: dotfiles users symlink
// ~/.config/portal, and not following the root would make every drop-in vanish
// with no row and no doctor line.
func statThemeDir(dir string) (bool, *Rejection) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, unreadable(err)
	}
	if !info.IsDir() {
		return false, unreadable(notADirectory(dir))
	}
	return true, nil
}

// Reconstructed in the exact shape os.ReadDir's own failure would take. Type,
// verb and errno all matter: the rejection's Detail is the OS error verbatim,
// and callers match the errno structurally.
func notADirectory(dir string) error {
	return &fs.PathError{Op: "open", Path: dir, Err: syscall.ENOTDIR}
}
