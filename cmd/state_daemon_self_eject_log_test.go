package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
)

// One sink backs both the returned daemon-component logger and log.For, so
// records from the self-eject line and from log.Close interleave in emission
// order and their relative order is assertable.
func newSharedSelfEjectCapture(t *testing.T) (*logtest.Sink, *daemonDeps) {
	t.Helper()
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	depsLogger := slog.New(sink).With("component", "daemon")
	return sink, &daemonDeps{Logger: depsLogger}
}

func TestDaemonSelfEject_EmitsCatalogedEventAtTrip(t *testing.T) {
	t.Setenv("PORTAL_LOG_LEVEL", "info")
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	withSaverMembershipProbeFake(t, func(_ *tmux.Client, _ int) bool { return false })
	withOsExitFake(t, func(_ int) { panic("osExit invoked") })

	fc := &daemonFakeCommander{}
	deps := makeDeps(t, dir, fc)
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps.Logger = logger
	deps.TickerPeriod = 1 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	done := runDaemonLoopUntilEject(t, deps, ctx)
	<-done

	want := fmt.Sprintf("ticks=%d", selfSupervisionHysteresisTicks)
	wantThreshold := fmt.Sprintf("threshold=%d", selfSupervisionHysteresisTicks)
	if n := countLines(sink, "INFO", "self-eject", "component=daemon", want, wantThreshold); n != 1 {
		t.Errorf("expected exactly one cataloged self-eject INFO with %s %s; got %d in:\n%s",
			want, wantThreshold, n, sink.Body())
	}
}

func TestDaemonSelfEject_RemovesLegacyInfoLine(t *testing.T) {
	t.Setenv("PORTAL_LOG_LEVEL", "info")
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	withSaverMembershipProbeFake(t, func(_ *tmux.Client, _ int) bool { return false })
	withOsExitFake(t, func(_ int) { panic("osExit invoked") })

	fc := &daemonFakeCommander{}
	deps := makeDeps(t, dir, fc)
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps.Logger = logger
	deps.TickerPeriod = 1 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	done := runDaemonLoopUntilEject(t, deps, ctx)
	<-done

	if strings.Contains(sink.Body(), "self-supervision: saver-membership lost") {
		t.Errorf("legacy 'self-supervision: saver-membership lost' INFO line still present in:\n%s", sink.Body())
	}
}

func TestDaemonSelfEject_OrderSelfEjectThenProcessExitThenOsExit(t *testing.T) {
	t.Setenv("PORTAL_LOG_LEVEL", "info")
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	withSaverMembershipProbeFake(t, func(_ *tmux.Client, _ int) bool { return false })

	sink, depsBase := newSharedSelfEjectCapture(t)

	// Snapshot at the instant osExit fires, so both lifecycle lines can be shown
	// to have been emitted before it.
	var linesAtExit []string
	var exitCalls int32
	withOsExitFake(t, func(_ int) {
		atomic.AddInt32(&exitCalls, 1)
		linesAtExit = sink.Lines()
		panic("osExit invoked")
	})

	fc := &daemonFakeCommander{}
	deps := makeDeps(t, dir, fc)
	deps.Logger = depsBase.Logger
	deps.TickerPeriod = 1 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	done := runDaemonLoopUntilEject(t, deps, ctx)
	<-done

	if atomic.LoadInt32(&exitCalls) != 1 {
		t.Fatalf("osExit invoked %d times; want exactly 1", exitCalls)
	}

	selfEjectIdx := indexOfLineContaining(linesAtExit, "self-eject", "component=daemon")
	processExitIdx := indexOfLineContaining(linesAtExit, "exit", "component=process", "code=0")
	if selfEjectIdx < 0 {
		t.Fatalf("self-eject INFO not recorded before osExit; lines at exit:\n%s", strings.Join(linesAtExit, "\n"))
	}
	if processExitIdx < 0 {
		t.Fatalf("process: exit (via log.Close) not recorded before osExit; lines at exit:\n%s", strings.Join(linesAtExit, "\n"))
	}
	if selfEjectIdx >= processExitIdx {
		t.Errorf("ordering violated: self-eject at index %d, process: exit at index %d; want self-eject FIRST then process: exit\nlines at exit:\n%s",
			selfEjectIdx, processExitIdx, strings.Join(linesAtExit, "\n"))
	}
}

func TestDaemonSelfEject_ProcessExitCarriesCodeZero(t *testing.T) {
	t.Setenv("PORTAL_LOG_LEVEL", "info")
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	withSaverMembershipProbeFake(t, func(_ *tmux.Client, _ int) bool { return false })
	withOsExitFake(t, func(_ int) { panic("osExit invoked") })

	sink, depsBase := newSharedSelfEjectCapture(t)

	fc := &daemonFakeCommander{}
	deps := makeDeps(t, dir, fc)
	deps.Logger = depsBase.Logger
	deps.TickerPeriod = 1 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	done := runDaemonLoopUntilEject(t, deps, ctx)
	<-done

	if n := countLines(sink, "INFO", "exit", "component=process", "code=0"); n != 1 {
		t.Errorf("expected exactly one process: exit INFO with code=0 (via log.Close(0)); got %d in:\n%s", n, sink.Body())
	}
}

