package theme

import (
	"slices"
	"testing"
)

// TestSortRows_BuiltinFirstIsARuleNotAnArtefactOfAssemblyOrder pins §9.5's
// third leg as what it claims to be: a RULE that settles the one tie guaranteed
// by construction, and not a side effect of the built-ins being assembled first.
//
// The distinction is invisible from outside the package and that is precisely
// why it is asserted here. Reassemble always appends the file rows AFTER the
// built-in rows, so a stable sort alone would produce a built-in-first result
// for every union the assembler can build — and would go on producing one right
// up until someone reordered the assembly, at which point the panel would start
// leading with the row explaining why a theme is unusable instead of with the
// theme. The input below is therefore the reverse of anything Reassemble
// produces: the `reserved name` file FIRST, its built-in second.
//
// The rows are minimal on purpose. The tie is the whole fixture — an identical
// sort key, which is the definition of `reserved name` (§6.2) — so nothing else
// about them may participate in the outcome.
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
