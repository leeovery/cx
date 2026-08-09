package theme_test

import (
	"cmp"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// TestUnion_AbsentDirectoryIsBuiltinsOnly pins the common install against the union rule's
// union: no themes directory at all yields the built-ins and nothing else — no
// rejections, no error, and no `directory unusable` record.
//
// The absent directory is deliberately NOT an error state, so the union
// has to be indistinguishable from a usable-but-empty one: the user has simply
// never dropped a theme in. `theme: enumerated` still fires, because the panel
// still opened, and its count is the built-ins.
//
// Every built-in row carries its PALETTE, which is what makes the panel's preview
// an O(1) restyle from values already in hand rather than a file read per
// keystroke.
func TestUnion_AbsentDirectoryIsBuiltinsOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")
	logger, sink := logtest.NewCaptureLogger(t)
	assembler := theme.Assembler{Loader: theme.NewLoader(theme.NewEventLogger(logger))}

	enumeration, union := assembler.Open(dir, theme.RawKeys{})

	if enumeration.DirUnusable {
		t.Errorf("enumeration reports the directory unusable, want an absent directory treated silently")
	}
	if len(enumeration.Entries) != 0 {
		t.Errorf("enumeration carries %d entries, want none: %+v", len(enumeration.Entries), enumeration.Entries)
	}
	if union.DirUnusable {
		t.Errorf("union reports the directory unusable, want the flag clear for an absent directory")
	}
	if got, want := unionSlugs(union), theme.BuiltinSlugs(); !slices.Equal(got, want) {
		t.Errorf("union rows = %v, want the built-ins %v", got, want)
	}
	for _, row := range union.Rows {
		if row.Source != theme.SourceBuiltin {
			t.Errorf("row %q source = %v, want %v", row.Slug, row.Source, theme.SourceBuiltin)
		}
		if !row.Selectable() {
			t.Errorf("row %q carries the rejection %v, want every built-in selectable", row.Slug, row.Rejection)
		}
		if row.Theme == (theme.Theme{}) {
			t.Errorf("row %q carries the zero palette, want the built-in's own — the panel previews from values already in hand", row.Slug)
		}
	}
	if union.Rejected != 0 {
		t.Errorf("union rejected = %d, want 0", union.Rejected)
	}

	record := sink.OnlyRecord(t)
	if got, want := record.Msg, "enumerated"; got != want {
		t.Errorf("record message = %q, want %q — an absent directory earns no `directory unusable` line", got, want)
	}
}

// TestUnion_ReservedNameIsTheOnlyTwoRowCase pins the union rule's one legitimate exception
// to "one slug is one row": a `nord.theme` drop-in beside the `nord` built-in
// stands as TWO rows.
//
// The collision IS the reason's entire content, so deduping the file
// against the built-in it collides with would delete the only explanation the
// user gets for why their file never appeared. The built-in comes first — the
// valid, selectable thing to act on, immediately followed by the row saying why
// the file is not it.
//
// Every OTHER slug in the same directory stays single, which is the structural
// half of the claim: the enumeration rule mints no duplicate slug, so no other dedup arises
// between the built-in rows and the file rows at all.
func TestUnion_ReservedNameIsTheOnlyTwoRowCase(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "nord.theme", themetest.Lines())
	themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	_, union := assembler.Open(dir, theme.RawKeys{})

	collided := rowsWithSlug(union, "nord")
	if len(collided) != 2 {
		t.Fatalf("slug %q has %d rows, want 2 — the built-in and its reserved-name collider: %v", "nord", len(collided), unionSlugs(union))
	}
	if collided[0].Source != theme.SourceBuiltin || !collided[0].Selectable() {
		t.Errorf("first %q row = %+v, want the valid built-in", "nord", collided[0])
	}
	if collided[1].Source != theme.SourceFile {
		t.Errorf("second %q row source = %v, want %v", "nord", collided[1].Source, theme.SourceFile)
	}
	if collided[1].Rejection == nil || collided[1].Rejection.Reason != theme.ReasonReservedName {
		t.Errorf("second %q row rejection = %v, want %q", "nord", collided[1].Rejection, theme.ReasonReservedName)
	}
	if got, want := collided[1].Filename, "nord.theme"; got != want {
		t.Errorf("second %q row filename = %q, want %q — §9.5 labels a reserved-name row by its filename", "nord", got, want)
	}

	for _, slug := range unionSlugs(union) {
		if slug == "nord" {
			continue
		}
		if rows := rowsWithSlug(union, slug); len(rows) != 1 {
			t.Errorf("slug %q has %d rows, want exactly 1 — `reserved name` is the only two-rows-for-one-slug case", slug, len(rows))
		}
	}
}

