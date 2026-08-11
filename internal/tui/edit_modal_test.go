package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
)

func editModalStrip(m Model) string {
	return ansi.Strip(m.renderEditProjectContent())
}

func reverseBlockPresent(content string) bool {
	return strings.Contains(content, "\x1b[7m") ||
		strings.Contains(content, ";7m") ||
		strings.Contains(content, "[7;") ||
		strings.Contains(content, "7;")
}

func editModalModel(t *testing.T, focus editField, aliasCur, tagCur int) Model {
	t.Helper()
	return Model{
		themeState:      themeState{active: testDarkTheme(t)},
		modal:           modalEditProject,
		editProject:     project.Project{Name: "flow-v1-api"},
		editMode:        editModeNavigate,
		editFocus:       focus,
		editName:        "flow-v1-api",
		editAliases:     []string{"fapi", "v1"},
		editTags:        []string{"Fabric", "api"},
		editAliasCursor: aliasCur,
		editTagCursor:   tagCur,
	}
}

func TestEditModal_Header(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldName, 0, 0)
		content := m.renderEditProjectContent()
		if !strings.Contains(ansi.Strip(content), "Edit Project flow-v1-api") {
			t.Errorf("[%v] header must read 'Edit Project flow-v1-api'; got:\n%s", themeLabel(th), editModalStrip(m))
		}
		m.themeState.active = th
		content = m.renderEditProjectContent()
		if seq := tokenFgSeq(t, th.TextPrimary); !strings.Contains(content, seq) {
			t.Errorf("[%v] 'Edit Project' must render in text.primary SGR core %q", themeLabel(th), seq)
		}
		if seq := tokenFgSeq(t, th.TextMuted); !strings.Contains(content, seq) {
			t.Errorf("[%v] header <name> must render in text.detail SGR core %q", themeLabel(th), seq)
		}
	}
}

func TestEditModal_SingleBundledModal(t *testing.T) {
	m := editModalModel(t, editFieldName, 0, 0)
	out := editModalStrip(m)
	nameIdx := strings.Index(out, "NAME")
	aliasIdx := strings.Index(out, "ALIASES")
	tagIdx := strings.Index(out, "TAGS")
	if nameIdx < 0 || aliasIdx < 0 || tagIdx < 0 {
		t.Fatalf("modal must carry NAME, ALIASES and TAGS labels; got:\n%s", out)
	}
	if nameIdx >= aliasIdx || aliasIdx >= tagIdx {
		t.Errorf("labels must render NAME → ALIASES → TAGS in order; got:\n%s", out)
	}
}

func TestEditModal_FocusedFieldLabelViolet(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		violet := tokenFgSeq(t, th.AccentPrimary)
		detail := tokenFgSeq(t, th.TextMuted)

		m := editModalModel(t, editFieldName, 0, 0)
		m.themeState.active = th
		if seg := labelSegment(t, m.renderEditProjectContent(), "NAME"); !strings.Contains(seg, violet) {
			t.Errorf("[%v] focused NAME label must be accent.violet; seg=%q", themeLabel(th), seg)
		}
		if seg := labelSegment(t, m.renderEditProjectContent(), "ALIASES"); !strings.Contains(seg, detail) || strings.Contains(seg, violet) {
			t.Errorf("[%v] unfocused ALIASES label must be text.detail (not violet); seg=%q", themeLabel(th), seg)
		}

		mt := editModalModel(t, editFieldTags, 0, 0)
		mt.themeState.active = th
		if seg := labelSegment(t, mt.renderEditProjectContent(), "TAGS"); !strings.Contains(seg, violet) {
			t.Errorf("[%v] focused TAGS label must be accent.violet; seg=%q", themeLabel(th), seg)
		}
	}
}

func labelSegment(t *testing.T, content, label string) string {
	t.Helper()
	for line := range strings.SplitSeq(content, "\n") {
		if strings.Contains(ansi.Strip(line), label) {
			return line
		}
	}
	t.Fatalf("label %q not found in content:\n%s", label, content)
	return ""
}

func TestEditModal_NameInputNeverFilled_GreyUnfocused(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldAliases, 0, 0)
		m.themeState.active = th
		content := m.renderEditProjectContent()
		assertNoFill(t, content, th, "name-input-unfocused")
		if seq := tokenFgSeq(t, th.Border); !strings.Contains(content, seq) {
			t.Errorf("[%v] unfocused NAME box border must be border.separator (grey) SGR core %q", themeLabel(th), seq)
		}
	}
}

