package cmd

// Tests in this file seed prefs.json / PORTAL_THEMES_DIR through t.Setenv and
// drive package-level cmd seams, so they MUST NOT use t.Parallel.
//
// The panel's constructor slots are asserted BEHAVIOURALLY, over models built
// through the production chokepoint: the open below proves the enumerator and
// the keys reach a rendered frame, and open_theme_commit_test.go carries the
// write half — the same wiring taking a keypress all the way to prefs.json, and
// back out of it on the next launch.

import (
	"go/ast"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// themeEnumeratorForTest builds the production adapter over a loader emitting
// into a capture sink, so the §12.3 `theme: enumerated` cadence is observable
// while the directory resolution stays the production one (themesDirPath).
func themeEnumeratorForTest(t *testing.T) (theme.Loader, *logtest.Sink) {
	t.Helper()
	sink := installMigrateCapture(t)
	return newThemeLoader(), sink
}

// pressThemeKeyOnModel drives a built model's live Update with `t` and returns
// the frame it then composes.
func pressThemeKeyOnModel(t *testing.T, m tui.Model) string {
	t.Helper()
	updated, _ := m.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	model, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	return model.View().Content
}

// themePanelHeaderCopy is §9.1's pinned panel header label, restated here rather
// than reached for: internal/tui holds it unexported, and a cmd-side test
// asserting the composed frame is asserting the user-visible copy anyway.
const themePanelHeaderCopy = "Themes"

// TestThemeEnumerator_ReadsOnlyWhenOpened pins the cadence half of §5.7/§5.8 on
// the PRODUCTION adapter: constructing it touches nothing, and each Open is one
// directory read emitting exactly one `theme: enumerated`.
//
// The directory is POPULATED and resolved through PORTAL_THEMES_DIR, so a
// construction-time sweep would be visible as a record and a drop-in row.
func TestThemeEnumerator_ReadsOnlyWhenOpened(t *testing.T) {
	dir := useThemesDir(t)
	writeThemeFile(t, dir, "sunset", "#101010")
	loader, sink := themeEnumeratorForTest(t)

	enumerator := newThemeEnumerator(loader)

	if got := themeEventCount(t, sink, "enumerated"); got != 0 {
		t.Errorf("constructing the adapter emitted %d `theme: enumerated` records, want 0 — the read is on the keypress (§5.7)", got)
	}

	const opens = 3
	for i := range opens {
		_, union := enumerator.Open(theme.RawKeys{})
		if !slugListed(union, "sunset") {
			t.Fatalf("open %d produced no row for the drop-in (rows: %v) — the adapter must read the resolved themes directory", i+1, unionSlugs(union))
		}
	}

	if got := themeEventCount(t, sink, "enumerated"); got != opens {
		t.Errorf("%d opens emitted %d `theme: enumerated` records, want %d (§12.3 — per event, no dedup)", opens, got, opens)
	}
}

// TestThemeEnumerator_ReassembleDoesNoIO pins the second method's contract at the
// production boundary: §9.2's post-commit recompute and §5.8's `Esc`
// re-resolution both re-derive from the RETAINED enumeration with no fresh read,
// so a directory emptied underneath a live panel changes nothing.
func TestThemeEnumerator_ReassembleDoesNoIO(t *testing.T) {
	dir := useThemesDir(t)
	writeThemeFile(t, dir, "sunset", "#101010")
	loader, sink := themeEnumeratorForTest(t)
	enumerator := newThemeEnumerator(loader)

	enumeration, _ := enumerator.Open(theme.RawKeys{})
	before := themeEventCount(t, sink, "enumerated")

	union := enumerator.Reassemble(enumeration, theme.RawKeys{Theme: "sunset"})

	if !slugListed(union, "sunset") {
		t.Errorf("Reassemble dropped the retained drop-in (rows: %v)", unionSlugs(union))
	}
	if got := themeEventCount(t, sink, "enumerated"); got != before {
		t.Errorf("Reassemble emitted %d further `theme: enumerated` records, want 0 — it performs no I/O", got-before)
	}
}

// TestThemeEnumerator_SharesTheConstructionReadsDedupScope pins §5.5's
// requirement on the loader this adapter is given: ONE loader per launch is ONE
// dedup scope per launch.
//
// The construction-time by-name read and the panel's enumeration hit the same
// condition — an unusable themes directory — and a Loader owns the `theme`
// component's per-process dedup state, so a panel handed a SECOND loader would
// WARN the user twice about one misconfiguration on a single launch. The negative
// control drives exactly that, or the positive half could pass for want of a
// second emission being possible at all.
func TestThemeEnumerator_SharesTheConstructionReadsDedupScope(t *testing.T) {
	// A DROP-IN slug, so the by-name read must consult the themes directory and
	// meet §5.5's unusable verdict; a built-in would resolve out of the embedded
	// set and never look.
	const dropIn = "a-drop-in"

	openAfterConstruction := func(t *testing.T, panelLoader func(theme.Loader) theme.Loader) int {
		t.Helper()
		poisonThemesDir(t)
		sink := installMigrateCapture(t)
		construction := newThemeLoader()

		if _, _, err := themeResolution(prefs.ThemeKeys{Theme: dropIn}, construction); err != nil {
			t.Fatalf("construction-time resolution: %v", err)
		}
		if got := themeEventCount(t, sink, "directory unusable"); got != 1 {
			t.Fatalf("precondition: the construction read emitted %d `theme: directory unusable` records, want 1", got)
		}

		newThemeEnumerator(panelLoader(construction)).Open(theme.RawKeys{Theme: dropIn})
		return themeEventCount(t, sink, "directory unusable")
	}

	t.Run("the construction loader dedups the panel's repeat", func(t *testing.T) {
		got := openAfterConstruction(t, func(l theme.Loader) theme.Loader { return l })
		if got != 1 {
			t.Errorf("`theme: directory unusable` was emitted %d times, want 1 — the panel must share the construction read's dedup scope (§5.5)", got)
		}
	})

	t.Run("a second loader would not", func(t *testing.T) {
		got := openAfterConstruction(t, func(theme.Loader) theme.Loader { return newThemeLoader() })
		if got != 2 {
			t.Fatalf("a fresh loader emitted %d records, want 2 — the control does not demonstrate the duplicate the shared loader prevents", got)
		}
	})
}

// TestOpenTUI_BuildsOneThemeLoader guards the sharing the test above depends on,
// at the one site that decides it: openTUI must build no more than one theme
// loader, and must hand the panel's enumerator a loader already bound in the
// function rather than one constructed in the argument. Building a second is
// silent — everything still works, the user just gets every theme WARN twice per
// launch — and openTUI ends in a running Bubble Tea program, so only a source
// guard reaches it.
//
// BOTH are asserted, because the construction count does not reach the second on
// its own: `newThemeEnumerator(newThemeLoader())` keeps every constructor at one
// call and still hands the panel a dedup scope of its own.
func TestOpenTUI_BuildsOneThemeLoader(t *testing.T) {
	fn := funcDeclForTest(t, "open.go", "openTUI")

	for _, constructor := range []string{"buildThemeLoader", "newThemeLoader"} {
		if got := callCount(fn, constructor); got > 1 {
			t.Errorf("openTUI calls %s %d times, want at most 1 — one loader per launch is one dedup scope per launch (§5.5)", constructor, got)
		}
	}

	assertEnumeratorTakesBoundLoader(t, fn)
}

// assertEnumeratorTakesBoundLoader fails unless every newThemeEnumerator call
// inside n takes a bare identifier for its argument.
//
// It asserts the argument's SHAPE, not its identity: a call expression in that
// position is a loader built for the panel alone, whether it is the package-local
// constructor or an inlined theme.NewLoader(…) — and the inlined form additionally
// slips the `theme` component logger, nothing forcing an inline construction to
// pass themeLogger.
func assertEnumeratorTakesBoundLoader(t *testing.T, n ast.Node) {
	t.Helper()
	ast.Inspect(n, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, isIdent := call.Fun.(*ast.Ident); !isIdent || ident.Name != "newThemeEnumerator" || len(call.Args) != 1 {
			return true
		}
		if _, isIdent := call.Args[0].(*ast.Ident); !isIdent {
			t.Errorf("openTUI hands newThemeEnumerator a freshly-constructed loader; it must share the construction-time instance (§5.5)")
		}
		return true
	})
}

