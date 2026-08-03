package cmd

// Tests in this file seed prefs.json / PORTAL_THEMES_DIR through t.Setenv and
// drive package-level cmd seams, so they MUST NOT use t.Parallel.

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tui"
)

// darkBackgroundReply / lightBackgroundReply are deterministic OSC 11 answers —
// the message the terminal's reply arrives as. The dark value is near-black
// (luminance < 0.5), the light value near-white, so the gate's classification is
// unambiguous either way.
var (
	darkBackgroundReply  = tea.BackgroundColorMsg{Color: color.RGBA{R: 0x0b, G: 0x0c, B: 0x14, A: 0xff}}
	lightBackgroundReply = tea.BackgroundColorMsg{Color: color.RGBA{R: 0xe1, G: 0xe2, B: 0xe7, A: 0xff}}
	// otherBackgroundReply is a dark background that is NO built-in's canvas — the
	// ordinary terminal whose own colour must be set back on exit.
	otherBackgroundReply = tea.BackgroundColorMsg{Color: color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}}
)

// nordSlug is the third shipped built-in — the one theme in the embedded set
// that is NEITHER shipped default, which is what makes it the honest fixture for
// "the user nominated something". Asserting against tokyo-night would pass
// identically if the nomination were ignored and the fallback rendered.
const nordSlug = "nord"

// themeNominationForTest runs the PRODUCTION construction-time resolution —
// prefs.json's three keys → §8.2's setting → §8.4's per-slot load — against
// whatever prefs file the test seeded, and fails on §7.6's fatal (which
// TestOpenTUI_FatalBeforeModelConstruction drives deliberately instead).
func themeNominationForTest(t *testing.T) theme.Nomination {
	t.Helper()

	load, err := loadPrefsStore()
	if err != nil {
		t.Fatalf("load prefs store: %v", err)
	}
	nomination, err := themeNomination(load.Keys, newThemeLoader())
	if err != nil {
		t.Fatalf("resolve the persisted theme setting: %v", err)
	}
	return nomination
}

// modelForNomination builds the real model through the production chokepoint
// (buildTUIModel → tui.Build) carrying the given nomination, so what is asserted
// is what the picker paints rather than what the resolution returned.
func modelForNomination(n theme.Nomination) tui.Model {
	cfg := defaultTestTUIConfig()
	cfg.theme = n
	return buildTUIModel(cfg, "", nil)
}

// assertPaintedCanvas asserts the model's first frame paints want as the owned
// canvas — the OSC 11 background View carries. A nil want is the un-resolved
// adaptive gate, which paints nothing at all.
func assertPaintedCanvas(t *testing.T, m tui.Model, want color.Color) {
	t.Helper()
	if got := m.View().BackgroundColor; got != want {
		t.Errorf("painted canvas = %v, want %v", got, want)
	}
}

// TestConstruction_PersistedConstantSkipsTheGate pins §8.8's real startup win,
// now reachable from prefs.json rather than only from a legacy appearance pin: a
// persisted CONSTANT is loaded at construction and painted from frame one, with
// the light/dark gate never consulted.
//
// The reply half is what makes "never consulted" observable from here: a light
// OSC 11 answer landing on a constant must change nothing, because a constant
// nomination has no member for an answer to select.
func TestConstruction_PersistedConstantSkipsTheGate(t *testing.T) {
	setPrefsFile(t, `{"theme":"`+nordSlug+`"}`)
	nord := builtinThemeForTest(t, nordSlug)

	nomination := themeNominationForTest(t)
	assertConstant(t, nomination, nord)

	m := modelForNomination(nomination)
	assertPaintedCanvas(t, m, nord.Canvas.Color())

	after, _ := m.Update(lightBackgroundReply)
	assertPaintedCanvas(t, after.(tui.Model), nord.Canvas.Color())
}

