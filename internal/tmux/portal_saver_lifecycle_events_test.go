package tmux_test

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/commandertest"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

func TestSaverPaneID_ReturnsTrimmedFirstLine(t *testing.T) {
	var observed []string
	mock := commandertest.FromFunc(func(args ...string) (string, error) {
		if args[0] == "list-panes" {
			observed = append([]string{}, args...)
			return "%42\n", nil
		}
		t.Fatalf("unexpected command: %v", args)
		return "", nil
	})
	client := tmux.NewClient(mock)

	got, err := client.SaverPaneID("_portal-saver")
	if err != nil {
		t.Fatalf("SaverPaneID returned error: %v", err)
	}
	if got != "%42" {
		t.Errorf("SaverPaneID = %q, want %q", got, "%42")
	}

	want := "list-panes -t =_portal-saver: -F #{pane_id}"
	if joined := strings.Join(observed, " "); joined != want {
		t.Errorf("list-panes argv = %q, want %q", joined, want)
	}
}

func TestSaverPaneID_PropagatesError(t *testing.T) {
	mock := commandertest.FromFunc(func(args ...string) (string, error) {
		return "", errors.New("can't find session: _portal-saver")
	})
	client := tmux.NewClient(mock)

	_, err := client.SaverPaneID("_portal-saver")
	if err == nil {
		t.Fatal("expected error from SaverPaneID, got nil")
	}
}

func TestBootstrapPortalSaver_EmitsPlaceholderCreatedWithTmuxPane(t *testing.T) {
	stubAliveCheck(t, false)
	shrinkRetryDelay(t)
	stubReadinessReady(t)
	sink := logtest.Install(t)

	script := &portalSaverScript{
		hasSession: func(int) (string, error) {
			return "", errors.New("can't find session: _portal-saver")
		},
		newSession:  func(int) (string, error) { return "", nil },
		setOption:   func(int) (string, error) { return "", nil },
		respawnPane: func(int) (string, error) { return "", nil },
		listPanes: func(format string, call int) (string, error) {
			if format == "#{pane_id}" {
				return "%7\n", nil
			}
			return "1234\n", nil
		},
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.BootstrapPortalSaver(client, "/tmp/portal-state"); err != nil {
		t.Fatalf("BootstrapPortalSaver returned error: %v", err)
	}

	rec := sink.RecordsWith("saver", "placeholder created").Only(t, "saver placeholder created record")
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO", rec.Level)
	}
	if got := rec.AttrString(t, "tmux_pane"); got != "%7" {
		t.Errorf("tmux_pane = %q, want %q", got, "%7")
	}
}

func TestBootstrapPortalSaver_EmitsDestroyUnattachedOffOnCreateBranch(t *testing.T) {
	stubAliveCheck(t, false)
	shrinkRetryDelay(t)
	stubReadinessReady(t)
	sink := logtest.Install(t)

	script := &portalSaverScript{
		hasSession: func(int) (string, error) {
			return "", errors.New("can't find session: _portal-saver")
		},
		newSession:  func(int) (string, error) { return "", nil },
		setOption:   func(int) (string, error) { return "", nil },
		respawnPane: func(int) (string, error) { return "", nil },
		listPanes: func(format string, call int) (string, error) {
			if format == "#{pane_id}" {
				return "%7\n", nil
			}
			return "1234\n", nil
		},
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.BootstrapPortalSaver(client, "/tmp/portal-state"); err != nil {
		t.Fatalf("BootstrapPortalSaver returned error: %v", err)
	}

	rec := sink.RecordsWith("saver", "destroy-unattached off").Only(t, "saver destroy-unattached off record")
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO", rec.Level)
	}
	if got := rec.AttrString(t, "tmux_pane"); got != "%7" {
		t.Errorf("tmux_pane = %q, want %q", got, "%7")
	}
}

