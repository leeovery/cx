package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// The consolidation gate for keyColumnRow — the one builder behind BOTH key-column
// surfaces: the help modal's two-column body row and the theme panel's vertical
// footer row. Each used to compose the same measure/pad/gap/join sequence
// independently, differing only in tokens, boldness, column width and gap.
//
// preHelpModalRow and preThemePanelFooterRow reproduce the ORIGINAL bodies
// verbatim, and every descriptor entry either surface can be handed is asserted
// against them across both palettes and colourless on/off — so a drift in the pad
// arithmetic, the measurement, the canvas painting or the segment order is caught
// byte-for-byte.
//
// No t.Parallel() — the shared canvas helpers make parallelism unsafe.

// preHelpModalRow reproduces the ORIGINAL helpModalRow body verbatim — the golden
// the extraction must preserve.
func preHelpModalRow(e keymapEntry, th theme.Theme, colourless bool) string {
	keyTok := th.AccentKey
	if isDestructiveHelpKey(e) {
		keyTok = th.StateDestructive
	}
	key := headerStyle(keyTok, th, colourless).Bold(true).Render(helpKeyGlyph(e))
	keyWidth := lipgloss.Width(key)
	pad := ""
	if keyWidth < helpKeyColumnWidth {
		pad = headerCanvasBg(th, colourless).Render(strings.Repeat(" ", helpKeyColumnWidth-keyWidth))
	}
	gap := headerCanvasBg(th, colourless).Render(helpColumnGap)
	label := headerStyle(th.TextSecondary, th, colourless).Render(helpActionLabel(e))
	return lipgloss.JoinHorizontal(lipgloss.Top, key, pad, gap, label)
}

// preThemePanelFooterRow reproduces the ORIGINAL themePanelFooterRow body verbatim —
// the golden the extraction must preserve, pad-to-width wrap included.
func preThemePanelFooterRow(e keymapEntry, width int, th theme.Theme, colourless bool) string {
	key := headerStyle(th.AccentKey, th, colourless).Render(helpKeyGlyph(e))
	keyWidth := lipgloss.Width(key)
	pad := ""
	if keyWidth < themePanelFooterKeyColumnWidth {
		pad = headerCanvasBg(th, colourless).Render(spaces(themePanelFooterKeyColumnWidth - keyWidth))
	}
	gap := headerCanvasBg(th, colourless).Render(footerKeyLabelGap)
	label := headerStyle(th.TextMuted, th, colourless).Render(e.Action)

	row := lipgloss.JoinHorizontal(lipgloss.Top, key, pad, gap, label)
	return headerPadRight(row, lipgloss.Width(row), width, th, colourless)
}

// keyColumnRowEntries is every descriptor entry either surface can hand a row
// builder — the two page scopes and the preview scope the help body renders, plus
// the panel scope and the confirm scope substituted into its footer.
//
// BOTH surfaces are exercised over the WHOLE set rather than each over its own
// scope: the row builders are entry-agnostic, and the widest glyphs (`^↑/↓`) are
// what drive the panel's narrow column past its width — the pad-omission branch
// its own four rows only reach on `esc`.
func keyColumnRowEntries() []keymapEntry {
	scopes := [][]keymapEntry{
		sessionsKeymap(),
		projectsKeymap(),
		previewKeymap(),
		themePanelKeymap(),
		themePanelConfirmKeymap(),
	}
	all := make([]keymapEntry, 0)
	for _, scope := range scopes {
		all = append(all, scope...)
	}
	return all
}

// TestHelpModalRow_ByteIdenticalAcrossExtraction asserts the post-extraction
// helpModalRow reproduces the pre-extraction body byte-for-byte for every
// descriptor entry, both palettes, colourless on/off — the destructive key token
// and the bold key glyph included.
func TestHelpModalRow_ByteIdenticalAcrossExtraction(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		for _, colourless := range []bool{false, true} {
			for _, e := range keyColumnRowEntries() {
				want := preHelpModalRow(e, th, colourless)
				if got := helpModalRow(e, th, colourless); got != want {
					t.Errorf("[%v col=%v] help row for %q drifted from the pre-extraction golden\n got: %q\nwant: %q",
						themeLabel(th), colourless, e.Key, got, want)
				}
			}
		}
	}
}

