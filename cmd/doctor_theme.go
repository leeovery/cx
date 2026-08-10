package cmd

import (
	"fmt"
	"slices"

	"github.com/leeovery/portal/internal/theme"
)

// The pinned copy this file produces, one constant per frame. They are whole
// lines including the leading "⚠ ": the advisory renderer only indents what a
// producer hands it (see the advisory type in cmd/doctor.go), so a producer owns
// the whole string.
//
// The file frame is generic across every reason that has a slug in hand —
// `missing tokens`, `bad colour`, `bad syntax`, `unreadable` — because the loader
// already guarantees exactly one reason per file and fixes the detail's shape per
// reason. Doctor frames the reason and the detail the loader produced and
// re-derives nothing.
//
// The three filename frames below are the exception: the two filename reasons are
// decided from the name before the file is opened, so the frame is labelled
// `⚠ theme file <filename>:` rather than `⚠ theme <slug>:`. That differing frame
// carries the input class — a file versus a slug — which the panel row has no
// width to discriminate.
//
// The two `bad name` causes take distinct messages off the loader's BadNameCause
// because their fixes differ: with a wrong-cased extension the slug portion is
// already legal, so a single message telling the user to fix their slug would
// send them to correct the one thing that is fine.
//
// `reserved name` is labelled by filename too, despite being the one filename
// reason that has a valid slug, because that slug is identical to the built-in's:
// labelling by slug would print the same name twice with no way to tell which row
// is the user's file. Its line also does not follow the generic
// `<reason> — <detail>` frame — it names the conflict and the fix, which makes
// the rename workaround self-documenting.
//
// The persisted frame is one format with an optional slot insert rather than two
// whole lines, since the two renderings differ in exactly that insert. It carries
// no ` — <detail>` tail, unlike the file frame above: the reason is the whole
// answer for a slug that names nothing, and the one reason with a detail to give
// (`unreadable`) is about the directory, which already has its own line.
const (
	themeFileAdvisoryFormat   = "⚠ theme %s: %s — %s"
	themesDirUnreadableFormat = "⚠ themes directory unreadable: %s"

	badNameSlugAdvisoryFormat      = "⚠ theme file %s: slug must be lowercase letters, digits and hyphens"
	badNameExtensionAdvisoryFormat = "⚠ theme file %s: extension must be lowercase .theme"
	reservedNameAdvisoryFormat     = "⚠ theme file %s: %s is a built-in — rename it (e.g. %s-mine.theme)"

	persistedThemeAdvisoryFormat = "⚠ theme %s%s does not resolve: %s"
	persistedThemeSlotFormat     = " (%s)"
)

// The two slot labels doctor renders, read off theme.Slot's own name mapping
// rather than restated: the parenthetical a user reads here and the `slot` attr
// the log carries are one vocabulary, and a literal pair would be free to drift
// from it silently.
var (
	themeSlotLight, _ = theme.SlotLight.AttrName()
	themeSlotDark, _  = theme.SlotDark.AttrName()
)

// themeSlotBoth is not a third slot — the setting has exactly two — it is the
// label for one slug occupying both of them, an ordinary state two keypresses in
// the theme panel reach. Having no slot to be named by, it is declared here
// beside the pair it joins rather than derived.
const themeSlotBoth = "both"

// themeAdvisory is one line of doctor's theme block while it is still being
// assembled — the rendered copy plus the identity the assembly turns on.
//
// slug and fromPrefs are the one-slug-one-line union's dedup identity: an
// unresolvable persisted slug outranks the same slug's file-validity line, so a
// line must say which slug it is about and which producer it came from. Both
// live here rather than on advisory so the rule is reachable only from the
// producers that participate in it — a producer that knows nothing of the union
// has no identity field to leave unset and therefore cannot defeat it.
type themeAdvisory struct {
	line      string
	slug      string
	fromPrefs bool
}

