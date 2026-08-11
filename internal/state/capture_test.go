package state_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

type captureMock struct {
	listSessions  string
	listSessionsE error
	listPanes     string
	listPanesE    error
	envBySession  map[string]string
	envErrs       map[string]error
	t             *testing.T

	listSessionsCalls int
	listPanesCalls    int
	showEnvCalls      int
}

func (m *captureMock) Run(args ...string) (string, error) {
	if len(args) == 0 {
		m.t.Fatalf("captureMock invoked with no args")
	}
	switch args[0] {
	case "list-sessions":
		m.listSessionsCalls++
		return m.listSessions, m.listSessionsE
	case "list-panes":
		m.listPanesCalls++
		return m.listPanes, m.listPanesE
	case "show-environment":
		m.showEnvCalls++
		if len(args) < 3 {
			m.t.Fatalf("show-environment called with insufficient args: %v", args)
		}
		session := args[2]
		if err, ok := m.envErrs[session]; ok {
			return "", err
		}
		if out, ok := m.envBySession[session]; ok {
			return out, nil
		}
		return "", nil
	default:
		m.t.Fatalf("captureMock: unexpected command %v", args)
		return "", nil
	}
}

func (m *captureMock) RunRaw(args ...string) (string, error) {
	m.t.Fatalf("captureMock.RunRaw unexpectedly called with %v", args)
	return "", nil
}

func listSessionsFor(names ...string) string {
	lines := make([]string, 0, len(names))
	for _, n := range names {
		lines = append(lines, n+"|1|0|")
	}
	return strings.Join(lines, "\n")
}

func paneLine(session string, windowIdx int, windowName, layout string, zoomed, windowActive bool, paneIdx int, cwd string, paneActive bool, currentCommand string) string {
	return paneLineWithID(session, windowIdx, windowName, layout, zoomed, windowActive, paneIdx, cwd, paneActive, currentCommand, "")
}

func paneLineWithID(session string, windowIdx int, windowName, layout string, zoomed, windowActive bool, paneIdx int, cwd string, paneActive bool, currentCommand, portalID string) string {
	bool01 := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	return fmt.Sprintf(
		"%s|||%d|||%s|||%s|||%s|||%s|||%d|||%s|||%s|||%s|||%s",
		session, windowIdx, windowName, layout, bool01(zoomed), bool01(windowActive), paneIdx, cwd, bool01(paneActive), currentCommand, portalID,
	)
}