func TestEditModal_NameInputFocusedViolet(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldName, 0, 0)
		m.themeState.active = th
		content := m.renderEditProjectContent()
		assertNoFill(t, content, th, "name-input-focused")
		if seq := tokenFgSeq(t, th.AccentPrimary); !strings.Contains(content, seq) {
			t.Errorf("[%v] focused NAME box border must be accent.violet SGR core %q", themeLabel(th), seq)
		}
	}
}

func TestEditModal_NameInputEditingOrangeWithCursor(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldName, 0, 0)
		m.editMode = editModeEdit
		m.editBuffer = "flow-v1-api"
		m.editCursor = len([]rune("flow-v1-api"))
		m.themeState.active = th
		content := m.renderEditProjectContent()
		assertNoFill(t, content, th, "name-input-editing")
		if seq := tokenFgSeq(t, th.AccentAttention); !strings.Contains(content, seq) {
			t.Errorf("[%v] editing NAME box border must be accent.orange SGR core %q", themeLabel(th), seq)
		}
		if !reverseBlockPresent(content) {
			t.Errorf("[%v] editing NAME input must carry a live block cursor (SGR 7)", themeLabel(th))
		}
	}
}

func TestEditModal_ChipNormalGreyNoCross(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldName, 0, 0)
		m.themeState.active = th
		content := m.renderEditProjectContent()
		assertNoFill(t, content, th, "chip-normal")
		assertNoCross(t, content)
		assertNoGreen(t, content, th)
		if seq := tokenFgSeq(t, th.Border); !strings.Contains(content, seq) {
			t.Errorf("[%v] normal chip border must be border.separator (grey) SGR core %q", themeLabel(th), seq)
		}
		if seq := tokenFgSeq(t, th.TextPrimary); !strings.Contains(content, seq) {
			t.Errorf("[%v] chip text must be text.primary SGR core %q", themeLabel(th), seq)
		}
	}
}

func TestEditModal_ChipFocusedVioletNoCross(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldTags, 0, 0)
		m.themeState.active = th
		content := m.renderEditProjectContent()
		assertNoFill(t, content, th, "chip-focused")
		assertNoCross(t, content)
		assertNoGreen(t, content, th)
		if seq := tokenFgSeq(t, th.AccentPrimary); !strings.Contains(content, seq) {
			t.Errorf("[%v] focused chip border must be accent.violet SGR core %q", themeLabel(th), seq)
		}
	}
}

func TestEditModal_ChipEditingOrangeCursorNoCross(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldTags, 0, 0)
		m.editMode = editModeEdit
		m.editBuffer = "Fabric"
		m.editCursor = len([]rune("Fabric"))
		m.themeState.active = th
		content := m.renderEditProjectContent()
		assertNoFill(t, content, th, "chip-editing")
		assertNoCross(t, content)
		assertNoGreen(t, content, th)
		if seq := tokenFgSeq(t, th.AccentAttention); !strings.Contains(content, seq) {
			t.Errorf("[%v] editing chip border must be accent.orange SGR core %q", themeLabel(th), seq)
		}
		if !reverseBlockPresent(content) {
			t.Errorf("[%v] editing chip must carry a live block cursor (SGR 7)", themeLabel(th))
		}
	}
}

func TestEditModal_AddSlotFaint(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldName, 0, 0)
		m.themeState.active = th
		content := m.renderEditProjectContent()
		if !strings.Contains(ansi.Strip(content), "+ add") {
			t.Errorf("[%v] modal must render a `+ add` slot; got:\n%s", themeLabel(th), ansi.Strip(content))
		}
		seg := addSlotSegment(t, content)
		if seq := tokenFgSeq(t, th.TextFaint); !strings.Contains(seg, seq) {
			t.Errorf("[%v] `+ add` slot must be text.faint SGR core %q; seg=%q", themeLabel(th), seq, seg)
		}
	}
}

func addSlotSegment(t *testing.T, content string) string {
	t.Helper()
	for line := range strings.SplitSeq(content, "\n") {
		if strings.Contains(ansi.Strip(line), "+ add") {
			return line
		}
	}
	t.Fatalf("`+ add` slot not found in content:\n%s", content)
	return ""
}

