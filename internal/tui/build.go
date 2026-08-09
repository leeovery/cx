package tui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/resolver"
	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/theme"
)

// Deps is the compiler-enforced seam set from which Build assembles a Model.
//
// It is the single shared description of every dependency the TUI model needs:
// production (cmd/open.go) populates it with the real *tmux.Client and the
// concrete config stores; the offline capture harness (cmd/capturetool) populates
// it with in-memory fakes. Because both paths construct the same struct, a fake
// that drifts from the real seam set is a compile error, not a silent render
// divergence — which is exactly the property the visual-verification harness
// depends on (the captured TUI is the production TUI).
//
// Optional seams are nil-tolerant: a nil interface field leaves that capability
// unwired, mirroring the behaviour of omitting the corresponding With* option.
// The exceptions are Lister (the one required seam, mirroring New's required
// first argument) and InitialMode (Flat is a valid explicit value that always
// applies).
//
// Everything below is PRODUCTION WIRING except Capture: the harness's first-frame
// seeds live there so the two halves are separable at a glance rather than
// interleaved.
type Deps struct {
	// Required seam — the session source (mirrors New's first argument).
	Lister SessionLister

	// Core action seams (always passed in production, no-ops in the harness).
	Killer  SessionKiller
	Renamer SessionRenamer
	Creator SessionCreator

	// Optional seams — a nil field leaves the capability unwired (matching an
	// omitted With* option).
	ProjectStore    ProjectStore
	ProjectEditor   ProjectEditor
	AliasEditor     AliasEditor
	Enumerator      TmuxEnumerator
	Reader          ScrollbackReader
	PreviewAttacher PreviewAttacher
	DirReader       session.PaneCurrentPathReader
	DirRunner       resolver.CommandRunner
	ModePersister   ModePersister
	// ThemePersister is the theme-commit seam. Production (cmd/open.go) passes
	// its own cmd-owned persister — the single emission site for
	// `theme: commit failed` — and the offline capture harness passes none, so a
	// commit during a capture writes nowhere, exactly as ModePersister behaves.
	ThemePersister ThemePersister
	// ThemeEnumerator is the panel's theme seam — the TmuxEnumerator /
	// ScrollbackReader idiom applied to the theme union. Production (cmd/open.go)
	// passes an adapter closing over the process's theme.Loader and the resolved
	// themes directory; a fixture passes a fake holding a hand-built union, which
	// is what lets internal/capture render a panel under its no-real-config import
	// guard. Nil is the ordinary unwired state and makes `t` a silent no-op.
	ThemeEnumerator ThemeEnumerator
	// Detector + Resolve are the async host-terminal detection seams. Both are
	// injected together by cmd/open.go (Detector = spawn.NewDetector(client), Resolve
	// = the config-aware resolver's Resolve, loaded once from terminals.json) and
	// nil in the offline capture harness. Nil-tolerant: a nil Detector leaves
	// detection unwired.
	Detector TerminalDetector
	Resolve  spawn.AdapterResolver

	// N≥2 picker-burst seams. Injected together by cmd/open.go (defaults
	// client.HasSession / a shared server-option ack channel / os.Executable /
	// os.Getenv) and nil in the offline capture harness. Nil-tolerant: a nil field
	// leaves that capability unwired, matching an omitted With* option. The RESOLVE
	// seam the burst needs is Resolve above (reused), not re-injected here.
	SessionExists func(string) bool
	AckChannel    spawn.AckChannelFull
	SpawnExe      spawn.ExecutableResolver
	SpawnGetenv   func(string) string
	// SpawnLogger is the spawn-component logger the burst completion chokepoint
	// emits its batch summary + per-window detail through. Injected by cmd/open.go
	// (log.For("spawn")) and nil in the offline capture harness. Nil-tolerant: the
	// emit methods route through log.OrDiscard, so a nil logger silently discards.
	SpawnLogger *slog.Logger

	// Scalar configuration.
	CWD         string
	InitialMode prefs.SessionListMode
	// Theme is the LOADED theme setting — one Theme under a constant, both under
	// an adaptive pair. It is the SINGLE driver of the owned canvas
	// and replaces the light/dark appearance that used to be injected here: a
	// theme IS the mode, so there is no mode left to pin.
	//
	// Build injects it via WithThemeNomination and the model resolves the painted
	// palette from its SHAPE — a constant paints from frame one with no detection,
	// a pair resolves through OSC 11 detection (the detect-or-timeout gate)
	// with a dark fallback. The offline capture harness always passes the CONSTANT
	// shape, so its frames are un-gated and byte-stable.
	Theme theme.Nomination
	// ThemeKeys are prefs.json's three theme keys AS READ — control-stripped,
	// post-translation, no defaults substituted. The nomination alone is
	// insufficient for the panel in three ways it cannot recover: a slug that never
	// loaded is not in it, a badge needs the PERSISTED slug rather than the
	// nomination's wherever the two differ under a fallback, and the confirm
	// renders the persisted constant. Its zero value is the shipped adaptive pair —
	// what an unconfigured install renders anyway — so there is no nil-guard.
	//
	// It is a SNAPSHOT: the panel re-reads the themes directory on every open
	// on every open but never re-reads prefs.json, because prefs is what Portal
	// itself writes and re-reading it would import another instance's commit.
	//
	// There is deliberately NO per-slot resolution record beside it. The `●`
	// needs one, but the panel derives it at OPEN from ThemeEnumerator.Resolve —
	// the re-resolution against the enumeration it just read — and badges
	// exist only while the panel is open. An injected record would be a second,
	// staler source of truth for which slug carries the marker, and the one a
	// fixture author would reach for; a fixture declares its slots through the
	// faked seam's Resolve return instead.
	ThemeKeys theme.RawKeys

	InitialFilter  string
	Command        []string
	ServerStarted  bool
	InsideTmux     bool
	CurrentSession string
	// ProgressReceiver is the concurrent cold-boot route's channel-receive
	// tea.Cmd. cmd/open.go sets it only on the cold + TUI path (bootstrap deferred
	// to a goroutine); nil on every synchronous path. When set, Build wires
	// WithProgressReceiver so Init streams live per-step progress from the channel
	// and the channel owns the terminal BootstrapCompleteMsg.
	ProgressReceiver tea.Cmd
	// NoColor is the NO_COLOR carve-out decision. The cmd layer reads
	// os.Getenv("NO_COLOR") (present and non-empty, the no-color.org convention)
	// and injects the boolean here so internal/tui stays env-free. Build sets ONE
	// colourless flag on the model (WithColourless); every canvas-dependent surface
	// inherits that single flag rather than re-deriving NO_COLOR. When true, Portal
	// paints no canvas at all and skips light/dark detection + the first-paint wait
	// — there is no canvas to select.
	NoColor bool

	// Capture holds the first-frame seeds the offline harness declares and
	// PRODUCTION NEVER SETS. Its zero value is the whole production shape, so the
	// nesting is what tells a reader which half of this struct wires a real Portal
	// and which half authors a fixture.
	Capture CaptureSeeds
}

