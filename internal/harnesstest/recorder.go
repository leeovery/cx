package harnesstest

import "fmt"

// TestingT is the subset of *testing.T a fatal-on-failure helper depends on, so
// the helper can be handed a stand-in and its own failure paths asserted on
// without aborting the test that drives it.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// NamingT is TestingT plus the name of the running test, for the helpers that
// put it into a diagnostic.
type NamingT interface {
	TestingT
	Name() string
}

// Recorder is the stand-in itself: it records what a helper reported rather
// than failing anything. Fatalf stops the helper as the real one does, by
// panicking with a sentinel that Run absorbs — so a helper's statements after a
// Fatalf stay unreached here as they would in a real run.
//
// A helper whose fatal path is meant to return rather than abort — one that
// writes an explicit return after each Fatalf — needs a stand-in that keeps
// going, which this is not.
//
// Unlike the *testing.T it stands in for, it is not safe for concurrent use: a
// helper that reports from more than one goroutine needs its own recorder.
type Recorder struct {
	HelperCalls int
	Errors      []string
	Fatals      []string
}

// fatalSentinel is the panic value Fatalf raises. It is private so that Run can
// tell the abort it staged from a panic the helper itself hit, and it carries
// the message so that a Fatalf reached outside Run says what happened rather
// than crashing the suite anonymously.
type fatalSentinel struct{ msg string }

func (f fatalSentinel) String() string {
	return "harnesstest: Recorder.Fatalf outside Run: " + f.msg
}

func (r *Recorder) Helper() { r.HelperCalls++ }

// Name answers for the helpers that put the running test's name into a
// diagnostic.
func (r *Recorder) Name() string { return "harnesstest.Recorder" }

func (r *Recorder) Errorf(format string, args ...any) {
	r.Errors = append(r.Errors, fmt.Sprintf(format, args...))
}

func (r *Recorder) Fatalf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	r.Fatals = append(r.Fatals, msg)
	panic(fatalSentinel{msg: msg})
}

// Run drives fn, absorbing the abort a Fatalf stands for. Any other panic is
// re-raised: swallowing one would report a helper that crashed as a helper that
// reported nothing.
func (r *Recorder) Run(fn func()) {
	defer func() {
		if raised := recover(); raised != nil {
			if _, staged := raised.(fatalSentinel); !staged {
				panic(raised)
			}
		}
	}()
	fn()
}

// Failed reports whether the helper reported anything at all, fatal or not.
func (r *Recorder) Failed() bool { return len(r.Errors)+len(r.Fatals) > 0 }

// Report renders everything recorded, for the message of a case the recorder
// was expected to come back clean from.
func (r *Recorder) Report() string {
	return fmt.Sprintf("errors=%v fatals=%v", r.Errors, r.Fatals)
}