func TestEditModal_EditModeIndicatorOnlyWhileEditing(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		nav := editModalModel(t, editFieldName, 0, 0)
		nav.themeState.active = th
		if strings.Contains(ansi.Strip(nav.renderEditProjectContent()), "EDIT MODE") {
			t.Errorf("[%v] navigate mode must NOT show `◉ EDIT MODE`", themeLabel(th))
		}

		ed := editModalModel(t, editFieldTags, 0, 0)
		ed.editMode = editModeEdit
		ed.editBuffer = "Fabric"
		ed.themeState.active = th
		content := ed.renderEditProjectContent()
		if !strings.Contains(ansi.Strip(content), "◉ EDIT MODE") {
			t.Errorf("[%v] editing must show `◉ EDIT MODE`; got:\n%s", themeLabel(th), ansi.Strip(content))
		}
		seg := labelSegment(t, content, "EDIT MODE")
		if seq := tokenFgSeq(t, th.AccentAttention); !strings.Contains(seg, seq) {
			t.Errorf("[%v] `◉ EDIT MODE` must be accent.orange SGR core %q; seg=%q", themeLabel(th), seq, seg)
		}
	}
}

func TestEditModal_EditModeBadgeRightAligned(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldTags, 0, 0)
		m.editMode = editModeEdit
		m.editBuffer = "Fabric"
		m.themeState.active = th

		headerLine := headerLineOf(t, m.renderEditProjectContent())
		trimmed := strings.TrimRight(headerLine, " │")
		if !strings.HasSuffix(trimmed, "◉ EDIT MODE") {
			t.Errorf("[%v] `◉ EDIT MODE` must be right-aligned (trailing) in the header; got line:\n%q", themeLabel(th), headerLine)
		}
		titleIdx := strings.Index(headerLine, "Edit Project flow-v1-api")
		badgeIdx := strings.Index(headerLine, "◉ EDIT MODE")
		if titleIdx < 0 || badgeIdx < 0 || badgeIdx <= titleIdx {
			t.Fatalf("[%v] header must read title then far-right badge; got:\n%q", themeLabel(th), headerLine)
		}
		gap := badgeIdx - (titleIdx + len("Edit Project flow-v1-api"))
		if gap < 10 {
			t.Errorf("[%v] badge must be far-right with a wide flexible gap after the title (gap=%d); got:\n%q", themeLabel(th), gap, headerLine)
		}
		if badgeIdx < len(headerLine)/2 {
			t.Errorf("[%v] badge must sit in the right half of the header (corner), idx=%d lineLen=%d; got:\n%q", themeLabel(th), badgeIdx, len(headerLine), headerLine)
		}
	}
}

func headerLineOf(t *testing.T, content string) string {
	t.Helper()
	for line := range strings.SplitSeq(ansi.Strip(content), "\n") {
		if strings.Contains(line, "Edit Project") {
			return line
		}
	}
	t.Fatalf("header line not found in content:\n%s", ansi.Strip(content))
	return ""
}

func TestEditModal_PanelWidthStableAcrossModes(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		nav := editModalModel(t, editFieldName, 0, 0)
		nav.themeState.active = th
		navWidth := lipgloss.Width(nav.renderEditProjectContent())

		nameEdit := editModalModel(t, editFieldName, 0, 0)
		nameEdit.editMode = editModeEdit
		nameEdit.editBuffer = "flow-v1-api"
		nameEdit.editCursor = len([]rune("flow-v1-api"))
		nameEdit.themeState.active = th
		if w := lipgloss.Width(nameEdit.renderEditProjectContent()); w != navWidth {
			t.Errorf("[%v] name-edit panel width %d != navigate width %d (entering edit must not resize)", themeLabel(th), w, navWidth)
		}

		chipEdit := editModalModel(t, editFieldTags, 0, 0)
		chipEdit.editMode = editModeEdit
		chipEdit.editBuffer = "Fabric"
		chipEdit.editCursor = len([]rune("Fabric"))
		chipEdit.themeState.active = th
		if w := lipgloss.Width(chipEdit.renderEditProjectContent()); w != navWidth {
			t.Errorf("[%v] chip-edit panel width %d != navigate width %d (entering edit must not resize)", themeLabel(th), w, navWidth)
		}

		chipMid := editModalModel(t, editFieldTags, 0, 0)
		chipMid.editMode = editModeEdit
		chipMid.editBuffer = "Fabric"
		chipMid.editCursor = 2
		chipMid.themeState.active = th
		if w := lipgloss.Width(chipMid.renderEditProjectContent()); w != navWidth {
			t.Errorf("[%v] chip-edit (mid cursor) panel width %d != navigate width %d", themeLabel(th), w, navWidth)
		}
	}
}

