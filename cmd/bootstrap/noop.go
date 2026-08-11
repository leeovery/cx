package bootstrap

// No-op step seams for the Orchestrator steps that may degrade and continue.
// The fatal steps (Server, RestoringMarker) deliberately have none — reaching
// for a default there would silently violate the bootstrap contract.

// NoOpHooks is a HookRegistrar that always succeeds.
type NoOpHooks struct{}

func (NoOpHooks) RegisterPortalHooks() error { return nil }

// NoOpOrphanSweeper is an OrphanSweeper that always succeeds.
type NoOpOrphanSweeper struct{}

func (NoOpOrphanSweeper) SweepOrphanDaemons() error { return nil }

// NoOpSaver is a SaverBootstrapper that always succeeds.
type NoOpSaver struct{}

func (NoOpSaver) EnsureSaver() error { return nil }

// NoOpRestorer is a Restorer that always reports the happy path.
type NoOpRestorer struct{}

func (NoOpRestorer) Restore() (bool, error) { return false, nil }

// NoOpEagerHydrateSignaler is an EagerHydrateSignaler that always succeeds.
type NoOpEagerHydrateSignaler struct{}

func (NoOpEagerHydrateSignaler) EagerSignalHydrate() error { return nil }

// NoOpMarkerCleaner is a MarkerCleaner that always succeeds.
type NoOpMarkerCleaner struct{}

func (NoOpMarkerCleaner) CleanStaleMarkers() error { return nil }

// NoOpFIFOSweeper is a FIFOSweeper that always succeeds.
type NoOpFIFOSweeper struct{}

func (NoOpFIFOSweeper) Sweep() error { return nil }
