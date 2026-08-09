package theme

import (
	"slices"
	"strings"
	"testing"
)

// bom is the UTF-8 byte-order mark, restated here as an escape rather than
// reused from production so the tests pin the byte sequence independently — and
// because a literal BOM anywhere but a Go file's first byte is a compile error.
const bom = "\uFEFF"

// TestLex_ParsesKeyValuePairsWithLineNumbers pins the shape of a lexed file:
// pairs in file order, and a 1-based line number counted over the ORIGINAL file
// including the comments and blanks that produce no pair. The line number is the
// only carrier of which line is wrong in the pinned copy's detail, so it has to survive the
// lines the lexer discards.
func TestLex_ParsesKeyValuePairsWithLineNumbers(t *testing.T) {
	file := "# Nord — https://www.nordtheme.com/\n" +
		"\n" +
		"canvas = #2E3440\n" +
		"text.primary = #ECEFF4\n"

	got, rejection := lexPairs([]byte(file))

	requireNoRejection(t, rejection)
	requirePairs(t, got,
		Pair{Key: "canvas", Value: "#2E3440", Line: 3},
		Pair{Key: "text.primary", Value: "#ECEFF4", Line: 4},
	)
}

// TestLex_TrimsLineAndAroundSeparator covers the lexical rules's whitespace rule: the whole
// line is trimmed at both ends BEFORE anything is classified, so indentation
// before a key is fine — the same tolerance the comment rule already grants '#' —
// and then the key and value are trimmed around the '='.
func TestLex_TrimsLineAndAroundSeparator(t *testing.T) {
	file := "  text.primary  =  #ECEFF4  \n" +
		"   # an indented comment is still a comment\n" +
		"\t\taccent.key\t=\t#7AA2F7\t\n"

	got, rejection := lexPairs([]byte(file))

	requireNoRejection(t, rejection)
	requirePairs(t, got,
		Pair{Key: "text.primary", Value: "#ECEFF4", Line: 1},
		Pair{Key: "accent.key", Value: "#7AA2F7", Line: 3},
	)
}

// TestLex_HashStartsCommentOnlyAtLineStart is the forcing case of the lexical rules: '#' is
// both the comment marker and the hex prefix, and every value in a theme file
// starts with one. Comments exist only at line start, so there are no trailing
// comments and a '#' after the '=' is part of the value — here producing a value
// that fails hex validation later, rather than a colour plus a note.
func TestLex_HashStartsCommentOnlyAtLineStart(t *testing.T) {
	file := "text.primary = #ECEFF4 # tuned for the lighter canvas\n" +
		"# a whole-line comment\n"

	got, rejection := lexPairs([]byte(file))

	requireNoRejection(t, rejection)
	requirePairs(t, got, Pair{Key: "text.primary", Value: "#ECEFF4 # tuned for the lighter canvas", Line: 1})
}

// TestLex_ToleratesCRLF asserts a CRLF file lexes identically to its LF twin —
// values carry no stray carriage return, and the line numbering is unchanged.
func TestLex_ToleratesCRLF(t *testing.T) {
	lf := "# a comment\n\ncanvas = #2E3440\ntext.primary = #ECEFF4\n"
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")

	wantPairs, rejection := lexPairs([]byte(lf))
	requireNoRejection(t, rejection)

	got, rejection := lexPairs([]byte(crlf))

	requireNoRejection(t, rejection)
	requirePairs(t, got, wantPairs...)
}

// TestLex_StripsBOMAtFileStartOnly pins the strip as a file-level pre-pass: a
// leading BOM is invisible, so the first line still lexes as a pair on line 1.
// Its counterpart TestLex_InteriorBOMIsBadSyntax covers the other half of the
// rule — the strip applies at the first bytes of the file and nowhere else.
func TestLex_StripsBOMAtFileStartOnly(t *testing.T) {
	file := bom + "canvas = #2E3440\ntext.primary = #ECEFF4\n"

	got, rejection := lexPairs([]byte(file))

	requireNoRejection(t, rejection)
	requirePairs(t, got,
		Pair{Key: "canvas", Value: "#2E3440", Line: 1},
		Pair{Key: "text.primary", Value: "#ECEFF4", Line: 2},
	)
}

// TestLex_BlankAndCommentOnlyFileYieldsNoPairs pins that declaring nothing is
// not a lexical failure. The file parsed; it simply declares nothing, and the
// presence check is what fails it as `missing tokens` — so the lexer must not
// pre-empt that with a syntax verdict.
func TestLex_BlankAndCommentOnlyFileYieldsNoPairs(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "empty file", file: ""},
		{name: "single newline", file: "\n"},
		{name: "blank lines only", file: "\n\n   \n\t\n"},
		{name: "comments only", file: "# Nord\n  # attribution\n"},
		{name: "comments and blanks", file: "# Nord\n\n   # attribution\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rejection := lexPairs([]byte(tt.file))

			requireNoRejection(t, rejection)
			requirePairs(t, got)
		})
	}
}

