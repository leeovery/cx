package bootstrap

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

func TestSweepOrphanDaemons_EmitsCleanSummaryCountingSuccessfulKills(t *testing.T) {
	sink := logtest.Install(t)
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{2001, 2002}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       log.For("bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}

	rec := sink.OnlyRecordWith(t, "clean", "orphan-daemon sweep complete")
	if rec.Level != slog.LevelInfo {
		t.Errorf("summary level = %v, want INFO", rec.Level)
	}
	if got := rec.IntAttr(t, "killed"); got != 2 {
		t.Errorf("killed = %d, want 2", got)
	}
	rec.RequireDuration(t, "took")
}

func TestSweepOrphanDaemons_DemotesPerKillInfoToDebug(t *testing.T) {
	sink := logtest.Install(t)
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{2001}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       log.For("bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}

	for _, r := range sink.Records() {
		if r.Level == slog.LevelInfo && r.Msg == "orphan killed" {
			t.Errorf("per-kill line must not be INFO: %+v", r)
		}
		if r.Level == slog.LevelInfo && r.Msg == "sweep: killed orphan daemon" {
			t.Errorf("old per-kill INFO message must be gone: %+v", r)
		}
	}
	dbg := sink.RecordsWith("clean", "orphan killed").AtExactLevel(slog.LevelDebug)
	if len(dbg) != 1 {
		t.Fatalf("expected 1 DEBUG 'orphan killed' under clean, got %d: %+v", len(dbg), sink.Records())
	}
	if pid, ok := dbg[0].Attrs["target_pid"]; !ok || pid.Int64() != 2001 {
		t.Errorf("DEBUG 'orphan killed' target_pid = %v, want 2001", dbg[0].Attrs["target_pid"])
	}
}

func TestSweepOrphanDaemons_ExcludesSkippedAndFailedFromKilled(t *testing.T) {
	sink := logtest.Install(t)
	identify := &recordingIdentify{
		results: map[int]identifyOutcome{
			3001: {res: state.IdentifyNotPortalDaemon},
			3002: {res: state.IdentifyIsPortalDaemon},
			3003: {res: state.IdentifyIsPortalDaemon},
		},
	}
	kill := &recordingKill{errs: map[int]error{3002: errors.New("kill: no such process")}}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{3001, 3002, 3003}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       log.For("bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}

	rec := sink.OnlyRecordWith(t, "clean", "orphan-daemon sweep complete")
	if got := rec.IntAttr(t, "killed"); got != 1 {
		t.Errorf("killed = %d, want 1 (excludes skip + failed kill)", got)
	}

	skips := sink.RecordsWith("bootstrap", "sweep: pid not identity-checked as portal daemon, skipping").AtExactLevel(slog.LevelDebug)
	if len(skips) != 1 {
		t.Errorf("expected 1 identity-skip DEBUG under bootstrap, got %d: %+v", len(skips), sink.Records())
	}
	warns := sink.RecordsWith("bootstrap", "sweep: kill failed").AtExactLevel(slog.LevelWarn)
	if len(warns) != 1 {
		t.Errorf("expected 1 kill-failure WARN under bootstrap, got %d: %+v", len(warns), sink.Records())
	}
}

func TestSweepOrphanDaemons_NoSummaryWhenPgrepFails(t *testing.T) {
	sink := logtest.Install(t)
	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return nil, errors.New("pgrep boom") },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     func(pid int) (state.IdentifyResult, error) { return state.IdentifyIsPortalDaemon, nil },
		Kill:         func(pid int) error { return nil },
		Logger:       log.For("bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}

	if got := sink.RecordsWith("clean", "orphan-daemon sweep complete"); len(got) != 0 {
		t.Errorf("expected no summary on pgrep failure (returns before loop), got %d: %+v", len(got), got)
	}
}

func TestSweepOrphanDaemons_SummaryWithZeroKilledWhenSaverPanePIDErrors(t *testing.T) {
	sink := logtest.Install(t)
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyDead}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{4001, 4002}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, errors.New("list-panes boom") },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       log.For("bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}

	rec := sink.OnlyRecordWith(t, "clean", "orphan-daemon sweep complete")
	if got := rec.IntAttr(t, "killed"); got != 0 {
		t.Errorf("killed = %d, want 0", got)
	}
	rec.RequireDuration(t, "took")
}

func TestCleanStaleMarkers_EmitsCleanSummaryCountingSuccessfulUnsets(t *testing.T) {
	sink := logtest.Install(t)
	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"stale1__0.0": {},
		"stale2__1.2": {},
		"live__0.0":   {},
	}}
	live := &fakeLivePaneLister{output: "live:0.0\n"}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
		Logger:   log.For("bootstrap"),
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers returned error: %v", err)
	}

	rec := sink.OnlyRecordWith(t, "clean", "marker sweep complete")
	if rec.Level != slog.LevelInfo {
		t.Errorf("summary level = %v, want INFO", rec.Level)
	}
	if got := rec.IntAttr(t, "unset"); got != 2 {
		t.Errorf("unset = %d, want 2", got)
	}
	rec.RequireDuration(t, "took")
}

