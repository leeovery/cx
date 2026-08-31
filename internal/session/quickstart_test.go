package session_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/leeovery/portal/internal/session"
)

type mockSessionChecker struct {
	existingSessions map[string]bool
}

func (m *mockSessionChecker) HasSession(name string) bool {
	return m.existingSessions[name]
}

func wantExecArgs(name, dir, shellCmd string) []string {
	args := []string{"tmux", "new-session", "-d", "-s", name, "-c", dir}
	if shellCmd != "" {
		args = append(args, shellCmd)
	}
	return append(args,
		";", "set-option", "-t", "="+name+":", session.PortalDirOption, dir,
		";", "attach-session", "-t", "="+name,
	)
}

func TestQuickStart(t *testing.T) {
	namePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+-[a-zA-Z0-9]{6}$`)

	t.Run("creates session with git-root-resolved directory", func(t *testing.T) {
		gitRoot := t.TempDir()
		subDir := filepath.Join(gitRoot, "subdir")

		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		result, err := qs.Run(subDir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Dir != gitRoot {
			t.Errorf("result.Dir = %q, want %q", result.Dir, gitRoot)
		}

		wantSessionName := filepath.Base(gitRoot) + "-abc123"
		wantArgs := wantExecArgs(wantSessionName, gitRoot, "")
		if !reflect.DeepEqual(result.ExecArgs, wantArgs) {
			t.Fatalf("result.ExecArgs = %v, want %v", result.ExecArgs, wantArgs)
		}
	})

	t.Run("stamps @portal-dir at creation via the exec chain", func(t *testing.T) {
		gitRoot := t.TempDir()
		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		result, err := qs.Run(gitRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantSessionName := filepath.Base(gitRoot) + "-abc123"
		assertContainsSubseq(t, result.ExecArgs, []string{
			"set-option", "-t", "=" + wantSessionName + ":", session.PortalDirOption, gitRoot,
		})
		setIdx := indexOf(result.ExecArgs, "set-option")
		attachIdx := indexOf(result.ExecArgs, "attach-session")
		if setIdx < 0 || attachIdx < 0 || setIdx >= attachIdx {
			t.Errorf("set-option (%d) must precede attach-session (%d) in %v", setIdx, attachIdx, result.ExecArgs)
		}
		if indexOf(result.ExecArgs, "-A") >= 0 {
			t.Errorf("ExecArgs must not use new-session -A: %v", result.ExecArgs)
		}
	})

	t.Run("it emits no legacy session-id link in the QuickStart ExecArgs", func(t *testing.T) {
		gitRoot := t.TempDir()
		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		result, err := qs.Run(gitRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		name := filepath.Base(gitRoot) + "-abc123"
		want := []string{
			"tmux", "new-session", "-d", "-s", name, "-c", gitRoot,
			";", "set-option", "-t", "=" + name + ":", session.PortalDirOption, gitRoot,
			";", "attach-session", "-t", "=" + name,
		}
		if !reflect.DeepEqual(result.ExecArgs, want) {
			t.Fatalf("result.ExecArgs = %v, want %v", result.ExecArgs, want)
		}
	})

	t.Run("it keeps @portal-dir before attach-session in the ExecArgs chain", func(t *testing.T) {
		gitRoot := t.TempDir()
		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		result, err := qs.Run(gitRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		dirIdx := indexOfSubseq(result.ExecArgs, []string{session.PortalDirOption})
		attachIdx := indexOf(result.ExecArgs, "attach-session")
		if dirIdx < 0 || attachIdx < 0 || dirIdx >= attachIdx {
			t.Errorf("@portal-dir stamp (%d) must precede attach-session (%d) in %v", dirIdx, attachIdx, result.ExecArgs)
		}
	})

	t.Run("does not use new-session -A", func(t *testing.T) {
		gitRoot := t.TempDir()
		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		result, err := qs.Run(gitRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if indexOf(result.ExecArgs, "-A") >= 0 {
			t.Errorf("ExecArgs must not use new-session -A: %v", result.ExecArgs)
		}
	})

	t.Run("registers new project in store", func(t *testing.T) {
		gitRoot := t.TempDir()
		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		_, err := qs.Run(gitRoot, nil)
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

	t.Run("updates last_used for existing project", func(t *testing.T) {
		gitRoot := t.TempDir()
		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		_, err := qs.Run(gitRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error on first run: %v", err)
		}

		_, err = qs.Run(gitRoot, nil)
		if err != nil {
			t.Fatalf("unexpected error on second run: %v", err)
		}

		if store.upsertCount != 2 {
			t.Errorf("upsert count = %d, want 2", store.upsertCount)
		}

		if store.upsertPath != gitRoot {
			t.Errorf("upsert path = %q, want %q", store.upsertPath, gitRoot)
		}
	})

	t.Run("exec args create detached, stamp, then attach", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		result, err := qs.Run(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantSessionName := filepath.Base(dir) + "-abc123"

		if result.SessionName != wantSessionName {
			t.Errorf("result.SessionName = %q, want %q", result.SessionName, wantSessionName)
		}

		wantArgs := wantExecArgs(wantSessionName, dir, "")
		if !reflect.DeepEqual(result.ExecArgs, wantArgs) {
			t.Fatalf("result.ExecArgs = %v, want %v", result.ExecArgs, wantArgs)
		}
	})

	t.Run("session name follows project-nanoid format", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "x7k2m9", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		result, err := qs.Run(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !namePattern.MatchString(result.SessionName) {
			t.Errorf("session name %q does not match pattern {project}-{nanoid}", result.SessionName)
		}

		wantName := filepath.Base(dir) + "-x7k2m9"
		if result.SessionName != wantName {
			t.Errorf("session name = %q, want %q", result.SessionName, wantName)
		}
	})

	t.Run("project name derived from directory basename after git root resolution", func(t *testing.T) {
		gitRoot := "/tmp/myproject"
		subDir := "/tmp/myproject/src/pkg"

		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		_, err := qs.Run(subDir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if store.upsertName != "myproject" {
			t.Errorf("project name = %q, want %q", store.upsertName, "myproject")
		}
	})

	t.Run("exec args include shell-command when command provided", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/zsh")
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		result, err := qs.Run(dir, []string{"claude", "--resume"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantSessionName := filepath.Base(dir) + "-abc123"
		shellCmd := "/bin/zsh -ic 'claude --resume; exec /bin/zsh'"
		wantArgs := wantExecArgs(wantSessionName, dir, shellCmd)
		if !reflect.DeepEqual(result.ExecArgs, wantArgs) {
			t.Fatalf("result.ExecArgs = %v, want %v", result.ExecArgs, wantArgs)
		}
	})

	t.Run("uses shell resolved at construction time", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/local/bin/fish")
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		t.Setenv("SHELL", "/bin/bash")

		result, err := qs.Run(dir, []string{"vim"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantSessionName := filepath.Base(dir) + "-abc123"
		shellCmd := "/usr/local/bin/fish -ic 'vim; exec /usr/local/bin/fish'"
		wantArgs := wantExecArgs(wantSessionName, dir, shellCmd)
		if !reflect.DeepEqual(result.ExecArgs, wantArgs) {
			t.Fatalf("result.ExecArgs = %v, want %v", result.ExecArgs, wantArgs)
		}
	})

	t.Run("no shell-command in exec args when command is nil", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		result, err := qs.Run(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantSessionName := filepath.Base(dir) + "-abc123"
		wantArgs := wantExecArgs(wantSessionName, dir, "")
		if !reflect.DeepEqual(result.ExecArgs, wantArgs) {
			t.Fatalf("result.ExecArgs = %v, want %v", result.ExecArgs, wantArgs)
		}
	})

	t.Run("returns error when git resolution fails", func(t *testing.T) {
		gitResolver := &mockGitResolver{err: fmt.Errorf("git error")}
		store := &mockProjectStore{}
		checker := &mockSessionChecker{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		qs := session.NewQuickStart(gitResolver, store, checker, gen)

		_, err := qs.Run("/some/path", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func indexOfSubseq(got, want []string) int {
	for i := 0; i+len(want) <= len(got); i++ {
		if reflect.DeepEqual(got[i:i+len(want)], want) {
			return i
		}
	}
	return -1
}

func assertContainsSubseq(t *testing.T, got, want []string) {
	t.Helper()
	for i := 0; i+len(want) <= len(got); i++ {
		if reflect.DeepEqual(got[i:i+len(want)], want) {
			return
		}
	}
	t.Errorf("ExecArgs %v does not contain subsequence %v", got, want)
}
