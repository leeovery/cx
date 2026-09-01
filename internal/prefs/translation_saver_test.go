// White-box: write counts and decline paths are only observable at the
// unexported atomicWrite and mutate seams.
package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
)

var themeKeyNames = []string{"theme", "theme_light", "theme_dark"}

func onlyCommit(t *testing.T, made [][]byte) map[string]any {
	t.Helper()

	if len(made) != 1 {
		t.Fatalf("AtomicWrite called %d times, want exactly 1 — the theme key and the marker must land together", len(made))
	}

	var decoded map[string]any
	if err := json.Unmarshal(made[0], &decoded); err != nil {
		t.Fatalf("the committed bytes are not valid JSON (%q): %v", made[0], err)
	}
	return decoded
}

func seedWithOneThemeKey(key, value string) string {
	return fmt.Sprintf(`{"session_list_mode":"flat","appearance":"dark","%s":%q}`, key, value)
}

func otherThemeKeys(key string) []string {
	var others []string
	for _, name := range themeKeyNames {
		if name != key {
			others = append(others, name)
		}
	}
	return others
}

func TestSaveTranslation_KeyAndMarkerInOneWrite(t *testing.T) {
	path := seedPrefsFile(t, `{"session_list_mode":"by-tag","appearance":"dark"}`)
	commits := recordWrites(t)

	persisted, err := NewStore(path).SaveTranslation("tokyo-night")
	if err != nil {
		t.Fatalf("unexpected SaveTranslation error: %v", err)
	}
	if !persisted {
		t.Errorf("persisted = false, want true — a theme key was written")
	}

	// Both keys on the same commit's bytes: the single write must already
	// carry the final state.
	committed := onlyCommit(t, commits())
	assertWrittenValue(t, committed, "theme", "tokyo-night")
	assertMarkerValue(t, committed, true)

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "theme", "tokyo-night")
	assertMarkerValue(t, decoded, true)
	assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
	assertWrittenValue(t, decoded, "appearance", "dark")
}

func TestSaveTranslation_ExistingKeySkipsThemeKeys(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{name: "a constant", key: "theme"},
		{name: "the light slot", key: "theme_light"},
		{name: "the dark slot", key: "theme_dark"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, seedWithOneThemeKey(c.key, "nord"))
			commits := recordWrites(t)

			persisted, err := NewStore(path).SaveTranslation("tokyo-night")
			if err != nil {
				t.Fatalf("unexpected SaveTranslation error: %v", err)
			}
			if persisted {
				t.Errorf("persisted = true, want false — no theme key was written")
			}

			// Skip means skip the theme keys: the marker still rides the one
			// write.
			committed := onlyCommit(t, commits())
			assertMarkerValue(t, committed, true)

			decoded := decodeWritten(t, path)
			assertWrittenValue(t, decoded, c.key, "nord")
			assertKeysAbsent(t, decoded, otherThemeKeys(c.key)...)
			assertMarkerOnDisk(t, path, true)
			assertWrittenValue(t, decoded, "session_list_mode", "flat")
			assertWrittenValue(t, decoded, "appearance", "dark")
		})
	}
}

func TestSaveTranslation_DoesNotCreateAbsentFile(t *testing.T) {
	for _, c := range absentPathCases() {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := c.path(dir)

			persisted, err := NewStore(path).SaveTranslation("tokyo-night")
			if err != nil {
				t.Fatalf("unexpected SaveTranslation error: %v — declining to write is not a failure", err)
			}
			if persisted {
				t.Errorf("persisted = true, want false — nothing was written")
			}

			assertNothingCreated(t, dir, path)
		})
	}
}

