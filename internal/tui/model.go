// Package tui provides the Bubble Tea TUI for Portal.
package tui

import (
	"context"
	"fmt"
	"image/color"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/resolver"
	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

type page int

const (
	PageLoading page = iota
	PageSessions
	PageProjects
	pagePreview
)

// paginationDotGlyph is the bubbles/list built-in dot glyph; the restyle
// recolours it without changing the glyph or the engine's page count.
const paginationDotGlyph = "•"

type SessionLister interface {
	ListSessions() ([]tmux.Session, error)
}

// PreviewAttacher is the seam through which the preview page's Enter handler
// dispatches the pre-select pipeline. Run returns a tea.Cmd that executes the
// pre-select calls (has-session probe, select-window, select-pane) and emits
// a previewAttachSelectedMsg or previewAttachBailMsg; the connector handoff
// happens post-TUI.
type PreviewAttacher interface {
	Run(session string, window, pane int) tea.Cmd
}

type ProjectStore interface {
	List() ([]project.Project, error)
	CleanStale() ([]project.Project, error)
	Remove(path, via string) error
}

type SessionKiller interface {
	KillSession(name string) error
}

type SessionCreator interface {
	CreateFromDir(dir string, command []string) (string, error)
}

type SessionRenamer interface {
	RenameSession(oldName, newName string) error
}

// ModePersister persists the session-list grouping mode.
type ModePersister interface {
	Save(prefs.SessionListMode) error
}

// ThemePersister commits the theme panel's choice. The production
// implementation is cmd-owned, not *prefs.Store — cmd resolves the prefs path
// and logs the failure; the model holds the seam and logs nothing. The method
// names deliberately differ from the store's savers so *prefs.Store cannot
// accidentally satisfy the seam. Errors are returned as well as logged: the
// panel renders its outstanding-failure state from the value. theme.Member
// names the adaptive-pair half so the seam cannot mint a third slot.
type ThemePersister interface {
	CommitTheme(slug string) error
	CommitThemeSlot(slug string, member theme.Member) error
}

// ProjectEditor defines the interface for renaming projects and mutating
// their tag set. AddTag/RemoveTag take the raw tag value and delegate
// canonicalisation and dedup to the store; the modal never re-normalises.
type ProjectEditor interface {
	Rename(path, newName, via string) error
	AddTag(path, rawTag string) error
	RemoveTag(path, rawTag string) error
}

// AliasEditor defines the interface for managing aliases in edit mode.
// Mutations go through the audited combined methods (SetAndSave /
// DeleteAndSave), not raw Set/Delete/Save, so every TUI alias edit is
// audited; Load stays for the collision pre-check.
type AliasEditor interface {
	Load() (map[string]string, error)
	SetAndSave(name, path, via string) error
	DeleteAndSave(name, via string) (bool, error)
}

type editField int

const (
	editFieldName editField = iota
	editFieldAliases
	editFieldTags
)

type editMode int

const (
	editModeNavigate editMode = iota
	editModeEdit
)

type SessionsMsg struct {
	Sessions []tmux.Session
	Err      error
}

// LoadingMinDuration is the minimum amount of time the loading page is
// displayed before being dismissed. Bootstrap may take longer (the page
// stays until bootstrap completes); if bootstrap is faster, the page
// is padded to this duration so it does not flash as a UI glitch.
const LoadingMinDuration = 1200 * time.Millisecond

// LoadingMinElapsedMsg signals that LoadingMinDuration has elapsed since
// the loading page was first shown.
type LoadingMinElapsedMsg struct{}

// BootstrapCompleteMsg signals that the bootstrap orchestrator has finished.
// The TUI dismisses the loading page once both LoadingMinElapsedMsg and this
// have been received. Warnings carries soft bootstrap warnings, buffered
// through the loading window and surfaced after dismissal; may be nil/empty.
type BootstrapCompleteMsg struct {
	Warnings []BootstrapWarning
}

// BootstrapProgressMsg is one live per-step progress event streamed over the
// progress channel. Index is the 1-based canonical bootstrap step number; the
// message deliberately carries no friendly label, which would be a second,
// drift-prone encoding of loading_progress.go's mapping. RestoreN/RestoreM
// are the restore per-session counter, both zero meaning no counter.
type BootstrapProgressMsg struct {
	Index    int
	RestoreN int
	RestoreM int
}

// BootstrapFatalMsg is the terminal fatal event streamed over the progress
// channel: it drives the in-TUI error state instead of dismissing the loading
// page. FailedStep is the 1-based index of the aborting step, Message the
// single user-facing line, and Err the underlying cause — carried as an error
// so internal/tui stays decoupled from cmd/bootstrap.
type BootstrapFatalMsg struct {
	FailedStep int
	Message    string
	Err        error
}

type SessionCreatedMsg struct {
	SessionName string
}

type sessionCreateErrMsg struct {
	Err error
}

type ProjectsLoadedMsg struct {
	Projects []project.Project
	Err      error
}

// Model is the Bubble Tea model for the session list TUI.
type Model struct {
	sessionList list.Model
	sessions    []tmux.Session
	projects    []project.Project
	// projectIndex is a derived cache of projects — a pre-canonicalised
	// dir→project lookup so a grouped render is map lookups, not per-render
	// canonicalisation. It must be rebuilt whenever projects changes: always
	// mutate both via setProjects.
	projectIndex    project.Index
	sessionListMode prefs.SessionListMode
	modePersister   ModePersister
	// colourless is the single NO_COLOR carve-out flag; every
	// canvas-dependent surface reads it rather than re-deriving NO_COLOR.
	// Portal then paints no canvas at all and skips detection.
	colourless     bool
	selected       string
	sessionLister  SessionLister
	sessionKiller  SessionKiller
	sessionRenamer SessionRenamer
	projectStore   ProjectStore
	projectEditor  ProjectEditor
	aliasEditor    AliasEditor
	sessionCreator SessionCreator
	cwd            string
	activePage     page
	projectList    list.Model
	initialFilter  string
	// initialCursor is the capture-only cursor anchor: the session row the
	// cursor lands on once the list loads. Applied and cleared in
	// evaluateDefaultPage after items ingest; empty is a no-op and production
	// never sets it.
	initialCursor      string
	insideTmux         bool
	currentSession     string
	modal              modalState
	pendingKillName    string
	pendingKillWindows int
	renameInput        textinput.Model
	renameTarget       string
	pendingDeletePath  string
	pendingDeleteName  string
	command            []string
	commandPending     bool

	serverStarted     bool
	minElapsed        bool
	bootstrapComplete bool

	// Fatal cold-boot error state: once fatalActive is set the model stays on
	// the loading page and stops gating on BootstrapCompleteMsg, which will
	// never arrive.
	fatalActive  bool
	fatalStep    int
	fatalMessage string
	fatalErr     error

	// progressReceiver is the cold-boot route's channel-receive tea.Cmd: one
	// blocking receive re-issued per event, which preserves exact event order
	// even under command batching. When set, the channel owns the terminal
	// BootstrapCompleteMsg; when nil, Init synthesizes it.
	progressReceiver tea.Cmd

	// loadingProgress is the loading-screen accumulator; its zero value
	// renders the all-pending screen.
	loadingProgress LoadingProgress

	// Bootstrap warnings: pending is staged before Init and folded into the
	// BootstrapCompleteMsg payload; bufferedWarnings holds the slice until
	// the loading page dismisses, cleared on emit.
	pendingBootstrapWarnings []BootstrapWarning
	bufferedWarnings         []BootstrapWarning

	termWidth, termHeight int

	// originalBg is what the terminal looked like before Portal painted, as
	// reported by the OSC 11 query — distinct from Portal's chosen canvas,
	// and captured only so exit can set it back for terminals that ignore the
	// OSC 111 reset. Async and non-gating; empty if no response arrived.
	originalBg string

	// Preview page seams and live model. previewAttacher is propagated onto
	// the preview at construction so its Enter binding dispatches without
	// re-resolving the connector.
	enumerator      TmuxEnumerator
	reader          ScrollbackReader
	previewAttacher PreviewAttacher
	preview         previewModel

	// Render-layer directory-resolution seam for the lazy grouping fallback.
	// The derived directory is never stamped back to tmux — a pane's cwd can
	// drift from the origin directory, and freezing that drift would
	// permanently mis-group the session. Either seam nil skips the pass.
	dirReader session.PaneCurrentPathReader
	dirRunner resolver.CommandRunner

	// Sessions-page inline-flash state. flashText empty means no flash.
	// flashGen is a monotonic counter bumped on every setFlash; tick handlers
	// capture the generation at schedule time so a stale tick from a replaced
	// flash cannot early-clear the currently-displayed message.
	flashText string
	flashGen  uint64
	// flashKind selects the warning or success styling of the active flash.
	// flashWarning is the zero value; every setFlash resets to it, and
	// setSuccessFlash flips to the success variant.
	flashKind flashKind
	// flashOrigin is the precedence tier of the active flash: a theme-origin
	// flash claims the notice slot even while the filter line is live. Reset
	// by setFlash / setSuccessFlash and stamped only by setThemeFlash, so the
	// tier can never be inherited by an unrelated message.
	flashOrigin flashOrigin
	// byTagSignpost is the "No tags yet" signpost flag: By Tag active with
	// zero tags anywhere, in which case the list degrades to flat under a
	// dimmed signpost row. Unlike the transient flashText it is a derived
	// flag recomputed on every rebuild.
	byTagSignpost bool

	// Multi-select mode state. selectedSessions is keyed on Session.Name —
	// the same identity the attach path uses — so a multi-tag session
	// spanning several By-Tag rows is marked exactly once. The map is lazily
	// initialised; exitMultiSelect nils it back out.
	multiSelectMode  bool
	selectedSessions map[string]struct{}

	// Pre-flight abort state: when an N≥2 Enter pre-flight finds a marked
	// session gone the burst aborts atomically. abortBannerText holds the red
	// section-header banner (a section-header claimant, not a ▌-barred band);
	// goneFlagged is the transient gone-row set the delegate badges. Both
	// clear on dismiss or refresh; survivors stay marked.
	abortBannerText string
	goneFlagged     map[string]struct{}

	// Async host-terminal detection, dispatched once on reaching PageSessions
	// and cached for any later burst. Caching the Resolution — not just
	// IsNull() — is load-bearing: a recognised-but-undriven terminal is
	// non-NULL yet resolves unsupported.
	detector         TerminalDetector
	resolve          spawn.AdapterResolver
	detectIdentity   spawn.Identity
	detectResolution spawn.Resolution
	// detectAdapter is the adapter half of the same single resolve that
	// produced detectResolution, cached in lockstep so the gate decision and
	// the dispatch adapter cannot disagree. dispatchBurst must read it rather
	// than re-resolve: a second resolve is not deterministic for the
	// config-script recipe (the script is re-statted every call), so a script
	// removed between detection and Enter would nil-panic the un-recovered
	// burst goroutine.
	detectAdapter    spawn.Adapter
	detectResolved   bool
	detectDispatched bool

	// N≥2 picker-burst seams: sessionExists is the pre-flight has-session
	// probe, ackChannel the token-ack channel, spawnExe resolves the picker's
	// own binary, spawnGetenv reads PATH. The resolve seam is reused from the
	// detection block above, never re-injected.
	sessionExists func(string) bool
	ackChannel    spawn.AckChannelFull
	spawnExe      spawn.ExecutableResolver
	spawnGetenv   func(string) string
	// spawnLogger is the spawn-component logger the burst completion
	// chokepoint emits through; wrapped in log.OrDiscard at emit time so nil
	// never panics.
	spawnLogger *slog.Logger

	// Burst lifecycle state. burstTrigger and burstExternal are the net-N
	// split (trigger = self-attach target) and burstTotal includes the
	// trigger. The resolved terminal outcome travels on spawnCompleteMsg,
	// never on the model.
	burstPending  bool
	burstPipe     *burstProgressPipe
	burstCancel   context.CancelFunc
	burstTrigger  string
	burstExternal []string
	burstTotal    int
	burstDone     int
	// burstCancelled records a user Ctrl-C/Esc cancel; the terminal
	// spawnCompleteMsg arm reads it to suppress both the self-exec quit and
	// the failed-window flash (cancellation is user-initiated, so silent).
	// resetBurstState clears it on every terminal path.
	burstCancelled bool

	// pendingBurstEnter flags a deferred N≥2 Enter pressed while detection is
	// still in flight. No Enter-time snapshot is stashed — the
	// terminalDetectedMsg arm re-derives the marked set live, so a mark
	// toggle during the defer window is honoured and the burst never fires
	// before the host terminal is known.
	pendingBurstEnter bool

	// Data loading tracking
	sessionsLoaded       bool
	projectsLoaded       bool
	defaultPageEvaluated bool

	// Edit project modal state — the two-mode (navigate/edit),
	// immediate-persist state machine: no batch buffer and no dirty state,
	// every element persists on exit-edit (or immediately on chip delete).
	editProject project.Project
	editMode    editMode
	editFocus   editField
	editName    string
	editAliases []string
	editTags    []string
	// editAliasCursor / editTagCursor are the focused-element index within a
	// chip field: chips occupy [0, len-1] and the trailing + add slot is at
	// index len. Only the focused field's index is meaningful.
	editAliasCursor int
	editTagCursor   int
	// editChanged records that at least one field persisted this session, so
	// closing on Esc refreshes the cached project records + grouping index.
	// Not a dirty flag — everything is already on disk.
	editChanged bool

	// Live-edit buffer (only meaningful while editMode == editModeEdit).
	editBuffer    string
	editCursor    int
	editIsNewChip bool // the live chip is brand-new (vanishes on Esc-discard)

	// themePanel is the slide-over theme picker — a non-blanking overlay,
	// unlike the modals above: the page beneath stays visible and re-themes
	// live, which is the live-preview premise. While closed, the composed
	// frame is byte-identical to a model with no panel at all.
	themePanel themePanel

	// themeState is the model-level theme machinery the panel is a surface
	// onto: the loaded nomination, seams, light/dark resolution, the palette
	// every renderer paints from, and the exit-time canvas anchor. It
	// outlives the panel, which is why the two are separate structs.
	themeState themeState
}

// Selected returns the name of the session chosen by the user, or empty if
// the user quit without selecting.
func (m Model) Selected() string {
	return m.selected
}

func (m Model) InitialFilter() string {
	return m.initialFilter
}

func (m Model) SessionListItems() []list.Item {
	return m.sessionList.Items()
}

func (m Model) SessionListTitle() string {
	return m.sessionList.Title
}

func (m Model) SessionListSize() (int, int) {
	return m.sessionList.Width(), m.sessionList.Height()
}

func (m Model) SessionListFilterState() list.FilterState {
	return m.sessionList.FilterState()
}

func (m Model) SessionListVisibleItems() []list.Item {
	return m.sessionList.VisibleItems()
}

func (m Model) SessionListFilterValue() string {
	return m.sessionList.FilterValue()
}

func (m Model) MultiSelectActive() bool {
	return m.multiSelectMode
}

func (m Model) IsSessionSelected(name string) bool {
	_, ok := m.selectedSessions[name]
	return ok
}

func (m Model) SelectedSessionCount() int {
	return len(m.selectedSessions)
}

func (m *Model) SetSessionListFilter(text string) {
	m.sessionList.SetFilterText(text)
	m.sessionList.SetFilterState(list.FilterApplied)
}

func (m Model) ProjectListFilterValue() string {
	return m.projectList.FilterValue()
}

func (m Model) ProjectListItems() []list.Item {
	return m.projectList.Items()
}

func (m Model) ProjectListSize() (int, int) {
	return m.projectList.Width(), m.projectList.Height()
}

func (m Model) ProjectListFilterState() list.FilterState {
	return m.projectList.FilterState()
}

func (m Model) ProjectListVisibleItems() []list.Item {
	return m.projectList.VisibleItems()
}

func (m *Model) SetProjectListFilter(text string) {
	m.projectList.SetFilterText(text)
	m.projectList.SetFilterState(list.FilterApplied)
}

func (m Model) ActivePage() page {
	return m.activePage
}

func (m Model) ServerStarted() bool {
	return m.serverStarted
}

func (m Model) MinElapsed() bool {
	return m.minElapsed
}

func (m Model) BootstrapComplete() bool {
	return m.bootstrapComplete
}

// FatalError returns the fatal cold-boot error carried after a
// BootstrapFatalMsg, or nil when no fatal occurred. Carried as an error so
// internal/tui stays decoupled from cmd/bootstrap; callers recover the
// concrete type via errors.As.
func (m Model) FatalError() error {
	return m.fatalErr
}

// BufferedWarnings returns the warnings held between BootstrapCompleteMsg
// and loading-page dismissal; the slice is cleared once they are emitted.
func (m Model) BufferedWarnings() []BootstrapWarning {
	return m.bufferedWarnings
}

func (m Model) PendingBootstrapWarnings() []BootstrapWarning {
	return m.pendingBootstrapWarnings
}

// SetPendingBootstrapWarnings stores warnings to be folded into the
// BootstrapCompleteMsg emitted from Init's first event-loop tick, so they
// ride the same gate that dismisses the loading page.
func (m *Model) SetPendingBootstrapWarnings(warnings []BootstrapWarning) {
	m.pendingBootstrapWarnings = warnings
}

func (m Model) CommandPending() bool {
	return m.commandPending
}

func (m Model) Command() []string {
	return m.command
}

func (m Model) InsideTmux() bool {
	return m.insideTmux
}

func (m Model) CurrentSession() string {
	return m.currentSession
}

func (m Model) CWD() string {
	return m.cwd
}

// OriginalBackground returns the terminal's original background colour as
// captured via the OSC 11 query (a hex string like "#1e1e2e"), or empty if
// no response ever arrived. Launch sites set it back on exit so the owned
// canvas does not stick on terminals that ignore the OSC 111 reset.
func (m Model) OriginalBackground() string {
	return m.originalBg
}

// WithInitialFilter returns a copy of the Model with the initial filter set.
// The filter is applied to the session list after items load.
func (m Model) WithInitialFilter(filter string) Model {
	m.initialFilter = filter
	return m
}

// WithCommand returns a copy of the Model with the given command set. When
// command is non-empty, the TUI starts in command-pending mode: the projects
// page shows directly with the command-pending banner and footer.
func (m Model) WithCommand(command []string) Model {
	m.command = command
	if len(command) > 0 {
		m.commandPending = true
		m.activePage = PageProjects
	}
	return m
}

// WithInsideTmux returns a copy of the Model configured as running inside tmux
// with the given current session name. The current session is excluded from the
// session list and the list title shows the current session name.
func (m Model) WithInsideTmux(currentSession string) Model {
	m.insideTmux = true
	m.currentSession = currentSession
	// Population routes through rebuildSessionList so inside-tmux exclusion
	// composes with mode/grouping/dir-resolution. The returned cmd is
	// discarded: at construction m.sessions is empty, so SetItems yields no
	// real cmd.
	(&m).rebuildSessionList()
	return m
}

type Option func(*Model)

func WithKiller(k SessionKiller) Option {
	return func(m *Model) {
		m.sessionKiller = k
	}
}

func WithRenamer(r SessionRenamer) Option {
	return func(m *Model) {
		m.sessionRenamer = r
	}
}

// WithInitialMode sets the session-list grouping mode the model opens in.
// Flat is a valid explicit value, so callers always pass the option. The mode
// is re-applied on every session ingestion, so it does not depend on sessions
// being loaded at construction time.
func WithInitialMode(mode prefs.SessionListMode) Option {
	return func(m *Model) {
		m.sessionListMode = mode
	}
}

// WithModePersister sets the session-list mode persister dependency; the
// handler tolerates a nil persister.
func WithModePersister(p ModePersister) Option {
	return func(m *Model) {
		m.modePersister = p
	}
}

// WithThemePersister sets the theme-commit persister dependency; a nil seam
// makes a commit write nowhere.
func WithThemePersister(p ThemePersister) Option {
	return func(m *Model) {
		m.themeState.persister = p
	}
}

// WithThemeKeys injects prefs.json's theme keys as read — the
// construction-time snapshot the panel lists and marks from. The zero value
// is meaningful (no keys is the shipped adaptive pair), so the option is
// always applied.
func WithThemeKeys(keys theme.RawKeys) Option {
	return func(m *Model) {
		m.themeState.keys = keys
	}
}

// WithThemeSource injects the panel's theme enumeration/resolution seam; an
// unwired seam makes `t` a silent no-op.
func WithThemeSource(e ThemeSource) Option {
	return func(m *Model) {
		m.themeState.source = e
	}
}

// WithThemeNomination injects the loaded theme setting the model renders
// from: one Theme under a constant, both under an adaptive pair. The shape
// decides the first paint: a constant paints from frame one with no
// detection; a pair holds the first paint until the gate selects a member.
// Omitting the option leaves the zero nomination — the model keeps New's dark
// built-in seed and paints immediately.
func WithThemeNomination(n theme.Nomination) Option {
	return func(m *Model) {
		m.themeState.nomination = n
	}
}

// WithCanvasMode is the test/capture-only direct override of the gate's
// light/dark answer: it resolves the gate onto the given member so the model
// paints from frame one with no OSC 11 detection. Production never uses it.
// It pins the answer, not the palette: paired with a constant nomination it
// changes nothing (a constant ignores the answer by design).
func WithCanvasMode(appearance theme.Member) Option {
	return func(m *Model) {
		// pinned=true so New's gate-init guard preserves this override
		// instead of rebuilding an auto gate over it.
		m.themeState.gate = appearanceGate{appearance: appearance, pinned: true}
		m.themeState.adoptGateAnswer()
	}
}

// WithColourless sets the NO_COLOR carve-out flag (detected by the cmd layer
// so internal/tui stays env-free). When set, the model paints no canvas at
// all and skips light/dark detection — there is no canvas to select.
func WithColourless(colourless bool) Option {
	return func(m *Model) {
		m.colourless = colourless
	}
}

func WithProjectStore(s ProjectStore) Option {
	return func(m *Model) {
		m.projectStore = s
	}
}

func WithSessionCreator(c SessionCreator) Option {
	return func(m *Model) {
		m.sessionCreator = c
	}
}

// WithCWD sets the current working directory on the model.
// Used by the n key to create a new session in the current directory.
func WithCWD(path string) Option {
	return func(m *Model) {
		m.cwd = path
	}
}

func WithProjectEditor(e ProjectEditor) Option {
	return func(m *Model) {
		m.projectEditor = e
	}
}

func WithAliasEditor(a AliasEditor) Option {
	return func(m *Model) {
		m.aliasEditor = a
	}
}

// WithServerStarted configures the model to start on the loading page
// when the tmux server was just started by bootstrap.
func WithServerStarted(started bool) Option {
	return func(m *Model) {
		m.serverStarted = started
		if started {
			m.activePage = PageLoading
		}
	}
}

// WithProgressReceiver wires the concurrent cold-boot route's
// channel-receive tea.Cmd. When set, Init streams live per-step bootstrap
// progress from the channel and the channel owns the terminal
// BootstrapCompleteMsg; a nil receiver keeps the synchronous path unchanged.
func WithProgressReceiver(receiver tea.Cmd) Option {
	return func(m *Model) {
		m.progressReceiver = receiver
	}
}

// WithEnumerator wires the TmuxEnumerator seam used by the scrollback
// preview page.
func WithEnumerator(e TmuxEnumerator) Option {
	return func(m *Model) {
		m.enumerator = e
	}
}

// WithScrollbackReader wires the ScrollbackReader seam used by the
// scrollback preview page.
func WithScrollbackReader(r ScrollbackReader) Option {
	return func(m *Model) {
		m.reader = r
	}
}

// WithPreviewAttachPipeline wires the PreviewAttacher seam used by the
// preview page's Enter binding.
func WithPreviewAttachPipeline(p PreviewAttacher) Option {
	return func(m *Model) {
		m.previewAttacher = p
	}
}

// WithDirResolver wires the render-layer directory-resolution seam used by
// rebuildSessionList's lazy fallback: sessions with an absent @portal-dir are
// resolved live from their active pane → git-root before grouping and cached
// in-memory, never stamped back to tmux. Unwired seams skip the pass.
func WithDirResolver(reader session.PaneCurrentPathReader, runner resolver.CommandRunner) Option {
	return func(m *Model) {
		m.dirReader = reader
		m.dirRunner = runner
	}
}

// WithInitialFlash seeds the inline warning flash at construction — the
// capture-harness entry point for the otherwise-transient flash. It sets the
// state fields directly (no flashGen bump, no layout resync — dimensions are
// still zero at construction; the first WindowSizeMsg reserves the band's
// row). Empty is a no-op; only the warning variant is seedable.
func WithInitialFlash(text string) Option {
	return func(m *Model) {
		if text == "" {
			return
		}
		m.flashText = text
		m.flashKind = flashWarning
	}
}

// WithInitialMultiSelect seeds multi-select mode at construction with the
// named sessions pre-marked — the capture-harness entry point for the
// otherwise user-driven mode. The delegate refresh arms the ● column from
// the first frame. Nil/empty is a no-op; the names need not resolve to a
// loaded session yet, since the delegate matches on name as rows ingest.
func WithInitialMultiSelect(names []string) Option {
	return func(m *Model) {
		if len(names) == 0 {
			return
		}
		m.multiSelectMode = true
		m.selectedSessions = make(map[string]struct{}, len(names))
		for _, n := range names {
			m.selectedSessions[n] = struct{}{}
		}
		m.refreshSessionDelegate()
	}
}

// WithInitialCursor seeds the capture-only cursor anchor. It only stores the
// name — positioning happens after items ingest so it survives the SetItems
// that would otherwise reset the cursor to index 0. Empty is a no-op;
// production never sets it.
func WithInitialCursor(name string) Option {
	return func(m *Model) {
		m.initialCursor = name
	}
}

// WithInitialThemeCursor seeds the capture-only panel cursor anchor: the row
// the theme panel's cursor lands on once the panel opens. Placement only — it
// applies no theme, so the rendered palette stays the one the nomination
// carries. Empty is a no-op; production never sets it.
func WithInitialThemeCursor(slug string) Option {
	return func(m *Model) {
		m.themeState.initialCursor = slug
	}
}

// WithInitialThemeConfirm seeds the slot-from-constant confirm for the
// offline capture harness, raised against the persisted constant once the
// panel opens. It seeds state, never text — the copy is composed by the
// message slot from its own pinned format. False is a no-op; production
// never sets it.
func WithInitialThemeConfirm(on bool) Option {
	return func(m *Model) {
		m.themeState.initialConfirm = on
	}
}

// WithInitialThemeCommitFailed seeds the failed-commit report for the
// offline capture harness: once the panel opens, the message slot carries
// the pinned line and the outstanding-failure state, exactly as a write that
// did not land leaves them. Seeds state, never text. False is a no-op;
// production never sets it.
func WithInitialThemeCommitFailed(on bool) Option {
	return func(m *Model) {
		m.themeState.initialCommitFailed = on
	}
}

// canvasHelpStyles backgrounds the bubbles/list HelpStyle wrapper with the
// active theme's canvas. The footer content paints its own canvas; this only
// ensures the wrapper around the footer area carries it too.
func canvasHelpStyles(l *list.Model, th theme.Theme) {
	canvas := th.Canvas.Color()
	l.Styles.HelpStyle = l.Styles.HelpStyle.Background(canvas)
}

// colourlessHelpStyles strips the bubbles/list HelpStyle wrapper background
// for the NO_COLOR carve-out, so the footer area renders on the terminal's
// native bg.
func colourlessHelpStyles(l *list.Model) {
	l.Styles.HelpStyle = l.Styles.HelpStyle.UnsetBackground()
}

// canvasNoItemsStyle re-points bubbles/list's zero-items body style onto the
// theme's tokens. Re-pointed rather than unset because it reaches a rendered
// frame: the command-pending + zero-projects frame falls through to the
// library's own line, which would otherwise paint in its hardcoded grey.
func canvasNoItemsStyle(l *list.Model, th theme.Theme) {
	l.Styles.NoItems = lipgloss.NewStyle().
		Foreground(th.TextMuted.Color()).
		Background(th.Canvas.Color())
}

// colourlessNoItemsStyle strips the zero-items body style for the NO_COLOR
// carve-out, so the line renders on the terminal's native fg/bg.
func colourlessNoItemsStyle(l *list.Model) {
	l.Styles.NoItems = lipgloss.NewStyle()
}

// canvasPaginationDots re-points the paginator's dot styles onto the theme's
// tokens over the canvas (active accent.primary, inactive text.faint); the
// engine's glyph and page count are untouched. list.New reads the dot styles
// into Paginator.ActiveDot/InactiveDot once at construction, so the restyled
// dots must be re-fed into the live paginator here.
func canvasPaginationDots(l *list.Model, th theme.Theme) {
	canvas := th.Canvas.Color()
	l.Styles.ActivePaginationDot = lipgloss.NewStyle().
		Foreground(th.AccentPrimary.Color()).
		Background(canvas).
		SetString(paginationDotGlyph)
	l.Styles.InactivePaginationDot = lipgloss.NewStyle().
		Foreground(th.TextFaint.Color()).
		Background(canvas).
		SetString(paginationDotGlyph)
	l.Paginator.ActiveDot = l.Styles.ActivePaginationDot.String()
	l.Paginator.InactiveDot = l.Styles.InactivePaginationDot.String()
	centrePaginationRow(l, lipgloss.NewStyle().Background(canvas))
}

// colourlessPaginationDots strips the paginator dot styles for the NO_COLOR
// carve-out, keeping the bullet glyph and centred placement without a canvas
// fill.
func colourlessPaginationDots(l *list.Model) {
	l.Styles.ActivePaginationDot = lipgloss.NewStyle().SetString(paginationDotGlyph)
	l.Styles.InactivePaginationDot = lipgloss.NewStyle().SetString(paginationDotGlyph)
	l.Paginator.ActiveDot = l.Styles.ActivePaginationDot.String()
	l.Paginator.InactiveDot = l.Styles.InactivePaginationDot.String()
	centrePaginationRow(l, lipgloss.NewStyle())
}

// centrePaginationRow re-points the paginator wrapper so the dot row renders
// centred across the list width (not the terminal width), replacing the
// engine default's left alignment. Re-run after each SetSize so the width
// stays current.
func centrePaginationRow(l *list.Model, base lipgloss.Style) {
	l.Styles.PaginationStyle = base.Width(l.Width()).Align(lipgloss.Center)
}

// sessionListTitleForMode computes the session list title for the active
// grouping mode, with the inside-tmux "(current: %s)" decoration composed as
// a suffix onto the mode base.
func sessionListTitleForMode(mode prefs.SessionListMode, insideTmux bool, currentSession string) string {
	var base string
	switch mode {
	case prefs.ModeByProject:
		base = "Sessions — by project"
	case prefs.ModeByTag:
		base = "Sessions — by tag"
	default:
		base = "Sessions"
	}
	if insideTmux && currentSession != "" {
		return fmt.Sprintf("%s (current: %s)", base, currentSession)
	}
	return base
}

// newSessionList creates and configures a new bubbles/list.Model for
// sessions. The built-in help renderer is disabled because Portal renders
// the descriptor-driven footer itself.
func newSessionList(items []list.Item) list.Model {
	l := list.New(items, SessionDelegate{}, 0, 0)
	l.Title = "Sessions"
	l.DisableQuitKeybindings()
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetStatusBarItemName("session", "sessions running")
	l.SetShowHelp(false)
	l.KeyMap.ShowFullHelp.Unbind()
	l.KeyMap.CloseFullHelp.Unbind()
	pinArrowOnlyNav(&l.KeyMap)
	l.InfiniteScrolling = true
	return l
}

// pinArrowOnlyNav rebinds the bubbles/list nav KeyMap to arrows only. The v2
// defaults re-introduce vim aliases and page-jump keys this codebase drops,
// and bind k to CursorUp, which would shadow the Sessions k=kill verb.
// Rebinding here means a banned key never reaches the list's own Update.
func pinArrowOnlyNav(km *list.KeyMap) {
	km.CursorUp.SetKeys("up")
	km.CursorDown.SetKeys("down")
	km.PrevPage.SetKeys("ctrl+up")
	km.NextPage.SetKeys("ctrl+down")
	km.GoToStart.SetKeys()
	km.GoToEnd.SetKeys()
}

// newProjectList creates and configures a bubbles/list.Model for projects;
// see newSessionList for the SetShowHelp(false) rationale.
func newProjectList() list.Model {
	l := list.New(nil, ProjectDelegate{}, 0, 0)
	l.Title = "Projects"
	l.DisableQuitKeybindings()
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetStatusBarItemName("project", "saved projects")
	l.SetShowHelp(false)
	l.KeyMap.ShowFullHelp.Unbind()
	l.KeyMap.CloseFullHelp.Unbind()
	pinArrowOnlyNav(&l.KeyMap)
	l.InfiniteScrolling = true
	return l
}

// New creates a Model that fetches sessions from the given SessionLister.
// Optional dependencies are configured via functional options.
func New(lister SessionLister, opts ...Option) Model {
	m := Model{
		sessionLister: lister,
		sessionList:   newSessionList(nil),
		projectList:   newProjectList(),
		activePage:    PageSessions,
		// Seed the active palette from the dark built-in before the options
		// apply, so a model constructed without Build is still themed — an
		// empty Theme renders silently colourless with no failing assertion.
		// An injected nomination overwrites it in syncResolvedMode below.
		themeState: themeState{active: defaultDarkTheme()},
	}
	for _, opt := range opts {
		opt(&m)
	}
	// Recompute the title now that mode / inside-tmux are known
	// (newSessionList set the Flat default), so the first frame paints the
	// correct mode heading.
	m.sessionList.Title = sessionListTitleForMode(m.sessionListMode, m.insideTmux, m.currentSession)
	// Initialise the gate from the nomination's shape. A constant (or absent)
	// nomination resolves immediately; an adaptive pair is constructed
	// resolved to the dark fallback so a directly constructed model still
	// paints — production opens the detect-or-timeout window explicitly via
	// Build. The NO_COLOR carve-out wins over every shape: no canvas to
	// select, so the gate is colourless and detection is skipped.
	if m.colourless {
		m.themeState.gate = newColourlessGate()
	} else if !m.themeState.gate.pinned {
		// A WithCanvasMode override set its own resolved gate in the options
		// loop; rebuilding from the shape would discard it.
		m.themeState.gate = newNominationGate(m.themeState.nomination)
	}
	m.syncResolvedMode()
	return m
}

// armAppearanceDetection opens the detect-or-timeout first-paint window on an
// adaptive gate; a no-op on a constant or absent nomination, under NO_COLOR,
// and under a WithCanvasMode override. Directly constructed models never call
// it, so they paint immediately.
func (m *Model) armAppearanceDetection() {
	m.themeState.gate.arm()
	m.syncResolvedMode()
}

func (m Model) modeResolved() bool {
	return m.themeState.gate.resolved()
}

func (m Model) hasNomination() bool {
	return m.themeState.nomination != (theme.Nomination{})
}

// syncResolvedMode adopts the gate's answer, selects the active palette, and
// re-applies the leaf canvas styles; called after every gate transition. With
// no nomination injected the active palette is left at New's dark seed —
// selecting from an empty nomination would hand back the zero Theme, which
// renders silently colourless.
func (m *Model) syncResolvedMode() {
	m.themeState.adoptGateAnswer()
	if m.hasNomination() {
		m.themeState.active = m.themeState.nomination.Select(m.themeState.inForceMode())
	}
	m.captureStartupCanvasHex()
	m.applyCanvasMode()
}

// captureStartupCanvasHex retains the canvas of the theme the gate selected,
// for the exit-time canvas-echo guard. While the detection window is open
// nothing has painted, so the hex is empty.
func (m *Model) captureStartupCanvasHex() {
	if !m.themeState.gate.resolved() {
		m.themeState.startupCanvasHex = ""
		return
	}
	m.themeState.startupCanvasHex = m.themeState.active.Canvas.Value
}

func (m *Model) sessionDelegate() SessionDelegate {
	return SessionDelegate{
		Theme:       m.themeState.active,
		Colourless:  m.colourless,
		MultiSelect: m.multiSelectMode,
		Selected:    m.selectedSessions,
		GoneFlagged: m.goneFlagged,
	}
}

// A marked-set mutation is invisible until the delegate is re-pointed at the
// live set — call this after mutating it.
func (m *Model) refreshSessionDelegate() {
	m.sessionList.SetDelegate(m.sessionDelegate())
}

// ApplyTheme swaps the palette every renderer paints from, mid-session. It is
// the restyle and nothing else: it must not rebuild the session list (too
// heavy for the arrow-preview path), read a file, or write startupCanvasHex,
// which is frozen at gate resolution — moving it would stick the wrong colour
// in the user's terminal on exit.
func (m *Model) ApplyTheme(th theme.Theme) {
	m.themeState.active = th
	m.applyCanvasMode()
}

// applyCanvasMode re-points every colour-bearing value that is assigned once
// rather than re-derived per frame; anything caching a colour needs a line
// here or it silently keeps the previous theme's. Deliberately excluded:
// startupCanvasHex (see ApplyTheme) and the bubbles/list styles whose render
// paths are disabled.
func (m *Model) applyCanvasMode() {
	m.styleFilterInput()
	// The preview caches a whole palette at open; re-point it or its chrome
	// keeps the theme that happened to be live when it was opened.
	m.preview.th = m.themeState.active
	applyPageListCanvasMode(&m.sessionList, m.sessionDelegate(), m.themeState.active, m.colourless)
	m.applyProjectCanvasMode()
	m.applyThemePanelCanvasMode()
}

// applyListCanvasMode is the single per-list restyle sequence — a step
// missing from a per-list copy would silently keep the previous theme's
// colours, so keep it in one home. The title-bar geometry is deliberately not
// here: only the two page lists carry it (applyPageListCanvasMode).
func applyListCanvasMode(l *list.Model, delegate list.ItemDelegate, th theme.Theme, colourless bool) {
	l.SetDelegate(delegate)
	if colourless {
		colourlessHelpStyles(l)
		colourlessNoItemsStyle(l)
		colourlessPaginationDots(l)
		l.Styles.TitleBar = l.Styles.TitleBar.UnsetBackground()
		l.Styles.Title = stripListTitleColours(l.Styles.Title)
		return
	}
	canvasHelpStyles(l, th)
	canvasNoItemsStyle(l, th)
	canvasPaginationDots(l, th)
	// Background the title bar so its left-pad cells are canvas rather than
	// the terminal background.
	l.Styles.TitleBar = l.Styles.TitleBar.Background(th.Canvas.Color())
	l.Styles.Title = stripListTitleColours(l.Styles.Title)
}

// applyPageListCanvasMode adds the title-bar geometry the two page lists
// carry and the slide-over's list does not, layered over the background the
// shared sequence just set so the gap row inherits the canvas.
func applyPageListCanvasMode(l *list.Model, delegate list.ItemDelegate, th theme.Theme, colourless bool) {
	applyListCanvasMode(l, delegate, th, colourless)
	l.Styles.TitleBar = listTitleBarStyle(l.Styles.TitleBar)
}

// stripListTitleColours drops the bubbles/list default Title box colours.
// Unset rather than re-pointed: the section-header surgery replaces the whole
// title line, so nothing paints the box — unsetting keeps the library's
// default violet out of a themed model.
func stripListTitleColours(title lipgloss.Style) lipgloss.Style {
	return title.UnsetBackground().UnsetForeground()
}

// listTitleBarBottomGap is one row of the page's vertical rhythm, not just
// padding; consumers measure it through listTitleBarStyle rather than
// restating the count.
const listTitleBarBottomGap = 1

// listTitleBarStyle applies the shared page-list TitleBar geometry.
// PaddingLeft(0) keeps the filter input at the same column as everything
// else; PaddingBottom makes the title bar two lines, which bubbles/list
// auto-budgets from the item area. Dependency: applySectionHeader's surgery
// replaces only line 0 and preserves the gap row.
func listTitleBarStyle(base lipgloss.Style) lipgloss.Style {
	return base.PaddingLeft(0).PaddingBottom(listTitleBarBottomGap)
}

// sectionHeaderBlockRows is measured through the real renderer, never
// restated, so the slide-over lands its label on the page's header row by
// construction rather than by a count that can drift.
func sectionHeaderBlockRows() int {
	return lipgloss.Height(listTitleBarStyle(lipgloss.NewStyle()).
		Render(renderSectionHeaderRow("", 0, theme.Theme{}, true)))
}

func (m *Model) applyProjectCanvasMode() {
	delegate := ProjectDelegate{Theme: m.themeState.active, Colourless: m.colourless}
	applyPageListCanvasMode(&m.projectList, delegate, m.themeState.active, m.colourless)
}

func (m *Model) styleFilterInput() {
	m.styleListFilterInput(&m.sessionList)
	m.styleListFilterInput(&m.projectList)
}

// styleListFilterInput applies the shared filter-input treatment. No
// .Background(canvas) here deliberately — the input renders over the canvas
// the title bar already paints. Cursor blink is disabled so captured frames
// are deterministic.
func (m *Model) styleListFilterInput(l *list.Model) {
	l.FilterInput.Prompt = filterPromptPrefix
	styles := l.FilterInput.Styles()
	if m.colourless {
		// Pin hue-free styles so no accent SGR is emitted at all, beyond what
		// the writer layer would strip.
		styles.Focused.Prompt = lipgloss.NewStyle()
		styles.Focused.Text = lipgloss.NewStyle()
		styles.Cursor.Color = lipgloss.NoColor{}
		styles.Cursor.Blink = false
		l.FilterInput.SetStyles(styles)
		return
	}
	orange := m.themeState.active.AccentAttention.Color()
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(orange)
	styles.Focused.Text = lipgloss.NewStyle().Foreground(orange)
	styles.Cursor.Color = orange
	styles.Cursor.Blink = false
	l.FilterInput.SetStyles(styles)
}

// NewModelWithSessions creates a Model pre-populated with sessions.
func NewModelWithSessions(sessions []tmux.Session) Model {
	items := ToListItems(sessions)
	l := newSessionList(items)
	pl := newProjectList()
	m := Model{
		sessions:    sessions,
		sessionList: l,
		projectList: pl,
		activePage:  PageSessions,
		// Seed the active palette as New does: a zero Theme renders silently
		// colourless. The zero-value gate is already resolved, so this model
		// paints immediately.
		themeState: themeState{active: defaultDarkTheme()},
	}
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	return m
}

// applyListSize pairs SetSize with the paginator re-centre (the centred
// PaginationStyle pins an explicit Width, so it must track every resize).
// Callers invoke a per-page wrapper, which owns the reserved-height
// computation, not this core directly.
func (m *Model) applyListSize(l *list.Model, width, height, reserved int) {
	l.SetSize(width, height-reserved)
	if m.colourless {
		centrePaginationRow(l, lipgloss.NewStyle())
		return
	}
	centrePaginationRow(l, lipgloss.NewStyle().Background(m.themeState.active.Canvas.Color()))
}

// applySessionListSize owns the Sessions height budget: header, footer, and
// any active notice band are reserved here — counted rows, not uncounted
// bands — so the composed view never overflows termH. Each reserve resolves
// against the same width as this size-apply, so budget and render agree at
// every call site.
func (m *Model) applySessionListSize(width, height int) {
	reserved := m.sessionBandHeight() + m.headerHeight(width) + m.sessionFooterHeight(width)
	m.applyListSize(&m.sessionList, width, height, reserved)
}

// sessionFooterHeight measures the footer against the same entries, width
// and mode the render uses, so budget and render agree exactly — including
// in a blocked state.
func (m Model) sessionFooterHeight(width int) int {
	return lipgloss.Height(renderSessionsFooter(m.sessionsHelpKeymap(), width, m.themeState.active, m.colourless))
}

// sessionBandHeight is measured off renderSessionBandSlot — the same block
// viewSessionList composes — so the reserved rows are by construction exactly
// what is inserted.
func (m *Model) sessionBandHeight() int {
	slot := m.renderSessionBandSlot()
	if slot == "" {
		return 0
	}
	return lipgloss.Height(slot)
}

// applyProjectListSize owns the Projects height budget: header, footer and
// band are reserved here, each against the same width as this size-apply, so
// the budget and the render agree at every call site.
func (m *Model) applyProjectListSize(width, height int) {
	reserved := m.headerHeight(width) + m.projectFooterHeight(width) + m.projectBandHeight()
	m.applyListSize(&m.projectList, width, height, reserved)
}

// projectFooterHeight measures the footer against the same entries, width and
// mode the render uses, so budget and render agree exactly — blocked state
// included.
func (m Model) projectFooterHeight(width int) int {
	return lipgloss.Height(renderProjectsFooter(m.projectsHelpKeymap(), width, m.themeState.active, m.colourless))
}

// projectBandHeight is measured off renderProjectBandSlot — the same block
// viewProjectList composes — so the reserved rows are by construction exactly
// what is inserted.
func (m Model) projectBandHeight() int {
	slot := m.renderProjectBandSlot()
	if slot == "" {
		return 0
	}
	return lipgloss.Height(slot)
}

// filteredSessions returns sessions with the current session excluded when inside tmux.
func (m Model) filteredSessions() []tmux.Session {
	if !m.insideTmux || m.currentSession == "" {
		return m.sessions
	}
	filtered := make([]tmux.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.Name != m.currentSession {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func (m *Model) applySessions(sessions []tmux.Session) tea.Cmd {
	m.sessions = sessions
	// Prune before the rebuild so the refreshed set feeds the delegate's ●.
	m.pruneSelectionToLiveSessions()
	return m.rebuildSessionList()
}

// pruneSelectionToLiveSessions drops marked sessions no longer present in
// m.sessions — a set-difference on the aliased map, so the delegate drops
// their ● on the next frame without a re-point.
func (m *Model) pruneSelectionToLiveSessions() {
	if len(m.selectedSessions) == 0 {
		return
	}
	live := make(map[string]struct{}, len(m.sessions))
	for i := range m.sessions {
		live[m.sessions[i].Name] = struct{}{}
	}
	for name := range m.selectedSessions {
		if _, ok := live[name]; !ok {
			delete(m.selectedSessions, name)
		}
	}
}

// An out-of-range value collapses to Flat so the cycle can never get stuck
// on an unrecognised mode.
func nextSessionListMode(mode prefs.SessionListMode) prefs.SessionListMode {
	switch mode {
	case prefs.ModeFlat:
		return prefs.ModeByProject
	case prefs.ModeByProject:
		return prefs.ModeByTag
	case prefs.ModeByTag:
		return prefs.ModeFlat
	default:
		return prefs.ModeFlat
	}
}

// anyTagsExist is directory-tag presence, independent of which sessions are
// live — so tags existing on no live session still keeps the signpost away.
func anyTagsExist(projects []project.Project) bool {
	for _, p := range projects {
		if len(p.Tags) > 0 {
			return true
		}
	}
	return false
}

// setProjects updates the cached records and their derived index together.
// Always mutate projects through it, or the index goes stale.
func (m *Model) setProjects(projects []project.Project) {
	m.projects = projects
	m.projectIndex = project.NewIndex(projects)
}

// resolveSessionDirs derives a directory for each session whose @portal-dir
// is absent and caches it back into m.sessions. Best-effort: an unresolvable
// session keeps an empty Dir and is re-attempted next rebuild. The result is
// never stamped back to tmux (see dirReader). Call it only from the grouped
// arms — Flat must pay zero pane reads.
func (m *Model) resolveSessionDirs(sessions []tmux.Session) []tmux.Session {
	if m.dirReader == nil || m.dirRunner == nil {
		return sessions
	}

	resolved := make([]tmux.Session, len(sessions))
	for i, s := range sessions {
		if s.Dir == "" {
			if dir, ok, err := session.ResolveSessionDir(s.Name, m.dirReader, m.dirRunner); ok && err == nil {
				s.Dir = dir
				m.cacheSessionDir(s.Name, dir)
			}
		}
		resolved[i] = s
	}
	return resolved
}

// cacheSessionDir is in-memory only: a SessionsMsg refresh replaces
// m.sessions, so the guess is re-derived rather than frozen.
func (m *Model) cacheSessionDir(name, dir string) {
	for i := range m.sessions {
		if m.sessions[i].Name == name {
			m.sessions[i].Dir = dir
			return
		}
	}
}

func (m *Model) rebuildSessionList() tea.Cmd {
	filtered := m.filteredSessions()

	// By Tag with no tags anywhere degrades to the flat list plus a signpost,
	// rather than silently flattening.
	m.byTagSignpost = m.sessionListMode == prefs.ModeByTag && !anyTagsExist(m.projects)

	var items []list.Item
	switch {
	case m.byTagSignpost:
		items = ToListItems(filtered)
	case m.sessionListMode == prefs.ModeByProject:
		items = buildByProject(m.resolveSessionDirs(filtered), m.projectIndex)
	case m.sessionListMode == prefs.ModeByTag:
		items = buildByTag(m.resolveSessionDirs(filtered), m.projectIndex)
	default:
		items = ToListItems(filtered)
	}

	cmd := m.sessionList.SetItems(items)

	m.sessionList.Title = sessionListTitleForMode(m.sessionListMode, m.insideTmux, m.currentSession)

	// Without this re-apply the list keeps sizing against pre-load defaults
	// and overflows under the footer once items populate.
	if m.termWidth > 0 || m.termHeight > 0 {
		m.applySessionListSize(m.contentWidth(), m.contentHeight())
	}

	m.ensureSessionRowSelected()
	return cmd
}

// ensureSessionRowSelected nudges the selection off a non-selectable
// HeaderItem. A single step suffices: no two headers are ever adjacent.
func (m *Model) ensureSessionRowSelected() {
	if _, isHeader := m.sessionList.SelectedItem().(HeaderItem); isHeader {
		m.sessionList.CursorDown()
	}
}

// skipHeaderRow keeps the cursor off group headers after a navigation key,
// stepping once more in the direction the user was moving. At the top row an
// upward intent flips downward so the selection never falls off the list.
func (m *Model) skipHeaderRow(msg tea.KeyPressMsg) {
	if _, isHeader := m.sessionList.SelectedItem().(HeaderItem); !isHeader {
		return
	}
	km := m.sessionList.KeyMap
	if key.Matches(msg, km.CursorUp, km.PrevPage, km.GoToStart) && m.sessionList.Index() > 0 {
		m.sessionList.CursorUp()
		return
	}
	m.sessionList.CursorDown()
}

// evaluateDefaultPage picks the landing page once the data it depends on has
// loaded, and applies the deferred initial filter and cursor. It runs at most
// once per model.
func (m *Model) evaluateDefaultPage() {
	if m.defaultPageEvaluated {
		return
	}
	if m.commandPending {
		if !m.projectsLoaded {
			return
		}
	} else if !m.sessionsLoaded || !m.projectsLoaded {
		return
	}
	m.defaultPageEvaluated = true
	if m.commandPending {
		m.activePage = PageProjects
	} else if len(m.sessionList.Items()) > 0 {
		m.activePage = PageSessions
	} else {
		m.activePage = PageProjects
	}

	m.applyInitialFilter()
	m.applyInitialCursor()
}

// applyInitialFilter commits the deferred initial filter onto the settled
// page's list, then clears it so it applies at most once.
func (m *Model) applyInitialFilter() {
	if m.initialFilter == "" {
		return
	}
	if m.activePage == PageSessions && !m.commandPending {
		m.sessionList.SetFilterText(m.initialFilter)
		m.sessionList.SetFilterState(list.FilterApplied)
	} else {
		m.projectList.SetFilterText(m.initialFilter)
		m.projectList.SetFilterState(list.FilterApplied)
	}
	m.initialFilter = ""
}

// applyInitialCursor re-anchors the session-list cursor onto the seeded row
// name, then clears it. Runs after applyInitialFilter so the re-anchor
// operates on the post-filter visible set.
func (m *Model) applyInitialCursor() {
	if m.initialCursor == "" {
		return
	}
	if m.activePage == PageSessions {
		m.reanchorSessionCursor(m.initialCursor)
	}
	m.initialCursor = ""
}

// refreshSessionsAfterPreviewCmd re-fetches the sessions list on preview
// dismissal so an externally killed session disappears. Returns nil when no
// SessionLister is wired — callers must tolerate a nil cmd.
func (m Model) refreshSessionsAfterPreviewCmd(preserveName string) tea.Cmd {
	if m.sessionLister == nil {
		return nil
	}
	lister := m.sessionLister
	return func() tea.Msg {
		sessions, err := lister.ListSessions()
		return previewSessionsRefreshedMsg{
			Sessions:     sessions,
			Err:          err,
			PreserveName: preserveName,
		}
	}
}

// exitPreviewToSessions tears the preview down and returns the refresh
// command anchored on preserveName (nil when no SessionLister is wired).
// Capture preserveName before calling: m.preview is zeroed in place, so
// reading m.preview.session afterwards yields an empty string.
func (m *Model) exitPreviewToSessions(preserveName string) tea.Cmd {
	m.activePage = PageSessions
	m.preview = previewModel{}
	return m.refreshSessionsAfterPreviewCmd(preserveName)
}

// reanchorSessionCursor moves the cursor onto the visible item matching name,
// clamping to the last index when the name is gone. Operates on filtered
// (visible) order so it lands correctly under a committed filter.
func (m *Model) reanchorSessionCursor(name string) {
	if name == "" {
		return
	}
	visible := m.sessionList.VisibleItems()
	if len(visible) == 0 {
		return
	}
	for i, it := range visible {
		si, ok := it.(SessionItem)
		if !ok {
			continue
		}
		if si.Session.Name == name {
			m.sessionList.Select(i)
			return
		}
	}
	m.sessionList.Select(len(visible) - 1)
}

// setFlash records an inline-flash message, bumping flashGen. Callers that
// schedule a delayed clear must capture the post-bump generation and compare
// it on fire, so a stale tick from a superseded flash cannot early-clear the
// current one. setFlash("") still bumps the generation.
func (m *Model) setFlash(text string) {
	m.flashGen++
	m.flashText = text
	// Reset both defaults here so a success flash or a theme flash cannot be
	// inherited by an unrelated message raised after it.
	m.flashKind = flashWarning
	m.flashOrigin = flashOriginDefault
	m.resyncPageLayouts()
}

// setThemeFlash is setFlash plus the origin stamp giving the flash
// precedence over the filter line. Raise every theme signal through it: the
// tier is granted by the setter, not the wording, so a copy edit cannot move
// a signal out of the tier.
func (m *Model) setThemeFlash(text string) {
	m.setFlash(text)
	m.flashOrigin = flashOriginTheme
}

// setSuccessFlash is setFlash plus the success kind (✓ glyph, never
// colour-only).
func (m *Model) setSuccessFlash(text string) {
	m.setFlash(text)
	m.flashKind = flashSuccess
}

// clearFlash zeros flashText, deliberately leaving flashGen untouched so
// in-flight ticks keep comparing against the same monotonic sequence.
func (m *Model) clearFlash() {
	m.flashText = ""
	m.resyncPageLayouts()
}

// resyncPageLayouts re-applies both page lists' sizes for the current notice
// band state; a no-op before the first WindowSizeMsg. Both, not just the
// active page: a page can be entered by a message rather than a keypress, and
// sizing only the set-time page leaves the entered one budgeted for no band,
// cutting its footer off the bottom.
func (m *Model) resyncPageLayouts() {
	if m.termWidth <= 0 || m.termHeight <= 0 {
		return
	}
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
}

// loadProjects returns a command that cleans stale projects and loads the list.
func (m Model) loadProjects() tea.Cmd {
	if m.projectStore == nil {
		return nil
	}
	return func() tea.Msg {
		_, _ = m.projectStore.CleanStale()
		projects, err := m.projectStore.List()
		return ProjectsLoadedMsg{Projects: projects, Err: err}
	}
}

// fetchSessionsCmd enumerates live tmux sessions into a SessionsMsg. A pure
// read — it never mutates tmux or state.
func (m Model) fetchSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.sessionLister.ListSessions()
		return SessionsMsg{Sessions: sessions, Err: err}
	}
}

// refetchSessionsAfterRestore fixes the cold-boot route's stale snapshot:
// there the orchestrator runs in a goroutine, so Init's frame-one enumeration
// happened before Restore created the saved sessions. The warm route returns
// nil — its snapshot is already post-restore, so a re-fetch would be wasted.
func (m Model) refetchSessionsAfterRestore() tea.Cmd {
	if m.progressReceiver == nil {
		return nil
	}
	return m.fetchSessionsCmd()
}

// transitionFromLoading moves from the loading page to the sessions page. On
// the cold concurrent route it deliberately leaves sessionsLoaded false:
// deciding the landing page here would latch defaultPageEvaluated against the
// stale pre-restore list, and the post-restore refetch could never re-decide.
func (m *Model) transitionFromLoading() {
	m.activePage = PageSessions
	if m.progressReceiver != nil {
		return
	}
	m.sessionsLoaded = true
	m.evaluateDefaultPage()
}

// deleteAndRefreshProjects removes a project and reloads the list; errors
// from Remove are propagated via ProjectsLoadedMsg.
func (m Model) deleteAndRefreshProjects(path string) tea.Cmd {
	return func() tea.Msg {
		if err := m.projectStore.Remove(path, "cli"); err != nil {
			return ProjectsLoadedMsg{Err: fmt.Errorf("failed to delete project '%s': %w", path, err)}
		}
		_, _ = m.projectStore.CleanStale()
		projects, err := m.projectStore.List()
		return ProjectsLoadedMsg{Projects: projects, Err: err}
	}
}

// Init fetches tmux sessions (or projects in command-pending mode) and, on
// the loading page, schedules the LoadingMinElapsedMsg tick. Dismissing the
// loading page requires that tick and BootstrapCompleteMsg.
func (m Model) Init() tea.Cmd {
	// The OSC 11 query is async and non-gating: a missing response just
	// leaves originalBg empty. It is issued regardless of the nomination's
	// shape — a constant skips the gate, never the query — because the
	// exit-time background restore and the mid-session constant → adaptive
	// conversion both need the reply already in hand.
	requestBg := tea.Cmd(tea.RequestBackgroundColor)

	// Races the OSC 11 query so a non-responding terminal still resolves to
	// the dark fallback; whichever fires first resolves the answer and the
	// loser is ignored. Nil for an already-resolved gate, harmless in Batch.
	detectTimeout := m.themeState.gate.timeoutCmd()

	if m.commandPending {
		return tea.Batch(requestBg, detectTimeout, m.loadProjects())
	}
	fetchSessions := m.fetchSessionsCmd()
	loadProjects := m.loadProjects()

	if m.activePage == PageLoading {
		loadingPadTick := tea.Tick(LoadingMinDuration, func(time.Time) tea.Msg {
			return LoadingMinElapsedMsg{}
		})
		cmds := []tea.Cmd{requestBg, detectTimeout, fetchSessions, loadingPadTick}
		if m.progressReceiver != nil {
			// The channel owns the terminal BootstrapCompleteMsg; synthesizing
			// one here would dismiss the loading page before the orchestrator
			// finished.
			cmds = append(cmds, m.progressReceiver)
		} else {
			// Warm route: the orchestrator already ran before this Init, so
			// satisfy the bootstrapComplete gate from the first tick.
			pending := m.pendingBootstrapWarnings
			cmds = append(cmds, func() tea.Msg { return BootstrapCompleteMsg{Warnings: pending} })
		}
		if loadProjects != nil {
			cmds = append(cmds, loadProjects)
		}
		return tea.Batch(cmds...)
	}

	if loadProjects != nil {
		return tea.Batch(requestBg, detectTimeout, fetchSessions, loadProjects)
	}
	return tea.Batch(requestBg, detectTimeout, fetchSessions)
}

func (m Model) Update(msg tea.Msg) (model tea.Model, cmd tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.termWidth = wsm.Width
		m.termHeight = wsm.Height
		// Lists size to the inset content region, not the full terminal, so
		// the composed view fits inside the global gutter.
		m.applySessionListSize(m.contentWidth(), m.contentHeight())
		m.applyProjectListSize(m.contentWidth(), m.contentHeight())
		// Sequenced after the two page lists: a forced panel close raises a
		// notice band, which re-syncs the layouts they just set. Its command
		// must neither be swallowed nor returned here (the message still has
		// to reach the arms below), so it is deferred onto whichever return
		// fires.
		if closed := (&m).resizeThemePanel(); closed != nil {
			defer func() { cmd = tea.Batch(closed, cmd) }()
		}
	}

	// Cross-view messages, handled regardless of the active page.
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		// Order matters: the reply is retained unconditionally, and only then
		// offered to the gate — a late reply must still reach restore-on-exit
		// and the mid-session conversion without re-theming. The nil guard is
		// required: BackgroundColorMsg.String() panics on a nil Color.
		// COLORFGBG is deliberately not consulted — OSC 11 is authoritative.
		m.themeState.reply = terminalReplyFrom(msg)
		if msg.Color != nil {
			m.originalBg = msg.String()
		}
		if m.themeState.gate.resolve(m.themeState.reply.member) {
			m.syncResolvedMode()
		}
		return m, nil
	case appearanceTimeoutMsg:
		// A no-op once resolved, so a timeout that lost the race to the OSC 11
		// reply never re-resolves — no second resolution, no flip.
		if m.themeState.gate.resolveDark() {
			m.syncResolvedMode()
		}
		return m, nil
	case SessionsMsg:
		if msg.Err != nil {
			return m, tea.Quit
		}
		cmd := m.applySessions(msg.Sessions)

		// While loading, ingest the list but leave sessionsLoaded alone:
		// dismissal is gated on BootstrapCompleteMsg + the min-elapsed tick,
		// and transitionFromLoading owns the flip.
		if m.activePage == PageLoading {
			return m, cmd
		}

		m.sessionsLoaded = true
		m.evaluateDefaultPage()
		return m, tea.Batch(cmd, m.maybeDispatchDetectionCmd())
	case LoadingMinElapsedMsg:
		m.minElapsed = true
		// A fatal parks the model in the error state — never dismiss into the
		// picker, even if this gate fires afterwards.
		if m.fatalActive {
			return m, nil
		}
		if m.bootstrapComplete && m.activePage == PageLoading {
			m.transitionFromLoading()
			return m, tea.Batch(m.surfaceBufferedWarnings(), m.refetchSessionsAfterRestore(), m.maybeDispatchDetectionCmd())
		}
		return m, nil
	case BootstrapProgressMsg:
		// Non-terminal: re-issuing the single blocking receive preserves exact
		// event order even though Bubble Tea batches commands. It never drives
		// the loading transition, which keeps the TUI inert during loading.
		m.loadingProgress = m.loadingProgress.Apply(msg)
		return m, m.progressReceiver
	case BootstrapCompleteMsg:
		// Defensive: a stray complete must never dismiss the error frame into
		// a picker.
		if m.fatalActive {
			return m, nil
		}
		m.bootstrapComplete = true
		// Warnings arriving after dismissal are dropped so they cannot
		// accumulate dead state on the model.
		if m.activePage == PageLoading {
			m.bufferedWarnings = msg.Warnings
		}
		if m.minElapsed && m.activePage == PageLoading {
			m.transitionFromLoading()
			return m, tea.Batch(m.surfaceBufferedWarnings(), m.refetchSessionsAfterRestore(), m.maybeDispatchDetectionCmd())
		}
		return m, nil
	case BootstrapFatalMsg:
		// The model must stay on PageLoading — never a half-restored picker —
		// and this is a terminal event, so the receiver is not re-issued.
		m.fatalActive = true
		m.fatalStep = msg.FailedStep
		m.fatalMessage = msg.Message
		m.fatalErr = msg.Err
		return m, nil
	case ProjectsLoadedMsg:
		var setItemsCmd tea.Cmd
		if msg.Err == nil {
			m.setProjects(msg.Projects)
			items := ProjectsToListItems(msg.Projects)
			setItemsCmd = m.projectList.SetItems(items)
			if m.termWidth > 0 || m.termHeight > 0 {
				m.applyProjectListSize(m.contentWidth(), m.contentHeight())
			}
			// Startup-ordering correction: sessions and projects load
			// concurrently, so a SessionsMsg that arrived first grouped every
			// session against empty projects and dumped them in the catch-all.
			// Re-group now the records are cached.
			grouped := m.sessionListMode == prefs.ModeByProject || m.sessionListMode == prefs.ModeByTag
			if grouped && len(m.sessions) > 0 {
				setItemsCmd = tea.Batch(setItemsCmd, (&m).rebuildSessionList())
			}
		}
		m.projectsLoaded = true
		m.evaluateDefaultPage()
		return m, setItemsCmd
	case SessionCreatedMsg:
		m.selected = msg.SessionName
		return m, tea.Quit
	case sessionCreateErrMsg:
		return m, nil
	case previewDismissedMsg:
		// The sessionList is deliberately not mutated: cursor position and
		// filter state must round-trip byte-identically. Capture preserveName
		// before exitPreviewToSessions — it zeroes m.preview.
		captured := m.preview.session
		return m, m.exitPreviewToSessions(captured)
	case previewAttachBailMsg:
		// Batch, not Sequence: the flash must render on the same frame as the
		// page flip, with list consistency converging a render or two later.
		refreshCmd := m.exitPreviewToSessions(msg.Session)
		m.setFlash(formatSessionGoneFlash(msg.Session))
		return m, tea.Batch(refreshCmd, flashTickCmd(m.flashGen))
	case previewAttachSelectedMsg:
		// Record and quit so the connector runs after the program shuts down.
		// Connecting from inside the event loop would leave an orphan portal
		// process event-looping with no UI after switch-client moved the
		// client away.
		m.selected = msg.Session
		return m, tea.Quit
	case previewSessionsRefreshedMsg:
		// Lister errors are non-fatal: the pre-refresh snapshot is the best
		// information left, and the user expects to land on the list.
		if msg.Err != nil {
			return m, nil
		}
		cmd := m.applySessions(msg.Sessions)
		m.reanchorSessionCursor(msg.PreserveName)
		return m, cmd
	case flashTickMsg:
		// Generation guard: a tick belonging to a superseded flash is dropped
		// rather than early-clearing the current one.
		if msg.Gen == m.flashGen {
			m.clearFlash()
		}
		return m, nil
	case terminalDetectedMsg:
		// Adapter and resolution come from one resolve and are cached
		// together, so the burst's gate decision and its dispatch adapter can
		// never disagree.
		m.detectIdentity = msg.identity
		m.detectAdapter, m.detectResolution = m.resolve(msg.identity)
		m.detectResolved = true
		// Resolve a deferred Enter now the terminal is known. The marked set
		// is re-derived live rather than replayed from an Enter-time snapshot:
		// the defer window does not engage the input lock, so a toggle between
		// the Enter and this reply must be honoured.
		if m.pendingBurstEnter {
			return m.decideBurst(m.orderedMarkedSessions())
		}
		return m, nil
	case spawnProgressMsg:
		// burstTotal is deliberately not touched: it is N including the
		// trigger, while msg.Total is the external count N-1 — assigning it
		// would shrink the total mid-burst.
		m.burstDone = msg.Done
		if m.burstPipe == nil {
			return m, nil
		}
		return m, m.burstPipe.receiver()
	case spawnCompleteMsg:
		// Full success self-attaches the trigger through the ordinary
		// selected+Quit path, giving net N windows rather than N+1. Anything
		// else is leave-what-opened. A user cancel routes there too even when
		// the outcome is all-confirmed — Ctrl-C/Esc must never attach.
		if m.burstAllConfirmed(msg) && !m.burstCancelled {
			// Emit before the Quit handoff.
			m.emitBurstSummary(msg.Batch, msg.Identity, msg.Resolution, msg.Results, true)
			m.selected = m.burstTrigger
			(&m).resetBurstState()
			return m, tea.Quit
		}
		return m.handleBurstPartialFailure(msg)
	case spawnAbortMsg:
		// The burst aborted before spawning, so nothing opened and there is
		// no self-attach.
		return m.handlePreflightAbort(msg), nil
	case burstChannelClosedMsg:
		// Post-terminal sentinel; the terminal event already cleared pending.
		return m, nil
	}

	// The open slide-over owns the keyboard, so its arm sits ahead of the
	// page dispatch. Only key input is intercepted — resizes, refreshes and
	// the appearance gate's messages must keep reaching the model beneath,
	// which stays live and visible.
	if m.themePanel.open {
		if keyMsg, isKey := msg.(tea.KeyPressMsg); isKey {
			return m.updateThemePanel(keyMsg)
		}
	}

	switch m.activePage {
	case PageLoading:
		keyMsg, isKey := msg.(tea.KeyPressMsg)
		if isKey && keyIsCtrlC(keyMsg) {
			return m, tea.Quit
		}
		// Outside the fatal state the loading page stays inert (animation
		// only) for race containment, so q/Esc quit only once it is active.
		if m.fatalActive && isKey && (isRuneKey(keyMsg, "q") || keyIsCode(keyMsg, tea.KeyEscape)) {
			return m, tea.Quit
		}
		return m, nil
	case PageProjects:
		return m.updateProjectsPage(msg)
	case pagePreview:
		// A WindowSizeMsg is rewritten to the inset content-region dims so the
		// preview's chrome sits inside the global gutter.
		var cmd tea.Cmd
		m.preview, cmd = m.preview.Update(insetWindowSizeMsg(msg, m.contentWidth(), m.contentHeight()))
		return m, cmd
	default:
		return m.updateSessionList(msg)
	}
}

