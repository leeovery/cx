package themetest

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// Builtin returns the palette the embedded built-in named slug parses to,
// failing the test on anything but a clean parse.
//
// The two failure classes are reported SEPARATELY and must stay that way: "the
// slug names no built-in" is a test naming a theme Portal does not ship, while
// "the shipped file is broken" is the state build-time validation of the
// embedded set exists to make impossible. A message that could mean either
// hides the second behind a typo in the first.
//
// A rejection is therefore a Fatal rather than a fallback — there is no degraded
// palette a test could meaningfully carry on with.
func Builtin(t *testing.T, slug string) theme.Theme {
	t.Helper()

	loaded, rejection, found := theme.NewLoader(nil).LoadBuiltin(slug)
	if !found {
		t.Fatalf("built-in %q not found in the embedded set", slug)
	}
	if rejection != nil {
		t.Fatalf("built-in %q was rejected: %s", slug, rejection.Reason)
	}
	return loaded.Theme
}

// DefaultDark returns the shipped dark built-in's palette — the one a model
// resolves for the dark half of the default pair.
func DefaultDark(t *testing.T) theme.Theme {
	t.Helper()
	return Builtin(t, theme.DefaultDarkSlug)
}

// DefaultLight returns the shipped light built-in's palette — the one a model
// resolves for the light half of the default pair.
func DefaultLight(t *testing.T) theme.Theme {
	t.Helper()
	return Builtin(t, theme.DefaultLightSlug)
}
