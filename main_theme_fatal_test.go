package main

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// TestRun_ThemeFatalPrintsOnePinnedLine closes §7.6's escalation at the only
// place it ends: main, the single owner of os.Exit.
//
// A binary whose embedded set cannot supply the theme a slot falls back to has
// nothing honest left to paint, and the whole design of the failure is what the
// user sees instead — §14A's ONE LINE, then a non-zero exit. Not a Go panic
// trace, not a colourless picker painted from a zero palette, and not a silent
// exit code.
//
// The assertion is byte-exact on both counts. The line is the pinned sentence
// plus exactly one newline, so a second write anywhere on the path — a
// well-meaning "theme:" log line rendered to stderr, or a duplicate print from
// Execute's fatal branch — fails it; and the code is 1, the ordinary-error code,
// rather than 2, which this process reserves for a usage error and a recovered
// panic.
//
// It exercises run() rather than main() because main() calls the real os.Exit.
// run() is the panic-recovering closure main wraps, so this covers every step
// between Execute returning and the exit code being handed over — which is the
// whole of what main adds.
func TestRun_ThemeFatalPrintsOnePinnedLine(t *testing.T) {
	// §14A's sentence, transcribed from the specification rather than composed
	// from internal/theme's own format string, so the two must agree.
	const want = "built-in theme tokyo-night is missing or invalid — this binary is broken\n"

	buf := withSeams(t, func() error { return theme.BrokenBuiltinError(theme.DefaultDarkSlug) })

	code, panicked := run()

	if panicked {
		t.Error("panicked = true, want false — the fatal is a returned error, and main's recover stays the backstop for a programming fault")
	}
	if code != 1 {
		t.Errorf("code = %d, want 1 — a non-zero exit through the ordinary-error path", code)
	}
	if got := buf.String(); got != want {
		t.Errorf("stderr =\n %q\nwant %q", got, want)
	}
}