// CaptureSeeds is the offline capture harness's first-frame state, declared
// rather than driven.
//
// A fixture is a ONE-SHOT RENDER: it loads, it paints, it is screenshotted. Every
// state below is otherwise reached by a keypress the harness never makes or by an
// async lifecycle it never runs, so a designed surface with no seed here has no
// frame at all. Each is a no-op at its zero value, which is why a production
// Deps leaves the whole struct alone.
//
// THEY DECLARE STATE AND NEVER TEXT wherever copy is involved: the two theme
// message seeds are BOOLs whose lines are composed by the message slot from its
// own pinned constants, so no fixture can put a paraphrase on a captured frame.
type CaptureSeeds struct {
	// Flash seeds the inline WARNING flash on the first frame (the orange ▌ bar +
	// ⚠ + message on the bg.warning tint). The flash is otherwise transient (set
	// only by the preview-bail path). Only the warning variant is seedable — the
	// success variant is not separately captured.
	Flash string
	// MultiSelect seeds multi-select mode on the first frame with the named
	// sessions pre-marked (keyed on Session.Name); the mode is otherwise entered by
	// the `m` key.
	MultiSelect []string
	// Cursor seeds the session-row cursor anchor: the name of the row the cursor
	// lands on once the list loads, so the multi-select capture can put it on a
	// marked (banded) row. The live picker keeps the default index-0 cursor.
	Cursor string
	// ThemeCursor seeds the theme panel's cursor anchor: the row IDENTITY the
	// cursor lands on once the panel has opened. The open puts the cursor on the
	// theme actually rendering, so the mandated constant-while-previewing frame — a
	// cursor on a row OTHER than the marked one — is otherwise reachable only by
	// arrowing. It is PLACEMENT ONLY and applies no theme, so the rendered palette
	// stays the one the nomination carries.
	ThemeCursor string
	// ThemeConfirm raises the slot-from-constant confirm once the theme panel has
	// opened, exactly as an `l` over a persisted constant would. It is the only
	// surface on which the footer substitution (`y confirm` / `n cancel` in place
	// of the standing four keys, none of which would act) can be seen.
	ThemeConfirm bool
	// ThemeCommitFailed raises the failed-commit report once the theme panel has
	// opened: the message slot's pinned line plus the outstanding-failure state,
	// exactly as a write that did not land leaves them. A capture wires NO theme
	// persister at all, so the state is unreachable by committing however many keys
	// a fixture pressed.
	ThemeCommitFailed bool
	// Detection seeds the host-terminal detection cache with the given identity
	// already resolved (via spawn.ResolveAdapter, so a non-NULL Apple Terminal
	// resolves unsupported and the proactive banner renders). Detection is
	// otherwise an async lifecycle dispatched on reaching PageSessions.
	Detection *spawn.Identity
	// GoneFlagged seeds the pre-flight abort state: the gone-row set (the delegate
	// draws the red ⚠ + `session gone` badge for it) plus the red section-header
	// abort banner text, composed identically to handlePreflightAbort. The abort is
	// otherwise reached only when an N≥2 Enter finds a marked session gone.
	GoneFlagged []string
	// BurstOpening seeds the in-burst Opening band as (done, total): a non-zero
	// pair marks the burst pending and seeds the `Opening done/total…` counters.
	// The burst is otherwise driven by dispatchBurst + its progress goroutine.
	BurstOpening [2]int
}

