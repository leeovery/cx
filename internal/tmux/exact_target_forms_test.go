package tmux_test

import (
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

func TestSessionTargetExact(t *testing.T) {
	if got := tmux.SessionTargetExact("foo"); got != "=foo" {
		t.Errorf("SessionTargetExact(\"foo\") = %q, want \"=foo\"", got)
	}
}

func TestCoordTargetExact(t *testing.T) {
	if got := tmux.CoordTargetExact("foo"); got != "=foo:" {
		t.Errorf("CoordTargetExact(\"foo\") = %q, want \"=foo:\"", got)
	}
}
