// Package xdg resolves XDG Base Directory paths for Portal. It is a dependency-
// free leaf, so any package may import it and $XDG_CONFIG_HOME resolution has a
// single home.
package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigBase returns $XDG_CONFIG_HOME verbatim when set and non-empty, else
// $HOME/.config. It errors only when neither is available.
func ConfigBase() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}

	return filepath.Join(home, ".config"), nil
}
