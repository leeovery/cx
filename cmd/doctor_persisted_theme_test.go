// Tests in this file seed PORTAL_* env vars and mutate package-level Cobra/DI
// state (doctorDeps, rootCmd), so they MUST NOT use t.Parallel.
//
// Concern-split from cmd/doctor_theme_test.go, which owns the same source file's
// FIRST producer — the themes-directory scan. This file owns the SECOND: the
// persisted-theme producer, which reads prefs.json through the non-migrating
// variant and reports the pinned copy's `does not resolve` line for a slug the user chose
// that no longer answers to anything.
//
// The two are split rather than merged because their inputs are different files
// and their failure modes are different diagnoses: the scan is about what is IN a
// directory, this is about what a user PICKED. The union of the two blocks, and
// its pinned order, belong to neither and are asserted in doctor_theme_union_test.go.
package cmd

import (
	"encoding/json"
	"go/ast"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/sourceguard"
	"github.com/leeovery/portal/internal/theme"
)

// prefsJSONWith renders a prefs.json body from raw key/value pairs, so a fixture
// can carry a control character or an escape sequence VERBATIM without the test
// hand-rolling JSON's own escaping alongside it.
func prefsJSONWith(t *testing.T, keys map[string]string) string {
	t.Helper()

	data, err := json.Marshal(keys)
	if err != nil {
		t.Fatalf("marshal prefs fixture %v: %v", keys, err)
	}
	return string(data)
}

// persistedThemeDeps seeds prefs.json with content — an empty string leaves the
// file absent — and returns doctor's deps carrying a store built by the
// PRODUCTION non-migrating read, so every test below drives the same load path
// resolveDoctorDeps does.
func persistedThemeDeps(t *testing.T, content, themesDir string) *DoctorDeps {
	t.Helper()

	setPrefsFile(t, content)
	store, err := loadPrefsStoreNoMigrate()
	if err != nil {
		t.Fatalf("loadPrefsStoreNoMigrate: %v", err)
	}
	return &DoctorDeps{PrefsStore: store, ThemesDir: themesDir}
}

// persistedAdvisoriesFor runs the persisted-theme producer over a seeded
// prefs.json and a themes directory.
func persistedAdvisoriesFor(t *testing.T, content, themesDir string) []themeAdvisory {
	t.Helper()

	return persistedAdvisoriesUnder(t, persistedThemeDeps(t, content, themesDir), theme.NewSilentLoader())
}

// persistedAdvisoriesUnder runs the persisted-theme producer over deps, against
// the deps' themes directory enumerated the way the union enumerates it — so
// every fixture here resolves through the same retained parse doctor resolves
// through.
func persistedAdvisoriesUnder(t *testing.T, deps *DoctorDeps, loader theme.Loader) []themeAdvisory {
	t.Helper()

	return persistedThemeAdvisories(deps, loader, enumerateThemesDir(loader, deps.ThemesDir))
}

// requireNoAdvisories fails unless the producer stayed silent, naming what it
// produced instead.
func requireNoAdvisories(t *testing.T, advisories []themeAdvisory) {
	t.Helper()

	if len(advisories) != 0 {
		t.Fatalf("producer emitted %d advisories, want none:\n  %s", len(advisories), strings.Join(advisoryLines(advisories), "\n  "))
	}
}

// requireDropInSlug fails when slug names a built-in — the vacuity guard every
// deliberately-unresolvable fixture needs, since a built-in resolves from the
// embedded set with no directory involved and would produce no line at all.
func requireDropInSlug(t *testing.T, slug string) {
	t.Helper()

	for _, builtin := range theme.BuiltinSlugs() {
		if builtin == slug {
			t.Fatalf("%q is a built-in slug (the embedded set is %v) — it resolves from the embedded set, so an unresolvable-slug assertion over it would prove nothing", slug, theme.BuiltinSlugs())
		}
	}
}

// TestPersistedThemeAdvisory_ConstantOmitsSlot: it reports an unresolvable
// constant with no slot parenthetical.
//
// The pinned copy omits the parenthetical entirely under a constant, because the
// constant-or-pair rule's constant state HAS no slots: a `(light)` there would assert a
// distinction the two-state setting does not have. The identity fields ride alongside for
// doctor's union — a persisted line is the one that outranks a file line for the
// same slug, and `fromPrefs` is what says so.
func TestPersistedThemeAdvisory_ConstantOmitsSlot(t *testing.T) {
	requireDropInSlug(t, "nord-lee")

	got := requireOneAdvisory(t, persistedAdvisoriesFor(t, `{"theme":"nord-lee"}`, t.TempDir()))

	const want = "⚠ theme nord-lee does not resolve: not found"
	if got.line != want {
		t.Errorf("advisory line = %q; want %q", got.line, want)
	}
	if strings.Contains(got.line, "(") {
		t.Errorf("advisory line = %q carries a slot parenthetical; a constant setting has no slots", got.line)
	}
	if got.slug != "nord-lee" {
		t.Errorf("advisory slug = %q; want %q — §12.2's union dedups on it", got.slug, "nord-lee")
	}
	if !got.fromPrefs {
		t.Error("advisory fromPrefs = false; this line comes from prefs.json and outranks a file line for the same slug")
	}
}

