package session

import "github.com/leeovery/portal/internal/nanoid"

// NewPaneToken mints an opaque token for a pane's durable identity at the
// pane-token width, the one nanoid.IsTokenShaped recognises. It returns an
// error only when the system entropy source fails.
func NewPaneToken() (string, error) {
	return nanoid.NewPaneTokenGenerator()()
}
