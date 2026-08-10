package capture

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tui"
)

// errFixtureFatal is the canned underlying cause for the loading-error fixture's
// mocked fatal. The capture only renders the FatalError.UserMessage (set on the
// BootstrapFatalMsg directly), so this is a placeholder cause for the error
// interface — the harness never exits, so the value is never classified.
var errFixtureFatal = errors.New("permission denied")

// Fixture is a named, fully in-memory seam set for the capture harness. It
// bundles the canned read seams (lister, project store) so a test can assert
// the data, and exposes Deps() to assemble the production tui.Model via the
// shared tui.Build constructor.
type Fixture struct {
	name string

	// Lister is exported so tests can assert the deterministic session set.
	Lister *fakeLister

	projectStore  *fakeProjectStore
	projectEditor tui.ProjectEditor
	aliasEditor   tui.AliasEditor
	initialMode   prefs.SessionListMode
	// initialMultiSelect seeds multi-select mode with the named sessions
	// pre-marked (empty for every fixture that does not capture multi-select). It is
	// the only way to render the otherwise user-driven mode in the inert harness.
	initialMultiSelect []string
	// initialCursor seeds the cursor anchor: the session-row name the cursor lands
	// on once the list loads (empty → default index 0). Paired with initialMultiSelect
	// so the multi-select capture puts the cursor on a marked, banded row.
	initialCursor string
	// themeKeys is the SECOND panel-fixture input: prefs.json's three theme keys as
	// read. It is declared INDEPENDENTLY of the `--theme` palette, and the
	// separation is what makes the adaptive-pair frame reachable at all: capturetool
	// always passes the CONSTANT nomination shape, and a non-empty `theme` renders a
	// bare `●` with no slot badges — so a fixture built from the nomination alone
	// could only ever produce that one state. The union is what the panel LISTS;
	// these keys are what it MARKS. Zero for every fixture that declares no panel.
	themeKeys theme.RawKeys
	// themeUnion / themeEnumeration / themeSlots are the THIRD panel-fixture input,
	// all three fed to the no-I/O fakeThemeSource (see theme_fake.go): the
	// finished row set the panel lists, the retained enumeration Open hands back,
	// and the badge source Resolve returns.
	//
	// themeSlots is the one that is easy to get wrong. That Resolve return IS the
	// panel's badge source, so a fixture declaring its slots anywhere else renders
	// no `●` on any row, and the adaptive-pair frame loses its entire subject.
	//
	// Declaring NO union is what leaves a fixture without a panel: Deps wires no
	// seam at all in that case, so `t` stays the silent no-op the model's nil guard
	// gives it, rather than opening an empty frame nobody designed.
	themeUnion       theme.Union
	themeEnumeration theme.Enumeration
	themeSlots       []theme.SlotResolution
	// initialThemeCursor is the FOURTH panel-fixture input: the row IDENTITY the
	// panel's cursor lands on after the open. The panel puts the cursor on the theme
	// actually rendering, so without this a constant-while-previewing frame is
	// unreachable — its whole point is a cursor on a row OTHER than the marked one,
	// otherwise reachable only by arrowing, and a fixture is a one-shot render. It is
	// PLACEMENT ONLY and applies no theme (see tui.CaptureSeeds.ThemeCursor).
	initialThemeCursor string
	// initialThemeConfirm seeds the slot-from-constant confirm on the opened panel,
	// exactly as an `l` over a persisted constant raises it. It is the only route to
	// the confirm half of the message slot: a fixture is a ONE-SHOT RENDER, so a
	// state reached by a keypress has to be declarable directly — the same reasoning
	// that gives the panel its cursor seed.
	//
	// IT DECLARES STATE, NEVER TEXT. The slot composes its copy from its own pinned
	// constants, so no fixture can put a paraphrase on a captured frame.
	initialThemeConfirm bool
	// initialThemeCommitFailed seeds the failed-commit report on the opened panel:
	// the message slot's line and the outstanding-failure state together. A capture
	// wires NO theme persister, so the state is unreachable by committing however
	// many keys a fixture pressed — declaring it is the only route. It declares
	// STATE, never text, exactly as its confirm sibling does.
	initialThemeCommitFailed bool
	// initialDetection seeds the host-terminal detection cache with a resolved
	// identity (nil for every fixture that does not capture the proactive banner). It
	// is the only way to render the otherwise async-resolved detection in the inert
	// harness — used by the unsupported-terminal fixture to seed a non-NULL Apple
	// Terminal identity (which resolves unsupported).
	initialDetection *spawn.Identity
	// initialGoneFlagged seeds the pre-flight abort state — the gone-row set + the
	// red abort banner text (empty for every fixture that does not capture the abort).
	// Paired with initialMultiSelect so survivors keep their ● while the gone row shows
	// the red flag.
	initialGoneFlagged []string
	// initialBurstOpening seeds the in-burst Opening band as (done, total) (the
	// zero value for every fixture that does not capture the band). Paired with
	// initialMultiSelect so the marked rows show their ● beneath the Opening band.
	initialBurstOpening [2]int
	// initialFlash seeds the inline WARNING flash band on the first frame (empty for
	// every fixture that does not capture the flash). It is the only way to render
	// the otherwise-transient flash in the inert capture harness.
	initialFlash string
	// command seeds the command-pending mode (empty for every fixture that is not
	// the command-pending capture). When non-empty, tui.Build applies WithCommand so
	// the model opens on the Projects page with the command-pending banner shown.
	command []string
	// scrollback seeds the preview overlay's canned scrollback content (empty for
	// every fixture that does not open the preview). The seeded content is GENERIC
	// example terminal output — the preview mechanism is tool-agnostic.
	scrollback string
	// enumeratorGroups pins the preview's window/pane structure (empty → the
	// default multi-window fake). Used by the preview-screen fixture to render a
	// single Window 1/1 · Pane 1/1 counter shape matching the reference frame.
	enumeratorGroups []tmux.WindowGroup
	// serverStarted parks the model on PageLoading (the cold-boot loading screen) —
	// only the loading-screen fixture sets it.
	serverStarted bool
	// loadingEvents seeds the loading screen's live progress: the sequence of
	// BootstrapProgressMsg events folded through the model's loadingProgress
	// accumulator (the real Update path) to reach the captured mid-restore state.
	// Streamed via a receiver that blocks after the last event (never emitting the
	// terminal BootstrapCompleteMsg), so the loading page never dismisses and the
	// capture stays deterministically on PageLoading. Empty for every non-loading
	// fixture.
	loadingEvents []tui.BootstrapProgressMsg
	// fatalEvent seeds the fatal cold-boot error frame: after the
	// loadingEvents stream, the receiver emits this terminal BootstrapFatalMsg so
	// the model enters the error state (failed step ✗ + one-line message). Set ONLY
	// by the loading-error fixture; its zero value (FailedStep==0) means "no fatal"
	// and the receiver streams progress-then-blocks as usual.
	fatalEvent tui.BootstrapFatalMsg
	// captureKeys is the POST-LOAD key script that carries the fixture from its
	// loaded state to its CAPTURED one — the same sequence the fixture's tape
	// typed, declared once here so the two cannot drift.
	//
	// Captures and the tapes that render them are scaffolding and do not live in
	// the repository, so the sequences are declared and kept here rather than in
	// a tape.
	//
	// Most fixtures reach their captured state through a SEED and need no script.
	// Three do not: `projects` types `x` (it opens on Sessions, the production
	// no-arg default, and the capture is of the Projects page), `preview-screen`
	// presses Space (opening the preview overlay on the default-selected session),
	// and `sessions-empty` types `x` (with zero sessions the model default-routes to
	// Projects, so the empty-SESSIONS state is reached by switching back).
	//
	// Without the script those three render as the wrong screen entirely, and the
	// swap-and-diff guard — which enumerates fixtures and never names them — would
	// report coverage of the Projects page, the preview overlay and the empty state
	// while having rendered none of them.
	captureKeys []tea.KeyPressMsg
	// noColor renders the fixture under the NO_COLOR carve-out. The colourless
	// captures are taken by running the whole tool with NO_COLOR set, which
	// capturetool reads and injects itself, but the flag is declared so a fixture
	// that pins the carve-out is possible and so Fixture.Colourless has a value to
	// read: the swap-and-diff guard excludes colourless fixtures, and that exclusion
	// is meant to be structural rather than a hand-maintained name list.
	noColor bool
}

// Deps maps the fixture onto the shared tui.Deps seam set at the palette th.
// Every tmux seam is a fake: the read seams return canned data; the mutating
// seams are no-ops. The resulting model is the exact production model (built via
// tui.Build) — no bespoke render path that could drift from reality.
//
// THE PALETTE IS A PARAMETER BECAUSE TWO SEAMS DEPEND ON IT, and assembling them
// anywhere but here would let them disagree. `--theme` pins the CONSTANT
// nomination the model paints from — no gate, no wait, byte-deterministic
// frames — and the panel's faked ThemeSource must report that SAME palette
// from its Resolve, because the panel's open applies the theme that return names
// and a fake reporting anything else repaints the frame off `--theme`. There is
// exactly one construction site for the pair, and this is it; see
// newFakeThemeSource for what follows from getting it wrong.
//
// A fixture that declares no union gets NO panel seam, so `t` stays the silent
// no-op the model's nil guard gives it.
func (f *Fixture) Deps(th theme.Theme) tui.Deps {
	return tui.Deps{
		Theme:         theme.ConstantNomination(th),
		ThemeKeys:     f.themeKeys,
		ThemeSource:   f.themeSource(th),
		Lister:        f.Lister,
		Killer:        fakeKiller{},
		Renamer:       fakeRenamer{},
		Creator:       fakeCreator{},
		ProjectStore:  f.projectStore,
		ProjectEditor: f.projectEditor,
		AliasEditor:   f.aliasEditor,
		Enumerator:    fakeEnumerator{groups: f.enumeratorGroups},
		Reader:        fakeScrollbackReader{content: f.scrollback},
		// DirReader/DirRunner are deliberately left nil: the fixture sessions are
		// pre-stamped (Session.Dir set), so the lazy pane-read fallback never
		// fires — and the harness has no tmux server to read panes from anyway.
		// ModePersister is nil so an `s`-toggle during a capture writes nowhere.
		InitialMode:   f.initialMode,
		Command:       f.command,
		CWD:           "/home/user",
		ServerStarted: f.serverStarted,
		NoColor:       f.noColor,
		Capture: tui.CaptureSeeds{
			ThemeCursor:  f.initialThemeCursor,
			ThemeConfirm: f.initialThemeConfirm,
			// The panel's message slot is a single-slot arbiter, so a fixture declares AT
			// MOST one of these two — the seeds are alternatives and the model treats
			// them as such.
			ThemeCommitFailed: f.initialThemeCommitFailed,
			MultiSelect:       f.initialMultiSelect,
			Cursor:            f.initialCursor,
			Detection:         f.initialDetection,
			GoneFlagged:       f.initialGoneFlagged,
			BurstOpening:      f.initialBurstOpening,
			Flash:             f.initialFlash,
		},
		// Wire the loading-screen progress receiver only when the fixture seeds
		// events — it streams the seeded mid-restore sequence then blocks so the
		// loading page never dismisses (no terminal BootstrapCompleteMsg). The
		// loading-error fixture additionally seeds a terminal fatal so the receiver
		// streams progress then emits the terminal BootstrapFatalMsg.
		ProgressReceiver: loadingReceiverOrNil(f.loadingEvents, f.fatalEvent),
	}
}

