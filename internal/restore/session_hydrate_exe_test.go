package restore

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
)

func TestSessionRestorerHydrateExe(t *testing.T) {
	t.Run("it defaults to the running executable's absolute path", func(t *testing.T) {
		want, err := os.Executable()
		if err != nil {
			t.Skipf("os.Executable unavailable on this platform: %v", err)
		}
		r := &SessionRestorer{}

		got := r.hydrateExe()

		if got != want {
			t.Errorf("hydrateExe() = %q; want %q", got, want)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("hydrateExe() = %q; want an absolute path", got)
		}
		if got == "portal" {
			t.Error("hydrateExe() resolved to the bare name; the armed pane would take a PATH lookup")
		}
	})

	t.Run("it uses the injected resolver when one is set", func(t *testing.T) {
		r := &SessionRestorer{Exe: func() (string, error) { return "/staged/bin/portal", nil }}

		if got := r.hydrateExe(); got != "/staged/bin/portal" {
			t.Errorf("hydrateExe() = %q; want %q", got, "/staged/bin/portal")
		}
	})

	t.Run("it falls back to the bare name when the resolver errors", func(t *testing.T) {
		r := &SessionRestorer{Exe: func() (string, error) { return "", errors.New("unresolvable") }}

		if got := r.hydrateExe(); got != "portal" {
			t.Errorf("hydrateExe() = %q; want %q", got, "portal")
		}
	})

	t.Run("it falls back to the bare name when the resolver yields an empty path", func(t *testing.T) {
		r := &SessionRestorer{Exe: func() (string, error) { return "", nil }}

		if got := r.hydrateExe(); got != "portal" {
			t.Errorf("hydrateExe() = %q; want %q", got, "portal")
		}
	})

	t.Run("the fallback still composes a runnable command", func(t *testing.T) {
		r := &SessionRestorer{Exe: func() (string, error) { return "", errors.New("unresolvable") }}

		got := buildHydrateCommand(r.hydrateExe(), "/x.fifo", "/y.bin", "tok")

		want := "'portal' state hydrate --fifo '/x.fifo' --file '/y.bin' --hook-key 'tok'"
		if got != want {
			t.Errorf("fallback command:\n got %q\nwant %q", got, want)
		}
	})

	t.Run("the resolved path leads the composed command", func(t *testing.T) {
		got := buildHydrateCommand("/staged/bin/portal", "/x.fifo", "/y.bin", "")

		want := "'/staged/bin/portal' state hydrate --fifo '/x.fifo' --file '/y.bin'"
		if got != want {
			t.Errorf("composed command:\n got %q\nwant %q", got, want)
		}
		if strings.HasPrefix(got, "'portal' ") {
			t.Errorf("composed command %q starts with a bare PATH lookup", got)
		}
	})
}

func TestSessionRestorerHydrateExe_FallbackDiagnostics(t *testing.T) {
	t.Run("an errored resolver reports the error", func(t *testing.T) {
		sink := &logtest.Sink{}
		r := &SessionRestorer{
			Logger: slog.New(sink),
			Exe:    func() (string, error) { return "", errors.New("unresolvable") },
		}

		r.hydrateExe()

		rec := onlyWarn(t, sink)
		if got := rec.AttrString(t, "error"); got != "unresolvable" {
			t.Errorf("error attr = %q; want %q", got, "unresolvable")
		}
	})

	t.Run("an empty path from a nil-error resolver carries no nil error attr", func(t *testing.T) {
		sink := &logtest.Sink{}
		r := &SessionRestorer{
			Logger: slog.New(sink),
			Exe:    func() (string, error) { return "", nil },
		}

		r.hydrateExe()

		rec := onlyWarn(t, sink)
		if rec.HasAttr("error") {
			t.Errorf("record %q carries an error attr for a nil-error resolver: %v", rec.Msg, rec.Keys)
		}
	})
}

func onlyWarn(t *testing.T, sink *logtest.Sink) logtest.Record {
	t.Helper()
	rec := sink.OnlyRecord(t)
	if rec.Level != slog.LevelWarn {
		t.Fatalf("record level = %v; want WARN", rec.Level)
	}
	return rec
}

func TestOrchestratorNewSessionRestorer(t *testing.T) {
	exe := func() (string, error) { return "/staged/bin/portal", nil }
	logger := slog.New(&logtest.Sink{})
	o := &Orchestrator{StateDir: "/state", Logger: logger, Exe: exe}

	sr := o.newSessionRestorer()

	if sr.StateDir != "/state" {
		t.Errorf("StateDir = %q; want %q", sr.StateDir, "/state")
	}
	if sr.Logger != logger {
		t.Error("Logger did not propagate to the SessionRestorer")
	}
	if sr.Exe == nil {
		t.Fatal("Exe did not propagate; the restorer would fall back to os.Executable")
	}
	if got, _ := sr.Exe(); got != "/staged/bin/portal" {
		t.Errorf("propagated Exe resolved to %q; want %q", got, "/staged/bin/portal")
	}
}
