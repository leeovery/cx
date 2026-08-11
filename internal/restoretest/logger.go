package restoretest

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const portalLogName = "portal.log"

// Mirrors internal/log's dateLayout and must stay in lockstep with it: a
// co-resident real binary has to append to the same dated file, not a divergent
// one.
const portalLogDateLayout = "2006-01-02"

// OpenTestLogger returns a *slog.Logger writing into the production sink's
// on-disk shape under stateDir: a dated portal.log.<date> file plus a portal.log
// symlink to it, closed on test cleanup.
//
// The symlink shape is load-bearing when the test also spawns the real portal
// binary against the same stateDir: production's reopen removes a regular-file
// portal.log but no-ops on a symlink, so both writers append to the same file.
// To capture output in-process instead, use log.SetTestHandler.
func OpenTestLogger(t *testing.T, stateDir string) *slog.Logger {
	t.Helper()

	dayName := portalLogName + "." + time.Now().Format(portalLogDateLayout)
	dayPath := filepath.Join(stateDir, dayName)

	f, err := os.OpenFile(dayPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenTestLogger: open %s: %v", dayPath, err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if err := swingPortalLogSymlink(stateDir, dayName); err != nil {
		t.Fatalf("OpenTestLogger: %v", err)
	}

	return slog.New(slog.NewTextHandler(f, nil))
}

// swingPortalLogSymlink mirrors the production sink's swing, so refreshing an
// existing portal.log always leaves a symlink behind — which is what the
// production migration guard expects to find.
func swingPortalLogSymlink(stateDir, target string) error {
	link := filepath.Join(stateDir, portalLogName)
	tmp := link + ".restoretest.symlink.tmp"

	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale symlink temp %s: %w", tmp, err)
	}
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("create symlink temp %s -> %s: %w", tmp, target, err)
	}
	if err := os.Rename(tmp, link); err != nil {
		return fmt.Errorf("rename symlink temp %s -> %s: %w", tmp, link, err)
	}
	return nil
}
