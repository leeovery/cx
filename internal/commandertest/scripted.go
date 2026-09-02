// Package commandertest provides the shared tmux Commander fake. It lives
// outside _test.go so any package's tests can import it; production code must
// not. It is stdlib-only, and structurally typed against the tmux Commander
// interface rather than importing it, so a test inside the tmux package itself
// can use it without an import cycle.
package commandertest

import (
	"fmt"
	"strings"
	"sync"
)

// Commander is the interface the fake satisfies and delegates to.
type Commander interface {
	Run(args ...string) (string, error)
	RunRaw(args ...string) (string, error)
}

// TestingT is the *testing.T subset the fake reports through, so the fake's own
// failure paths stay assertable.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Matcher accepts the argv an entry answers for.
type Matcher func(args []string) bool

// AnswerFunc resolves an argv to the result the fake will hand back, before the
// Run/RunRaw trim contract is applied to it.
type AnswerFunc func(args ...string) (string, error)

// Scripted is the tmux Commander fake: an ordered argv-pattern-to-result
// script, a recorded call log, and the one implementation of the Run/RunRaw
// trim-versus-verbatim contract.
//
// Its unmatched-argv policy is deliberately loud. A fake that answers an
// unscripted argv with ("", nil) is how a test passes while exercising nothing,
// so the default reports the argv through the TestingT AND returns an error to
// the caller — a production path that swallows the error still leaves a failed
// test behind. A test that genuinely wants a quiet catch-all opts in at its
// construction site via Quiet, AllowingUnmatched or DelegatingTo, so the choice
// is visible where it is made rather than buried in the fake.
//
// The mutex covers Run/RunRaw interleaving from tests that drive production
// goroutines. Errorf (not Fatalf) is used for the unmatched report precisely so
// it is legal from a non-test goroutine.
type Scripted struct {
	mu     sync.Mutex
	tb     TestingT
	script []Entry
	calls  [][]string

	// Exactly one unmatched policy is in force: report (the default), the
	// canned pair, or the delegate.
	allowUnmatched bool
	unmatchedOut   string
	unmatchedErr   error
	fallback       Commander

	strict bool
}

// Entry is one argv pattern and the result it produces. doing, when set, runs
// before the result is returned — the seam for a test that must observe the
// moment a particular argv is dispatched.
type Entry struct {
	match  Matcher
	answer AnswerFunc
	doing  func(args []string)
}

// New builds a fake whose script is consulted in declaration order, first match
// winning. An argv no entry matches fails the test.
func New(tb TestingT, entries ...Entry) *Scripted {
	tb.Helper()
	return &Scripted{tb: tb, script: entries}
}

// Quiet answers every unscripted argv with ("", nil) and records the calls. It
// is the fake for a test that cares only about which tmux commands were issued,
// or about nothing tmux does at all.
func Quiet(entries ...Entry) *Scripted {
	return (&Scripted{script: entries}).AllowingUnmatched("", nil)
}

// Delegating scripts entries ahead of inner, which answers every argv the
// script does not match, through inner's own Run/RunRaw.
func Delegating(inner Commander, entries ...Entry) *Scripted {
	return (&Scripted{script: entries}).DelegatingTo(inner)
}

// FromFunc builds a fake that answers every argv from f. It is the form for a
// test whose fake is a model of tmux rather than an argv script.
func FromFunc(f AnswerFunc) *Scripted {
	return &Scripted{script: []Entry{Answering(Any, f)}}
}

// AllowingUnmatched states the answer for an argv the script does not cover.
func (s *Scripted) AllowingUnmatched(out string, err error) *Scripted {
	s.allowUnmatched = true
	s.unmatchedOut = out
	s.unmatchedErr = err
	return s
}

// DelegatingTo hands an unmatched argv to inner, through inner's own
// Run/RunRaw so its trim semantics are the ones that apply.
func (s *Scripted) DelegatingTo(inner Commander) *Scripted {
	s.fallback = inner
	return s
}

