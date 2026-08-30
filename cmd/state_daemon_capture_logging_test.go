package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// errorAttrRecorder keeps the first matching WARN's error attr as a live error
// value, so a test can assert errors.Is against it rather than against a string.
type errorAttrRecorder struct {
	wantMsg string
	gotErr  error
	found   bool
}

func (h *errorAttrRecorder) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *errorAttrRecorder) WithAttrs(_ []slog.Attr) slog.Handler         { return h }
func (h *errorAttrRecorder) WithGroup(_ string) slog.Handler              { return h }

func (h *errorAttrRecorder) Handle(_ context.Context, r slog.Record) error {
	if h.found || r.Level != slog.LevelWarn || r.Message != h.wantMsg {
		return nil
	}
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "error" {
			if e, ok := a.Value.Any().(error); ok {
				h.gotErr = e
				h.found = true
				return false
			}
		}
		return true
	})
	return nil
}

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

	rec := &errorAttrRecorder{wantMsg: "capture pane failed"}
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	deps := &daemonDeps{
		Dir:          dir,
		Logger:       slog.New(rec).With("component", "daemon"),
		Client:       tmux.NewClient(fc),
		HashMap:      state.HashMap{},
		TickerPeriod: 1 * time.Millisecond,
		MaxGap:       30 * time.Second,
		LastSaveAt:   time.Now(),
	}
	touchSaveRequested(t, dir)

	tick(t.Context(), deps)

	if !rec.found {
		t.Fatal("no WARN 'capture pane failed' record with an error attr was captured")
	}
	if !errors.Is(rec.gotErr, sentinel) {
		t.Errorf("error attr does not wrap the sentinel via errors.Is; got %v", rec.gotErr)
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

	wrapped := &envFailingCommander{
		inner: fc,
		envErrs: map[string]error{
			"B": errors.New("bravo-boom-sentinel"),
		},
	}

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
	wrapped := &envFailingCommander{
		inner: fc,
		envErrs: map[string]error{
			"A": noSuchSessionCommandErr("A"),
			"B": noSuchSessionCommandErr("B"),
		},
	}

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

// sessionFromExactTarget undoes the "=" exact-match prefix the tmux client
// composes onto a session target, so a fake resolves the same session real tmux
// would.
func sessionFromExactTarget(target string) string {
	return strings.TrimPrefix(target, "=")
}

type envFailingCommander struct {
	inner   *daemonFakeCommander
	envErrs map[string]error
}

func (c *envFailingCommander) Run(args ...string) (string, error) {
	if len(args) >= 3 && args[0] == "show-environment" && args[1] == "-t" {
		if err, ok := c.envErrs[sessionFromExactTarget(args[2])]; ok {
			return "", err
		}
	}
	return c.inner.Run(args...)
}

func (c *envFailingCommander) RunRaw(args ...string) (string, error) {
	return c.inner.RunRaw(args...)
}
