package session

import (
	"fmt"
	"os"
	"strings"
)

// PortalDirOption stamps a session with its resolved directory, so a live
// session maps back to its directory without going through its renameable name.
const PortalDirOption = "@portal-dir"

// PortalIDOption stamps a session with an immutable opaque token frozen at
// creation — the identity resume hooks key on, so a rename cannot orphan them.
// Unlike a directory it cannot be re-derived, so it must be persisted across
// reboots and re-stamped on restore.
//
// The literal must stay byte-identical to the "@portal-id" embedded in
// tmux.HookKeyFormat, or key-producing sites disagree.
const PortalIDOption = "@portal-id"

// ShellFromEnv returns the user's shell from $SHELL, falling back to /bin/sh.
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
	joined := strings.Join(command, " ")
	escaped := strings.ReplaceAll(joined, "'", "'\\''")
	return fmt.Sprintf("%s -ic '%s; exec %s'", shell, escaped, shell)
}

// GitResolver resolves a directory to its git repository root.
type GitResolver interface {
	Resolve(dir string) (string, error)
}

// ProjectStore persists project data.
type ProjectStore interface {
	Upsert(path, name, via string) error
}

// TmuxClient provides tmux session operations.
type TmuxClient interface {
	HasSession(name string) bool
	NewSession(name, dir, shellCommand string) error
	SetSessionOption(session, name, value string) error
}

// SessionCreator creates a new tmux session from a directory.
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

// CreateFromDir resolves dir to a git root, registers the project, creates the
// tmux session and returns its generated name. A non-empty command becomes the
// session's initial shell-command.
func (sc *SessionCreator) CreateFromDir(dir string, command []string) (string, error) {
	prepared, err := PrepareSession(dir, command, sc.git, sc.store, sc.tmux, sc.gen, sc.shell)
	if err != nil {
		return "", err
	}

	if err := sc.tmux.NewSession(prepared.SessionName, prepared.ResolvedDir, prepared.ShellCmd); err != nil {
		return "", fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Both stamps are best-effort and swallowed: a stamp failure must never fail
	// session creation, and this package has no log component to report it. An
	// unstamped session falls back to a derived directory and to its name as
	// identity. The id is fire-and-forget with no uniqueness check — collision
	// resistance rests entirely on the generator's width.
	_ = sc.tmux.SetSessionOption(prepared.SessionName, PortalDirOption, prepared.ResolvedDir)

	if token, genErr := sc.gen(); genErr == nil {
		_ = sc.tmux.SetSessionOption(prepared.SessionName, PortalIDOption, token)
	}

	return prepared.SessionName, nil
}
