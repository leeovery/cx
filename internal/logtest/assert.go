package logtest

import "log/slog"

// RecordWant is the shape every audit-trail record shares: its level, its
// message, the component it was emitted under, and the op and via attrs. The
// message and the op attr are separate fields because they are separate parts
// of the line's contract, even where an emission sets both to the same string.
type RecordWant struct {
	Level     slog.Level
	Msg       string
	Component string
	Op        string
	Via       string
}

// AssertRecord checks the five shared properties, reporting each mismatch
// separately. The attrs that belong to one emission — a stand-down's reason, a
// failure's error — stay with their own caller.
func AssertRecord(t TestingT, rec Record, want RecordWant) {
	t.Helper()
	if rec.Level != want.Level {
		t.Errorf("level = %v, want %v", rec.Level, want.Level)
	}
	if rec.Msg != want.Msg {
		t.Errorf("message = %q, want %q", rec.Msg, want.Msg)
	}
	if got := rec.AttrString(t, "component"); got != want.Component {
		t.Errorf("component = %q, want %q", got, want.Component)
	}
	if got := rec.AttrString(t, "op"); got != want.Op {
		t.Errorf("op = %q, want %q", got, want.Op)
	}
	if got := rec.AttrString(t, "via"); got != want.Via {
		t.Errorf("via = %q, want %q", got, want.Via)
	}
}
