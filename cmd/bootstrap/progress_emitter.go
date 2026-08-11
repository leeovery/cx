package bootstrap

// The emitter rides the context rather than an Orchestrator field, so Run's
// signature is untouched and the synchronous route simply resolves it to nil.

import "context"

// StepEvent is the per-step progress signal streamed on the concurrent
// cold-boot route. Index is the 1-based step number and Name its StepName.
// RestoreN and RestoreM are zero except on the restore step's per-session
// events, which set RestoreM > 0 and RestoreN in 1..M.
type StepEvent struct {
	Index    int
	Name     string
	RestoreN int
	RestoreM int
}

// ProgressEmitter is invoked once per completed bootstrap step. It must not
// block the orchestrator.
type ProgressEmitter func(StepEvent)

type progressEmitterKey struct{}

// WithProgressEmitter returns a context carrying the emitter so Run streams a
// StepEvent per completed step.
func WithProgressEmitter(ctx context.Context, emit ProgressEmitter) context.Context {
	return context.WithValue(ctx, progressEmitterKey{}, emit)
}

func progressEmitterFromContext(ctx context.Context) ProgressEmitter {
	if ctx == nil {
		return nil
	}
	emit, _ := ctx.Value(progressEmitterKey{}).(ProgressEmitter)
	return emit
}

// ProgressEmitterFromContextForTest exposes the context read to cmd-layer
// tests. Production code uses WithProgressEmitter plus Run.
func ProgressEmitterFromContextForTest(ctx context.Context) ProgressEmitter {
	return progressEmitterFromContext(ctx)
}
