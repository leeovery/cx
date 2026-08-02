package tui

import "github.com/leeovery/portal/internal/theme"

// builtinThemePair is the TRANSITIONAL source of the model's active palette: the
// two embedded built-ins the appearance gate's light/dark answer selects between.
//
// It exists because §3.4's plumbing (the model holds the active Theme and passes
// it where a mode was passed) and §8.4's input path (the constructor takes a
// loaded NOMINATION rather than an appearance) are separate changes landing in
// separate tasks. This holder bridges them: the render layer already threads a
// Theme, while the input is still the surviving `appearance` pref, so "rendering
// is unchanged" stays provable against the unchanged input. Task 3-2 replaces
// this holder with the injected nomination and nothing downstream of activeTheme
// moves.
//
// It is a VALUE on the model, never a package-level var. §3.4 rejects package-level
// mutable theme state outright — it would put order-dependent state on the render
// path in a suite that already forbids t.Parallel() — and §11.2's guard is what
// stops an init-time theme copy returning.
type builtinThemePair struct {
	light theme.Theme
	dark  theme.Theme
}

// selectFor returns the member the gate's answer names. The dark member is the
// default arm, so the zero-value canvasAppearance (§8.8's no-answer fallback)
// selects the dark built-in without a special case.
func (p builtinThemePair) selectFor(appearance canvasAppearance) theme.Theme {
	if appearance == appearanceLightCanvas {
		return p.light
	}
	return p.dark
}

// loadBuiltinThemePair loads the shipped light and dark built-ins through the
// production loader.
//
// It resolves NO PATH and reads no config: LoadBuiltin serves the embedded set,
// which is what keeps TUI construction free of config discovery (and keeps the
// offline capture harness reachable under its no-real-config import guard).
//
// There is deliberately NO fallback beneath it. §7.6 makes a broken built-in
// impossible at BUILD time rather than handled at runtime — the embedded set is
// parsed and validated by a unit test, and the two default slugs are asserted to
// resolve within it — so a rejection here would mean a binary shipped with a
// broken default. §8.5's per-slot fallback and §7.6's fatal startup message
// belong to the nomination path that replaces this bridge; inventing an interim
// fallback here would be a second, unspecified resolution policy.
func loadBuiltinThemePair() builtinThemePair {
	loader := theme.NewLoader(nil)
	return builtinThemePair{
		light: loadBuiltinTheme(loader, theme.DefaultLightSlug),
		dark:  loadBuiltinTheme(loader, theme.DefaultDarkSlug),
	}
}

// loadBuiltinTheme loads one built-in by slug, returning the zero Theme if the
// embedded set somehow does not carry it — the build-time-impossible case above.
func loadBuiltinTheme(loader theme.Loader, slug string) theme.Theme {
	loaded, _, _ := loader.LoadBuiltin(slug)
	return loaded.Theme
}

// defaultDarkTheme is the shipped dark built-in — the palette every directly
// constructed model and preview seeds itself with, so a zero Theme (which
// lipgloss.Color("") turns into the no-colour sentinel) can never reach a
// renderer as a silently colourless frame.
func defaultDarkTheme() theme.Theme {
	return loadBuiltinTheme(theme.NewLoader(nil), theme.DefaultDarkSlug)
}
