package hooks

import "fmt"

// LookupOnResume returns the on-resume command registered for hookKey. A missing
// or malformed hooks.json degrades silently to "no hook", as does an empty
// command; only a genuine I/O error is returned.
//
// hookKey is the raw saved identifier, un-sanitised, so colons in a session name
// round-trip verbatim.
func LookupOnResume(store *Store, hookKey string) (string, bool, error) {
	h, err := store.Load()
	if err != nil {
		return "", false, fmt.Errorf("load hooks: %w", err)
	}
	events, ok := h[hookKey]
	if !ok {
		return "", false, nil
	}
	cmd, ok := events["on-resume"]
	if !ok || cmd == "" {
		return "", false, nil
	}
	return cmd, true, nil
}
