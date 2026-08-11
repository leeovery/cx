package spawn

import (
	"log/slog"
	"os"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/tmux"
)

var spawnLogger = log.For("spawn")

const (
	msgDetectionResolved   = "detection resolved host terminal"
	msgDetectionNullBundle = "detection resolved no host-local terminal"
	msgDetectionTransient  = "detection transient failure"
)

const (
	routeInsideTmux  = "inside-tmux client walk"
	routeOutsideTmux = "outside-tmux env/self walk"
)

type Detector struct {
	insideTmux     func() bool
	getenv         func(string) string
	selfPID        int
	walker         ProcessWalker
	reader         BundleReader
	lister         clientLister
	currentSession func() (string, error)
	logger         *slog.Logger
}

func NewDetector(client *tmux.Client) *Detector {
	return &Detector{
		insideTmux:     tmux.InsideTmux,
		getenv:         os.Getenv,
		selfPID:        os.Getpid(),
		walker:         realProcessWalker{},
		reader:         realBundleReader{},
		lister:         tmuxClientLister{c: client},
		currentSession: client.CurrentSessionName,
		logger:         spawnLogger,
	}
}

// Detect emits exactly one spawn-component record. A transient failure folds to
// the NULL identity — the same unsupported no-op path as a clean NULL — but
// WARNs rather than INFOs.
func (d *Detector) Detect() Identity {
	id, route, err := d.resolve()

	switch {
	case err != nil:
		d.logger.Warn(msgDetectionTransient, "detail", err.Error())
		return Identity{}
	case !id.IsNull():
		d.logger.Info(msgDetectionResolved, "terminal", id.Name, "bundle_id", id.BundleID, "detail", route)
		return id
	default:
		d.logger.Info(msgDetectionNullBundle, "detail", route)
		return Identity{}
	}
}

// Every non-nil error returned here satisfies errors.Is(err,
// ErrDetectTransient), including the normalised current-session read failure.
func (d *Detector) resolve() (Identity, string, error) {
	if d.insideTmux() {
		session, err := d.currentSession()
		if err != nil {
			return Identity{}, routeInsideTmux, transient("resolve current tmux session", err)
		}
		id, derr := detectInsideTmux(session, d.lister, d.walker, d.reader)
		return id, routeInsideTmux, derr
	}

	id, derr := detectOutsideTmux(d.getenv, d.selfPID, d.walker, d.reader)
	return id, routeOutsideTmux, derr
}
