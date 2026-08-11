package cmd

import (
	"fmt"
	"slices"

	"github.com/leeovery/portal/internal/theme"
)

// Whole lines including the leading "⚠ " — the advisory renderer only indents
// what a producer hands it.
const (
	themeFileAdvisoryFormat   = "⚠ theme %s: %s — %s"
	themesDirUnreadableFormat = "⚠ themes directory unreadable: %s"

	badNameSlugAdvisoryFormat      = "⚠ theme file %s: slug must be lowercase letters, digits and hyphens"
	badNameExtensionAdvisoryFormat = "⚠ theme file %s: extension must be lowercase .theme"
	reservedNameAdvisoryFormat     = "⚠ theme file %s: %s is a built-in — rename it (e.g. %s-mine.theme)"

	persistedThemeAdvisoryFormat = "⚠ theme %s%s does not resolve: %s"
	persistedThemeSlotFormat     = " (%s)"
)

// themeSlotBoth is not a third slot — it labels one slug occupying both halves
// of the pair, which no slot can name.
const themeSlotBoth = "both"

// slug and fromPrefs are the dedup identity the assembly keys on.
type themeAdvisory struct {
	line      string
	slug      string
	fromPrefs bool
}

// collectThemeAdvisories must be called freshly beside each report render —
// `doctor --fix` renders two, and reusing one slice would pair a stale advisory
// block with freshly-read check lines. Read-only; the loader stays silent (the
// `theme` component records use, never diagnosis).
func collectThemeAdvisories(deps *DoctorDeps) []advisory {
	assembled := themeAdvisoryUnion(deps)

	block := make([]advisory, 0, len(assembled))
	for _, a := range assembled {
		block = append(block, advisory{line: a.line})
	}
	return block
}

// The directory is read once and the retained enumeration drives both
// producers, so the file line and persisted line about one slug describe one
// parse of one file.
func themeAdvisoryUnion(deps *DoctorDeps) []themeAdvisory {
	loader := theme.NewSilentLoader()
	enumeration := enumerateThemesDir(loader, deps.ThemesDir)

	return assembleThemeAdvisories(scanThemesDirectory(enumeration), persistedThemeAdvisories(deps, loader, enumeration))
}

func enumerateThemesDir(loader theme.Loader, dir string) theme.Enumeration {
	if dir == "" {
		return theme.Enumeration{}
	}

	entries, dirRejection := loader.Enumerate(dir)
	return theme.Enumeration{Entries: entries, DirUnusable: dirRejection != nil, DirPath: dir}
}

// Region order is pinned (directory, files, persisted) and nothing here sorts
// or iterates a map, so two runs over unchanged inputs render byte-identically.
// The drop is one-slug-one-line: a persisted line wins over the same slug's
// file line because it carries strictly more (reason and slot).
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

// A slice, not a map: it cannot be iterated in a random order, keeping the
// assembly's determinism a property of its data structures.
func persistedSlugs(persisted []themeAdvisory) []string {
	slugs := make([]string, 0, len(persisted))
	for _, a := range persisted {
		if a.fromPrefs && a.slug != "" {
			slugs = append(slugs, a.slug)
		}
	}
	return slugs
}

// The store read's error is discarded on purpose: every degenerate prefs.json
// yields zero keys and therefore zero lines — a diagnosis must not fail because
// one of the files it reads is the broken one. Resolution goes by name and
// never through ResolveNomination, which substitutes fallbacks (hiding the
// failure being reported) and can raise the broken-built-in fatal.
func persistedThemeAdvisories(deps *DoctorDeps, loader theme.Loader, enumeration theme.Enumeration) []themeAdvisory {
	if deps.PrefsStore == nil {
		return nil
	}

	keys, _ := deps.PrefsStore.LoadThemeKeys()
	raw := theme.NewRawKeys(keys.Theme, keys.Light, keys.Dark)

	nominations := persistedThemeNominations(raw)

	advisories := make([]themeAdvisory, 0, len(nominations))
	for _, nomination := range nominations {
		if a, reported := persistedThemeAdvisory(loader, enumeration, nomination); reported {
			advisories = append(advisories, a)
		}
	}
	return advisories
}

type persistedThemeNomination struct {
	slug string
	slot string
}

func persistedThemeNominations(keys theme.RawKeys) []persistedThemeNomination {
	inForce := theme.InForceKeys(keys)

	nominations := make([]persistedThemeNomination, 0, len(inForce))
	for _, key := range inForce {
		nominations = append(nominations, persistedThemeNomination{slug: key.Value, slot: persistedThemeSlotLabel(key)})
	}
	return nominations
}

func persistedThemeSlotLabel(key theme.InForceKey) string {
	if key.Both {
		return themeSlotBoth
	}

	name, _ := key.Slot.AttrName()
	return name
}

// The slug renders control-stripped but untruncated: truncation stays
// panel-local, doctor has the full width.
func persistedThemeAdvisory(loader theme.Loader, enumeration theme.Enumeration, nomination persistedThemeNomination) (themeAdvisory, bool) {
	_, rejection := loader.ResolveByNameFrom(enumeration, nomination.slug)
	if rejection == nil {
		return themeAdvisory{}, false
	}

	return themeAdvisory{
		line:      fmt.Sprintf(persistedThemeAdvisoryFormat, nomination.slug, persistedThemeSlotSuffix(nomination.slot), rejection.Reason),
		slug:      nomination.slug,
		fromPrefs: true,
	}, true
}

func persistedThemeSlotSuffix(slot string) string {
	if slot == "" {
		return ""
	}
	return fmt.Sprintf(persistedThemeSlotFormat, slot)
}

// An absent directory or unresolved path produces nothing at all: zero drop-ins
// is not an error, and Portal never creates or seeds the directory.
func scanThemesDirectory(enumeration theme.Enumeration) []themeAdvisory {
	if enumeration.DirUnusable {
		return []themeAdvisory{{line: fmt.Sprintf(themesDirUnreadableFormat, enumeration.DirPath)}}
	}

	advisories := make([]themeAdvisory, 0, len(enumeration.Entries))
	for _, entry := range enumeration.Entries {
		if a, reported := themeFileAdvisory(entry); reported {
			advisories = append(advisories, a)
		}
	}
	return advisories
}

// The switch is exhaustive rather than a has-a-slug test so the reason this
// producer does not own is visibly skipped. A `bad name` row's empty slug is
// stated here, not copied from the entry: the never-collides-with-a-persisted-
// slug property must not rest on an upstream field happening to be empty.
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
		// `not found` belongs to the persisted producer: it applies to a slug
		// with no file, which nothing enumerated here can be.
		return themeAdvisory{}, false
	}
}

// BadNameNone is unreachable: both causes are set by the constructor that
// builds this reason.
func badNameAdvisoryLine(entry theme.Entry) string {
	if entry.Rejection.BadNameCause == theme.BadNameExtension {
		return fmt.Sprintf(badNameExtensionAdvisoryFormat, entry.Filename)
	}
	return fmt.Sprintf(badNameSlugAdvisoryFormat, entry.Filename)
}

// The Err fallback is for `unreadable` alone — reading the structured error
// where the rendered one is absent keeps the OS's message the detail without
// ever rendering it twice.
func rejectionDetail(rejection *theme.Rejection) string {
	if rejection.Detail == "" && rejection.Err != nil {
		return rejection.Err.Error()
	}
	return rejection.Detail
}
