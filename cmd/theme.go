package cmd

import (
	"path/filepath"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/theme"
	"github.com/spf13/cobra"
)

// themeFileExtension is the extension a drop-in carries (§5.3). The by-name
// path composes exactly `<slug>.theme` and looks for nothing else, which is
// what gives it §5.6's structural-uniqueness guarantee for free: no other
// spelling of the extension can name a file here.
const themeFileExtension = ".theme"

// themeCmd is the theme verb group. It has EXACTLY ONE member, deliberately
// (§12.1): `portal theme list` and a `--theme` flag were both ruled out as
// redundant with the picker's theme panel, and export is redundant with
// nothing — it is the only route from a built-in living inside the binary to a
// file on disk the user can edit.
var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage themes",
}

// themeExportCmd writes one theme's SOURCE FILE to stdout, so the published
// drop-in workflow is a shell redirect:
//
//	mkdir -p ~/.config/portal/themes
//	portal theme export nord > ~/.config/portal/themes/nord-lee.theme
//
// The `mkdir -p` is part of that workflow rather than an omission — Portal
// deliberately never creates or seeds the themes directory (§5.5), and a
// redirect will not create it either.
var themeExportCmd = &cobra.Command{
	Use:   "export <slug>",
	Short: "Write a theme's file to stdout",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source, err := resolveThemeSource(args[0])
		if err != nil {
			return err
		}

		_, err = cmd.OutOrStdout().Write(source)
		return err
	},
}

// resolveThemeSource resolves slug to the bytes of the file that declares it,
// having parsed and validated them first.
//
// The order is §8.4's, and export is the fourth by-name resolver to inherit it:
// THE EMBEDDED SET FIRST, THEN the themes directory. A slug naming a built-in
// resolves to the built-in and never reads — never even LOCATES — the themes
// directory, which is how §5.4's no-shadowing guarantee is carried on a path
// that does not enumerate and so has no collision to detect. The charset check
// runs ahead of both, so a hostile argument is refused before any path is
// composed from it.
//
// What it returns is the SOURCE, not a re-serialisation of the parsed Theme
// (§12.1). Re-serialising would drop every `#` comment, and the comments are
// the attribution header the file format was chosen to carry and the
// eyeball-pin derivation notes that are the only surviving record of a
// judgement no test can re-derive. The theme is still parsed first: that is
// what refuses an invalid drop-in, and it is what makes the bytes printed the
// bytes validated, with no second read in which the two could differ.
//
// The loader is handed log.Discard() so export emits no `theme` events at all
// (§12.3): the component records where a theme is USED, never where one is
// DIAGNOSED, and export's whole output is already the diagnostic the user is
// reading.
//
// Every failure is returned as a plain error, which main.classify prints to
// stderr at exit 1 with stdout untouched. The rejection travels in its
// structured form so the refusal frames composed over it name the right thing.
func resolveThemeSource(slug string) ([]byte, error) {
	if !theme.ValidSlug(slug) {
		return nil, &theme.Rejection{Reason: theme.ReasonBadName, BadNameCause: theme.BadNameSlug}
	}

	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	if result, rejection, found := loader.LoadBuiltin(slug); found {
		if rejection != nil {
			return nil, rejection
		}
		return result.Source, nil
	}

	dir, err := themesDirPath()
	if err != nil {
		return nil, err
	}

	result, rejection := loader.LoadFile(filepath.Join(dir, slug+themeFileExtension))
	if rejection != nil {
		return nil, rejection
	}

	return result.Source, nil
}

func init() {
	themeCmd.AddCommand(themeExportCmd)
	rootCmd.AddCommand(themeCmd)
}
