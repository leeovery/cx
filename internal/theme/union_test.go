package theme_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

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
		t.Errorf("second %q row filename = %q, want %q — a reserved-name row is labelled by its filename", "nord", got, want)
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

func TestUnion_EnumeratedFiresPerOpenUndeduped(t *testing.T) {
	dir := t.TempDir()
	themetest.WriteWithCanvas(t, dir, "bad-colour.theme", "blue")
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

	enumerated := sink.RecordsWithMessage("enumerated")
	if len(enumerated) != opens {
		t.Fatalf("%d opens emitted %d `enumerated` records, want one each:\n%s", opens, len(enumerated), sink.Body())
	}
	if rejected := sink.RecordsWithMessage("rejected"); len(rejected) != 2 {
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

func TestUnion_DiscardSilencesEnumerated(t *testing.T) {
	sink := logtest.Install(t)
	dir := t.TempDir()
	themetest.WriteWithCanvas(t, dir, "bad-colour.theme", "blue")
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

func TestUnion_ZeroValueLoaderIsASilentSeam(t *testing.T) {
	sink := logtest.Install(t)
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

func TestUnion_PersistedInvalidFileIsOneRow(t *testing.T) {
	dir := t.TempDir()
	themetest.WriteWithCanvas(t, dir, "nord-lee.theme", "blue")
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

func TestUnion_UnresolvablePersistedSlugIsUnreadableWhenDirUnusable(t *testing.T) {
	dir := themesDirWithOneTheme(t)
	_ = themetest.DenyDir(t, dir)
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

func TestUnion_ConstantContributesOnlyTheConstant(t *testing.T) {
	assembler := theme.Assembler{Loader: theme.NewSilentLoader()}

	_, union := assembler.Open(t.TempDir(), theme.RawKeys{Theme: "ghost", Light: "phantom", Dark: "spectre"})

	row := onlyPersistedRow(t, union)
	if row.Slug != "ghost" {
		t.Errorf("persisted row slug = %q, want the constant %q — the slots are not read under a constant", row.Slug, "ghost")
	}
}

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
				got = append(got, row.Identity())
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("persisted rows = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnion_DirUnusableIsAFlagNotAMember(t *testing.T) {
	dir := themesDirWithOneTheme(t)
	_ = themetest.DenyDir(t, dir)
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

func TestUnion_CountAndRejectedAttrs(t *testing.T) {
	dir := t.TempDir()
	themetest.WriteWithCanvas(t, dir, "bad-colour.theme", "blue")
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

	record := sink.RecordsWithMessage("enumerated")[0]
	if got := record.IntAttr(t, "count"); got != int64(union.Count) {
		t.Errorf("record count = %d, want the union's %d", got, union.Count)
	}
	if got := record.IntAttr(t, "rejected"); got != int64(union.Rejected) {
		t.Errorf("record rejected = %d, want the union's %d", got, union.Rejected)
	}
}

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

func unionSlugs(union theme.Union) []string {
	slugs := make([]string, 0, len(union.Rows))
	for _, row := range union.Rows {
		slugs = append(slugs, row.Slug)
	}
	return slugs
}

func rowsWithSlug(union theme.Union, slug string) []theme.Row {
	rows := []theme.Row{}
	for _, row := range union.Rows {
		if row.Slug == slug {
			rows = append(rows, row)
		}
	}
	return rows
}

func onlyRowWithSlug(t *testing.T, union theme.Union, slug string) theme.Row {
	t.Helper()

	rows := rowsWithSlug(union, slug)
	if len(rows) != 1 {
		t.Fatalf("slug %q has %d rows, want exactly 1: %v", slug, len(rows), unionSlugs(union))
	}
	return rows[0]
}

func stripRowIdentity(row theme.Row) theme.Row {
	row.Slug = ""
	row.Filename = ""
	row.Source = theme.SourceBuiltin
	row.Theme = theme.Theme{}
	return row
}

func persistedRows(union theme.Union) []theme.Row {
	rows := []theme.Row{}
	for _, row := range union.Rows {
		if row.Source == theme.SourcePersisted {
			rows = append(rows, row)
		}
	}
	return rows
}

func onlyPersistedRow(t *testing.T, union theme.Union) theme.Row {
	t.Helper()

	rows := persistedRows(union)
	if len(rows) != 1 {
		t.Fatalf("union carries %d persisted rows, want exactly 1: %+v", len(rows), rows)
	}
	return rows[0]
}
