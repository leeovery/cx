package session

// NewPaneToken mints an opaque token for a pane's durable identity, drawn from
// the same generator as every other Portal id so IsTokenShaped recognises it.
// It returns an error only when the system entropy source fails.
func NewPaneToken() (string, error) {
	return NewNanoIDGenerator()()
}