func (m Model) createSession(dir string) tea.Cmd {
	return func() tea.Msg {
		name, err := m.sessionCreator.CreateFromDir(dir, m.command)
		if err != nil {
			return sessionCreateErrMsg{Err: err}
		}
		return SessionCreatedMsg{SessionName: name}
	}
}

func (m Model) selectedProjectItem() (ProjectItem, bool) {
	item := m.projectList.SelectedItem()
	if item == nil {
		return ProjectItem{}, false
	}
	pi, ok := item.(ProjectItem)
	return pi, ok
}

func (m Model) updateProjectsPage(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.modal != modalNone {
		return m.updateModal(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// One key, one intent: an actionable key clears the flash as a side
		// effect and deliberately falls through to its normal handler. Keep
		// this in the same position as updateSessionList's clear or the rule
		// forks per page.
		if m.flashText != "" && isActionableKey(msg) {
			m.clearFlash()
		}
		if keyIsCtrlC(msg) {
			return m, tea.Quit
		}
		if m.projectList.SettingFilter() {
			break
		}
		// Opening our modal also consumes the key, so bubbles/list never
		// toggles its own help.
		if isRuneKey(msg, "?") {
			m.modal = modalHelp
			return m, nil
		}
		switch {
		case keyIsCode(msg, tea.KeyEscape):
			if m.projectList.FilterState() == list.FilterApplied {
				break
			}
			return m, tea.Quit
		case isRuneKey(msg, "q"):
			return m, tea.Quit
		case isRuneKey(msg, "x"):
			if m.commandPending {
				return m, nil
			}
			m.activePage = PageSessions
			// Re-group refresh so tag edits made in the projects modal are
			// visible on return. Empty preserveName: no anchored cursor on a
			// page switch.
			return m, m.refreshSessionsAfterPreviewCmd("")
		case isRuneKey(msg, "n"):
			return m.handleNewInCWD()
		case isRuneKey(msg, "d"):
			if m.commandPending {
				return m, nil
			}
			return m.handleDeleteProjectKey()
		case isRuneKey(msg, "e"):
			if m.commandPending {
				return m, nil
			}
			return m.handleEditProjectKey()
		// Below the SettingFilter guard, so t stays a literal filter character
		// while the / input is focused — as on Sessions.
		case isRuneKey(msg, "t"):
			return m.handleThemePanelKey()
		case keyIsCode(msg, tea.KeyEnter):
			return m.handleProjectEnter()
		}
	}

	var cmd tea.Cmd
	m.projectList, cmd = m.projectList.Update(msg)
	return m, cmd
}

func (m Model) handleProjectEnter() (tea.Model, tea.Cmd) {
	pi, ok := m.selectedProjectItem()
	if !ok {
		return m, nil
	}
	if m.sessionCreator == nil {
		return m, nil
	}
	return m, m.createSession(pi.Project.Path)
}

func (m Model) handleDeleteProjectKey() (tea.Model, tea.Cmd) {
	pi, ok := m.selectedProjectItem()
	if !ok {
		return m, nil
	}
	if m.projectStore == nil {
		return m, nil
	}
	m.modal = modalDeleteProject
	m.pendingDeletePath = pi.Project.Path
	m.pendingDeleteName = pi.Project.Name
	return m, nil
}

func (m Model) updateDeleteProjectModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch {
	case isRuneKey(keyMsg, "y"):
		path := m.pendingDeletePath
		m.modal = modalNone
		m.pendingDeletePath = ""
		m.pendingDeleteName = ""
		return m, m.deleteAndRefreshProjects(path)
	case keyIsCode(keyMsg, tea.KeyEscape):
		// Cancel is Esc only; clear both pending fields so no stale state
		// leaks past dismissal.
		m.modal = modalNone
		m.pendingDeletePath = ""
		m.pendingDeleteName = ""
		return m, nil
	}
	return m, nil
}

