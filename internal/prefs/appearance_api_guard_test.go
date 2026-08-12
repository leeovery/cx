package prefs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The on-disk field is deliberately not banned here: it must stay declared, or
// re-encodes drop the key and erase a downgrading user's pin.
var appearanceAPIIdentifiers = []string{
	"prefs.Appearance",
	"type Appearance",
	"parseAppearance",
	"LoadAppearance",
	"SaveAppearance",
	"AppearanceAuto",
	"AppearanceLight",
	"AppearanceDark",
}

// A source guard, not a compile-time one: the API died with its last caller,
// so a reintroduction would still build.
func TestPrefs_AppearanceAPIIsGone(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	// This file names every identifier it bans, so it exempts itself.
	self := filepath.Join("internal", "prefs", "appearance_api_guard_test.go")

	paths, err := sourceguardtest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate .go files: %v", err)
	}
	for _, path := range paths {
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatalf("relativise %s: %v", path, relErr)
		}
		if rel == self {
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", rel, readErr)
		}
		for _, identifier := range appearanceAPIIdentifiers {
			if strings.Contains(string(data), identifier) {
				t.Errorf("%s still references %s; the appearance enum and its API are deleted along with their last caller (the raw on-disk field stays)", rel, identifier)
			}
		}
	}
}