// TestUnion_BuiltinRowsCarryNoMarker pins the row-rendering rule's "built-in rows are
// deliberately indistinguishable from drop-in rows": a valid built-in and a valid drop-in
// differ in Source — which only the ordering tie-break reads — and in the
// filename an embedded theme does not have, and in NOTHING else.
//
// No reason, no marker and no flag may reach a built-in row, or the panel would
// quietly grow a two-tier list where the row-rendering rule promises one. The comparison is
// STRUCTURAL rather than field-by-field so a field added to Row later cannot
// carry a distinction in unnoticed.
func TestUnion_BuiltinRowsCarryNoMarker(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	_, union := assembler.Open(dir, theme.RawKeys{})

	builtin := onlyRowWithSlug(t, union, theme.DefaultDarkSlug)
	dropIn := onlyRowWithSlug(t, union, "nord-lee")

	if !builtin.Selectable() || !dropIn.Selectable() {
		t.Fatalf("built-in selectable = %v, drop-in selectable = %v, want both true", builtin.Selectable(), dropIn.Selectable())
	}
	if builtin.Theme == (theme.Theme{}) || dropIn.Theme == (theme.Theme{}) {
		t.Errorf("built-in palette = %+v, drop-in palette = %+v, want both populated", builtin.Theme, dropIn.Theme)
	}
	if builtin.Filename != "" {
		t.Errorf("built-in row filename = %q, want none — a built-in is embedded, not a directory entry", builtin.Filename)
	}
	if got, want := stripRowIdentity(builtin), stripRowIdentity(dropIn); got != want {
		t.Errorf("built-in row beyond its identity = %+v, want the drop-in's %+v — no reason, marker or flag may distinguish the two", got, want)
	}
}

// TestUnion_BrokenBuiltinNeverBecomesASelectableBlankRow pins what the union
// does in the one state the build-time guarantee says cannot ship: a binary
// whose embedded set does not supply a built-in.
//
// The two causes are deliberately NOT collapsed, because the right answer
// differs. A built-in the binary cannot supply at all yields NO ROW — "this
// theme is not in this binary" is exactly what the list should say, and the
// alternative is worse than useless: a row with no rejection and no palette is
// SELECTABLE, so committing it would paint the panel from a zero Theme. One that
// is present and unparseable keeps its row and carries its rejection like any
// other, since there is a named theme to explain.
//
// The state is reachable only through Loader.BuiltinSource, which exists
// precisely because an unreachable path with no test is a path nobody has ever
// run.
func TestUnion_BrokenBuiltinNeverBecomesASelectableBlankRow(t *testing.T) {
	tests := []struct {
		name    string
		source  func(string) ([]byte, bool)
		wantRow bool
	}{
		{name: "absent from the embedded set", source: withoutBuiltin(theme.DefaultDarkSlug)},
		{name: "present and unparseable", source: corruptBuiltin(theme.DefaultDarkSlug), wantRow: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := theme.NewSilentLoader()
			loader.BuiltinSource = tt.source

			_, union := theme.Assembler{Loader: loader}.Open(t.TempDir(), theme.RawKeys{})

			rows := rowsWithSlug(union, theme.DefaultDarkSlug)
			if !tt.wantRow {
				if len(rows) != 0 {
					t.Fatalf("slug %q has %d rows, want none — a built-in the binary cannot supply is absent, never a selectable blank: %+v", theme.DefaultDarkSlug, len(rows), rows)
				}
				if got, want := union.Count, len(theme.BuiltinSlugs())-1; got != want {
					t.Errorf("union count = %d, want %d — the remaining built-ins", got, want)
				}
				if union.Rejected != 0 {
					t.Errorf("union rejected = %d, want 0 — an absent built-in is no row at all, so it is no rejection either", union.Rejected)
				}
				return
			}
			if len(rows) != 1 {
				t.Fatalf("slug %q has %d rows, want exactly 1 — a built-in that is present and broken keeps its row: %v", theme.DefaultDarkSlug, len(rows), unionSlugs(union))
			}
			if rows[0].Selectable() {
				t.Errorf("row %q is selectable, want it carrying the rejection its bytes earned", theme.DefaultDarkSlug)
			}
			if rows[0].Theme != (theme.Theme{}) {
				t.Errorf("row %q carries the palette %+v alongside a rejection, want the zero one — a rejected row is never half-populated", theme.DefaultDarkSlug, rows[0].Theme)
			}
			if union.Rejected != 1 {
				t.Errorf("union rejected = %d, want 1 — the broken built-in's row", union.Rejected)
			}
		})
	}
}

