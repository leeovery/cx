package theme_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// TestEnumerate_AbsentDirectoryIsSilent pins the first row of the directory-resolution rule's
// directory-state table: an absent themes directory is THE COMMON CASE and is
// completely silent — no entries, no rejection, and no doctor line. Zero
// drop-ins is not an error, and Portal never creates or seeds the directory.
//
// The verdict has to stay distinguishable from the unusable-directory one below,
// because that is what the `theme` log component's emission decision rests on:
// an unusable directory owes the user a record, an absent one owes nothing.
func TestEnumerate_AbsentDirectoryIsSilent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")

	entries, rejection := theme.Loader{}.Enumerate(dir)

	if rejection != nil {
		t.Errorf("Enumerate(%q) = %v, want no rejection — an absent directory is silent", dir, rejection)
	}
	if len(entries) != 0 {
		t.Errorf("Enumerate(%q) returned %d entries, want none", dir, len(entries))
	}
}

// TestEnumerate_RegularFileWhereDirectoryBelongs pins the other row of the
// directory-resolution rule's table on the state that is easiest to mistake for the silent
// one: the path EXISTS, so an absent-directory check alone would report nothing, yet nothing
// there can ever be read as a themes directory. It is a genuine misconfiguration
// and reports `unreadable`.
//
// `unreadable`, never `not found`: `not found` sends the user to check the
// filename, `unreadable` sends them to check the directory — and the directory is
// the actual problem. The errno is asserted structurally rather than by
// its rendered text, which is the platform's wording rather than Portal's.
//
// This is the one state whose error nothing hands back — the stat decides it, so
// no failed read is left to carry one and the error is reconstructed. The detail
// is checked against what the OS ITSELF produces for the same path, because the pinned copy
// makes that field the OS error verbatim: a reconstruction that renders any other
// wording — a verb no code path in Go produces, say — would be a fabrication
// sitting in the one field whose whole contract is that it is not.
func TestEnumerate_RegularFileWhereDirectoryBelongs(t *testing.T) {
	dir := writeFile(t, t.TempDir(), "themes", "this is not a directory\n")

	_, readErr := os.ReadDir(dir)
	if readErr == nil {
		t.Fatalf("os.ReadDir(%q) succeeded, the fixture is not a regular file", dir)
	}

	entries, rejection := theme.Loader{}.Enumerate(dir)

	requireDirectoryUnusable(t, entries, rejection)
	if !errors.Is(rejection.Err, syscall.ENOTDIR) {
		t.Errorf("rejection.Err = %v, want an error carrying ENOTDIR", rejection.Err)
	}
	if rejection.Detail != readErr.Error() {
		t.Errorf("rejection detail = %q, want the OS's own wording for this state: %q", rejection.Detail, readErr.Error())
	}
}

// TestEnumerate_UnreadableDirectory pins the directory-resolution rule's misconfiguration row
// on the case it is actually written for: a real themes directory whose bytes cannot be
// listed. The reason is `unreadable` because permissions are what the user has
// to go and fix.
//
// The directory holds a perfectly valid theme, which is what makes the verdict
// load-bearing rather than incidental: the file exists and would enumerate, and
// the assertion is that the directory's failure suppresses it entirely rather
// than half-reporting a set nobody could read.
func TestEnumerate_UnreadableDirectory(t *testing.T) {
	dir := unreadableDir(t)

	if _, readErr := os.ReadDir(dir); readErr == nil {
		t.Fatalf("os.ReadDir(%q) succeeded, the fixture is not unreadable", dir)
	}

	entries, rejection := theme.Loader{}.Enumerate(dir)

	requireDirectoryUnusable(t, entries, rejection)
	if !errors.Is(rejection.Err, fs.ErrPermission) {
		t.Errorf("rejection.Err = %v, want an error carrying a permission denial", rejection.Err)
	}
}

