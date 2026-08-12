package sourceguardtest

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// GoSourceFiles returns every .go file under root, test sources included, as
// paths joined onto root. Dot-directories, vendor and node_modules are skipped:
// Go's own tooling ignores them, so nothing a guard polices can hide there.
func GoSourceFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if excludedGuardDir(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	return paths, nil
}

func excludedGuardDir(name string) bool {
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}
	return name == "vendor" || name == "node_modules"
}