func TestBootstrapPortalSaver_EmitsDestroyUnattachedOffOnAliveHappyPath_AndNotRespawnOrReady(t *testing.T) {
	stubAliveCheck(t, true)
	shrinkRetryDelay(t)
	sink := logtest.Install(t)

	script := &portalSaverScript{
		hasSession: func(int) (string, error) { return "", nil },
		setOption:  func(int) (string, error) { return "", nil },
		listPanes: func(format string, call int) (string, error) {
			if format == "#{pane_id}" {
				return "%9\n", nil
			}
			return "5678\n", nil
		},
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.BootstrapPortalSaver(client, "/tmp/portal-state"); err != nil {
		t.Fatalf("BootstrapPortalSaver returned error: %v", err)
	}

	rec := sink.RecordsWith("saver", "destroy-unattached off").Only(t, "saver destroy-unattached off record")
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO", rec.Level)
	}
	if got := rec.AttrString(t, "tmux_pane"); got != "%9" {
		t.Errorf("tmux_pane = %q, want %q", got, "%9")
	}

	if evs := sink.RecordsWith("saver", "respawn-daemon"); len(evs) != 0 {
		t.Errorf("expected 0 respawn-daemon events on alive happy path, got %d: %+v", len(evs), evs)
	}
	if evs := sink.RecordsWith("saver", "daemon ready"); len(evs) != 0 {
		t.Errorf("expected 0 daemon ready events on alive happy path, got %d: %+v", len(evs), evs)
	}
	if evs := sink.RecordsWith("saver", "placeholder created"); len(evs) != 0 {
		t.Errorf("expected 0 placeholder created events on alive happy path, got %d: %+v", len(evs), evs)
	}
}

func TestBootstrapPortalSaver_EmitsRespawnDaemonWithFromToPidAndTmuxPane(t *testing.T) {
	stubAliveCheck(t, false)
	shrinkRetryDelay(t)
	stubReadinessReady(t)
	sink := logtest.Install(t)

	panePIDCall := 0
	script := &portalSaverScript{
		hasSession: func(int) (string, error) {
			return "", errors.New("can't find session: _portal-saver")
		},
		newSession:  func(int) (string, error) { return "", nil },
		setOption:   func(int) (string, error) { return "", nil },
		respawnPane: func(int) (string, error) { return "", nil },
		listPanes: func(format string, call int) (string, error) {
			if format == "#{pane_id}" {
				return "%7\n", nil
			}
			panePIDCall++
			if panePIDCall == 1 {
				return "1111\n", nil
			}
			return "2222\n", nil
		},
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.BootstrapPortalSaver(client, "/tmp/portal-state"); err != nil {
		t.Fatalf("BootstrapPortalSaver returned error: %v", err)
	}

	rec := sink.RecordsWith("saver", "respawn-daemon").Only(t, "saver respawn-daemon record")
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO", rec.Level)
	}
	if got := rec.IntAttr(t, "from_pid"); got != 1111 {
		t.Errorf("from_pid = %d, want 1111", got)
	}
	if got := rec.IntAttr(t, "to_pid"); got != 2222 {
		t.Errorf("to_pid = %d, want 2222", got)
	}
	if got := rec.AttrString(t, "tmux_pane"); got != "%7" {
		t.Errorf("tmux_pane = %q, want %q", got, "%7")
	}
}

func TestBootstrapPortalSaver_StillEmitsRespawnDaemonBestEffortWhenPanePIDReadFails(t *testing.T) {
	stubAliveCheck(t, false)
	shrinkRetryDelay(t)
	stubReadinessReady(t)
	sink := logtest.Install(t)

	panePIDCall := 0
	script := &portalSaverScript{
		hasSession: func(int) (string, error) {
			return "", errors.New("can't find session: _portal-saver")
		},
		newSession:  func(int) (string, error) { return "", nil },
		setOption:   func(int) (string, error) { return "", nil },
		respawnPane: func(int) (string, error) { return "", nil },
		listPanes: func(format string, call int) (string, error) {
			if format == "#{pane_id}" {
				return "%7\n", nil
			}
			panePIDCall++
			if panePIDCall == 1 {
				return "", errors.New("transient list-panes failure")
			}
			return "2222\n", nil
		},
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.BootstrapPortalSaver(client, "/tmp/portal-state"); err != nil {
		t.Fatalf("BootstrapPortalSaver must not abort on a pid-read miss, got: %v", err)
	}

	rec := sink.RecordsWith("saver", "respawn-daemon").Only(t, "saver respawn-daemon record")
	if got := rec.IntAttr(t, "from_pid"); got != 0 {
		t.Errorf("from_pid = %d, want 0 (read failed)", got)
	}
	if got := rec.IntAttr(t, "to_pid"); got != 2222 {
		t.Errorf("to_pid = %d, want 2222", got)
	}

	failures := sink.RecordsWith("saver", "saver respawn: pane-pid read failed")
	if len(failures) == 0 {
		t.Fatalf("expected a pane-pid read-failure log line, got none: %+v", sink.Records())
	}
	if !failures[0].HasAttr("error") {
		t.Errorf("read-failure line missing error attr: %+v", failures[0].Attrs)
	}
}

func TestWaitForSaverDaemonReady_EmitsDaemonReadyWithTargetPidOnSuccess(t *testing.T) {
	installReadinessPollInterval(t, 1*time.Millisecond)
	installReadinessTimeout(t, 500*time.Millisecond)
	sink := logtest.Install(t)

	installReadinessReadPID(t, func(string) (int, error) { return 4321, nil })
	installReadinessIdentify(t, func(int) (state.IdentifyResult, error) {
		return state.IdentifyIsPortalDaemon, nil
	})

	if err := tmux.WaitForSaverDaemonReady(t.TempDir()); err != nil {
		t.Fatalf("WaitForSaverDaemonReady returned error: %v", err)
	}

	rec := sink.RecordsWith("saver", "daemon ready").Only(t, "saver daemon ready record")
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO", rec.Level)
	}
	if got := rec.IntAttr(t, "target_pid"); got != 4321 {
		t.Errorf("target_pid = %d, want 4321", got)
	}
}

