package theme

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestValidate_AcceptsNineteenWellFormedTokens(t *testing.T) {
	built, rejection := themeFromPairs(wellFormedPairs())

	requireAccepted(t, rejection)

	want := wantTokens()
	if got := built.All(); !slices.Equal(got, want) {
		t.Errorf("themeFromPairs().All() = %+v, want %+v", got, want)
	}
	if got, wantByName := storedTokens(t, built), tokenMap(want); !maps.Equal(got, wantByName) {
		t.Errorf("stored fields = %v, want %v", got, wantByName)
	}
}

func TestValidate_RejectsMalformedHexForms(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "three-digit shorthand", value: "#FFF"},
		{name: "eight digits with alpha", value: "#FFFFFFFF"},
		{name: "non-hex digits", value: "#GGGGGG"},
		{name: "empty value", value: ""},
		{name: "interior whitespace", value: "#FF FFFF"},
		{name: "named colour", value: "blue"},
		{name: "ansi-256 index", value: "212"},
		{name: "negative number", value: "-5"},
		{name: "packed rgb", value: "16777215"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs := valued(wellFormedPairs(), "text.primary", tt.value)

			built, rejection := themeFromPairs(pairs)

			requireRejection(t, built, rejection, ReasonBadColour, "text.primary = "+tt.value)
		})
	}
}

func TestValidate_CanonicalisesHexToUppercase(t *testing.T) {
	lower := valued(pairsFrom(lowercaseHexForRow), "text.primary", "#c0caf5")
	upper := uppercased(lower)

	fromLower, rejection := themeFromPairs(lower)
	requireAccepted(t, rejection)

	fromUpper, twinRejection := themeFromPairs(upper)
	requireAccepted(t, twinRejection)

	if got, want := fromLower.TextPrimary, (Token{Name: "text.primary", Value: "#C0CAF5"}); got != want {
		t.Errorf("text.primary = %+v, want %+v", got, want)
	}
	if fromLower != fromUpper {
		t.Errorf("a lower-case file produced %+v and its upper-case twin %+v, want identical themes", fromLower, fromUpper)
	}
}

func TestValidate_IgnoresUnknownKeyAndItsValue(t *testing.T) {
	pairs := append(wellFormedPairs(), Pair{Key: "legacy.thing", Value: "nonsense", Line: 20})

	built, rejection := themeFromPairs(pairs)

	requireAccepted(t, rejection)

	if got, want := built.All(), wantTokens(); !slices.Equal(got, want) {
		t.Errorf("themeFromPairs().All() = %+v, want %+v", got, want)
	}
}

func TestValidate_WrongCaseKeyFailsAsMissingTokens(t *testing.T) {
	pairs := rekeyed(wellFormedPairs(), "text.primary", "Text.Primary")

	built, rejection := themeFromPairs(pairs)

	requireRejection(t, built, rejection, ReasonMissingTokens, "missing text.primary")
}

func TestValidate_EmptyFileMissesAllNineteen(t *testing.T) {
	tests := []struct {
		name  string
		pairs []Pair
	}{
		{name: "no pairs at all", pairs: nil},
		{name: "an empty slice", pairs: []Pair{}},
	}

	wantDetail := "missing " + strings.Join(TokenNames(), ", ")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			built, rejection := themeFromPairs(tt.pairs)

			requireRejection(t, built, rejection, ReasonMissingTokens, wantDetail)
		})
	}
}

func TestValidate_BadColourDetailEnumeratesEveryOffendingPair(t *testing.T) {
	offending := []Pair{
		{Key: "text.primary", Value: "#GGGGGG"},
		{Key: "canvas", Value: "blue"},
		{Key: "bg.subtle", Value: "#FFF"},
	}

	declared := wellFormedPairs()
	for _, bad := range offending {
		declared = valued(declared, bad.Key, bad.Value)
	}
	declared = append([]Pair{{Key: "legacy.thing", Value: "nonsense"}}, reversed(declared)...)

	built, rejection := themeFromPairs(declared)

	requireRejection(t, built, rejection, ReasonBadColour, "bg.subtle = #FFF, canvas = blue, text.primary = #GGGGGG")
}

func TestValidate_MissingTokensDetailEnumeratesEveryAbsentName(t *testing.T) {
	declared := reversed(without(wellFormedPairs(), "text.primary", "bg.subtle"))

	built, rejection := themeFromPairs(declared)

	requireRejection(t, built, rejection, ReasonMissingTokens, "missing text.primary, bg.subtle")
}

func TestValidate_MissingTokensCarriesTheAbsentNamesAsData(t *testing.T) {
	declared := reversed(without(wellFormedPairs(), "text.primary", "bg.subtle"))

	built, rejection := themeFromPairs(declared)

	requireRejection(t, built, rejection, ReasonMissingTokens, "missing text.primary, bg.subtle")
	if got, want := rejection.Tokens, []string{"text.primary", "bg.subtle"}; !slices.Equal(got, want) {
		t.Errorf("rejection tokens = %v, want %v", got, want)
	}
	if len(rejection.Values) != 0 {
		t.Errorf("rejection values = %v, want none — a missing token has no offending value", rejection.Values)
	}
	if got, want := rejection.Detail, "missing "+strings.Join(rejection.Tokens, ", "); got != want {
		t.Errorf("rejection detail = %q, want %q — the detail is the token list rendered", got, want)
	}
}