func (m Model) handleEditProjectKey() (tea.Model, tea.Cmd) {
	pi, ok := m.selectedProjectItem()
	if !ok {
		return m, nil
	}
	if m.projectEditor == nil || m.aliasEditor == nil {
		return m, nil
	}

	m.modal = modalEditProject
	m.editProject = pi.Project
	m.editName = pi.Project.Name
	m.editMode = editModeNavigate
	m.editFocus = editFieldName
	m.editChanged = false
	m.editAliasCursor = 0
	m.editTagCursor = 0
	m.resetEditBuffer()

	// Clone so mutating the buffer never aliases the stored project slice.
	m.editTags = slices.Clone(pi.Project.Tags)

	// Load() returns a map, so sort the matches for a stable chip order.
	allAliases, err := m.aliasEditor.Load()
	if err != nil {
		m.editAliases = nil
	} else {
		var matching []string
		for name, path := range allAliases {
			if path == pi.Project.Path {
				matching = append(matching, name)
			}
		}
		sort.Strings(matching)
		m.editAliases = matching
	}

	return m, nil
}

func (m *Model) resetEditBuffer() {
	m.editBuffer = ""
	m.editCursor = 0
	m.editIsNewChip = false
}

func (m Model) updateEditProjectModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.editMode == editModeEdit {
		return m.updateEditModeKey(keyMsg)
	}
	return m.updateNavigateModeKey(keyMsg)
}

