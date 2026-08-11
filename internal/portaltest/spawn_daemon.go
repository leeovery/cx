package portaltest

import (
	"os"
	"os/exec"
	"testing"

	"github.com/leeovery/portal/internal/state"
)

// SpawnIsolatedDaemon starts a `portal state daemon` under envSlice plus a fresh
// per-call PORTAL_STATE_DIR and TMUX pinned to tmuxSocketPath, and reaps it via
// RegisterSubprocessCleanup. tmuxSocketPath is required: a daemon inheriting the
// ambient TMUX attaches to the developer's real server and capture-panes their
// live sessions.
//
// The unqualified "portal" argv[0] is load-bearing: darwin's ps reports argv[0]
// as comm and the daemon identity check requires comm == "portal" exactly, so
// spawning by absolute path would make the daemon unidentifiable.
//
// Each call gets its own state dir so several orphans can coexist without
// colliding on daemon.lock; daemons sharing one dir must be spawned directly and
// wrapped in RegisterSubprocessCleanup.
func SpawnIsolatedDaemon(t *testing.T, envSlice []string, tmuxSocketPath string) (*exec.Cmd, string) {
	t.Helper()
	if tmuxSocketPath == "" {
		t.Fatalf("portaltest: SpawnIsolatedDaemon requires the test's tmux socket path — " +
			"an orphan without one would attach to the developer's real tmux server")
	}
	stateDir := t.TempDir()
	env := append([]string{}, envSlice...)
	env = append(env, "PORTAL_STATE_DIR="+stateDir)
	env = append(env, "TMUX="+tmuxSocketPath+",0,0")
	cmd := exec.Command("portal", "state", "daemon")
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		t.Fatalf("portaltest: start isolated portal state daemon (stateDir=%s): %v", stateDir, err)
	}
	RegisterSubprocessCleanup(t, cmd)
	// Make this daemon a legitimate sweep target for the test: by state dir
	// (respawn-immune) and by its initial PID.
	state.RegisterSandboxStateDir(stateDir)
	state.RegisterSandboxDaemon(cmd.Process.Pid)
	// Extend the same ownership to subprocess enumerations; the registry file is
	// re-read each time, so appending after env construction is fine.
	appendSandboxRegistryDir(t, stateDir)
	return cmd, stateDir
}

func appendSandboxRegistryDir(t *testing.T, dir string) {
	t.Helper()
	path := os.Getenv(state.SandboxRegistryEnv)
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("portaltest: open sandbox registry for append: %v", err)
	}
	if _, err := f.WriteString(dir + "\n"); err != nil {
		_ = f.Close()
		t.Fatalf("portaltest: append sandbox registry: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("portaltest: close sandbox registry: %v", err)
	}
}

// RegisterSubprocessCleanup arranges a guaranteed SIGKILL and reap for cmd on
// test exit, returning a channel that closes once it has been reaped so a caller
// can time its death. The reaper goroutine and the cleanup hook coordinate
// through that channel rather than both calling Wait, which would be two
// concurrent Waits on one Process. An unreaped child stays kernel-resident after
// SIGKILL — kill(pid, 0) keeps returning 0 — which would hang any ESRCH poll.
func RegisterSubprocessCleanup(t *testing.T, cmd *exec.Cmd) <-chan struct{} {
	t.Helper()
	reaped := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(reaped)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-reaped
	})
	return reaped
}