func TestCaptureStructure(t *testing.T) {
	t.Run("captures a single session with one window and one pane", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes: paneLine(
				"work", 0, "main", "b25f,200x50,0,0",
				false, true,
				0, "/Users/leeovery/Code/portal", true, "zsh",
			),
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(idx.Sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(idx.Sessions))
		}
		s := idx.Sessions[0]
		if s.Name != "work" {
			t.Errorf("session name = %q, want %q", s.Name, "work")
		}
		if len(s.Windows) != 1 {
			t.Fatalf("got %d windows, want 1", len(s.Windows))
		}
		w := s.Windows[0]
		if w.Index != 0 || w.Name != "main" || w.Layout != "b25f,200x50,0,0" {
			t.Errorf("window header = %+v", w)
		}
		if !w.Active || w.Zoomed {
			t.Errorf("window flags = active:%v zoomed:%v, want true/false", w.Active, w.Zoomed)
		}
		if len(w.Panes) != 1 {
			t.Fatalf("got %d panes, want 1", len(w.Panes))
		}
		p := w.Panes[0]
		if p.Index != 0 || p.CWD != "/Users/leeovery/Code/portal" || !p.Active || p.CurrentCommand != "zsh" {
			t.Errorf("pane = %+v", p)
		}
		if p.ScrollbackFile != "scrollback/work__0.0.bin" {
			t.Errorf("scrollback_file = %q, want %q", p.ScrollbackFile, "scrollback/work__0.0.bin")
		}
	})

	t.Run("filters sessions whose names begin with underscore", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("_portal-saver", "work"),
			listPanes: strings.Join([]string{
				paneLine("_portal-saver", 0, "main", "L1", false, true, 0, "/", true, "portal"),
				paneLine("work", 0, "main", "L2", false, true, 0, "/tmp", true, "zsh"),
			}, "\n"),
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(idx.Sessions))
		}
		if idx.Sessions[0].Name != "work" {
			t.Errorf("session name = %q, want %q", idx.Sessions[0].Name, "work")
		}
	})

	t.Run("returns an empty Sessions slice when zero non-internal sessions exist", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("_portal-saver"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx.Sessions == nil {
			t.Fatal("Sessions is nil; want non-nil empty slice")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("got %d sessions, want 0", len(idx.Sessions))
		}
	})

	t.Run("captures per-session environment from show-environment", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "m", "L", false, true, 0, "/", true, "zsh"),
			envBySession: map[string]string{
				"work": "LANG=en_US.UTF-8\nTERM=xterm-256color",
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := idx.Sessions[0].Environment
		if env["LANG"] != "en_US.UTF-8" {
			t.Errorf("env[LANG] = %q, want %q", env["LANG"], "en_US.UTF-8")
		}
		if env["TERM"] != "xterm-256color" {
			t.Errorf("env[TERM] = %q, want %q", env["TERM"], "xterm-256color")
		}
	})

	t.Run("ignores removed-form environment entries starting with a dash", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "m", "L", false, true, 0, "/", true, "zsh"),
			envBySession: map[string]string{
				"work": "-OLD_VAR\nLANG=en_US.UTF-8",
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := idx.Sessions[0].Environment
		if _, present := env["OLD_VAR"]; present {
			t.Errorf("env contains OLD_VAR; removed-form entries must be skipped")
		}
		if _, present := env["-OLD_VAR"]; present {
			t.Errorf("env contains -OLD_VAR; removed-form entries must be skipped")
		}
		if env["LANG"] != "en_US.UTF-8" {
			t.Errorf("env[LANG] = %q, want %q", env["LANG"], "en_US.UTF-8")
		}
	})

	t.Run("returns an empty Environment map when show-environment output is empty", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "m", "L", false, true, 0, "/", true, "zsh"),
			envBySession: map[string]string{
				"work": "",
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := idx.Sessions[0].Environment
		if env == nil {
			t.Fatal("Environment is nil; want non-nil empty map")
		}
		if len(env) != 0 {
			t.Errorf("got %d env entries, want 0", len(env))
		}
	})

	t.Run("preserves multi-byte UTF-8 characters in session names", func(t *testing.T) {
		const name = "café-проект"
		mock := &captureMock{
			listSessions: listSessionsFor(name),
			listPanes:    paneLine(name, 0, "m", "L", false, true, 0, "/", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != name {
			t.Fatalf("Sessions[0].Name = %q, want %q", idx.Sessions[0].Name, name)
		}
	})

	t.Run("splits environment lines on the first equals sign only", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "m", "L", false, true, 0, "/", true, "zsh"),
			envBySession: map[string]string{
				"work": "X=A=B",
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		env := idx.Sessions[0].Environment
		if env["X"] != "A=B" {
			t.Errorf("env[X] = %q, want %q", env["X"], "A=B")
		}
	})

	t.Run("captures zoomed and active flags per window", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes: strings.Join([]string{
				paneLine("work", 0, "a", "L0", false, true, 0, "/", true, "zsh"),
				paneLine("work", 1, "b", "L1", true, false, 0, "/", true, "zsh"),
			}, "\n"),
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		windows := idx.Sessions[0].Windows
		if len(windows) != 2 {
			t.Fatalf("got %d windows, want 2", len(windows))
		}
		if !windows[0].Active || windows[0].Zoomed {
			t.Errorf("window[0] = %+v, want active:true zoomed:false", windows[0])
		}
		if windows[1].Active || !windows[1].Zoomed {
			t.Errorf("window[1] = %+v, want active:false zoomed:true", windows[1])
		}
	})

	t.Run("captures cwd, active, and current_command per pane", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes: strings.Join([]string{
				paneLine("work", 0, "m", "L", false, true, 0, "/a", true, "zsh"),
				paneLine("work", 0, "m", "L", false, true, 1, "/b", false, "vim"),
			}, "\n"),
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		panes := idx.Sessions[0].Windows[0].Panes
		if len(panes) != 2 {
			t.Fatalf("got %d panes, want 2", len(panes))
		}
		if panes[0].CWD != "/a" || !panes[0].Active || panes[0].CurrentCommand != "zsh" {
			t.Errorf("pane[0] = %+v", panes[0])
		}
		if panes[1].CWD != "/b" || panes[1].Active || panes[1].CurrentCommand != "vim" {
			t.Errorf("pane[1] = %+v", panes[1])
		}
	})

	t.Run("sets scrollback_file via the canonical sanitizer", func(t *testing.T) {
		const session = "foo/bar"
		mock := &captureMock{
			listSessions: listSessionsFor(session),
			listPanes:    paneLine(session, 2, "m", "L", false, true, 3, "/", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := idx.Sessions[0].Windows[0].Panes[0].ScrollbackFile
		want := "scrollback/" + state.SanitizePaneKey(session, 2, 3) + ".bin"
		if got != want {
			t.Errorf("ScrollbackFile = %q, want %q", got, want)
		}
	})

	t.Run("sorts sessions alphabetically by name for stable output", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("zeta", "alpha", "mike"),
			listPanes: strings.Join([]string{
				paneLine("zeta", 0, "m", "L", false, true, 0, "/", true, "zsh"),
				paneLine("alpha", 0, "m", "L", false, true, 0, "/", true, "zsh"),
				paneLine("mike", 0, "m", "L", false, true, 0, "/", true, "zsh"),
			}, "\n"),
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got []string
		for _, s := range idx.Sessions {
			got = append(got, s.Name)
		}
		want := []string{"alpha", "mike", "zeta"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("Sessions[%d].Name = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("sorts windows by index and panes by index ascending", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes: strings.Join([]string{
				paneLine("work", 1, "b", "L1", false, false, 1, "/b1", false, "zsh"),
				paneLine("work", 0, "a", "L0", false, true, 1, "/a1", false, "zsh"),
				paneLine("work", 1, "b", "L1", false, false, 0, "/b0", true, "zsh"),
				paneLine("work", 0, "a", "L0", false, true, 0, "/a0", true, "zsh"),
			}, "\n"),
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		windows := idx.Sessions[0].Windows
		if len(windows) != 2 {
			t.Fatalf("got %d windows, want 2", len(windows))
		}
		if windows[0].Index != 0 || windows[1].Index != 1 {
			t.Errorf("window indices = [%d, %d], want [0, 1]", windows[0].Index, windows[1].Index)
		}
		w0 := windows[0]
		if len(w0.Panes) != 2 || w0.Panes[0].Index != 0 || w0.Panes[1].Index != 1 {
			t.Errorf("window[0] panes = %+v, want indices 0,1 ascending", w0.Panes)
		}
	})

	t.Run("sets Index.Version to the schema constant", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "m", "L", false, true, 0, "/", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx.Version != state.SchemaVersion {
			t.Errorf("Version = %d, want %d", idx.Version, state.SchemaVersion)
		}
	})

	t.Run("sets SavedAt to UTC within the call window", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "m", "L", false, true, 0, "/", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		before := time.Now().UTC()
		idx, err := state.CaptureStructure(client, nil, nil, nil)
		after := time.Now().UTC()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx.SavedAt.Location() != time.UTC {
			t.Errorf("SavedAt.Location() = %v, want UTC", idx.SavedAt.Location())
		}
		if idx.SavedAt.Before(before) || idx.SavedAt.After(after) {
			t.Errorf("SavedAt = %v, want in [%v, %v]", idx.SavedAt, before, after)
		}
	})

	t.Run("returns an error and no partial index when list-panes fails", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanesE:   errors.New("tmux exploded"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("expected no sessions on error, got %d", len(idx.Sessions))
		}
	})

	t.Run("returns an error when show-environment fails for a session", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "m", "L", false, true, 0, "/", true, "zsh"),
			envErrs: map[string]error{
				"work": errors.New("can't find session"),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		_, err := state.CaptureStructure(client, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestCaptureStructurePortalID(t *testing.T) {
	t.Run("it captures a stamped session's @portal-id into Session.PortalID", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes: paneLineWithID(
				"work", 0, "main", "L", false, true, 0, "/tmp", true, "zsh", "aB3xY9kZ",
			),
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(idx.Sessions))
		}
		if idx.Sessions[0].PortalID != "aB3xY9kZ" {
			t.Errorf("PortalID = %q, want %q", idx.Sessions[0].PortalID, "aB3xY9kZ")
		}
	})

	t.Run("it captures an un-stamped session as an empty PortalID", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "main", "L", false, true, 0, "/tmp", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(idx.Sessions))
		}
		if idx.Sessions[0].PortalID != "" {
			t.Errorf("PortalID = %q, want empty for un-stamped session", idx.Sessions[0].PortalID)
		}
	})

	t.Run("it lifts PortalID from the first pane row for a multi-pane session", func(t *testing.T) {
		const id = "sessionScoped1"
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes: strings.Join([]string{
				paneLineWithID("work", 0, "main", "L", false, true, 0, "/a", true, "zsh", id),
				paneLineWithID("work", 0, "main", "L", false, true, 1, "/b", false, "vim", id),
				paneLineWithID("work", 1, "side", "L2", false, false, 0, "/c", true, "bash", id),
			}, "\n"),
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(idx.Sessions))
		}
		if idx.Sessions[0].PortalID != id {
			t.Errorf("PortalID = %q, want %q (lifted once from the first row)", idx.Sessions[0].PortalID, id)
		}
	})

	t.Run("it rejects a wrong-arity pane row after the field-count bump", func(t *testing.T) {
		tenFieldRow := "work|||0|||main|||L|||0|||1|||0|||/tmp|||1|||zsh"
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    tenFieldRow,
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error for a 10-field row under 11-arity, got nil")
		}
		if !strings.Contains(err.Error(), "unexpected pane row field count") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "unexpected pane row field count")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("expected empty Sessions on wrong-arity fail-fatal, got %d", len(idx.Sessions))
		}
	})

	t.Run("it leaves every existing field index unchanged after the append", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes: paneLineWithID(
				"work", 3, "editor", "b25f,200x50,0,0", true, false, 5, "/Users/leeovery/Code/portal", true, "nvim", "keepIndex",
			),
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(idx.Sessions))
		}
		s := idx.Sessions[0]
		if s.Name != "work" {
			t.Errorf("Name = %q, want %q", s.Name, "work")
		}
		if len(s.Windows) != 1 {
			t.Fatalf("got %d windows, want 1", len(s.Windows))
		}
		w := s.Windows[0]
		if w.Index != 3 || w.Name != "editor" || w.Layout != "b25f,200x50,0,0" {
			t.Errorf("window header = %+v, want index 3 / editor / b25f,200x50,0,0", w)
		}
		if !w.Zoomed || w.Active {
			t.Errorf("window flags = zoomed:%v active:%v, want true/false", w.Zoomed, w.Active)
		}
		if len(w.Panes) != 1 {
			t.Fatalf("got %d panes, want 1", len(w.Panes))
		}
		p := w.Panes[0]
		if p.Index != 5 || p.CWD != "/Users/leeovery/Code/portal" || !p.Active || p.CurrentCommand != "nvim" {
			t.Errorf("pane = %+v, want index 5 / portal cwd / active / nvim", p)
		}
		if s.PortalID != "keepIndex" {
			t.Errorf("PortalID = %q, want %q", s.PortalID, "keepIndex")
		}
	})

	t.Run("it yields an empty PortalID and no windows for a zero-row session without panicking", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    "",
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(idx.Sessions))
		}
		s := idx.Sessions[0]
		if s.PortalID != "" {
			t.Errorf("PortalID = %q, want empty for zero-row session", s.PortalID)
		}
		if len(s.Windows) != 0 {
			t.Errorf("got %d windows, want 0 for zero-row session", len(s.Windows))
		}
	})
}

