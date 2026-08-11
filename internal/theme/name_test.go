package theme_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

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

func TestValidSlug_RejectsIllegalForms(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{name: "empty — the unset sentinel, never a slug", slug: ""},
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

func TestSlugFromFilename_RejectsNonLowercaseExtension(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		wantCause theme.BadNameCause
	}{
		{name: "shouted extension", base: "nord.THEME", wantCause: theme.BadNameExtension},
		{name: "title-cased extension", base: "nord.Theme", wantCause: theme.BadNameExtension},
		{name: "shouted stem and extension", base: "Nord.THEME", wantCause: theme.BadNameSlug},
		{name: "a second extension after it", base: "nord.theme.bak", wantCause: theme.BadNameExtension},
		{name: "no extension at all", base: "nord", wantCause: theme.BadNameExtension},
		{name: "extension without the dot", base: "nordtheme", wantCause: theme.BadNameExtension},
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
			if rejection.BadNameCause != tt.wantCause {
				t.Errorf("SlugFromFilename(%q) cause = %v, want %v", tt.base, rejection.BadNameCause, tt.wantCause)
			}
		})
	}
}

func TestSlugFromFilename_ExtensionCauseOnlyWhenStemIsLegal(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		wantSlug  string
		wantCause theme.BadNameCause
	}{
		{name: "legal stem, shouted extension", base: "nord.THEME", wantCause: theme.BadNameExtension},
		{name: "legal stem, title-cased extension", base: "sunset.Theme", wantCause: theme.BadNameExtension},
		{name: "illegal stem, shouted extension", base: "Nord.THEME", wantCause: theme.BadNameSlug},
		{name: "spaced stem, shouted extension", base: "My Theme.THEME", wantCause: theme.BadNameSlug},
		{name: "illegal stem, exact extension", base: "My Theme.theme", wantCause: theme.BadNameSlug},
		{name: "no .theme-shaped suffix", base: "nord.txt", wantCause: theme.BadNameExtension},
		{name: "legal stem, exact extension", base: "nord.theme", wantSlug: "nord"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rejection := theme.SlugFromFilename(tt.base)

			if tt.wantSlug != "" {
				if rejection != nil {
					t.Fatalf("SlugFromFilename(%q) rejected with %v, want the slug %q", tt.base, rejection, tt.wantSlug)
				}
				if got != tt.wantSlug {
					t.Errorf("SlugFromFilename(%q) = %q, want %q", tt.base, got, tt.wantSlug)
				}
				return
			}

			if rejection == nil {
				t.Fatalf("SlugFromFilename(%q) = %q with no rejection, want a bad name rejection", tt.base, got)
			}
			if got != "" {
				t.Errorf("SlugFromFilename(%q) = %q alongside a rejection, want no slug", tt.base, got)
			}
			if rejection.Reason != theme.ReasonBadName {
				t.Errorf("SlugFromFilename(%q) reason = %q, want %q", tt.base, rejection.Reason, theme.ReasonBadName)
			}
			if rejection.BadNameCause != tt.wantCause {
				t.Errorf("SlugFromFilename(%q) cause = %v, want %v", tt.base, rejection.BadNameCause, tt.wantCause)
			}
		})
	}
}

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

func TestStripControl_RemovesAnsiEscapes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "an SGR colour sequence", in: "\x1b[31mnord\x1b[0m", want: "nord"},
		{name: "a cursor-move sequence", in: "no\x1b[2Krd", want: "nord"},
		{name: "an OSC sequence", in: "\x1b]0;title\x07nord", want: "nord"},
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