// TestUnion_EnumeratedFiresPerOpenUndeduped pins the `theme` log component's cadence for the
// one event this assembly emits: `theme: enumerated` is a per-event INFO and fires on
// EVERY open, so five opens are five lines rather than one.
//
// Its neighbours behave the opposite way and must keep doing so in the same
// process: enumeration re-reads the directory on every open, and the loader's
// per-process dedup is what keeps five opens over the same broken directory from
// producing five identical WARN sets.
//
// The attrs are pinned exactly — `count` and `rejected`, in that order, and
// nothing else. Both are stated because the union makes them genuinely ambiguous:
// "files considered" and "valid themes" give different numbers on the same
// install.
func TestUnion_EnumeratedFiresPerOpenUndeduped(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "bad-colour.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))
	themetest.Write(t, dir, "missing.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	themetest.Write(t, dir, "valid.theme", themetest.Lines())
	logger, sink := logtest.NewCaptureLogger(t)
	assembler := theme.Assembler{Loader: theme.NewLoader(theme.NewEventLogger(logger))}

	const opens = 5
	for open := range opens {
		if _, union := assembler.Open(dir, theme.RawKeys{}); len(union.Rows) != len(theme.BuiltinSlugs())+3 {
			t.Fatalf("open %d: union has %d rows, want the built-ins plus the three staged files", open, len(union.Rows))
		}
	}

	enumerated := recordsNamed(sink, "enumerated")
	if len(enumerated) != opens {
		t.Fatalf("%d opens emitted %d `enumerated` records, want one each:\n%s", opens, len(enumerated), sink.Body())
	}
	if rejected := recordsNamed(sink, "rejected"); len(rejected) != 2 {
		t.Errorf("%d opens emitted %d `rejected` records, want one per distinct slug+reason (2):\n%s", opens, len(rejected), sink.Body())
	}

	for i, record := range enumerated {
		if record.Level != slog.LevelInfo {
			t.Errorf("record %d emitted at %v, want %v", i, record.Level, slog.LevelInfo)
		}
		if want := []string{"count", "rejected"}; !slices.Equal(record.Keys, want) {
			t.Errorf("record %d keys = %v, want exactly %v", i, record.Keys, want)
		}
		if got, want := record.IntAttr(t, "count"), int64(len(theme.BuiltinSlugs())+3); got != want {
			t.Errorf("record %d count = %d, want %d — rows produced, built-ins included", i, got, want)
		}
		if got, want := record.IntAttr(t, "rejected"), int64(2); got != want {
			t.Errorf("record %d rejected = %d, want %d — the unselectable subset", i, got, want)
		}
	}
}

// TestUnion_DiscardSilencesEnumerated pins the diagnose-shaped callers' contract
// on the new event: a silenced Loader produces ZERO records for any sequence of
// opens.
//
// The process handler captures everything for the test's duration, so a record
// reaching the sink would mean the discard logger had been bypassed rather than
// merely being quiet. The union is asserted non-empty so the silence cannot be
// vacuous — the opens really did produce rows, and really did meet a rejected
// file and an unresolvable persisted slug on the way.
func TestUnion_DiscardSilencesEnumerated(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	dir := t.TempDir()
	themetest.Write(t, dir, "bad-colour.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	for open := range 5 {
		_, union := assembler.Open(dir, theme.RawKeys{Theme: "ghost"})
		if union.Count == 0 || union.Rejected == 0 {
			t.Fatalf("open %d: union = %+v, want rows and rejections so the silence is not vacuous", open, union)
		}
	}

	if lines := sink.Lines(); len(lines) != 0 {
		t.Errorf("a discard-backed loader emitted %d records over five opens, want none:\n%s", len(lines), sink.Body())
	}
}

// TestUnion_ZeroValueLoaderIsASilentSeam pins the other half of the emission
// contract on the union's entry point: a Loader ASSEMBLED BY HAND carries no
// event seam at all, and Open is silent over it rather than dereferencing a nil.
//
// It is the shape every other emitting entry point in this package is already
// driven through, and the one a caller reaches by writing theme.Loader{} instead
// of NewLoader — so the new event has to tolerate it exactly as its neighbours
// do (see Loader.events).
func TestUnion_ZeroValueLoaderIsASilentSeam(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	dir := t.TempDir()
	themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())

	_, union := theme.Assembler{Loader: theme.Loader{}}.Open(dir, theme.RawKeys{Theme: "ghost"})

	if len(rowsWithSlug(union, "nord-lee")) != 1 || len(persistedRows(union)) != 1 {
		t.Fatalf("union = %v, want the staged file and the dead persisted slug — the silence must not be vacuous", unionSlugs(union))
	}
	if lines := sink.Lines(); len(lines) != 0 {
		t.Errorf("a seamless loader emitted %d records, want none:\n%s", len(lines), sink.Body())
	}
}

