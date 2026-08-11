package cmd

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
)

func replayCfg(t *testing.T, fifo, scrollback, hookKey string, stdout io.Writer, exec func(string, []string), logger *slog.Logger) hydrateConfig {
	t.Helper()
	return hydrateConfig{
		FIFO:              fifo,
		File:              scrollback,
		HookKey:           hookKey,
		Stdout:            stdout,
		Client:            tmux.NewClient(&recordingCommander{}),
		Logger:            logger,
		ExecShell:         exec,
		OpenFIFO:          openFIFOWithTimeout,
		HandleFileMissing: handleHydrateFileMissing,
		HandleTimeout:     handleHydrateTimeout,
	}
}

func TestHydrateReplayedLog_EmitsScrollbackReplayedBytesTookOnSuccessPath(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-rep__0.0.fifo")
	scrollback := filepath.Join(dir, "sb")
	payload := []byte("line1\nline2\nline3\n")
	if err := os.WriteFile(scrollback, payload, 0o600); err != nil {
		t.Fatalf("seed scrollback: %v", err)
	}

	signalFIFOAsync(t, fifo)

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := replayCfg(t, fifo, scrollback, "rep:0.0", io.Discard, (&stubExecShell{}).fn(), logger)

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	info := execLogLine(t, sink.Body(), "INFO", "scrollback replayed")
	if !strings.Contains(info, fmt.Sprintf("bytes=%d", len(payload))) {
		t.Errorf("scrollback replayed INFO missing bytes=%d: %q", len(payload), info)
	}
	if !strings.Contains(info, "took=") {
		t.Errorf("scrollback replayed INFO missing took=: %q", info)
	}
}

func TestHydrateReplayedLog_BytesEqualsCopyCountForPopulatedFile(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-pop__0.0.fifo")
	scrollback := filepath.Join(dir, "sb")
	// NUL, non-UTF8 and escape bytes so the count is verbatim byte length rather
	// than a rune count.
	payload := []byte("line1\r\nline2\x00\xff\x1b[31mred\x1b[0m")
	if err := os.WriteFile(scrollback, payload, 0o600); err != nil {
		t.Fatalf("seed scrollback: %v", err)
	}

	signalFIFOAsync(t, fifo)

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := replayCfg(t, fifo, scrollback, "pop:0.0", io.Discard, (&stubExecShell{}).fn(), logger)

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	info := execLogLine(t, sink.Body(), "INFO", "scrollback replayed")
	if !strings.Contains(info, fmt.Sprintf("bytes=%d", len(payload))) {
		t.Errorf("bytes must equal io.Copy count (%d): %q", len(payload), info)
	}

	// Read as a structured int attr: the rendered bytes=N is indistinguishable
	// from a stringified count.
	rec := scrollbackReplayedRecord(t, sink)
	if got := rec.IntAttr(t, "bytes"); got != int64(len(payload)) {
		t.Errorf("bytes attr = %d, want %d", got, len(payload))
	}
}

func TestHydrateReplayedLog_ZeroByteScrollbackEmitsBytesZero(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-zero__0.0.fifo")
	scrollback := filepath.Join(dir, "sb")
	if err := os.WriteFile(scrollback, []byte(""), 0o600); err != nil {
		t.Fatalf("seed empty scrollback: %v", err)
	}

	signalFIFOAsync(t, fifo)

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := replayCfg(t, fifo, scrollback, "zero:0.0", io.Discard, (&stubExecShell{}).fn(), logger)

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	info := execLogLine(t, sink.Body(), "INFO", "scrollback replayed")
	if !strings.Contains(info, "bytes=0") {
		t.Errorf("zero-byte scrollback must emit bytes=0: %q", info)
	}
}

func TestHydrateReplayedLog_FiveMegabyteFileReportsExactByteCount(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-big__0.0.fifo")
	scrollback := filepath.Join(dir, "sb")
	const size = 5 * 1024 * 1024
	payload := bytes.Repeat([]byte("A"), size)
	if err := os.WriteFile(scrollback, payload, 0o600); err != nil {
		t.Fatalf("seed 5MB scrollback: %v", err)
	}

	signalFIFOAsync(t, fifo)

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := replayCfg(t, fifo, scrollback, "big:0.0", io.Discard, (&stubExecShell{}).fn(), logger)

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	info := execLogLine(t, sink.Body(), "INFO", "scrollback replayed")
	if !strings.Contains(info, fmt.Sprintf("bytes=%d", size)) {
		t.Errorf("5MB file must report exact byte count bytes=%d: %q", size, info)
	}
}

