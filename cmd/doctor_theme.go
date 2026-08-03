package cmd

import (
	"fmt"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/theme"
)

// The §14A copy this file produces, one constant per pinned frame. They are
// whole lines INCLUDING the leading "⚠ ": the copy table pins each line together
// with its glyph, and the advisory renderer only indents what a producer hands
// it (see the advisory type in cmd/doctor.go), so a producer owns the whole
// string.
//
// The file frame is GENERIC across every reason that has a slug in hand —
// `missing tokens`, `bad colour`, `bad syntax`, `unreadable` — because §6.2
// already guarantees exactly one reason per file and §14A already fixes the
// detail's shape per reason. Doctor therefore re-derives nothing: it frames the
// reason and the detail Phase 1 produced.
const (
	themeFileAdvisoryFormat   = "⚠ theme %s: %s — %s"
	themesDirUnreadableFormat = "⚠ themes directory unreadable: %s"
)

// collectThemeAdvisories is doctor's whole theme-advisory producer: the single
// entry point the report's advisory block is built from, run once per diagnosis
// pass.
//
// It is strictly READ-ONLY — no write, no repair, no directory creation — which
// is what lets it run unchanged on the `--fix` path, where there is no repair to
// perform and suppressing it would make `--fix` a LESS informative diagnosis
// than a plain run.
//
// The loader is handed log.Discard(), ALWAYS, on every doctor path. §12.3: the
// `theme` component records where a theme is USED, never where one is
// DIAGNOSED. Doctor is the user looking — its whole output is already the
// diagnostic they are reading — and it is the run most likely to hit a full
// reject set, so emitting here would put the largest possible WARN volume on the
// surface that needs it least. It also keeps doctor's read-only claim literal: a
// diagnosis command writing WARNs about the state it just printed is the same
// shape of side effect as repairing one.
func collectThemeAdvisories(deps *DoctorDeps) []advisory {
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	return scanThemesDirectory(loader, deps.ThemesDir)
}

// scanThemesDirectory enumerates dir through Phase 1's ladder and renders one
// advisory per finding: the directory's own verdict where it has one, else one
// line per rejected file.
//
// An UNRESOLVED path — themesDirPath() failed, so resolveDoctorDeps left the
// field empty — skips the scan ENTIRELY and yields nothing. The advisory class
// has no not-evaluable form, so degrading to zero lines is the only shape
// available, and a path that could not be resolved must never abort the
// diagnosis. The skip is scoped HERE rather than at collectThemeAdvisories, so
// producers that need no path at all still run.
//
// §5.5's directory-state table is what the two Enumerate returns separate, and
// each row gets a different answer:
//
//   - an UNUSABLE directory (unreadable, or a regular file where a directory
//     belongs) → its one pinned line. Enumerate returns no entries in that
//     state, so it is the only theme-file line the scan can produce.
//   - an ABSENT directory → nothing at all: no line, no error, no log. Zero
//     drop-ins is not an error and Portal never creates or seeds the directory.
//   - a usable directory → one line per rejected entry, in the enumeration's
//     own deterministic filename order.
func scanThemesDirectory(loader theme.Loader, dir string) []advisory {
	if dir == "" {
		return nil
	}

	entries, dirRejection := loader.Enumerate(dir)
	if dirRejection != nil {
		return []advisory{{line: fmt.Sprintf(themesDirUnreadableFormat, dir)}}
	}

	var advisories []advisory
	for _, entry := range entries {
		if a, reported := themeFileAdvisory(entry); reported {
			advisories = append(advisories, a)
		}
	}
	return advisories
}

// themeFileAdvisory renders one enumerated entry's advisory, and reports whether
// the entry earns one at all.
//
// A valid entry (nil Rejection) produces no line — the scan reports problems,
// not inventory. A rejected one produces EXACTLY ONE line, for the one reason
// §6.2's ladder settled on: doctor enumerates within the reason and never
// across, so a file is never reported as both `bad colour` and `missing tokens`.
//
// The switch is exhaustive over the reasons rather than a bare "has a slug"
// test, so the ones this producer does NOT own are visibly skipped rather than
// silently swept into the generic frame: `bad name` and `reserved name` have
// their own §14A lines composed from the FILENAME (a bad-name file yields no
// slug at all), and `not found` applies to a persisted slug with no file, where
// nothing was enumerated.
//
// slug and fromPrefs are populated alongside the line because they are the
// identity §12.2's one-slug-one-line union dedups on — an unresolvable persisted
// slug outranks the same slug's file-validity line. A producer setting `line`
// alone would silently defeat that dedup and make <M> count detections rather
// than problems.
func themeFileAdvisory(entry theme.Entry) (advisory, bool) {
	if entry.Rejection == nil {
		return advisory{}, false
	}

	switch entry.Rejection.Reason {
	case theme.ReasonMissingTokens, theme.ReasonBadColour, theme.ReasonBadSyntax, theme.ReasonUnreadable:
		return advisory{
			line:      fmt.Sprintf(themeFileAdvisoryFormat, entry.Slug, entry.Rejection.Reason, rejectionDetail(entry.Rejection)),
			slug:      entry.Slug,
			fromPrefs: false,
		}, true
	default:
		// The two filename reasons and `not found` are other producers' lines.
		return advisory{}, false
	}
}

// rejectionDetail is the loader's own detail, carried verbatim: nothing is
// re-derived, re-ordered, re-wrapped or double-prefixed, because Phase 1 already
// renders each reason in the exact §14A form its surfaces print — `missing
// text.primary, bg.subtle`, `text.primary = #GGGGGG, canvas = blue`, `line 12:
// duplicate key text.primary`, and an OS error verbatim.
//
// The Err fallback is for `unreadable` alone, the one reason produced by
// something other than Portal's own rules: it carries the OS error on a
// dedicated field as well as in Detail, and reading the structured error where
// the rendered one is absent keeps the OS's message the detail on every shape a
// failed read can take — without ever rendering it twice.
func rejectionDetail(rejection *theme.Rejection) string {
	if rejection.Detail == "" && rejection.Err != nil {
		return rejection.Err.Error()
	}
	return rejection.Detail
}
