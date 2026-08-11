package spawn

// SplitNetN takes the trigger from the tail — net N windows, never N+1. Callers
// must guarantee len(ordered) >= 1; an empty slice panics.
func SplitNetN(ordered []string) (external []string, trigger string) {
	return ordered[:len(ordered)-1], ordered[len(ordered)-1]
}

// SplitTriggerFirst takes the trigger from the head — the surface the invoking
// terminal absorbs and self-connects to last. Callers must guarantee
// len(ordered) >= 1; an empty slice panics.
func SplitTriggerFirst(ordered []Surface) (trigger Surface, external []Surface) {
	return ordered[0], ordered[1:]
}
