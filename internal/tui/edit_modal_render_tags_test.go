package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/project"
)

func renderTagsModel(t *testing.T, focus editField, tags []string, tagCursor int) Model {
	t.Helper()
	return Model{
		themeState:    themeState{active: testDarkTheme(t)},
		modal:         modalEditProject,
		editMode:      editModeNavigate,
		editProject:   project.Project{Name: "Portal"},
		editFocus:     focus,
		editName:      "Portal",
		editTags:      tags,
		editTagCursor: tagCursor,
	}
}

func TestRenderEditProjectContent_TagsBlockAfterAliases(t *testing.T) {
	m := renderTagsModel(t, editFieldTags, []string{"work"}, 0)
	out := ansi.Strip(m.renderEditProjectContent())

	tagsIdx := strings.Index(out, "TAGS")
	aliasesIdx := strings.Index(out, "ALIASES")
	if tagsIdx == -1 {
		t.Fatalf("output missing TAGS heading\n%s", out)
	}
	if aliasesIdx == -1 {
		t.Fatalf("output missing ALIASES heading\n%s", out)
	}
	if tagsIdx < aliasesIdx {
		t.Errorf("TAGS (index %d) should render after ALIASES (index %d)\n%s", tagsIdx, aliasesIdx, out)
	}
}

func TestRenderEditProjectContent_EachTagRendersAsChip(t *testing.T) {
	m := renderTagsModel(t, editFieldTags, []string{"work", "personal"}, 0)
	out := ansi.Strip(m.renderEditProjectContent())

	if !strings.Contains(out, "work") {
		t.Errorf("output missing tag value 'work'\n%s", out)
	}
	if !strings.Contains(out, "personal") {
		t.Errorf("output missing tag value 'personal'\n%s", out)
	}
	if strings.Contains(out, "[x]") {
		t.Errorf("chips must not carry the legacy [x] marker\n%s", out)
	}
}

func TestRenderEditProjectContent_FocusedTagBorderViolet(t *testing.T) {
	m := renderTagsModel(t, editFieldTags, []string{"work", "personal"}, 1)
	content := m.renderEditProjectContent()
	violet := tokenFgSeq(t, m.themeState.active.AccentPrimary)
	if !strings.Contains(content, violet) {
		t.Errorf("focused tag chip should carry the accent.violet border\n%s", ansi.Strip(content))
	}
}

func TestRenderEditProjectContent_EmptyTagsShowsOnlyAddSlot(t *testing.T) {
	m := renderTagsModel(t, editFieldTags, nil, 0)
	out := ansi.Strip(m.renderEditProjectContent())

	tagsIdx := strings.Index(out, "TAGS")
	if tagsIdx == -1 {
		t.Fatalf("output missing TAGS heading\n%s", out)
	}
	tail := out[tagsIdx:]
	if !strings.Contains(tail, "+ add") {
		t.Errorf("empty tags should still render the `+ add` slot\n%s", tail)
	}
	if strings.Contains(out, "(none)") {
		t.Errorf("MV grammar must not render the legacy `(none)` line\n%s", out)
	}
}

func TestRenderEditProjectContent_AddSlotAlwaysRendered(t *testing.T) {
	emptyOut := ansi.Strip(renderTagsModel(t, editFieldTags, nil, 0).renderEditProjectContent())
	if !strings.Contains(emptyOut, "+ add") {
		t.Errorf("`+ add` slot should render with zero tags\n%s", emptyOut)
	}

	fullOut := ansi.Strip(renderTagsModel(t, editFieldTags, []string{"work"}, 0).renderEditProjectContent())
	tagsIdx := strings.Index(fullOut, "TAGS")
	if !strings.Contains(fullOut[tagsIdx:], "+ add") {
		t.Errorf("`+ add` slot should render after TAGS with existing tags\n%s", fullOut)
	}
}

func TestRenderEditProjectContent_NewChipShowsLiveBuffer(t *testing.T) {
	m := renderTagsModel(t, editFieldTags, []string{"work"}, 1)
	m.editMode = editModeEdit
	m.editIsNewChip = true
	m.editBuffer = "draft"
	m.editCursor = len([]rune("draft"))

	out := ansi.Strip(m.renderEditProjectContent())
	if !strings.Contains(out, "draft") {
		t.Errorf("new chip should show its live buffer value\n%s", out)
	}
}

func TestRenderEditProjectContent_TagsHeadingFocusScoped(t *testing.T) {
	m := renderTagsModel(t, editFieldName, []string{"work"}, 0)
	content := m.renderEditProjectContent()
	violet := tokenFgSeq(t, m.themeState.active.AccentPrimary)

	nameSeg := labelSegment(t, content, "NAME")
	if !strings.Contains(nameSeg, violet) {
		t.Errorf("NAME label should be accent.violet when Name focused; seg=%q", nameSeg)
	}
	tagsSeg := labelSegment(t, content, "TAGS")
	if strings.Contains(tagsSeg, violet) {
		t.Errorf("TAGS label should NOT be accent.violet when Name focused; seg=%q", tagsSeg)
	}

	tagsFocused := renderTagsModel(t, editFieldTags, []string{"work"}, 0).renderEditProjectContent()
	if seg := labelSegment(t, tagsFocused, "TAGS"); !strings.Contains(seg, violet) {
		t.Errorf("TAGS label should be accent.violet when Tags focused; seg=%q", seg)
	}
}
