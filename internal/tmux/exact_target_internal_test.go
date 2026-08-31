package tmux

import "testing"

func TestExactSessionTarget(t *testing.T) {
	if got := ExactSessionTarget("foo"); got != "=foo" {
		t.Errorf("ExactSessionTarget(\"foo\") = %q, want \"=foo\"", got)
	}
}

func TestExactCoordTarget(t *testing.T) {
	if got := ExactCoordTarget("foo"); got != "=foo:" {
		t.Errorf("ExactCoordTarget(\"foo\") = %q, want \"=foo:\"", got)
	}
}
