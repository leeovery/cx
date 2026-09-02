package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/tmux"
)

// rmCase describes one `hook rm` route: how the pane resolves, what the store
// holds, and what the command is handed.
type rmCase struct {
	name   string
	paneID string
	// resolver answers the pane read on the resolved-token path. A paneKeyPath
	// row leaves it nil: runRmCase builds the poisoned pair for it instead.
	resolver    *mockKeyResolver
	paneKeyPath bool
	seeded      map[string]map[string]string
	extra       []string
	wantErr     bool
}

// rmOutcome is what a driven case left behind for its table to assert on.
type rmOutcome struct {
	hooksFile string
	before    []byte
	stamper   *recordingPaneStamper
	minted    int
}

// runRmCase stages tt, drives `hook rm`, and holds it to its expected exit.
// A --pane-key row gets its poisoned pane seams built here, per row, so a
// second such row cannot count against another's calls.
func runRmCase(t *testing.T, tt rmCase) rmOutcome {
	t.Helper()

	_, hooksFile := hooksFileInTempDir(t)
	t.Setenv("TMUX_PANE", tt.paneID)
	before := seedHooksFile(t, hooksFile, tt.seeded)

	resolver, stamper := tt.resolver, &recordingPaneStamper{}
	if tt.paneKeyPath {
		resolver, stamper = paneKeyPathSeams()
	}
	minted := 0
	withHooksDeps(t, HooksDeps{
		KeyResolver: resolver,
		PaneStamper: stamper,
		TokenMinter: func() (string, error) { minted++; return hookstest.SubjectSeedC, nil },
	})

	_, err := runHookRm(t, tt.extra...)
	if tt.wantErr != (err != nil) {
		t.Fatalf("hook rm error = %v, wantErr %v", err, tt.wantErr)
	}
	if tt.paneKeyPath {
		assertNoPaneTmuxCalls(t, resolver, stamper)
	}

	return rmOutcome{hooksFile: hooksFile, before: before, stamper: stamper, minted: minted}
}

