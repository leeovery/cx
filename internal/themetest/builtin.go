package themetest

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// Builtin returns the palette the embedded built-in named slug parses to,
// failing the test on anything but a clean parse. The two failure classes are
// reported separately and must stay that way: a message that could mean either
// hides a broken shipped file behind a typo in the slug.
func Builtin(t *testing.T, slug string) theme.Theme {
	t.Helper()

	loaded, rejection, found := theme.NewSilentLoader().LoadBuiltin(slug)
	if !found {
		t.Fatalf("built-in %q not found in the embedded set", slug)
	}
	if rejection != nil {
		t.Fatalf("built-in %q was rejected: %s", slug, rejection.Reason)
	}
	return loaded.Theme
}

// DefaultDark returns the shipped dark built-in's palette.
func DefaultDark(t *testing.T) theme.Theme {
	t.Helper()
	return Builtin(t, theme.DefaultDarkSlug)
}

// DefaultLight returns the shipped light built-in's palette.
func DefaultLight(t *testing.T) theme.Theme {
	t.Helper()
	return Builtin(t, theme.DefaultLightSlug)
}
