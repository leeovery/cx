package theme

import (
	"strings"
	"testing"
)

// TestParseThemeBytes_BrokenInputIsARejectionNotAPanic pins the shared parse
// path's failure mode: an ordinary *Rejection, for each of the three rungs that
// read a file's CONTENTS.
//
// This is the guarantee §7.6 rests on. Embedding built-ins moves their parse
// failures from compile time to load time, and the answer to that is a
// build-time test (task 2.8) plus a loader that reports a broken embedded file
// the same way it reports a broken drop-in — the escalation happens where a
// fallback is NEEDED, not here. A panic on the built-in path would turn "your
// theme is invalid" into a Go stack trace on someone's cold start.
//
// The inputs are the SHIPPED file's bytes, deliberately corrupted one way each,
// so the fixtures are embedded-shaped rather than synthetic minimal cases. The
// no-panic half needs no assertion: a panic fails the test where it happens,
// with the stack a recover() would have swallowed.
//
// Only the reason class is asserted. The exact §14A detail text of each rung is
// pinned by the file-level ladder tests, and re-asserting it here would tie this
// test to the shipped file's line layout for nothing.
func TestParseThemeBytes_BrokenInputIsARejectionNotAPanic(t *testing.T) {
	source, found := BuiltinBytes("tokyo-night")
	if !found {
		t.Fatalf("BuiltinBytes(%q) reported not found — the fixtures have nothing to corrupt", "tokyo-night")
	}

	const canvasLine = "canvas = #0b0c14"
	if !strings.Contains(string(source), canvasLine) {
		t.Fatalf("the shipped built-in no longer declares %q — the corruptions below would be no-ops", canvasLine)
	}

	tests := []struct {
		name       string
		corruption string
		wantReason Reason
	}{
		{name: "a line that is not a pair", corruption: "canvas", wantReason: ReasonBadSyntax},
		{name: "a value that is not a hex", corruption: "canvas = blue", wantReason: ReasonBadColour},
		{name: "a token no line declares", corruption: "", wantReason: ReasonMissingTokens},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broken := strings.Replace(string(source), canvasLine, tt.corruption, 1)

			built, rejection := parseThemeBytes([]byte(broken))

			if rejection == nil {
				t.Fatalf("parseThemeBytes() accepted the broken bytes as %+v, want the rejection %q", built, tt.wantReason)
			}
			if rejection.Reason != tt.wantReason {
				t.Errorf("rejection reason = %q, want %q", rejection.Reason, tt.wantReason)
			}
			if rejection.Detail == "" {
				t.Errorf("rejection %q carries no detail, want one naming what is wrong", rejection.Reason)
			}
			if built != (Theme{}) {
				t.Errorf("parseThemeBytes() returned %+v alongside a rejection, want the zero Theme", built)
			}
		})
	}
}
