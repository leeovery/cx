package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

type paneStampCall struct {
	target string
	name   string
	value  string
}

type recordingPaneStamper struct {
	calls  []paneStampCall
	err    error
	onCall func()
}

func (r *recordingPaneStamper) SetPaneOption(target, name, value string) error {
	if r.onCall != nil {
		r.onCall()
	}
	r.calls = append(r.calls, paneStampCall{target: target, name: name, value: value})
	return r.err
}

func hooksFileInTempDir(t *testing.T) string {
	t.Helper()
	hooksFile := filepath.Join(t.TempDir(), "hooks.json")
	t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
	return hooksFile
}

func runHookSet(t *testing.T, command string) error {
	t.Helper()
	resetRootCmd()
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"hook", "set", "--on-resume", command})
	return rootCmd.Execute()
}

func TestHooksSetStampsPaneToken(t *testing.T) {
	t.Run("it stamps and writes under a freshly minted token", func(t *testing.T) {
		hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%7")

		stamper := &recordingPaneStamper{}
		hooksDeps = &HooksDeps{KeyResolver: &mockKeyResolver{key: ""}, PaneStamper: stamper}
		t.Cleanup(func() { hooksDeps = nil })

		if err := runHookSet(t, "claude --resume abc123"); err != nil {
			t.Fatalf("hook set: %v", err)
		}

		if len(stamper.calls) != 1 {
			t.Fatalf("set-option call count = %d, want 1", len(stamper.calls))
		}
		call := stamper.calls[0]
		if call.target != "%7" {
			t.Errorf("stamp target = %q, want %q", call.target, "%7")
		}
		if call.name != state.PortalPaneIDOption {
			t.Errorf("stamp option = %q, want %q", call.name, state.PortalPaneIDOption)
		}

		data := readHooksJSON(t, hooksFile)
		if len(data) != 1 {
			t.Fatalf("hooks.json entry count = %d, want 1 (%v)", len(data), data)
		}
		if data[call.value]["on-resume"] != "claude --resume abc123" {
			t.Errorf("entry under the stamped token %q = %v, want the registered command", call.value, data)
		}
		if !session.IsTokenShaped(call.value) {
			t.Errorf("stamped value %q is not token-shaped", call.value)
		}
	})

	t.Run("it reuses an existing token and issues no set-option", func(t *testing.T) {
		hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%7")

		stamper := &recordingPaneStamper{}
		hooksDeps = &HooksDeps{KeyResolver: &mockKeyResolver{key: "tok999"}, PaneStamper: stamper}
		t.Cleanup(func() { hooksDeps = nil })

		if err := runHookSet(t, "some-cmd"); err != nil {
			t.Fatalf("hook set: %v", err)
		}

		if len(stamper.calls) != 0 {
			t.Errorf("set-option call count = %d, want 0 (a stamped pane must not be re-minted): %+v", len(stamper.calls), stamper.calls)
		}
		data := readHooksJSON(t, hooksFile)
		if len(data) != 1 || data["tok999"]["on-resume"] != "some-cmd" {
			t.Errorf("hooks.json = %v, want a single entry under the pane's existing token", data)
		}
	})

	t.Run("it writes nothing when the stamp fails", func(t *testing.T) {
		hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%999")

		stamper := &recordingPaneStamper{err: fmt.Errorf("no such pane: %%999")}
		hooksDeps = &HooksDeps{KeyResolver: &mockKeyResolver{key: ""}, PaneStamper: stamper}
		t.Cleanup(func() { hooksDeps = nil })

		err := runHookSet(t, "some-cmd")
		if err == nil {
			t.Fatal("expected an error from a failed stamp, got nil")
		}
		if err.Error() != "no such pane: %999" {
			t.Errorf("error = %q, want tmux's own words unaltered", err.Error())
		}
		if _, statErr := os.Stat(hooksFile); statErr == nil {
			t.Error("hooks.json was created despite the stamp failing")
		}
	})

	t.Run("it stamps before it writes", func(t *testing.T) {
		hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%7")

		stamper := &recordingPaneStamper{}
		stamper.onCall = func() {
			if _, err := os.Stat(hooksFile); err == nil {
				t.Error("hooks.json already existed when the stamp ran: the write must not precede the stamp")
			}
		}
		hooksDeps = &HooksDeps{KeyResolver: &mockKeyResolver{key: ""}, PaneStamper: stamper}
		t.Cleanup(func() { hooksDeps = nil })

		if err := runHookSet(t, "some-cmd"); err != nil {
			t.Fatalf("hook set: %v", err)
		}
		if len(stamper.calls) != 1 {
			t.Fatalf("set-option call count = %d, want 1", len(stamper.calls))
		}
		if _, err := os.Stat(hooksFile); err != nil {
			t.Fatalf("hooks.json was not written: %v", err)
		}
	})

	t.Run("it leaves the token in place when the write fails", func(t *testing.T) {
		denied := t.TempDir()
		hooksFile := filepath.Join(denied, "hooks.json")
		t.Setenv("PORTAL_HOOKS_FILE", hooksFile)
		t.Setenv("TMUX_PANE", "%7")
		if err := os.Chmod(denied, 0o000); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(denied, 0o700) })

		stamper := &recordingPaneStamper{}
		hooksDeps = &HooksDeps{KeyResolver: &mockKeyResolver{key: ""}, PaneStamper: stamper}
		t.Cleanup(func() { hooksDeps = nil })

		if err := runHookSet(t, "some-cmd"); err == nil {
			t.Fatal("expected an error from an unwritable hooks path, got nil")
		}

		if len(stamper.calls) != 1 {
			t.Fatalf("set-option call count = %d, want exactly 1 (the stamp stands; there is no unstamp): %+v", len(stamper.calls), stamper.calls)
		}
		if stamper.calls[0].value == "" {
			t.Error("the pane was stamped with an empty token")
		}
	})

	t.Run("it never writes an empty key", func(t *testing.T) {
		hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%7")

		stamper := &recordingPaneStamper{}
		hooksDeps = &HooksDeps{
			KeyResolver: &mockKeyResolver{key: ""},
			PaneStamper: stamper,
			TokenMinter: func() (string, error) { return "", fmt.Errorf("entropy source unavailable") },
		}
		t.Cleanup(func() { hooksDeps = nil })

		err := runHookSet(t, "some-cmd")
		if err == nil {
			t.Fatal("expected an error from a failed mint, got nil")
		}
		if len(stamper.calls) != 0 {
			t.Errorf("set-option call count = %d, want 0 when no token was minted", len(stamper.calls))
		}
		if _, statErr := os.Stat(hooksFile); statErr == nil {
			t.Error("hooks.json was created despite the mint failing")
		}
	})

	t.Run("it takes no tmux call on the --pane-key path", func(t *testing.T) {
		hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "")
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"tok999": {"on-resume": "claude --resume xyz"},
		})

		stamper := &recordingPaneStamper{err: fmt.Errorf("the stamper must not be called on the --pane-key path")}
		hooksDeps = &HooksDeps{
			KeyResolver: &mockKeyResolver{err: fmt.Errorf("the resolver must not be called on the --pane-key path")},
			PaneStamper: stamper,
		}
		t.Cleanup(func() { hooksDeps = nil })

		resetRootCmd()
		rootCmd.SetOut(new(bytes.Buffer))
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"hook", "rm", "--pane-key", "tok999", "--on-resume"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("hook rm --pane-key: %v", err)
		}

		if len(stamper.calls) != 0 {
			t.Errorf("set-option call count = %d, want 0", len(stamper.calls))
		}
		if data := readHooksJSON(t, hooksFile); len(data) != 0 {
			t.Errorf("hooks.json = %v, want the seeded entry removed", data)
		}
	})
}

