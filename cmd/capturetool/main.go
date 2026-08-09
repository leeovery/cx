// Command capturetool is the offline visual-capture harness for Portal's TUI.
//
// It is a SEPARATE, permanent program — NOT a subcommand of the shipped portal
// binary. It imports Portal's real internal/tui library, builds the production
// model via the shared tui.Build constructor, and binds every tmux seam to an
// in-memory fake from internal/capture. It runs NO bootstrap: it opens no tmux
// server, spawns no daemon, runs no orphan-sweep, and reads no real config.
//
// Its sole job is to render a deterministic, named fixture so vhs can screenshot
// the live TUI and a reviewer can compare it to the committed design reference.
// The captured frame is the REAL production TUI because the model is built
// through the exact constructor cmd/open.go uses.
//
// Usage:
//
//	capturetool --fixture sessions-flat
//	capturetool --fixture sessions-flat --theme nord
//	capturetool --fixture contrast-validation --theme ~/themes/mytheme.theme
//
// --theme names either a built-in (resolved out of the embedded set) or an
// explicit path to a theme file. Both are an INPUT rather than config discovery —
// nothing here reads prefs, resolves the themes directory or looks at XDG — which
// is what keeps internal/capture's no-real-config guarantee intact.
//
// Run via vhs (see testdata/vhs/README.md). The portal binary never imports
// internal/capture — an import guard test enforces that production stays clean.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// defaultThemeSlug is the built-in --theme resolves to when the flag is omitted:
// the shipped dark default. Every capture taken without the flag depends on it,
// so it is READ OFF the shipped default rather than restating its value — moving
// that default moves every unflagged capture with it.
const defaultThemeSlug = theme.DefaultDarkSlug

// themeFileExtension is the extension that marks a --theme argument as a FILE
// even when it carries no directory at all (`nord.theme` in the working
// directory). It is a local restatement of the theme-file extension because
// internal/theme keeps its own copy unexported — the loader decides validity,
// this program only decides which of its two entry points to call.
const themeFileExtension = ".theme"

// main parses the two flags and maps any resolution or program failure onto a
// non-zero exit.
//
// There are exactly two flags, and --appearance is deliberately NOT among them:
// the light/dark injection it resolved into no longer exists, and a theme IS the
// mode, so there is nothing left for it to pin.
func main() {
	fixture := flag.String("fixture", "", "named fixture to render (e.g. sessions-flat)")
	themeArg := flag.String("theme", defaultThemeSlug, "theme to render: a built-in slug (e.g. "+defaultThemeSlug+") or a path to a .theme file")
	flag.Parse()

	if err := run(*fixture, *themeArg); err != nil {
		fmt.Fprintln(os.Stderr, "capturetool:", err)
		os.Exit(1)
	}
}

