package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/xdg"
)

// configFileComponents is the closed filename -> owning-component mapping for
// the in-scope user-config files (the state-mutation audit-trail set). It is
// the single source of truth that lets configFilePath thread the correct
// owning component into migrateConfigFile's breadcrumb. A filename absent from
// this map (none today, but defensive) resolves to "" and suppresses the
// migrate emission entirely — see migrateConfigFile's empty-component guard.
var configFileComponents = map[string]string{
	"hooks.json":    "hooks",
	"aliases":       "aliases",
	"projects.json": "projects",
	// prefs.json is intentionally mapped to the empty component. It is NOT part
	// of the state-mutation audit-trail set and "prefs" is not one of the 15
	// closed log components, so it must never log under a non-catalogued
	// component. The "" value routes through migrateConfigFile's empty-component
	// guard: the one-shot move still runs best-effort while all emission is
	// suppressed. The entry is explicit (rather than relying on the unmapped ->
	// "" default) so the intent is visible in the map itself.
	"prefs.json": "",
	// terminals.json is the read-only host-terminal escape hatch. Like prefs.json
	// it is NOT part of the state-mutation audit-trail set, and it has NO
	// old-macOS-path predecessor, so its one-shot migrate is a guaranteed no-op
	// and its breadcrumb is suppressed via the empty component (mirrors the
	// prefs.json precedent).
	"terminals.json": "",
}

// migrateConfigFile moves a config file from oldPath to newPath if oldPath
// exists and newPath does not. This handles one-shot migration from the old
// macOS config location (~/Library/Application Support/portal/) to the
// XDG-compliant path. Migration is best-effort.
//
// component is the owning component of the migrated file (one of the closed
// "hooks"/"aliases"/"projects" values from configFileComponents). On a
// successful move it emits one INFO breadcrumb and on a MkdirAll/Rename failure
// one WARN — both under the owning component's logger, bound dynamically here
// because migrateConfigFile is generic across the three config files. The op
// verb "migrate" is carried BOTH as the slog message (preserving the
// `<component>: migrate` catalog shape) AND as the required "op" attr drawn from
// the closed value space, matching the store-mutation sites so JSON output and
// `grep op=migrate` filtering both work; via="migrate"; path=newPath. There is
// deliberately NO per-entry key attr (hook_key/alias/project): a whole-file move
// has no single entry key, and the path attr plus the component already identify
// the file.
//
// An EMPTY component (an unmapped filename) suppresses every emission — we must
// never log under an empty/invalid component — but the move itself still runs
// best-effort.
//
// PR-timing caveat: migrateConfigFile lands with the state-mutation work; a
// migration firing in an earlier window goes unlogged — accepted (rare
// idempotent one-shot most users already ran).
func migrateConfigFile(oldPath, newPath, component string) {
	if _, err := os.Stat(oldPath); err != nil {
		return
	}

	if _, err := os.Stat(newPath); err == nil {
		return
	} else if !os.IsNotExist(err) {
		return
	}

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		if component != "" {
			log.For(component).Warn("migrate", "op", "migrate", "via", "migrate", "path", filepath.Dir(newPath), "error", err, "error_class", "write-failed-temp-create")
		}
		return
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		if component != "" {
			log.For(component).Warn("migrate", "op", "migrate", "via", "migrate", "path", newPath, "error", err, "error_class", "write-failed-rename")
		}
		return
	}

	if component != "" {
		log.For(component).Info("migrate", "op", "migrate", "via", "migrate", "path", newPath)
	}

	// Clean up old directory if empty.
	_ = os.Remove(filepath.Dir(oldPath))
}

// configFilePath returns a config file path by checking the given environment
// variable first. If the env var is set and non-empty, its value is returned.
// Otherwise, it uses XDG-compliant resolution via xdg.ConfigBase, then appends
// portal/<filename>.
// Before returning, it attempts a one-shot migration from the old macOS
// config path (~/Library/Application Support/portal/) if the file exists there.
func configFilePath(envVar, filename string) (string, error) {
	if envPath := os.Getenv(envVar); envPath != "" {
		return envPath, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}

	configDir, err := xdg.ConfigBase()
	if err != nil {
		return "", err
	}
	newPath := filepath.Join(configDir, "portal", filename)

	oldPath := filepath.Join(homeDir, "Library", "Application Support", "portal", filename)
	// configFileComponents is the closed filename->component mapping; an
	// unmapped filename yields "" and migrateConfigFile suppresses emission.
	migrateConfigFile(oldPath, newPath, configFileComponents[filename])

	return newPath, nil
}

