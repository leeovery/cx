package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

func TestHooksRmExitsZeroOnlyWhenItRemoved(t *testing.T) {
	t.Run("it exits non-zero for a pane no live pane answers to", func(t *testing.T) {
		const stderr = "no such pane: %999"

		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%999")
		before := seedHooksFile(t, hooksFile, map[string]map[string]string{
			"tok123": {"on-resume": "claude --resume abc"},
		})

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{err: &tmux.CommandError{Stderr: stderr}}})

		_, err := runHookRm(t)
		if err == nil {
			t.Fatal("expected an error for a pane no live pane answers to, got nil")
		}
		if !strings.Contains(err.Error(), stderr) {
			t.Errorf("error = %q, want it to carry tmux's own words %q", err.Error(), stderr)
		}
		if _, ok := errors.AsType[*tmux.CommandError](err); !ok {
			t.Errorf("error %v is not a recoverable *tmux.CommandError (errors.As failed)", err)
		}
		assertHooksFileUnchanged(t, hooksFile, before)
	})

	t.Run("it exits non-zero with its own words for a live pane carrying no token", func(t *testing.T) {
		hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: ""}})

		_, err := runHookRm(t)
		if err == nil {
			t.Fatal("expected an error for a live pane carrying no token, got nil")
		}
		if err.Error() != "no resume hook registered for this pane" {
			t.Errorf("error = %q, want %q", err.Error(), "no resume hook registered for this pane")
		}
	})

	t.Run("it consults the store for nothing when the pane carries no token", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: ""}})

		if _, err := runHookRm(t); err == nil {
			t.Fatal("expected an error for a live pane carrying no token, got nil")
		}
		if _, err := os.Stat(hooksFile); err == nil {
			t.Error("hooks.json was created for a pane carrying no token: the store must not be consulted")
		}

		before := seedHooksFile(t, hooksFile, map[string]map[string]string{
			"":       {"on-resume": "an empty-key entry"},
			"tok123": {"on-resume": "claude --resume abc"},
		})
		if _, err := runHookRm(t); err == nil {
			t.Fatal("expected an error for a live pane carrying no token, got nil")
		}
		assertHooksFileUnchanged(t, hooksFile, before)
	})

	t.Run("it exits non-zero when the resolved token has no entry", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		before := seedHooksFile(t, hooksFile, map[string]map[string]string{
			"tok999": {"on-resume": "npm start"},
		})

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		_, err := runHookRm(t)
		if err == nil {
			t.Fatal("expected an error when the resolved token has no entry, got nil")
		}
		if err.Error() != "no resume hook registered for tok123" {
			t.Errorf("error = %q, want %q", err.Error(), "no resume hook registered for tok123")
		}
		assertHooksFileUnchanged(t, hooksFile, before)
	})

	t.Run("it exits non-zero when --pane-key names no entry", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		before := seedHooksFile(t, hooksFile, map[string]map[string]string{
			"tok999": {"on-resume": "npm start"},
		})

		resolver, stamper := paneKeyPathSeams()
		withHooksDeps(t, HooksDeps{KeyResolver: resolver, PaneStamper: stamper})

		_, err := runHookRm(t, "--pane-key", "sess:0.1")
		if err == nil {
			t.Fatal("expected an error when --pane-key names no entry, got nil")
		}
		if err.Error() != "no resume hook registered for sess:0.1" {
			t.Errorf("error = %q, want %q", err.Error(), "no resume hook registered for sess:0.1")
		}
		assertNoPaneTmuxCalls(t, resolver, stamper)
		assertHooksFileUnchanged(t, hooksFile, before)
	})

	t.Run("it exits 0 and removes on the resolved-token path", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"tok123": {"on-resume": "claude --resume abc"},
			"tok999": {"on-resume": "npm start"},
		})

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		if _, err := runHookRm(t); err != nil {
			t.Fatalf("hook rm: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["tok123"]; ok {
			t.Error("expected the resolved token's entry to be removed")
		}
		if data["tok999"]["on-resume"] != "npm start" {
			t.Errorf("hooks.json = %v, want the other pane's entry left in place", data)
		}
	})

	t.Run("it exits 0 and removes on the --pane-key path", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "")
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"sess:0.1": {"on-resume": "claude --resume abc"},
			"tok999":   {"on-resume": "npm start"},
		})

		resolver, stamper := paneKeyPathSeams()
		withHooksDeps(t, HooksDeps{KeyResolver: resolver, PaneStamper: stamper})

		// An old-format key: the pass-through validates nothing, so removing one
		// by hand keeps working and keeps exiting 0.
		if _, err := runHookRm(t, "--pane-key", "sess:0.1"); err != nil {
			t.Fatalf("hook rm --pane-key: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data["sess:0.1"]; ok {
			t.Error("expected the verbatim key's entry to be removed")
		}
		assertNoPaneTmuxCalls(t, resolver, stamper)
	})

	t.Run("it leaves hooks.json byte-identical on every failing route", func(t *testing.T) {
		seeded := map[string]map[string]string{
			"tok999": {"on-resume": "npm start"},
		}

		paneKeyResolver, paneKeyStamper := paneKeyPathSeams()

		tests := []struct {
			name     string
			paneID   string
			resolver *mockKeyResolver
			// A row whose subject includes reaching tmux for nothing sets stamper to
			// the poisoned pair; the loop supplies a plain one for any row that does not.
			stamper *recordingPaneStamper
			extra   []string
		}{
			{
				name:     "gone pane",
				paneID:   "%999",
				resolver: &mockKeyResolver{err: &tmux.CommandError{Stderr: "no such pane: %999"}},
			},
			{
				name:     "unset TMUX_PANE",
				paneID:   "",
				resolver: &mockKeyResolver{key: "tok123"},
			},
			{
				name:     "live pane carrying no token",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: ""},
			},
			{
				name:     "resolved token naming no entry",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: "tok123"},
			},
			{
				name:     "--pane-key naming no entry",
				paneID:   "%3",
				resolver: paneKeyResolver,
				stamper:  paneKeyStamper,
				extra:    []string{"--pane-key", "sess:0.1"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, hooksFile := hooksFileInTempDir(t)
				t.Setenv("TMUX_PANE", tt.paneID)
				before := seedHooksFile(t, hooksFile, seeded)

				stamper := tt.stamper
				if stamper == nil {
					stamper = &recordingPaneStamper{}
				}
				withHooksDeps(t, HooksDeps{KeyResolver: tt.resolver, PaneStamper: stamper})

				if _, err := runHookRm(t, tt.extra...); err == nil {
					t.Fatal("expected a non-zero exit, got nil")
				}
				if tt.stamper != nil {
					assertNoPaneTmuxCalls(t, tt.resolver, tt.stamper)
				}
				assertHooksFileUnchanged(t, hooksFile, before)
			})
		}
	})

	t.Run("it mints and stamps nothing on either path", func(t *testing.T) {
		paneKeyResolver, paneKeyStamper := paneKeyPathSeams()

		tests := []struct {
			name     string
			paneID   string
			resolver *mockKeyResolver
			// A row that must additionally guard the resolver sets stamper to the
			// poisoned pair; the loop supplies a plain one for any row that does not.
			stamper *recordingPaneStamper
			seeded  map[string]map[string]string
			extra   []string
			wantErr bool
		}{
			{
				name:     "successful resolved-token removal",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: "tok123"},
				seeded:   map[string]map[string]string{"tok123": {"on-resume": "npm start"}},
			},
			{
				name:     "live pane carrying no token",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: ""},
				seeded:   map[string]map[string]string{"tok123": {"on-resume": "npm start"}},
				wantErr:  true,
			},
			{
				name:     "--pane-key",
				paneID:   "",
				resolver: paneKeyResolver,
				stamper:  paneKeyStamper,
				seeded:   map[string]map[string]string{"tok123": {"on-resume": "npm start"}},
				extra:    []string{"--pane-key", "tok123"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, hooksFile := hooksFileInTempDir(t)
				t.Setenv("TMUX_PANE", tt.paneID)
				writeHooksJSON(t, hooksFile, tt.seeded)

				stamper := tt.stamper
				if stamper == nil {
					stamper = &recordingPaneStamper{}
				}
				minted := 0
				withHooksDeps(t, HooksDeps{
					KeyResolver: tt.resolver,
					PaneStamper: stamper,
					TokenMinter: func() (string, error) { minted++; return "tok000", nil },
				})

				_, err := runHookRm(t, tt.extra...)
				if tt.wantErr && err == nil {
					t.Fatal("expected a non-zero exit, got nil")
				}
				if !tt.wantErr && err != nil {
					t.Fatalf("hook rm: %v", err)
				}

				if minted != 0 {
					t.Errorf("mint count = %d, want 0: hook rm mints nothing", minted)
				}
				if tt.stamper != nil {
					assertNoPaneTmuxCalls(t, tt.resolver, tt.stamper)
				}
				if len(stamper.calls) != 0 {
					t.Errorf("set-option call count = %d, want 0: hook rm neither stamps nor unstamps: %+v",
						len(stamper.calls), stamper.calls)
				}
			})
		}
	})

	t.Run("it touches no dirty flag on either path", func(t *testing.T) {
		tests := []struct {
			name    string
			key     string
			wantErr bool
		}{
			{name: "successful removal", key: "tok123"},
			{name: "removing nothing", key: "tok404", wantErr: true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, hooksFile := hooksFileInTempDir(t)
				stateDir := t.TempDir()
				t.Setenv("PORTAL_STATE_DIR", stateDir)
				t.Setenv("TMUX_PANE", "%3")
				writeHooksJSON(t, hooksFile, map[string]map[string]string{
					"tok123": {"on-resume": "npm start"},
				})

				withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: tt.key}})

				_, err := runHookRm(t)
				if tt.wantErr != (err != nil) {
					t.Fatalf("hook rm error = %v, wantErr %v", err, tt.wantErr)
				}
				if saveRequestedExists(t, stateDir) {
					t.Error("hook rm touched save.requested")
				}
			})
		}
	})

	t.Run("it reports removing nothing as a plain error, not a usage error", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		writeHooksJSON(t, hooksFile, map[string]map[string]string{})

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: "tok123"}})

		out, err := runHookRm(t)
		if err == nil {
			t.Fatal("expected a non-zero exit, got nil")
		}

		// main's classify prints a non-silent, non-usage error to stderr and exits
		// 1; cobra's own SilenceUsage/SilenceErrors keep the streams clean.
		if _, ok := errors.AsType[*UsageError](err); ok {
			t.Errorf("error %v is a *UsageError; removing nothing is not a usage error", err)
		}
		if IsSilentExitError(err) {
			t.Error("error is a silent-exit error; its message would never reach stderr")
		}
		if strings.Contains(out, "Usage:") {
			t.Errorf("command output = %q, want no usage dump", out)
		}
	})
}
