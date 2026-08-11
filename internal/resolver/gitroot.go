package resolver

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/leeovery/portal/internal/log"
)

type CommandRunner interface {
	Run(name string, args ...string) (string, error)
}

type RealCommandRunner struct{}

// Run executes a command; on failure the error embeds the binary path, argv, exit
// status and the child's trimmed stderr.
func (r *RealCommandRunner) Run(name string, args ...string) (string, error) {
	out, err := log.CombinedOutputWithContext(exec.Command(name, args...))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ResolveGitRoot returns dir's git repository root, or dir unchanged when it is
// not in a repository or git is unavailable. A missing directory is an error.
func ResolveGitRoot(dir string, runner CommandRunner) (string, error) {
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("directory does not exist: %w", err)
	}

	output, err := runner.Run("git", "-C", dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return dir, nil
	}

	return strings.TrimSpace(output), nil
}
