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
//
// The three FILENAME frames below are the deliberate exception, and the
// difference is not decoration: §6.2's two filename reasons are decided from the
// name before the file is opened, so the frame is labelled `⚠ theme file
// <filename>:` rather than `⚠ theme <slug>:`. That differing frame is what
// carries the INPUT CLASS — a file versus a slug — which is why §6.2 keeps one
// reason class for the panel row (which has no width to discriminate) while
// doctor, which has the width, names which cause.
//
// The two `bad name` causes take DISTINCT messages off Phase 1's BadNameCause
// because their fixes are different: with a wrong-cased extension the slug
// portion is ALREADY LEGAL, so a single message telling the user to fix their
// slug would send them to correct the one thing that is fine — the misdirection
// §9.4, §12.1 and §14A discriminate against at three other sites.
//
// `reserved name` is labelled by filename too, despite being the one filename
// reason that HAS a valid slug, because that slug is identical to the built-in's:
// labelling by slug would print the same name twice with no way to tell which row
// is the user's file. Its line also deliberately does not follow the generic
// `<reason> — <detail>` frame — it names the conflict AND the fix, which is what
// makes §5.4's two-second-rename workaround self-documenting rather than merely
// short, and is why §14A records no separate detail for it.
//
// The persisted frame is ONE format with an OPTIONAL slot insert rather than two
// whole lines, because §14A's two renderings differ in exactly that insert. Two
// consts would state "does not resolve" twice — the drift the single-sourcing
// convention exists to prevent, and invisible at review since the two would sit
// three lines apart. It carries NO ` — <detail>` tail, unlike the file frame
// above: the reason is the whole answer for a slug that names nothing, and the
// one reason with a detail to give (`unreadable`) is about the DIRECTORY, which
// already has its own line.
const (
	themeFileAdvisoryFormat   = "⚠ theme %s: %s — %s"
	themesDirUnreadableFormat = "⚠ themes directory unreadable: %s"

	badNameSlugAdvisoryFormat      = "⚠ theme file %s: slug must be lowercase letters, digits and hyphens"
	badNameExtensionAdvisoryFormat = "⚠ theme file %s: extension must be lowercase .theme"
	reservedNameAdvisoryFormat     = "⚠ theme file %s: %s is a built-in — rename it (e.g. %s-mine.theme)"

	persistedThemeAdvisoryFormat = "⚠ theme %s%s does not resolve: %s"
	persistedThemeSlotFormat     = " (%s)"
)

// The three §14A slot labels. `both` is not a third slot — §8.2 has exactly two
// — it is the rendering of ONE slug occupying both of them, which §9.5 makes
// reachable in two keypresses and §12.2's one-slug-one-line rule requires be one
// line rather than two.
const (
	themeSlotLight = "light"
	themeSlotDark  = "dark"
	themeSlotBoth  = "both"
)

// collectThemeAdvisories is doctor's whole theme-advisory surface: the single
// entry point the report's advisory block is built from, run once per diagnosis
// pass, over the TWO producers behind it — the themes-directory scan (what is IN
// a directory) and the persisted-theme read (what the user PICKED).
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
//
// The two producers share ONE loader, built here and passed down. That is what
// gives the whole diagnosis a single owned dedup set (theme.EventLogger holds it
// per instance) rather than two that could each report the same condition — and
// because it is Discard-backed the sharing is free, since neither emits anything
// whatever the dedup says.
func collectThemeAdvisories(deps *DoctorDeps) []advisory {
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	return append(scanThemesDirectory(loader, deps.ThemesDir), persistedThemeAdvisories(deps, loader)...)
}

