package bootstrap

// The fatal steps (Server, RestoringMarker) deliberately have no NoOp form:
// defaulting one would silently violate the bootstrap contract.

type NoOpHooks struct{}

func (NoOpHooks) RegisterPortalHooks() error { return nil }

type NoOpOrphanSweeper struct{}

func (NoOpOrphanSweeper) SweepOrphanDaemons() error { return nil }

type NoOpSaver struct{}

func (NoOpSaver) EnsureSaver() error { return nil }

type NoOpRestorer struct{}

func (NoOpRestorer) Restore() (bool, error) { return false, nil }

type NoOpEagerHydrateSignaler struct{}

func (NoOpEagerHydrateSignaler) EagerSignalHydrate() error { return nil }

type NoOpMarkerCleaner struct{}

func (NoOpMarkerCleaner) CleanStaleMarkers() error { return nil }

type NoOpFIFOSweeper struct{}

func (NoOpFIFOSweeper) Sweep() error { return nil }
