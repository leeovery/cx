// The hook-key subject vocabulary shared across the cmd test suites: the
// hooks.json bodies the seed keys are written into, the enumeration rows they
// arrive in, and the seam fakes that answer with them. The seed keys themselves
// — and the stale-beside-live body — are named once in internal/hookstest, so
// no suite can re-point a name at a different key. Staging — how a test is set
// up and driven, including the store and file fixtures these bodies are staged
// into — lives in testhelpers_test.go.
package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/tmux"
)

// hooksBody renders a hooks.json body registering one on-resume entry per key,
// so a fixture that cares only about which keys are present says exactly that.
// With no keys it renders the empty registry: a file that exists and holds
// nothing.
func hooksBody(keys ...string) string {
	if len(keys) == 0 {
		return "{}"
	}
	entries := make([]string, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, fmt.Sprintf("  %q: {\"on-resume\": \"echo hi\"}", key))
	}
	return "{\n" + strings.Join(entries, ",\n") + "\n}"
}

// tokenRows models the enumeration's answer for stamped panes, and
// unstampedRows for panes carrying no token. The location half is display-only,
// so these fabricate a distinct one per row rather than asserting on it.
func tokenRows(tokens ...string) []tmux.PaneHookRow {
	rows := make([]tmux.PaneHookRow, 0, len(tokens))
	for i, token := range tokens {
		rows = append(rows, tmux.PaneHookRow{Token: token, Location: fmt.Sprintf("stamped%d:0.0", i)})
	}
	return rows
}

func unstampedRows(n int) []tmux.PaneHookRow {
	rows := make([]tmux.PaneHookRow, 0, n)
	for i := range n {
		rows = append(rows, tmux.PaneHookRow{Location: fmt.Sprintf("bare%d:0.0", i)})
	}
	return rows
}

// restoringOption models the @portal-restoring read for the sweep's seam fakes:
// absent by default, so a fake that says nothing about a restore is not
// restoring.
func restoringOption(restoring bool, readErr error) (string, bool, error) {
	if readErr != nil {
		return "", false, readErr
	}
	if !restoring {
		return "", false, nil
	}
	return "1", true, nil
}

// recordingPaneHookLister answers the pane-token enumeration with a fixed set
// of rows (or a fixed failure) and records every read it is asked for, so a
// test can assert on the read count as well as the rows.
type recordingPaneHookLister struct {
	rows  []tmux.PaneHookRow
	err   error
	calls int
}

func (r *recordingPaneHookLister) ListAllPaneHookKeys() ([]tmux.PaneHookRow, error) {
	r.calls++
	return r.rows, r.err
}

var _ PaneHookLister = (*recordingPaneHookLister)(nil)

// stubStaleSweepReader answers the sweep's two seams with fixed values: a fixed
// row set (or failure) for the pane-token enumeration, and a fixed
// @portal-restoring read. It counts the enumeration reads, so a test can assert
// the sweep stood down before enumerating as well as what it read. The optional
// during hook runs at the top of the enumeration, so a test can land a
// concurrent writer's mutation inside it — in the window between the sweep's
// snapshot and the token set that snapshot is weighed against.
type stubStaleSweepReader struct {
	rows         []tmux.PaneHookRow
	err          error
	restoring    bool
	restoringErr error
	during       func()
	calls        int
}

func (s *stubStaleSweepReader) ListAllPaneHookKeys() ([]tmux.PaneHookRow, error) {
	s.calls++
	if s.during != nil {
		s.during()
	}
	return s.rows, s.err
}

func (s *stubStaleSweepReader) TryGetServerOption(string) (string, bool, error) {
	return restoringOption(s.restoring, s.restoringErr)
}

var _ staleSweepReader = (*stubStaleSweepReader)(nil)

// mockKeyResolver answers the registration read with one fixed key (or one
// fixed failure) however many times it is asked, and counts the asks. The fixed
// answer is the point: a case that stamps still reads back the same key, so a
// test can tell a re-read apart from a mint.
type mockKeyResolver struct {
	key   string
	err   error
	calls int
}

func (m *mockKeyResolver) ResolveHookKey(_ string) (string, error) {
	m.calls++
	return m.key, m.err
}

// paneStampCall is one recorded set-option -p, kept whole so a test can assert
// the option name and value as well as the target.
type paneStampCall struct {
	target string
	name   string
	value  string
}

// recordingPaneStamper records every stamp it is asked for without applying
// any, and can be armed with an err a case expects the command to surface — or,
// on a path where no stamp is legal at all, with an err whose text names the
// violation, so a call that should never happen fails loudly.
type recordingPaneStamper struct {
	calls  []paneStampCall
	err    error
	onCall func()
}

func (r *recordingPaneStamper) SetPaneOption(target, name, value string) error {
	if r.onCall != nil {
		r.onCall()
	}
	r.calls = append(r.calls, paneStampCall{target: target, name: name, value: value})
	return r.err
}

// stampedPane models the pane itself: it answers with whatever token has been
// stamped onto it, so a retry reads back the token the failed attempt left.
type stampedPane struct {
	token  string
	stamps []paneStampCall
}

func (p *stampedPane) ResolveHookKey(_ string) (string, error) { return p.token, nil }

func (p *stampedPane) SetPaneOption(target, name, value string) error {
	p.stamps = append(p.stamps, paneStampCall{target: target, name: name, value: value})
	p.token = value
	return nil
}

// The --pane-key path removes a verbatim key and reaches tmux for nothing, so
// both pane seams are armed with an error naming the violation: a call that
// should never happen fails loudly rather than passing silently.
var (
	errPaneKeyResolverCalled = errors.New("the resolver must not be called on the --pane-key path")
	errPaneKeyStamperCalled  = errors.New("the stamper must not be called on the --pane-key path")
)

// paneKeyPathSeams returns the poisoned pair to inject on a --pane-key case,
// and assertNoPaneTmuxCalls is its other half: poisoning one seam proves
// nothing about the other, so every such case guards both.
func paneKeyPathSeams() (*mockKeyResolver, *recordingPaneStamper) {
	return &mockKeyResolver{err: errPaneKeyResolverCalled}, &recordingPaneStamper{err: errPaneKeyStamperCalled}
}

func assertNoPaneTmuxCalls(t *testing.T, resolver *mockKeyResolver, stamper *recordingPaneStamper) {
	t.Helper()
	if resolver.calls != 0 {
		t.Errorf("resolver call count = %d, want 0 on the --pane-key path", resolver.calls)
	}
	if len(stamper.calls) != 0 {
		t.Errorf("set-option call count = %d, want 0 on the --pane-key path: %+v", len(stamper.calls), stamper.calls)
	}
}

// assertHookKeysStaged proves the seed keys a sweep fixture is about to measure
// were staged before it ran. Without it an "absent afterwards" assertion passes
// just as readily on a key the seed never held, so a seed re-pointed at another
// key defangs the fixture instead of failing it.
func assertHookKeysStaged(t *testing.T, store *hooks.Store, keys ...string) {
	t.Helper()
	staged, err := store.Load(hooks.ViaInternal)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	for _, key := range keys {
		if _, ok := staged[key]; !ok {
			t.Fatalf("seed key %q absent from the staged hooks.json before the sweep runs; hooks=%v", key, keysOf(staged))
		}
	}
}