// persistedThemeAdvisories is doctor's SECOND theme-advisory producer: the one
// reporting that a theme the user CHOSE no longer resolves — a deleted file, a
// renamed file, a typo in prefs.json. Portal falls back silently by design and
// never overwrites the persisted name, so without this line the only signal a
// user gets is "my colours changed".
//
// The prefs read is deps.PrefsStore, which resolveDoctorDeps builds through the
// NON-MIGRATING loadPrefsStoreNoMigrate (§10.5, §12.2). A nil store — the
// unresolvable-config-path degradation — produces no lines rather than an error:
// the advisory class has no not-evaluable form, and a path that could not be
// computed must never abort a diagnosis.
//
// The read is TOLERANT and its error is discarded on purpose. Every degenerate
// prefs.json — absent, empty, corrupt, unreadable, missing every key — yields
// zero keys, which yields zero nominations and therefore zero lines. The one
// thing a diagnosis must not do is fail to diagnose because one of the files it
// reads is the broken one.
//
// Resolution goes through ResolveByName, NEVER ResolveNomination: the latter
// substitutes §8.5's fallbacks, which would HIDE the very failure being reported,
// and can raise §7.6's broken-built-in fatal, which would abort the diagnosis
// over a state this line exists to describe.
func persistedThemeAdvisories(deps *DoctorDeps, loader theme.Loader) []advisory {
	if deps.PrefsStore == nil {
		return nil
	}

	keys, _ := deps.PrefsStore.LoadThemeKeys()
	setting, raw := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)

	var advisories []advisory
	for _, nomination := range persistedThemeNominations(setting, raw) {
		if a, reported := persistedThemeAdvisory(loader, nomination, deps.ThemesDir); reported {
			advisories = append(advisories, a)
		}
	}
	return advisories
}

// persistedThemeNomination is one persisted slug doctor checks, carrying the
// §14A slot label it renders under — EMPTY under a constant, where the
// parenthetical is omitted entirely rather than filled with a placeholder.
type persistedThemeNomination struct {
	slug string
	slot string
}

// persistedThemeNominations selects which of prefs.json's three keys doctor
// checks, per §8.4: THE KEYS IN FORCE, never every key present.
//
// The Setting says which state §8.2's tiebreak settled on; the RAW keys say
// which values are actually PERSISTED. Both are needed and neither substitutes
// for the other:
//
//   - A CONSTANT is checked alone, with no slot. The slots are not read at all
//     under §8.2's `theme`-wins rule — a hand-edited file may legally carry all
//     three keys — and reporting one Portal is not reading would send the user
//     to fix something that has no effect.
//   - Otherwise ONLY THE SLOTS WITH A NON-EMPTY RAW VALUE are checked. An unset
//     slot arrives in the Setting as the shipped default, which is a built-in
//     and always resolves, so checking it could only ever produce a line about
//     §7.6's should-never-happen state — which is a fatal, not an advisory. The
//     raw value is what distinguishes "the user chose this" from "we substituted
//     it", and only the former is reportable.
//
// Two raw slots naming the SAME slug collapse to one `both` nomination (§9.5,
// reachable in two keypresses), so one slug yields one line per §12.2 — the rule
// two lines for one slug would break, along with <M>'s problems-not-detections
// property.
func persistedThemeNominations(setting theme.Setting, raw theme.RawKeys) []persistedThemeNomination {
	if setting.IsConstant {
		return []persistedThemeNomination{{slug: setting.Constant}}
	}

	if raw.Light != "" && raw.Light == raw.Dark {
		return []persistedThemeNomination{{slug: raw.Light, slot: themeSlotBoth}}
	}

	var nominations []persistedThemeNomination
	if raw.Light != "" {
		nominations = append(nominations, persistedThemeNomination{slug: raw.Light, slot: themeSlotLight})
	}
	if raw.Dark != "" {
		nominations = append(nominations, persistedThemeNomination{slug: raw.Dark, slot: themeSlotDark})
	}
	return nominations
}

