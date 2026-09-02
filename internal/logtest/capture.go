// Package logtest provides an in-process log-capturing slog.Handler. It is
// test-only: production code must not import it.
//
// Sink renders each record as
//
//	<LEVEL> <msg> key=value...
//
// with any bound component on every line. Consumers' substring assertions key on
// that rendering, so it is declared here rather than per consumer.
package logtest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestingT is the subset of *testing.T the failing-path accessors depend on, so
// their own failure paths can be unit-tested without aborting the harness.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Record is a flattened view of one captured slog.Record. Keys holds the attr
// keys in order, including those bound via WithAttrs; Attrs maps the same keys
// to values, last-write-wins on a duplicate.
type Record struct {
	Level slog.Level
	Msg   string
	Keys  []string
	Attrs map[string]slog.Value
}

// AttrString fails the test if the record carries no attr named key.
func (r Record) AttrString(t TestingT, key string) string {
	t.Helper()
	v, ok := r.Attrs[key]
	if !ok {
		t.Fatalf("record missing attr %q: %+v", key, r.Attrs)
	}
	return v.String()
}

// AttrOrEmpty returns the rendered value of the named attr, or the empty string
// when the record carries none. It is the non-fatal counterpart of AttrString,
// for a caller whose assertion covers the absent case itself.
func (r Record) AttrOrEmpty(key string) string {
	v, ok := r.Attrs[key]
	if !ok {
		return ""
	}
	return v.String()
}

// IntAttr fails the test if the attr is absent or is not a slog.KindInt64 value.
func (r Record) IntAttr(t TestingT, key string) int64 {
	t.Helper()
	v, ok := r.Attrs[key]
	if !ok {
		t.Fatalf("record missing attr %q: %+v", key, r.Attrs)
	}
	if v.Kind() != slog.KindInt64 {
		t.Fatalf("attr %q kind = %v, want Int64: %+v", key, v.Kind(), v)
	}
	return v.Int64()
}

// ErrorAttr fails the test if the attr is absent or carries a non-error value.
func (r Record) ErrorAttr(t TestingT, key string) error {
	t.Helper()
	v, ok := r.Attrs[key]
	if !ok {
		t.Fatalf("record missing attr %q: %+v", key, r.Attrs)
	}
	err, ok := v.Any().(error)
	if !ok {
		t.Fatalf("attr %q = %+v, want an error value: %+v", key, v, r.Attrs)
	}
	return err
}

// DurationAttr fails the test if the attr is absent or is not a
// slog.KindDuration value. It checks the kind, not the rendering: a rendered
// Duration is indistinguishable from a stringified one.
func (r Record) DurationAttr(t TestingT, key string) time.Duration {
	t.Helper()
	v, ok := r.Attrs[key]
	if !ok {
		t.Fatalf("record missing attr %q: %+v", key, r.Attrs)
	}
	if v.Kind() != slog.KindDuration {
		t.Fatalf("attr %q kind = %v, want Duration", key, v.Kind())
	}
	return v.Duration()
}

// RequireDuration asserts the attr's kind for a caller the value itself does not
// concern — a measured elapsed time no assertion can pin.
func (r Record) RequireDuration(t TestingT, key string) {
	t.Helper()
	_ = r.DurationAttr(t, key)
}

func (r Record) HasAttr(key string) bool {
	_, ok := r.Attrs[key]
	return ok
}

// Matches reports whether the record was emitted under component carrying
// message msg. It is the single home of that predicate: the Sink queries are
// built on it, and a caller walking records in capture order (to assert one
// event preceded another) uses it rather than restating the rule.
func (r Record) Matches(component, msg string) bool {
	if r.Msg != msg {
		return false
	}
	c, ok := r.Attrs["component"]
	return ok && c.String() == component
}

// Records is a captured slice of records. Every query returns a new slice and
// leaves the receiver untouched; nil when none match. The filters themselves are
// unexported: Sink exposes exactly one method per query, so a caller never has
// two ways to ask the same question.
type Records []Record

// atExactLevel keeps only the records logged at exactly level — a record one
// level higher does not match. atOrAboveLevel is the threshold filter.
func (rs Records) atExactLevel(level slog.Level) Records {
	return rs.filter(func(r Record) bool { return r.Level == level })
}

// atOrAboveLevel keeps the records logged at minLevel or any higher level.
func (rs Records) atOrAboveLevel(minLevel slog.Level) Records {
	return rs.filter(func(r Record) bool { return r.Level >= minLevel })
}