// TestEnumerate_FollowsSymlinkedRoot pins the enumeration rule's root rule: the RESOLVED
// themes directory may itself be a symlink, and it is followed. Dotfiles users symlink
// ~/.config/portal and its contents as a matter of course, and not following the
// root would make every drop-in vanish with no row and no doctor line — the
// "completely in the dark" state the union rule exists to prevent.
//
// This is what makes os.Stat rather than os.Lstat the load-bearing choice at the
// root: an Lstat would see a symlink, not a directory, and report the whole
// directory `unreadable`.
func TestEnumerate_FollowsSymlinkedRoot(t *testing.T) {
	real := t.TempDir()
	themetest.Write(t, real, "nord-lee.theme", themetest.Lines())
	link := filepath.Join(t.TempDir(), "themes")
	linkAt(t, real, link)

	entries, rejection := theme.Loader{}.Enumerate(link)

	if rejection != nil {
		t.Fatalf("Enumerate(%q) reported the directory unusable: %v", link, rejection)
	}
	entry := requireSingleEntry(t, entries)
	requireValidEntry(t, entry, "nord-lee")
	if want := filepath.Join(link, "nord-lee.theme"); entry.Path != want {
		t.Errorf("entry path = %q, want %q — the path is the one enumerated, not the link's target", entry.Path, want)
	}
}

// TestEnumerate_TopLevelOnly pins the enumeration rule's first bullet: enumeration matches
// files in the directory ITSELF and never recurses.
//
// The nested file is a perfectly valid theme, so the only thing keeping it out
// of the result is the rule. It must not be listed and must not be rejected
// either — the user never asked for a theme there.
func TestEnumerate_TopLevelOnly(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "top.theme", themetest.Lines())
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", sub, err)
	}
	themetest.Write(t, sub, "inner.theme", themetest.Lines())

	entries, rejection := theme.Loader{}.Enumerate(dir)

	if rejection != nil {
		t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
	}
	entry := requireSingleEntry(t, entries)
	requireValidEntry(t, entry, "top")
}

// TestEnumerate_FollowsSymlinkedFiles pins the enumeration rule's symlinked-file rule and the
// half of it that decides identity: the link IS followed — the standard dotfiles
// shape, and dotfiles users are exactly who hand-authors a theme — and the slug
// derives from the LINK NAME as enumerated, not from the target's.
//
// The target is deliberately named something else entirely, so a slug taken from
// the resolved path would be visibly wrong rather than accidentally right.
func TestEnumerate_FollowsSymlinkedFiles(t *testing.T) {
	target := themetest.Write(t, t.TempDir(), "original-name.theme", themetest.Lines())
	dir := t.TempDir()
	linkAt(t, target, filepath.Join(dir, "link.theme"))

	entries, rejection := theme.Loader{}.Enumerate(dir)

	if rejection != nil {
		t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
	}
	entry := requireSingleEntry(t, entries)
	requireValidEntry(t, entry, "link")
}

// TestEnumerate_DanglingSymlinkIsUnreadable pins the enumeration rule's other half of the
// symlink rule: a dangling link ENUMERATES and then fails to read, reason
// `unreadable` — which covers every read failure, not only permissions.
//
// It is the case that stops "what does this entry resolve to?" being answered
// with a skip: the link resolves to nothing, and a skip would make the user's
// broken link silently absent instead of present and named. The slug
// survives because the FILENAME is fine — only the bytes behind it are not.
func TestEnumerate_DanglingSymlinkIsUnreadable(t *testing.T) {
	dir := t.TempDir()
	writeDanglingThemeLink(t, dir, "gone.theme")

	entries, rejection := theme.Loader{}.Enumerate(dir)

	if rejection != nil {
		t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
	}
	entry := requireSingleEntry(t, entries)
	requireRejectedEntry(t, entry, theme.ReasonUnreadable)
	if want := "gone"; entry.Slug != want {
		t.Errorf("entry slug = %q, want %q — only a bad name costs an entry its slug", entry.Slug, want)
	}
}

