package spawn

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/leeovery/portal/internal/log"
)

func runArgvCombined(argv []string) (out string, exitCode int, err error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	combined, runErr := log.CombinedOutputWithContext(cmd)
	if runErr == nil {
		return string(combined), 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return combineOutput(combined, runErr), exitErr.ExitCode(), nil
	}
	return string(combined), 0, runErr
}

// CombinedOutputWithContext keeps stderr inside the wrapped error, so the runner
// seam's out = stdout+stderr contract needs both folded together here.
func combineOutput(stdout []byte, wrapErr error) string {
	parts := make([]string, 0, 2)
	if s := strings.TrimSpace(string(stdout)); s != "" {
		parts = append(parts, s)
	}
	if wrapErr != nil {
		parts = append(parts, wrapErr.Error())
	}
	return strings.Join(parts, "\n")
}

// fallbackLabel is a "%d" format string for the exit code, used so Detail is
// never empty.
func execFailureDetail(out string, exitCode int, err error, fallbackLabel string) string {
	detail := strings.TrimSpace(out)
	if err != nil {
		if detail == "" {
			return err.Error()
		}
		return detail + ": " + err.Error()
	}
	if detail == "" {
		return fmt.Sprintf(fallbackLabel, exitCode)
	}
	return detail
}