// TestUnion_IsAnOrdinaryValue pins the harness contract's fixture requirement: a Union is an
// ordinary value with exported fields, constructible WHOLESALE with no loader, no
// themes directory and no filesystem of any kind.
//
// That is what lets internal/capture fake a panel's row set under its
// no-real-config import guard — including the invalid rows that could otherwise
// never be rendered offline, which is the whole reason the seam returns a
// finished union rather than a directory listing.
func TestUnion_IsAnOrdinaryValue(t *testing.T) {
	union := theme.Union{
		Rows: []theme.Row{
			{Slug: "nord", Source: theme.SourceBuiltin, Theme: theme.Theme{Canvas: theme.Token{Name: "canvas", Value: "#101010"}}},
			{Slug: "ghost", Source: theme.SourcePersisted, Rejection: &theme.Rejection{Reason: theme.ReasonNotFound}},
		},
		DirUnusable: true,
		Count:       2,
		Rejected:    1,
	}

	if !union.Rows[0].Selectable() {
		t.Error("a hand-built row with no rejection is unselectable, want selectable")
	}
	if union.Rows[1].Selectable() {
		t.Error("a hand-built row carrying a rejection is selectable, want unselectable")
	}
	if got, want := union.Rows[0].Theme.Canvas.Value, "#101010"; got != want {
		t.Errorf("hand-built palette canvas = %q, want %q", got, want)
	}
}

// TestUnion_PersistedBuiltinIsOneRow pins the distinction the union rule calls load-bearing:
// the union keys on "RESOLVES", not on "has a file".
//
// A built-in is embedded rather than a directory entry, so a file-existence rule
// would mint a second `⚠ not found` row for every persisted built-in slug — which
// is the state the panel's MOST COMMON ACTION produces, pressing `Enter` on
// `tokyo-night`. The persisted slug contributes nothing because the built-in's
// row already IS its row.
func TestUnion_PersistedBuiltinIsOneRow(t *testing.T) {
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	_, union := assembler.Open(t.TempDir(), theme.RawKeys{Theme: theme.DefaultDarkSlug})

	rows := rowsWithSlug(union, theme.DefaultDarkSlug)
	if len(rows) != 1 {
		t.Fatalf("slug %q has %d rows, want exactly 1 — one slug is one row: %v", theme.DefaultDarkSlug, len(rows), unionSlugs(union))
	}
	if rows[0].Source != theme.SourceBuiltin {
		t.Errorf("row %q source = %v, want %v — the persisted slug IS the built-in's row", theme.DefaultDarkSlug, rows[0].Source, theme.SourceBuiltin)
	}
	if !rows[0].Selectable() {
		t.Errorf("row %q carries the rejection %v, want it selectable", theme.DefaultDarkSlug, rows[0].Rejection)
	}
	if persisted := persistedRows(union); len(persisted) != 0 {
		t.Errorf("union carries %d persisted rows, want none — a resolving slug mints no `not found` twin: %+v", len(persisted), persisted)
	}
}

