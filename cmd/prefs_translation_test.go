package cmd

// Tests in this file seed prefs.json through t.Setenv and drive package-level
// cmd seams, so they MUST NOT use t.Parallel.

import (
	"bytes"
	"go/ast"
	"os"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/sourceguard"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// migratingLoadForTest runs the production migrating prefs load against
// whatever prefs.json the test seeded, failing on the path-resolution error
// openTUI degrades from (no test here seeds an unresolvable path).
func migratingLoadForTest(t *testing.T) prefsLoad {
	t.Helper()

	load, err := loadPrefsStore()
	if err != nil {
		t.Fatalf("load prefs store: %v", err)
	}
	if load.Store == nil {
		t.Fatal("loadPrefsStore returned no store; the initial mode read and the theme persister both need one")
	}
	return load
}

// assertLoad asserts the whole computed result in one go — the in-memory keys,
// whether the marker is still owed, and the slug the write half will
// re-evaluate. The store is not part of the computation, so want carries none.
func assertLoad(t *testing.T, got, want prefsLoad) {
	t.Helper()

	if got.Keys != want.Keys {
		t.Errorf("Keys = %+v, want %+v", got.Keys, want.Keys)
	}
	if got.TranslationPending != want.TranslationPending {
		t.Errorf("TranslationPending = %v, want %v", got.TranslationPending, want.TranslationPending)
	}
	if got.TranslatedSlug != want.TranslatedSlug {
		t.Errorf("TranslatedSlug = %q, want %q", got.TranslatedSlug, want.TranslatedSlug)
	}
}

// TestLoadPrefsStore_TranslatesDark: it translates a dark pin to the equivalent
// constant.
//
// This is §10.1's protected population — the user who deliberately pinned dark
// on a light terminal — and the assertion is that THIS LAUNCH already renders
// their pin as a constant theme, with detection still off, before anything is
// written.
func TestLoadPrefsStore_TranslatesDark(t *testing.T) {
	setPrefsFile(t, `{"appearance":"dark"}`)

	load := migratingLoadForTest(t)

	assertLoad(t, load, prefsLoad{Keys: prefs.ThemeKeys{Theme: theme.DefaultDarkSlug}, TranslationPending: true, TranslatedSlug: theme.DefaultDarkSlug})
}

// TestLoadPrefsStore_TranslatesLight: it translates a light pin.
//
// The mirror of the dark case, and it is worth its own test rather than a table
// row on the dark one: the two arms map to two different constants, so a
// mapping that answered with one slug for both would still satisfy either test
// alone.
func TestLoadPrefsStore_TranslatesLight(t *testing.T) {
	setPrefsFile(t, `{"appearance":"light"}`)

	load := migratingLoadForTest(t)

	assertLoad(t, load, prefsLoad{Keys: prefs.ThemeKeys{Theme: theme.DefaultLightSlug}, TranslationPending: true, TranslatedSlug: theme.DefaultLightSlug})
}

// TestLoadPrefsStore_TranslatedPinRendersAsAConstantThisLaunch: it renders the
// translated pin this launch.
//
// The end-to-end half of the two struct assertions above, driven through the
// production chain (load → §8.2's setting → §8.4's per-slot load) on exactly
// §10.1's protected population: a pinned `appearance` and NO theme keys. What
// renders is a CONSTANT of that pin's shipped default, with detection still off.
//
// The file holding no theme keys is what makes this the case only reachable from
// here: resolving the nomination from a SECOND read of the store would answer
// with zero keys, and zero keys are the shipped adaptive PAIR — §10.1's silent
// flip, delivered on the very launch that translated the pin, to the one group
// who expressed a preference. The keys must therefore reach the resolution as
// §8.4's post-translation in-memory value, and the struct assertions above
// cannot tell that apart from a second read: their claim stops at prefsLoad.
//
// Both arms are needed: assertConstant on one alone would pass identically for a
// mapping that answered with the same slug for both pins.
func TestLoadPrefsStore_TranslatedPinRendersAsAConstantThisLaunch(t *testing.T) {
	for _, tc := range []struct{ appearance, want string }{
		{"dark", theme.DefaultDarkSlug},
		{"light", theme.DefaultLightSlug},
	} {
		t.Run("appearance "+tc.appearance, func(t *testing.T) {
			setPrefsFile(t, `{"appearance":"`+tc.appearance+`"}`)

			assertConstant(t, themeNominationForTest(t), themetest.Builtin(t, tc.want))
		})
	}
}

// TestLoadPrefsStore_NoTranslationCases: it translates nothing for auto, absent
// or unrecognised.
//
// `auto` and an absent field both mean "keep detecting", which the shipped
// adaptive pair already is — so ignoring them lands exactly where the user was.
//
// THE MARKER IS STILL OWED IN EVERY ROW. §10.2's "Nothing" refers to the theme
// KEYS, not to the migration: leaving the translation pending here would make
// the condition re-evaluate on every launch forever.
func TestLoadPrefsStore_NoTranslationCases(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"auto", `{"appearance":"auto"}`},
		{"absent", `{"session_list_mode":"by-tag"}`},
		{"empty", `{"appearance":""}`},
		{"unrecognised", `{"appearance":"sepia"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setPrefsFile(t, tc.content)

			load := migratingLoadForTest(t)

			assertLoad(t, load, prefsLoad{TranslationPending: true})
		})
	}
}

// TestLoadPrefsStore_MarkerGatesTheTranslation: it does not translate once the
// marker is set.
//
// The trigger is the MARKER, never the absence of theme keys (§10.3). The
// second case is the one absence-gating gets wrong: a user who hand-edited
// their theme keys away to return to the shipped pair — §9.9's documented
// escape hatch — must not be silently re-pinned to what they just undid.
func TestLoadPrefsStore_MarkerGatesTheTranslation(t *testing.T) {
	t.Run("a migrated file with a key set is returned exactly as read", func(t *testing.T) {
		setPrefsFile(t, `{"appearance":"dark","theme":"`+nordSlug+`","theme_migrated":true}`)

		load := migratingLoadForTest(t)

		assertLoad(t, load, prefsLoad{Keys: prefs.ThemeKeys{Theme: nordSlug}})
	})

	t.Run("a migrated file whose keys were deleted stays on the shipped pair", func(t *testing.T) {
		setPrefsFile(t, `{"appearance":"dark","theme_migrated":true}`)

		load := migratingLoadForTest(t)

		assertLoad(t, load, prefsLoad{})
	})
}

// TestLoadPrefsStore_ExistingKeySuppressesTheInMemoryValue: it applies the
// no-op condition in memory.
//
// §10.5 evaluates §10.3's no-op condition against the LOAD-TIME SNAPSHOT for
// the in-memory half — the only moment early enough to affect what is painted.
// Whatever the user has already set is what renders, under all three keys.
//
// TranslatedSlug survives the check unzeroed on purpose: the condition is
// checked twice against two reads, and the write half's re-read is what absorbs
// a commit another instance made in between.
func TestLoadPrefsStore_ExistingKeySuppressesTheInMemoryValue(t *testing.T) {
	for _, tc := range []struct {
		key  string
		want prefs.ThemeKeys
	}{
		{"theme", prefs.ThemeKeys{Theme: nordSlug}},
		{"theme_light", prefs.ThemeKeys{Light: nordSlug}},
		{"theme_dark", prefs.ThemeKeys{Dark: nordSlug}},
	} {
		t.Run(tc.key+" already set", func(t *testing.T) {
			setPrefsFile(t, `{"appearance":"dark","`+tc.key+`":"`+nordSlug+`"}`)

			load := migratingLoadForTest(t)

			assertLoad(t, load, prefsLoad{Keys: tc.want, TranslationPending: true, TranslatedSlug: theme.DefaultDarkSlug})
		})
	}
}

// TestLoadPrefsStore_HandEditedSlotWinsOnTheTranslatingLaunch: it renders the
// hand-edited slot this launch.
//
// §10.3's own reachable loss-of-setting sequence, closed end to end through the
// production chain (load → §8.2's setting → §8.4's per-slot load): a user
// upgrades, runs only bootstrap-exempt commands so the migration never fires,
// reads the new docs and hand-edits `theme_dark`, then launches the picker.
// What paints is NORD, on that launch and every later one — not one launch of
// Tokyo Night followed by Nord, which is §10.1's failure delivered by the
// mechanism added to prevent it.
func TestLoadPrefsStore_HandEditedSlotWinsOnTheTranslatingLaunch(t *testing.T) {
	setPrefsFile(t, `{"appearance":"dark","theme_dark":"`+nordSlug+`"}`)

	nomination := themeNominationForTest(t)

	assertPair(t, nomination, themetest.Builtin(t, theme.DefaultLightSlug), themetest.Builtin(t, nordSlug))
}

// TestLoadPrefsStore_ComputesWithoutWriting: it writes nothing.
//
// §10.5 separates COMPUTING from PERSISTING, and this task owns only the
// compute half: the translated theme is used in memory immediately while the
// on-disk bytes stay exactly as the user left them.
//
// The absent-file row carries a second rule (§8.1): the migration never CREATES
// prefs.json. A fresh install has no appearance to translate, so creating the
// file purely to record a marker would add a side effect to a path the feature
// otherwise leaves free.
func TestLoadPrefsStore_ComputesWithoutWriting(t *testing.T) {
	t.Run("a translating launch leaves the file byte-identical", func(t *testing.T) {
		before := []byte(`{"session_list_mode":"by-tag","appearance":"dark"}`)
		path := setPrefsFile(t, string(before))

		load := migratingLoadForTest(t)
		assertLoad(t, load, prefsLoad{Keys: prefs.ThemeKeys{Theme: theme.DefaultDarkSlug}, TranslationPending: true, TranslatedSlug: theme.DefaultDarkSlug})

		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back prefs.json: %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Errorf("prefs.json = %s, want it untouched: %s", after, before)
		}
	})

	t.Run("an absent prefs.json stays absent", func(t *testing.T) {
		path := setPrefsFile(t, "")

		load := migratingLoadForTest(t)
		assertLoad(t, load, prefsLoad{TranslationPending: true})

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("prefs.json exists at %s (stat err %v); the migration must create nothing", path, err)
		}
	})
}

// TestLoadPrefsStore_TolerantOnDegenerateFiles: it tolerates an absent or
// corrupt file.
//
// Every degenerate read collapses to empty keys with no error — which IS the
// shipped adaptive pair, what an unconfigured install renders anyway — so an
// unreadable preferences file can never be fatal to opening the picker. The
// nomination is resolved as well as the load, because "does not block TUI
// construction" is a claim about the whole chain rather than about the read.
func TestLoadPrefsStore_TolerantOnDegenerateFiles(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"absent", ""},
		{"corrupt", "{ this is not json"},
		{"empty", " "},
		{"a top-level array", `[1,2]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setPrefsFile(t, tc.content)

			load := migratingLoadForTest(t)
			assertLoad(t, load, prefsLoad{TranslationPending: true})

			assertPair(t, themeNominationForTest(t),
				themetest.Builtin(t, theme.DefaultLightSlug),
				themetest.Builtin(t, theme.DefaultDarkSlug))
		})
	}
}