// collectThemeAdvisories is doctor's whole theme-advisory surface: the entry
// point the report's advisory block is built from, run once per diagnosis pass,
// over the two producers behind it — the themes-directory scan (what is in a
// directory) and the persisted-theme read (what the user picked).
//
// "Once per diagnosis pass" is literal, and `portal doctor --fix` is where it
// bites: that path runs two passes and calls this freshly beside each render.
// Collecting once and handing the same slice to both renders would pair a stale
// advisory block with freshly-read check lines, so one report would describe two
// different moments.
//
// It is strictly read-only — no write, no repair, no directory creation — which
// lets it run unchanged on the `--fix` path, where there is no repair to perform
// and suppressing it would make `--fix` a less informative diagnosis than a plain
// run. Being read-only is also what makes the second call free of consequence:
// runDoctorFix touches no theme state, so nothing between the passes can change
// its answer.
//
// The loader is the silent one on every doctor path: the `theme` component
// records where a theme is used, never where one is diagnosed. Doctor's whole
// output is already the diagnostic the user is reading, and it is the run most
// likely to hit a full reject set, so emitting here would put the largest
// possible WARN volume on the surface that needs it least — and would make a
// read-only diagnosis write about the state it just printed.
//
// The two producers share one loader, built here and passed down, so the whole
// diagnosis has a single owned dedup set rather than two that could each report
// the same condition.
//
// Their two results are assembled rather than concatenated — see
// assembleThemeAdvisories — so the block the renderer receives is already the
// deduplicated union, in a pinned order.
//
// This is also the boundary where the union's dedup identity is dropped: the
// renderer prints lines, so it is handed lines.
func collectThemeAdvisories(deps *DoctorDeps) []advisory {
	assembled := themeAdvisoryUnion(deps)

	block := make([]advisory, 0, len(assembled))
	for _, a := range assembled {
		block = append(block, advisory{line: a.line})
	}
	return block
}

// themeAdvisoryUnion runs both producers and the assembly between them,
// yielding the union while it still carries the identity that assembly turned
// on.
func themeAdvisoryUnion(deps *DoctorDeps) []themeAdvisory {
	loader := theme.NewSilentLoader()

	return assembleThemeAdvisories(scanThemesDirectory(loader, deps.ThemesDir), persistedThemeAdvisories(deps, loader))
}

// assembleThemeAdvisories unions doctor's two theme producers into the one block
// the report renders, in three pinned regions — the directory line, the file
// lines, then the persisted lines.
//
// The region order is what makes the report reproducible: a block whose sequence
// depended on which producer appended first would shift between runs and read as
// noise. Directory → files → persisted reads outermost-to-innermost, and the
// directory line, when present, is the condition that explains the absence of
// every file line beneath it. The first two regions arrive in one slice because
// scanThemesDirectory yields one or the other and never both (an unusable
// directory enumerates nothing), and both are internally ordered by their
// producers: the enumeration's own os.ReadDir filename order, and the fixed prefs
// key order. Nothing is sorted here and no map is iterated anywhere in the
// assembly, so two runs over an unchanged directory and prefs.json render
// byte-identically.
//
// The drop is the one-slug-one-line rule, mirroring the panel's "one slug is one
// row, always" so the two surfaces cannot disagree about how many problems exist.
// When a persisted slug is the invalid file — the most likely failure of all —
// the persisted line wins: it carries strictly more, the reason and which slot is
// affected, so the advisory count reports problems rather than detections.
//
// Two structural non-collisions need no special case here:
//
//   - a `bad name` file has no slug (the name yields no usable identity, which
//     themeFileAdvisory states rather than copies), so the non-empty-slug guard
//     means it can never match a persisted slug and both lines legitimately
//     stand. The directory line carries no slug either, by the same guard.
//   - a persisted slug naming a `reserved name` file resolves to the built-in at
//     ResolveByName's step 2, so the persisted producer emits no line for it at
//     all — the file keeps its own line, and that collision is the entire content
//     of the reason.
func assembleThemeAdvisories(scanned, persisted []themeAdvisory) []themeAdvisory {
	covered := persistedSlugs(persisted)

	assembled := make([]themeAdvisory, 0, len(scanned)+len(persisted))
	for _, a := range scanned {
		if a.slug != "" && slices.Contains(covered, a.slug) {
			continue
		}
		assembled = append(assembled, a)
	}
	return append(assembled, persisted...)
}

// persistedSlugs collects the slugs the persisted lines carry — the set a file
// line is dropped against.
//
// Membership is decided by the record's own fromPrefs field rather than by which
// slice it arrived in: fromPrefs and slug are the union's declared dedup
// identity, and a rank read off the argument position would leave that identity
// unread and free to drift.
//
// A slice rather than a map: the set is at most two entries, and a slice cannot
// be iterated in a random order the way a map can, which keeps the assembly's
// determinism a property of its data structures.
func persistedSlugs(persisted []themeAdvisory) []string {
	slugs := make([]string, 0, len(persisted))
	for _, a := range persisted {
		if a.fromPrefs && a.slug != "" {
			slugs = append(slugs, a.slug)
		}
	}
	return slugs
}

