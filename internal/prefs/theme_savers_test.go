// White-box (package prefs) for the same reason store_write_path_test.go is: the
// theme savers' whole contract is expressed at unexported seams. They are §8.9's
// field-specific savers precisely so no whole-record API exists to drive them
// from outside, and "exactly one atomic write per save" is not observable from
// the filesystem after the fact — a second write leaves no trace on disk — so it
// is counted at the atomicWrite indirection.
//
// The helpers declared in store_write_path_test.go (seedPrefsFile, readRaw,
// decodeWritten, assertWrittenValue, assertNoTempFiles, assertUntouched) are
// reused rather than redeclared: this file is the same package.
package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// assertKeysAbsent asserts none of the given keys is present in the encoded
// JSON. §8.3's "an unset slot holds the shipped default" means a cleared key is
// ABSENT (omitempty renders the empty string as key-absent), never present as
// "" — so presence is asserted structurally on the decoded map rather than by
// substring, where `"theme"` would also match `"theme_light"`.
func assertKeysAbsent(t *testing.T, decoded map[string]any, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if value, ok := decoded[key]; ok {
			t.Errorf("written JSON carries %q = %#v, want the key absent", key, value)
		}
	}
}

// TestSaveTheme_ClearsBothSlots pins the first half of §8.2's write-enforced
// mutual exclusion: committing a constant clears both slots, so "both a constant
// and a pair are present" cannot arise from Portal's own writes.
func TestSaveTheme_ClearsBothSlots(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "a file holding the adaptive pair",
			content: `{"session_list_mode":"flat","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`,
		},
		{
			// The hand-edited both-present file §8.2 documents: `theme` wins on
			// read, and the next constant commit prunes the stale slots.
			name:    "a hand-edited file holding a constant and both slots",
			content: `{"theme":"gruvbox","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`,
		},
		{
			name:    "a file holding only one slot",
			content: `{"theme_dark":"tokyo-night"}`,
		},
		{
			name:    "a file holding a constant alone",
			content: `{"theme":"gruvbox"}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			store := NewStore(path)

			if err := store.SaveTheme("nord"); err != nil {
				t.Fatalf("unexpected SaveTheme error: %v", err)
			}

			decoded := decodeWritten(t, path)
			assertWrittenValue(t, decoded, "theme", "nord")
			assertKeysAbsent(t, decoded, "theme_light", "theme_dark")
		})
	}
}

// TestSaveThemeSlot_ClearsConstant pins the other half of §8.2: assigning a slot
// clears the constant. Whichever was set last wins.
func TestSaveThemeSlot_ClearsConstant(t *testing.T) {
	path := seedPrefsFile(t, `{"theme":"gruvbox","theme_light":"tokyo-night-day"}`)
	store := NewStore(path)

	if err := store.SaveThemeSlot("nord", SlotDark); err != nil {
		t.Fatalf("unexpected SaveThemeSlot error: %v", err)
	}

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "theme_dark", "nord")
	assertWrittenValue(t, decoded, "theme_light", "tokyo-night-day")
	assertKeysAbsent(t, decoded, "theme")
}

// TestSaveThemeSlot_OtherSlotUnaffected pins that a slot save touches its own
// slot only — the property that makes §9.5's `● both` reachable in two
// keypresses rather than clobbering the pair on every commit.
func TestSaveThemeSlot_OtherSlotUnaffected(t *testing.T) {
	cases := []struct {
		name      string
		slot      ThemeSlot
		wantKey   string
		otherKey  string
		otherWant string
	}{
		{
			name:      "assigning dark leaves light exactly as it was",
			slot:      SlotDark,
			wantKey:   "theme_dark",
			otherKey:  "theme_light",
			otherWant: "tokyo-night-day",
		},
		{
			name:      "assigning light leaves dark exactly as it was",
			slot:      SlotLight,
			wantKey:   "theme_light",
			otherKey:  "theme_dark",
			otherWant: "tokyo-night",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, `{"theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`)
			store := NewStore(path)

			if err := store.SaveThemeSlot("nord", c.slot); err != nil {
				t.Fatalf("unexpected SaveThemeSlot error: %v", err)
			}

			decoded := decodeWritten(t, path)
			assertWrittenValue(t, decoded, c.wantKey, "nord")
			assertWrittenValue(t, decoded, c.otherKey, c.otherWant)
		})
	}
}

