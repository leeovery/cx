package sourceguardtest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

func TestRepoSourcesRooted(t *testing.T) {
	// A guard driven against a staged tree is the same scan at another root,
	// which is what lets its own rule tests exercise it on fixtures.
	t.Run("it scans the tree it is rooted at", func(t *testing.T) {
		root := stageScanTree(t, map[string]string{
			"production.go":      "package fixture\n",
			"fixture_test.go":    "package fixture\n",
			"sub/nested_test.go": "package nested\n",
		})

		gotRoot, sources := sourceguardtest.RepoSources(t, sourceguardtest.TestSources, sourceguardtest.Rooted(root))

		if gotRoot != root {
			t.Errorf("root = %q, want %q", gotRoot, root)
		}
		var paths []string
		for _, source := range sources {
			paths = append(paths, source.Path)
		}
		want := []string{"fixture_test.go", filepath.Join("sub", "nested_test.go")}
		if strings.Join(paths, ",") != strings.Join(want, ",") {
			t.Errorf("paths = %v, want %v", paths, want)
		}
	})

	t.Run("it fatals when the selection matches no source at all", func(t *testing.T) {
		root := stageScanTree(t, map[string]string{"production.go": "package fixture\n"})

		recorder := &harnesstest.Recorder{}
		var sources []sourceguardtest.ParsedSource
		recorder.Run(func() {
			_, sources = sourceguardtest.RepoSources(recorder, sourceguardtest.TestSources, sourceguardtest.Rooted(root))
		})

		if len(sources) != 0 {
			t.Fatalf("returned %d sources, want none", len(sources))
		}
		if len(recorder.Fatals) != 1 || !strings.Contains(recorder.Fatals[0], "stopped looking") {
			t.Fatalf("fatals = %v, want the single scanned-nothing tripwire", recorder.Fatals)
		}
	})
}

func stageScanTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("stage %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}
