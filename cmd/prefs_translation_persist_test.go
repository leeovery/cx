package cmd

// Tests in this file seed prefs.json through t.Setenv and drive the
// package-level persistTranslation seam, so they MUST NOT use t.Parallel.

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/token"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// migratedEvent is the one line §12.3's catalogue defines for this task,
// rendered in logtest's canonical "<LEVEL> <msg> key=value" shape.
func migratedEvent(slug string) string {
	return "INFO appearance migrated slug=" + slug
}

// syncPersistTranslation installs the PRODUCTION persist body as the dispatch
// seam, minus the goroutine, restoring the previous value with t.Cleanup.
//
// It substitutes rather than re-implements on purpose: a test that re-wrote the
// save-decide-emit body would assert against itself, and the one-shot cadence of
// `theme: appearance migrated` is exactly what would go unguarded. What the
// substitution removes is the goroutine and nothing else — which is also what
// makes every assertion below deterministic without a sleep.
func syncPersistTranslation(t *testing.T) {
	t.Helper()
	previous := persistTranslation
	persistTranslation = runTranslationPersist
	t.Cleanup(func() { persistTranslation = previous })
}

// prefsOnDisk is the whole on-disk record, decoded field by field so the
// assertions below judge VALUES rather than the serialised form (key order,
// indentation and omitempty are the writer's business, not this task's).
type prefsOnDisk struct {
	SessionListMode string `json:"session_list_mode"`
	Appearance      string `json:"appearance"`
	Theme           string `json:"theme"`
	ThemeLight      string `json:"theme_light"`
	ThemeDark       string `json:"theme_dark"`
	ThemeMigrated   bool   `json:"theme_migrated"`
}

// readPrefsOnDisk decodes prefs.json, failing if it is absent or unparseable —
// every fixture here writes a valid file or is asserted absent separately.
func readPrefsOnDisk(t *testing.T, path string) prefsOnDisk {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read prefs.json: %v", err)
	}
	var got prefsOnDisk
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode prefs.json (%s): %v", data, err)
	}
	return got
}

// assertPrefsOnDisk asserts the WHOLE record, not just the two keys the
// translation writes: the untouched fields are what prove the write merged
// through §8.9's read-modify-write instead of committing a fresh record.
func assertPrefsOnDisk(t *testing.T, path string, want prefsOnDisk) {
	t.Helper()
	if got := readPrefsOnDisk(t, path); got != want {
		t.Errorf("prefs.json = %+v, want %+v", got, want)
	}
}

// TestPersistTranslation_WritesAndEmits: it persists the translation and emits
// the event.
//
// §10.1's protected population, end to end: a user upgrading with
// `"appearance": "dark"` launches once, renders their pin as a constant, and
// finds BOTH the theme key and the marker in prefs.json with one
// `theme: appearance migrated` line in the log.
//
// The two keys land together (§10.5): a write that persisted the theme key
// without the marker would leave the next launch writing the marker alone and
// never emitting the event — the translation succeeding while the log says it
// failed, which is the one reading §12.3 designs the event to make impossible.
//
// session_list_mode and the retained raw appearance are asserted as survivors:
// §10.4 keeps `appearance` in place for a downgraded binary, and the mode is
// the sibling field a whole-record write would erase.
func TestPersistTranslation_WritesAndEmits(t *testing.T) {
	path := setPrefsFile(t, `{"session_list_mode":"by-tag","appearance":"dark"}`)
	syncPersistTranslation(t)
	sink := installMigrateCapture(t)

	load := migratingLoadForTest(t)

	assertLoad(t, load, prefsLoad{
		Keys:               prefs.ThemeKeys{Theme: theme.DefaultDarkSlug},
		TranslationPending: true,
		TranslatedSlug:     theme.DefaultDarkSlug,
	})
	assertPrefsOnDisk(t, path, prefsOnDisk{
		SessionListMode: "by-tag",
		Appearance:      "dark",
		Theme:           theme.DefaultDarkSlug,
		ThemeMigrated:   true,
	})
	assertThemeEvents(t, sink, migratedEvent(theme.DefaultDarkSlug))
}

