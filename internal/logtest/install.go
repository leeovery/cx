package logtest

import (
	"testing"

	"github.com/leeovery/portal/internal/log"
)

// Install routes every component logger into a fresh Sink for the duration of
// t, restoring the prior handler on cleanup.
func Install(t *testing.T) *Sink {
	t.Helper()
	sink := &Sink{}
	log.SetTestHandler(t, sink)
	return sink
}
