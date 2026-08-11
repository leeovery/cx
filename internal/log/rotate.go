package log

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

var rotateLogger = For("log-rotate")

// Rotated files are sealed read-only so the destruction surface is today's file
// alone.
const sealedMode os.FileMode = 0o400

var chmodFunc = os.Chmod

// sealPastDayFiles is best-effort: a chmod failure warns and the sweep continues.
//
// Today's file and today's overflow segments are deliberately left writable — a
// peer process may hold an open O_APPEND fd on one, and chmod does not evict an
// already-open writer. So is the swing temp, which must stay reclaimable. The
// next day's sweep seals all of today's segments at once, so a multi-day gap
// catches up in a single pass.
func sealPastDayFiles(stateDir, today string) {
	matches, err := filepath.Glob(filepath.Join(stateDir, portalLogName+".*"))
	if err != nil {
		return
	}

	for _, path := range matches {
		date, ok := pastDayLogDate(filepath.Base(path))
		if !ok || date == today {
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Mode().Perm() == sealedMode {
			continue
		}

		if err := chmodFunc(path, sealedMode); err != nil {
			rotateLogger.Warn("chmod failed", "error", err, "path", path)
		}
	}
}

// pastDayLogDate accepts only the strict portal.log.<YYYY-MM-DD>[.<N>] shape, so
// the swing temp, the swept sentinel and any other sibling are never candidates
// for sealing or deletion.
func pastDayLogDate(base string) (date string, ok bool) {
	const prefix = portalLogName + "."
	rest, found := strings.CutPrefix(base, prefix)
	if !found || rest == "" {
		return "", false
	}

	segments := strings.SplitN(rest, ".", 2)
	dateSeg := segments[0]
	if _, err := time.Parse(dateLayout, dateSeg); err != nil {
		return "", false
	}

	if len(segments) == 2 && !isAllDigits(segments[1]) {
		return "", false
	}

	return dateSeg, true
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
