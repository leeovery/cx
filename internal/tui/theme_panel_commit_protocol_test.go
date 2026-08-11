package tui

import (
	"errors"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// The two commit shapes differ only in the write and the key mirror; every
// assertion below is the protocol they share.
type commitShape struct {
	name         string
	commit       func(m *Model, slug string) error
	mirrored     func(slug string) theme.RawKeys
	requireWrote func(t *testing.T, p *fakeThemePersister, slug string)
}

func commitShapes() []commitShape {
	return []commitShape{
		{
			name:     "constant",
			commit:   func(m *Model, slug string) error { return m.commitConstant(slug) },
			mirrored: func(slug string) theme.RawKeys { return theme.RawKeys{Theme: slug} },
			requireWrote: func(t *testing.T, p *fakeThemePersister, slug string) {
				t.Helper()
				requireCommitted(t, p, slug)
			},
		},
		{
			name:     "slot",
			commit:   func(m *Model, slug string) error { return m.commitSlot(slug, theme.MemberDark) },
			mirrored: func(slug string) theme.RawKeys { return theme.RawKeys{Dark: slug} },
			requireWrote: func(t *testing.T, p *fakeThemePersister, slug string) {
				t.Helper()
				requireSlotCommits(t, p, slotCommit{slug: slug, member: theme.MemberDark})
			},
		},
	}
}

func commitProtocolSource(t *testing.T, m Model) *fakeThemeSource {
	t.Helper()

	source, ok := m.themeState.source.(*fakeThemeSource)
	if !ok {
		t.Fatalf("the fixture holds source %T, want the recording fake", m.themeState.source)
	}
	return source
}

func TestCommitProtocol_FailedWriteMovesNothing(t *testing.T) {
	for _, shape := range commitShapes() {
		t.Run(shape.name, func(t *testing.T) {
			rows := arrowValidRows(t, 4)
			persisted, target := rows[0].Slug, rows[2].Slug
			m, persister := newCommitPanelModel(t, rows, persisted)
			source := commitProtocolSource(t, m)
			source.reassembles = 0
			persister.err = errThemeCommitFailed

			err := shape.commit(&m, target)

			if !errors.Is(err, errThemeCommitFailed) {
				t.Errorf("the commit returned %v, want the persister's error — the caller reads the outcome from it", err)
			}
			shape.requireWrote(t, persister, target)
			requireConstantKeys(t, m, persisted)
			if source.reassembles != 0 {
				t.Errorf("the failed commit ran %d reassemblies, want 0 — only a LANDED write recomputes", source.reassembles)
			}
			requireCommitFailedMessage(t, m)
			if !m.themeState.commitFailed {
				t.Error("the failed commit left no outstanding failure; the state runs until a commit SUCCEEDS")
			}
		})
	}
}

func TestCommitProtocol_LandedWriteMirrorsThenRecomputes(t *testing.T) {
	for _, shape := range commitShapes() {
		t.Run(shape.name, func(t *testing.T) {
			rows := arrowValidRows(t, 4)
			persisted, target := rows[0].Slug, rows[2].Slug
			m, persister := newCommitPanelModel(t, rows, persisted)
			source := commitProtocolSource(t, m)
			source.reassembles, source.reassembleKeys = 0, nil

			if err := shape.commit(&m, target); err != nil {
				t.Fatalf("the commit returned %v, want nil", err)
			}

			shape.requireWrote(t, persister, target)
			mirrored := shape.mirrored(target)
			if m.themeState.keys != mirrored {
				t.Errorf("the landed commit left keys %+v, want %+v", m.themeState.keys, mirrored)
			}
			if source.reassembles != 1 {
				t.Fatalf("the landed commit ran %d reassemblies, want 1", source.reassembles)
			}
			if got := source.reassembleKeys[0]; got != mirrored {
				t.Errorf("the recompute was handed keys %+v, want the mirrored %+v — the keys move BEFORE the panel is rebuilt from them", got, mirrored)
			}
			if got := m.themePanel.message; got.Kind != themeMessageNone {
				t.Errorf("the landed commit raised the message %+v, want none", got)
			}
		})
	}
}

func TestCommitProtocol_NilPersisterIsInert(t *testing.T) {
	for _, shape := range commitShapes() {
		t.Run(shape.name, func(t *testing.T) {
			rows := arrowValidRows(t, 4)
			persisted, target := rows[0].Slug, rows[2].Slug
			m := openCommitPanel(t, newArrowPanelDeps(t, rows, persisted), PageSessions, persisted)
			if m.themeState.persister != nil {
				t.Fatalf("fixture: the model holds persister %#v, want none", m.themeState.persister)
			}
			source := commitProtocolSource(t, m)
			source.reassembles = 0
			before := m.View().Content

			if err := shape.commit(&m, target); err != nil {
				t.Errorf("the commit returned %v over a nil persister, want nil — the absence of a WRITER is not a failed write", err)
			}

			requireConstantKeys(t, m, persisted)
			if source.reassembles != 0 {
				t.Errorf("the writer-less commit ran %d reassemblies, want 0", source.reassembles)
			}
			if got := m.themePanel.message; got.Kind != themeMessageNone {
				t.Errorf("the writer-less commit raised the message %+v, want none", got)
			}
			if m.themeState.commitFailed {
				t.Error("the writer-less commit raised an outstanding failure; nothing failed")
			}
			if got := m.View().Content; got != before {
				t.Errorf("the writer-less commit changed the frame\nbefore: %q\nafter:  %q", escSeq(before), escSeq(got))
			}
		})
	}
}
