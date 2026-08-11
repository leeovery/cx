package tui

import (
	"context"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/spawn"
)

// 64 comfortably exceeds any realistic marked-set, so the terminal event's
// naked send always finds buffer space or a waiting receiver.
const burstProgressBufferSize = 64

// burstProgress is the picker-burst channel event. Exactly one terminal event
// (Done=true) is sent, last, before the channel closes; it carries either Gone
// (pre-flight abort — nothing spawned) or the burst outcome.
type burstProgress struct {
	DoneCount int
	Total     int

	Done       bool
	Batch      string
	Results    []spawn.WindowResult
	Identity   spawn.Identity
	Resolution spawn.Resolution
	Gone       []string
	Err        error
}

type spawnProgressMsg struct {
	Done  int
	Total int
}

type spawnCompleteMsg struct {
	Batch      string
	Results    []spawn.WindowResult
	Identity   spawn.Identity
	Resolution spawn.Resolution
	Err        error
}

type spawnAbortMsg struct {
	Gone []string
}

type burstChannelClosedMsg struct{}

type burstProgressPipe struct {
	ch     chan burstProgress
	cancel context.CancelFunc
}

func newBurstProgressPipe() *burstProgressPipe {
	return &burstProgressPipe{ch: make(chan burstProgress, burstProgressBufferSize)}
}

func (p *burstProgressPipe) start(ctx context.Context, run func(ctx context.Context, emit func(burstProgress))) {
	go func() {
		defer close(p.ch)
		run(ctx, func(ev burstProgress) { p.send(ctx, ev) })
	}()
}

// The terminal event must be a naked send: a ctx-guarded terminal send raced
// ctx.Done on cancel and left the picker permanently input-locked. It cannot
// block — it is the last send before the deferred close and a receiver is
// consuming. Progress events are advisory and stay droppable on cancel.
func (p *burstProgressPipe) send(ctx context.Context, ev burstProgress) {
	if ev.Done {
		p.ch <- ev
		return
	}
	select {
	case p.ch <- ev:
	case <-ctx.Done():
	}
}

func (p *burstProgressPipe) receiver() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-p.ch
		if !ok {
			return burstChannelClosedMsg{}
		}
		if ev.Done {
			if len(ev.Gone) > 0 {
				return spawnAbortMsg{Gone: ev.Gone}
			}
			return spawnCompleteMsg{
				Batch:      ev.Batch,
				Results:    ev.Results,
				Identity:   ev.Identity,
				Resolution: ev.Resolution,
				Err:        ev.Err,
			}
		}
		return spawnProgressMsg{Done: ev.DoneCount, Total: ev.Total}
	}
}

type burstRunner struct {
	burster       *spawn.Burster
	ackChannel    spawn.AckChannelFull
	sessionExists func(string) bool
	external      []string
	trigger       string
	identity      spawn.Identity
	resolution    spawn.Resolution
}

func (r burstRunner) run(ctx context.Context, emit func(burstProgress)) {
	all := append(slices.Clone(r.external), r.trigger)
	if gone := spawn.PreflightMissing(all, r.sessionExists); len(gone) > 0 {
		emit(burstProgress{Done: true, Gone: gone})
		return
	}
	batch, results, err := r.burster.Run(ctx, spawn.AttachSurfaces(r.external), nil, func(done, total int) {
		emit(burstProgress{DoneCount: done, Total: total})
	})
	_ = r.ackChannel.Clean(batch)
	emit(burstProgress{
		Done:       true,
		Batch:      batch,
		Results:    results,
		Identity:   r.identity,
		Resolution: r.resolution,
		Err:        err,
	})
}

// WithSessionExists wires the burst pre-flight has-session probe.
// Nil-tolerant; production passes client.HasSession.
func WithSessionExists(fn func(string) bool) Option {
	return func(m *Model) { m.sessionExists = fn }
}

// WithAckChannel wires the burst token-ack channel. Nil-tolerant; production
// passes spawn.NewServerOptionAckChannel(client, client).
func WithAckChannel(ack spawn.AckChannelFull) Option {
	return func(m *Model) { m.ackChannel = ack }
}

// WithSpawnExe wires the burst executable resolver. Nil-tolerant; production
// passes os.Executable.
func WithSpawnExe(exe spawn.ExecutableResolver) Option {
	return func(m *Model) { m.spawnExe = exe }
}

// WithSpawnGetenv wires the burst PATH reader for argv composition.
// Nil-tolerant; production passes os.Getenv.
func WithSpawnGetenv(fn func(string) string) Option {
	return func(m *Model) { m.spawnGetenv = fn }
}

func (m Model) burstAllConfirmed(msg spawnCompleteMsg) bool {
	_, failed := spawn.PartitionResults(msg.Results)
	return msg.Err == nil && len(msg.Results) == len(m.burstExternal) && len(failed) == 0
}

func (m *Model) resetBurstState() {
	m.burstPending = false
	m.burstPipe = nil
	m.burstCancel = nil
	m.burstTotal = 0
	m.burstDone = 0
	m.burstCancelled = false
}

// WithInitialBurstOpening seeds the in-burst Opening band at construction —
// the capture-harness entry point; no goroutine, pipe, or cancel is wired.
// The zero value is a no-op.
func WithInitialBurstOpening(done, total int) Option {
	return func(m *Model) {
		if done == 0 && total == 0 {
			return
		}
		m.burstPending = true
		m.burstDone = done
		m.burstTotal = total
	}
}

