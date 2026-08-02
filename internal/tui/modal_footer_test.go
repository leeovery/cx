package tui

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// The golden strings below were CAPTURED from the PRE-refactor render functions (the
// per-modal key-hint / footer-row clones and the edit/preview footer hints) before the
// shared renderKeyHint / renderBlueKeyHint / renderConfirmCancelFooter helpers existed.
// They pin the exact rendered bytes — SGR colour runs (accent.blue key, text.detail
// label), the owned canvas background on every cell, and the canvas-painted gaps — so
// the consolidation is proven byte-identical in both modes AND under the colourless
// carve-out. Do NOT regenerate these to match new behaviour: they ARE the regression
// contract.

// renderKeyHint(key=y, label=kill, AccentBlue): the canonical single footer hint.
const (
	goldenKeyHintYKillDark  = "\x1b[38;2;122;162;247;48;2;11;12;20my\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mkill\x1b[m"
	goldenKeyHintYKillLight = "\x1b[38;2;45;92;202;48;2;225;226;231my\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mkill\x1b[m"
	goldenKeyHintYKillNoCol = "y kill"
)

// renderKeyHint(key="", label="empty on save = delete"): the label-only fast path
// that editFooterGroup's "empty on save = delete" group collapses onto.
const (
	goldenKeyHintEmptyDark  = "\x1b[38;2;115;122;162;48;2;11;12;20mempty on save = delete\x1b[m"
	goldenKeyHintEmptyLight = "\x1b[38;2;88;96;147;48;2;225;226;231mempty on save = delete\x1b[m"
	goldenKeyHintEmptyNoCol = "empty on save = delete"
)

// renderConfirmCancelFooter golden rows for the three destructive/rename footer shapes:
// kill (y/kill, esc/cancel), delete (y/delete, esc/cancel) and rename (⏎/rename,
// esc/cancel) — the fixed-gap "   " spacer between the two hints.
const (
	goldenKillFooterDark  = "\x1b[38;2;122;162;247;48;2;11;12;20my\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mkill\x1b[m\x1b[48;2;11;12;20m   \x1b[m\x1b[38;2;122;162;247;48;2;11;12;20mesc\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mcancel\x1b[m"
	goldenKillFooterLight = "\x1b[38;2;45;92;202;48;2;225;226;231my\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mkill\x1b[m\x1b[48;2;225;226;231m   \x1b[m\x1b[38;2;45;92;202;48;2;225;226;231mesc\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mcancel\x1b[m"
	goldenKillFooterNoCol = "y kill   esc cancel"

	goldenDeleteFooterDark  = "\x1b[38;2;122;162;247;48;2;11;12;20my\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mdelete\x1b[m\x1b[48;2;11;12;20m   \x1b[m\x1b[38;2;122;162;247;48;2;11;12;20mesc\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mcancel\x1b[m"
	goldenDeleteFooterLight = "\x1b[38;2;45;92;202;48;2;225;226;231my\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mdelete\x1b[m\x1b[48;2;225;226;231m   \x1b[m\x1b[38;2;45;92;202;48;2;225;226;231mesc\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mcancel\x1b[m"
	goldenDeleteFooterNoCol = "y delete   esc cancel"

	goldenRenameFooterDark  = "\x1b[38;2;122;162;247;48;2;11;12;20m⏎\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mrename\x1b[m\x1b[48;2;11;12;20m   \x1b[m\x1b[38;2;122;162;247;48;2;11;12;20mesc\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mcancel\x1b[m"
	goldenRenameFooterLight = "\x1b[38;2;45;92;202;48;2;225;226;231m⏎\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mrename\x1b[m\x1b[48;2;225;226;231m   \x1b[m\x1b[38;2;45;92;202;48;2;225;226;231mesc\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mcancel\x1b[m"
	goldenRenameFooterNoCol = "⏎ rename   esc cancel"
)

// previewFooterHint golden (glyph=←→, label=window) — proves the preview hint render.
const (
	goldenPreviewHintDark  = "\x1b[38;2;122;162;247;48;2;11;12;20m←→\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20mwindow\x1b[m"
	goldenPreviewHintLight = "\x1b[38;2;45;92;202;48;2;225;226;231m←→\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231mwindow\x1b[m"
	goldenPreviewHintNoCol = "←→ window"
)

// editFooterGroup golden (key=⏎/e, label=edit) — the normal non-empty edit hint.
const (
	goldenEditEditDark  = "\x1b[38;2;122;162;247;48;2;11;12;20m⏎/e\x1b[m\x1b[48;2;11;12;20m \x1b[m\x1b[38;2;115;122;162;48;2;11;12;20medit\x1b[m"
	goldenEditEditLight = "\x1b[38;2;45;92;202;48;2;225;226;231m⏎/e\x1b[m\x1b[48;2;225;226;231m \x1b[m\x1b[38;2;88;96;147;48;2;225;226;231medit\x1b[m"
	goldenEditEditNoCol = "⏎/e edit"
)

