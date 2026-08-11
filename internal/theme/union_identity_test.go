package theme_test

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

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
