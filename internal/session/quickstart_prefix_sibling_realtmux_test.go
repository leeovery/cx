package session_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// A create that fails leaves the chain's later steps addressing a session that
// does not exist, next to whatever live sessions share its name as a prefix.
// The stamp must miss there rather than land on a stranger.
//
// The step is run on its own rather than by executing the whole chain: tmux
// abandons a ";" chain at the first failing command, so a whole-chain run would
// pass without the stamp target ever being resolved, and the guarantee here must
// not rest on that behaviour.
func TestQuickStart_DoesNotStampAPrefixSiblingWhenNewSessionFailsMidChain(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	result, name := runQuickStart(t)
	sibling := name + "-2"

	ts := tmuxtest.New(t, "ptl-qsstamp-")
	ts.Run(t, "new-session", "-d", "-s", sibling)
	ts.WaitForSession(t, sibling, 2*time.Second)

	stamp := chainStep(t, result.ExecArgs, "set-option")
	if _, err := ts.TryRun(stamp...); err == nil {
		t.Errorf("tmux %v succeeded against a server holding only %q", stamp, sibling)
	}

	if got := ts.Run(t, "show-options", "-t", sibling); strings.Contains(got, "@portal-dir") {
		t.Errorf("the stamp landed on the live %q session:\n%s", sibling, got)
	}

	// Without the control, a stamp step that tmux refuses for some unrelated
	// reason would read as a pass.
	bare := slices.Clone(stamp)
	bare[targetIndex(t, bare)] = name
	if _, err := ts.TryRun(bare...); err != nil {
		t.Fatalf("tmux %v failed, so this server cannot show a bare target reaching %q: %v", bare, sibling, err)
	}
	if got := ts.Run(t, "show-options", "-t", sibling); !strings.Contains(got, "@portal-dir") {
		t.Fatalf("a bare target did not reach %q either, so the pinned target above proves nothing:\n%s", sibling, got)
	}
}

// chainStep returns the run of arguments starting at cmd and ending before the
// next ";" separator — one command of the exec chain, without the "tmux" argv[0].
func chainStep(t *testing.T, execArgs []string, cmd string) []string {
	t.Helper()
	start := slices.Index(execArgs, cmd)
	if start < 0 {
		t.Fatalf("no %q step in %v", cmd, execArgs)
	}
	rest := execArgs[start:]
	if end := slices.Index(rest, ";"); end >= 0 {
		return rest[:end]
	}
	return rest
}

func targetIndex(t *testing.T, step []string) int {
	t.Helper()
	flag := slices.Index(step, "-t")
	if flag < 0 || flag+1 >= len(step) {
		t.Fatalf("no -t target in %v", step)
	}
	return flag + 1
}

// The miss direction alone would be satisfied by a target form tmux cannot
// resolve at all. That would be worse than the bare form it replaced: tmux
// abandons a ";" chain at its first failure, so a stamp step that never resolves
// takes the attach down with it and quick-start opens nothing.
func TestQuickStart_ComposedStepsResolveTheLiveSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	result, name := runQuickStart(t)
	sibling := name + "-2"

	ts := tmuxtest.New(t, "ptl-qslive-")
	for _, s := range []string{name, sibling} {
		ts.Run(t, "new-session", "-d", "-s", s)
		ts.WaitForSession(t, s, 2*time.Second)
	}

	stamp := chainStep(t, result.ExecArgs, "set-option")
	if out, err := ts.TryRun(stamp...); err != nil {
		t.Fatalf("tmux %v: %v\n%s", stamp, err, out)
	}

	wantOption := session.PortalDirOption + " " + result.Dir
	if got := sessionOptions(t, ts, name); !strings.Contains(got, wantOption) {
		t.Errorf("%q carries no %q after the stamp step:\n%s", name, wantOption, got)
	}
	if got := sessionOptions(t, ts, sibling); strings.Contains(got, session.PortalDirOption) {
		t.Errorf("the stamp also reached the live %q session:\n%s", sibling, got)
	}

	// tmux refuses to attach without a terminal, which is as far as this can
	// go: the step's stdin is the null device, so the attach resolves its
	// target and stops. Absence is what a mis-composed target reads as, and
	// these are the words the client itself classifies on.
	attach := chainStep(t, result.ExecArgs, "attach-session")
	out, _ := ts.TryRun(attach...)
	for _, absence := range []string{"no such session", "can't find session"} {
		if strings.Contains(out, absence) {
			t.Errorf("tmux %v did not resolve the live %q session: %s", attach, name, out)
		}
	}
}

func sessionOptions(t *testing.T, ts *tmuxtest.Socket, name string) string {
	t.Helper()
	return ts.Run(t, "show-options", "-t", string(tmux.CoordTargetExact(name)))
}
