package transienttest

import (
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/leeovery/portal/internal/tmux"
)

// FailureMode selects the policy Commander applies to an intercepted call. A
// single Commander can be rotated between modes during a test.
type FailureMode int

const (
	// PassThrough delegates every call to the inner Commander verbatim.
	PassThrough FailureMode = iota
	// FailExitNonZero models tmux exiting non-zero.
	FailExitNonZero
	// FailEmptyStdout models tmux exiting 0 with empty stdout.
	FailEmptyStdout
)

// Commander wraps an inner tmux.Commander and intercepts only `list-panes -a`
// invocations, delegating everything else verbatim so the rest of a test keeps
// production fidelity. The zero value passes everything through, so a test must
// opt in to a failure policy.
//
// OneShot makes only the first intercepted call apply the policy, letting an
// earlier consumer succeed before a later one observes the transient; the
// default is sticky failure. Only that counter is atomic — Mode and Inner are
// meant to be flipped between phases, not during concurrent tmux activity.
type Commander struct {
	Inner   tmux.Commander
	Mode    FailureMode
	OneShot bool

	intercepted atomic.Int64
}

func (c *Commander) shouldIntercept(args []string) bool {
	if len(args) == 0 || args[0] != "list-panes" {
		return false
	}
	return slices.Contains(args[1:], "-a")
}

func (c *Commander) applyPolicy() (string, error) {
	switch c.Mode {
	case FailExitNonZero:
		return "", fmt.Errorf("tmux list-panes -a: exit 1 (simulated transient)")
	case FailEmptyStdout:
		return "", nil
	case PassThrough:
		// Unreachable: the caller filters PassThrough out. Erroring loudly beats
		// degrading silently if that ever stops being true.
		return "", fmt.Errorf("transienttest.Commander: applyPolicy called with PassThrough mode")
	default:
		return "", fmt.Errorf("transienttest.Commander: unknown failure mode %d", c.Mode)
	}
}

// intercept returns (output, error, true) when the policy applied, or
// ("", nil, false) when the caller should delegate to Inner.
func (c *Commander) intercept(args []string) (string, error, bool) {
	if c.Mode == PassThrough {
		return "", nil, false
	}
	if !c.shouldIntercept(args) {
		return "", nil, false
	}
	n := c.intercepted.Add(1)
	if c.OneShot && n > 1 {
		return "", nil, false
	}
	out, err := c.applyPolicy()
	return out, err, true
}

// Run implements tmux.Commander.
func (c *Commander) Run(args ...string) (string, error) {
	if out, err, handled := c.intercept(args); handled {
		return out, err
	}
	return c.Inner.Run(args...)
}

// RunRaw implements tmux.Commander.
func (c *Commander) RunRaw(args ...string) (string, error) {
	if out, err, handled := c.intercept(args); handled {
		return out, err
	}
	return c.Inner.RunRaw(args...)
}

var _ tmux.Commander = (*Commander)(nil)
