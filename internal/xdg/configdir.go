package xdg

import "path/filepath"

// portalDirName is the segment Portal owns under the config base. Every path
// this package resolves passes through it, file and directory alike.
const portalDirName = "portal"

// ConfigDirID identifies one of Portal's directories under the config base: the
// environment variable that overrides its location and the name it takes
// otherwise. A directory is a `_DIR` where a config file is a `_FILE`, and it
// has no predecessor under the old macOS config path — so unlike ConfigFileID it
// carries no log component, because nothing about it is ever migrated or
// announced.
type ConfigDirID struct {
	EnvVar  string
	Dirname string
}

// Portal's directories under the config base. The state directory is not config
// content, but it takes its location by the same rule.
var (
	StateDir  = ConfigDirID{EnvVar: "PORTAL_STATE_DIR", Dirname: "state"}
	ThemesDir = ConfigDirID{EnvVar: "PORTAL_THEMES_DIR", Dirname: "themes"}
)

// ConfigDirPath declares Portal's config-directory precedence, once and for
// every reader of it: the directory's own environment variable, else
// <config base>/portal/<dirname>, resolved through the same config base as a
// config file.
//
// It returns a bare path where ConfigFilePath returns a ConfigFile: there is no
// old location for a directory to have come from, so no caller has a migration
// to gate on where the path was resolved from. It resolves and nothing more —
// creating nothing and stat-ing nothing.
func ConfigDirPath(lookup Lookup, id ConfigDirID) (string, error) {
	if dir := lookup(id.EnvVar); dir != "" {
		return dir, nil
	}

	base, err := ConfigBaseFrom(lookup)
	if err != nil {
		return "", err
	}

	return filepath.Join(base, portalDirName, id.Dirname), nil
}
