package restoretest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/restoretest"
)

func TestOpenTestLogger_WritesToPortalLog(t *testing.T) {
	stateDir := t.TempDir()

	logger := restoretest.OpenTestLogger(t, stateDir)
	if logger == nil {
		t.Fatal("OpenTestLogger returned nil; want non-nil *slog.Logger")
	}

	logger.Info("smoke-marker", "key", "value")

	path := filepath.Join(stateDir, "portal.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read portal.log: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "smoke-marker") {
		t.Errorf("portal.log missing logged message; got:\n%s", got)
	}
	if !strings.Contains(got, "key=value") {
		t.Errorf("portal.log missing logged attr; got:\n%s", got)
	}
	if !strings.Contains(got, "INFO") {
		t.Errorf("portal.log missing slog text level label; got:\n%s", got)
	}
}

func TestOpenTestLogger_ProducesProductionSinkShape(t *testing.T) {
	stateDir := t.TempDir()

	_ = restoretest.OpenTestLogger(t, stateDir)

	link := filepath.Join(stateDir, "portal.log")
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat portal.log: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("portal.log is not a symlink (mode %v); the production migration guard would unlink a regular file", info.Mode())
	}

	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink portal.log: %v", err)
	}
	wantTarget := "portal.log." + time.Now().Format("2006-01-02")
	if target != wantTarget {
		t.Fatalf("portal.log symlink target = %q; want %q (bare relative dated basename)", target, wantTarget)
	}

	dayPath := filepath.Join(stateDir, wantTarget)
	dayInfo, err := os.Stat(dayPath)
	if err != nil {
		t.Fatalf("stat %s: %v", wantTarget, err)
	}
	if !dayInfo.Mode().IsRegular() {
		t.Fatalf("%s is not a regular file (mode %v)", wantTarget, dayInfo.Mode())
	}
}
