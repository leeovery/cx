package theme_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/theme"
)

// TestRowOrder_ReservedNameSortsBySlugLabelsByFilename pins the row §9.5 built
// the sort-key/label split around: a `nord.theme` drop-in beside the `nord`
// built-in sorts by its SLUG and is labelled by its FILENAME — on the same row,
// at the same time.
//
// Neither value is a reading of the other, and this row is the proof: its slug
// is perfectly valid and would be a fine label if it were not IDENTICAL to the
// built-in's, which is the definition of `reserved name` (§6.2). Labelling by
// slug would put two rows reading `nord` in a list where §9.5 deliberately makes
// built-in and drop-in rows indistinguishable; sorting by filename would move it
// away from the built-in it collides with, which is exactly where the
// explanation is useful.
func TestRowOrder_ReservedNameSortsBySlugLabelsByFilename(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "nord.theme", themeLines())
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	_, union := loader.Open(dir, theme.RawKeys{})

	row := onlyRejectedRow(t, union, theme.ReasonReservedName)
	if got, want := row.SortKey(), "nord"; got != want {
		t.Errorf("reserved-name row SortKey() = %q, want the slug %q — the slug is what puts it beside the built-in it collides with", got, want)
	}
	if got, want := row.Label(), "nord.theme"; got != want {
		t.Errorf("reserved-name row Label() = %q, want the filename %q — a second row reading `nord` would not say which one is theirs", got, want)
	}
	if row.SortKey() == row.Label() {
		t.Errorf("reserved-name row SortKey() and Label() are both %q, want two separate values", row.SortKey())
	}
}

// TestRowOrder_BuiltinFirstOnTheGuaranteedTie pins the one tie §9.5 says is
// guaranteed by construction and cannot be settled byte-wise: a `reserved name`
// row and the built-in it collides with hold an IDENTICAL sort key, because that
// identity is the definition of the reason (§6.2).
//
// The built-in wins — the valid, selectable thing the user can act on,
// immediately followed by the row explaining why their file is not it.
//
// `nord-lee` is staged as the nearest possible neighbour: its key is the closest
// any other row can sort to `nord` without being it. Asserting it does not fall
// between the pair is what makes the adjacency argument concrete rather than
// incidental — the two rows are adjacent because the ordering says so, not
// because nothing happened to sort between them.
func TestRowOrder_BuiltinFirstOnTheGuaranteedTie(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "nord.theme", themeLines())
	writeTheme(t, dir, "nord-lee.theme", themeLines())
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	_, union := loader.Open(dir, theme.RawKeys{})

	collided := rowsWithSlug(union, "nord")
	if len(collided) != 2 {
		t.Fatalf("slug %q has %d rows, want the built-in and its reserved-name collider: %v", "nord", len(collided), rowLabels(union))
	}
	if collided[0].SortKey() != collided[1].SortKey() {
		t.Fatalf("the colliding rows sort by %q and %q, want one identical key — the tie is what the built-in-first rule exists to settle", collided[0].SortKey(), collided[1].SortKey())
	}
	if collided[0].Source != theme.SourceBuiltin {
		t.Errorf("the first %q row is a %v, want the built-in ahead of the file it collides with", "nord", collided[0].Source)
	}

	at := slices.Index(rowLabels(union), "nord")
	if got, want := slices.Index(rowLabels(union), "nord.theme"), at+1; got != want {
		t.Errorf("the reserved-name row is at index %d, want %d — directly beneath the built-in, with nothing able to fall between", got, want)
	}
	want := slices.Insert(theme.BuiltinSlugs(), builtinIndex(t, "nord")+1, "nord.theme", "nord-lee")
	if got := rowLabels(union); !slices.Equal(got, want) {
		t.Errorf("union labels = %v, want %v — the nearest neighbouring slug sorts after the pair, never between it", got, want)
	}
}

