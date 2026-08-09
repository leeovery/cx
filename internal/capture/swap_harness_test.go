package capture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// The harness dimensions every swap-harness assertion renders at. Wide and tall
// enough that the section header, the list rows and the footer all fit, so a
// fixture's distinguishing content is genuinely on the frame rather than clipped.
//
// IT MUST ALSO CLEAR THE §9.8 PANEL RENDER FLOOR, which is a second and stricter
// claim on the same pair of numbers: a panel fixture's captureKeys press `t`, so
// the guard drives it through the panel's entry gate. Below that floor the gate
// REFUSES and the guard renders a panel-less frame carrying a blocked-`t` flash —
// with every assertion still passing, because the guard scans whatever was
// painted. The panel's own bubbles/list instance, which §11.2 names the worst
// case of the cached-style class, would then be covered by nothing while reading
// as covered. RAISE these rather than lowering them; the clearance is proven by
// TestPanelFixture_PanelIsCompositedUnderTheGuard rather than assumed.
const (
	harnessWidth  = 120
	harnessHeight = 40
)

// buildBackedFixtureNames is the registered fixture set MINUS the
// contrast-validation swatch, which is a standalone tea.Model resolved by the
// capture tool and deliberately not routed through tui.Build (§13.3) — so it has
// no tui.Model to drive.
//
// It is derived from FixtureNames rather than hand-listed, so a fixture added
// later is covered without anyone remembering to add it (§13.4: the fixture list
// IS the coverage list).
func buildBackedFixtureNames() []string {
	names := capture.FixtureNames()
	return slices.DeleteFunc(names, func(n string) bool { return n == capture.ContrastValidationFixture })
}

// capturedStateWant describes one fixture's CAPTURED state — the frame its tape
// screenshotted, not the frame its first Update produced.
//
// present/absent are asserted against the ANSI-stripped frame. absent is what
// distinguishes "the driver reached the captured state" from "the driver rendered
// something": a sessions fixture that shows the empty state, or a projects fixture
// still sitting on Sessions, is a frame nobody ever captured.
//
// page holds one of tui's exported page constants. Its declared type is `any`
// because internal/tui exports the CONSTANTS but not the type they belong to, so
// an external test cannot name it; nil means "do not assert the page", which the
// preview fixture needs — the preview page has no exported constant at all and is
// identified by its rendered chrome instead.
type capturedStateWant struct {
	fixture string
	page    any
	present []string
	absent  []string
}

