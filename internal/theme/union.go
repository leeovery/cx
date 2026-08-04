package theme

import (
	"cmp"
	"slices"
	"sort"
	"strings"
)

// RowSource says where one union row came from: the embedded set, the themes
// directory, or prefs.json naming something neither of those answers to.
//
// It exists for ORDERING and for nothing else (§9.5's tie-break, task 8-2): a
// built-in row is otherwise deliberately indistinguishable from a valid drop-in,
// so nothing here may reach a row's rendered content.
//
// SourceBuiltin is the zero value, which is the always-present source rather than
// an "unset" sentinel — every union has built-in rows, and a Row always came from
// somewhere (the Slot precedent in resolution.go).
type RowSource int

const (
	// SourceBuiltin is a row for one of the embedded built-ins (§7.1).
	SourceBuiltin RowSource = iota
	// SourceFile is a row for one candidate found in the themes directory,
	// valid or not (§5.6).
	SourceFile
	// SourcePersisted is a row for a slug named in prefs.json that resolves to
	// NEITHER — the row that gives §9.4's `●` marker something to sit on when
	// the theme it marks has gone.
	SourcePersisted
)

// Row is one line of §9.4's list: one theme, or one persisted value that names
// no theme.
//
// The identity fields are three because §9.5 gives a row three different things
// to be labelled and sorted by, and exactly one of them applies per row:
//
//   - Slug — every row that HAS an identity. Absent only where the name yields
//     none: a `bad name` file (§6.2 rung 1) and a charset-rejected persisted
//     string.
//   - Filename — set iff SourceFile, because §9.5 labels a `bad name` row and a
//     `reserved name` row by filename (the first has no slug; the second's slug
//     is identical to the built-in's it collides with).
//   - Persisted — the raw persisted value, set iff the row is a SourcePersisted
//     one that yielded no slug. It is the only thing such a row can be labelled
//     or sorted by, and it arrives already control-stripped (§9.5 puts the
//     removal on the value, at the point it was read).
//
// Theme is populated IFF Rejection is nil, exactly as the loader's Result is: a
// rejected row never comes back half-populated. Holding the palette on the row is
// what makes the panel's preview an O(1) restyle from values already in hand
// (§5.8) rather than a file read per keystroke.
//
// Rejection is the ONE §6.2 reason the row is not usable, carried unchanged from
// whoever produced it — the ladder for a file, this assembly for a persisted
// value that answers to nothing. Nothing here re-derives, re-orders or re-words
// it.
type Row struct {
	Slug      string
	Filename  string
	Persisted string
	Source    RowSource
	Theme     Theme
	Rejection *Rejection
}

// Selectable reports whether the panel may commit this row: valid rows only
// (§9.5), which is the same fact as carrying no rejection.
func (r Row) Selectable() bool {
	return r.Rejection == nil
}

// SortKey is §9.5's ordering value: the slug wherever one exists, else the
// filename, else the persisted string itself.
//
// It is FULLY DETERMINED — every row shape yields exactly one non-empty key —
// and that is what makes the order total (see sortRows). The three arms are the
// identity fields in the one order that leaves no row unplaceable:
//
//   - The SLUG, for every row that has one. Including a `reserved name` row,
//     whose slug is valid and identical to the built-in's it collides with:
//     sorting on it is what stands the row explaining the collision immediately
//     beside the thing it collides with, which is the whole of §9.5's adjacency
//     argument — even though that row is LABELLED by its filename. A `not found`
//     persisted row sorts by its slug too.
//   - The FILENAME, for the one row shape that has no slug: a `bad name` file,
//     since §5.2 rejects rather than normalises and so mints none for it.
//   - The PERSISTED STRING, for a charset-rejected persisted value, which has
//     neither a slug nor a file. There is EXACTLY ONE thing to sort such a row
//     by, and using it is what keeps the ordering total rather than leaving a
//     member the comparator cannot place. The value arrives already
//     control-stripped (§9.5 puts the removal on the value, at the point it was
//     read); §9.5's truncation is a render concern and never reaches this key.
//
// IT IS NOT DERIVED FROM Label, AND Label IS NOT DERIVED FROM IT — see Label.
func (r Row) SortKey() string {
	return cmp.Or(r.Slug, r.Filename, r.Persisted)
}

