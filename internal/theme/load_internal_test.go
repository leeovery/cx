package theme

import (
	"strings"
	"testing"
)

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
