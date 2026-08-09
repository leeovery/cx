package theme_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// TestValidSlug_AcceptsCharsetAndAnchors pins the slug charset rule's `^[a-z0-9][a-z0-9-]*$`
// from the accepting side, including the two edges the anchoring deliberately
// leaves legal: a single character, and a trailing hyphen.
func TestValidSlug_AcceptsCharsetAndAnchors(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{name: "a word", slug: "nord"},
		{name: "one character", slug: "a"},
		{name: "one digit", slug: "0"},
		{name: "interior hyphens", slug: "a-b-c"},
		{name: "a built-in slug", slug: "tokyo-night-day"},
		{name: "digits within a word", slug: "n0rd"},
		{name: "trailing hyphen — legal but pointless", slug: "nord-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !theme.ValidSlug(tt.slug) {
				t.Errorf("ValidSlug(%q) = false, want true", tt.slug)
			}
		})
	}
}

// TestValidSlug_RejectsIllegalForms pins the rule from the rejecting side.
//
// `../evil` is the case the validate-before-use rule exists for: a persisted slug is used
// verbatim as a path component on a by-name lookup that deliberately does not enumerate, so
// the charset check is what stops a hand-edited prefs.json from escaping the
// themes directory.
func TestValidSlug_RejectsIllegalForms(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{name: "empty — the unset sentinel of §8.1, never a slug", slug: ""},
		{name: "path traversal", slug: "../evil"},
		{name: "path separator", slug: "nord/evil"},
		{name: "leading hyphen — reads as a flag", slug: "-nord"},
		{name: "uppercase", slug: "Nord"},
		{name: "space", slug: "nord lee"},
		{name: "underscore", slug: "nord_lee"},
		{name: "dot — a filename is not a slug", slug: "nord.theme"},
		{name: "multi-byte rune", slug: "nörd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if theme.ValidSlug(tt.slug) {
				t.Errorf("ValidSlug(%q) = true, want false", tt.slug)
			}
		})
	}
}

// TestValidSlug_NoLengthBound pins the slug charset rule's "there is no length bound": the
// slug is an identity, and the row-rendering rule/the geometry rule's truncation is a display
// concern that must not silently become a validity rule.
//
// Both surfaces are checked, because the rule is that no length bound exists
// anywhere in the name rules — not merely that the charset check has none.
func TestValidSlug_NoLengthBound(t *testing.T) {
	long := strings.Repeat("a", 300)

	if !theme.ValidSlug(long) {
		t.Errorf("ValidSlug(300 legal characters) = false, want true — there is no length bound")
	}

	slug, rejection := theme.SlugFromFilename(long + ".theme")
	if rejection != nil {
		t.Fatalf("SlugFromFilename(300-character stem) rejected with %v, want the slug", rejection)
	}
	if slug != long {
		t.Errorf("SlugFromFilename(300-character stem) = %q (%d chars), want the whole stem (%d chars)", slug, len(slug), len(long))
	}
}