// TestEnumerate_SkipsDirectoryValuedEntriesSilently pins the enumeration rule's
// one-rule-not-two clause: a `.theme`-named entry that RESOLVES to a directory is skipped
// silently, whether it is a real subdirectory or a symlink pointing at one.
// "What the entry resolves to is what decides, not whether a link is involved."
//
// Silently is the operative word — enumeration matches files, and a directory is
// not a candidate that failed, it is not a candidate at all. So it must produce
// neither an entry nor a rejection, which is what separates this rule from every
// other enumeration bullet.
//
// A valid theme sits beside it in both cases, so the assertion is that
// enumeration skipped one entry and carried on rather than stopping at it.
func TestEnumerate_SkipsDirectoryValuedEntriesSilently(t *testing.T) {
	tests := []struct {
		name  string
		base  string
		stage func(t *testing.T, path string)
	}{
		{
			name: "a real subdirectory",
			base: "x.theme",
			stage: func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", path, err)
				}
			},
		},
		{
			name: "a symlink whose target is a directory",
			base: "y.theme",
			stage: func(t *testing.T, path string) {
				linkAt(t, t.TempDir(), path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
			tt.stage(t, filepath.Join(dir, tt.base))

			entries, rejection := theme.Loader{}.Enumerate(dir)

			if rejection != nil {
				t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
			}
			entry := requireSingleEntry(t, entries)
			requireValidEntry(t, entry, "nord-lee")
		})
	}
}

// TestEnumerate_CaseInsensitiveExtensionVisibleThenBadName pins the enumeration rule's
// two-part extension rule, and both parts are load-bearing.
//
// Matched case-insensitively for ENUMERATION, so the file is VISIBLE — a
// silently absent file is the "completely in the dark" state the union rule exists to
// prevent, and it would be absent most often on the case-insensitive filesystem
// where a user is most likely to type it that way. Accepted only at the exact
// lowercase `.theme`, so the file is rejected `bad name` on its EXTENSION and
// contributes no slug — which is what makes a duplicate slug impossible and
// leaves no precedence rule or ordering tie-break to define.
//
// The empty slug is asserted for that reason: it is the mechanism, not a detail.
//
// `Nord.THEME` carries the slug cause rather than the extension one: its stem is
// illegal too, and the extension message claims the stem is fine.
func TestEnumerate_CaseInsensitiveExtensionVisibleThenBadName(t *testing.T) {
	cases := []struct {
		base      string
		wantCause theme.BadNameCause
	}{
		{base: "Nord.THEME", wantCause: theme.BadNameSlug},
		{base: "nord.Theme", wantCause: theme.BadNameExtension},
	}

	for _, tc := range cases {
		t.Run(tc.base, func(t *testing.T) {
			dir := t.TempDir()
			themetest.Write(t, dir, tc.base, themetest.Lines())

			entries, rejection := theme.Loader{}.Enumerate(dir)

			if rejection != nil {
				t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
			}
			entry := requireSingleEntry(t, entries)
			if entry.Filename != tc.base {
				t.Errorf("entry filename = %q, want %q — the file must be visible", entry.Filename, tc.base)
			}
			requireRejectedEntry(t, entry, theme.ReasonBadName)
			if entry.Rejection.BadNameCause != tc.wantCause {
				t.Errorf("entry cause = %v, want %v", entry.Rejection.BadNameCause, tc.wantCause)
			}
			if entry.Slug != "" {
				t.Errorf("entry slug = %q, want empty — a non-exact extension contributes no slug", entry.Slug)
			}
		})
	}
}