func noSuchSessionErr(session string) error {
	return &tmux.CommandError{
		Stderr: "no such session: " + session,
		Err:    errors.New("exit status 1"),
	}
}

func TestCaptureStructurePerSessionLogAndContinue(t *testing.T) {
	t.Run("it skips a failing session and captures the survivors", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("alpha", "bravo", "charlie"),
			listPanes: strings.Join([]string{
				paneLine("alpha", 0, "m", "L", false, true, 0, "/a", true, "zsh"),
				paneLine("bravo", 0, "m", "L", false, true, 0, "/b", true, "zsh"),
				paneLine("charlie", 0, "m", "L", false, true, 0, "/c", true, "zsh"),
			}, "\n"),
			envErrs: map[string]error{
				"alpha": errors.New("boom"),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 2 {
			t.Fatalf("got %d sessions, want 2 (alpha skipped)", len(idx.Sessions))
		}
		if idx.Sessions[0].Name != "bravo" || idx.Sessions[1].Name != "charlie" {
			t.Errorf("Sessions = [%s, %s], want [bravo, charlie]",
				idx.Sessions[0].Name, idx.Sessions[1].Name)
		}
	})

	t.Run("it proceeds with empty index when every session is natural churn", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("alpha", "bravo"),
			listPanes: strings.Join([]string{
				paneLine("alpha", 0, "m", "L", false, true, 0, "/a", true, "zsh"),
				paneLine("bravo", 0, "m", "L", false, true, 0, "/b", true, "zsh"),
			}, "\n"),
			envErrs: map[string]error{
				"alpha": noSuchSessionErr("alpha"),
				"bravo": noSuchSessionErr("bravo"),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected nil error for natural-churn-only, got %v", err)
		}
		if idx.Sessions == nil {
			t.Fatal("Sessions is nil; want non-nil empty slice")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("got %d sessions, want 0 (all natural churn)", len(idx.Sessions))
		}
	})

	t.Run("it returns an error when every session fails with anomalous errors", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("alpha", "bravo"),
			listPanes: strings.Join([]string{
				paneLine("alpha", 0, "m", "L", false, true, 0, "/a", true, "zsh"),
				paneLine("bravo", 0, "m", "L", false, true, 0, "/b", true, "zsh"),
			}, "\n"),
			envErrs: map[string]error{
				"alpha": errors.New("alpha boom"),
				"bravo": errors.New("bravo boom"),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err == nil {
			t.Fatal("expected non-nil error for all-anomalous, got nil")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("expected empty Sessions on total-failure error, got %d", len(idx.Sessions))
		}
	})

	t.Run("it returns an error when failure set is mixed natural+anomalous and no session succeeded", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("alpha", "bravo"),
			listPanes: strings.Join([]string{
				paneLine("alpha", 0, "m", "L", false, true, 0, "/a", true, "zsh"),
				paneLine("bravo", 0, "m", "L", false, true, 0, "/b", true, "zsh"),
			}, "\n"),
			envErrs: map[string]error{
				"alpha": noSuchSessionErr("alpha"),
				"bravo": errors.New("anomalous"),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err == nil {
			t.Fatal("expected non-nil error for mixed natural+anomalous with 0 successes, got nil")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("expected empty Sessions, got %d", len(idx.Sessions))
		}
	})

	t.Run("it returns nil error and partial index when some sessions succeed despite mixed failures", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("alpha", "bravo", "charlie"),
			listPanes: strings.Join([]string{
				paneLine("alpha", 0, "m", "L", false, true, 0, "/a", true, "zsh"),
				paneLine("bravo", 0, "m", "L", false, true, 0, "/b", true, "zsh"),
				paneLine("charlie", 0, "m", "L", false, true, 0, "/c", true, "zsh"),
			}, "\n"),
			envErrs: map[string]error{
				"alpha": noSuchSessionErr("alpha"),
				"bravo": errors.New("anomalous"),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("expected nil error when ≥1 session succeeded, got %v", err)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "charlie" {
			t.Fatalf("Sessions = %+v, want only [charlie]", idx.Sessions)
		}
	})

	t.Run("it emits a WARN log entry per failing session naming the session and error", func(t *testing.T) {
		dir := t.TempDir()
		logger, sink := openTestLogger(t, dir)

		mock := &captureMock{
			listSessions: listSessionsFor("alpha", "bravo", "charlie"),
			listPanes: strings.Join([]string{
				paneLine("alpha", 0, "m", "L", false, true, 0, "/a", true, "zsh"),
				paneLine("bravo", 0, "m", "L", false, true, 0, "/b", true, "zsh"),
				paneLine("charlie", 0, "m", "L", false, true, 0, "/c", true, "zsh"),
			}, "\n"),
			envErrs: map[string]error{
				"alpha": noSuchSessionErr("alpha"),
				"bravo": errors.New("bravo-boom-sentinel"),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		_, err := state.CaptureStructure(client, nil, nil, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		log := sink.Body()
		warnCount := strings.Count(log, "WARN ")
		if warnCount != 2 {
			t.Errorf("WARN entries = %d, want 2; log:\n%s", warnCount, log)
		}
		if !strings.Contains(log, "session=alpha") {
			t.Errorf("expected WARN for session alpha; log:\n%s", log)
		}
		if !strings.Contains(log, "session=bravo") {
			t.Errorf("expected WARN for session bravo; log:\n%s", log)
		}
		if !strings.Contains(log, "bravo-boom-sentinel") {
			t.Errorf("expected anomalous error text in WARN body; log:\n%s", log)
		}
	})

	t.Run("it does not invoke the per-session loop when keep is empty", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("_portal-saver"),
			envErrs: map[string]error{
				"_portal-saver": errors.New("would fail if iterated"),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx.Sessions == nil {
			t.Fatal("Sessions is nil; want non-nil empty slice")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("got %d sessions, want 0", len(idx.Sessions))
		}
	})

	t.Run("it preserves canonical ordering of surviving sessions", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("zeta", "alpha", "mike"),
			listPanes: strings.Join([]string{
				paneLine("zeta", 0, "m", "L", false, true, 0, "/z", true, "zsh"),
				paneLine("alpha", 0, "m", "L", false, true, 0, "/a", true, "zsh"),
				paneLine("mike", 0, "m", "L", false, true, 0, "/m", true, "zsh"),
			}, "\n"),
			envErrs: map[string]error{
				"mike": noSuchSessionErr("mike"),
			},
			t: t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 2 {
			t.Fatalf("got %d sessions, want 2", len(idx.Sessions))
		}
		if idx.Sessions[0].Name != "alpha" || idx.Sessions[1].Name != "zeta" {
			t.Errorf("Sessions = [%s, %s], want [alpha, zeta] (canonical ascending)",
				idx.Sessions[0].Name, idx.Sessions[1].Name)
		}
	})
}

func findPane(idx state.Index, session string, window, pane int) *state.Pane {
	for si := range idx.Sessions {
		s := &idx.Sessions[si]
		if s.Name != session {
			continue
		}
		for wi := range s.Windows {
			w := &s.Windows[wi]
			if w.Index != window {
				continue
			}
			for pi := range w.Panes {
				if w.Panes[pi].Index == pane {
					return &w.Panes[pi]
				}
			}
		}
	}
	return nil
}

func TestCaptureStructureMergeSkippedPanes(t *testing.T) {
	t.Run("preserves prior pane data when its key is in the skip set", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "work",
				Environment: map[string]string{},
				Windows: []state.Window{{
					Index: 0, Name: "main", Layout: "L", Active: true,
					Panes: []state.Pane{{
						Index:          0,
						CWD:            "/old",
						Active:         true,
						CurrentCommand: "vim",
						ScrollbackFile: "scrollback/work__0.0.bin",
					}},
				}},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "main", "L", false, true, 0, "/new", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey("work", 0, 0): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p := findPane(idx, "work", 0, 0)
		if p == nil {
			t.Fatalf("missing pane work:0.0 in result")
		}
		if p.CWD != "/old" || p.CurrentCommand != "vim" {
			t.Errorf("pane = %+v, want prev's /old + vim (skip-set wins)", p)
		}
	})

	t.Run("merges hydrate-in-progress pane from prev at matching coords", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "work",
				Environment: map[string]string{},
				Windows: []state.Window{{
					Index: 0, Name: "main", Layout: "L", Active: true,
					Panes: []state.Pane{{
						Index:          0,
						CWD:            "/old",
						Active:         true,
						CurrentCommand: "vim",
						ScrollbackFile: "scrollback/work__0.0.bin",
					}},
				}},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "main", "L", false, true, 0, "/new", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey("work", 0, 0): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		p := findPane(idx, "work", 0, 0)
		if p == nil {
			t.Fatalf("missing pane work:0.0 in result")
		}
		if p.CWD != "/old" || p.CurrentCommand != "vim" {
			t.Errorf("pane = %+v, want prev's /old + vim (skip-set wins at matching coords)", p)
		}

		if len(idx.Sessions) != 1 {
			t.Errorf("len(idx.Sessions) = %d, want 1 (no duplication when session is in both fresh and prev)", len(idx.Sessions))
		}
		if idx.Sessions[0].Name != "work" {
			t.Errorf("Sessions[0].Name = %q, want %q", idx.Sessions[0].Name, "work")
		}

		work := idx.Sessions[0]
		if len(work.Windows) != 1 || work.Windows[0].Index != 0 {
			t.Errorf("work.Windows = %+v, want one window at index 0 (canonical ordering)", work.Windows)
		}
		w0 := work.Windows[0]
		if len(w0.Panes) != 1 || w0.Panes[0].Index != 0 {
			t.Errorf("work:0.Panes = %+v, want one pane at index 0 (canonical ordering)", w0.Panes)
		}
	})

	t.Run("does not merge a skipped pane whose session is absent from fresh", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "old",
				Environment: map[string]string{},
				Windows: []state.Window{{
					Index: 1, Name: "win", Layout: "L", Active: true,
					Panes: []state.Pane{{
						Index:          2,
						CWD:            "/prev",
						Active:         true,
						CurrentCommand: "tmux",
						ScrollbackFile: "scrollback/old__1.2.bin",
					}},
				}},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("new"),
			listPanes:    paneLine("new", 0, "n", "L", false, true, 0, "/new", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey("old", 1, 2): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if findPane(idx, "new", 0, 0) == nil {
			t.Errorf("fresh pane new:0.0 missing")
		}
		if p := findPane(idx, "old", 1, 2); p != nil {
			t.Errorf("dead session pane old:1.2 was reintroduced via merge: %+v", p)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "new" {
			t.Errorf("Sessions = %+v, want only fresh 'new'", idx.Sessions)
		}
	})

	t.Run("leaves the fresh capture unchanged when skip set is empty", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "old",
				Environment: map[string]string{},
				Windows: []state.Window{{
					Index: 0, Name: "m", Layout: "L", Active: true,
					Panes: []state.Pane{{
						Index: 0, CWD: "/prev", Active: true, CurrentCommand: "vim",
						ScrollbackFile: "scrollback/old__0.0.bin",
					}},
				}},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "m", "L", false, true, 0, "/new", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, map[string]struct{}{}, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "work" {
			t.Errorf("Sessions = %+v, want only fresh 'work'", idx.Sessions)
		}
	})

	t.Run("leaves the fresh capture unchanged when prev is nil", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "m", "L", false, true, 0, "/new", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{"work__0.0": {}}

		idx, err := state.CaptureStructure(client, skip, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "work" {
			t.Errorf("Sessions = %+v, want only fresh 'work'", idx.Sessions)
		}
		p := findPane(idx, "work", 0, 0)
		if p == nil || p.CWD != "/new" {
			t.Errorf("pane CWD = %v, want /new (no merge applied)", p)
		}
	})

	t.Run("does not merge a skipped pane whose window is absent from a live fresh session", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "work",
				Environment: map[string]string{},
				Windows: []state.Window{
					{
						Index: 0, Name: "main", Layout: "L0", Active: true,
						Panes: []state.Pane{{
							Index:          0,
							CWD:            "/old0",
							Active:         true,
							CurrentCommand: "vim",
							ScrollbackFile: "scrollback/work__0.0.bin",
						}},
					},
					{
						Index: 5, Name: "stale", Layout: "L5", Active: false,
						Panes: []state.Pane{{
							Index:          0,
							CWD:            "/old5",
							Active:         true,
							CurrentCommand: "tmux",
							ScrollbackFile: "scrollback/work__5.0.bin",
						}},
					},
				},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "main", "L0", false, true, 0, "/new", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey("work", 5, 0): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p := findPane(idx, "work", 5, 0); p != nil {
			t.Errorf("dead window pane work:5.0 was reintroduced via merge: %+v", p)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "work" {
			t.Fatalf("Sessions = %+v, want only fresh 'work'", idx.Sessions)
		}
		work := idx.Sessions[0]
		if len(work.Windows) != 1 || work.Windows[0].Index != 0 {
			t.Errorf("work windows = %+v, want only [0]", work.Windows)
		}
	})

	t.Run("drops only stale windows from a mixed prev session", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "work",
				Environment: map[string]string{},
				Windows: []state.Window{
					{
						Index: 0, Name: "main", Layout: "L0", Active: true,
						Panes: []state.Pane{{
							Index:          0,
							CWD:            "/prev0",
							Active:         true,
							CurrentCommand: "vim",
							ScrollbackFile: "scrollback/work__0.0.bin",
						}},
					},
					{
						Index: 7, Name: "stale", Layout: "L7", Active: false,
						Panes: []state.Pane{{
							Index:          0,
							CWD:            "/prev7",
							Active:         true,
							CurrentCommand: "tmux",
							ScrollbackFile: "scrollback/work__7.0.bin",
						}},
					},
				},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "main", "L0", false, true, 0, "/fresh0", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey("work", 0, 0): {},
			state.SanitizePaneKey("work", 7, 0): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p0 := findPane(idx, "work", 0, 0)
		if p0 == nil {
			t.Fatalf("live-window pane work:0.0 missing from merge")
		}
		if p0.CWD != "/prev0" || p0.CurrentCommand != "vim" {
			t.Errorf("work:0.0 = %+v, want prev's /prev0 + vim", p0)
		}
		if p7 := findPane(idx, "work", 7, 0); p7 != nil {
			t.Errorf("dead window pane work:7.0 was reintroduced via merge: %+v", p7)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "work" {
			t.Fatalf("Sessions = %+v, want only 'work'", idx.Sessions)
		}
		work := idx.Sessions[0]
		if len(work.Windows) != 1 || work.Windows[0].Index != 0 {
			t.Errorf("work windows = %+v, want only [0]", work.Windows)
		}
	})

	t.Run("canonical ordering preserved after window-level drop", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "work",
				Environment: map[string]string{},
				Windows: []state.Window{
					{
						Index: 9, Name: "stale", Layout: "L9", Active: false,
						Panes: []state.Pane{{
							Index: 0, CWD: "/prev9", Active: true, CurrentCommand: "tmux",
							ScrollbackFile: "scrollback/work__9.0.bin",
						}},
					},
					{
						Index: 2, Name: "two", Layout: "L2", Active: false,
						Panes: []state.Pane{{
							Index: 0, CWD: "/prev2", Active: true, CurrentCommand: "vim",
							ScrollbackFile: "scrollback/work__2.0.bin",
						}},
					},
					{
						Index: 0, Name: "main", Layout: "L0", Active: true,
						Panes: []state.Pane{{
							Index: 0, CWD: "/prev0", Active: true, CurrentCommand: "zsh",
							ScrollbackFile: "scrollback/work__0.0.bin",
						}},
					},
				},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes: strings.Join([]string{
				paneLine("work", 2, "two", "L2", false, false, 0, "/fresh2", true, "zsh"),
				paneLine("work", 0, "main", "L0", false, true, 0, "/fresh0", true, "zsh"),
			}, "\n"),
			t: t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey("work", 0, 0): {},
			state.SanitizePaneKey("work", 2, 0): {},
			state.SanitizePaneKey("work", 9, 0): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "work" {
			t.Fatalf("Sessions = %+v, want only 'work'", idx.Sessions)
		}
		work := idx.Sessions[0]
		if len(work.Windows) != 2 {
			t.Fatalf("work windows = %+v, want 2 (stale window 9 dropped)", work.Windows)
		}
		if work.Windows[0].Index != 0 || work.Windows[1].Index != 2 {
			t.Errorf("window order = [%d, %d], want [0, 2]",
				work.Windows[0].Index, work.Windows[1].Index)
		}
	})

	t.Run("does not merge a skipped pane whose pane index is absent from a live fresh window", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "work",
				Environment: map[string]string{},
				Windows: []state.Window{{
					Index: 0, Name: "main", Layout: "L0", Active: true,
					Panes: []state.Pane{
						{
							Index:          0,
							CWD:            "/old0",
							Active:         true,
							CurrentCommand: "vim",
							ScrollbackFile: "scrollback/work__0.0.bin",
						},
						{
							Index:          1,
							CWD:            "/old1",
							Active:         false,
							CurrentCommand: "tmux",
							ScrollbackFile: "scrollback/work__0.1.bin",
						},
					},
				}},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    paneLine("work", 0, "main", "L0", false, true, 0, "/new", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey("work", 0, 1): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p := findPane(idx, "work", 0, 1); p != nil {
			t.Errorf("dead pane work:0.1 was reintroduced via merge: %+v", p)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "work" {
			t.Fatalf("Sessions = %+v, want only 'work'", idx.Sessions)
		}
		work := idx.Sessions[0]
		if len(work.Windows) != 1 || work.Windows[0].Index != 0 {
			t.Fatalf("work windows = %+v, want only [0]", work.Windows)
		}
		w0 := work.Windows[0]
		if len(w0.Panes) != 1 || w0.Panes[0].Index != 0 {
			t.Errorf("work:0 panes = %+v, want only [0]", w0.Panes)
		}
	})

	t.Run("canonical ordering preserved after pane-level drop", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "work",
				Environment: map[string]string{},
				Windows: []state.Window{{
					Index: 0, Name: "main", Layout: "L0", Active: true,
					Panes: []state.Pane{
						{
							Index: 9, CWD: "/prev9", Active: false, CurrentCommand: "tmux",
							ScrollbackFile: "scrollback/work__0.9.bin",
						},
						{
							Index: 2, CWD: "/prev2", Active: false, CurrentCommand: "vim",
							ScrollbackFile: "scrollback/work__0.2.bin",
						},
						{
							Index: 0, CWD: "/prev0", Active: true, CurrentCommand: "zsh",
							ScrollbackFile: "scrollback/work__0.0.bin",
						},
					},
				}},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes: strings.Join([]string{
				paneLine("work", 0, "main", "L0", false, true, 2, "/fresh2", false, "zsh"),
				paneLine("work", 0, "main", "L0", false, true, 0, "/fresh0", true, "zsh"),
			}, "\n"),
			t: t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey("work", 0, 0): {},
			state.SanitizePaneKey("work", 0, 2): {},
			state.SanitizePaneKey("work", 0, 9): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "work" {
			t.Fatalf("Sessions = %+v, want only 'work'", idx.Sessions)
		}
		work := idx.Sessions[0]
		if len(work.Windows) != 1 || work.Windows[0].Index != 0 {
			t.Fatalf("work windows = %+v, want only [0]", work.Windows)
		}
		w0 := work.Windows[0]
		if len(w0.Panes) != 2 {
			t.Fatalf("work:0 panes = %+v, want 2 (stale pane 9 dropped)", w0.Panes)
		}
		if w0.Panes[0].Index != 0 || w0.Panes[1].Index != 2 {
			t.Errorf("pane order = [%d, %d], want [0, 2]",
				w0.Panes[0].Index, w0.Panes[1].Index)
		}
	})

	t.Run("self-heals after a killed mid-flight session leaks into prev", func(t *testing.T) {
		// Seeding prev with the killed session is load-bearing: mergeSkippedPanes
		// only resurrects sessions present in prev, so without it this test would
		// pass against the buggy code.
		const killed = "agentic-workflows-XXrJ3J"
		const survivor = "survivor"

		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        killed,
				Environment: map[string]string{},
				Windows: []state.Window{{
					Index: 1, Name: "main", Layout: "L", Active: true,
					Panes: []state.Pane{{
						Index:          1,
						CWD:            "/old",
						Active:         true,
						CurrentCommand: "vim",
						ScrollbackFile: "scrollback/" + state.SanitizePaneKey(killed, 1, 1) + ".bin",
					}},
				}},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor(survivor),
			listPanes:    paneLine(survivor, 0, "main", "L", false, true, 0, "/new", true, "zsh"),
			t:            t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey(killed, 1, 1): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("tick 1: unexpected error: %v", err)
		}
		if p := findPane(idx, killed, 1, 1); p != nil {
			t.Errorf("tick 1: killed session %q reintroduced via merge: %+v", killed, p)
		}
		if findPane(idx, survivor, 0, 0) == nil {
			t.Errorf("tick 1: survivor session %q missing from result", survivor)
		}
		if len(idx.Sessions) != 1 || idx.Sessions[0].Name != survivor {
			t.Fatalf("tick 1: Sessions = %+v, want only %q", idx.Sessions, survivor)
		}

		idx2, err := state.CaptureStructure(client, skip, &idx, nil)
		if err != nil {
			t.Fatalf("tick 2: unexpected error: %v", err)
		}
		if p := findPane(idx2, killed, 1, 1); p != nil {
			t.Errorf("tick 2 (self-heal): killed session %q reintroduced: %+v", killed, p)
		}
		if findPane(idx2, survivor, 0, 0) == nil {
			t.Errorf("tick 2: survivor session %q missing from result", survivor)
		}
		if len(idx2.Sessions) != 1 || idx2.Sessions[0].Name != survivor {
			t.Errorf("tick 2: Sessions = %+v, want only %q", idx2.Sessions, survivor)
		}
	})

	t.Run("re-sorts sessions, windows, and panes after merge", func(t *testing.T) {
		prev := state.Index{
			Version: state.SchemaVersion,
			Sessions: []state.Session{{
				Name:        "zeta",
				Environment: map[string]string{},
				Windows: []state.Window{{
					Index: 0, Name: "z", Layout: "L", Active: true,
					Panes: []state.Pane{{
						Index: 0, CWD: "/prev", Active: true, CurrentCommand: "vim",
						ScrollbackFile: "scrollback/zeta__0.0.bin",
					}},
				}},
			}},
		}
		mock := &captureMock{
			listSessions: listSessionsFor("zeta", "alpha"),
			listPanes: strings.Join([]string{
				paneLine("zeta", 1, "z1", "L", false, false, 1, "/z1.1", false, "zsh"),
				paneLine("zeta", 1, "z1", "L", false, false, 0, "/z1.0", true, "zsh"),
				paneLine("zeta", 0, "z0", "L", false, true, 0, "/z0.0", true, "zsh"),
				paneLine("alpha", 0, "a", "L", false, true, 0, "/a", true, "zsh"),
			}, "\n"),
			t: t,
		}
		client := tmux.NewClient(mock)
		skip := map[string]struct{}{
			state.SanitizePaneKey("zeta", 0, 0): {},
		}

		idx, err := state.CaptureStructure(client, skip, &prev, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(idx.Sessions) != 2 {
			t.Fatalf("got %d sessions, want 2", len(idx.Sessions))
		}
		if idx.Sessions[0].Name != "alpha" || idx.Sessions[1].Name != "zeta" {
			t.Errorf("session order = [%s, %s], want [alpha, zeta]",
				idx.Sessions[0].Name, idx.Sessions[1].Name)
		}
		zeta := idx.Sessions[1]
		if len(zeta.Windows) != 2 || zeta.Windows[0].Index != 0 || zeta.Windows[1].Index != 1 {
			t.Errorf("zeta window order = %+v, want indices [0, 1]", zeta.Windows)
		}
		w1 := zeta.Windows[1]
		if len(w1.Panes) != 2 || w1.Panes[0].Index != 0 || w1.Panes[1].Index != 1 {
			t.Errorf("zeta window 1 pane order = %+v, want indices [0, 1]", w1.Panes)
		}
		p := findPane(idx, "zeta", 0, 0)
		if p == nil || p.CWD != "/prev" || p.CurrentCommand != "vim" {
			t.Errorf("zeta:0.0 = %+v, want prev's /prev + vim", p)
		}
	})
}