func (m Model) updateNavigateModeKey(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.Code {
	case tea.KeyEscape:
		return m.closeEditModal()

	case tea.KeyTab:
		if keyMsg.Mod&tea.ModShift != 0 {
			m.focusField(m.prevField())
		} else {
			m.focusField(m.nextField())
		}
		return m, nil

	case tea.KeyDown:
		m.focusField(m.nextField())
		return m, nil

	case tea.KeyUp:
		m.focusField(m.prevField())
		return m, nil

	case tea.KeyLeft:
		m.moveElement(-1)
		return m, nil

	case tea.KeyRight:
		m.moveElement(+1)
		return m, nil

	case tea.KeyEnter:
		return m.enterEditFromNavigate()

	default:
		if keyMsg.Mod != 0 || keyMsg.Text == "" {
			return m, nil
		}
		switch keyMsg.Text {
		case "x":
			return m.deleteFocusedChip()
		case "e":
			// The + add slot uses `+`/Enter, not `e`.
			if m.editFocus == editFieldName || m.focusedOnChip() {
				return m.enterEditFromNavigate()
			}
			return m, nil
		case "+":
			if m.focusedOnAddSlot() {
				return m.enterEditFromNavigate()
			}
			return m, nil
		}
		return m, nil
	}
}

func (m Model) updateEditModeKey(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyMsg.Code {
	case tea.KeyEscape:
		return m.discardEdit()

	case tea.KeyEnter:
		return m.commitEdit()

	case tea.KeyLeft:
		if m.editCursor > 0 {
			m.editCursor--
		}
		return m, nil

	case tea.KeyRight:
		if m.editCursor < len([]rune(m.editBuffer)) {
			m.editCursor++
		}
		return m, nil

	case tea.KeyBackspace:
		m.backspaceEditBuffer()
		return m, nil

	default:
		// In edit mode x is a literal character, not a delete.
		if keyMsg.Mod != 0 || keyMsg.Text == "" {
			return m, nil
		}
		m.insertEditRunes(keyMsg.Text)
		return m, nil
	}
}