// TestLoadPrefsStoreNoMigrate_ComputesAndWritesNothing: it keeps the
// non-migrating read inert.
//
// This is the read `portal doctor` uses (§10.5), and its contract is that a
// diagnosis heals nothing. The fixture is the one file that would otherwise
// translate — a pending `appearance: dark` — and the variant must return a
// bound store having read no bytes, computed nothing and written nothing.
//
// The mode-0000 fixture is what the RUNTIME half can prove: the load answers
// with a bound store and NO error against a file it cannot read, and leaves the
// bytes exactly as they were. It does NOT prove "no read" — the fixture carries
// no theme keys, so a read would answer with zero keys too. The structural
// sub-test is what pins the absence of a read.
func TestLoadPrefsStoreNoMigrate_ComputesAndWritesNothing(t *testing.T) {
	t.Run("it returns a bound store against an unreadable file", func(t *testing.T) {
		before := []byte(`{"appearance":"dark"}`)
		path := setPrefsFile(t, string(before))
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatalf("chmod 0000 prefs.json: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

		store, err := loadPrefsStoreNoMigrate()
		if err != nil {
			t.Fatalf("loadPrefsStoreNoMigrate: %v", err)
		}
		if store == nil {
			t.Fatal("loadPrefsStoreNoMigrate returned no store")
		}

		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatalf("chmod 0644 prefs.json: %v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back prefs.json: %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Errorf("prefs.json = %s, want it untouched: %s", after, before)
		}

		// Reading through the returned store afterwards still answers with zero
		// keys: the variant computed nothing and recorded no translated value on
		// its behalf. This is not the "no read" claim — the fixture carries no
		// theme keys, so a read would answer with zero keys as well.
		keys, err := store.LoadThemeKeys()
		if err != nil {
			t.Fatalf("LoadThemeKeys: %v", err)
		}
		if (keys != prefs.ThemeKeys{}) {
			t.Errorf("keys = %+v, want zero — the non-migrating read translates nothing", keys)
		}
	})

	t.Run("its body resolves the path and constructs the store, and nothing else", func(t *testing.T) {
		// The runtime half above cannot see a read whose error is discarded, and
		// a discarded read is exactly the shape behaviour creeps in as. This is
		// the structural half: the closed call set IS the "must never gain
		// behaviour" contract, so anything added to the body fails here rather
		// than silently landing on doctor's read-only path.
		want := map[string]bool{"prefsFilePath": true, "prefs.NewStore": true}

		var calls []string
		ast.Inspect(funcDeclForTest(t, "config.go", "loadPrefsStoreNoMigrate"), func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch target := call.Fun.(type) {
			case *ast.Ident:
				calls = append(calls, target.Name)
			case *ast.SelectorExpr:
				if pkg, ok := target.X.(*ast.Ident); ok {
					calls = append(calls, pkg.Name+"."+target.Sel.Name)
				}
			}
			return true
		})

		for _, call := range calls {
			if !want[call] {
				t.Errorf("loadPrefsStoreNoMigrate calls %s — it must never gain behaviour; doctor's read-only path is what a read or a mutation here would break", call)
			}
		}
		if len(calls) != len(want) {
			t.Errorf("loadPrefsStoreNoMigrate makes %d calls (%v), want the %d that resolve the path and construct the store", len(calls), calls, len(want))
		}
	})
}

