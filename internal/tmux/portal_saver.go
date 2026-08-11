package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"syscall"
	"time"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/state"
)

var saverLogger = log.For("saver")

// PortalSaverName is the tmux session name hosting the long-running save
// daemon. Its leading underscore marks it Portal-internal, which is what keeps
// it out of every user-facing listing and out of sessions.json capture.
const PortalSaverName = "_portal-saver"

// PortalBootstrapName is the tmux session name Client.StartServer creates to
// keep a freshly-started server alive. Its leading underscore marks it
// Portal-internal; no other code may create or re-use a session with this name.
const PortalBootstrapName = "_portal-bootstrap"

// Deliberately inert: the placeholder cannot write to the state directory or
// contend for the daemon lock, so destroy-unattached=off can be applied to a
// guaranteed-live session before the real daemon is swapped in. `sleep
// infinity` is not a substitute — macOS's BSD sleep(1) rejects "infinity".
const portalSaverPlaceholderCommand = "sh -c 'exec tail -f /dev/null'"

// Installed by respawn-pane only after destroy-unattached=off is in force:
// running it as the pane's initial process races the daemon's startup against
// tmux's destroy-unattached default.
const portalSaverDaemonCommand = "portal state daemon"

// BootstrapAliveCheck reports whether a daemon is alive for a state directory.
var BootstrapAliveCheck = state.DaemonAlive

// PortalSaverRetryDelay is the sleep between new-session retry attempts.
var PortalSaverRetryDelay = 100 * time.Millisecond

const portalSaverMaxAttempts = 3

// KillBarrierTimeoutCeiling bounds the kill barrier's wait for the prior daemon
// to exit after kill-session is issued.
const KillBarrierTimeoutCeiling = 5 * time.Second

// SaverBarrierSeams holds the kill barrier's probe and signal seams, plus the
// WARN sink it shares with the readiness barrier. SIGKILL is the only signal
// ever sent through SendSIGKILL. Timeout is sized above the daemon's cold-sweep
// ceiling so the WARN path stays reserved for genuinely stuck daemons, and
// Logger is never nil — it defaults to a discard sink.
type SaverBarrierSeams struct {
	IsAlive           func(int) bool
	SendSIGKILL       func(int) error
	PollInterval      time.Duration
	Timeout           time.Duration
	EscalationTimeout time.Duration
	Logger            *slog.Logger
}

// SaverReadinessSeams paces the readiness barrier's poll loop. Timeout is sized
// to cover normal daemon startup — fork, exec, flock, PID-file write — while
// keeping the bootstrap step bounded.
type SaverReadinessSeams struct {
	PollInterval time.Duration
	Timeout      time.Duration
}

// SaverVersionSeams holds the daemon.version read/write seams and the sink for
// the write's DEBUG breadcrumb, which is never nil — it defaults to discard, so
// a write made before the real sink is installed simply logs nothing.
type SaverVersionSeams struct {
	ReadVersionFile  func(string) (string, error)
	WriteVersionFile func(dir, version string) error
	WriterLogger     *slog.Logger
}

// SaverOperationSeams substitutes whole flows — the kill-and-wait and
// readiness-wait barriers — rather than the primitives inside them.
type SaverOperationSeams struct {
	WaitForReady func(string) error
	KillAndWait  func(*Client, string) error
}

// SaverSeams is every saver-side mutable seam in one struct: the primitives
// both barriers share at the top level, the rest grouped by the flow that owns
// them.
type SaverSeams struct {
	ReadPID        func(string) (int, error)
	IdentifyDaemon func(int) (state.IdentifyResult, error)

	Barrier   SaverBarrierSeams
	Readiness SaverReadinessSeams
	Version   SaverVersionSeams
	Ops       SaverOperationSeams
}

var saver = SaverSeams{
	ReadPID:        state.ReadPIDFile,
	IdentifyDaemon: state.IdentifyDaemon,

	Barrier: SaverBarrierSeams{
		IsAlive: state.IsProcessAlive,
		SendSIGKILL: func(pid int) error {
			return syscall.Kill(pid, syscall.SIGKILL)
		},
		PollInterval:      50 * time.Millisecond,
		Timeout:           KillBarrierTimeoutCeiling,
		EscalationTimeout: 1 * time.Second,
		Logger:            log.Discard(),
	},

	Readiness: SaverReadinessSeams{
		PollInterval: 50 * time.Millisecond,
		Timeout:      2 * time.Second,
	},

	Version: SaverVersionSeams{
		ReadVersionFile: state.ReadVersionFile,
		WriterLogger:    log.Discard(),
	},
}

func init() {
	// Wired here, not in the literal above: the WriteVersionFile closure
	// captures saver itself (an initialisation cycle) and must read the
	// current WriterLogger at call time, and the Ops defaults are functions
	// declared later in the file.
	saver.Version.WriteVersionFile = func(dir, version string) error {
		return state.WriteVersionFile(dir, version, saver.Version.WriterLogger)
	}
	saver.Ops.WaitForReady = waitForSaverDaemonReady
	saver.Ops.KillAndWait = killSaverAndWaitForDaemon
}

