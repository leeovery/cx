package cmd

// Tests in this file seed prefs.json through t.Setenv and swap the process-wide
// log handler, so they MUST NOT use t.Parallel.
//
// They drive the persister DIRECTLY, which is one end of the commit path. The
// other end — the panel keypress that calls it, and what an UNWIRED slot does
// (a persister built over a nil *prefs.Store would box into a NON-NIL seam and
// hand the panel a live-looking writer whose every commit panics, so the slot
// is left empty instead) — is asserted behaviourally over a built model in
// open_theme_commit_test.go. Whether TUI construction leaves that slot empty
// when the store did not load is out of reach from either: openTUI ends in a
// running Bubble Tea program.

import (
	"log/slog"
	"maps"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// prefsStoreForTest builds the process's prefs store WITHOUT the migrating load,
// which is what keeps every assertion below about the persister alone:
// the migrating loader would compute a translation and dispatch a write of its
// own, and cmd's TestMain neutralises that dispatch SILENTLY — so a negative
// assertion taken over the migrating path could pass for a reason that has
// nothing to do with this seam.
func prefsStoreForTest(t *testing.T) *prefs.Store {
	t.Helper()
	store, err := loadPrefsStoreNoMigrate()
	if err != nil {
		t.Fatalf("resolve the prefs store: %v", err)
	}
	return store
}

// TestThemePersister_CommitTheme: it commits a constant through the store.
//
// The fixture seeds BOTH slots and both sibling fields, because the write is only
// correct if it does three things at once: writes `theme`, clears
// both slots in the same atomic write, and merges rather than overwrites — so
// session_list_mode and the retained raw appearance survive.
func TestThemePersister_CommitTheme(t *testing.T) {
	path := setPrefsFile(t, `{"session_list_mode":"by-tag","appearance":"dark","theme_light":"`+theme.DefaultLightSlug+`","theme_dark":"`+theme.DefaultDarkSlug+`"}`)
	persister := newThemePersister(prefsStoreForTest(t))

	if err := persister.CommitTheme(nordSlug); err != nil {
		t.Fatalf("CommitTheme(%s): %v", nordSlug, err)
	}

	assertPrefsOnDisk(t, path, prefsOnDisk{
		SessionListMode: "by-tag",
		Appearance:      "dark",
		Theme:           nordSlug,
	})
}

// TestThemePersister_CommitThemeSlot: it commits a slot through the store.
//
// Both halves are driven, and each fixture pins what the other slot does: the
// constant is cleared (the constant-or-pair rule's mutual exclusion) while the OTHER slot is
// left exactly as it was — the property that makes the row-rendering rule's `● both` reachable
// in two keypresses.
func TestThemePersister_CommitThemeSlot(t *testing.T) {
	for _, tc := range []struct {
		name string
		slot prefs.ThemeSlot
		want prefsOnDisk
	}{
		{"dark", prefs.SlotDark, prefsOnDisk{Appearance: "light", ThemeLight: theme.DefaultLightSlug, ThemeDark: nordSlug}},
		{"light", prefs.SlotLight, prefsOnDisk{Appearance: "light", ThemeLight: nordSlug, ThemeDark: theme.DefaultDarkSlug}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seeded := `{"appearance":"light","theme":"` + theme.DefaultDarkSlug + `","theme_light":"` + theme.DefaultLightSlug + `","theme_dark":"` + theme.DefaultDarkSlug + `"}`
			path := setPrefsFile(t, seeded)
			persister := newThemePersister(prefsStoreForTest(t))

			if err := persister.CommitThemeSlot(nordSlug, tc.slot); err != nil {
				t.Fatalf("CommitThemeSlot(%s, %v): %v", nordSlug, tc.slot, err)
			}

			assertPrefsOnDisk(t, path, tc.want)
		})
	}
}

