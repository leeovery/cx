package sourceguardtest_test

import (
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// A finding names the file the way the scan does, so a guard's complaint reads
// as a repository path rather than as one machine's checkout.
func TestParsedSourcePosition(t *testing.T) {
	t.Run("it names the position's file as the source's own path", func(t *testing.T) {
		root := stageScanTree(t, map[string]string{"pkg/decl.go": "package pkg\n\nvar Declared = 1\n"})

		_, sources := sourceguardtest.RepoSources(t, sourceguardtest.NonTestSources, sourceguardtest.Rooted(root))
		source := sources[0]

		got := source.Position(source.File.Decls[0].Pos())
		if got.Filename != source.Path {
			t.Errorf("Filename = %q, want %q", got.Filename, source.Path)
		}
		if got.Line != 3 {
			t.Errorf("Line = %d, want 3", got.Line)
		}
		if got.String() != source.Path+":3:1" {
			t.Errorf("String() = %q, want %q", got.String(), source.Path+":3:1")
		}
	})
}
