package cmd

import (
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

func TestHookSeams(t *testing.T) {
	t.Run("it resolves every seam to its production default when nothing is injected", func(t *testing.T) {
		hooksDeps = nil

		seams := hookSeams()

		if _, ok := seams.KeyResolver.(*tmux.Client); !ok {
			t.Errorf("KeyResolver = %T, want the production *tmux.Client", seams.KeyResolver)
		}
		if _, ok := seams.PaneLister.(*tmux.Client); !ok {
			t.Errorf("PaneLister = %T, want the production *tmux.Client", seams.PaneLister)
		}
		if _, ok := seams.PaneStamper.(*tmux.Client); !ok {
			t.Errorf("PaneStamper = %T, want the production *tmux.Client", seams.PaneStamper)
		}
		if seams.TokenMinter == nil {
			t.Fatal("TokenMinter = nil, want the production minter")
		}
		token, err := seams.TokenMinter()
		if err != nil {
			t.Fatalf("production TokenMinter: %v", err)
		}
		if token == "" {
			t.Error("production TokenMinter minted an empty token")
		}
	})

	t.Run("it resolves each seam to its injected fake", func(t *testing.T) {
		resolver := &mockKeyResolver{key: "tok123"}
		lister := &recordingPaneHookLister{}
		stamper := &recordingPaneStamper{}
		hooksDeps = &HooksDeps{
			KeyResolver: resolver,
			PaneLister:  lister,
			PaneStamper: stamper,
			TokenMinter: func() (string, error) { return "tok000", nil },
		}
		t.Cleanup(func() { hooksDeps = nil })

		seams := hookSeams()

		if seams.KeyResolver != HookKeyResolver(resolver) {
			t.Errorf("KeyResolver = %v, want the injected fake", seams.KeyResolver)
		}
		if seams.PaneLister != PaneHookLister(lister) {
			t.Errorf("PaneLister = %v, want the injected fake", seams.PaneLister)
		}
		if seams.PaneStamper != PaneOptionSetter(stamper) {
			t.Errorf("PaneStamper = %v, want the injected fake", seams.PaneStamper)
		}
		token, err := seams.TokenMinter()
		if err != nil {
			t.Fatalf("injected TokenMinter: %v", err)
		}
		if token != "tok000" {
			t.Errorf("TokenMinter minted %q, want the injected fake's %q", token, "tok000")
		}
	})

	t.Run("it fills the production default for every seam a test left unset", func(t *testing.T) {
		resolver := &mockKeyResolver{key: "tok123"}
		hooksDeps = &HooksDeps{KeyResolver: resolver}
		t.Cleanup(func() { hooksDeps = nil })

		seams := hookSeams()

		if seams.KeyResolver != HookKeyResolver(resolver) {
			t.Errorf("KeyResolver = %v, want the injected fake", seams.KeyResolver)
		}
		if _, ok := seams.PaneLister.(*tmux.Client); !ok {
			t.Errorf("PaneLister = %T, want the production *tmux.Client", seams.PaneLister)
		}
		if _, ok := seams.PaneStamper.(*tmux.Client); !ok {
			t.Errorf("PaneStamper = %T, want the production *tmux.Client", seams.PaneStamper)
		}
		if seams.TokenMinter == nil {
			t.Error("TokenMinter = nil, want the production minter")
		}
	})
}

// gonePaneCommander refuses the existence probe with tmux's own stderr for a
// pane that does not exist, so the true error chain runs — the client's own
// wrap around those words — rather than a shape a fake resolver invented. It
// carries no *exec.ExitError, so the rendering omits the exit-status segment
// a real refusal adds between the argv and the stderr.
type gonePaneCommander struct{ paneID string }

func (g *gonePaneCommander) Run(args ...string) (string, error) {
	return "", &tmux.CommandError{
		Args:   args,
		Stderr: "no such pane: " + g.paneID,
	}
}

func (g *gonePaneCommander) RunRaw(args ...string) (string, error) {
	return g.Run(args...)
}

var _ tmux.Commander = (*gonePaneCommander)(nil)

func TestGonePaneErrorCarriesOnePortalClause(t *testing.T) {
	// One Portal-authored clause, then tmux's own words unaltered. Both verbs
	// resolve the key through the same call, so both are pinned to it.
	const want = `no pane answers to "%999": tmux show-options -p -t %999: no such pane: %999`

	run := map[string]func(t *testing.T) error{
		"hook set": func(t *testing.T) error {
			_, err := runHookSet(t, "npm start")
			return err
		},
		"hook rm": func(t *testing.T) error {
			_, err := runHookRm(t)
			return err
		},
	}

	for verb, drive := range run {
		t.Run("it reports a gone pane in one Portal clause plus tmux's words for "+verb, func(t *testing.T) {
			hooksFileInTempDir(t)
			t.Setenv("TMUX_PANE", "%999")

			hooksDeps = &HooksDeps{KeyResolver: tmux.NewClient(&gonePaneCommander{paneID: "%999"})}
			t.Cleanup(func() { hooksDeps = nil })

			err := drive(t)
			if err == nil {
				t.Fatalf("%s: expected an error for a pane no live pane answers to, got nil", verb)
			}
			if err.Error() != want {
				t.Errorf("%s error = %q, want %q", verb, err.Error(), want)
			}
		})
	}
}