// The no-theme-keys seed is the dangerous one: it is otherwise fully eligible,
// so only the marker stands between it and a second translation.
func TestSaveTranslation_AlreadyMigratedIsANoOp(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "a file with no theme key set",
			content: `{"session_list_mode":"flat","appearance":"dark","theme_migrated":true}`,
		},
		{
			name:    "a file the user has since themed",
			content: `{"appearance":"dark","theme":"nord","theme_migrated":true}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			before := readRaw(t, path)
			commits := recordWrites(t)

			persisted, err := NewStore(path).SaveTranslation("tokyo-night")
			if err != nil {
				t.Fatalf("unexpected SaveTranslation error: %v", err)
			}
			if persisted {
				t.Errorf("persisted = true, want false — the translation was already recorded")
			}

			if made := commits(); len(made) != 0 {
				t.Errorf("AtomicWrite called %d times, want none — a spent trigger writes nothing", len(made))
			}
			assertUntouched(t, path, before)
		})
	}
}

func TestSaveTranslation_EmptySlugIsMarkerOnly(t *testing.T) {
	path := seedPrefsFile(t, `{"session_list_mode":"flat","appearance":"auto"}`)
	commits := recordWrites(t)

	persisted, err := NewStore(path).SaveTranslation("")
	if err != nil {
		t.Fatalf("unexpected SaveTranslation error: %v — an empty slug is not an error", err)
	}
	if persisted {
		t.Errorf("persisted = true, want false — there was nothing to translate")
	}

	committed := onlyCommit(t, commits())
	assertMarkerValue(t, committed, true)
	assertKeysAbsent(t, committed, themeKeyNames...)

	decoded := decodeWritten(t, path)
	assertKeysAbsent(t, decoded, themeKeyNames...)
	assertMarkerOnDisk(t, path, true)
	assertWrittenValue(t, decoded, "appearance", "auto")
}

func TestSaveTranslation_NoOpEvaluatedAtReRead(t *testing.T) {
	t.Run("a theme committed after this instance's snapshot survives", func(t *testing.T) {
		path := seedPrefsFile(t, `{"session_list_mode":"flat","appearance":"dark"}`)

		instance := NewStore(path)
		if err := NewStore(path).SaveTheme("nord"); err != nil {
			t.Fatalf("unexpected SaveTheme error from the other writer: %v", err)
		}
		persisted, err := instance.SaveTranslation("tokyo-night")
		if err != nil {
			t.Fatalf("unexpected SaveTranslation error: %v", err)
		}
		if persisted {
			t.Errorf("persisted = true, want false — the committed theme is a choice already made")
		}

		decoded := decodeWritten(t, path)
		assertWrittenValue(t, decoded, "theme", "nord")
		assertKeysAbsent(t, decoded, "theme_light", "theme_dark")
		assertMarkerOnDisk(t, path, true)

		if raw := readRaw(t, path); bytes.Contains(raw, []byte("tokyo-night")) {
			t.Errorf("written JSON %q names the translated slug, want the committed theme untouched", raw)
		}
	})

	t.Run("a marker recorded after this instance's snapshot is observed", func(t *testing.T) {
		path := seedPrefsFile(t, `{"session_list_mode":"flat","appearance":"dark"}`)

		instance := NewStore(path)
		if err := NewStore(path).SaveMigrationMarker(); err != nil {
			t.Fatalf("unexpected SaveMigrationMarker error from the other writer: %v", err)
		}
		before := readRaw(t, path)

		persisted, err := instance.SaveTranslation("tokyo-night")
		if err != nil {
			t.Fatalf("unexpected SaveTranslation error: %v", err)
		}
		if persisted {
			t.Errorf("persisted = true, want false — another instance already migrated")
		}

		assertUntouched(t, path, before)
	})
}

func TestSaveTranslation_SkipStillRecordsTheMarker(t *testing.T) {
	path := seedPrefsFile(t, `{"session_list_mode":"flat","appearance":"dark","theme_dark":"nord"}`)
	store := NewStore(path)

	persisted, err := store.SaveTranslation("tokyo-night")
	if err != nil {
		t.Fatalf("unexpected SaveTranslation error: %v", err)
	}
	if persisted {
		t.Errorf("persisted = true, want false — the theme keys were skipped")
	}
	assertMarkerOnDisk(t, path, true)

	before := readRaw(t, path)
	commits := recordWrites(t)

	if _, err := store.SaveTranslation("tokyo-night"); err != nil {
		t.Fatalf("unexpected second SaveTranslation error: %v", err)
	}

	if made := commits(); len(made) != 0 {
		t.Errorf("AtomicWrite called %d times on the second run, want none", len(made))
	}
	assertUntouched(t, path, before)
}

func TestSaveTranslation_AbortsOnUndecodable(t *testing.T) {
	for _, c := range undecodablePrefsCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			before := readRaw(t, path)

			persisted, err := NewStore(path).SaveTranslation("tokyo-night")
			if err == nil {
				t.Fatalf("SaveTranslation returned nil, want an abort error")
			}
			if persisted {
				t.Errorf("persisted = true, want false alongside the abort")
			}
			c.assertErr(t, err)

			assertUntouched(t, path, before)
		})
	}
}

func TestSaveTranslation_ReportsPersisted(t *testing.T) {
	cases := []struct {
		name          string
		content       string
		absent        bool
		slug          string
		wantPersisted bool
		wantErr       bool
	}{
		{name: "an eligible file", content: `{"appearance":"dark"}`, slug: "tokyo-night", wantPersisted: true},
		{name: "a file already carrying a constant", content: `{"theme":"nord"}`, slug: "tokyo-night"},
		{name: "a file already carrying a slot", content: `{"theme_light":"nord"}`, slug: "tokyo-night"},
		{name: "an empty slug", content: `{"appearance":"auto"}`},
		{name: "a file already migrated", content: `{"appearance":"dark","theme_migrated":true}`, slug: "tokyo-night"},
		{name: "an absent file", absent: true, slug: "tokyo-night"},
		{name: "an undecodable file", content: `{`, slug: "tokyo-night", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "prefs.json")
			if !c.absent {
				path = seedPrefsFile(t, c.content)
			}

			persisted, err := NewStore(path).SaveTranslation(c.slug)
			if c.wantErr {
				if err == nil {
					t.Fatalf("SaveTranslation returned nil, want an abort error")
				}
			} else if err != nil {
				t.Fatalf("unexpected SaveTranslation error: %v", err)
			}

			if persisted != c.wantPersisted {
				t.Errorf("persisted = %v, want %v", persisted, c.wantPersisted)
			}
		})
	}

	t.Run("a failed write reports nothing persisted", func(t *testing.T) {
		// The mutator chose to write but the commit failed: reporting
		// persisted would announce a translation that is not on disk.
		path := seedPrefsFile(t, `{"appearance":"dark"}`)
		before := readRaw(t, path)

		wantErr := errors.New("no space left on device")
		previous := atomicWrite
		atomicWrite = func(string, []byte) error { return wantErr }
		t.Cleanup(func() { atomicWrite = previous })

		persisted, err := NewStore(path).SaveTranslation("tokyo-night")
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want the write error returned verbatim", err)
		}
		if persisted {
			t.Errorf("persisted = true, want false — the write failed, so nothing was persisted")
		}

		assertUntouched(t, path, before)
	})

	t.Run("twice in succession persists a theme key only on the first call", func(t *testing.T) {
		path := seedPrefsFile(t, `{"appearance":"dark"}`)
		store := NewStore(path)

		first, err := store.SaveTranslation("tokyo-night")
		if err != nil {
			t.Fatalf("unexpected first SaveTranslation error: %v", err)
		}
		if !first {
			t.Errorf("first call persisted = false, want true")
		}

		second, err := store.SaveTranslation("gruvbox")
		if err != nil {
			t.Fatalf("unexpected second SaveTranslation error: %v", err)
		}
		if second {
			t.Errorf("second call persisted = true, want false — it sees the marker")
		}

		assertWrittenValue(t, decodeWritten(t, path), "theme", "tokyo-night")
	})
}

// The slots are empty by construction, but the clear is asserted anyway:
// trivial satisfaction is a property of branch order, not of the write.
func TestSaveTranslation_WritesAConstant(t *testing.T) {
	path := seedPrefsFile(t, `{"session_list_mode":"by-tag","appearance":"light"}`)
	commits := recordWrites(t)

	persisted, err := NewStore(path).SaveTranslation("tokyo-night-day")
	if err != nil {
		t.Fatalf("unexpected SaveTranslation error: %v", err)
	}
	if !persisted {
		t.Errorf("persisted = false, want true")
	}

	committed := onlyCommit(t, commits())
	assertWrittenValue(t, committed, "theme", "tokyo-night-day")
	assertKeysAbsent(t, committed, "theme_light", "theme_dark")

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "theme", "tokyo-night-day")
	assertKeysAbsent(t, decoded, "theme_light", "theme_dark")
	assertMarkerOnDisk(t, path, true)
	assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
	assertWrittenValue(t, decoded, "appearance", "light")
}

// Complements the leaf guard: prefs cannot reach a logger, and nothing it
// calls emits on its behalf.
func TestSaveTranslation_IsSilent(t *testing.T) {
	sink := logtest.Install(t)

	writing := seedPrefsFile(t, `{"appearance":"dark"}`)
	markerOnly := seedPrefsFile(t, `{"appearance":"dark","theme":"nord"}`)
	spent := seedPrefsFile(t, `{"appearance":"dark","theme_migrated":true}`)
	absent := filepath.Join(t.TempDir(), "prefs.json")

	for _, path := range []string{writing, markerOnly, spent, absent} {
		if _, err := NewStore(path).SaveTranslation("tokyo-night"); err != nil {
			t.Fatalf("unexpected SaveTranslation error for %s: %v", path, err)
		}
	}
	if _, err := NewStore(seedPrefsFile(t, `{`)).SaveTranslation("tokyo-night"); err == nil {
		t.Fatalf("SaveTranslation returned nil for an undecodable file, want an abort error")
	}

	if recs := sink.Records(); len(recs) != 0 {
		t.Errorf("expected no log records from SaveTranslation, got %d: %+v", len(recs), recs)
	}

	// The sink is live, so the assertion above is not vacuous.
	log.For("theme").Info("control")
	if recs := sink.Records(); len(recs) != 1 {
		t.Fatalf("the capture sink recorded %d entries for the control emission, want 1 — it is not installed", len(recs))
	}
}