func TestHooksRmExitsZeroOnlyWhenItRemoved(t *testing.T) {
	t.Run("it exits non-zero for a pane no live pane answers to", func(t *testing.T) {
		const stderr = "no such pane: %999"

		hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%999")

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{err: &tmux.CommandError{Stderr: stderr}}})

		_, err := runHookRm(t)
		if err == nil {
			t.Fatal("expected an error for a pane no live pane answers to, got nil")
		}
		if err.Error() != stderr {
			t.Errorf("error = %q, want tmux's own words %q unaltered", err.Error(), stderr)
		}
		if _, ok := errors.AsType[*tmux.CommandError](err); !ok {
			t.Errorf("error %v is not a recoverable *tmux.CommandError (errors.As failed)", err)
		}
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
	})

	t.Run("it exits non-zero when the resolved token has no entry", func(t *testing.T) {
		hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: hookstest.SubjectSeedA}})

		_, err := runHookRm(t)
		if err == nil {
			t.Fatal("expected an error when the resolved token has no entry, got nil")
		}
		want := fmt.Sprintf("no resume hook registered for %s", hookstest.SubjectSeedA)
		if err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("it exits non-zero when --pane-key names no entry", func(t *testing.T) {
		hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")

		// The poisoned pair doubles as the assertion here: a body that reached
		// the pane would surface the seam's error in place of this message.
		resolver, stamper := paneKeyPathSeams()
		withHooksDeps(t, HooksDeps{KeyResolver: resolver, PaneStamper: stamper})

		_, err := runHookRm(t, "--pane-key", hookstest.UnjudgeableSeedA)
		if err == nil {
			t.Fatal("expected an error when --pane-key names no entry, got nil")
		}
		want := fmt.Sprintf("no resume hook registered for %s", hookstest.UnjudgeableSeedA)
		if err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("it exits 0 and removes on the resolved-token path", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "%3")
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			hookstest.SubjectSeedA: {"on-resume": "claude --resume abc"},
			hookstest.SubjectSeedB: {"on-resume": "npm start"},
		})

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: hookstest.SubjectSeedA}})

		if _, err := runHookRm(t); err != nil {
			t.Fatalf("hook rm: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data[hookstest.SubjectSeedA]; ok {
			t.Error("expected the resolved token's entry to be removed")
		}
		if data[hookstest.SubjectSeedB]["on-resume"] != "npm start" {
			t.Errorf("hooks.json = %v, want the other pane's entry left in place", data)
		}
	})

	t.Run("it exits 0 and removes on the --pane-key path", func(t *testing.T) {
		_, hooksFile := hooksFileInTempDir(t)
		t.Setenv("TMUX_PANE", "")
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			hookstest.UnjudgeableSeedA: {"on-resume": "claude --resume abc"},
			hookstest.SubjectSeedB:     {"on-resume": "npm start"},
		})

		resolver, stamper := paneKeyPathSeams()
		withHooksDeps(t, HooksDeps{KeyResolver: resolver, PaneStamper: stamper})

		// An old-format key: the pass-through validates nothing, so removing one
		// by hand keeps working and keeps exiting 0.
		if _, err := runHookRm(t, "--pane-key", hookstest.UnjudgeableSeedA); err != nil {
			t.Fatalf("hook rm --pane-key: %v", err)
		}

		data := readHooksJSON(t, hooksFile)
		if _, ok := data[hookstest.UnjudgeableSeedA]; ok {
			t.Error("expected the verbatim key's entry to be removed")
		}
		assertNoPaneTmuxCalls(t, resolver, stamper)
	})

	t.Run("it leaves hooks.json byte-identical on every failing route", func(t *testing.T) {
		anotherPane := map[string]map[string]string{
			hookstest.SubjectSeedB: {"on-resume": "npm start"},
		}

		tests := []rmCase{
			{
				name:     "gone pane",
				paneID:   "%999",
				resolver: &mockKeyResolver{err: &tmux.CommandError{Stderr: "no such pane: %999"}},
				seeded:   anotherPane,
				wantErr:  true,
			},
			{
				name:     "unset TMUX_PANE",
				paneID:   "",
				resolver: &mockKeyResolver{key: hookstest.SubjectSeedA},
				seeded:   anotherPane,
				wantErr:  true,
			},
			{
				// Seeded with an empty-key entry too: the unresolved key must not
				// match one, which it could only do by reaching the store at all.
				name:     "live pane carrying no token",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: ""},
				seeded: map[string]map[string]string{
					"":                     {"on-resume": "an empty-key entry"},
					hookstest.SubjectSeedB: {"on-resume": "npm start"},
				},
				wantErr: true,
			},
			{
				name:     "resolved token naming no entry",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: hookstest.SubjectSeedA},
				seeded:   anotherPane,
				wantErr:  true,
			},
			{
				name:        "--pane-key naming no entry",
				paneID:      "%3",
				paneKeyPath: true,
				seeded:      anotherPane,
				extra:       []string{"--pane-key", hookstest.UnjudgeableSeedA},
				wantErr:     true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := runRmCase(t, tt)
				assertHooksFileUnchanged(t, got.hooksFile, got.before)
			})
		}
	})

	t.Run("it mints and stamps nothing on either path", func(t *testing.T) {
		onePane := map[string]map[string]string{hookstest.SubjectSeedA: {"on-resume": "npm start"}}

		tests := []rmCase{
			{
				name:     "successful resolved-token removal",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: hookstest.SubjectSeedA},
				seeded:   onePane,
			},
			{
				name:     "live pane carrying no token",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: ""},
				seeded:   onePane,
				wantErr:  true,
			},
			{
				name:        "--pane-key",
				paneID:      "",
				paneKeyPath: true,
				seeded:      onePane,
				extra:       []string{"--pane-key", hookstest.SubjectSeedA},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := runRmCase(t, tt)

				if got.minted != 0 {
					t.Errorf("mint count = %d, want 0: hook rm mints nothing", got.minted)
				}
				if len(got.stamper.calls) != 0 {
					t.Errorf("set-option call count = %d, want 0: hook rm neither stamps nor unstamps: %+v",
						len(got.stamper.calls), got.stamper.calls)
				}
			})
		}
	})

	t.Run("it touches no dirty flag on either path", func(t *testing.T) {
		onePane := map[string]map[string]string{hookstest.SubjectSeedA: {"on-resume": "npm start"}}

		tests := []rmCase{
			{
				name:     "successful resolved-token removal",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: hookstest.SubjectSeedA},
				seeded:   onePane,
			},
			{
				name:     "resolved token removing nothing",
				paneID:   "%3",
				resolver: &mockKeyResolver{key: hookstest.SubjectSeedD},
				seeded:   onePane,
				wantErr:  true,
			},
			{
				name:        "--pane-key removal",
				paneID:      "",
				paneKeyPath: true,
				seeded:      onePane,
				extra:       []string{"--pane-key", hookstest.SubjectSeedA},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				stateDir := t.TempDir()
				t.Setenv("PORTAL_STATE_DIR", stateDir)

				runRmCase(t, tt)

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

		withHooksDeps(t, HooksDeps{KeyResolver: &mockKeyResolver{key: hookstest.SubjectSeedA}})

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
