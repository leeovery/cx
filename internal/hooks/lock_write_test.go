package hooks_test

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
)

// assertLockWarn pins the single record a mutation that could not take the lock
// leaves: the operation's own op, its key, its caller, and the lock error.
func assertLockWarn(t *testing.T, sink *logtest.Sink, wantOp, wantKey, wantVia string) logtest.Record {
	t.Helper()

	rec := sink.OnlyRecord(t)
	if rec.Level != slog.LevelWarn {
		t.Errorf("level = %v, want WARN", rec.Level)
	}
	if rec.Msg != wantOp {
		t.Errorf("msg = %q, want %q", rec.Msg, wantOp)
	}
	if got := rec.AttrString(t, "op"); got != wantOp {
		t.Errorf("op = %q, want %q", got, wantOp)
	}
	if got := rec.AttrString(t, "component"); got != "hooks" {
		t.Errorf("component = %q, want %q", got, "hooks")
	}
	if got := rec.AttrString(t, "hook_key"); got != wantKey {
		t.Errorf("hook_key = %q, want %q", got, wantKey)
	}
	if got := rec.AttrString(t, "via"); got != wantVia {
		t.Errorf("via = %q, want %q", got, wantVia)
	}
	loggedErr := rec.ErrorAttr(t, "error")
	if loggedErr == nil || loggedErr.Error() == "" {
		t.Error("error attr is empty — the lock failure must be carried")
	}
	return rec
}

func TestMutationLockTimeoutWritesNothing(t *testing.T) {
	t.Run("it writes nothing when Set cannot take the lock", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)

		t.Run("an absent hooks.json stays absent", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hooks.json")
			hookstest.HoldHooksSidecar(t, path)

			err := hooks.NewStore(path).Set("tok123", "on-resume", "npm start", hooks.ViaCLI)
			if err == nil {
				t.Fatal("expected an error when the lock will not yield, got nil")
			}
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("hooks.json created despite the timeout: %v", statErr)
			}
		})

		t.Run("an existing hooks.json is byte-identical", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hooks.json")
			if err := os.WriteFile(path, []byte(`{"tok999":{"on-resume":"cmd-a"}}`), 0o600); err != nil {
				t.Fatalf("seed: %v", err)
			}
			before := readFileBytes(t, path)
			hookstest.HoldHooksSidecar(t, path)

			if err := hooks.NewStore(path).Set("tok123", "on-resume", "npm start", hooks.ViaCLI); err == nil {
				t.Fatal("expected an error when the lock will not yield, got nil")
			}

			if after := readFileBytes(t, path); string(after) != string(before) {
				t.Errorf("hooks.json changed on a timed-out Set:\nbefore %s\nafter  %s", before, after)
			}
		})
	})

	t.Run("it writes nothing when Remove cannot take the lock", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)

		path := filepath.Join(t.TempDir(), "hooks.json")
		if err := os.WriteFile(path, []byte(`{"tok123":{"on-resume":"cmd-a"}}`), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		before := readFileBytes(t, path)
		hookstest.HoldHooksSidecar(t, path)

		removed, err := hooks.NewStore(path).Remove("tok123", "on-resume", hooks.ViaCLI)
		if err == nil {
			t.Fatal("expected an error when the lock will not yield, got nil")
		}
		if removed {
			t.Error("removed = true, want false when the lock could not be taken")
		}
		if after := readFileBytes(t, path); string(after) != string(before) {
			t.Errorf("hooks.json changed on a timed-out Remove:\nbefore %s\nafter  %s", before, after)
		}
	})

	t.Run("it matches the sentinel through the wrap", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)

		path := filepath.Join(t.TempDir(), "hooks.json")
		hookstest.HoldHooksSidecar(t, path)
		store := hooks.NewStore(path)

		setErr := store.Set("tok123", "on-resume", "npm start", hooks.ViaCLI)
		if !errors.Is(setErr, hooks.ErrLockHeld) {
			t.Errorf("Set error = %v, want errors.Is ErrLockHeld", setErr)
		}

		_, rmErr := store.Remove("tok123", "on-resume", hooks.ViaCLI)
		if !errors.Is(rmErr, hooks.ErrLockHeld) {
			t.Errorf("Remove error = %v, want errors.Is ErrLockHeld", rmErr)
		}
	})
}

