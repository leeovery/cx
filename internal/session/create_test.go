package session_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/leeovery/portal/internal/session"
)

func TestBuildShellCommand(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		shell   string
		want    string
	}{
		{
			name:    "single word command uses SHELL env var",
			command: []string{"claude"},
			shell:   "/bin/zsh",
			want:    `'/bin/zsh' -ic 'claude; exec '\''/bin/zsh'\'''`,
		},
		{
			name:    "multi-word command joined with spaces",
			command: []string{"claude", "--resume", "--model", "opus"},
			shell:   "/bin/zsh",
			want:    `'/bin/zsh' -ic 'claude --resume --model opus; exec '\''/bin/zsh'\'''`,
		},
		{
			name:    "uses bash when SHELL is bash",
			command: []string{"vim"},
			shell:   "/bin/bash",
			want:    `'/bin/bash' -ic 'vim; exec '\''/bin/bash'\'''`,
		},
		{
			name:    "single quotes in command are escaped",
			command: []string{"echo", "'hello'"},
			shell:   "/bin/zsh",
			want:    `'/bin/zsh' -ic 'echo '\''hello'\''; exec '\''/bin/zsh'\'''`,
		},
		{
			name:    "special shell chars passed through",
			command: []string{"ls", "|", "grep", "foo", "&&", "echo", "done"},
			shell:   "/bin/zsh",
			want:    `'/bin/zsh' -ic 'ls | grep foo && echo done; exec '\''/bin/zsh'\'''`,
		},
		{
			name:    "returns empty string for nil command",
			command: nil,
			shell:   "/bin/zsh",
			want:    "",
		},
		{
			name:    "it returns the empty string for an empty command",
			command: []string{},
			shell:   "/bin/zsh",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := session.BuildShellCommand(tt.command, tt.shell)
			if got != tt.want {
				t.Errorf("BuildShellCommand() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("it quotes a shell path containing a space so it survives word-splitting", func(t *testing.T) {
		shell := stageFakeShell(t, filepath.Join(t.TempDir(), "My Apps"), "zsh")

		assertComposedRunsShell(t, shell)
	})

	t.Run("it quotes a shell path containing a single quote", func(t *testing.T) {
		shell := stageFakeShell(t, t.TempDir(), "it's-zsh")

		assertComposedRunsShell(t, shell)
	})

	t.Run("it still renders a metacharacter-free shell path as a working command", func(t *testing.T) {
		shell := stageFakeShell(t, t.TempDir(), "zsh")

		assertComposedRunsShell(t, shell)
	})
}

// stageFakeShell writes an executable stand-in for a login shell at dir/name
// and returns its path. Handed arguments it reports its own $0 and the flag it
// was given, then runs the script; handed none — as the composed command's
// `exec <shell>` tail does — it reports the re-exec and stops, so the tail can
// be observed without recursing.
func stageFakeShell(t *testing.T, dir, name string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create fake shell dir %q: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	const body = `#!/bin/sh
if [ "$#" -eq 0 ]; then
	printf 'reexec=%s\n' "$0"
	exit 0
fi
printf 'shell=%s\n' "$0"
printf 'flag=%s\n' "$1"
exec /bin/sh -c "$2"
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("failed to write fake shell %q: %v", path, err)
	}
	return path
}

// assertComposedRunsShell hands the composed command to a real shell, which
// word-splits it, and asserts that shell invoked the whole path — both as the
// leading word and as the `exec <shell>` tail — rather than a fragment of it.
func assertComposedRunsShell(t *testing.T, shell string) {
	t.Helper()

	composed := session.BuildShellCommand([]string{"echo", "cmd-ran"}, shell)

	var stdout, stderr bytes.Buffer
	run := exec.Command("/bin/sh", "-c", composed)
	run.Stdout = &stdout
	run.Stderr = &stderr
	if err := run.Run(); err != nil {
		t.Fatalf("running %q failed: %v (stderr: %q)", composed, err, stderr.String())
	}

	want := fmt.Sprintf("shell=%s\nflag=-ic\ncmd-ran\nreexec=%s\n", shell, shell)
	if stdout.String() != want {
		t.Errorf("running %q produced %q, want %q", composed, stdout.String(), want)
	}
}

func TestShellFromEnv(t *testing.T) {
	t.Run("returns SHELL env var when set", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/zsh")
		got := session.ShellFromEnv()
		if got != "/bin/zsh" {
			t.Errorf("ShellFromEnv() = %q, want %q", got, "/bin/zsh")
		}
	})

	t.Run("falls back to /bin/sh when SHELL not set", func(t *testing.T) {
		t.Setenv("SHELL", "")
		got := session.ShellFromEnv()
		if got != "/bin/sh" {
			t.Errorf("ShellFromEnv() = %q, want %q", got, "/bin/sh")
		}
	})
}

type mockGitResolver struct {
	resolvedDir string
	err         error
}

func (m *mockGitResolver) Resolve(dir string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.resolvedDir != "" {
		return m.resolvedDir, nil
	}
	return dir, nil
}

type mockProjectStore struct {
	upsertPath  string
	upsertName  string
	upsertVia   string
	upsertCount int
	upsertErr   error
}

func (m *mockProjectStore) Upsert(path, name, via string) error {
	m.upsertPath = path
	m.upsertName = name
	m.upsertVia = via
	m.upsertCount++
	return m.upsertErr
}

type setOptionCall struct {
	Session string
	Name    string
	Value   string
}

type mockTmuxClient struct {
	existingSessions   map[string]bool
	newSessionName     string
	newSessionDir      string
	newSessionShellCmd string
	newSessionErr      error

	setOptionCalls []setOptionCall
	setOptionErr   error
}

func (m *mockTmuxClient) setOptionCallFor(name string) (setOptionCall, bool) {
	for _, c := range m.setOptionCalls {
		if c.Name == name {
			return c, true
		}
	}
	return setOptionCall{}, false
}

func (m *mockTmuxClient) HasSession(name string) bool {
	return m.existingSessions[name]
}

func (m *mockTmuxClient) NewSession(name, dir, shellCommand string) error {
	m.newSessionName = name
	m.newSessionDir = dir
	m.newSessionShellCmd = shellCommand
	return m.newSessionErr
}

func (m *mockTmuxClient) SetSessionOption(session, name, value string) error {
	m.setOptionCalls = append(m.setOptionCalls, setOptionCall{
		Session: session,
		Name:    name,
		Value:   value,
	})
	return m.setOptionErr
}

func TestCreateFromDir(t *testing.T) {
	namePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+-[a-zA-Z0-9]{6}$`)

	t.Run("creates session with git-root-resolved directory", func(t *testing.T) {
		gitRoot := t.TempDir()
		subDir := filepath.Join(gitRoot, "subdir")

		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(subDir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tmuxClient.newSessionDir != gitRoot {
			t.Errorf("tmux session dir = %q, want %q", tmuxClient.newSessionDir, gitRoot)
		}
	})

	t.Run("derives project name from basename of resolved directory", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantName := filepath.Base(dir)
		if store.upsertName != wantName {
			t.Errorf("project name = %q, want %q", store.upsertName, wantName)
		}
	})

	t.Run("generates unique session name with nanoid suffix", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "x7k2m9", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		sessionName, err := creator.CreateFromDir(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !namePattern.MatchString(sessionName) {
			t.Errorf("session name %q does not match pattern {project}-{nanoid}", sessionName)
		}

		wantSuffix := "x7k2m9"
		baseName := filepath.Base(dir)
		wantName := baseName + "-" + wantSuffix
		if sessionName != wantName {
			t.Errorf("session name = %q, want %q", sessionName, wantName)
		}
	})

	t.Run("upserts project in store with resolved path and derived name", func(t *testing.T) {
		gitRoot := t.TempDir()
		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(gitRoot, nil)
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

		if store.upsertVia != "internal" {
			t.Errorf("upsert via = %q, want %q", store.upsertVia, "internal")
		}
	})

	t.Run("handles tmux server not running by creating session normally", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		sessionName, err := creator.CreateFromDir(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if sessionName == "" {
			t.Error("expected non-empty session name")
		}

		if tmuxClient.newSessionName == "" {
			t.Error("expected NewSession to be called")
		}
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		gitResolver := &mockGitResolver{err: fmt.Errorf("directory does not exist: stat /nonexistent: no such file or directory")}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir("/nonexistent/path", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when session name generation fails", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "", fmt.Errorf("random source exhausted") }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(dir, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when project upsert fails", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{upsertErr: fmt.Errorf("disk full")}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(dir, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when tmux NewSession fails", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{
			existingSessions: map[string]bool{},
			newSessionErr:    fmt.Errorf("tmux error"),
		}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(dir, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("passes shell-command to tmux when command provided", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/zsh")
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(dir, []string{"claude", "--resume"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := `'/bin/zsh' -ic 'claude --resume; exec '\''/bin/zsh'\'''`
		if tmuxClient.newSessionShellCmd != want {
			t.Errorf("shell command = %q, want %q", tmuxClient.newSessionShellCmd, want)
		}
	})

	t.Run("no shell-command passed to tmux when command is nil", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tmuxClient.newSessionShellCmd != "" {
			t.Errorf("shell command = %q, want empty", tmuxClient.newSessionShellCmd)
		}
	})

	t.Run("uses shell resolved at construction time", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/local/bin/fish")
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		t.Setenv("SHELL", "/bin/bash")

		_, err := creator.CreateFromDir(dir, []string{"vim"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := `'/usr/local/bin/fish' -ic 'vim; exec '\''/usr/local/bin/fish'\'''`
		if tmuxClient.newSessionShellCmd != want {
			t.Errorf("shell command = %q, want %q", tmuxClient.newSessionShellCmd, want)
		}
	})

	t.Run("falls back to /bin/sh when SHELL not set", func(t *testing.T) {
		t.Setenv("SHELL", "")
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(dir, []string{"vim"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := `'/bin/sh' -ic 'vim; exec '\''/bin/sh'\'''`
		if tmuxClient.newSessionShellCmd != want {
			t.Errorf("shell command = %q, want %q", tmuxClient.newSessionShellCmd, want)
		}
	})

	t.Run("stamps @portal-dir with the resolved git root after creating a session", func(t *testing.T) {
		gitRoot := t.TempDir()
		subDir := filepath.Join(gitRoot, "subdir")

		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		sessionName, err := creator.CreateFromDir(subDir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		dirCall, ok := tmuxClient.setOptionCallFor(session.PortalDirOption)
		if !ok {
			t.Fatal("expected SetSessionOption to be called for @portal-dir")
		}
		if dirCall.Session != sessionName {
			t.Errorf("stamp session = %q, want %q", dirCall.Session, sessionName)
		}
		if dirCall.Value != gitRoot {
			t.Errorf("stamp value = %q, want %q", dirCall.Value, gitRoot)
		}
	})

	t.Run("returns the session name even when SetSessionOption fails", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{
			existingSessions: map[string]bool{},
			setOptionErr:     fmt.Errorf("set-option failed"),
		}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		sessionName, err := creator.CreateFromDir(dir, nil)
		if err != nil {
			t.Fatalf("SetSessionOption failure must not fail creation, got error: %v", err)
		}

		wantName := filepath.Base(dir) + "-abc123"
		if sessionName != wantName {
			t.Errorf("session name = %q, want %q", sessionName, wantName)
		}
	})

	t.Run("stamps using the prepared resolved dir, not a re-derived path", func(t *testing.T) {
		gitRoot := t.TempDir()
		subDir := filepath.Join(gitRoot, "a", "b", "c")

		gitResolver := &mockGitResolver{resolvedDir: gitRoot}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(subDir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		dirCall, ok := tmuxClient.setOptionCallFor(session.PortalDirOption)
		if !ok {
			t.Fatal("expected SetSessionOption to be called for @portal-dir")
		}
		if dirCall.Value != gitRoot {
			t.Errorf("stamp value = %q, want resolved git root %q", dirCall.Value, gitRoot)
		}
		if dirCall.Value == subDir {
			t.Errorf("stamp value must not be the input subdir %q", subDir)
		}
	})

	t.Run("does not stamp at creation when NewSession fails", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{
			existingSessions: map[string]bool{},
			newSessionErr:    fmt.Errorf("tmux error"),
		}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		_, err := creator.CreateFromDir(dir, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if len(tmuxClient.setOptionCalls) != 0 {
			t.Error("SetSessionOption must not be called when NewSession fails")
		}
	})

	t.Run("it stamps only @portal-dir at session creation", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		sessionName, err := creator.CreateFromDir(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(tmuxClient.setOptionCalls) != 1 {
			t.Fatalf("SetSessionOption calls = %v, want exactly one @portal-dir stamp", tmuxClient.setOptionCalls)
		}
		got := tmuxClient.setOptionCalls[0]
		want := setOptionCall{Session: sessionName, Name: session.PortalDirOption, Value: dir}
		if got != want {
			t.Errorf("stamp = %+v, want %+v", got, want)
		}
	})

	t.Run("it omits the stamp when the directory is unavailable", func(t *testing.T) {
		gitResolver := &mockGitResolver{err: fmt.Errorf("directory does not exist")}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "abc123", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		if _, err := creator.CreateFromDir("/nonexistent/portal-test-dir", nil); err == nil {
			t.Fatal("expected an error for a directory that does not exist")
		}
		if len(tmuxClient.setOptionCalls) != 0 {
			t.Errorf("SetSessionOption must not be called when the directory is unavailable, got %v", tmuxClient.setOptionCalls)
		}
	})

	t.Run("it still generates a session name from the injected generator", func(t *testing.T) {
		dir := t.TempDir()
		gitResolver := &mockGitResolver{}
		store := &mockProjectStore{}
		tmuxClient := &mockTmuxClient{existingSessions: map[string]bool{}}
		gen := func() (string, error) { return "z9y8x7", nil }

		creator := session.NewSessionCreator(gitResolver, store, tmuxClient, gen)

		sessionName, err := creator.CreateFromDir(dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantName := filepath.Base(dir) + "-z9y8x7"
		if sessionName != wantName {
			t.Errorf("session name = %q, want %q", sessionName, wantName)
		}
	})
}
