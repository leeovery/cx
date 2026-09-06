package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/commandertest"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

func TestDaemonTick_CapturePaneFailureErrorAttrIsWrappedError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	sess, panes := oneSession()
	sentinel := errors.New("capture-pane wrapped-sentinel")
	fc := &daemonFakeCommander{
		sessionsOut:        sess,
		panesOut:           panes,
		captureErrByTarget: map[string]error{"work:0.0": sentinel},
	}

	sink := &logtest.Sink{}
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	deps := &daemonDeps{
		Dir:          dir,
		Logger:       slog.New(sink).With("component", "daemon"),
		Client:       tmux.NewClient(fc),
		HashMap:      state.HashMap{},
		TickerPeriod: 1 * time.Millisecond,
		MaxGap:       30 * time.Second,
		LastSaveAt:   time.Now(),
	}
	touchSaveRequested(t, dir)

	tick(t.Context(), deps)

	warn := sink.Records().WithMessage("capture pane failed").AtExactLevel(slog.LevelWarn).Only(t, "'capture pane failed' WARN")
	if got := warn.ErrorAttr(t, "error"); !errors.Is(got, sentinel) {
		t.Errorf("error attr does not wrap the sentinel via errors.Is; got %v", got)
	}
}

// noSuchSessionCommandErr carries tmux's canonical lowercase "no such session"
// stderr phrasing, which ShowEnvironment wraps into an errors.Is match against
// tmux.ErrNoSuchSession - the natural-churn branch.
func noSuchSessionCommandErr(session string) error {
	return &tmux.CommandError{
		Stderr: "no such session: " + session,
		Err:    errors.New("exit status 1"),
	}
}

func TestDaemonTick_LogsAnomalousShowEnvironmentFailureUnderComponentDaemon(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	fc := &daemonFakeCommander{
		sessionsOut: "A|1|0|\nB|1|0|",
		panesOut: "A|||0|||main|||layout|||0|||1|||0|||/tmp|||1|||zsh|||\n" +
			"B|||0|||main|||layout|||0|||1|||0|||/tmp|||1|||zsh|||",
		envBySession: map[string]string{
			"A": "FOO=bar",
		},
	}

	wrapped := commandertest.Delegating(fc,
		commandertest.When(showEnvironmentFor("B"), "", errors.New("bravo-boom-sentinel")),
	)

	logger, sink := newCaptureLoggerForComponent(t, "daemon")

	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	deps := &daemonDeps{
		Dir:          dir,
		Logger:       logger,
		Client:       tmux.NewClient(wrapped),
		HashMap:      state.HashMap{},
		TickerPeriod: 1 * time.Millisecond,
		MaxGap:       30 * time.Second,
		LastSaveAt:   time.Now(),
	}
	touchSaveRequested(t, dir)

	tick(t.Context(), deps)

	log := sink.Body()

	if !strings.Contains(log, "WARN") {
		t.Errorf("expected WARN-level entry; log:\n%s", log)
	}
	if !strings.Contains(log, "component="+"daemon") {
		t.Errorf("expected ComponentDaemon (%q) entry; log:\n%s", "daemon", log)
	}
	if !strings.Contains(log, "session=B") {
		t.Errorf("expected failing session name %q to appear in WARN body; log:\n%s", "B", log)
	}
	if !strings.Contains(log, "bravo-boom-sentinel") {
		t.Errorf("expected underlying error text to appear in WARN body; log:\n%s", log)
	}
}

func TestDaemonTick_LogsPerSessionWarnAndCommitsEmptyOnAllNaturalChurn(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	fc := &daemonFakeCommander{
		sessionsOut: "A|1|0|\nB|1|0|",
		panesOut: "A|||0|||main|||layout|||0|||1|||0|||/tmp|||1|||zsh|||\n" +
			"B|||0|||main|||layout|||0|||1|||0|||/tmp|||1|||zsh|||",
	}
	wrapped := commandertest.Delegating(fc,
		commandertest.When(showEnvironmentFor("A"), "", noSuchSessionCommandErr("A")),
		commandertest.When(showEnvironmentFor("B"), "", noSuchSessionCommandErr("B")),
	)

	logger, sink := newCaptureLoggerForComponent(t, "daemon")

	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	deps := &daemonDeps{
		Dir:          dir,
		Logger:       logger,
		Client:       tmux.NewClient(wrapped),
		HashMap:      state.HashMap{},
		TickerPeriod: 1 * time.Millisecond,
		MaxGap:       30 * time.Second,
		LastSaveAt:   time.Now(),
	}
	touchSaveRequested(t, dir)

	tick(t.Context(), deps)

	log := sink.Body()

	// One per failing session and no more: the all-natural-churn path returns a
	// nil error, so tick must add no "tick failed" wrapper of its own.
	warnCount := strings.Count(log, "WARN")
	if warnCount != 2 {
		t.Errorf("WARN entries = %d, want 2; log:\n%s", warnCount, log)
	}
	if !strings.Contains(log, "session=A") {
		t.Errorf("expected WARN for session A; log:\n%s", log)
	}
	if !strings.Contains(log, "session=B") {
		t.Errorf("expected WARN for session B; log:\n%s", log)
	}

	data, err := os.ReadFile(state.SessionsJSON(dir))
	if err != nil {
		t.Fatalf("sessions.json must be committed on all-natural-churn: %v", err)
	}
	committed, err := state.DecodeIndex(data)
	if err != nil {
		t.Fatalf("decode sessions.json: %v", err)
	}
	if len(committed.Sessions) != 0 {
		t.Errorf("committed sessions length = %d, want 0 (empty Commit on all-natural-churn)",
			len(committed.Sessions))
	}
}