// TestRowOrder_BadNameSortsByFilename pins the one row shape with no slug at
// all: a file §5.2 rejects rather than normalises sorts — and is labelled — by
// its FILENAME.
//
// It is the fallback arm of the sort key, and it is what makes the key fully
// determined: `Bad_Name.theme` yields no identity whatsoever, so a slug-only key
// would leave it with an empty one and every such row would clump at the head of
// the list in whatever order the directory happened to be read in.
//
// The whole sequence is asserted, not just the row's key, because the failure
// this guards against is positional: a row sorting by a value nothing else can
// tie with still has to land in the right place among the built-ins.
func TestRowOrder_BadNameSortsByFilename(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "Bad_Name.theme", themeLines())
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	_, union := loader.Open(dir, theme.RawKeys{})

	row := onlyRejectedRow(t, union, theme.ReasonBadName)
	if row.Slug != "" {
		t.Errorf("bad-name row slug = %q, want none — §5.2 rejects rather than normalises, so the name yields no identity", row.Slug)
	}
	if got, want := row.SortKey(), "Bad_Name.theme"; got != want {
		t.Errorf("bad-name row SortKey() = %q, want the filename %q — it is the only thing the row has", got, want)
	}
	if got, want := row.Label(), "Bad_Name.theme"; got != want {
		t.Errorf("bad-name row Label() = %q, want the filename %q", got, want)
	}

	want := append([]string{"Bad_Name.theme"}, theme.BuiltinSlugs()...)
	if got := rowSortKeys(union); !slices.Equal(got, want) {
		t.Errorf("union sort keys = %v, want %v — the filename sorts among the slugs, not ahead of the list", got, want)
	}
}

// TestRowOrder_CharsetRejectedSortsByItself pins the third arm of the sort key,
// which exists so the ordering stays TOTAL: a persisted string §8.6's charset
// check rejected has neither a slug nor a file, and sorts by the string itself.
//
// There is exactly one thing to sort such a row by, and using it is the whole
// argument — the alternative is a union member the comparator cannot place,
// which is precisely what a total order is not. The value arrives already
// control-stripped (§9.5 strips at the point it is read, not at the point it is
// drawn), so the key is the ordinary one-line text the row also displays.
func TestRowOrder_CharsetRejectedSortsByItself(t *testing.T) {
	const illegal = "../evil"
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	_, union := loader.Open(t.TempDir(), theme.RawKeys{Theme: illegal})

	row := onlyPersistedRow(t, union)
	if row.Slug != "" || row.Filename != "" {
		t.Fatalf("charset-rejected row carries slug %q and filename %q, want neither — it is the row shape with nothing but the raw value", row.Slug, row.Filename)
	}
	if got, want := row.SortKey(), illegal; got != want {
		t.Errorf("charset-rejected row SortKey() = %q, want the persisted string %q", got, want)
	}
	if got, want := row.Label(), illegal; got != want {
		t.Errorf("charset-rejected row Label() = %q, want the persisted string %q", got, want)
	}

	want := append([]string{illegal}, theme.BuiltinSlugs()...)
	if got := rowSortKeys(union); !slices.Equal(got, want) {
		t.Errorf("union sort keys = %v, want %v — the string is placed by the same rule as every other key", got, want)
	}
}