// scrollbackReplayedRecord returns the single hydrate "scrollback replayed"
// record. The structured form is what lets a test assert the took attr's Kind,
// which substring rendering cannot distinguish from a stringified duration.
func scrollbackReplayedRecord(t *testing.T, sink *logtest.Sink) logtest.Record {
	t.Helper()
	var out []logtest.Record
	for _, r := range sink.Records() {
		comp, ok := r.Attrs["component"]
		if !ok || comp.String() != "hydrate" || r.Msg != "scrollback replayed" {
			continue
		}
		out = append(out, r)
	}
	if len(out) != 1 {
		t.Fatalf("expected exactly 1 hydrate: scrollback replayed record, got %d: %+v", len(out), sink.Records())
	}
	return out[0]
}

func TestHydrateReplayedLog_TookIsDurationAcrossReplayNotSettleSleep(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-dur__0.0.fifo")
	scrollback := filepath.Join(dir, "sb")
	if err := os.WriteFile(scrollback, []byte("CONTENT"), 0o600); err != nil {
		t.Fatalf("seed scrollback: %v", err)
	}

	signalFIFOAsync(t, fifo)

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := replayCfg(t, fifo, scrollback, "dur:0.0", io.Discard, (&stubExecShell{}).fn(), logger)

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	rec := scrollbackReplayedRecord(t, sink)
	took, ok := rec.Attrs["took"]
	if !ok {
		t.Fatalf("scrollback replayed record missing took attr: %+v", rec.Attrs)
	}
	if took.Kind() != slog.KindDuration {
		t.Errorf("took kind = %v, want Duration (must be the measured time.Duration, not stringified)", took.Kind())
	}
	// took spans the io.Copy only, never the 100ms settle sleep.
	if took.Duration() >= hydrateSettleSleep {
		t.Errorf("took = %v, must be the copy duration (well under the %v settle sleep), not the settle sleep", took.Duration(), hydrateSettleSleep)
	}
}

func TestHydrateReplayedLog_PrecedesExecINFOAndFiresOnce(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-ord__0.0.fifo")
	scrollback := filepath.Join(dir, "sb")
	if err := os.WriteFile(scrollback, []byte("DUMP"), 0o600); err != nil {
		t.Fatalf("seed scrollback: %v", err)
	}

	signalFIFOAsync(t, fifo)

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := replayCfg(t, fifo, scrollback, "ord:0.0", io.Discard, (&stubExecShell{}).fn(), logger)

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	body := sink.Body()

	if n := countLogLines(body, "INFO", "scrollback replayed"); n != 1 {
		t.Fatalf("want exactly one INFO scrollback replayed, got %d: %q", n, body)
	}

	replayedIdx := strings.Index(body, "INFO scrollback replayed")
	execIdx := strings.Index(body, "INFO exec")
	if replayedIdx < 0 {
		t.Fatalf("no INFO scrollback replayed line: %q", body)
	}
	if execIdx < 0 {
		t.Fatalf("no INFO exec line: %q", body)
	}
	if replayedIdx >= execIdx {
		t.Errorf("scrollback replayed INFO must precede the exec INFO; replayedIdx=%d execIdx=%d body=%q", replayedIdx, execIdx, body)
	}
}

func TestHydrateReplayedLog_NotEmittedOnTimeoutPath(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-not__0.0.fifo")

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := timeoutCfg(t, fifo, filepath.Join(dir, "sb"), "not:0.0", io.Discard, &recordingCommander{}, (&stubExecShell{}).fn(), logger)

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	if strings.Contains(sink.Body(), "scrollback replayed") {
		t.Errorf("timeout path must NOT emit scrollback replayed: %q", sink.Body())
	}
}

func TestHydrateReplayedLog_NotEmittedOnFileMissingPath(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-fm__0.0.fifo")
	scrollback := filepath.Join(dir, "missing-sb")

	signalFIFOAsync(t, fifo)

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := replayCfg(t, fifo, scrollback, "fm:0.0", io.Discard, (&stubExecShell{}).fn(), logger)

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	if strings.Contains(sink.Body(), "scrollback replayed") {
		t.Errorf("file-missing path must NOT emit scrollback replayed: %q", sink.Body())
	}
}
