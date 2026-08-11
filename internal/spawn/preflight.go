package spawn

// PreflightMissing returns nil when every session is present, preserving input
// order otherwise. Production wires exists to a probe that folds an error to
// false, so an unprobeable session is conservatively reported gone.
func PreflightMissing(sessions []string, exists func(name string) bool) []string {
	var gone []string
	for _, s := range sessions {
		if !exists(s) {
			gone = append(gone, s)
		}
	}
	return gone
}
