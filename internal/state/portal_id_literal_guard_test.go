package state

import (
	"strings"
	"testing"
)

// Spelled out rather than imported from session.PortalIDOption: internal/session
// transitively depends on internal/state, so the import would cycle. It must
// stay byte-identical to that constant.
const portalIDLiteral = "@portal-id"

func TestCaptureFormatContainsPortalIDLiteral(t *testing.T) {
	if portalIDLiteral != "@portal-id" {
		t.Fatalf("portalIDLiteral = %q; want %q (must stay byte-identical to session.PortalIDOption)", portalIDLiteral, "@portal-id")
	}
	if !strings.Contains(captureFormat, portalIDLiteral) {
		t.Errorf("captureFormat = %q does not contain the exact literal %q", captureFormat, portalIDLiteral)
	}
}