func TestEditModal_NameFocusedFooter(t *testing.T) {
	m := editModalModel(t, editFieldName, 0, 0)
	out := editModalStrip(m)
	want := "⏎/e edit · ⇥ next field · esc close"
	if !strings.Contains(out, want) {
		t.Errorf("name-focused footer must read %q; got:\n%s", want, out)
	}
}

func TestEditModal_ChipFocusedFooter(t *testing.T) {
	m := editModalModel(t, editFieldTags, 0, 0)
	out := editModalStrip(m)
	want := "⏎/e edit · x remove · ←→ move · ⇥ next field · esc close"
	if !strings.Contains(out, want) {
		t.Errorf("chip-focused footer must read %q; got:\n%s", want, out)
	}
}

func TestEditModal_AddSlotFocusedFooter(t *testing.T) {
	m := editModalModel(t, editFieldTags, 0, 2)
	out := editModalStrip(m)
	want := "⏎/e edit · ⇥ next field · esc close"
	if !strings.Contains(out, want) {
		t.Errorf("add-slot-focused footer must read %q; got:\n%s", want, out)
	}
	if strings.Contains(out, "x remove") {
		t.Errorf("add-slot-focused footer must NOT carry `x remove`; got:\n%s", out)
	}
}

func TestEditModal_EditingFooter(t *testing.T) {
	m := editModalModel(t, editFieldTags, 0, 0)
	m.editMode = editModeEdit
	m.editBuffer = "Fabric"
	out := editModalStrip(m)
	for _, want := range []string{
		"⏎ save · esc discard · ←→ cursor",
		"empty on save = delete",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("editing footer must contain %q; got:\n%s", want, out)
		}
	}
}

func TestEditModal_EditingFooterConsequenceRightAligned(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldTags, 0, 0)
		m.editMode = editModeEdit
		m.editBuffer = "Fabric"
		m.themeState.active = th

		footerLine := footerLineOf(t, m.renderEditProjectContent())
		leftGroup := "⏎ save · esc discard · ←→ cursor"
		consequence := "empty on save = delete"

		if trimmed := strings.TrimRight(footerLine, " │"); !strings.HasSuffix(trimmed, consequence) {
			t.Errorf("[%v] consequence note must be right-aligned (trailing) in the footer; got line:\n%q", themeLabel(th), footerLine)
		}
		if !strings.Contains(footerLine, leftGroup) {
			t.Fatalf("[%v] footer must carry the left hint group %q; got:\n%q", themeLabel(th), leftGroup, footerLine)
		}
		if li, ci := strings.Index(footerLine, leftGroup), strings.Index(footerLine, consequence); ci <= li {
			t.Fatalf("[%v] footer must read left group then far-right consequence note; got:\n%q", themeLabel(th), footerLine)
		}
		body := footerBetween(t, footerLine, leftGroup, consequence)
		if gap := lipgloss.Width(body); gap <= lipgloss.Width(footerEntrySeparator) {
			t.Errorf("[%v] spacer between left group and consequence note (%d cells) must exceed the inline separator (%d cells) — note must be right-aligned; got:\n%q",
				themeLabel(th), gap, lipgloss.Width(footerEntrySeparator), footerLine)
		}
	}
}

func footerBetween(t *testing.T, line, left, right string) string {
	t.Helper()
	li := strings.Index(line, left)
	ri := strings.Index(line, right)
	if li < 0 || ri < 0 || ri < li+len(left) {
		t.Fatalf("could not locate %q before %q in line:\n%q", left, right, line)
	}
	return line[li+len(left) : ri]
}

func footerLineOf(t *testing.T, content string) string {
	t.Helper()
	for line := range strings.SplitSeq(ansi.Strip(content), "\n") {
		if strings.Contains(line, "⏎ save") {
			return line
		}
	}
	t.Fatalf("footer line not found in content:\n%s", ansi.Strip(content))
	return ""
}