// Strict makes every RunRaw call a fatal test failure, for a package whose
// production path is meant to reach for Run alone. It reports through the
// TestingT, so it belongs on a fake built by New.
func (s *Scripted) Strict() *Scripted {
	s.strict = true
	return s
}

// Any matches every argv.
func Any([]string) bool { return true }

// ArgvPrefix matches an argv beginning with prefix.
func ArgvPrefix(prefix ...string) Matcher {
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

// Returns scripts (out, nil) for an argv beginning with prefix.
func Returns(out string, prefix ...string) Entry {
	return When(ArgvPrefix(prefix...), out, nil)
}

// Fails scripts ("", err) for an argv beginning with prefix.
func Fails(err error, prefix ...string) Entry {
	return When(ArgvPrefix(prefix...), "", err)
}

// When scripts a fixed result for any argv the matcher accepts — the form for a
// pattern that is not a leading-argument prefix.
func When(match Matcher, out string, err error) Entry {
	return Answering(match, func(...string) (string, error) { return out, err })
}

// Answering scripts a computed result for any argv the matcher accepts — the
// form for a fake that models tmux state rather than a fixed table.
func Answering(match Matcher, f AnswerFunc) Entry {
	return Entry{match: match, answer: f}
}

// Doing attaches a side effect to an entry, run when that entry matches.
func (e Entry) Doing(f func(args []string)) Entry {
	e.doing = f
	return e
}

func (s *Scripted) Run(args ...string) (string, error) {
	out, err, matched := s.dispatch(args)
	if !matched && s.fallback != nil {
		return s.fallback.Run(args...)
	}
	return Trim(out, err)
}

func (s *Scripted) RunRaw(args ...string) (string, error) {
	if s.strict {
		s.tb.Fatalf("commandertest: RunRaw called in strict mode with %v", args)
		return "", nil
	}
	out, err, matched := s.dispatch(args)
	if !matched && s.fallback != nil {
		return s.fallback.RunRaw(args...)
	}
	return Verbatim(out, err)
}

// Trim and Verbatim are the single home of the Run/RunRaw trim-versus-verbatim
// contract, and both answer an error with an empty string exactly as the real
// commander does. A bespoke fake that models a subsystem rather than scripting
// argv resolves its own result and hands it to one of these, so a change to the
// contract lands in one place.
//
// Trim is Run's half: the output with surrounding whitespace removed.
func Trim(out string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Verbatim is RunRaw's half: the output exactly as it was produced.
func Verbatim(out string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return out, nil
}

// dispatch records the call and resolves it against the script. The bool
// reports whether an entry matched, so an unmatched argv can still reach a
// delegate through the delegate's own method.
func (s *Scripted) dispatch(args []string) (string, error, bool) {
	s.mu.Lock()
	s.calls = append(s.calls, append([]string(nil), args...))
	entries := s.script
	s.mu.Unlock()

	for _, e := range entries {
		if e.match == nil || !e.match(args) {
			continue
		}
		if e.doing != nil {
			e.doing(args)
		}
		out, err := e.answer(args...)
		return out, err, true
	}
	if s.fallback != nil {
		return "", nil, false
	}
	if s.allowUnmatched {
		return s.unmatchedOut, s.unmatchedErr, false
	}
	if s.tb != nil {
		s.tb.Errorf("commandertest: unscripted tmux argv %v", args)
	}
	return "", fmt.Errorf("commandertest: unscripted tmux argv %v", args), false
}

// Calls returns the argv of every Run and RunRaw call in order. Both methods
// record identically, so an assertion on Calls holds regardless of which one
// production reached for.
func (s *Scripted) Calls() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]string(nil), s.calls...)
}

// ResetCalls forgets the calls recorded so far, so a test can assert what a
// second run of the same production path issued on its own.
func (s *Scripted) ResetCalls() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
}

// CallsMatching returns the recorded calls whose first argument is cmd.
func (s *Scripted) CallsMatching(cmd string) [][]string {
	out := [][]string{}
	for _, call := range s.Calls() {
		if len(call) > 0 && call[0] == cmd {
			out = append(out, call)
		}
	}
	return out
}