// loadProjectStore creates a project store from the configured file path.
// Uses PORTAL_PROJECTS_FILE env var if set (for testing), otherwise
// defaults to ~/.config/portal/projects.json.
func loadProjectStore() (*project.Store, error) {
	path, err := projectsFilePath()
	if err != nil {
		return nil, err
	}
	return project.NewStore(path), nil
}

// projectsFilePath returns the path to the projects.json file.
// Uses PORTAL_PROJECTS_FILE env var if set (for testing), otherwise
// defaults to ~/.config/portal/projects.json.
func projectsFilePath() (string, error) {
	return configFilePath("PORTAL_PROJECTS_FILE", "projects.json")
}

// loadPrefsStoreNoMigrate creates a prefs store from the configured file path
// and does NOTHING else: it resolves the path and constructs the store, reading
// no bytes and computing no translation.
//
// It is the read `portal doctor` uses (§10.5). Doctor must read prefs.json to
// report an unresolvable theme, but its contract is that it HEALS NOTHING on
// the read-only path — and a one-shot config mutation as a side effect of
// running a diagnosis is exactly what would break that claim.
//
// IT MUST NEVER GAIN BEHAVIOUR. Anything added here lands on doctor's read-only
// path by construction, invisibly: the migrating loader below is the one that
// may grow, and it is the strict superset of this one precisely so the
// inert variant cannot drift into doing work.
func loadPrefsStoreNoMigrate() (*prefs.Store, error) {
	path, err := prefsFilePath()
	if err != nil {
		return nil, err
	}
	return prefs.NewStore(path), nil
}

// prefsLoad is what the migrating prefs load produces. Keys is §8.4's "as read":
// the POST-translation in-memory value, not the on-disk bytes.
type prefsLoad struct {
	// Store is the ONE store instance for the process — it serves the initial
	// grouping-mode read, the theme keys here, and the theme persister.
	Store *prefs.Store
	// Keys are the theme keys THIS LAUNCH renders and the panel reads, with
	// §10.2's translation already applied in memory where §10.3's no-op
	// condition allowed it.
	Keys prefs.ThemeKeys
	// TranslationPending reports that the migration marker is not yet recorded,
	// so the write half is still owed. It is true even when nothing translated:
	// §10.2's "Nothing" refers to the theme keys, and the marker is what stops
	// the condition being re-evaluated forever.
	TranslationPending bool
	// TranslatedSlug is §10.2's mapping of the retained raw appearance; "" when
	// there is nothing to translate.
	TranslatedSlug string
}