// callCount counts calls to the named package-local function inside n.
func callCount(n ast.Node, name string) int {
	count := 0
	ast.Inspect(n, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, isIdent := call.Fun.(*ast.Ident); isIdent && ident.Name == name {
			count++
		}
		return true
	})
	return count
}

// TestThemePanelOpen_WiredThroughBuildTUIModel: it threads the constructor slots
// end to end.
//
// Phase 5 task 5-7 deliberately left the control-stripped raw keys unthreaded —
// computed, then discarded — because §8.4's slot had no consumer until the panel
// existed. This asserts the whole chain from the cmd config through tui.Build to a
// rendered frame: `t` on the built model paints the §9.1 slide-over, listing a
// drop-in that only a real directory read could have produced and marking the
// persisted slug the adapter's own re-resolution names.
//
// The `●` is what proves the third method is wired: nothing is injected for it,
// so the marker exists only if the panel reached themeEnumerator.Resolve and got
// the constant back.
func TestThemePanelOpen_WiredThroughBuildTUIModel(t *testing.T) {
	dir := useThemesDir(t)
	writeThemeFile(t, dir, "sunset", "#101010")
	loader, _ := themeEnumeratorForTest(t)

	cfg := defaultTestTUIConfig()
	cfg.themeEnumerator = newThemeEnumerator(loader)
	cfg.themeKeys = theme.RawKeys{Theme: "sunset"}

	view := pressThemeKeyOnModel(t, buildTUIModel(cfg, "", nil))

	for _, want := range []string{themePanelHeaderCopy, "sunset", "●"} {
		if !strings.Contains(view, want) {
			t.Errorf("the frame after `t` does not contain %q:\n%s", want, view)
		}
	}
}

