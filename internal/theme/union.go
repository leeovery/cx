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
// It exists for ordering only: a built-in row is otherwise deliberately
// indistinguishable from a valid drop-in, so nothing here may reach a row's
// rendered content.
//
// SourceBuiltin is the zero value, which is the always-present source rather than
// an "unset" sentinel — every union has built-in rows, and a Row always came from
// somewhere (the Slot precedent in resolution.go).
type RowSource int

const (
	// SourceBuiltin is a row for one of the embedded built-ins.
	SourceBuiltin RowSource = iota
	// SourceFile is a row for one candidate found in the themes directory,
	// valid or not.
	SourceFile
	// SourcePersisted is a row for a slug named in prefs.json that resolves to
	// neither — the row that gives the `●` marker something to sit on when the
	// theme it marks has gone.
	SourcePersisted
)

// Row is one line of the panel's list: one theme, or one persisted value that
// names no theme.
//
// The identity fields are three because a row has three different things it might
// be labelled and sorted by, and exactly one of them applies per row:
//
//   - Slug — every row that has an identity. Absent only where the name yields
//     none: a `bad name` file and a charset-rejected persisted string.
//   - Filename — set iff SourceFile, because a `bad name` row and a
//     `reserved name` row are labelled by filename (the first has no slug; the
//     second's slug is identical to the built-in's it collides with).
//   - Persisted — the raw persisted value, set iff the row is a SourcePersisted
//     one that yielded no slug. It is the only thing such a row can be labelled
//     or sorted by, and it arrives already control-stripped.
//
// Theme is populated iff Rejection is nil, as the loader's Result is: a rejected
// row never comes back half-populated. Holding the palette on the row is what
// makes the panel's preview an O(1) restyle from values already in hand rather
// than a file read per keystroke.
//
// Rejection is the one reason the row is not usable, carried unchanged from
// whoever produced it — the ladder for a file, this assembly for a persisted
// value that answers to nothing.
type Row struct {
	Slug      string
	Filename  string
	Persisted string
	Source    RowSource
	Theme     Theme
	Rejection *Rejection
}

// Selectable reports whether the panel may commit this row: valid rows only,
// which is the same fact as carrying no rejection.
func (r Row) Selectable() bool {
	return r.Rejection == nil
}

// Identity is what the row IS, whatever order it happens to be listed in: the
// slug wherever one exists, else the filename, else the persisted string itself.
//
// It is what the badge table keys on and what the panel's cursor anchors to
// across a recompute, so every row shape must yield exactly one non-empty value —
// a row identified by nothing is a row that cannot be found again. The three arms
// are the identity fields in the one order that leaves no row unidentified:
//
//   - The slug, for every row that has one — including a `reserved name` row,
//     whose slug is valid and identical to the built-in's it collides with, and a
//     `not found` persisted row.
//   - The filename, for the one row shape that has no slug: a `bad name` file,
//     since names are rejected rather than normalised.
//   - The persisted string, for a charset-rejected persisted value, which has
//     neither a slug nor a file. Display truncation is a render concern and never
//     reaches this value.
//
// It is not an ordering value: what a row is and where it sits are separate
// questions, and identity must hold still while the ordering is free to change —
// see SortKey.
func (r Row) Identity() string {
	return cmp.Or(r.Slug, r.Filename, r.Persisted)
}

// SortKey is the row's ordering value, which today is the row's Identity.
//
// The two coincide by choice rather than by necessity — listing rows
// alphabetically by what each one is happens to be what the panel wants — and may
// diverge: ordering by label, or standing the built-ins first, would change this
// value alone and leave identity untouched.
//
// Every row therefore yields exactly one non-empty key, which is what makes the
// order total (see sortRows). Sorting a `reserved name` row on its slug stands the
// row explaining the collision immediately beside the thing it collides with, even
// though that row is labelled by its filename.
//
// It is not derived from Label, and Label is not derived from it — see Label.
func (r Row) SortKey() string {
	return r.Identity()
}