// TestConstruction_PersistedPairSelectsByGate pins the other half of §8.8 from
// prefs: a persisted PAIR loads both palettes and paints nothing until the gate
// answers, and the answer only SELECTS between values already in hand.
//
// `theme_dark` alone is the fixture on purpose — §8.3 makes "nothing set" and
// "pair nominated" the same state, so an unset `theme_light` is not a partial
// pair but a slot holding the shipped default, and the light reply proves it
// resolved to a real palette rather than to nothing.
func TestConstruction_PersistedPairSelectsByGate(t *testing.T) {
	setPrefsFile(t, `{"theme_dark":"`+nordSlug+`"}`)
	nord := builtinThemeForTest(t, nordSlug)
	day := builtinThemeForTest(t, theme.DefaultLightSlug)

	nomination := themeNominationForTest(t)
	assertPair(t, nomination, day, nord)

	// No member is active until the gate resolves, so the first frame paints
	// nothing at all — the no-paint-then-flip property.
	assertPaintedCanvas(t, modelForNomination(nomination), nil)

	for _, tc := range []struct {
		name    string
		resolve func(*testing.T, tui.Model) tui.Model
		want    theme.Theme
	}{
		{
			name:    "a dark reply selects the nominated dark member",
			resolve: func(t *testing.T, m tui.Model) tui.Model { return update(t, m, darkBackgroundReply) },
			want:    nord,
		},
		{
			name:    "a light reply selects the shipped light default",
			resolve: func(t *testing.T, m tui.Model) tui.Model { return update(t, m, lightBackgroundReply) },
			want:    day,
		},
		{
			name:    "no answer at all selects the dark member",
			resolve: resolveByTimeout,
			want:    nord,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolved := tc.resolve(t, modelForNomination(nomination))

			assertPaintedCanvas(t, resolved, tc.want.Canvas.Color())
		})
	}
}

// TestConstruction_LateReplyNeverReThemes pins §8.8's resolve-once rule at its
// sharpest point, now that the members are two DIFFERENT NAMED THEMES sourced
// from prefs: the timeout has already selected the dark member when the reply
// lands. The reply is still consumed — restore.go needs the original background
// for the exit-time set-back — but a late flip would swap the canvas and every
// accent a second after the user began reading the picker.
func TestConstruction_LateReplyNeverReThemes(t *testing.T) {
	setPrefsFile(t, `{"theme_dark":"`+nordSlug+`"}`)
	nord := builtinThemeForTest(t, nordSlug)

	resolved := resolveByTimeout(t, modelForNomination(themeNominationForTest(t)))
	assertPaintedCanvas(t, resolved, nord.Canvas.Color())

	after := update(t, resolved, lightBackgroundReply)

	assertPaintedCanvas(t, after, nord.Canvas.Color())
	if got, want := after.OriginalBackground(), "#e1e2e7"; got != want {
		t.Errorf("OriginalBackground() = %q after a late reply, want %q — the reply is still consumed for restore-on-exit", got, want)
	}
}

// TestConstruction_ConstantWinsOverStaleSlots pins §8.2's tiebreak against a
// hand-edited file carrying all three keys: a non-empty `theme` wins and THE
// SLOTS ARE NOT READ AT ALL.
//
// Both slot values are unusable — one is illegal by charset, one names nothing —
// so a read of either would be loud: an illegal slug or a missing theme would
// each raise a `theme: fallback applied` WARN. Exactly one `loaded` line and no
// warning at all is what "never read" looks like from the log, and it is also why
// a broken slot value cannot fail the launch.
func TestConstruction_ConstantWinsOverStaleSlots(t *testing.T) {
	setPrefsFile(t, `{"theme":"`+nordSlug+`","theme_light":"../evil","theme_dark":"no-such-theme"}`)
	nord := builtinThemeForTest(t, nordSlug)
	sink := installMigrateCapture(t)

	nomination := themeNominationForTest(t)

	assertConstant(t, nomination, nord)
	assertPaintedCanvas(t, modelForNomination(nomination), nord.Canvas.Color())
	assertThemeEvents(t, sink, "INFO loaded slug="+nordSlug)
}