// themeSource returns the fixture's no-I/O panel seam bound to the palette
// the frame is being rendered at, or nil when the fixture declares no panel.
//
// THE NIL IS THE POINT for every other fixture. tui's seam guard makes a nil
// source a silent `t` no-op, so a fixture with nothing to list simply has no
// panel — rather than opening a zero-row slide-over over a screen the frame was
// never designed to carry.
func (f *Fixture) themeSource(th theme.Theme) tui.ThemeSource {
	if len(f.themeUnion.Rows) == 0 {
		return nil
	}
	return newFakeThemeSource(th, f.themeEnumeration, f.themeUnion, f.themeSlots)
}

// loadingReceiverOrNil returns the loading-progress receiver for the seeded
// events, or nil when there are none (so non-loading fixtures leave
// ProgressReceiver unwired and keep the synchronous path). When a fatal is seeded
// (FailedStep > 0) it returns the fatal receiver (streams progress then
// emits the terminal BootstrapFatalMsg); otherwise the mid-restore
// streaming-then-blocking loading receiver.
func loadingReceiverOrNil(events []tui.BootstrapProgressMsg, fatal tui.BootstrapFatalMsg) tea.Cmd {
	if fatal.FailedStep > 0 {
		return loadingFatalReceiver(events, fatal)
	}
	if len(events) == 0 {
		return nil
	}
	return loadingProgressReceiver(events)
}

// Name returns the fixture's registered name.
func (f *Fixture) Name() string { return f.name }

// FixtureByName resolves a fixture by its registered name. An unknown name
// returns an error listing the available fixtures so a bad --fixture flag fails
// loudly with a useful message.
func FixtureByName(name string) (*Fixture, error) {
	switch name {
	case "sessions-flat":
		return sessionsFlatFixture(), nil
	case "sessions-empty":
		return sessionsEmptyFixture(), nil
	case "sessions-by-project":
		return sessionsByProjectFixture(), nil
	case "sessions-by-tag":
		return sessionsByTagFixture(), nil
	case "sessions-paged":
		return sessionsPagedFixture(), nil
	case "sessions-inline-flash":
		return sessionsInlineFlashFixture(), nil
	case "sessions-multi-select-active":
		return sessionsMultiSelectActiveFixture(), nil
	case "sessions-unsupported-terminal":
		return sessionsUnsupportedTerminalFixture(), nil
	case "sessions-unsupported-null":
		return sessionsUnsupportedNullFixture(), nil
	case "sessions-multi-select-preflight-abort":
		return sessionsMultiSelectPreflightAbortFixture(), nil
	case "sessions-burst-opening":
		return sessionsBurstOpeningFixture(), nil
	case "sessions-no-tags-signpost":
		return sessionsNoTagsSignpostFixture(), nil
	case "projects":
		return projectsFixture(), nil
	case "projects-command-pending":
		return projectsCommandPendingFixture(), nil
	case "theme-panel-adaptive-pair":
		return themePanelAdaptivePairFixture(), nil
	case "theme-panel-constant-previewing":
		return themePanelConstantPreviewingFixture(), nil
	case "theme-panel-invalid-row":
		return themePanelInvalidRowFixture(), nil
	case "theme-panel-dir-unreadable":
		return themePanelDirUnreadableFixture(), nil
	case "theme-panel-narrow":
		return themePanelNarrowFixture(), nil
	case "theme-panel-paginated":
		return themePanelPaginatedFixture(), nil
	case "theme-panel-projects":
		return themePanelProjectsFixture(), nil
	case "theme-panel-confirm":
		return themePanelConfirmFixture(), nil
	case "theme-panel-commit-failed":
		return themePanelCommitFailedFixture(), nil
	case "theme-panel-min-height-message":
		return themePanelMinHeightMessageFixture(), nil
	case "preview-screen":
		return previewScreenFixture(), nil
	case "loading-screen":
		return loadingScreenFixture(), nil
	case "loading-error":
		return loadingErrorFixture(), nil
	default:
		return nil, fmt.Errorf("unknown fixture %q (available: %s)", name, strings.Join(FixtureNames(), ", "))
	}
}

// FixtureNames returns the sorted list of registered fixture names — the single
// source of truth for "what can --fixture take", used by FixtureByName's error
// and the capture tool's help text. It includes the contrast-validation swatch
// (a standalone tea.Model resolved by the capture tool, NOT a tui.Model-backed
// *Fixture) so the swatch is discoverable from the same listing.
func FixtureNames() []string {
	names := []string{"sessions-flat", "sessions-empty", "sessions-by-project", "sessions-by-tag", "sessions-paged", "sessions-inline-flash", "sessions-multi-select-active", "sessions-unsupported-terminal", "sessions-unsupported-null", "sessions-multi-select-preflight-abort", "sessions-burst-opening", "sessions-no-tags-signpost", "theme-panel-adaptive-pair", "theme-panel-constant-previewing", "theme-panel-invalid-row", "theme-panel-dir-unreadable", "theme-panel-narrow", "theme-panel-paginated", "theme-panel-projects", "theme-panel-confirm", "theme-panel-commit-failed", "theme-panel-min-height-message", "projects", "projects-command-pending", "preview-screen", "loading-screen", "loading-error", ContrastValidationFixture}
	sort.Strings(names)
	return names
}

// sessionsFlatFixture builds the deterministic "sessions-flat" fixture: the
// fixed session set the design mock used, in order, with window counts and
// attached flags. Determinism is load-bearing — the data is injected in-memory so
// no real config or tmux server is read.
//
// Every session carries a stamped Dir so the grouped-render lazy pane-read
// fallback never fires (the harness has no tmux server). The Flat view (the only
// view sessions-flat captures) ignores Dir entirely, but stamping it keeps the
// fixture honest for any future grouped-mode fixture that reuses this set.
func sessionsFlatFixture() *Fixture {
	sessions := []tmux.Session{
		{Name: "agentic-workflows-code-based", Windows: 3, Attached: true, Dir: "/home/user/code/agentic-workflows"},
		{Name: "agentic-workflows-codify", Windows: 2, Attached: false, Dir: "/home/user/code/agentic-workflows"},
		{Name: "fab-flowx-explore", Windows: 1, Attached: false, Dir: "/home/user/code/fab"},
		{Name: "evvi webhooks and watchers", Windows: 4, Attached: false, Dir: "/home/user/code/evvi"},
		{Name: "aviva-proxy-qNyfEO", Windows: 1, Attached: false, Dir: "/home/user/code/aviva"},
		{Name: "designlab-web-r8suyU", Windows: 2, Attached: false, Dir: "/home/user/code/designlab"},
		{Name: "evvi-sync-engine", Windows: 1, Attached: false, Dir: "/home/user/code/evvi"},
		{Name: "fab-aws-migration", Windows: 5, Attached: false, Dir: "/home/user/code/fab"},
		{Name: "flow-v1-api-XkkhTN", Windows: 1, Attached: false, Dir: "/home/user/code/flow"},
		{Name: "flowx-7UKPZH", Windows: 2, Attached: false, Dir: "/home/user/code/flowx"},
		{Name: "fabric-lk26UG", Windows: 1, Attached: false, Dir: "/home/user/code/fabric"},
		{Name: "folio-Jiz4el", Windows: 1, Attached: false, Dir: "/home/user/code/folio"},
	}

	return &Fixture{
		name:         "sessions-flat",
		Lister:       &fakeLister{sessions: sessions},
		projectStore: &fakeProjectStore{projects: nil},
		initialMode:  prefs.ModeFlat,
	}
}

// sessionsEmptyFixture builds the deterministic "sessions-empty" fixture: ZERO
// sessions (an empty Lister) opened in Flat mode, so the empty-sessions state
// renders — the centred `▌ ▌ ▌` block glyph (text.faint) + `No sessions yet`
// (text.primary) + the hint (text.detail), with the FULLY-REPLACED footer
// (`n new in cwd · x projects · / filter · ? help`). It drives the empty-sessions
// reskin capture (mirrors testdata/vhs/reference/sessions-empty-mv.png).
//
// The project store is empty and no DirReader/DirRunner is wired (the empty-state
// path performs no pane reads). Like the other fixtures it NEVER opens a tmux server
// or touches ~/.config/portal: the Lister returns nil deterministically.
func sessionsEmptyFixture() *Fixture {
	return &Fixture{
		name:         "sessions-empty",
		Lister:       &fakeLister{sessions: nil},
		projectStore: &fakeProjectStore{projects: nil},
		initialMode:  prefs.ModeFlat,
		// With ZERO sessions the default-page rule routes the model to Projects, so
		// `x` is what lands it on the empty SESSIONS screen this fixture exists to
		// show. The rule is a one-time init flag, so the switch sticks.
		captureKeys: []tea.KeyPressMsg{keyRune('x')},
	}
}