func TestHooksSetRefusesAnUnresolvablePane(t *testing.T) {
	const stderr = "no such pane: %999"

	t.Run("it exits non-zero from hook set on an unresolvable pane", func(t *testing.T) {
		hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%999")

		hooksDeps = &HooksDeps{
			KeyResolver: &mockKeyResolver{err: &tmux.CommandError{Stderr: stderr}},
			PaneStamper: &recordingPaneStamper{},
		}
		t.Cleanup(func() { hooksDeps = nil })

		err := runHookSet(t, "some-cmd")
		if err == nil {
			t.Fatal("expected an error from an unresolvable pane, got nil")
		}
		if !strings.Contains(err.Error(), stderr) {
			t.Errorf("error = %q, want it to carry tmux's own words %q", err.Error(), stderr)
		}
		var cmdErr *tmux.CommandError
		if !errors.As(err, &cmdErr) {
			t.Errorf("error %v is not a recoverable *tmux.CommandError (errors.As failed)", err)
		}
		if _, statErr := os.Stat(hooksFile); statErr == nil {
			t.Error("hooks.json was created despite the pane being unresolvable")
		}
	})

	t.Run("it mints and stamps nothing when the probe fails", func(t *testing.T) {
		hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%999")

		stamper := &recordingPaneStamper{}
		minted := 0
		hooksDeps = &HooksDeps{
			KeyResolver: &mockKeyResolver{err: &tmux.CommandError{Stderr: stderr}},
			PaneStamper: stamper,
			TokenMinter: func() (string, error) { minted++; return "tok000", nil },
		}
		t.Cleanup(func() { hooksDeps = nil })

		if err := runHookSet(t, "some-cmd"); err == nil {
			t.Fatal("expected an error from an unresolvable pane, got nil")
		}
		if minted != 0 {
			t.Errorf("mint count = %d, want 0", minted)
		}
		if len(stamper.calls) != 0 {
			t.Errorf("set-option call count = %d, want 0: %+v", len(stamper.calls), stamper.calls)
		}
	})
}
