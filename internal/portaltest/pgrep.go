package portaltest

import (
	"github.com/leeovery/portal/internal/state"
)

// PgrepPortalDaemons enumerates live `portal state daemon` PIDs, forwarding to
// the same primitive the production sweep uses so a test observes the same
// candidate set.
func PgrepPortalDaemons() ([]int, error) {
	return state.PgrepPortalDaemons()
}
