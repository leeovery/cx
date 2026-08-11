package state

import "testing"

// White-box: the append branch is unreachable through CaptureStructure, whose
// merge gates every prev pane on its session being live in the fresh capture.
func TestFindOrAppendSessionCopiesPortalID(t *testing.T) {
	fresh := &Index{Sessions: []Session{}}
	ps := Session{
		Name:        "portal-aB3xY9kZ",
		PortalID:    "aB3xY9kZ",
		Environment: map[string]string{"FOO": "bar"},
		Windows: []Window{
			{Index: 0, Panes: []Pane{{Index: 0}}},
		},
	}

	si := findOrAppendSession(fresh, ps)

	if si != 0 {
		t.Fatalf("findOrAppendSession index = %d; want 0 (appended into empty index)", si)
	}
	got := fresh.Sessions[si]
	if got.PortalID != ps.PortalID {
		t.Errorf("appended Session.PortalID = %q; want %q", got.PortalID, ps.PortalID)
	}
	if got.Name != ps.Name {
		t.Errorf("appended Session.Name = %q; want %q", got.Name, ps.Name)
	}
	if got.Environment["FOO"] != "bar" {
		t.Errorf("appended Session.Environment = %v; want FOO=bar", got.Environment)
	}
	// ps.Windows is deliberately not copied: the caller populates windows.
	if len(got.Windows) != 0 {
		t.Errorf("appended Session.Windows = %v; want empty (windows populated by caller)", got.Windows)
	}
	if got.Windows == nil {
		t.Errorf("appended Session.Windows is nil; want empty non-nil []Window{}")
	}
}
