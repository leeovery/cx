package tmux_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/commandertest"
	"github.com/leeovery/portal/internal/tmux"
)

func TestPortalHookCountsByEvent_CountsOnlyPortalFingerprintEntries(t *testing.T) {
	const foreign = `run-shell "tmux-resurrect save"`
	seeded := convergedTable() +
		fmt.Sprintf("pane-focus-out[1] => '%s'\n", foreign) +
		fmt.Sprintf("window-layout-changed[1] => '%s'\n", expectedNotifyCommand)

	mock := commandertest.FromFunc(perEventDispatch(t, seeded, nil))
	client := tmux.NewClient(mock)

	counts, err := tmux.PortalHookCountsByEvent(client)
	if err != nil {
		t.Fatalf("PortalHookCountsByEvent: %v", err)
	}

	for _, ev := range tmux.ManagedEventNames() {
		if _, ok := counts[ev]; !ok {
			t.Errorf("event %q missing from counts map %v", ev, counts)
		}
	}

	if counts["pane-focus-out"] != 1 {
		t.Errorf("pane-focus-out count = %d, want 1 (foreign entry must be ignored)", counts["pane-focus-out"])
	}

	if counts["window-layout-changed"] != 2 {
		t.Errorf("window-layout-changed count = %d, want 2 (stacked Portal duplicate)", counts["window-layout-changed"])
	}

	for _, ev := range tmux.ManagedEventNames() {
		if ev == "window-layout-changed" {
			continue
		}
		if counts[ev] != 1 {
			t.Errorf("event %q count = %d, want 1", ev, counts[ev])
		}
	}
}

func TestPortalHookCountsByEvent_PerEventReadFailurePropagates(t *testing.T) {
	sentinel := errors.New("tmux show-hooks failure on pane-focus-out")
	mock := commandertest.FromFunc(perEventDispatchWithFaults(t, convergedTable(), nil,
		map[string]error{"pane-focus-out": sentinel}, nil))
	client := tmux.NewClient(mock)

	counts, err := tmux.PortalHookCountsByEvent(client)
	if counts != nil {
		t.Errorf("counts = %v, want nil on read failure", counts)
	}
	if err == nil {
		t.Fatal("expected wrapped error on per-event read failure, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
	}
	if !strings.Contains(err.Error(), "show-hooks failed on pane-focus-out") {
		t.Errorf("error %q missing the 'show-hooks failed on pane-focus-out' wrap", err.Error())
	}
}
