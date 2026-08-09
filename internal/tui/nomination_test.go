package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// testBuiltinPair is the two embedded built-ins as an ADAPTIVE nomination — the
// shape of §8.3's shipped default, and what every test needing a gate to select
// between two distinguishable palettes injects.
func testBuiltinPair(t *testing.T) theme.Nomination {
	t.Helper()
	return theme.AdaptivePair(
		theme.MemberLight.Palette(testLightTheme(t)),
		theme.MemberDark.Palette(testDarkTheme(t)),
	)
}

// memberForSlot is the pair member whose palette a setting slot nominates — the
// test-side counterpart of theme.Member.Slot, for the stubs and assertions that
// hold a slot and need the member the nomination is selected by.
func memberForSlot(slot theme.Slot) theme.Member {
	if slot == theme.SlotLight {
		return theme.MemberLight
	}
	return theme.MemberDark
}

// TestDeps_HasNoAppearanceField pins §8.8's removal: the light/dark appearance
// injection is GONE, not left alongside the nomination. A dead option is a second
// injection path — one a caller could still reach and one that would silently
// contradict the theme it was handed — so both halves are asserted: the Deps
// field by reflection, and WithAppearance by a source guard (an unreferenced
// exported option would compile forever without one).
func TestDeps_HasNoAppearanceField(t *testing.T) {
	depsType := reflect.TypeFor[Deps]()
	if _, found := depsType.FieldByName("Appearance"); found {
		t.Errorf("Deps still carries an Appearance field; §8.8 removes the appearance injection, it is not kept alongside Deps.Theme")
	}
	if _, found := depsType.FieldByName("Theme"); !found {
		t.Errorf("Deps carries no Theme field; the loaded nomination is what replaces the appearance injection")
	}

	for _, name := range exportedFuncsInPackage(t) {
		if name == "WithAppearance" {
			t.Errorf("WithAppearance is still declared; §8.8 removes the option rather than leaving a second, dead injection path")
		}
	}
}

// TestNew_SeedsTheDarkBuiltinWhenNoNominationIsGiven pins the seed that SURVIVES
// this task. A model constructed without Build (and so without a nomination) must
// still be themed: an empty Theme resolves through lipgloss.Color("")'s no-colour
// sentinel, which is a silent colourless render — no compile error, no failing
// assertion anywhere else — so the render half is asserted too.
func TestNew_SeedsTheDarkBuiltinWhenNoNominationIsGiven(t *testing.T) {
	m := New(fakeLister{})

	if got, want := m.themeState.active, testDarkTheme(t); got != want {
		t.Errorf("activeTheme = %s, want the dark built-in %s", themeLabel(got), themeLabel(want))
	}
	assertActiveTheme(t, m, testDarkTheme(t).Canvas.Value)
}

// TestNomination_ConstantSkipsDetectionAndWait pins §8.8's real startup win: a
// constant needs no detection, so its gate is resolved AND unarmable at
// construction, Init issues no timeout tick, and the first frame paints the
// constant's canvas. The OSC 11 query is still issued — a constant skips the
// GATE, never the QUERY (§8.8 / §9.3).
func TestNomination_ConstantSkipsDetectionAndWait(t *testing.T) {
	light := testLightTheme(t)
	m := detectModel(t, theme.ConstantNomination(light))

	if !m.modeResolved() {
		t.Fatalf("a constant nomination left the first-paint gate open; want resolved at construction (no detection, no wait)")
	}
	// Unarmable, not merely un-armed: Build already called arm(). Calling it again
	// must stay a no-op, or a later arm site would re-open a window the constant
	// has no answer for.
	m.armAppearanceDetection()
	if !m.modeResolved() {
		t.Errorf("arming re-opened a constant's gate; want it unarmable")
	}

	assertNoTimeoutTick(t, m)
	assertBackgroundQueryIssued(t, m)
	assertActiveTheme(t, m, light.Canvas.Value)
}

// TestNomination_AdaptiveArmsTheGate pins the other half of §8.8: a pair carries
// no provisional active member, so the gate is armed and NOTHING is painted until
// it resolves.
func TestNomination_AdaptiveArmsTheGate(t *testing.T) {
	m := detectModel(t, testBuiltinPair(t))

	if m.modeResolved() {
		t.Fatalf("an adaptive pair resolved at construction; want the detect-or-timeout gate open")
	}
	assertBlankFrame(t, m)
}