// TestConstruction_UnloadableNominationFallsBackWithoutWriting pins §8.5's
// fallback and §6.3's keep-the-persisted-name rule together: a typo'd slug
// renders the mode-matched shipped default, says so once in the log, and leaves
// prefs.json untouched.
//
// The byte comparison is the point of the second half. Persisting the fallback
// would turn a transient failure into a destructive one — fixing the theme file
// would no longer restore the theme, because the name would already be gone — at
// the moment the user is least able to tell what happened.
func TestConstruction_UnloadableNominationFallsBackWithoutWriting(t *testing.T) {
	const content = `{"session_list_mode":"by-tag","theme":"no-such-theme"}`
	setPrefsFile(t, content)
	sink := installMigrateCapture(t)

	nomination := themeNominationForTest(t)

	assertConstant(t, nomination, builtinThemeForTest(t, theme.DefaultDarkSlug))
	assertThemeEvents(t, sink,
		"WARN fallback applied slug=no-such-theme reason=not found",
		"INFO loaded slug="+theme.DefaultDarkSlug,
	)

	after, err := os.ReadFile(os.Getenv("PORTAL_PREFS_FILE"))
	if err != nil {
		t.Fatalf("read prefs file back: %v", err)
	}
	if string(after) != content {
		t.Errorf("prefs.json changed:\n got %s\nwant %s\n(falling back never overwrites the persisted name)", after, content)
	}
}

// TestConstruction_EmitsLoadedPerNomination pins §12.3's cadence: ONE
// `theme: loaded` per NOMINATION — one under a constant, two under a pair, in
// light-then-dark order — never one combined line.
//
// The slot attr is part of the assertion because it is what makes a pair's two
// lines tell each other apart, and its ABSENCE under a constant is equally
// deliberate: the attr names which half of an adaptive pair a line is about, and
// a constant setting has no halves.
func TestConstruction_EmitsLoadedPerNomination(t *testing.T) {
	t.Run("a constant emits one line carrying no slot", func(t *testing.T) {
		setPrefsFile(t, `{"theme":"`+nordSlug+`"}`)
		sink := installMigrateCapture(t)

		themeNominationForTest(t)

		assertThemeEvents(t, sink, "INFO loaded slug="+nordSlug)
	})

	t.Run("a pair emits one line per slot, light then dark", func(t *testing.T) {
		setPrefsFile(t, `{"theme_dark":"`+nordSlug+`"}`)
		sink := installMigrateCapture(t)

		themeNominationForTest(t)

		assertThemeEvents(t, sink,
			"INFO loaded slug="+theme.DefaultLightSlug+" slot=light",
			"INFO loaded slug="+nordSlug+" slot=dark",
		)
	})
}

// TestConstruction_ReadBudget pins §8.4's cold-path cost from prefs: ONE file
// read for a constant, TWO for a pair, and NO directory listing on any path.
//
// The budget is measured three ways, because "how many files were read" has no
// counter to inspect:
//
//   - WHICH palette loaded says which file was read. The directory carries a
//     `nord.theme` whose canvas differs from the embedded Nord's, so a built-in
//     nomination that reached the directory would paint a visibly different
//     canvas (§5.4's no-shadowing guarantee on the path that matters).
//   - HOW MANY `theme: loaded` lines were emitted says how many slots loaded, and
//     the absence of any other event says nothing else was read or judged.
//   - The themes directory is mode 0111 — searchable but NOT listable — so any
//     ReadDir on this path fails loudly instead of silently paying §5.7's startup
//     scan. Reading a file BY NAME still works, which is exactly the asymmetry
//     lazy discovery relies on.
func TestConstruction_ReadBudget(t *testing.T) {
	const (
		shadowCanvas = "#111213"
		aloneCanvas  = "#141516"
		lightCanvas  = "#171819"
		darkCanvas   = "#1A1B1C"
	)
	seedUnlistableThemesDir(t, map[string]string{
		nordSlug:     shadowCanvas,
		"drop-alone": aloneCanvas,
		"drop-light": lightCanvas,
		"drop-dark":  darkCanvas,
	})

	t.Run("a built-in constant reads the directory zero times", func(t *testing.T) {
		setPrefsFile(t, `{"theme":"`+nordSlug+`"}`)
		sink := installMigrateCapture(t)

		nomination := themeNominationForTest(t)

		assertConstant(t, nomination, builtinThemeForTest(t, nordSlug))
		assertThemeEvents(t, sink, "INFO loaded slug="+nordSlug)
	})

	t.Run("a drop-in constant reads exactly its own file", func(t *testing.T) {
		setPrefsFile(t, `{"theme":"drop-alone"}`)
		sink := installMigrateCapture(t)

		nomination := themeNominationForTest(t)

		assertCanvasValue(t, nomination.Constant(), aloneCanvas)
		assertThemeEvents(t, sink, "INFO loaded slug=drop-alone")
	})

	t.Run("a drop-in pair reads exactly two files", func(t *testing.T) {
		setPrefsFile(t, `{"theme_light":"drop-light","theme_dark":"drop-dark"}`)
		sink := installMigrateCapture(t)

		nomination := themeNominationForTest(t)

		assertCanvasValue(t, nomination.Select(false), lightCanvas)
		assertCanvasValue(t, nomination.Select(true), darkCanvas)
		assertThemeEvents(t, sink,
			"INFO loaded slug=drop-light slot=light",
			"INFO loaded slug=drop-dark slot=dark",
		)
	})
}