// TestThemePanelFooterRow_ByteIdenticalAcrossExtraction asserts the
// post-extraction themePanelFooterRow reproduces the pre-extraction body
// byte-for-byte across the panel's inner-width band, both palettes and colourless
// on/off. Width 0 is in the band because the footer's self-measured height renders
// the block at it.
func TestThemePanelFooterRow_ByteIdenticalAcrossExtraction(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		for _, colourless := range []bool{false, true} {
			for _, width := range []int{0, themePanelFooterTestMinWidth, themePanelFooterTestWidth} {
				for _, e := range keyColumnRowEntries() {
					want := preThemePanelFooterRow(e, width, th, colourless)
					if got := themePanelFooterRow(e, width, th, colourless); got != want {
						t.Errorf("[%v col=%v w=%d] panel footer row for %q drifted from the pre-extraction golden\n got: %q\nwant: %q",
							themeLabel(th), colourless, width, e.Key, got, want)
					}
				}
			}
		}
	}
}

// TestKeyColumnRow_BuildsBothSurfacesRows asserts BOTH callers delegate to the one
// builder: each surface's row is exactly keyColumnRow handed that surface's own
// styles, column width and gap.
//
// The help side passes its destructive-aware key token and its bold key glyph as an
// already-resolved STYLE, which is the point of the seam — the divergence lives at
// the call site, so the builder carries no branch for either.
func TestKeyColumnRow_BuildsBothSurfacesRows(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		for _, colourless := range []bool{false, true} {
			for _, e := range keyColumnRowEntries() {
				keyTok := th.AccentKey
				if e.Destructive {
					keyTok = th.StateDestructive
				}
				help := keyColumnRow(
					helpKeyGlyph(e), helpActionLabel(e),
					headerStyle(keyTok, th, colourless).Bold(true),
					headerStyle(th.TextSecondary, th, colourless),
					helpKeyColumnWidth, helpColumnGap, th, colourless)
				if got := helpModalRow(e, th, colourless); got != help {
					t.Errorf("[%v col=%v] help row for %q is not the shared builder's row\n got: %q\nwant: %q",
						themeLabel(th), colourless, e.Key, got, help)
				}

				panel := keyColumnRow(
					helpKeyGlyph(e), e.Action,
					headerStyle(th.AccentKey, th, colourless),
					headerStyle(th.TextMuted, th, colourless),
					themePanelFooterKeyColumnWidth, footerKeyLabelGap, th, colourless)
				padded := headerPadRight(panel, lipgloss.Width(panel), themePanelFooterTestWidth, th, colourless)
				if got := themePanelFooterRow(e, themePanelFooterTestWidth, th, colourless); got != padded {
					t.Errorf("[%v col=%v] panel footer row for %q is not the shared builder's row\n got: %q\nwant: %q",
						themeLabel(th), colourless, e.Key, got, padded)
				}
			}
		}
	}
}

// TestKeyColumnRow_PadsOnlyWhenTheGlyphIsNarrowerThanTheColumn asserts the pad
// segment is emitted for a short glyph and OMITTED entirely once the glyph reaches
// the column width.
//
// The omission is load-bearing rather than cosmetic: a canvas style renders the
// empty string as a styled EMPTY RUN, not as nothing, so padding unconditionally
// would put a stray SGR pair on every full-width row — the panel's `esc close` row
// is one at its three-cell column.
func TestKeyColumnRow_PadsOnlyWhenTheGlyphIsNarrowerThanTheColumn(t *testing.T) {
	const columnWidth = 3
	th := testDarkTheme(t)
	keyStyle := headerStyle(th.AccentKey, th, false)
	labelStyle := headerStyle(th.TextMuted, th, false)
	canvas := headerCanvasBg(th, false)

	for _, tc := range []struct {
		name  string
		glyph string
		pad   string
	}{
		{"narrower than the column", "d", spaces(columnWidth - 1)},
		{"exactly the column width", "esc", ""},
		{"wider than the column", "^↑/↓", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			segments := []string{keyStyle.Render(tc.glyph)}
			if tc.pad != "" {
				segments = append(segments, canvas.Render(tc.pad))
			}
			segments = append(segments, canvas.Render(footerKeyLabelGap), labelStyle.Render("close"))
			want := lipgloss.JoinHorizontal(lipgloss.Top, segments...)

			got := keyColumnRow(tc.glyph, "close", keyStyle, labelStyle, columnWidth, footerKeyLabelGap, th, false)
			if got != want {
				t.Errorf("keyColumnRow(%q) = %q, want %q", tc.glyph, got, want)
			}
		})
	}
}