// TestNomination_GateSelectsMember pins that the gate SELECTS between values
// already in hand (§8.4): the dark reply takes the dark member, the light reply
// the light member, and the no-answer timeout the dark member (§8.8's fallback).
func TestNomination_GateSelectsMember(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  tea.Msg
		want func(*testing.T) theme.Theme
	}{
		{"a dark OSC 11 reply selects the dark member", darkBg, testDarkTheme},
		{"a light OSC 11 reply selects the light member", lightBg, testLightTheme},
		{"the no-answer timeout selects the dark member", appearanceTimeoutMsg{}, testDarkTheme},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := detectModel(t, testBuiltinPair(t))

			updated, _ := m.Update(tc.msg)

			assertActiveTheme(t, updated.(Model), tc.want(t).Canvas.Value)
		})
	}
}

// TestGate_LateReplyCapturesBackgroundButNeverReThemes pins §8.8's resolve-once
// rule at its sharpest point: the timeout has already resolved the gate when the
// OSC 11 reply lands. The reply is still CONSUMED — restore.go needs the original
// background, and §9.3's mid-session conversion needs the answer in hand — but it
// must not re-theme, because under split a late flip swaps a whole named theme
// (canvas and every accent) a second after the user began reading the picker.
func TestGate_LateReplyCapturesBackgroundButNeverReThemes(t *testing.T) {
	m := detectModel(t, testBuiltinPair(t))

	timedOut, _ := m.Update(appearanceTimeoutMsg{})
	resolved := timedOut.(Model)
	assertActiveTheme(t, resolved, testDarkTheme(t).Canvas.Value)

	late, _ := resolved.Update(lightBg)
	after := late.(Model)

	if got := after.OriginalBackground(); got != "#e1e2e7" {
		t.Errorf("OriginalBackground() = %q after a late reply, want %q (the reply is still consumed)", got, "#e1e2e7")
	}
	if !after.themeState.bgReplyArrived {
		t.Errorf("bgReplyArrived = false after a late reply, want true (the arrival is retained for later classification)")
	}
	assertActiveTheme(t, after, testDarkTheme(t).Canvas.Value)
}

// TestGate_QueryIssuedRegardlessOfSettingShape pins that the OSC 11 query is
// issued from Init under EVERY shape (§8.8). A constant path that skips it breaks
// restore.go's original-background capture and §9.3's conversion, which relies on
// the answer already being in hand rather than on a second query and race.
func TestGate_QueryIssuedRegardlessOfSettingShape(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*testing.T) Model
	}{
		{"constant", func(t *testing.T) Model {
			return Build(Deps{Lister: fakeLister{}, Theme: theme.ConstantNomination(testDarkTheme(t))})
		}},
		{"adaptive pair", func(t *testing.T) Model {
			return Build(Deps{Lister: fakeLister{}, Theme: testBuiltinPair(t)})
		}},
		{"NO_COLOR", func(t *testing.T) Model {
			return Build(Deps{Lister: fakeLister{}, Theme: testBuiltinPair(t), NoColor: true})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertBackgroundQueryIssued(t, tc.build(t))
		})
	}
}

// TestGate_ConstantRetainsReplyWithoutClassifying pins the edge a constant makes
// reachable: the reply arrives on a launch that asked no light/dark question. The
// background and the fact a reply came are retained (task 9-6 classifies them),
// but NO answer is derived from it — a constant's pre-resolved gate carries the
// standing dark fallback, which must never be read as "the terminal is dark".
func TestGate_ConstantRetainsReplyWithoutClassifying(t *testing.T) {
	dark := testDarkTheme(t)
	m := detectModel(t, theme.ConstantNomination(dark))

	// A LIGHT terminal: the one case where a derived answer would be visible.
	updated, _ := m.Update(lightBg)
	after := updated.(Model)

	if got := after.OriginalBackground(); got != "#e1e2e7" {
		t.Errorf("OriginalBackground() = %q, want %q (the reply is retained for restore-on-exit)", got, "#e1e2e7")
	}
	if !after.themeState.bgReplyArrived {
		t.Errorf("bgReplyArrived = false, want true (a reply did arrive, whatever it said)")
	}
	if after.themeState.canvasMode != appearanceDarkCanvas {
		t.Errorf("canvasMode = %v, want the standing dark fallback — a constant derives no light/dark answer from the reply", after.themeState.canvasMode)
	}
	assertActiveTheme(t, after, dark.Canvas.Value)
}