// TestPersistTranslation_OneShot: it emits nothing on a subsequent launch.
//
// §10.3's trigger fires exactly once EVER, and the marker written by the first
// launch is what enforces it. The second launch is asserted on all three
// surfaces — the returned load, the on-disk bytes and the log — because a
// re-armed translation is silent in two of them: it would recompute the same
// slug and rewrite the same theme key, so only the event and the pending flag
// tell a genuine no-op apart from a redundant repeat.
func TestPersistTranslation_OneShot(t *testing.T) {
	path := setPrefsFile(t, `{"appearance":"dark"}`)
	syncPersistTranslation(t)

	first := installMigrateCapture(t)
	migratingLoadForTest(t)
	assertThemeEvents(t, first, migratedEvent(theme.DefaultDarkSlug))
	migrated := prefsBytes(t, path)

	second := installMigrateCapture(t)
	load := migratingLoadForTest(t)

	assertLoad(t, load, prefsLoad{Keys: prefs.ThemeKeys{Theme: theme.DefaultDarkSlug}})
	assertPrefsUnchanged(t, path, migrated)
	assertThemeEvents(t, second)
}

// TestPersistTranslation_MarkerOnlyIsSilent: it emits nothing for a marker-only
// write.
//
// The event fires only on a PERSISTED THEME KEY (§10.5). A run that records the
// marker alone translated nothing — either there was nothing to translate, or
// the user already had a key set and §10.3 refuses to clobber it — so announcing
// a migration would be false, and "absence is the signal the write failed" would
// stop meaning anything.
//
// The marker is still recorded in every row: "skip" means skip the theme keys,
// not the whole write (§8.9), and without it the translation stays pending
// forever. The absent-file row is the one exception, and for the opposite
// reason: §8.1 bars the migration from CREATING prefs.json, so there is nothing
// to record the marker in.
func TestPersistTranslation_MarkerOnlyIsSilent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    prefsOnDisk
	}{
		{"appearance auto", `{"appearance":"auto"}`, prefsOnDisk{Appearance: "auto", ThemeMigrated: true}},
		{"appearance absent", `{"session_list_mode":"by-tag"}`, prefsOnDisk{SessionListMode: "by-tag", ThemeMigrated: true}},
		{"a constant already set", `{"appearance":"dark","theme":"` + nordSlug + `"}`, prefsOnDisk{Appearance: "dark", Theme: nordSlug, ThemeMigrated: true}},
		{"a slot already set", `{"appearance":"dark","theme_dark":"` + nordSlug + `"}`, prefsOnDisk{Appearance: "dark", ThemeDark: nordSlug, ThemeMigrated: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := setPrefsFile(t, tc.content)
			syncPersistTranslation(t)
			sink := installMigrateCapture(t)

			migratingLoadForTest(t)

			assertPrefsOnDisk(t, path, tc.want)
			assertThemeEvents(t, sink)
		})
	}

	t.Run("an absent prefs.json is neither created nor announced", func(t *testing.T) {
		path := setPrefsFile(t, "")
		syncPersistTranslation(t)
		sink := installMigrateCapture(t)

		migratingLoadForTest(t)

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("prefs.json exists at %s (stat err %v); the migration must create nothing", path, err)
		}
		assertThemeEvents(t, sink)
	})
}