// TestRenderKeyHint asserts the shared renderKeyHint helper renders the canonical
// `<key> <label>` footer-hint shape (key glyph in keyTok over the owned canvas, a
// single canvas-painted gap, label in text.detail) byte-identically across both
// modes and the colourless carve-out, AND that the empty-key path collapses to the
// label-only render.
func TestRenderKeyHint(t *testing.T) {
	cases := []struct {
		name       string
		key        string
		label      string
		keyTok     theme.Token
		th         theme.Theme
		colourless bool
		want       string
	}{
		{"normal/dark/colour", "y", "kill", testDarkTheme(t).AccentKey, testDarkTheme(t), false, goldenKeyHintYKillDark},
		{"normal/light/colour", "y", "kill", testLightTheme(t).AccentKey, testLightTheme(t), false, goldenKeyHintYKillLight},
		{"normal/dark/colourless", "y", "kill", testDarkTheme(t).AccentKey, testDarkTheme(t), true, goldenKeyHintYKillNoCol},
		{"normal/light/colourless", "y", "kill", testLightTheme(t).AccentKey, testLightTheme(t), true, goldenKeyHintYKillNoCol},

		{"empty/dark/colour", "", "empty on save = delete", testDarkTheme(t).AccentKey, testDarkTheme(t), false, goldenKeyHintEmptyDark},
		{"empty/light/colour", "", "empty on save = delete", testLightTheme(t).AccentKey, testLightTheme(t), false, goldenKeyHintEmptyLight},
		{"empty/dark/colourless", "", "empty on save = delete", testDarkTheme(t).AccentKey, testDarkTheme(t), true, goldenKeyHintEmptyNoCol},
		{"empty/light/colourless", "", "empty on save = delete", testLightTheme(t).AccentKey, testLightTheme(t), true, goldenKeyHintEmptyNoCol},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderKeyHint(tc.key, tc.label, tc.keyTok, tc.th, tc.colourless)
			if got != tc.want {
				t.Errorf("renderKeyHint(%q,%q) mismatch\n got: %q\nwant: %q", tc.key, tc.label, got, tc.want)
			}
		})
	}
}

// TestRenderBlueKeyHint asserts the shared key-hint seam pins the theme's
// accent.key token: it must render byte-identically to renderKeyHint with an
// explicit AccentKey keyTok AND match the captured golden bytes across both
// canvases and the colourless carve-out. This is the SINGLE canonical blue-key-hint path the edit and
// preview footers route through (replacing the five removed per-modal wrappers).
func TestRenderBlueKeyHint(t *testing.T) {
	cases := []struct {
		name       string
		key, label string
		th         theme.Theme
		colourless bool
		want       string
	}{
		{"yKill/dark/colour", "y", "kill", testDarkTheme(t), false, goldenKeyHintYKillDark},
		{"yKill/light/colour", "y", "kill", testLightTheme(t), false, goldenKeyHintYKillLight},
		{"yKill/dark/colourless", "y", "kill", testDarkTheme(t), true, goldenKeyHintYKillNoCol},
		{"yKill/light/colourless", "y", "kill", testLightTheme(t), true, goldenKeyHintYKillNoCol},

		{"empty/dark/colour", "", "empty on save = delete", testDarkTheme(t), false, goldenKeyHintEmptyDark},
		{"empty/light/colour", "", "empty on save = delete", testLightTheme(t), false, goldenKeyHintEmptyLight},
		{"empty/dark/colourless", "", "empty on save = delete", testDarkTheme(t), true, goldenKeyHintEmptyNoCol},
		{"empty/light/colourless", "", "empty on save = delete", testLightTheme(t), true, goldenKeyHintEmptyNoCol},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderBlueKeyHint(tc.key, tc.label, tc.th, tc.colourless)
			if got != tc.want {
				t.Errorf("renderBlueKeyHint(%q,%q) golden mismatch\n got: %q\nwant: %q", tc.key, tc.label, got, tc.want)
			}
			// Pins AccentBlue: identical to the explicit-token renderKeyHint path.
			if pinned := renderKeyHint(tc.key, tc.label, tc.th.AccentKey, tc.th, tc.colourless); got != pinned {
				t.Errorf("renderBlueKeyHint(%q,%q) does not pin AccentBlue\n got: %q\nwant: %q", tc.key, tc.label, got, pinned)
			}
		})
	}
}

