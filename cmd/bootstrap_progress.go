package cmd

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/tui"
)

const bootstrapProgressBufferSize = 64

// bootstrapProgress is the event shape carried on the progress channel. Exactly
// one terminal event (Done) is sent, last, before the channel closes.
type bootstrapProgress struct {
	Step bootstrap.StepEvent

	RestoreN int
	RestoreM int

	Done          bool
	ServerStarted bool
	Warnings      []bootstrap.Warning
	Fatal         error

	// FailedStep is the last emitted step index + 1: a fatal step emits no
	// step-complete event for itself and nothing after, so the next un-emitted
	// index is the one that failed. Zero on a successful run.
	FailedStep int
}

// bootstrapChannelClosedMsg is the receiver's reply once the goroutine has
// closed the channel, so a post-close re-issue returns instead of blocking.
type bootstrapChannelClosedMsg struct{}

type bootstrapProgressPipe struct {
	ch chan bootstrapProgress

	// Written by the goroutine before it closes the channel and read only after
	// the program exits, so the close the receiver observed is the ordering edge
	// and no further synchronisation is needed.
	serverStarted bool
	warnings      []bootstrap.Warning
	err           error
}

func newBootstrapProgressPipe() *bootstrapProgressPipe {
	return &bootstrapProgressPipe{
		ch: make(chan bootstrapProgress, bootstrapProgressBufferSize),
	}
}

// start launches the orchestrator goroutine, sends exactly one terminal Done
// event, and closes the channel on success or fatal alike, so the receiver
// always observes a close and never leaks a blocked receive.
func (p *bootstrapProgressPipe) start(ctx context.Context, runner bootstrap.Runner) {
	// Written and read only on the orchestrator goroutine — the emitter runs
	// synchronously inside Run — so it needs no synchronisation.
	lastStep := 0
	emitCtx := bootstrap.WithProgressEmitter(ctx, func(ev bootstrap.StepEvent) {
		if ev.Index > lastStep {
			lastStep = ev.Index
		}
		p.send(ctx, bootstrapProgress{
			Step:     ev,
			RestoreN: ev.RestoreN,
			RestoreM: ev.RestoreM,
		})
	})

	go func() {
		defer close(p.ch)
		started, warnings, err := runner.Run(emitCtx)
		p.serverStarted = started
		p.warnings = warnings
		p.err = err
		failedStep := 0
		if err != nil {
			failedStep = lastStep + 1
		}
		p.send(ctx, bootstrapProgress{
			Done:          true,
			ServerStarted: started,
			Warnings:      warnings,
			Fatal:         err,
			FailedStep:    failedStep,
		})
	}()
}

// send abandons the event if ctx is cancelled. Without the guard, a restore
// burst larger than the buffer against a TUI that stopped draining (early Quit)
// would wedge the orchestrator goroutine on a full channel forever; the drop is
// benign because nothing is consuming.
func (p *bootstrapProgressPipe) send(ctx context.Context, ev bootstrapProgress) {
	select {
	case p.ch <- ev:
	case <-ctx.Done():
	}
}

// receiver returns the tea.Cmd the loading-page model blocks on: a single
// blocking receive, re-issued per event, which preserves exact event order even
// under command batching.
func (p *bootstrapProgressPipe) receiver() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-p.ch
		if !ok {
			return bootstrapChannelClosedMsg{}
		}
		if ev.Done {
			// A fatal must not become a BootstrapCompleteMsg, which would dismiss the
			// loading page into the picker.
			if ev.Fatal != nil {
				return fatalMsgFromEvent(ev)
			}
			return tui.BootstrapCompleteMsg{Warnings: ev.Warnings}
		}
		// Only the step index rides the wire; the friendly label is derived
		// consumer-side, which is the sole authority for that mapping.
		return tui.BootstrapProgressMsg{
			Index:    ev.Step.Index,
			RestoreN: ev.RestoreN,
			RestoreM: ev.RestoreM,
		}
	}
}

// fatalMsgFromEvent carries the underlying error through on Err so the caller
// can errors.As it back for exit classification. The err.Error() branch is
// defensive: the orchestrator always wraps fatals.
func fatalMsgFromEvent(ev bootstrapProgress) tui.BootstrapFatalMsg {
	message := ev.Fatal.Error()
	var fatal *bootstrap.FatalError
	if errors.As(ev.Fatal, &fatal) {
		message = fatal.UserMessage
	}
	return tui.BootstrapFatalMsg{
		FailedStep: ev.FailedStep,
		Message:    message,
		Err:        ev.Fatal,
	}
}

// ServerStarted reports the orchestrator's serverStarted return. Read after the
// program exits.
func (p *bootstrapProgressPipe) ServerStarted() bool { return p.serverStarted }

// Warnings reports the soft warnings the orchestrator accumulated. Read after
// the program exits.
func (p *bootstrapProgressPipe) Warnings() []bootstrap.Warning { return p.warnings }

// Err reports the orchestrator's fatal error, if any. Read after the program
// exits.
func (p *bootstrapProgressPipe) Err() error { return p.err }