// Label is the row's display value, deliberately a separate value from SortKey
// rather than a second reading of it: the row delegate consumes Label and the
// ordering consumes SortKey, and neither may be re-derived from the other at
// render time. The two are meant to disagree on exactly these rows:
//
//   - A `reserved name` row is labelled by its filename while sorting by its
//     slug — `nord.theme` beside `nord` tells the user which one is theirs,
//     where two rows reading `nord` would not.
//   - A `bad name` file is labelled by its filename because it has no slug.
//   - A charset-rejected persisted value is labelled by the raw string, the only
//     thing it has: no slug was derived and no file was ever sought.
//   - Every other row is labelled by its slug.
func (r Row) Label() string {
	if r.labelledByFilename() {
		return r.Filename
	}
	return cmp.Or(r.Slug, r.Persisted)
}

// labelledByFilename reports whether this row is labelled by its filename rather
// than by the slug it would otherwise be listed under.
//
// Two reasons, both a file's: `bad name`, which yields no slug at all, and
// `reserved name`, whose slug is the built-in's. The filename check keeps the
// charset-rejected persisted row out of this arm — it is `bad name` by reason
// too, but has no file behind it and is labelled by the raw value instead.
func (r Row) labelledByFilename() bool {
	if r.Rejection == nil || r.Filename == "" {
		return false
	}
	return r.Rejection.Reason == ReasonBadName || r.Rejection.Reason == ReasonReservedName
}

// Enumeration is one directory read, retained: what the themes directory held
// and what the directory itself was.
//
// It is kept apart from the Union it produces because the two have different
// lifetimes: the parse results are held for the panel's lifetime so arrowing
// previews from values already in hand, while the union is re-derived from them
// whenever the persisted state changes — the post-commit recompute and the `Esc`
// re-resolution both re-run the derivation against the same enumeration, with no
// fresh read.
type Enumeration struct {
	// Entries is every candidate the directory held, classified — valid or not.
	// Empty for an absent directory and for an unusable one alike.
	Entries []Entry

	// DirUnusable is the directory verdict: true for an unreadable directory or a
	// regular file where a directory belongs, false for an absent one.
	//
	// It is a bool rather than the rejection itself because its consumers need
	// only the condition: the pinned `⚠ dir unreadable` chrome row has no room for
	// a detail, and a persisted slug made unreachable by the directory carries the
	// bare reason `unreadable`. The OS error verbatim belongs to doctor, which
	// reads the directory itself.
	DirUnusable bool

	// DirPath is the directory this enumeration read, whatever the outcome — so
	// a retained enumeration says which directory it is OF rather than only what
	// it found.
	DirPath string
}

// Union is the finished row set: every built-in, every file in the themes
// directory, and every persisted slug resolving to neither — deduped one slug,
// one row, each row already carrying its single rejection reason.
//
// It is an ordinary value with exported fields and no method that reads the
// filesystem, so a fixture can fake one wholesale with no loader and no
// directory — the only way internal/capture, under its no-real-config import
// guard, can render an invalid-theme row at all.
type Union struct {
	// Rows is the union in display order — alphabetical by sort key,
	// case-insensitively, with the built-in ahead of the one row guaranteed to
	// tie with it (see sortRows).
	//
	// The order is applied by the assembler rather than by the panel, so every
	// consumer receives it ordered and none can forget to sort. Enumeration order
	// — built-ins, then os.ReadDir's, then the persisted leftovers — is neither
	// alphabetical nor stable across filesystems.
	Rows []Row

	// DirUnusable drives the pinned `⚠ dir unreadable` chrome row. It is a flag
	// rather than a member of Rows because that row is viewport chrome, not a
	// list row: a list row participates in pagination and would vanish the moment
	// the user paged down.
	DirUnusable bool

	// Count is len(Rows) — what `theme: enumerated` reports. The chrome row above
	// is not a union member and is never counted.
	Count int

	// Rejected is the unselectable subset: the rows carrying a rejection.
	Rejected int
}

// Assembler builds the panel's row model — the Rows, their order and the counts
// over them — from what a Loader parses.
//
// It is a type of its own rather than more surface on the Loader because it loads
// nothing itself: everything here is assembly — which rows exist, which reason
// each carries, and what order they are listed in. The Loader beneath it answers
// the only question assembly cannot, so a consumer that merely reads a theme
// never holds the row model's entry points.
//
// It is constructed from a Loader rather than from a Loader's parts so the
// assembly shares that loader's `theme` dedup scope: the enumeration beneath Open
// hits the same directory conditions a by-name read does, and one dedup scope per
// launch is what stops a broken file being reported twice.
type Assembler struct {
	// Loader is what parses the built-ins and the directory's candidates. The
	// zero value is a valid silent one: it reserves nothing and emits nothing.
	Loader Loader
}