func (m Model) nextField() editField {
	switch m.editFocus {
	case editFieldName:
		return editFieldAliases
	case editFieldAliases:
		return editFieldTags
	default:
		return editFieldName
	}
}

func (m Model) prevField() editField {
	switch m.editFocus {
	case editFieldName:
		return editFieldTags
	case editFieldTags:
		return editFieldAliases
	default:
		return editFieldName
	}
}

// focusField lands a chip field on its trailing + add slot — the common
// action; ← then reaches the existing chips.
func (m *Model) focusField(field editField) {
	m.editFocus = field
	switch field {
	case editFieldAliases:
		m.editAliasCursor = len(m.editAliases)
	case editFieldTags:
		m.editTagCursor = len(m.editTags)
	}
}

// moveElement shifts the focused chip index by delta, bounded to [0, len] —
// index len being the + add slot. A no-op on Name, which has no elements.
func (m *Model) moveElement(delta int) {
	switch m.editFocus {
	case editFieldAliases:
		m.editAliasCursor = clampInt(m.editAliasCursor+delta, 0, len(m.editAliases))
	case editFieldTags:
		m.editTagCursor = clampInt(m.editTagCursor+delta, 0, len(m.editTags))
	}
}

// focusedChips reports ok only when a chip field (not Name) is focused.
func (m Model) focusedChips() (chips []string, idx int, ok bool) {
	switch m.editFocus {
	case editFieldAliases:
		return m.editAliases, m.editAliasCursor, true
	case editFieldTags:
		return m.editTags, m.editTagCursor, true
	default:
		return nil, 0, false
	}
}

func (m Model) focusedOnChip() bool {
	chips, idx, ok := m.focusedChips()
	return ok && idx < len(chips)
}

func (m Model) focusedOnAddSlot() bool {
	chips, idx, ok := m.focusedChips()
	return ok && idx == len(chips)
}

// deleteFocusedChip removes the focused chip and persists the delete
// immediately; a no-op on the + add slot or Name.
func (m Model) deleteFocusedChip() (tea.Model, tea.Cmd) {
	if !m.focusedOnChip() {
		return m, nil
	}
	switch m.editFocus {
	case editFieldAliases:
		removed := m.editAliases[m.editAliasCursor]
		if m.aliasEditor != nil {
			if _, err := m.aliasEditor.DeleteAndSave(removed, "cli"); err != nil {
				return m, nil
			}
		}
		m.editAliases = slices.Delete(m.editAliases, m.editAliasCursor, m.editAliasCursor+1)
		m.editAliasCursor = clampInt(m.editAliasCursor, 0, len(m.editAliases))
	case editFieldTags:
		removed := m.editTags[m.editTagCursor]
		if m.projectEditor != nil {
			if err := m.projectEditor.RemoveTag(m.editProject.Path, removed); err != nil {
				return m, nil
			}
		}
		m.editTags = slices.Delete(m.editTags, m.editTagCursor, m.editTagCursor+1)
		m.editTagCursor = clampInt(m.editTagCursor, 0, len(m.editTags))
	}
	m.editChanged = true
	return m, nil
}

