package session_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/session"
)

func TestPrepareSession(t *testing.T) {
	t.Run("resolves directory to git root", func(t *testing.T) {
		gitRoot := t.TempDir()
		subDir := filepath.Join(gitRoot, "subdir")

		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		result, err := session.PrepareSession(subDir, nil, gitResolver, store, checker, gen, "/bin/zsh")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ResolvedDir != gitRoot {
			t.Errorf("ResolvedDir = %q, want %q", result.ResolvedDir, gitRoot)
		}
	})

	t.Run("derives project name from basename of resolved directory", func(t *testing.T) {
		gitRoot := "/tmp/myproject"
		subDir := "/tmp/myproject/src/pkg"

		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		result, err := session.PrepareSession(subDir, nil, gitResolver, store, checker, gen, "/bin/zsh")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ProjectName != "myproject" {
			t.Errorf("ProjectName = %q, want %q", result.ProjectName, "myproject")
		}
	})

	t.Run("generates session name with project-nanoid format", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "x7k2m9", nil }

		result, err := session.PrepareSession(dir, nil, gitResolver, store, checker, gen, "/bin/zsh")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantName := filepath.Base(dir) + "-x7k2m9"
		if result.SessionName != wantName {
			t.Errorf("SessionName = %q, want %q", result.SessionName, wantName)
		}
	})

	t.Run("upserts project in store with resolved path and derived name", func(t *testing.T) {
		gitRoot := t.TempDir()
		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		_, err := session.PrepareSession(gitRoot, nil, gitResolver, store, checker, gen, "/bin/zsh")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if store.upsertPath != gitRoot {
			t.Errorf("upsert path = %q, want %q", store.upsertPath, gitRoot)
		}

		wantName := filepath.Base(gitRoot)
		if store.upsertName != wantName {
			t.Errorf("upsert name = %q, want %q", store.upsertName, wantName)
		}
	})

	t.Run("builds shell command when command provided", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		result, err := session.PrepareSession(dir, []string{"claude", "--resume"}, gitResolver, store, checker, gen, "/bin/zsh")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := "/bin/zsh -ic 'claude --resume; exec /bin/zsh'"
		if result.ShellCmd != want {
			t.Errorf("ShellCmd = %q, want %q", result.ShellCmd, want)
		}
	})

	t.Run("shell command empty when command is nil", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		result, err := session.PrepareSession(dir, nil, gitResolver, store, checker, gen, "/bin/zsh")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ShellCmd != "" {
			t.Errorf("ShellCmd = %q, want empty", result.ShellCmd)
		}
	})

	t.Run("returns error when git resolution fails", func(t *testing.T) {
		gitResolver := &mockGitResolver{err: fmt.Errorf("git error")}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		_, err := session.PrepareSession("/some/path", nil, gitResolver, store, checker, gen, "/bin/zsh")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when session name generation fails", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "", fmt.Errorf("random source exhausted") }

		_, err := session.PrepareSession(dir, nil, gitResolver, store, checker, gen, "/bin/zsh")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when project upsert fails", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{upsertErr: fmt.Errorf("disk full")}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		_, err := session.PrepareSession(dir, nil, gitResolver, store, checker, gen, "/bin/zsh")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
