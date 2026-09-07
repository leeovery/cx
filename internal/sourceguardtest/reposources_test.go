package sourceguardtest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

func TestRepoSources(t *testing.T) {
	t.Run("it returns root-relative paths for every parsed source", func(t *testing.T) {
		root, sources := sourceguardtest.RepoSources(t, sourceguardtest.AllSources)

		if !filepath.IsAbs(root) {
			t.Fatalf("root = %q, want an absolute path", root)
		}
		self := filepath.Join("internal", "sourceguardtest", "reposources_test.go")
		found := false
		for _, source := range sources {
			if filepath.IsAbs(source.Path) {
				t.Fatalf("Path = %q, want a path relative to %s", source.Path, root)
			}
			if source.File == nil || source.Fset == nil {
				t.Fatalf("%s came back unparsed", source.Path)
			}
			if source.Path == self {
				found = true
			}
		}
		if !found {
			t.Fatalf("this suite's own source %s is absent from the scan of %d sources", self, len(sources))
		}
	})

	t.Run("it narrows to test sources or to non-test sources as asked", func(t *testing.T) {
		_, all := sourceguardtest.RepoSources(t, sourceguardtest.AllSources)
		_, tests := sourceguardtest.RepoSources(t, sourceguardtest.TestSources)
		_, production := sourceguardtest.RepoSources(t, sourceguardtest.NonTestSources)

		for _, source := range tests {
			if !strings.HasSuffix(source.Path, "_test.go") {
				t.Errorf("TestSources returned %s, which is not a test source", source.Path)
			}
		}
		for _, source := range production {
			if strings.HasSuffix(source.Path, "_test.go") {
				t.Errorf("NonTestSources returned %s, which is a test source", source.Path)
			}
		}
		if len(tests)+len(production) != len(all) {
			t.Errorf("test sources (%d) + non-test sources (%d) = %d, want the whole tree's %d — the two selections partition it", len(tests), len(production), len(tests)+len(production), len(all))
		}
	})
}