// BurstPending reports whether an async spawn burst is in flight, for testing.
func (m Model) BurstPending() bool { return m.burstPending }

// BurstExternal returns the net-N external set in list order, for testing.
func (m Model) BurstExternal() []string { return m.burstExternal }

// BurstTrigger returns the burst's self-attach target, for testing.
func (m Model) BurstTrigger() string { return m.burstTrigger }

// BurstTotal returns N, the full marked-set size, for testing.
func (m Model) BurstTotal() int { return m.burstTotal }

// BurstDone returns the per-window confirm counter, for testing. It advances
// 0…N-1, never reaching N: the trigger self-attaches silently.
func (m Model) BurstDone() int { return m.burstDone }

// cancelBurst deliberately keeps burstPending true: the goroutine still sends
// its terminal event (delivered reliably via the naked send), so the picker
// stays input-locked until that event lands and never wedges.
func (m Model) cancelBurst() (tea.Model, tea.Cmd) {
	if m.burstCancel != nil {
		m.burstCancel()
	}
	m.burstCancelled = true
	// Only the capture harness sets pending without a pipe; guard the nil
	// dereference there.
	if m.burstPipe == nil {
		return m, nil
	}
	return m, m.burstPipe.receiver()
}

// Walks Items(), not VisibleItems, so marks survive an applied filter;
// de-duped because a multi-tag By-Tag session spans one row per tag.
func (m Model) orderedMarkedSessions() []string {
	var ordered []string
	seen := make(map[string]struct{}, len(m.selectedSessions))
	for _, item := range m.sessionList.Items() {
		si, ok := item.(SessionItem)
		if !ok {
			continue
		}
		name := si.Session.Name
		if _, marked := m.selectedSessions[name]; !marked {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		ordered = append(ordered, name)
	}
	return ordered
}

// While detection is in flight the Enter defers (and dispatches detection if
// it never ran, so the defer can resolve). No snapshot is stashed — the marked
// set is re-derived at resolution so toggles during the window are honoured.
func (m Model) beginBurst(ordered []string) (Model, tea.Cmd) {
	if !m.detectResolved {
		m.pendingBurstEnter = true
		var cmd tea.Cmd
		if !m.detectDispatched {
			cmd = (&m).maybeDispatchDetectionCmd()
		}
		return m, cmd
	}
	return m.decideBurst(ordered)
}

// Branches on the cached resolution, not IsNull(): a recognised-but-undriven
// terminal is non-NULL yet unsupported. The unsupported arm pre-flights first
// so a gone session surfaces and is pruned rather than the banner masking it.
func (m Model) decideBurst(ordered []string) (Model, tea.Cmd) {
	m.pendingBurstEnter = false
	// Empty set: no-op — dispatchBurst would panic indexing the trigger.
	if len(ordered) == 0 {
		return m, nil
	}
	if m.DetectUnsupported() {
		if m.sessionExists != nil {
			if gone := spawn.PreflightMissing(ordered, m.sessionExists); len(gone) > 0 {
				return m.handlePreflightAbort(spawnAbortMsg{Gone: gone}), nil
			}
		}
		m.emitUnsupportedNoop(m.detectIdentity)
		m.setFlash(unsupportedFlashText(m.detectIdentity))
		return m, nil
	}
	return m.dispatchBurst(ordered)
}

// The ⚠ glyph is added by the warning notice band, not the message text.
func unsupportedFlashText(id spawn.Identity) string {
	return spawn.UnsupportedNoopMessage(id)
}

// dispatchBurst reads the adapter/resolution from the detection-time cache,
// never a fresh re-resolve: re-resolving was a TOCTOU nil-adapter panic (the
// config-script recipe re-stats its script on every resolve, so a script
// deleted between detection and Enter slipped past the DetectUnsupported gate
// and nil-panicked the burst goroutine). The nil-adapter guard routes an
// inconsistent resolve to the unsupported no-op instead.
func (m Model) dispatchBurst(ordered []string) (Model, tea.Cmd) {
	external, trigger := spawn.SplitNetN(ordered)

	adapter, resolution := m.detectAdapter, m.detectResolution
	if adapter == nil {
		m.emitUnsupportedNoop(m.detectIdentity)
		m.setFlash(unsupportedFlashText(m.detectIdentity))
		return m, nil
	}
	burster := spawn.NewBurster(adapter, m.ackChannel, m.spawnExe, m.spawnGetenv)

	ctx, cancel := context.WithCancel(context.Background())
	pipe := newBurstProgressPipe()
	pipe.cancel = cancel

	m.burstPipe = pipe
	m.burstCancel = cancel
	m.burstPending = true
	m.burstTrigger = trigger
	m.burstExternal = external
	m.burstTotal = len(ordered)
	m.burstDone = 0

	runner := burstRunner{
		burster:       burster,
		ackChannel:    m.ackChannel,
		sessionExists: m.sessionExists,
		external:      external,
		trigger:       trigger,
		identity:      m.detectIdentity,
		resolution:    resolution,
	}
	pipe.start(ctx, runner.run)
	return m, pipe.receiver()
}
