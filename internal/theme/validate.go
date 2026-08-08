package theme

import (
	"fmt"
	"strings"
)

// hexValueLength is the length of a well-formed value — `#RRGGBB`, a '#' and
// exactly six digits. There is no `#RGB` shorthand and no 8-digit `#RRGGBBAA`
// alpha form.
const hexValueLength = len("#RRGGBB")

// The detail phrases the two stages are built from, in ladder order.
// detailBadColourPair renders ONE offending pair; the pairs are joined with ", "
// into `text.primary = #GGGGGG, canvas = blue`. detailMissingTokens takes the
// whole joined list, producing `missing text.primary, bg.subtle`.
//
// The lead-in is a constant of its own, and detailMissingTokens is composed from
// it, because the `theme` log component's `token` attr carries the joined LIST
// without it (see tokenAttr in events.go). Composing the two rather than writing
// the word twice is what keeps the render and the one place that reads it back
// off from ever disagreeing.
const (
	detailBadColourPair       = "%s = %s"
	detailMissingTokensLeadIn = "missing "
	detailMissingTokens       = detailMissingTokensLeadIn + "%s"
)

// themeFromPairs turns the pairs a file declared into the Theme it describes, or
// into exactly one `bad colour` or `missing tokens` rejection.
//
// The two stages are the rejection ladder's last two rungs and run in that
// order: every known key's value is validated first, and the presence check runs
// only on a file whose every known value is well-formed. A file that is both
// bad-coloured and short of a token is `bad colour`, and its detail says nothing
// whatsoever about presence — detail enumerates WITHIN the reason, never across
// two.
//
// Unknown keys are ignored entirely, key AND value. Forward compatibility
// requires it: if a removed token's stale line could reject a file on its value,
// "old files keep working" would hold only for values that happen to still be
// well-formed hex. It is also what makes a wrong-case key — matching is
// case-sensitive — fail as `missing tokens` rather than on its value.
//
// Validity here is syntactic, never perceptual. Nothing is asked about whether
// the colours are good, readable or clear a contrast floor; floors are the
// bundled tier's concern.
//
// On rejection the Theme is the zero value. A caller never sees a partly
// populated palette alongside an error.
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

// applyPairs writes every well-formed known pair through to its field, or
// returns the one `bad colour` rejection enumerating every known pair whose
// value is not a `#RRGGBB` hex.
//
// Offenders are collected in file order and reported together: a file with three
// typos is one message naming all three, not three runs of the loader. Each
// field is populated with the WHOLE token — the name from the canonical table
// rather than only the value — so a parsed theme's fields agree with the names
// All() reports for them.
//
// The stored value is upper-cased into the canonical form: the hex comparisons
// downstream — the startup canvas hex retained for the exit-time terminal
// background restore, and background diffing — depend on it, so a file written
// `#c0caf5` must not fail to match one written `#C0CAF5`. The offender detail is
// deliberately NOT canonicalised: it echoes back what the user wrote.
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
	return &Rejection{Reason: ReasonBadColour, Detail: strings.Join(offenders, ", ")}
}

// requireEveryToken returns the one `missing tokens` rejection enumerating every
// token no pair populated, in canonical table order, or nil once all 19 are
// declared.
//
// A theme file must declare the whole palette: there is no partial file and no
// merge-over-a-base. The hazard that closes is that the canvas is ITSELF a
// token — a partial theme supplying a new canvas while inheriting text.primary
// would produce a foreground measured against a background it was never tuned
// for.
//
// A field is populated iff its value is non-empty, because a stored value is
// always a well-formed hex — applyPairs writes nothing else — and this runs only
// once every known value has passed that check.
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
	}
}

// indexByName keys the canonical table's rows by token name, so classifying a
// pair is one lookup against the same table All() and TokenNames() derive from.
//
// The lookup is case-sensitive: `Text.Primary` is an unknown key, not
// text.primary written loudly.
func indexByName(refs []fieldRef) map[string]fieldRef {
	index := make(map[string]fieldRef, len(refs))
	for _, ref := range refs {
		index[ref.Name] = ref
	}
	return index
}

// wellFormedHex reports whether value is exactly '#' followed by six hex digits,
// in either case.
//
// Portal owns this check because lipgloss.Color never returns an error and its
// accepted domain is wider and stranger than a theme format wants: `212` is a
// valid ANSI-256 index, `-5` is silently abs'd, `16777215` is reinterpreted as
// packed RGB, and every failure is the silent noColor sentinel. The length gate
// also settles the shapes a digit scan alone would miss — an empty value, and
// interior whitespace such as `#FF FFFF`.
//
// A multi-byte rune cannot slip through the byte-wise scan: it fails the digit
// test on its first byte, having already had to fit the length gate.
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

// isHexDigit reports whether c is one of 0-9, a-f or A-F.
func isHexDigit(c byte) bool {
	digit := c >= '0' && c <= '9'
	lower := c >= 'a' && c <= 'f'
	upper := c >= 'A' && c <= 'F'
	return digit || lower || upper
}