// TestRowOrder_CaseInsensitiveThenByteWise pins §9.5's comparison as the two
// legs it is, in order.
//
// Slugs are lowercase by construction but FILENAMES ARE NOT, and every uppercase
// byte sorts below every lowercase one — so a byte-wise-only comparison files
// `Zed.theme` ahead of every valid theme, at the head of the list, which is the
// failure the first case is written against. The second case is why the
// byte-wise leg survives underneath it: two keys equal but for case tie
// case-insensitively, and without a second leg which one came first would depend
// on how the union happened to be assembled.
//
// Each case is re-derived several times to pin the result as identical across
// runs rather than merely correct once.
func TestRowOrder_CaseInsensitiveThenByteWise(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		keys  theme.RawKeys
		want  []string
	}{
		{
			name:  "an uppercase filename sorts among the z's, not ahead of everything",
			files: []string{"Zed.theme", "aa-early.theme", "zz-late.theme"},
			want:  append(append([]string{"aa-early"}, theme.BuiltinSlugs()...), "Zed.theme", "zz-late"),
		},
		{
			name:  "keys equal but for case are settled byte-wise",
			files: []string{"zEd.theme"},
			keys:  theme.RawKeys{Theme: "Zed.theme"},
			want:  append(theme.BuiltinSlugs(), "Zed.theme", "zEd.theme"),
		},
	}

	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, file := range tt.files {
				writeTheme(t, dir, file, themeLines())
			}

			enumeration, union := loader.Open(dir, tt.keys)

			if got := rowSortKeys(union); !slices.Equal(got, tt.want) {
				t.Fatalf("union sort keys = %v, want %v", got, tt.want)
			}
			for again := range 5 {
				if got := rowSortKeys(loader.Reassemble(enumeration, tt.keys)); !slices.Equal(got, tt.want) {
					t.Fatalf("re-derivation %d = %v, want the identical %v — the order must not vary by run", again, got, tt.want)
				}
			}
		})
	}
}

// TestRowOrder_TotalAndDeterministic pins the property the whole ordering exists
// for: the union comes back in ONE sequence, whatever order it was assembled in.
//
// The pre-sort input is enumeration order — the built-ins, then os.ReadDir's,
// then the persisted leftovers — which is neither alphabetical nor stable across
// filesystems. So the retained entries are permuted and re-derived, and every
// permutation must produce the identical union, down to each row's label and
// source and not merely its key. Without that, §13.3's panel fixtures are not
// reproducible and §9.5's adjacency argument holds only by accident.
//
// The fixture is deliberately every row shape at once: a valid drop-in, a broken
// one, a `reserved name` collision, a `bad name` file, a dead persisted slug and
// a charset-rejected persisted string.
func TestRowOrder_TotalAndDeterministic(t *testing.T) {
	if got, want := theme.BuiltinSlugs(), []string{"nord", "tokyo-night", "tokyo-night-day"}; !slices.Equal(got, want) {
		t.Fatalf("the shipped built-ins are %v, want %v — the canonical sequence below states the WHOLE ordered union, so a new built-in belongs in it", got, want)
	}

	dir := t.TempDir()
	writeTheme(t, dir, "nord.theme", themeLines())
	writeTheme(t, dir, "Bad_Name.theme", themeLines())
	writeTheme(t, dir, "zz-late.theme", themeLines())
	writeTheme(t, dir, "aa-early.theme", withValue(themeLines(), "canvas", "blue"))
	keys := theme.RawKeys{Light: "ghost", Dark: "../evil"}
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	enumeration, union := loader.Open(dir, keys)

	want := []string{"../evil", "aa-early", "Bad_Name.theme", "ghost", "nord", "nord.theme", "tokyo-night", "tokyo-night-day", "zz-late"}
	if got := rowLabels(union); !slices.Equal(got, want) {
		t.Fatalf("union labels = %v, want %v", got, want)
	}

	tests := []struct {
		name  string
		order []int
	}{
		{name: "the order the directory was read in", order: []int{0, 1, 2, 3}},
		{name: "reversed", order: []int{3, 2, 1, 0}},
		{name: "rotated", order: []int{1, 2, 3, 0}},
		{name: "the ends swapped", order: []int{3, 1, 2, 0}},
		{name: "the collision read first", order: []int{2, 0, 3, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shuffled := theme.Enumeration{
				Entries:     permuted(t, enumeration.Entries, tt.order),
				DirUnusable: enumeration.DirUnusable,
				DirPath:     enumeration.DirPath,
			}

			again := loader.Reassemble(shuffled, keys)

			if got, want := rowIdentities(again), rowIdentities(union); !slices.Equal(got, want) {
				t.Errorf("re-derived union = %v, want the identical %v — the order must not carry any of the input's", got, want)
			}
		})
	}
}

