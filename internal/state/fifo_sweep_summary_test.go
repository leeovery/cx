package state_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
)

type fifoSummarySink struct {
	*logtest.Sink
}

func (s *fifoSummarySink) summariesFor(comp, msg string) []logtest.Record {
	var out []logtest.Record
	for _, r := range s.Records() {
		c, ok := r.Attrs["component"]
		if !ok || c.String() != comp || r.Msg != msg {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (s *fifoSummarySink) onlySummary(t *testing.T, comp, msg string) logtest.Record {
	t.Helper()
	sums := s.summariesFor(comp, msg)
	if len(sums) != 1 {
		t.Fatalf("expected exactly 1 %q %q summary, got %d: %+v", comp, msg, len(sums), s.Records())
	}
	return sums[0]
}

func (s *fifoSummarySink) matching(level slog.Level, comp, msg string) []logtest.Record {
	var out []logtest.Record
	for _, r := range s.Records() {
		if r.Level != level || r.Msg != msg {
			continue
		}
		c, ok := r.Attrs["component"]
		if !ok || c.String() != comp {
			continue
		}
		out = append(out, r)
	}
	return out
}

func installFIFOSummarySink(t *testing.T) *fifoSummarySink {
	t.Helper()
	sink := &fifoSummarySink{Sink: &logtest.Sink{}}
	log.SetTestHandler(t, sink.Sink)
	return sink
}

func TestSweepOrphanFIFOs_EmitsCleanSummaryCountingReapedAndSkipped(t *testing.T) {
	sink := installFIFOSummarySink(t)
	dir := t.TempDir()

	reapedA := filepath.Join(dir, "hydrate-a__0.0.fifo")
	reapedB := filepath.Join(dir, "hydrate-b__0.0.fifo")
	protected := filepath.Join(dir, "hydrate-keep__0.0.fifo")
	for _, p := range []string{reapedA, reapedB, protected} {
		if err := state.CreateFIFO(p); err != nil {
			t.Fatalf("create FIFO %s: %v", p, err)
		}
	}

	live := map[string]struct{}{"keep__0.0": {}}

	if err := state.SweepOrphanFIFOs(dir, live, log.For("bootstrap")); err != nil {
		t.Fatalf("SweepOrphanFIFOs: %v", err)
	}

	rec := sink.onlySummary(t, "clean", "orphan-fifo sweep complete")
	if rec.Level != slog.LevelInfo {
		t.Errorf("summary level = %v, want INFO", rec.Level)
	}
	if got := rec.IntAttr(t, "reaped"); got != 2 {
		t.Errorf("reaped = %d, want 2", got)
	}
	if got := rec.IntAttr(t, "skipped"); got != 1 {
		t.Errorf("skipped = %d, want 1 (live-marker-protected)", got)
	}
	rec.RequireDuration(t, "took")
}

func TestSweepOrphanFIFOs_EmitsZeroReapedZeroSkippedForMissingStateDir(t *testing.T) {
	sink := installFIFOSummarySink(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	if err := state.SweepOrphanFIFOs(missing, map[string]struct{}{}, log.For("bootstrap")); err != nil {
		t.Fatalf("SweepOrphanFIFOs: %v", err)
	}

	rec := sink.onlySummary(t, "clean", "orphan-fifo sweep complete")
	if got := rec.IntAttr(t, "reaped"); got != 0 {
		t.Errorf("reaped = %d, want 0 (loop runs zero times)", got)
	}
	if got := rec.IntAttr(t, "skipped"); got != 0 {
		t.Errorf("skipped = %d, want 0 (loop runs zero times)", got)
	}
	rec.RequireDuration(t, "took")
}

func TestSweepOrphanFIFOs_PreservedNonFIFOCountsAsSkipped(t *testing.T) {
	sink := installFIFOSummarySink(t)
	dir := t.TempDir()

	regular := filepath.Join(dir, "hydrate-foo__0.0.fifo")
	if err := os.WriteFile(regular, []byte("not a fifo"), 0o600); err != nil {
		t.Fatalf("seed regular file: %v", err)
	}

	if err := state.SweepOrphanFIFOs(dir, map[string]struct{}{}, log.For("bootstrap")); err != nil {
		t.Fatalf("SweepOrphanFIFOs: %v", err)
	}

	if info, err := os.Lstat(regular); err != nil {
		t.Fatalf("regular file removed by sweep: %v", err)
	} else if !info.Mode().IsRegular() {
		t.Errorf("file mode changed: got %v", info.Mode())
	}

	rec := sink.onlySummary(t, "clean", "orphan-fifo sweep complete")
	if got := rec.IntAttr(t, "reaped"); got != 0 {
		t.Errorf("reaped = %d, want 0", got)
	}
	if got := rec.IntAttr(t, "skipped"); got != 1 {
		t.Errorf("skipped = %d, want 1 (non-FIFO sibling)", got)
	}
}

func TestSweepOrphanFIFOs_RemoveFailureWarnsOnLoggerAndCountsAsSkipped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based EACCES setup is unix-specific")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses 0500 directory write protection")
	}

	sink := installFIFOSummarySink(t)
	dir := t.TempDir()

	a := filepath.Join(dir, "hydrate-a__0.0.fifo")
	b := filepath.Join(dir, "hydrate-b__0.0.fifo")
	if err := state.CreateFIFO(a); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if err := state.CreateFIFO(b); err != nil {
		t.Fatalf("create b: %v", err)
	}

	// Order matters: the FIFOs exist before the directory loses write permission.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := state.SweepOrphanFIFOs(dir, map[string]struct{}{}, log.For("bootstrap")); err != nil {
		t.Errorf("SweepOrphanFIFOs returned error: %v", err)
	}

	// Restore permissions so the temp-dir cleanup can remove files.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("restore chmod: %v", err)
	}

	warns := sink.matching(slog.LevelWarn, "bootstrap", "remove orphan fifo failed")
	if len(warns) != 2 {
		t.Fatalf("expected 2 remove-failure WARNs under bootstrap, got %d: %+v", len(warns), sink.Records())
	}
	for _, w := range warns {
		if _, ok := w.Attrs["error"]; !ok {
			t.Errorf("remove-failure WARN missing error attr: %+v", w.Attrs)
		}
		if _, ok := w.Attrs["path"]; !ok {
			t.Errorf("remove-failure WARN missing path attr: %+v", w.Attrs)
		}
	}

	rec := sink.onlySummary(t, "clean", "orphan-fifo sweep complete")
	if got := rec.IntAttr(t, "reaped"); got != 0 {
		t.Errorf("reaped = %d, want 0", got)
	}
	if got := rec.IntAttr(t, "skipped"); got != 2 {
		t.Errorf("skipped = %d, want 2 (both remove failures)", got)
	}
}

