package bootstrapadapter_test

import (
	"errors"
	"testing"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmuxtest"
)

type listerStub struct {
	out string
	err error
}

func (s *listerStub) ShowAllServerOptions() (string, error) {
	return s.out, s.err
}

func TestFIFOSweeper_PropagatesListSkeletonMarkersError(t *testing.T) {
	sentinel := errors.New("show-options boom")
	s := &bootstrapadapter.FIFOSweeper{
		Client:   &listerStub{err: sentinel},
		StateDir: t.TempDir(),
		Logger:   nil,
	}

	err := s.Sweep()
	if err == nil {
		t.Fatal("Sweep returned nil; want wrapped error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("Sweep err = %v; want errors.Is(err, sentinel)=true", err)
	}
	if got := err.Error(); got == "" || got == sentinel.Error() {
		t.Errorf("Sweep err = %q; want a wrapped message containing the cause", got)
	}
}

func TestRestoringMarker_SetClearsTogglesServerOption(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-bsa-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	m := &bootstrapadapter.RestoringMarker{Client: client}

	if _, found, err := client.TryGetServerOption(state.RestoringMarkerName); err != nil {
		t.Fatalf("TryGetServerOption pre-Set: %v", err)
	} else if found {
		t.Fatal("@portal-restoring unexpectedly set before Set()")
	}

	if err := m.Set(); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, found, err := client.TryGetServerOption(state.RestoringMarkerName)
	if err != nil {
		t.Fatalf("TryGetServerOption post-Set: %v", err)
	}
	if !found || val != "1" {
		t.Errorf("post-Set: found=%v value=%q; want found=true value=%q", found, val, "1")
	}

	if err := m.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, found, err := client.TryGetServerOption(state.RestoringMarkerName); err != nil {
		t.Fatalf("TryGetServerOption post-Clear: %v", err)
	} else if found {
		t.Error("@portal-restoring still present after Clear()")
	}

	if err := m.Clear(); err != nil {
		t.Errorf("Clear (second invocation): %v", err)
	}
}

func TestHookRegistrar_RegistersPortalHooks(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-bsa-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	r := &bootstrapadapter.HookRegistrar{Client: client}
	if err := r.RegisterPortalHooks(); err != nil {
		t.Fatalf("RegisterPortalHooks: %v", err)
	}

	if err := r.RegisterPortalHooks(); err != nil {
		t.Errorf("RegisterPortalHooks (second invocation): %v", err)
	}
}

// Exe falls back to os.Executable, which under `go test` is the test binary: a
// pane armed with it re-runs the suite inside itself and takes the session with
// it, silently. The constructor is the seam a test reaches the orchestrator
// through, so it refuses to build one that has not pinned the binary.
func TestNewRestoreAdapter_RequiresAnExeResolver(t *testing.T) {
	t.Run("it refuses to build the restore adapter without an exe", func(t *testing.T) {
		a, err := bootstrapadapter.NewRestoreAdapter(nil, t.TempDir(), nil, nil)

		if err == nil {
			t.Fatal("NewRestoreAdapter with a nil exe returned no error")
		}
		if !errors.Is(err, bootstrapadapter.ErrRestoreExeRequired) {
			t.Errorf("err = %v; want errors.Is(err, ErrRestoreExeRequired)=true", err)
		}
		if a != nil {
			t.Errorf("adapter = %v; want nil alongside the error", a)
		}
	})

	t.Run("it pins the resolver it is given on the inner orchestrator", func(t *testing.T) {
		stateDir := t.TempDir()
		const exePath = "/staged/portal"

		a, err := bootstrapadapter.NewRestoreAdapter(nil, stateDir, nil,
			func() (string, error) { return exePath, nil })

		if err != nil {
			t.Fatalf("NewRestoreAdapter: %v", err)
		}
		if a.Inner.Exe == nil {
			t.Fatal("inner orchestrator Exe is nil; want the resolver passed in")
		}
		got, resolveErr := a.Inner.Exe()
		if resolveErr != nil || got != exePath {
			t.Errorf("Exe() = %q, %v; want %q, nil", got, resolveErr, exePath)
		}
		if a.Inner.StateDir != stateDir {
			t.Errorf("StateDir = %q; want %q", a.Inner.StateDir, stateDir)
		}
	})
}
