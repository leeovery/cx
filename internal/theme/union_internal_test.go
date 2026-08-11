package theme

import (
	"slices"
	"testing"
)

func TestSortRows_BuiltinFirstIsARuleNotAnArtefactOfAssemblyOrder(t *testing.T) {
	collider := Row{
		Slug:      "nord",
		Filename:  "nord.theme",
		Source:    SourceFile,
		Rejection: &Rejection{Reason: ReasonReservedName},
	}
	builtin := Row{Slug: "nord", Source: SourceBuiltin}
	rows := []Row{collider, builtin}

	sortRows(rows)

	if rows[0].SortKey() != rows[1].SortKey() {
		t.Fatalf("the fixture's keys are %q and %q, want one identical key — there is no tie to settle otherwise", rows[0].SortKey(), rows[1].SortKey())
	}
	if got := []RowSource{rows[0].Source, rows[1].Source}; !slices.Equal(got, []RowSource{SourceBuiltin, SourceFile}) {
		t.Errorf("the tied rows came back as %v, want the built-in first — the valid, selectable thing to act on, then the row explaining why the file is not it", got)
	}
	if got, want := rows[1].Label(), "nord.theme"; got != want {
		t.Errorf("the trailing row's label = %q, want %q — the rejected file, not a second copy of the built-in", got, want)
	}
}

func TestPersistedRows_NothingContributedIsEmptyNotNil(t *testing.T) {
	rows := persistedRows(nil, Enumeration{}, RawKeys{})

	if rows == nil {
		t.Errorf("persistedRows over unset keys = nil, want an empty slice — one shape for \"nothing found\"")
	}
	if len(rows) != 0 {
		t.Errorf("persistedRows over unset keys = %+v, want no rows", rows)
	}
}