// capturedStates is the per-fixture expectation table. Every registered
// tui.Build-backed fixture appears exactly once — asserted below, so a new
// fixture fails the table rather than passing unnoticed.
func capturedStates() []capturedStateWant {
	sessionRow := "agentic-workflows-code-based"
	return []capturedStateWant{
		{fixture: "sessions-flat", page: tui.PageSessions, present: []string{sessionRow}, absent: []string{"No sessions yet"}},
		// The one sessions fixture whose captured state IS the empty state: with
		// zero sessions the model default-routes to Projects, and the tape presses
		// `x` to land on the empty Sessions screen the reference shows.
		{fixture: "sessions-empty", page: tui.PageSessions, present: []string{"No sessions yet"}},
		{fixture: "sessions-by-project", page: tui.PageSessions, present: []string{sessionRow, "— by project"}, absent: []string{"No sessions yet"}},
		{fixture: "sessions-by-tag", page: tui.PageSessions, present: []string{sessionRow, "— by tag"}, absent: []string{"No sessions yet"}},
		{fixture: "sessions-paged", page: tui.PageSessions, present: []string{"session-00"}, absent: []string{"No sessions yet"}},
		{fixture: "sessions-inline-flash", page: tui.PageSessions, present: []string{"fab-flowx-explore", "closed externally"}, absent: []string{"No sessions yet"}},
		{fixture: "sessions-multi-select-active", page: tui.PageSessions, present: []string{sessionRow, "3 selected"}, absent: []string{"No sessions yet"}},
		{fixture: "sessions-unsupported-terminal", page: tui.PageSessions, present: []string{sessionRow, "unsupported terminal", "com.apple.Terminal"}, absent: []string{"No sessions yet"}},
		{fixture: "sessions-unsupported-null", page: tui.PageSessions, present: []string{sessionRow}, absent: []string{"No sessions yet", "unsupported terminal"}},
		{fixture: "sessions-multi-select-preflight-abort", page: tui.PageSessions, present: []string{sessionRow, "is gone", "nothing opened"}, absent: []string{"No sessions yet"}},
		{fixture: "sessions-burst-opening", page: tui.PageSessions, present: []string{sessionRow, "Opening 2/3"}, absent: []string{"No sessions yet"}},
		{fixture: "sessions-no-tags-signpost", page: tui.PageSessions, present: []string{"fab-flowx-explore", "No tags yet"}, absent: []string{"No sessions yet"}},
		// Reached by the `t` their tapes type: the §9.1 slide-over composited over
		// the Sessions list. The badges are what distinguishes the two — the pair's
		// two slot badges against the constant's single bare `●` — so each names its
		// own and rules out the other's, which is what stops one silently rendering
		// as the other.
		{fixture: "theme-panel-adaptive-pair", page: tui.PageSessions, present: []string{sessionRow, "Themes", "● light", "● dark"}, absent: []string{"No sessions yet"}},
		{fixture: "theme-panel-constant-previewing", page: tui.PageSessions, present: []string{sessionRow, "Themes", "●"}, absent: []string{"No sessions yet", "● light", "● dark"}},
		// §13.3's five remaining panel surfaces. Each names the ONE thing no other
		// panel frame carries, so a fixture that silently re-rendered a sibling's
		// screen fails here rather than reading as coverage of its own.
		//
		// The invalid row rules out the reason its BADGED row drops — `bad colour`
		// belongs to nord-lee, which carries `● dark` — while the unbadged row's
		// `bad syntax` must be present, so the pair states §9.5's priority as a
		// captured fact rather than as a rendering coincidence.
		{fixture: "theme-panel-invalid-row", page: tui.PageSessions, present: []string{sessionRow, "Themes", "⚠ bad syntax", "My Gorgeous Midnight Palett…", "● dark"}, absent: []string{"No sessions yet", "bad colour"}},
		{fixture: "theme-panel-dir-unreadable", page: tui.PageSessions, present: []string{sessionRow, "Themes", "⚠ dir unreadable", "solarized-lee", "● light", "● dark"}, absent: []string{"No sessions yet"}},
		// The narrow frame's captured state at THIS size is the adaptive pair's, by
		// construction: its subject is §9.8's width ladder, which is a function of
		// the terminal rather than of the fixture, so the degraded band is asserted
		// at its own size by TestPanelFixture_NarrowIsBetweenMinAndPreferred. What
		// belongs here is only that the driver opened the panel at all.
		{fixture: "theme-panel-narrow", page: tui.PageSessions, present: []string{sessionRow, "Themes", "● light", "● dark"}, absent: []string{"No sessions yet"}},
		{fixture: "theme-panel-paginated", page: tui.PageSessions, present: []string{sessionRow, "Themes", "vivid-01", "● light", "● dark"}, absent: []string{"No sessions yet"}},
		// Reached by the `x` then `t` its captureKeys type: the slide-over over the
		// Projects page, which every other panel fixture leaves unrendered.
		{fixture: "theme-panel-projects", page: tui.PageProjects, present: []string{"flow-v1-api", "Projects", "Themes", "● light", "● dark"}},
		// §13.3's message-slot surfaces, whose seeds raise §9.1's slot on the opened
		// panel. Each names its own contender AND rules out what must not be live
		// beside it — the confirm's whole subject is the SUBSTITUTED footer, so a
		// frame still advertising `set as dark` is exactly what to catch.
		{fixture: "theme-panel-confirm", page: tui.PageSessions, present: []string{sessionRow, "Themes", "clear constant nord?  y / n", "confirm", "cancel"}, absent: []string{"No sessions yet", "set as dark", "set as light"}},
		{fixture: "theme-panel-commit-failed", page: tui.PageSessions, present: []string{sessionRow, "Themes", "⚠ couldn't save theme", "● light", "● dark", "set as dark"}, absent: []string{"No sessions yet", "confirm", "cancel"}},
		// The minimum-height frame's captured state at THIS size is the failed-commit
		// frame's, by construction: its subject is §9.8's floor, which is a function of
		// the terminal rather than of the fixture, so the arithmetic is asserted at its
		// own size by TestPanelFixture_MinHeightMessageFrame. What belongs here is only
		// that the driver opened the panel with the message live.
		{fixture: "theme-panel-min-height-message", page: tui.PageSessions, present: []string{sessionRow, "Themes", "⚠ couldn't save theme", "● light", "● dark"}, absent: []string{"No sessions yet"}},
		// Reached by the `x` its tape types: the fixture opens on Sessions (the
		// production no-arg default) and the capture is of the Projects page.
		{fixture: "projects", page: tui.PageProjects, present: []string{"flow-v1-api", "Projects"}},
		{fixture: "projects-command-pending", page: tui.PageProjects, present: []string{"flow-v1-api", "Pick a project to run", "npm run dev"}},
		// Reached by the Space its tape presses: the §9 preview overlay over the
		// first (default-selected) session, with its canned scrollback.
		{fixture: "preview-screen", present: []string{"aviva-proxy-qNyfEO", "Window 1/1", "kubectl rollout"}},
		{fixture: "loading-screen", page: tui.PageLoading, present: []string{"Restoring sessions", "8", "12"}},
		{fixture: "loading-error", page: tui.PageLoading, present: []string{"✗", "Portal failed to set @portal-restoring marker"}},
	}
}

