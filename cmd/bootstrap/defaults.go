package bootstrap

import (
	"log/slog"

	"github.com/leeovery/portal/internal/state"
)

// ServerSeam unions the two server capabilities NewWithDefaults needs:
// EnsureServer for step 1, and marker enumeration for the eager-signal default.
// Declared here so cmd/bootstrap need not import internal/tmux.
type ServerSeam interface {
	ServerBootstrapper
	state.ServerOptionLister
}

// Option overrides one step seam in a NewWithDefaults call.
type Option func(*defaultsConfig)

type defaultsConfig struct {
	hooks         HookRegistrar
	orphanSweeper OrphanSweeper
	saver         SaverBootstrapper
	restore       Restorer
	eagerSignaler EagerHydrateSignaler
	staleMarkers  MarkerCleaner
	sweeper       FIFOSweeper

	// Latch caller intent, not the field value: after defaulting, a NoOp is
	// indistinguishable from a caller-chosen one.
	restoreSet       bool
	eagerSignalerSet bool
}

// WithHooks supplies a real HookRegistrar for step 2.
func WithHooks(h HookRegistrar) Option {
	return func(c *defaultsConfig) { c.hooks = h }
}

// WithOrphanSweeper supplies a real OrphanSweeper for step 4.
func WithOrphanSweeper(s OrphanSweeper) Option {
	return func(c *defaultsConfig) { c.orphanSweeper = s }
}

// WithSaver supplies a real SaverBootstrapper for step 5.
func WithSaver(s SaverBootstrapper) Option {
	return func(c *defaultsConfig) { c.saver = s }
}

// WithRestore supplies a real Restorer for step 6. Setting it also flips the
// EagerSignaler default from NoOp to a real *EagerSignalCore.
func WithRestore(r Restorer) Option {
	return func(c *defaultsConfig) {
		c.restore = r
		c.restoreSet = true
	}
}

// WithEagerSignaler supplies a real EagerHydrateSignaler for step 7. Passing
// NoOpEagerHydrateSignaler{} suppresses the real default WithRestore triggers.
func WithEagerSignaler(s EagerHydrateSignaler) Option {
	return func(c *defaultsConfig) {
		c.eagerSignaler = s
		c.eagerSignalerSet = true
	}
}

// WithStaleMarkers supplies a real MarkerCleaner for step 9.
func WithStaleMarkers(m MarkerCleaner) Option {
	return func(c *defaultsConfig) { c.staleMarkers = m }
}

// WithSweeper supplies a real FIFOSweeper for step 10.
func WithSweeper(s FIFOSweeper) Option {
	return func(c *defaultsConfig) { c.sweeper = s }
}

// NewWithDefaults constructs an *Orchestrator with NoOp defaults for every
// degradable step seam, so no seam is ever nil. server and restoring are
// mandatory — their steps are fatal-on-failure and have no NoOp form. logger
// may be nil; stateDir matters only when the eager signaler defaults to real.
func NewWithDefaults(
	server ServerSeam,
	stateDir string,
	logger *slog.Logger,
	restoring RestoringMarker,
	opts ...Option,
) *Orchestrator {
	cfg := defaultsConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.hooks == nil {
		cfg.hooks = NoOpHooks{}
	}
	if cfg.orphanSweeper == nil {
		cfg.orphanSweeper = NoOpOrphanSweeper{}
	}
	if cfg.saver == nil {
		cfg.saver = NoOpSaver{}
	}
	if cfg.restore == nil {
		cfg.restore = NoOpRestorer{}
	}
	if !cfg.eagerSignalerSet {
		if cfg.restoreSet {
			cfg.eagerSignaler = &EagerSignalCore{
				Markers:  server,
				StateDir: stateDir,
				Signaler: state.DefaultFIFOSignaler{},
				Logger:   logger,
			}
		} else {
			cfg.eagerSignaler = NoOpEagerHydrateSignaler{}
		}
	}
	if cfg.staleMarkers == nil {
		cfg.staleMarkers = NoOpMarkerCleaner{}
	}
	if cfg.sweeper == nil {
		cfg.sweeper = NoOpFIFOSweeper{}
	}

	return &Orchestrator{
		Server:        server,
		Hooks:         cfg.hooks,
		Restoring:     restoring,
		OrphanSweeper: cfg.orphanSweeper,
		Saver:         cfg.saver,
		Restore:       cfg.restore,
		EagerSignaler: cfg.eagerSignaler,
		StaleMarkers:  cfg.staleMarkers,
		Sweeper:       cfg.sweeper,
		Logger:        logger,
	}
}
