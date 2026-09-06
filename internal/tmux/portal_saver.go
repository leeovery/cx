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

// PortalSaverName hosts the long-running save daemon. Its leading underscore
// marks it Portal-internal, which is what keeps it out of every user-facing
// listing and out of sessions.json capture.
const PortalSaverName = "_portal-saver"

// PortalBootstrapName keeps a freshly-started server alive. Its leading
// underscore marks it Portal-internal, and no other code may create or re-use a
// session with this name.
const PortalBootstrapName = "_portal-bootstrap"

// Deliberately inert: the placeholder cannot write to the state directory or
// contend for the daemon lock, so destroy-unattached=off can be applied to a
// guaranteed-live session before the real daemon is swapped in. `sleep infinity`
// is no substitute — macOS's BSD sleep(1) rejects "infinity".
const portalSaverPlaceholderCommand = "sh -c 'exec tail -f /dev/null'"

// Installed by respawn-pane only after destroy-unattached=off is in force:
// running it as the pane's initial process races the daemon's startup against
// tmux's destroy-unattached default.
const portalSaverDaemonCommand = "portal state daemon"

var BootstrapAliveCheck = state.DaemonAlive

var PortalSaverRetryDelay = 100 * time.Millisecond

const portalSaverMaxAttempts = 3

const KillBarrierTimeoutCeiling = 5 * time.Second

// SaverBarrierSeams holds the kill barrier's seams and the WARN sink it shares
// with the readiness barrier. No signal but SIGKILL is ever sent through
// SendSIGKILL, Timeout sits above the daemon's cold-sweep ceiling so the WARN
// path stays reserved for genuinely stuck daemons, and Logger is never nil.
type SaverBarrierSeams struct {
	IsAlive           func(int) bool
	SendSIGKILL       func(int) error
	PollInterval      time.Duration
	Timeout           time.Duration
	EscalationTimeout time.Duration
	Logger            *slog.Logger
}

// SaverReadinessSeams paces the readiness barrier's poll loop. Stall and
// Ceiling are separate budgets on purpose: a host under IO contention makes a
// healthy daemon slower to fork, exec, flock and write its PID file without
// making it broken, so wall-clock alone must not decide the verdict.
type SaverReadinessSeams struct {
	PollInterval time.Duration
	// Stall is how long a readiness observation that has already moved may
	// stay put before the barrier gives up. Every change restarts it, and it
	// does not run until the first change: before that there is no progress to
	// have stopped, only a daemon that has not surfaced yet.
	Stall time.Duration
	// Ceiling bounds the whole wait unconditionally, so a daemon that never
	// surfaces — and one whose observation churns without ever identifying —
	// still ends the step rather than hanging bootstrap.
	Ceiling time.Duration
}

type SaverVersionSeams struct {
	ReadVersionFile  func(string) (string, error)
	WriteVersionFile func(dir, version string) error

	// Never nil: it defaults to discard, so a write made before the real sink
	// is installed simply logs nothing.
	WriterLogger *slog.Logger
}

// SaverOperationSeams substitutes whole flows rather than the primitives inside
// them.
type SaverOperationSeams struct {
	WaitForReady func(string) error
	KillAndWait  func(*Client, string) error
}

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
		Stall:        2 * time.Second,
		Ceiling:      10 * time.Second,
	},

	Version: SaverVersionSeams{
		ReadVersionFile: state.ReadVersionFile,
		WriterLogger:    log.Discard(),
	},
}

