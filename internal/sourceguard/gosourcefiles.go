package sourceguard

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// GoSourceFiles returns every .go file under root, test sources included,
// as paths joined onto root.
//
// It records in one place what the guards that consume it cover:
// directories whose name begins with "." are skipped, as are vendor and
// node_modules. The skipped tree holds only documentation scaffolding that Go's
// own tooling already ignores (a leading dot keeps a directory out of every
// build and every ./... pattern), so nothing a guard is written to police can
// hide there. A guard that walks its own subset is a guard narrower than its
// siblings by accident.
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
