package spawn

import (
	"fmt"
	"strings"

	"github.com/leeovery/portal/internal/nanoid"
)

// SpawnMarkerPrefix is deliberately distinct from internal/state's
// SkeletonMarkerPrefix, so the two server-option enumerators stay blind to each
// other's markers.
const SpawnMarkerPrefix = "@portal-spawn-"

// NewSpawnID errors on a value outside nanoid.Alphabet rather than let
// it become a marker name set-option would reject.
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
		return !strings.ContainsRune(nanoid.Alphabet, r)
	}) < 0
}

// SpawnMarkerName joins batch and token with a hyphen: option-safe ids are
// hyphen-free, so the delimiter is unambiguous.
func SpawnMarkerName(batch, token string) string {
	return SpawnMarkerPrefix + batch + "-" + token
}

// ParseSpawnMarkerName is the inverse of SpawnMarkerName.
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

// FormatSpawnAckFlag renders the open command's --ack value: option-safe ids are
// colon-free, so the colon is an unambiguous delimiter.
func FormatSpawnAckFlag(batch, token string) string {
	return batch + ":" + token
}

// ParseSpawnAckFlag is the inverse of FormatSpawnAckFlag.
func ParseSpawnAckFlag(value string) (batch, token string, ok bool) {
	b, t, found := strings.Cut(value, ":")
	if !found || b == "" || t == "" {
		return "", "", false
	}
	return b, t, true
}
