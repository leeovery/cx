package cmd

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

func migratedEvent(slug string) string {
	return "INFO appearance migrated slug=" + slug
}

// syncPersistTranslation installs the production persist body as the dispatch
// seam, minus the goroutine. Substituting rather than re-implementing keeps the
// suite from asserting against its own copy of the save-decide-emit body, and
// dropping the goroutine is what makes the assertions deterministic.
func syncPersistTranslation(t *testing.T) {
	t.Helper()
	previous := persistTranslation
	persistTranslation = runTranslationPersist
	t.Cleanup(func() { persistTranslation = previous })
}

type prefsOnDisk struct {
	SessionListMode string `json:"session_list_mode"`
	Appearance      string `json:"appearance"`
	Theme           string `json:"theme"`
	ThemeLight      string `json:"theme_light"`
	ThemeDark       string `json:"theme_dark"`
	ThemeMigrated   bool   `json:"theme_migrated"`
}

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

func assertPrefsOnDisk(t *testing.T, path string, want prefsOnDisk) {
	t.Helper()
	if got := readPrefsOnDisk(t, path); got != want {
		t.Errorf("prefs.json = %+v, want %+v", got, want)
	}
}

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

		retry := migratingLoadForTest(t)
		assertLoad(t, retry, prefsLoad{
			Keys:               prefs.ThemeKeys{Theme: theme.DefaultDarkSlug},
			TranslationPending: true,
			TranslatedSlug:     theme.DefaultDarkSlug,
		})
	})
}

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

func assertStillPending(t *testing.T, load prefsLoad) {
	t.Helper()
	if !load.TranslationPending {
		t.Error("TranslationPending = false after a failed write; the condition must stay true so the next launch retries")
	}
}

func TestLoadPrefsStore_PersistIsNonBlocking(t *testing.T) {
	t.Run("the load returns with the write not yet done", func(t *testing.T) {
		const seeded = `{"appearance":"dark"}`
		path := setPrefsFile(t, seeded)
		dispatched := recordPersistTranslation(t)
		sink := installMigrateCapture(t)

		load := migratingLoadForTest(t)

		assertDispatched(t, dispatched, load.Store, theme.DefaultDarkSlug)
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

func TestPersistTranslation_DoesNotRevertAConcurrentCommit(t *testing.T) {
	path := setPrefsFile(t, `{"appearance":"dark"}`)
	dispatched := recordPersistTranslation(t)
	sink := installMigrateCapture(t)

	load := migratingLoadForTest(t)
	assertDispatched(t, dispatched, load.Store, theme.DefaultDarkSlug)

	if err := load.Store.SaveTheme(nordSlug); err != nil {
		t.Fatalf("commit %s: %v", nordSlug, err)
	}

	runTranslationPersist(dispatched.calls[0].store, dispatched.calls[0].slug)

	assertPrefsOnDisk(t, path, prefsOnDisk{Appearance: "dark", Theme: nordSlug, ThemeMigrated: true})
	assertThemeEvents(t, sink)
}

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

		assertPrefsOnDisk(t, path, prefsOnDisk{Appearance: "dark", Theme: theme.DefaultDarkSlug, ThemeMigrated: true})

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

// Compile-time check that the seam returns nothing, so the translation cannot
// hand the model a signal to render.
var _ func(*prefs.Store, string) = persistTranslation

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

type translationCall struct {
	store *prefs.Store
	slug  string
}

type translationDispatch struct {
	calls []translationCall
}

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

func assertDispatched(t *testing.T, dispatched *translationDispatch, store *prefs.Store, slug string) {
	t.Helper()
	if len(dispatched.calls) != 1 {
		t.Fatalf("dispatches = %d, want exactly 1", len(dispatched.calls))
	}
	if got := dispatched.calls[0]; got.store != store || got.slug != slug {
		t.Errorf("dispatched (store %p, slug %q), want (store %p, slug %q)", got.store, got.slug, store, slug)
	}
}

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

func prefsBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read prefs.json: %v", err)
	}
	return data
}

func assertPrefsUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()
	if got := prefsBytes(t, path); !bytes.Equal(got, want) {
		t.Errorf("prefs.json = %s, want it untouched: %s", got, want)
	}
}

// makePrefsDirUnwritable strips write permission from prefs.json's directory so
// AtomicWrite's temp-file creation fails while the file stays readable.
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
