package spawn

import "strings"

type serverOptionWriter interface {
	SetServerOption(name, value string) error
	UnsetServerOption(name string) error
}

type serverOptionLister interface {
	ShowAllServerOptions() (string, error)
}

type AckCollector interface {
	Collect(batch string) (map[string]struct{}, error)
}

// AckCleaner is best-effort: a leaked marker self-expires with the tmux server.
type AckCleaner interface {
	Clean(batch string) error
}

// AckWriter writes a spawned window's own token marker just before it execs.
type AckWriter interface {
	Write(batch, token string) error
}

// AckChannelFull omits Write deliberately: the burster never writes markers,
// the spawned windows do.
type AckChannelFull interface {
	AckCollector
	AckCleaner
}

// ServerOptionAckChannel implements the token-ack contract over tmux server
// options. Markers are presence-only — the value is never read.
type ServerOptionAckChannel struct {
	w serverOptionWriter
	l serverOptionLister
}

func NewServerOptionAckChannel(w serverOptionWriter, l serverOptionLister) *ServerOptionAckChannel {
	return &ServerOptionAckChannel{w: w, l: l}
}

var (
	_ AckWriter      = (*ServerOptionAckChannel)(nil)
	_ AckCollector   = (*ServerOptionAckChannel)(nil)
	_ AckCleaner     = (*ServerOptionAckChannel)(nil)
	_ AckChannelFull = (*ServerOptionAckChannel)(nil)
)

// Write is idempotent: presence is the signal, the value opaque.
func (c *ServerOptionAckChannel) Write(batch, token string) error {
	return c.w.SetServerOption(SpawnMarkerName(batch, token), "1")
}

// Collect returns the tokens marked for batch. A listing failure returns
// (nil, err) rather than an empty-as-success set, which would mis-classify every
// window as failed; on success the map is non-nil.
func (c *ServerOptionAckChannel) Collect(batch string) (map[string]struct{}, error) {
	out, err := c.l.ShowAllServerOptions()
	if err != nil {
		return nil, err
	}
	tokens := map[string]struct{}{}
	forEachBatchMarker(out, batch, func(_, token string) {
		tokens[token] = struct{}{}
	})
	return tokens, nil
}

// Clean is idempotent and tolerates a concurrent unset. A per-marker failure
// does not abort the sweep; the first error encountered is returned.
func (c *ServerOptionAckChannel) Clean(batch string) error {
	out, err := c.l.ShowAllServerOptions()
	if err != nil {
		return err
	}
	var firstErr error
	forEachBatchMarker(out, batch, func(name, _ string) {
		if uerr := c.w.UnsetServerOption(name); uerr != nil && firstErr == nil {
			firstErr = uerr
		}
	})
	return firstErr
}

func forEachBatchMarker(out, batch string, fn func(name, token string)) {
	for _, name := range optionNames(out) {
		b, token, ok := ParseSpawnMarkerName(name)
		if !ok || b != batch {
			continue
		}
		fn(name, token)
	}
}

func optionNames(out string) []string {
	if out == "" {
		return nil
	}
	var names []string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.IndexAny(line, " \t")
		if idx < 0 {
			continue
		}
		names = append(names, line[:idx])
	}
	return names
}