// TestThemePersister_FailedCommitLogsAndReturns: it logs and returns a failed
// write.
//
// Both halves are asserted together because either alone is the bug the concurrent-write rule
// and the failed-commit rule are written against. Logging without returning leaves the panel
// unable to render `⚠ couldn't save theme` and unable to hold the failure outstanding —
// silently "applied but not persisted", the state the picker idiom exists to
// close. Returning without logging leaves `theme: commit failed` with no emission
// site at all, since nothing else may produce it.
//
// The fixture is a malformed file, so the abort happens at the concurrent-write rule's strict
// re-read and the on-disk bytes are asserted untouched: a write never becomes an
// overwrite.
func TestThemePersister_FailedCommitLogsAndReturns(t *testing.T) {
	const malformed = `{"session_list_mode":"by-tag",`
	path := setPrefsFile(t, malformed)
	persister := newThemePersister(prefsStoreForTest(t))
	sink := installMigrateCapture(t)

	err := persister.CommitTheme(nordSlug)

	if err == nil {
		t.Fatal("CommitTheme returned nil over a malformed prefs.json; Phase 9's outstanding-failure state machine reports the VALUE, not the log")
	}
	assertPrefsUnchanged(t, path, []byte(malformed))

	rec := sink.OnlyRecord(t)
	if rec.Level != slog.LevelWarn {
		t.Errorf("level = %v, want WARN per §12.3", rec.Level)
	}
	if want := "commit failed"; rec.Msg != want {
		t.Errorf("message = %q, want %q verbatim from §12.3's catalogue", rec.Msg, want)
	}
	if got := rec.AttrString(t, "reason"); got != err.Error() {
		t.Errorf("reason = %q, want the returned error %q — the log and the value must describe the same failure", got, err.Error())
	}
}

// TestThemePersister_CommitFailedAttrs: it carries slot only for a slot commit.
//
// The `theme` log component pins the attrs: `slug`, `slot` — ABSENT when committing a constant
// — and `reason`. The absence is the load-bearing half: the attr names which half of an
// adaptive PAIR a line is about, and a constant has no halves, so an empty value
// would grep as a slot named nothing.
func TestThemePersister_CommitFailedAttrs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		commit func(themePersister) error
		want   []string
		slot   string
	}{
		{
			name:   "a constant carries no slot",
			commit: func(p themePersister) error { return p.CommitTheme(nordSlug) },
			want:   []string{"component", "slug", "reason"},
		},
		{
			name:   "a dark slot carries slot=dark",
			commit: func(p themePersister) error { return p.CommitThemeSlot(nordSlug, prefs.SlotDark) },
			want:   []string{"component", "slug", "slot", "reason"},
			slot:   "dark",
		},
		{
			name:   "a light slot carries slot=light",
			commit: func(p themePersister) error { return p.CommitThemeSlot(nordSlug, prefs.SlotLight) },
			want:   []string{"component", "slug", "slot", "reason"},
			slot:   "light",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setPrefsFile(t, `{"appearance":"dark",`)
			persister := newThemePersister(prefsStoreForTest(t))
			sink := installMigrateCapture(t)

			if err := tc.commit(persister); err == nil {
				t.Fatal("the commit succeeded over a malformed prefs.json; there is no failure to describe")
			}

			rec := sink.OnlyRecord(t)
			if !slices.Equal(rec.Keys, tc.want) {
				t.Errorf("attr keys = %v, want %v", rec.Keys, tc.want)
			}
			if got := rec.AttrString(t, "component"); got != "theme" {
				t.Errorf("component = %q, want %q", got, "theme")
			}
			if got := rec.AttrString(t, "slug"); got != nordSlug {
				t.Errorf("slug = %q, want %q — the slug that failed to commit", got, nordSlug)
			}
			if tc.slot != "" && rec.AttrString(t, "slot") != tc.slot {
				t.Errorf("slot = %q, want %q", rec.AttrString(t, "slot"), tc.slot)
			}

			// The `theme` log component's closed attr-key set, asserted key by key so an attr added
			// later reads as a vocabulary breach rather than a changed expectation.
			closed := map[string]bool{"slug": true, "slot": true, "reason": true, "path": true, "token": true, "count": true, "rejected": true}
			for _, key := range rec.Keys {
				if key != "component" && !closed[key] {
					t.Errorf("attr %q is outside §12.3's closed key set %v", key, slices.Sorted(maps.Keys(closed)))
				}
			}
		})
	}

	t.Run("the slot renders exactly as the theme component renders its own", func(t *testing.T) {
		// The two vocabularies are different types (prefs.ThemeSlot here,
		// theme.Slot in the loader's events) but ONE user-visible string, so this
		// reads the loader's rendering back off a real emission rather than
		// restating the literals a second time.
		for _, tc := range []struct {
			prefsSlot prefs.ThemeSlot
			themeSlot theme.Slot
		}{
			{prefs.SlotLight, theme.SlotLight},
			{prefs.SlotDark, theme.SlotDark},
		} {
			sink := installMigrateCapture(t)
			theme.NewEventLogger(themeLogger).Loaded(nordSlug, tc.themeSlot)
			want := sink.OnlyRecord(t).AttrString(t, "slot")

			if got, named := themeSlotAttr(tc.prefsSlot); !named || got != want {
				t.Errorf("themeSlotAttr(%v) = (%q, %v), want (%q, true) — the persister's slot attr must not drift from the loader's", tc.prefsSlot, got, named, want)
			}
		}
	})
}