// loadPrefsStore creates the process's prefs store and computes §10.5's
// appearance translation — the COMPUTE half only. It writes nothing: the
// persist and its `theme: appearance migrated` event belong to the caller.
//
// Separating computing from persisting is what makes the fix safe: the
// translated theme is used in memory immediately, so a launch renders the
// correct theme even when the write is deferred, fails, or never happens. A
// failed write leaves the condition true and the next launch retries.
//
// Ownership sits here rather than in prefs because three constraints meet at
// this call site (§10.5): prefs is a deliberate leaf that must not import
// internal/log, the translation happens at prefs load, and the `theme`
// component records it. prefs stays dumb.
//
// EVERY DEGENERATE READ IS TOLERATED AND DISCARDED, exactly as the initial-mode
// read discards its own: a missing, empty, corrupt or unreadable prefs.json
// yields empty keys and a zero migration state, which is the shipped adaptive
// pair — what an unconfigured install renders anyway. Only the path resolution
// can fail the load, and openTUI degrades from that rather than blocking.
func loadPrefsStore() (prefsLoad, error) {
	store, err := loadPrefsStoreNoMigrate()
	if err != nil {
		return prefsLoad{}, err
	}

	keys, _ := store.LoadThemeKeys()
	state, _ := store.LoadMigrationState()

	load := prefsLoad{Store: store, Keys: keys}

	// The trigger is the MARKER, never the absence of theme keys (§10.3):
	// absence-gating is re-armable, so a user who hand-edits their keys away to
	// return to the shipped pair would be silently re-pinned on the next launch.
	if state.Migrated {
		return load, nil
	}

	load.TranslationPending = true
	load.TranslatedSlug = translateAppearance(state.Appearance)

	// §10.3's no-op condition, applied to the IN-MEMORY half against the
	// load-time snapshot — the only moment early enough to affect what is
	// painted. A theme key the user has already set is what renders; scoping the
	// condition to the write alone would flip them to the translated theme for
	// exactly one launch, which is §10.1's failure delivered by the mechanism
	// added to prevent it.
	if load.TranslatedSlug != "" && keys.Theme == "" && keys.Light == "" && keys.Dark == "" {
		// §8.2's two states: a pinned mode becomes a pinned CONSTANT, so
		// detection stays off for them just as it was.
		load.Keys = prefs.ThemeKeys{Theme: load.TranslatedSlug}
	}

	// TranslatedSlug is deliberately NOT zeroed by the check above. §10.5 checks
	// the condition TWICE against two reads: here for the in-memory half, and
	// again at the write's read-modify-write re-read, where it also absorbs a
	// commit another instance made in between. Collapsing them into one would
	// lose the second read's job.

	// §10.5's PERSIST half, dispatched off the launch path. It is the last thing
	// the load does, nothing here waits on it, no error is propagated, and the
	// returned prefsLoad is identical whether the write lands, fails or never
	// finishes — which is what "best-effort and non-blocking" buys: Portal
	// renders the correct theme THIS launch regardless, so a write failure can
	// never flip the user to the wrong theme.
	//
	// The slug may be "" (nothing translated); SaveTranslation turns that into a
	// marker-only write, which is what stops the condition being re-evaluated on
	// every launch forever.
	//
	// Three properties are what make a fire-and-forget write safe here:
	//
	//   - The process may exit mid-write. fileutil.AtomicWrite is temp-file +
	//     rename, so there is no partial prefs.json to leave behind — the file is
	//     either the old bytes or the new ones.
	//   - An unfinished or failed write leaves `theme_migrated` unset, so the
	//     condition is still true and the next launch simply retries.
	//   - The write decides everything at its own read-modify-write re-read
	//     (§8.9), never against the snapshot read above. That is what stops a
	//     deferred write reverting a theme the user committed in the window
	//     between compute and persist.
	persistTranslation(store, load.TranslatedSlug)

	return load, nil
}

// persistTranslation performs §10.5's best-effort, NON-BLOCKING persist of the
// one-shot appearance translation: it dispatches the write off the launch path
// so nothing the user waits for — first paint least of all — waits on it.
//
// A package-level var so tests substitute a synchronous implementation and
// restore it with t.Cleanup (cmd's established *Deps idiom) rather than sleeping
// on a goroutine. The body is runTranslationPersist rather than an inline
// literal for exactly that reason: a synchronous substitute then runs the REAL
// save-decide-emit body instead of a re-implementation of it, so the one-shot
// cadence of `theme: appearance migrated` is actually under test. This wrapper
// adds the goroutine and nothing else.
var persistTranslation = func(store *prefs.Store, slug string) {
	go runTranslationPersist(store, slug)
}

