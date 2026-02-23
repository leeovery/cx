package session

import (
	"fmt"
	"path/filepath"
)

// GitResolver resolves a directory to its git repository root.
type GitResolver interface {
	Resolve(dir string) (string, error)
}

// ProjectStore persists project data.
type ProjectStore interface {
	Upsert(path, name string) error
}

// TmuxClient provides tmux session operations.
type TmuxClient interface {
	HasSession(name string) bool
	NewSession(name, dir string) error
}

// SessionCreator orchestrates the creation of a new tmux session from a directory.
type SessionCreator struct {
	git   GitResolver
	store ProjectStore
	tmux  TmuxClient
	gen   IDGenerator
}

// NewSessionCreator creates a SessionCreator with the given dependencies.
func NewSessionCreator(git GitResolver, store ProjectStore, tmux TmuxClient, gen IDGenerator) *SessionCreator {
	return &SessionCreator{
		git:   git,
		store: store,
		tmux:  tmux,
		gen:   gen,
	}
}

// CreateFromDir resolves the directory to a git root, generates a session name,
// upserts the project in the store, and creates a tmux session.
// Returns the generated session name.
func (sc *SessionCreator) CreateFromDir(dir string) (string, error) {
	resolvedDir, err := sc.git.Resolve(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve directory: %w", err)
	}

	projectName := filepath.Base(resolvedDir)

	exists := func(name string) bool {
		return sc.tmux.HasSession(name)
	}

	sessionName, err := GenerateSessionName(projectName, sc.gen, exists)
	if err != nil {
		return "", fmt.Errorf("failed to generate session name: %w", err)
	}

	if err := sc.store.Upsert(resolvedDir, projectName); err != nil {
		return "", fmt.Errorf("failed to upsert project: %w", err)
	}

	if err := sc.tmux.NewSession(sessionName, resolvedDir); err != nil {
		return "", fmt.Errorf("failed to create tmux session: %w", err)
	}

	return sessionName, nil
}
