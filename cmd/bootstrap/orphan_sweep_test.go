package bootstrap

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
)

// listPanesWarnMessage is the message of the sweep's saver-read failure WARN.
const listPanesWarnMessage = "sweep: list-panes _portal-saver failed, legitimate set empty"

type recordingIdentify struct {
	calls   []int
	results map[int]identifyOutcome
	def     identifyOutcome
}

type identifyOutcome struct {
	res state.IdentifyResult
	err error
}

func (r *recordingIdentify) fn(pid int) (state.IdentifyResult, error) {
	r.calls = append(r.calls, pid)
	if r.results != nil {
		if v, ok := r.results[pid]; ok {
			return v.res, v.err
		}
	}
	return r.def.res, r.def.err
}

type recordingKill struct {
	calls []int
	errs  map[int]error
}

func (r *recordingKill) fn(pid int) error {
	r.calls = append(r.calls, pid)
	if r.errs != nil {
		if e, ok := r.errs[pid]; ok {
			return e
		}
	}
	return nil
}

func TestSweepOrphanDaemons_killsTwoOrphansLeavesLegitimate(t *testing.T) {
	const legitPID = 1000
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{legitPID, 2001, 2002}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return legitPID, true, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}

	if len(kill.calls) != 2 {
		t.Fatalf("expected 2 kill calls, got %d (%v)", len(kill.calls), kill.calls)
	}
	got := map[int]struct{}{kill.calls[0]: {}, kill.calls[1]: {}}
	for _, want := range []int{2001, 2002} {
		if _, ok := got[want]; !ok {
			t.Errorf("expected pid %d killed; got %v", want, kill.calls)
		}
	}
	for _, p := range kill.calls {
		if p == legitPID {
			t.Errorf("legitimate pid %d must not be killed", legitPID)
		}
	}
}

func TestSweepOrphanDaemons_saverAbsentKillsAllIdentifying(t *testing.T) {
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{3001, 3002, 3003}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}

	if len(kill.calls) != 3 {
		t.Fatalf("expected 3 kill calls (legitimate set empty), got %d (%v)", len(kill.calls), kill.calls)
	}
}

func TestSweepOrphanDaemons_pgrepErrorLogsWarnReturnsNil(t *testing.T) {
	sink := &logtest.Sink{}
	sentinel := errors.New("pgrep boom")
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return nil, sentinel },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     func(pid int) (state.IdentifyResult, error) { return state.IdentifyIsPortalDaemon, nil },
		Kill:         kill.fn,
		Logger:       slog.New(sink).With("component", "bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("expected nil err under pgrep failure; got %v", err)
	}
	if len(kill.calls) != 0 {
		t.Errorf("expected zero kill calls on pgrep error; got %v", kill.calls)
	}
	warn := sink.Records().WithMessage("sweep: pgrep failed").AtExactLevel(slog.LevelWarn).Only(t, "pgrep-failure WARN")
	if comp := warn.AttrString(t, "component"); comp != "bootstrap" {
		t.Errorf("pgrep Warn component = %q, want %q", comp, "bootstrap")
	}
	if got := warn.ErrorAttr(t, "error"); !errors.Is(got, sentinel) {
		t.Errorf("pgrep Warn error = %v, want %v", got, sentinel)
	}
}

func TestSweepOrphanDaemons_listPanesErrorTreatsLegitimateEmpty(t *testing.T) {
	sink := &logtest.Sink{}
	sentinel := errors.New("list-panes boom")
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{4001, 4002}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, sentinel },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       slog.New(sink).With("component", "bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	if len(kill.calls) != 2 {
		t.Fatalf("expected 2 kill calls (legitimate empty), got %d (%v)", len(kill.calls), kill.calls)
	}
	warn := sink.Records().WithMessage(listPanesWarnMessage).AtExactLevel(slog.LevelWarn).Only(t, "list-panes-failure WARN")
	if comp := warn.AttrString(t, "component"); comp != "bootstrap" {
		t.Errorf("list-panes Warn component = %q, want %q", comp, "bootstrap")
	}
	if got := warn.ErrorAttr(t, "error"); !errors.Is(got, sentinel) {
		t.Errorf("list-panes Warn error = %v, want %v", got, sentinel)
	}
}

func TestSweepOrphanDaemons_identifyDeadSkipped(t *testing.T) {
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyDead}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{5001, 5002}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	if len(kill.calls) != 0 {
		t.Errorf("IdentifyDead must skip kill; got %v", kill.calls)
	}
}

func TestSweepOrphanDaemons_identifyNotPortalDaemonSkipped(t *testing.T) {
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyNotPortalDaemon}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{6001, 6002}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	if len(kill.calls) != 0 {
		t.Errorf("IdentifyNotPortalDaemon must skip kill; got %v", kill.calls)
	}
}

