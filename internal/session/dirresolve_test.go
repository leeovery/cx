package session_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/tmux"
)

type fakePaneReader struct {
	path     string
	err      error
	sessions []string
}

func (f *fakePaneReader) ActivePaneCurrentPath(session string) (string, error) {
	f.sessions = append(f.sessions, session)
	return f.path, f.err
}

type fakeRunner struct {
	gitRoot string
	err     error
	called  bool
	lastCmd string
	lastArg []string
}

func (r *fakeRunner) Run(name string, args ...string) (string, error) {
	r.called = true
	r.lastCmd = name
	r.lastArg = args
	if r.err != nil {
		return "", r.err
	}
	return r.gitRoot, nil
}

var _ session.PaneCurrentPathReader = (*tmux.Client)(nil)

func TestResolveSessionDir(t *testing.T) {
	t.Run("resolves the active pane current_path to a canonical git root", func(t *testing.T) {
		gitRoot := t.TempDir()
		paneCwd := filepath.Join(gitRoot, "sub", "dir")
		if err := os.MkdirAll(paneCwd, 0o755); err != nil {
			t.Fatalf("mkdir pane cwd: %v", err)
		}

		reader := &fakePaneReader{path: paneCwd}
		runner := &fakeRunner{gitRoot: gitRoot}

		dir, ok, err := session.ResolveSessionDir("my-session", reader, runner)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected ok=true for a resolvable session")
		}
		want := project.CanonicalDirKey(gitRoot)
		if dir != want {
			t.Errorf("dir = %q, want canonical %q", dir, want)
		}
	})

	t.Run("reads only the active pane exactly once", func(t *testing.T) {
		gitRoot := t.TempDir()
		reader := &fakePaneReader{path: gitRoot}
		runner := &fakeRunner{gitRoot: gitRoot}

		_, _, err := session.ResolveSessionDir("my-session", reader, runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(reader.sessions) != 1 {
			t.Fatalf("expected exactly 1 active-pane read, got %d: %v", len(reader.sessions), reader.sessions)
		}
		if reader.sessions[0] != "my-session" {
			t.Errorf("read session %q, want %q", reader.sessions[0], "my-session")
		}
	})

	// A reader error is not routine session churn — an absent session arrives as
	// an empty path, below. So every reader error is reported as an error, with
	// no class of them absorbed into the not-ok-but-nil result.
	t.Run("reports a reader failure as an error naming the session", func(t *testing.T) {
		readFailed := errors.New("no server running on /private/tmp/tmux-501/default")
		reader := &fakePaneReader{err: readFailed}
		runner := &fakeRunner{gitRoot: "/should/not/be/used"}

		dir, ok, err := session.ResolveSessionDir("gone", reader, runner)

		if err == nil {
			t.Fatal("expected an error for a failed pane read, got nil")
		}
		if !errors.Is(err, readFailed) {
			t.Errorf("error %v does not preserve the underlying read failure", err)
		}
		if !strings.Contains(err.Error(), "gone") {
			t.Errorf("error %q does not name the session", err.Error())
		}
		if ok {
			t.Error("expected ok=false for a failed pane read")
		}
		if dir != "" {
			t.Errorf("expected empty dir, got %q", dir)
		}
		if runner.called {
			t.Error("git-root resolution must not run when the pane read failed")
		}
	})

	t.Run("treats an empty path with a nil error as unresolved", func(t *testing.T) {
		// The pair *tmux.Client actually returns for a session no pane answers
		// to, and for a live pane with no readable current_path yet: tmux exits
		// 0 with an empty expansion, so there is no error to classify. A blank
		// path must also never reach ResolveGitRoot, which would os.Stat("").
		reader := &fakePaneReader{path: "", err: nil}
		runner := &fakeRunner{gitRoot: "/should/not/be/used"}

		dir, ok, err := session.ResolveSessionDir("blank", reader, runner)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected ok=false for an empty current_path")
		}
		if dir != "" {
			t.Errorf("expected empty dir, got %q", dir)
		}
		if runner.called {
			t.Error("git-root resolution must not run for an empty current_path")
		}
	})

	t.Run("canonicalises the derived directory to match stored Project.Path keying", func(t *testing.T) {
		gitRoot := t.TempDir()
		paneCwd := filepath.Join(gitRoot, "internal", "pkg")
		if err := os.MkdirAll(paneCwd, 0o755); err != nil {
			t.Fatalf("mkdir pane cwd: %v", err)
		}
		reader := &fakePaneReader{path: paneCwd}
		runner := &fakeRunner{gitRoot: gitRoot}

		dir, ok, err := session.ResolveSessionDir("my-session", reader, runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected ok=true")
		}

		// The output must equal the key the project store derives for the stored
		// path, or the lookup misses.
		stored := []project.Project{{Path: gitRoot, Name: "proj"}}
		matched := false
		want := project.CanonicalDirKey(dir)
		for _, p := range stored {
			if project.CanonicalDirKey(p.Path) == want {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("derived dir %q did not match stored Project.Path %q via canonical key", dir, gitRoot)
		}
	})

	t.Run("a non-repo pane still yields its real cwd directory", func(t *testing.T) {
		cwd := t.TempDir()
		reader := &fakePaneReader{path: cwd}
		runner := &fakeRunner{err: errors.New("not a git repository")}

		dir, ok, err := session.ResolveSessionDir("my-session", reader, runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("a real cwd must resolve to ok=true even outside a git repo")
		}
		if dir != project.CanonicalDirKey(cwd) {
			t.Errorf("dir = %q, want canonical cwd %q", dir, project.CanonicalDirKey(cwd))
		}
	})
}