// TestConstruction_PathFailuresDegradeNotBlock pins the tolerance both config
// paths owe the launch: neither an unresolvable prefs path nor an unresolvable
// themes directory may stop the picker opening. Each degrades to the shipped
// pair, which is what an unconfigured install renders anyway.
func TestConstruction_PathFailuresDegradeNotBlock(t *testing.T) {
	shippedLight := builtinThemeForTest(t, theme.DefaultLightSlug)
	shippedDark := builtinThemeForTest(t, theme.DefaultDarkSlug)

	t.Run("a prefs store that could not be built opens on the shipped pair", func(t *testing.T) {
		// Zero keys are openTUI's own degradation: a prefs path-resolution failure
		// leaves a zero prefsLoad — no store, no keys — exactly as it leaves the
		// grouping mode at Flat.
		nomination, err := themeNomination(prefs.ThemeKeys{}, newThemeLoader())
		if err != nil {
			t.Fatalf("a prefs load that failed must not fail construction: %v", err)
		}

		assertPair(t, nomination, shippedLight, shippedDark)
	})

	t.Run("a themes directory that cannot be resolved opens on the shipped pair", func(t *testing.T) {
		setPrefsFile(t, "")
		unresolvableThemesDir(t)

		nomination := themeNominationForTest(t)

		assertPair(t, nomination, shippedLight, shippedDark)
	})

	t.Run("a drop-in nomination falls back when there is no directory to look in", func(t *testing.T) {
		setPrefsFile(t, `{"theme":"a-drop-in"}`)
		unresolvableThemesDir(t)
		sink := installMigrateCapture(t)

		nomination := themeNominationForTest(t)

		assertConstant(t, nomination, shippedDark)
		assertThemeEvents(t, sink,
			"WARN fallback applied slug=a-drop-in reason=not found",
			"INFO loaded slug="+theme.DefaultDarkSlug,
		)
	})
}

// TestConstruction_NoColorLoadsBothSelectsDark pins §9.10: under NO_COLOR the
// theme machinery runs unchanged below the render layer. Both nominated themes
// are still loaded — a commit made in that session must have something in hand to
// persist against — and `theme: loaded` fires twice as normal.
//
// The gate is SKIPPED rather than left pending, which is what the frame shows:
// the same pair holds a neutral blank frame while its gate is open, so real
// content on the first frame is the observable difference between "skipped" and
// "not yet resolved". Which member is selected is internal/tui's own assertion
// (TestNoColor_LoadsBothAndSelectsDark) — there is nothing coloured left on
// screen for this layer to read it off.
func TestConstruction_NoColorLoadsBothSelectsDark(t *testing.T) {
	setPrefsFile(t, `{"theme_dark":"`+nordSlug+`"}`)
	sink := installMigrateCapture(t)

	nomination := themeNominationForTest(t)

	assertPair(t, nomination, builtinThemeForTest(t, theme.DefaultLightSlug), builtinThemeForTest(t, nordSlug))
	assertThemeEvents(t, sink,
		"INFO loaded slug="+theme.DefaultLightSlug+" slot=light",
		"INFO loaded slug="+nordSlug+" slot=dark",
	)

	cfg := defaultTestTUIConfig()
	cfg.theme = nomination
	cfg.noColor = true
	m := buildTUIModel(cfg, "", nil)

	assertPaintedCanvas(t, m, nil)
	if content := m.View().Content; !strings.Contains(content, "Sessions") {
		t.Errorf("the first NO_COLOR frame painted no real content; want the gate skipped rather than pending")
	}
}

