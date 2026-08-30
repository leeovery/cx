package cmd

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

func TestHydrateTimeoutLog_EmitsSignalTimeoutTookOnTimeoutPath(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-stl__0.0.fifo")

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := hydrateCfg(t, hydrateCfgOpts{FIFO: fifo, File: filepath.Join(dir, "sb"), HookKey: "stl:0.0",
		OpenFIFO: instantTimeoutOpenFIFO, Logger: logger})

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	info := execLogLine(t, sink.Body(), "INFO", "signal timeout")
	if !strings.Contains(info, "took=3s") {
		t.Errorf("signal timeout INFO missing took=3s: %q", info)
	}
}

func TestHydrateTimeoutLog_TookAttrIsDurationNotString(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-dur__0.0.fifo")

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := hydrateCfg(t, hydrateCfgOpts{FIFO: fifo, File: filepath.Join(dir, "sb"), HookKey: "dur:0.0",
		OpenFIFO: instantTimeoutOpenFIFO, Logger: logger})

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	rec := sink.OnlyRecordWith(t, "hydrate", "signal timeout")
	took, ok := rec.Attrs["took"]
	if !ok {
		t.Fatalf("signal timeout record missing took attr: %+v", rec.Attrs)
	}
	if took.Kind() != slog.KindDuration {
		t.Errorf("took kind = %v, want Duration (must be passed as the hydrateTimeout time.Duration, not stringified)", took.Kind())
	}
	if took.Duration() != hydrateTimeout {
		t.Errorf("took = %v, want hydrateTimeout (%v)", took.Duration(), hydrateTimeout)
	}
}

func TestHydrateTimeoutLog_SignalTimeoutPrecedesExecINFO(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-ord__0.0.fifo")

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	// A nil HookStore takes the bare-shell exec path, which emits the exec INFO.
	cfg := hydrateCfg(t, hydrateCfgOpts{FIFO: fifo, File: filepath.Join(dir, "sb"), HookKey: "ord:0.0",
		OpenFIFO: instantTimeoutOpenFIFO, Logger: logger})

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	body := sink.Body()
	signalIdx := strings.Index(body, "INFO signal timeout")
	execIdx := strings.Index(body, "INFO exec")
	if signalIdx < 0 {
		t.Fatalf("no INFO signal timeout line: %q", body)
	}
	if execIdx < 0 {
		t.Fatalf("no INFO exec line: %q", body)
	}
	if signalIdx >= execIdx {
		t.Errorf("signal timeout INFO must precede the exec INFO; signalIdx=%d execIdx=%d body=%q", signalIdx, execIdx, body)
	}
}

func TestHydrateTimeoutLog_PreservesWarnUnlinkAndMarkerUnset(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-pre__0.0.fifo")

	cmder := &recordingCommander{}
	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	cfg := hydrateCfg(t, hydrateCfgOpts{FIFO: fifo, File: filepath.Join(dir, "sb"), HookKey: "pre:0.0",
		OpenFIFO: instantTimeoutOpenFIFO, Commander: cmder, Logger: logger})

	if err := runHydrate(cfg); err != nil {
		t.Fatalf("runHydrate: %v", err)
	}

	body := sink.Body()

	if n := strings.Count(body, "timeout waiting for hydrate signal"); n != 1 {
		t.Errorf("want exactly one existing timeout WARN, got %d: %q", n, body)
	}

	if _, err := os.Stat(fifo); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("FIFO not removed on timeout; stat err = %v", err)
	}

	wantUnset := "set-option -su @portal-skeleton-pre__0.0"
	found := false
	for _, c := range cmder.Calls {
		if strings.Join(c, " ") == wantUnset {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected marker-unset call %q; calls: %v", wantUnset, cmder.Calls)
	}
}

func TestHydrateTimeoutLog_NilHandleTimeout_NoSignalTimeoutNoExec(t *testing.T) {
	dir := t.TempDir()
	fifo := makeFIFO(t, dir, "hydrate-nil__0.0.fifo")

	logger, sink := newCaptureLoggerForComponent(t, "hydrate")
	exec := &stubExecShell{}
	cfg := hydrateConfig{
		FIFO:      fifo,
		File:      filepath.Join(dir, "sb"),
		HookKey:   "nil:0.0",
		Stdout:    new(bytes.Buffer),
		Client:    tmux.NewClient(&recordingCommander{}),
		Logger:    logger,
		ExecShell: exec.fn(),
		OpenFIFO:  instantTimeoutOpenFIFO,
		// HandleTimeout intentionally left nil — test-only fall-through.
	}

	if err := runHydrate(cfg); err == nil {
		t.Fatal("runHydrate must return the timeout error when HandleTimeout is nil")
	}

	if exec.called {
		t.Error("ExecShell must NOT be called on the nil-HandleTimeout fall-through")
	}
	if strings.Contains(sink.Body(), "signal timeout") {
		t.Errorf("nil-HandleTimeout fall-through must NOT emit signal timeout: %q", sink.Body())
	}
}
