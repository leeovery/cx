package cmd

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
)

// assertLockFailureReachesStderr pins the route a lock failure takes to the
// user: a plain error cobra neither silences nor dresses as a usage problem, so
// main's classify prints its message and exits 1.
func assertLockFailureReachesStderr(t *testing.T, out string, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a non-zero exit under a held lock, got nil")
	}
	if !errors.Is(err, hooks.ErrLockHeld) {
		t.Errorf("error = %v, want errors.Is ErrLockHeld", err)
	}
	if !strings.Contains(err.Error(), hooks.ErrLockHeld.Error()) {
		t.Errorf("error = %q, want it to carry the reason %q", err.Error(), hooks.ErrLockHeld.Error())
	}
	if _, ok := errors.AsType[*UsageError](err); ok {
		t.Errorf("error %v is a *UsageError; a lock timeout is not a usage error", err)
	}
	if IsSilentExitError(err) {
		t.Error("error is a silent-exit error; its reason would never reach stderr")
	}
	if strings.Contains(out, "Usage:") {
		t.Errorf("command output = %q, want no usage dump", out)
	}
}

// assertOneLockWarn pins the single WARN a timed-out mutation leaves as the
// command runs it: the operation's own op, the key it could not write, and the
// caller. One line, so the dirty-flag touch cannot have run behind it either.
func assertOneLockWarn(t *testing.T, sink *logtest.Sink, wantOp, wantKey string) {
	t.Helper()
	warns := sink.RecordsAtOrAboveLevel(slog.LevelWarn)
	if len(warns) != 1 {
		t.Fatalf("WARN record count = %d, want exactly 1: %+v", len(warns), warns)
	}
	rec := warns[0]
	assertHooksRecord(t, rec, hooksRecordWant{
		level: slog.LevelWarn,
		msg:   wantOp,
		op:    wantOp,
		via:   "cli",
	})
	if got := rec.AttrString(t, "hook_key"); got != wantKey {
		t.Errorf("hook_key = %q, want %q", got, wantKey)
	}
	if rec.HasAttr("error_class") {
		t.Errorf("lock WARN carries error_class — no write phase ran: %+v", rec.Attrs)
	}
	if rec.HasAttr("value") {
		t.Errorf("lock WARN carries value — the file was never opened: %+v", rec.Attrs)
	}
}

// assertReturnsAtLockBound proves the mutation waits the bound out and returns,
// rather than blocking forever on a lock it can never take.
func assertReturnsAtLockBound(t *testing.T, verb string, run func() error) {
	t.Helper()
	start := time.Now()
	err := run()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a non-zero exit under a held lock, got nil")
	}
	if elapsed < lockBound {
		t.Errorf("hook %s returned after %v — it did not wait out the %v bound", verb, elapsed, lockBound)
	}
	// A generous multiple: the assertion separates bounded from unbounded, and
	// a unit lane under load must not read as a hang.
	if limit := 20 * lockBound; elapsed > limit {
		t.Errorf("hook %s took %v against a %v bound (limit %v) — it is not returning at the bound", verb, elapsed, lockBound, limit)
	}
}

