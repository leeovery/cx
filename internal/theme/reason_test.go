package theme_test

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// A rejection is returned as a concrete *Rejection everywhere in this package,
// but it satisfies error so a caller that only wants to propagate one can.
var _ error = (*theme.Rejection)(nil)

// TestReason_LabelsAreTheTerseVocabulary pins §6.2's seven reject classes and
// the exact string each carries. These values are user-facing copy, not internal
// identifiers: the panel row renders the label verbatim behind §14A's "⚠ "
// prefix, so rewording a constant is a UI change and has to be a deliberate one.
//
// The constants are listed here in §6.2's short-circuit ladder order, with
// `not found` — which is outside the ladder — last.
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

// TestRejection_ErrorRendersReasonAndDetail pins the error string, which is the
// generic propagation form — the surfaces that matter (the panel row, doctor,
// the theme log component) read Reason and Detail directly rather than parsing
// this back apart.
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
