package spawn

import (
	"fmt"

	"github.com/leeovery/portal/internal/tmux"
)

// ClientActivity is one tmux client's detection-relevant data. It mirrors
// tmux.ClientInfo so inside-tmux resolution carries no tmux dependency.
type ClientActivity struct {
	PID      int
	Activity int64
}

type clientLister interface {
	ListClients(session string) ([]ClientActivity, error)
}

type tmuxClientLister struct {
	c *tmux.Client
}

var _ clientLister = tmuxClientLister{}

func (l tmuxClientLister) ListClients(session string) ([]ClientActivity, error) {
	infos, err := l.c.ListClients(session)
	if err != nil {
		return nil, err
	}
	clients := make([]ClientActivity, 0, len(infos))
	for _, info := range infos {
		clients = append(clients, ClientActivity{PID: info.PID, Activity: info.Activity})
	}
	return clients, nil
}

// Inside tmux, Portal's own ancestry leads to the tmux server, so locality is
// gated on the client that triggered the burst — the most-active one, since
// client_activity tracks sent input. Selecting the winner before checking
// locality is load-bearing: filtering to local clients first drives windows onto
// the wrong machine when a remote trigger shares a session with a local bystander.
func detectInsideTmux(session string, lister clientLister, walker ProcessWalker, reader BundleReader) (Identity, error) {
	clients, err := lister.ListClients(session)
	if err != nil {
		return Identity{}, transient(fmt.Sprintf("list tmux clients for session %q", session), err)
	}

	if len(clients) == 0 {
		// No winner to select — the honest no-op, not a transient error.
		return Identity{}, nil
	}

	winner := selectTriggeringClient(clients)

	// Only the winner is walked: a transient walk failure fails safe to NULL
	// rather than falling back to a lower-activity local client.
	return walkToBundle(winner.PID, walker, reader)
}

func selectTriggeringClient(clients []ClientActivity) ClientActivity {
	winner := clients[0]
	for _, client := range clients[1:] {
		if client.Activity > winner.Activity {
			winner = client
		}
	}
	return winner
}
