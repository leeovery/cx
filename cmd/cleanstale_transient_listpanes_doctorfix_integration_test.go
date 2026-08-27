//go:build integration

package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
	"github.com/leeovery/portal/internal/transienttest"
)

func runDoctorFixHookPrune(t *testing.T, lister AllPaneLister) (string, error) {
	t.Helper()
	hookStore, err := loadHookStore()
	if err != nil {
		return "", err
	}
	w := new(bytes.Buffer)
	pruneDoctorStaleHooks(w, &DoctorDeps{HookLister: lister, HookStore: hookStore})
	return w.String(), nil
}

func assertNoStaleHookPrunesOnStdout(t *testing.T, output string, seededKeys ...string) {
	t.Helper()
	for _, key := range seededKeys {
		needle := fmt.Sprintf("Pruned stale hook: %s", key)
		if strings.Contains(output, needle) {
			t.Fatalf("stdout reported %q under transient — the wipe regression has surfaced to the user\n"+
				"  full output:\n%s", needle, output)
		}
	}
	if strings.Contains(output, "Pruned stale hook:") {
		t.Fatalf("stdout contained an unexpected `Pruned stale hook:` line under transient\n"+
			"  full output:\n%s", output)
	}
}

func TestDoctorFix_TmuxTransient_DoesNotWipeHooks(t *testing.T) {
	// A live server behind the stub is load-bearing: runHookStaleCleanup reads
	// @portal-restoring before it enumerates, and that read is not intercepted —
	// against a dead socket it fails, and a failed read stands the whole cycle
	// down before either list-panes failure mode is reached.
	doctorInvoker := func(mode transienttest.FailureMode) func(t *testing.T, env []string, stateDir string) (string, error) {
		return func(t *testing.T, env []string, stateDir string) (string, error) {
			t.Helper()
			tmuxtest.SkipIfNoTmux(t)

			sock := tmuxtest.New(t, "ptl-doctorfix-transient-")
			if _, err := sock.TryRun("new-session", "-d", "-s", "live"); err != nil {
				t.Fatalf("seed live session: %v", err)
			}

			stub := &transienttest.Commander{
				Inner: &transienttest.SocketCommander{SocketPath: sock.SocketPath()},
				Mode:  mode,
			}
			return runDoctorFixHookPrune(t, tmux.NewClient(stub))
		}
	}
	doctorNoStdoutPrunesAssert := func(t *testing.T, output string, seededKeys []string) {
		t.Helper()
		assertNoStaleHookPrunesOnStdout(t, output, seededKeys...)
	}

	t.Run("mode_a_list_panes_exit_nonzero", func(t *testing.T) {
		runTransientCleanStaleModeSubtest(t, transientModeSpec{
			name:        "doctor-fix mode (a)",
			mode:        transienttest.FailExitNonZero,
			invoke:      doctorInvoker(transienttest.FailExitNonZero),
			extraAssert: doctorNoStdoutPrunesAssert,
		})
	})

	t.Run("mode_b_list_panes_empty_stdout", func(t *testing.T) {
		runTransientCleanStaleModeSubtest(t, transientModeSpec{
			name:        "doctor-fix mode (b)",
			mode:        transienttest.FailEmptyStdout,
			invoke:      doctorInvoker(transienttest.FailEmptyStdout),
			extraAssert: doctorNoStdoutPrunesAssert,
		})
	})

	t.Run("normal_path_legitimate_stale_removal_still_works", func(t *testing.T) {
		tmuxtest.SkipIfNoTmux(t)

		env, stateDir := isolateCleanStaleTestEnv(t)
		sock := tmuxtest.New(t, "ptl-doctorfix-pass-")

		// The default session/window/pane indices yield the key "live:0.0".
		if _, err := sock.TryRun("new-session", "-d", "-s", "live"); err != nil {
			t.Fatalf("seed live session: %v", err)
		}

		seedEntries := map[string]string{
			"live:0.0": "echo live",
			"gone01":   "echo gone",
		}
		transienttest.SeedHooksJSON(t, env, seedEntries)

		passThroughStub := &transienttest.Commander{
			Inner: &transienttest.SocketCommander{SocketPath: sock.SocketPath()},
			Mode:  transienttest.PassThrough,
		}

		output, err := runDoctorFixHookPrune(t, tmux.NewClient(passThroughStub))
		if err != nil {
			t.Fatalf("doctor --fix hook prune returned error on the normal path; want nil: %v\n  output:\n%s", err, output)
		}

		afterStr := string(transienttest.HooksJSONBytes(t, env))
		if !strings.Contains(afterStr, `"live:0.0"`) {
			t.Fatalf("normal path destroyed the live entry `live:0.0`; want it preserved\n"+
				"  hooks.json after: %s", afterStr)
		}
		if strings.Contains(afterStr, `"gone01"`) {
			t.Fatalf("normal path failed to remove the stale entry `gone01`; want it removed\n"+
				"  hooks.json after: %s", afterStr)
		}

		wantLine := "Pruned stale hook: gone01"
		if !strings.Contains(output, wantLine) {
			t.Fatalf("normal-path stdout missing %q\n  full output:\n%s", wantLine, output)
		}
		unwantLine := "Pruned stale hook: live:0.0"
		if strings.Contains(output, unwantLine) {
			t.Fatalf("normal-path stdout unexpectedly reported %q (live entry must survive)\n  full output:\n%s", unwantLine, output)
		}

		lines := staleHookCleanupLogLines(portaltest.ReadPortalLogSafe(stateDir))
		if len(lines) == 0 {
			t.Fatalf("no `stale-hook cleanup:` lines found in portal.log on the normal path; want entry-point + completion Debug\n"+
				"  full log:\n%s", portaltest.ReadPortalLogSafe(stateDir))
		}
		if !containsLineMatching(lines, "stale-hook cleanup counts", "entries=2") {
			t.Fatalf("missing normal-path entry-point Debug; want a `stale-hook cleanup counts` line containing `entries=2`\n"+
				"  matched stale-hook lines:\n%s", strings.Join(lines, "\n"))
		}
		if !containsLineMatching(lines, "stale-hook cleanup removed", "reaped=1") {
			t.Fatalf("missing normal-path completion Debug; want a `stale-hook cleanup removed` line containing `reaped=1`\n"+
				"  matched stale-hook lines:\n%s", strings.Join(lines, "\n"))
		}
	})
}
