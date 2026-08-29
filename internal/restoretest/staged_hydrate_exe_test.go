//go:build integration

package restoretest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStagedHydrateExe(t *testing.T) {
	t.Run("it resolves the portal binary inside the staged dir", func(t *testing.T) {
		dir := t.TempDir()

		got, err := StagedHydrateExe(t, dir)()

		if err != nil {
			t.Fatalf("resolver returned err = %v; want nil", err)
		}
		if want := filepath.Join(dir, "portal"); got != want {
			t.Errorf("resolver = %q; want %q", got, want)
		}
	})

	t.Run("an empty binDir fatals rather than degrading to a PATH lookup", func(t *testing.T) {
		fake := &fakeFataller{}

		resolver := stagedHydrateExe(fake, "")

		if !fake.fatalCalled {
			t.Fatalf("expected Fatalf for an empty binDir; resolver returned %v", resolver)
		}
		if !strings.Contains(fake.fatalMsg, "binDir") {
			t.Errorf("diagnostic %q does not name the empty parameter", fake.fatalMsg)
		}
		if fake.helperCalls == 0 {
			t.Error("expected Helper() to be called")
		}
	})
}