// SetBarrierLogger installs the shared WARN sink for both saver-side barriers.
// A nil argument is ignored, so a wiring mistake cannot leave the package
// without a sink.
func SetBarrierLogger(l *slog.Logger) {
	if l == nil {
		return
	}
	saver.Barrier.Logger = l
}

// SetVersionWriterLogger installs the sink for the daemon.version write
// breadcrumb. A nil argument is ignored, so a wiring mistake cannot leave the
// package without a sink.
func SetVersionWriterLogger(l *slog.Logger) {
	if l == nil {
		return
	}
	saver.Version.WriterLogger = l
}

// Blocking until the prior daemon has actually exited is what keeps the
// recycle quiet: otherwise the incoming daemon logs a "lock held" WARN while
// the outgoing one finishes its tick. That lock is the real safety net, so
// every failure here is tolerated and returns nil.
func killSaverAndWaitForDaemon(c *Client, stateDir string) error {
	priorPID, readErr := saver.ReadPID(stateDir)
	if readErr != nil {
		_ = c.KillSession(PortalSaverName)
		return nil
	}

	if !saver.Barrier.IsAlive(priorPID) {
		_ = c.KillSession(PortalSaverName)
		return nil
	}

	// Kill error tolerated: the session may have auto-destroyed between the
	// probe and the kill, which is the outcome we wanted anyway.
	saverLogger.Info("kill-barrier started", "target_pid", priorPID)
	_ = c.KillSession(PortalSaverName)

	if waitForPriorPIDExit(priorPID, saver.Barrier.Timeout) {
		saverLogger.Info("placeholder died", "target_pid", priorPID, "reason", "signal")
		return nil
	}

	return escalateKillToSIGKILL(priorPID)
}

func waitForPriorPIDExit(pid int, budget time.Duration) bool {
	ticker := time.NewTicker(saver.Barrier.PollInterval)
	defer ticker.Stop()

	deadline := time.Now().Add(budget)
	for range ticker.C {
		if !saver.Barrier.IsAlive(pid) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
	}
	return false
}

// Never signal a PID that does not positively identify as a portal state
// daemon, and let nothing but the two non-mutating log lines sit between that
// check and SendSIGKILL — a wider window lets the PID recycle. SIGKILL rather
// than SIGTERM is deliberate: the orphan must not run one last capture.
func escalateKillToSIGKILL(priorPID int) error {
	result, err := saver.IdentifyDaemon(priorPID)
	if err != nil || result != state.IdentifyIsPortalDaemon {
		saver.Barrier.Logger.Warn(
			"prior daemon not identity-checked as portal state daemon; skipping SIGKILL",
			"target_pid", priorPID,
		)
		return nil
	}
	saverLogger.Info("kill-barrier escalated", "target_pid", priorPID, "reason", "kill-session-timeout")
	saverLogger.Debug("kill-barrier escalating to SIGKILL", "target_pid", priorPID)
	_ = saver.Barrier.SendSIGKILL(priorPID)

	if waitForPriorPIDExit(priorPID, saver.Barrier.EscalationTimeout) {
		saverLogger.Info("placeholder died", "target_pid", priorPID, "reason", "signal")
		return nil
	}
	saver.Barrier.Logger.Warn(
		"prior daemon survived SIGKILL escalation",
		"target_pid", priorPID,
	)
	return nil
}

// Closes the race between the respawn and the bootstrap steps after it, which
// assume a healthy daemon. Every not-yet-ready shape is a continue, and a
// timeout only WARNs — it returns nil, because the daemon's own lock
// acquisition is the safety net for a truly stuck case.
func waitForSaverDaemonReady(stateDir string) error {
	deadline := time.Now().Add(saver.Readiness.Timeout)

	ticker := time.NewTicker(saver.Readiness.PollInterval)
	defer ticker.Stop()

	for {
		if pid, ready := isSaverDaemonReady(stateDir); ready {
			saverLogger.Info("daemon ready", "target_pid", pid)
			return nil
		}
		if !time.Now().Before(deadline) {
			saver.Barrier.Logger.Warn("saver respawn: daemon did not come up")
			return nil
		}
		<-ticker.C
	}
}

func isSaverDaemonReady(stateDir string) (int, bool) {
	pid, err := saver.ReadPID(stateDir)
	if err != nil {
		return 0, false
	}
	result, err := saver.IdentifyDaemon(pid)
	if err != nil {
		return 0, false
	}
	if result != state.IdentifyIsPortalDaemon {
		return 0, false
	}
	return pid, true
}