// Build constructs a Model from the shared Deps seam set. It is the single
// model-construction chokepoint used by BOTH cmd/open.go (real *tmux.Client) and
// cmd/capturetool (in-memory fakes), so the harness captures the exact model
// production renders.
//
// The option assembly below mirrors the legacy inline construction in
// cmd/open.go one-for-one: the same nil-guards, the always-injected initial
// mode, and the post-construction WithCommand / WithInitialFilter / WithInsideTmux
// chained mutations applied in the same order. It is a behaviour-preserving lift,
// not a new construction path.
func Build(deps Deps) Model {
	opts := []Option{
		WithKiller(deps.Killer),
		WithRenamer(deps.Renamer),
		WithProjectStore(deps.ProjectStore),
		WithSessionCreator(deps.Creator),
		WithCWD(deps.CWD),
	}
	if deps.ServerStarted {
		opts = append(opts, WithServerStarted(true))
	}
	if deps.ProgressReceiver != nil {
		opts = append(opts, WithProgressReceiver(deps.ProgressReceiver))
	}
	if deps.ProjectEditor != nil {
		opts = append(opts, WithProjectEditor(deps.ProjectEditor))
	}
	if deps.AliasEditor != nil {
		opts = append(opts, WithAliasEditor(deps.AliasEditor))
	}
	if deps.Enumerator != nil {
		opts = append(opts, WithEnumerator(deps.Enumerator))
	}
	if deps.Reader != nil {
		opts = append(opts, WithScrollbackReader(deps.Reader))
	}
	if deps.PreviewAttacher != nil {
		opts = append(opts, WithPreviewAttachPipeline(deps.PreviewAttacher))
	}
	if deps.DirReader != nil && deps.DirRunner != nil {
		opts = append(opts, WithDirResolver(deps.DirReader, deps.DirRunner))
	}
	// Initial mode is always injected — Flat is a valid explicit value, and New
	// recomputes the list title after options apply so the first frame paints the
	// correct mode heading.
	opts = append(opts, WithInitialMode(deps.InitialMode))
	// The nomination is always injected — its ZERO value is meaningful (neither
	// state: nothing nominated, so the model keeps New's dark built-in seed and
	// paints immediately), so there is no nil-guard to apply. The model resolves
	// the painted palette from its shape: constant → immediate; pair → OSC 11
	// detect-or-timeout.
	opts = append(opts, WithThemeNomination(deps.Theme))
	// NoColor is the single NO_COLOR carve-out. When set it WINS over the
	// shape-driven gate (New consumes it after the options apply): the canvas is
	// suppressed and detection is skipped, though both nominated themes are still
	// held and the dark member still selected. Always injected — false is
	// the no-op coloured path, so omitting it leaves the canvas painted.
	opts = append(opts, WithColourless(deps.NoColor))
	if deps.ModePersister != nil {
		opts = append(opts, WithModePersister(deps.ModePersister))
	}
	if deps.ThemePersister != nil {
		opts = append(opts, WithThemePersister(deps.ThemePersister))
	}
	// The raw persisted keys are always injected — the zero value is meaningful
	// (no keys IS the shipped adaptive pair), so there is nothing to guard.
	opts = append(opts, WithThemeKeys(deps.ThemeKeys))
	if deps.ThemeEnumerator != nil {
		opts = append(opts, WithThemeEnumerator(deps.ThemeEnumerator))
	}
	// Async host-terminal detection seams. Always injected via nil-tolerant
	// options — a nil Detector/Resolve leaves detection unwired, mirroring the
	// capture harness (which passes neither).
	opts = append(opts, WithTerminalDetector(deps.Detector))
	opts = append(opts, WithResolve(deps.Resolve))

	// N≥2 picker-burst seams. Always injected via nil-tolerant options — a nil
	// seam leaves the burst unwired, mirroring the capture harness (which passes
	// none). Production supplies all four via cmd/open.go's buildTUIModel.
	opts = append(opts, WithSessionExists(deps.SessionExists))
	opts = append(opts, WithAckChannel(deps.AckChannel))
	opts = append(opts, WithSpawnExe(deps.SpawnExe))
	opts = append(opts, WithSpawnGetenv(deps.SpawnGetenv))
	// Spawn batch-summary logger. Always injected via the nil-tolerant option —
	// a nil logger (the capture harness) leaves emission discarded.
	opts = append(opts, WithSpawnLogger(deps.SpawnLogger))

	// Seed the inline warning flash for the capture harness (no-op when empty,
	// the production default). Applied as an Option so it is set before the
	// armAppearanceDetection / first WindowSizeMsg resync reserves the band row.
	opts = append(opts, WithInitialFlash(deps.Capture.Flash))

	// Seed multi-select mode + the cursor anchor for the capture harness (both
	// no-ops when empty, the production default). Applied as Options in the same
	// window as WithInitialFlash — before armAppearanceDetection — so the marked-set
	// delegate is armed on the first frame; the cursor anchor is applied later, once
	// items ingest (evaluateDefaultPage), since it must survive the initial SetItems.
	opts = append(opts, WithInitialMultiSelect(deps.Capture.MultiSelect))
	opts = append(opts, WithInitialCursor(deps.Capture.Cursor))
	// The PANEL seeds are applied here alongside their session-list sibling,
	// but they land much later: the panel does not exist until `t` is pressed, so
	// armThemePanel consumes all of them. The cursor anchor places the cursor; the
	// two message seeds raise the message slot in one or other of its states. Unset on
	// every production model.
	opts = append(opts, WithInitialThemeCursor(deps.Capture.ThemeCursor))
	opts = append(opts, WithInitialThemeConfirm(deps.Capture.ThemeConfirm))
	opts = append(opts, WithInitialThemeCommitFailed(deps.Capture.ThemeCommitFailed))

	// Seed the picker-burst capture-only frames (all no-ops when unset, the
	// production default). WithInitialGoneFlagged is applied AFTER WithInitial
	// MultiSelect so its delegate refresh observes the seeded marked set (survivors
	// keep their ●, the gone row shows the red flag). WithInitialDetection seeds the
	// resolved detection cache for the proactive unsupported banner; WithInitialBurst
	// Opening seeds the in-burst Opening band. Production never sets any of these.
	opts = append(opts, WithInitialDetection(deps.Capture.Detection))
	opts = append(opts, WithInitialGoneFlagged(deps.Capture.GoneFlagged))
	opts = append(opts, WithInitialBurstOpening(deps.Capture.BurstOpening[0], deps.Capture.BurstOpening[1]))

	m := New(deps.Lister, opts...)
	if len(deps.Command) > 0 {
		m = m.WithCommand(deps.Command)
	}
	if deps.InitialFilter != "" {
		m = m.WithInitialFilter(deps.InitialFilter)
	}
	if deps.InsideTmux && deps.CurrentSession != "" {
		m = m.WithInsideTmux(deps.CurrentSession)
	}
	// Open the detect-or-timeout first-paint window for the LIVE picker. New
	// constructs an adaptive gate already resolved to the dark fallback (so
	// directly constructed test models paint immediately); production opens the
	// window here so the live program holds the neutral blank frame until OSC 11
	// selects the member — no paint-then-flip. armAppearanceDetection is a no-op on
	// a CONSTANT (or absent) nomination and on a WithCanvasMode override, so those
	// keep painting from frame one. The capture harness always passes the constant
	// shape, so its frames stay deterministic and un-gated.
	m.armAppearanceDetection()
	return m
}