// *tmux.Client swallows list-sessions exec errors, so only a bespoke fake can
// drive a ListSessionNames failure. The other two methods fail the test.
type failFastCaptureClient struct {
	t                   *testing.T
	listSessionNames    []string
	listSessionNamesErr error
}

func (f *failFastCaptureClient) ListSessionNames() ([]string, error) {
	return f.listSessionNames, f.listSessionNamesErr
}

func (f *failFastCaptureClient) ListAllPanesWithFormat(format string) (string, error) {
	f.t.Fatalf("ListAllPanesWithFormat unexpectedly called with format %q", format)
	return "", nil
}

func (f *failFastCaptureClient) ShowEnvironment(session string) (string, error) {
	f.t.Fatalf("ShowEnvironment unexpectedly called for session %q", session)
	return "", nil
}

func TestCaptureStructurePreLoopFailFatal(t *testing.T) {
	t.Run("it returns an error when ListSessionNames fails and does not call show-environment", func(t *testing.T) {
		client := &failFastCaptureClient{
			t:                   t,
			listSessionNamesErr: errors.New("exec: tmux broken"),
		}

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error from ListSessionNames failure, got nil")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("expected empty Sessions on pre-loop fail-fatal, got %d", len(idx.Sessions))
		}
	})

	t.Run("it returns an error when ListAllPanesWithFormat fails with non-empty keep", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work", "side"),
			listPanesE:   errors.New("list-panes failed"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error from ListAllPanesWithFormat failure, got nil")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("expected empty Sessions on pre-loop fail-fatal, got %d", len(idx.Sessions))
		}
		if mock.listPanesCalls != 1 {
			t.Errorf("list-panes calls = %d, want 1", mock.listPanesCalls)
		}
		if mock.showEnvCalls != 0 {
			t.Errorf("show-environment calls = %d, want 0 (must not run after ListAllPanesWithFormat failure)", mock.showEnvCalls)
		}
	})

	t.Run("it returns an error when parsePaneRows hits a malformed row", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("work"),
			listPanes:    "work|||0|||main",
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error from parsePaneRows on malformed row, got nil")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("expected empty Sessions on pre-loop fail-fatal, got %d", len(idx.Sessions))
		}
		if mock.showEnvCalls != 0 {
			t.Errorf("show-environment calls = %d, want 0 (must not run after parsePaneRows failure)", mock.showEnvCalls)
		}
	})

	t.Run("it returns an empty index with nil error when keep is empty after filtering", func(t *testing.T) {
		mock := &captureMock{
			listSessions: listSessionsFor("_portal-saver"),
			t:            t,
		}
		client := tmux.NewClient(mock)

		idx, err := state.CaptureStructure(client, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error when keep is empty: %v", err)
		}
		if idx.Sessions == nil {
			t.Fatal("Sessions is nil; want non-nil empty slice")
		}
		if len(idx.Sessions) != 0 {
			t.Errorf("got %d sessions, want 0", len(idx.Sessions))
		}
		if mock.listPanesCalls != 0 {
			t.Errorf("list-panes calls = %d, want 0 (must skip when keep is empty)", mock.listPanesCalls)
		}
		if mock.showEnvCalls != 0 {
			t.Errorf("show-environment calls = %d, want 0 (must skip when keep is empty)", mock.showEnvCalls)
		}
	})
}
