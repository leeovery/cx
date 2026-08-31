package session_test

import (
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/session"
)

// The chain's two per-session steps take different target kinds: set-option
// resolves a pane target to reach a session option, so it needs the trailing
// separator, while attach-session takes a session target and rejects one.
func TestQuickStartTargetsAreExact(t *testing.T) {
	t.Run("it pins the quickstart set-option target to the exact session", func(t *testing.T) {
		result, name := runQuickStart(t)

		assertContainsSubseq(t, result.ExecArgs, []string{"set-option", "-t", "=" + name + ":"})
	})

	t.Run("it pins the quickstart attach-session target to the exact session", func(t *testing.T) {
		result, name := runQuickStart(t)

		assertContainsSubseq(t, result.ExecArgs, []string{"attach-session", "-t", "=" + name})
	})
}

// runQuickStart runs a QuickStart over a temp git root with a fixed id, and
// returns the result alongside the session name it generated.
func runQuickStart(t *testing.T) (*session.QuickStartResult, string) {
	t.Helper()
	gitRoot := t.TempDir()
	qs := session.NewQuickStart(
		&mockGitResolver{resolvedDir: gitRoot},
		&mockProjectStore{},
		&mockSessionChecker{existingSessions: map[string]bool{}},
		func() (string, error) { return "abc123", nil },
	)

	result, err := qs.Run(gitRoot, nil)
	if err != nil {
		t.Fatalf("QuickStart.Run: %v", err)
	}
	return result, filepath.Base(gitRoot) + "-abc123"
}
