package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The window is comfortably under the daemon's 1s ticker period, so a
// sessions.json observed inside it can only have come from notify itself.
const sixEventFiringWindow = 500 * time.Millisecond

func TestNotifyCommand_TouchesSaveRequestedAndWritesNoSessionsJSON(t *testing.T) {
	t.Run("firing notifyCommand touches save.requested and writes nothing to sessions.json", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", dir)

		if _, _, err := runStateNotify(t); err != nil {
			t.Fatalf("notify subprocess failed: %v", err)
		}

		saveRequestedPath := filepath.Join(dir, "save.requested")
		if _, err := os.Stat(saveRequestedPath); err != nil {
			t.Fatalf(
				"notify did not touch save.requested (the dirty-flag invariant "+
					"the six events rely on): stat err = %v\n"+
					"--- state dir listing ---\n%s",
				err, dumpStateDirForNotifyTest(dir),
			)
		}

		// notify is synchronous today, so a single post-call stat would do; the
		// poll defends against a future regression that writes sessions.json from a
		// goroutine.
		sessionsJSONPath := filepath.Join(dir, "sessions.json")
		deadline := time.Now().Add(sixEventFiringWindow)
		const pollInterval = 25 * time.Millisecond
		for time.Now().Before(deadline) {
			if _, err := os.Stat(sessionsJSONPath); err == nil {
				// Surface the contents so the post-mortem shows what notify wrote.
				blob, _ := os.ReadFile(sessionsJSONPath)
				t.Fatalf(
					"notify wrote sessions.json within %s of exit "+
						"(spec § Acceptance Criteria items 9 and 13 "+
						"require zero sessions.json writes from notify):\n"+
						"--- sessions.json contents ---\n%s\n"+
						"--- state dir listing ---\n%s",
					sixEventFiringWindow, string(blob), dumpStateDirForNotifyTest(dir),
				)
			} else if !os.IsNotExist(err) {
				t.Fatalf("stat sessions.json during bounded window: %v", err)
			}
			time.Sleep(pollInterval)
		}

		if _, err := os.Stat(sessionsJSONPath); err == nil {
			blob, _ := os.ReadFile(sessionsJSONPath)
			t.Fatalf(
				"sessions.json exists at end of %s window (notify must not write it):\n"+
					"--- sessions.json contents ---\n%s",
				sixEventFiringWindow, string(blob),
			)
		} else if !os.IsNotExist(err) {
			t.Fatalf("final stat sessions.json: %v", err)
		}
	})
}

func dumpStateDirForNotifyTest(stateDir string) string {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return fmt.Sprintf("(readdir %s: %v)", stateDir, err)
	}
	var lines []string
	for _, e := range entries {
		info, ierr := e.Info()
		if ierr != nil {
			lines = append(lines, fmt.Sprintf("%s (stat err: %v)", e.Name(), ierr))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s (size=%d, mode=%s)", e.Name(), info.Size(), info.Mode()))
	}
	return strings.Join(lines, "\n")
}