// TestPersistedThemeAdvisory_SlotRendersLightOrDark: it reports an unresolvable
// slot by name.
//
// Under the constant-or-pair rule's adaptive pair the slot is what tells the user WHICH half
// to go and fix — the whole reason the pinned copy carries a parenthetical at all.
func TestPersistedThemeAdvisory_SlotRendersLightOrDark(t *testing.T) {
	requireDropInSlug(t, "solar")

	cases := []struct{ name, content, want string }{
		{
			name:    "the light slot",
			content: `{"theme_light":"solar"}`,
			want:    "⚠ theme solar (light) does not resolve: not found",
		},
		{
			name:    "the dark slot",
			content: `{"theme_dark":"solar"}`,
			want:    "⚠ theme solar (dark) does not resolve: not found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := requireOneAdvisory(t, persistedAdvisoriesFor(t, tc.content, t.TempDir()))
			if got.line != tc.want {
				t.Errorf("advisory line = %q; want %q", got.line, tc.want)
			}
			if !got.fromPrefs {
				t.Error("advisory fromPrefs = false; this line comes from prefs.json")
			}
		})
	}
}

// TestPersistedThemeAdvisory_BothSlots: it collapses two slots naming one slug
// to a single both line.
//
// The row-rendering rule's `● both` state is reachable in two keypresses, so it is an ordinary
// setting rather than a curiosity — and doctor's one-slug-one-line rule means it
// must produce ONE line, not two. The two-different-slugs case is the contrast
// that keeps the collapse honest: it is a collapse of one slug named twice, not
// a cap of one line per report.
func TestPersistedThemeAdvisory_BothSlots(t *testing.T) {
	t.Run("one slug in both slots collapses to a single line", func(t *testing.T) {
		requireDropInSlug(t, "solar")

		got := requireOneAdvisory(t, persistedAdvisoriesFor(t, `{"theme_light":"solar","theme_dark":"solar"}`, t.TempDir()))

		const want = "⚠ theme solar (both) does not resolve: not found"
		if got.line != want {
			t.Errorf("advisory line = %q; want %q", got.line, want)
		}
	})

	t.Run("two slugs stay two lines in slot order", func(t *testing.T) {
		requireDropInSlug(t, "solar")
		requireDropInSlug(t, "gruv")

		got := advisoryLines(persistedAdvisoriesFor(t, `{"theme_light":"solar","theme_dark":"gruv"}`, t.TempDir()))
		want := []string{
			"⚠ theme solar (light) does not resolve: not found",
			"⚠ theme gruv (dark) does not resolve: not found",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("advisory lines =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		}
	})
}

// TestPersistedThemeAdvisory_ConstantWinsOverSlots: it reports only the keys in
// force.
//
// The construction-time load rule: doctor reports the keys IN FORCE, which under the
// constant-or-pair rule's `theme`-wins tiebreak is the constant alone. A hand-edited file may
// legally carry all three keys, and reporting the stale slot would send the user to fix
// something Portal is not reading.
//
// The second half is what stops the first being vacuous: the constant IS checked
// on the very same fixture shape, so "no line for the slot" is evidence about the
// tiebreak rather than about a producer that reports nothing.
func TestPersistedThemeAdvisory_ConstantWinsOverSlots(t *testing.T) {
	requireBuiltinSlug(t, "nord")
	requireDropInSlug(t, "broken")

	t.Run("a resolvable constant silences a broken slot", func(t *testing.T) {
		requireNoAdvisories(t, persistedAdvisoriesFor(t, `{"theme":"nord","theme_dark":"broken"}`, t.TempDir()))
	})

	t.Run("an unresolvable constant is still the only line", func(t *testing.T) {
		requireDropInSlug(t, "nord-lee")

		got := requireOneAdvisory(t, persistedAdvisoriesFor(t, `{"theme":"nord-lee","theme_dark":"broken"}`, t.TempDir()))

		const want = "⚠ theme nord-lee does not resolve: not found"
		if got.line != want {
			t.Errorf("advisory line = %q; want %q", got.line, want)
		}
	})
}

// TestPersistedThemeAdvisory_VirginInstallIsSilent: it produces no line for an
// unset slot.
//
// The shipped adaptive default: an unset slot holds the shipped default, which is a built-in
// and always resolves — so a virgin install produces nothing, and the SET half of a partial
// pair is the only thing reported. An unresolvable built-in is the startup fatal,
// not an advisory.
func TestPersistedThemeAdvisory_VirginInstallIsSilent(t *testing.T) {
	cases := []struct{ name, content string }{
		{name: "an absent prefs.json", content: ""},
		{name: "an empty object", content: `{}`},
		{name: "no theme keys at all", content: `{"session_list_mode":"by-tag"}`},
		{name: "a retained appearance and nothing else", content: `{"appearance":"dark"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireNoAdvisories(t, persistedAdvisoriesFor(t, tc.content, t.TempDir()))
		})
	}

	t.Run("only the set half of a partial pair is reported", func(t *testing.T) {
		requireDropInSlug(t, "solar")

		got := requireOneAdvisory(t, persistedAdvisoriesFor(t, `{"theme_dark":"solar"}`, t.TempDir()))
		if want := "⚠ theme solar (dark) does not resolve: not found"; got.line != want {
			t.Errorf("advisory line = %q; want %q — the unset light slot holds the shipped default and resolves", got.line, want)
		}
	})

	t.Run("an unset slot is not checked even when no built-in resolves", func(t *testing.T) {
		requireDropInSlug(t, "solar")

		// The RAW keys are what say which values are persisted; the Setting has
		// already substituted a shipped default into the unset slot. In a correct
		// binary the two are indistinguishable — the substituted default is a
		// built-in and always resolves — so the difference is only observable on
		// the build-time guarantee's should-never-happen binary, staged here. Checking the Setting's
		// slots would print a line naming a value THE USER NEVER CHOSE, for a
		// state that is the startup fatal rather than an advisory.
		dir := t.TempDir()
		loader := theme.NewSilentLoader()
		loader.BuiltinSource = func(string) ([]byte, bool) { return nil, false }

		// Vacuity guard: under this loader the shipped default really would fail
		// to resolve, so its absence below is evidence about which key was read.
		if _, rejection := loader.ResolveByName(theme.DefaultLightSlug, dir); rejection == nil {
			t.Fatalf("the staged loader still resolves the shipped light default %q — the assertion below would be vacuous", theme.DefaultLightSlug)
		}

		deps := persistedThemeDeps(t, `{"theme_dark":"solar"}`, dir)
		got := advisoryLines(persistedAdvisoriesUnder(t, deps, loader))
		want := []string{"⚠ theme solar (dark) does not resolve: not found"}
		if !slices.Equal(got, want) {
			t.Errorf("advisory lines =\n  %s\nwant\n  %s — only a PERSISTED key produces a line", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		}
	})
}

// TestPersistedThemeAdvisory_ValidSlugIsSilent: it produces no line for a slug
// that resolves.
//
// The producer reports PROBLEMS, not inventory — the same rule the file scan
// follows — and it reports them for both routes a slug can resolve by: the
// embedded set (the construction-time load rule's first step, with no directory involved) and
// a valid drop-in in the themes directory.
func TestPersistedThemeAdvisory_ValidSlugIsSilent(t *testing.T) {
	t.Run("a built-in", func(t *testing.T) {
		requireBuiltinSlug(t, "nord")

		requireNoAdvisories(t, persistedAdvisoriesFor(t, `{"theme":"nord"}`, t.TempDir()))
	})

	t.Run("a valid drop-in", func(t *testing.T) {
		requireDropInSlug(t, "nord-lee")
		dir := themesDirWith(t, map[string][]byte{"nord-lee.theme": validThemeSource(t)})

		requireNoAdvisories(t, persistedAdvisoriesFor(t, `{"theme":"nord-lee"}`, dir))
	})

	t.Run("a valid drop-in in each slot", func(t *testing.T) {
		requireDropInSlug(t, "nord-lee")
		requireDropInSlug(t, "solar")
		dir := themesDirWith(t, map[string][]byte{
			"nord-lee.theme": validThemeSource(t),
			"solar.theme":    validThemeSource(t),
		})

		requireNoAdvisories(t, persistedAdvisoriesFor(t, `{"theme_light":"solar","theme_dark":"nord-lee"}`, dir))
	})
}

// TestPersistedThemeAdvisory_CharsetFailureIsBadName: it reports a
// charset-failing value as bad name.
//
// The validate-before-use rule: the persisted value comes from a hand-editable file and is
// used to LOCATE A FILE BY NAME on a path that deliberately does not enumerate, so
// `../something` would be used verbatim as a path component. The charset check
// runs BEFORE any path is composed, and its reason is `bad name` rather than
// `not found` — telling a user their file is missing when they typed an
// illegal name sends them looking in the wrong place.
//
// The traversal case plants a perfectly loadable theme exactly where a naive
// join would land, so "no path was composed" is a real assertion rather than an
// unfalsifiable claim: had the value reached a Join, that file would have loaded
// and this producer would have printed nothing at all.
func TestPersistedThemeAdvisory_CharsetFailureIsBadName(t *testing.T) {
	cases := []struct{ name, value string }{
		{name: "a path traversal", value: "../evil"},
		{name: "an uppercase letter", value: "Nord"},
		{name: "an underscore", value: "nord_lee"},
		{name: "a space", value: "nord lee"},
		{name: "a leading hyphen", value: "-nord"},
		{name: "an absolute path", value: "/etc/passwd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := prefsJSONWith(t, map[string]string{"theme": tc.value})

			got := requireOneAdvisory(t, persistedAdvisoriesFor(t, content, t.TempDir()))
			want := "⚠ theme " + tc.value + " does not resolve: bad name"
			if got.line != want {
				t.Errorf("advisory line = %q; want %q", got.line, want)
			}
			if strings.Contains(got.line, string(theme.ReasonNotFound)) {
				t.Errorf("advisory line = %q reports %q; an illegal name sends the user to re-type it, not to look for a file", got.line, theme.ReasonNotFound)
			}
		})
	}

	t.Run("a traversal composes no path", func(t *testing.T) {
		root := t.TempDir()
		dir := themesDirIn(t, root, nil)
		// Exactly where filepath.Join(dir, "../evil.theme") lands.
		planted := filepath.Join(root, "evil.theme")
		if err := os.WriteFile(planted, validThemeSource(t), 0o644); err != nil {
			t.Fatalf("plant %s: %v", planted, err)
		}

		// Vacuity guard: the plant really is a theme this resolver would load, so
		// the assertion below is about the charset check refusing to compose the
		// path rather than about a file that would have failed anyway.
		requireNoAdvisories(t, persistedAdvisoriesFor(t, `{"theme":"evil"}`, root))

		got := requireOneAdvisory(t, persistedAdvisoriesFor(t, `{"theme":"../evil"}`, dir))
		if want := "⚠ theme ../evil does not resolve: bad name"; got.line != want {
			t.Errorf("advisory line = %q; want %q — the planted file must never be reached", got.line, want)
		}
	})
}

// TestPersistedThemeAdvisory_NotFoundVersusUnreadable: it distinguishes not
// found from unreadable.
//
// The directory-resolution rule: a theme made unreachable by an UNUSABLE directory carries
// `unreadable`, never `not found`. The two send the user to different places — `not found`
// says check the filename, `unreadable` says check permissions — and permissions
// is the actual problem. An ABSENT directory is the ordinary case and stays
// `not found`.
func TestPersistedThemeAdvisory_NotFoundVersusUnreadable(t *testing.T) {
	requireDropInSlug(t, "nord-lee")

	cases := []struct {
		name string
		make func(*testing.T) string
		want string
	}{
		{
			name: "an absent directory is not found",
			make: func(t *testing.T) string { return filepath.Join(t.TempDir(), "themes") },
			want: "⚠ theme nord-lee does not resolve: not found",
		},
		{
			name: "an empty directory is not found",
			make: func(t *testing.T) string { return t.TempDir() },
			want: "⚠ theme nord-lee does not resolve: not found",
		},
		{
			name: "a mode-0000 directory is unreadable",
			make: func(t *testing.T) string {
				skipUnlessModeBitsDeny(t)
				dir := themesDirWith(t, map[string][]byte{"nord-lee.theme": validThemeSource(t)})
				if err := os.Chmod(dir, 0o000); err != nil {
					t.Fatalf("chmod 0000 %s: %v", dir, err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
				return dir
			},
			want: "⚠ theme nord-lee does not resolve: unreadable",
		},
		{
			name: "a regular file where the directory belongs is unreadable",
			make: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "themes")
				if err := os.WriteFile(path, []byte("not a directory\n"), 0o644); err != nil {
					t.Fatalf("seed %s: %v", path, err)
				}
				return path
			},
			want: "⚠ theme nord-lee does not resolve: unreadable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := requireOneAdvisory(t, persistedAdvisoriesFor(t, `{"theme":"nord-lee"}`, tc.make(t)))
			if got.line != tc.want {
				t.Errorf("advisory line = %q; want %q", got.line, tc.want)
			}
		})
	}
}

// TestPersistedThemeAdvisory_UnresolvedThemesDirStillReports: it still reports a
// persisted slug with no themes directory.
//
// The unresolved-path degradation is scoped to the DIRECTORY SCAN, which has
// nothing to enumerate — this producer needs no path at all. The construction-time load rule's
// embedded set resolves first, so a built-in still answers and a drop-in slug is honestly
// `not found`, with no path composed (a join onto "" would yield a RELATIVE path
// resolving against the process's working directory). The scan's silence over
// the same deps is asserted alongside, since that asymmetry is the whole point.
func TestPersistedThemeAdvisory_UnresolvedThemesDirStillReports(t *testing.T) {
	requireDropInSlug(t, "nord-lee")

	deps := persistedThemeDeps(t, `{"theme":"nord-lee"}`, "")
	loader := theme.NewSilentLoader()

	got := requireOneAdvisory(t, persistedAdvisoriesUnder(t, deps, loader))
	if want := "⚠ theme nord-lee does not resolve: not found"; got.line != want {
		t.Errorf("advisory line = %q; want %q", got.line, want)
	}

	requireNoAdvisories(t, scanThemesDirectory(enumerateThemesDir(loader, deps.ThemesDir)))

	t.Run("a built-in still resolves with no directory", func(t *testing.T) {
		requireBuiltinSlug(t, "nord")

		requireNoAdvisories(t, persistedAdvisoriesUnder(t, persistedThemeDeps(t, `{"theme":"nord"}`, ""), loader))
	})
}

// TestPersistedThemeAdvisory_ControlStrippedUntruncated: it renders the slug
// control-stripped and untruncated.
//
// The row-rendering rule: stripping happens at the point the value is READ, so it is a
// property of the value every consumer inherits — a pasted newline would otherwise split
// this frame into two lines, the second looking like a message Portal never
// wrote, and a bare escape sequence would corrupt the very surface the user is
// reading in order to FIND the problem. TRUNCATION IS SEPARATE and stays
// panel-local: doctor has full width and wants the whole value, however long.
func TestPersistedThemeAdvisory_ControlStrippedUntruncated(t *testing.T) {
	// Long enough that no terminal width could carry it, and short enough that
	// the composed basename stays inside the filesystem's per-name limit — past
	// that the resolver honestly answers `unreadable` (ENAMETOOLONG against a
	// healthy directory), which would be a different claim entirely.
	stripped := strings.Repeat("nord-lee-", 12)
	requireDropInSlug(t, stripped)
	raw := "\x1b[31m" + stripped + "\n\t"

	content := prefsJSONWith(t, map[string]string{"theme": raw})

	got := requireOneAdvisory(t, persistedAdvisoriesFor(t, content, t.TempDir()))
	want := "⚠ theme " + stripped + " does not resolve: not found"
	if got.line != want {
		t.Errorf("advisory line = %q; want %q", got.line, want)
	}
	if strings.ContainsAny(got.line, "\x1b\n\t") {
		t.Errorf("advisory line = %q still carries a control character or escape", got.line)
	}
	if strings.Contains(got.line, "[31m") {
		t.Errorf("advisory line = %q carries the escape's printable tail; the strip is a terminal-grammar parse, not a byte filter", got.line)
	}
	if got.slug != stripped {
		t.Errorf("advisory slug = %q; want the stripped value %q — the union dedups on it", got.slug, stripped)
	}
}

// TestPersistedThemeAdvisory_ControlOnlyValueIsUnset: it treats a control-only
// value as unset.
//
// A value that strips to empty is UNSET rather than an illegal slug: the row-rendering rule
// makes the stripped form "the value" for every consumer, and the anchored slug charset
// makes the empty string the unambiguous unset sentinel. A `bad name` line
// labelled with an empty string would name nothing.
//
// The last case is what keeps the rest honest: a control-only constant leaves the
// SLOTS in force, so the read is genuinely continuing rather than being abandoned.
func TestPersistedThemeAdvisory_ControlOnlyValueIsUnset(t *testing.T) {
	const controlOnly = "\x1b[0m\n\t"

	cases := []struct {
		name string
		keys map[string]string
	}{
		{name: "a control-only constant", keys: map[string]string{"theme": controlOnly}},
		{name: "a control-only light slot", keys: map[string]string{"theme_light": controlOnly}},
		{name: "a control-only dark slot", keys: map[string]string{"theme_dark": controlOnly}},
		{name: "control-only everywhere", keys: map[string]string{"theme": controlOnly, "theme_light": controlOnly, "theme_dark": controlOnly}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireNoAdvisories(t, persistedAdvisoriesFor(t, prefsJSONWith(t, tc.keys), t.TempDir()))
		})
	}

	t.Run("a control-only constant leaves the slots in force", func(t *testing.T) {
		requireDropInSlug(t, "solar")
		content := prefsJSONWith(t, map[string]string{"theme": controlOnly, "theme_dark": "solar"})

		got := requireOneAdvisory(t, persistedAdvisoriesFor(t, content, t.TempDir()))
		if want := "⚠ theme solar (dark) does not resolve: not found"; got.line != want {
			t.Errorf("advisory line = %q; want %q — a constant that strips to empty never wins the tiebreak", got.line, want)
		}
	})
}

// TestPersistedThemeSlotLabel_ReadsTheSlotsOwnName: the parenthetical is the
// slot's own word, and `both` is doctor's.
//
// The light/dark words are one vocabulary shared with the `theme` component's
// `slot` attr, so doctor renders whatever theme.Slot names itself rather than a
// pair of literals kept beside it — a slot added to the vocabulary must arrive
// here rendered, not silently labelled with nothing.
//
// `both` is the exception and is genuinely doctor's own: one slug occupying the
// whole pair sits in no single slot, so there is nothing for AttrName to name it
// by.
func TestPersistedThemeSlotLabel_ReadsTheSlotsOwnName(t *testing.T) {
	for _, slot := range []theme.Slot{theme.SlotConstant, theme.SlotLight, theme.SlotDark} {
		want, _ := slot.AttrName()
		if got := persistedThemeSlotLabel(theme.InForceKey{Slot: slot}); got != want {
			t.Errorf("persistedThemeSlotLabel(slot %v) = %q; want theme.Slot's own name %q", slot, got, want)
		}
	}

	if got := persistedThemeSlotLabel(theme.InForceKey{Slot: theme.SlotLight, Both: true}); got != themeSlotBoth {
		t.Errorf("persistedThemeSlotLabel(both) = %q; want %q", got, themeSlotBoth)
	}

	t.Run("the words are declared nowhere in doctor", func(t *testing.T) {
		// The behavioural half above passes over a switch that restates the two
		// words, since a restatement agrees with the definition until the day it
		// does not. What makes the derivation structural is that the words — and a
		// per-slot branch to render them from — are absent from doctor entirely.
		light, _ := theme.SlotLight.AttrName()
		dark, _ := theme.SlotDark.AttrName()
		for _, word := range []string{light, dark} {
			if n := cmdLiteralSites(t, word)["doctor_theme.go"]; n != 0 {
				t.Errorf("doctor_theme.go declares the literal %q %d times; the light/dark words live in internal/theme alone", word, n)
			}
		}

		var arms []string
		ast.Inspect(parsePackageFilesByName(t)["doctor_theme.go"], func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "theme" && strings.HasPrefix(sel.Sel.Name, "Slot") {
				arms = append(arms, sel.Sel.Name)
			}
			return true
		})
		if len(arms) != 0 {
			t.Errorf("doctor_theme.go names %v; a slot added to the vocabulary would compile here and render no label at all", arms)
		}
	})
}

// TestPersistedThemeAdvisory_TolerantOnDegeneratePrefs: it tolerates an absent or
// corrupt prefs file.
//
// Every degenerate prefs.json yields zero keys with no error, so it yields zero
// lines — and the read's error is discarded deliberately. The one thing a
// diagnosis must not do is fail to diagnose because one of the files it reads is
// the broken one.
//
// The nil-store case is the unresolvable-config-path degradation: the advisory
// class has no not-evaluable form, so silence is the only shape available.
func TestPersistedThemeAdvisory_TolerantOnDegeneratePrefs(t *testing.T) {
	cases := []struct{ name, content string }{
		{name: "an absent file", content: ""},
		{name: "an empty file", content: " "},
		{name: "a stray comma", content: `{"theme":"nord-lee",}`},
		{name: "a truncated document", content: `{"theme":`},
		{name: "a top-level array", content: `["nord-lee"]`},
		{name: "a top-level string", content: `"nord-lee"`},
		{name: "a wrong-typed theme key", content: `{"theme":42}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireNoAdvisories(t, persistedAdvisoriesFor(t, tc.content, t.TempDir()))
		})
	}

	t.Run("an unreadable file", func(t *testing.T) {
		skipUnlessModeBitsDeny(t)

		deps := persistedThemeDeps(t, `{"theme":"nord-lee"}`, t.TempDir())
		path, err := prefsFilePath()
		if err != nil {
			t.Fatalf("prefsFilePath: %v", err)
		}
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatalf("chmod 0000 %s: %v", path, err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
		// Called for the vacuity guard alone: a readable fixture would carry a
		// theme key and the assertion would be arguing about the wrong run.
		_ = requireDeniedRead(t, path)

		requireNoAdvisories(t, persistedAdvisoriesUnder(t, deps, theme.NewSilentLoader()))
	})

	t.Run("a nil store", func(t *testing.T) {
		requireNoAdvisories(t, persistedAdvisoriesUnder(t, &DoctorDeps{ThemesDir: t.TempDir()}, theme.NewSilentLoader()))
	})

	t.Run("a corrupt file never aborts the diagnosis", func(t *testing.T) {
		setPrefsFile(t, `{"theme":`)

		outBuf, _, err := runDoctorCmd(t, healthyDoctorDeps(t))
		if err != nil {
			t.Fatalf("Execute err = %v; a corrupt prefs.json must never abort the diagnosis", err)
		}
		if out := outBuf.String(); !strings.HasSuffix(out, "\n  7 checks passed\n") {
			t.Errorf("report does not close with the full all-passed summary:\n%s", out)
		}
	})
}

// TestPersistedThemeAdvisory_UsesNonMigratingRead: it reads prefs through the
// non-migrating variant.
//
// Doctor's theme line: doctor's contract is that it HEALS NOTHING on the read-only path, and a
// one-shot config mutation as a side effect of running a diagnosis is exactly
// what would break it. The write-path ownership rule's translation therefore stays behind
// loadPrefsStore, which doctor must never reach.
//
// The Execute half is also the production-wiring proof: no PrefsStore is set on
// the deps, so the line below can only have arrived through resolveDoctorDeps'
// own best-effort loadPrefsStoreNoMigrate call — delete that one line and an
// override-only suite stays green while doctor silently reports nothing about a
// real user's prefs.json.
func TestPersistedThemeAdvisory_UsesNonMigratingRead(t *testing.T) {
	t.Run("a pending appearance translation is left pending", func(t *testing.T) {
		const seeded = `{"appearance":"dark"}`
		path := setPrefsFile(t, seeded)

		// Vacuity guard: THIS fixture is the one the migrating read would
		// translate, so the byte-identity below is evidence about doctor rather
		// than about a file nothing would have touched anyway.
		load, err := loadPrefsStore()
		if err != nil {
			t.Fatalf("loadPrefsStore: %v", err)
		}
		if !load.TranslationPending || load.TranslatedSlug == "" {
			t.Fatalf("the migrating read computes pending=%t slug=%q over this fixture; it would translate nothing, so the assertions below would be vacuous", load.TranslationPending, load.TranslatedSlug)
		}

		if _, _, err := runDoctorCmd(t, healthyDoctorDeps(t)); err != nil {
			t.Fatalf("Execute err = %v; want nil over a healthy catalog", err)
		}

		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back prefs.json: %v", err)
		}
		if string(after) != seeded {
			t.Errorf("prefs.json = %s; want it byte-identical: %s", after, seeded)
		}
		if strings.Contains(string(after), "theme_migrated") {
			t.Errorf("prefs.json = %s; the migration marker must still be absent after a diagnosis", after)
		}
	})

	t.Run("the persisted theme is still reported", func(t *testing.T) {
		requireDropInSlug(t, "nord-lee")
		const seeded = `{"appearance":"dark","theme":"nord-lee"}`
		path := setPrefsFile(t, seeded)

		outBuf, _, err := runDoctorCmd(t, healthyDoctorDeps(t))
		if err != nil {
			t.Fatalf("Execute err = %v; an unresolvable persisted theme must never drive the exit code", err)
		}

		out := outBuf.String()
		want := "" +
			"  ⚠ theme nord-lee does not resolve: not found\n" +
			"  7 checks passed · 1 advisory\n"
		if !strings.HasSuffix(out, want) {
			t.Errorf("report does not close with the persisted-theme advisory and its summary:\n%s", out)
		}

		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back prefs.json: %v", err)
		}
		if string(after) != seeded {
			t.Errorf("prefs.json = %s; want it byte-identical: %s", after, seeded)
		}
	})

	t.Run("doctor's sources call only the non-migrating read", func(t *testing.T) {
		// The runtime halves above cannot see a migrating read whose translation
		// happened to be a no-op on the day, and the structural guard is what makes
		// the rule survive a future edit:
		// TestLoadPrefsStore_SingleProductionCaller pins the migrating loader to
		// open.go:openTUI, and this pins the other side — doctor reaches only the
		// inert variant.
		var migrating, nonMigrating []string
		for name, file := range parsePackageFilesByName(t) {
			if !strings.HasPrefix(name, "doctor") {
				continue
			}
			sourceguard.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
				ident, ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				switch ident.Name {
				case "loadPrefsStore":
					migrating = append(migrating, name+":"+funcName)
				case "loadPrefsStoreNoMigrate":
					nonMigrating = append(nonMigrating, name+":"+funcName)
				}
				return true
			})
		}

		if len(migrating) != 0 {
			t.Errorf("doctor calls the MIGRATING loadPrefsStore from %v; a diagnosis must never trigger the one-shot appearance translation", migrating)
		}
		if want := []string{"doctor.go:resolveDoctorDeps"}; !slices.Equal(nonMigrating, want) {
			t.Errorf("loadPrefsStoreNoMigrate is called from %v; want %v", nonMigrating, want)
		}
	})
}