// persistedThemeAdvisory resolves one nomination and renders its advisory,
// reporting whether it earns one at all. A nil rejection produces no line — this
// producer reports problems, not inventory.
//
// EVERY discrimination is ResolveByName's own and none is re-derived here, which
// is what keeps doctor's vocabulary identical to the panel's and to the log's: a
// charset failure is `bad name` and is decided BEFORE any path is composed (§8.6
// — so a hand-edited `../evil` never becomes a path component), an absent
// directory or an absent file is `not found`, and an unusable directory is
// `unreadable` because permissions is the actual problem (§5.5). An EMPTY
// themesDir — the unresolved-path degradation — still resolves the embedded set
// and answers `not found` for a drop-in slug, composing no path, which is why
// this producer runs where the directory scan skips.
//
// The slug renders CONTROL-STRIPPED BUT UNTRUNCATED. Stripping already happened
// at the point the value was read (§9.5 puts it on the value, not on the
// surface), and truncation stays panel-local because doctor has the full width
// and wants the whole value.
//
// slug and fromPrefs ride alongside the line for §12.2's one-slug-one-line
// union, where a persisted line OUTRANKS the same slug's file-validity line: it
// carries strictly more — the reason AND which slot is affected.
func persistedThemeAdvisory(loader theme.Loader, nomination persistedThemeNomination, themesDir string) (advisory, bool) {
	_, rejection := loader.ResolveByName(nomination.slug, themesDir)
	if rejection == nil {
		return advisory{}, false
	}

	return advisory{
		line:      fmt.Sprintf(persistedThemeAdvisoryFormat, nomination.slug, persistedThemeSlotSuffix(nomination.slot), rejection.Reason),
		slug:      nomination.slug,
		fromPrefs: true,
	}, true
}

// persistedThemeSlotSuffix renders §14A's slot parenthetical, or nothing at all
// under a constant. The empty label is the constant's, and it yields an empty
// string rather than "()" — the parenthetical is omitted ENTIRELY, because §8.2's
// constant state has no halves for one to name.
func persistedThemeSlotSuffix(slot string) string {
	if slot == "" {
		return ""
	}
	return fmt.Sprintf(persistedThemeSlotFormat, slot)
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
// test, so the one this producer does NOT own is visibly skipped rather than
// silently swept into the generic frame: `not found` applies to a persisted slug
// with no file, where nothing was enumerated.
//
// The two FILENAME reasons take the frames declared above, and their identity
// fields differ from the generic arm's in the one way §12.2's union depends on. A
// `bad name` row carries NO slug — the zero value, stated here rather than copied
// from the entry, because "a bad-name file can never collide with a persisted
// slug" is a consequence of the REASON (§6.2 rung 1 yields no usable identity)
// and must not rest on an upstream field happening to be empty. A `reserved name`
// row carries its slug: it has a valid one, and that is precisely what collided.
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
	case theme.ReasonBadName:
		return advisory{
			line:      badNameAdvisoryLine(entry),
			slug:      "",
			fromPrefs: false,
		}, true
	case theme.ReasonReservedName:
		return advisory{
			line:      fmt.Sprintf(reservedNameAdvisoryFormat, entry.Filename, entry.Slug, entry.Slug),
			slug:      entry.Slug,
			fromPrefs: false,
		}, true
	default:
		// `not found` is persistedThemeAdvisories' line, below: it applies to a
		// persisted slug with no file, which nothing enumerated here can be.
		return advisory{}, false
	}
}

// badNameAdvisoryLine picks between §14A's two `bad name` lines on Phase 1's
// cause, both labelled by the FILENAME AS ENUMERATED — never the full path, which
// §14A's `<filename>` placeholder excludes and which would spend the width these
// frames exist to use on a directory the user already named.
//
// The extension cause is the one discriminated explicitly, and the slug cause is
// what everything else renders as. That asymmetry is deliberate rather than a
// coin toss: the extension message asserts something SPECIFIC — that the stem is
// already fine and only the extension is not — so it is claimed only where Phase
// 1 says exactly that, while the slug message is the general statement about a
// name that is not usable as an identity. The third cause value (BadNameNone) is
// unreachable here, both causes being set by the one constructor that builds this
// reason.
func badNameAdvisoryLine(entry theme.Entry) string {
	if entry.Rejection.BadNameCause == theme.BadNameExtension {
		return fmt.Sprintf(badNameExtensionAdvisoryFormat, entry.Filename)
	}
	return fmt.Sprintf(badNameSlugAdvisoryFormat, entry.Filename)
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
