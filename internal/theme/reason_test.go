package theme_test

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

var _ error = (*theme.Rejection)(nil)

func TestReason_LabelsAreTheTerseVocabulary(t *testing.T) {
	got := []theme.Reason{
		theme.ReasonBadName,
		theme.ReasonReservedName,
		theme.ReasonUnreadable,
		theme.ReasonBadSyntax,
		theme.ReasonBadColour,
		theme.ReasonMissingTokens,
		theme.ReasonNotFound,
	}

	want := []theme.Reason{
		"bad name",
		"reserved name",
		"unreadable",
		"bad syntax",
		"bad colour",
		"missing tokens",
		"not found",
	}

	if !slices.Equal(got, want) {
		t.Errorf("reason labels = %v, want %v", got, want)
	}
}

func TestRejection_ErrorRendersReasonAndDetail(t *testing.T) {
	tests := []struct {
		name      string
		rejection theme.Rejection
		want      string
	}{
		{
			name:      "reason and detail",
			rejection: theme.Rejection{Reason: theme.ReasonBadSyntax, Detail: "line 12: duplicate key text.primary", Line: 12},
			want:      "bad syntax: line 12: duplicate key text.primary",
		},
		{
			name:      "reason alone",
			rejection: theme.Rejection{Reason: theme.ReasonNotFound},
			want:      "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rejection.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
