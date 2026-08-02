package cmd

import (
	"fmt"
	"os"
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
// EVERY failure comes back as a Rejection, never as a bare error, so
// exportRefusal composes §14A's frames over a closed vocabulary rather than over
// whatever an arbitrary error happened to say. That is also why a themes
// directory that cannot even be LOCATED is folded into `unreadable`: it is the
// same fact from the user's side — the theme could not be read, and here is the
// system's reason — and leaving it raw would put a fifth, unpinned sentence on a
// surface §14A closes at four.
func resolveThemeSource(slug string) ([]byte, *theme.Rejection) {
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
		return nil, unreadableRejection(err)
	}

	path := filepath.Join(dir, slug+themeFileExtension)
	result, rejection := loader.LoadFile(path)
	if rejection != nil {
		return nil, absentOrUnreadable(path, rejection)
	}

	return result.Source, nil
}

// absentOrUnreadable narrows a failed read into §5.5's two answers: `not found`
// when there is NOTHING at the composed path, `unreadable` when there is
// something Portal could not read.
//
// LoadFile cannot draw the line itself — `not found` is outside §6.2's ladder,
// and producing it from a path would send a user to look for a file that
// function was just handed — so the by-name resolver that composed the path
// draws it, which is where the same discrimination lives for a persisted slug.
//
// It matters because the two answers send the user to different places: `not
// found` says check the filename, `unreadable` says check permissions. Against a
// themes directory the user cannot read, "no theme named nord-lee" is a sentence
// about a file that plainly exists (§12.1).
//
// The check is Lstat, not Stat, and that is the whole reason it is a stat at all
// rather than an os.IsNotExist over the read error. A DANGLING SYMLINK fails to
// read with exactly the errno an absent file does; only the name's own existence
// tells them apart, and §5.6 puts a dangling link under `unreadable` — it is a
// read that failed, not a name that is free.
func absentOrUnreadable(path string, rejection *theme.Rejection) *theme.Rejection {
	if rejection.Reason != theme.ReasonUnreadable {
		return rejection
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return &theme.Rejection{Reason: theme.ReasonNotFound}
	}

	return rejection
}

// unreadableRejection wraps an OS error as §6.2's `unreadable`, rendering the
// detail §14A's way: the error verbatim, since it is the only thing that says
// what actually went wrong.
func unreadableRejection(err error) *theme.Rejection {
	return &theme.Rejection{Reason: theme.ReasonUnreadable, Detail: err.Error(), Err: err}
}

func init() {
	themeCmd.AddCommand(themeExportCmd)
	rootCmd.AddCommand(themeCmd)
}
