package bootstrap

import (
	"log/slog"

	"github.com/leeovery/portal/internal/state"
)

// ServerSeam is declared here so this package need not import internal/tmux
// for the concrete client.
type ServerSeam interface {
	ServerBootstrapper
	state.ServerOptionLister
}

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

func WithHooks(h HookRegistrar) Option {
	return func(c *defaultsConfig) { c.hooks = h }
}

func WithOrphanSweeper(s OrphanSweeper) Option {
	return func(c *defaultsConfig) { c.orphanSweeper = s }
}

func WithSaver(s SaverBootstrapper) Option {
	return func(c *defaultsConfig) { c.saver = s }
}

// WithRestore also flips the EagerSignaler default from NoOp to a real
// *EagerSignalCore.
func WithRestore(r Restorer) Option {
	return func(c *defaultsConfig) {
		c.restore = r
		c.restoreSet = true
	}
}

// WithEagerSignaler accepts NoOpEagerHydrateSignaler{} to suppress the real
// default that WithRestore triggers.
func WithEagerSignaler(s EagerHydrateSignaler) Option {
	return func(c *defaultsConfig) {
		c.eagerSignaler = s
		c.eagerSignalerSet = true
	}
}

func WithStaleMarkers(m MarkerCleaner) Option {
	return func(c *defaultsConfig) { c.staleMarkers = m }
}

func WithSweeper(s FIFOSweeper) Option {
	return func(c *defaultsConfig) { c.sweeper = s }
}

// NewWithDefaults fills every degradable step seam with a NoOp, so no seam is
// ever nil. server and restoring are mandatory — their steps are fatal and have
// no NoOp form. stateDir is read only when the eager signaler defaults to real.
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