// TestSaveThemeSlot_LightThenDarkYieldsBoth pins §9.5's `● both` state: two slot
// saves of the same slug leave both slots naming it.
func TestSaveThemeSlot_LightThenDarkYieldsBoth(t *testing.T) {
	path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"gruvbox"}`)
	store := NewStore(path)

	if err := store.SaveThemeSlot("nord", SlotLight); err != nil {
		t.Fatalf("unexpected SaveThemeSlot(light) error: %v", err)
	}
	if err := store.SaveThemeSlot("nord", SlotDark); err != nil {
		t.Fatalf("unexpected SaveThemeSlot(dark) error: %v", err)
	}

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "theme_light", "nord")
	assertWrittenValue(t, decoded, "theme_dark", "nord")
	assertKeysAbsent(t, decoded, "theme")
}

// recordWrites swaps the package-level atomicWrite seam for a recorder that
// still performs the real write, and restores it on cleanup. Tests in this
// repository never run in parallel, so the swap is unsynchronised by design.
// It returns an accessor for the bytes of every commit made since the swap, in
// order.
func recordWrites(t *testing.T) func() [][]byte {
	t.Helper()

	var commits [][]byte
	previous := atomicWrite
	atomicWrite = func(path string, data []byte) error {
		commits = append(commits, bytes.Clone(data))
		return previous(path, data)
	}
	t.Cleanup(func() { atomicWrite = previous })

	return func() [][]byte { return commits }
}

// TestSaveTheme_SingleAtomicWrite pins that a saver's commit and its
// mutual-exclusion clear land in ONE AtomicWrite, never two (§8.9). Two writes
// would leave a reachable window where prefs.json holds both a constant and a
// pair — the state §8.2 says cannot arise from Portal's own writes.
//
// The count is taken at the atomicWrite seam because it is not recoverable from
// the filesystem afterwards: a second write leaves no trace, so a post-hoc
// assertion could only ever prove "at least one". Recording each commit's bytes
// also pins the stronger property — that the single write already carries the
// final state, rather than the clear arriving in a later one.
//
// Despite the name it covers both savers: they are the two halves of one rule.
func TestSaveTheme_SingleAtomicWrite(t *testing.T) {
	for _, c := range themeSaverCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"gruvbox","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`)
			commits := recordWrites(t)

			if err := c.save(NewStore(path), "nord"); err != nil {
				t.Fatalf("unexpected save error: %v", err)
			}

			made := commits()
			if len(made) != 1 {
				t.Fatalf("AtomicWrite called %d times, want exactly 1 — the commit and its clear must land together", len(made))
			}

			var committed map[string]any
			if err := json.Unmarshal(made[0], &committed); err != nil {
				t.Fatalf("the committed bytes are not valid JSON (%q): %v", made[0], err)
			}
			assertWrittenValue(t, committed, c.writtenKey, "nord")
			assertKeysAbsent(t, committed, c.clearedKeys...)
		})
	}
}

// TestThemeSavers_PreserveUnrelatedFields pins §8.9's reason for keeping the
// merge inside the leaf: a saver owns its own key and nothing else, so the raw
// `appearance` round-trip is a property of the store rather than a rule every
// caller has to remember.
func TestThemeSavers_PreserveUnrelatedFields(t *testing.T) {
	// The seeded appearance is deliberately an unrecognised value: §8.8 keeps the
	// field as a plain string that is read and preserved, never parsed.
	const seeded = `{"session_list_mode":"by-tag","appearance":"sepia","theme_light":"tokyo-night-day"}`

	for _, c := range themeSaverCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, seeded)

			if err := c.save(NewStore(path), "nord"); err != nil {
				t.Fatalf("unexpected save error: %v", err)
			}

			decoded := decodeWritten(t, path)
			assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
			assertWrittenValue(t, decoded, "appearance", "sepia")
			// Neither saver may invent the migration marker. The field is now
			// declared, and omitempty keeps a false marker absent — see
			// TestMigrationMarker_NotTouchedByThemeSavers for the full
			// both-directions rule (§8.1: the marker never participates in
			// mutual exclusion).
			assertKeysAbsent(t, decoded, "theme_migrated")
		})
	}
}

// seedWithStaleKeys renders prefs.json content carrying a stale value for each
// of the given keys, over the same `session_list_mode` base as the bare seed it
// is compared against — so the two differ in nothing else.
func seedWithStaleKeys(t *testing.T, keys []string) string {
	t.Helper()

	content := map[string]string{"session_list_mode": "flat"}
	for _, key := range keys {
		content[key] = "gruvbox"
	}

	encoded, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal test content: %v", err)
	}
	return string(encoded)
}

