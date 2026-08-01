package theme

import (
	"fmt"
	"strings"
	"unicode"
)

// utf8BOM is the UTF-8 byte-order mark. It is stripped from the first bytes of a
// theme file and nowhere else (§4.2).
const utf8BOM = "\uFEFF"

// The §14A detail phrases a `bad syntax` rejection is built from. badSyntax
// renders one behind "line N: ", producing the exact string doctor prints
// verbatim — `line 4: quoted value`, `line 7: not a key = value pair`,
// `line 12: duplicate key text.primary`.
const (
	detailNotAPair     = "not a key = value pair"
	detailQuotedValue  = "quoted value"
	detailDuplicateKey = "duplicate key %s"
)

// Pair is one `key = value` line a theme file declared, with the 1-based line it
// was written on.
//
// A Pair is lexical only: the key is whatever the file wrote, not necessarily
// one of the 19 token names, and the value is an untouched string that has not
// been near a colour parser. Line is retained because it is the only carrier of
// WHICH line is wrong in a later §14A detail.
type Pair struct {
	Key, Value string
	Line       int
}

// lexPairs turns theme-file bytes into the pairs the file declares, in file
// order, or into exactly one `bad syntax` rejection (§4.2).
//
// It is purely lexical. It never asks whether a key is one of the 19, never
// validates a value, and never compares two values — which is what §6.2's ladder
// requires, since a lexical failure aborts the parse before any value-level or
// presence check runs. It is also why the duplicate-key check here is
// unconditional rather than scoped to known keys or to differing values: making
// it conditional would add branches to buy nothing.
//
// A file that declares nothing — empty, blank, or comments only — is NOT a
// lexical failure. It lexes to zero pairs and fails later as `missing tokens`:
// it parsed; it declares nothing.
//
// On rejection the pairs are nil. A caller never sees a partial file alongside
// an error.
func lexPairs(data []byte) ([]Pair, *Rejection) {
	pairs := []Pair{}
	seen := map[string]struct{}{}

	// Splitting the BOM-stripped text on "\n" makes the slice index the 1-based
	// line number of the original file, comments and blanks included.
	for index, raw := range strings.Split(strings.TrimPrefix(string(data), utf8BOM), "\n") {
		line := index + 1

		text := trimLine(raw)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		pair, rejection := lexLine(text, line)
		if rejection != nil {
			return nil, rejection
		}
		if _, duplicate := seen[pair.Key]; duplicate {
			return nil, badSyntax(line, fmt.Sprintf(detailDuplicateKey, pair.Key))
		}

		seen[pair.Key] = struct{}{}
		pairs = append(pairs, pair)
	}

	return pairs, nil
}

// lexLine turns one trimmed, non-blank, non-comment line into its pair, or
// rejects it.
//
// The BOM check lands here rather than at the top of the loop so it covers only
// the lines the lexer actually interprets: a BOM that survived the file-start
// strip is invisible garbage that would otherwise corrupt a key into an unknown
// one — reported as `missing tokens` — or a value into a `bad colour`, both of
// which name the wrong thing.
func lexLine(text string, line int) (Pair, *Rejection) {
	if strings.Contains(text, utf8BOM) {
		return Pair{}, badSyntax(line, detailNotAPair)
	}

	rawKey, rawValue, separated := strings.Cut(text, "=")
	if !separated {
		return Pair{}, badSyntax(line, detailNotAPair)
	}

	// Cut splits on the FIRST '=' only, so everything after it is the value
	// verbatim: a '#', a second '=' or interior spaces are all part of it. The
	// format never re-interprets anything right of the separator.
	key, value := strings.TrimSpace(rawKey), strings.TrimSpace(rawValue)
	if !wellFormedKey(key) {
		return Pair{}, badSyntax(line, detailNotAPair)
	}
	if startsQuoted(value) {
		return Pair{}, badSyntax(line, detailQuotedValue)
	}

	return Pair{Key: key, Value: value, Line: line}, nil
}

// trimLine drops one trailing carriage return, so a CRLF file lexes identically
// to its LF twin, then trims both ends — before anything is classified, so
// indentation ahead of a key gets the same tolerance the comment rule already
// grants '#' (§4.2).
func trimLine(raw string) string {
	return strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
}

// wellFormedKey reports whether key satisfies §4.2's rule: non-empty, no
// whitespace, no '='.
//
// Without the whitespace half, `text primary = …` would be a well-formed pair
// with an unknown key, which is IGNORED — and the file would then fail as
// `missing tokens`, a reason pointing at the wrong thing for what is plainly a
// typo in a key that is otherwise right.
//
// The '=' half has no case reachable through lexPairs, because the left part of
// a first-'=' split cannot contain one. It is stated anyway so a well-formed key
// is defined here in full rather than half-delegated to the splitter.
func wellFormedKey(key string) bool {
	if key == "" || strings.Contains(key, "=") {
		return false
	}
	return !strings.ContainsFunc(key, unicode.IsSpace)
}

// startsQuoted reports whether the value opens with a quote.
//
// §4.2 defines "quoted" by the FIRST character alone — matched or not, either
// quote — so `"#FFFFFF"`, `'#FFFFFF'` and `"#FFFFFF` are alike. Defining it by a
// matched outer pair would send the unmatched case on down the ladder to
// `bad colour`, telling the user their colour is wrong when their quoting is.
func startsQuoted(value string) bool {
	return strings.HasPrefix(value, `"`) || strings.HasPrefix(value, "'")
}

// badSyntax builds the one rejection a lexical failure produces, rendering
// §14A's "line N: <phrase>" detail; Line carries the same number in
// machine-readable form.
//
// Every lexical failure routes through here, so a second detail shape cannot
// grow: §6.2 gives a rejected theme exactly one reason, and §14A gives
// `bad syntax` exactly one detail format.
func badSyntax(line int, phrase string) *Rejection {
	return &Rejection{
		Reason: ReasonBadSyntax,
		Detail: fmt.Sprintf("line %d: %s", line, phrase),
		Line:   line,
	}
}
