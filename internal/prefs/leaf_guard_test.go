package prefs_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

const prefsPkg = "github.com/leeovery/portal/internal/prefs"

// prefs stays a leaf so internal/tui can import it without a cycle.
var forbiddenLeafDeps = []string{
	"github.com/leeovery/portal/internal/log",
	"github.com/leeovery/portal/internal/storelog",
}

func TestPrefsIsALeaf(t *testing.T) {
	// Anchored at the import path so it resolves regardless of the test
	// binary's runtime CWD.
	deps := sourceguardtest.PackageDeps(t, prefsPkg)
	for _, dep := range deps {
		for _, forbidden := range forbiddenLeafDeps {
			if dep == forbidden {
				t.Fatalf("internal/prefs transitively imports %s — prefs must stay a leaf (stdlib + internal/fileutil only)", forbidden)
			}
		}
	}

	// Keeps the guard from passing vacuously.
	const fileutilPkg = "github.com/leeovery/portal/internal/fileutil"
	var sawFileutil bool
	for _, dep := range deps {
		if !strings.HasPrefix(dep, "github.com/leeovery/portal/internal/") {
			continue
		}
		switch dep {
		case prefsPkg, fileutilPkg:
			if dep == fileutilPkg {
				sawFileutil = true
			}
		default:
			t.Errorf("internal/prefs has an unexpected internal dependency %s — prefs is meant to be a leaf over stdlib + internal/fileutil", dep)
		}
	}
	if !sawFileutil {
		t.Errorf("internal/prefs no longer depends on %s — the leaf guard may be vacuous", fileutilPkg)
	}
}
