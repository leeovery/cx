package theme

import (
	"slices"
	"strings"
	"testing"
)

const bom = "\uFEFF"

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

func TestLex_HashStartsCommentOnlyAtLineStart(t *testing.T) {
	file := "text.primary = #ECEFF4 # tuned for the lighter canvas\n" +
		"# a whole-line comment\n"

	got, rejection := lexPairs([]byte(file))

	requireNoRejection(t, rejection)
	requirePairs(t, got, Pair{Key: "text.primary", Value: "#ECEFF4 # tuned for the lighter canvas", Line: 1})
}

func TestLex_ToleratesCRLF(t *testing.T) {
	lf := "# a comment\n\ncanvas = #2E3440\ntext.primary = #ECEFF4\n"
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")

	wantPairs, rejection := lexPairs([]byte(lf))
	requireNoRejection(t, rejection)

	got, rejection := lexPairs([]byte(crlf))

	requireNoRejection(t, rejection)
	requirePairs(t, got, wantPairs...)
}

func TestLex_StripsBOMAtFileStartOnly(t *testing.T) {
	file := bom + "canvas = #2E3440\ntext.primary = #ECEFF4\n"

	got, rejection := lexPairs([]byte(file))

	requireNoRejection(t, rejection)
	requirePairs(t, got,
		Pair{Key: "canvas", Value: "#2E3440", Line: 1},
		Pair{Key: "text.primary", Value: "#ECEFF4", Line: 2},
	)
}

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

func TestLex_TrailingWhitespaceIsNotAnError(t *testing.T) {
	file := "text.primary = #ECEFF4   \t \n"

	got, rejection := lexPairs([]byte(file))

	requireNoRejection(t, rejection)
	requirePairs(t, got, Pair{Key: "text.primary", Value: "#ECEFF4", Line: 1})
}

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

func requirePairs(t *testing.T, got []Pair, want ...Pair) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Errorf("lexPairs() = %+v, want %+v", got, want)
	}
}

func requireNoRejection(t *testing.T, rejection *Rejection) {
	t.Helper()

	if rejection != nil {
		t.Fatalf("lexPairs() rejected the file: %v", rejection)
	}
}

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