// funcDeclForTest returns the named top-level function declaration from one of
// the cmd package's production sources.
func funcDeclForTest(t *testing.T, file, name string) *ast.FuncDecl {
	t.Helper()

	parsed, ok := parsePackageFilesByName(t)[file]
	if !ok {
		t.Fatalf("cmd/%s is not a production source of the package", file)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("cmd/%s declares no func %s", file, name)
	return nil
}

// TestLoadPrefsStore_SingleProductionCaller: it has one migrating caller.
//
// §10.5's "the migration runs only where a TUI is constructed" holds
// STRUCTURALLY rather than by review, which is what this guard is for: a second
// caller — a doctor check, a bootstrap step, a shared pre-run — would run the
// translation on a path that paints nothing and, once task 6-6 adds the write,
// mutate the user's config from a verb that has no business doing so. The
// non-migrating variant exists precisely so such a caller has an inert read to
// use instead.
func TestLoadPrefsStore_SingleProductionCaller(t *testing.T) {
	var callers []string

	for name, file := range parsePackageFilesByName(t) {
		sourceguard.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "loadPrefsStore" {
				callers = append(callers, name+":"+funcName)
			}
			return true
		})
	}

	// parsePackageFilesByName is map-ordered, so sort for a stable failure message.
	slices.Sort(callers)

	if len(callers) != 1 || callers[0] != "open.go:openTUI" {
		t.Errorf("loadPrefsStore is called from %v, want exactly [open.go:openTUI] — the translation runs only where a TUI is constructed; use loadPrefsStoreNoMigrate for a read-only one", callers)
	}
}