// TestSlugFromFilename_DerivesStem pins the filename-is-identity rule: the filename minus its
// extension IS the slug. Nothing is added, trimmed or rearranged on the accepting path.
func TestSlugFromFilename_DerivesStem(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "a word", base: "nord.theme", want: "nord"},
		{name: "a built-in slug", base: "tokyo-night-day.theme", want: "tokyo-night-day"},
		{name: "one character", base: "a.theme", want: "a"},
		{name: "trailing hyphen — legal but pointless", base: "nord-.theme", want: "nord-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rejection := theme.SlugFromFilename(tt.base)

			if rejection != nil {
				t.Fatalf("SlugFromFilename(%q) rejected with %v, want the slug %q", tt.base, rejection, tt.want)
			}
			if got != tt.want {
				t.Errorf("SlugFromFilename(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}

// TestSlugFromFilename_RejectsNonLowercaseExtension pins the enumeration rule: only the exact
// lowercase `.theme` is ACCEPTED. Enumeration matches the extension
// case-insensitively so the file is still visible, but a non-exact extension
// never contributes a slug — which is what keeps the filename-is-identity rule's structural
// uniqueness true without a precedence rule.
//
// `Nord.THEME` reports the extension cause even though its stem is illegal too:
// a name that is not a theme filename at all is decided first, so the stem is
// never reached.
func TestSlugFromFilename_RejectsNonLowercaseExtension(t *testing.T) {
	tests := []struct {
		name string
		base string
	}{
		{name: "shouted extension", base: "nord.THEME"},
		{name: "title-cased extension", base: "nord.Theme"},
		{name: "shouted stem and extension", base: "Nord.THEME"},
		{name: "a second extension after it", base: "nord.theme.bak"},
		{name: "no extension at all", base: "nord"},
		{name: "extension without the dot", base: "nordtheme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rejection := theme.SlugFromFilename(tt.base)

			if rejection == nil {
				t.Fatalf("SlugFromFilename(%q) = %q with no rejection, want a bad name rejection", tt.base, got)
			}
			if got != "" {
				t.Errorf("SlugFromFilename(%q) returned slug %q alongside a rejection, want no slug", tt.base, got)
			}
			if rejection.Reason != theme.ReasonBadName {
				t.Errorf("SlugFromFilename(%q) reason = %q, want %q", tt.base, rejection.Reason, theme.ReasonBadName)
			}
			if rejection.BadNameCause != theme.BadNameExtension {
				t.Errorf("SlugFromFilename(%q) cause = %v, want BadNameExtension (%v)", tt.base, rejection.BadNameCause, theme.BadNameExtension)
			}
		})
	}
}

// TestFileExtension_IsWhatSlugFromFilenameAccepts pins the exported extension to
// the comparison it names: a slug composed with FileExtension round-trips, and an
// uppercased FileExtension is rejected as a bad extension.
func TestFileExtension_IsWhatSlugFromFilenameAccepts(t *testing.T) {
	const slug = "nord-lee"

	got, rejection := theme.SlugFromFilename(slug + theme.FileExtension)
	if rejection != nil {
		t.Fatalf("SlugFromFilename(%q) rejected with %v, want the slug %q", slug+theme.FileExtension, rejection, slug)
	}
	if got != slug {
		t.Errorf("SlugFromFilename(%q) = %q, want %q", slug+theme.FileExtension, got, slug)
	}

	shouted := slug + strings.ToUpper(theme.FileExtension)
	got, rejection = theme.SlugFromFilename(shouted)
	if rejection == nil {
		t.Fatalf("SlugFromFilename(%q) = %q with no rejection, want a bad name rejection", shouted, got)
	}
	if rejection.Reason != theme.ReasonBadName || rejection.BadNameCause != theme.BadNameExtension {
		t.Errorf("SlugFromFilename(%q) = {reason %q, cause %v}, want {%q, BadNameExtension (%v)}", shouted, rejection.Reason, rejection.BadNameCause, theme.ReasonBadName, theme.BadNameExtension)
	}
}

// TestSlugFromFilename_CausesAreDistinct pins the discrimination the pinned copy needs: one
// reason class, two causes, so doctor can render `slug must be lowercase
// letters, digits and hyphens` and `extension must be lowercase .theme` from
// the same `bad name` reason.
//
// The zero value is checked too — a rejection that is not a bad-name one
// carries no cause, so a caller can read the field unconditionally.
func TestSlugFromFilename_CausesAreDistinct(t *testing.T) {
	_, badSlug := theme.SlugFromFilename("Nord.theme")
	_, badExtension := theme.SlugFromFilename("nord.THEME")

	if badSlug == nil || badExtension == nil {
		t.Fatalf("both inputs must reject: slug rejection = %v, extension rejection = %v", badSlug, badExtension)
	}
	if badSlug.Reason != badExtension.Reason {
		t.Errorf("reasons differ (%q vs %q), want both %q — one reason class, two causes", badSlug.Reason, badExtension.Reason, theme.ReasonBadName)
	}
	if badSlug.BadNameCause == badExtension.BadNameCause {
		t.Errorf("both causes = %v, want the slug and extension causes to be distinguishable", badSlug.BadNameCause)
	}
	if badSlug.BadNameCause != theme.BadNameSlug {
		t.Errorf("slug cause = %v, want BadNameSlug (%v)", badSlug.BadNameCause, theme.BadNameSlug)
	}
	if badExtension.BadNameCause != theme.BadNameExtension {
		t.Errorf("extension cause = %v, want BadNameExtension (%v)", badExtension.BadNameCause, theme.BadNameExtension)
	}

	if theme.BadNameSlug == theme.BadNameNone || theme.BadNameExtension == theme.BadNameNone {
		t.Errorf("a real cause equals the zero value BadNameNone (%v), want both distinct from it", theme.BadNameNone)
	}
	if other := (theme.Rejection{Reason: theme.ReasonBadSyntax}); other.BadNameCause != theme.BadNameNone {
		t.Errorf("a non-bad-name rejection carries cause %v, want the zero value BadNameNone (%v)", other.BadNameCause, theme.BadNameNone)
	}
}

// TestSlugFromFilename_NeverNormalisesCase pins the slug charset rule's reject-never-normalise
// rule, which is a safety property rather than a style choice: lowercasing
// `Nord.theme` to `nord` would let a user file shadow the built-in that is the
// the per-slot fallback rule fallback, so a typo'd drop-in could break the very thing Portal
// falls back to. Rejecting instead is also what keeps the reserved-slug rule's reserved-name
// check exact string equality.
func TestSlugFromFilename_NeverNormalisesCase(t *testing.T) {
	tests := []struct {
		name string
		base string
	}{
		{name: "leading capital", base: "Nord.theme"},
		{name: "shouted", base: "NORD.theme"},
		{name: "mixed case with hyphens", base: "Tokyo-Night.theme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rejection := theme.SlugFromFilename(tt.base)

			if rejection == nil {
				t.Fatalf("SlugFromFilename(%q) = %q with no rejection, want a bad name rejection", tt.base, got)
			}
			if got != "" {
				t.Errorf("SlugFromFilename(%q) = %q, want no slug — a name is never lowercased into one", tt.base, got)
			}
			if rejection.Reason != theme.ReasonBadName || rejection.BadNameCause != theme.BadNameSlug {
				t.Errorf("SlugFromFilename(%q) = {reason %q, cause %v}, want {%q, BadNameSlug (%v)}", tt.base, rejection.Reason, rejection.BadNameCause, theme.ReasonBadName, theme.BadNameSlug)
			}
		})
	}
}

// TestSlugFromFilename_EmptyStemRejected pins the one edge the anchoring closes
// on the file side: a file named exactly `.theme` has a legal extension and an
// empty stem, and the empty string stays unambiguously the on-disk prefs shape's unset
// sentinel rather than becoming a selectable theme with no name.
func TestSlugFromFilename_EmptyStemRejected(t *testing.T) {
	got, rejection := theme.SlugFromFilename(".theme")

	if rejection == nil {
		t.Fatalf("SlugFromFilename(\".theme\") = %q with no rejection, want a bad name rejection", got)
	}
	if got != "" {
		t.Errorf("SlugFromFilename(\".theme\") = %q, want no slug", got)
	}
	if rejection.Reason != theme.ReasonBadName || rejection.BadNameCause != theme.BadNameSlug {
		t.Errorf("SlugFromFilename(\".theme\") = {reason %q, cause %v}, want {%q, BadNameSlug (%v)} — the extension is legal, the stem is not", rejection.Reason, rejection.BadNameCause, theme.ReasonBadName, theme.BadNameSlug)
	}
}

// TestSlugFromFilename_RejectsLeadingHyphenStem pins the other anchor on the
// file side: a leading hyphen reads as a flag in every context a slug is typed
// into, so `-nord.theme` yields no slug even though its extension is exact.
func TestSlugFromFilename_RejectsLeadingHyphenStem(t *testing.T) {
	tests := []struct {
		name string
		base string
	}{
		{name: "leading hyphen", base: "-nord.theme"},
		{name: "underscore", base: "nord_lee.theme"},
		{name: "space", base: "nord lee.theme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rejection := theme.SlugFromFilename(tt.base)

			if rejection == nil {
				t.Fatalf("SlugFromFilename(%q) = %q with no rejection, want a bad name rejection", tt.base, got)
			}
			if rejection.Reason != theme.ReasonBadName || rejection.BadNameCause != theme.BadNameSlug {
				t.Errorf("SlugFromFilename(%q) = {reason %q, cause %v}, want {%q, BadNameSlug (%v)}", tt.base, rejection.Reason, rejection.BadNameCause, theme.ReasonBadName, theme.BadNameSlug)
			}
		})
	}
}