// TestConstruction_StartupCanvasHexFromSelectedMember pins §11.4's anchor against
// a nomination that now comes from disk: the retained startup canvas hex is the
// canvas of the theme the GATE SELECTED, never of the other member and never of
// whatever the constructor was handed.
//
// It is read through the one surface that consumes it — restore.go's canvas-echo
// guard, which suppresses the exit-time set-back when the terminal's reported
// original IS the canvas Portal painted. Each member's own canvas is replayed as
// the reply, so the two cases discriminate in opposite directions: a hex captured
// from the wrong member would emit a set-back in one case and swallow a real one
// in the other.
func TestConstruction_StartupCanvasHexFromSelectedMember(t *testing.T) {
	setPrefsFile(t, `{"theme_dark":"`+nordSlug+`"}`)
	nomination := themeNominationForTest(t)
	nord := builtinThemeForTest(t, nordSlug)
	day := builtinThemeForTest(t, theme.DefaultLightSlug)

	for _, tc := range []struct {
		name     string
		reported theme.Theme
	}{
		{"the dark reply selects the dark member", nord},
		{"the light reply selects the light member", day},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := update(t, modelForNomination(nomination), backgroundReplyFor(tc.reported))

			if got := restoreWrite(m); got != "" {
				t.Errorf("exit-time restore wrote %q, want nothing: the terminal reported the very canvas the gate selected, so the echo guard must suppress the set-back", got)
			}
		})
	}

	t.Run("a background that is neither member's canvas is still set back", func(t *testing.T) {
		m := update(t, modelForNomination(nomination), otherBackgroundReply)

		if got := restoreWrite(m); got == "" {
			t.Errorf("exit-time restore wrote nothing; want the captured original set back (it is not the canvas the gate selected)")
		}
	})
}

// TestOpenTUI_FatalBeforeModelConstruction pins §7.6's one genuinely fatal state
// at the call site that created it: the theme a slot FALLS BACK to cannot be
// loaded from the embedded set, so there is nothing honest left to paint —
// openTUI returns §14A's pinned sentence and constructs NO TUI.
//
// The state is unreachable in a correctly built binary, which is exactly why it
// is staged rather than skipped: an unreachable fatal with no test is a path
// nobody has ever run. Loader.BuiltinSource exists for this, and the nomination is
// an ordinary unloadable slug so the FALLBACK is what fails, not the nomination.
//
// "Constructs no TUI" is BRACKETED on both sides of the construction statement,
// because either half alone is satisfiable by a fatal that returns too late:
//
//   - Before it: the last thing openTUI does ahead of buildTUIModel is ask the
//     live server for the current session name (inside tmux). A recording
//     commander that saw NO call therefore proves execution never reached the
//     statement immediately preceding construction.
//   - After it: staging the pending bootstrap warnings DRAINS the package-level
//     sink, so a still-full sink proves execution never reached the statement
//     immediately following construction — and, transitively, that no Bubble Tea
//     program was ever launched.
//
// The pair is what makes the assertion discriminating: the sink half alone stays
// green if the fatal is moved to sit BETWEEN construction and the staging call,
// which builds a full tui.Model on the fatal path — exactly the implementation
// this criterion forbids.
func TestOpenTUI_FatalBeforeModelConstruction(t *testing.T) {
	setPrefsFile(t, `{"theme":"no-such-theme"}`)

	openDeps = &OpenDeps{ThemeLoader: brokenBuiltinLoader(theme.DefaultDarkSlug)}
	t.Cleanup(func() { openDeps = nil })

	bootstrapWarnings.Drain()
	staged := bootstrap.Warning{Lines: []string{"staged: must survive the fatal"}}
	bootstrapWarnings.Add(staged)
	t.Cleanup(func() { bootstrapWarnings.Drain() })

	// Own the inside-tmux condition rather than inheriting TestMain's package-wide
	// TMUX poison: the pre-construction tripwire below is only armed while
	// tmux.InsideTmux() is true, so the fixture must set it explicitly instead of
	// depending on a value another file happens to supply.
	t.Setenv("TMUX", "/nonexistent/portal-test-must-set-tmux-socket,0,0")

	commander := &recordingCommander{}
	err := openTUI(cmdWithClient(tmux.NewClient(commander)), "", nil, false)

	if err == nil {
		t.Fatal("openTUI returned nil; want the broken-built-in fatal")
	}
	if want := theme.BrokenBuiltinError(theme.DefaultDarkSlug).Error(); err.Error() != want {
		t.Errorf("openTUI error = %q, want %q", err.Error(), want)
	}
	if len(commander.Calls) != 0 {
		t.Errorf("tmux commander saw %v, want no calls at all — the current-session read is the last statement before construction, so any call means the fatal returned past it", commander.Calls)
	}
	if pending := bootstrapWarnings.Drain(); len(pending) != 1 || !slices.Equal(pending[0].Lines, staged.Lines) {
		t.Errorf("bootstrap warnings after the fatal = %+v, want the seeded one still pending — a drained sink means a model was constructed on the fatal path", pending)
	}
}