// TestUnion_PersistedInvalidFileIsOneRow pins the same rule for the other half of
// the union rule's sentence: a persisted slug naming an existing-but-INVALID file is that
// file's row, carrying its reason and its badge.
//
// One slug is one row always, so the failure here would be the same shape as the
// built-in's: a second `⚠ not found` row beside a file that plainly exists,
// sending the user to look for a missing file rather than at the broken one.
func TestUnion_PersistedInvalidFileIsOneRow(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "nord-lee.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	_, union := assembler.Open(dir, theme.RawKeys{Theme: "nord-lee"})

	rows := rowsWithSlug(union, "nord-lee")
	if len(rows) != 1 {
		t.Fatalf("slug %q has %d rows, want exactly 1: %v", "nord-lee", len(rows), unionSlugs(union))
	}
	if rows[0].Source != theme.SourceFile {
		t.Errorf("row %q source = %v, want %v — the persisted slug IS the file's row", "nord-lee", rows[0].Source, theme.SourceFile)
	}
	if rows[0].Rejection == nil || rows[0].Rejection.Reason != theme.ReasonBadColour {
		t.Errorf("row %q rejection = %v, want %q carried across from the ladder", "nord-lee", rows[0].Rejection, theme.ReasonBadColour)
	}
	if persisted := persistedRows(union); len(persisted) != 0 {
		t.Errorf("union carries %d persisted rows, want none: %+v", len(persisted), persisted)
	}
}

// TestUnion_UnresolvablePersistedSlugIsNotFound pins the third member of the union rule's
// union: a persisted slug that resolves to NEITHER a built-in nor a file gets a
// row of its own — marked, unselectable, reason `not found`.
//
// This is what covers a deleted file, a renamed file and a typo in prefs.json,
// and it is what makes "the `●` marker always has something to sit on" true.
// Portal falls back silently and never overwrites the persisted name, so without
// this row the only signal the user gets is "my colours changed".
func TestUnion_UnresolvablePersistedSlugIsNotFound(t *testing.T) {
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	_, union := assembler.Open(t.TempDir(), theme.RawKeys{Theme: "ghost"})

	row := onlyPersistedRow(t, union)
	if row.Slug != "ghost" {
		t.Errorf("persisted row slug = %q, want %q", row.Slug, "ghost")
	}
	if row.Persisted != "" {
		t.Errorf("persisted row carries Persisted = %q alongside a slug, want it empty — the raw value is only what a row with NO slug is labelled by", row.Persisted)
	}
	if row.Filename != "" {
		t.Errorf("persisted row carries Filename = %q, want none — nothing was enumerated for it", row.Filename)
	}
	if row.Rejection == nil || row.Rejection.Reason != theme.ReasonNotFound {
		t.Fatalf("persisted row rejection = %v, want %q", row.Rejection, theme.ReasonNotFound)
	}
	if row.Selectable() {
		t.Error("persisted row is selectable, want it unselectable — there is no theme behind it")
	}
}

// TestUnion_UnresolvablePersistedSlugIsUnreadableWhenDirUnusable pins the directory-resolution
// rule's distinction on the row the user actually reads: the SAME missing slug reports
// `unreadable` rather than `not found` when the directory itself is unusable.
//
// `not found` sends the user to check the filename; `unreadable` sends them to
// check permissions — and permissions is the actual problem. The theme may well
// be sitting right there in a directory nothing can list.
func TestUnion_UnresolvablePersistedSlugIsUnreadableWhenDirUnusable(t *testing.T) {
	dir := unreadableDir(t)
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	enumeration, union := assembler.Open(dir, theme.RawKeys{Theme: "ghost"})

	if !enumeration.DirUnusable || !union.DirUnusable {
		t.Fatalf("enumeration/union DirUnusable = %v/%v, want both true", enumeration.DirUnusable, union.DirUnusable)
	}
	if got, want := enumeration.DirPath, dir; got != want {
		t.Errorf("enumeration DirPath = %q, want %q", got, want)
	}
	row := onlyPersistedRow(t, union)
	if row.Rejection == nil || row.Rejection.Reason != theme.ReasonUnreadable {
		t.Errorf("persisted row rejection = %v, want %q — permissions is the actual problem", row.Rejection, theme.ReasonUnreadable)
	}
}