// sessionsByProjectFixture builds the deterministic "sessions-by-project"
// fixture: opened in By-Project mode (initialMode) with a set of projects whose
// paths match most session dirs, plus one session whose directory matches NO
// project so it lands in the pinned Unknown catch-all. This drives the
// By-Project grouped capture so the `— by project` mode suffix and the restyled
// group headings + nested rows (heading text.detail, `··· N` count text.dim,
// rows indented one level under their heading) are visible in the screenshot.
//
// By-Project is Pattern A — ONE row per session under its project, the key being
// the session's directory reduced to a canonical path. agentic-workflows carries
// two sessions (so a group with N>1 renders), and `orphan-explore` is stamped to
// a directory with no matching project so the Unknown catch-all heading appears in
// the same frame. The project paths match the session Dir values exactly; both
// reduce to the same canonical key (CanonicalDirKey falls back to Clean(abs) for
// these non-existent paths), so the grouping is deterministic without a real
// filesystem.
func sessionsByProjectFixture() *Fixture {
	sessions := []tmux.Session{
		{Name: "agentic-workflows-code-based", Windows: 3, Attached: true, Dir: "/home/user/code/agentic-workflows"},
		{Name: "agentic-workflows-codify", Windows: 2, Attached: false, Dir: "/home/user/code/agentic-workflows"},
		{Name: "fab-flowx-explore", Windows: 1, Attached: false, Dir: "/home/user/code/fab"},
		{Name: "evvi webhooks and watchers", Windows: 4, Attached: false, Dir: "/home/user/code/evvi"},
		{Name: "aviva-proxy-qNyfEO", Windows: 1, Attached: false, Dir: "/home/user/code/aviva"},
		{Name: "designlab-web-r8suyU", Windows: 2, Attached: false, Dir: "/home/user/code/designlab"},
		// orphan-explore's directory matches NO project → the Unknown catch-all.
		{Name: "orphan-explore", Windows: 1, Attached: false, Dir: "/home/user/code/untracked-scratch"},
	}

	projects := []project.Project{
		{Path: "/home/user/code/agentic-workflows", Name: "agentic-workflows"},
		{Path: "/home/user/code/fab", Name: "fab"},
		{Path: "/home/user/code/evvi", Name: "evvi"},
		{Path: "/home/user/code/aviva", Name: "aviva"},
		{Path: "/home/user/code/designlab", Name: "designlab"},
		// No project for /home/user/code/untracked-scratch → orphan-explore is Unknown.
	}

	return &Fixture{
		name:         "sessions-by-project",
		Lister:       &fakeLister{sessions: sessions},
		projectStore: &fakeProjectStore{projects: projects},
		initialMode:  prefs.ModeByProject,
	}
}

// sessionsByTagFixture builds the deterministic "sessions-by-tag" fixture: the
// same session set as sessions-flat, but opened in By-Tag mode (initialMode) with
// tagged projects whose paths match the session dirs. This drives the By-Tag
// grouped capture so the `— by tag` mode suffix (and the restyled group headings)
// are visible in the screenshot.
//
// Tags are directory-anchored: they live on the project record and a
// session inherits its directory's tags at grouped-render time. The project paths
// match the session Dir values exactly; both reduce to the same canonical key
// (CanonicalDirKey falls back to Clean(abs) for these non-existent paths), so the
// match is deterministic without a real filesystem. The `evvi` project is left
// UNTAGGED so its sessions collect under the pinned `Untagged` catch-all — showing
// both a tagged heading and the catch-all in one frame.
func sessionsByTagFixture() *Fixture {
	sessions := []tmux.Session{
		{Name: "agentic-workflows-code-based", Windows: 3, Attached: true, Dir: "/home/user/code/agentic-workflows"},
		{Name: "agentic-workflows-codify", Windows: 2, Attached: false, Dir: "/home/user/code/agentic-workflows"},
		{Name: "fab-flowx-explore", Windows: 1, Attached: false, Dir: "/home/user/code/fab"},
		{Name: "evvi webhooks and watchers", Windows: 4, Attached: false, Dir: "/home/user/code/evvi"},
		{Name: "aviva-proxy-qNyfEO", Windows: 1, Attached: false, Dir: "/home/user/code/aviva"},
		{Name: "designlab-web-r8suyU", Windows: 2, Attached: false, Dir: "/home/user/code/designlab"},
		{Name: "evvi-sync-engine", Windows: 1, Attached: false, Dir: "/home/user/code/evvi"},
		{Name: "fab-aws-migration", Windows: 5, Attached: false, Dir: "/home/user/code/fab"},
	}

	projects := []project.Project{
		{Path: "/home/user/code/agentic-workflows", Name: "agentic-workflows", Tags: []string{"work"}},
		{Path: "/home/user/code/fab", Name: "fab", Tags: []string{"work", "client"}},
		{Path: "/home/user/code/aviva", Name: "aviva", Tags: []string{"client"}},
		{Path: "/home/user/code/designlab", Name: "designlab", Tags: []string{"personal"}},
		// evvi is deliberately untagged → its sessions land in the Untagged catch-all.
		{Path: "/home/user/code/evvi", Name: "evvi"},
	}

	return &Fixture{
		name:         "sessions-by-tag",
		Lister:       &fakeLister{sessions: sessions},
		projectStore: &fakeProjectStore{projects: projects},
		initialMode:  prefs.ModeByTag,
	}
}

// sessionsPagedFixture builds the deterministic "sessions-paged" fixture: a fixed
// set of 100 sessions — enough to span SEVERAL pages at the 1280×800 capture size
// (the measured per-page capacity there is ≤ 31, so ~4 pages), so the
// height-driven paginator renders a multi-dot centred row (one active accent.violet
// dot + several inactive text.faint dots) the restyle task captures. It opens in
// Flat mode (the dots are a Flat-list concern; By-Tag has its own fixture).
//
// The data is FIXED (count, names, window counts, attached flags) — determinism is
// the capture gate. The session set is generated by a pure index function so the
// 100 rows are stable across machines and runs; only the first row is attached so
// the capture shows the `● attached` marker on a single deterministic row. Every
// session carries a stamped Dir so the grouped-render lazy pane-read fallback never
// fires (the harness has no tmux server); Flat ignores Dir but stamping keeps the
// fixture honest.
func sessionsPagedFixture() *Fixture {
	const count = 100
	// A small ring of project dirs so the stamped Dirs are realistic and stable
	// without introducing per-row variance that could perturb the capture.
	dirs := []string{
		"/home/user/code/agentic-workflows",
		"/home/user/code/fab",
		"/home/user/code/evvi",
		"/home/user/code/aviva",
		"/home/user/code/designlab",
		"/home/user/code/flow",
	}
	sessions := make([]tmux.Session, 0, count)
	for i := range count {
		sessions = append(sessions, tmux.Session{
			Name: fmt.Sprintf("session-%02d", i),
			// A deterministic 1–4 window-count cycle so the rows are not all identical.
			Windows:  (i % 4) + 1,
			Attached: i == 0,
			Dir:      dirs[i%len(dirs)],
		})
	}

	return &Fixture{
		name:         "sessions-paged",
		Lister:       &fakeLister{sessions: sessions},
		projectStore: &fakeProjectStore{projects: nil},
		initialMode:  prefs.ModeFlat,
	}
}

// sessionsInlineFlashFixture builds the deterministic "sessions-inline-flash"
// fixture: a small Flat-mode session set with the inline WARNING flash band
// seeded on the first frame, mirroring testdata/vhs/reference/sessions-inline-flash-mv.png.
// The flash band (orange ▌ left-bar + ⚠ + "folio-Jiz4el closed externally — list
// updated" on the bg.warning tint, text.on-warning message) sits directly under
// the title separator, above the `Sessions 4` section header.
//
// The flash is otherwise transient (production sets it only on the preview-bail
// path), so it is seeded via CaptureSeeds.Flash — the only way to render it in the
// inert harness. The session set matches the reference exactly: fab-flowx-explore
// (attached, 3 windows), agentic-workflows-codify (1), flowx-7UKPZH (2),
// aviva-proxy-qNyfEO (1). Every session carries a stamped Dir for honesty even
// though Flat ignores it (no project store, no grouping).
func sessionsInlineFlashFixture() *Fixture {
	sessions := []tmux.Session{
		{Name: "fab-flowx-explore", Windows: 3, Attached: true, Dir: "/home/user/code/fab"},
		{Name: "agentic-workflows-codify", Windows: 1, Attached: false, Dir: "/home/user/code/agentic-workflows"},
		{Name: "flowx-7UKPZH", Windows: 2, Attached: false, Dir: "/home/user/code/flowx"},
		{Name: "aviva-proxy-qNyfEO", Windows: 1, Attached: false, Dir: "/home/user/code/aviva"},
	}

	return &Fixture{
		name:         "sessions-inline-flash",
		Lister:       &fakeLister{sessions: sessions},
		projectStore: &fakeProjectStore{projects: nil},
		initialMode:  prefs.ModeFlat,
		initialFlash: "folio-Jiz4el closed externally — list updated",
	}
}