// TestPersistedThemeAdvisory_NoFallbackAndNoFatal: it resolves without fallbacks
// and never raises the fatal.
//
// ResolveNomination is the wrong resolver here TWICE OVER: it substitutes the per-slot
// fallback rule's mode-matched default for an unloadable nomination — which would hide the
// very failure this line exists to report — and it raises the build-time guarantee's
// broken-built-in fatal when a fallback itself will not resolve, which would abort a diagnosis
// over a state a diagnosis is supposed to describe.
//
// The broken-binary subtest stages exactly that fatal and proves the producer
// walks straight past it, with the guard that the SAME loader really does raise
// it through ResolveNomination.
func TestPersistedThemeAdvisory_NoFallbackAndNoFatal(t *testing.T) {
	t.Run("it names the persisted slug, never the fallback", func(t *testing.T) {
		requireDropInSlug(t, "solar")
		requireDropInSlug(t, "gruv")

		got := advisoryLines(persistedAdvisoriesFor(t, `{"theme_light":"solar","theme_dark":"gruv"}`, t.TempDir()))
		want := []string{
			"⚠ theme solar (light) does not resolve: not found",
			"⚠ theme gruv (dark) does not resolve: not found",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("advisory lines =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		}
		for _, line := range got {
			for _, fallback := range []string{theme.DefaultLightSlug, theme.DefaultDarkSlug} {
				if strings.Contains(line, fallback) {
					t.Errorf("advisory line = %q names the fallback %q; the line must report the slug the user chose", line, fallback)
				}
			}
		}
	})

	t.Run("a binary whose embedded set cannot supply a fallback still reports", func(t *testing.T) {
		requireDropInSlug(t, "solar")
		requireDropInSlug(t, "gruv")

		dir := t.TempDir()
		loader := theme.NewSilentLoader()
		// The build-time guarantee's should-never-happen binary, staged through the one seam that
		// can: no built-in resolves at all, so every fallback fails.
		loader.BuiltinSource = func(string) ([]byte, bool) { return nil, false }

		// Vacuity guard: the staged loader really does raise the fatal on the
		// resolver this producer refuses to use.
		setting, _ := theme.ResolveSetting(theme.RawKeys{Light: "solar", Dark: "gruv"})
		if _, err := loader.ResolveNomination(setting, dir); err == nil {
			t.Fatal("the staged loader raises no fatal through ResolveNomination — the assertion below would be vacuous")
		}

		deps := persistedThemeDeps(t, `{"theme_light":"solar","theme_dark":"gruv"}`, dir)
		got := advisoryLines(persistedAdvisoriesUnder(t, deps, loader))
		want := []string{
			"⚠ theme solar (light) does not resolve: not found",
			"⚠ theme gruv (dark) does not resolve: not found",
		}
		if !slices.Equal(got, want) {
			t.Errorf("advisory lines =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		}
	})

	t.Run("doctor's theme source never calls ResolveNomination", func(t *testing.T) {
		calls := doctorThemeCallCounts(t)

		if calls["ResolveNomination"] != 0 {
			t.Errorf("doctor_theme.go calls ResolveNomination %d times; it resolves fallbacks and can raise the fatal, and neither belongs in a diagnosis", calls["ResolveNomination"])
		}
		if calls["ResolveByNameFrom"] == 0 {
			t.Error("doctor_theme.go calls no by-name resolver at all; the guard above would pass over a producer that resolves nothing")
		}
	})
}

// doctorThemeCallCounts counts the calls doctor's theme source makes, by the
// method name each one selects.
func doctorThemeCallCounts(t *testing.T) map[string]int {
	t.Helper()

	source := parsePackageFilesByName(t)["doctor_theme.go"]
	if source == nil {
		t.Fatal("the cmd package declares no doctor_theme.go — the guard has nothing to scan")
	}

	calls := map[string]int{}
	ast.Inspect(source, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			calls[sel.Sel.Name]++
		}
		return true
	})
	return calls
}

// TestThemeAdvisories_DirectoryIsReadOnce: one diagnosis reads the themes
// directory once and opens no theme file after it.
//
// The runtime half of the claim — that neither producer can be made to disagree
// with the other by a file changing under it — is asserted over the assembled
// block in doctor_theme_union_test.go. This is the other half: a re-read that
// happened to AGREE, because nothing disturbed the directory between the two,
// would leave that fixture green while doctor paid for a second ReadDir and a
// second parse of every candidate on every run.
func TestThemeAdvisories_DirectoryIsReadOnce(t *testing.T) {
	calls := doctorThemeCallCounts(t)

	if calls["Enumerate"] != 1 {
		t.Errorf("doctor_theme.go calls Enumerate %d times; the directory is read once per diagnosis and the retained enumeration drives both producers", calls["Enumerate"])
	}
	for _, reader := range []string{"ResolveByName", "LoadFile", "ReadDir", "ReadFile", "Open"} {
		if calls[reader] != 0 {
			t.Errorf("doctor_theme.go calls %s %d times; nothing may open a theme file after the enumeration", reader, calls[reader])
		}
	}
}

// TestPersistedThemeAdvisory_EmitsNoThemeRecords: it emits zero theme log
// records.
//
// The `theme` log component: the `theme` component records where a theme is USED, never where
// one is DIAGNOSED. This is asserted across a WHOLE `portal doctor` Execute rather than
// at the producer, because the loader is shared by both theme producers and the
// claim is about the command: whatever doctor does, nothing lands in the log.
//
// The fixture is deliberately the `unreadable`-directory condition, which is the
// one state that WOULD emit — the resolver reports `theme: directory unusable`
// from the composed-path read as well as from the enumeration — and its vacuity
// guard resolves the identical fixture through a real component logger to prove
// the sink is listening and the condition is emission-worthy.
func TestPersistedThemeAdvisory_EmitsNoThemeRecords(t *testing.T) {
	skipUnlessModeBitsDeny(t)
	requireDropInSlug(t, "nord-lee")

	deniedDir := func(t *testing.T) string {
		t.Helper()
		dir := themesDirWith(t, map[string][]byte{"nord-lee.theme": validThemeSource(t)})
		if err := os.Chmod(dir, 0o000); err != nil {
			t.Fatalf("chmod 0000 %s: %v", dir, err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		return dir
	}

	t.Run("the condition emits through a real component logger", func(t *testing.T) {
		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		loud := theme.NewLoader(theme.NewEventLogger(log.For("theme")))
		if _, rejection := loud.ResolveByName("nord-lee", deniedDir(t)); rejection == nil {
			t.Fatal("the fixture resolved cleanly — the zero-record assertion below would be about the wrong run")
		}

		if events := themeEvents(t, sink); len(events) == 0 {
			t.Fatal("the capture harness recorded no theme events over an unusable directory — the assertion below would be vacuous")
		}
	})

	t.Run("a full doctor run writes nothing", func(t *testing.T) {
		dir := deniedDir(t)
		t.Setenv("PORTAL_THEMES_DIR", dir)
		setPrefsFile(t, `{"theme":"nord-lee"}`)

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		outBuf, _, err := runDoctorCmd(t, healthyDoctorDeps(t))
		if err != nil {
			t.Fatalf("Execute err = %v; theme advisories never drive the exit code", err)
		}

		// Both lines must be PRESENT, since each reaches the emitting condition by
		// its own route — the enumeration's ReadDir and the by-name read's composed
		// path. Their relative ORDER is deliberately not asserted here: the block's
		// pinned order (and the union that dedups it) is asserted in doctor_theme_union_test.go, and
		// pinning it from this side would put the same rule in two places.
		out := outBuf.String()
		for _, want := range []string{
			"  ⚠ themes directory unreadable: " + dir + "\n",
			"  ⚠ theme nord-lee does not resolve: unreadable\n",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("report is missing %q — the zero-record assertion needs both emitting conditions to have been reached:\n%s", want, out)
			}
		}
		if !strings.HasSuffix(out, "\n  7 checks passed · 2 advisories\n") {
			t.Fatalf("report does not close with two advisories counted:\n%s", out)
		}
		if events := themeEvents(t, sink); len(events) != 0 {
			t.Errorf("the run emitted %d theme records, want none:\n  %s", len(events), strings.Join(events, "\n  "))
		}
	})
}

// TestPersistedThemeAdvisory_FrameIsSingleSourced: it states the persisted frame
// exactly once.
//
// The copy is single-sourced following the convention spawn.UnsupportedNoopMessage
// set, and this frame is rendered in two shapes — with and without the slot
// parenthetical — so it is exactly the one a second call site would silently
// duplicate. One format const with an optional insert is what keeps the two
// shapes one string.
func TestPersistedThemeAdvisory_FrameIsSingleSourced(t *testing.T) {
	sites := cmdLiteralSites(t, "does not resolve")

	if want := map[string]int{"doctor_theme.go": 1}; !maps.Equal(sites, want) {
		t.Errorf("the literal %q is declared at %v; want %v — one const in the file that owns doctor's theme copy", "does not resolve", sites, want)
	}
}
