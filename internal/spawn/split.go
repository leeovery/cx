package spawn

// SplitNetN splits an ordered selection under the trailing-trigger convention:
// external is the leading N-1 rows, trigger the last — net N windows, never N+1.
// Callers must guarantee len(ordered) >= 1; an empty slice panics.
func SplitNetN(ordered []string) (external []string, trigger string) {
	return ordered[:len(ordered)-1], ordered[len(ordered)-1]
}

// SplitTriggerFirst splits an ordered surface selection under the
// leading-trigger convention: trigger is the first surface — the one the invoking
// terminal absorbs and self-connects to last — and external the trailing N-1.
// Callers must guarantee len(ordered) >= 1; an empty slice panics.
func SplitTriggerFirst(ordered []Surface) (trigger Surface, external []Surface) {
	return ordered[0], ordered[1:]
}
