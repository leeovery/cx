// Command capturetool is the offline visual-capture harness for Portal's TUI.
//
// It is a SEPARATE, permanent program — NOT a subcommand of the shipped portal
// binary. It imports Portal's real internal/tui library, builds the production
// model via the shared tui.Build constructor, and binds every tmux seam to an
// in-memory fake from internal/capture. It runs NO bootstrap: it opens no tmux
// server, spawns no daemon, runs no orphan-sweep, and reads no real config.
//
// Its sole job is to render a deterministic, named fixture so vhs can screenshot
// the live TUI and a reviewer can compare it to the committed Paper reference
// (spec § 15). The captured frame is the REAL production TUI because the model is
// built through the exact constructor cmd/open.go uses.
//
// Usage:
//
//	capturetool --fixture sessions-flat
//	capturetool --fixture contrast-validation --theme tokyo-night
//
// --theme names a built-in and is resolved out of the embedded set, which is an
// input rather than config discovery — nothing here resolves a path or reads
// prefs.
//
// Run via vhs (see testdata/vhs/README.md). The portal binary never imports
// internal/capture — an import guard test enforces that production stays clean.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// defaultThemeSlug is the built-in --theme resolves to when the flag is
// omitted: the shipped dark default (§13.3). Every capture taken without the
// flag depends on it.
const defaultThemeSlug = "tokyo-night"

// --theme and --appearance coexist deliberately, and for exactly one more task.
//
// Both now resolve a PALETTE: --theme names the built-in the contrast-validation
// swatch renders, and --appearance names the built-in every tui.Build fixture is
// pinned to as a constant nomination. They are two spellings of one input, which
// is why task 3-4 collapses them — --appearance is removed, not kept alongside
// (§13.3), and --theme widens to the slug-or-path form with its `bad name` /
// `reserved name` stderr warnings on the path form.
func main() {
	fixture := flag.String("fixture", "", "named fixture to render (e.g. sessions-flat)")
	appearance := flag.String("appearance", "dark", "owned-canvas mode to render: dark|light")
	themeSlug := flag.String("theme", defaultThemeSlug, "built-in theme slug to render the contrast-validation swatch with (e.g. tokyo-night)")
	flag.Parse()

	if err := run(*fixture, *appearance, *themeSlug); err != nil {
		fmt.Fprintln(os.Stderr, "capturetool:", err)
		os.Exit(1)
	}
}

// run resolves the fixture into a renderable model and runs the Bubble Tea
// program on the alt screen until the user (or vhs) quits. It returns any
// resolution or program error so main can map it to a non-zero exit.
func run(fixture, appearance, themeSlug string) error {
	m, err := resolveProgram(fixture, appearance, themeSlug)
	if err != nil {
		return err
	}

	// Alt screen mirrors cmd/open.go's production launch so the captured frame
	// matches what a user sees. In Bubble Tea v2 the alt screen is declared via
	// the tea.View.AltScreen field (set in tui.Model.View()), not the removed
	// tea.WithAltScreen() option. No bootstrap, no warnings staging — this is
	// the inert, fixture-only render path.
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Restore the terminal's original background on exit (§ background restore-
	// on-exit). The owned canvas paint sets the terminal background via OSC 11;
	// terminals that ignore the OSC 111 reset would keep the canvas colour, so
	// SET the captured original back. No-op when no OSC 11 response was captured.
	// Wired identically to cmd/open.go via the shared tui helper. Writes to
	// os.Stdout (the program's output) so the sequence reaches the terminal.
	if fm, ok := finalModel.(tui.Model); ok {
		tui.RestoreTerminalBackground(os.Stdout, fm)
	}
	return nil
}

