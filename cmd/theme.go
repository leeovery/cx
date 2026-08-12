package cmd

import (
	"fmt"

	"github.com/leeovery/portal/internal/theme"
	"github.com/spf13/cobra"
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage themes",
}

var themeExportCmd = &cobra.Command{
	Use:           "export <slug>",
	Short:         "Write a theme's file to stdout",
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Stripped at the read, not at the echo: the stripped value is what
		// the charset check judges and what the refusal frame names.
		slug := theme.StripControl(args[0])

		source, rejection := resolveThemeSource(slug)
		if rejection != nil {
			return exportRefusal(slug, rejection)
		}

		_, err := cmd.OutOrStdout().Write(source)
		return err
	},
}

// `reserved name` deliberately has no arm: a slug colliding with a built-in
// resolves to the embedded set first, so it cannot arise here. A plain error on
// purpose — a *UsageError would exit 2, a silent-exit sentinel would print
// nothing.
func exportRefusal(slug string, rejection *theme.Rejection) error {
	switch rejection.Reason {
	case theme.ReasonNotFound:
		return fmt.Errorf("no theme named %s", slug)
	case theme.ReasonUnreadable:
		return fmt.Errorf("theme %s could not be read: %s", slug, rejection.Detail)
	default:
		return fmt.Errorf("theme %s is not valid: %s", slug, rejection.Reason)
	}
}

// resolveThemeSource returns the file's bytes, not a re-serialisation of the
// parsed Theme — comments must survive; the theme is still parsed first, which
// is what refuses an invalid drop-in. The loader stays silent: the `theme` log
// component records where a theme is used, never where one is diagnosed.
func resolveThemeSource(slug string) ([]byte, *theme.Rejection) {
	loader := theme.NewSilentLoader()

	dir, dirErr := themesDirPath()
	result, rejection := loader.ResolveByName(slug, dir)
	if rejection != nil {
		return nil, unlocatableAsUnreadable(rejection, dirErr)
	}

	return result.Source, nil
}

// themesDirPath answers "" on failure, which the resolver turns into `not
// found` — right for TUI construction, wrong for export, which would name a
// theme never actually looked for. `not found` alongside a resolution error can
// only be this state, so the fold is exact.
func unlocatableAsUnreadable(rejection *theme.Rejection, dirErr error) *theme.Rejection {
	if dirErr == nil || rejection.Reason != theme.ReasonNotFound {
		return rejection
	}

	return &theme.Rejection{Reason: theme.ReasonUnreadable, Detail: dirErr.Error(), Err: dirErr}
}

func init() {
	themeCmd.AddCommand(themeExportCmd)
	rootCmd.AddCommand(themeCmd)
}
