package theme

import (
	"cmp"
	"slices"
	"sort"
	"strings"
)

// RowSource exists for ordering: a built-in row is otherwise deliberately
// indistinguishable from a valid drop-in, so this must not reach rendered
// content.
type RowSource int

const (
	SourceBuiltin RowSource = iota
	SourceFile
	// SourcePersisted is a slug named in prefs.json that resolves to neither —
	// the row giving the `●` marker something to sit on when the theme it marks
	// has gone.
	SourcePersisted
)

// Row is one line of the panel's list: one theme, or one persisted value naming
// no theme. Theme is populated iff Rejection is nil — holding the palette here
// is what makes the panel's preview a restyle rather than a file read per
// keystroke — and Persisted is already control-stripped.
type Row struct {
	Slug      string
	Filename  string
	Persisted string
	Source    RowSource
	Theme     Theme
	Rejection *Rejection
}

func (r Row) Selectable() bool {
	return r.Rejection == nil
}

// Identity is what the badge table keys on and the panel's cursor anchors to
// across a recompute, so every row shape must yield a non-empty value or the row
// cannot be found again. It is not an ordering value.
func (r Row) Identity() string {
	return cmp.Or(r.Slug, r.Filename, r.Persisted)
}

// SortKey coincides with Identity by choice, not necessity: changing the order
// must change this value alone and leave identity untouched.
func (r Row) SortKey() string {
	return r.Identity()
}

// Label is deliberately separate from SortKey and never re-derived from it. The
// two disagree on a `reserved name` row, which is labelled by filename while
// sorting by slug, so the file sits beside the built-in it collides with.
func (r Row) Label() string {
	if r.labelledByFilename() {
		return r.Filename
	}
	return cmp.Or(r.Slug, r.Persisted)
}

// The Filename check keeps the charset-rejected persisted row out of this arm:
// it carries `bad name` too, but has no file behind it.
func (r Row) labelledByFilename() bool {
	if r.Rejection == nil || r.Filename == "" {
		return false
	}
	return r.Rejection.Reason == ReasonBadName || r.Rejection.Reason == ReasonReservedName
}

// Enumeration is one directory read, retained. It is kept apart from the Union
// it produces because the two have different lifetimes: the parses are held for
// the panel's lifetime, while the union is re-derived from them whenever the
// persisted state changes, with no fresh read.
type Enumeration struct {
	// Entries is empty for an absent directory and an unusable one alike.
	Entries []Entry

	// DirUnusable is true for an unreadable directory or a regular file where a
	// directory belongs, false for an absent one.
	DirUnusable bool

	DirPath string
}

// Union is the finished row set: every built-in, every file in the themes
// directory, and every persisted slug resolving to neither, deduped one slug to
// one row. It touches no filesystem.
type Union struct {
	// Rows arrive already in display order — no consumer has to sort.
	Rows []Row

	// DirUnusable drives the pinned `⚠ dir unreadable` chrome row — a flag
	// rather than a member of Rows, since a list row would participate in
	// pagination and vanish the moment the user paged down.
	DirUnusable bool

	// Count excludes the chrome row, which is no union member.
	Count int

	Rejected int
}

type Assembler struct {
	// One Loader keeps a launch to one `theme` dedup scope; a second would
	// report one broken file twice.
	Loader Loader
}

// Open is the panel's entry point. Call it on open and Reassemble for everything
// after, so no later step reads the directory. Neither an absent nor an unusable
// directory is an error — the built-ins are always there to list — and it emits
// one `theme: enumerated` per call either way.
func (a Assembler) Open(themesDir string, keys RawKeys) (Enumeration, Union) {
	enumeration := a.Loader.OpenEnumeration(themesDir)

	union := a.Reassemble(enumeration, keys)
	a.Loader.events.Enumerated(union.Count, union.Rejected)

	return enumeration, union
}

// Reassemble re-derives the union from a retained enumeration and the current
// persisted keys. It must perform no I/O and emit nothing — it re-runs on every
// recompute, and re-reading the directory would let the list change under a user
// who only pressed `Enter`. Step order is load-bearing: membership is decided
// before the sort, and each step's dedup reads the rows already assembled.
func (a Assembler) Reassemble(e Enumeration, keys RawKeys) Union {
	rows := a.builtinRows()
	rows = append(rows, fileRows(e.Entries)...)
	rows = append(rows, persistedRows(rows, e, keys)...)
	sortRows(rows)

	return Union{Rows: rows, DirUnusable: e.DirUnusable, Count: len(rows), Rejected: countRejected(rows)}
}

// Built-ins are re-loaded on every derivation rather than cached: a cache would
// be a second source of truth for a palette the panel is about to paint. A slug
// the injected byte source does not answer to yields no row at all.
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

// Every candidate gets a row, valid or not: a broken file must be present and
// named rather than absent.
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

// A persisted value already listed contributes nothing — whether or not that row
// resolves — or every persisted built-in slug would mint a second `⚠ not found`
// row.
func persistedRows(listed []Row, e Enumeration, keys RawKeys) []Row {
	inForce := InForceKeys(keys)

	rows := make([]Row, 0, len(inForce))
	for _, key := range inForce {
		if listedUnder(listed, key.Value) {
			continue
		}
		rows = append(rows, persistedRow(key.Value, e))
	}
	return rows
}

func listedUnder(listed []Row, value string) bool {
	return slices.ContainsFunc(listed, func(row Row) bool { return row.Slug == value })
}

// A charset failure is `bad name`, never `not found`, decided before the value
// is treated as a slug at all. Otherwise the reason is the directory's state:
// the theme may be sitting right there in a directory nothing can read.
func persistedRow(value string, e Enumeration) Row {
	if !ValidSlug(value) {
		return Row{Persisted: value, Source: SourcePersisted, Rejection: badName(BadNameSlug)}
	}
	return Row{Slug: value, Source: SourcePersisted, Rejection: unresolvedRejection(e)}
}

// No Detail even for `unreadable`: the row renders the terse reason alone, and
// the verbatim OS error belongs to doctor.
func unresolvedRejection(e Enumeration) *Rejection {
	if e.DirUnusable {
		return &Rejection{Reason: ReasonUnreadable}
	}
	return notFound()
}

// Stable, so a pair the legs still tie on holds its assembly order and the
// rendering stays reproducible.
func sortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool { return rowBefore(rows[i], rows[j]) })
}

// Case-insensitive first, or a byte-wise comparison would file `Zed.theme` ahead
// of every valid theme; byte-wise second, since case-folding alone is not a
// total order; built-in last, for the tie the byte-wise leg cannot settle — a
// `reserved name` row's sort key is identical to the built-in's by definition,
// and the built-in must lead.
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

func countRejected(rows []Row) int {
	rejected := 0
	for _, row := range rows {
		if !row.Selectable() {
			rejected++
		}
	}
	return rejected
}
