package cmd

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

// scriptedCommander is the shared tmux.Commander fake for this package: an
// ordered argv-pattern-to-result script, a recorded call log, and one
// implementation of the Run/RunRaw trim-versus-verbatim contract.
//
// Its unmatched-argv policy is deliberately loud. A fake that answers an
// unscripted argv with ("", nil) is how a test passes while exercising nothing,
// so the default reports the argv through the *testing.TB AND returns an error
// to the caller — a production path that swallows the error still leaves a
// failed test behind. A test that genuinely wants a quiet catch-all opts in at
// its construction site via allowingUnmatched or delegatingTo, so the choice is
// visible where it is made rather than buried in the fake.
//
// The mutex covers Run/RunRaw interleaving from tests that drive production
// goroutines. tb.Errorf (not Fatalf) is used precisely so the unmatched report
// is legal from a non-test goroutine.
type scriptedCommander struct {
	mu     sync.Mutex
	tb     testing.TB
	script []scriptEntry
	calls  [][]string

	// Exactly one unmatched policy is in force: report (the default), the
	// canned pair, or the delegate.
	allowUnmatched bool
	unmatchedOut   string
	unmatchedErr   error
	fallback       tmux.Commander
}

// scriptEntry is one argv pattern and the result it produces. onCall, when set,
// runs before the result is returned — the seam for a test that must observe
// the moment a particular argv is dispatched.
type scriptEntry struct {
	match  func(args []string) bool
	out    string
	err    error
	onCall func(args []string)
}

// newScriptedCommander builds a fake whose script is consulted in declaration
// order, first match winning. An argv no entry matches fails the test.
func newScriptedCommander(tb testing.TB, entries ...scriptEntry) *scriptedCommander {
	tb.Helper()
	return &scriptedCommander{tb: tb, script: entries}
}

// allowingUnmatched states the answer for an argv the script does not cover.
func (s *scriptedCommander) allowingUnmatched(out string, err error) *scriptedCommander {
	s.allowUnmatched = true
	s.unmatchedOut = out
	s.unmatchedErr = err
	return s
}

// delegatingTo hands an unmatched argv to inner, through inner's own Run/RunRaw
// so its trim semantics are the ones that apply.
func (s *scriptedCommander) delegatingTo(inner tmux.Commander) *scriptedCommander {
	s.fallback = inner
	return s
}

// commanderDelegatingTo scripts entries ahead of inner, which answers every
// argv the script does not match, through inner's own Run/RunRaw.
func commanderDelegatingTo(inner tmux.Commander, entries ...scriptEntry) *scriptedCommander {
	return (&scriptedCommander{script: entries}).delegatingTo(inner)
}

// quietCommander answers every argv with ("", nil) and records the calls. It is
// the fake for a test that cares only about which tmux commands were issued, or
// about nothing tmux does at all.
func quietCommander(entries ...scriptEntry) *scriptedCommander {
	return (&scriptedCommander{script: entries}).allowingUnmatched("", nil)
}

// argvPrefix matches an argv beginning with prefix.
func argvPrefix(prefix ...string) func(args []string) bool {
	return func(args []string) bool {
		if len(args) < len(prefix) {
			return false
		}
		for i, p := range prefix {
			if args[i] != p {
				return false
			}
		}
		return true
	}
}

// returns scripts (out, nil) for an argv beginning with prefix.
func returns(out string, prefix ...string) scriptEntry {
	return scriptEntry{match: argvPrefix(prefix...), out: out}
}

// fails scripts ("", err) for an argv beginning with prefix.
func fails(err error, prefix ...string) scriptEntry {
	return scriptEntry{match: argvPrefix(prefix...), err: err}
}

// when scripts a result for any argv the predicate accepts — the form for a
// pattern that is not a leading-argument prefix.
func when(match func(args []string) bool, out string, err error) scriptEntry {
	return scriptEntry{match: match, out: out, err: err}
}

// doing attaches a side effect to an entry, run when that entry matches.
func (e scriptEntry) doing(f func(args []string)) scriptEntry {
	e.onCall = f
	return e
}

// commanderRun and commanderRunRaw are the single home of the Run/RunRaw
// trim-versus-verbatim contract for this package's fakes: Run trims the
// output, RunRaw returns it verbatim, and both answer an error with an empty
// string exactly as the real commander does. Every fake here resolves its own
// result and then hands it to one of these, so a change to the contract lands
// in one place.
func commanderRun(out string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func commanderRunRaw(out string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return out, nil
}

func (s *scriptedCommander) Run(args ...string) (string, error) {
	out, err, matched := s.dispatch(args)
	if !matched && s.fallback != nil {
		return s.fallback.Run(args...)
	}
	return commanderRun(out, err)
}

func (s *scriptedCommander) RunRaw(args ...string) (string, error) {
	out, err, matched := s.dispatch(args)
	if !matched && s.fallback != nil {
		return s.fallback.RunRaw(args...)
	}
	return commanderRunRaw(out, err)
}

// dispatch records the call and resolves it against the script. The bool
// reports whether an entry matched, so an unmatched argv can still reach a
// delegate through the delegate's own method.
func (s *scriptedCommander) dispatch(args []string) (string, error, bool) {
	s.mu.Lock()
	s.calls = append(s.calls, append([]string(nil), args...))
	entries := s.script
	s.mu.Unlock()

	for _, e := range entries {
		if e.match == nil || !e.match(args) {
			continue
		}
		if e.onCall != nil {
			e.onCall(args)
		}
		return e.out, e.err, true
	}
	if s.fallback != nil {
		return "", nil, false
	}
	if s.allowUnmatched {
		return s.unmatchedOut, s.unmatchedErr, false
	}
	if s.tb != nil {
		s.tb.Errorf("scriptedCommander: unscripted tmux argv %v", args)
	}
	return "", fmt.Errorf("scriptedCommander: unscripted tmux argv %v", args), false
}

// Calls returns the argv of every Run and RunRaw call in order. Both methods
// record identically, so an assertion on Calls holds regardless of which one
// production reached for.
func (s *scriptedCommander) Calls() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]string(nil), s.calls...)
}

// callsMatching returns the recorded calls whose first argument is cmd.
func (s *scriptedCommander) callsMatching(cmd string) [][]string {
	out := [][]string{}
	for _, call := range s.Calls() {
		if len(call) > 0 && call[0] == cmd {
			out = append(out, call)
		}
	}
	return out
}

var _ tmux.Commander = (*scriptedCommander)(nil)
