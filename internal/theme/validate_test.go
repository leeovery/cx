package theme

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestValidate_AcceptsNineteenWellFormedTokens pins the shape of a file that
// validates: every one of the 19 keys present, every value a well-formed hex,
// and the result a Theme carrying them.
//
// The two assertions are complementary. All() reports the canonical table's
// names whatever the fields hold, so it pins the ordered result but is blind to
// a Theme populated with values alone; storedTokens reads the fields directly
// and is what pins that each field carries its own name.
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

// TestValidate_RejectsMalformedHexForms covers the hex-only value rule's value domain: exactly
// '#' and six hex digits, and nothing else.
//
// The last four forms are the ones that make Portal own a validator at all —
// lipgloss.Color never returns an error and accepts every one of them silently:
// `blue` is a named colour, `212` an ANSI-256 index, `-5` is abs'd to 5, and
// `16777215` is reinterpreted as packed RGB. Without this check a typo'd value
// becomes an invisible wrong colour rather than one honest message.
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

// TestValidate_CanonicalisesHexToUppercase pins the hex-only value rule's canonical form. Two
// hex comparison sites depend on it — the startup canvas hex retained for the
// exit-time restore and background diffing — so a file written
// `#c0caf5` must not fail to match one written `#C0CAF5`.
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

// TestValidate_IgnoresUnknownKeyAndItsValue pins the unknown-key rule's forward-compatibility
// lever: an unknown key is ignored ENTIRELY, key and value both. Were a removed
// token's stale line able to reject a file on its value, "old files keep
// working" would hold only for values that happen to still be well-formed hex —
// a far weaker guarantee than the one the file-contents rule states — so the value here is one
// no hex validator would accept.
func TestValidate_IgnoresUnknownKeyAndItsValue(t *testing.T) {
	pairs := append(wellFormedPairs(), Pair{Key: "legacy.thing", Value: "nonsense", Line: 20})

	built, rejection := themeFromPairs(pairs)

	requireAccepted(t, rejection)

	if got, want := built.All(), wantTokens(); !slices.Equal(got, want) {
		t.Errorf("themeFromPairs().All() = %+v, want %+v", got, want)
	}
}

// TestValidate_WrongCaseKeyFailsAsMissingTokens pins the reason vocabulary's consequence of
// case-sensitive matching: `Text.Primary` is an unknown key, so it is ignored
// and the file fails on the token it did not declare. That reason is technically
// accurate but capable of misdirecting, which is precisely why the detail names
// the missing token — it is what makes the mistake findable.
func TestValidate_WrongCaseKeyFailsAsMissingTokens(t *testing.T) {
	pairs := rekeyed(wellFormedPairs(), "text.primary", "Text.Primary")

	built, rejection := themeFromPairs(pairs)

	requireRejection(t, built, rejection, ReasonMissingTokens, "missing text.primary")
}

// TestValidate_EmptyFileMissesAllNineteen pins the verdict on a file that
// declares nothing. It is not a lexical failure — it parsed; it simply declares
// nothing — so the presence check is what fails it, naming all 19.
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

// TestValidate_BadColourDetailEnumeratesEveryOffendingPair pins doctor's
// "enumerate within the reason" contract on the value stage: a file with three
// typos is one message naming all three, not three runs of the loader.
//
// The pairs are declared in REVERSE table order, so the detail's ordering is
// provably the file's rather than the canonical table's, and the unknown key
// carries a malformed value of its own to pin that it reaches no detail.
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

// TestValidate_MissingTokensDetailEnumeratesEveryAbsentName is the presence
// stage's half of the same contract, and pins the order the names come back in.
// An absent token has no file position, so the ordering can only come from the
// canonical table — canonical table order, not alphabetical, which for this pair would
// invert them.
//
// The two names are the pinned copy's own example, so the detail is the exact string the
// spec pins for it.
func TestValidate_MissingTokensDetailEnumeratesEveryAbsentName(t *testing.T) {
	declared := reversed(without(wellFormedPairs(), "text.primary", "bg.subtle"))

	built, rejection := themeFromPairs(declared)

	requireRejection(t, built, rejection, ReasonMissingTokens, "missing text.primary, bg.subtle")
}