// brokenBuiltinLoader stages §7.6's broken binary: a loader whose embedded set
// answers for every built-in EXCEPT missing, which is the slug a slot falls back
// to. Production carries a nil source and reads the real embedded set.
func brokenBuiltinLoader(missing string) *theme.Loader {
	loader := theme.NewLoader(theme.NewEventLogger(nil))
	loader.BuiltinSource = func(slug string) ([]byte, bool) {
		if slug == missing {
			return nil, false
		}
		return theme.BuiltinBytes(slug)
	}
	return &loader
}

// assertPair asserts the nomination is the adaptive state holding wantLight and
// wantDark, with no active member of its own.
func assertPair(t *testing.T, n theme.Nomination, wantLight, wantDark theme.Theme) {
	t.Helper()
	if n.IsConstant() {
		t.Fatalf("nomination is constant; want the adaptive pair the persisted slots nominate")
	}
	if got := n.Select(false); got != wantLight {
		t.Errorf("light member = %s, want %s", canvasOf(got), canvasOf(wantLight))
	}
	if got := n.Select(true); got != wantDark {
		t.Errorf("dark member = %s, want %s", canvasOf(got), canvasOf(wantDark))
	}
}

// update applies one message to the model and returns the updated model.
func update(t *testing.T, m tui.Model, msg tea.Msg) tui.Model {
	t.Helper()
	updated, _ := m.Update(msg)
	model, ok := updated.(tui.Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", updated)
	}
	return model
}

// resolveByTimeout drives the gate to its NO-ANSWER outcome exactly as the live
// program does: Init batches the §2.6 deadline tick alongside the OSC 11 query,
// so draining Init's commands and applying every message they produce is the
// timeout path — without this package naming internal/tui's unexported deadline
// message, which is precisely the kind of internal a cmd-level test should not
// know.
func resolveByTimeout(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	for _, msg := range drainCmd(t, m.Init()) {
		m = update(t, m, msg)
	}
	return m
}

// drainCmd runs a tea.Cmd to completion and returns every message it produced,
// flattening the tea.BatchMsg Init returns.
func drainCmd(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		msgs = append(msgs, drainCmd(t, c)...)
	}
	return msgs
}

