package spawn

// Confirmed reports whether this window's token marker appeared within its ack
// budget. It is the sole "opened" predicate.
func (r WindowResult) Confirmed() bool {
	return r.Ack == AckConfirmed
}

// PartitionResults splits results into confirmed and failed session names,
// preserving list order. An adapter spawn-failure and an ack timeout are one
// failed class; an absent class comes back nil rather than empty.
func PartitionResults(results []WindowResult) (confirmed, failed []string) {
	for _, r := range results {
		if r.Confirmed() {
			confirmed = append(confirmed, r.Session)
			continue
		}
		failed = append(failed, r.Session)
	}
	return confirmed, failed
}

// FirstPermission returns the first result whose Outcome is
// permission-required, or the zero WindowResult and false. It is the burst-stop
// signal: the macOS Automation grant is per-(source, target), so once one window
// hits the wall every later window would too.
func FirstPermission(results []WindowResult) (WindowResult, bool) {
	for _, r := range results {
		if r.Result.Outcome == OutcomePermissionRequired {
			return r, true
		}
	}
	return WindowResult{}, false
}
