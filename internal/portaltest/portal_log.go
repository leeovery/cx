package portaltest

import (
	"fmt"
	"os"

	"github.com/leeovery/portal/internal/state"
)

// ReadPortalLogSafe returns portal.log's contents under stateDir, or a
// placeholder describing the read failure, so a caller can always embed the
// result in a failure message.
func ReadPortalLogSafe(stateDir string) string {
	data, err := os.ReadFile(state.PortalLog(stateDir))
	if err != nil {
		return fmt.Sprintf("(read portal.log failed: %v)", err)
	}
	return string(data)
}