func TestMutationLockTimeoutLogging(t *testing.T) {
	t.Run("it emits one WARN under op=set for a timed-out registration", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)
		path := filepath.Join(t.TempDir(), "hooks.json")
		hookstest.HoldHooksSidecar(t, path)

		sink := logtest.Install(t)
		if err := hooks.NewStore(path).Set("tok123", "on-resume", "npm start", hooks.ViaCLI); err == nil {
			t.Fatal("expected an error when the lock will not yield, got nil")
		}

		assertLockWarn(t, sink, "set", "tok123", "cli")
	})

	t.Run("it files under op=set even where a completed call would have classified as modify", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)
		path := filepath.Join(t.TempDir(), "hooks.json")
		// The same key and event already carry a different command, so a call that
		// got as far as loading and classifying would file this under op=modify.
		// The op is decided before the lock is even attempted, so it stays set.
		if err := os.WriteFile(path, []byte(`{"tok123":{"on-resume":"cmd-a"}}`), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		hookstest.HoldHooksSidecar(t, path)

		sink := logtest.Install(t)
		if err := hooks.NewStore(path).Set("tok123", "on-resume", "npm start", hooks.ViaCLI); err == nil {
			t.Fatal("expected an error when the lock will not yield, got nil")
		}

		assertLockWarn(t, sink, "set", "tok123", "cli")
	})

	t.Run("it emits one WARN under op=rm for a timed-out removal", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)
		path := filepath.Join(t.TempDir(), "hooks.json")
		if err := os.WriteFile(path, []byte(`{"tok123":{"on-resume":"cmd-a"}}`), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		hookstest.HoldHooksSidecar(t, path)

		sink := logtest.Install(t)
		removed, err := hooks.NewStore(path).Remove("tok123", "on-resume", hooks.ViaInternal)
		if err == nil {
			t.Fatal("expected an error when the lock will not yield, got nil")
		}
		if removed {
			t.Error("removed = true, want false when the lock could not be taken")
		}

		assertLockWarn(t, sink, "rm", "tok123", "internal")
	})

	t.Run("it carries no error_class and no value on the lock WARN", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)

		t.Run("set", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hooks.json")
			hookstest.HoldHooksSidecar(t, path)

			sink := logtest.Install(t)
			if err := hooks.NewStore(path).Set("tok123", "on-resume", "npm start", hooks.ViaCLI); err == nil {
				t.Fatal("expected an error, got nil")
			}

			rec := assertLockWarn(t, sink, "set", "tok123", "cli")
			if _, ok := rec.Attrs["error_class"]; ok {
				t.Errorf("lock WARN carries error_class — no write phase ran: %+v", rec.Attrs)
			}
			if _, ok := rec.Attrs["value"]; ok {
				t.Errorf("lock WARN carries value — the file was never opened: %+v", rec.Attrs)
			}
		})

		t.Run("rm", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hooks.json")
			hookstest.HoldHooksSidecar(t, path)

			sink := logtest.Install(t)
			if _, err := hooks.NewStore(path).Remove("tok123", "on-resume", hooks.ViaCLI); err == nil {
				t.Fatal("expected an error, got nil")
			}

			rec := assertLockWarn(t, sink, "rm", "tok123", "cli")
			if _, ok := rec.Attrs["error_class"]; ok {
				t.Errorf("lock WARN carries error_class — no write phase ran: %+v", rec.Attrs)
			}
			if _, ok := rec.Attrs["value"]; ok {
				t.Errorf("lock WARN carries value — the file was never opened: %+v", rec.Attrs)
			}
		})
	})

	t.Run("it still emits nothing when Remove removes nothing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hooks.json")
		store := hooks.NewStore(path)
		if err := store.Set("tok999", "on-resume", "npm start", hooks.ViaCLI); err != nil {
			t.Fatalf("seed: %v", err)
		}
		before := readFileBytes(t, path)

		sink := logtest.Install(t)
		removed, err := store.Remove("tok123", "on-resume", hooks.ViaCLI)
		if err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if removed {
			t.Error("removed = true, want false for an absent key")
		}
		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("a removal that removed nothing emitted %d records, want 0: %+v", len(recs), recs)
		}
		if after := readFileBytes(t, path); string(after) != string(before) {
			t.Errorf("hooks.json changed on a no-removal:\nbefore %s\nafter  %s", before, after)
		}
	})

	t.Run("it emits no store-side WARN when CleanStale cannot take the lock", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)
		path := filepath.Join(t.TempDir(), "hooks.json")
		if err := os.WriteFile(path, []byte(`{"tok123":{"on-resume":"cmd-a"}}`), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		hookstest.HoldHooksSidecar(t, path)

		sink := logtest.Install(t)
		removed, err := hooks.NewStore(path).CleanStale(enumerating())
		if !errors.Is(err, hooks.ErrLockHeld) {
			t.Fatalf("CleanStale error = %v, want errors.Is ErrLockHeld", err)
		}
		if len(removed) != 0 {
			t.Errorf("removed = %v, want none", removed)
		}
		// The clean's own snapshot read degrades against the same held sidecar
		// and says so; nothing else may be said.
		for _, r := range sink.Records() {
			if r.Msg != "load-unlocked" {
				t.Errorf("CleanStale emitted %q, want only the read degradation — the stood-down line belongs to its call site: %+v", r.Msg, r)
			}
		}
	})

	t.Run("it fails the write when the sidecar cannot be created", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "ro")
		if err := os.Mkdir(parent, 0o500); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
		path := filepath.Join(parent, "portal", "hooks.json")

		sink := logtest.Install(t)
		if err := hooks.NewStore(path).Set("tok123", "on-resume", "npm start", hooks.ViaCLI); err == nil {
			t.Fatal("Set succeeded under a directory that permits no file creation")
		}

		assertLockWarn(t, sink, "set", "tok123", "cli")
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("hooks.json written: %v", statErr)
		}
	})
}