// sessionsMultiSelectActiveFixture builds the deterministic
// "sessions-multi-select-active" fixture: the sessions-flat set (same 12 sessions,
// same order) opened in Flat mode directly in multi-select mode, mirroring the
// delivered frame testdata/vhs/reference/sessions-multi-select-active-mv.png. Three
// sessions are pre-marked — agentic-workflows-codify, fab-flowx-explore, and
// designlab-web-r8suyU — so the violet `3 selected` banner + `esc cancel` and the
// ● markers render, and the cursor is anchored on fab-flowx-explore so the marked
// CURSOR row shows both the selection band and the ● (marker takes precedence over
// the ▌ selector).
//
// Multi-select is otherwise user-driven (the `m` key), so it is seeded via the
// CaptureSeeds MultiSelect + Cursor seed seam — the only way to render the mode in
// the inert harness (the same pattern as CaptureSeeds.Flash for the transient
// flash). It reuses sessionsFlatFixture's session set verbatim; only the seed-seam
// fields and the fixture name differ. Like the other fixtures it NEVER opens a tmux server or
// touches ~/.config/portal.
func sessionsMultiSelectActiveFixture() *Fixture {
	fx := sessionsFlatFixture()
	fx.name = "sessions-multi-select-active"
	fx.initialMultiSelect = []string{
		"agentic-workflows-codify",
		"fab-flowx-explore",
		"designlab-web-r8suyU",
	}
	fx.initialCursor = "fab-flowx-explore"
	return fx
}

// sessionsUnsupportedTerminalFixture builds the deterministic
// "sessions-unsupported-terminal" fixture: the sessions-flat set (same 12 sessions,
// same order) opened in Flat mode in the NORMAL list (no multi-select), with the
// host-terminal detection cache seeded to a non-NULL Apple Terminal identity. Because
// com.apple.Terminal is a recognised-but-undriven terminal, spawn.ResolveAdapter
// resolves it UNSUPPORTED (non-NULL yet undriven), so DetectUnsupported() is true and
// the proactive amber `⚠ unsupported terminal — Apple Terminal · com.apple.Terminal`
// banner (+ blue `see docs`) REPLACES the standard `Sessions ··· N` section header —
// mirroring the delivered frame testdata/vhs/reference/sessions-unsupported-terminal-mv.png.
//
// Detection is otherwise an async lifecycle dispatched on reaching PageSessions,
// so it is seeded via the CaptureSeeds.Detection seed seam — the only way to render
// the resolved banner in the inert harness. It reuses sessionsFlatFixture's session set verbatim;
// only the seed-seam field and the fixture name differ. Like the other fixtures it
// NEVER opens a tmux server or touches ~/.config/portal.
func sessionsUnsupportedTerminalFixture() *Fixture {
	fx := sessionsFlatFixture()
	fx.name = "sessions-unsupported-terminal"
	fx.initialDetection = &spawn.Identity{Name: "Apple Terminal", BundleID: "com.apple.Terminal"}
	return fx
}

// sessionsUnsupportedNullFixture builds the deterministic "sessions-unsupported-null"
// fixture: the sessions-flat set opened in Flat mode in the NORMAL list (no
// multi-select), with the host-terminal detection cache seeded to an EMPTY
// spawn.Identity{}. An empty BundleID makes IsNull() true, so spawn.ResolveAdapter
// resolves it UNSUPPORTED (the NULL — no host-local terminal — shape), yet the
// banner is named-only, so a NULL identity renders the STANDARD `Sessions ··· N` header
// with NO banner — so this frame is visually identical to sessions-flat. It is the
// regression anchor that the resolved-unsupported NULL seed path does not intrude a
// banner.
//
// Detection is otherwise an async lifecycle dispatched on reaching PageSessions,
// so it is seeded via the same CaptureSeeds.Detection seed seam the named
// unsupported-terminal fixture uses — the only way to render the resolved detection
// in the inert harness. It reuses sessionsFlatFixture's session set verbatim; only the seed-seam field and the
// fixture name differ. Like the other fixtures it NEVER opens a tmux server or touches
// ~/.config/portal.
func sessionsUnsupportedNullFixture() *Fixture {
	fx := sessionsFlatFixture()
	fx.name = "sessions-unsupported-null"
	fx.initialDetection = &spawn.Identity{}
	return fx
}

// sessionsMultiSelectPreflightAbortFixture builds the deterministic
// "sessions-multi-select-preflight-abort" fixture: the sessions-flat set (same 12
// sessions, same order) opened in Flat multi-select mode with three sessions marked —
// agentic-workflows-codify, fab-flowx-explore, designlab-web-r8suyU — the cursor
// anchored on the gone row fab-flowx-explore, and the pre-flight abort seeded on
// fab-flowx-explore. So the red `⚠ 'fab-flowx-explore' is gone — nothing opened` banner
// (+ dim `esc dismiss`) owns the section-header row, the gone row shows the red ⚠ +
// `session gone` badge (the delegate gives goneFlagged precedence over the ● marker),
// and the two survivors keep their violet ● — mirroring the delivered frame
// testdata/vhs/reference/sessions-multi-select-preflight-abort-mv.png.
//
// The abort is otherwise reached only when an N≥2 Enter finds a marked session gone,
// so it is seeded via the CaptureSeeds MultiSelect + Cursor + GoneFlagged seed
// seams — the only way to render the frame in the inert harness. It reuses
// sessionsFlatFixture's session set verbatim; only the seed-seam fields and the fixture
// name differ. Like the other fixtures it NEVER opens a tmux server or touches
// ~/.config/portal.
func sessionsMultiSelectPreflightAbortFixture() *Fixture {
	fx := sessionsFlatFixture()
	fx.name = "sessions-multi-select-preflight-abort"
	fx.initialMultiSelect = []string{
		"agentic-workflows-codify",
		"fab-flowx-explore",
		"designlab-web-r8suyU",
	}
	fx.initialCursor = "fab-flowx-explore"
	fx.initialGoneFlagged = []string{"fab-flowx-explore"}
	return fx
}

// sessionsBurstOpeningFixture builds the deterministic "sessions-burst-opening"
// fixture: the sessions-flat set (same 12 sessions, same order) opened in Flat
// multi-select mode with the same three sessions marked, and the in-burst Opening
// band seeded as (2, 3). So the `Opening 2/3…` band (accent.violet — the HIGHEST
// section-header claimant) owns the section-header row while the three marked rows keep
// their violet ● beneath it. No design mock exists for this frame, so this capture is
// its own reference.
//
// The burst is otherwise driven by dispatchBurst + its progress goroutine, so it is
// seeded via the CaptureSeeds MultiSelect + BurstOpening seed seams — the only way
// to render the band in the inert harness (no goroutine, pipe, or cancel is wired). It
// reuses sessionsFlatFixture's session set verbatim; only the seed-seam fields and the
// fixture name differ. Like the other fixtures it NEVER opens a tmux server or touches
// ~/.config/portal.
func sessionsBurstOpeningFixture() *Fixture {
	fx := sessionsFlatFixture()
	fx.name = "sessions-burst-opening"
	fx.initialMultiSelect = []string{
		"agentic-workflows-codify",
		"fab-flowx-explore",
		"designlab-web-r8suyU",
	}
	fx.initialBurstOpening = [2]int{2, 3}
	return fx
}

// sessionsNoTagsSignpostFixture builds the deterministic
// "sessions-no-tags-signpost" fixture: opened in By-Tag mode (initialMode) with
// projects that carry NO tags. Because no project anywhere carries a tag,
// anyTagsExist is false, so the By-Tag view degrades to the flat session list
// under the persistent violet "No tags yet" info signpost (degrade with message,
// not silent flatten) rather than grouping.
//
// The project store IS populated (the session dirs map to real project records)
// so the gate is exercised honestly — it is the ZERO-tags condition, not a
// missing project store, that drives the signpost. The session set matches the
// reference frame exactly (testdata/vhs/reference/sessions-no-tags-signpost-mv.png):
// fab-flowx-explore (attached, 3 windows), agentic-workflows-codify (1),
// flowx-7UKPZH (2), aviva-proxy-qNyfEO (4). Every session carries a stamped Dir so
// the zero-pane-reads invariant on the signpost/flat arm holds without a tmux
// server (and the harness has none anyway). Like the other fixtures it NEVER opens
// a tmux server or touches ~/.config/portal.
func sessionsNoTagsSignpostFixture() *Fixture {
	sessions := []tmux.Session{
		{Name: "fab-flowx-explore", Windows: 3, Attached: true, Dir: "/home/user/code/fab"},
		{Name: "agentic-workflows-codify", Windows: 1, Attached: false, Dir: "/home/user/code/agentic-workflows"},
		{Name: "flowx-7UKPZH", Windows: 2, Attached: false, Dir: "/home/user/code/flowx"},
		{Name: "aviva-proxy-qNyfEO", Windows: 4, Attached: false, Dir: "/home/user/code/aviva"},
	}

	// Projects exist and match the session dirs, but NONE carry a tag → the
	// zero-tags-anywhere gate fires and the signpost shows over the flat list.
	projects := []project.Project{
		{Path: "/home/user/code/fab", Name: "fab"},
		{Path: "/home/user/code/agentic-workflows", Name: "agentic-workflows"},
		{Path: "/home/user/code/flowx", Name: "flowx"},
		{Path: "/home/user/code/aviva", Name: "aviva"},
	}

	return &Fixture{
		name:         "sessions-no-tags-signpost",
		Lister:       &fakeLister{sessions: sessions},
		projectStore: &fakeProjectStore{projects: projects},
		initialMode:  prefs.ModeByTag,
	}
}

// themePanelDropInSlug is the one valid DROP-IN in both panel fixtures' faked
// unions, standing beside the three built-ins.
//
// It matters that exactly one row is a drop-in: built-in and drop-in rows are
// DELIBERATELY INDISTINGUISHABLE, so a union of built-ins alone could not show
// that. It sorts first, which puts it at the top of both frames.
const themePanelDropInSlug = "catppuccin-latte"

// themePanelUnion is the faked row set both panel fixtures list: the three
// built-ins plus one valid drop-in, already in alphabetical display order.
//
// IT IS FAKED WHOLESALE, with no loader, no directory and no file — theme.Union
// is an ordinary value with exported fields precisely so a fixture can build one,
// which is the only way internal/capture renders a panel under its no-real-config
// import guard.
//
// Every row is left with a ZERO palette here and repainted by the fake onto the
// `--theme` value the frame is rendered at (newFakeThemeSource). Declaring a
// palette here would be the hard-coded built-in that makes `--theme` inert.
func themePanelUnion() theme.Union {
	rows := []theme.Row{
		{Slug: themePanelDropInSlug, Filename: themePanelDropInSlug + theme.FileExtension, Source: theme.SourceFile},
		{Slug: "nord", Source: theme.SourceBuiltin},
		{Slug: theme.DefaultDarkSlug, Source: theme.SourceBuiltin},
		{Slug: theme.DefaultLightSlug, Source: theme.SourceBuiltin},
	}
	return theme.Union{Rows: rows, Count: len(rows)}
}

