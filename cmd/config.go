package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// configFilePath returns a config file path by checking the given environment
// variable first. If the env var is set and non-empty, its value is returned.
// Otherwise, it falls back to os.UserConfigDir()/portal/<filename>.
func configFilePath(envVar, filename string) (string, error) {
	if envPath := os.Getenv(envVar); envPath != "" {
		return envPath, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine config directory: %w", err)
	}

	return filepath.Join(configDir, "portal", filename), nil
}