// Label is §9.5's DISPLAY value, and it is deliberately a SEPARATE VALUE from
// SortKey rather than a second reading of it.
//
// NEITHER MAY BE RE-DERIVED FROM THE OTHER AT RENDER TIME: the row delegate
// (task 8-4) consumes Label and nothing else, the ordering consumes SortKey and
// nothing else. The two disagree on exactly the rows §9.5 wants them to disagree
// on:
//
//   - A `reserved name` row is labelled by its FILENAME while sorting by its
//     slug — `nord.theme` beside `nord` tells the user which one is theirs,
//     where two rows reading `nord` would not.
//   - A `bad name` file is labelled by its filename because it HAS no slug.
//   - A charset-rejected persisted value is labelled by the raw string, which is
//     the only thing it has: no slug was derived and no file was ever sought.
//   - Every other row — valid, or rejected for anything its NAME did not cause —
//     is labelled by its slug.
func (r Row) Label() string {
	if r.labelledByFilename() {
		return r.Filename
	}
	return cmp.Or(r.Slug, r.Persisted)
}

// labelledByFilename reports whether §9.5 labels this row by its filename rather
// than by the slug it would otherwise be listed under.
//
// Two reasons and no others, and both of them a FILE's: `bad name`, which yields
// no slug at all, and `reserved name`, whose slug is the built-in's. The
// filename check is what keeps the charset-rejected PERSISTED row out of this
// arm — it is `bad name` by reason too, but has no file behind it and so is
// labelled by the raw value instead.
func (r Row) labelledByFilename() bool {
	if r.Rejection == nil || r.Filename == "" {
		return false
	}
	return r.Rejection.Reason == ReasonBadName || r.Rejection.Reason == ReasonReservedName
}

// Enumeration is one directory read, RETAINED: what the themes directory held
// and what the directory itself was.
//
// It is kept apart from the Union it produces because the two have different
// lifetimes (§5.8): the parse results are held for the panel's lifetime so
// arrowing previews from values already in hand, while the union is re-derived
// from them whenever the persisted state changes — §9.2's post-commit recompute
// and §5.8's `Esc` re-resolution both re-run the derivation against the SAME
// enumeration, with no fresh read.
type Enumeration struct {
	// Entries is every candidate the directory held, classified — valid or not
	// (§9.4). Empty for an absent directory and for an unusable one alike.
	Entries []Entry

	// DirUnusable is §5.5's directory verdict: true for an unreadable directory
	// or a regular file where a directory belongs, false for an absent one.
	//
	// It is a BOOL rather than the rejection itself because the two consumers
	// need only the condition: the pinned `⚠ dir unreadable` chrome row (§9.5)
	// has no room for a detail, and a persisted slug made unreachable by the
	// directory carries the bare reason `unreadable` (§5.5). The OS error
	// verbatim belongs to doctor, which reads the directory itself.
	DirUnusable bool

	// DirPath is the directory this enumeration read, whatever the outcome — so
	// a retained enumeration says which directory it is OF rather than only what
	// it found.
	DirPath string
}

// Union is §9.4's finished row set: every built-in, every file in the themes
// directory, and every persisted slug resolving to neither — deduped one slug,
// one row, each row already carrying its single §6.2 reason.
//
// It is an ORDINARY VALUE with exported fields and no method that reads the
// filesystem, so a fixture can fake one wholesale with no loader and no directory
// (§13.3) — which is the only way internal/capture, under its no-real-config
// import guard, can render an invalid-theme row at all.
type Union struct {
	// Rows is the union in §9.5's display order — alphabetical by sort key,
	// case-insensitively, with the built-in ahead of the one row guaranteed to
	// tie with it (see sortRows).
	//
	// The order is applied by the ASSEMBLER rather than by the panel, so every
	// consumer receives it ordered and none can forget to sort. Enumeration
	// order — built-ins, then os.ReadDir's, then the persisted leftovers — is
	// neither alphabetical nor stable across filesystems, so it is not something
	// a consumer could safely be handed.
	Rows []Row

	// DirUnusable drives §9.5's pinned `⚠ dir unreadable` chrome row. It is a
	// FLAG rather than a member of Rows because that row is viewport chrome, not
	// a list row: a list row participates in pagination and would vanish the
	// moment the user paged down.
	DirUnusable bool

	// Count is len(Rows) — what `theme: enumerated` reports (§12.3). The chrome
	// row above is not a union member and is never counted.
	Count int

	// Rejected is the unselectable subset: the rows carrying a §6.2 reason.
	Rejected int
}