// BootstrapPortalSaver idempotently ensures _portal-saver exists and hosts a
// live daemon, recreating the session when it lingers with a dead one.
//
// Order is load-bearing on the create branch: the session must come up on the
// inert placeholder and take destroy-unattached=off before the daemon is
// respawned in, or a lock-loser daemon exits first and tmux self-destroys the
// session out from under the next bootstrap.
func BootstrapPortalSaver(c *Client, stateDir string) error {
	sessionPresent := c.HasSession(PortalSaverName)

	if sessionPresent && !BootstrapAliveCheck(stateDir) {
		_ = saver.Ops.KillAndWait(c, stateDir)
		sessionPresent = false
	}

	// Best-effort: the pane id is observability context, so a failed read
	// leaves it empty rather than aborting the bootstrap.
	var paneID string

	createdSession := false
	if !sessionPresent {
		if err := createPortalSaverWithRetry(c); err != nil {
			return err
		}
		createdSession = true

		paneID, _ = c.SaverPaneID(PortalSaverName)
		saverLogger.Info("placeholder created", "tmux_pane", paneID)
	}

	if err := c.SetSessionOption(PortalSaverName, "destroy-unattached", "off"); err != nil {
		return fmt.Errorf("bootstrap _portal-saver: set destroy-unattached: %w", err)
	}
	if !createdSession {
		paneID, _ = c.SaverPaneID(PortalSaverName)
	}
	saverLogger.Info("destroy-unattached off", "tmux_pane", paneID)

	if createdSession {
		fromPID := saverPanePIDBestEffort(c)
		if err := c.RespawnPane(PortalSaverName, portalSaverDaemonCommand); err != nil {
			return fmt.Errorf("bootstrap _portal-saver: respawn daemon: %w", err)
		}
		toPID := saverPanePIDBestEffort(c)
		saverLogger.Info("respawn-daemon", "from_pid", fromPID, "to_pid", toPID, "tmux_pane", paneID)

		_ = saver.Ops.WaitForReady(stateDir)
	}

	return nil
}

func saverPanePIDBestEffort(c *Client) int {
	pid, err := saverPanePID(c, PortalSaverName)
	if err != nil {
		saverLogger.Warn("saver respawn: pane-pid read failed", "error", err)
		return 0
	}
	return pid
}

// EnsurePortalSaverVersion bootstraps _portal-saver, recycling a live daemon
// first when the recorded version disagrees with currentVersion. Only the kill
// decision is version-aware; the new daemon writes daemon.version itself.
//
// A missing daemon.version is repaired in place rather than recycled: a
// lock-loser daemon exits before writing the file, so an alive daemon with no
// version file is an expected shape, not a mismatch worth a kill-respawn.
func EnsurePortalSaverVersion(c *Client, stateDir, currentVersion string) error {
	stored, readErr := saver.Version.ReadVersionFile(stateDir)
	alive := BootstrapAliveCheck(stateDir)

	if alive && shouldKillSaverOnVersionDecision(stored, currentVersion, readErr) {
		_ = saver.Ops.KillAndWait(c, stateDir)
	} else if alive && errors.Is(readErr, state.ErrVersionFileAbsent) {
		// A failed repair must propagate: BootstrapPortalSaver does not run.
		if err := saver.Version.WriteVersionFile(stateDir, currentVersion); err != nil {
			return fmt.Errorf("defensive daemon.version write failed: %w", err)
		}
	}
	return BootstrapPortalSaver(c, stateDir)
}

// Callers must have established that the daemon is alive; this does not
// consult BootstrapAliveCheck itself.
func shouldKillSaverOnVersionDecision(stored, currentVersion string, readErr error) bool {
	// Dev builds always recycle. The stored side counts only on a clean read:
	// a read error means the stored version is unknown, not empty.
	if currentVersion == "" || currentVersion == "dev" {
		return true
	}
	if readErr == nil && (stored == "" || stored == "dev") {
		return true
	}

	if errors.Is(readErr, state.ErrVersionFileAbsent) {
		return false
	}
	if readErr != nil {
		// Conservative: an unreadable version file is treated as needing a
		// recycle rather than assumed to match.
		return true
	}

	return stored != currentVersion
}

func createPortalSaverWithRetry(c *Client) error {
	var lastErr error
	for attempt := 1; attempt <= portalSaverMaxAttempts; attempt++ {
		err := c.NewDetachedSessionNoCwd(PortalSaverName, portalSaverPlaceholderCommand)
		if err == nil {
			return nil
		}
		lastErr = err

		// A concurrent bootstrap may have won the create; that counts as
		// success rather than a duplicate-creation error.
		if c.HasSession(PortalSaverName) {
			return nil
		}

		if attempt < portalSaverMaxAttempts {
			time.Sleep(PortalSaverRetryDelay)
		}
	}
	return fmt.Errorf("bootstrap _portal-saver: create after %d attempts: %w", portalSaverMaxAttempts, lastErr)
}
