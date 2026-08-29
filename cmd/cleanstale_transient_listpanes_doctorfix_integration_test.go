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

func runDoctorFixHookPrune(t *testing.T, lister staleSweepReader) (string, error) {
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

		if _, err := sock.TryRun("new-session", "-d", "-s", "live"); err != nil {
			t.Fatalf("seed live session: %v", err)
		}
		// The entry must survive because its pane is live, so the pane carries a
		// token-shaped key the reaper can judge.
		liveKey := transienttest.ReapableHookKey(1)
		sock.StampPaneToken(t, "live:0.0", liveKey)

		staleKey := transienttest.ReapableHookKey(0)
		seedEntries := map[string]string{
			liveKey:  "echo live",
			staleKey: "echo gone",
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
		if !strings.Contains(afterStr, `"`+liveKey+`"`) {
			t.Fatalf("normal path destroyed the live entry %q; want it preserved\n"+
				"  hooks.json after: %s", liveKey, afterStr)
		}
		if strings.Contains(afterStr, `"`+staleKey+`"`) {
			t.Fatalf("normal path failed to remove the stale entry %q; want it removed\n"+
				"  hooks.json after: %s", staleKey, afterStr)
		}

		wantLine := "Pruned stale hook: " + staleKey
		if !strings.Contains(output, wantLine) {
			t.Fatalf("normal-path stdout missing %q\n  full output:\n%s", wantLine, output)
		}
		unwantLine := "Pruned stale hook: " + liveKey
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