// Open is the panel's entry point (§13.3): ONE directory read, producing the
// retained enumeration and the finished union together.
//
// The read happens here and NOWHERE ELSE on this path, which is what §5.8's
// "re-read on every open, retain for the panel's lifetime" means mechanically —
// the panel calls this when it opens and Reassemble for everything after.
//
// §5.5's directory-state table separates the two Enumerate returns: an unusable
// directory sets the flag and contributes no entries, while an absent one is
// silent and simply contributed none. Neither is an error, because neither stops
// the panel opening — the built-ins are always there to list.
//
// It emits `theme: enumerated` exactly ONCE per call, from the one place both its
// attrs are computable (§12.3): the count is rows produced and the rejected count
// is the unselectable subset, and neither is knowable before the merge. It fires
// on an absent directory and on an unusable one alike — the panel opened either
// way, which is what the event records — and it does NOT dedup, unlike the WARNs
// Enumerate emits beneath it.
func (l Loader) Open(themesDir string, keys RawKeys) (Enumeration, Union) {
	entries, rejection := l.Enumerate(themesDir)
	enumeration := Enumeration{Entries: entries, DirUnusable: rejection != nil, DirPath: themesDir}

	union := l.Reassemble(enumeration, keys)
	l.events.Enumerated(union.Count, union.Rejected)

	return enumeration, union
}

// Reassemble re-derives the union from a RETAINED enumeration and the current
// persisted keys.
//
// It performs NO I/O of any kind and emits NOTHING: it is a pure function of its
// two arguments, and that is a requirement rather than a property it happens to
// have. §9.2's post-commit recompute and §5.8's `Esc` re-resolution both re-run
// it with changed prefs state, and re-reading the directory there would both cost
// a syscall per keypress and let the list change under a user who only pressed
// `Enter`.
//
// The order below is fixed, and each step is why the one before it cannot dedup
// it away:
//
//  1. One row per built-in. Always valid — §7.6's build-time test is what makes
//     that true — and carrying no marker of any kind, since §9.5 makes a built-in
//     row deliberately indistinguishable from a valid drop-in.
//  2. One row per enumerated file, carrying the entry's palette or its single
//     rejection unchanged. A `reserved name` file stands ALONGSIDE the built-in
//     it collides with — the one legitimate two-rows-for-one-slug case, because
//     that collision is the reason's entire content. Every other file slug is
//     unique by construction (§5.6 mints no duplicate slug).
//  3. The persisted keys in force, each contributing a row only where nothing
//     above already answers to it — see persistedRows.
//  4. §9.5's order, applied HERE rather than by the panel, so both the Open path
//     and every recompute hand back an ordered union and no consumer can forget
//     to sort — see sortRows. It runs last because it is a pure rearrangement:
//     the three steps above decide MEMBERSHIP, and each one's dedup reads the
//     rows already assembled by slug rather than by position.
func (l Loader) Reassemble(e Enumeration, keys RawKeys) Union {
	rows := l.builtinRows()
	rows = append(rows, fileRows(e.Entries)...)
	rows = append(rows, persistedRows(rows, e, keys)...)
	sortRows(rows)

	return Union{Rows: rows, DirUnusable: e.DirUnusable, Count: len(rows), Rejected: countRejected(rows)}
}

// builtinRows is step 1: one row per embedded built-in, in BuiltinSlugs' sorted
// order.
//
// They are loaded rather than cached, on every derivation, for the reason §5.8
// gives for not caching anything else: the parse is of a handful of small files
// already in memory, and a cache is a second source of truth for a palette the
// panel is about to paint.
//
// A slug the injected byte source does not answer to yields NO ROW rather than an
// empty or rejected one — that source exists solely to stage §7.6's broken binary
// (see Loader.BuiltinSource), where "this built-in is not in this binary" is
// exactly what the union should say. A built-in that IS present and does not
// parse carries its rejection like any other row.
func (l Loader) builtinRows() []Row {
	slugs := BuiltinSlugs()

	rows := make([]Row, 0, len(slugs))
	for _, slug := range slugs {
		result, rejection, found := l.LoadBuiltin(slug)
		if !found {
			continue
		}
		rows = append(rows, Row{Slug: slug, Source: SourceBuiltin, Theme: result.Theme, Rejection: rejection})
	}
	return rows
}