func TestValidate_BadColourCarriesTheOffendingNamesAndValuesAsData(t *testing.T) {
	declared := valued(valued(wellFormedPairs(), "canvas", "blue"), "text.primary", "#gGgGgG")

	built, rejection := themeFromPairs(declared)

	requireRejection(t, built, rejection, ReasonBadColour, "text.primary = #gGgGgG, canvas = blue")
	if got, want := rejection.Tokens, []string{"text.primary", "canvas"}; !slices.Equal(got, want) {
		t.Errorf("rejection tokens = %v, want %v", got, want)
	}
	if got, want := rejection.Values, []string{"#gGgGgG", "blue"}; !slices.Equal(got, want) {
		t.Errorf("rejection values = %v, want %v — echoed back as the user wrote them", got, want)
	}
}

func TestValidate_BadColourPrecedesMissingTokens(t *testing.T) {
	declared := without(valued(wellFormedPairs(), "canvas", "blue"), "text.primary")

	built, rejection := themeFromPairs(declared)

	requireRejection(t, built, rejection, ReasonBadColour, "canvas = blue")
}

func wellFormedPairs() []Pair {
	return pairsFrom(hexForRow)
}

func pairsFrom(value func(row int) string) []Pair {
	names := TokenNames()
	pairs := make([]Pair, 0, len(names))
	for i, name := range names {
		pairs = append(pairs, Pair{Key: name, Value: value(i + 1), Line: i + 1})
	}
	return pairs
}

func wantTokens() []Token {
	names := TokenNames()
	tokens := make([]Token, 0, len(names))
	for i, name := range names {
		tokens = append(tokens, Token{Name: name, Value: hexForRow(i + 1)})
	}
	return tokens
}

func hexForRow(row int) string {
	return fmt.Sprintf("#0000%02d", row)
}

func lowercaseHexForRow(row int) string {
	return fmt.Sprintf("#abcd%02x", row)
}

func tokenMap(tokens []Token) map[string]string {
	byName := make(map[string]string, len(tokens))
	for _, tok := range tokens {
		byName[tok.Name] = tok.Value
	}
	return byName
}

func storedTokens(t *testing.T, built Theme) map[string]string {
	t.Helper()

	fields := reflect.ValueOf(built)
	stored := make(map[string]string, fields.NumField())
	for i := range fields.NumField() {
		tok, ok := fields.Field(i).Interface().(Token)
		if !ok {
			t.Fatalf("Theme field %d is %s, want a Token", i, fields.Field(i).Type())
		}
		stored[tok.Name] = tok.Value
	}
	return stored
}

func valued(pairs []Pair, key, value string) []Pair {
	replaced := slices.Clone(pairs)
	for i := range replaced {
		if replaced[i].Key == key {
			replaced[i].Value = value
		}
	}
	return replaced
}

func without(pairs []Pair, keys ...string) []Pair {
	return slices.DeleteFunc(slices.Clone(pairs), func(pair Pair) bool {
		return slices.Contains(keys, pair.Key)
	})
}

func reversed(pairs []Pair) []Pair {
	backwards := slices.Clone(pairs)
	slices.Reverse(backwards)
	return backwards
}

func rekeyed(pairs []Pair, from, to string) []Pair {
	renamed := slices.Clone(pairs)
	for i := range renamed {
		if renamed[i].Key == from {
			renamed[i].Key = to
		}
	}
	return renamed
}

func uppercased(pairs []Pair) []Pair {
	twin := slices.Clone(pairs)
	for i := range twin {
		twin[i].Value = strings.ToUpper(twin[i].Value)
	}
	return twin
}

func requireAccepted(t *testing.T, rejection *Rejection) {
	t.Helper()

	if rejection != nil {
		t.Fatalf("themeFromPairs() rejected the pairs: %v", rejection)
	}
}

func requireRejection(t *testing.T, built Theme, rejection *Rejection, wantReason Reason, wantDetail string) {
	t.Helper()

	if rejection == nil {
		t.Fatalf("themeFromPairs() accepted the pairs, want %q: %s", wantReason, wantDetail)
	}
	if built != (Theme{}) {
		t.Errorf("themeFromPairs() returned %+v alongside a rejection, want the zero Theme", built)
	}
	if rejection.Reason != wantReason {
		t.Errorf("rejection reason = %q, want %q", rejection.Reason, wantReason)
	}
	if rejection.Detail != wantDetail {
		t.Errorf("rejection detail = %q, want %q", rejection.Detail, wantDetail)
	}
	if rejection.Line != 0 {
		t.Errorf("rejection line = %d, want 0 — only bad syntax carries a line", rejection.Line)
	}
}