// darkBuiltinTheme loads the shipped dark built-in — the palette capturetool
// pins by default.
func darkBuiltinTheme(t *testing.T) theme.Theme {
	t.Helper()
	return builtinPalette(t, theme.DefaultDarkSlug)
}

// TestModelAt_ReachesCapturedState pins the harness driver: every registered
// tui.Build-backed fixture must arrive at the state its tape screenshotted,
// driven deterministically through Update with no tea program.
//
// This is what §13.4's coverage claim rests on. The guard enumerates the fixture
// set, so a fixture the driver leaves on the wrong screen is not an obviously
// missing assertion — it silently re-covers a screen some other fixture already
// covers, and the screen it was added for goes unguarded.
func TestModelAt_ReachesCapturedState(t *testing.T) {
	pinned := darkBuiltinTheme(t)

	t.Run("the table covers every build-backed fixture", func(t *testing.T) {
		listed := buildBackedFixtureNames()
		covered := make([]string, 0, len(capturedStates()))
		for _, want := range capturedStates() {
			covered = append(covered, want.fixture)
		}
		slices.Sort(covered)
		if !slices.Equal(listed, covered) {
			t.Errorf("captured-state table covers %v, want every build-backed fixture %v", covered, listed)
		}
	})

	for _, want := range capturedStates() {
		t.Run(want.fixture, func(t *testing.T) {
			fx, err := capture.FixtureByName(want.fixture)
			if err != nil {
				t.Fatalf("FixtureByName(%s): %v", want.fixture, err)
			}

			m := fx.ModelAt(pinned, harnessWidth, harnessHeight)
			if want.page != nil && any(m.ActivePage()) != want.page {
				t.Errorf("ActivePage() = %d, want %d — the driver did not reach the captured screen", m.ActivePage(), want.page)
			}

			frame := ansi.Strip(m.View().Content)
			for _, s := range want.present {
				if !strings.Contains(frame, s) {
					t.Errorf("captured frame is missing %q:\n%s", s, frame)
				}
			}
			for _, s := range want.absent {
				if strings.Contains(frame, s) {
					t.Errorf("captured frame carries %q, which the captured state must not show:\n%s", s, frame)
				}
			}
		})
	}
}

// lightBuiltinTheme loads the shipped light built-in, so the swap assertions have
// a second palette whose canvas provably differs from the dark one's.
func lightBuiltinTheme(t *testing.T) theme.Theme {
	t.Helper()
	return builtinPalette(t, theme.DefaultLightSlug)
}

// bgSeq is a token's rendered BACKGROUND SGR core (`48;2;R;G;B`). Styled output
// carries no hex, so a frame is searched for this rather than for the value the
// theme file declares.
func bgSeq(t *testing.T, tok theme.Token) string {
	t.Helper()
	return sgrParameterRun(t, lipgloss.NewStyle().Background(tok.Color()))
}

// swapPalettes returns the two palettes the swap assertions move between, with
// their canvases asserted distinct — the one property every canvas assertion here
// depends on.
func swapPalettes(t *testing.T) (a, b theme.Theme) {
	t.Helper()
	a, b = darkBuiltinTheme(t), lightBuiltinTheme(t)
	if a.Canvas.Value == b.Canvas.Value {
		t.Fatalf("the two palettes share a canvas (%s); a swap between them would be unobservable", a.Canvas.Value)
	}
	return a, b
}