func TestDaemonSelfEject_DoesNotEmitShutdownLine(t *testing.T) {
	t.Setenv("PORTAL_LOG_LEVEL", "info")
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	withSaverMembershipProbeFake(t, func(_ *tmux.Client, _ int) bool { return false })
	withOsExitFake(t, func(_ int) { panic("osExit invoked") })

	var shutdownCalls atomic.Int32
	withDaemonShutdownFuncFake(t, func(_ *daemonDeps) error {
		shutdownCalls.Add(1)
		return nil
	})

	fc := &daemonFakeCommander{}
	deps := makeDeps(t, dir, fc)
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps.Logger = logger
	deps.TickerPeriod = 1 * time.Millisecond
	deps.LastSaveAt = time.Now() // gap=false so tick body is a no-op fast path

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	done := runDaemonLoopUntilEject(t, deps, ctx)
	<-done

	if got := shutdownCalls.Load(); got != 0 {
		t.Errorf("daemonShutdownFunc invoked %d times on eject path; want 0", got)
	}
	if n := countLines(sink, "shutdown"); n != 0 {
		t.Errorf("expected no 'shutdown' line on self-eject path; got %d in:\n%s", n, sink.Body())
	}
}

func TestDaemonSelfEject_BelowThresholdEmitsDebugOnly(t *testing.T) {
	t.Setenv("PORTAL_LOG_LEVEL", "debug")
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	N := selfSupervisionHysteresisTicks
	// false × (N-1) then true forever — the counter climbs to N-1, never trips.
	var tickIdx atomic.Int32
	withSaverMembershipProbeFake(t, func(_ *tmux.Client, _ int) bool {
		idx := tickIdx.Add(1)
		return int(idx) >= N // first N-1 false, then true forever
	})

	var exitCalls atomic.Int32
	withOsExitFake(t, func(_ int) {
		exitCalls.Add(1)
		panic("osExit invoked unexpectedly below threshold")
	})
	withDaemonShutdownFuncFake(t, func(_ *daemonDeps) error { return nil })

	fc := &daemonFakeCommander{}
	deps := makeDeps(t, dir, fc)
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps.Logger = logger
	deps.TickerPeriod = 1 * time.Millisecond
	deps.LastSaveAt = time.Now() // gap=false → tick body is a no-op fast path

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel)

	if err := defaultDaemonRun(ctx, deps); err != nil {
		t.Fatalf("defaultDaemonRun: %v", err)
	}

	if got := exitCalls.Load(); got != 0 {
		t.Fatalf("osExit invoked %d times below threshold; want 0", got)
	}
	wantThreshold := fmt.Sprintf("threshold=%d", N)
	if n := countLines(sink, "DEBUG", "saver-membership probe failed", wantThreshold); n < N-1 {
		t.Errorf("expected at least %d DEBUG probe-failure breadcrumbs; got %d in:\n%s", N-1, n, sink.Body())
	}
	if n := countLines(sink, "INFO", "self-eject"); n != 0 {
		t.Errorf("expected no self-eject INFO below threshold; got %d in:\n%s", n, sink.Body())
	}
}

func TestDaemonSelfEject_PassingProbeResetsCounter(t *testing.T) {
	t.Setenv("PORTAL_LOG_LEVEL", "debug")
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	N := selfSupervisionHysteresisTicks
	// (false × N-1, true) × 2, then true forever.
	script := make([]bool, 0, 2*N)
	for i := 0; i < N-1; i++ {
		script = append(script, false)
	}
	script = append(script, true)
	for i := 0; i < N-1; i++ {
		script = append(script, false)
	}
	script = append(script, true)

	probe, probeCalls := scriptedProbe(script)
	withSaverMembershipProbeFake(t, probe)

	var exitCalls atomic.Int32
	withOsExitFake(t, func(_ int) {
		exitCalls.Add(1)
		panic("osExit invoked despite reset")
	})
	withDaemonShutdownFuncFake(t, func(_ *daemonDeps) error { return nil })

	fc := &daemonFakeCommander{}
	deps := makeDeps(t, dir, fc)
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps.Logger = logger
	deps.TickerPeriod = 1 * time.Millisecond
	deps.LastSaveAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel)

	if err := defaultDaemonRun(ctx, deps); err != nil {
		t.Fatalf("defaultDaemonRun: %v", err)
	}

	if got := exitCalls.Load(); got != 0 {
		t.Fatalf("osExit invoked %d times despite counter reset; want 0", got)
	}
	if n := countLines(sink, "INFO", "self-eject"); n != 0 {
		t.Errorf("expected no self-eject INFO when counter resets; got %d in:\n%s", n, sink.Body())
	}
	if got := probeCalls(); got < int32(2*N-1) {
		t.Fatalf("probe invoked %d times; want >= %d to exercise the full reset script", got, 2*N-1)
	}
}

func TestDaemonSelfEject_UsesOsExitSeam(t *testing.T) {
	t.Setenv("PORTAL_LOG_LEVEL", "info")
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	withSaverMembershipProbeFake(t, func(_ *tmux.Client, _ int) bool { return false })

	var exitCalls atomic.Int32
	var exitCode int32 = -1
	withOsExitFake(t, func(code int) {
		atomic.StoreInt32(&exitCode, int32(code))
		exitCalls.Add(1)
		panic("osExit invoked")
	})

	fc := &daemonFakeCommander{}
	deps := makeDeps(t, dir, fc)
	deps.TickerPeriod = 1 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	done := runDaemonLoopUntilEject(t, deps, ctx)
	<-done

	if got := exitCalls.Load(); got != 1 {
		t.Fatalf("osExit seam invoked %d times; want exactly 1", got)
	}
	if got := atomic.LoadInt32(&exitCode); got != 0 {
		t.Errorf("osExit code = %d; want 0", got)
	}
}

func indexOfLineContaining(lines []string, substrs ...string) int {
	for i, line := range lines {
		all := true
		for _, s := range substrs {
			if !strings.Contains(line, s) {
				all = false
				break
			}
		}
		if all {
			return i
		}
	}
	return -1
}
