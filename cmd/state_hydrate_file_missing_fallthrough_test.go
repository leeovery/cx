package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHydrate_NilHandleFileMissing_OpenFailureReturnsError(t *testing.T) {
	t.Run("it falls through when no file-missing handler is set", func(t *testing.T) {
		dir := t.TempDir()
		fifo := makeFIFO(t, dir, "hydrate-nofm-open__0.0.fifo")
		scrollback := filepath.Join(dir, "missing-sb")

		signalFIFOAsync(t, fifo)

		exec := &stubExecShell{}
		cfg := hydrateCfg(t, hydrateCfgOpts{
			FIFO:          fifo,
			File:          scrollback,
			HookKey:       "nofm:0.0",
			OpenFIFO:      openFIFOWithTimeout,
			ExecShell:     exec.fn(),
			AbsentHandler: hydrateAbsentFileMissing,
		})

		err := runHydrate(cfg)
		if err == nil {
			t.Fatal("runHydrate must return the open error when HandleFileMissing is nil")
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("error = %v, want one traversing to fs.ErrNotExist", err)
		}
		if want := "open scrollback " + scrollback; !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to carry %q", err, want)
		}
		if exec.called {
			t.Error("ExecShell must NOT be called on the nil-HandleFileMissing fall-through")
		}
	})
}

func TestHydrate_NilHandleFileMissing_CopyFailureReturnsError(t *testing.T) {
	t.Run("it falls through when no file-missing handler is set", func(t *testing.T) {
		dir := t.TempDir()
		fifo := makeFIFO(t, dir, "hydrate-nofm-copy__0.0.fifo")
		// A directory opens cleanly and fails io.Copy's first read (EISDIR), which
		// is the mid-stream shape this fall-through guards.
		scrollbackDir := filepath.Join(dir, "sb-as-dir")
		if err := os.Mkdir(scrollbackDir, 0o700); err != nil {
			t.Fatalf("mkdir scrollback dir: %v", err)
		}

		signalFIFOAsync(t, fifo)

		exec := &stubExecShell{}
		cfg := hydrateCfg(t, hydrateCfgOpts{
			FIFO:          fifo,
			File:          scrollbackDir,
			HookKey:       "nofmc:0.0",
			OpenFIFO:      openFIFOWithTimeout,
			ExecShell:     exec.fn(),
			AbsentHandler: hydrateAbsentFileMissing,
		})

		err := runHydrate(cfg)
		if err == nil {
			t.Fatal("runHydrate must return the copy error when HandleFileMissing is nil")
		}
		if _, ok := errors.AsType[*os.PathError](err); !ok {
			t.Errorf("error = %v, want the copy error verbatim (an *os.PathError)", err)
		}
		if strings.Contains(err.Error(), "open scrollback") {
			t.Errorf("error = %q, want the copy error rather than the open-failure wrapping", err)
		}
		if exec.called {
			t.Error("ExecShell must NOT be called on the nil-HandleFileMissing fall-through")
		}
	})
}
