// The hook-key subject vocabulary shared across the cmd test suites: the seed
// keys, the enumeration rows they arrive in, and the seam fakes that answer
// with them.
package cmd

import (
	"fmt"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/transienttest"
)

// The hook-key seed vocabulary the cmd suites share: a reapable key is one the
// staleness rule can judge, so it is swept once absent from the live set; an
// unjudgeable key is retained whatever the live set says.
var (
	reapableSeedA = transienttest.ReapableHookKey(0)
	reapableSeedB = transienttest.ReapableHookKey(1)
	reapableSeedC = transienttest.ReapableHookKey(2)
	reapableSeedD = transienttest.ReapableHookKey(3)

	unjudgeableSeedA = transienttest.UnjudgeableHookKey(0)
	unjudgeableSeedB = transienttest.UnjudgeableHookKey(1)

	// The live half of the vocabulary: token-shaped keys the enumeration
	// reports, so an entry under one is preserved because its pane is live and
	// not because the reaper cannot judge its shape.
	liveSeedA = transienttest.ReapableHookKey(4)
	liveSeedB = transienttest.ReapableHookKey(5)
	liveSeedC = transienttest.ReapableHookKey(6)
)

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

// recordingHookKeyLister is the sweep's seam: the same enumeration fake, plus
// the @portal-restoring marker read the sweep gates itself on.
type recordingHookKeyLister struct {
	recordingPaneHookLister
	restoring    bool
	restoringErr error
}

func (r *recordingHookKeyLister) TryGetServerOption(string) (string, bool, error) {
	return restoringOption(r.restoring, r.restoringErr)
}

var _ AllPaneLister = (*recordingHookKeyLister)(nil)