// enterEditFromNavigate seeds the buffer from the focused element's value; on
// the + add slot it spawns a brand-new empty chip.
func (m Model) enterEditFromNavigate() (tea.Model, tea.Cmd) {
	m.editMode = editModeEdit
	switch m.editFocus {
	case editFieldName:
		m.editBuffer = m.editName
		m.editIsNewChip = false
	default:
		chips, idx, _ := m.focusedChips()
		if idx < len(chips) {
			m.editBuffer = chips[idx]
			m.editIsNewChip = false
		} else {
			m.editBuffer = ""
			m.editIsNewChip = true
		}
	}
	m.editCursor = len([]rune(m.editBuffer))
	return m, nil
}

// closeEditModal never discards — all work is already persisted; it only
// refreshes the cached records when something changed this session.
func (m Model) closeEditModal() (tea.Model, tea.Cmd) {
	m.modal = modalNone
	if m.editChanged {
		m.editChanged = false
		return m, m.loadProjects()
	}
	return m, nil
}

func (m *Model) insertEditRunes(text string) {
	runes := []rune(m.editBuffer)
	pos := clampInt(m.editCursor, 0, len(runes))
	inserted := []rune(text)
	out := make([]rune, 0, len(runes)+len(inserted))
	out = append(out, runes[:pos]...)
	out = append(out, inserted...)
	out = append(out, runes[pos:]...)
	m.editBuffer = string(out)
	m.editCursor = pos + len(inserted)
}

func (m *Model) backspaceEditBuffer() {
	if m.editCursor == 0 {
		return
	}
	runes := []rune(m.editBuffer)
	pos := clampInt(m.editCursor, 0, len(runes))
	m.editBuffer = string(runes[:pos-1]) + string(runes[pos:])
	m.editCursor = pos - 1
}

// discardEdit backs out the in-progress edit: a brand-new chip vanishes, an
// existing chip or Name keeps its prior value. Nothing persists.
func (m Model) discardEdit() (tea.Model, tea.Cmd) {
	m.editMode = editModeNavigate
	m.resetEditBuffer()
	// No slice mutation is needed either way — the buffer was never written
	// through — but the index must be re-clamped back into bounds.
	switch m.editFocus {
	case editFieldAliases:
		m.editAliasCursor = clampInt(m.editAliasCursor, 0, len(m.editAliases))
	case editFieldTags:
		m.editTagCursor = clampInt(m.editTagCursor, 0, len(m.editTags))
	}
	return m, nil
}

// commitEdit persists the live element and returns to navigate mode: empty
// commits delete (an empty Name reverts), duplicates dedupe silently, and a
// cross-project alias collision reverts silently. It always exits edit mode —
// there is no blocking error modal.
func (m Model) commitEdit() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.editBuffer)
	field := m.editFocus
	m.editMode = editModeNavigate

	switch field {
	case editFieldName:
		m.commitName(value)
	case editFieldAliases:
		m.commitAlias(value)
	case editFieldTags:
		m.commitTag(value)
	}

	m.resetEditBuffer()
	return m, nil
}

// commitName persists a Name change; an empty Name reverts to the prior value
// with no Rename call and no error.
func (m *Model) commitName(value string) {
	if value == "" {
		return
	}
	if value == m.editName {
		return
	}
	if m.projectEditor != nil {
		if err := m.projectEditor.Rename(m.editProject.Path, value, "cli"); err != nil {
			return
		}
	}
	m.editName = value
	m.editChanged = true
}

// commitAlias persists an alias chip edit: empty deletes the chip, a
// duplicate dedupes silently, and an alias mapping to a different project is
// a silent revert.
func (m *Model) commitAlias(value string) {
	idx := m.editAliasCursor
	existing := idx < len(m.editAliases)

	if value == "" {
		m.deleteAliasAt(idx)
		return
	}

	for i, a := range m.editAliases {
		if a == value && i != idx {
			if existing {
				// Renamed onto a duplicate: drop the edited chip, leave the
				// original.
				m.deleteAliasAt(idx)
			}
			return
		}
	}

	if m.aliasEditor != nil {
		if all, err := m.aliasEditor.Load(); err == nil {
			if path, ok := all[value]; ok && path != m.editProject.Path {
				if !existing {
					m.dropNewChip()
				}
				return
			}
		}
		if err := m.aliasEditor.SetAndSave(value, m.editProject.Path, "cli"); err != nil {
			if !existing {
				m.dropNewChip()
			}
			return
		}
	}

	if existing {
		old := m.editAliases[idx]
		if old != value && m.aliasEditor != nil {
			_, _ = m.aliasEditor.DeleteAndSave(old, "cli")
		}
		m.editAliases[idx] = value
	} else {
		m.editAliases = append(m.editAliases, value)
		m.editAliasCursor = len(m.editAliases)
	}
	m.editChanged = true
}

// commitTag persists a tag chip edit: empty deletes, and a duplicate dedupes
// silently (case-sensitive).
func (m *Model) commitTag(value string) {
	idx := m.editTagCursor
	existing := idx < len(m.editTags)

	if value == "" {
		m.deleteTagAt(idx)
		return
	}

	tag, ok := project.NormaliseTag(value)
	if !ok {
		if existing {
			m.deleteTagAt(idx)
		} else {
			m.dropNewChip()
		}
		return
	}

	for i, existingTag := range m.editTags {
		if existingTag == tag && i != idx {
			if existing {
				m.deleteTagAt(idx)
			} else {
				m.dropNewChip()
			}
			return
		}
	}

	if existing {
		old := m.editTags[idx]
		if old != tag && m.projectEditor != nil {
			_ = m.projectEditor.RemoveTag(m.editProject.Path, old)
		}
		if m.projectEditor != nil {
			if err := m.projectEditor.AddTag(m.editProject.Path, tag); err != nil {
				return
			}
		}
		m.editTags[idx] = tag
	} else {
		if m.projectEditor != nil {
			if err := m.projectEditor.AddTag(m.editProject.Path, tag); err != nil {
				m.dropNewChip()
				return
			}
		}
		m.editTags = append(m.editTags, tag)
		m.editTagCursor = len(m.editTags)
	}
	m.editChanged = true
}

// deleteAliasAt persists the delete when idx addresses an existing chip; an
// out-of-range index (a just-spawned new chip) simply vanishes.
func (m *Model) deleteAliasAt(idx int) {
	if idx < len(m.editAliases) {
		removed := m.editAliases[idx]
		if m.aliasEditor != nil {
			_, _ = m.aliasEditor.DeleteAndSave(removed, "cli")
		}
		m.editAliases = slices.Delete(m.editAliases, idx, idx+1)
		m.editChanged = true
	}
	m.editAliasCursor = len(m.editAliases)
}

func (m *Model) deleteTagAt(idx int) {
	if idx < len(m.editTags) {
		removed := m.editTags[idx]
		if m.projectEditor != nil {
			_ = m.projectEditor.RemoveTag(m.editProject.Path, removed)
		}
		m.editTags = slices.Delete(m.editTags, idx, idx+1)
		m.editChanged = true
	}
	m.editTagCursor = len(m.editTags)
}

// dropNewChip re-clamps the element index after a brand-new chip fails to
// commit and so never enters the slice.
func (m *Model) dropNewChip() {
	switch m.editFocus {
	case editFieldAliases:
		m.editAliasCursor = len(m.editAliases)
	case editFieldTags:
		m.editTagCursor = len(m.editTags)
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m Model) selectedSessionItem() (SessionItem, bool) {
	item := m.sessionList.SelectedItem()
	if item == nil {
		return SessionItem{}, false
	}
	si, ok := item.(SessionItem)
	return si, ok
}

func (m Model) updateSessionList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.modal != modalNone {
		return m.updateModal(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Burst input-lock: while a burst is in flight the picker is inert to
		// row actions so nothing races the completion handler's selection
		// mutation. Only cancellation stays live. Must precede the
		// flash-clear, the SettingFilter guard, and the rune switch.
		if m.burstPending {
			if keyIsCtrlC(msg) || keyIsCode(msg, tea.KeyEscape) {
				return m.cancelBurst()
			}
			return m, nil
		}
		// Abort-banner dismissal, staying in multi-select mode. Esc dismisses
		// the banner without falling through to the mode-exit branch, so a
		// second Esc exits the mode; every other actionable key clears the
		// banner and continues to its handler.
		if m.abortBannerText != "" && isActionableKey(msg) {
			dismissEsc := keyIsCode(msg, tea.KeyEscape)
			(&m).clearAbortBanner()
			if dismissEsc {
				return m, nil
			}
		}
		// One key, one intent: clear the flash as a side effect and fall
		// through to the keystroke's normal handler.
		if m.flashText != "" && isActionableKey(msg) {
			m.clearFlash()
		}
		if keyIsCtrlC(msg) {
			return m, tea.Quit
		}
		// While filtering, every key is a literal filter character — the rune
		// cases below all depend on this guard.
		if m.sessionList.SettingFilter() {
			break
		}
		// Opening our modal also consumes the key, so bubbles/list never
		// toggles its own help.
		if isRuneKey(msg, "?") {
			m.modal = modalHelp
			return m, nil
		}
		// Bound before the rest of the keymap so it never collides with the
		// rune handlers.
		if keyIsCode(msg, tea.KeySpace) {
			if len(m.sessionList.Items()) == 0 {
				return m, nil
			}
			si, ok := m.selectedSessionItem()
			if !ok {
				return m, nil
			}
			pmodel, ok := NewPreviewModel(si.Session.Name, m.enumerator, m.reader, m.previewAttacher, m.contentWidth(), m.contentHeight())
			if !ok {
				return m, nil
			}
			pmodel.th = m.themeState.active
			pmodel.colourless = m.colourless
			m.preview = pmodel
			m.activePage = pagePreview
			return m, nil
		}
		switch {
		case keyIsCode(msg, tea.KeyEscape):
			// Checked before the progressive-back filter branch: exiting the
			// mode outranks clearing the filter.
			if m.multiSelectMode {
				return m.exitMultiSelect(), nil
			}
			if m.sessionList.FilterState() == list.FilterApplied {
				break
			}
			return m, tea.Quit
		case isRuneKey(msg, "q"):
			return m, tea.Quit
		// Multi-select suppresses the row actions k/r/n/x — none compose with
		// a marked set, and n would quit the picker, silently discarding it.
		// The arms stay present rather than deleted so default-mode dispatch
		// is unchanged.
		case isRuneKey(msg, "k"):
			if m.multiSelectMode {
				return m, nil
			}
			return m.handleKillKey()
		case isRuneKey(msg, "r"):
			if m.multiSelectMode {
				return m, nil
			}
			return m.handleRenameKey()
		case isRuneKey(msg, "n"):
			if m.multiSelectMode {
				return m, nil
			}
			return m.handleNewInCWD()
		case isRuneKey(msg, "s"):
			return m.handleSwitchViewKey()
		case isRuneKey(msg, "m"):
			return m.handleMultiSelectToggle()
		// Deliberately not suppressed in multi-select: the panel nests over
		// the mode with the marked set unaffected.
		case isRuneKey(msg, "t"):
			return m.handleThemePanelKey()
		case isRuneKey(msg, "x"):
			if m.multiSelectMode {
				return m, nil
			}
			m.activePage = PageProjects
			return m, nil
		case keyIsCode(msg, tea.KeyEnter):
			if m.multiSelectMode {
				return m.handleMultiSelectEnter()
			}
			return m.handleSessionListEnter()
		}
	}

	var cmd tea.Cmd
	m.sessionList, cmd = m.sessionList.Update(msg)
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		m.skipHeaderRow(keyMsg)
	}
	return m, cmd
}

// handleSwitchViewKey advances the grouping mode one step, re-renders, and
// persists the new mode. The cycle is unconditional — it fires regardless of
// session or tag count. Persistence is best-effort: remembering the last mode
// is a convenience, so a write failure must not abort the toggle.
func (m Model) handleSwitchViewKey() (tea.Model, tea.Cmd) {
	m.sessionListMode = nextSessionListMode(m.sessionListMode)
	cmd := (&m).rebuildSessionList()
	// The previous page offset and cursor are meaningless under the new
	// grouping.
	m.sessionList.ResetSelected()
	(&m).ensureSessionRowSelected()
	if m.modePersister != nil {
		_ = m.modePersister.Save(m.sessionListMode)
	}
	return m, cmd
}

// Blocked-entry flash copy for the proactive multi-select block. No ⚠ glyph
// (the band prepends it) and no "nothing opened" suffix — a pre-emptive block
// attempts nothing. It names neither the identity nor "see docs": on a named
// terminal it co-renders with the banner, which supplies both.
const (
	multiSelectBlockedRemoteFlash = "multi-select isn't available over a remote connection"
	multiSelectBlockedNamedFlash  = "multi-select isn't available on this terminal"
)

func multiSelectBlockedFlashText(id spawn.Identity) string {
	if id.IsNull() {
		return multiSelectBlockedRemoteFlash
	}
	return multiSelectBlockedNamedFlash
}

// handleMultiSelectToggle drives the `m` key: the first press enters the mode
// marking the highlighted session, and each later press toggles the
// highlighted row. Entry or a toggle on a header row (or an empty list) marks
// nothing rather than being special-cased.
func (m Model) handleMultiSelectToggle() (tea.Model, tea.Cmd) {
	if !m.multiSelectMode {
		// Proactive block: on a resolved-unsupported terminal `m` must not
		// open a walkable dead-end mode whose Enter can never fire a burst.
		// Detection unwired reports false here, so `m` still enters the mode.
		if m.DetectUnsupported() {
			(&m).setFlash(multiSelectBlockedFlashText(m.detectIdentity))
			return m, flashTickCmd(m.flashGen)
		}
		m.multiSelectMode = true
		m.selectedSessions = map[string]struct{}{}
		if si, ok := m.selectedSessionItem(); ok {
			m.selectedSessions[si.Session.Name] = struct{}{}
		}
		(&m).refreshSessionDelegate()
		return m, nil
	}
	si, ok := m.selectedSessionItem()
	if !ok {
		return m, nil
	}
	if m.selectedSessions == nil {
		m.selectedSessions = map[string]struct{}{}
	}
	name := si.Session.Name
	if _, marked := m.selectedSessions[name]; marked {
		delete(m.selectedSessions, name)
	} else {
		m.selectedSessions[name] = struct{}{}
	}
	(&m).refreshSessionDelegate()
	return m, nil
}

// exitMultiSelect leaves the mode and clears the whole marked set.
func (m Model) exitMultiSelect() Model {
	m.multiSelectMode = false
	m.selectedSessions = nil
	(&m).refreshSessionDelegate()
	return m
}

// handleMultiSelectEnter commits the marked set; the cursor is irrelevant.
// N=0 exits without opening anything (not a quit), N=1 degenerates to a plain
// single attach, and N≥2 is the spawn-burst boundary.
func (m Model) handleMultiSelectEnter() (tea.Model, tea.Cmd) {
	switch len(m.selectedSessions) {
	case 0:
		m = m.exitMultiSelect()
		return m, nil
	case 1:
		var name string
		// len == 1, so this ranges once, binding the sole key.
		for name = range m.selectedSessions {
		}
		m.selected = name
		return m, tea.Quit
	default:
		return m.beginBurst(m.orderedMarkedSessions())
	}
}

func (m Model) handleKillKey() (tea.Model, tea.Cmd) {
	si, ok := m.selectedSessionItem()
	if !ok {
		return m, nil
	}
	if m.sessionKiller == nil {
		return m, nil
	}
	m.modal = modalKillConfirm
	m.pendingKillName = si.Session.Name
	m.pendingKillWindows = si.Session.Windows
	return m, nil
}

func (m Model) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyIsCtrlC(keyMsg) {
		return m, tea.Quit
	}

	switch m.modal {
	case modalKillConfirm:
		return m.updateKillConfirmModal(msg)
	case modalRename:
		return m.updateRenameModal(msg)
	case modalDeleteProject:
		return m.updateDeleteProjectModal(msg)
	case modalEditProject:
		return m.updateEditProjectModal(msg)
	case modalHelp:
		return m.updateHelpModal(msg)
	default:
		return m, nil
	}
}