// TestRowOrder_SortKeyAndLabelAreSeparateValues pins the independence of the two
// derived values directly, on the row where they disagree: renaming a `reserved
// name` file moves its LABEL and does not move its POSITION.
//
// Either value re-derived from the other collapses here. A label read off the
// sort key would show `nord` whatever the file is called, losing the one thing
// the label is for — saying which of the two rows is the user's file. A sort key
// read off the label would send the row wherever the filename sorts, which is
// away from the built-in whose collision is the row's entire content.
//
// The rows are built by hand because no directory can stage this: a reserved
// name is reserved BY its filename, so the real enumerator cannot vary one
// without varying the other. A Row is an ordinary value (§13.3), which is what
// makes the pair separable at all.
func TestRowOrder_SortKeyAndLabelAreSeparateValues(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{name: "the filename a reserved-name file really has", filename: "nord.theme"},
		{name: "a filename that would sort to the head", filename: "AAA.theme"},
		{name: "a filename that would sort to the tail", filename: "zzz.theme"},
	}

	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))
	wantAt := builtinIndex(t, "nord") + 1

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enumeration := theme.Enumeration{Entries: []theme.Entry{{
				Path:      "/themes/" + tt.filename,
				Filename:  tt.filename,
				Slug:      "nord",
				Rejection: &theme.Rejection{Reason: theme.ReasonReservedName},
			}}}

			union := loader.Reassemble(enumeration, theme.RawKeys{})

			if got, want := union.Rows[wantAt].Label(), tt.filename; got != want {
				t.Errorf("the reserved-name row's label = %q, want the filename %q — the label follows the file", got, want)
			}
			if got, want := union.Rows[wantAt].SortKey(), "nord"; got != want {
				t.Errorf("the reserved-name row's sort key = %q, want the slug %q — the key does not follow the file", got, want)
			}
			if got := rowSortKeys(union); !slices.Equal(got, slices.Insert(theme.BuiltinSlugs(), wantAt, "nord")) {
				t.Errorf("union sort keys = %v, want the row still beneath its built-in — renaming the file must not move it", got)
			}
		})
	}
}

// TestRowOrder_UnionIsOrderedOnReturn pins WHERE the ordering lives: inside the
// assembler, so both entry points hand back an ordered union and no consumer
// sorts.
//
// §9.2's post-commit recompute and §5.8's `Esc` re-resolution both go through
// Reassemble rather than Open, and a sort applied by the panel on open alone
// would leave every one of those re-derivations in raw assembly order — a list
// that reshuffles itself the moment the user presses `Enter`.
func TestRowOrder_UnionIsOrderedOnReturn(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "zz-late.theme", themeLines())
	writeTheme(t, dir, "aa-early.theme", themeLines())
	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))

	enumeration, opened := loader.Open(dir, theme.RawKeys{Theme: "zzz-ghost"})

	want := append(append([]string{"aa-early"}, theme.BuiltinSlugs()...), "zz-late", "zzz-ghost")
	if got := rowSortKeys(opened); !slices.Equal(got, want) {
		t.Errorf("the opened union's sort keys = %v, want %v", got, want)
	}

	again := loader.Reassemble(enumeration, theme.RawKeys{Theme: "../evil"})

	want = append(append([]string{"../evil", "aa-early"}, theme.BuiltinSlugs()...), "zz-late")
	if got := rowSortKeys(again); !slices.Equal(got, want) {
		t.Errorf("the re-derived union's sort keys = %v, want %v — a recompute is ordered too", got, want)
	}
}

