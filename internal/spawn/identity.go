package spawn

import (
	"path"
	"strings"
)

// Identity is a detected host terminal's macOS bundle id plus a friendly display
// name. The zero value is the NULL identity — no host-local terminal — which a
// remote/mosh client or a transient detection failure resolves to.
type Identity struct {
	BundleID string
	Name     string
}

// IsNull reports whether the identity is the NULL state; NULL is defined solely
// by an empty bundle id.
func (i Identity) IsNull() bool {
	return i.BundleID == ""
}

// NewIdentity builds a host-terminal identity from a bundle id and an optional
// app name. A blank bundle id yields the NULL identity regardless of appName;
// any other bundle id passes through even when Portal does not know it, with
// Name derived from the bundle id when appName is blank.
func NewIdentity(bundleID, appName string) Identity {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		return Identity{}
	}

	name := strings.TrimSpace(appName)
	if name == "" {
		name = deriveName(bundleID)
	}

	return Identity{BundleID: bundleID, Name: name}
}

func deriveName(bundleID string) string {
	segment := bundleID
	if i := strings.LastIndex(segment, "."); i >= 0 {
		segment = segment[i+1:]
	}
	if i := strings.IndexByte(segment, '-'); i >= 0 {
		segment = segment[:i]
	}
	if segment == "" {
		return bundleID
	}
	return segment
}

// MatchesFamily reports whether bundleID belongs to the bundle-id family
// described by pattern, using path.Match semantics: bundle ids carry no "/", so
// "*" spans the whole remainder including a channel suffix. A malformed pattern
// is a non-match rather than a failure.
func MatchesFamily(bundleID, pattern string) bool {
	ok, err := path.Match(pattern, bundleID)
	if err != nil {
		return false
	}
	return ok
}