// TestThemePersister_SuccessIsSilent: it emits nothing on success.
//
// The `theme` log component gives the persister ONE event and it is a failure. The commit-time
// `theme: loaded` belongs to the loader, so a success line here would
// be a second, unspecified record of the same act — and it would dilute the
// forensic trail the component earns its place with.
//
// The assertion is over EVERY record the sink captured, not merely the `theme`
// ones: a success announced under some other component would be just as wrong.
// The failing commit at the end is the sink's own positive control — silence
// asserted against a sink nothing could ever have reached would be vacuous.
func TestThemePersister_SuccessIsSilent(t *testing.T) {
	path := setPrefsFile(t, `{"session_list_mode":"by-tag"}`)
	persister := newThemePersister(prefsStoreForTest(t))
	sink := installMigrateCapture(t)

	if err := persister.CommitTheme(nordSlug); err != nil {
		t.Fatalf("CommitTheme(%s): %v", nordSlug, err)
	}
	if err := persister.CommitThemeSlot(nordSlug, prefs.SlotDark); err != nil {
		t.Fatalf("CommitThemeSlot(%s, dark): %v", nordSlug, err)
	}

	if records := sink.Records(); len(records) != 0 {
		t.Errorf("two successful commits emitted %d record(s): %v, want none", len(records), sink.Lines())
	}

	makePrefsDirUnwritable(t, path)
	if err := persister.CommitTheme(nordSlug); err == nil {
		t.Fatal("the control commit succeeded over an unwritable directory; the silence above is unproven")
	}
	if records := sink.Records(); len(records) != 1 {
		t.Errorf("the failing control commit emitted %d record(s): %v, want exactly 1 — the sink was live all along", len(records), sink.Lines())
	}
}

// TestThemePersister_PerInstanceLastWriteWins: it does not sync across instances.
//
// The concurrent-write rule's concurrency contract, driven with two persisters over two stores
// on one file — the shape the multi-window burst makes ordinary. There is no file watch
// and no cross-instance sync: the second commit simply wins, and every other
// field survives because each write is a read-modify-write rather than a
// whole-record flush of a stale snapshot.
//
// The second commit is a SLOT while the first is a constant, so the assertion
// also shows the later write observing the earlier one's bytes (its clear of
// `theme` is over the value the first instance wrote, not over the seeded file).
func TestThemePersister_PerInstanceLastWriteWins(t *testing.T) {
	path := setPrefsFile(t, `{"session_list_mode":"by-tag","appearance":"dark"}`)
	first := newThemePersister(prefsStoreForTest(t))
	second := newThemePersister(prefsStoreForTest(t))

	if err := first.CommitTheme(nordSlug); err != nil {
		t.Fatalf("first instance CommitTheme: %v", err)
	}
	assertPrefsOnDisk(t, path, prefsOnDisk{SessionListMode: "by-tag", Appearance: "dark", Theme: nordSlug})

	if err := second.CommitThemeSlot(theme.DefaultLightSlug, prefs.SlotLight); err != nil {
		t.Fatalf("second instance CommitThemeSlot: %v", err)
	}

	assertPrefsOnDisk(t, path, prefsOnDisk{
		SessionListMode: "by-tag",
		Appearance:      "dark",
		ThemeLight:      theme.DefaultLightSlug,
	})
}
