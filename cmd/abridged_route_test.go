package cmd

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/warning"
	"github.com/spf13/cobra"
)

func satisfiedLatchAliveSaverCommander() *recordingCommander {
	return &recordingCommander{
		RunFunc: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == "show-option":
				return version, nil // stored latch == running version -> satisfied
			case len(args) > 0 && args[0] == "list-panes" && isPanePIDProbe(args):
				return "12345\n", nil // _portal-saver pane alive
			}
			return "", nil
		},
	}
}

// Carries no latch arm: ensureSaverLiveness never reads the latch.
func saverAbsentReviveFailsCommander() *recordingCommander {
	return &recordingCommander{
		RunFunc: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == "list-panes":
				return "", noSuchSessionErr() // saver absent -> revive
			case len(args) > 0 && args[0] == "has-session":
				return "", errors.New("can't find session") // absent
			case len(args) > 0 && args[0] == "new-session":
				return "", errors.New("create denied") // revive fails across all retries
			}
			return "", nil
		},
	}
}

func satisfiedLatchSaverAbsentCommander() *recordingCommander {
	base := saverAbsentReviveFailsCommander()
	return &recordingCommander{
		RunFunc: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == "show-option" {
				return version, nil // stored latch == running version -> satisfied
			}
			return base.RunFunc(args...)
		},
	}
}

// Carries tmux's option-absent phrasing, so TryGetServerOption collapses it
// to ("", false, nil) — the "latch absent" classification.
func optionAbsentErr() error {
	return &tmux.CommandError{
		Stderr: "unknown option: @portal-bootstrapped",
		Err:    errors.New("exit status 1"),
	}
}

// Reads the latch as absent, so the route is deterministic regardless of any
// real tmux server the developer is running (whose latch may coincide with
// the dev version).
func notSatisfiedLatchClient() *tmux.Client {
	return tmux.NewClient(&recordingCommander{
		RunFunc: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == "show-option" {
				return "", optionAbsentErr()
			}
			return "", nil
		},
	})
}

func installMockList(t *testing.T) {
	t.Helper()
	listDeps = &ListDeps{
		Lister: &mockSessionLister{sessions: []tmux.Session{}},
		IsTTY:  func() bool { return false },
	}
	t.Cleanup(func() { listDeps = nil })
}

func TestPersistentPreRunE_FullBootstrap_WhenNotSatisfied(t *testing.T) {
	cases := []struct {
		name            string
		showOptValue    string
		showOptErr      error
		versionOverride string // "" -> leave the running version untouched
		reason          string // parenthetical rationale in the assertion message
	}{
		{
			name:       "latch absent",
			showOptErr: optionAbsentErr(), // ("",false,nil) -> not satisfied
			reason:     "full bootstrap",
		},
		{
			name:            "version mismatch",
			showOptValue:    "v1.0.0", // present but != running version -> not satisfied
			versionOverride: "v2.0.0",
			reason:          "full bootstrap re-stamps",
		},
		{
			name:       "latch read error",
			showOptErr: errors.New("tmux socket connect failed"), // read error -> not satisfied
			reason:     "folds into full bootstrap",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetBootstrapOnce(t)

			if tc.versionOverride != "" {
				prevVersion := version
				version = tc.versionOverride
				t.Cleanup(func() { version = prevVersion })
			}

			client := tmux.NewClient(&recordingCommander{
				RunFunc: func(args ...string) (string, error) {
					if len(args) > 0 && args[0] == "show-option" {
						return tc.showOptValue, tc.showOptErr
					}
					return "", nil
				},
			})
			runner := &recordingRunner{started: false}
			bootstrapDeps = &BootstrapDeps{Orchestrator: runner, Client: client}
			t.Cleanup(func() { bootstrapDeps = nil })

			installMockList(t)

			resetRootCmd()
			rootCmd.SetArgs([]string{"list"})
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if runner.calls != 1 {
				t.Errorf("%s: orchestrator calls = %d, want 1 (%s)", tc.name, runner.calls, tc.reason)
			}
		})
	}
}