func TestSweepOrphanDaemons_identifyTransientErrorSkipped(t *testing.T) {
	sink := &logtest.Sink{}
	transient := errors.New("ps malformed output")
	identify := &recordingIdentify{def: identifyOutcome{err: transient}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{7001}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       slog.New(sink).With("component", "bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	if len(kill.calls) != 0 {
		t.Errorf("Identify transient error must skip kill; got %v", kill.calls)
	}
	warn := sink.Records().WithMessage("sweep: identity-check failed, skipping").AtExactLevel(slog.LevelWarn).
		Only(t, "identity-check-failure WARN")
	if pid := warn.IntAttr(t, "target_pid"); pid != 7001 {
		t.Errorf("identity-check Warn target_pid = %d, want 7001", pid)
	}
}

func TestSweepOrphanDaemons_killErrorLogsWarnContinues(t *testing.T) {
	sink := &logtest.Sink{}
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	killSentinel := errors.New("kill: no such process")
	kill := &recordingKill{errs: map[int]error{8001: killSentinel}}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{8001, 8002}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       slog.New(sink).With("component", "bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	if len(kill.calls) != 2 {
		t.Errorf("expected both PIDs attempted despite first kill error; got %v", kill.calls)
	}
	warn := sink.Records().WithMessage("sweep: kill failed").AtExactLevel(slog.LevelWarn).Only(t, "kill-failure WARN")
	if pid := warn.IntAttr(t, "target_pid"); pid != 8001 {
		t.Errorf("kill Warn target_pid = %d, want 8001", pid)
	}
}

func TestSweepOrphanDaemons_cleanStateZeroInfo(t *testing.T) {
	const legitPID = 9000
	sink := &logtest.Sink{}
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{legitPID}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return legitPID, true, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       slog.New(sink).With("component", "bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	if len(kill.calls) != 0 {
		t.Errorf("clean state must send zero signals; got %v", kill.calls)
	}
	for _, rec := range sink.Records().AtExactLevel(slog.LevelInfo) {
		if strings.Contains(rec.Msg, "killed orphan daemon") {
			t.Errorf("clean state must emit zero killed-orphan INFO entries; got %+v", rec)
		}
	}
}

func TestSweepOrphanDaemons_neverSIGTERM(t *testing.T) {
	var capturedSig syscall.Signal
	var capturedPID int
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill: func(pid int) error {
			capturedPID = pid
			capturedSig = syscall.SIGKILL
			return nil
		},
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	if capturedPID != 0 {
		t.Errorf("unexpected Kill invocation; pid=%d sig=%v", capturedPID, capturedSig)
	}
	if capturedSig != 0 {
		t.Errorf("unexpected signal recorded; sig=%v", capturedSig)
	}
}

func TestSweepOrphanDaemons_defensiveOwnPIDSkip(t *testing.T) {
	ownPID := os.Getpid()
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{ownPID, 10001}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	for _, p := range kill.calls {
		if p == ownPID {
			t.Fatalf("own pid %d must never be killed; got %v", ownPID, kill.calls)
		}
	}
	if len(kill.calls) != 1 || kill.calls[0] != 10001 {
		t.Errorf("expected only 10001 killed; got %v", kill.calls)
	}
}

func TestSweepOrphanDaemons_pgrepEmptyListNoOp(t *testing.T) {
	sink := &logtest.Sink{}
	kill := &recordingKill{}
	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     func(pid int) (state.IdentifyResult, error) { return 0, nil },
		Kill:         kill.fn,
		Logger:       slog.New(sink).With("component", "bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	if len(kill.calls) != 0 {
		t.Errorf("empty pgrep must produce zero kill calls; got %v", kill.calls)
	}
	if warns := sink.Records().AtExactLevel(slog.LevelWarn); warns != nil {
		t.Errorf("empty pgrep must produce zero warnings; got %+v", warns)
	}
	if infos := sink.Records().AtExactLevel(slog.LevelInfo); infos != nil {
		t.Errorf("empty pgrep must produce zero INFO entries; got %+v", infos)
	}
}

func TestSweepOrphanDaemons_perKillNotEmittedAtInfoOnBootstrapLogger(t *testing.T) {
	sink := &logtest.Sink{}
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	kill := &recordingKill{}

	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{11001}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       slog.New(sink).With("component", "bootstrap"),
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error: %v", err)
	}
	if len(kill.calls) != 1 || kill.calls[0] != 11001 {
		t.Fatalf("expected pid 11001 killed; got %v", kill.calls)
	}
	for _, rec := range sink.Records().AtExactLevel(slog.LevelInfo) {
		if strings.Contains(rec.Msg, "killed orphan daemon") {
			t.Errorf("per-kill INFO must be demoted off the bootstrap logger; got %q (component %q)", rec.Msg, rec.AttrOrEmpty("component"))
		}
	}
}

func TestSweepOrphanDaemons_nilLoggerSafe(t *testing.T) {
	identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
	kill := &recordingKill{}
	c := &OrphanSweepCore{
		Pgrep:        func() ([]int, error) { return []int{12001}, nil },
		SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
		Identify:     identify.fn,
		Kill:         kill.fn,
		Logger:       nil,
	}
	if err := c.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned error under nil Logger: %v", err)
	}
}

