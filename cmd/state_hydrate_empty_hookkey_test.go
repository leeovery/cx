package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

// parseHydrateHookKey runs the real `state hydrate` argv through cobra and
// hands back the hook key the command body would have run with.
func parseHydrateHookKey(t *testing.T, args []string) string {
	t.Helper()
	t.Setenv("PORTAL_STATE_DIR", t.TempDir())

	var got string
	prev := hydrateRunFunc
	hydrateRunFunc = func(cfg hydrateConfig) error {
		got = cfg.HookKey
		return nil
	}
	t.Cleanup(func() { hydrateRunFunc = prev })

	resetRootCmd()
	resetStateCmdFlags()
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("state hydrate %v: %v", args, err)
	}
	return got
}

func writeFileForHydrate(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestHydrate_AbsentAndEmptyHookKeyAreEquivalent(t *testing.T) {
	t.Run("it treats an absent hook-key and an empty one identically", func(t *testing.T) {
		absent := parseHydrateHookKey(t, []string{"state", "hydrate", "--fifo", "/tmp/f", "--file", "/tmp/s"})
		empty := parseHydrateHookKey(t, []string{"state", "hydrate", "--fifo", "/tmp/f", "--file", "/tmp/s", "--hook-key", ""})
		if absent != "" || empty != "" {
			t.Fatalf("hook keys = %q (absent) / %q (empty), want both empty", absent, empty)
		}

		t.Setenv("SHELL", "/bin/zsh")
		// A "" entry is the hand-edited hooks.json this guard exists for: no
		// hydrate path may fire it.
		dir := t.TempDir()
		store := seedHookStore(t, dir, map[string]map[string]string{
			"": {"on-resume": "rm -rf /"},
		})

		run := func(hookKey, fifoName string) (string, []string) {
			t.Helper()
			fifo := makeFIFO(t, dir, fifoName)
			scrollback := filepath.Join(dir, "sb")
			writeFileForHydrate(t, scrollback, "")
			signalFIFOAsync(t, fifo)

			exec := &stubExecShell{}
			cfg := hydrateConfig{
				FIFO: fifo, File: scrollback, HookKey: hookKey,
				Stdout:    io.Discard,
				Client:    tmux.NewClient(&recordingCommander{}),
				HookStore: store,
				ExecShell: exec.fn(),
				OpenFIFO:  openFIFOWithTimeout,
			}
			if err := runHydrate(cfg); err != nil {
				t.Fatalf("runHydrate: %v", err)
			}
			return exec.target, exec.args
		}

		absentTarget, absentArgs := run(absent, "hydrate-abs__0.0.fifo")
		emptyTarget, emptyArgs := run(empty, "hydrate-emp__0.0.fifo")

		if absentTarget != "/bin/zsh" {
			t.Errorf("absent-flag exec target = %q, want /bin/zsh (a \"\" entry must fire on nothing)", absentTarget)
		}
		if absentTarget != emptyTarget || !reflect.DeepEqual(absentArgs, emptyArgs) {
			t.Errorf("absent-flag exec %q %#v differs from empty-flag exec %q %#v",
				absentTarget, absentArgs, emptyTarget, emptyArgs)
		}
	})
}

func TestHydrate_UnstampedPaneHydratesToBareShell(t *testing.T) {
	t.Run("it hydrates an unstamped restored pane to a bare shell", func(t *testing.T) {
		dir := t.TempDir()
		fifo := makeFIFO(t, dir, "hydrate-unstamped__0.0.fifo")
		scrollback := filepath.Join(dir, "sb")
		const replayed = "before reboot\n"
		writeFileForHydrate(t, scrollback, replayed)

		signalFIFOAsync(t, fifo)

		t.Setenv("SHELL", "/bin/zsh")
		store := seedHookStore(t, dir, map[string]map[string]string{
			"": {"on-resume": "rm -rf /"},
		})

		logger, sink := newCaptureLoggerForComponent(t, "hydrate")
		stdout := new(bytes.Buffer)
		exec := &stubExecShell{}
		cfg := hydrateConfig{
			FIFO: fifo, File: scrollback, HookKey: "",
			Stdout:    stdout,
			Client:    tmux.NewClient(&recordingCommander{}),
			Logger:    logger,
			HookStore: store,
			ExecShell: exec.fn(),
			OpenFIFO:  openFIFOWithTimeout,
		}
		if err := runHydrate(cfg); err != nil {
			t.Fatalf("runHydrate: %v", err)
		}

		if !strings.Contains(stdout.String(), replayed) {
			t.Errorf("scrollback %q not replayed to stdout: %q", replayed, stdout.String())
		}
		if exec.target != "/bin/zsh" {
			t.Errorf("exec target = %q, want /bin/zsh", exec.target)
		}
		if !reflect.DeepEqual(exec.args, []string{"/bin/zsh"}) {
			t.Errorf("exec args = %#v, want [/bin/zsh]", exec.args)
		}

		line := execLogLine(t, sink.Body(), "INFO", "exec")
		if !strings.Contains(line, "hook_present=false") {
			t.Errorf("exec INFO %q missing hook_present=false", line)
		}
	})
}

func TestHydrateTimeoutLog_EmptyHookKeyRendersEmpty(t *testing.T) {
	t.Run("it renders an empty hook_key in the timeout warn", func(t *testing.T) {
		dir := t.TempDir()
		fifo := makeFIFO(t, dir, "hydrate-tw__0.0.fifo")

		logger, sink := newCaptureLoggerForComponent(t, "hydrate")
		cfg := timeoutCfg(t, fifo, filepath.Join(dir, "sb"), "", io.Discard,
			&recordingCommander{}, (&stubExecShell{}).fn(), logger)

		if err := runHydrate(cfg); err != nil {
			t.Fatalf("runHydrate: %v", err)
		}

		rec := sink.OnlyRecordWith(t, "hydrate", "timeout waiting for hydrate signal")
		hookKey, ok := rec.Attrs["hook_key"]
		if !ok {
			t.Fatalf("timeout WARN missing hook_key attr: %+v", rec.Attrs)
		}
		if hookKey.String() != "" {
			t.Errorf("hook_key = %q, want empty", hookKey.String())
		}
	})
}
