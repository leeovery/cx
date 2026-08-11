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

// Deps is the seam set from which Build assembles a Model. Production and the
// offline capture harness construct the same struct, so a fake that drifts
// from the real seam set is a compile error rather than a silent render
// divergence. Optional seams are nil-tolerant — a nil field leaves that
// capability unwired; Lister is required.
type Deps struct {
	Lister SessionLister

	Killer  SessionKiller
	Renamer SessionRenamer
	Creator SessionCreator

	ProjectStore    ProjectStore
	ProjectEditor   ProjectEditor
	AliasEditor     AliasEditor
	Enumerator      TmuxEnumerator
	Reader          ScrollbackReader
	PreviewAttacher PreviewAttacher
	DirReader       session.PaneCurrentPathReader
	DirRunner       resolver.CommandRunner
	ModePersister   ModePersister
	ThemePersister  ThemePersister
	// ThemeSource is the theme panel's enumeration/resolution seam; nil
	// makes `t` a silent no-op.
	ThemeSource ThemeSource
	Detector    TerminalDetector
	Resolve     spawn.AdapterResolver

	// Picker-burst seams. The burst reuses Resolve above rather than taking
	// a second resolver.
	SessionExists func(string) bool
	AckChannel    spawn.AckChannelFull
	SpawnExe      spawn.ExecutableResolver
	SpawnGetenv   func(string) string
	SpawnLogger   *slog.Logger

	CWD         string
	InitialMode prefs.SessionListMode
	// Theme is the loaded theme nomination. Its shape decides the first
	// paint: a constant paints from frame one, an adaptive pair resolves
	// through the OSC 11 detect-or-timeout gate with a dark fallback.
	Theme theme.Nomination
	// ThemeKeys are prefs.json's theme keys as read — post-translation, no
	// defaults substituted; the zero value is the shipped adaptive pair. A
	// snapshot: the panel never re-reads prefs.json, which would import
	// another instance's commit.
	ThemeKeys theme.RawKeys

	InitialFilter  string
	Command        []string
	ServerStarted  bool
	InsideTmux     bool
	CurrentSession string
	// ProgressReceiver is the concurrent cold-boot channel-receive tea.Cmd —
	// set only on the cold + TUI path, nil on every synchronous path.
	ProgressReceiver tea.Cmd
	// NoColor is the NO_COLOR carve-out, read by the cmd layer so
	// internal/tui stays env-free.
	NoColor bool

	// Capture holds the capture harness's first-frame seeds; production
	// never sets it.
	Capture CaptureSeeds
}

// CaptureSeeds is the offline capture harness's first-frame state: a fixture
// is a one-shot render, so states otherwise reached by a keypress or an async
// lifecycle are declared here. Every field is a no-op at its zero value.
// Seeds declare state, never text — message copy comes from the model's own
// constants, so a fixture cannot put a paraphrase on a captured frame.
type CaptureSeeds struct {
	// Flash is the warning variant only; the success variant is not seedable.
	Flash string
	// MultiSelect names the pre-marked sessions.
	MultiSelect []string
	// Cursor names the session row the cursor lands on.
	Cursor string
	// ThemeCursor is placement only — it applies no theme, so the rendered
	// palette stays the one the nomination carries.
	ThemeCursor       string
	ThemeConfirm      bool
	ThemeCommitFailed bool
	Detection         *spawn.Identity
	GoneFlagged       []string
	// BurstOpening is (done, total).
	BurstOpening [2]int
}

// Build constructs a Model from the shared Deps seam set — the single
// model-construction chokepoint for both production and the capture harness.
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
	opts = append(opts, WithInitialMode(deps.InitialMode))
	opts = append(opts, WithThemeNomination(deps.Theme))
	opts = append(opts, WithColourless(deps.NoColor))
	if deps.ModePersister != nil {
		opts = append(opts, WithModePersister(deps.ModePersister))
	}
	if deps.ThemePersister != nil {
		opts = append(opts, WithThemePersister(deps.ThemePersister))
	}
	opts = append(opts, WithThemeKeys(deps.ThemeKeys))
	if deps.ThemeSource != nil {
		opts = append(opts, WithThemeSource(deps.ThemeSource))
	}
	opts = append(opts, WithTerminalDetector(deps.Detector))
	opts = append(opts, WithResolve(deps.Resolve))

	opts = append(opts, WithSessionExists(deps.SessionExists))
	opts = append(opts, WithAckChannel(deps.AckChannel))
	opts = append(opts, WithSpawnExe(deps.SpawnExe))
	opts = append(opts, WithSpawnGetenv(deps.SpawnGetenv))
	opts = append(opts, WithSpawnLogger(deps.SpawnLogger))

	// Capture seeds must apply as Options — before armAppearanceDetection and
	// the first WindowSizeMsg resync reserve the band row.
	opts = append(opts, WithInitialFlash(deps.Capture.Flash))

	opts = append(opts, WithInitialMultiSelect(deps.Capture.MultiSelect))
	opts = append(opts, WithInitialCursor(deps.Capture.Cursor))
	opts = append(opts, WithInitialThemeCursor(deps.Capture.ThemeCursor))
	opts = append(opts, WithInitialThemeConfirm(deps.Capture.ThemeConfirm))
	opts = append(opts, WithInitialThemeCommitFailed(deps.Capture.ThemeCommitFailed))

	// WithInitialGoneFlagged applies after WithInitialMultiSelect so its
	// delegate refresh observes the seeded marked set.
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
	// Opens the detect-or-timeout first-paint window for the live picker; a
	// no-op on a constant or absent nomination, so test models and capture
	// frames paint immediately.
	m.armAppearanceDetection()
	return m
}
