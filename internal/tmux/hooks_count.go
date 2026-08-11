package tmux

import "fmt"

// PortalHookCountsByEvent reports, for every Portal-managed tmux event, how
// many global hook entries are Portal-authored. The map always carries every
// managed event, zero-count included, so a caller can tell "not registered"
// from "registered once"; a read failure returns a nil map and an error.
//
// The per-event loop must not be collapsed into a single whole-scope read —
// see ShowGlobalHooksForEvent.
func PortalHookCountsByEvent(c *Client) (map[string]int, error) {
	counts := make(map[string]int, len(managedEvents))
	for _, me := range managedEvents {
		raw, err := c.ShowGlobalHooksForEvent(me.event)
		if err != nil {
			return nil, fmt.Errorf("show-hooks failed on %s: %w", me.event, err)
		}
		n := 0
		for _, entry := range parseEventEntries(raw, me.event) {
			if containsAny(entry.Command, me.fingerprints) {
				n++
			}
		}
		counts[me.event] = n
	}
	return counts, nil
}
