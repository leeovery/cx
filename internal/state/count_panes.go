package state

// CountPanes returns the total number of panes across every window of every
// session in idx.
func CountPanes(idx Index) int {
	total := 0
	for _, s := range idx.Sessions {
		for _, w := range s.Windows {
			total += len(w.Panes)
		}
	}
	return total
}