// themePanelEnumeration is the retained parse Open hands back: the one drop-in
// the directory held, and the directory it was read from.
func themePanelEnumeration() theme.Enumeration {
	return themePanelDirEnumeration(themePanelDirEntry(themePanelDropInSlug+theme.FileExtension, themePanelDropInSlug))
}

// themePanelAdaptivePairFixture builds the deterministic
// "theme-panel-adaptive-pair" fixture: the sessions-flat list with the theme
// slide-over open over it, an adaptive pair persisted, and BOTH slot badges live
// — `● light` on tokyo-night-day and `● dark` on nord — with the cursor on nord.
//
// It is the primary reference for the badge vocabulary. Assigning a theme to a
// light/dark slot FROM INSIDE A PICKER has no established shape to borrow, so
// `d`/`l` and the `● dark` / `● light` / `● both` badges have no precedent: this
// frame and the design artboard beside it are the only reference that exists.
// Mirrors testdata/vhs/reference/theme-panel-adaptive-pair-mv.png.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme nord`. The cursor's row is ALWAYS what is painted
// behind the panel, so the palette follows the CURSOR and not the persisted
// setting. At open that resolves to the DARK slot's theme: capturetool runs no
// light/dark gate, so the standing no-answer fallback selects dark, and the dark
// slot here is nord.
//
// IT CANNOT BE AUTOMATED, which is why it is written down. An incoherent frame —
// say this one captured under `--theme tokyo-night-day` — is INDISTINGUISHABLE
// from a correct one to a reviewer: the panel would list the same rows, mark the
// same badges and sit the cursor on the same row, in the wrong colours, and
// nothing on the frame would say so. The swap-and-diff guard enumerates fixtures
// and diffs colours, so it passes either way. The pairing is the author's
// obligation.
//
// The panel is opened through the REAL path — the `t` its captureKeys declare, as
// the tape types it — so the frame goes through the production open and
// cursor-anchor with the faked seam supplying the union, rather than through a
// bespoke "already open" seed that could drift from what a keypress does.
func themePanelAdaptivePairFixture() *Fixture {
	fx := sessionsFlatFixture()
	fx.name = "theme-panel-adaptive-pair"
	fx.themeKeys = theme.RawKeys{Light: theme.DefaultLightSlug, Dark: "nord"}
	fx.themeUnion = themePanelUnion()
	fx.themeEnumeration = themePanelEnumeration()
	// The badge source. Both slots are set and loadable, so each badge sits on its
	// own persisted slug; the palettes are supplied by the fake.
	fx.themeSlots = []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: theme.DefaultLightSlug, Resolved: theme.DefaultLightSlug},
		{Slot: theme.SlotDark, Requested: "nord", Resolved: "nord"},
	}
	// The row the open already anchors — the in-force dark slot's. Declared
	// anyway, so the frame's cursor is stated by the fixture rather than inferred
	// from the gate's fallback, and so `--theme nord`'s coherent partner is visible
	// in one place.
	fx.initialThemeCursor = "nord"
	fx.captureKeys = []tea.KeyPressMsg{keyRune('t')}
	return fx
}

// themePanelConstantPreviewingFixture builds the deterministic
// "theme-panel-constant-previewing" fixture: the same list and the same union,
// with a CONSTANT persisted — a single bare `●` on nord — while the cursor sits on
// a different row entirely (tokyo-night-day).
//
// IT COMPLETES THE PANEL'S COVERAGE, because the two setting states never coexist
// on screen: no single frame can show both a bare `●` and the slot badges, so the
// constant form needs a capture of its own. It is also the picker idiom made
// visible — the `●` is what is PERSISTED, the cursor plus the canvas is what is
// PREVIEWED, and `Esc` would restore the marked one. Mirrors
// testdata/vhs/reference/theme-panel-constant-previewing-mv.png.
//
// The bare `●` follows from the setting rather than from a rendering choice: a
// non-empty `theme` wins and THE SLOTS ARE NOT READ AT ALL, so there is no slot
// for the marker to name.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme tokyo-night-day` — the PREVIEWED theme, deliberately
// NOT the marked constant `nord`. The cursor's row is always what is painted
// behind the panel, and on this frame those two are different rows by design;
// capturing it under `--theme nord` would paint the persisted constant while the
// cursor sat elsewhere, which is precisely the state the frame exists to rule out.
//
// IT CANNOT BE AUTOMATED. An incoherent frame is indistinguishable from a correct
// one to a reviewer — same rows, same badge, same cursor, wrong colours — and the
// swap-and-diff guard enumerates fixtures and diffs colours, so it passes either
// way.
//
// The cursor seed is what makes this frame reachable at all: the open lands the
// cursor on the theme actually rendering, which under a constant is the
// constant's own row, so the seed is the ONLY way a one-shot render reaches a
// cursor on another row (arrowing is the alternative, and a fixture never arrows).
func themePanelConstantPreviewingFixture() *Fixture {
	fx := sessionsFlatFixture()
	fx.name = "theme-panel-constant-previewing"
	fx.themeKeys = theme.RawKeys{Theme: "nord"}
	fx.themeUnion = themePanelUnion()
	fx.themeEnumeration = themePanelEnumeration()
	// The constant state: EXACTLY ONE slot record, and its Slot is SlotConstant.
	// theme.Badges reads both — the length and the slot — so a second record here
	// would render the pair's badges instead of the bare `●`.
	fx.themeSlots = []theme.SlotResolution{
		{Slot: theme.SlotConstant, Requested: "nord", Resolved: "nord"},
	}
	fx.initialThemeCursor = theme.DefaultLightSlug
	fx.captureKeys = []tea.KeyPressMsg{keyRune('t')}
	return fx
}

// themePanelConfirmFixture builds the deterministic "theme-panel-confirm" fixture:
// the constant-while-previewing frame's list, keys, union and cursor exactly, with
// the slot-from-constant confirm raised on the opened panel — captured through a
// tape whose terminal lands the panel on the panel's MINIMUM WIDTH.
//
// IT IS THE ONLY VISUAL PROOF OF THE FOOTER SUBSTITUTION. While the confirm is live
// it is key-exclusive within the panel and resolves on `y`/`n`/`Esc` alone, so the
// standing `⏎ set theme` / `d set as dark` / `l set as light` / `esc close` would
// advertise four keys of which NONE would act. The footer becomes `y confirm` /
// `n cancel`, and no other frame shows that.
//
// THE MINIMUM WIDTH IS THE OTHER HALF. The message slot may wrap to two rows
// there, and panel wording is a layout constraint as much as a copy choice, so the
// one line that might not fit has to be capturable at the width it might not fit
// at. Every other panel fixture but the narrow one is captured wide enough to take
// the preferred width, where the confirm fits on one line and the wrap is about
// nothing observable.
//
// THE FIXTURE DATA IS DELIBERATELY IDENTICAL to the constant-while-previewing
// frame's, so the two are a controlled pair: the persisted CONSTANT is what makes
// the confirm the state a real `l` would produce (it is raised over a constant and
// over nothing else), and everything that differs between the images is the
// question, its footer and the width.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme tokyo-night-day` — the previewed row the cursor is
// seeded onto, deliberately NOT the persisted constant `nord` the confirm names.
// The cursor's row is always what is painted behind the panel, and the two are
// different rows here by design.
//
// IT CANNOT BE AUTOMATED. An incoherent frame is indistinguishable from a correct
// one to a reviewer — same rows, same question, same cursor, wrong colours — and
// the swap-and-diff guard enumerates fixtures and diffs colours, so it passes
// either way.
func themePanelConfirmFixture() *Fixture {
	fx := themePanelConstantPreviewingFixture()
	fx.name = "theme-panel-confirm"
	fx.initialThemeConfirm = true
	return fx
}

// themePanelCommitFailedFixture builds the deterministic
// "theme-panel-commit-failed" fixture: the adaptive-pair frame's list, keys, union
// and cursor exactly, with the failed-commit report raised on the opened panel.
//
// IT IS THE ONLY PLACE THE ABSENCE OF A BAND IS CHECKABLE. The line renders in
// `accent.attention` for both the `⚠` and the text with NO `bg.attention` band,
// because the main screen's full-width warning band would read as HEAVY inside a
// 27–34 column panel — a judgement only an image settles, and one no assertion can
// make for a reviewer.
//
// THE BADGES ARE THE OTHER SUBJECT, and the adaptive pair is chosen for it: a
// failed write must not move the `●` (the marker means "what is PERSISTED" and
// would be lying if it moved), and two slot badges on two different rows show that
// far more legibly than a single bare `●` would.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme nord`, the adaptive pair's dark slot — the row the open
// anchors the cursor to under capturetool's gate-free dark fallback.
//
// IT CANNOT BE AUTOMATED. An incoherent frame is indistinguishable from a correct
// one to a reviewer, and the swap-and-diff guard enumerates fixtures and diffs
// colours, so it passes either way.
func themePanelCommitFailedFixture() *Fixture {
	fx := themePanelAdaptivePairFixture()
	fx.name = "theme-panel-commit-failed"
	fx.initialThemeCommitFailed = true
	return fx
}