// TestPersistTranslation_FailureIsSilentAndRetryable: it emits nothing when the
// write fails.
//
// A failed write is the state §10.5 designs the whole shape around: Portal
// renders the correct theme THIS launch (the in-memory half already happened)
// and the condition is still true, so the next launch simply retries. Nothing
// is emitted, nothing is written, and the user is never flipped to the wrong
// theme — which was the translation's entire purpose.
//
// The two fixtures fail at different rungs and both must be silent: the
// malformed file aborts at §8.9's strict re-read (a write must never become an
// overwrite), and the unwritable directory aborts inside AtomicWrite with the
// record already merged.
func TestPersistTranslation_FailureIsSilentAndRetryable(t *testing.T) {
	t.Run("a malformed prefs.json aborts the write", func(t *testing.T) {
		const malformed = `{"appearance":"dark",`
		path := setPrefsFile(t, malformed)
		syncPersistTranslation(t)
		sink := installMigrateCapture(t)

		migratingLoadForTest(t)

		assertPrefsUnchanged(t, path, []byte(malformed))
		assertThemeEvents(t, sink)
		assertStillPending(t, migratingLoadForTest(t))
	})

	t.Run("an unwritable directory leaves the condition true", func(t *testing.T) {
		const seeded = `{"appearance":"dark"}`
		path := setPrefsFile(t, seeded)
		makePrefsDirUnwritable(t, path)
		syncPersistTranslation(t)
		sink := installMigrateCapture(t)

		migratingLoadForTest(t)

		assertPrefsUnchanged(t, path, []byte(seeded))
		assertThemeEvents(t, sink)

		// The retry is the point: the same launch conditions still compute the
		// same translation, so the write is owed again next time.
		retry := migratingLoadForTest(t)
		assertLoad(t, retry, prefsLoad{
			Keys:               prefs.ThemeKeys{Theme: theme.DefaultDarkSlug},
			TranslationPending: true,
			TranslatedSlug:     theme.DefaultDarkSlug,
		})
	})
}

