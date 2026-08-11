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

	for _, entry := range entries {
		badName := entry.Rejection != nil && entry.Rejection.Reason == theme.ReasonBadName
		if gotEmpty := entry.Slug == ""; gotEmpty != badName {
			t.Errorf("entry %q slug = %q with rejection %v, want it empty exactly when the reason is %q", entry.Filename, entry.Slug, entry.Rejection, theme.ReasonBadName)
		}
	}
}

func requireSingleEntry(t *testing.T, entries []theme.Entry) theme.Entry {
	t.Helper()

	if len(entries) != 1 {
		t.Fatalf("Enumerate() returned %d entries, want exactly 1: %+v", len(entries), entries)
	}
	return entries[0]
}

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

func filenamesOf(entries []theme.Entry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Filename)
	}
	return names
}

func writeFile(t *testing.T, dir, base, contents string) string {
	t.Helper()

	path := filepath.Join(dir, base)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func linkAt(t *testing.T, target, path string) {
	t.Helper()

	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink %s -> %s: %v", path, target, err)
	}
}

// Mode bits do not deny root, so this fixture is impossible there and the test
// skips. The mode is restored on cleanup so the temp dir tears down.
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
