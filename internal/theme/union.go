package theme

import (
	"cmp"
	"slices"
	"sort"
	"strings"
)

// RowSource says where one union row came from. It exists for ordering only:
// a built-in row is otherwise deliberately indistinguishable from a valid
// drop-in, so nothing here may reach a row's rendered content.
type RowSource int

const (
	SourceBuiltin RowSource = iota
	SourceFile
	// SourcePersisted is a slug named in prefs.json that resolves to neither
	// — the row giving the `●` marker something to sit on when the theme it
	// marks has gone.
	SourcePersisted
)

// Row is one line of the panel's list: one theme, or one persisted value that
// names no theme. Exactly one of Slug, Filename and Persisted applies per row
// — Slug is absent only where the name yields none, and Persisted (already
// control-stripped) is all a charset-rejected persisted value has. Theme is
// populated iff Rejection is nil; holding the palette here is what makes the
// panel's preview an O(1) restyle rather than a file read per keystroke.
type Row struct {
	Slug      string
	Filename  string
	Persisted string
	Source    RowSource
	Theme     Theme
	Rejection *Rejection
}

// Selectable reports whether the panel may commit this row.
func (r Row) Selectable() bool {
	return r.Rejection == nil
}

// Identity is what the row is: the slug where one exists, else the filename,
// else the persisted string. The badge table keys on it and the panel's cursor
// anchors to it across a recompute, so every row shape must yield exactly one
// non-empty value — a row identified by nothing cannot be found again. It is
// not an ordering value; identity must hold still while ordering is free to
// change.
func (r Row) Identity() string {
	return cmp.Or(r.Slug, r.Filename, r.Persisted)
}

// SortKey is the row's ordering value. It coincides with Identity by choice,
// not necessity: a different order would change this value alone and leave
// identity untouched. It is never derived from Label, nor Label from it.
func (r Row) SortKey() string {
	return r.Identity()
}

// Label is the row's display value — deliberately separate from SortKey, and
// never re-derived from it at render time. The two disagree on a
// `reserved name` row, which is labelled by filename while sorting by slug so
// `nord.theme` beside `nord` tells the user which one is theirs.
func (r Row) Label() string {
	if r.labelledByFilename() {
		return r.Filename
	}
	return cmp.Or(r.Slug, r.Persisted)
}

// The filename check keeps the charset-rejected persisted row out of this arm:
// it carries `bad name` too, but has no file behind it and is labelled by the
// raw value instead.
func (r Row) labelledByFilename() bool {
	if r.Rejection == nil || r.Filename == "" {
		return false
	}
	return r.Rejection.Reason == ReasonBadName || r.Rejection.Reason == ReasonReservedName
}

// Enumeration is one directory read, retained. It is kept apart from the Union
// it produces because the two have different lifetimes: the parse results are
// held for the panel's lifetime, while the union is re-derived from them
// whenever the persisted state changes, with no fresh read.
type Enumeration struct {
	// Entries is empty for an absent directory and an unusable one alike.
	Entries []Entry

	// DirUnusable is true for an unreadable directory or a regular file where
	// a directory belongs, false for an absent one. A bool rather than the
	// rejection because consumers need only the condition; the verbatim OS
	// error belongs to doctor, which reads the directory itself.
	DirUnusable bool

	DirPath string
}

// Union is the finished row set: every built-in, every file in the themes
// directory, and every persisted slug resolving to neither, deduped one slug
// to one row. It reads no filesystem, so one can be assembled wholesale with
// no loader and no directory.
type Union struct {
	// Rows arrive already in display order, applied by the assembler so no
	// consumer can forget to sort.
	Rows []Row

	// DirUnusable drives the pinned `⚠ dir unreadable` chrome row — a flag
	// rather than a member of Rows, since a list row would participate in
	// pagination and vanish the moment the user paged down.
	DirUnusable bool

	// Count is len(Rows); the chrome row is not a union member.
	Count int

	// Rejected is the unselectable subset.
	Rejected int
}

