package cmd

import (
	"fmt"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/theme"
	"github.com/spf13/cobra"
)

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
		// The argument is control-stripped HERE, at the point it is read
		// (§9.5). Export never reads prefs (§10.5), so it is not covered by the
		// rule that strips a persisted slug on the way in and needs its own
		// site — but §14A echoes the argument back on stderr, and an argument
		// can carry a pasted escape exactly as a prefs value can. Stripping at
		// the read rather than at the echo is what makes the stripped value THE
		// value: it is what the charset check judges, what a path is composed
		// from, and what the refusal frame names.
		slug := theme.StripControl(args[0])

		source, rejection := resolveThemeSource(slug)
		if rejection != nil {
			return exportRefusal(slug, rejection)
		}

		_, err := cmd.OutOrStdout().Write(source)
		return err
	},
}

// exportRefusal composes §14A's stderr frame for one export failure.
//
// Export is a diagnosis tool — "show me what Portal parsed" — so its failure
// message is the whole answer the user gets, and each class sends them somewhere
// different: a name to re-type, a file to fix, or a permission to check. The
// reason string is also all that discriminates them, because §12.1 fixes the
// exit code at 1 for every class; nothing about the failure is scriptable, so
// nothing is encoded numerically.
//
// `unreadable` gets a frame of its own rather than folding into "is not valid"
// because NOTHING WAS READ — calling a file invalid would describe a judgement
// that was never made. `not found` is the concept behind the unknown-slug case,
// but its label is deliberately not printed: the frame is a sentence about the
// name the user typed, which is the thing they have to go and fix.
//
// `reserved name` needs no arm of its own. It cannot arise here — a slug
// colliding with a built-in IS a built-in, so it resolves to the embedded set
// and the file is never opened (§8.4) — and the generic frame is the right
// rendering if that ever ceased to hold, so there is no dead special case.
//
// The result is a PLAIN error. main.classify prints it once to stderr and exits
// 1: not a *UsageError, which would exit 2, and not a silent-exit sentinel,
// which would print nothing at all and leave the user with only the exit code.
func exportRefusal(slug string, rejection *theme.Rejection) error {
	switch rejection.Reason {
	case theme.ReasonNotFound:
		return fmt.Errorf("no theme named %s", slug)
	case theme.ReasonUnreadable:
		// Detail is the OS error verbatim (§14A) — the only thing that
		// separates a permission denial from a dangling symlink — and it is
		// rendered by whoever produced the rejection, so nothing is re-derived
		// here.
		return fmt.Errorf("theme %s could not be read: %s", slug, rejection.Detail)
	default:
		return fmt.Errorf("theme %s is not valid: %s", slug, rejection.Reason)
	}
}

// resolveThemeSource resolves slug to the bytes of the file that declares it,
// having parsed and validated them first.
//
// The ordering is §8.4's — charset check, THE EMBEDDED SET, then the themes
// directory — and export does not implement it. `theme.Loader.ResolveByName`
// does, and export is one of its two callers alongside TUI construction, so the
// two cannot drift about what a slug means. A second implementation of an
// ordering rule is a rule that will eventually be two rules, and this one
// carries §5.4's no-shadowing guarantee on a path that never enumerates.
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
// (§12.3) — including the resolver's own `directory unusable` line. The
// component records where a theme is USED, never where one is DIAGNOSED, and
// export's whole output is already the diagnostic the user is reading.
//
// EVERY failure comes back as a Rejection, never as a bare error, so
// exportRefusal composes §14A's frames over a closed vocabulary rather than over
// whatever an arbitrary error happened to say.
func resolveThemeSource(slug string) ([]byte, *theme.Rejection) {
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	dir, dirErr := themesDirPath()
	result, rejection := loader.ResolveByName(slug, dir)
	if rejection != nil {
		return nil, unlocatableAsUnreadable(rejection, dirErr)
	}

	return result.Source, nil
}

// unlocatableAsUnreadable folds a themes directory that could not even be
// LOCATED into §6.2's `unreadable`, carrying the resolution error verbatim.
//
// themesDirPath answers with the empty string when it fails, which the resolver
// reads as "there is no directory to look in" and turns into `not found`. That
// is right for TUI construction, which must open regardless — but for export it
// would print "no theme named nord-lee" about a name that was never actually
// looked for. The fold restores the honest answer: from the user's side it is
// the identical fact to any other unreadable directory — the theme could not be
// read, and here is the system's reason.
//
// It is exact rather than approximate. An empty directory yields `not found` on
// every path and no other reason, so a `not found` alongside a resolution error
// can only be this state; a bad name or a content reason is returned untouched,
// and a built-in resolves without the directory being needed at all.
//
// Surfacing the resolution error raw instead would put a fifth, unpinned
// sentence on a surface §14A closes at four.
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
