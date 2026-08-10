// White-box (package prefs) for the same reasons theme_savers_test.go and
// migration_marker_test.go are. Two of this saver's contracts are unobservable
// from outside: "the theme key and the marker land in ONE atomic write" leaves
// no trace on the filesystem afterwards — a second write is indistinguishable
// from one — so it is counted at the unexported atomicWrite seam, and the
// decline paths write nothing at all, so they are asserted byte-for-byte
// against the file the unexported mutate seam declined to touch.
//
// The helpers declared in store_write_path_test.go (seedPrefsFile, readRaw,
// decodeWritten, assertWrittenValue, assertNoTempFiles, assertUntouched),
// theme_savers_test.go (assertKeysAbsent, recordWrites) and
// migration_marker_test.go (assertMarkerValue, assertMarkerOnDisk) are reused
// rather than redeclared: this file is the same package.
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

// themeKeyNames are the three keys §10.3's no-op condition inspects, in the
// order the file declares them.
var themeKeyNames = []string{"theme", "theme_light", "theme_dark"}

// onlyCommit asserts exactly one AtomicWrite was made and returns its decoded
// payload. Every writing branch of the translation is under this rule (§10.5):
// issuing two writes would leave a reachable window in which the theme key is
// persisted with the marker unset — the state that makes a successful
// translation look, forever after, like a failed one.
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

// seedWithOneThemeKey renders prefs.json content holding a retained appearance
// and exactly one of the three theme keys.
func seedWithOneThemeKey(key, value string) string {
	return fmt.Sprintf(`{"session_list_mode":"flat","appearance":"dark","%s":%q}`, key, value)
}

// otherThemeKeys returns the theme keys that are not key.
func otherThemeKeys(key string) []string {
	var others []string
	for _, name := range themeKeyNames {
		if name != key {
			others = append(others, name)
		}
	}
	return others
}

// TestSaveTranslation_KeyAndMarkerInOneWrite pins §10.5's central rule: the
// translated theme key and the `theme_migrated` marker land in ONE atomic
// write.
//
// Two writes would leave a reachable window, and the translation's write is
// explicitly best-effort and non-blocking — i.e. liable to be cut short. A
// failure landing between them persists the theme key with the marker unset;
// the next launch then finds the marker false, sees a theme key already set,
// writes only the marker, and therefore never emits `theme: appearance
// migrated` — the translation succeeded while the log says it failed, which is
// the one reading §12.3 designs the event to make impossible.
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

	// The count is taken at the seam because it is not recoverable afterwards.
	// Asserting both keys on the SAME commit's bytes is the stronger property:
	// the single write already carries the final state, rather than the marker
	// arriving in a later one.
	committed := onlyCommit(t, commits())
	assertWrittenValue(t, committed, "theme", "tokyo-night")
	assertMarkerValue(t, committed, true)

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "theme", "tokyo-night")
	assertMarkerValue(t, decoded, true)
	assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
	assertWrittenValue(t, decoded, "appearance", "dark")
}

// TestSaveTranslation_ExistingKeySkipsThemeKeys pins §10.3's no-op condition:
// if ANY theme key is already set the translation writes no theme key. This is
// not absence-gating the trigger (which §10.3 rejects as re-armable); it is
// refusing to clobber a choice the user has already made — the reachable
// sequence being a user who hand-edits `theme_dark = nord` before the migration
// has ever fired, and whose slot would otherwise be pinned away by §8.2's
// clear on the next launch.
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

			// "Skip" means skip the theme KEYS, not the whole write: the marker
			// is still recorded, so the translation does not stay pending
			// forever — and it still rides one write.
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

// TestSaveTranslation_DoesNotCreateAbsentFile pins the half of task 6-1's
// write-path rule the translation deliberately does NOT inherit. §8.1 bars the
// migration from creating prefs.json: a fresh install has no `appearance` to
// translate, so creating the file purely to record a marker would add a side
// effect to a path this feature otherwise leaves free.
//
// Declining is not a failure — nil is returned and persisted is false, because
// an error would invite the caller to treat an ordinary fresh install as one.
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

// TestSaveTranslation_AlreadyMigratedIsANoOp pins the trigger's exactly-once
// contract as observed at the RMW re-read: another instance recorded the
// translation between this instance's load and this write, so there is nothing
// left to do and the file is left byte-identical.
//
// The markerless-file seed is the dangerous one: with no theme keys set it is
// otherwise a perfectly eligible file, so only the marker stands between it and
// a second translation.
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