func TestWaitForSaverDaemonReady_EmitsNoDaemonReadyAndKeepsWarnOnTimeout(t *testing.T) {
	installReadinessPollInterval(t, 1*time.Millisecond)
	installReadinessTimeout(t, 20*time.Millisecond)
	sink := logtest.Install(t)

	installReadinessReadPID(t, func(string) (int, error) { return 4321, nil })
	installReadinessIdentify(t, func(int) (state.IdentifyResult, error) {
		return state.IdentifyDead, nil
	})
	barrier := &barrierLog{}
	installBarrierLogger(t, barrier)

	if err := tmux.WaitForSaverDaemonReady(t.TempDir()); err != nil {
		t.Fatalf("WaitForSaverDaemonReady returned error: %v", err)
	}

	if evs := sink.RecordsWith("saver", "daemon ready"); len(evs) != 0 {
		t.Errorf("expected 0 daemon ready events on timeout, got %d: %+v", len(evs), evs)
	}
	if len(barrier.warns()) != 1 {
		t.Errorf("expected exactly 1 WARN on timeout, got %d: %v", len(barrier.warns()), barrier.warns())
	}
}

// firstSaverIndex is the capture-order position of the first saver record
// carrying msg, so a test can assert one event was emitted before another. -1
// when none was.
func firstSaverIndex(sink *logtest.Sink, msg string) int {
	for i, r := range sink.Records() {
		if r.Matches("saver", msg) {
			return i
		}
	}
	return -1
}

func TestKillSaverAndWaitForDaemon_EmitsKillBarrierStartedWhenPriorDaemonAlive(t *testing.T) {
	installBarrierPollInterval(t, 1*time.Millisecond)
	installBarrierTimeout(t, 500*time.Millisecond)
	installBarrierReadPID(t, func(string) (int, error) { return 4321, nil })

	calls := 0
	installBarrierIsAlive(t, func(int) bool {
		calls++
		return calls < 3
	})
	barrier := &barrierLog{}
	installBarrierLogger(t, barrier)
	sink := logtest.Install(t)

	startedAtKillTime := -1
	script := &portalSaverScript{
		killSession: func(int) (string, error) {
			startedAtKillTime = len(sink.RecordsWith("saver", "kill-barrier started"))
			return "", nil
		},
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.KillSaverAndWaitForDaemon(client, t.TempDir()); err != nil {
		t.Fatalf("KillSaverAndWaitForDaemon returned error: %v", err)
	}

	rec := sink.RecordsWith("saver", "kill-barrier started").Only(t, "saver kill-barrier started record")
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO", rec.Level)
	}
	if got := rec.IntAttr(t, "target_pid"); got != 4321 {
		t.Errorf("target_pid = %d, want 4321", got)
	}
	if startedAtKillTime != 1 {
		t.Errorf("kill-barrier started must be emitted BEFORE kill-session (count at kill time = %d, want 1)", startedAtKillTime)
	}
	if len(barrier.warns()) != 0 {
		t.Errorf("expected 0 WARN lines on clean exit, got %d: %v", len(barrier.warns()), barrier.warns())
	}
}

