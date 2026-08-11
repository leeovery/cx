package theme

import (
	"fmt"
	"strings"
	"unicode"
)

const utf8BOM = "\uFEFF"

// User-facing copy: badSyntax renders these behind "line N: ", and doctor
// prints the result verbatim.
const (
	detailNotAPair     = "not a key = value pair"
	detailQuotedValue  = "quoted value"
	detailDuplicateKey = "duplicate key %s"
)

// Pair is one `key = value` line a theme file declared, with the 1-based line
// it was written on. A Pair is lexical only: the key is whatever the file
// wrote and the value is untouched. Line is retained for later rejection
// details.
type Pair struct {
	Key, Value string
	Line       int
}

// lexPairs is purely lexical — no key or value validation. A file that
// declares nothing lexes to zero pairs and fails later as `missing tokens`;
// on rejection the pairs are nil.
func lexPairs(data []byte) ([]Pair, *Rejection) {
	pairs := []Pair{}
	seen := map[string]struct{}{}

	// The slice index maps to the original file's 1-based line number,
	// comments and blanks included.
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

// A BOM surviving the file-start strip is rejected as `bad syntax`: left
// alone it would corrupt a key or value and be reported under a reason naming
// the wrong thing.
func lexLine(text string, line int) (Pair, *Rejection) {
	if strings.Contains(text, utf8BOM) {
		return Pair{}, badSyntax(line, detailNotAPair)
	}

	rawKey, rawValue, separated := strings.Cut(text, "=")
	if !separated {
		return Pair{}, badSyntax(line, detailNotAPair)
	}

	// Cut splits on the first '=' only: everything after it is the value
	// verbatim — a '#', a second '=' or interior spaces are all part of it.
	key, value := strings.TrimSpace(rawKey), strings.TrimSpace(rawValue)
	if !wellFormedKey(key) {
		return Pair{}, badSyntax(line, detailNotAPair)
	}
	if startsQuoted(value) {
		return Pair{}, badSyntax(line, detailQuotedValue)
	}

	return Pair{Key: key, Value: value, Line: line}, nil
}

// A CRLF file must lex identically to its LF twin.
func trimLine(raw string) string {
	return strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
}

// Without the whitespace half, `text primary = …` would lex as an unknown key
// and the file would fail as `missing tokens` — the wrong reason for a plain
// typo.
func wellFormedKey(key string) bool {
	if key == "" || strings.Contains(key, "=") {
		return false
	}
	return !strings.ContainsFunc(key, unicode.IsSpace)
}

// Judged by the first character alone, matched or not: requiring a matched
// pair would send the unmatched case on to `bad colour`, telling the user
// their colour is wrong when their quoting is.
func startsQuoted(value string) bool {
	return strings.HasPrefix(value, `"`) || strings.HasPrefix(value, "'")
}

// Every lexical failure routes through here, so a second detail shape cannot
// grow.
func badSyntax(line int, phrase string) *Rejection {
	return &Rejection{
		Reason: ReasonBadSyntax,
		Detail: fmt.Sprintf("line %d: %s", line, phrase),
		Line:   line,
	}
}