// resolveProgram maps a fixture name to the tea.Model the harness runs. Most
// fixtures resolve to the production tui.Model via the shared tui.Build
// constructor (resolveModel) so the capture is the REAL production frame. The
// contrast-validation swatch is the one exception: it is a standalone
// validation surface (a labelled tint swatch on the theme's own canvas) that
// deliberately does NOT route through tui.Build, which is exactly what lets it
// take a whole Theme before the render layer does. Returning tea.Model lets
// run() drive both identically.
//
// The two flags divide along that same line: --theme drives the swatch, and
// --appearance drives the tui.Build path. Bad input fails loudly on either.
func resolveProgram(fixture, appearance, themeSlug string) (tea.Model, error) {
	if fixture == capture.ContrastValidationFixture {
		th, err := resolveTheme(newThemeLoader(), themeSlug)
		if err != nil {
			return nil, err
		}
		return capture.NewContrastValidationModel(th), nil
	}
	m, err := resolveModel(fixture, appearance)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// builtinLoader is the seam --theme resolves a slug through: the embedded
// built-in half of theme.Loader, and nothing else.
//
// It is an interface so a test can drive the rejection branch, which §7.6's
// build-time guarantee makes unreachable through the shipped set — the
// alternative being to break a shipped .theme file, which would fail that
// guarantee's own test instead.
type builtinLoader interface {
	LoadBuiltin(slug string) (theme.Result, *theme.Rejection, bool)
}

// newThemeLoader builds the loader --theme resolves through, handed
// log.Discard.
//
// capturetool is §12.3's fifth caller and neither USES nor DIAGNOSES a theme —
// it is an offline renderer whose output is a frame — so it emits no `theme`
// events at all, and Discard leaves the loader's per-process dedup state owned
// rather than dangling.
func newThemeLoader() theme.Loader {
	return theme.NewLoader(theme.NewEventLogger(log.Discard()))
}

// resolveTheme resolves a --theme slug to the palette the swatch renders.
//
// Both failures are HARD: an unknown slug and a built-in whose file the §6.2
// ladder refused each return an error naming the slug (and, where there is one,
// the reason), and neither ever falls back. Silently rendering the wrong theme
// at a visual gate is precisely the failure this tool exists to prevent (§13.3),
// and the gate's whole job is judging colours the tests cannot.
//
// Slugs only, this phase: §13.3's slug-or-path discrimination and its filename
// warnings arrive in Phase 3 with the rest of the flag.
func resolveTheme(loader builtinLoader, slug string) (theme.Theme, error) {
	result, rejection, found := loader.LoadBuiltin(slug)
	switch {
	case !found:
		return theme.Theme{}, fmt.Errorf("--theme %q names no built-in theme (available: %v)", slug, theme.BuiltinSlugs())
	case rejection != nil:
		return theme.Theme{}, fmt.Errorf("--theme %q is not usable: %w", slug, rejection)
	}
	return result.Theme, nil
}

// resolveModel maps a fixture name to the production tui.Model via the shared
// tui.Build constructor, passing the CONSTANT nomination shape (§13.3): the one
// built-in the --appearance flag names, pinned as the whole theme setting.
//
// Constant is what makes a capture usable as a visual gate. A constant needs no
// light/dark detection, so the model is resolved at construction — no OSC 11
// query race, no first-paint wait, no chance of a frame captured mid-gate — and
// the palette is exactly the one named on the command line. An empty or unknown
// fixture, an invalid appearance, or a built-in that will not load is an error, so
// a bad flag fails loudly rather than rendering the wrong theme at the gate.
func resolveModel(fixture, appearance string) (tui.Model, error) {
	if fixture == "" {
		return tui.Model{}, fmt.Errorf("--fixture is required (available: %v)", capture.FixtureNames())
	}
	slug, err := resolveAppearanceSlug(appearance)
	if err != nil {
		return tui.Model{}, err
	}
	pinned, err := resolveTheme(newThemeLoader(), slug)
	if err != nil {
		return tui.Model{}, err
	}
	fx, err := capture.FixtureByName(fixture)
	if err != nil {
		return tui.Model{}, err
	}
	deps := fx.Deps()
	deps.Theme = theme.ConstantNomination(pinned)
	// NO_COLOR carve-out (§2.5): read the env (present and non-empty, the
	// no-color.org convention) and inject the single colourless flag so the
	// NO_COLOR tape (NO_COLOR=1 inline) renders the colourless native-bg path —
	// the same flag cmd/open.go drives in production. When set it wins over the
	// nomination (no canvas to select), so the capture shows no painted canvas.
	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		deps.NoColor = true
	}
	return tui.Build(deps), nil
}

// resolveAppearanceSlug maps the --appearance flag to the built-in slug whose
// palette the capture pins: dark names the shipped dark default (#0b0c14 canvas),
// light the shipped light one (#e1e2e7). An unrecognised value fails loudly
// rather than silently defaulting, so a typo in a tape is caught.
//
// It exists for exactly one more task. §13.3 replaces --appearance with --theme,
// which takes a slug or a path directly — there is no mode left to name, since a
// theme IS the mode — so this is the last translation from the old axis to the
// new one.
func resolveAppearanceSlug(appearance string) (string, error) {
	switch appearance {
	case "dark":
		return theme.DefaultDarkSlug, nil
	case "light":
		return theme.DefaultLightSlug, nil
	default:
		return "", fmt.Errorf("--appearance must be dark or light, got %q", appearance)
	}
}
