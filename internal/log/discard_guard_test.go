package log_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

const forbiddenDiscardConstruction = "slog.NewTextHandler(io.Discard"

var discardConstructionAllowed = map[string]bool{
	filepath.Join("internal", "log", "discard.go"): true,
}

func TestNoDiscardLoggerConstructionInProductionSource(t *testing.T) {
	root, sources := sourceguardtest.RepoSources(t, sourceguardtest.NonTestSources)
	for _, source := range sources {
		if discardConstructionAllowed[source.Path] {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(root, source.Path))
		if readErr != nil {
			t.Fatalf("read %s: %v", source.Path, readErr)
		}
		if strings.Contains(string(data), forbiddenDiscardConstruction) {
			t.Errorf("production source %s constructs a discard-backed *slog.Logger; route through log.OrDiscard / log.Discard instead", source.Path)
		}
	}
}
