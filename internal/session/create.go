package session

import (
	"fmt"
	"os"
	"strings"

	"github.com/leeovery/portal/internal/shellquote"
)

// PortalDirOption stamps a session with its resolved directory, so a live
// session maps back to its directory without going through its renameable name.
const PortalDirOption = "@portal-dir"

func ShellFromEnv() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "/bin/sh"
	}
	return shell
}

// BuildShellCommand renders command as a tmux shell-command of the form
// $SHELL -ic '<joined>; exec $SHELL', returning "" for an empty command. A
// single quote inside command is re-quoted so the outer quoting survives.
func BuildShellCommand(command []string, shell string) string {
	if len(command) == 0 {
		return ""
	}
	script := strings.Join(command, " ") + "; exec " + shell
	return fmt.Sprintf("%s -ic %s", shell, shellquote.Single(script))
}

type GitResolver interface {
	Resolve(dir string) (string, error)
}

type ProjectStore interface {
	Upsert(path, name, via string) error
}

type TmuxClient interface {
	HasSession(name string) bool
	NewSession(name, dir, shellCommand string) error
	SetSessionOption(session, name, value string) error
}

type SessionCreator struct {
	git   GitResolver
	store ProjectStore
	tmux  TmuxClient
	gen   IDGenerator
	shell string
}

// NewSessionCreator resolves the user's shell from $SHELL at construction time.
func NewSessionCreator(git GitResolver, store ProjectStore, tmux TmuxClient, gen IDGenerator) *SessionCreator {
	return &SessionCreator{
		git:   git,
		store: store,
		tmux:  tmux,
		gen:   gen,
		shell: ShellFromEnv(),
	}
}

// CreateFromDir creates a session for dir and returns its generated name. A
// non-empty command becomes the session's initial shell-command.
func (sc *SessionCreator) CreateFromDir(dir string, command []string) (string, error) {
	prepared, err := PrepareSession(dir, command, sc.git, sc.store, sc.tmux, sc.gen, sc.shell)
	if err != nil {
		return "", err
	}

	if err := sc.tmux.NewSession(prepared.SessionName, prepared.ResolvedDir, prepared.ShellCmd); err != nil {
		return "", fmt.Errorf("failed to create tmux session: %w", err)
	}

	// The stamp is best-effort and swallowed: a stamp failure must never fail
	// session creation, and this package has no log component to report it. An
	// unstamped session falls back to a derived directory.
	_ = sc.tmux.SetSessionOption(prepared.SessionName, PortalDirOption, prepared.ResolvedDir)

	return prepared.SessionName, nil
}
