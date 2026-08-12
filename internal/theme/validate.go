package theme

import (
	"fmt"
	"strings"
)

// The format has no `#RGB` shorthand and no 8-digit `#RRGGBBAA` alpha form.
const hexValueLength = len("#RRGGBB")

// Rendered copy: a consumer needing the tokens themselves reads
// Rejection.Tokens.
const (
	detailBadColourPair = "%s = %s"
	detailMissingTokens = "missing %s"
)

// Unknown keys are ignored entirely, key and value, so the stale line of a
// removed token cannot reject an otherwise-good file. Validity is syntactic
// only — no contrast or readability judgement.
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

// Offenders are collected and reported together, so a file with several typos
// is one message. The stored value is upper-cased because downstream hex
// comparisons (the retained startup canvas hex, background diffing) depend on
// it; the offending value is deliberately not canonicalised, echoing back what
// the user wrote.
func applyPairs(refs []fieldRef, pairs []Pair) *Rejection {
	index := indexByName(refs)

	names, values := []string{}, []string{}
	for _, pair := range pairs {
		ref, known := index[pair.Key]
		if !known {
			continue
		}
		if !wellFormedHex(pair.Value) {
			names = append(names, pair.Key)
			values = append(values, pair.Value)
			continue
		}
		*ref.Field = Token{Name: ref.Name, Value: strings.ToUpper(pair.Value)}
	}

	if len(names) == 0 {
		return nil
	}
	return &Rejection{
		Reason: ReasonBadColour,
		Detail: strings.Join(renderedPairs(names, values), ", "),
		Tokens: names,
		Values: values,
	}
}

// A theme file must declare the whole palette — no partial file, no
// merge-over-a-base: the canvas is itself a token, so a partial theme would put
// a foreground against a background it was never tuned for. The absent names are
// the whole story, so the rejection carries no values.
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

// renderedPairs composes the "<name> = <value>" copy from the structured name
// and value lists. A name with no matching value renders bare, so a
// hand-constructed Rejection with unpaired lists degrades to names rather than
// panicking.
func renderedPairs(names, values []string) []string {
	rendered := make([]string, 0, len(names))
	for i, name := range names {
		if i >= len(values) {
			rendered = append(rendered, name)
			continue
		}
		rendered = append(rendered, fmt.Sprintf(detailBadColourPair, name, values[i]))
	}
	return rendered
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

// Portal owns this check because lipgloss.Color never errors and accepts a far
// wider domain — ANSI-256 indices, packed RGB — with every failure resolving to
// the silent noColor sentinel.
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