// themePanelMinHeightMessageFixture builds the deterministic
// "theme-panel-min-height-message" fixture: the failed-commit frame's data exactly,
// captured through a tape whose terminal lands the panel ON the panel's HEIGHT
// FLOOR and at its minimum width.
//
// IT IS THE ONLY SURFACE ON WHICH THE FLOOR ARITHMETIC IS OBSERVABLE. The floor is
// header + footer + one list row + one message row, and that arithmetic is only
// observable on a frame that renders it. It is also the only check that the
// per-dimension rule holds: the message slot WRAPS at the minimum width but is
// TRUNCATED to one line at the minimum height, because a second row there would
// leave zero list rows or overflow the frame.
//
// THE FAILED-COMMIT LINE IS THE CONTENDER, DELIBERATELY. The confirm substitutes a
// SHORTER footer, so a frame built on it would render the floor's arithmetic for a
// scope the floor is not computed from — and the floor is deliberately the STANDING
// scope's, because it must hold for the footer that is live longest rather than for
// the transient one.
//
// THE FIXTURE DATA IS DELIBERATELY IDENTICAL to the failed-commit frame's, so the
// two are a controlled pair: everything that differs between the images is the size.
// A fixture cannot resize itself — it opens through captureKeys and is a one-shot
// render — so the floor is reachable ONLY by capturing at a terminal that lands on
// it.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme nord`, the adaptive pair's dark slot — the row the open
// anchors the cursor to under capturetool's gate-free dark fallback, and at the
// floor the ONE list row that renders is that row.
func themePanelMinHeightMessageFixture() *Fixture {
	fx := themePanelCommitFailedFixture()
	fx.name = "theme-panel-min-height-message"
	return fx
}

// themesDirPath is the plausible-looking themes directory every panel fixture's
// retained enumeration says it is OF. It is never resolved, opened or stat'ed —
// the fake reads nothing — and it is shared so no fixture invents a second,
// different-looking one.
const themesDirPath = "/home/user/.config/portal/themes"

// themePanelDirEntry is one candidate the themes directory held, as a retained
// enumeration carries it: the filename, the slug it yielded (empty for a
// `bad name` file, which mints none) and the path it was read from.
//
// The palette and the rejection are deliberately NOT restated here. Nothing reads
// Entries — the fake hands the enumeration straight back — so what an entry owes
// is an honest account of WHICH FILES WERE THERE, and duplicating each row's
// verdict would be a second place for it to drift from the union that renders it.
func themePanelDirEntry(filename, slug string) theme.Entry {
	return theme.Entry{Path: themesDirPath + "/" + filename, Filename: filename, Slug: slug}
}

// themePanelDirEnumeration is the retained parse for a READABLE themes directory
// holding the given candidates.
func themePanelDirEnumeration(entries ...theme.Entry) theme.Enumeration {
	return theme.Enumeration{Entries: entries, DirPath: themesDirPath}
}

// themePanelInvalidRowFixture builds the deterministic "theme-panel-invalid-row"
// fixture: the slide-over over the sessions-flat list, listing valid built-ins
// beside THREE invalid rows — the only frame on which the four-element row
// composition is observable at all.
//
// # THE THREE ROWS ARE THE THREE THINGS THE PRIORITY DECIDES
//
//   - `aurora-glow` carries a reason that FITS: the `⚠` and its terse reason as
//     one accent.attention run beside a text.subtle label, on ONE line — the
//     one-delegate-line invariant, which pagination and the arrow skip both rest
//     on. text.subtle and never text.faint: the label is the file the user must
//     read to know which of theirs is broken, and text.faint is decorative-only
//     and must never carry content.
//   - `nord-lee` is invalid AND PERSISTED, so the badge competes for the right edge
//     and the reason is DROPPED. That pairing is what makes the priority visible
//     rather than merely stated: the two rows differ in nothing but the badge.
//   - `My Gorgeous Midnight Palette.theme` is labelled by its FILENAME (a
//     `bad name` file has no slug) and is long enough to truncate with `…`. It
//     drops its reason too, by the OTHER drop rule: the label outranks the reason,
//     so a name long enough to fill the row takes the columns.
//
// The persisted-and-invalid row also makes the badge rule concrete — SET BUT
// UNLOADABLE still badges the PERSISTED slug, and the fallback's own row carries
// none. That is why the dark slot resolves to `tokyo-night` while `● dark` stays
// on `nord-lee`.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme tokyo-night` — the fallback the broken dark slot
// resolves to, which is where the open lands the cursor and therefore what is
// painted behind the panel. The cursor is on a VALID row deliberately: the arrow
// skip keeps it off invalid ones, so a frame with it parked on a rejection is a
// state production cannot reach.
func themePanelInvalidRowFixture() *Fixture {
	fx := sessionsFlatFixture()
	fx.name = "theme-panel-invalid-row"
	// The dark slot names a BROKEN drop-in; the light slot names a built-in that
	// loads. So one badge sits on an invalid row and the other on a valid one.
	fx.themeKeys = theme.RawKeys{Light: theme.DefaultLightSlug, Dark: themePanelBrokenSlug}
	fx.themeUnion = themePanelInvalidRowUnion()
	fx.themeEnumeration = themePanelDirEnumeration(
		themePanelDirEntry("aurora-glow"+theme.FileExtension, "aurora-glow"),
		themePanelDirEntry("My Gorgeous Midnight Palette"+theme.FileExtension, ""),
		themePanelDirEntry(themePanelBrokenSlug+theme.FileExtension, themePanelBrokenSlug),
	)
	fx.themeSlots = []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: theme.DefaultLightSlug, Resolved: theme.DefaultLightSlug},
		// The fallback: the nomination did not load, so Resolved names the
		// mode-matched default while Requested — the field the badge table keys
		// on — still names what the user set.
		{Slot: theme.SlotDark, Requested: themePanelBrokenSlug, Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonBadColour},
	}
	fx.initialThemeCursor = theme.DefaultDarkSlug
	fx.captureKeys = []tea.KeyPressMsg{keyRune('t')}
	return fx
}

// themePanelBrokenSlug is the drop-in that is BOTH persisted and invalid — the row
// on which the badge outranks the reason. It is named once because two fixtures
// use it for two different reasons (a broken file here, an unreachable one under an
// unreadable directory).
const themePanelBrokenSlug = "nord-lee"

// themePanelInvalidRowUnion is the faked row set the invalid-row frame lists,
// already in display order — alphabetical by SORT KEY, case-insensitively, which
// is the slug wherever one exists and the filename for the `bad name` row that has
// none.
//
// The order is written out rather than sorted here for the same reason the union
// is faked at all: a fixture declares the finished row set the seam hands back,
// and re-deriving it would put a second copy of the union's comparison in the
// harness.
func themePanelInvalidRowUnion() theme.Union {
	rows := []theme.Row{
		// Invalid with a reason that FITS beside its label.
		{
			Slug: "aurora-glow", Filename: "aurora-glow" + theme.FileExtension, Source: theme.SourceFile,
			Rejection: &theme.Rejection{Reason: theme.ReasonBadSyntax, Line: 12, Detail: "quoted value"},
		},
		// `bad name`: no slug at all (names are rejected, never normalised), so it is
		// LABELLED and SORTED by its filename — and that filename is long enough to
		// take the whole row, which is what drops its reason.
		{
			Filename: "My Gorgeous Midnight Palette" + theme.FileExtension, Source: theme.SourceFile,
			Rejection: &theme.Rejection{Reason: theme.ReasonBadName, BadNameCause: theme.BadNameSlug},
		},
		{Slug: "nord", Source: theme.SourceBuiltin},
		// Invalid AND persisted: `bad colour` never renders, because the `● dark`
		// badge takes the right edge they compete for.
		{
			Slug: themePanelBrokenSlug, Filename: themePanelBrokenSlug + theme.FileExtension, Source: theme.SourceFile,
			Rejection: &theme.Rejection{Reason: theme.ReasonBadColour, Line: 7, Detail: "text.primary = #12345"},
		},
		{Slug: theme.DefaultDarkSlug, Source: theme.SourceBuiltin},
		{Slug: theme.DefaultLightSlug, Source: theme.SourceBuiltin},
	}
	return theme.Union{Rows: rows, Count: len(rows), Rejected: 3}
}

// themePanelDirUnreadableFixture builds the deterministic
// "theme-panel-dir-unreadable" fixture: the slide-over with the pinned
// `⚠ dir unreadable` chrome row, captured ON PAGE 2.
//
// # THE PAGE IS THE WHOLE POINT
//
// The warning is chrome pinned to the viewport directly beneath the header, NOT a
// list row — a list row participates in pagination, so the warning would vanish
// the moment the user paged down. A page-1 capture is INDISTINGUISHABLE from one
// of a warning implemented as a list delegate — both show the row — so the claim
// is only observable once the user has paged past it. Hence the `Ctrl+↓` in
// captureKeys, and hence the tape's reduced height: the union must OVERFLOW the
// panel body or there is no second page to reach.
//
// # AND ROWS MUST STILL RENDER BENEATH IT
//
// Built-in rows and persisted-slug rows still render beneath it — the PERSISTED
// ROWS ESPECIALLY, or a user with an unreadable directory loses the `●` entirely.
// Page 2 therefore carries both: the persisted `solarized-lee` with its surviving
// `● light`, and the built-in `tokyo-night` under the cursor.
//
// An unusable directory contributes NO entries, so the union is the three built-ins
// plus the two persisted slugs that answer to nothing — each rejected `unreadable`
// rather than `not found`, because the theme may be sitting right there in a
// directory nothing can read and `not found` would send the user past the actual
// problem.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme tokyo-night`. The cursor is SEEDED onto `nord` — a
// page-1 row, since a `Ctrl+↓` from the last page moves nothing — and the paging
// key then lands it on `tokyo-night`, which is both the in-force dark slot's
// fallback and the theme the frame is painted in.
func themePanelDirUnreadableFixture() *Fixture {
	fx := sessionsFlatFixture()
	fx.name = "theme-panel-dir-unreadable"
	// Both slots name drop-ins the unreadable directory cannot answer for, so each
	// contributes a persisted row AND keeps its badge — the `●` the user must not
	// lose.
	fx.themeKeys = theme.RawKeys{Light: themePanelUnreachableLightSlug, Dark: themePanelBrokenSlug}
	fx.themeUnion = themePanelDirUnreadableUnion()
	// An unusable directory sets the flag and holds no entries. It still says
	// which directory it is OF.
	fx.themeEnumeration = theme.Enumeration{DirUnusable: true, DirPath: themesDirPath}
	fx.themeSlots = []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: themePanelUnreachableLightSlug, Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonUnreadable},
		{Slot: theme.SlotDark, Requested: themePanelBrokenSlug, Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonUnreadable},
	}
	// A PAGE-1 row, deliberately: the open would otherwise anchor the cursor on
	// the dark slot's fallback, which is already on page 2, and the paging key
	// below would then move nothing.
	fx.initialThemeCursor = "nord"
	fx.captureKeys = []tea.KeyPressMsg{keyRune('t'), keyPageDown()}
	return fx
}