// updateHelpModal is key-exclusive: ? and Esc dismiss, every other key is
// consumed. Esc must not fall through to the page's clear-filter/quit — help
// opened over an applied filter dismisses the help only.
func (m Model) updateHelpModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if isRuneKey(keyMsg, "?") || keyIsCode(keyMsg, tea.KeyEscape) {
		m.modal = modalNone
		return m, nil
	}
	return m, nil
}

func (m Model) updateKillConfirmModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch {
	case isRuneKey(keyMsg, "y"):
		name := m.pendingKillName
		m.modal = modalNone
		m.pendingKillName = ""
		m.pendingKillWindows = 0
		return m, m.killAndRefresh(name)
	case keyIsCode(keyMsg, tea.KeyEscape):
		// Cancel is Esc only; `n` falls through to the ignore-all tail.
		m.modal = modalNone
		m.pendingKillName = ""
		m.pendingKillWindows = 0
		return m, nil
	}
	return m, nil
}

func (m Model) killAndRefresh(name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.sessionKiller.KillSession(name); err != nil {
			return SessionsMsg{Err: fmt.Errorf("failed to kill session '%s': %w", name, err)}
		}
		sessions, err := m.sessionLister.ListSessions()
		return SessionsMsg{Sessions: sessions, Err: err}
	}
}

func (m Model) handleRenameKey() (tea.Model, tea.Cmd) {
	si, ok := m.selectedSessionItem()
	if !ok {
		return m, nil
	}
	if m.sessionRenamer == nil {
		return m, nil
	}
	m.modal = modalRename
	m.renameTarget = si.Session.Name
	ti := textinput.New()
	// No inline prompt: the modal renders its own labelled input box, so a
	// prompt here would double up inside it.
	ti.Prompt = ""
	ti.SetValue(m.renameTarget)
	ti.Focus()
	m.renameInput = ti
	return m, nil
}

func (m Model) updateRenameModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch keyMsg.Code {
		case tea.KeyEnter:
			newName := strings.TrimSpace(m.renameInput.Value())
			if newName == "" {
				return m, nil
			}
			oldName := m.renameTarget
			m.modal = modalNone
			m.renameTarget = ""
			return m, m.renameAndRefresh(oldName, newName)
		case tea.KeyEscape:
			m.modal = modalNone
			m.renameTarget = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m Model) renameAndRefresh(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		if err := m.sessionRenamer.RenameSession(oldName, newName); err != nil {
			return SessionsMsg{Err: fmt.Errorf("failed to rename session '%s': %w", oldName, err)}
		}
		sessions, err := m.sessionLister.ListSessions()
		return SessionsMsg{Sessions: sessions, Err: err}
	}
}

func (m Model) handleNewInCWD() (tea.Model, tea.Cmd) {
	if m.sessionCreator == nil {
		return m, nil
	}
	return m, m.createSessionInCWD()
}

func (m Model) createSessionInCWD() tea.Cmd {
	return m.createSession(m.cwd)
}

func (m Model) handleSessionListEnter() (tea.Model, tea.Cmd) {
	si, ok := m.selectedSessionItem()
	if !ok {
		return m, nil
	}
	m.selected = si.Session.Name
	return m, tea.Quit
}

// View renders the current frame. Alt-screen is set declaratively here so
// every page composer stays a plain string builder.
func (m Model) View() tea.View {
	// First-paint gate: under an adaptive pair, hold the real paint until
	// the gate resolves, since painting first would risk a visible flip of
	// the whole theme. The wait is tens of ms, invisible against the
	// multi-hundred-ms bootstrap. It holds every owned-canvas surface,
	// including the cold-path loading page.
	if !m.modeResolved() {
		v := tea.NewView(m.blankFrame())
		v.AltScreen = true
		return v
	}
	v := tea.NewView(m.fillCanvas(m.viewString()))
	v.AltScreen = true
	// Under NO_COLOR the terminal keeps its native bg: leaving
	// BackgroundColor nil completes the canvas suppression.
	if m.colourless {
		return v
	}
	// Belt-and-braces gutter fill only — it paints the terminal's padding
	// outside the rendered grid on terminals that honour OSC 11. In-grid
	// correctness does not depend on it; fillCanvas paints every interior
	// cell with an explicit SGR.
	v.BackgroundColor = m.themeState.active.Canvas.Color()
	return v
}

// blankFrame is the neutral pre-resolution frame held by the first-paint
// gate. It deliberately paints no canvas background — the mode is undecided,
// so committing to either canvas here is the flip the gate exists to avoid.
func (m Model) blankFrame() string {
	w, h := m.termDims()
	blank := strings.Repeat(" ", w)
	lines := make([]string, h)
	for i := range lines {
		lines[i] = blank
	}
	return strings.Join(lines, "\n")
}

// Hinset and Vinset are the global content gutter, in cells and rows. It is
// folded into the width and height budgets at every SetSize site — part of the
// budget, not an uncounted band — so the pagination invariant stays exact.
const (
	Hinset = 2
	Vinset = 1
)

// Pre-first-WindowSizeMsg dimensions, so the frame never sizes to zero and
// blanks the screen.
const (
	fallbackTermWidth  = 80
	fallbackTermHeight = 24
)

// termDims returns the cached terminal dimensions with the zero/unset
// fallback applied.
func (m Model) termDims() (w, h int) {
	w, h = m.termWidth, m.termHeight
	if w <= 0 {
		w = fallbackTermWidth
	}
	if h <= 0 {
		h = fallbackTermHeight
	}
	return w, h
}

// insetRegion reduces a dimension by 2× the inset, clamping the inset to 0
// when the dimension cannot hold it, so a tiny terminal degrades rather than
// producing a negative region.
func insetRegion(dim, inset int) int {
	if dim <= 2*inset {
		return dim
	}
	return dim - 2*inset
}

// contentWidth is the inset content-region width every page composes into.
func (m Model) contentWidth() int {
	w, _ := m.termDims()
	return insetRegion(w, Hinset)
}

// insetWindowSizeMsg rewrites a resize to the inset content-region size so a
// sub-model that frames its own view sits inside the global gutter. Any other
// message passes through unchanged.
func insetWindowSizeMsg(msg tea.Msg, contentW, contentH int) tea.Msg {
	wsm, ok := msg.(tea.WindowSizeMsg)
	if !ok {
		return msg
	}
	wsm.Width = contentW
	wsm.Height = contentH
	return wsm
}

// contentHeight is the inset content-region height every page composes into.
// The vertical inset is subtracted in addition to what the list budget
// already reserves.
func (m Model) contentHeight() int {
	_, h := m.termDims()
	return insetRegion(h, Vinset)
}

// fillCanvas is the outer full-terminal canvas fill — the last layer over the
// composed page, bar the theme slide-over, which is composited on the way out
// so the fill never paints over a panel cell.
//
// Each line is padded individually rather than via lipgloss.Place, which pads
// only beyond the single widest line and so would suppress padding on every
// shorter one. Content taller than the region is clamped.
func (m Model) fillCanvas(view string) string {
	w, h := m.termDims()
	contentW := m.contentWidth()
	contentH := m.contentHeight()
	// NO_COLOR: keep the layout, but every padding/gutter cell is a plain
	// space with no background SGR.
	if m.colourless {
		content := m.overlayThemePanelOnContent(fillColourless(view, contentW, contentH), contentW, contentH)
		return insetColourless(content, w, h, contentW, contentH)
	}
	canvas := lipgloss.NewStyle().Background(m.themeState.active.Canvas.Color())
	canvasBg := canvasBgParams(m.themeState.active.Canvas.Color())
	parser := ansi.NewParser() // one instance reused across every line this frame

	lines := strings.Split(view, "\n")
	out := make([]string, 0, contentH)
	for _, line := range lines {
		if len(out) == contentH {
			break
		}
		// Backfill mid-line gaps before padding, so every interior cell
		// carries the canvas bg without depending on OSC 11.
		line = backfillCanvasBackground(line, canvasBg, parser)
		out = append(out, padLineToCanvasWidth(line, contentW, canvas))
	}
	blank := canvas.Render(strings.Repeat(" ", contentW))
	for len(out) < contentH {
		out = append(out, blank)
	}
	content := m.overlayThemePanelOnContent(strings.Join(out, "\n"), contentW, contentH)
	return insetCanvasCanvas(strings.Split(content, "\n"), w, h, contentW, canvas)
}

// overlayThemePanelOnContent composites the slide-over over the filled
// content region. The position is load-bearing: after the fill, because the
// panel is opaque and paints its own cells, so the fill's backfill and pad
// must not process them; before the gutter inset, because the panel's right
// edge is the content region's, not the terminal's. A no-op while closed.
func (m Model) overlayThemePanelOnContent(content string, contentW, contentH int) string {
	if !m.themePanel.open {
		return content
	}
	panel := renderThemePanel(m.themePanel, contentH, m.themeState.active, m.colourless)
	return overlayThemePanel(content, panel, contentW)
}

// gutterPadding splits the gutter evenly across both sides — shared by both
// inset placers so the coloured and colourless frames match in layout.
func gutterPadding(w, h, contentW, contentH int) (leftPad, rightPad, topPad, botPad int) {
	hGutter := w - contentW
	leftPad = hGutter / 2
	rightPad = hGutter - leftPad
	vGutter := h - contentH
	topPad = vGutter / 2
	botPad = vGutter - topPad
	return leftPad, rightPad, topPad, botPad
}

// insetCanvasCanvas places the content rows inside the full terminal canvas
// for the coloured path, every gutter cell painted the owned canvas.
func insetCanvasCanvas(contentRows []string, w, h, contentW int, canvas lipgloss.Style) string {
	leftPad, rightPad, topPad, botPad := gutterPadding(w, h, contentW, len(contentRows))

	fullBlank := canvas.Render(strings.Repeat(" ", w))
	leftCol := ""
	if leftPad > 0 {
		leftCol = canvas.Render(strings.Repeat(" ", leftPad))
	}
	rightCol := ""
	if rightPad > 0 {
		rightCol = canvas.Render(strings.Repeat(" ", rightPad))
	}

	out := make([]string, 0, h)
	for range topPad {
		out = append(out, fullBlank)
	}
	for _, row := range contentRows {
		out = append(out, leftCol+row+rightCol)
	}
	for range botPad {
		out = append(out, fullBlank)
	}
	return strings.Join(out, "\n")
}

// fillColourless is the NO_COLOR variant of fillCanvas's per-line fill: same
// line geometry, but every padding cell is a plain space with no background
// SGR and there is no mid-line backfill.
func fillColourless(view string, w, h int) string {
	blank := strings.Repeat(" ", w)
	lines := strings.Split(view, "\n")
	out := make([]string, 0, h)
	for _, line := range lines {
		if len(out) == h {
			break
		}
		line = strings.TrimRight(line, " ")
		gap := w - lipgloss.Width(line)
		if gap > 0 {
			line += strings.Repeat(" ", gap)
		}
		out = append(out, line)
	}
	for len(out) < h {
		out = append(out, blank)
	}
	return strings.Join(out, "\n")
}

// insetColourless is the NO_COLOR counterpart of insetCanvasCanvas: the same
// geometry with plain-space gutters and no background SGR.
func insetColourless(content string, w, h, contentW, contentH int) string {
	leftPad, rightPad, topPad, botPad := gutterPadding(w, h, contentW, contentH)

	fullBlank := strings.Repeat(" ", w)
	leftCol := strings.Repeat(" ", leftPad)
	rightCol := strings.Repeat(" ", rightPad)

	out := make([]string, 0, h)
	for range topPad {
		out = append(out, fullBlank)
	}
	for row := range strings.SplitSeq(content, "\n") {
		out = append(out, leftCol+row+rightCol)
	}
	for range botPad {
		out = append(out, fullBlank)
	}
	return strings.Join(out, "\n")
}

// backfillCanvasBackground folds the canvas background into every SGR that
// resets the background to default, so bare spacer cells emitted by
// sub-renderers cannot bleed the terminal's own theme through on terminals
// that ignore OSC 11. SGRs setting an explicit background are left untouched.
//
// The parser is passed in so the per-line loop reuses one instance; sharing is
// safe because this never reads parser.Params.
func backfillCanvasBackground(line, canvasBg string, parser *ansi.Parser) string {
	if canvasBg == "" {
		return line
	}
	canvasSetParams := strings.Split(canvasBg, ";")

	var b strings.Builder
	b.Grow(len(line) + len(canvasBg) + 8)

	src := []byte(line)
	state := byte(0)
	bgActive := false

	// Content arrives grapheme-by-grapheme, so buffer a contiguous run and
	// wrap it once rather than per cell — that keeps the rendered text
	// searchable and the SGR overhead minimal.
	var run []byte
	flushRun := func() {
		if len(run) == 0 {
			return
		}
		if bgActive {
			b.Write(run)
		} else {
			// Close with a reset so no state leaks past the run.
			b.WriteString("\x1b[")
			b.WriteString(canvasBg)
			b.WriteByte('m')
			b.Write(run)
			b.WriteString("\x1b[m")
		}
		run = run[:0]
	}

	for len(src) > 0 {
		seq, width, n, newState := ansi.DecodeSequence(src, state, parser)
		if n == 0 {
			break
		}
		isSGR := ansi.HasCsiPrefix(seq) && seq[len(seq)-1] == 'm'
		switch {
		case isSGR:
			flushRun()
			params := sgrParamsList(string(seq))
			bgActive = sgrBackgroundActive(bgActive, params)
			if !bgActive {
				// The sequence dropped the bg to terminal-default; fold the
				// canvas params into it so the following cells stay painted.
				b.WriteString(rewriteSGRWithCanvasBg(params, canvasSetParams))
				bgActive = true
			} else {
				b.Write(seq)
			}
		case width > 0:
			run = append(run, seq...)
		default:
			// Non-SGR control / escape — pass through verbatim.
			flushRun()
			b.Write(seq)
		}
		src = src[n:]
		state = newState
	}
	flushRun()
	return b.String()
}

// rewriteSGRWithCanvasBg appends the canvas background parameters to an SGR
// sequence, carrying its foreground and attribute codes through unchanged.
func rewriteSGRWithCanvasBg(originalParams, canvasParams []string) string {
	merged := make([]string, 0, len(originalParams)+len(canvasParams))
	for _, p := range originalParams {
		// A bare reset arrives as one empty param; substitute an explicit "0"
		// so the rewritten sequence has no leading ";".
		if p == "" {
			merged = append(merged, "0")
			continue
		}
		merged = append(merged, p)
	}
	merged = append(merged, canvasParams...)
	return "\x1b[" + strings.Join(merged, ";") + "m"
}

// sgrParamsList splits the ";"-separated parameters from a CSI ...m sequence.
// A bare "\x1b[m" yields a single empty element, which callers treat as a full
// reset.
func sgrParamsList(seq string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	if inner == "" {
		return []string{""}
	}
	return strings.Split(inner, ";")
}

// sgrBackgroundActive folds an SGR sequence's parameters into the running "is
// an explicit background active?" flag: 0/empty and 49 clear it, 48;… and the
// named bg codes set it, everything else leaves it. Extended-colour runs are
// consumed whole so a colour channel value that happens to equal a bg code can
// never be misread as a background change.
func sgrBackgroundActive(active bool, params []string) bool {
	for i := 0; i < len(params); i++ {
		switch p := params[i]; {
		case p == "" || p == "0" || p == "49":
			active = false
		case p == "48":
			active = true
			i = consumeExtendedColorRun(params, i)
		case p == "38":
			i = consumeExtendedColorRun(params, i)
		case isNamedBackground(p):
			active = true
		}
	}
	return active
}

// consumeExtendedColorRun returns the index of the last channel parameter of
// an extended-colour SGR run starting at i (38/48 followed by 2;r;g;b or 5;n).
// An over-run index needs no clamp — the caller's loop re-checks the bound.
func consumeExtendedColorRun(params []string, i int) int {
	if i+1 >= len(params) {
		return i
	}
	switch params[i+1] {
	case "2":
		return i + 4
	case "5":
		return i + 2
	}
	return i
}

