package session

import "github.com/leeovery/portal/internal/tmux"

type SessionChecker interface {
	HasSession(name string) bool
}

type QuickStartResult struct {
	SessionName string
	Dir         string
	ExecArgs    []string
}

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
// Order is load-bearing: the session is created detached so @portal-dir can be
// stamped before attach-session blocks the chain. The name is unique upstream,
// so plain new-session always creates — -A would attach to an existing session
// immediately and skip the stamp.
//
// The two later steps pin their targets rather than naming the session bare.
// Should the create not produce the session, a bare name prefix-matches onto a
// live sibling and the stamp lands on a stranger; that the tmux in hand
// abandons a ";" chain at its first failure is one version's behaviour, not
// something a write to someone else's session may rest on. Both steps take a
// target tmux parses as a window or pane — set-option resolves a pane target to
// reach a session option, and attach-session resolves the session's current
// window through the same parse — so both take CoordTargetExact, whose trailing
// separator is what keeps a period in the name from being read as a window and
// pane.
func (qs *QuickStart) Run(path string, command []string) (*QuickStartResult, error) {
	prepared, err := PrepareSession(path, command, qs.git, qs.store, qs.checker, qs.gen, qs.shell)
	if err != nil {
		return nil, err
	}

	execArgs := []string{"tmux", "new-session", "-d", "-s", prepared.SessionName, "-c", prepared.ResolvedDir}
	if prepared.ShellCmd != "" {
		execArgs = append(execArgs, prepared.ShellCmd)
	}
	// Spent as a string because the chain is an argv the tmux client does not
	// run; the pinning is the constructor's, not this slice's.
	target := string(tmux.CoordTargetExact(prepared.SessionName))
	execArgs = append(execArgs,
		";", "set-option", "-t", target, PortalDirOption, prepared.ResolvedDir,
		";", "attach-session", "-t", target,
	)

	return &QuickStartResult{
		SessionName: prepared.SessionName,
		Dir:         prepared.ResolvedDir,
		ExecArgs:    execArgs,
	}, nil
}