// TestLex_SplitsOnFirstEqualsOnly pins that the format never re-interprets
// anything right of the first separator: a second '=' is part of the value and
// fails hex validation later, not here.
//
// The same rule is why the lexical rules's "a key contains no '='" clause has no reachable
// case through the lexer — the left part of a first-'=' split cannot contain one,
// so a line the user meant as `text=primary = …` yields the key `text`, which is
// merely unknown. The clause is stated in wellFormedKey regardless, so the
// definition of a well-formed key does not silently depend on the splitter.
func TestLex_SplitsOnFirstEqualsOnly(t *testing.T) {
	tests := []struct {
		name string
		file string
		want Pair
	}{
		{
			name: "second equals lands in the value",
			file: "text.primary = #ECEFF4 = x\n",
			want: Pair{Key: "text.primary", Value: "#ECEFF4 = x", Line: 1},
		},
		{
			name: "an equals the user meant as part of the key ends the key",
			file: "text=primary = #FFFFFF\n",
			want: Pair{Key: "text", Value: "primary = #FFFFFF", Line: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rejection := lexPairs([]byte(tt.file))

			requireNoRejection(t, rejection)
			requirePairs(t, got, tt.want)
		})
	}
}

// TestLex_TrailingWhitespaceIsNotAnError pins that whitespace after a value is
// whitespace around the pair, not inside the value: it is trimmed away. Interior
// whitespace (`#FF FFFF`) is a different matter and fails later as `bad colour`.
func TestLex_TrailingWhitespaceIsNotAnError(t *testing.T) {
	file := "text.primary = #ECEFF4   \t \n"

	got, rejection := lexPairs([]byte(file))

	requireNoRejection(t, rejection)
	requirePairs(t, got, Pair{Key: "text.primary", Value: "#ECEFF4", Line: 1})
}

// TestLex_EmptyValueIsAWellFormedPair pins the deliberate absence of an
// empty-value branch. The line IS a well-formed pair; the value simply is not a
// hex, so it fails later as `bad colour`. That is also the route the file format closes for
// the deferred transparent theme — a distinguished keyword, never btop's empty
// right-hand side.
func TestLex_EmptyValueIsAWellFormedPair(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "spaced separator", file: "text.primary =\n"},
		{name: "bare separator", file: "text.primary=\n"},
		{name: "trailing whitespace after the separator", file: "text.primary =   \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rejection := lexPairs([]byte(tt.file))

			requireNoRejection(t, rejection)
			requirePairs(t, got, Pair{Key: "text.primary", Value: "", Line: 1})
		})
	}
}

// TestLex_InteriorBOMIsBadSyntax is the other half of the BOM rule: the strip
// applies at the first bytes of the file and nowhere else, so a BOM that
// survives is rejected rather than silently corrupting whatever it landed in —
// a key into an unknown one reported as `missing tokens`, or a value into a
// `bad colour`. Both would name the wrong thing for an invisible character.
func TestLex_InteriorBOMIsBadSyntax(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		wantLine   int
		wantDetail string
	}{
		{
			name:       "before a key on a later line",
			file:       "canvas = #2E3440\n" + bom + "text.primary = #ECEFF4\n",
			wantLine:   2,
			wantDetail: "line 2: not a key = value pair",
		},
		{
			name:       "inside a key",
			file:       "text.pri" + bom + "mary = #ECEFF4\n",
			wantLine:   1,
			wantDetail: "line 1: not a key = value pair",
		},
		{
			name:       "inside a value",
			file:       "text.primary = " + bom + "#ECEFF4\n",
			wantLine:   1,
			wantDetail: "line 1: not a key = value pair",
		},
		{
			name:       "a second BOM immediately after the stripped one",
			file:       bom + bom + "canvas = #2E3440\n",
			wantLine:   1,
			wantDetail: "line 1: not a key = value pair",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs, rejection := lexPairs([]byte(tt.file))

			requireBadSyntax(t, pairs, rejection, tt.wantLine, tt.wantDetail)
		})
	}
}

