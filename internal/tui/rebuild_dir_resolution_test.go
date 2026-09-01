package tui

import (
	"testing"

	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/tmux"
)

type fakeStamper struct {
	path string
	err  error

	setErr   error
	setCalls []stamperSetCall
	reads    []string
}

type stamperSetCall struct {
	session string
	name    string
	value   string
}

func (f *fakeStamper) ActivePaneCurrentPath(sess string) (string, error) {
	f.reads = append(f.reads, sess)
	return f.path, f.err
}

func (f *fakeStamper) SetSessionOption(sess, name, value string) error {
	f.setCalls = append(f.setCalls, stamperSetCall{session: sess, name: name, value: value})
	return f.setErr
}

var _ session.PaneCurrentPathReader = (*fakeStamper)(nil)

type fakeDirRunner struct {
	gitRoot string
}

func (r *fakeDirRunner) Run(name string, args ...string) (string, error) {
	return r.gitRoot, nil
}

func TestRebuildSessionListDirResolution(t *testing.T) {
	t.Run("By Project: an empty-Dir session resolving via the reader appears under its project, not Unknown", func(t *testing.T) {
		dir := t.TempDir()
		key := project.CanonicalDirKey(dir)
		projects := []project.Project{{Path: dir, Name: "Portal"}}
		sessions := []tmux.Session{{Name: "portal-abc", Dir: ""}}

		m := newRebuildTestModel(t, prefs.ModeByProject, sessions, projects)
		m.dirReader = &fakeStamper{path: dir}
		m.dirRunner = &fakeDirRunner{gitRoot: dir}

		m.rebuildSessionList()

		rows := sessionRows(m.sessionList.Items())
		if len(rows) != 1 {
			t.Fatalf("len(session rows) = %d, want 1", len(rows))
		}
		si := rows[0]
		if si.CatchAll {
			t.Fatalf("session routed to Unknown catch-all, want resolved under its project")
		}
		if si.GroupKey != key {
			t.Errorf("GroupKey = %q, want %q", si.GroupKey, key)
		}
		if si.GroupHeading != "Portal" {
			t.Errorf("GroupHeading = %q, want %q", si.GroupHeading, "Portal")
		}
	})

	t.Run("By Tag: an empty-Dir session resolving to a tagged project appears under its tags, not Untagged", func(t *testing.T) {
		dir := t.TempDir()
		projects := []project.Project{{Path: dir, Name: "Portal", Tags: []string{"work", "infra"}}}
		sessions := []tmux.Session{{Name: "portal-abc", Dir: ""}}

		m := newRebuildTestModel(t, prefs.ModeByTag, sessions, projects)
		m.dirReader = &fakeStamper{path: dir}
		m.dirRunner = &fakeDirRunner{gitRoot: dir}

		m.rebuildSessionList()

		rows := sessionRows(m.sessionList.Items())
		if len(rows) != 2 {
			t.Fatalf("len(session rows) = %d, want 2 (one per tag)", len(rows))
		}
		for _, si := range rows {
			if si.CatchAll {
				t.Fatalf("session routed to Untagged catch-all, want resolved under its tags")
			}
			if si.GroupKey == "" {
				t.Errorf("By Tag item has empty GroupKey (canonical tag): %+v", si)
			}
		}
	})

	t.Run("caches the derived directory into m.sessions and never stamps tmux", func(t *testing.T) {
		dir := t.TempDir()
		key := project.CanonicalDirKey(dir)
		projects := []project.Project{{Path: dir, Name: "Portal"}}
		sessions := []tmux.Session{{Name: "portal-abc", Dir: ""}}

		reader := &fakeStamper{path: dir}
		m := newRebuildTestModel(t, prefs.ModeByProject, sessions, projects)
		m.dirReader = reader
		m.dirRunner = &fakeDirRunner{gitRoot: dir}

		m.rebuildSessionList()

		if len(reader.setCalls) != 0 {
			t.Fatalf("expected 0 stamp writes (no freezing), got %d: %v", len(reader.setCalls), reader.setCalls)
		}
		if m.sessions[0].Dir != key {
			t.Errorf("m.sessions[0].Dir = %q, want %q (cached)", m.sessions[0].Dir, key)
		}
	})

	t.Run("second rebuild reuses the cache and performs no further pane read", func(t *testing.T) {
		dir := t.TempDir()
		projects := []project.Project{{Path: dir, Name: "Portal"}}
		sessions := []tmux.Session{{Name: "portal-abc", Dir: ""}}

		reader := &fakeStamper{path: dir}
		m := newRebuildTestModel(t, prefs.ModeByProject, sessions, projects)
		m.dirReader = reader
		m.dirRunner = &fakeDirRunner{gitRoot: dir}

		m.rebuildSessionList()
		if len(reader.reads) != 1 {
			t.Fatalf("first rebuild reads = %d, want 1", len(reader.reads))
		}

		reader.reads = nil
		m.rebuildSessionList()
		if len(reader.reads) != 0 {
			t.Errorf("second rebuild performed %d pane reads, want 0 (cache fast-path)", len(reader.reads))
		}
	})

	// The exact pair tmux gives for a session no pane answers to: display-message
	// exits 0 with an empty expansion, so the reader returns ("", nil) and the
	// grouped render must read that as unresolved rather than as a directory.
	t.Run("it treats an empty current path as unresolved in the grouped render", func(t *testing.T) {
		dir := t.TempDir()
		projects := []project.Project{{Path: dir, Name: "Portal"}}
		sessions := []tmux.Session{{Name: "portal-abc", Dir: ""}}

		m := newRebuildTestModel(t, prefs.ModeByProject, sessions, projects)
		m.dirReader = &fakeStamper{path: "", err: nil}
		m.dirRunner = &fakeDirRunner{gitRoot: dir}

		m.rebuildSessionList()

		rows := sessionRows(m.sessionList.Items())
		if len(rows) != 1 {
			t.Fatalf("len(session rows) = %d, want 1", len(rows))
		}
		if !rows[0].CatchAll {
			t.Fatalf("an empty-and-nil pane read must route the session to the Unknown catch-all")
		}
		if rows[0].GroupHeading != unknownHeading {
			t.Errorf("GroupHeading = %q, want %q (nothing was resolved)", rows[0].GroupHeading, unknownHeading)
		}
		if m.sessions[0].Dir != "" {
			t.Errorf("m.sessions[0].Dir = %q, want \"\" (unresolvable, nothing to cache)", m.sessions[0].Dir)
		}
	})

	t.Run("nil seam does not panic and routes an empty-Dir session to Unknown", func(t *testing.T) {
		dir := t.TempDir()
		projects := []project.Project{{Path: dir, Name: "Portal"}}
		sessions := []tmux.Session{{Name: "portal-abc", Dir: ""}}

		m := newRebuildTestModel(t, prefs.ModeByProject, sessions, projects)

		m.rebuildSessionList()

		rows := sessionRows(m.sessionList.Items())
		if len(rows) != 1 {
			t.Fatalf("len(session rows) = %d, want 1", len(rows))
		}
		if !rows[0].CatchAll {
			t.Fatalf("with a nil seam an empty-Dir session must route to the Unknown catch-all")
		}
	})
}