// TestSaveTranslation_EmptySlugIsMarkerOnly pins that an empty slug is a
// legitimate marker-only call, not an error. It means "there was nothing to
// translate" — `appearance` was `auto`, absent or unrecognised (§10.2) — and
// §8.1's rule still applies: the marker is set, so the condition is not
// re-evaluated forever.
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

// TestSaveTranslation_NoOpEvaluatedAtReRead pins WHERE §10.3's no-op condition
// is evaluated: at the RMW re-read, against the bytes about to be merged, never
// against the load-time snapshot (§8.9).
//
// The translation's write is non-blocking, so a user can commit a theme in the
// window between compute and persist. Evaluated against a stale snapshot the
// pending translation would write its own key over the one they just committed
// and clear the slots — §10.3's own failure, displaced from cross-launch to
// intra-process. The same re-read is what lets this instance see that another
// instance already recorded the migration.
func TestSaveTranslation_NoOpEvaluatedAtReRead(t *testing.T) {
	t.Run("a theme committed after this instance's snapshot survives", func(t *testing.T) {
		path := seedPrefsFile(t, `{"session_list_mode":"flat","appearance":"dark"}`)

		// This instance is constructed while the file is still eligible...
		instance := NewStore(path)
		// ...the user commits a theme in between...
		if err := NewStore(path).SaveTheme("nord"); err != nil {
			t.Fatalf("unexpected SaveTheme error from the other writer: %v", err)
		}
		// ...and the pending translation must observe it.
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

		// The translated slug must appear nowhere at all: writing it into any
		// key, however harmlessly, is the clobber this rule exists to prevent.
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

// TestSaveTranslation_SkipStillRecordsTheMarker pins §8.9's "'skip' means skip
// the theme keys, not the whole write". Recording the marker on a skipped run is
// what stops the translation staying pending forever — and, once recorded, the
// condition is spent: the next run writes nothing at all.
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

	// Spent: a second run is a complete no-op rather than another marker write.
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

// TestSaveTranslation_AbortsOnUndecodable pins the half of task 6-1's rule the
// translation DOES inherit (§8.9): a re-read that does not decode aborts rather
// than overwrites, so the bytes stay byte-identical. Merging into the tolerant
// decode's zero-valued record would erase session_list_mode, every theme key and
// the retained raw appearance in order to record a translation.
//
// The abort is reported by returning — prefs is a leaf — and persisted is false,
// because nothing was. The marker stays unset, so the condition is still true
// and the next launch retries, which is what makes a best-effort write safe.
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

// TestSaveTranslation_ReportsPersisted pins the result as a first-class answer
// rather than something inferred from the error: task 6-6's `theme: appearance
// migrated` fires ONLY when a theme key was actually persisted (§10.5), and
// every other outcome here — marker-only, spent, declined, aborted — translated
// nothing.
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
		// The mutator decided to write a theme key, and the commit itself
		// failed. Reporting persisted would fire task 6-6's `theme: appearance
		// migrated` for a translation that is not on disk — the exact inversion
		// §12.3 designs the event to make impossible, in the other direction.
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

		// And the condition is still true, so the next launch retries.
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

		// And the second call changed nothing: the first translation stands.
		assertWrittenValue(t, decodeWritten(t, path), "theme", "tokyo-night")
	})
}

// TestSaveTranslation_WritesAConstant pins §10.3's "mutual exclusion still
// applies when the translation does write": it writes a constant, so it clears
// both slots. They are empty by construction — a file with any key set is
// absorbed by the marker-only branch — but the clear is asserted anyway, because
// "trivially satisfied" is a property of the branch order, not of the write.
//
// Everything the translation does not own round-trips verbatim: session_list_mode
// and the retained raw appearance, which §10.4 keeps for a downgraded binary.
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

// TestSaveTranslation_IsSilent pins that the saver emits NOTHING on any of its
// four branches. prefs is a leaf that must not import the logging machinery
// (§10.5), and the split of emission sites is deliberate: the migration's
// failure signal is the ABSENCE of task 6-6's `theme: appearance migrated`, and
// `theme: commit failed` stays single-sited on the theme persister (§8.9).
//
// A behavioural belt to the leaf guard's braces: the guard proves prefs cannot
// reach a logger, this proves that nothing it calls does so on its behalf.
func TestSaveTranslation_IsSilent(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)

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