// persistedThemeAdvisories is doctor's second theme-advisory producer: the one
// reporting that a theme the user chose no longer resolves — a deleted file, a
// renamed file, a typo in prefs.json. Portal falls back silently by design and
// never overwrites the persisted name, so without this line the only signal a
// user gets is "my colours changed".
//
// The prefs read is deps.PrefsStore, which resolveDoctorDeps builds through the
// non-migrating loadPrefsStoreNoMigrate. A nil store — the unresolvable-config-path
// degradation — produces no lines rather than an error: the advisory class has no
// not-evaluable form, and a path that could not be computed must never abort a
// diagnosis.
//
// The read is tolerant and its error is discarded on purpose. Every degenerate
// prefs.json — absent, empty, corrupt, unreadable, missing every key — yields
// zero keys and therefore zero lines. A diagnosis must not fail to diagnose
// because one of the files it reads is the broken one.
//
// Resolution goes through ResolveByName and never ResolveNomination: the latter
// substitutes the mode-matched fallbacks, hiding the very failure being reported,
// and can raise the broken-built-in fatal, aborting the diagnosis over a state
// this line exists to describe.
func persistedThemeAdvisories(deps *DoctorDeps, loader theme.Loader) []themeAdvisory {
	if deps.PrefsStore == nil {
		return nil
	}

	keys, _ := deps.PrefsStore.LoadThemeKeys()
	raw := theme.NewRawKeys(keys.Theme, keys.Light, keys.Dark)

	var advisories []themeAdvisory
	for _, nomination := range persistedThemeNominations(raw) {
		if a, reported := persistedThemeAdvisory(loader, nomination, deps.ThemesDir); reported {
			advisories = append(advisories, a)
		}
	}
	return advisories
}

// persistedThemeNomination is one persisted slug doctor checks, carrying the slot
// label it renders under — empty under a constant, where the parenthetical is
// omitted entirely rather than filled with a placeholder.
type persistedThemeNomination struct {
	slug string
	slot string
}

// persistedThemeNominations renders the keys in force as the nominations doctor
// checks: theme.InForceKeys decides WHICH keys those are, and this puts a label
// on each one.
//
// Labelling is all doctor does here. The `theme`-wins tiebreak, the rule that
// only a slot with a non-empty raw value is in force, and the collapse of two
// slots naming one value all belong to the selector, and none is restated or
// re-derived here. The label itself is doctor's own, being a rendering of where a
// value sits rather than a decision about which values are read.
func persistedThemeNominations(keys theme.RawKeys) []persistedThemeNomination {
	inForce := theme.InForceKeys(keys)

	nominations := make([]persistedThemeNomination, 0, len(inForce))
	for _, key := range inForce {
		nominations = append(nominations, persistedThemeNomination{slug: key.Value, slot: persistedThemeSlotLabel(key)})
	}
	return nominations
}

// persistedThemeSlotLabel is the label one in-force key renders under: `both`
// where a single value occupies the whole pair, else the slot's own name.
//
// A constant yields the empty label, which persistedThemeSlotSuffix renders as no
// parenthetical at all rather than as a placeholder — the constant state has no
// halves for a label to name.
func persistedThemeSlotLabel(key theme.InForceKey) string {
	if key.Both {
		return themeSlotBoth
	}

	switch key.Slot {
	case theme.SlotLight:
		return themeSlotLight
	case theme.SlotDark:
		return themeSlotDark
	default:
		return ""
	}
}

// persistedThemeAdvisory resolves one nomination and renders its advisory,
// reporting whether it earns one at all. A nil rejection produces no line — this
// producer reports problems, not inventory.
//
// Every discrimination is ResolveByName's own and none is re-derived here, which
// keeps doctor's vocabulary identical to the panel's and to the log's: a charset
// failure is `bad name` and is decided before any path is composed — so a
// hand-edited `../evil` never becomes a path component — an absent directory or
// an absent file is `not found`, and an unusable directory is `unreadable`
// because permissions is the actual problem. An empty themesDir — the
// unresolved-path degradation — still resolves the embedded set and answers
// `not found` for a drop-in slug, composing no path, which is why this producer
// runs where the directory scan skips.
//
// The slug renders control-stripped but untruncated: stripping is a property of
// the value rather than of the surface, and truncation stays panel-local because
// doctor has the full width and wants the whole value.
//
// slug and fromPrefs ride alongside the line for the one-slug-one-line union,
// where a persisted line outranks the same slug's file-validity line.
func persistedThemeAdvisory(loader theme.Loader, nomination persistedThemeNomination, themesDir string) (themeAdvisory, bool) {
	_, rejection := loader.ResolveByName(nomination.slug, themesDir)
	if rejection == nil {
		return themeAdvisory{}, false
	}

	return themeAdvisory{
		line:      fmt.Sprintf(persistedThemeAdvisoryFormat, nomination.slug, persistedThemeSlotSuffix(nomination.slot), rejection.Reason),
		slug:      nomination.slug,
		fromPrefs: true,
	}, true
}

// persistedThemeSlotSuffix renders the slot parenthetical, or nothing at all
// under a constant. The empty label yields an empty string rather than "()":
// the constant state has no halves for a parenthetical to name.
func persistedThemeSlotSuffix(slot string) string {
	if slot == "" {
		return ""
	}
	return fmt.Sprintf(persistedThemeSlotFormat, slot)
}

