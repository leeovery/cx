package bootstrap_test

import (
	"context"
	"testing"

	"github.com/leeovery/portal/cmd/bootstrap"
)

type stubServerSeam struct{}

func (stubServerSeam) EnsureServer() (bool, error)           { return false, nil }
func (stubServerSeam) ShowAllServerOptions() (string, error) { return "", nil }

type stubRestoringMarker struct{}

func (stubRestoringMarker) Set() error   { return nil }
func (stubRestoringMarker) Clear() error { return nil }

type stubRestorer struct{}

func (stubRestorer) Restore() (bool, error) { return false, nil }

func TestNewWithDefaults_DefaultsAllDegradableStepsToNoOp(t *testing.T) {
	o := bootstrap.NewWithDefaults(stubServerSeam{}, "", nil, stubRestoringMarker{})

	if _, ok := o.Hooks.(bootstrap.NoOpHooks); !ok {
		t.Errorf("Hooks type = %T; want bootstrap.NoOpHooks", o.Hooks)
	}
	if _, ok := o.OrphanSweeper.(bootstrap.NoOpOrphanSweeper); !ok {
		t.Errorf("OrphanSweeper type = %T; want bootstrap.NoOpOrphanSweeper", o.OrphanSweeper)
	}
	if _, ok := o.Saver.(bootstrap.NoOpSaver); !ok {
		t.Errorf("Saver type = %T; want bootstrap.NoOpSaver", o.Saver)
	}
	if _, ok := o.Restore.(bootstrap.NoOpRestorer); !ok {
		t.Errorf("Restore type = %T; want bootstrap.NoOpRestorer", o.Restore)
	}
	if _, ok := o.EagerSignaler.(bootstrap.NoOpEagerHydrateSignaler); !ok {
		t.Errorf("EagerSignaler type = %T; want bootstrap.NoOpEagerHydrateSignaler (Restore is NoOp → eager step is vacuous)", o.EagerSignaler)
	}
	if _, ok := o.StaleMarkers.(bootstrap.NoOpMarkerCleaner); !ok {
		t.Errorf("StaleMarkers type = %T; want bootstrap.NoOpMarkerCleaner", o.StaleMarkers)
	}
	if _, ok := o.Sweeper.(bootstrap.NoOpFIFOSweeper); !ok {
		t.Errorf("Sweeper type = %T; want bootstrap.NoOpFIFOSweeper", o.Sweeper)
	}
}

func TestNewWithDefaults_WiresPositionalSeams(t *testing.T) {
	server := stubServerSeam{}
	restoring := stubRestoringMarker{}

	o := bootstrap.NewWithDefaults(server, "", nil, restoring)

	if o.Server != server {
		t.Errorf("Server = %v; want stubServerSeam{}", o.Server)
	}
	if o.Restoring != restoring {
		t.Errorf("Restoring = %v; want stubRestoringMarker{}", o.Restoring)
	}
	if o.Logger != nil {
		t.Errorf("Logger = %v; want nil (preserved verbatim)", o.Logger)
	}
}

func TestNewWithDefaults_HonorsAllWithOptions(t *testing.T) {
	hooks := stubHooks{}
	orphanSweeper := stubOrphanSweeper{}
	saver := stubSaver{}
	restore := stubRestorer{}
	eager := stubEager{}
	staleMarkers := stubMarkerCleaner{}
	sweeper := stubSweeper{}

	o := bootstrap.NewWithDefaults(stubServerSeam{}, "", nil, stubRestoringMarker{},
		bootstrap.WithHooks(hooks),
		bootstrap.WithOrphanSweeper(orphanSweeper),
		bootstrap.WithSaver(saver),
		bootstrap.WithRestore(restore),
		bootstrap.WithEagerSignaler(eager),
		bootstrap.WithStaleMarkers(staleMarkers),
		bootstrap.WithSweeper(sweeper),
	)

	if o.Hooks != hooks {
		t.Errorf("Hooks = %v; want stubHooks{}", o.Hooks)
	}
	if o.OrphanSweeper != orphanSweeper {
		t.Errorf("OrphanSweeper = %v; want stubOrphanSweeper{}", o.OrphanSweeper)
	}
	if o.Saver != saver {
		t.Errorf("Saver = %v; want stubSaver{}", o.Saver)
	}
	if o.Restore != restore {
		t.Errorf("Restore = %v; want stubRestorer{}", o.Restore)
	}
	if o.EagerSignaler != eager {
		t.Errorf("EagerSignaler = %v; want stubEager{}", o.EagerSignaler)
	}
	if o.StaleMarkers != staleMarkers {
		t.Errorf("StaleMarkers = %v; want stubMarkerCleaner{}", o.StaleMarkers)
	}
	if o.Sweeper != sweeper {
		t.Errorf("Sweeper = %v; want stubSweeper{}", o.Sweeper)
	}
}

