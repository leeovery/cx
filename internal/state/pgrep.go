package state

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/leeovery/portal/internal/log"
)

var pgrepCommand = defaultPgrepCommand

func defaultPgrepCommand() *exec.Cmd {
	return exec.Command("pgrep", "-fx", PortalDaemonArgvPattern)
}

// PgrepPortalDaemons enumerates the pids of live `portal state daemon`
// processes. A pgrep exit status of 1 with empty stdout is its "no matches"
// signal and returns (nil, nil); any other failure returns a wrapped error.
//
// `-fx` rather than `-fxc`: BSD pgrep has no `-c`.
func PgrepPortalDaemons() ([]int, error) {
	out, err := log.CombinedOutputWithContext(pgrepCommand())
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && len(strings.TrimSpace(string(out))) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("pgrep -fx %q: %w", PortalDaemonArgvPattern, err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		// pgrep documents exit 1 for no matches; this shape is a defensive guard.
		return nil, nil
	}

	var pids []int
	for line := range strings.SplitSeq(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, parseErr := strconv.Atoi(line)
		if parseErr != nil {
			continue
		}
		pids = append(pids, pid)
	}
	// Identity in production; under -tags integration this drops every pid the
	// running test did not register, so a sweep cannot reach the real daemon.
	return sandboxFilterPgrep(pids), nil
}
