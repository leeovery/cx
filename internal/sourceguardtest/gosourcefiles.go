package sourceguardtest

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// GoSourceFiles returns every .go file under root, test sources included, as
// paths joined onto root. Dot-directories, vendor and node_modules are skipped:
// nothing a guard polices lives there.
func GoSourceFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			// WalkDir calls back for root itself first; excluding it on its own
			// base name would skip the whole walk in a dot-prefixed checkout
			// (a worktree under .worktrees/), so no guard could run there.
			if path != root && excludedGuardDir(entry.Name()) {
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
	if len(paths) == 0 {
		return nil, fmt.Errorf("walk %s: no .go files, so a guard over them would pass by having stopped looking", root)
	}
	return paths, nil
}

func excludedGuardDir(name string) bool {
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}
	return name == "vendor" || name == "node_modules"
}