func init() {
	// Wired here, not in the literal above: the WriteVersionFile closure
	// captures saver itself (an initialisation cycle) and must read the current
	// WriterLogger at call time, and the Ops defaults are declared below.
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

// Never signal a PID that does not positively identify as a portal state daemon,
// and let nothing but non-mutating log lines sit between that check and
// SendSIGKILL — a wider window lets the PID recycle. SIGKILL rather than SIGTERM
// is deliberate: the orphan must not run one last capture.
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

// ErrSaverDaemonNotReady reports that the readiness barrier gave up before the
// saver's daemon identified itself. Bootstrap turns it into a user-visible
// warning; it never aborts a step.
var ErrSaverDaemonNotReady = errors.New("saver daemon did not come up")

// readinessObservation is one poll of the daemon's readiness: what daemon.pid
// read back and what identifying that pid said. It is comparable so the barrier
// can tell a daemon that is still moving from one that has stopped, and it
// renders itself into the give-up error so a failed bootstrap says what the
// daemon was last doing.
type readinessObservation struct {
	pid         int
	readErr     string
	verdict     state.IdentifyResult
	identifyErr string
}

func (o readinessObservation) ready() bool {
	return o.readErr == "" && o.identifyErr == "" && o.verdict == state.IdentifyIsPortalDaemon
}

func (o readinessObservation) String() string {
	switch {
	case o.readErr != "":
		return fmt.Sprintf("daemon.pid unreadable: %s", o.readErr)
	case o.identifyErr != "":
		return fmt.Sprintf("pid %d unidentifiable: %s", o.pid, o.identifyErr)
	default:
		return fmt.Sprintf("pid %d %s", o.pid, describeVerdict(o.verdict))
	}
}

func describeVerdict(v state.IdentifyResult) string {
	switch v {
	case state.IdentifyIsPortalDaemon:
		return "is a portal state daemon"
	case state.IdentifyNotPortalDaemon:
		return "is not a portal state daemon"
	case state.IdentifyDead:
		return "is dead"
	default:
		return fmt.Sprintf("identifies as %d", v)
	}
}

func observeSaverReadiness(stateDir string) readinessObservation {
	pid, err := saver.ReadPID(stateDir)
	if err != nil {
		return readinessObservation{readErr: err.Error()}
	}
	verdict, err := saver.IdentifyDaemon(pid)
	if err != nil {
		return readinessObservation{pid: pid, identifyErr: err.Error()}
	}
	return readinessObservation{pid: pid, verdict: verdict}
}

// Closes the race between the respawn and the bootstrap steps after it, which
// assume a healthy daemon. Every not-yet-ready shape is a continue: the wait
// only ends when the observation reads ready, when it has moved and then sat
// still for the whole stall budget, or when the ceiling elapses.
func waitForSaverDaemonReady(stateDir string) error {
	ceiling := time.Now().Add(saver.Readiness.Ceiling)

	var last readinessObservation
	var stallDeadline time.Time
	for first := true; ; first = false {
		observation := observeSaverReadiness(stateDir)
		if observation.ready() {
			saverLogger.Info("daemon ready", "target_pid", observation.pid)
			return nil
		}
		if !first && observation != last {
			stallDeadline = time.Now().Add(saver.Readiness.Stall)
		}
		last = observation

		now := time.Now()
		stalled := !stallDeadline.IsZero() && now.After(stallDeadline)
		if stalled || now.After(ceiling) {
			err := fmt.Errorf("%w: %s", ErrSaverDaemonNotReady, observation)
			saver.Barrier.Logger.Warn("saver respawn: daemon did not come up", "error", err)
			return err
		}
		time.Sleep(saver.Readiness.PollInterval)
	}
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
		if err := c.RespawnPane(CoordTargetExact(PortalSaverName), portalSaverDaemonCommand); err != nil {
			return fmt.Errorf("bootstrap _portal-saver: respawn daemon: %w", err)
		}
		toPID := saverPanePIDBestEffort(c)
		saverLogger.Info("respawn-daemon", "from_pid", fromPID, "to_pid", toPID, "tmux_pane", paneID)

		if err := saver.Ops.WaitForReady(stateDir); err != nil {
			return err
		}
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
// decision is version-aware; the new daemon writes daemon.version itself. A
// missing daemon.version is repaired in place rather than recycled: a lock-loser
// daemon exits before writing the file, so an alive daemon with no version file
// is an expected shape, not a mismatch worth a kill-respawn.
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