// scanThemesDirectory enumerates dir through the loader's rejection ladder and
// renders one advisory per finding: the directory's own verdict where it has one,
// else one line per rejected file.
//
// An unresolved path — themesDirPath() failed, so resolveDoctorDeps left the
// field empty — skips the scan entirely and yields nothing. The advisory class
// has no not-evaluable form, so degrading to zero lines is the only shape
// available. The skip is scoped here rather than at collectThemeAdvisories, so
// producers that need no path still run.
//
// The two Enumerate returns separate the three directory states, and each gets a
// different answer:
//
//   - an unusable directory (unreadable, or a regular file where a directory
//     belongs) → its one pinned line. Enumerate returns no entries in that state,
//     so it is the only theme-file line the scan can produce.
//   - an absent directory → nothing at all: no line, no error, no log. Zero
//     drop-ins is not an error and Portal never creates or seeds the directory.
//   - a usable directory → one line per rejected entry, in the enumeration's own
//     deterministic filename order.
func scanThemesDirectory(loader theme.Loader, dir string) []themeAdvisory {
	if dir == "" {
		return nil
	}

	entries, dirRejection := loader.Enumerate(dir)
	if dirRejection != nil {
		return []themeAdvisory{{line: fmt.Sprintf(themesDirUnreadableFormat, dir)}}
	}

	var advisories []themeAdvisory
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
// not inventory. A rejected one produces exactly one line, for the one reason the
// ladder settled on: doctor enumerates within the reason and never across, so a
// file is never reported as both `bad colour` and `missing tokens`.
//
// The switch is exhaustive over the reasons rather than a bare "has a slug" test,
// so the one this producer does not own is visibly skipped rather than silently
// swept into the generic frame: `not found` applies to a persisted slug with no
// file, where nothing was enumerated.
//
// The two filename reasons take the frames declared above, and their identity
// fields differ from the generic arm's in the way the union depends on. A
// `bad name` row carries no slug — the zero value, stated here rather than copied
// from the entry, because "a bad-name file can never collide with a persisted
// slug" is a consequence of the reason and must not rest on an upstream field
// happening to be empty. A `reserved name` row carries its slug: it has a valid
// one, and that is precisely what collided.
//
// slug and fromPrefs are populated alongside the line because they are the
// identity the one-slug-one-line union dedups on.
func themeFileAdvisory(entry theme.Entry) (themeAdvisory, bool) {
	if entry.Rejection == nil {
		return themeAdvisory{}, false
	}

	switch entry.Rejection.Reason {
	case theme.ReasonMissingTokens, theme.ReasonBadColour, theme.ReasonBadSyntax, theme.ReasonUnreadable:
		return themeAdvisory{
			line:      fmt.Sprintf(themeFileAdvisoryFormat, entry.Slug, entry.Rejection.Reason, rejectionDetail(entry.Rejection)),
			slug:      entry.Slug,
			fromPrefs: false,
		}, true
	case theme.ReasonBadName:
		return themeAdvisory{
			line:      badNameAdvisoryLine(entry),
			slug:      "",
			fromPrefs: false,
		}, true
	case theme.ReasonReservedName:
		return themeAdvisory{
			line:      fmt.Sprintf(reservedNameAdvisoryFormat, entry.Filename, entry.Slug, entry.Slug),
			slug:      entry.Slug,
			fromPrefs: false,
		}, true
	default:
		// `not found` is persistedThemeAdvisories' line, below: it applies to a
		// persisted slug with no file, which nothing enumerated here can be.
		return themeAdvisory{}, false
	}
}

// badNameAdvisoryLine picks between the two `bad name` lines on the loader's
// cause, both labelled by the filename as enumerated — never the full path, which
// the pinned copy excludes and which would spend the width these frames exist to
// use on a directory the user already named.
//
// The extension cause is discriminated explicitly and the slug cause is what
// everything else renders as, because the extension message asserts something
// specific — that the stem is already fine and only the extension is not — so it
// is claimed only where the loader says exactly that, while the slug message is
// the general statement about a name that is not usable as an identity.
// BadNameNone is unreachable here, both causes being set by the one constructor
// that builds this reason.
func badNameAdvisoryLine(entry theme.Entry) string {
	if entry.Rejection.BadNameCause == theme.BadNameExtension {
		return fmt.Sprintf(badNameExtensionAdvisoryFormat, entry.Filename)
	}
	return fmt.Sprintf(badNameSlugAdvisoryFormat, entry.Filename)
}

// rejectionDetail is the loader's own detail, carried verbatim: nothing is
// re-derived, re-ordered, re-wrapped or double-prefixed, because the loader
// already renders each reason in the exact form its surfaces print — `missing
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
