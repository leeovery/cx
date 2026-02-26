package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigFilePath(t *testing.T) {
	t.Run("returns env var value when set", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_PATH", "/custom/path/file.json")

		got, err := configFilePath("TEST_CONFIG_PATH", "file.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := "/custom/path/file.json"
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}
	})

	t.Run("returns UserConfigDir fallback when env var unset", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_UNSET", "")

		got, err := configFilePath("TEST_CONFIG_UNSET", "myfile")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		configDir, err := os.UserConfigDir()
		if err != nil {
			t.Fatalf("failed to get config dir: %v", err)
		}

		want := filepath.Join(configDir, "portal", "myfile")
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}
	})
}