func TestSweepOrphanFIFOs_LiveMarkerProtectedCountsAsSkippedAndIsLeftInPlace(t *testing.T) {
	sink := installFIFOSummarySink(t)
	dir := t.TempDir()

	protected := filepath.Join(dir, "hydrate-keep__0.0.fifo")
	if err := state.CreateFIFO(protected); err != nil {
		t.Fatalf("create FIFO: %v", err)
	}

	live := map[string]struct{}{"keep__0.0": {}}

	if err := state.SweepOrphanFIFOs(dir, live, log.For("bootstrap")); err != nil {
		t.Fatalf("SweepOrphanFIFOs: %v", err)
	}

	if _, err := os.Lstat(protected); err != nil {
		t.Errorf("live-marker-protected FIFO removed: %v", err)
	}

	rec := sink.onlySummary(t, "clean", "orphan-fifo sweep complete")
	if got := rec.IntAttr(t, "reaped"); got != 0 {
		t.Errorf("reaped = %d, want 0", got)
	}
	if got := rec.IntAttr(t, "skipped"); got != 1 {
		t.Errorf("skipped = %d, want 1 (live-marker-protected)", got)
	}
}

func TestSweepOrphanFIFOs_DemotesPerRemovalInfoToDebugUnderClean(t *testing.T) {
	sink := installFIFOSummarySink(t)
	dir := t.TempDir()

	orphan := filepath.Join(dir, "hydrate-gone__0.0.fifo")
	if err := state.CreateFIFO(orphan); err != nil {
		t.Fatalf("create orphan: %v", err)
	}

	if err := state.SweepOrphanFIFOs(dir, map[string]struct{}{}, log.For("bootstrap")); err != nil {
		t.Fatalf("SweepOrphanFIFOs: %v", err)
	}

	for _, r := range sink.Records() {
		if r.Msg == "removed orphan fifo" {
			t.Errorf("old per-removal INFO message must be gone: %+v", r)
		}
	}

	dbg := sink.matching(slog.LevelDebug, "clean", "orphan fifo reaped")
	if len(dbg) != 1 {
		t.Fatalf("expected 1 DEBUG 'orphan fifo reaped' under clean, got %d: %+v", len(dbg), sink.Records())
	}
	if p, ok := dbg[0].Attrs["path"]; !ok || p.String() != orphan {
		t.Errorf("DEBUG 'orphan fifo reaped' path = %v, want %s", dbg[0].Attrs["path"], orphan)
	}
}