// fileRows is step 2: one row per enumerated candidate, valid or not.
//
// EVERY candidate gets a row (§9.4). That is the whole promise of the drop-in
// route — a broken file is present and named, so the user sees "there's my theme,
// it's registered, but it's invalid" rather than being completely in the dark.
//
// Nothing is re-derived here: the slug, the filename, the palette and the single
// rejection all ride across from the entry exactly as the ladder produced them.
func fileRows(entries []Entry) []Row {
	rows := make([]Row, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, Row{
			Slug:      entry.Slug,
			Filename:  entry.Filename,
			Source:    SourceFile,
			Theme:     entry.Theme,
			Rejection: entry.Rejection,
		})
	}
	return rows
}

// persistedRows is step 3: the rows prefs.json contributes that nothing already
// listed answers to.
//
// THE RULE IS "RESOLVES", NOT "HAS A FILE" (§9.4). A persisted value matching a
// listed row's slug contributes NOTHING, because that row already IS its row —
// whether it is a built-in's, a valid drop-in's or a broken drop-in's. Keying on
// file existence instead would mint a second `⚠ not found` row for every
// persisted built-in slug, which is the state the panel's most common action
// produces (`Enter` on `tokyo-night`).
//
// The listed slugs are all valid by construction — a built-in's, or one
// SlugFromFilename derived — so a persisted value that is not a legal slug cannot
// match one by accident.
//
// Only what is IN FORCE is considered, and only what the user actually SET: see
// inForceValues.
func persistedRows(listed []Row, e Enumeration, keys RawKeys) []Row {
	var rows []Row
	for _, value := range inForceValues(keys) {
		if listedUnder(listed, value) {
			continue
		}
		rows = append(rows, persistedRow(value, e))
	}
	return rows
}

// inForceValues selects which of prefs.json's three keys the union reports on,
// per §8.2: THE KEYS IN FORCE, never every key present.
//
// The tiebreak is applied HERE, through ResolveSetting, rather than restated: a
// non-empty `theme` wins and the slots are not read at all. A hand-edited file
// may legally carry all three keys, and listing rows for two slugs Portal is not
// reading would put the user to work fixing something with no effect. Phase 7's
// doctor line applies the identical call for the identical reason, so the two
// surfaces cannot disagree about which slug is live. Passing the already-stripped
// raw keys back through it is safe: stripping is idempotent, and the resolution is
// pure and total.
//
// Under a pair only the slots with a NON-EMPTY RAW value contribute. An unset
// slot arrives in the Setting as the shipped default, which is a built-in and
// therefore already has a row — it is §9.5's "never set" badge row, not a
// nomination that failed.
//
// Two slots naming the same value collapse to ONE, keyed on the persisted VALUE
// rather than on a derived slug, so a value yielding no slug at all collapses by
// the same rule: one value the user set is one problem, and one row.
func inForceValues(keys RawKeys) []string {
	setting, raw := ResolveSetting(keys.Theme, keys.Light, keys.Dark)
	if setting.IsConstant {
		return []string{setting.Constant}
	}

	var values []string
	for _, slot := range []string{raw.Light, raw.Dark} {
		if slot != "" && !slices.Contains(values, slot) {
			values = append(values, slot)
		}
	}
	return values
}

// listedUnder reports whether one of the already-assembled rows is the row for
// this persisted value.
func listedUnder(listed []Row, value string) bool {
	return slices.ContainsFunc(listed, func(row Row) bool { return row.Slug == value })
}