// TestPersistTranslation_NeverEmitsCommitFailed: it never emits commit failed.
//
// §8.9 keeps `theme: commit failed` SINGLE-SITED on the panel's theme persister:
// the migration runs before any panel exists, has no reporting surface and needs
// none, and its failure signal is the absence of `theme: appearance migrated`.
// Emitting a commit-failed here would take the one event the panel reports from
// and give it a second, unreportable source.
//
// The assertion is over EVERY record the sink captured, not just the `theme`
// ones: a commit-failed emitted under some other component would be just as
// wrong, and would still not be visible to the surface that reports it.
func TestPersistTranslation_NeverEmitsCommitFailed(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"a malformed prefs.json", `{"appearance":"dark",`},
		{"a valid file whose write cannot land", `{"appearance":"dark"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := setPrefsFile(t, tc.content)
			makePrefsDirUnwritable(t, path)
			syncPersistTranslation(t)
			sink := installMigrateCapture(t)

			migratingLoadForTest(t)

			for _, rec := range sink.Records() {
				if rec.Msg == "commit failed" {
					t.Errorf("the migration emitted %q; commit failed is single-sited on the panel's theme persister", rec.Msg)
				}
			}
			assertThemeEvents(t, sink)
		})
	}
}

// assertStillPending asserts the translation is owed again — the property that
// makes a best-effort write safe (§10.5).
func assertStillPending(t *testing.T, load prefsLoad) {
	t.Helper()
	if !load.TranslationPending {
		t.Error("TranslationPending = false after a failed write; the condition must stay true so the next launch retries")
	}
}

// TestLoadPrefsStore_PersistIsNonBlocking: it does not block the load.
//
// §10.5's write is best-effort and NON-BLOCKING, which is what keeps a failing
// or slow write off the path to first paint.
//
// The proof runs through the SEAM rather than through timing, in three parts
// that fail differently. A dispatch that wrote nothing shows the load returning
// with the write demonstrably not done. The success/failure pair shows the
// load's result does not depend on the write's outcome. And the structural half
// pins the one property a substituted seam necessarily hides: that PRODUCTION's
// dispatch is a goroutine and adds nothing else to the body it defers.
func TestLoadPrefsStore_PersistIsNonBlocking(t *testing.T) {
	t.Run("the load returns with the write not yet done", func(t *testing.T) {
		const seeded = `{"appearance":"dark"}`
		path := setPrefsFile(t, seeded)
		dispatched := recordPersistTranslation(t)
		sink := installMigrateCapture(t)

		load := migratingLoadForTest(t)

		// The dispatch happened and carries what the deferred write needs...
		assertDispatched(t, dispatched, load.Store, theme.DefaultDarkSlug)
		// ...while the file is untouched and nothing has been announced, so the
		// load plainly returned without waiting for either.
		assertPrefsUnchanged(t, path, []byte(seeded))
		assertThemeEvents(t, sink)
	})

	t.Run("its result is identical whether the write succeeds or fails", func(t *testing.T) {
		want := prefsLoad{
			Keys:               prefs.ThemeKeys{Theme: theme.DefaultDarkSlug},
			TranslationPending: true,
			TranslatedSlug:     theme.DefaultDarkSlug,
		}

		t.Run("the write lands", func(t *testing.T) {
			setPrefsFile(t, `{"appearance":"dark"}`)
			syncPersistTranslation(t)
			// The successful arm emits; capture it so the assertion below is
			// about the load's result rather than about the log, and so the
			// line does not escape to the suite's stderr.
			installMigrateCapture(t)

			assertLoad(t, migratingLoadForTest(t), want)
		})

		t.Run("the write cannot land", func(t *testing.T) {
			path := setPrefsFile(t, `{"appearance":"dark"}`)
			makePrefsDirUnwritable(t, path)
			syncPersistTranslation(t)

			assertLoad(t, migratingLoadForTest(t), want)
		})
	})

	t.Run("the production dispatch runs the persist off the caller's goroutine", func(t *testing.T) {
		// Every runtime assertion above substitutes the seam, which is exactly
		// what removes the goroutine from view — so a production dispatch changed
		// to a synchronous call would leave the whole suite green. This is the
		// half that catches it, and it also pins that the wrapper adds NOTHING
		// but the deferral: a body that grew a second statement would be doing
		// work on the launch path that the substituted body never sees.
		body := varFuncLitForTest(t, "config.go", "persistTranslation").Body.List
		if len(body) != 1 {
			t.Fatalf("persistTranslation's body has %d statements, want exactly 1 (the goroutine dispatch and nothing else)", len(body))
		}
		gone, ok := body[0].(*ast.GoStmt)
		if !ok {
			t.Fatalf("persistTranslation's body is %T, want a go statement — the persist must not run on the launch path", body[0])
		}
		fn, ok := gone.Call.Fun.(*ast.Ident)
		if !ok || fn.Name != "runTranslationPersist" {
			t.Errorf("persistTranslation dispatches %v, want runTranslationPersist — the deferred body is the one tests substitute synchronously", gone.Call.Fun)
		}
	})
}

// TestPersistTranslation_DoesNotRevertAConcurrentCommit: it does not revert a
// commit made in between.
//
// The sequence the non-blocking write makes reachable, driven end to end: the
// launch computes `tokyo-night`, the user commits `nord` before the deferred
// write runs, and the write must NOT put its own key over theirs. Evaluated
// against the load-time snapshot it would — §10.3's failure displaced from
// cross-launch to intra-process — so this is what the write's read-modify-write
// re-read exists for.
//
// It is silent, too: the re-read finds a theme key set, so the run records the
// marker alone and translated nothing.
func TestPersistTranslation_DoesNotRevertAConcurrentCommit(t *testing.T) {
	path := setPrefsFile(t, `{"appearance":"dark"}`)
	dispatched := recordPersistTranslation(t)
	sink := installMigrateCapture(t)

	load := migratingLoadForTest(t)
	assertDispatched(t, dispatched, load.Store, theme.DefaultDarkSlug)

	// The user commits a theme in the window between compute and persist.
	if err := load.Store.SaveTheme(nordSlug); err != nil {
		t.Fatalf("commit %s: %v", nordSlug, err)
	}

	// Only now does the deferred write run — with exactly the arguments the
	// dispatch carried, so it is the real deferral rather than a re-derivation.
	runTranslationPersist(dispatched.calls[0].store, dispatched.calls[0].slug)

	assertPrefsOnDisk(t, path, prefsOnDisk{Appearance: "dark", Theme: nordSlug, ThemeMigrated: true})
	assertThemeEvents(t, sink)
}

// TestPersistTranslation_ConcurrentInstancesConverge: it is idempotent across
// concurrent instances.
//
// The multi-window burst makes several live instances normal, so several of them
// hitting the pending condition at once is an ordinary state rather than a
// pathological one. It is safe twice over (§10.5): every instance computes the
// same value from the same input, and the write's re-read is what lets a later
// one observe that the translation is already recorded.
func TestPersistTranslation_ConcurrentInstancesConverge(t *testing.T) {
	t.Run("both instances compute the same value from the same input", func(t *testing.T) {
		setPrefsFile(t, `{"appearance":"dark"}`)
		dispatched := recordPersistTranslation(t)

		first := migratingLoadForTest(t)
		second := migratingLoadForTest(t)

		if first.TranslatedSlug != second.TranslatedSlug {
			t.Errorf("two instances computed %q and %q; the translation is a pure function of the retained appearance", first.TranslatedSlug, second.TranslatedSlug)
		}
		if len(dispatched.calls) != 2 {
			t.Fatalf("dispatches = %d, want 2 — both instances still owe the write", len(dispatched.calls))
		}
	})

	t.Run("the loser observes the marker and writes nothing", func(t *testing.T) {
		path := setPrefsFile(t, `{"appearance":"dark"}`)
		dispatched := recordPersistTranslation(t)

		// Two instances, both constructed against the pre-migration file.
		winner := migratingLoadForTest(t)
		loser := migratingLoadForTest(t)

		won := installMigrateCapture(t)
		runTranslationPersist(winner.Store, winner.TranslatedSlug)
		assertThemeEvents(t, won, migratedEvent(theme.DefaultDarkSlug))
		migrated := prefsBytes(t, path)

		lost := installMigrateCapture(t)
		runTranslationPersist(loser.Store, loser.TranslatedSlug)

		assertPrefsUnchanged(t, path, migrated)
		assertThemeEvents(t, lost)

		if len(dispatched.calls) != 2 {
			t.Errorf("dispatches = %d, want 2 — each instance dispatches its own write", len(dispatched.calls))
		}
	})

	t.Run("a genuine race converges on one file and one line per persister", func(t *testing.T) {
		path := setPrefsFile(t, `{"appearance":"dark"}`)
		dispatched := recordPersistTranslation(t)
		sink := installMigrateCapture(t)

		instances := []prefsLoad{migratingLoadForTest(t), migratingLoadForTest(t)}
		if len(dispatched.calls) != len(instances) {
			t.Fatalf("dispatches = %d, want %d", len(dispatched.calls), len(instances))
		}

		var wg sync.WaitGroup
		for _, instance := range instances {
			wg.Go(func() {
				runTranslationPersist(instance.Store, instance.TranslatedSlug)
			})
		}
		wg.Wait()

		// Whatever the interleaving, both merged the same key into the same
		// record, so the file converges on one content.
		assertPrefsOnDisk(t, path, prefsOnDisk{Appearance: "dark", Theme: theme.DefaultDarkSlug, ThemeMigrated: true})

		// And every line that was emitted announces that same persist — an
		// instance either persisted the key or observed the marker and stayed
		// silent, never anything else.
		events := themeEvents(t, sink)
		if len(events) == 0 || len(events) > len(instances) {
			t.Fatalf("theme events = %v, want between 1 and %d", events, len(instances))
		}
		for _, event := range events {
			if want := migratedEvent(theme.DefaultDarkSlug); event != want {
				t.Errorf("theme event = %q, want %q", event, want)
			}
		}
	})
}

// TestPersistTranslation_EventShape: it emits at INFO with closed attrs.
//
// §12.3 pins the event's level and its verbatim message; its attrs are this
// task's recorded choice. `slug` carries the constant ACTUALLY persisted — the
// light fixture is deliberate, since a hardcoded dark default would satisfy a
// dark one — and there is no `slot`, because the translation always writes a
// constant (§8.2) and a slot attr would have nothing to carry.
func TestPersistTranslation_EventShape(t *testing.T) {
	setPrefsFile(t, `{"appearance":"light"}`)
	syncPersistTranslation(t)
	sink := installMigrateCapture(t)

	migratingLoadForTest(t)

	rec := sink.OnlyRecord(t)
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO — the migration is a forensic trail, not a warning", rec.Level)
	}
	if want := "appearance migrated"; rec.Msg != want {
		t.Errorf("message = %q, want %q verbatim from §12.3's catalogue", rec.Msg, want)
	}
	if want := []string{"component", "slug"}; !slices.Equal(rec.Keys, want) {
		t.Errorf("attr keys = %v, want %v (the bound component plus slug alone)", rec.Keys, want)
	}
	if got := rec.AttrString(t, "component"); got != "theme" {
		t.Errorf("component = %q, want %q", got, "theme")
	}
	if got, want := rec.AttrString(t, "slug"), theme.DefaultLightSlug; got != want {
		t.Errorf("slug = %q, want %q — the constant actually persisted", got, want)
	}

	// §12.3's closed attr-key set. Membership is asserted key by key rather than
	// by the exact set above alone, so an attr added later is caught as a
	// vocabulary breach and not merely as a changed expectation.
	closed := map[string]bool{"slug": true, "slot": true, "reason": true, "path": true, "token": true, "count": true, "rejected": true}
	for _, key := range rec.Keys {
		if key == "component" {
			continue
		}
		if !closed[key] {
			t.Errorf("attr %q is outside §12.3's closed key set %v", key, slices.Sorted(maps.Keys(closed)))
		}
	}
}

// TestPersistTranslation_NoFlashOrNoticeBand: it is silent to the user.
//
// §10.5 makes the translation silent at RUNTIME — no flash, no notice band, no
// banner. It runs before any surface exists to render one into, there is nothing
// to explain because intent is preserved exactly, and §6.3 has already refused
// the single-slot notice band a permanent extra contender.
//
// "No model-facing signal" is asserted as an identity between two frames rather
// than as the absence of some particular string: a translating launch and a
// settled launch that renders the same theme must paint the same bytes. The
// package-level assertion below is the other half — the dispatch produces no
// value at all, so there is nothing for a caller to route to the model even by
// mistake.
func TestPersistTranslation_NoFlashOrNoticeBand(t *testing.T) {
	setPrefsFile(t, `{"appearance":"dark"}`)
	syncPersistTranslation(t)
	translating := modelForNomination(themeNominationForTest(t)).View().Content

	if translating == "" {
		t.Fatal("the translating launch painted nothing; the frame comparison below would be vacuous")
	}

	setPrefsFile(t, `{"theme":"`+theme.DefaultDarkSlug+`","theme_migrated":true}`)
	settled := modelForNomination(themeNominationForTest(t)).View().Content

	if translating != settled {
		t.Errorf("the translating launch painted\n%s\nwant the settled launch's frame\n%s", translating, settled)
	}
}

// persistTranslation returns nothing, so the translation cannot hand the model a
// signal to render even by accident — the compile-time half of the silence rule.
var _ func(*prefs.Store, string) = persistTranslation

// TestOpenExecPath_NoTranslation: it never runs on the exec path.
//
// §10.5 puts the migration where a TUI is constructed, which is also the only
// place its result is used — so `portal open <target>`, which resolves and execs
// without painting, must reach neither the dispatch nor the saver. The fixture is
// a file that WOULD translate, so a stray call site is visible three ways at
// once: a recorded dispatch, a mutated prefs.json, or a `theme` record.
func TestOpenExecPath_NoTranslation(t *testing.T) {
	const seeded = `{"appearance":"dark"}`
	path := setPrefsFile(t, seeded)
	dispatched := recordPersistTranslation(t)
	sink := installMigrateCapture(t)

	if got := execOpenSession(t, "api-x7Kd9a"); got != "api-x7Kd9a" {
		t.Fatalf("open attached %q, want the session it resolved — the exec path must have run", got)
	}

	if len(dispatched.calls) != 0 {
		t.Errorf("the exec path dispatched %d translation writes, want 0", len(dispatched.calls))
	}
	assertPrefsUnchanged(t, path, []byte(seeded))
	assertThemeEvents(t, sink)
}

// translationCall is one dispatch of the persist seam: the store it was handed
// and the slug it would write.
type translationCall struct {
	store *prefs.Store
	slug  string
}

// translationDispatch records every dispatch without performing any of them, so
// a test can inspect what the load deferred — and, by the file being untouched
// afterwards, that the load did not wait for it.
type translationDispatch struct {
	calls []translationCall
}

// recordPersistTranslation installs the recording seam, restoring the previous
// value with t.Cleanup.
func recordPersistTranslation(t *testing.T) *translationDispatch {
	t.Helper()
	dispatched := &translationDispatch{}
	previous := persistTranslation
	persistTranslation = func(store *prefs.Store, slug string) {
		dispatched.calls = append(dispatched.calls, translationCall{store: store, slug: slug})
	}
	t.Cleanup(func() { persistTranslation = previous })
	return dispatched
}

// assertDispatched asserts exactly one write was deferred, carrying the process's
// own store and the computed slug.
func assertDispatched(t *testing.T, dispatched *translationDispatch, store *prefs.Store, slug string) {
	t.Helper()
	if len(dispatched.calls) != 1 {
		t.Fatalf("dispatches = %d, want exactly 1", len(dispatched.calls))
	}
	if got := dispatched.calls[0]; got.store != store || got.slug != slug {
		t.Errorf("dispatched (store %p, slug %q), want (store %p, slug %q)", got.store, got.slug, store, slug)
	}
}

// varFuncLitForTest returns the func literal a named package-level var in one of
// the cmd package's production sources is initialised with.
func varFuncLitForTest(t *testing.T, file, name string) *ast.FuncLit {
	t.Helper()

	parsed, ok := parsePackageFilesByName(t)[file]
	if !ok {
		t.Fatalf("cmd/%s is not a production source of the package", file)
	}
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, ident := range value.Names {
				if ident.Name != name || i >= len(value.Values) {
					continue
				}
				lit, ok := value.Values[i].(*ast.FuncLit)
				if !ok {
					t.Fatalf("cmd/%s's %s is initialised with %T, want a func literal", file, name, value.Values[i])
				}
				return lit
			}
		}
	}
	t.Fatalf("cmd/%s declares no var %s", file, name)
	return nil
}

// prefsBytes returns the raw on-disk bytes of prefs.json.
func prefsBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read prefs.json: %v", err)
	}
	return data
}

// assertPrefsUnchanged asserts prefs.json still holds exactly want — the
// byte-level form of "a write never becomes an overwrite".
func assertPrefsUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()
	if got := prefsBytes(t, path); !bytes.Equal(got, want) {
		t.Errorf("prefs.json = %s, want it untouched: %s", got, want)
	}
}

// makePrefsDirUnwritable strips write permission from prefs.json's directory, so
// AtomicWrite's temp-file creation fails while the file itself stays readable —
// the honest shape of a write that cannot land, with the record already merged.
//
// The mode is restored before t.TempDir's own cleanup removes the directory:
// this cleanup is registered after it, and cleanups run last-registered-first.
func makePrefsDirUnwritable(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("make prefs dir unwritable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}