// TestStripControl_RemovesAnsiEscapes pins that a whole escape SEQUENCE goes,
// not merely its ESC byte.
//
// Stripping the ESC alone would leave `[31m` behind — printable, so it survives
// any control-character pass, and it would then be echoed into the pinned copy's message as
// if the user had typed it. The sequence is the unit, which is why this composes
// the terminal-grammar parser rather than filtering bytes.
func TestStripControl_RemovesAnsiEscapes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "an SGR colour sequence", in: "\x1b[31mnord\x1b[0m", want: "nord"},
		{name: "a cursor-move sequence", in: "no\x1b[2Krd", want: "nord"},
		{name: "an OSC sequence", in: "\x1b]0;title\x07nord", want: "nord"},
		// A trailing ESC has nothing to terminate it, so it is the one shape
		// that isolates the byte itself. An ESC anywhere else is deliberately
		// NOT tested as a lone byte: `\x1br` is a complete two-byte escape
		// sequence, so a parser that removed only the ESC and kept the `r`
		// would be the wrong one — the sequence is the unit.
		{name: "a trailing escape", in: "nord\x1b", want: "nord"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := theme.StripControl(tt.in); got != tt.want {
				t.Errorf("StripControl(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestStripControl_RemovesControlCharacters pins the other half: the bare C0
// characters a paste carries, which an escape-sequence parser leaves in place
// because they open no sequence.
//
// The newline is the one that matters most — the pinned copy's frames are single lines, and
// a value carrying one would split a refusal in two, with the second half
// looking like a message Portal never wrote.
func TestStripControl_RemovesControlCharacters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "a newline", in: "no\nrd", want: "nord"},
		{name: "a carriage return", in: "no\rrd", want: "nord"},
		{name: "a tab", in: "no\trd", want: "nord"},
		{name: "a NUL", in: "no\x00rd", want: "nord"},
		{name: "a delete", in: "no\x7frd", want: "nord"},
		{name: "a trailing newline", in: "nord\n", want: "nord"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := theme.StripControl(tt.in); got != tt.want {
				t.Errorf("StripControl(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestStripControl_LeavesEverythingElseAlone pins the negative half: stripping
// is not normalising.
//
// A value that is merely WRONG — the wrong case, an illegal punctuation mark, a
// traversal attempt — must reach the charset check unaltered, so it is reported
// as the thing the user typed rather than as a quietly corrected version of it.
// Only what cannot be echoed is removed.
func TestStripControl_LeavesEverythingElseAlone(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "a valid slug", in: "nord-lee"},
		{name: "the wrong case", in: "Nord"},
		{name: "a leading hyphen", in: "-nord"},
		{name: "a traversal attempt", in: "../evil"},
		{name: "a space", in: "nord lee"},
		{name: "a multi-byte rune", in: "nørd"},
		{name: "the empty string", in: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := theme.StripControl(tt.in); got != tt.in {
				t.Errorf("StripControl(%q) = %q, want it unchanged — stripping is not normalising", tt.in, got)
			}
		})
	}
}
