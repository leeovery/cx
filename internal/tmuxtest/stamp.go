package tmuxtest

import (
	"testing"

	"github.com/leeovery/portal/internal/state"
)

// StampPaneToken writes Portal's pane-token option onto target using raw tmux,
// so a fixture never stages itself through the client call under test.
func (s *Socket) StampPaneToken(t *testing.T, target, token string) {
	t.Helper()
	s.Run(t, "set-option", "-p", "-t", target, state.PortalPaneIDOption, token)
}