func TestEditModal_FooterKeyGlyphsBlue(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldName, 0, 0)
		m.themeState.active = th
		content := m.renderEditProjectContent()
		if seq := tokenFgSeq(t, th.AccentKey); !strings.Contains(content, seq) {
			t.Errorf("[%v] footer key glyphs must render in accent.blue SGR core %q", themeLabel(th), seq)
		}
		if seq := tokenFgSeq(t, th.TextMuted); !strings.Contains(content, seq) {
			t.Errorf("[%v] footer labels must render in text.detail SGR core %q", themeLabel(th), seq)
		}
	}
}

func TestEditModal_UsesEnterGlyphNotLegacy(t *testing.T) {
	m := editModalModel(t, editFieldName, 0, 0)
	out := editModalStrip(m)
	if !strings.Contains(out, "⏎") {
		t.Errorf("footer must use ⏎ (U+23CE); got:\n%s", out)
	}
	if strings.Contains(out, "↵") {
		t.Errorf("footer must NOT use the legacy ↵ glyph; got:\n%s", out)
	}
}

func TestEditModal_NoLegacyGrammar(t *testing.T) {
	m := editModalModel(t, editFieldTags, 0, 0)
	out := editModalStrip(m)
	for _, legacy := range []string{"[x]", "Add:", "[Enter]", "(none)"} {
		if strings.Contains(out, legacy) {
			t.Errorf("legacy grammar %q must be removed; got:\n%s", legacy, out)
		}
	}
}

func TestEditModal_ZeroChipFieldOnlyAddSlot(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := Model{
			modal:       modalEditProject,
			editProject: project.Project{Name: "flow-v1-api"},
			editMode:    editModeNavigate,
			editFocus:   editFieldTags,
			editName:    "flow-v1-api",
			editAliases: []string{"fapi"},
			editTags:    nil,
			themeState:  themeState{active: th},
		}
		content := m.renderEditProjectContent()
		out := ansi.Strip(content)
		tagsIdx := strings.Index(out, "TAGS")
		if tagsIdx < 0 {
			t.Fatalf("[%v] missing TAGS label; got:\n%s", themeLabel(th), out)
		}
		tail := out[tagsIdx:]
		if !strings.Contains(tail, "+ add") {
			t.Errorf("[%v] zero-chip TAGS must still show `+ add`; got:\n%s", themeLabel(th), tail)
		}
		if seg := labelSegment(t, content, "TAGS"); !strings.Contains(seg, tokenFgSeq(t, th.AccentPrimary)) {
			t.Errorf("[%v] zero-chip focused TAGS label must stay accent.violet; seg=%q", themeLabel(th), seg)
		}
	}
}

func TestEditModal_NewEmptyChipEditingOrangeCursor(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldTags, 0, 2)
		m.editMode = editModeEdit
		m.editIsNewChip = true
		m.editBuffer = ""
		m.editCursor = 0
		m.themeState.active = th
		content := m.renderEditProjectContent()
		assertNoCross(t, content)
		if seq := tokenFgSeq(t, th.AccentAttention); !strings.Contains(content, seq) {
			t.Errorf("[%v] brand-new editing chip must have an accent.orange border", themeLabel(th))
		}
		if !reverseBlockPresent(content) {
			t.Errorf("[%v] brand-new editing chip must carry a live cursor", themeLabel(th))
		}
	}
}

func TestEditModal_NoColorStateViaBorderAndCursor(t *testing.T) {
	focused := editModalModel(t, editFieldTags, 0, 0)
	focused.colourless = true
	fout := focused.renderEditProjectContent()
	if !strings.Contains(ansi.Strip(fout), "┌") && !strings.Contains(ansi.Strip(fout), "╭") {
		t.Errorf("NO_COLOR focused chip must still draw a box border; got:\n%s", ansi.Strip(fout))
	}
	if strings.Contains(ansi.Strip(fout), "EDIT MODE") {
		t.Errorf("NO_COLOR navigate must NOT show EDIT MODE")
	}

	editing := editModalModel(t, editFieldTags, 0, 0)
	editing.editMode = editModeEdit
	editing.editBuffer = "Fabric"
	editing.editCursor = len([]rune("Fabric"))
	editing.colourless = true
	eout := editing.renderEditProjectContent()
	if !strings.Contains(ansi.Strip(eout), "◉ EDIT MODE") {
		t.Errorf("NO_COLOR editing must show `◉ EDIT MODE` text (state not colour-only); got:\n%s", ansi.Strip(eout))
	}
	if !reverseBlockPresent(eout) {
		t.Errorf("NO_COLOR editing must carry a live cursor (state not colour-only)")
	}
}