func TestCleanStaleMarkers_SummaryUnsetCountsOnlySuccessfulUnsets(t *testing.T) {
	sink := logtest.Install(t)
	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"a__0.0": {},
		"b__0.0": {},
		"c__0.0": {},
	}}
	live := &fakeLivePaneLister{output: "alive:9.9\n"}
	sentinel := errors.New("tmux: option boom")
	unsetter := &fakeMarkerUnsetter{errs: map[int]error{2: sentinel}}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
		Logger:   log.For("bootstrap"),
	}
	err := c.CleanStaleMarkers()
	if err == nil {
		t.Fatalf("expected aggregate error when one unset fails; got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected returned error to wrap sentinel %v, got %v", sentinel, err)
	}

	rec := sink.OnlyRecordWith(t, "clean", "marker sweep complete")
	if got := rec.IntAttr(t, "unset"); got != 2 {
		t.Errorf("unset = %d, want 2 (counts successful unsets only)", got)
	}
}

func TestCleanStaleMarkers_SummaryUnsetZeroOnMassUnsetHazardDeferral(t *testing.T) {
	sink := logtest.Install(t)
	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"protected__0.0": {},
		"another__1.2":   {},
	}}
	live := &fakeLivePaneLister{output: ""}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
		Logger:   log.For("bootstrap"),
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers must return nil for mass-unset-hazard deferral; got %v", err)
	}
	if len(unsetter.calls) != 0 {
		t.Errorf("expected zero unset calls under deferral, got %v", unsetter.calls)
	}

	rec := sink.OnlyRecordWith(t, "clean", "marker sweep complete")
	if got := rec.IntAttr(t, "unset"); got != 0 {
		t.Errorf("unset = %d, want 0 (never a false unset on deferral)", got)
	}
	warns := sink.RecordsWith("bootstrap", "stale-marker cleanup: zero live panes parsed with markers present; skipping to avoid mass-unset hazard (next bootstrap retries)").AtExactLevel(slog.LevelWarn)
	if len(warns) != 1 {
		t.Errorf("expected 1 deferral WARN under bootstrap, got %d: %+v", len(warns), sink.Records())
	}
}

func TestCleanStaleMarkers_NoSummaryWhenListErrorReturns(t *testing.T) {
	t.Run("ListSkeletonMarkers error", func(t *testing.T) {
		sink := logtest.Install(t)
		lister := &fakeMarkerLister{err: errors.New("show-options: tmux dead")}
		live := &fakeLivePaneLister{output: "live:0.0\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
			Logger:   log.For("bootstrap"),
		}
		if err := c.CleanStaleMarkers(); err == nil {
			t.Fatalf("expected non-nil error from ListSkeletonMarkers failure")
		}
		if got := sink.RecordsWith("clean", "marker sweep complete"); len(got) != 0 {
			t.Errorf("expected no summary on ListSkeletonMarkers error, got %d: %+v", len(got), got)
		}
	})

	t.Run("ListAllPanesWithFormat error", func(t *testing.T) {
		sink := logtest.Install(t)
		lister := &fakeMarkerLister{markers: map[string]struct{}{"m__0.0": {}}}
		live := &fakeLivePaneLister{err: errors.New("list-panes: socket gone")}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
			Logger:   log.For("bootstrap"),
		}
		if err := c.CleanStaleMarkers(); err == nil {
			t.Fatalf("expected non-nil error from ListAllPanesWithFormat failure")
		}
		if got := sink.RecordsWith("clean", "marker sweep complete"); len(got) != 0 {
			t.Errorf("expected no summary on ListAllPanesWithFormat error, got %d: %+v", len(got), got)
		}
	})
}

func TestCleanStaleMarkers_SummaryUnsetZeroOnEmptyMarkersNoOp(t *testing.T) {
	sink := logtest.Install(t)
	lister := &fakeMarkerLister{markers: map[string]struct{}{}}
	live := &fakeLivePaneLister{output: ""}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
		Logger:   log.For("bootstrap"),
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers returned error: %v", err)
	}

	rec := sink.OnlyRecordWith(t, "clean", "marker sweep complete")
	if got := rec.IntAttr(t, "unset"); got != 0 {
		t.Errorf("unset = %d, want 0 on empty-markers no-op", got)
	}
	rec.RequireDuration(t, "took")
}

var _ = tmux.StructuralKeyFormat
