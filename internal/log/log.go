package log

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
)

// swapHandler forwards to a replaceable inner handler. WithAttrs/WithGroup must
// not pre-bind onto the inner handler of the moment — that would freeze a cached
// logger to a stale delegate — so they record the operation and replay it against
// the live handler on every Enabled/Handle.
type swapHandler struct {
	// Pointer, not value: derived handlers must share this one cell, and
	// atomic.Pointer must not be copied.
	inner *atomic.Pointer[slog.Handler]
	mods  []handlerMod
}

// handlerMod is one deferred operation; exactly one of attrs / group is set.
type handlerMod struct {
	attrs []slog.Attr
	group string
}

func (s *swapHandler) applyMods(h slog.Handler) slog.Handler {
	for _, m := range s.mods {
		if m.group != "" {
			h = h.WithGroup(m.group)
			continue
		}
		h = h.WithAttrs(m.attrs)
	}
	return h
}

func (s *swapHandler) load() slog.Handler {
	return s.applyMods(*s.inner.Load())
}

func (s *swapHandler) store(h slog.Handler) {
	s.inner.Store(&h)
}

func (s *swapHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return s.load().Enabled(ctx, level)
}

func (s *swapHandler) Handle(ctx context.Context, r slog.Record) error {
	return s.load().Handle(ctx, r)
}

func (s *swapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return s.derive(handlerMod{attrs: attrs})
}

func (s *swapHandler) WithGroup(name string) slog.Handler {
	return s.derive(handlerMod{group: name})
}

func (s *swapHandler) derive(m handlerMod) *swapHandler {
	mods := make([]handlerMod, len(s.mods)+1)
	copy(mods, s.mods)
	mods[len(s.mods)] = m
	return &swapHandler{inner: s.inner, mods: mods}
}

var swap = newSwapHandler()

var root = slog.New(swap)

// newSwapHandler installs a pre-Init default so root is usable before Init runs.
func newSwapHandler() *swapHandler {
	s := &swapHandler{inner: &atomic.Pointer[slog.Handler]{}}
	s.store(defaultHandler())
	return s
}

func defaultHandler() slog.Handler {
	return slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
}

func setHandler(h slog.Handler) {
	swap.store(h)
}

// currentHandler reads the pinned inner handler without applying any mod chain.
func currentHandler() slog.Handler {
	return *swap.inner.Load()
}

// For returns a component-bound child logger. It is safe to call before Init and
// never returns nil. An empty component is accepted: the component taxonomy is
// convention, not a runtime guard.
func For(component string) *slog.Logger {
	return root.With("component", component)
}
