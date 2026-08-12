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

// The empty component is deliberate for files outside the closed log-component
// vocabulary: the one-shot move still runs, but every emission is suppressed.
var configFileComponents = map[string]string{
	"hooks.json":     "hooks",
	"aliases":        "aliases",
	"projects.json":  "projects",
	"prefs.json":     "",
	"terminals.json": "",
}

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

	_ = os.Remove(filepath.Dir(oldPath))
}

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
	migrateConfigFile(oldPath, newPath, configFileComponents[filename])

	return newPath, nil
}

func loadProjectStore() (*project.Store, error) {
	path, err := projectsFilePath()
	if err != nil {
		return nil, err
	}
	return project.NewStore(path), nil
}

func projectsFilePath() (string, error) {
	return configFilePath("PORTAL_PROJECTS_FILE", "projects.json")
}

// Must stay inert: this is the read `portal doctor` uses, and anything added
// here becomes a config mutation on doctor's read-only diagnosis path.
func loadPrefsStoreNoMigrate() (*prefs.Store, error) {
	path, err := prefsFilePath()
	if err != nil {
		return nil, err
	}
	return prefs.NewStore(path), nil
}

// Keys carries the theme setting as this launch renders it — the
// post-translation in-memory value, not the on-disk bytes.
type prefsLoad struct {
	Store *prefs.Store
	Keys  prefs.ThemeKeys
	// True even when nothing translated: recording the marker is what stops the
	// condition being re-evaluated on every future launch.
	TranslationPending bool
	TranslatedSlug     string
}

// The translation is applied in memory before it is persisted, so this launch
// renders the correct theme even if the write fails — which leaves the marker
// unset for the next launch to retry. Every degenerate read is tolerated; only
// path resolution can fail the load. It lives here rather than in prefs, a leaf
// that must not import internal/log.
func loadPrefsStore() (prefsLoad, error) {
	store, err := loadPrefsStoreNoMigrate()
	if err != nil {
		return prefsLoad{}, err
	}

	keys, migration, _ := store.LoadThemeState()

	load := prefsLoad{Store: store, Keys: keys}

	// The trigger is the marker, never the absence of theme keys: absence-gating
	// is re-armable, so a user hand-editing their keys away to return to the
	// shipped pair would be silently re-pinned on the next launch.
	if migration.Migrated {
		return load, nil
	}

	load.TranslationPending = true
	load.TranslatedSlug = translateAppearance(migration.Appearance)

	// Applied against the load-time snapshot: the only moment early enough to
	// affect what is painted. Scoping this to the write alone would flip a user
	// who already set a theme key onto the translated theme for one launch.
	if load.TranslatedSlug != "" && keys == (prefs.ThemeKeys{}) {
		// A pinned appearance becomes a pinned constant, so detection stays off.
		load.Keys = prefs.ThemeKeys{Theme: load.TranslatedSlug}
	}

	// TranslatedSlug is deliberately not zeroed above: the write re-checks the
	// same condition against its own re-read, which is what absorbs a theme
	// another instance committed in between.
	persistTranslation(store, load.TranslatedSlug)

	return load, nil
}

// A var so a synchronous implementation can be substituted; the body lives in
// runTranslationPersist so a substitute still runs the real save-and-emit path.
var persistTranslation = func(store *prefs.Store, slug string) {
	go runTranslationPersist(store, slug)
}

// Emits only when a theme key was actually persisted — a marker-only write would
// otherwise announce a migration that translated nothing. Failure is silent: the
// absence of the event is the signal, and commit-failed belongs to the panel's
// theme persister.
func runTranslationPersist(store *prefs.Store, slug string) {
	persisted, err := store.SaveTranslation(slug)
	if err != nil || !persisted {
		return
	}

	themeLogger.Info("appearance migrated", "slug", slug)
}

// The match is exact: anything the old appearance decode treated as `auto`
// (`Dark`, `" dark"`, a trailing newline) must translate to nothing, so do not
// trim or lowercase here.
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

func prefsFilePath() (string, error) {
	return configFilePath("PORTAL_PREFS_FILE", "prefs.json")
}

// Not a configFilePath member: it resolves a directory, and there is no old
// macOS path to migrate from. It returns a path and nothing more — never
// creating, seeding or stat-ing the directory, whose absence is the common case.
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