// TestEnumerate_IllegalStemIsBadNameWithNoSlug pins `bad name`'s OTHER cause at
// the enumeration entry point: an exact lowercase `.theme` extension over a stem
// that is not a legal slug. The extension is fine here, so nothing about
// the file's visibility is in question — it is the identity that fails.
//
// Both causes are asserted at this level, rather than only in the loader's own
// suite, because the empty slug is re-derived here (the reason vocabulary's rung 1 is what
// makes it empty, but enumeration is what asks again after a rejection). A re-derivation
// that consulted only the extension would answer `Nord` for this file — a slug
// the slug charset rule refuses to mint, and the exact quiet normalisation the reserved-slug
// rule's no-shadowing rule depends on never happening.
func TestEnumerate_IllegalStemIsBadNameWithNoSlug(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "Nord.theme", themetest.Lines())

	entries, rejection := theme.Loader{}.Enumerate(dir)

	if rejection != nil {
		t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
	}
	entry := requireSingleEntry(t, entries)
	requireRejectedEntry(t, entry, theme.ReasonBadName)
	if entry.Rejection.BadNameCause != theme.BadNameSlug {
		t.Errorf("entry cause = %v, want %v — the extension is exact, the stem is not a legal slug", entry.Rejection.BadNameCause, theme.BadNameSlug)
	}
	if entry.Slug != "" {
		t.Errorf("entry slug = %q, want empty — a name that yields no identity is never quietly corrected into one", entry.Slug)
	}
}

// TestEnumerate_AppliesTheInjectedReservedSlugs pins that enumeration runs
// candidates through the loader IT WAS CALLED ON, not a fresh zero one.
//
// The reserved set is the only rung whose input arrives on the Loader, so it is
// the only one that can prove this — and it is the rung that must not be lost.
// The reserved-slug rule is a hard constraint, not a preference: an invalid theme falls back
// to a built-in, so a user file that could shadow the built-in could break the very
// thing Portal falls back to, and "that must be impossible". An enumeration that
// dropped the injected set would list a shadowing drop-in as a VALID, SELECTABLE
// panel row — the failure would surface as a broken fallback rather than as a
// missing check.
//
// The slug survives the rejection, which is the same "only a bad name costs an
// entry its slug" rule the rejected entries elsewhere assert: `reserved name` is
// decided FROM a perfectly good slug, so the row is listed under the name that
// collided — which is what makes the reserved-slug rule's two-second fix (rename it
// `nord-lee`) self-evident from the panel.
func TestEnumerate_AppliesTheInjectedReservedSlugs(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "nord.theme", themetest.Lines())

	loader := theme.Loader{ReservedSlugs: map[string]struct{}{"nord": {}}}
	entries, rejection := loader.Enumerate(dir)

	if rejection != nil {
		t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
	}
	entry := requireSingleEntry(t, entries)
	requireRejectedEntry(t, entry, theme.ReasonReservedName)
	if want := "nord"; entry.Slug != want {
		t.Errorf("entry slug = %q, want %q — only a bad name costs an entry its slug", entry.Slug, want)
	}
}

// TestEnumerate_IgnoresNonThemeFiles pins the other side of the candidate
// filter: a file that is not a theme file is ignored ENTIRELY — no entry, no
// reason, and no log line. It did not fail to be a theme; it
// never was one.
//
// The three fixtures are the near misses the filter has to get right: a
// different extension, no extension at all, and the bare word `theme` — which
// contains the extension's letters but carries no dot, so filepath.Ext yields
// nothing to match. The valid file beside them is what proves the filter
// discriminated rather than rejecting everything.
func TestEnumerate_IgnoresNonThemeFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "notes.txt", "not a theme\n")
	writeFile(t, dir, "README", "nor this\n")
	writeFile(t, dir, "theme", "nor this either\n")
	themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())

	entries, rejection := theme.Loader{}.Enumerate(dir)

	if rejection != nil {
		t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
	}
	entry := requireSingleEntry(t, entries)
	requireValidEntry(t, entry, "nord-lee")
}

