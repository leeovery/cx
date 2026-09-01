// Package xdg resolves XDG Base Directory paths for Portal. A dependency-free
// leaf, so any package may import it rather than re-implementing the resolution.
package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

// OSEnv is the Lookup over the process's own environment — what production
// resolution reads.
func OSEnv(name string) string { return os.Getenv(name) }

// ConfigBase returns $XDG_CONFIG_HOME verbatim when set and non-empty, else
// $HOME/.config. It errors only when neither is available.
func ConfigBase() (string, error) {
	return ConfigBaseFrom(OSEnv)
}

// ConfigBaseFrom is ConfigBase against a supplied environment. The home
// fallback reads the process's own home directory whatever the lookup is: a
// lookup carrying neither layer has nothing to say about where home is.
func ConfigBaseFrom(lookup Lookup) (string, error) {
	if base := lookup("XDG_CONFIG_HOME"); base != "" {
		return base, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}

	return filepath.Join(home, ".config"), nil
}
