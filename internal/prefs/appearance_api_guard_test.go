package prefs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	// This file names every identifier it bans, so it exempts itself.
	self := filepath.Join("internal", "prefs", "appearance_api_guard_test.go")

	root, sources := sourceguardtest.RepoSources(t, sourceguardtest.AllSources)
	for _, source := range sources {
		if source.Path == self {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(root, source.Path))
		if readErr != nil {
			t.Fatalf("read %s: %v", source.Path, readErr)
		}
		for _, identifier := range appearanceAPIIdentifiers {
			if strings.Contains(string(data), identifier) {
				t.Errorf("%s still references %s; the appearance enum and its API are deleted along with their last caller (the raw on-disk field stays)", source.Path, identifier)
			}
		}
	}
}
