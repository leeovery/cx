package spawn

type SurfaceKind int

const (
	// SurfaceAttach carries a session name in Surface.Value. It is the zero
	// value, so a zero Surface is an attach.
	SurfaceAttach SurfaceKind = iota
	// SurfaceMint carries a literal directory in Surface.Value.
	SurfaceMint
)

func (k SurfaceKind) String() string {
	switch k {
	case SurfaceAttach:
		return "attach"
	case SurfaceMint:
		return "mint"
	default:
		return "unknown"
	}
}

// Surface is one resolved open target. A mint's Value must already be the
// resolved absolute dir, never an alias key or zoxide query — those could
// re-resolve differently inside the spawned window.
type Surface struct {
	Kind  SurfaceKind
	Value string
}