func TestSweepOrphanFIFOs_BoundaryContract_CallerWarnSinkVsCleanSummary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based EACCES setup is unix-specific")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses 0500 directory write protection")
	}

	sink := installFIFOSummarySink(t)
	dir := t.TempDir()

	orphan := filepath.Join(dir, "hydrate-gone__0.0.fifo")
	if err := state.CreateFIFO(orphan); err != nil {
		t.Fatalf("create orphan: %v", err)
	}

	// Make os.Remove fail, forcing the per-item WARN path.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	const callerComponent = "restore"
	if err := state.SweepOrphanFIFOs(dir, map[string]struct{}{}, log.For(callerComponent)); err != nil {
		t.Errorf("SweepOrphanFIFOs returned error: %v", err)
	}

	// Restore permissions so the temp-dir cleanup can remove files.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("restore chmod: %v", err)
	}

	warns := sink.matching(slog.LevelWarn, callerComponent, "remove orphan fifo failed")
	if len(warns) != 1 {
		t.Fatalf("expected 1 remove-failure WARN under %q, got %d: %+v", callerComponent, len(warns), sink.Records())
	}
	if cleanWarns := sink.matching(slog.LevelWarn, "clean", "remove orphan fifo failed"); len(cleanWarns) != 0 {
		t.Errorf("per-item WARN must NOT be attributed to clean, got %d: %+v", len(cleanWarns), cleanWarns)
	}

	rec := sink.onlySummary(t, "clean", "orphan-fifo sweep complete")
	if rec.Level != slog.LevelInfo {
		t.Errorf("summary level = %v, want INFO", rec.Level)
	}
	if sums := sink.summariesFor(callerComponent, "orphan-fifo sweep complete"); len(sums) != 0 {
		t.Errorf("summary must NOT be attributed to caller component %q, got %d: %+v", callerComponent, len(sums), sums)
	}
}

func TestSweepOrphanFIFOs_NoSummaryWhenGlobFails(t *testing.T) {
	sink := installFIFOSummarySink(t)
	// An unterminated "[" makes filepath.Glob report ErrBadPattern.
	badDir := filepath.Join(t.TempDir(), "[")

	if err := state.SweepOrphanFIFOs(badDir, map[string]struct{}{}, log.For("bootstrap")); err == nil {
		t.Fatalf("expected non-nil error from filepath.Glob failure")
	}

	if got := sink.summariesFor("clean", "orphan-fifo sweep complete"); len(got) != 0 {
		t.Errorf("expected no summary on glob failure (returns before loop), got %d: %+v", len(got), got)
	}
}
