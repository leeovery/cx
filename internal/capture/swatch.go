package capture

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// ContrastValidationFixture is the registered fixture name for the contrast-
// validation swatch — the labelled-tint surface the human eyeball gate judges a
// theme's pinned tints on.
const ContrastValidationFixture = "contrast-validation"

// NewContrastValidationModel builds the contrast-validation swatch tea.Model for
// the theme the capture harness pins via --theme.
//
// It takes a WHOLE PALETTE rather than a mode: a theme carries its own
// canvas, so a light theme is judged against its own near-white surface and a
// dark one against its own near-black, and there is no compiled-in palette left
// for the swatch to fall back to. The result is a tea.Model so the capture tool
// drives it identically to the production model.
func NewContrastValidationModel(th theme.Theme) tea.Model {
	return newSwatchModel(th)
}

// swatchModel is the contrast-validation swatch — a self-contained tea.Model the
// offline capture harness renders for the human visual gate on a theme's pinned
// tints. It is NOT the real Sessions surface, and it deliberately does NOT route
// through tui.Build: it is a standalone validation surface that renders, for each
// of the four pinned tints, a labelled band filled with that tint carrying its
// on-tint foreground text on the theme's own canvas.
//
// The human captures this per theme (vhs) and eyeballs each tint against that
// theme's canvas — the wash-out risk the numeric floors alone cannot catch, which
// is why the light-tint pins are eyeball-established rather than computed. The
// foreground-on-tint pairings are rendered explicitly so they are eyeballed ON
// their tints, not just numerically verified.
type swatchModel struct {
	th theme.Theme

	// width/height cache the terminal size from the first WindowSizeMsg so the
	// frame fills the screen on the painted canvas. A fixed fallback keeps a direct
	// render (tests) deterministic without a size message.
	width  int
	height int
}

// newSwatchModel builds the swatch for one palette. The palette pins both the
// owned canvas and every tint and foreground rendered on it.
func newSwatchModel(th theme.Theme) swatchModel {
	return swatchModel{th: th}
}

// canvasHex returns the hex of the owned canvas this swatch paints — the
// theme's own `canvas` token. Exposed for the harness/tests to assert the
// correct canvas is selected.
func (s swatchModel) canvasHex() string {
	return s.th.Canvas.Value
}

// Init returns no command — the swatch is a static, deterministic render (no tmux,
// no detection, no timers).
func (s swatchModel) Init() tea.Cmd { return nil }

// Update caches the terminal size and quits on q / ctrl+c / esc so vhs can drive
// the capture, then exit cleanly.
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

// View paints the swatch on the owned canvas. It mirrors tui.Model.View: the
// frame is alt-screen and the screen background is set (OSC 11) to the theme's
// canvas, so the tints are eyeballed against the exact surface they will render
// against rather than an arbitrary terminal bg.
func (s swatchModel) View() tea.View {
	v := tea.NewView(s.fillCanvas(renderSwatch(s.th)))
	v.AltScreen = true
	v.BackgroundColor = s.th.Canvas.Color()
	return v
}

// fillCanvas pads the rendered swatch to the cached terminal size, backfilling
// every cell with the owned-canvas background so the whole frame (not just the
// band rows) reads on the canvas — the same invariant tui.Model enforces. A fixed
// 80x24 fallback keeps a sizeless render bounded.
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

// bandWidth is the fixed cell width of each swatch band — wide enough to carry
// the label + the on-tint sample text, narrow enough to leave the canvas visible
// on either side (so the tint reads AS a band against the canvas, not edge-to-edge
// fill).
const bandWidth = 56

// The sample content each band lays over its tint. They are constants rather
// than literals because the bands and the tests that assert the pairings render
// the same text through the same token, and a second copy of a sample string is
// a second thing to keep in step.
const (
	swatchSessionName  = "portal-9fk2"
	swatchSessionPath  = "~/code/portal"
	swatchWindowCount  = "3 windows"
	swatchAttached     = "● attached"
	swatchAttentionMsg = "⚠ state saver is not running — restore may be incomplete"
)