// themePanelUnreachableLightSlug is the light slot's persisted drop-in, which the
// unreadable directory cannot answer for. It sorts between the two `nord*` rows
// and the two `tokyo-night*` ones, which is what puts one badged persisted row on
// each page of the captured frame.
const themePanelUnreachableLightSlug = "solarized-lee"

// themePanelDirUnreadableUnion is the row set an unusable themes directory
// produces: the built-ins, plus one row per IN-FORCE persisted slug that nothing
// answers to, in display order.
//
// Both persisted rows carry `unreadable` and NO detail — the row renders the terse
// reason alone, the condition already has its own pinned chrome row, and the
// verbatim system message belongs to doctor.
func themePanelDirUnreadableUnion() theme.Union {
	rows := []theme.Row{
		{Slug: "nord", Source: theme.SourceBuiltin},
		{Slug: themePanelBrokenSlug, Source: theme.SourcePersisted, Rejection: &theme.Rejection{Reason: theme.ReasonUnreadable}},
		{Slug: themePanelUnreachableLightSlug, Source: theme.SourcePersisted, Rejection: &theme.Rejection{Reason: theme.ReasonUnreadable}},
		{Slug: theme.DefaultDarkSlug, Source: theme.SourceBuiltin},
		{Slug: theme.DefaultLightSlug, Source: theme.SourceBuiltin},
	}
	return theme.Union{Rows: rows, DirUnusable: true, Count: len(rows), Rejected: 2}
}

// themePanelNarrowFixture builds the deterministic "theme-panel-narrow" fixture:
// the adaptive-pair frame's list, keys, union and cursor exactly, captured through
// a tape whose terminal lands the panel in the DEGRADED BAND between the
// preferred and minimum widths.
//
// IT IS THE ONLY OBSERVABLE CHECK ON THE LADDER. The shrink is staged — the
// panel takes half the content region, clamped to the two ends — and every other
// panel fixture is captured wide enough to take the preferred width, so without
// this frame the middle of the ladder is rendered by nothing. A fixture cannot
// resize itself either: it opens through captureKeys and is a one-shot render, so
// the band is reachable ONLY by capturing at a narrower terminal.
//
// THE FIXTURE DATA IS DELIBERATELY IDENTICAL to the adaptive-pair frame's, so the
// two are a controlled pair: everything that differs between the images is the
// width. Both badges must survive the shrink (row composition reserves them before
// anything can crowd them out) and every row must still be exactly one line.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme nord`, the adaptive pair's dark slot — capturetool runs
// no light/dark gate, so the standing no-answer fallback selects dark and the open
// lands the cursor there.
func themePanelNarrowFixture() *Fixture {
	fx := themePanelAdaptivePairFixture()
	fx.name = "theme-panel-narrow"
	return fx
}

// themePanelPaginatedFixture builds the deterministic "theme-panel-paginated"
// fixture: a union long enough to OVERFLOW the panel body, so the list's
// pagination dots actually render.
//
// IT EXISTS BECAUSE PAGINATION DOTS ONLY RENDER WHEN THE PANEL'S LIST PAGINATES,
// so one panel fixture must carry enough theme rows to overflow — otherwise the
// dots are covered by nothing. They are the exemplar of the cached-style class:
// `bubbles/list` reads their STRINGS out of its styles once at construction, so a
// restyle that failed to re-feed the paginator would leave the library's hardcoded
// greys rendering under every theme, IDENTICALLY before and after a swap, which
// the swap-and-diff guard structurally cannot see.
//
// The drop-ins are SYNTHETIC and say so: they are a plausible directory of user
// files, generated by a pure index function so the row set is stable across
// machines and runs. They sort AFTER every built-in, which is what keeps both slot
// badges on the captured first page.
//
// EVERYTHING ELSE IS THE ADAPTIVE-PAIR FRAME'S, so the two are a controlled pair:
// the same list, keys, slots and cursor, and the longer union is the only reason
// the dots are there.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme nord` — the in-force dark slot, which the open anchors
// the cursor to and which sits on page 1.
func themePanelPaginatedFixture() *Fixture {
	fx := themePanelAdaptivePairFixture()
	fx.name = "theme-panel-paginated"
	fx.themeUnion = themePanelPaginatedUnion()
	fx.themeEnumeration = themePanelDirEnumeration(themePanelPaginatedEntries()...)
	return fx
}

// themePanelSyntheticDropIns is how many synthetic drop-ins the paginating union
// carries beside the built-ins.
//
// The count is chosen for MARGIN rather than for the exact overflow threshold: the
// panel body is the render height minus its header, footer and slot rows, so the
// row count at which a list stops paginating moves with the capture size. A count
// comfortably above the widest body any capture uses keeps the frame paginating at
// the tape's size and at the swap-and-diff guard's pinned harness size alike —
// which matters because a union that fitted on one page at either would leave the
// pagination dots uncovered again.
const themePanelSyntheticDropIns = 30

// themePanelSyntheticSlug is the slug of one synthetic drop-in. They are named to
// sort AFTER every built-in slug, so the two badged rows stay on page 1.
func themePanelSyntheticSlug(i int) string {
	return fmt.Sprintf("vivid-%02d", i+1)
}

// themePanelPaginatedUnion is the row set the paginating frame lists: the base
// panel union, then the synthetic drop-ins — in display order throughout.
//
// The base set is DERIVED rather than restated, so a row added to it reaches this
// frame too; the clone keeps the base's backing array out of the append's reach.
func themePanelPaginatedUnion() theme.Union {
	rows := slices.Clone(themePanelUnion().Rows)
	for i := range themePanelSyntheticDropIns {
		slug := themePanelSyntheticSlug(i)
		rows = append(rows, theme.Row{Slug: slug, Filename: slug + theme.FileExtension, Source: theme.SourceFile})
	}
	return theme.Union{Rows: rows, Count: len(rows)}
}

// themePanelPaginatedEntries is the retained parse behind that union: every file
// row it lists, in the directory order a real read produces.
//
// It extends the base panel enumeration's entries for the same reason the union
// extends the base rows — one declaration of the shared drop-in, cloned so the
// append cannot reach the base.
func themePanelPaginatedEntries() []theme.Entry {
	entries := slices.Clone(themePanelEnumeration().Entries)
	for i := range themePanelSyntheticDropIns {
		slug := themePanelSyntheticSlug(i)
		entries = append(entries, themePanelDirEntry(slug+theme.FileExtension, slug))
	}
	return entries
}

// themePanelProjectsFixture builds the deterministic "theme-panel-projects"
// fixture: the adaptive-pair panel composited over the PROJECTS page.
//
// `t` is bound there because theme is a GLOBAL setting — refusing would make it
// feel page-scoped for no reason — and Projects has its own transient-flash slot
// for exactly the panel's signals. Every other panel fixture is implicitly
// Sessions-based, which is why this one exists: the page is seen with the panel
// over it before release rather than after.
//
// It reuses the `projects` fixture's store verbatim, so the page beneath is the
// one that frame already establishes, and its captureKeys are that fixture's `x`
// followed by the panel's `t` — the real user path to this screen, typed in the
// order a user types them.
//
// The frame also shows the overlay's ACCEPTED COST at its most visible: it cuts
// wherever its left border falls, mid-label included, because the page beneath is
// composed at the UNREDUCED content width and deliberately not re-laid-out.
//
// # THE COHERENCE RULE: `--theme` MUST NAME THE THEME UNDER THE CURSOR
//
// Capture it with `--theme nord`, the adaptive pair's dark slot — the row the open
// anchors the cursor to under capturetool's gate-free dark fallback.
func themePanelProjectsFixture() *Fixture {
	fx := projectsFixture()
	fx.name = "theme-panel-projects"
	pair := themePanelAdaptivePairFixture()
	fx.themeKeys = pair.themeKeys
	fx.themeUnion = pair.themeUnion
	fx.themeEnumeration = pair.themeEnumeration
	fx.themeSlots = pair.themeSlots
	fx.initialThemeCursor = pair.initialThemeCursor
	// `x` reaches the Projects page (the fixture opens on Sessions, the production
	// no-arg default); `t` opens the slide-over over it.
	fx.captureKeys = []tea.KeyPressMsg{keyRune('x'), keyRune('t')}
	return fx
}

