package spawn

func (r WindowResult) Confirmed() bool {
	return r.Ack == AckConfirmed
}

// PartitionResults preserves list order. An adapter spawn-failure and an ack
// timeout are one failed class; an absent class comes back nil, not empty.
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

// FirstPermission is the burst-stop signal: the macOS Automation grant is
// per-(source, target), so once one window hits the wall every later one would.
func FirstPermission(results []WindowResult) (WindowResult, bool) {
	for _, r := range results {
		if r.Result.Outcome == OutcomePermissionRequired {
			return r, true
		}
	}
	return WindowResult{}, false
}
