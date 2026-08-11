package theme

import (
	"fmt"
	"strings"
)

// The format has no `#RGB` shorthand and no 8-digit `#RRGGBBAA` alpha form.
const hexValueLength = len("#RRGGBB")

// User-facing copy only: a consumer needing the tokens themselves reads
// Rejection.Tokens, so editing these moves nothing but what a human reads.
const (
	detailBadColourPair       = "%s = %s"
	detailMissingTokensLeadIn = "missing "
	detailMissingTokens       = detailMissingTokensLeadIn + "%s"
)

// themeFromPairs turns the pairs a file declared into the Theme it describes,
// or into one `bad colour` or `missing tokens` rejection; on rejection the
// Theme is the zero value. Unknown keys are ignored entirely, key and value,
// so a removed token's stale line cannot reject an otherwise-good file.
// Validity is syntactic only — no contrast or readability judgement.
func themeFromPairs(pairs []Pair) (Theme, *Rejection) {
	var built Theme
	refs := built.fields()

	if rejection := applyPairs(refs, pairs); rejection != nil {
		return Theme{}, rejection
	}
	if rejection := requireEveryToken(refs); rejection != nil {
		return Theme{}, rejection
	}

	return built, nil
}

// Offenders are collected and reported together, so a file with three typos
// is one message. The stored value is upper-cased because downstream hex
// comparisons (the retained startup canvas hex, background diffing) depend on
// it; the offender detail is deliberately not canonicalised, echoing back
// what the user wrote.
func applyPairs(refs []fieldRef, pairs []Pair) *Rejection {
	index := indexByName(refs)

	offenders := []string{}
	for _, pair := range pairs {
		ref, known := index[pair.Key]
		if !known {
			continue
		}
		if !wellFormedHex(pair.Value) {
			offenders = append(offenders, fmt.Sprintf(detailBadColourPair, pair.Key, pair.Value))
			continue
		}
		*ref.Field = Token{Name: ref.Name, Value: strings.ToUpper(pair.Value)}
	}

	if len(offenders) == 0 {
		return nil
	}
	return &Rejection{
		Reason: ReasonBadColour,
		Detail: strings.Join(offenders, ", "),
		Tokens: offenders,
	}
}

// A theme file must declare the whole palette — no partial file, no
// merge-over-a-base: the canvas is itself a token, so a partial theme would
// produce a foreground measured against a background it was never tuned for.
func requireEveryToken(refs []fieldRef) *Rejection {
	missing := []string{}
	for _, ref := range refs {
		if ref.Field.Value == "" {
			missing = append(missing, ref.Name)
		}
	}

	if len(missing) == 0 {
		return nil
	}
	return &Rejection{
		Reason: ReasonMissingTokens,
		Detail: fmt.Sprintf(detailMissingTokens, strings.Join(missing, ", ")),
		Tokens: missing,
	}
}

// The lookup is case-sensitive: `Text.Primary` is an unknown key, not
// text.primary written loudly.
func indexByName(refs []fieldRef) map[string]fieldRef {
	index := make(map[string]fieldRef, len(refs))
	for _, ref := range refs {
		index[ref.Name] = ref
	}
	return index
}

// Portal owns this check because lipgloss.Color never errors and accepts a
// far wider domain: `212` is an ANSI-256 index, `-5` is silently abs'd,
// `16777215` is packed RGB, and every failure is the silent noColor
// sentinel.
func wellFormedHex(value string) bool {
	if len(value) != hexValueLength || value[0] != '#' {
		return false
	}

	for i := 1; i < hexValueLength; i++ {
		if !isHexDigit(value[i]) {
			return false
		}
	}
	return true
}

func isHexDigit(c byte) bool {
	digit := c >= '0' && c <= '9'
	lower := c >= 'a' && c <= 'f'
	upper := c >= 'A' && c <= 'F'
	return digit || lower || upper
}
