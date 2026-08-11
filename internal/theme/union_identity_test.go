package theme_test

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// TestRowIdentity_Precedence pins what a row IS, independent of where it is
// listed: the slug wherever one exists, else the filename, else the raw persisted
// string.
//
// Exactly one arm applies per row shape, which is what makes every row
// identifiable — the badge table keys on this value and the panel's cursor
// re-anchors to it across a recompute, so a row shape yielding nothing would be a
// row that cannot be found again.
//
// The rows are built by hand because the claim is about the Row shape rather than
// about how one comes to exist.
func TestRowIdentity_Precedence(t *testing.T) {
	tests := []struct {
		name string
		row  theme.Row
		want string
	}{
		{
			name: "a slugged row identifies by its slug",
			row:  theme.Row{Slug: "nord-lee", Filename: "nord-lee.theme", Source: theme.SourceFile},
			want: "nord-lee",
		},
		{
			name: "a bad-name file has only its filename",
			row:  theme.Row{Filename: "Nord Lee.theme", Source: theme.SourceFile, Rejection: &theme.Rejection{Reason: theme.ReasonBadName}},
			want: "Nord Lee.theme",
		},
		{
			name: "a charset-rejected persisted value has only its raw string",
			row:  theme.Row{Persisted: "../evil", Source: theme.SourcePersisted, Rejection: &theme.Rejection{Reason: theme.ReasonBadName}},
			want: "../evil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.row.Identity(); got != tt.want {
				t.Errorf("Identity() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRowIdentity_SortKeyIsTodayTheIdentity pins the relationship the two values
// hold right now: ordering by identity is a choice, and this is where a change to
// it would announce itself rather than silently relocating the cursor anchor and
// the badge lookup with it.
func TestRowIdentity_SortKeyIsTodayTheIdentity(t *testing.T) {
	rows := []theme.Row{
		{Slug: theme.DefaultDarkSlug, Source: theme.SourceBuiltin},
		{Slug: "nord", Filename: "nord.theme", Source: theme.SourceFile, Rejection: &theme.Rejection{Reason: theme.ReasonReservedName}},
		{Filename: "Nord Lee.theme", Source: theme.SourceFile, Rejection: &theme.Rejection{Reason: theme.ReasonBadName}},
		{Persisted: "../evil", Source: theme.SourcePersisted, Rejection: &theme.Rejection{Reason: theme.ReasonBadName}},
	}

	for _, row := range rows {
		if got, want := row.SortKey(), row.Identity(); got != want {
			t.Errorf("SortKey() = %q, want the identity %q", got, want)
		}
	}
}
