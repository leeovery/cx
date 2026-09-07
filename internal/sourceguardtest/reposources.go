package sourceguardtest

import (
	"path/filepath"
	"strings"

	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/portalbintest"
)

// Selection names the lane of the tree a scan reads. A guard states which lane
// it polices and nothing else: which files that is, and how they are found, is
// decided here rather than at each guard.
type Selection int

const (
	// AllSources reads every .go file, production and test alike.
	AllSources Selection = iota
	// TestSources reads the _test.go files alone.
	TestSources
	// NonTestSources reads everything but the _test.go files.
	NonTestSources
)

func (s Selection) accepts(path string) bool {
	isTest := strings.HasSuffix(path, "_test.go")
	switch s {
	case TestSources:
		return isTest
	case NonTestSources:
		return !isTest
	default:
		return true
	}
}

// ScanOption adjusts which tree a scan reads.
type ScanOption func(*scanConfig)

type scanConfig struct{ root string }

// Rooted anchors a scan at a tree of the caller's choosing rather than at the
// project root, so a guard's own rule tests drive the same scan over a staged
// fixture.
func Rooted(root string) ScanOption {
	return func(c *scanConfig) { c.root = root }
}

// ProjectRoot is the module root every guard is anchored at, resolution failure
// fatal. A guard that only needs to name a path within the tree — to anchor a
// `go list`, or to join a file it reads itself — takes it here rather than
// re-authoring the resolution beside its own fatal.
func ProjectRoot(t harnesstest.TestingT) string {
	t.Helper()

	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
		return ""
	}
	return root
}

// scanRoot resolves the tree a scan reads: the one the options name, or the
// project root when they name none.
func scanRoot(t harnesstest.TestingT, opts []ScanOption) string {
	t.Helper()

	var config scanConfig
	for _, opt := range opts {
		opt(&config)
	}
	if config.root != "" {
		return config.root
	}
	return ProjectRoot(t)
}

// RepoSources parses the repository's .go files that sel selects, returning the
// project root alongside them. Each ParsedSource carries a Path relative to that
// root, so a guard's finding reads as a repository path rather than as one
// machine's checkout, and a caller needing the bytes joins the two.
//
// Resolving the root, enumerating the tree, parsing a file and selecting nothing
// at all are all fatal: a guard that scanned nothing reports a safety it is not
// providing.
func RepoSources(t harnesstest.TestingT, sel Selection, opts ...ScanOption) (root string, sources []ParsedSource) {
	t.Helper()

	root = scanRoot(t, opts)
	if root == "" {
		return "", nil
	}

	paths, err := GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate .go files: %v", err)
		return "", nil
	}

	selected := make([]string, 0, len(paths))
	for _, path := range paths {
		if sel.accepts(path) {
			selected = append(selected, path)
		}
	}

	sources = ParseSources(t, selected)
	for i, source := range sources {
		rel, relErr := filepath.Rel(root, source.Path)
		if relErr != nil {
			t.Fatalf("relativise %s: %v", source.Path, relErr)
			return "", nil
		}
		sources[i].Path = rel
	}
	return root, sources
}
