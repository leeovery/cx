package portaltest

import (
	"fmt"

	"github.com/leeovery/portal/internal/state"
)

// DaemonPIDObservation is one reading of a state directory's daemon.pid: the
// PID recorded there, whether that process still answers, and the read failure
// if there was one. It is comparable so a progress-based wait can tell a daemon
// that is still coming up from one that has stopped moving, and it keeps the
// error rather than collapsing it to "not yet" so a red run says why the PID
// was unreadable.
type DaemonPIDObservation struct {
	PID   int
	Alive bool
	Err   string
}

func (o DaemonPIDObservation) String() string {
	if o.Err != "" {
		return fmt.Sprintf("pid=%d alive=%v err=%s", o.PID, o.Alive, o.Err)
	}
	return fmt.Sprintf("pid=%d alive=%v", o.PID, o.Alive)
}

// ObserveDaemonPID reads daemon.pid under stateDir and probes whatever PID it
// records. A read failure yields a zero PID and a dead probe, never a panic:
// the pidfile is routinely absent at the moment a wait first looks.
func ObserveDaemonPID(stateDir string) DaemonPIDObservation {
	pid, err := state.ReadPIDFile(stateDir)
	if err != nil {
		return DaemonPIDObservation{PID: pid, Err: err.Error()}
	}
	return DaemonPIDObservation{PID: pid, Alive: state.IsProcessAlive(pid)}
}