// Open is the panel's entry point: one directory read, producing the retained
// enumeration and the finished union together.
//
// It is where "re-read on every open, retain for the panel's lifetime" is
// realised — the panel calls this when it opens and Reassemble for everything
// after, so no later step reads the directory.
//
// The two Enumerate returns are separate directory states: an unusable directory
// sets the flag and contributes no entries, while an absent one is silent and
// contributes none. Neither is an error, because neither stops the panel opening
// — the built-ins are always there to list.
//
// It emits `theme: enumerated` once per call, from the one place both its attrs
// are computable: the count is rows produced and the rejected count is the
// unselectable subset, neither knowable before the merge. It fires on an absent
// directory and on an unusable one alike — the panel opened either way — and it
// does not dedup, unlike the WARNs Enumerate emits beneath it.
func (a Assembler) Open(themesDir string, keys RawKeys) (Enumeration, Union) {
	entries, rejection := a.Loader.Enumerate(themesDir)
	enumeration := Enumeration{Entries: entries, DirUnusable: rejection != nil, DirPath: themesDir}

	union := a.Reassemble(enumeration, keys)
	a.Loader.events.Enumerated(union.Count, union.Rejected)

	return enumeration, union
}

// Reassemble re-derives the union from a retained enumeration and the current
// persisted keys.
//
// It must perform no I/O and emit nothing: it is a pure function of its two
// arguments. The post-commit recompute and the `Esc` re-resolution both re-run it
// with changed prefs state, and re-reading the directory there would cost a
// syscall per keypress and let the list change under a user who only pressed
// `Enter`.
//
// The order below is fixed:
//
//  1. One row per built-in. Always valid — build-time validation of the embedded
//     set makes that true — and carrying no marker of any kind, since a built-in
//     row is deliberately indistinguishable from a valid drop-in.
//  2. One row per enumerated file, carrying the entry's palette or its single
//     rejection unchanged. A `reserved name` file stands alongside the built-in
//     it collides with — the one legitimate two-rows-for-one-slug case, because
//     that collision is the reason's entire content. Every other file slug is
//     unique by construction.
//  3. The persisted keys in force, each contributing a row only where nothing
//     above already answers to it — see persistedRows.
//  4. The display order, applied here rather than by the panel, so both the Open
//     path and every recompute hand back an ordered union — see sortRows. It runs
//     last because it is a pure rearrangement: the three steps above decide
//     membership, and each one's dedup reads the rows already assembled by slug
//     rather than by position.
func (a Assembler) Reassemble(e Enumeration, keys RawKeys) Union {
	rows := a.builtinRows()
	rows = append(rows, fileRows(e.Entries)...)
	rows = append(rows, persistedRows(rows, e, keys)...)
	sortRows(rows)

	return Union{Rows: rows, DirUnusable: e.DirUnusable, Count: len(rows), Rejected: countRejected(rows)}
}

// builtinRows is step 1: one row per embedded built-in, in BuiltinSlugs' sorted
// order.
//
// They are loaded rather than cached, on every derivation, for the reason nothing
// else here is cached either: the parse is of a handful of small files already in
// memory, and a cache is a second source of truth for a palette the panel is
// about to paint.
//
// A slug the injected byte source does not answer to yields no row rather than an
// empty or rejected one — that source exists solely to stage the broken-binary
// state (see Loader.BuiltinSource), where "this built-in is not in this binary"
// is what the union should say. A built-in that is present and does not parse
// carries its rejection like any other row.
func (a Assembler) builtinRows() []Row {
	slugs := BuiltinSlugs()

	rows := make([]Row, 0, len(slugs))
	for _, slug := range slugs {
		result, rejection, found := a.Loader.LoadBuiltin(slug)
		if !found {
			continue
		}
		rows = append(rows, Row{Slug: slug, Source: SourceBuiltin, Theme: result.Theme, Rejection: rejection})
	}
	return rows
}