// TestThemePanelOpen_ExecPathUntouched: it does no theme work on the exec path.
//
// §12.3 records the win this feature is careful to keep: `portal open <target>`
// constructs no TUI, so the loader never runs there — and the panel seam added
// here must not change that. The themes directory is poisoned (mode 0000), which
// makes any read of it LOUD: §5.5 gives an unusable directory a
// `theme: directory unusable` WARN where an absent one is silent, so "emitted
// nothing" is not vacuous.
func TestThemePanelOpen_ExecPathUntouched(t *testing.T) {
	poisonThemesDir(t)
	// A DROP-IN slug, so resolving it must consult the themes directory — a
	// built-in would resolve out of the embedded set and never touch the poison.
	setPrefsFile(t, `{"theme":"a-drop-in"}`)

	// The fixture has to be LOUD or the zero-record assertion below could pass for
	// want of anything observable. Opening the panel's own seam against the poison
	// proves the records exist to be seen — and proves it through the seam THIS
	// task adds rather than through the construction-time read alone.
	loud := installMigrateCapture(t)
	newThemeEnumerator(newThemeLoader()).Open(theme.RawKeys{Theme: "a-drop-in"})
	if len(themeEvents(t, loud)) == 0 {
		t.Fatal("the panel seam emitted no theme record against the poisoned directory; the zero-record assertion below would be vacuous")
	}

	sink := installMigrateCapture(t)

	if got := execOpenSession(t, "api-x7Kd9a"); got != "api-x7Kd9a" {
		t.Fatalf("open attached %q, want the session it resolved — the exec path must have run", got)
	}

	assertThemeEvents(t, sink)
}

// themeEventCount counts `theme` component records carrying the given message.
func themeEventCount(t *testing.T, sink *logtest.Sink, msg string) int {
	t.Helper()
	count := 0
	for _, rec := range sink.Records() {
		if rec.Msg == msg && rec.HasAttr("component") && rec.AttrString(t, "component") == "theme" {
			count++
		}
	}
	return count
}

// slugListed reports whether the union holds a row for slug.
func slugListed(union theme.Union, slug string) bool {
	for _, row := range union.Rows {
		if row.Slug == slug {
			return true
		}
	}
	return false
}

// unionSlugs lists the union's row labels, for failure messages.
func unionSlugs(union theme.Union) []string {
	labels := make([]string, 0, len(union.Rows))
	for _, row := range union.Rows {
		labels = append(labels, row.Label())
	}
	return labels
}
