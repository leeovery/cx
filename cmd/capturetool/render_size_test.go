package main

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/tui"
)

func TestRenderSizeFilter_SubstitutesADeclaredSize(t *testing.T) {
	const termW, termH = 200, 60

	tests := []struct {
		name    string
		fixture string
		wantW   int
		wantH   int
	}{
		{
			name:    "a fixture whose frame is the panel's minimum width",
			fixture: "theme-panel-narrow",
			wantW:   tui.ThemePanelMinWidthTerminal(),
			wantH:   termH,
		},
		{
			name:    "a fixture whose frame is the panel's height floor",
			fixture: "theme-panel-min-height-message",
			wantW:   tui.ThemePanelMinWidthTerminal(),
			wantH:   tui.ThemePanelFloorTerminalHeight(),
		},
		{
			name:    "a fixture that declares no size",
			fixture: "sessions-flat",
			wantW:   termW,
			wantH:   termH,
		},
		{
			name:    "the swatch, which is no fixture at all",
			fixture: capture.ContrastValidationFixture,
			wantW:   termW,
			wantH:   termH,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := renderSizeFilter(tt.fixture)(nil, tea.WindowSizeMsg{Width: termW, Height: termH}).(tea.WindowSizeMsg)
			if !ok {
				t.Fatal("the filter returned something other than a resize")
			}
			if got.Width != tt.wantW || got.Height != tt.wantH {
				t.Errorf("the resize reaches the model as %d×%d, want %d×%d", got.Width, got.Height, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestRenderSizeFilter_PassesEverythingElseThrough(t *testing.T) {
	key := tea.KeyPressMsg{Code: 't', Text: "t"}

	if got := renderSizeFilter("theme-panel-narrow")(nil, key); got != tea.Msg(key) {
		t.Errorf("the filter rewrote a keypress to %#v; it substitutes a render size and nothing else", got)
	}
}

func TestRenderSizeFilter_IsWiredIntoTheProgram(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if want := "tea.WithFilter(renderSizeFilter("; !strings.Contains(string(body), want) {
		t.Errorf("main.go does not pass %s to the program, so a fixture's declared render size never reaches the live model", want)
	}
}