// assertThemeEvents asserts the `theme` component emitted exactly want, in
// order, rendered as "<LEVEL> <event> <key>=<value>…".
//
// Exact and ordered rather than "contains": §12.3's cadence is the contract —
// the failure BEFORE the palette that replaced it, one `loaded` per slot and no
// second line for a slot that simply worked — and a contains-assertion would pass
// against a broken install logged in the one state §12.3 exists to make
// impossible.
func assertThemeEvents(t *testing.T, sink *logtest.Sink, want ...string) {
	t.Helper()
	got := themeEvents(t, sink)
	if !slices.Equal(got, want) {
		t.Errorf("theme events =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// assertCanvasValue asserts the theme's canvas token holds exactly want — the
// one token that identifies WHICH file was parsed, since every fixture below
// differs from every other in it and in nothing else.
func assertCanvasValue(t *testing.T, th theme.Theme, want string) {
	t.Helper()
	if got := th.Canvas.Value; got != want {
		t.Errorf("loaded canvas = %q, want %q", got, want)
	}
}

// backgroundReplyFor is the OSC 11 answer a terminal whose background IS th's
// canvas would send — the reply that makes the exit-time canvas-echo guard's
// comparison decidable without restating a hex the theme file already holds.
func backgroundReplyFor(th theme.Theme) tea.BackgroundColorMsg {
	return tea.BackgroundColorMsg{Color: th.Canvas.Color()}
}

// restoreWrite returns exactly what the production exit-time restore would write
// for the model — the empty string when the canvas-echo guard (or the NO_COLOR
// carve-out) suppresses the set-back.
func restoreWrite(m tui.Model) string {
	var out strings.Builder
	tui.RestoreTerminalBackground(&out, m)
	return out.String()
}

// seedUnlistableThemesDir points PORTAL_THEMES_DIR at a directory holding 50
// valid drop-ins — the named ones carrying the given canvas values, the rest
// filler — and makes the directory SEARCHABLE BUT NOT LISTABLE (mode 0111).
//
// The mode is the test's whole point: a by-name read of `<dir>/<slug>.theme`
// still succeeds, while any ReadDir fails. §5.7's lazy discovery is exactly that
// asymmetry, so an implementation that enumerated at construction would fail here
// rather than quietly paying for 50 parses on the cold path.
//
// The mode is restored before the temp directory is removed: this cleanup is
// registered after t.TempDir's, and cleanups run last-registered-first.
func seedUnlistableThemesDir(t *testing.T, canvases map[string]string) {
	t.Helper()

	dir := useThemesDir(t)
	for slug, canvas := range canvases {
		writeThemeFile(t, dir, slug, canvas)
	}
	for i := len(canvases); i < 50; i++ {
		writeThemeFile(t, dir, fmt.Sprintf("filler-%02d", i), fmt.Sprintf("#%06X", 0x300000+i))
	}

	if err := os.Chmod(dir, 0o111); err != nil {
		t.Fatalf("make themes dir unlistable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if _, err := os.ReadDir(dir); err == nil {
		t.Fatal("themes dir is still listable — the no-ReadDir tripwire would be vacuous")
	}
}

// writeThemeFile writes one valid drop-in whose canvas is the given hex.
//
// The body is a BUILT-IN's own source with its canvas line swapped, so a fixture
// cannot drift out of validity as the token vocabulary evolves, and the canvas is
// the only thing that tells two fixtures apart.
func writeThemeFile(t *testing.T, dir, slug, canvas string) {
	t.Helper()

	lines := strings.Split(string(validThemeSource(t)), "\n")
	swapped := 0
	for i, line := range lines {
		key, _, found := strings.Cut(line, "=")
		if found && strings.TrimSpace(key) == "canvas" {
			lines[i] = "canvas = " + canvas
			swapped++
		}
	}
	if swapped != 1 {
		t.Fatalf("swapped %d canvas lines in the built-in source, want exactly 1", swapped)
	}
	if err := os.WriteFile(filepath.Join(dir, slug+".theme"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write %s.theme: %v", slug, err)
	}
}

// unresolvableThemesDir removes every input themesDirPath resolves from, so it
// fails outright — the state task 5-7 degrades to an EMPTY directory string. The
// vacuity guard is load-bearing: with any one of the three still set the path
// resolves fine and the subtest would assert nothing.
func unresolvableThemesDir(t *testing.T) {
	t.Helper()

	t.Setenv("PORTAL_THEMES_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if got, err := themesDirPath(); err == nil {
		t.Fatalf("themesDirPath() = %q with no env var, no XDG_CONFIG_HOME and no HOME; this subtest would be vacuous", got)
	}
}

// themeEvents renders every `theme` component record the sink captured. The
// component attr is dropped from the rendering (it is the filter, not part of the
// line's content); every other attr is rendered in emission order.
func themeEvents(t *testing.T, sink *logtest.Sink) []string {
	t.Helper()
	var events []string
	for _, rec := range sink.Records() {
		if !rec.HasAttr("component") || rec.AttrString(t, "component") != "theme" {
			continue
		}
		parts := []string{rec.Level.String(), rec.Msg}
		for _, key := range rec.Keys {
			if key == "component" {
				continue
			}
			parts = append(parts, key+"="+rec.Attrs[key].String())
		}
		events = append(events, strings.Join(parts, " "))
	}
	return events
}