func TestDaemonTick_CapturePaneFailureLogsWarnWithPaneKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	sess, panes := oneSession()
	sentinel := errors.New("capture-pane boom-sentinel")
	fc := &daemonFakeCommander{
		sessionsOut:        sess,
		panesOut:           panes,
		captureErrByTarget: map[string]error{"work:0.0": sentinel},
	}

	logger, sink := newCaptureLoggerForComponent(t, "daemon")

	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	deps := &daemonDeps{
		Dir:          dir,
		Logger:       logger,
		Client:       tmux.NewClient(fc),
		HashMap:      state.HashMap{},
		TickerPeriod: 1 * time.Millisecond,
		MaxGap:       30 * time.Second,
		LastSaveAt:   time.Now(),
	}
	touchSaveRequested(t, dir)

	tick(t.Context(), deps)

	log := sink.Body()
	if !strings.Contains(log, "WARN") {
		t.Errorf("expected WARN-level entry; log:\n%s", log)
	}
	if !strings.Contains(log, "component=daemon") {
		t.Errorf("expected component=daemon entry; log:\n%s", log)
	}
	if !strings.Contains(log, "capture pane failed") {
		t.Errorf("expected 'capture pane failed' message; log:\n%s", log)
	}
	if !strings.Contains(log, "pane_key=work:0.0") {
		t.Errorf("expected pane_key=work:0.0 attr on the WARN; log:\n%s", log)
	}
	if !strings.Contains(log, "capture-pane boom-sentinel") {
		t.Errorf("expected wrapped error text in the WARN; log:\n%s", log)
	}
}

// sessionFromExactTarget undoes the exact target form the tmux client composes
// for a per-session read: the "=" exact-match prefix and the trailing ":" that
// makes tmux read the leading component as a session name rather than splitting
// it on a period. A fake resolves the same session real tmux would.
func sessionFromExactTarget(target string) string {
	return strings.TrimSuffix(strings.TrimPrefix(target, "="), ":")
}

// showEnvironmentFor matches the per-session environment read, resolving the
// session the same way real tmux resolves the client's exact-match target.
func showEnvironmentFor(session string) func(args []string) bool {
	return func(args []string) bool {
		return len(args) >= 3 && args[0] == "show-environment" && args[1] == "-t" &&
			sessionFromExactTarget(args[2]) == session
	}
}

// TestCaptureAndCommit_AddressesEachPaneThroughThePinnedTarget pins the capture
// read's target form. A session killed between the structure read and the
// capture leaves an unpinned target resolving onto a prefix sibling's pane,
// whose scrollback would then be written under the gone pane's key.
func TestCaptureAndCommit_AddressesEachPaneThroughThePinnedTarget(t *testing.T) {
	t.Run("it captures a pane through the pinned pane target", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", dir)

		sess, panes := oneSession()
		fc := &daemonFakeCommander{sessionsOut: sess, panesOut: panes}
		deps := makeCaptureDeps(t, dir, fc)

		if err := captureAndCommit(context.Background(), deps); err != nil {
			t.Fatalf("captureAndCommit: %v", err)
		}

		calls := fc.callsContaining("capture-pane")
		if len(calls) != 1 {
			t.Fatalf("capture-pane calls = %d, want 1: %v", len(calls), calls)
		}
		got := calls[0][len(calls[0])-1]
		if want := "=work:0.0"; got != want {
			t.Errorf("capture-pane target = %q, want %q", got, want)
		}
	})
}