// TestThemeSavers_ClearedKeysAreAbsent pins that clearing is writing the empty
// string, which omitempty renders as key-absent — §8.3's "an unset slot holds
// the shipped default", and a hand-editable file that stays clean.
func TestThemeSavers_ClearedKeysAreAbsent(t *testing.T) {
	t.Run("a cleared key is omitted, never written as an empty string", func(t *testing.T) {
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				path := seedPrefsFile(t, `{"theme":"gruvbox","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`)

				if err := c.save(NewStore(path), "nord"); err != nil {
					t.Fatalf("unexpected save error: %v", err)
				}

				assertKeysAbsent(t, decodeWritten(t, path), c.clearedKeys...)

				// Belt to the decoded map's braces: the key name must not appear
				// in the bytes at all. The trailing `":` anchors it so `"theme"`
				// does not match `"theme_light"`.
				raw := readRaw(t, path)
				for _, key := range c.clearedKeys {
					if needle := []byte(`"` + key + `":`); bytes.Contains(raw, needle) {
						t.Errorf("written JSON %q still names %q, want the cleared key omitted", raw, key)
					}
				}
			})
		}
	})

	t.Run("clearing an already-absent key is not an error and lands the same bytes", func(t *testing.T) {
		// Nothing to clear: the constant save prunes slots that were never set,
		// and the slot save prunes a constant that was never set. The result must
		// be byte-identical to the same save over a file that DID hold them, or
		// "cleared" and "never set" would be two different on-disk states.
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				// The two seeds differ ONLY in the keys this saver clears, so the
				// comparison isolates the clear — a slot saver's untouched other
				// slot would otherwise show up as a difference of its own.
				nothingToClear := seedPrefsFile(t, `{"session_list_mode":"flat"}`)
				somethingToClear := seedPrefsFile(t, seedWithStaleKeys(t, c.clearedKeys))

				for _, path := range []string{nothingToClear, somethingToClear} {
					if err := c.save(NewStore(path), "nord"); err != nil {
						t.Fatalf("unexpected save error: %v", err)
					}
				}

				assertWrittenValue(t, decodeWritten(t, nothingToClear), c.writtenKey, "nord")

				got, want := readRaw(t, nothingToClear), readRaw(t, somethingToClear)
				if !bytes.Equal(got, want) {
					t.Errorf("clearing absent keys wrote %q, want %q — byte-identical to clearing present ones", got, want)
				}
			})
		}
	})
}

// TestThemeSavers_RepeatedCommitIsByteIdentical pins the idempotence that makes
// §9.13's "a commit is always re-attemptable" free: the commit keys are
// unconditional writes, so pressing the same key again simply retries.
func TestThemeSavers_RepeatedCommitIsByteIdentical(t *testing.T) {
	for _, c := range themeSaverCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, `{"session_list_mode":"flat","appearance":"dark","theme":"gruvbox","theme_light":"tokyo-night-day"}`)
			store := NewStore(path)

			var first []byte
			for i := range 3 {
				if err := c.save(store, "nord"); err != nil {
					t.Fatalf("unexpected save error on commit %d: %v", i+1, err)
				}

				data := readRaw(t, path)
				if i == 0 {
					first = data
					continue
				}
				if !bytes.Equal(data, first) {
					t.Fatalf("commit %d rewrote the file: got %q, want %q byte-identical", i+1, data, first)
				}
			}
		})
	}
}