// Assembler builds the panel's row model from what a Loader parses. It takes
// the whole Loader so the assembly shares that loader's `theme` dedup scope —
// one scope per launch is what stops a broken file being reported twice.
type Assembler struct {
	// The zero Loader is valid and silent: it reserves nothing and emits
	// nothing.
	Loader Loader
}

// Open is the panel's entry point: one directory read producing the retained
// enumeration and the finished union. Call it on open and Reassemble for
// everything after, so no later step reads the directory. Neither an absent
// nor an unusable directory is an error — the built-ins are always there to
// list — and it emits one `theme: enumerated` per call either way.
func (a Assembler) Open(themesDir string, keys RawKeys) (Enumeration, Union) {
	entries, rejection := a.Loader.Enumerate(themesDir)
	enumeration := Enumeration{Entries: entries, DirUnusable: rejection != nil, DirPath: themesDir}

	union := a.Reassemble(enumeration, keys)
	a.Loader.events.Enumerated(union.Count, union.Rejected)

	return enumeration, union
}

// Reassemble re-derives the union from a retained enumeration and the current
// persisted keys. It must perform no I/O and emit nothing — it re-runs on
// every recompute, and re-reading the directory would cost a syscall per
// keypress and let the list change under a user who only pressed `Enter`.
// Step order is load-bearing: membership is decided before the sort, and each
// step's dedup reads the rows already assembled.
func (a Assembler) Reassemble(e Enumeration, keys RawKeys) Union {
	rows := a.builtinRows()
	rows = append(rows, fileRows(e.Entries)...)
	rows = append(rows, persistedRows(rows, e, keys)...)
	sortRows(rows)

	return Union{Rows: rows, DirUnusable: e.DirUnusable, Count: len(rows), Rejected: countRejected(rows)}
}

// Built-ins are re-loaded on every derivation rather than cached: a cache
// would be a second source of truth for a palette the panel is about to
// paint. A slug the injected byte source does not answer to yields no row at
// all — "this built-in is not in this binary" is what the union should say.
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
// named rather than absent. Nothing is re-derived — slug, filename, palette
// and rejection all ride across as the ladder produced them.
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

// The rule is "resolves", not "has a file": a persisted value matching a
// listed row contributes nothing, or every persisted built-in slug would mint
// a second `⚠ not found` row. Only the in-force keys are considered, and only
// their values — the slot makes no difference to the row a value earns.
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

// A charset failure is `bad name`, never `not found`, and is decided before
// anything is treated as a slug. Otherwise the reason is the directory's
// state: `unreadable` where it could not be listed, since the theme may be
// sitting right there in a directory nothing can read.
func persistedRow(value string, e Enumeration) Row {
	if !ValidSlug(value) {
		return Row{Persisted: value, Source: SourcePersisted, Rejection: badName(BadNameSlug)}
	}
	return Row{Slug: value, Source: SourcePersisted, Rejection: unresolvedRejection(e)}
}

// The bare reason, with no detail even for `unreadable`: the row renders the
// terse reason alone, and the verbatim OS error belongs to doctor.
func unresolvedRejection(e Enumeration) *Rejection {
	if e.DirUnusable {
		return &Rejection{Reason: ReasonUnreadable}
	}
	return notFound()
}

// Alphabetical by sort key and nothing else — no palette and no light/dark
// concept enters. Stable, so any pair the three legs still tie on holds its
// assembly order and the rendering stays reproducible.
func sortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool { return rowBefore(rows[i], rows[j]) })
}

// Three legs, in this order. Case-insensitive first, or a byte-wise
// comparison would file `Zed.theme` ahead of every valid theme. Byte-wise
// second, since case-folding alone is not a total order. Built-in last, for
// the tie the byte-wise leg cannot settle: a `reserved name` row's sort key is
// identical to the built-in's by definition, and the built-in must lead so the
// panel does not open with the row explaining why a theme is unusable.
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

// Derived from the rows rather than tallied as they are appended, so the count
// cannot drift from the set it describes.
func countRejected(rows []Row) int {
	rejected := 0
	for _, row := range rows {
		if !row.Selectable() {
			rejected++
		}
	}
	return rejected
}