// persistedRow builds the row for one in-force persisted value that nothing
// answers to, carrying the reason for the state it is actually in.
//
// A CHARSET FAILURE IS `bad name`, NEVER `not found` (§9.4). Each §6.2 reason has
// exactly one condition, and telling a user their file is missing when they typed
// an illegal name sends them looking in the wrong place. It is decided FIRST,
// before anything is treated as a slug, which is the same ordering ResolveByName
// applies for the stronger reason that a value like `../something` would
// otherwise be used verbatim as a path component (§8.6). Such a row is labelled
// by the raw value because it has nothing else — the value yields no slug, and no
// file was ever sought.
//
// Otherwise the reason is the DIRECTORY's state (§5.5): `unreadable` where the
// themes directory could not be listed, `not found` where it could. The theme may
// be sitting right there in a directory nothing can read, so `not found` — check
// the filename — would send the user past the actual problem, which is
// permissions.
func persistedRow(value string, e Enumeration) Row {
	if !ValidSlug(value) {
		return Row{Persisted: value, Source: SourcePersisted, Rejection: badName(BadNameSlug)}
	}
	return Row{Slug: value, Source: SourcePersisted, Rejection: unresolvedRejection(e)}
}

// unresolvedRejection is §5.5's verdict for a slug nothing answers to, as the
// panel needs it: the bare reason.
//
// It carries no detail even for `unreadable`, where the OS error would otherwise
// ride along verbatim (§14A). The row renders the terse reason alone (§9.5) and
// the condition already has its own pinned chrome row; the verbatim system
// message belongs to doctor, which reads the directory itself and has the width
// to print it.
func unresolvedRejection(e Enumeration) *Rejection {
	if e.DirUnusable {
		return &Rejection{Reason: ReasonUnreadable}
	}
	return notFound()
}

// sortRows puts the union in §9.5's display order, in place.
//
// ALPHABETICAL BY SLUG AND NOTHING ELSE. No palette is read here and no variant
// concept enters: ordering same-mode themes first was proposed as a mitigation
// for §9.2's mixed-mode flash and REJECTED, so a Row's Theme is not an input to
// this comparison at all. The `⚠ dir unreadable` condition is likewise outside
// the ordering entirely — it is Union.DirUnusable rather than a row, so there is
// nothing here to sort.
//
// The sort is STABLE, so any pair the three legs below still tie on — a
// charset-rejected persisted string reading byte-for-byte like a file's name,
// say — holds its assembly order rather than being permuted by run. That, plus
// legs that decide every other pair, is what makes the panel's fixtures
// reproducible (§13.3) and §9.5's adjacency argument concrete rather than
// incidental.
func sortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool { return rowBefore(rows[i], rows[j]) })
}

// rowBefore is §9.5's three-leg comparison, in the fixed order the legs are
// tried:
//
//  1. CASE-INSENSITIVE on the sort key. Slugs are lowercase by construction but
//     filenames are not, and a byte-wise-only comparison files `Zed.theme` ahead
//     of every valid theme, every uppercase byte sorting below every lowercase
//     one.
//  2. BYTE-WISE on the sort key, where the first leg ties. Case-insensitive
//     alone is not an order: two keys differing only in case would tie, and
//     which came out first would depend on how the union was built rather than
//     on a rule.
//  3. THE BUILT-IN FIRST, where both above tie. This tie is GUARANTEED BY
//     CONSTRUCTION and the byte-wise leg cannot settle it — a `reserved name`
//     row and the built-in it collides with have an IDENTICAL sort key, because
//     that identity is the definition of the reason (§6.2). The built-in wins
//     because that is the useful order: the valid, selectable thing the user can
//     act on, immediately followed by the row explaining why their file is not
//     it. Stating it as a RULE rather than leaning on the built-ins happening
//     to be assembled first is what makes the comparison total for any input:
//     a later change to the assembly order cannot silently lead the panel with
//     the row explaining why a theme is unusable instead of with the theme.
func rowBefore(a, b Row) bool {
	aKey, bKey := a.SortKey(), b.SortKey()
	if aFolded, bFolded := strings.ToLower(aKey), strings.ToLower(bKey); aFolded != bFolded {
		return aFolded < bFolded
	}
	if aKey != bKey {
		return aKey < bKey
	}
	return a.Source == SourceBuiltin && b.Source != SourceBuiltin
}

// countRejected counts the unselectable rows — the value `theme: enumerated`
// reports as `rejected` (§12.3).
//
// It is DERIVED from the rows rather than tallied as they are appended, so the
// count cannot drift from the set it describes.
func countRejected(rows []Row) int {
	rejected := 0
	for _, row := range rows {
		if !row.Selectable() {
			rejected++
		}
	}
	return rejected
}
