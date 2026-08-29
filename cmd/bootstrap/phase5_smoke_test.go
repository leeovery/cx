package bootstrap_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestPhase5_OrchestratorEndToEndSmoke(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-p5-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	ts.Run(t, "new-session", "-d", "-s", "alpha")
	ts.WaitForSession(t, "alpha", 2*time.Second)

	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		Hooks: &bootstrapadapter.HookRegistrar{Client: client},
	})

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := ts.Run(t, "list-sessions", "-F", "#{session_name}")
	if !strings.Contains(out, "alpha") {
		t.Errorf("expected alpha in list-sessions; got %q", out)
	}

	if val, found, err := client.TryGetServerOption(state.RestoringMarkerName); err != nil {
		t.Fatalf("TryGetServerOption: %v", err)
	} else if found {
		t.Errorf("@portal-restoring still set; value=%q", val)
	}

	type hookExpect struct {
		event     string
		substring string
	}
	wantHooks := []hookExpect{
		{"session-created", "portal state notify"},
		{"session-closed", "portal state commit-now"},
		{"session-renamed", "portal state notify"},
		{"window-linked", "portal state notify"},
		{"window-unlinked", "portal state notify"},
		{"window-layout-changed", "portal state notify"},
		{"pane-focus-out", "portal state notify"},
		{"client-attached", "portal state signal-hydrate"},
		{"client-session-changed", "portal state signal-hydrate"},
	}
	for _, want := range wantHooks {
		out, err := ts.TryRun("show-hooks", "-g", want.event)
		if err != nil {
			t.Errorf("show-hooks -g %s: %v\n%s", want.event, err, out)
			continue
		}
		if !strings.Contains(out, want.substring) {
			t.Errorf("hook on %s missing %q; got %q", want.event, want.substring, out)
		}
	}
}
