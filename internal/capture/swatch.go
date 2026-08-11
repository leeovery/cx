package capture

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// ContrastValidationFixture is the registered fixture name for the
// contrast-validation swatch.
const ContrastValidationFixture = "contrast-validation"

// NewContrastValidationModel builds the contrast-validation swatch tea.Model
// for the given theme. It takes a whole palette rather than a mode: each
// theme's tints are judged against its own canvas.
func NewContrastValidationModel(th theme.Theme) tea.Model {
	return newSwatchModel(th)
}

// Deliberately not routed through tui.Build — a standalone validation surface.
type swatchModel struct {
	th theme.Theme

	width  int
	height int
}

func newSwatchModel(th theme.Theme) swatchModel {
	return swatchModel{th: th}
}

func (s swatchModel) canvasHex() string {
	return s.th.Canvas.Value
}

func (s swatchModel) Init() tea.Cmd { return nil }

func (s swatchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return s, tea.Quit
		}
	}
	return s, nil
}

// The terminal background is set (OSC 11) to the theme's canvas so tints are
// judged against the surface they will render on, not an arbitrary bg.
func (s swatchModel) View() tea.View {
	v := tea.NewView(s.fillCanvas(renderSwatch(s.th)))
	v.AltScreen = true
	v.BackgroundColor = s.th.Canvas.Color()
	return v
}

// A fixed 80x24 fallback keeps a sizeless render bounded.
func (s swatchModel) fillCanvas(view string) string {
	w, h := s.width, s.height
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}
	canvasStyle := lipgloss.NewStyle().Background(s.th.Canvas.Color())
	blank := canvasStyle.Render(strings.Repeat(" ", w))
	lines := strings.Split(view, "\n")
	out := make([]string, 0, h)
	for _, line := range lines {
		if len(out) == h {
			break
		}
		gap := w - lipgloss.Width(line)
		if gap > 0 {
			line += canvasStyle.Render(strings.Repeat(" ", gap))
		}
		out = append(out, line)
	}
	for len(out) < h {
		out = append(out, blank)
	}
	return strings.Join(out, "\n")
}

// Narrow enough to leave the canvas visible on either side, so the tint reads
// as a band rather than edge-to-edge fill.
const bandWidth = 56

const (
	swatchSessionName  = "portal-9fk2"
	swatchSessionPath  = "~/code/portal"
	swatchWindowCount  = "3 windows"
	swatchAttached     = "● attached"
	swatchAttentionMsg = "⚠ state saver is not running — restore may be incomplete"
)

// Token names are literals — a hand-built Theme carries values without names,
// and the label is exactly what a user types into a theme file.
func renderSwatch(th theme.Theme) string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Foreground(th.TextPrimary.Color()).
		Background(th.Canvas.Color()).
		Bold(true).
		Render(fmt.Sprintf("CONTRAST VALIDATION — canvas %s", th.Canvas.Value))
	b.WriteString(title)
	b.WriteString("\n\n")

	b.WriteString(tintLabel(th, "bg.selection", th.BgSelection))
	b.WriteString("\n")
	b.WriteString(selectionBand(th))
	b.WriteString("\n")
	// Caption order matches the band's left-to-right order — the order is the mapping.
	b.WriteString(caption(th, "fg-on-tint: text.on-selection · text.tertiary · text.secondary · state.positive"))
	b.WriteString("\n\n")

	b.WriteString(tintLabel(th, "bg.attention", th.BgAttention))
	b.WriteString("\n")
	b.WriteString(attentionBand(th))
	b.WriteString("\n")
	b.WriteString(caption(th, "fg-on-tint: text.on-attention (⚠ message)"))
	b.WriteString("\n\n")

	b.WriteString(tintLabel(th, "bg.subtle", th.BgSubtle))
	b.WriteString("\n")
	b.WriteString(subtleBand(th))
	b.WriteString("\n")
	b.WriteString(caption(th, "bar: accent.primary over the track"))
	b.WriteString("\n\n")

	b.WriteString(tintLabel(th, "border", th.Border))
	b.WriteString("\n")
	b.WriteString(borderRule(th))

	return b.String()
}

func tintLabel(th theme.Theme, name string, tok theme.Token) string {
	return caption(th, fmt.Sprintf("%s  %s", name, tok.Value))
}

func caption(th theme.Theme, text string) string {
	return onCanvas(th, th.TextMuted).Render(text)
}

func onCanvas(th theme.Theme, tok theme.Token) lipgloss.Style {
	return onTint(tok, th.Canvas.Color())
}

func onTint(tok theme.Token, tint color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tok.Color()).Background(tint)
}

func selectionBand(th theme.Theme) string {
	tint := th.BgSelection.Color()
	gap := onTint(th.TextOnSelection, tint).Render("  ")
	content := strings.Join([]string{
		onTint(th.TextOnSelection, tint).Bold(true).Render(swatchSessionName),
		onTint(th.TextTertiary, tint).Render(swatchSessionPath),
		onTint(th.TextSecondary, tint).Render(swatchWindowCount),
		onTint(th.StatePositive, tint).Render(swatchAttached),
	}, gap)
	return padBand(content, tint)
}

func attentionBand(th theme.Theme) string {
	tint := th.BgAttention.Color()
	return padBand(onTint(th.TextOnAttention, tint).Render(swatchAttentionMsg), tint)
}

// Leaves enough filled bar and empty track for both to be judged.
const subtleBarWidth = 18

// The loading-bar shape: an accent.primary head over the bg.subtle track.
func subtleBand(th theme.Theme) string {
	bar := lipgloss.NewStyle().
		Background(th.AccentPrimary.Color()).
		Render(strings.Repeat(" ", subtleBarWidth))
	empty := lipgloss.NewStyle().
		Background(th.BgSubtle.Color()).
		Render(strings.Repeat(" ", bandWidth-subtleBarWidth))
	return bar + empty
}

func borderRule(th theme.Theme) string {
	return onCanvas(th, th.Border).Render(strings.Repeat("─", bandWidth))
}

func padBand(content string, tint color.Color) string {
	gap := bandWidth - lipgloss.Width(content)
	if gap > 0 {
		content += lipgloss.NewStyle().Background(tint).Render(strings.Repeat(" ", gap))
	}
	return content
}
