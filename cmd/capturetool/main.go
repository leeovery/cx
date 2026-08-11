// Command capturetool renders a named, deterministic fixture of Portal's real
// TUI offline: a separate program from the shipped portal binary, building the
// production model through the shared tui.Build constructor with every tmux
// seam faked from internal/capture. It runs no bootstrap — no tmux server, no
// daemon, no config read.
//
// Usage:
//
//	capturetool --fixture sessions-flat
//	capturetool --fixture sessions-flat --theme nord
//	capturetool --fixture contrast-validation --theme ~/themes/mytheme.theme
//
// --theme names either a built-in slug or a path to a theme file. Both are an
// input rather than config discovery: nothing here reads prefs, resolves the
// themes directory or looks at XDG.
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

// Read off the shipped default rather than restated, so moving that default
// moves every unflagged capture with it.
const defaultThemeSlug = theme.DefaultDarkSlug

func main() {
	fixture := flag.String("fixture", "", "named fixture to render (e.g. sessions-flat)")
	themeArg := flag.String("theme", defaultThemeSlug, "theme to render: a built-in slug (e.g. "+defaultThemeSlug+") or a path to a .theme file")
	flag.Parse()

	if err := run(*fixture, *themeArg); err != nil {
		fmt.Fprintln(os.Stderr, "capturetool:", err)
		os.Exit(1)
	}
}

// Warnings are written to stderr before the program starts, so they land on
// the primary screen and stay readable once the alt screen is left.
func run(fixture, themeArg string) error {
	m, err := resolveProgram(fixture, themeArg, os.Stderr)
	if err != nil {
		return err
	}

	// The alt screen is declared through tea.View.AltScreen in tui.Model.View,
	// not a program option — Bubble Tea v2 removed tea.WithAltScreen.
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Terminals that ignore the OSC 111 reset keep the painted canvas, so set
	// the captured original background back.
	if fm, ok := finalModel.(tui.Model); ok {
		tui.RestoreTerminalBackground(os.Stdout, fm)
	}
	return nil
}

// The theme resolves here, before either branch, so a screen capture and the
// contrast swatch cannot be judged against different palettes; an unusable
// theme renders nothing on either branch. The loader is the silent one:
// capturetool renders a theme rather than using or diagnosing one, so it emits
// no theme events.
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

// An interface only so the built-in rejection branch is drivable: validation of
// the embedded set makes it otherwise unreachable without breaking a shipped
// .theme file.
type themeLoader interface {
	LoadBuiltin(slug string) (theme.Result, *theme.Rejection, bool)
}

// Failure is hard, never a fallback: silently rendering the wrong theme is the
// one failure a visual gate cannot catch.
func resolveTheme(loader themeLoader, arg string, warnings io.Writer) (theme.Theme, error) {
	if isThemePath(arg) {
		return resolvePathTheme(arg, warnings)
	}
	return resolveBuiltinTheme(loader, arg)
}

// The separator half is load-bearing: without it a real file with an
// unexpected extension is rejected as an unknown built-in, an error naming the
// wrong problem. The extension half lets a file in the working directory be
// named without a `./`. No valid slug can satisfy either half.
func isThemePath(arg string) bool {
	return strings.ContainsRune(arg, filepath.Separator) || strings.HasSuffix(arg, theme.FileExtension)
}

// No filename warning here, deliberately: a slug argument names a built-in by
// design, so a reserved-name check would fire on the documented invocation.
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

// theme.LoadPath runs the content rungs and neither filename one, so the
// filename warning is emitted only after a successful load — a broken file
// reports its contents rather than trailing a remark about its name.
func resolvePathTheme(path string, warnings io.Writer) (theme.Theme, error) {
	result, rejection := theme.LoadPath(path)
	if rejection != nil {
		return theme.Theme{}, fmt.Errorf("--theme %q is not usable: %w", path, rejection)
	}

	warnAboutFilename(warnings, filepath.Base(path))
	return result.Theme, nil
}

// Warning rather than blocking: this is a drop-in author's only visual-
// verification route, and the export workflow produces a reserved slug right
// up until the user renames the file. The derived candidate slug is never
// identity — it exists only to decide these two reasons.
func warnAboutFilename(w io.Writer, base string) {
	candidate, rejection := theme.SlugFromFilename(base)
	switch {
	case rejection != nil:
		_, _ = fmt.Fprintf(w, "capturetool: warning: %s: %s — a themes-directory file must be named <slug>.theme, lowercase (rendering it anyway)\n", base, theme.ReasonBadName)
	case slices.Contains(theme.BuiltinSlugs(), candidate):
		_, _ = fmt.Fprintf(w, "capturetool: warning: %s: %s — %q is a built-in slug, so a file with this name is ignored in the themes directory (rendering it anyway)\n", base, theme.ReasonReservedName, candidate)
	}
}

// The palette is passed as the constant nomination shape: a constant needs no
// light/dark detection, so there is no OSC 11 race and no frame captured
// mid-gate.
func resolveModel(fixture string, pinned theme.Theme) (tui.Model, error) {
	if fixture == "" {
		return tui.Model{}, fmt.Errorf("--fixture is required (available: %v)", capture.FixtureNames())
	}
	fx, err := capture.FixtureByName(fixture)
	if err != nil {
		return tui.Model{}, err
	}
	// Handed to Deps rather than assigned afterwards: the palette drives both
	// the nomination and the faked ThemeSource, which must agree.
	deps := fx.Deps(pinned)
	// NO_COLOR wins over --theme (there is no canvas to select), so the capture
	// shows no painted canvas whatever palette was named.
	if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
		deps.NoColor = true
	}
	return tui.Build(deps), nil
}