// TestNoColor_LoadsBothAndSelectsDark pins §9.10: under NO_COLOR the theme
// machinery runs unchanged below the render layer. The gate is skipped, BOTH
// members are still loaded and held (a commit made in that session must have
// something in hand to persist against), and the standing dark fallback selects
// the active member.
func TestNoColor_LoadsBothAndSelectsDark(t *testing.T) {
	m := Build(Deps{Lister: fakeLister{}, Theme: testBuiltinPair(t), NoColor: true})

	if !m.modeResolved() {
		t.Errorf("colourless model is unresolved; want the gate skipped (no canvas to select)")
	}
	if got, want := m.themeState.active, testDarkTheme(t); got != want {
		t.Errorf("activeTheme = %s, want the dark member %s (the standing no-answer fallback)", themeLabel(got), themeLabel(want))
	}
	if got, want := m.themeState.nomination.Select(theme.MemberLight), testLightTheme(t); got != want {
		t.Errorf("the light member is no longer held (Select(MemberLight) = %s, want %s); NO_COLOR must not skip loading either nomination", themeLabel(got), themeLabel(want))
	}
}

// TestConstruction_ReadsNoThemesDirectory pins §8.4's construction contract from
// the one side internal/tui owns: it is HANDED a loaded nomination, so it reads
// no themes directory, no prefs theme keys and enumerates nothing.
//
// A source guard rather than a behavioural one, because "did not read" has no
// observable trace: the package makes no filesystem read call and never reaches
// the loader's directory enumeration at all, which is a property of the sources
// and is exactly what a future construction-time read would break.
func TestConstruction_ReadsNoThemesDirectory(t *testing.T) {
	banned := map[string]string{
		"os.ReadDir":       "a directory read",
		"os.ReadFile":      "a file read",
		"os.Open":          "a file open",
		"os.OpenFile":      "a file open",
		"os.Stat":          "a filesystem probe",
		"os.Lstat":         "a filesystem probe",
		"os.Getenv":        "a config-environment read",
		"os.LookupEnv":     "a config-environment read",
		"filepath.Walk":    "a directory walk",
		"filepath.WalkDir": "a directory walk",
	}

	for file, calls := range packageCalls(t) {
		for _, call := range calls {
			if what, bad := banned[call]; bad {
				t.Errorf("%s calls %s (%s); TUI construction takes a LOADED nomination and must read nothing", file, call, what)
			}
			if strings.HasSuffix(call, ".Enumerate") {
				t.Errorf("%s calls %s; construction enumerates nothing — the themes directory is read only by the §9 panel", file, call)
			}
		}
	}
}

// assertBackgroundQueryIssued drains Init's batched cmds and asserts one of them
// IS the tea.RequestBackgroundColor query.
//
// The cmd emits an UNEXPORTED internal request marker (the program runtime, not
// the cmd, turns it into the public tea.BackgroundColorMsg), so the match is by
// type against a reference taken from tea.RequestBackgroundColor itself.
func assertBackgroundQueryIssued(t *testing.T, m Model) {
	t.Helper()
	wantType := reflect.TypeOf(tea.Cmd(tea.RequestBackgroundColor)())
	for _, msg := range initCmds(t, m.Init()) {
		if reflect.TypeOf(msg) == wantType {
			return
		}
	}
	t.Errorf("Init issued no OSC 11 background query (no %v produced); restore-on-exit and §9.3's conversion both need the reply", wantType)
}

// exportedFuncsInPackage returns every exported top-level function declared in
// the package's production sources.
func exportedFuncsInPackage(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, file := range parsePackageFilesByName(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.IsExported() {
				names = append(names, fn.Name.Name)
			}
		}
	}
	return names
}

// packageCalls maps each production source file to the qualified call
// expressions it makes (`pkg.Func`, `x.Method`), for the source guards above.
func packageCalls(t *testing.T) map[string][]string {
	t.Helper()
	calls := map[string][]string{}
	for name, file := range parsePackageFilesByName(t) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			calls[name] = append(calls[name], ident.Name+"."+sel.Sel.Name)
			return true
		})
	}
	return calls
}

// parsePackageFilesByName parses the package's production sources, keyed by
// filename. go test runs in the package's source directory, so the relative walk
// resolves wherever the suite was invoked from.
func parsePackageFilesByName(t *testing.T) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files[name] = parsed
	}
	return files
}