// run resolves the fixture into a renderable model and runs the Bubble Tea
// program on the alt screen until the user (or vhs) quits. It returns any
// resolution or program error so main can map it to a non-zero exit.
//
// Warnings are written to stderr BEFORE the program starts, so they land on the
// primary screen and are still readable once the alt screen is left.
func run(fixture, themeArg string) error {
	m, err := resolveProgram(fixture, themeArg, os.Stderr)
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

	// Restore the terminal's original background on exit. The owned canvas paint
	// sets the terminal background via OSC 11; terminals that ignore the OSC 111
	// reset would keep the canvas colour, so
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
// ONE flag drives BOTH branches, which is why the theme is resolved here rather
// than inside either: the swatch validates the tints a screen then renders, so a
// capture of one and a capture of the other must be the same palette or the gate
// compares a screen against a swatch of something else.
//
// It is resolved FIRST, and a failure returns a nil model: an unusable theme means
// nothing renders at all, on either branch — there is no fallback palette.
//
// The loader is the SILENT one. capturetool neither uses nor diagnoses a theme —
// it is an offline renderer whose output is a frame — so it emits no `theme`
// events at all.
func resolveProgram(fixture, themeArg string, warnings io.Writer) (tea.Model, error) {
	pinned, err := resolveTheme(theme.NewSilentLoader(), themeArg, warnings)
	if err != nil {
		return nil, err
	}
	if fixture == capture.ContrastValidationFixture {
		return capture.NewContrastValidationModel(pinned), nil
	}
	m, err := resolveModel(fixture, pinned)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// themeLoader is the seam --theme's SLUG branch resolves through: the embedded
// built-in set, and nothing else. It reads no config, which is what keeps the
// no-real-config invariant true.
//
// It is an interface so the built-in rejection branch can be driven at all: the
// build-time validation of the embedded set makes that branch unreachable through
// the shipped themes, and the alternative would be breaking a shipped .theme file.
//
// The PATH branch takes no seam. theme.LoadPath is a function over a path the
// caller named, with nothing injected to fake.
type themeLoader interface {
	LoadBuiltin(slug string) (theme.Result, *theme.Rejection, bool)
}

// resolveTheme resolves the --theme argument to the palette every branch renders,
// writing any non-blocking FILENAME warning to warnings (stderr in production)
// rather than refusing to render.
//
// Failure is always HARD and never a fallback. Silently rendering the
// wrong theme at a visual gate is precisely the failure this tool exists to
// prevent, and the gate's whole job is judging colours the tests cannot.
func resolveTheme(loader themeLoader, arg string, warnings io.Writer) (theme.Theme, error) {
	if isThemePath(arg) {
		return resolvePathTheme(arg, warnings)
	}
	return resolveBuiltinTheme(loader, arg)
}

// isThemePath reports whether the --theme argument names a FILE rather than a
// built-in slug: it does if it carries a path separator, or if it ends in
// `.theme`.
//
// The separator half is load-bearing rather than a convenience. Without it a real
// file with an unexpected extension — `./mytheme.txt`, or an editor's `.bak` —
// classifies as a slug and is rejected as an unknown built-in: an error naming the
// wrong problem entirely, for a file that plainly exists. The extension half is
// what lets a file in the working directory be named without a `./` prefix.
//
// A slug can never be mistaken for a path in the other direction: the slug
// charset admits neither separators nor dots, so no valid slug can satisfy either
// half.
func isThemePath(arg string) bool {
	return strings.ContainsRune(arg, filepath.Separator) || strings.HasSuffix(arg, themeFileExtension)
}

// resolveBuiltinTheme resolves a --theme SLUG out of the embedded set.
//
// Both failures are hard: an unknown slug and a built-in whose file the rejection
// ladder refused each return an error naming the slug (and, where there is one,
// the reason).
//
// It emits NO filename warning, deliberately. A slug argument names a built-in by
// design, so a `reserved name` check here would fire on the normal, documented
// invocation — `--theme nord` would warn the user about the very thing they asked
// for.
func resolveBuiltinTheme(loader themeLoader, slug string) (theme.Theme, error) {
	result, rejection, found := loader.LoadBuiltin(slug)
	switch {
	case !found:
		return theme.Theme{}, fmt.Errorf("--theme %q names no built-in theme (available: %v)", slug, theme.BuiltinSlugs())
	case rejection != nil:
		return theme.Theme{}, fmt.Errorf("--theme %q is not usable: %w", slug, rejection)
	}
	return result.Theme, nil
}

// resolvePathTheme resolves a --theme PATH through the package's explicit-path
// entry point, which runs the four CONTENT rungs and neither filename one.
//
// Only the content reasons can therefore reject, and any of them is a hard error
// carrying the reason — never a fallback. The palette that comes back has NO
// SLUG: a theme handed in by path has no identity, and nothing downstream is
// given one.
//
// The warnings are emitted only once the file has loaded, so a broken file reports
// what is wrong with its contents rather than trailing a remark about its name.
func resolvePathTheme(path string, warnings io.Writer) (theme.Theme, error) {
	result, rejection := theme.LoadPath(path)
	if rejection != nil {
		return theme.Theme{}, fmt.Errorf("--theme %q is not usable: %w", path, rejection)
	}

	warnAboutFilename(warnings, filepath.Base(path))
	return result.Theme, nil
}

// warnAboutFilename emits the two FILENAME reasons — `bad name` and
// `reserved name` — as non-blocking stderr lines about a path argument.
//
// The candidate slug exists SOLELY to produce these two lines and is never used as
// identity: without deriving one there is no `reserved name` to detect, since both
// filename reasons are decided from the slug alone.
//
// Warning rather than blocking is what the flag's stated purpose demands. This is
// the only visual-verification route someone authoring a drop-in has, so it is the
// one place a fatal filename is worth catching before the file reaches the themes
// directory — while refusing to render would leave that author with no route at
// all, and would break the published export workflow, where an exported built-in
// is a reserved slug right up until the user renames it.
//
// One line per reason, and the two are mutually exclusive: a name yielding no slug
// has none to collide with.
func warnAboutFilename(w io.Writer, base string) {
	candidate, rejection := theme.SlugFromFilename(base)
	switch {
	case rejection != nil:
		_, _ = fmt.Fprintf(w, "capturetool: warning: %s: %s — a themes-directory file must be named <slug>.theme, lowercase (rendering it anyway)\n", base, theme.ReasonBadName)
	case slices.Contains(theme.BuiltinSlugs(), candidate):
		_, _ = fmt.Fprintf(w, "capturetool: warning: %s: %s — %q is a built-in slug, so a file with this name is ignored in the themes directory (rendering it anyway)\n", base, theme.ReasonReservedName, candidate)
	}
}

// resolveModel maps a fixture name to the production tui.Model via the shared
// tui.Build constructor, passing the CONSTANT nomination shape: the one palette
// --theme resolved, pinned as the whole theme setting.
//
// Constant is what makes a capture usable as a visual gate. A constant needs no
// light/dark detection, so the model is resolved at construction — no OSC 11
// query race, no first-paint wait, no chance of a frame captured mid-gate — and
// the palette is exactly the one named on the command line. An empty or unknown
// fixture is an error, so a bad flag fails loudly rather than rendering something
// nobody asked for.
func resolveModel(fixture string, pinned theme.Theme) (tui.Model, error) {
	if fixture == "" {
		return tui.Model{}, fmt.Errorf("--fixture is required (available: %v)", capture.FixtureNames())
	}
	fx, err := capture.FixtureByName(fixture)
	if err != nil {
		return tui.Model{}, err
	}
	// The palette is handed to Deps rather than assigned afterwards, because it
	// drives TWO seams: the constant nomination the model paints from, and the
	// panel's faked ThemeEnumerator, whose Resolve must report the SAME palette or
	// the panel's open-time apply repaints the frame off --theme. Fixture.Deps is
	// the one place the pair is assembled.
	deps := fx.Deps(pinned)
	// NO_COLOR carve-out: read the env (present and non-empty, the
	// no-color.org convention) and inject the single colourless flag so the
	// NO_COLOR tape (NO_COLOR=1 inline) renders the colourless native-bg path —
	// the same flag cmd/open.go drives in production. It is read AFTER the theme
	// resolves and WINS over it (there is no canvas to select), so the capture
	// shows no painted canvas whatever --theme named.
	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		deps.NoColor = true
	}
	return tui.Build(deps), nil
}