// TestValidate_BadColourPrecedesMissingTokens pins the reason vocabulary's ladder across the
// two rungs this file owns: the presence check runs LAST, on a file whose every
// known value is well-formed, so a file that is both bad-coloured and short of a
// token is `bad colour` alone.
//
// The detail is asserted whole, which is what pins the invariant that matters —
// it enumerates within the reason and carries no presence information at all.
func TestValidate_BadColourPrecedesMissingTokens(t *testing.T) {
	declared := without(valued(wellFormedPairs(), "canvas", "blue"), "text.primary")

	built, rejection := themeFromPairs(declared)

	requireRejection(t, built, rejection, ReasonBadColour, "canvas = blue")
}

// wellFormedPairs returns one pair per canonical table token, in table order, each carrying
// a distinct well-formed value — the lexer output of a file that validates.
func wellFormedPairs() []Pair {
	return pairsFrom(hexForRow)
}

// pairsFrom returns one pair per canonical table token, in table order, valued by row.
//
// The names come from TokenNames() rather than a restatement: theme_test.go
// holds the ONE deliberate restatement of the vocabulary, and this file
// exercises the validator rather than the vocabulary.
func pairsFrom(value func(row int) string) []Pair {
	names := TokenNames()
	pairs := make([]Pair, 0, len(names))
	for i, name := range names {
		pairs = append(pairs, Pair{Key: name, Value: value(i + 1), Line: i + 1})
	}
	return pairs
}

// wantTokens renders the tokens wellFormedPairs() should produce, in canonical table order.
func wantTokens() []Token {
	names := TokenNames()
	tokens := make([]Token, 0, len(names))
	for i, name := range names {
		tokens = append(tokens, Token{Name: name, Value: hexForRow(i + 1)})
	}
	return tokens
}

// hexForRow renders a distinct well-formed value for the given canonical table row.
func hexForRow(row int) string {
	return fmt.Sprintf("#0000%02d", row)
}

// lowercaseHexForRow renders a distinct value carrying lower-case letters, so
// canonicalisation has something to bite on — hexForRow's values are all digits
// and read the same in either case.
func lowercaseHexForRow(row int) string {
	return fmt.Sprintf("#abcd%02x", row)
}

// tokenMap keys tokens by name, the form storedTokens is compared against.
func tokenMap(tokens []Token) map[string]string {
	byName := make(map[string]string, len(tokens))
	for _, tok := range tokens {
		byName[tok.Name] = tok.Value
	}
	return byName
}

// storedTokens returns what the Theme's struct FIELDS carry, keyed by the name
// each field itself holds.
//
// Reading the fields rather than All() is the point: All() derives every name
// from the canonical table, so a Theme whose fields were populated with values
// alone looks correct through it and collapses here into a single "" key.
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

// valued returns pairs with the named key's value replaced, in the same file
// order.
func valued(pairs []Pair, key, value string) []Pair {
	replaced := slices.Clone(pairs)
	for i := range replaced {
		if replaced[i].Key == key {
			replaced[i].Value = value
		}
	}
	return replaced
}

// without returns pairs with the named keys removed — a file that never declared
// them.
func without(pairs []Pair, keys ...string) []Pair {
	return slices.DeleteFunc(slices.Clone(pairs), func(pair Pair) bool {
		return slices.Contains(keys, pair.Key)
	})
}

// reversed returns pairs in the opposite file order, which for a set built from
// the canonical table is an order no canonical-table-driven enumeration could produce.
func reversed(pairs []Pair) []Pair {
	backwards := slices.Clone(pairs)
	slices.Reverse(backwards)
	return backwards
}

// rekeyed returns pairs with one key rewritten, in the same file order.
func rekeyed(pairs []Pair, from, to string) []Pair {
	renamed := slices.Clone(pairs)
	for i := range renamed {
		if renamed[i].Key == from {
			renamed[i].Key = to
		}
	}
	return renamed
}

// uppercased returns pairs with every value upper-cased — the same file written
// in the other case.
func uppercased(pairs []Pair) []Pair {
	twin := slices.Clone(pairs)
	for i := range twin {
		twin[i].Value = strings.ToUpper(twin[i].Value)
	}
	return twin
}

// requireAccepted fails the test unless the validator accepted the pairs.
func requireAccepted(t *testing.T, rejection *Rejection) {
	t.Helper()

	if rejection != nil {
		t.Fatalf("themeFromPairs() rejected the pairs: %v", rejection)
	}
}

// requireRejection fails the test unless the validator rejected the pairs with
// the one expected reason and the complete pinned-copy detail, which every caller
// states literally rather than re-deriving from the implementation's format.
//
// It also pins the two invariants every rejection from here shares: no partial
// Theme comes back alongside one, and Line is 0 because `bad syntax` is the only
// reason that carries a line.
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
