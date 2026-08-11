package portaltest

import (
	"github.com/leeovery/portal/internal/state"
)

// PgrepPortalDaemons forwards to the primitive the production sweep uses, so a
// test observes the same candidate set.
func PgrepPortalDaemons() ([]int, error) {
	return state.PgrepPortalDaemons()
}
