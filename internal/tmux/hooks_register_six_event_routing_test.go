package tmux_test

import (
	"fmt"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

func TestRegisterPortalHooks_NonSessionClosedEventsRouteToNotifyCommand(t *testing.T) {
	for _, ev := range nonSessionClosedSaveTriggerEvents {
		t.Run(fmt.Sprintf("%s is registered with notifyCommand and not commitNowCommand", ev), func(t *testing.T) {
			mock := &MockCommander{RunFunc: perEventDispatch(t, "", nil)}
			client := tmux.NewClient(mock)

			if err := tmux.RegisterPortalHooks(client, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var eventCalls [][2]string
			for _, c := range setHookCalls(mock.Calls) {
				if c[0] == ev {
					eventCalls = append(eventCalls, c)
				}
			}

			if len(eventCalls) != 1 {
				t.Fatalf(
					"expected exactly 1 set-hook -ga on %q, got %d: %v\n"+
						"--- full set-hook -ga call log ---\n%v",
					ev, len(eventCalls), eventCalls, setHookCalls(mock.Calls),
				)
			}

			got := eventCalls[0][1]
			if got != expectedNotifyCommand {
				t.Errorf(
					"event %q registered with %q, want %q "+
						"(notifyCommand — the cheap dirty-flag touch)",
					ev, got, expectedNotifyCommand,
				)
			}
			if got == expectedCommitNowCommand {
				t.Errorf(
					"event %q was REGRESSION-routed onto commitNowCommand %q; "+
						"only session-closed may carry commitNowCommand per spec § "+
						"Registration Redesign — \"Ensure Exactly One\"",
					ev, expectedCommitNowCommand,
				)
			}
		})
	}
}