// TestTranslateAppearance_ExactMatchOnly: it matches the appearance value
// exactly.
//
// §10.2's mapping reproduces THE OLD BINARY'S reading of the value, and the
// appearance decode §8.8 deleted matched the three tokens exactly — so every
// value that binary read as `auto` (`Dark`, ` dark`, a trailing newline from a
// hand-edit) must translate to nothing. Trimming or lowercasing here would
// change the meaning of a value rather than preserve it, which is the one thing
// the translation may not do.
func TestTranslateAppearance_ExactMatchOnly(t *testing.T) {
	for _, raw := range []string{"Dark", " dark", "DARK", "dark\n", "Light", "light ", "LIGHT", "auto", "sepia", ""} {
		t.Run("translates "+raw+" to nothing", func(t *testing.T) {
			if got := translateAppearance(raw); got != "" {
				t.Errorf("translateAppearance(%q) = %q, want %q — only the exact tokens translate", raw, got, "")
			}
		})
	}
}

// TestTranslateAppearance_UsesSharedConstants: it uses the shared default slugs.
//
// The two mapped slugs are §7.2's shipped pair, named by the constants Phase 2
// declared, never by a literal — a hardcoded "tokyo-night" here would go on
// passing after the shipped default moved.
func TestTranslateAppearance_UsesSharedConstants(t *testing.T) {
	if theme.DefaultDarkSlug == theme.DefaultLightSlug {
		t.Fatal("the two shipped default slugs are equal; the assertions below could not tell the two mappings apart")
	}

	for _, tc := range []struct{ raw, want string }{
		{"dark", theme.DefaultDarkSlug},
		{"light", theme.DefaultLightSlug},
	} {
		t.Run("appearance "+tc.raw, func(t *testing.T) {
			if got := translateAppearance(tc.raw); got != tc.want {
				t.Errorf("translateAppearance(%q) = %q, want %q", tc.raw, got, tc.want)
			}
			// A slug that names no built-in would pin the user to §8.5's
			// fallback rather than to the theme their pin meant.
			themetest.Builtin(t, tc.want)
		})
	}
}
