//go:build integration

package restoretest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/restore"
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
		rec := &harnesstest.Recorder{}

		var resolver restore.ExecutableResolver
		rec.Run(func() { resolver = stagedHydrateExe(rec, "") })

		if len(rec.Fatals) != 1 {
			t.Fatalf("got %d fatals for an empty binDir, want exactly 1; resolver returned %v", len(rec.Fatals), resolver)
		}
		if !strings.Contains(rec.Fatals[0], "binDir") {
			t.Errorf("diagnostic %q does not name the empty parameter", rec.Fatals[0])
		}
		if rec.HelperCalls == 0 {
			t.Error("expected Helper() to be called")
		}
	})
}
