package tmux_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/commandertest"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
)

func TestUnregisterPortalHooks_ShowHooksFailureEmitsCanonicalWarn(t *testing.T) {
	sentinel := errors.New("tmux show-hooks failure (teardown)")
	mock := commandertest.FromFunc(perEventDispatchWithFaults(t, "", nil, readErrForAllManagedEvents(sentinel), nil))
	client := tmux.NewClient(mock)

	sink := &logtest.Sink{}
	injected := slog.New(sink).With("component", "bootstrap")

	err := tmux.UnregisterPortalHooksWithLogger(client, injected)
	if err == nil {
		t.Fatal("expected aggregate error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("aggregate error %v does not wrap sentinel %v", err, sentinel)
	}

	warns := showHooksWarnRecords(sink)
	wantWarns := len(tmux.PortalTeardownEvents())
	if len(warns) != wantWarns {
		t.Fatalf("expected exactly %d %q WARNs (one per teardown event, no aggregate double-log), got %d: %v",
			wantWarns, showHooksWarnMessage, len(warns), sink.Records())
	}
	for i, w := range warns {
		t.Run("warn-"+string(rune('0'+i)), func(t *testing.T) {
			assertShowHooksWarnShape(t, w, sentinel)
		})
	}
}
