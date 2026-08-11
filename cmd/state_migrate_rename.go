package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/spf13/cobra"
)

// Invoked from a session-renamed hook with the old and new session names.
var stateMigrateRenameCmd = &cobra.Command{
	Use:    "migrate-rename <old-name> <new-name>",
	Short:  "Migrate hook keys across a session rename (internal)",
	Args:   cobra.ExactArgs(2),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := loadHookStore()
		if err != nil {
			return fmt.Errorf("load hook store: %w", err)
		}

		return runMigrateRename(store, args[0], args[1], hooksLogger)
	},
}

// The prefix carries a trailing colon so similarly-prefixed session names do not
// match ("work" must not match "work-2:0.0"). Matching keys are collected before
// rewriting because mutating a map during iteration is unspecified.
func runMigrateRename(store *hooks.Store, oldName, newName string, logger *slog.Logger) error {
	if newName == "" {
		return fmt.Errorf("new name must be non-empty")
	}

	h, err := store.Load()
	if err != nil {
		// Load swallows missing-file and malformed-JSON, so this is genuine I/O.
		logger.Warn("load hooks failed", "error", err)
		return err
	}

	prefix := oldName + ":"
	var toMigrate []string
	for key := range h {
		if strings.HasPrefix(key, prefix) {
			toMigrate = append(toMigrate, key)
		}
	}

	if len(toMigrate) == 0 {
		return nil
	}

	for _, key := range toMigrate {
		events := h[key]
		newKey := newName + ":" + strings.TrimPrefix(key, prefix)
		if _, collision := h[newKey]; collision {
			logger.Warn("hook key collision; overwriting", "hook_key", newKey)
		}
		h[newKey] = events
		delete(h, key)
	}

	// The audited seam owns both the success INFO and the save-failure WARN, so
	// this path adds no warn of its own.
	return store.SaveAudited(h, "modify", len(toMigrate), "internal")
}

func init() {
	stateCmd.AddCommand(stateMigrateRenameCmd)
}
