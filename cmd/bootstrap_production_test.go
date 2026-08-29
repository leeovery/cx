package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/transienttest"
)

var _ bootstrap.LatchWriter = (*tmux.Client)(nil)

type recordedLog struct {
	level     string
	component string
	message   string
	attrs     map[string]slog.Value
}

// intAttr fails the test rather than returning a zero for an absent attr: an
// assertion on a value that was never emitted must not read as a mismatch.
func (r recordedLog) intAttr(t *testing.T, key string) int64 {
	t.Helper()
	v, ok := r.attrs[key]
	if !ok {
		t.Fatalf("record %q carries no %q attr; attrs=%v", r.message, key, r.attrs)
	}
	return v.Int64()
}

// WithAttrs replays the bound attrs onto each record, so the captured component
// is populated even though production binds it at the logger, not the call site.
type recordingLogger struct {
	entries []recordedLog
	// shared points at the entries-owning recorder so derived handlers record
	// back into the same slice; nil on the root.
	shared *recordingLogger
	bound  []slog.Attr
}

func (r *recordingLogger) Logger() *slog.Logger { return slog.New(r) }

func (r *recordingLogger) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (r *recordingLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Attr, 0, len(r.bound)+len(attrs))
	next = append(next, r.bound...)
	next = append(next, attrs...)
	return &recordingLogger{shared: r.owner(), bound: next}
}

func (r *recordingLogger) WithGroup(_ string) slog.Handler {
	return &recordingLogger{shared: r.owner(), bound: r.bound}
}

func (r *recordingLogger) owner() *recordingLogger {
	if r.shared != nil {
		return r.shared
	}
	return r
}

func (r *recordingLogger) Handle(_ context.Context, rec slog.Record) error {
	component := ""
	attrs := map[string]slog.Value{}
	read := func(a slog.Attr) bool {
		if a.Key == "component" {
			component = a.Value.String()
		}
		attrs[a.Key] = a.Value
		return true
	}
	for _, a := range r.bound {
		read(a)
	}
	rec.Attrs(read)
	var level string
	switch rec.Level {
	case slog.LevelDebug:
		level = "debug"
	case slog.LevelInfo:
		level = "info"
	case slog.LevelWarn:
		level = "warn"
	case slog.LevelError:
		level = "error"
	}
	owner := r.owner()
	owner.entries = append(owner.entries, recordedLog{level, component, rec.Message, attrs})
	return nil
}

var _ slog.Handler = (*recordingLogger)(nil)

type stubAllPaneLister struct {
	rows         []tmux.PaneHookRow
	err          error
	restoring    bool
	restoringErr error
}

func (s *stubAllPaneLister) ListAllPaneHookKeys() ([]tmux.PaneHookRow, error) {
	return s.rows, s.err
}

func (s *stubAllPaneLister) TryGetServerOption(string) (string, bool, error) {
	return restoringOption(s.restoring, s.restoringErr)
}

var _ AllPaneLister = (*tmux.Client)(nil)

func newTempHooksStore(t *testing.T, seed string) (*hooks.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.json")
	if seed != "" {
		if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
			t.Fatalf("write seed hooks.json: %v", err)
		}
	}
	// The sidecar stands in for the one a writer establishes on a real install,
	// so a read under this fixture takes its shared lock rather than degrading
	// and emitting a load-unlocked breadcrumb the fixture never meant to model.
	transienttest.CreateHooksSidecar(t, path)
	return hooks.NewStore(path), path
}

// readFileBytes returns nil on ENOENT, so callers can distinguish "file absent"
// from "file empty".
func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func countMatching(entries []recordedLog, level, component, message string) int {
	n := 0
	for _, e := range entries {
		if e.level == level && e.component == component && e.message == message {
			n++
		}
	}
	return n
}

// onlyMatching returns the single record matching the triple, failing on none
// or several so a per-attr assertion below it cannot silently read the wrong
// record.
func onlyMatching(t *testing.T, entries []recordedLog, level, component, message string) recordedLog {
	t.Helper()
	var found []recordedLog
	for _, e := range entries {
		if e.level == level && e.component == component && e.message == message {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("records matching %s/%s/%q = %d, want 1; entries=%+v", level, component, message, len(found), entries)
	}
	return found[0]
}

func keysOf(m map[string]map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
