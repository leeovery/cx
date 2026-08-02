package theme

import (
	"strings"
	"testing"
)

// TestEmbeddedParseFailureIsAnOrdinaryError pins the shared parse path's failure
// mode: an ordinary *Rejection, for each of the three rungs that read a file's
// CONTENTS.
//
// This is the mechanism §7.6's build-time guarantee rests on. Making the
// built-ins files moved their parse failures from compile time to load time,
// and the answer is a build-time test (embedded_test.go) plus a loader that
// reports a broken embedded file exactly as it reports a broken drop-in. The
// escalation happens where a fallback is NEEDED — Phase 5 — so the user sees
// one line rather than a Go stack trace; main.go's panic-recovering exit stays
// the backstop for a genuine programming fault, not the designed route.
//
// The fixtures are the SHIPPED file's bytes, corrupted one way each, so they
// are embedded-shaped rather than synthetic minimal cases — this is the parse
// of a real built-in going wrong, which is the situation being reasoned about.
// The source is reached through DefaultDarkSlug and corrupted at whichever
// token line it happens to declare first, so nothing here is pinned to one
// palette's contents.
//
// The no-panic half needs no assertion: a panic fails the test where it
// happens, with the stack a recover() would have swallowed.
//
// Only the reason class and the presence of a detail are asserted. The exact
// §14A detail text of each rung is pinned by the ladder tests in load_test.go
// and lex_test.go, and re-asserting it here would tie this test to the shipped
// file's line layout for nothing.
func TestEmbeddedParseFailureIsAnOrdinaryError(t *testing.T) {
	source, found := BuiltinBytes(DefaultDarkSlug)
	if !found {
		t.Fatalf("BuiltinBytes(DefaultDarkSlug) reported not found — the fixtures have nothing to corrupt")
	}

	text := string(source)
	line, key := firstTokenLine(t, text)

	tests := []struct {
		name       string
		corrupt    func(string) string
		wantReason Reason
	}{
		{
			name:       "a duplicate-keyed file",
			corrupt:    func(s string) string { return strings.Replace(s, line, line+"\n"+line, 1) },
			wantReason: ReasonBadSyntax,
		},
		{
			name:       "a bad-hex file",
			corrupt:    func(s string) string { return strings.Replace(s, line, key+" = blue", 1) },
			wantReason: ReasonBadColour,
		},
		{
			name:       "a short file",
			corrupt:    func(s string) string { return s[:strings.Index(s, line)] },
			wantReason: ReasonMissingTokens,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			built, rejection := parseThemeBytes([]byte(tt.corrupt(text)))

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

// firstTokenLine returns the first `key = value` line of a theme file, verbatim,
// alongside the key it declares.
//
// It is what keeps the corruptions above independent of which palette
// DefaultDarkSlug names and of which value that palette gives the token: the
// fixtures are built from the file as it is, not from a line restated here that
// an edit to the shipped theme could silently stop matching.
//
// The line is required to occur exactly once, so a strings.Replace against it
// is unambiguous and a truncation at its index cuts where it says.
func firstTokenLine(t *testing.T, text string) (line, key string) {
	t.Helper()

	for raw := range strings.SplitSeq(text, "\n") {
		candidate := strings.TrimSpace(raw)
		if candidate == "" || strings.HasPrefix(candidate, "#") {
			continue
		}

		name, _, separated := strings.Cut(candidate, "=")
		if !separated {
			t.Fatalf("the shipped built-in's first declaration %q is not a key = value pair", candidate)
		}
		if count := strings.Count(text, candidate); count != 1 {
			t.Fatalf("the shipped built-in declares %q %d times, want exactly one so the corruptions below are unambiguous", candidate, count)
		}
		return candidate, strings.TrimSpace(name)
	}

	t.Fatal("the shipped built-in declares no token at all — the corruptions below would be no-ops")
	return "", ""
}
