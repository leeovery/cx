package cmd_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/tmux"
)

// cmd is the only package importing both internal/session and internal/tmux
// cycle-free, so the constant and the format string can only be compared here.
func TestPortalIDOptionBindsHookKeyFormat(t *testing.T) {
	if session.PortalIDOption != "@portal-id" {
		t.Fatalf("session.PortalIDOption = %q; want %q (a change silently orphans every stamped session's resume hook)", session.PortalIDOption, "@portal-id")
	}
	if !strings.Contains(tmux.HookKeyFormat, session.PortalIDOption) {
		t.Errorf("tmux.HookKeyFormat = %q does not contain session.PortalIDOption %q", tmux.HookKeyFormat, session.PortalIDOption)
	}
}
