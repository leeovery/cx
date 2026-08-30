package tmux

import "testing"

func TestExactTarget(t *testing.T) {
	if got := exactTarget("foo"); got != "=foo" {
		t.Errorf("exactTarget(\"foo\") = %q, want \"=foo\"", got)
	}
}

func TestExactCoordTarget(t *testing.T) {
	if got := exactCoordTarget("foo"); got != "=foo:" {
		t.Errorf("exactCoordTarget(\"foo\") = %q, want \"=foo:\"", got)
	}
}