func TestSweepOrphanDaemons_presentVsAbsentTriState(t *testing.T) {
	t.Run("absent — (0, false, nil) — empty legit set, no warning", func(t *testing.T) {
		sink := &logtest.Sink{}
		identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
		kill := &recordingKill{}
		c := &OrphanSweepCore{
			Pgrep:        func() ([]int, error) { return []int{20001}, nil },
			SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
			Identify:     identify.fn,
			Kill:         kill.fn,
			Logger:       slog.New(sink).With("component", "bootstrap"),
		}
		if err := c.SweepOrphanDaemons(); err != nil {
			t.Fatalf("SweepOrphanDaemons returned error: %v", err)
		}
		if len(kill.calls) != 1 || kill.calls[0] != 20001 {
			t.Errorf("absent: expected pid 20001 killed (empty legit set); got %v", kill.calls)
		}
		if warns := sink.Records().WithMessage(listPanesWarnMessage).AtExactLevel(slog.LevelWarn); warns != nil {
			t.Errorf("absent path must NOT emit list-panes Warn; got %+v", warns)
		}
	})

	t.Run("present with pid 0 — (0, true, nil) — distinct from absent, no warning", func(t *testing.T) {
		sink := &logtest.Sink{}
		identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
		kill := &recordingKill{}
		c := &OrphanSweepCore{
			Pgrep:        func() ([]int, error) { return []int{20002}, nil },
			SaverPanePID: func() (pid int, present bool, err error) { return 0, true, nil },
			Identify:     identify.fn,
			Kill:         kill.fn,
			Logger:       slog.New(sink).With("component", "bootstrap"),
		}
		if err := c.SweepOrphanDaemons(); err != nil {
			t.Fatalf("SweepOrphanDaemons returned error: %v", err)
		}
		if warns := sink.Records().WithMessage(listPanesWarnMessage).AtExactLevel(slog.LevelWarn); warns != nil {
			t.Errorf("present=true path must NOT emit list-panes Warn; got %+v", warns)
		}
		if len(kill.calls) != 1 || kill.calls[0] != 20002 {
			t.Errorf("present (pid 0): expected pid 20002 killed; got %v", kill.calls)
		}
	})

	t.Run("error — (0, false, err) — warning emitted, sweep proceeds", func(t *testing.T) {
		sink := &logtest.Sink{}
		sentinel := errors.New("list-panes tri boom")
		identify := &recordingIdentify{def: identifyOutcome{res: state.IdentifyIsPortalDaemon}}
		kill := &recordingKill{}
		c := &OrphanSweepCore{
			Pgrep:        func() ([]int, error) { return []int{20003}, nil },
			SaverPanePID: func() (pid int, present bool, err error) { return 0, false, sentinel },
			Identify:     identify.fn,
			Kill:         kill.fn,
			Logger:       slog.New(sink).With("component", "bootstrap"),
		}
		if err := c.SweepOrphanDaemons(); err != nil {
			t.Fatalf("SweepOrphanDaemons returned error: %v", err)
		}
		if len(kill.calls) != 1 || kill.calls[0] != 20003 {
			t.Errorf("error: expected pid 20003 killed (legit empty); got %v", kill.calls)
		}
		warn := sink.Records().WithMessage(listPanesWarnMessage).AtExactLevel(slog.LevelWarn).Only(t, "list-panes-failure WARN")
		if got := warn.ErrorAttr(t, "error"); !errors.Is(got, sentinel) {
			t.Errorf("list-panes Warn error = %v, want %v", got, sentinel)
		}
	})
}

func TestSweepOrphanDaemons_neverReturnsError(t *testing.T) {
	cases := []struct {
		name string
		core *OrphanSweepCore
	}{
		{
			name: "pgrep error",
			core: &OrphanSweepCore{
				Pgrep:        func() ([]int, error) { return nil, errors.New("pgrep") },
				SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
				Identify:     func(pid int) (state.IdentifyResult, error) { return 0, nil },
				Kill:         func(pid int) error { return nil },
			},
		},
		{
			name: "list-panes error",
			core: &OrphanSweepCore{
				Pgrep:        func() ([]int, error) { return []int{1}, nil },
				SaverPanePID: func() (pid int, present bool, err error) { return 0, false, errors.New("list-panes") },
				Identify:     func(pid int) (state.IdentifyResult, error) { return state.IdentifyDead, nil },
				Kill:         func(pid int) error { return nil },
			},
		},
		{
			name: "identify error",
			core: &OrphanSweepCore{
				Pgrep:        func() ([]int, error) { return []int{1}, nil },
				SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
				Identify:     func(pid int) (state.IdentifyResult, error) { return 0, errors.New("identify") },
				Kill:         func(pid int) error { return nil },
			},
		},
		{
			name: "kill error",
			core: &OrphanSweepCore{
				Pgrep:        func() ([]int, error) { return []int{1}, nil },
				SaverPanePID: func() (pid int, present bool, err error) { return 0, false, nil },
				Identify:     func(pid int) (state.IdentifyResult, error) { return state.IdentifyIsPortalDaemon, nil },
				Kill:         func(pid int) error { return errors.New("kill") },
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.core.SweepOrphanDaemons(); err != nil {
				t.Errorf("expected nil err; got %v", err)
			}
		})
	}
}
