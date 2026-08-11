package main

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// Exercises run() rather than main() because main() calls the real os.Exit.
func TestRun_ThemeFatalPrintsOnePinnedLine(t *testing.T) {
	// Transcribed from the specification rather than composed from
	// internal/theme's format string, so the two must agree.
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