// TestRenderConfirmCancelFooter asserts the shared confirm/cancel row helper matches
// the prior hand-assembled output for the kill (y/esc), delete (y/esc) and rename
// (⏎/esc) constant sets — confirm hint + the fixed "   " canvas gap + cancel hint.
func TestRenderConfirmCancelFooter(t *testing.T) {
	cases := []struct {
		name                                             string
		confirmKey, confirmLabel, cancelKey, cancelLabel string
		th                                               theme.Theme
		colourless                                       bool
		want                                             string
	}{
		{"kill/dark/colour", "y", "kill", "esc", "cancel", testDarkTheme(t), false, goldenKillFooterDark},
		{"kill/light/colour", "y", "kill", "esc", "cancel", testLightTheme(t), false, goldenKillFooterLight},
		{"kill/dark/colourless", "y", "kill", "esc", "cancel", testDarkTheme(t), true, goldenKillFooterNoCol},
		{"kill/light/colourless", "y", "kill", "esc", "cancel", testLightTheme(t), true, goldenKillFooterNoCol},

		{"delete/dark/colour", "y", "delete", "esc", "cancel", testDarkTheme(t), false, goldenDeleteFooterDark},
		{"delete/light/colour", "y", "delete", "esc", "cancel", testLightTheme(t), false, goldenDeleteFooterLight},
		{"delete/dark/colourless", "y", "delete", "esc", "cancel", testDarkTheme(t), true, goldenDeleteFooterNoCol},
		{"delete/light/colourless", "y", "delete", "esc", "cancel", testLightTheme(t), true, goldenDeleteFooterNoCol},

		{"rename/dark/colour", "⏎", "rename", "esc", "cancel", testDarkTheme(t), false, goldenRenameFooterDark},
		{"rename/light/colour", "⏎", "rename", "esc", "cancel", testLightTheme(t), false, goldenRenameFooterLight},
		{"rename/dark/colourless", "⏎", "rename", "esc", "cancel", testDarkTheme(t), true, goldenRenameFooterNoCol},
		{"rename/light/colourless", "⏎", "rename", "esc", "cancel", testLightTheme(t), true, goldenRenameFooterNoCol},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderConfirmCancelFooter(tc.confirmKey, tc.confirmLabel, tc.cancelKey, tc.cancelLabel, tc.th, tc.colourless)
			if got != tc.want {
				t.Errorf("renderConfirmCancelFooter mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// TestFooterHintCallSitesByteIdentical pins each refactored call-site render function
// against the captured PRE-refactor golden, proving the consolidation produced zero
// output drift for every footer/modal-footer in both modes and the colourless carve-out.
func TestFooterHintCallSitesByteIdentical(t *testing.T) {
	modes := []struct {
		name string
		m    theme.Theme
	}{{"dark", testDarkTheme(t)}, {"light", testLightTheme(t)}}

	// previewFooterHint and editFooterGroup were deleted in favour of routing the
	// Preview nav footer and the contextual edit footer through the shared
	// renderBlueKeyHint seam. These two sub-tests pin those LIVE shapes (the ←→/window
	// preview glyph and the ⏎/e edit key, plus the empty-key consequence note) against
	// the same PRE-refactor goldens, asserted through renderBlueKeyHint — proving the
	// reroute is byte-identical.
	t.Run("previewFooterHint", func(t *testing.T) {
		for _, md := range modes {
			for _, cl := range []bool{false, true} {
				got := renderBlueKeyHint("←→", "window", md.m, cl)
				exp := goldenPreviewHintDark
				if md.name == "light" {
					exp = goldenPreviewHintLight
				}
				if cl {
					exp = goldenPreviewHintNoCol
				}
				if got != exp {
					t.Errorf("preview footer hint %s colourless=%v drift\n got: %q\nwant: %q", md.name, cl, got, exp)
				}
			}
		}
	})

	t.Run("editFooterGroup/normal", func(t *testing.T) {
		for _, md := range modes {
			for _, cl := range []bool{false, true} {
				got := renderBlueKeyHint("⏎/e", "edit", md.m, cl)
				exp := goldenEditEditDark
				if md.name == "light" {
					exp = goldenEditEditLight
				}
				if cl {
					exp = goldenEditEditNoCol
				}
				if got != exp {
					t.Errorf("edit footer group(normal) %s colourless=%v drift\n got: %q\nwant: %q", md.name, cl, got, exp)
				}
			}
		}
	})

	t.Run("editFooterGroup/empty", func(t *testing.T) {
		for _, md := range modes {
			for _, cl := range []bool{false, true} {
				got := renderBlueKeyHint("", "empty on save = delete", md.m, cl)
				exp := goldenKeyHintEmptyDark
				if md.name == "light" {
					exp = goldenKeyHintEmptyLight
				}
				if cl {
					exp = goldenKeyHintEmptyNoCol
				}
				if got != exp {
					t.Errorf("edit footer group(empty) %s colourless=%v drift\n got: %q\nwant: %q", md.name, cl, got, exp)
				}
			}
		}
	})

	t.Run("renameModalFooterRow", func(t *testing.T) {
		for _, md := range modes {
			for _, cl := range []bool{false, true} {
				got := renameModalFooterRow(md.m, cl)
				exp := goldenRenameFooterDark
				if md.name == "light" {
					exp = goldenRenameFooterLight
				}
				if cl {
					exp = goldenRenameFooterNoCol
				}
				if got != exp {
					t.Errorf("renameModalFooterRow %s colourless=%v drift\n got: %q\nwant: %q", md.name, cl, got, exp)
				}
			}
		}
	})
}
