package tmuxtest

import (
	"fmt"
	"testing"
	"time"
)

// Defaults applied to a ProgressWait field left at its zero value.
const (
	DefaultProgressStall   = 6 * time.Second
	DefaultProgressCeiling = 60 * time.Second
	DefaultProgressTick    = 50 * time.Millisecond
)

// ProgressWait bounds an AwaitProgress call. Stall and Ceiling are separate
// budgets on purpose: a machine under load makes a converging system slower
// without making it wrong, so wall-clock alone must not decide the verdict.
type ProgressWait struct {
	// Stall is how long the observation may stay unchanged before the wait
	// gives up. Every change restarts it.
	Stall time.Duration
	// Ceiling bounds the whole wait, so an observation that keeps changing
	// without ever reaching the target still fails rather than hanging.
	Ceiling time.Duration
	// Tick is the interval between observations.
	Tick time.Duration
}

func (w ProgressWait) withDefaults() ProgressWait {
	if w.Stall <= 0 {
		w.Stall = DefaultProgressStall
	}
	if w.Ceiling <= 0 {
		w.Ceiling = DefaultProgressCeiling
	}
	if w.Tick <= 0 {
		w.Tick = DefaultProgressTick
	}
	return w
}

// ProgressResult reports how an AwaitProgress call ended. Last is the final
// observation, which a caller's failure message should quote so a red run says
// what the system was actually doing when the wait gave up.
type ProgressResult[T comparable] struct {
	Last    T
	Reached bool
	Stalled bool
	Elapsed time.Duration
	Changes int
}

func (r ProgressResult[T]) String() string {
	return fmt.Sprintf("last=%v reached=%v stalled=%v elapsed=%s changes=%d",
		r.Last, r.Reached, r.Stalled, r.Elapsed.Round(time.Millisecond), r.Changes)
}

// AwaitProgress polls observe at the wait's tick cadence until reached reports
// the target, the observation stops changing for the whole stall budget, or the
// absolute ceiling elapses — whichever comes first. It never fails the test
// itself; the caller decides what a not-Reached result means.
func AwaitProgress[T comparable](t *testing.T, w ProgressWait, observe func() T, reached func(T) bool) ProgressResult[T] {
	t.Helper()
	w = w.withDefaults()

	start := time.Now()
	ceiling := start.Add(w.Ceiling)
	stallDeadline := start.Add(w.Stall)

	var res ProgressResult[T]
	for first := true; ; first = false {
		observation := observe()
		if first || observation != res.Last {
			if !first {
				res.Changes++
			}
			res.Last = observation
			stallDeadline = time.Now().Add(w.Stall)
		}
		if reached(observation) {
			res.Reached = true
			res.Elapsed = time.Since(start)
			return res
		}
		now := time.Now()
		if now.After(ceiling) {
			res.Elapsed = time.Since(start)
			return res
		}
		if now.After(stallDeadline) {
			res.Stalled = true
			res.Elapsed = time.Since(start)
			return res
		}
		time.Sleep(w.Tick)
	}
}
