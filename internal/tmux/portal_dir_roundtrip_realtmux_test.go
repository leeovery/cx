package tmux_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const portalDirOption = "@portal-dir"

func findSessionDir(t *testing.T, sessions []tmux.Session, name string) string {
	t.Helper()
	for _, s := range sessions {
		if s.Name == name {
			return s.Dir
		}
	}
	t.Fatalf("session %q not found in ListSessions result %+v", name, sessions)
	return ""
}

func TestPortalDirStampRoundTrip(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	cases := []struct {
		name        string
		sessionName string
		stamp       string
	}{
		{
			name:        "plain path",
			sessionName: "rt-plain",
			stamp:       "/code/portal",
		},
		{
			name:        "path with a space",
			sessionName: "rt-space",
			stamp:       "/code/my project",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := tmuxtest.New(t, "portaldir-")
			client := ts.Client()

			cwd := t.TempDir()
			if err := client.NewSession(tc.sessionName, cwd, ""); err != nil {
				t.Fatalf("NewSession(%q): %v", tc.sessionName, err)
			}
			ts.WaitForSession(t, tc.sessionName, 2*time.Second)

			if err := client.SetSessionOption(tc.sessionName, portalDirOption, tc.stamp); err != nil {
				t.Fatalf("SetSessionOption(%q, %q, %q): %v", tc.sessionName, portalDirOption, tc.stamp, err)
			}

			sessions, err := client.ListSessions()
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}
			if got := findSessionDir(t, sessions, tc.sessionName); got != tc.stamp {
				t.Errorf("round-trip Dir = %q, want %q (tmux quoting/format-string drift)", got, tc.stamp)
			}
		})
	}
}

func TestPortalDirStampRoundTrip_TempDirWithSpace(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "portaldir-")
	client := ts.Client()

	dir := filepath.Join(t.TempDir(), "dir with space")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir with space: %v", err)
	}

	const sessionName = "rt-tempspace"
	if err := client.NewSession(sessionName, dir, ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	if err := client.SetSessionOption(sessionName, portalDirOption, dir); err != nil {
		t.Fatalf("SetSessionOption(%q, %q, %q): %v", sessionName, portalDirOption, dir, err)
	}

	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if got := findSessionDir(t, sessions, sessionName); got != dir {
		t.Errorf("round-trip Dir = %q, want %q (real space-containing path must survive intact)", got, dir)
	}
}

func TestPortalDirStampRoundTrip_UnstampedSessionEmptyDir(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "portaldir-")
	client := ts.Client()

	const sessionName = "rt-unstamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if got := findSessionDir(t, sessions, sessionName); got != "" {
		t.Errorf("unstamped session Dir = %q, want \"\" (absent @portal-dir must parse to empty)", got)
	}
}