// renderSwatch is the pure render of the contrast-validation swatch for one
// palette. It composes a title, the four pinned-tint bands (each with its
// on-tint foreground pairings labelled) and the border rule, all on the theme's
// own canvas. Kept pure (theme-in, string-out) so it is unit-testable without a
// tea.Program.
//
// The token names are written as literals here — the label a human reads off a
// capture is the name they will type into a theme file, so it is not derived from
// a Token.Name (a Theme built by hand carries values without names).
func renderSwatch(th theme.Theme) string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Foreground(th.TextPrimary.Color()).
		Background(th.Canvas.Color()).
		Bold(true).
		Render(fmt.Sprintf("CONTRAST VALIDATION — canvas %s", th.Canvas.Value))
	b.WriteString(title)
	b.WriteString("\n\n")

	// bg.selection: the selected session row, carrying every foreground-on-tint
	// pairing that tint has — the name (text.on-selection),
	// the path (text.tertiary), the window count (text.secondary) and the
	// `● attached` marker (state.positive, the single token that must clear
	// against the canvas AND against this tint).
	b.WriteString(tintLabel(th, "bg.selection", th.BgSelection))
	b.WriteString("\n")
	b.WriteString(selectionBand(th))
	b.WriteString("\n")
	// The caption lists the four in the band's own left-to-right order, so the
	// order IS the mapping and the line stays inside 80 cells.
	b.WriteString(caption(th, "fg-on-tint: text.on-selection · text.tertiary · text.secondary · state.positive"))
	b.WriteString("\n\n")

	// bg.attention: the warning-flash band, carrying its one pairing — the
	// message in text.on-attention.
	b.WriteString(tintLabel(th, "bg.attention", th.BgAttention))
	b.WriteString("\n")
	b.WriteString(attentionBand(th))
	b.WriteString("\n")
	b.WriteString(caption(th, "fg-on-tint: text.on-attention (⚠ message)"))
	b.WriteString("\n\n")

	// bg.subtle: the loading-bar empty track with its accent.primary fill over
	// it, so the bar reads against the track and the track against the canvas.
	// Nothing renders text on this tint, so it carries no pairing.
	b.WriteString(tintLabel(th, "bg.subtle", th.BgSubtle))
	b.WriteString("\n")
	b.WriteString(subtleBand(th))
	b.WriteString("\n")
	b.WriteString(caption(th, "bar: accent.primary over the track"))
	b.WriteString("\n\n")

	// border: the sole border token — one rule on the canvas, where there were a
	// separator and a footer rule before.
	b.WriteString(tintLabel(th, "border", th.Border))
	b.WriteString("\n")
	b.WriteString(borderRule(th))

	return b.String()
}

// tintLabel renders the token name + its hex above its band, so the human reads
// exactly which token + value the band below is.
func tintLabel(th theme.Theme, name string, tok theme.Token) string {
	return caption(th, fmt.Sprintf("%s  %s", name, tok.Value))
}

// caption renders one line of text.muted on the canvas — the label above each
// band and the pairing legend below it, which together are how the human knows
// which token each surface and each on-tint foreground is.
func caption(th theme.Theme, text string) string {
	return onCanvas(th, th.TextMuted).Render(text)
}

// onCanvas is the style for a foreground token rendered directly on the theme's
// canvas — the labels and captions between the bands.
func onCanvas(th theme.Theme, tok theme.Token) lipgloss.Style {
	return onTint(tok, th.Canvas.Color())
}

// onTint is the style for a foreground token rendered ON a background. Every
// pairing goes through it, which is what guarantees the pairing is eyeballed
// against its tint rather than against the canvas.
func onTint(tok theme.Token, tint color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(tok.Color()).Background(tint)
}

// selectionBand fills bandWidth cells with the bg.selection tint and lays every
// selected-row foreground over it: the name (text.on-selection), the path
// (text.tertiary), the window count (text.secondary) and the `● attached`
// marker (state.positive). Each run keeps the tint as its background so every
// pairing is rendered ON the tint.
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

// attentionBand fills bandWidth cells with the bg.attention tint and lays the
// warning message in text.on-attention over it.
func attentionBand(th theme.Theme) string {
	tint := th.BgAttention.Color()
	return padBand(onTint(th.TextOnAttention, tint).Render(swatchAttentionMsg), tint)
}

// subtleBarWidth is how much of the bg.subtle band the accent.primary bar
// fills, leaving the rest as empty track — enough of each for both to be judged.
const subtleBarWidth = 18

// subtleBand renders the loading-bar track: a filled head (accent.primary, the
// bar token) over the bg.subtle empty track, so the human eyeballs the bar
// against the track and the track against the canvas.
func subtleBand(th theme.Theme) string {
	bar := lipgloss.NewStyle().
		Background(th.AccentPrimary.Color()).
		Render(strings.Repeat(" ", subtleBarWidth))
	empty := lipgloss.NewStyle().
		Background(th.BgSubtle.Color()).
		Render(strings.Repeat(" ", bandWidth-subtleBarWidth))
	return bar + empty
}

// borderRule renders a full-bandWidth rule in the sole border token over the
// canvas, so the rule's perceptibility against the canvas is eyeballed.
func borderRule(th theme.Theme) string {
	return onCanvas(th, th.Border).Render(strings.Repeat("─", bandWidth))
}

// padBand right-pads content to bandWidth cells, keeping the tint background
// across the padding so the band is a continuous filled surface.
func padBand(content string, tint color.Color) string {
	gap := bandWidth - lipgloss.Width(content)
	if gap > 0 {
		content += lipgloss.NewStyle().Background(tint).Render(strings.Repeat(" ", gap))
	}
	return content
}