// TestThemeSavers_InheritWritePathRules pins that both savers get task 6-1's two
// persistence rules from the shared mutator rather than re-implementing either.
func TestThemeSavers_InheritWritePathRules(t *testing.T) {
	t.Run("create-on-absent", func(t *testing.T) {
		// The ordinary first write: a fresh install has no prefs.json at all, and
		// an abort here would be permanent because nothing else creates the file.
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "sub", "prefs.json")

				if err := c.save(NewStore(path), "nord"); err != nil {
					t.Fatalf("unexpected save error: %v", err)
				}

				assertWrittenValue(t, decodeWritten(t, path), c.writtenKey, "nord")
				assertNoTempFiles(t, filepath.Dir(path))
			})
		}
	})

	t.Run("abort-on-undecodable", func(t *testing.T) {
		// A stray comma must not become an overwrite: merging into the tolerant
		// decode's zero-valued record would erase every other key in one commit.
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"gruvbox",}`)
				before := readRaw(t, path)

				err := c.save(NewStore(path), "nord")
				if err == nil {
					t.Fatalf("save returned nil, want an abort error")
				}

				// Returned verbatim: prefs is a leaf and reports by returning.
				var syntaxErr *json.SyntaxError
				if !errors.As(err, &syntaxErr) {
					t.Errorf("error = %v (%T), want the decoder's *json.SyntaxError", err, err)
				}

				assertUntouched(t, path, before)
			})
		}
	})
}

// TestThemeSavers_RMWDoesNotLoseAnotherWritersField is the lost-update rule this
// phase exists for (§8.9): each saver re-reads immediately before writing, so an
// instance constructed before another instance's commit does not revert it.
func TestThemeSavers_RMWDoesNotLoseAnotherWritersField(t *testing.T) {
	for _, c := range themeSaverCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, `{"session_list_mode":"flat"}`)

			// Instance A is constructed against the pre-commit file...
			instanceA := NewStore(path)
			// ...instance B presses `s` in between...
			if err := NewStore(path).Save(ModeByTag); err != nil {
				t.Fatalf("unexpected Save error from the other writer: %v", err)
			}
			// ...and A's theme commit must merge into B's bytes, not its own.
			if err := c.save(instanceA, "nord"); err != nil {
				t.Fatalf("unexpected save error: %v", err)
			}

			decoded := decodeWritten(t, path)
			assertWrittenValue(t, decoded, c.writtenKey, "nord")
			assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
		})
	}
}

// TestSaveThemeSlot_InvalidSlotWritesNothing pins the structural half of "no
// caller can mint a third slot" (the typed constant is the other half): the zero
// value is deliberately invalid, so a forgotten argument cannot silently write
// the light slot.
func TestSaveThemeSlot_InvalidSlotWritesNothing(t *testing.T) {
	cases := []struct {
		name    string
		slot    ThemeSlot
		wantErr string
	}{
		{name: "the zero value", slot: ThemeSlot(0), wantErr: "prefs: invalid theme slot 0"},
		{name: "one past the pair", slot: ThemeSlot(3), wantErr: "prefs: invalid theme slot 3"},
		{name: "a negative value", slot: ThemeSlot(-1), wantErr: "prefs: invalid theme slot -1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("a present file is left byte-identical", func(t *testing.T) {
				path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"gruvbox"}`)
				before := readRaw(t, path)

				err := NewStore(path).SaveThemeSlot("nord", c.slot)
				if err == nil {
					t.Fatalf("SaveThemeSlot returned nil, want an error naming the invalid slot")
				}
				if err.Error() != c.wantErr {
					t.Errorf("error = %q, want %q", err, c.wantErr)
				}

				assertUntouched(t, path, before)
			})

			t.Run("an absent file is not created", func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "prefs.json")

				if err := NewStore(path).SaveThemeSlot("nord", c.slot); err == nil {
					t.Fatalf("SaveThemeSlot returned nil, want an error naming the invalid slot")
				}

				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("os.Stat error = %v, want the file still absent", err)
				}
				assertNoTempFiles(t, dir)
			})
		})
	}
}

// TestThemeSavers_NoSlugKnowledge pins that prefs gains no slug knowledge: no
// charset check, no trimming, no lowercasing, no default substitution. Those are
// read-side resolution rules owned by internal/theme, and validating here would
// diverge from the resolver — and would turn a stray-space value into a silently
// different slug instead of the honest `bad name` rejection the user is owed.
func TestThemeSavers_NoSlugKnowledge(t *testing.T) {
	slugs := []struct {
		name string
		raw  string
	}{
		{name: "path traversal", raw: "../evil"},
		{name: "uppercase", raw: "Nord"},
		{name: "leading space", raw: "  nord"},
		{name: "embedded tab", raw: "no\trd"},
	}

	for _, c := range themeSaverCases() {
		for _, slug := range slugs {
			t.Run(c.name+" "+slug.name, func(t *testing.T) {
				path := seedPrefsFile(t, `{"session_list_mode":"flat"}`)

				if err := c.save(NewStore(path), slug.raw); err != nil {
					t.Fatalf("unexpected save error: %v", err)
				}

				assertWrittenValue(t, decodeWritten(t, path), c.writtenKey, slug.raw)
			})
		}
	}
}

// themeSaverCase drives the properties both savers share — preservation,
// omission, idempotence, the inherited write-path rules, the RMW re-read and the
// absence of slug knowledge — over one table, so the two cannot drift.
type themeSaverCase struct {
	name        string
	writtenKey  string
	clearedKeys []string
	save        func(store *Store, slug string) error
}

func themeSaverCases() []themeSaverCase {
	return []themeSaverCase{
		{
			name:        "SaveTheme",
			writtenKey:  "theme",
			clearedKeys: []string{"theme_light", "theme_dark"},
			save:        func(store *Store, slug string) error { return store.SaveTheme(slug) },
		},
		{
			name:        "SaveThemeSlot light",
			writtenKey:  "theme_light",
			clearedKeys: []string{"theme"},
			save:        func(store *Store, slug string) error { return store.SaveThemeSlot(slug, SlotLight) },
		},
		{
			name:        "SaveThemeSlot dark",
			writtenKey:  "theme_dark",
			clearedKeys: []string{"theme"},
			save:        func(store *Store, slug string) error { return store.SaveThemeSlot(slug, SlotDark) },
		},
	}
}
