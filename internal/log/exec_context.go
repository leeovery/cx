package log

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CombinedOutputWithContext runs cmd and returns its stdout. Despite the name it
// does not merge the streams: stderr is captured privately and surfaces only
// inside the error, which wraps the exit error with %w so errors.As still
// reaches *exec.ExitError / *exec.Error. Stdout is returned verbatim on the
// error path too — callers discriminate on its emptiness after a non-zero exit.
func CombinedOutputWithContext(cmd *exec.Cmd) ([]byte, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}

	trimmed := strings.TrimSpace(stderr.String())
	if trimmed == "" {
		return out, fmt.Errorf("%s %v: %w", cmd.Path, cmd.Args[1:], err)
	}
	return out, fmt.Errorf("%s %v: %w (stderr: %s)", cmd.Path, cmd.Args[1:], err, trimmed)
}
