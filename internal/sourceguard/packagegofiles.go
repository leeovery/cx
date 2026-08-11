package sourceguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PackageGoFiles returns the .go files held directly by dir, in filename order,
// never descending into a subdirectory. Test sources are included only when
// includeTests is set. An unreadable directory and one yielding no matching file
// are both errors: a guard that enumerates nothing must fail rather than pass
// having stopped looking.
func PackageGoFiles(dir string, includeTests bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read package dir %s: %w", dir, err)
	}
	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if !includeTests && strings.HasSuffix(name, "_test.go") {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .go files in %s", dir)
	}
	return paths, nil
}
