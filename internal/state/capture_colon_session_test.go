package state_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// The name at the heart of the case: tmux accepts it, Portal's exact-target form
// cannot address it, and tmux reports that failure in the words it uses for a
// session that no longer exists.
const colonSession = "a:b"

func TestCaptureStructureColonNamedSession(t *testing.T) {
	t.Run("it classifies an unaddressable session name as anomalous rather than natural churn", func(t *testing.T) {
		logger, sink := openTestLogger(t, t.TempDir())
		mock := &captureMock{
			listSessions: listSessionsFor(colonSession, "plain"),
			listPanes: strings.Join([]string{
				paneLine(colonSession, 0, "m", "L", false, true, 0, "/a", true, "zsh"),
				paneLine("plain", 0, "m", "L", false, true, 0, "/p", true, "zsh"),
			}, "\n"),
			envErrs: map[string]error{
				colonSession: noSuchSessionErr("=" + colonSession),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "plain" {
			t.Fatalf("Sessions = %+v, want only [plain]", idx.Sessions)
		}

		log := sink.Body()
		if !strings.Contains(log, "capture anomalous session error") {
			t.Errorf("expected the anomalous WARN for %q; log:\n%s", colonSession, log)
		}
		if strings.Contains(log, "vanished") {
			t.Errorf("a live colon-named session must not be reported as vanished; log:\n%s", log)
		}
	})
}