// TestEnumerate_ValidAndInvalidFilesBothProduceEntries pins the union rule's reason for
// enumerating everything: an invalid theme is PRESENT AND NAMED, so the user
// sees "there's my theme, it's registered, but it's invalid" rather than being
// completely in the dark about why it did not appear. Five files in, five
// entries out — the valid ones carrying their whole palette, each invalid one
// carrying exactly one reason and no palette.
//
// The order is asserted too. Entries come back in os.ReadDir's filename order,
// which is BYTE-WISE, so a caller gets a deterministic result; the row-rendering rule's
// display sort key (slug with a filename fallback, case-insensitive with a byte-wise
// tie-break) belongs to Reassemble (see sortRows) and is deliberately NOT applied
// here. The names
// are staged in an order that differs from their sorted order, so a result that
// merely echoed the staging would fail — and `Zed.THEME` beside `apple.theme` is
// what makes the two orders DISTINGUISHABLE: byte-wise the capital sorts first,
// case-insensitively it would sort last. Without that pair every fixture sorts
// identically under both rules and the assembler's sort key applied here would
// pass unnoticed.
func TestEnumerate_ValidAndInvalidFilesBothProduceEntries(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "valid.theme", themetest.Lines())
	themetest.Write(t, dir, "missing.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	themetest.Write(t, dir, "bad-colour.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))
	themetest.Write(t, dir, "Zed.THEME", themetest.Lines())
	themetest.Write(t, dir, "apple.theme", themetest.Lines())

	entries, rejection := theme.Loader{}.Enumerate(dir)

	if rejection != nil {
		t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
	}
	if len(entries) != 5 {
		t.Fatalf("Enumerate(%q) returned %d entries, want 5: %+v", dir, len(entries), entries)
	}

	want := []string{"Zed.THEME", "apple.theme", "bad-colour.theme", "missing.theme", "valid.theme"}
	if got := filenamesOf(entries); !slices.Equal(got, want) {
		t.Fatalf("entry filenames = %v, want %v in os.ReadDir's byte-wise filename order", got, want)
	}
	requireRejectedEntry(t, entries[0], theme.ReasonBadName)
	requireValidEntry(t, entries[1], "apple")
	requireRejectedEntry(t, entries[2], theme.ReasonBadColour)
	requireRejectedEntry(t, entries[3], theme.ReasonMissingTokens)
	requireValidEntry(t, entries[4], "valid")

	// Slug empty exactly when the rejection is `bad name`, asserted as the
	// biconditional across the whole set: every other entry — valid or rejected —
	// keeps the identity its filename yields, so a broken file is still listed
	// under a name.
	for _, entry := range entries {
		badName := entry.Rejection != nil && entry.Rejection.Reason == theme.ReasonBadName
		if gotEmpty := entry.Slug == ""; gotEmpty != badName {
			t.Errorf("entry %q slug = %q with rejection %v, want it empty exactly when the reason is %q", entry.Filename, entry.Slug, entry.Rejection, theme.ReasonBadName)
		}
	}
}

// requireSingleEntry fails the test unless exactly one entry came back, and
// returns it.
func requireSingleEntry(t *testing.T, entries []theme.Entry) theme.Entry {
	t.Helper()

	if len(entries) != 1 {
		t.Fatalf("Enumerate() returned %d entries, want exactly 1: %+v", len(entries), entries)
	}
	return entries[0]
}

// requireValidEntry fails the test unless the entry loaded cleanly under the
// expected slug, carrying the whole parsed palette and no rejection.
func requireValidEntry(t *testing.T, entry theme.Entry, wantSlug string) {
	t.Helper()

	if entry.Rejection != nil {
		t.Fatalf("entry %q carries the rejection %v, want a valid theme", entry.Filename, entry.Rejection)
	}
	if entry.Slug != wantSlug {
		t.Errorf("entry slug = %q, want %q", entry.Slug, wantSlug)
	}
	if want := wantSlug + ".theme"; entry.Filename != want {
		t.Errorf("entry filename = %q, want %q", entry.Filename, want)
	}
	if tokens, want := entry.Theme.All(), wantThemeTokens(); !slices.Equal(tokens, want) {
		t.Errorf("entry theme = %+v, want %+v", tokens, want)
	}
}