// TestUnion_CharsetRejectedPersistedStringIsBadName pins the union rule's other reason
// rule: a persisted string rejected by the validate-before-use rule's charset check is `bad
// name`, NEVER `not found`.
//
// Each terse reason has exactly one condition, and telling a user their file is
// missing when they typed an illegal name sends them looking in the wrong place.
// The row is labelled by the raw value because it has nothing else — no slug was
// derived and no file was sought, which is also what stops a hand-edited
// `../something` ever becoming a path component.
func TestUnion_CharsetRejectedPersistedStringIsBadName(t *testing.T) {
	const illegal = "../evil"
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	_, union := assembler.Open(t.TempDir(), theme.RawKeys{Theme: illegal})

	row := onlyPersistedRow(t, union)
	if row.Slug != "" {
		t.Errorf("charset-rejected row slug = %q, want empty — the value yields no usable identity", row.Slug)
	}
	if row.Persisted != illegal {
		t.Errorf("charset-rejected row Persisted = %q, want the raw value %q", row.Persisted, illegal)
	}
	if row.Rejection == nil || row.Rejection.Reason != theme.ReasonBadName {
		t.Fatalf("charset-rejected row rejection = %v, want %q", row.Rejection, theme.ReasonBadName)
	}
	if row.Rejection.BadNameCause != theme.BadNameSlug {
		t.Errorf("charset-rejected row cause = %v, want %v — no extension is involved in a persisted value", row.Rejection.BadNameCause, theme.BadNameSlug)
	}
}

// TestUnion_ConstantContributesOnlyTheConstant pins the constant-or-pair rule's tiebreak
// inside the union: under a constant the two slot keys are NOT READ AT ALL, even when both
// name unresolvable slugs.
//
// A hand-edited prefs.json may legally carry all three keys — mutual exclusion is
// enforced on write, not on the file — so the union has to apply the same
// `theme`-wins rule doctor applies, in one place, or the panel would list rows
// for two slugs Portal is not reading and put the user to work fixing something
// with no effect.
func TestUnion_ConstantContributesOnlyTheConstant(t *testing.T) {
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	_, union := assembler.Open(t.TempDir(), theme.RawKeys{Theme: "ghost", Light: "phantom", Dark: "spectre"})

	row := onlyPersistedRow(t, union)
	if row.Slug != "ghost" {
		t.Errorf("persisted row slug = %q, want the constant %q — the slots are not read under a constant", row.Slug, "ghost")
	}
}