func TestHookSetLockTimeout(t *testing.T) {
	t.Run("it exits non-zero from hook set on a lock timeout", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		before := seedHooksFile(t, hooksFile, map[string]map[string]string{
			"tok999": {"on-resume": "npm start"},
		})
		hookstest.HoldHooksSidecar(t, hooksFile)

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		sink := logtest.Install(t)
		out, err := runHookSet(t, "claude --resume abc")
		assertLockFailureReachesStderr(t, out, err)
		assertOneLockWarn(t, sink, "set", "tok123")
		assertHooksFileUnchanged(t, hooksFile, before)
	})

	t.Run("it leaves an absent hooks.json absent on a lock timeout", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		hookstest.HoldHooksSidecar(t, hooksFile)

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		if _, err := runHookSet(t, "claude --resume abc"); err == nil {
			t.Fatal("expected a non-zero exit under a held lock, got nil")
		}
		if _, statErr := os.Stat(hooksFile); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("hooks.json created despite the timeout: %v", statErr)
		}
	})

	t.Run("it touches no save.requested on a timed-out registration", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		_, hooksFile := hooksFileInTempDir(t)
		stateDir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", stateDir)
		t.Setenv("TMUX_PANE", "%3")
		hookstest.HoldHooksSidecar(t, hooksFile)

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		if _, err := runHookSet(t, "claude --resume abc"); err == nil {
			t.Fatal("expected a non-zero exit under a held lock, got nil")
		}
		if saveRequestedExists(t, stateDir) {
			t.Error("a timed-out hook set touched save.requested")
		}
	})

	t.Run("it keeps the pane's token after a timed-out registration", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%7")
		release := hookstest.HoldHooksSidecar(t, hooksFile)

		pane := &stampedPane{}
		minted := 0
		withHooksDeps(t, HooksDeps{
			KeyResolver: pane,
			PaneStamper: pane,
			TokenMinter: func() (string, error) { minted++; return "tok000", nil },
		})

		if _, err := runHookSet(t, "claude --resume abc"); err == nil {
			t.Fatal("expected a non-zero exit under a held lock, got nil")
		}

		if len(pane.stamps) != 1 {
			t.Fatalf("set-option call count = %d, want exactly 1 — the stamp stands, and nothing rolls it back: %+v",
				len(pane.stamps), pane.stamps)
		}
		stamp := pane.stamps[0]
		if stamp.target != "%7" || stamp.name != state.PortalPaneIDOption || stamp.value != "tok000" {
			t.Errorf("stamp = %+v, want %s=tok000 on %%7", stamp, state.PortalPaneIDOption)
		}
		if pane.token != "tok000" {
			t.Errorf("pane token = %q after the timeout, want it left in place", pane.token)
		}

		// The retry reads the token back and reuses it: no second mint, no second
		// stamp, and one entry under the token the failed attempt stamped.
		release()
		if _, err := runHookSet(t, "claude --resume abc"); err != nil {
			t.Fatalf("retry after the lock freed: %v", err)
		}

		if minted != 1 {
			t.Errorf("mint count = %d, want 1 — the retry reuses the stamped token", minted)
		}
		if len(pane.stamps) != 1 {
			t.Errorf("set-option call count = %d, want 1 — a pane already carrying a token is not re-stamped: %+v",
				len(pane.stamps), pane.stamps)
		}
		data := readHooksJSON(t, hooksFile)
		if len(data) != 1 {
			t.Fatalf("hooks.json = %v, want exactly one entry", data)
		}
		if data["tok000"]["on-resume"] != "claude --resume abc" {
			t.Errorf("hooks.json = %v, want the entry under tok000", data)
		}
	})

	t.Run("it returns at the bound rather than hanging", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		hookstest.HoldHooksSidecar(t, hooksFile)

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		assertReturnsAtLockBound(t, "set", func() error {
			_, err := runHookSet(t, "claude --resume abc")
			return err
		})
	})
}

func TestHookRmLockTimeout(t *testing.T) {
	t.Run("it exits non-zero from hook rm on a lock timeout", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		before := seedHooksFile(t, hooksFile, map[string]map[string]string{
			"tok123": {"on-resume": "claude --resume abc"},
		})
		hookstest.HoldHooksSidecar(t, hooksFile)

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		sink := logtest.Install(t)
		out, err := runHookRm(t)
		assertLockFailureReachesStderr(t, out, err)
		assertOneLockWarn(t, sink, "rm", "tok123")
		assertHooksFileUnchanged(t, hooksFile, before)
	})

	t.Run("it exits non-zero on the --pane-key path and still issues no tmux call", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "")
		before := seedHooksFile(t, hooksFile, map[string]map[string]string{
			"tok123": {"on-resume": "claude --resume abc"},
		})
		hookstest.HoldHooksSidecar(t, hooksFile)

		resolver, stamper := paneKeyPathSeams()
		withHooksDeps(t, HooksDeps{KeyResolver: resolver, PaneStamper: stamper})

		out, err := runHookRm(t, "--pane-key", "tok123")
		assertLockFailureReachesStderr(t, out, err)
		assertNoPaneTmuxCalls(t, resolver, stamper)
		assertHooksFileUnchanged(t, hooksFile, before)
	})

	t.Run("it returns at the bound rather than hanging", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"tok123": {"on-resume": "claude --resume abc"},
		})
		hookstest.HoldHooksSidecar(t, hooksFile)

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		assertReturnsAtLockBound(t, "rm", func() error {
			_, err := runHookRm(t)
			return err
		})
	})
}
