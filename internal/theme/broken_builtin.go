package theme

import "fmt"

// brokenBuiltinFormat is the PINNED user-facing sentence for the one theming
// state that is genuinely fatal: a theme a slot falls back to cannot be loaded
// from the embedded set.
//
// It is a single-sourced constant rather than an fmt.Errorf at the return site
// because it is user-facing copy, and copy is pinned rather than left implicit
// even when — especially when — it sits on a path the build-time guarantee makes
// unreachable. Terse is right here: the binary is broken, the user cannot fix
// it, and there is nothing to advise beyond naming which built-in is at fault.
//
// The EM DASH is part of the copy, not typography this file chose.
const brokenBuiltinFormat = "built-in theme %s is missing or invalid — this binary is broken"

// BrokenBuiltinError returns the fatal error for a fallback slug that does not
// resolve within the embedded set.
//
// It is an ORDINARY ERROR, and everything about the shape follows from that: it
// travels the normal return path to cmd, Execute hands it to main, and main's
// single os.Exit owner writes one line and exits non-zero. This package neither
// panics, nor exits, nor prints — main.go's panic-recovering exit and its
// `process: panic` lifecycle marker stay the backstop for a genuine programming
// fault rather than the designed route for a broken embed.
//
// It carries NO WRAPPED CAUSE, and that is deliberate rather than an omission:
// the underlying rejection says whether the built-in was absent or unparseable,
// and neither answer changes what the user can do. One not-loadable path serves
// every cause, and appending a reason to a sentence that already says "this
// binary is broken" would only make the line longer.
//
// The slug named is the FALLBACK's, never the nomination's. A nomination that
// will not load is ordinary and is absorbed silently by the fallback; naming it
// here would send a user to fix their own prefs.json for a fault in the binary.
func BrokenBuiltinError(slug string) error {
	return fmt.Errorf(brokenBuiltinFormat, slug)
}
