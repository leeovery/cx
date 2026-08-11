package tmux

import (
	"log/slog"
	"time"

	"github.com/leeovery/portal/internal/state"
)

var KillSaverAndWaitForDaemon = killSaverAndWaitForDaemon

func ManagedEventNames() []string { return managedEventNames() }

func PortalTeardownEvents() []string { return portalEvents }

func PortalTeardownFingerprints() []string { return portalCommandSubstrings }

func ManagedEventFingerprintUnion() []string { return managedEventFingerprintUnion() }

const MigrateRenameSubstring = migrateRenameSubstring

var UnregisterPortalHooksWithLogger = unregisterPortalHooks

var SaverPanePID = saverPanePID

const PortalSaverPlaceholderCommand = portalSaverPlaceholderCommand

const PortalSaverDaemonCommand = portalSaverDaemonCommand

var ShouldKillSaverOnVersionDecision = shouldKillSaverOnVersionDecision

var WaitForSaverDaemonReady = waitForSaverDaemonReady

func Saver() *SaverSeams { return &saver }

func SaverBarrier() *SaverBarrierSeams { return &saver.Barrier }

func SaverReadiness() *SaverReadinessSeams { return &saver.Readiness }

func SaverVersion() *SaverVersionSeams { return &saver.Version }

func SaverOps() *SaverOperationSeams { return &saver.Ops }

func SaverReadPIDSeam() *func(string) (int, error) { return &saver.ReadPID }

func SaverIdentifyDaemonSeam() *func(int) (state.IdentifyResult, error) {
	return &saver.IdentifyDaemon
}

func BarrierIsAliveSeam() *func(int) bool { return &saver.Barrier.IsAlive }

func BarrierPollIntervalSeam() *time.Duration { return &saver.Barrier.PollInterval }

func BarrierTimeoutSeam() *time.Duration { return &saver.Barrier.Timeout }

func BarrierEscalationTimeoutSeam() *time.Duration { return &saver.Barrier.EscalationTimeout }

func BarrierSendSIGKILLSeam() *func(int) error { return &saver.Barrier.SendSIGKILL }

func BarrierLoggerSeam() **slog.Logger { return &saver.Barrier.Logger }

func SaverReadinessPollIntervalSeam() *time.Duration {
	return &saver.Readiness.PollInterval
}

func SaverReadinessTimeoutSeam() *time.Duration {
	return &saver.Readiness.Timeout
}

func PortalSaverReadVersionFileSeam() *func(string) (string, error) {
	return &saver.Version.ReadVersionFile
}

func PortalSaverWriteVersionFileSeam() *func(string, string) error {
	return &saver.Version.WriteVersionFile
}

func VersionWriterLoggerSeam() **slog.Logger { return &saver.Version.WriterLogger }

func WaitForSaverDaemonReadyFnSeam() *func(string) error {
	return &saver.Ops.WaitForReady
}

func KillSaverAndWaitForDaemonFnSeam() *func(*Client, string) error {
	return &saver.Ops.KillAndWait
}
