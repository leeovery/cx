// The hook-staleness cycle's own test vocabulary: the seam fake the cycle
// reads through, the enumeration rows it answers with, and the hooks.json
// bodies a fixture stages. The seed keys themselves are named once in
// internal/hookstest, so no suite can re-point a name at a different key.
package hooksweep

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
)

// hooksBody renders a hooks.json body registering one on-resume entry per key,
// so a fixture that cares only about which keys are present says exactly that.
// With no keys it renders the empty registry: a file that exists and holds
// nothing.
func hooksBody(keys ...string) string {
	if len(keys) == 0 {
		return "{}"
	}
	entries := make([]string, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, fmt.Sprintf("  %q: {\"on-resume\": \"echo hi\"}", key))
	}
	return "{\n" + strings.Join(entries, ",\n") + "\n}"
}

// tokenRows models the enumeration's answer for stamped panes, and
// unstampedRows for panes carrying no token. The location half is display-only,
// so these fabricate a distinct one per row rather than asserting on it.
func tokenRows(tokens ...string) []tmux.PaneHookRow {
	rows := make([]tmux.PaneHookRow, 0, len(tokens))
	for i, token := range tokens {
		rows = append(rows, tmux.PaneHookRow{Token: token, Location: fmt.Sprintf("stamped%d:0.0", i)})
	}
	return rows
}

func unstampedRows(n int) []tmux.PaneHookRow {
	rows := make([]tmux.PaneHookRow, 0, n)
	for i := range n {
		rows = append(rows, tmux.PaneHookRow{Location: fmt.Sprintf("bare%d:0.0", i)})
	}
	return rows
}

// stubReader answers the cycle's two seams with fixed values: a fixed row set
// (or failure) for the pane-token enumeration, and a fixed @portal-restoring
// read. It counts the enumeration reads, so a test can assert the cycle stood
// down before enumerating as well as what it read. The optional during hook
// runs at the top of the enumeration, so a test can land a concurrent writer's
// mutation inside it — in the window between the cycle's snapshot and the token
// set that snapshot is weighed against.
type stubReader struct {
	rows         []tmux.PaneHookRow
	err          error
	restoring    bool
	restoringErr error
	during       func()
	calls        int
}

func (s *stubReader) ListAllPaneHookKeys() ([]tmux.PaneHookRow, error) {
	s.calls++
	if s.during != nil {
		s.during()
	}
	return s.rows, s.err
}

func (s *stubReader) TryGetServerOption(string) (string, bool, error) {
	switch {
	case s.restoringErr != nil:
		return "", false, s.restoringErr
	case !s.restoring:
		return "", false, nil
	}
	return "1", true, nil
}

var _ Reader = (*stubReader)(nil)

// keysOf names the entries a loaded store holds, for a failure message that
// says which keys survived a cycle rather than dumping the whole file.
func keysOf(snapshot hooks.Snapshot) []string {
	keys := make([]string, 0, len(snapshot))
	for key := range snapshot {
		keys = append(keys, key)
	}
	return keys
}

// runErr drives one cycle and keeps only its error, for the cases that assert
// on hooks.json or on the log rather than on what the cycle reported.
func runErr(reader Reader, store *hooks.Store) error {
	_, err := Run(reader, store)
	return err
}

// assertStandDown pins the stand-down breadcrumb at the level the cycle is
// expected to report it, and returns it for per-case attr assertions. The level
// picks the record set the count is taken over: a DEBUG stand-down is the only
// line the sink holds, so anything at WARN or above is itself a failure, while a
// WARN stand-down shares the sink with the degraded pre-read's own DEBUG
// breadcrumb and can only be counted among the WARNs.
func assertStandDown(t *testing.T, sink *logtest.Sink, level slog.Level, reason Reason) logtest.Record {
	t.Helper()

	var rec logtest.Record
	if level < slog.LevelWarn {
		for _, r := range sink.Records().AtOrAboveLevel(slog.LevelWarn) {
			t.Errorf("stand-down emitted at %v: %+v", r.Level, r)
		}
		rec = sink.Records().Only(t, "log record")
	} else {
		rec = sink.Records().AtOrAboveLevel(level).Only(t, "record at or above level")
	}

	logtest.AssertRecord(t, rec, logtest.RecordWant{
		Level:     level,
		Msg:       standDownMsg,
		Component: "hooks",
		Op:        standDownMsg,
		Via:       "internal",
	})
	if got := rec.AttrString(t, "reason"); got != string(reason) {
		t.Errorf("reason = %q, want %q", got, reason)
	}
	return rec
}

// lockBound is the lowered sidecar timeout a contention fixture runs at: long
// enough to be a real wait, short enough that a suite can afford one.
const lockBound = 60 * time.Millisecond

// errTmuxDead is the transient tmux failure a seam is armed with when the case
// is about what the cycle does with a failed read, not about which read failed.
var errTmuxDead = errors.New("tmux dead")

// dirListing is the directory's entry names, so a test can assert a cycle that
// had nothing to do added nothing to it.
func dirListing(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