func isNamedBackground(p string) bool {
	switch p {
	case "40", "41", "42", "43", "44", "45", "46", "47",
		"100", "101", "102", "103", "104", "105", "106", "107":
		return true
	}
	return false
}

// canvasBgParams returns the raw background-parameter form the given colour
// renders as, derived through lipgloss so the backfill folds the same bytes
// the leaf styles paint. An empty result disables the backfill.
func canvasBgParams(c color.Color) string {
	probe := lipgloss.NewStyle().Background(c).Render(" ")
	idx := strings.IndexByte(probe, 'm')
	if idx <= 0 || !strings.HasPrefix(probe, "\x1b[") {
		return ""
	}
	return probe[len("\x1b["):idx]
}

// padLineToCanvasWidth right-pads a line to width with canvas-background
// whitespace, stripping bubbles/list's trailing block-padding first. An
// over-width line is never truncated here — horizontal overflow is handled by
// name truncation.
func padLineToCanvasWidth(line string, width int, canvas lipgloss.Style) string {
	line = strings.TrimRight(line, " ")
	gap := width - lipgloss.Width(line)
	if gap <= 0 {
		return line
	}
	return line + canvas.Render(strings.Repeat(" ", gap))
}

// viewString renders the current page to a plain string; the tea.View
// wrapping lives in View.
func (m Model) viewString() string {
	switch m.activePage {
	case PageLoading:
		return m.viewLoading()
	case PageProjects:
		return m.viewProjectList()
	case pagePreview:
		return m.preview.View()
	default:
		return m.viewSessionList()
	}
}

// viewLoading renders the loading interstitial from the live progress
// accumulator, composed into the inset content region.
func (m Model) viewLoading() string {
	view := m.loadingProgress.View()
	if m.fatalActive {
		// No page transition — the model stays on PageLoading awaiting q/Esc.
		view = m.loadingProgress.FailedView(m.fatalStep, m.fatalMessage)
	}
	return renderLoadingScreen(
		view,
		m.contentWidth(),
		m.contentHeight(),
		m.themeState.active,
		m.colourless,
	)
}

// viewProjectList renders the Projects page. With a modal open the page
// chrome is not composed at all: only the centred panel is returned, and the
// outer fill paints the cleared backdrop.
func (m Model) viewProjectList() string {
	switch m.modal {
	case modalDeleteProject:
		return renderDeleteModalOnClearedCanvas(m.pendingDeleteName, m.pendingDeletePath, m.contentWidth(), m.contentHeight(), m.themeState.active, m.colourless)
	case modalEditProject:
		return renderEditModalOnClearedCanvas(m, m.contentWidth(), m.contentHeight(), m.themeState.active, m.colourless)
	case modalHelp:
		// Filtered through the same slice the footer renders from, so a
		// blocked key leaves both surfaces in lockstep.
		return renderHelpModalOnClearedCanvas(m.projectsHelpKeymap(), m.contentWidth(), m.contentHeight(), m.themeState.active, m.colourless)
	}
	listView := m.projectList.View()
	listView = m.applyProjectsSectionHeader(listView)
	// Suppressed while a command is pending: the user is being asked to pick
	// a project to run, so the empty-state guidance would be wrong.
	if m.projectListEmpty() && !m.commandPending {
		listView = m.replaceListBodyWithEmptyState(listView, m.projectList.Height(), emptyProjectsGlyph, emptyProjectsMessage, emptyProjectsHint)
	}
	footer := m.renderProjectsFooterForFilterState()
	header := m.renderHeader()
	if slot := m.renderProjectBandSlot(); slot != "" {
		return lipgloss.JoinVertical(lipgloss.Left, header, slot, listView, footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, listView, footer)
}

// renderProjectBandSlot is the single source of truth for what the Projects
// notice slot inserts — the arbitrated band plus one blank breathing row, or
// empty when no band owns the slot. Both the composition and the height
// reserve consume it, so the reserved rows and the rendered ones cannot drift.
func (m Model) renderProjectBandSlot() string {
	band := m.renderActiveProjectNoticeBand()
	if band == "" {
		return ""
	}
	blank := blankCanvasRow(m.contentWidth(), m.themeState.active, m.colourless)
	return lipgloss.JoinVertical(lipgloss.Left, band, blank)
}

// applyProjectsSectionHeader swaps the section header into the list's first
// line. Replacing the title row's content rather than inserting a row keeps
// the pagination invariant exact. While the filter input is active that line
// is the input itself, so it is left untouched.
func (m Model) applyProjectsSectionHeader(listView string) string {
	if m.projectList.FilterState() == list.Filtering {
		return listView
	}
	if m.projectList.FilterState() == list.FilterApplied {
		return replaceHeaderLine(listView, renderFilterQueryHeader(
			m.projectList.FilterValue(),
			m.contentWidth(),
			m.themeState.active,
			m.colourless,
		))
	}
	return replaceHeaderLine(listView, renderProjectsSectionHeader(
		m.visibleProjectRowCount(),
		m.contentWidth(),
		m.themeState.active,
		m.colourless,
	))
}

// visibleProjectRowCount counts visible rows, so an applied filter is
// reflected in the section header.
func (m Model) visibleProjectRowCount() int {
	return len(m.projectList.VisibleItems())
}

// renderProjectsFooterForFilterState resolves the footer variant for the
// current filter mode. Every variant is the same height, so the swap is
// height-neutral against the reserved budget.
func (m Model) renderProjectsFooterForFilterState() string {
	switch m.projectList.FilterState() {
	case list.Filtering:
		return renderFilteringFooter(m.contentWidth(), m.themeState.active, m.colourless)
	case list.FilterApplied:
		// Enter on Projects is "new session", not "attach" — do not leak the
		// Sessions filter-applied copy here.
		return renderProjectsFilterAppliedFooter(m.contentWidth(), m.themeState.active, m.colourless)
	default:
		if m.commandPending {
			return renderCommandPendingFooter(m.contentWidth(), m.themeState.active, m.colourless)
		}
		// Gated after command-pending so that footer wins.
		if m.projectListEmpty() {
			return renderEmptyProjectsFooter(m.contentWidth(), m.themeState.active, m.colourless)
		}
		return renderProjectsFooter(m.projectsHelpKeymap(), m.contentWidth(), m.themeState.active, m.colourless)
	}
}

// viewSessionList composes the Sessions page. With a modal open the page
// chrome is not composed at all: only the centred panel is returned, so no
// list rows or bands leak into the cleared view.
func (m Model) viewSessionList() string {
	switch m.modal {
	case modalKillConfirm:
		return renderKillModalOnClearedCanvas(m.pendingKillName, m.pendingKillWindows, m.contentWidth(), m.contentHeight(), m.themeState.active, m.colourless)
	case modalRename:
		return renderRenameModalOnClearedCanvas(m.renameInput, m.renameTarget, m.contentWidth(), m.contentHeight(), m.themeState.active, m.colourless)
	case modalHelp:
		// Filtered through the same slice the footer renders from, so a
		// blocked key leaves both surfaces in lockstep.
		return renderHelpModalOnClearedCanvas(m.sessionsHelpKeymap(), m.contentWidth(), m.contentHeight(), m.themeState.active, m.colourless)
	}
	listView := m.sessionList.View()
	listView = m.applySectionHeader(listView)
	// The no-matches and empty states are mutually exclusive — one requires
	// an active query, the other the Unfiltered state — and both replace the
	// body at its existing height, so the pagination invariant is unaffected.
	if m.sessionListNoMatches() {
		listView = m.replaceListBodyWithNoMatches(listView)
	}
	if m.sessionListEmpty() {
		listView = m.replaceListBodyWithEmptyState(listView, m.sessionList.Height(), emptySessionsGlyph, emptySessionsMessage, emptySessionsHint)
	}
	// Every footer variant is the same height, so the filter-mode swap is
	// height-neutral against the budget sessionFooterHeight reserved.
	footer := m.renderSessionsFooterForFilterState()
	header := m.renderHeader()
	// The band composes between the header and the list — not inside it —
	// which is what lands it above the section header, flush under the title
	// separator.
	if slot := m.renderSessionBandSlot(); slot != "" {
		return lipgloss.JoinVertical(lipgloss.Left, header, slot, listView, footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, listView, footer)
}

// renderSessionsFooterForFilterState resolves the footer variant for the
// current state. Every variant is the same height, so each swap is
// height-neutral against the reserved budget. The order is the precedence:
// no-matches, then empty, then the focused filter input, then multi-select
// (which yields to the focused input as an inner sub-state), then the plain
// filter-applied and standard footers.
func (m Model) renderSessionsFooterForFilterState() string {
	if m.sessionListNoMatches() {
		return renderNoMatchesFooter(m.contentWidth(), m.themeState.active, m.colourless)
	}
	if m.sessionListEmpty() {
		return renderEmptySessionsFooter(m.contentWidth(), m.themeState.active, m.colourless)
	}
	if m.sessionList.FilterState() == list.Filtering {
		return renderFilteringFooter(m.contentWidth(), m.themeState.active, m.colourless)
	}
	if m.multiSelectMode {
		return renderMultiSelectFooter(m.contentWidth(), m.themeState.active, m.colourless)
	}
	switch m.sessionList.FilterState() {
	case list.FilterApplied:
		return renderFilterAppliedFooter(m.contentWidth(), m.themeState.active, m.colourless)
	default:
		return renderSessionsFooter(m.sessionsHelpKeymap(), m.contentWidth(), m.themeState.active, m.colourless)
	}
}

// renderHeader is the single header entry point, so the composed view and the
// height budget resolve it against the same width and mode.
func (m Model) renderHeader() string {
	return renderHeaderBlock(m.contentWidth(), m.themeState.active, m.colourless)
}

// unsupportedBannerActive reports whether the unsupported-terminal banner owns
// the section-header row. The banner is named-only: the !IsNull() leg keeps a
// NULL/remote client from claiming the row, since its banner would carry
// nothing actionable. Both the header swap and the signpost suppression read
// this one predicate so they cannot drift.
func (m Model) unsupportedBannerActive() bool {
	return m.DetectUnsupported() && !m.multiSelectMode && !m.detectIdentity.IsNull()
}

// sessionsHelpKeymap returns the descriptor slice both Sessions display
// surfaces read, filtering the copy only so the footer cannot advertise a key
// that can only flash. A narrow or short terminal is deliberately not
// filtered: that is a space shortage the panel degrades for, not a capability
// absence.
func (m Model) sessionsHelpKeymap() []keymapEntry {
	entries := sessionsKeymap()
	if m.multiKeyBlocked() {
		entries = dropKeymapKey(entries, "m")
	}
	if m.themeKeyBlocked() {
		entries = dropKeymapKey(entries, "t")
	}
	return entries
}

// projectsHelpKeymap is the Projects counterpart. It filters `t` alone — `m`
// is a Sessions binding this page never lists.
func (m Model) projectsHelpKeymap() []keymapEntry {
	entries := projectsKeymap()
	if m.themeKeyBlocked() {
		entries = dropKeymapKey(entries, "t")
	}
	return entries
}

// multiKeyBlocked reports whether `m` would be a dead end: unsupported and not
// already in the mode. Detection resolving unsupported while the mode is open
// still leaves `m` a live row-toggle, hence the second leg.
func (m Model) multiKeyBlocked() bool {
	return m.DetectUnsupported() && !m.multiSelectMode
}

// themeKeyBlocked reports whether `t` is proactively blocked — under NO_COLOR
// the panel previews nothing and a commit would persist a choice with no
// visible feedback. Both pages' filters read this one predicate.
func (m Model) themeKeyBlocked() bool {
	return m.colourless
}

// dropKeymapKey returns entries with the given Key glyph removed, preserving
// order. Called only on a blocked path, so the unblocked one allocates nothing.
func dropKeymapKey(entries []keymapEntry, key string) []keymapEntry {
	filtered := make([]keymapEntry, 0, len(entries))
	for _, e := range entries {
		if e.Key == key {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

// replaceHeaderLine swaps header in for the first line of listView, keeping
// the tail from the first newline onward. Replacing the title row's content
// rather than inserting a row keeps the pagination invariant exact. A
// no-newline listView has no tail, so header is returned bare.
func replaceHeaderLine(listView, header string) string {
	idx := strings.IndexByte(listView, '\n')
	if idx < 0 {
		return header
	}
	return header + listView[idx:]
}

// applySectionHeader swaps the section header into the list's first line;
// replacing its content rather than inserting a row keeps the pagination
// invariant exact. While the filter input is active that line is the input
// itself, so it is left untouched. The branches below are in precedence
// order.
func (m Model) applySectionHeader(listView string) string {
	if m.sessionList.FilterState() == list.Filtering {
		return listView
	}
	if m.burstPending {
		return replaceHeaderLine(listView, renderOpeningBand(
			m.burstDone,
			m.burstTotal,
			m.contentWidth(),
			m.themeState.active,
			m.colourless,
		))
	}
	// The abort banner owns this row, unlike its spawn-failure siblings in the
	// same flash tier, which route through the notice band and deliberately
	// co-render with the multi-select banner rather than replacing it.
	if m.abortBannerText != "" {
		return replaceHeaderLine(listView, renderPreflightAbortHeader(
			m.abortBannerText,
			m.contentWidth(),
			m.themeState.active,
			m.colourless,
		))
	}
	// Precedes the FilterApplied branch, so a committed filter while in the
	// mode shows the mode banner rather than the locked query header.
	if m.multiSelectMode {
		return replaceHeaderLine(listView, renderMultiSelectHeader(
			len(m.selectedSessions),
			m.contentWidth(),
			m.themeState.active,
			m.colourless,
		))
	}
	// Named-only: the gate's !IsNull() leg means only the named shape reaches
	// the renderer, so its `see docs` hint is unconditional.
	if m.unsupportedBannerActive() {
		return replaceHeaderLine(listView, renderUnsupportedHeader(
			m.detectIdentity.Name,
			m.detectIdentity.BundleID,
			m.contentWidth(),
			m.themeState.active,
			m.colourless,
		))
	}
	if m.sessionList.FilterState() == list.FilterApplied {
		return replaceHeaderLine(listView, renderFilterQueryHeader(
			m.sessionList.FilterValue(),
			m.contentWidth(),
			m.themeState.active,
			m.colourless,
		))
	}
	return replaceHeaderLine(listView, renderSectionHeader(
		m.sessionListMode,
		m.insideTmux,
		m.currentSession,
		m.visibleSessionRowCount(),
		m.contentWidth(),
		m.themeState.active,
		m.colourless,
	))
}

// visibleSessionRowCount counts visible SessionItem rows, excluding group
// headers — the rows actually drawn, so it tracks the inside-tmux exclusion,
// an applied filter, and By-Tag's per-tag row repeats alike.
func (m Model) visibleSessionRowCount() int {
	count := 0
	for _, it := range m.sessionList.VisibleItems() {
		if _, ok := it.(SessionItem); ok {
			count++
		}
	}
	return count
}

// headerHeight measures the header against the same width and mode the render
// uses, so the reserved budget and the render agree exactly.
func (m Model) headerHeight(width int) int {
	return lipgloss.Height(renderHeaderBlock(width, m.themeState.active, m.colourless))
}

// replaceListBodyWithNoMatches swaps the list body for the no-matches state,
// preserving the title row byte-for-byte and rendering at the body's existing
// height so the composed view height is unchanged.
func (m Model) replaceListBodyWithNoMatches(listView string) string {
	bodyHeight := max(
		m.sessionList.Height()-1, 1)
	body := renderNoMatchesBody(m.sessionList.FilterValue(), m.contentWidth(), bodyHeight, m.themeState.active, m.colourless)
	idx := strings.IndexByte(listView, '\n')
	if idx < 0 {
		return listView + "\n" + body
	}
	return listView[:idx+1] + body
}

const byTagSignpostText = "No tags yet — add tags in a project's editor: press x for projects, then e to edit"
