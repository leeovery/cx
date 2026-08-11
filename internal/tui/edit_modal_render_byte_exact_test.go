package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/project"
)

func TestRenderEditProjectContent_ByteExact(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m *Model)
		want  string
	}{
		{
			name:  "navigate-name-focused",
			setup: func(m *Model) {},
			want: "╭──────────────────────────────────────────────────────────────╮\n" +
				"│  Edit Project flow-v1-api                                    │\n" +
				"├──────────────────────────────────────────────────────────────┤\n" +
				"│  NAME                                                        │\n" +
				"│  ╭──────────────────────────────────────────────────────╮    │\n" +
				"│  │ flow-v1-api                                          │    │\n" +
				"│  ╰──────────────────────────────────────────────────────╯    │\n" +
				"│                                                              │\n" +
				"│  ALIASES                                                     │\n" +
				"│  ┌──────┐ ┌────┐                                             │\n" +
				"│  │ fapi │ │ v1 │ + add                                       │\n" +
				"│  └──────┘ └────┘                                             │\n" +
				"│                                                              │\n" +
				"│  TAGS                                                        │\n" +
				"│  ┌────────┐ ┌─────┐                                          │\n" +
				"│  │ Fabric │ │ api │ + add                                    │\n" +
				"│  └────────┘ └─────┘                                          │\n" +
				"├──────────────────────────────────────────────────────────────┤\n" +
				"│  ⏎/e edit · ⇥ next field · esc close                         │\n" +
				"╰──────────────────────────────────────────────────────────────╯",
		},
		{
			name: "navigate-tag-chip-focused",
			setup: func(m *Model) {
				m.editFocus = editFieldTags
				m.editTagCursor = 0
			},
			want: "╭──────────────────────────────────────────────────────────────╮\n" +
				"│  Edit Project flow-v1-api                                    │\n" +
				"├──────────────────────────────────────────────────────────────┤\n" +
				"│  NAME                                                        │\n" +
				"│  ╭──────────────────────────────────────────────────────╮    │\n" +
				"│  │ flow-v1-api                                          │    │\n" +
				"│  ╰──────────────────────────────────────────────────────╯    │\n" +
				"│                                                              │\n" +
				"│  ALIASES                                                     │\n" +
				"│  ┌──────┐ ┌────┐                                             │\n" +
				"│  │ fapi │ │ v1 │ + add                                       │\n" +
				"│  └──────┘ └────┘                                             │\n" +
				"│                                                              │\n" +
				"│  TAGS                                                        │\n" +
				"│  ┌────────┐ ┌─────┐                                          │\n" +
				"│  │ Fabric │ │ api │ + add                                    │\n" +
				"│  └────────┘ └─────┘                                          │\n" +
				"├──────────────────────────────────────────────────────────────┤\n" +
				"│  ⏎/e edit · x remove · ←→ move · ⇥ next field · esc close    │\n" +
				"╰──────────────────────────────────────────────────────────────╯",
		},
		{
			name: "editing-tag-chip",
			setup: func(m *Model) {
				m.editFocus = editFieldTags
				m.editMode = editModeEdit
				m.editTagCursor = 0
				m.editBuffer = "Fabric"
				m.editCursor = len([]rune("Fabric"))
			},
			want: "╭──────────────────────────────────────────────────────────────╮\n" +
				"│  Edit Project flow-v1-api                       ◉ EDIT MODE  │\n" +
				"├──────────────────────────────────────────────────────────────┤\n" +
				"│  NAME                                                        │\n" +
				"│  ╭──────────────────────────────────────────────────────╮    │\n" +
				"│  │ flow-v1-api                                          │    │\n" +
				"│  ╰──────────────────────────────────────────────────────╯    │\n" +
				"│                                                              │\n" +
				"│  ALIASES                                                     │\n" +
				"│  ┌──────┐ ┌────┐                                             │\n" +
				"│  │ fapi │ │ v1 │ + add                                       │\n" +
				"│  └──────┘ └────┘                                             │\n" +
				"│                                                              │\n" +
				"│  TAGS                                                        │\n" +
				"│  ┌─────────┐ ┌─────┐                                         │\n" +
				"│  │ Fabric  │ │ api │ + add                                   │\n" +
				"│  └─────────┘ └─────┘                                         │\n" +
				"├──────────────────────────────────────────────────────────────┤\n" +
				"│  ⏎ save · esc discard · ←→ cursor    empty on save = delete  │\n" +
				"╰──────────────────────────────────────────────────────────────╯",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				themeState:  themeState{active: testDarkTheme(t)},
				modal:       modalEditProject,
				editProject: project.Project{Name: "flow-v1-api"},
				editMode:    editModeNavigate,
				editFocus:   editFieldName,
				editName:    "flow-v1-api",
				editAliases: []string{"fapi", "v1"},
				editTags:    []string{"Fabric", "api"},
			}
			tc.setup(&m)
			got := ansi.Strip(m.renderEditProjectContent())
			if got != tc.want {
				t.Errorf("render mismatch\n got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}
