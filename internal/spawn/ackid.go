package spawn

import (
	"fmt"
	"strings"

	"github.com/leeovery/portal/internal/session"
)

// SpawnMarkerPrefix is the tmux server-option name prefix keying each spawned
// window's token-ack confirmation. It is deliberately distinct from
// internal/state's SkeletonMarkerPrefix so the two server-option enumerators
// stay blind to each other's markers.
const SpawnMarkerPrefix = "@portal-spawn-"

// NewSpawnID produces an option-name-safe ack id from gen. A generator error is
// wrapped and propagated; a value outside session.NanoIDAlphabet yields an
// error and an empty id rather than a marker name set-option would reject.
func NewSpawnID(gen func() (string, error)) (string, error) {
	id, err := gen()
	if err != nil {
		return "", fmt.Errorf("spawn: generate ack id: %w", err)
	}
	if !isOptionSafeID(id) {
		return "", fmt.Errorf("spawn: generated ack id %q is not option-safe", id)
	}
	return id, nil
}

func isOptionSafeID(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !strings.ContainsRune(session.NanoIDAlphabet, r)
	}) < 0
}

// SpawnMarkerName renders the tmux server-option name for a batch's token ack.
// Option-safe ids are hyphen-free, so the hyphen joining batch and token is an
// unambiguous delimiter.
func SpawnMarkerName(batch, token string) string {
	return SpawnMarkerPrefix + batch + "-" + token
}

// ParseSpawnMarkerName is the inverse of SpawnMarkerName. It reports false for
// a foreign prefix, a missing delimiter, or an empty batch or token.
func ParseSpawnMarkerName(name string) (batch, token string, ok bool) {
	rest, found := strings.CutPrefix(name, SpawnMarkerPrefix)
	if !found {
		return "", "", false
	}
	b, t, found := strings.Cut(rest, "-")
	if !found || b == "" || t == "" {
		return "", "", false
	}
	return b, t, true
}

// FormatSpawnAckFlag renders the "<batch>:<token>" value carried by the open
// command's --ack flag. Option-safe ids are colon-free, so the colon is an
// unambiguous delimiter.
func FormatSpawnAckFlag(batch, token string) string {
	return batch + ":" + token
}

// ParseSpawnAckFlag is the inverse of FormatSpawnAckFlag. It reports false for
// a missing colon or an empty batch or token.
func ParseSpawnAckFlag(value string) (batch, token string, ok bool) {
	b, t, found := strings.Cut(value, ":")
	if !found || b == "" || t == "" {
		return "", "", false
	}
	return b, t, true
}