// fileRows is step 2: one row per enumerated candidate, valid or not.
//
// Every candidate gets a row. That is the promise of the drop-in route: a broken
// file is present and named, so the user sees "there's my theme, it's registered,
// but it's invalid" rather than nothing at all.
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
// The rule is "resolves", not "has a file". A persisted value matching a listed
// row's slug contributes nothing, because that row already is its row — a
// built-in's, a valid drop-in's or a broken drop-in's alike. Keying on file
// existence instead would mint a second `⚠ not found` row for every persisted
// built-in slug, the state the panel's most common action produces (`Enter` on
// `tokyo-night`).
//
// The listed slugs are all valid by construction — a built-in's, or one
// SlugFromFilename derived — so a persisted value that is not a legal slug cannot
// match one by accident.
//
// Only what is in force is considered, and only what the user actually set: the
// selection is InForceKeys'. Only the value of each key is read here, because
// which slot a value sits in makes no difference to the row it earns.
func persistedRows(listed []Row, e Enumeration, keys RawKeys) []Row {
	var rows []Row
	for _, key := range InForceKeys(keys) {
		if listedUnder(listed, key.Value) {
			continue
		}
		rows = append(rows, persistedRow(key.Value, e))
	}
	return rows
}

// listedUnder reports whether one of the already-assembled rows is the row for
// this persisted value.
func listedUnder(listed []Row, value string) bool {
	return slices.ContainsFunc(listed, func(row Row) bool { return row.Slug == value })
}

// persistedRow builds the row for one in-force persisted value that nothing
// answers to, carrying the reason for the state it is actually in.
//
// A charset failure is `bad name` and never `not found`: telling a user their
// file is missing when they typed an illegal name sends them looking in the wrong
// place. It is decided first, before anything is treated as a slug — the same
// ordering ResolveByName applies for the stronger reason that a value like
// `../something` would otherwise be used verbatim as a path component. Such a row
// is labelled by the raw value, which is all it has.
//
// Otherwise the reason is the directory's state: `unreadable` where the themes
// directory could not be listed, `not found` where it could. The theme may be
// sitting right there in a directory nothing can read, so `not found` would send
// the user past the actual problem.
func persistedRow(value string, e Enumeration) Row {
	if !ValidSlug(value) {
		return Row{Persisted: value, Source: SourcePersisted, Rejection: badName(BadNameSlug)}
	}
	return Row{Slug: value, Source: SourcePersisted, Rejection: unresolvedRejection(e)}
}

// unresolvedRejection is the verdict for a slug nothing answers to, as the panel
// needs it: the bare reason.
//
// It carries no detail even for `unreadable`, where the OS error would otherwise
// ride along verbatim. The row renders the terse reason alone and the condition
// already has its own pinned chrome row; the verbatim system message belongs to
// doctor, which reads the directory itself and has the width to print it.
func unresolvedRejection(e Enumeration) *Rejection {
	if e.DirUnusable {
		return &Rejection{Reason: ReasonUnreadable}
	}
	return notFound()
}

// sortRows puts the union in display order, in place.
//
// Alphabetical by sort key and nothing else. No palette is read here and no
// light/dark concept enters: a Row's Theme is not an input to this comparison.
// The `⚠ dir unreadable` condition is likewise outside the ordering — it is
// Union.DirUnusable rather than a row.
//
// The sort is stable, so any pair the three legs below still tie on — a
// charset-rejected persisted string reading byte-for-byte like a file's name, say
// — holds its assembly order rather than being permuted by run, which is what
// makes the panel's rendering reproducible.
func sortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool { return rowBefore(rows[i], rows[j]) })
}

// rowBefore is the three-leg comparison, in the fixed order the legs are tried:
//
//  1. Case-insensitive on the sort key. Slugs are lowercase by construction but
//     filenames are not, and a byte-wise-only comparison files `Zed.theme` ahead
//     of every valid theme, every uppercase byte sorting below every lowercase
//     one.
//  2. Byte-wise on the sort key, where the first leg ties. Case-insensitive alone
//     is not an order: two keys differing only in case would tie, and which came
//     out first would depend on how the union was built rather than on a rule.
//  3. The built-in first, where both above tie. That tie is guaranteed by
//     construction and the byte-wise leg cannot settle it: a `reserved name` row
//     and the built-in it collides with have an identical sort key, because that
//     identity is the definition of the reason. The built-in wins because it is
//     the valid, selectable thing the user can act on, immediately followed by
//     the row explaining why their file is not it. Stating it as a rule rather
//     than leaning on assembly order is what keeps a later change to that order
//     from leading the panel with the row explaining why a theme is unusable.
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
// reports as `rejected`.
//
// It is derived from the rows rather than tallied as they are appended, so the
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