func TestEditModal_SinglePanelOnClearedCanvas(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		m := editModalModel(t, editFieldName, 0, 0)
		m.themeState.active = th
		placed := ansi.Strip(renderEditModalOnClearedCanvas(m, 100, 40, th, false))
		if got := strings.Count(placed, "╭"); got != 2 {
			t.Errorf("[%v] single-panel edit modal must have exactly 2 rounded top-corners (joined panel + NAME box), got %d; a nested outer box is a regression:\n%s", themeLabel(th), got, placed)
		}
	}
}

func TestEditModal_NoGreenEver(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		states := []Model{
			editModalModel(t, editFieldName, 0, 0),
			editModalModel(t, editFieldTags, 0, 0),
		}
		editing := editModalModel(t, editFieldTags, 0, 0)
		editing.editMode = editModeEdit
		editing.editBuffer = "Fabric"
		states = append(states, editing)
		for i, m := range states {
			m.themeState.active = th
			assertNoGreenLabelled(t, m.renderEditProjectContent(), th, i)
		}
	}
}

func assertNoFill(t *testing.T, content string, th theme.Theme, label string) {
	t.Helper()
	canvasBg := tokenBgSeq(t, th.Canvas)
	for _, forbidden := range []theme.Token{
		th.AccentPrimary, th.AccentAttention, th.Border,
		th.BgSelection, th.StatePositive,
	} {
		seq := tokenBgSeq(t, forbidden)
		if seq == canvasBg {
			continue
		}
		if strings.Contains(content, seq) {
			t.Errorf("[%v/%s] modal must not fill (found %s background SGR %q)", themeLabel(th), label, forbidden.Name, seq)
		}
	}
}

func assertNoCross(t *testing.T, content string) {
	t.Helper()
	if strings.ContainsRune(ansi.Strip(content), '✕') {
		t.Errorf("chips must not render an inline ✕; got:\n%s", ansi.Strip(content))
	}
}

func assertNoGreen(t *testing.T, content string, th theme.Theme) {
	t.Helper()
	assertNoGreenLabelled(t, content, th, 0)
}

func TestEditModalFooterRow_ByteExact(t *testing.T) {
	const wantNavDark = "\x1b[38;2;122;162;247;48;2;11;12;20m⏎/e\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20medit\x1b[m\x1b[38;2;115;122;162;48;2;11;12;20m · \x1b[m\x1b[38;2;122;162;247;48;2;11;12;20m⇥\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mnext field\x1b[m\x1b[38;2;115;122;162;48;2;11;12;20m · \x1b[m\x1b[38;2;122;162;247;48;2;11;12;20mesc\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mclose\x1b[m"
	const wantEditDark = "\x1b[38;2;122;162;247;48;2;11;12;20m⏎\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20msave\x1b[m\x1b[38;2;115;122;162;48;2;11;12;20m · \x1b[m\x1b[38;2;122;162;247;48;2;11;12;20mesc\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mdiscard\x1b[m\x1b[38;2;115;122;162;48;2;11;12;20m · \x1b[m\x1b[38;2;122;162;247;48;2;11;12;20m←→\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mcursor\x1b[m\x1b[48;2;11;12;20m    \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mempty on save = delete\x1b[m"

	t.Run("navigate-name-focused dark", func(t *testing.T) {
		m := editModalModel(t, editFieldName, 0, 0)
		if got := m.editModalFooterRow(testDarkTheme(t), false); got != wantNavDark {
			t.Errorf("navigate footer byte mismatch\n got: %q\nwant: %q", got, wantNavDark)
		}
	})

	t.Run("editing-in-place dark", func(t *testing.T) {
		m := editModalModel(t, editFieldTags, 0, 0)
		m.editMode = editModeEdit
		m.editBuffer = "Fabric"
		m.editCursor = len([]rune("Fabric"))
		if got := m.editModalFooterRow(testDarkTheme(t), false); got != wantEditDark {
			t.Errorf("editing footer byte mismatch\n got: %q\nwant: %q", got, wantEditDark)
		}
	})
}

func assertNoGreenLabelled(t *testing.T, content string, th theme.Theme, idx int) {
	t.Helper()
	if seq := tokenFgSeq(t, th.StatePositive); strings.Contains(content, seq) {
		t.Errorf("[%v/state%d] state.green must never appear on a chip; SGR core %q present", themeLabel(th), idx, seq)
	}
}
