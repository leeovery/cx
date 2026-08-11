package restore_test

import (
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
)

type captureSink struct {
	*logtest.Sink
}

type capturedRecord struct {
	level slog.Level
	msg   string
	keys  []string
}

func (s *captureSink) recordsWithMessage(msg string) []capturedRecord {
	var out []capturedRecord
	for _, r := range s.Records() {
		if r.Msg == msg {
			out = append(out, capturedRecord{level: r.Level, msg: r.Msg, keys: r.Keys})
		}
	}
	return out
}

func newCaptureLogger(t *testing.T) (*slog.Logger, *captureSink) {
	t.Helper()
	sink := &captureSink{Sink: &logtest.Sink{}}
	return slog.New(sink), sink
}
