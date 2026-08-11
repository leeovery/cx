package session

import (
	"fmt"
	"path/filepath"
)

// PreparedSession is the intermediate result of the shared session-preparation
// pipeline, from which each caller performs its own final step.
type PreparedSession struct {
	ResolvedDir string
	ProjectName string
	SessionName string
	ShellCmd    string
}

// PrepareSession resolves the git root, derives the project and session names,
// registers the project and builds the shell command.
func PrepareSession(
	path string,
	command []string,
	git GitResolver,
	store ProjectStore,
	checker SessionChecker,
	gen IDGenerator,
	shell string,
) (*PreparedSession, error) {
	resolvedDir, err := git.Resolve(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve directory: %w", err)
	}

	projectName := filepath.Base(resolvedDir)

	exists := func(name string) bool {
		return checker.HasSession(name)
	}

	sessionName, err := GenerateSessionName(projectName, gen, exists)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session name: %w", err)
	}

	if err := store.Upsert(resolvedDir, projectName, "internal"); err != nil {
		return nil, fmt.Errorf("failed to upsert project: %w", err)
	}

	shellCmd := BuildShellCommand(command, shell)

	return &PreparedSession{
		ResolvedDir: resolvedDir,
		ProjectName: projectName,
		SessionName: sessionName,
		ShellCmd:    shellCmd,
	}, nil
}
