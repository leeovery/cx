package prefs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/sourceguard"
)

// appearanceAPIIdentifiers are the identifiers §8.8 puts in the "Dies" row: the
// `auto|light|dark` TYPE, its tolerant decode, its three constants and its two
// accessors.
//
// The three constants are listed BARE because that is the only shape they can
// take inside internal/prefs; everywhere else in the tree they are necessarily
// qualified, and `prefs.Appearance` catches every one of those (it is a prefix of
// `prefs.AppearanceAuto` and of every other qualified reference to the type).
//
// The on-disk FIELD is deliberately absent from this list. §8.8 kills the type
// and its API, never the slot in the file: prefs.json decodes into a plain
// struct, so a field that stops being declared is dropped on the next re-encode —
// silently erasing a downgrading user's pin on their first `s` keypress.
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

// TestPrefs_AppearanceAPIIsGone walks the whole tree — production sources AND
// tests — and fails if any of §8.8's dead appearance identifiers survives.
//
// A source guard rather than a compile-time one, because the compiler cannot
// help here: the API dies WITH its last caller, so nothing would fail to build if
// a later change reintroduced it, or if the deletion had left it declared and
// merely unused. Tests are walked too — a suite still exercising the enum would
// be a caller keeping it alive.
//
// The alternative shape — leaving the read path alongside the new one — is what
// this closes: a dead read is a second source of truth for which theme is active,
// which is exactly the divergence §13.3 removed WithAppearance to avoid.
func TestPrefs_AppearanceAPIIsGone(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	// This file necessarily names every identifier it bans, so it exempts itself
	// — the discard-guard precedent.
	self := filepath.Join("internal", "prefs", "appearance_api_guard_test.go")

	paths, err := sourceguard.GoSourceFiles(root)
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
				t.Errorf("%s still references %s; §8.8 deletes the appearance enum and its API with its last caller (the raw on-disk field stays)", rel, identifier)
			}
		}
	}
}