// TestLex_RejectsMalformedPairLines covers the lexical rules's malformed-line branch. The
// well-formed-key rule is what keeps a plain typo honest: without it,
// `text primary = …` is a well-formed pair with an unknown key, which is
// IGNORED, and the file then fails as `missing tokens` — pointing at the wrong
// thing entirely.
func TestLex_RejectsMalformedPairLines(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		wantLine   int
		wantDetail string
	}{
		{
			name:       "no key",
			file:       "= #FFFFFF\n",
			wantLine:   1,
			wantDetail: "line 1: not a key = value pair",
		},
		{
			name:       "no separator",
			file:       "# Nord\n\ncanvas = #2E3440\ntext.primary\n",
			wantLine:   4,
			wantDetail: "line 4: not a key = value pair",
		},
		{
			name:       "key containing a space",
			file:       "text primary = #FFF\n",
			wantLine:   1,
			wantDetail: "line 1: not a key = value pair",
		},
		{
			name:       "key containing a tab",
			file:       "canvas = #2E3440\ntext\tprimary = #FFFFFF\n",
			wantLine:   2,
			wantDetail: "line 2: not a key = value pair",
		},
		{
			name:       "separator with nothing but whitespace to its left",
			file:       "   \t = #FFFFFF\n",
			wantLine:   1,
			wantDetail: "line 1: not a key = value pair",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs, rejection := lexPairs([]byte(tt.file))

			requireBadSyntax(t, pairs, rejection, tt.wantLine, tt.wantDetail)
		})
	}
}

// TestLex_RejectsDuplicateKey pins the duplicate check as purely lexical and
// unconditional: it runs before any key is classified as known or unknown and
// without comparing the two values, because silently taking one of two
// conflicting values is exactly the quiet wrongness the validity rule exists to
// prevent — and making the check conditional adds branches to buy nothing.
//
// The detail names the SECOND occurrence's line, which is the one to delete.
func TestLex_RejectsDuplicateKey(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		wantLine   int
		wantDetail string
	}{
		{
			name:       "known key, same value",
			file:       "canvas = #2E3440\ntext.primary = #ECEFF4\ntext.primary = #ECEFF4\n",
			wantLine:   3,
			wantDetail: "line 3: duplicate key text.primary",
		},
		{
			name:       "known key, different value",
			file:       "canvas = #2E3440\ntext.primary = #ECEFF4\ntext.primary = #C0CAF5\n",
			wantLine:   3,
			wantDetail: "line 3: duplicate key text.primary",
		},
		{
			name:       "unknown key, same value",
			file:       "legacy.thing = #111111\nlegacy.thing = #111111\n",
			wantLine:   2,
			wantDetail: "line 2: duplicate key legacy.thing",
		},
		{
			name:       "unknown key, different value",
			file:       "# header\nlegacy.thing = #111111\n\nlegacy.thing = #222222\n",
			wantLine:   4,
			wantDetail: "line 4: duplicate key legacy.thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs, rejection := lexPairs([]byte(tt.file))

			requireBadSyntax(t, pairs, rejection, tt.wantLine, tt.wantDetail)
		})
	}
}

// TestLex_RejectsAnyLeadingQuote pins "quoted" as a one-character rule about the
// first character: matched or unmatched, either quote. Defining it by a matched
// outer pair would send the unmatched case on down the ladder to `bad colour`,
// telling the user their colour is wrong when their quoting is.
func TestLex_RejectsAnyLeadingQuote(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "matched double quotes", file: `text.primary = "#FFFFFF"` + "\n"},
		{name: "matched single quotes", file: "text.primary = '#FFFFFF'\n"},
		{name: "unmatched double quote", file: `text.primary = "#FFFFFF` + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs, rejection := lexPairs([]byte(tt.file))

			requireBadSyntax(t, pairs, rejection, 1, "line 1: quoted value")
		})
	}
}

// requirePairs fails the test unless the lexer produced exactly want, in order.
func requirePairs(t *testing.T, got []Pair, want ...Pair) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Errorf("lexPairs() = %+v, want %+v", got, want)
	}
}

// requireNoRejection fails the test unless the lexer accepted the file.
func requireNoRejection(t *testing.T, rejection *Rejection) {
	t.Helper()

	if rejection != nil {
		t.Fatalf("lexPairs() rejected the file: %v", rejection)
	}
}

// requireBadSyntax fails the test unless the lexer rejected the file with the
// one expected `bad syntax` rejection. wantDetail is the complete pinned-copy string,
// stated literally by each caller rather than re-derived from the format the
// implementation uses. It also pins that no pairs come back alongside a
// rejection — a caller never sees a partial file.
func requireBadSyntax(t *testing.T, pairs []Pair, rejection *Rejection, wantLine int, wantDetail string) {
	t.Helper()

	if rejection == nil {
		t.Fatalf("lexPairs() accepted the file, want bad syntax at line %d", wantLine)
	}
	if len(pairs) != 0 {
		t.Errorf("lexPairs() returned %+v alongside a rejection, want no pairs", pairs)
	}
	if rejection.Reason != ReasonBadSyntax {
		t.Errorf("rejection reason = %q, want %q", rejection.Reason, ReasonBadSyntax)
	}
	if rejection.Line != wantLine {
		t.Errorf("rejection line = %d, want %d", rejection.Line, wantLine)
	}
	if rejection.Detail != wantDetail {
		t.Errorf("rejection detail = %q, want %q", rejection.Detail, wantDetail)
	}
}
