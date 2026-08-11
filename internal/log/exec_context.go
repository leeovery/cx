package log

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CombinedOutputWithContext runs cmd and returns its stdout. On failure it
// returns the captured stdout together with an error embedding the binary path,
// the argv, the underlying exit error (wrapped with %w, so errors.As against
// *exec.ExitError or *exec.Error still works) and the child's trimmed stderr.
//
// Despite the name it does not merge the two streams: stderr is captured
// privately and only ever appears inside the error. Returning stdout verbatim on
// the error path is load-bearing — callers discriminate on stdout emptiness after
// a non-zero exit.
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