// TestRowOrder_NoVariantConcept pins what the ordering must NOT read: a row's
// palette.
//
// Ordering same-mode themes first was proposed as a mitigation for §9.2's
// mixed-mode flash and REJECTED — list order is alphabetical by slug and nothing
// else. So the identical row set is ordered three times with the palettes swapped
// underneath it, including the light/dark canvases a mode-aware sort would have
// to read, and the sequence may not move. The middle case interleaves modes
// against the alphabet, so any grouping at all would show.
func TestRowOrder_NoVariantConcept(t *testing.T) {
	light := theme.Theme{Canvas: theme.Token{Name: "canvas", Value: "#E1E2E7"}}
	dark := theme.Theme{Canvas: theme.Token{Name: "canvas", Value: "#0B0C14"}}

	tests := []struct {
		name        string
		early, late theme.Theme
	}{
		{name: "the earlier slug is dark", early: dark, late: light},
		{name: "the earlier slug is light", early: light, late: dark},
		{name: "neither row carries a palette at all", early: theme.Theme{}, late: theme.Theme{}},
	}

	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))
	want := append(append([]string{"aa-early"}, theme.BuiltinSlugs()...), "zz-late")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enumeration := theme.Enumeration{Entries: []theme.Entry{
				{Path: "/themes/aa-early.theme", Filename: "aa-early.theme", Slug: "aa-early", Theme: tt.early},
				{Path: "/themes/zz-late.theme", Filename: "zz-late.theme", Slug: "zz-late", Theme: tt.late},
			}}

			union := loader.Reassemble(enumeration, theme.RawKeys{})

			if got := rowSortKeys(union); !slices.Equal(got, want) {
				t.Errorf("union sort keys = %v, want %v — the palette is not an input to the ordering", got, want)
			}
		})
	}
}

// rowSortKeys lists the union's rows by §9.5's sort key, in the order the union
// hands them back.
func rowSortKeys(union theme.Union) []string {
	keys := make([]string, 0, len(union.Rows))
	for _, row := range union.Rows {
		keys = append(keys, row.SortKey())
	}
	return keys
}

// rowLabels lists the union's rows by §9.5's display label, in the order the
// union hands them back.
func rowLabels(union theme.Union) []string {
	labels := make([]string, 0, len(union.Rows))
	for _, row := range union.Rows {
		labels = append(labels, row.Label())
	}
	return labels
}

// rowIdentities renders every row as the three values §9.5's ordering has to
// place it by and present it as — its sort key, its label and its source — so a
// comparison of two derivations catches a row that moved, a row that changed and
// a row that was substituted for its twin alike.
func rowIdentities(union theme.Union) []string {
	identities := make([]string, 0, len(union.Rows))
	for _, row := range union.Rows {
		identities = append(identities, fmt.Sprintf("%s|%s|%d", row.SortKey(), row.Label(), row.Source))
	}
	return identities
}

// onlyRejectedRow fails the test unless exactly one row carries the given §6.2
// reason, and returns it.
func onlyRejectedRow(t *testing.T, union theme.Union, reason theme.Reason) theme.Row {
	t.Helper()

	rows := []theme.Row{}
	for _, row := range union.Rows {
		if row.Rejection != nil && row.Rejection.Reason == reason {
			rows = append(rows, row)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("union carries %d rows rejected %q, want exactly 1: %+v", len(rows), reason, rows)
	}
	return rows[0]
}

// builtinIndex returns where the named built-in sits in BuiltinSlugs' sorted
// order, so an expectation states an interleaving rather than restating the
// shipped set.
func builtinIndex(t *testing.T, slug string) int {
	t.Helper()

	slugs := theme.BuiltinSlugs()
	at := slices.Index(slugs, slug)
	if at < 0 {
		t.Fatalf("%q is not a built-in slug (%v) — the fixture has nothing to collide with", slug, slugs)
	}
	return at
}

// permuted returns the entries in the given order, which is how a test stages a
// directory read that arrived in some order other than the one it really did.
func permuted(t *testing.T, entries []theme.Entry, order []int) []theme.Entry {
	t.Helper()

	if len(order) != len(entries) {
		t.Fatalf("the permutation covers %d of %d entries, want every one — a partial shuffle would drop rows rather than move them", len(order), len(entries))
	}
	shuffled := make([]theme.Entry, 0, len(order))
	for _, at := range order {
		shuffled = append(shuffled, entries[at])
	}
	return shuffled
}