// requireRejectedEntry fails the test unless the entry carries exactly the one
// expected reason and no palette. A rejected entry never comes back
// half-populated: the ladder returns the zero Result with every rejection.
func requireRejectedEntry(t *testing.T, entry theme.Entry, wantReason theme.Reason) {
	t.Helper()

	if entry.Rejection == nil {
		t.Fatalf("entry %q loaded cleanly, want the rejection %q", entry.Filename, wantReason)
	}
	if entry.Rejection.Reason != wantReason {
		t.Errorf("entry %q reason = %q, want %q", entry.Filename, entry.Rejection.Reason, wantReason)
	}
	if entry.Theme != (theme.Theme{}) {
		t.Errorf("entry %q carries the palette %+v alongside a rejection, want the zero Theme", entry.Filename, entry.Theme)
	}
}

// requireDirectoryUnusable fails the test unless Enumerate reported the
// directory itself as `unreadable`, returned no entries alongside that verdict,
// and kept the OS error — verbatim in the detail and structured on Err,
// which is the only thing distinguishing a denial from a non-directory.
func requireDirectoryUnusable(t *testing.T, entries []theme.Entry, rejection *theme.Rejection) {
	t.Helper()

	if rejection == nil {
		t.Fatalf("Enumerate() returned %d entries and no rejection, want the directory reported as %q", len(entries), theme.ReasonUnreadable)
	}
	if len(entries) != 0 {
		t.Errorf("Enumerate() returned %d entries alongside a directory rejection, want none", len(entries))
	}
	if rejection.Reason != theme.ReasonUnreadable {
		t.Errorf("rejection reason = %q, want %q", rejection.Reason, theme.ReasonUnreadable)
	}
	if rejection.Err == nil {
		t.Fatal("rejection carries no Err, want the OS error")
	}
	if rejection.Detail != rejection.Err.Error() {
		t.Errorf("rejection detail = %q, want the OS error verbatim: %q", rejection.Detail, rejection.Err.Error())
	}
}

// filenamesOf lists the entries' filenames in the order they came back.
func filenamesOf(entries []theme.Entry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Filename)
	}
	return names
}

// writeFile writes contents as a file named base inside dir and returns its
// path. Unlike themetest.Write it takes raw bytes, so it can stage the things
// that are NOT theme files.
func writeFile(t *testing.T, dir, base, contents string) string {
	t.Helper()

	path := filepath.Join(dir, base)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// linkAt symlinks path to target.
func linkAt(t *testing.T, target, path string) {
	t.Helper()

	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink %s -> %s: %v", path, target, err)
	}
}

// unreadableDir stages a themes directory holding one valid theme and then
// removes every permission bit, so the only thing wrong with it is that it
// cannot be listed.
//
// Mode bits do not deny root, so the fixture is impossible there and the test
// skips rather than asserting something the platform will not do. The mode is
// restored on cleanup so the temp dir tears down predictably.
func unreadableDir(t *testing.T) string {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("running as root: mode bits do not deny, so an unreadable directory cannot be staged")
	}

	dir := filepath.Join(t.TempDir(), "themes")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())

	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Errorf("restore mode on %s: %v", dir, err)
		}
	})

	return dir
}

// TestEnumerate_UsableDirectoryWithNoCandidatesIsEmptyNotNil pins the shape of a
// readable themes directory holding nothing to enumerate: an empty slice, not
// nil, so "the directory is usable and holds no themes" reads the same as every
// other empty answer this package gives.
func TestEnumerate_UsableDirectoryWithNoCandidatesIsEmptyNotNil(t *testing.T) {
	dir := t.TempDir()

	entries, rejection := theme.Loader{}.Enumerate(dir)

	if rejection != nil {
		t.Fatalf("Enumerate(%q) = %v, want no rejection — an empty directory is usable", dir, rejection)
	}
	if entries == nil {
		t.Errorf("Enumerate(%q) returned nil entries, want an empty slice", dir)
	}
	if len(entries) != 0 {
		t.Errorf("Enumerate(%q) returned %+v, want no entries", dir, entries)
	}
}
