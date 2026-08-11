package log

import (
	"path/filepath"
	"strconv"
)

// The literal "swept" occupies the date slot, so pastDayLogDate's strict
// date-parse rejects the sentinel: it is never sealed, and never deleted by the
// retention cutoff walk. Only the not-today prune reclaims it.
const sweptPrefix = portalLogName + ".swept."

func sweptSentinelFile(stateDir, date string) string {
	return filepath.Join(stateDir, sweptPrefix+date)
}

func dayFile(stateDir, date string) string {
	return filepath.Join(stateDir, portalLogName+"."+date)
}

func daySegmentFile(stateDir, date string, n int) string {
	return filepath.Join(stateDir, portalLogName+"."+date+"."+strconv.Itoa(n))
}

func symlinkPath(stateDir string) string {
	return filepath.Join(stateDir, portalLogName)
}