// runTranslationPersist writes §10.5's translation and emits §12.3's
// `theme: appearance migrated` — INFO, and ONLY when a theme key was actually
// persisted.
//
// That predicate is the whole point of the event, and each rejected alternative
// breaks it differently:
//
//   - Emitted on COMPUTE it could legitimately fire on several consecutive
//     launches (the write is best-effort and retries), so "one-shot" would be
//     false.
//   - Emitted on a MARKER-ONLY write it would announce a migration that
//     translated nothing — `appearance` was `auto`, or the user already had a
//     theme key set.
//
// prefs reports that predicate as a first-class result rather than as an
// inference from the error, so this is a plain read of `persisted`.
//
// NOTHING IS EMITTED ON FAILURE — no `theme: commit failed`, no WARN, no
// user-facing surface. The absence of `theme: appearance migrated` after a
// translation IS the failure signal (§10.5), which is what keeps the
// commit-failed event single-sited on the panel's theme persister (§8.9). An
// error and a not-persisted run are therefore the same silent return.
//
// The translation is also deliberately SILENT TO THE USER AT RUNTIME — no flash,
// no notice band, no banner. It runs at prefs load, before any surface exists to
// render a notice into; there is nothing to explain, because §10.2 preserves
// intent exactly (a pinned mode becomes a pinned theme and detection stays off,
// just as it was); and §6.3 has already refused the single-slot notice band a
// permanent extra contender for a rarer event. The CHANGELOG (§12.5) is the
// compensating channel.
//
// Attrs: `slug` alone, drawn from §12.3's closed key set — the constant actually
// persisted, which is what makes the line greppable per theme. NO `slot`: the
// translation always writes a constant (§8.2), so a slot attr would have no
// value to carry. §12.3 pins the event's level and cadence but not its attrs, so
// the choice is recorded here beside the emission.
func runTranslationPersist(store *prefs.Store, slug string) {
	persisted, err := store.SaveTranslation(slug)
	if err != nil || !persisted {
		return // the absence of the event IS the failure signal (§10.5)
	}

	themeLogger.Info("appearance migrated", "slug", slug)
}

// translateAppearance maps a retained raw `appearance` value onto §10.2's
// equivalent theme slug: the single source of that table.
//
//	dark  -> the shipped dark default   (a pinned mode becomes a pinned theme)
//	light -> the shipped light default
//	auto / absent / anything else -> "" (nothing to translate)
//
// THE MATCH IS EXACT, deliberately. The translation's job is to reproduce THE
// OLD BINARY'S reading of the value, and the appearance decode §8.8 deleted
// matched the three tokens exactly — so anything that binary treated as `auto`
// (`Dark`, `" dark"`, a hand-edit's trailing newline) must translate to
// nothing. Trimming or lowercasing here would change the meaning of a value
// rather than preserve it, which is the one thing §10.2 may not do.
//
// A present-but-unrecognised value translating to nothing does NOT leave the
// translation pending forever: §10.2's "Nothing" refers to the theme keys, and
// the marker is recorded either way (§10.3).
//
// Both slugs come from theme's shipped-default constants, never from literals,
// so the shipped pair has exactly one definition.
func translateAppearance(raw string) string {
	switch raw {
	case "dark":
		return theme.DefaultDarkSlug
	case "light":
		return theme.DefaultLightSlug
	default:
		return ""
	}
}

// prefsFilePath returns the path to the prefs.json file.
// Uses PORTAL_PREFS_FILE env var if set (for testing), otherwise
// defaults to ~/.config/portal/prefs.json.
func prefsFilePath() (string, error) {
	return configFilePath("PORTAL_PREFS_FILE", "prefs.json")
}

// themesDirPath returns the path to the user themes directory, resolving
// PORTAL_THEMES_DIR -> XDG_CONFIG_HOME/portal/themes/ -> ~/.config/portal/themes/.
// An empty PORTAL_THEMES_DIR is treated as unset and falls through, matching
// configFilePath's per-file env vars.
//
// It is deliberately NOT a configFilePath member, for two mechanical reasons.
// It resolves a *directory* where configFilePath resolves *files* — which is
// what the _DIR suffix on the env var marks, against the _FILE of
// PORTAL_TERMINALS_FILE and its siblings. And there is no one-shot migration
// from the old macOS Application Support path to run: the directory is new, so
// nothing exists there to move. It therefore takes no configFileComponents
// entry and emits no migrate breadcrumb.
//
// It returns a path and nothing more — it never creates, seeds or stats the
// directory. An absent themes directory is the common, silent case (which is
// why the documented drop-in workflow starts with `mkdir -p`), and judgements
// about what is at the returned path belong to the enumeration layer, not here.
//
// cmd owning this resolution is what lets internal/theme's loader take the
// directory as an injected value and hold no path-resolution code of its own,
// keeping the embedded built-in set reachable with no path at all.
func themesDirPath() (string, error) {
	if envDir := os.Getenv("PORTAL_THEMES_DIR"); envDir != "" {
		return envDir, nil
	}

	configDir, err := xdg.ConfigBase()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "portal", "themes"), nil
}
