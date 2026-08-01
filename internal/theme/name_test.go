package theme_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// TestValidSlug_AcceptsCharsetAndAnchors pins §5.2's `^[a-z0-9][a-z0-9-]*$`
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
// `../evil` is the case §8.6 exists for: a persisted slug is used verbatim as a
// path component on a by-name lookup that deliberately does not enumerate, so
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

// TestValidSlug_NoLengthBound pins §5.2's "there is no length bound": the slug
// is an identity, and §9.5/§9.8's truncation is a display concern that must not
// silently become a validity rule.
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

// TestSlugFromFilename_DerivesStem pins §5.1: the filename minus its extension
// IS the slug. Nothing is added, trimmed or rearranged on the accepting path.
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

// TestSlugFromFilename_RejectsNonLowercaseExtension pins §5.6: only the exact
// lowercase `.theme` is ACCEPTED. Enumeration (task 1.7) matches the extension
// case-insensitively so the file is still visible, but a non-exact extension
// never contributes a slug — which is what keeps §5.1's structural uniqueness
// true without a precedence rule.
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

// TestSlugFromFilename_CausesAreDistinct pins the discrimination §14A needs: one
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

// TestSlugFromFilename_NeverNormalisesCase pins §5.2's reject-never-normalise
// rule, which is a safety property rather than a style choice: lowercasing
// `Nord.theme` to `nord` would let a user file shadow the built-in that is the
// §8.5 fallback, so a typo'd drop-in could break the very thing Portal falls
// back to. Rejecting instead is also what keeps §5.4's reserved-name check
// exact string equality.
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
// empty stem, and the empty string stays unambiguously §8.1's unset sentinel
// rather than becoming a selectable theme with no name.
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