func TestPersistentPreRunE_Abridged_EmitsWarningsToStderrOnCLIPath(t *testing.T) {
	resetBootstrapOnce(t)
	resetBootstrapWarnings(t)
	stubSaverAliveCheck(t, false)
	shrinkSaverRetryDelay(t)

	client := tmux.NewClient(satisfiedLatchSaverAbsentCommander())
	runner := &recordingRunner{started: false}
	bootstrapDeps = &BootstrapDeps{Orchestrator: runner, Client: client}
	t.Cleanup(func() { bootstrapDeps = nil })

	installMockList(t)

	errBuf := new(bytes.Buffer)
	resetRootCmd()
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.calls != 0 {
		t.Errorf("abridged CLI path ran the full orchestrator (%d calls); want 0", runner.calls)
	}

	wantBuf := new(bytes.Buffer)
	warning.WriteLines(wantBuf, []warning.Warning{bootstrap.SaverDownWarning()})
	if errBuf.String() != wantBuf.String() {
		t.Errorf("stderr = %q, want the rendered SaverDownWarning %q", errBuf.String(), wantBuf.String())
	}
}

func TestPersistentPreRunE_Abridged_LeavesWarningsForOpenTUIOnTUIPath(t *testing.T) {
	resetBootstrapOnce(t)
	resetBootstrapWarnings(t)
	stubSaverAliveCheck(t, false)
	shrinkSaverRetryDelay(t)

	client := tmux.NewClient(satisfiedLatchSaverAbsentCommander())
	runner := &recordingRunner{started: false}
	bootstrapDeps = &BootstrapDeps{Orchestrator: runner, Client: client}
	t.Cleanup(func() { bootstrapDeps = nil })

	var pendingAtOpenTUI []bootstrap.Warning
	origFunc := openTUIFunc
	openTUIFunc = func(_ *cobra.Command, _ string, _ []string, _ bool) error {
		pendingAtOpenTUI = bootstrapWarnings.Drain()
		return nil
	}
	t.Cleanup(func() { openTUIFunc = origFunc })

	resetRootCmd()
	rootCmd.SetArgs([]string{"open"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.calls != 0 {
		t.Errorf("abridged TUI path ran the full orchestrator (%d calls); want 0", runner.calls)
	}
	if len(pendingAtOpenTUI) != 1 {
		t.Fatalf("openTUI saw %d pending warnings, want 1 (SaverDownWarning left for the notice band)", len(pendingAtOpenTUI))
	}
	if !reflect.DeepEqual(pendingAtOpenTUI[0], bootstrap.SaverDownWarning()) {
		t.Errorf("pending warning = %#v, want %#v", pendingAtOpenTUI[0], bootstrap.SaverDownWarning())
	}
}

func TestPersistentPreRunE_Abridged_OpenSessionTakesAbridgedPath(t *testing.T) {
	resetBootstrapOnce(t)
	resetBootstrapWarnings(t)

	client := tmux.NewClient(satisfiedLatchAliveSaverCommander())
	runner := &recordingRunner{started: false}
	bootstrapDeps = &BootstrapDeps{Orchestrator: runner, Client: client}
	t.Cleanup(func() { bootstrapDeps = nil })

	connector := &mockSessionConnector{}
	openDeps = &OpenDeps{SessionLister: &testSessionLister{names: []string{"proj-abc123"}}}
	t.Cleanup(func() { openDeps = nil })

	origSession := openSessionFunc
	openSessionFunc = func(_ *cobra.Command, name string) error { return connector.Connect(name) }
	t.Cleanup(func() { openSessionFunc = origSession })

	resetRootCmd()
	rootCmd.SetArgs([]string{"open", "--session", "proj-abc123"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.calls != 0 {
		t.Errorf("open --session + satisfied latch: orchestrator calls = %d, want 0 (abridged sync path is command-agnostic)", runner.calls)
	}
	if connector.connectedTo != "proj-abc123" {
		t.Errorf("open --session did not proceed: connectedTo = %q, want proj-abc123", connector.connectedTo)
	}
}