// projectsFixture builds the deterministic "projects" fixture: a rich project
// store (14 projects with real-looking absolute paths, mirroring the reference
// frame testdata/vhs/reference/projects-mv.png) so the Projects page reskin — the
// PORTAL header, the state.green `Projects 14` section header, the two-line rows
// (name text.primary heavy / path text.detail dim), the full-height accent.violet
// left-bar selection over the bg.selection tint, and the condensed footer — is
// visible in the screenshot.
//
// It opens on the Sessions page (the production default for a no-arg launch) and
// types `x` through its captureKeys to switch to the Projects page. The
// first project (flow-v1-api) is the cursor row in the reference, so it carries the
// full-height violet bar + selection tint in the capture. The session set is the
// sessions-flat set so the pre-`x` Sessions frame is a realistic, deterministic
// list; the capture is of the Projects page reached by the `x` key.
//
// Like the other fixtures it NEVER opens a tmux server or touches ~/.config/portal:
// the project store is the in-memory fake.
func projectsFixture() *Fixture {
	sessions := []tmux.Session{
		{Name: "agentic-workflows-code-based", Windows: 3, Attached: true, Dir: "/Users/leeovery/Code/agentic-workflows"},
		{Name: "portal", Windows: 2, Attached: false, Dir: "/Users/leeovery/Code/portal"},
		{Name: "flowx-7UKPZH", Windows: 1, Attached: false, Dir: "/Users/leeovery/Code/fabric/flowx"},
	}

	// 14 projects with real-looking absolute paths (matches the reference count).
	// flow-v1-api carries the reference Tags [Fabric, api] so the edit-project modal
	// capture (opened on it via the edit-modal tapes) renders the seeded chips.
	flowPath := "/Users/leeovery/Code/fabric/flowv1/flow-v1-api"
	projects := []project.Project{
		{Name: "flow-v1-api", Path: flowPath, Tags: []string{"Fabric", "api"}},
		{Name: "portal", Path: "/Users/leeovery/Code/portal"},
		{Name: "mint", Path: "/Users/leeovery/Code/mint"},
		{Name: "agntc", Path: "/Users/leeovery/Code/agntc"},
		{Name: "agentic-workflows", Path: "/Users/leeovery/Code/agentic-workflows"},
		{Name: "leeovery", Path: "/Users/leeovery"},
		{Name: "flowx", Path: "/Users/leeovery/Code/fabric/flowx"},
		{Name: "designlab-web", Path: "/Users/leeovery/Code/designlab/designlab-web"},
		{Name: "evvi", Path: "/Users/leeovery/Code/evvi"},
		{Name: "aviva-proxy", Path: "/Users/leeovery/Code/aviva"},
		{Name: "fab-aws-migration", Path: "/Users/leeovery/Code/fab"},
		{Name: "folio", Path: "/Users/leeovery/Code/folio"},
		{Name: "fabric", Path: "/Users/leeovery/Code/fabric"},
		{Name: "evvi-sync-engine", Path: "/Users/leeovery/Code/evvi-sync"},
	}

	return &Fixture{
		name:         "projects",
		Lister:       &fakeLister{sessions: sessions},
		projectStore: &fakeProjectStore{projects: projects},
		// Wire the edit-project modal editors so the `e` key opens the modal in the
		// harness (handleEditProjectKey nil-guards when either is nil). The aliases
		// map seeds flow-v1-api's reference alias chips [fapi, v1] (keyed to the same
		// path the project record carries, so the modal's per-project alias lookup
		// matches). All mutations are in-memory no-ops.
		projectEditor: fakeProjectEditor{},
		aliasEditor: fakeAliasEditor{aliases: map[string]string{
			"fapi": flowPath,
			"v1":   flowPath,
		}},
		initialMode: prefs.ModeFlat,
		// The fixture opens on Sessions (the production no-arg default); `x` is the
		// real user path to the Projects page this fixture captures.
		captureKeys: []tea.KeyPressMsg{keyRune('x')},
	}
}

// projectsCommandPendingFixture builds the deterministic "projects-command-pending"
// fixture: the SAME rich project store as the `projects` fixture, but built with a
// pending command so m.commandPending is true — the model opens directly on the
// command-pending Projects page (WithCommand sets PageProjects), rendering the
// banner (violet `▌` left-bar + `▸ Pick a project to run` + the command in an
// accent.orange chip) over the FULL Projects chrome (green `Projects 14` header +
// `/ to filter`), with the swapped `⏎ run here · n run in cwd · esc cancel` footer.
// Mirrors testdata/vhs/reference/projects-command-pending-mv.png.
//
// The seeded command is a GENERIC build command (`npm run dev`) — Portal's
// run-a-command mechanism is tool-agnostic, so no Portal artifact references any
// specific tool. The reference frame's chip shows a different example command; only
// the chip's command text differs (generic vs the frame's example) — the banner
// structure, colours, chrome, and footer match. Like the other fixtures it NEVER
// opens a tmux server or touches ~/.config/portal: the project store is the
// in-memory fake.
func projectsCommandPendingFixture() *Fixture {
	fx := projectsFixture()
	fx.name = "projects-command-pending"
	fx.command = []string{"npm", "run", "dev"}
	return fx
}

// previewScreenFixture builds the deterministic "preview-screen" fixture: a Flat
// session list whose FIRST (default-selected) session is aviva-proxy-qNyfEO, so
// pressing Space on it opens the preview overlay onto a single-window
// single-pane session (Window 1/1 · Pane 1/1) seeded with generic canned
// scrollback. This drives the cyan peek-mode chrome reskin capture — the
// accent.cyan top bar (◉ preview marker + session + counters + right-aligned nav
// hints) framing the untouched captured ANSI content — mirroring
// testdata/vhs/reference/preview-screen-mv.png.
//
// The scrollback is GENERIC example terminal output (a kubectl rollout + build
// lines) — Portal's preview mechanism is tool-agnostic, so the captured content
// references no specific tool or model — the content is whatever the pane
// printed, rendered as untouched real ANSI. The enumerator is pinned to a
// single 1/1 window/pane so the counters match the reference. Like the other
// fixtures it NEVER opens a tmux server or touches ~/.config/portal.
func previewScreenFixture() *Fixture {
	sessions := []tmux.Session{
		{Name: "aviva-proxy-qNyfEO", Windows: 1, Attached: false, Dir: "/home/user/code/aviva"},
		{Name: "agentic-workflows-code-based", Windows: 3, Attached: true, Dir: "/home/user/code/agentic-workflows"},
		{Name: "fab-flowx-explore", Windows: 1, Attached: false, Dir: "/home/user/code/fab"},
		{Name: "evvi-sync-engine", Windows: 1, Attached: false, Dir: "/home/user/code/evvi"},
	}

	scrollback := "$ kubectl rollout status deploy/aviva-proxy\n" +
		"deployment \"aviva-proxy\" successfully rolled out\n" +
		"$ make build\n" +
		"go build -o bin/aviva-proxy ./cmd/aviva-proxy\n" +
		"build complete in 4.2s\n" +
		"$ ./bin/aviva-proxy --check\n" +
		"config ok · 3 routes · listening on :8080\n"

	return &Fixture{
		name:             "preview-screen",
		Lister:           &fakeLister{sessions: sessions},
		projectStore:     &fakeProjectStore{projects: nil},
		initialMode:      prefs.ModeFlat,
		scrollback:       scrollback,
		enumeratorGroups: []tmux.WindowGroup{{WindowIndex: 0, WindowName: "main", PaneIndices: []int{0}}},
		// Space on the default-selected session is what opens the preview overlay
		// this fixture captures.
		captureKeys: []tea.KeyPressMsg{{Code: tea.KeySpace}},
	}
}

// loadingScreenFixture builds the deterministic "loading-screen" fixture: the
// cold-boot loading page (serverStarted) seeded with the reference mid-restore
// progress — steps 1–2 done (✓ Started tmux server, ✓ Registered hooks),
// "Restoring sessions" ACTIVE with the 8 / 12 counter, "Replaying scrollback" and
// "Running resume commands" pending, the bar partially filled. It mirrors
// testdata/vhs/reference/loading-mv.png (`Loading 6 — Combined (thick bar)`).
//
// The mid-restore state is reached by folding a real BootstrapProgressMsg
// sequence through the model's loadingProgress accumulator (the same path the
// live channel drives): steps 1–5 complete, then a step-6 skeleton event with
// RestoreN=8 / RestoreM=12 (which sets the active "Restoring sessions" counter
// without marking restore done — so the bar sits at 5/11 with that label active).
// The receiver streams these events then BLOCKS (never emits the terminal
// BootstrapCompleteMsg), so the dual-gate never dismisses the loading page and the
// capture stays deterministically parked on it.
//
// The session list is empty — it is never shown (the page never transitions). Like
// the other fixtures it NEVER opens a tmux server or touches ~/.config/portal.
func loadingScreenFixture() *Fixture {
	events := []tui.BootstrapProgressMsg{
		{Index: 1},
		{Index: 2},
		{Index: 3},
		{Index: 4},
		{Index: 5},
		// Step-6 skeleton event: sets the active "Restoring sessions" 8/12 counter
		// without completing restore, so the page sits mid-restore.
		{Index: 6, RestoreN: 8, RestoreM: 12},
	}
	return &Fixture{
		name:          "loading-screen",
		Lister:        &fakeLister{sessions: nil},
		projectStore:  &fakeProjectStore{projects: nil},
		initialMode:   prefs.ModeFlat,
		serverStarted: true,
		loadingEvents: events,
	}
}

// loadingErrorFixture builds the deterministic "loading-error" fixture: the fatal
// cold-boot error frame, MOCKED at implementation (no design mock exists — the
// capture IS the mock). Steps 1–2 complete (✓ Started tmux server,
// ✓ Registered hooks), then a fatal aborts at step 3 (SetRestoring) — so that
// step's row carries the state.red ✗ marker, the trailing labels stay pending (·,
// they never ran), the one-line FatalError.UserMessage renders beneath the
// step-list in state.red, and a `q quit · esc quit` hint sits at the bottom. The
// bar freezes at the fraction reached at fatal time (2/11) — not completed.
//
// The mid-then-fatal state is reached by folding a real BootstrapProgressMsg
// sequence (steps 1–2) through the model's accumulator, then a terminal
// BootstrapFatalMsg (FailedStep 3 → the "Registered hooks" group label) — the
// same path the live channel drives. The receiver streams these then BLOCKS,
// so the page parks on the error frame and NEVER transitions to the picker. The
// session list is empty (never shown). Like every fixture it NEVER opens a tmux
// server or touches ~/.config/portal.
func loadingErrorFixture() *Fixture {
	events := []tui.BootstrapProgressMsg{
		{Index: 1},
		{Index: 2},
	}
	return &Fixture{
		name:          "loading-error",
		Lister:        &fakeLister{sessions: nil},
		projectStore:  &fakeProjectStore{projects: nil},
		initialMode:   prefs.ModeFlat,
		serverStarted: true,
		loadingEvents: events,
		fatalEvent: tui.BootstrapFatalMsg{
			FailedStep: 3,
			Message:    "Portal failed to set @portal-restoring marker: permission denied",
			Err:        errFixtureFatal,
		},
	}
}
