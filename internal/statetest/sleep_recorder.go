// Package statetest holds shared test helpers for exercising the seam-bearing
// APIs of internal/state. Production code must not import it. Every recording
// helper is single-goroutine only and omits synchronisation.
package statetest

import "time"

// RecordingSleep records every duration handed to its Fn seam, so a test can
// assert a retry ladder's shape without waiting on real time.
type RecordingSleep struct {
	Durations []time.Duration
}

// Fn returns a closure appending each invocation's duration to r.Durations.
func (r *RecordingSleep) Fn() func(time.Duration) {
	return func(d time.Duration) { r.Durations = append(r.Durations, d) }
}
