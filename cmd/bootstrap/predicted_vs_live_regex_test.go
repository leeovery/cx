package bootstrap_test

import (
	"regexp"
	"testing"
)

// Matches the misleading `predicted=… live=…` WARN. Both segments are required
// so the preserved "live pane count != saved count" warning is not caught.
var predictedVsLiveWarnRegex = regexp.MustCompile(`predicted=.*__\d+\.\d+ live=.*__\d+\.\d+`)

func TestPredictedVsLiveRegex_MatchesOffendingShapeAndIgnoresArmPanesWarning(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantMatch bool
	}{
		{
			name:      "offending predicted-vs-live shape",
			line:      `WARN | restore | session "alpha": pane 0 predicted=alpha__0.0 live=alpha__1.1`,
			wantMatch: true,
		},
		{
			name:      "preserved armPanes:202 pane-count mismatch warning",
			line:      `WARN | restore | session "alpha": live pane count 2 != saved count 3`,
			wantMatch: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := predictedVsLiveWarnRegex.MatchString(tc.line)
			if got != tc.wantMatch {
				t.Fatalf("regex.MatchString(%q) = %v; want %v", tc.line, got, tc.wantMatch)
			}
		})
	}
}