// TestUnion_BothSlotsSameMissingSlugIsOneRow pins the adaptive pair's arithmetic:
// each non-empty slot contributes, an unset one contributes nothing, and two
// slots naming the SAME value collapse to a single row.
//
// One slug is one row applies to the pair exactly as it applies everywhere else
// — a user who set both slots to the same deleted theme has one problem, not two
// — and the collapse is keyed on the persisted VALUE rather than on the derived
// slug, so a value yielding no slug at all collapses by the same rule.
//
// An unset slot holds the shipped default, which is a built-in and
// therefore already has a row; it is the "never set" line of the row-rendering rule's badge
// table rather than a nomination the user made.
//
// The expectations are in the row-rendering rule's order rather than in slot order, because
// the union arrives sorted (see sortRows). Which slot contributed a row is not
// something the panel — or this test — reads position for.
func TestUnion_BothSlotsSameMissingSlugIsOneRow(t *testing.T) {
	tests := []struct {
		name string
		keys theme.RawKeys
		want []string
	}{
		{name: "both slots name the same missing slug", keys: theme.RawKeys{Light: "ghost", Dark: "ghost"}, want: []string{"ghost"}},
		{name: "each slot names its own missing slug", keys: theme.RawKeys{Light: "phantom", Dark: "ghost"}, want: []string{"ghost", "phantom"}},
		{name: "an unset slot holds the shipped default", keys: theme.RawKeys{Dark: "ghost"}, want: []string{"ghost"}},
		{name: "both slots name the same illegal string", keys: theme.RawKeys{Light: "../evil", Dark: "../evil"}, want: []string{"../evil"}},
	}

	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, union := assembler.Open(t.TempDir(), tt.keys)

			got := []string{}
			for _, row := range persistedRows(union) {
				got = append(got, cmp.Or(row.Slug, row.Persisted))
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("persisted rows = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestUnion_DirUnusableIsAFlagNotAMember pins the row-rendering rule's placement rule at the
// union level: the unusable-directory condition comes back as a FLAG and is never a row.
//
// The `⚠ dir unreadable` warning is viewport chrome pinned beneath the header,
// not a list row — a list row participates in pagination and would vanish the
// moment the user paged down, which is precisely the "completely in the dark"
// state it exists to prevent. Being outside Rows is also what keeps it out of
// `theme: enumerated`'s count.
//
// The built-in rows and the persisted rows still stand beneath it. The persisted
// ones especially: a user with an unreadable directory would otherwise lose the
// `●` entirely.
func TestUnion_DirUnusableIsAFlagNotAMember(t *testing.T) {
	dir := unreadableDir(t)
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	_, union := assembler.Open(dir, theme.RawKeys{Theme: "ghost"})

	if !union.DirUnusable {
		t.Fatal("union DirUnusable = false, want true")
	}
	if want := append([]string{"ghost"}, theme.BuiltinSlugs()...); !slices.Equal(unionSlugs(union), want) {
		t.Errorf("union rows = %v, want exactly %v — the directory condition is a flag, not a member", unionSlugs(union), want)
	}
	if union.Count != len(union.Rows) {
		t.Errorf("union count = %d, want len(Rows) = %d — the chrome row is never counted", union.Count, len(union.Rows))
	}
}

// TestUnion_CountAndRejectedAttrs pins the two values the `theme` log component makes `theme:
// enumerated` carry, over an install exercising every member of the union at
// once: valid and invalid files, built-ins, and two dead persisted slots.
//
// Count is rows PRODUCED and Rejected is the unselectable subset. Both are
// derived from the assembled rows rather than tallied along the way, and both are
// asserted against an independent recount of the union, so a drift between what
// the panel lists and what the log claims fails here.
func TestUnion_CountAndRejectedAttrs(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "bad-colour.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))
	themetest.Write(t, dir, "missing.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
	themetest.Write(t, dir, "valid.theme", themetest.Lines())
	logger, sink := logtest.NewCaptureLogger(t)
	assembler := theme.Assembler{Loader: theme.NewLoader(theme.NewEventLogger(logger))}

	_, union := assembler.Open(dir, theme.RawKeys{Light: "phantom", Dark: "ghost"})

	wantCount := len(theme.BuiltinSlugs()) + 3 + 2
	if len(union.Rows) != wantCount {
		t.Fatalf("union has %d rows, want %d (built-ins, three files, two dead slots): %v", len(union.Rows), wantCount, unionSlugs(union))
	}
	if union.Count != len(union.Rows) {
		t.Errorf("union count = %d, want len(Rows) = %d", union.Count, len(union.Rows))
	}
	wantRejected := 0
	for _, row := range union.Rows {
		if !row.Selectable() {
			wantRejected++
		}
	}
	if union.Rejected != wantRejected {
		t.Errorf("union rejected = %d, want %d — the rows carrying a rejection", union.Rejected, wantRejected)
	}
	if wantRejected != 4 {
		t.Errorf("the fixture produced %d unselectable rows, want 4 (two broken files, two dead slots)", wantRejected)
	}

	record := recordsNamed(sink, "enumerated")[0]
	if got := record.IntAttr(t, "count"); got != int64(union.Count) {
		t.Errorf("record count = %d, want the union's %d", got, union.Count)
	}
	if got := record.IntAttr(t, "rejected"); got != int64(union.Rejected) {
		t.Errorf("record rejected = %d, want the union's %d", got, union.Rejected)
	}
}

// TestUnion_ReassembleReadsNothing pins the entry point the picker idiom's post-commit
// recompute and the re-read-on-open rule's `Esc` re-resolution both depend on: the union
// re-derives from CHANGED prefs state with no fresh directory read and no event.
//
// The directory is REMOVED between the two calls, so a row for a file that no
// longer exists is proof the retained enumeration was used rather than re-read —
// and re-reading would additionally cost a syscall per keypress on a path that
// exists to be an O(1) restyle.
//
// Emitting nothing matters for the same reason: `theme: enumerated` is per PANEL
// OPEN, and firing it per recompute would turn one open into a line per
// keystroke.
func TestUnion_ReassembleReadsNothing(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
	logger, sink := logtest.NewCaptureLogger(t)
	assembler := theme.Assembler{Loader: theme.NewLoader(theme.NewEventLogger(logger))}

	enumeration, union := assembler.Open(dir, theme.RawKeys{})
	if len(rowsWithSlug(union, "nord-lee")) != 1 {
		t.Fatalf("union has no row for the staged file: %v", unionSlugs(union))
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove %s: %v", dir, err)
	}

	again := assembler.Reassemble(enumeration, theme.RawKeys{Theme: "ghost"})

	if len(rowsWithSlug(again, "nord-lee")) != 1 {
		t.Errorf("re-derived union = %v, want the retained file row — Reassemble re-read the directory", unionSlugs(again))
	}
	row := onlyPersistedRow(t, again)
	if row.Slug != "ghost" || row.Rejection == nil || row.Rejection.Reason != theme.ReasonNotFound {
		t.Errorf("re-derived persisted row = %+v, want %q reported %q — the changed keys are what it re-derives from", row, "ghost", theme.ReasonNotFound)
	}
	if again.Count != len(again.Rows) {
		t.Errorf("re-derived count = %d, want len(Rows) = %d", again.Count, len(again.Rows))
	}

	if records := sink.Records(); len(records) != 1 || records[0].Msg != "enumerated" {
		t.Errorf("the Open and the Reassemble emitted %d records, want only the open's `enumerated`:\n%s", len(records), sink.Body())
	}
}

// unionSlugs lists the union's row slugs in the row-rendering rule's display order, which is
// the order the union hands them back.
func unionSlugs(union theme.Union) []string {
	slugs := make([]string, 0, len(union.Rows))
	for _, row := range union.Rows {
		slugs = append(slugs, row.Slug)
	}
	return slugs
}

// rowsWithSlug returns the rows carrying the given slug, in the row-rendering rule's display
// order, which is the order the union hands them back.
func rowsWithSlug(union theme.Union, slug string) []theme.Row {
	rows := []theme.Row{}
	for _, row := range union.Rows {
		if row.Slug == slug {
			rows = append(rows, row)
		}
	}
	return rows
}

// onlyRowWithSlug fails the test unless exactly one row carries the slug, and
// returns it.
func onlyRowWithSlug(t *testing.T, union theme.Union, slug string) theme.Row {
	t.Helper()

	rows := rowsWithSlug(union, slug)
	if len(rows) != 1 {
		t.Fatalf("slug %q has %d rows, want exactly 1: %v", slug, len(rows), unionSlugs(union))
	}
	return rows[0]
}

// stripRowIdentity blanks what a row is legitimately allowed to differ in — the
// identity it is listed under, the palette it carries, the filename only a file
// has, and the Source consumed by the row-rendering rule's ordering tie-break — so what is
// left is everything a built-in and a valid drop-in must agree on.
func stripRowIdentity(row theme.Row) theme.Row {
	row.Slug = ""
	row.Filename = ""
	row.Source = theme.SourceBuiltin
	row.Theme = theme.Theme{}
	return row
}

// persistedRows returns the rows contributed by prefs.json — the slugs and
// strings that resolved to neither a built-in nor a file.
func persistedRows(union theme.Union) []theme.Row {
	rows := []theme.Row{}
	for _, row := range union.Rows {
		if row.Source == theme.SourcePersisted {
			rows = append(rows, row)
		}
	}
	return rows
}

// onlyPersistedRow fails the test unless prefs.json contributed exactly one row,
// and returns it.
func onlyPersistedRow(t *testing.T, union theme.Union) theme.Row {
	t.Helper()

	rows := persistedRows(union)
	if len(rows) != 1 {
		t.Fatalf("union carries %d persisted rows, want exactly 1: %+v", len(rows), rows)
	}
	return rows[0]
}