// withMessage keeps the records carrying message msg, whatever component
// emitted them.
func (rs Records) withMessage(msg string) Records {
	return rs.filter(func(r Record) bool { return r.Msg == msg })
}

// matching keeps the records emitted under component carrying message msg.
func (rs Records) matching(component, msg string) Records {
	return rs.filter(func(r Record) bool { return r.Matches(component, msg) })
}

// Only fails the test unless the set holds exactly one record, and returns it.
// It terminates any query chain, the level-filtered ones included: the level
// survives into the returned record. The description names the set in the
// failure message.
func (rs Records) Only(t TestingT, description string) Record {
	t.Helper()
	if len(rs) != 1 {
		t.Fatalf("expected exactly 1 %s, got %d: %+v", description, len(rs), rs)
	}
	return rs[0]
}

func (rs Records) filter(keep func(Record) bool) Records {
	var out Records
	for _, r := range rs {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

// Sink captures every record into an in-memory buffer, shared with the handlers
// derived from it via WithAttrs/WithGroup. The zero value is ready to use.
type Sink struct {
	mu      sync.Mutex
	lines   []string
	records []Record
	shared  *Sink
	bound   []slog.Attr
}

func (s *Sink) owner() *Sink {
	if s.shared != nil {
		return s.shared
	}
	return s
}

// Enabled admits every level: filtering is internal/log's concern, and these
// tests assert that a line was emitted at a given level.
func (s *Sink) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (s *Sink) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Attr, 0, len(s.bound)+len(attrs))
	next = append(next, s.bound...)
	next = append(next, attrs...)
	return &Sink{shared: s.owner(), bound: next}
}

func (s *Sink) WithGroup(_ string) slog.Handler {
	return &Sink{shared: s.owner(), bound: s.bound}
}

func (s *Sink) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Level.String())
	b.WriteString(" ")
	b.WriteString(r.Message)
	keys := make([]string, 0, len(s.bound)+r.NumAttrs())
	attrs := make(map[string]slog.Value, len(s.bound)+r.NumAttrs())
	for _, a := range s.bound {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value.Any())
		keys = append(keys, a.Key)
		attrs[a.Key] = a.Value
	}
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value.Any())
		keys = append(keys, a.Key)
		attrs[a.Key] = a.Value
		return true
	})
	owner := s.owner()
	owner.mu.Lock()
	owner.lines = append(owner.lines, b.String())
	owner.records = append(owner.records, Record{Level: r.Level, Msg: r.Message, Keys: keys, Attrs: attrs})
	owner.mu.Unlock()
	return nil
}

func (s *Sink) Body() string {
	owner := s.owner()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return strings.Join(owner.lines, "\n")
}

// Lines returns a snapshot: later writes do not mutate it.
func (s *Sink) Lines() []string {
	owner := s.owner()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return append([]string(nil), owner.lines...)
}

// Records returns a snapshot: later writes do not mutate it.
func (s *Sink) Records() Records {
	owner := s.owner()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return append(Records(nil), owner.records...)
}

// RecordsAtExactLevel returns the records captured at exactly level.
func (s *Sink) RecordsAtExactLevel(level slog.Level) Records {
	return s.Records().atExactLevel(level)
}

// RecordsAtOrAboveLevel returns the records captured at minLevel or above.
func (s *Sink) RecordsAtOrAboveLevel(minLevel slog.Level) Records {
	return s.Records().atOrAboveLevel(minLevel)
}

// RecordsWith returns the records emitted under component carrying message msg.
func (s *Sink) RecordsWith(component, msg string) Records {
	return s.Records().matching(component, msg)
}

// RecordsWithMessage returns the records carrying message msg under any
// component.
func (s *Sink) RecordsWithMessage(msg string) Records {
	return s.Records().withMessage(msg)
}

// RecordsAtExactLevelWith returns the records emitted under component carrying
// message msg at exactly level.
func (s *Sink) RecordsAtExactLevelWith(level slog.Level, component, msg string) Records {
	return s.RecordsWith(component, msg).atExactLevel(level)
}

// RecordsAtExactLevelWithMessage returns the records carrying message msg at
// exactly level, whatever component emitted them — so a caller can assert the
// component separately rather than filtering it away.
func (s *Sink) RecordsAtExactLevelWithMessage(level slog.Level, msg string) Records {
	return s.RecordsWithMessage(msg).atExactLevel(level)
}

func NewCaptureLogger(t *testing.T) (*slog.Logger, *Sink) {
	t.Helper()
	sink := &Sink{}
	return slog.New(sink), sink
}
