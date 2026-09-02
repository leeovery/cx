package cmd

import (
	"log/slog"
	"maps"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// Non-migrating deliberately: the migrating loader would dispatch a write of its
// own, and TestMain neutralises that dispatch silently — so a negative assertion
// over it could pass for a reason unrelated to this seam.
func prefsStoreForTest(t *testing.T) *prefs.Store {
	t.Helper()
	store, err := loadPrefsStoreNoMigrate()
	if err != nil {
		t.Fatalf("resolve the prefs store: %v", err)
	}
	return store
}

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

func TestThemePersister_CommitThemeSlot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		member theme.Member
		want   prefsOnDisk
	}{
		{"dark", theme.MemberDark, prefsOnDisk{Appearance: "light", ThemeLight: theme.DefaultLightSlug, ThemeDark: nordSlug}},
		{"light", theme.MemberLight, prefsOnDisk{Appearance: "light", ThemeLight: nordSlug, ThemeDark: theme.DefaultDarkSlug}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seeded := `{"appearance":"light","theme":"` + theme.DefaultDarkSlug + `","theme_light":"` + theme.DefaultLightSlug + `","theme_dark":"` + theme.DefaultDarkSlug + `"}`
			path := setPrefsFile(t, seeded)
			persister := newThemePersister(prefsStoreForTest(t))

			if err := persister.CommitThemeSlot(nordSlug, tc.member); err != nil {
				t.Fatalf("CommitThemeSlot(%s, %v): %v", nordSlug, tc.member, err)
			}

			assertPrefsOnDisk(t, path, tc.want)
		})
	}
}

func TestThemePersister_MemberToPrefsSlot(t *testing.T) {
	for _, tc := range []struct {
		member theme.Member
		want   prefs.ThemeSlot
	}{
		{theme.MemberLight, prefs.SlotLight},
		{theme.MemberDark, prefs.SlotDark},
	} {
		if got := prefsSlotFor(tc.member); got != tc.want {
			t.Errorf("prefsSlotFor(%v) = %v, want %v", tc.member, got, tc.want)
		}
	}
}

func TestThemePersister_FailedCommitLogsAndReturns(t *testing.T) {
	const malformed = `{"session_list_mode":"by-tag",`
	path := setPrefsFile(t, malformed)
	persister := newThemePersister(prefsStoreForTest(t))
	sink := logtest.Install(t)

	err := persister.CommitTheme(nordSlug)

	if err == nil {
		t.Fatal("CommitTheme returned nil over a malformed prefs.json; a failed commit must report the failure as its return VALUE, not only as a log line")
	}
	assertPrefsUnchanged(t, path, []byte(malformed))

	rec := sink.Records().Only(t, "log record")
	if rec.Level != slog.LevelWarn {
		t.Errorf("level = %v, want WARN", rec.Level)
	}
	if want := "commit failed"; rec.Msg != want {
		t.Errorf("message = %q, want %q verbatim from the event catalogue", rec.Msg, want)
	}
	if got := rec.AttrString(t, "reason"); got != err.Error() {
		t.Errorf("reason = %q, want the returned error %q — the log and the value must describe the same failure", got, err.Error())
	}
}

// The absence is the load-bearing half: the attr names which half of an adaptive
// pair a line is about, so an empty value would grep as a slot named nothing.
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
			commit: func(p themePersister) error { return p.CommitThemeSlot(nordSlug, theme.MemberDark) },
			want:   []string{"component", "slug", "slot", "reason"},
			slot:   "dark",
		},
		{
			name:   "a light slot carries slot=light",
			commit: func(p themePersister) error { return p.CommitThemeSlot(nordSlug, theme.MemberLight) },
			want:   []string{"component", "slug", "slot", "reason"},
			slot:   "light",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setPrefsFile(t, `{"appearance":"dark",`)
			persister := newThemePersister(prefsStoreForTest(t))
			sink := logtest.Install(t)

			if err := tc.commit(persister); err == nil {
				t.Fatal("the commit succeeded over a malformed prefs.json; there is no failure to describe")
			}

			rec := sink.Records().Only(t, "log record")
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

			// The component's closed attr-key set, asserted key by key so an attr
			// added later reads as a vocabulary breach, not a changed expectation.
			closed := map[string]bool{"slug": true, "slot": true, "reason": true, "path": true, "token": true, "count": true, "rejected": true}
			for _, key := range rec.Keys {
				if key != "component" && !closed[key] {
					t.Errorf("attr %q is outside the theme component's closed key set %v", key, slices.Sorted(maps.Keys(closed)))
				}
			}
		})
	}

	t.Run("the slot renders exactly as the theme component renders its own", func(t *testing.T) {
		// Reads the loader's rendering back off a real emission rather than
		// restating the literals a second time.
		for _, member := range []theme.Member{theme.MemberLight, theme.MemberDark} {
			sink := logtest.Install(t)
			theme.NewEventLogger(themeLogger).Loaded(nordSlug, member.Slot())
			want := sink.Records().Only(t, "log record").AttrString(t, "slot")

			if got := themeSlotAttr(member); got != want {
				t.Errorf("themeSlotAttr(%v) = %q, want %q — the persister's slot attr must not drift from the loader's", member, got, want)
			}
		}
	})
}

// The failing commit at the end is the sink's positive control.
func TestThemePersister_SuccessIsSilent(t *testing.T) {
	path := setPrefsFile(t, `{"session_list_mode":"by-tag"}`)
	persister := newThemePersister(prefsStoreForTest(t))
	sink := logtest.Install(t)

	if err := persister.CommitTheme(nordSlug); err != nil {
		t.Fatalf("CommitTheme(%s): %v", nordSlug, err)
	}
	if err := persister.CommitThemeSlot(nordSlug, theme.MemberDark); err != nil {
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

func TestThemePersister_PerInstanceLastWriteWins(t *testing.T) {
	path := setPrefsFile(t, `{"session_list_mode":"by-tag","appearance":"dark"}`)
	first := newThemePersister(prefsStoreForTest(t))
	second := newThemePersister(prefsStoreForTest(t))

	if err := first.CommitTheme(nordSlug); err != nil {
		t.Fatalf("first instance CommitTheme: %v", err)
	}
	assertPrefsOnDisk(t, path, prefsOnDisk{SessionListMode: "by-tag", Appearance: "dark", Theme: nordSlug})

	if err := second.CommitThemeSlot(theme.DefaultLightSlug, theme.MemberLight); err != nil {
		t.Fatalf("second instance CommitThemeSlot: %v", err)
	}

	assertPrefsOnDisk(t, path, prefsOnDisk{
		SessionListMode: "by-tag",
		Appearance:      "dark",
		ThemeLight:      theme.DefaultLightSlug,
	})
}
