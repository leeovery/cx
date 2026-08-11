package log_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

const forbiddenDiscardConstruction = "slog.NewTextHandler(io.Discard"

var discardConstructionAllowed = map[string]bool{
	filepath.Join("internal", "log", "discard.go"): true,
}

func TestNoDiscardLoggerConstructionInProductionSource(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if discardConstructionAllowed[rel] {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), forbiddenDiscardConstruction) {
			t.Errorf("production source %s constructs a discard-backed *slog.Logger; route through log.OrDiscard / log.Discard instead", rel)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk project tree: %v", walkErr)
	}
}
