package bootstrap_test

import (
	"testing"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/tmux"
)

// The restore adapters below are never run — what is under test is which eager
// signaler the builder picks from a real-vs-nil RestoreAdapter — so their inner
// orchestrator is the no-staged-binary constructor, with nothing to arm.
func TestBuildIntegrationOrchestrator_EagerSignalerDefaultsToRealWhenRestoreReal(t *testing.T) {
	t.Setenv("PORTAL_STATE_DIR", t.TempDir())

	client := &tmux.Client{}

	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		Restore: &bootstrapadapter.RestoreAdapter{Inner: restoretest.NewFakeExeOrchestrator(t, nil, "", nil)},
	})

	if _, ok := o.EagerSignaler.(*bootstrap.EagerSignalCore); !ok {
		t.Errorf("EagerSignaler type = %T; want *bootstrap.EagerSignalCore (real adapter when RestoreAdapter is real)", o.EagerSignaler)
	}
}

func TestBuildIntegrationOrchestrator_EagerSignalerDefaultsToNoOpWhenRestoreNil(t *testing.T) {
	client := &tmux.Client{}

	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{})

	if _, ok := o.EagerSignaler.(bootstrap.NoOpEagerHydrateSignaler); !ok {
		t.Errorf("EagerSignaler type = %T; want bootstrap.NoOpEagerHydrateSignaler (NoOp when RestoreAdapter is nil)", o.EagerSignaler)
	}
}

func TestBuildIntegrationOrchestrator_EagerSignalerExplicitOptOutHonoured(t *testing.T) {
	t.Setenv("PORTAL_STATE_DIR", t.TempDir())
	client := &tmux.Client{}

	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		Restore:       &bootstrapadapter.RestoreAdapter{Inner: restoretest.NewFakeExeOrchestrator(t, nil, "", nil)},
		EagerSignaler: bootstrap.NoOpEagerHydrateSignaler{},
	})

	if _, ok := o.EagerSignaler.(bootstrap.NoOpEagerHydrateSignaler); !ok {
		t.Errorf("EagerSignaler type = %T; want explicit bootstrap.NoOpEagerHydrateSignaler{} (manual-harness opt-out must be honoured)", o.EagerSignaler)
	}
}