func TestKillSaverAndWaitForDaemon_NoKillBarrierStartedOnNoPriorPIDShortcut(t *testing.T) {
	installBarrierReadPID(t, func(string) (int, error) {
		return 0, errors.New("daemon.pid absent")
	})
	barrier := &barrierLog{}
	installBarrierLogger(t, barrier)
	sink := logtest.Install(t)

	script := &portalSaverScript{
		killSession: func(int) (string, error) { return "", nil },
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.KillSaverAndWaitForDaemon(client, t.TempDir()); err != nil {
		t.Fatalf("KillSaverAndWaitForDaemon returned error: %v", err)
	}

	if evs := sink.RecordsWith("saver", "kill-barrier started"); len(evs) != 0 {
		t.Errorf("expected 0 kill-barrier started events on no-prior-PID shortcut, got %d: %+v", len(evs), evs)
	}
	if len(barrier.warns()) != 0 {
		t.Errorf("expected 0 WARN lines on tolerant-kill shortcut, got %d: %v", len(barrier.warns()), barrier.warns())
	}
}

func TestKillSaverAndWaitForDaemon_NoKillBarrierStartedWhenPriorDaemonAlreadyDead(t *testing.T) {
	installBarrierReadPID(t, func(string) (int, error) { return 4321, nil })
	installBarrierIsAlive(t, func(int) bool { return false })
	barrier := &barrierLog{}
	installBarrierLogger(t, barrier)
	sink := logtest.Install(t)

	script := &portalSaverScript{
		killSession: func(int) (string, error) { return "", nil },
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.KillSaverAndWaitForDaemon(client, t.TempDir()); err != nil {
		t.Fatalf("KillSaverAndWaitForDaemon returned error: %v", err)
	}

	if evs := sink.RecordsWith("saver", "kill-barrier started"); len(evs) != 0 {
		t.Errorf("expected 0 kill-barrier started events when prior daemon already dead, got %d: %+v", len(evs), evs)
	}
	if len(barrier.warns()) != 0 {
		t.Errorf("expected 0 WARN lines on already-dead shortcut, got %d: %v", len(barrier.warns()), barrier.warns())
	}
}

func TestKillSaverAndWaitForDaemon_EmitsKillBarrierEscalatedAboveDebugBreadcrumbOnPortalDaemonBranch(t *testing.T) {
	installBarrierPollInterval(t, 1*time.Millisecond)
	installBarrierTimeout(t, 5*time.Millisecond)
	installBarrierEscalationTimeout(t, 5*time.Millisecond)
	installBarrierReadPID(t, func(string) (int, error) { return 4321, nil })

	installBarrierIdentifyDaemon(t, func(int) (state.IdentifyResult, error) {
		return state.IdentifyIsPortalDaemon, nil
	})

	barrier := &barrierLog{}
	installBarrierLogger(t, barrier)
	sink := logtest.Install(t)

	killCalls := 0
	escalatedAtKillTime := -1
	installBarrierSendSIGKILL(t, func(int) error {
		escalatedAtKillTime = len(sink.RecordsWith("saver", "kill-barrier escalated"))
		killCalls++
		return nil
	})
	installBarrierIsAlive(t, func(int) bool { return killCalls == 0 })

	script := &portalSaverScript{
		killSession: func(int) (string, error) { return "", nil },
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.KillSaverAndWaitForDaemon(client, t.TempDir()); err != nil {
		t.Fatalf("KillSaverAndWaitForDaemon returned error: %v", err)
	}

	rec := sink.RecordsWith("saver", "kill-barrier escalated").Only(t, "saver kill-barrier escalated record")
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO", rec.Level)
	}
	if got := rec.IntAttr(t, "target_pid"); got != 4321 {
		t.Errorf("target_pid = %d, want 4321", got)
	}
	if got := rec.AttrString(t, "reason"); got != "kill-session-timeout" {
		t.Errorf("reason = %q, want %q", got, "kill-session-timeout")
	}

	if escalatedAtKillTime != 1 {
		t.Errorf("kill-barrier escalated must be emitted BEFORE SIGKILL (count at kill time = %d, want 1)", escalatedAtKillTime)
	}

	breadcrumbIdx := firstSaverIndex(sink, "kill-barrier escalating to SIGKILL")
	if breadcrumbIdx < 0 {
		t.Fatalf("expected the existing DEBUG breadcrumb %q to still be present: %+v", "kill-barrier escalating to SIGKILL", sink.Records())
	}
	if breadcrumbs := sink.RecordsWith("saver", "kill-barrier escalating to SIGKILL"); len(breadcrumbs) != 1 {
		t.Errorf("expected exactly 1 DEBUG breadcrumb, got %d: %+v", len(breadcrumbs), breadcrumbs)
	}
	escalatedIdx := firstSaverIndex(sink, "kill-barrier escalated")
	if escalatedIdx < 0 || escalatedIdx >= breadcrumbIdx {
		t.Errorf("escalated INFO (idx %d) must precede the DEBUG breadcrumb (idx %d)", escalatedIdx, breadcrumbIdx)
	}

	if len(barrier.warns()) != 0 {
		t.Errorf("expected 0 WARN lines on clean escalation, got %d: %v", len(barrier.warns()), barrier.warns())
	}
}

func TestKillSaverAndWaitForDaemon_NoKillBarrierEscalatedAndKeepsSingleWarnOnIdentitySkip(t *testing.T) {
	cases := []struct {
		name   string
		result state.IdentifyResult
		idErr  error
	}{
		{"IdentifyDead", state.IdentifyDead, nil},
		{"IdentifyNotPortalDaemon", state.IdentifyNotPortalDaemon, nil},
		{"TransientError", 0, errors.New("ps exec failed: transient")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installBarrierPollInterval(t, 1*time.Millisecond)
			installBarrierTimeout(t, 5*time.Millisecond)
			installBarrierEscalationTimeout(t, 5*time.Millisecond)
			installBarrierReadPID(t, func(string) (int, error) { return 4321, nil })
			installBarrierIsAlive(t, func(int) bool { return true })

			installBarrierIdentifyDaemon(t, func(int) (state.IdentifyResult, error) {
				return tc.result, tc.idErr
			})

			killCalls := 0
			installBarrierSendSIGKILL(t, func(int) error {
				killCalls++
				return nil
			})

			barrier := &barrierLog{}
			installBarrierLogger(t, barrier)
			sink := logtest.Install(t)

			script := &portalSaverScript{
				killSession: func(int) (string, error) { return "", nil },
			}
			mock := commandertest.FromFunc(script.run(t))
			client := tmux.NewClient(mock)

			if err := tmux.KillSaverAndWaitForDaemon(client, t.TempDir()); err != nil {
				t.Fatalf("KillSaverAndWaitForDaemon returned error: %v", err)
			}

			if killCalls != 0 {
				t.Errorf("expected 0 SIGKILL seam calls on identity-skip branch, got %d", killCalls)
			}
			if evs := sink.RecordsWith("saver", "kill-barrier escalated"); len(evs) != 0 {
				t.Errorf("expected 0 kill-barrier escalated events on identity-skip branch, got %d: %+v", len(evs), evs)
			}
			if len(barrier.warns()) != 1 {
				t.Errorf("expected exactly 1 WARN on identity-skip branch, got %d: %v", len(barrier.warns()), barrier.warns())
			}
		})
	}
}

func TestKillSaverAndWaitForDaemon_EmitsPlaceholderDiedReasonSignalOnKillSessionExit(t *testing.T) {
	installBarrierPollInterval(t, 1*time.Millisecond)
	installBarrierTimeout(t, 500*time.Millisecond)
	installBarrierReadPID(t, func(string) (int, error) { return 4321, nil })

	calls := 0
	installBarrierIsAlive(t, func(int) bool {
		calls++
		return calls < 3
	})
	barrier := &barrierLog{}
	installBarrierLogger(t, barrier)
	sink := logtest.Install(t)

	script := &portalSaverScript{
		killSession: func(int) (string, error) { return "", nil },
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.KillSaverAndWaitForDaemon(client, t.TempDir()); err != nil {
		t.Fatalf("KillSaverAndWaitForDaemon returned error: %v", err)
	}

	rec := sink.RecordsWith("saver", "placeholder died").Only(t, "saver placeholder died record")
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO", rec.Level)
	}
	if got := rec.IntAttr(t, "target_pid"); got != 4321 {
		t.Errorf("target_pid = %d, want 4321", got)
	}
	if got := rec.AttrString(t, "reason"); got != "signal" {
		t.Errorf("reason = %q, want %q", got, "signal")
	}
	if len(barrier.warns()) != 0 {
		t.Errorf("expected 0 WARN lines on observed exit, got %d: %v", len(barrier.warns()), barrier.warns())
	}
}

func TestKillSaverAndWaitForDaemon_EmitsPlaceholderDiedReasonSignalOnPostSIGKILLExit(t *testing.T) {
	installBarrierPollInterval(t, 1*time.Millisecond)
	installBarrierTimeout(t, 5*time.Millisecond)
	installBarrierEscalationTimeout(t, 500*time.Millisecond)
	installBarrierReadPID(t, func(string) (int, error) { return 4321, nil })

	installBarrierIdentifyDaemon(t, func(int) (state.IdentifyResult, error) {
		return state.IdentifyIsPortalDaemon, nil
	})

	killCalls := 0
	installBarrierSendSIGKILL(t, func(int) error {
		killCalls++
		return nil
	})
	installBarrierIsAlive(t, func(int) bool { return killCalls == 0 })

	barrier := &barrierLog{}
	installBarrierLogger(t, barrier)
	sink := logtest.Install(t)

	script := &portalSaverScript{
		killSession: func(int) (string, error) { return "", nil },
	}
	mock := commandertest.FromFunc(script.run(t))
	client := tmux.NewClient(mock)

	if err := tmux.KillSaverAndWaitForDaemon(client, t.TempDir()); err != nil {
		t.Fatalf("KillSaverAndWaitForDaemon returned error: %v", err)
	}

	rec := sink.RecordsWith("saver", "placeholder died").Only(t, "saver placeholder died record")
	if rec.Level != slog.LevelInfo {
		t.Errorf("level = %v, want INFO", rec.Level)
	}
	if got := rec.IntAttr(t, "target_pid"); got != 4321 {
		t.Errorf("target_pid = %d, want 4321", got)
	}
	if got := rec.AttrString(t, "reason"); got != "signal" {
		t.Errorf("reason = %q, want %q", got, "signal")
	}
	if len(barrier.warns()) != 0 {
		t.Errorf("expected 0 WARN lines on post-SIGKILL observed exit, got %d: %v", len(barrier.warns()), barrier.warns())
	}
}

func TestKillSaverAndWaitForDaemon_PreservesAtMostOneWarnContractAcrossLifecycleEvents(t *testing.T) {
	t.Run("escalation survives SIGKILL", func(t *testing.T) {
		installBarrierPollInterval(t, 1*time.Millisecond)
		installBarrierTimeout(t, 5*time.Millisecond)
		installBarrierEscalationTimeout(t, 5*time.Millisecond)
		installBarrierReadPID(t, func(string) (int, error) { return 4321, nil })
		installBarrierIdentifyDaemon(t, func(int) (state.IdentifyResult, error) {
			return state.IdentifyIsPortalDaemon, nil
		})
		installBarrierSendSIGKILL(t, func(int) error { return nil })
		installBarrierIsAlive(t, func(int) bool { return true })

		barrier := &barrierLog{}
		installBarrierLogger(t, barrier)
		sink := logtest.Install(t)

		script := &portalSaverScript{
			killSession: func(int) (string, error) { return "", nil },
		}
		mock := commandertest.FromFunc(script.run(t))
		client := tmux.NewClient(mock)

		if err := tmux.KillSaverAndWaitForDaemon(client, t.TempDir()); err != nil {
			t.Fatalf("KillSaverAndWaitForDaemon returned error: %v", err)
		}

		if len(barrier.warns()) != 1 {
			t.Errorf("expected exactly 1 WARN on escalation-survive path, got %d: %v", len(barrier.warns()), barrier.warns())
		}
		if evs := sink.RecordsWith("saver", "kill-barrier started"); len(evs) != 1 {
			t.Errorf("expected 1 kill-barrier started, got %d", len(evs))
		}
		if evs := sink.RecordsWith("saver", "kill-barrier escalated"); len(evs) != 1 {
			t.Errorf("expected 1 kill-barrier escalated, got %d", len(evs))
		}
		if evs := sink.RecordsWith("saver", "placeholder died"); len(evs) != 0 {
			t.Errorf("expected 0 placeholder died on never-exits path, got %d: %+v", len(evs), evs)
		}
	})
}
