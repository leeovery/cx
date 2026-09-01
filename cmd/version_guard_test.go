package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/spf13/cobra"
)

type stubVersionChecker struct {
	calls int
	err   error
}

func (s *stubVersionChecker) check(_ tmux.Commander) error {
	s.calls++
	return s.err
}

// installStubVersionChecker resets the sync.Once gate up front, because another
// test in the same binary may already have consumed it, and again on cleanup.
func installStubVersionChecker(t *testing.T, stub *stubVersionChecker) {
	t.Helper()
	prev := versionChecker
	versionChecker = stub.check
	resetVersionCheckForTest()
	t.Cleanup(func() { versionChecker = prev })
	t.Cleanup(resetVersionCheckForTest)
}

func TestVersionGuard_InvokedForNonExemptOpen(t *testing.T) {
	stub := &stubVersionChecker{}
	installStubVersionChecker(t, stub)

	// A client must be in context: open's session-domain pre-check reaches for it
	// before path resolution runs.
	withBootstrapDeps(t, BootstrapDeps{Orchestrator: &nopRunner{}, Client: tmux.NewClient(stubTmuxCommander())})

	resetRootCmd()
	rootCmd.SetArgs([]string{"open", "/nonexistent/path/that/does/not/exist"})
	// The resolver error is expected; what matters is that PersistentPreRunE ran
	// before the resolver was reached.
	_ = rootCmd.Execute()

	if stub.calls != 1 {
		t.Errorf("version checker call count = %d, want 1", stub.calls)
	}
}

func TestVersionGuard_InvokedForOtherNonExemptCommands(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		setup func(t *testing.T)
	}{
		{
			name: "portal list",
			args: []string{"list"},
			setup: func(t *testing.T) {
				withListDeps(t, ListDeps{
					Lister: &mockSessionLister{sessions: []tmux.Session{}},
					IsTTY:  func() bool { return false },
				})
			},
		},
		{
			name: "portal open --session",
			args: []string{"open", "--session", "my-session"},
			setup: func(t *testing.T) {
				withOpenDeps(t, OpenDeps{SessionLister: &testSessionLister{names: []string{"my-session"}}})

				// A no-op so no real connector fires.
				origSession := openSessionFunc
				openSessionFunc = func(_ *cobra.Command, _ string) error { return nil }
				t.Cleanup(func() { openSessionFunc = origSession })
			},
		},
		{
			name: "portal kill",
			args: []string{"kill", "my-session"},
			setup: func(t *testing.T) {
				withKillDeps(t, KillDeps{
					Killer:    &mockSessionKiller{},
					Validator: &mockSessionValidator{sessions: map[string]bool{"my-session": true}},
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubVersionChecker{}
			installStubVersionChecker(t, stub)

			withBootstrapDeps(t, BootstrapDeps{Orchestrator: &nopRunner{}})

			tt.setup(t)

			resetRootCmd()
			rootCmd.SetArgs(tt.args)
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if stub.calls != 1 {
				t.Errorf("version checker call count = %d, want 1", stub.calls)
			}
		})
	}
}

func TestVersionGuard_NotInvokedForExemptCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "portal version", args: []string{"version"}},
		{name: "portal init", args: []string{"init", "zsh"}},
		{name: "portal alias list", args: []string{"alias", "list"}},
		{name: "portal uninstall", args: []string{"uninstall"}},
		{name: "portal state daemon", args: []string{"state", "daemon"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubVersionChecker{}
			installStubVersionChecker(t, stub)

			// These exempt commands Execute their real bodies, so every
			// side-effect surface must be stubbed or poisoned. A command that
			// builds tmux.DefaultClient() would otherwise honour the ambient TMUX
			// and reach the developer's real server; the poison makes that fail
			// loudly against a dead socket instead.
			t.Setenv("TMUX", "/nonexistent/portal-version-guard-test,0,0")
			t.Setenv("PORTAL_STATE_DIR", t.TempDir())
			t.Setenv("PORTAL_ALIASES_FILE", t.TempDir()+"/aliases")
			t.Setenv("PORTAL_PROJECTS_FILE", t.TempDir()+"/projects.json")
			hooksFileInTempDir(t)

			// uninstall builds a tmux client in its real body.
			installUninstallDeps(t, &UninstallDeps{
				Client:     tmux.NewClient(quietCommander()),
				Unregister: func(*tmux.Client) error { return nil },
			})

			// The daemon's RunE blocks on a signal, so stub the run-func.
			if len(tt.args) >= 2 && tt.args[0] == "state" && tt.args[1] == "daemon" {
				t.Setenv("PORTAL_STATE_DIR", t.TempDir())
				prev := daemonRunFunc
				daemonRunFunc = func(_ context.Context, _ *daemonDeps) error { return nil }
				t.Cleanup(func() { daemonRunFunc = prev })
				withDaemonLockFileReset(t)
			}

			resetRootCmd()
			resetStateCmdFlags()
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if stub.calls != 0 {
				t.Errorf("version checker call count for exempt command = %d, want 0", stub.calls)
			}
		})
	}
}

func TestVersionGuard_RunsExactlyOnceAcrossRepeatedInvocations(t *testing.T) {
	stub := &stubVersionChecker{}
	installStubVersionChecker(t, stub)

	withBootstrapDeps(t, BootstrapDeps{Orchestrator: &nopRunner{}})

	withListDeps(t, ListDeps{
		Lister: &mockSessionLister{sessions: []tmux.Session{}},
		IsTTY:  func() bool { return false },
	})

	for i := range 3 {
		resetRootCmd()
		rootCmd.SetArgs([]string{"list"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
	}

	if stub.calls != 1 {
		t.Errorf("version checker call count = %d, want 1 across 3 invocations", stub.calls)
	}
}

func TestVersionGuard_ShortCircuitsBootstrapOnFailure(t *testing.T) {
	stub := &stubVersionChecker{err: errors.New("Portal requires tmux \u2265 3.0 (found 2.9). Please upgrade.")}
	installStubVersionChecker(t, stub)

	withBootstrapDeps(t, BootstrapDeps{Orchestrator: panicRunner{}})

	resetRootCmd()
	rootCmd.SetArgs([]string{"list"})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PersistentPreRunE panicked instead of returning error: %v", r)
		}
	}()

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error from version checker, got nil")
	}
	if stub.calls != 1 {
		t.Errorf("version checker call count = %d, want 1", stub.calls)
	}
}

func TestVersionGuard_PropagatesExactCheckerError(t *testing.T) {
	wantMsg := "Portal requires tmux \u2265 3.0 (found 2.9). Please upgrade."
	stub := &stubVersionChecker{err: errors.New(wantMsg)}
	installStubVersionChecker(t, stub)

	withBootstrapDeps(t, BootstrapDeps{Orchestrator: panicRunner{}})

	resetRootCmd()
	rootCmd.SetArgs([]string{"list"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error from version checker, got nil")
	}
	if err.Error() != wantMsg {
		t.Errorf("error = %q, want %q", err.Error(), wantMsg)
	}
	if strings.Contains(err.Error(), "wrap") {
		t.Errorf("error appears wrapped, want exact propagation: %q", err.Error())
	}
}
