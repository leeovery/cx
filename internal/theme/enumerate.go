package theme

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Entry is one candidate found in the themes directory, classified: either the
// slug and palette it yielded, or the one Rejection saying why it is not usable.
//
// EVERY candidate produces an entry, valid or not. That is the whole promise of
// the drop-in route — a broken file is PRESENT AND NAMED, so the user sees
// "there's my theme, it's registered, but it's invalid" rather than being
// completely in the dark about why it never appeared.
//
// Slug is empty EXACTLY when the rejection is `bad name`: those are the two
// causes that mean the filename yields no usable identity, and every other
// rejection leaves the name intact, so a broken file still has something to be
// listed under.
type Entry struct {
	Path      string
	Filename  string
	Slug      string
	Theme     Theme
	Rejection *Rejection
}

// Enumerate reads the TOP LEVEL of the injected themes directory — no
// recursion — and classifies every candidate in it through LoadFile's rejection
// ladder.
//
// The two returns are independent: the entries are what the directory held, the
// Rejection is the verdict on the DIRECTORY ITSELF. An absent directory — the
// common case — is silent: no entries and no verdict, since zero drop-ins is not
// an error. An unusable one — unreadable, or a regular file where a directory
// belongs — is a genuine misconfiguration and comes back as `unreadable` with no
// entries.
//
// Entries arrive in os.ReadDir's filename order, so the result is deterministic.
// The display sort key — slug with a filename fallback, case-insensitive with a
// byte-wise tie-break — belongs to Reassemble (see sortRows) and is deliberately
// NOT applied here.
//
// The directory is enumerated afresh on every call: no caching, because caching
// would break the loop the drop-in route exists for — copy a built-in, edit it,
// see it, without relaunching Portal.
//
// The three conditions the `theme` log component reports on are each
// distinguishable here and are emitted through the injected seam: one `rejected`
// per rejected entry, one `directory unusable` for the directory verdict, and
// NOTHING WHATSOEVER for an absent directory. Whether any of it is written is the
// caller's decision, not this function's — a Loader carrying a
// log.Discard-backed seam (doctor, export, capturetool) runs the identical path
// in silence — and repeated enumeration is deduplicated by the seam, which is
// what keeps re-reading on every panel open from turning a forensic trail into a
// running commentary.
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

// isCandidate reports whether a directory entry's name is a theme file's,
// matching the extension CASE-INSENSITIVELY.
//
// That is deliberately looser than what is accepted: `Nord.THEME` is a candidate
// so it is VISIBLE, and the ladder then rejects it as `bad name`, which is what
// stops the file being silently invisible on the case-insensitive filesystem
// where a user is most likely to type it that way. Since a non-exact extension
// never contributes a slug, no duplicate slug can be minted by the looser match.
//
// Anything else is ignored ENTIRELY — no entry, no reason, no log. A file that
// was never a theme file did not fail to be one.
func isCandidate(name string) bool {
	return strings.EqualFold(filepath.Ext(name), FileExtension)
}

// resolvesToDirectory reports whether a candidate resolves to a directory, which
// enumeration skips SILENTLY: it matches files, and a directory is not a
// candidate that failed, it is not a candidate at all.
//
// The stat resolves the entry, so a real subdirectory named `x.theme` and a
// symlink whose target is a directory are decided by ONE rule rather than two —
// what the entry resolves to is what decides, not whether a link is involved.
//
// A stat FAILURE is deliberately not a skip. A dangling symlink resolves to
// nothing, and skipping it would make a user's broken link silently absent; it
// has to reach the ladder instead, which reports it `unreadable`.
func resolvesToDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// classify runs one candidate through LoadFile's rejection ladder and records
// the answer, whichever way it went.
//
// A rejected candidate keeps the slug its filename yields, because a broken file
// still has to be listed under a name. The one exception is the one the ladder
// itself defines: `bad name` means the filename yields NO usable
// identity, so the slug is empty exactly there. That is re-derived through
// SlugFromFilename rather than reconstructed, since a rejected LoadFile returns
// the zero Result and the rule must not exist twice.
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

// slugOrEmpty returns the slug filename yields, or the empty string when it
// yields none — which is the case for, and only for, the two `bad name` causes.
func slugOrEmpty(filename string) string {
	slug, rejection := SlugFromFilename(filename)
	if rejection != nil {
		return ""
	}
	return slug
}

// readThemeDir lists dir's top-level entries, or returns the verdict on the
// directory itself.
//
// The three returns are the whole directory-state contract: entries for a usable
// directory, (nil, nil) for an absent one, and (nil, rejection) for an unusable
// one. Absent and
// unusable are kept apart here rather than collapsed into "no entries", because
// the first is silent by decision while the second is a misconfiguration that
// owes the user a doctor advisory and a log line.
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

// statThemeDir answers the directory-state question — is this a themes directory
// Portal can go on to use? — in three shapes: (true, nil) for a usable directory,
// (false, nil) for an ABSENT one, and (false, rejection) for an UNUSABLE one.
//
// It is stated ONCE and shared by the package's directory consumers, the panel's
// enumeration and ResolveByName's construction-time by-name read. The two must
// report the same condition — they are deduplicated against each other on
// `path`+`reason` — which a second, parallel stat could only make possible to
// break. Neither the enumeration's ReadDir nor the by-name read's composed path
// belongs here, so each caller adds its own; EMISSION is likewise the caller's,
// since a rejection is worth a line only where a theme is being used.
//
// The stat is deliberately Stat and NOT Lstat, so a themes directory reached
// through a symlink is FOLLOWED. Dotfiles users symlink ~/.config/portal and its
// contents as a matter of course; not following the root would make every
// drop-in vanish with no row and no doctor line — the "completely in the dark"
// state the drop-in route exists to prevent.
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

// notADirectory builds the error for the one directory-state the OS is never
// asked about: a path that exists and is readable but is not a directory — a
// regular file where a directory belongs.
//
// Deciding this from the stat rather than from a doomed read is what keeps the
// verdict the same on every platform, but it also leaves no OS error to carry —
// so the error is RECONSTRUCTED in the shape os.ReadDir's own failure would have
// taken: a *fs.PathError over ENOTDIR, under the verb `open`, which is the one
// os.ReadDir reports here because it opens with O_DIRECTORY. Type, verb and errno
// all matter — the rejection's detail is the OS error VERBATIM, so a verb no Go
// code path produces would be a fabrication in the one field whose contract is
// that it is not, and a caller matches on the errno structurally.
func notADirectory(dir string) error {
	return &fs.PathError{Op: "open", Path: dir, Err: syscall.ENOTDIR}
}
