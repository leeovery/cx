package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/tui"
)

type fatalAfterRunner struct {
	emitSteps int
	started   bool
	err       error
}

func (r *fatalAfterRunner) Run(ctx context.Context) (bool, []bootstrap.Warning, error) {
	emit := bootstrap.ProgressEmitterFromContextForTest(ctx)
	for i := 1; i <= r.emitSteps; i++ {
		if emit != nil {
			emit(bootstrap.StepEvent{Index: i, Name: "step"})
		}
	}
	return r.started, nil, r.err
}

func TestBootstrapProgressPipe_FatalMapsToFatalMsg(t *testing.T) {
	cause := errors.New("permission denied")
	fatal := bootstrap.NewFatal("Portal failed to set @portal-restoring marker: permission denied", cause)
	runner := &fatalAfterRunner{emitSteps: 2, started: true, err: fatal}
	pipe := newBootstrapProgressPipe()
	pipe.start(context.Background(), runner)

	msgs := drainPipe(t, pipe.receiver())

	var fatalMsg *tui.BootstrapFatalMsg
	for _, m := range msgs {
		if _, ok := m.(tui.BootstrapCompleteMsg); ok {
			t.Error("fatal run produced a BootstrapCompleteMsg; want a BootstrapFatalMsg")
		}
		if fm, ok := m.(tui.BootstrapFatalMsg); ok {
			fmCopy := fm
			fatalMsg = &fmCopy
		}
	}
	if fatalMsg == nil {
		t.Fatal("fatal run never produced a tui.BootstrapFatalMsg")
	}
	if fatalMsg.FailedStep != 3 {
		t.Errorf("BootstrapFatalMsg.FailedStep = %d, want 3 (the aborting step)", fatalMsg.FailedStep)
	}
	if fatalMsg.Message != fatal.UserMessage {
		t.Errorf("BootstrapFatalMsg.Message = %q, want %q (FatalError.UserMessage)", fatalMsg.Message, fatal.UserMessage)
	}
	if !errors.Is(fatalMsg.Err, cause) {
		t.Errorf("BootstrapFatalMsg.Err did not carry the fatal cause; got %v", fatalMsg.Err)
	}
	var asFatal *bootstrap.FatalError
	if !errors.As(fatalMsg.Err, &asFatal) {
		t.Error("BootstrapFatalMsg.Err is not a *bootstrap.FatalError (exit classification would miss it)")
	}
}

func TestBootstrapProgressPipe_FatalAtStep1(t *testing.T) {
	fatal := bootstrap.NewFatal("Portal failed to start tmux server: boom", errors.New("boom"))
	runner := &fatalAfterRunner{emitSteps: 0, started: false, err: fatal}
	pipe := newBootstrapProgressPipe()
	pipe.start(context.Background(), runner)

	msgs := drainPipe(t, pipe.receiver())

	var fatalMsg *tui.BootstrapFatalMsg
	for _, m := range msgs {
		if fm, ok := m.(tui.BootstrapFatalMsg); ok {
			fmCopy := fm
			fatalMsg = &fmCopy
		}
	}
	if fatalMsg == nil {
		t.Fatal("fatal-at-step-1 run never produced a tui.BootstrapFatalMsg")
	}
	if fatalMsg.FailedStep != 1 {
		t.Errorf("BootstrapFatalMsg.FailedStep = %d, want 1 (EnsureServer)", fatalMsg.FailedStep)
	}
}

func TestBootstrapProgressPipe_NonFatalStillCompletes(t *testing.T) {
	runner := &emittingRunner{steps: 11, started: true}
	pipe := newBootstrapProgressPipe()
	pipe.start(context.Background(), runner)

	msgs := drainPipe(t, pipe.receiver())

	var sawComplete, sawFatal bool
	for _, m := range msgs {
		switch m.(type) {
		case tui.BootstrapCompleteMsg:
			sawComplete = true
		case tui.BootstrapFatalMsg:
			sawFatal = true
		}
	}
	if !sawComplete {
		t.Error("successful run did not produce a BootstrapCompleteMsg")
	}
	if sawFatal {
		t.Error("successful run produced a BootstrapFatalMsg; the fatal path leaked")
	}
}