func TestNewWithDefaults_EagerSignalerDefaultsToRealWhenRestoreReal(t *testing.T) {
	o := bootstrap.NewWithDefaults(stubServerSeam{}, t.TempDir(), nil, stubRestoringMarker{},
		bootstrap.WithRestore(stubRestorer{}),
	)

	core, ok := o.EagerSignaler.(*bootstrap.EagerSignalCore)
	if !ok {
		t.Fatalf("EagerSignaler type = %T; want *bootstrap.EagerSignalCore (real adapter when WithRestore is supplied)", o.EagerSignaler)
	}
	if core.Markers == nil {
		t.Errorf("EagerSignalCore.Markers is nil; want the helper's server seam threaded through")
	}
	if core.Signaler == nil {
		t.Errorf("EagerSignalCore.Signaler is nil; want state.DefaultFIFOSignaler{}")
	}
}

func TestNewWithDefaults_EagerSignalerDefaultsToNoOpWhenRestoreUnset(t *testing.T) {
	o := bootstrap.NewWithDefaults(stubServerSeam{}, "", nil, stubRestoringMarker{})

	if _, ok := o.EagerSignaler.(bootstrap.NoOpEagerHydrateSignaler); !ok {
		t.Errorf("EagerSignaler type = %T; want bootstrap.NoOpEagerHydrateSignaler (Restore is NoOp → eager step vacuous)", o.EagerSignaler)
	}
}

func TestNewWithDefaults_EagerSignalerExplicitOptOutHonored(t *testing.T) {
	o := bootstrap.NewWithDefaults(stubServerSeam{}, t.TempDir(), nil, stubRestoringMarker{},
		bootstrap.WithRestore(stubRestorer{}),
		bootstrap.WithEagerSignaler(bootstrap.NoOpEagerHydrateSignaler{}),
	)

	if _, ok := o.EagerSignaler.(bootstrap.NoOpEagerHydrateSignaler); !ok {
		t.Errorf("EagerSignaler type = %T; want bootstrap.NoOpEagerHydrateSignaler (explicit opt-out must win over WithRestore)", o.EagerSignaler)
	}
}

func TestNewWithDefaults_RunCallableSmokeTest(t *testing.T) {
	o := bootstrap.NewWithDefaults(stubServerSeam{}, "", nil, stubRestoringMarker{})

	_, _, err := o.Run(context.Background())
	if err != nil {
		t.Errorf("Run returned error: %v; want nil (every default NoOp step succeeds)", err)
	}
}

type stubHooks struct{}

func (stubHooks) RegisterPortalHooks() error { return nil }

type stubOrphanSweeper struct{}

func (stubOrphanSweeper) SweepOrphanDaemons() error { return nil }

type stubSaver struct{}

func (stubSaver) EnsureSaver() error { return nil }

type stubEager struct{}

func (stubEager) EagerSignalHydrate() error { return nil }

type stubMarkerCleaner struct{}

func (stubMarkerCleaner) CleanStaleMarkers() error { return nil }

type stubSweeper struct{}

func (stubSweeper) Sweep() error { return nil }
