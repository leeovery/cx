package state

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/leeovery/portal/internal/fileutil"
)

// Commit atomically persists idx and garbage-collects unreferenced scrollback
// files, but only when something changed: the caller's anyScrollbackChanged
// flag, or a structural difference from the prior on-disk index with SavedAt
// ignored on both sides, so timestamp churn alone never triggers a write. A GC
// failure is logged, not returned — sessions.json is the source of truth.
func Commit(dir string, idx Index, anyScrollbackChanged bool, logger *slog.Logger) error {
	logger = loggerOrDiscard(logger)
	idx.Canonicalize()

	data, err := EncodeIndex(idx)
	if err != nil {
		return fmt.Errorf("encode sessions.json: %w", err)
	}

	if !structuralChange(dir, idx) && !anyScrollbackChanged {
		return nil
	}

	if err := fileutil.AtomicWrite0600(SessionsJSON(dir), data); err != nil {
		return fmt.Errorf("write sessions.json: %w", err)
	}

	if err := gcOrphanScrollback(dir, idx, logger); err != nil {
		logger.Warn("gc orphan scrollback failed", "error", err)
	}

	return nil
}

func structuralChange(dir string, idx Index) bool {
	priorBytes, err := os.ReadFile(SessionsJSON(dir))
	if err != nil {
		return true
	}
	prior, err := DecodeIndex(priorBytes)
	if err != nil {
		return true
	}
	prior.Canonicalize()

	a := idx
	a.SavedAt = time.Time{}
	b := prior
	b.SavedAt = time.Time{}
	return !reflect.DeepEqual(a, b)
}

// ComputeReferencedSet collects ScrollbackFile paths verbatim, as stored in idx.
func ComputeReferencedSet(idx Index) map[string]struct{} {
	set := make(map[string]struct{})
	for _, s := range idx.Sessions {
		for _, w := range s.Windows {
			for _, p := range w.Panes {
				set[p.ScrollbackFile] = struct{}{}
			}
		}
	}
	return set
}

func gcOrphanScrollback(dir string, idx Index, logger *slog.Logger) error {
	sbDir := ScrollbackDir(dir)
	entries, err := os.ReadDir(sbDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	refSet := ComputeReferencedSet(idx)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".bin") {
			continue
		}
		// The on-disk schema stores forward slashes on every platform.
		relPath := filepath.ToSlash(filepath.Join("scrollback", name))
		if _, found := refSet[relPath]; found {
			continue
		}

		fullPath := filepath.Join(sbDir, name)
		if err := os.Remove(fullPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			paneKey := strings.TrimSuffix(name, ".bin")
			logger.Warn("gc remove scrollback failed", "pane_key", paneKey, "error", err)
		}
	}
	return nil
}
