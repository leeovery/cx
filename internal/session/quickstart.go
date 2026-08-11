package session

// SessionChecker reports whether a tmux session exists by name.
type SessionChecker interface {
	HasSession(name string) bool
}

// QuickStartResult carries the created session's name and resolved directory,
// plus the argv for the syscall.Exec handoff to tmux.
type QuickStartResult struct {
	SessionName string
	Dir         string
	ExecArgs    []string
}

// QuickStart orchestrates the quick-start session creation pipeline: git root
// resolution, project registration and session naming.
type QuickStart struct {
	git     GitResolver
	store   ProjectStore
	checker SessionChecker
	gen     IDGenerator
	shell   string
}

// NewQuickStart resolves the user's shell from $SHELL at construction time.
func NewQuickStart(git GitResolver, store ProjectStore, checker SessionChecker, gen IDGenerator) *QuickStart {
	return &QuickStart{
		git:     git,
		store:   store,
		checker: checker,
		gen:     gen,
		shell:   ShellFromEnv(),
	}
}

// Run resolves the git root, registers the project and generates a session name,
// returning the argv for a single chained tmux invocation with literal ";"
// separators. When command is non-empty it is appended to the new-session step.
//
// Order is load-bearing: the session is created detached so @portal-dir and
// @portal-id can be stamped before attach-session blocks the chain. Anchoring
// @portal-dir at creation keeps grouping stable once the pane cd's elsewhere.
// The name is guaranteed unique upstream, so plain new-session always creates —
// -A would attach to an existing session immediately and skip the stamps.
func (qs *QuickStart) Run(path string, command []string) (*QuickStartResult, error) {
	prepared, err := PrepareSession(path, command, qs.git, qs.store, qs.checker, qs.gen, qs.shell)
	if err != nil {
		return nil, err
	}

	// Generated here because the argv chain has no error-return point: on failure
	// the stamp step is simply omitted and the session keeps its name as identity.
	idToken, idGenErr := qs.gen()

	execArgs := []string{"tmux", "new-session", "-d", "-s", prepared.SessionName, "-c", prepared.ResolvedDir}
	if prepared.ShellCmd != "" {
		execArgs = append(execArgs, prepared.ShellCmd)
	}
	execArgs = append(execArgs,
		";", "set-option", "-t", prepared.SessionName, PortalDirOption, prepared.ResolvedDir,
	)
	if idGenErr == nil && idToken != "" {
		execArgs = append(execArgs,
			";", "set-option", "-t", prepared.SessionName, PortalIDOption, idToken,
		)
	}
	execArgs = append(execArgs,
		";", "attach-session", "-t", prepared.SessionName,
	)

	return &QuickStartResult{
		SessionName: prepared.SessionName,
		Dir:         prepared.ResolvedDir,
		ExecArgs:    execArgs,
	}, nil
}