// TestRenderSwapRender_ARenderPopulatesCachesBeforeSwap pins §13.4's "the A-render
// is not optional set-up" rule from the observable side: the before-frame must be
// a REAL frame under theme A.
//
// Three ways it could fail to be one, all checked: an empty string, the
// pre-resolution blank frame the first-paint gate holds (all spaces, no content,
// no canvas SGR — which a nomination shape other than the constant would produce),
// and a frame carrying the wrong palette. Any of them means the caches were never
// populated under A, and every post-swap assertion downstream would pass trivially.
func TestRenderSwapRender_ARenderPopulatesCachesBeforeSwap(t *testing.T) {
	a, b := swapPalettes(t)
	fx, err := capture.FixtureByName("sessions-flat")
	if err != nil {
		t.Fatalf("FixtureByName(sessions-flat): %v", err)
	}

	before, after := fx.RenderSwapRender(a, b, harnessWidth, harnessHeight)

	if before == "" {
		t.Fatal("the before-frame is empty; the A-render never happened, so nothing populated the caches")
	}
	visible := strings.TrimSpace(ansi.Strip(before))
	if visible == "" {
		t.Fatalf("the before-frame is the pre-resolution blank frame (all spaces, no content); the constant nomination must paint from frame one:\n%q", before)
	}
	if !strings.Contains(ansi.Strip(before), "agentic-workflows-code-based") {
		t.Errorf("the before-frame does not render the fixture's session rows, so it is not the fixture's captured state:\n%s", ansi.Strip(before))
	}

	aCanvas, bCanvas := bgSeq(t, a.Canvas), bgSeq(t, b.Canvas)
	if !strings.Contains(before, aCanvas) {
		t.Errorf("the before-frame does not carry theme A's canvas %q — it was not painted under A", aCanvas)
	}
	if strings.Contains(before, bCanvas) {
		t.Errorf("the before-frame already carries theme B's canvas %q — the A-render was not under A", bCanvas)
	}
	if !strings.Contains(after, bCanvas) {
		t.Errorf("the after-frame does not carry theme B's canvas %q — the swap did not take", bCanvas)
	}
}

// TestRenderSwapRender_MutatesASingleModel pins §13.4's central construction rule:
// the swap is a live mutation of ONE already-rendered model. Building two — one
// per theme — assigns every cached style correctly in each and passes green while
// live swap is broken, and the fixture harness's one-shot render is exactly the
// shape that produces that vacuous pass.
//
// It is checked twice over, because the behavioural half alone cannot decide it.
// A correct restyle makes a swapped model's frame identical to a freshly built
// one's — that identity IS what the guard proves — so no rendered output can
// distinguish one model from two. The behavioural half therefore pins the weaker,
// still worthwhile property (state established before the swap survives it), and
// the structural half pins the construction count outright.
func TestRenderSwapRender_MutatesASingleModel(t *testing.T) {
	a, b := swapPalettes(t)

	t.Run("state established before the swap survives it", func(t *testing.T) {
		// The projects fixture reaches its captured state through a keypress, so a
		// post-swap frame that lost the Projects page is a model that was rebuilt
		// rather than mutated.
		fx, err := capture.FixtureByName("projects")
		if err != nil {
			t.Fatalf("FixtureByName(projects): %v", err)
		}

		before, after := fx.RenderSwapRender(a, b, harnessWidth, harnessHeight)
		for _, tc := range []struct {
			name  string
			frame string
		}{{"before", before}, {"after", after}} {
			visible := ansi.Strip(tc.frame)
			if !strings.Contains(visible, "Projects 14") {
				t.Errorf("the %s-frame is not on the Projects page; the captured state did not survive:\n%s", tc.name, visible)
			}
		}
	})

	t.Run("exactly one model is constructed", func(t *testing.T) {
		body := harnessFuncBody(t, "RenderSwapRender")
		if got := countCalls(body, "ModelAt"); got != 1 {
			t.Errorf("RenderSwapRender makes %d ModelAt call(s), want exactly 1 — building one model per theme is §13.4's vacuous-pass shape", got)
		}
		if got := countCalls(body, "Build"); got != 0 {
			t.Errorf("RenderSwapRender makes %d tui.Build call(s), want 0 — the second frame comes from swapping the first model, never from a second one", got)
		}
		if got := countCalls(body, "ApplyTheme"); got != 1 {
			t.Errorf("RenderSwapRender makes %d ApplyTheme call(s), want exactly 1 — the swap goes through the production entry point", got)
		}
	})
}

// harnessFuncBody parses internal/capture's harness source and returns the named
// method's body, so the construction-count assertion above reads the real code
// rather than trusting a comment.
func harnessFuncBody(t *testing.T, name string) *ast.BlockStmt {
	t.Helper()
	const harnessFile = "harness.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, harnessFile, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", harnessFile, err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name && fn.Body != nil {
			return fn.Body
		}
	}
	t.Fatalf("%s declares no method named %s", harnessFile, name)
	return nil
}

// countCalls counts the calls to a method or function of the given name within
// the block, matching on the selector's final identifier so both `x.Name(…)` and
// a bare `Name(…)` are seen.
func countCalls(block *ast.BlockStmt, name string) int {
	count := 0
	ast.Inspect(block, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fn.Sel.Name == name {
				count++
			}
		case *ast.Ident:
			if fn.Name == name {
				count++
			}
		}
		return true
	})
	return count
}
