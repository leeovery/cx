package restoretest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/harnesstest"
)

func TestWaitForFileExists_FilePresentImmediately(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ready")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	start := time.Now()
	WaitForFileExists(t, path, 1*time.Second, 50*time.Millisecond)
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("WaitForFileExists with present file took %v; expected near-immediate return", elapsed)
	}
}

func TestWaitForFileExists_FileAppearsMidPoll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "delayed")

	done := make(chan struct{})
	go func() {
		time.Sleep(80 * time.Millisecond)
		_ = os.WriteFile(path, []byte("x"), 0o600)
		close(done)
	}()

	WaitForFileExists(t, path, 2*time.Second, 25*time.Millisecond)
	<-done
}

func TestWaitForFileExists_TimeoutFatals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "never-appears")
	budget := 50 * time.Millisecond

	rec := &harnesstest.Recorder{}
	rec.Run(func() { waitForFileExists(rec, path, budget, 10*time.Millisecond) })

	if len(rec.Fatals) != 1 {
		t.Fatalf("got %d fatals, want exactly 1 when the file never appears within %v: %v", len(rec.Fatals), budget, rec.Fatals)
	}
	if !strings.Contains(rec.Fatals[0], path) {
		t.Errorf("diagnostic %q missing absolute path %q", rec.Fatals[0], path)
	}
	if !strings.Contains(rec.Fatals[0], budget.String()) {
		t.Errorf("diagnostic %q missing budget %v", rec.Fatals[0], budget)
	}
	if rec.HelperCalls == 0 {
		t.Errorf("expected Helper() to be called at least once")
	}
}
