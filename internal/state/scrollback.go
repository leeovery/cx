package state

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/leeovery/portal/internal/fileutil"
)

// HashMap holds the xxhash of the bytes most recently committed for each
// paneKey. A missing entry means nothing was persisted for that pane, which a
// zero hash does not — empty bytes hash non-zero.
type HashMap map[string]uint64

// PaneCapturer is declared here so internal/state need not import internal/tmux
// — which it must not, since that package imports this one. The target type is a
// parameter rather than a plain string for the same reason: the tmux client
// takes its own named target type, and hardcoding string here would put this
// seam out of its reach.
type PaneCapturer[T ~string] interface {
	CapturePane(target T) (string, error)
}

// SeedHashMap rebuilds the dedup map from the on-disk scrollback files, so the
// first cycle after a daemon restart does not rewrite every pane. It always
// returns a usable map: a missing directory is silent first-run state, and an
// unreadable directory or file is logged and skipped.
func SeedHashMap(dir string, logger *slog.Logger) HashMap {
	logger = loggerOrDiscard(logger)
	hm := HashMap{}
	sbDir := ScrollbackDir(dir)
	entries, err := os.ReadDir(sbDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return hm
		}
		logger.Warn("seed read scrollback dir failed", "path", sbDir, "error", err)
		return hm
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".bin") {
			continue
		}
		paneKey := strings.TrimSuffix(name, ".bin")
		path := filepath.Join(sbDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			logger.Warn("seed read scrollback file failed", "path", path, "error", err)
			continue
		}
		hm[paneKey] = xxhash.Sum64(data)
	}
	return hm
}

func CaptureAndHashPane[T ~string](c PaneCapturer[T], target T) ([]byte, uint64, error) {
	out, err := c.CapturePane(target)
	if err != nil {
		return nil, 0, err
	}
	return []byte(out), xxhash.Sum64String(out), nil
}

// WriteScrollbackIfChanged writes only when newHash differs from hm's entry,
// updating hm on a write. A dedup hit touches no disk and returns (false, nil).
func WriteScrollbackIfChanged(dir, paneKey string, data []byte, newHash uint64, hm HashMap) (bool, error) {
	if existing, ok := hm[paneKey]; ok && existing == newHash {
		return false, nil
	}
	path := ScrollbackFile(dir, paneKey)
	if err := fileutil.AtomicWrite0600(path, data); err != nil {
		return false, fmt.Errorf("write scrollback %s: %w", paneKey, err)
	}
	hm[paneKey] = newHash
	return true, nil
}
