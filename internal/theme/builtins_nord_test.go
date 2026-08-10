package theme_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// nordSlug is the third built-in's slug — the stem of its committed filename,
// which the filename-is-identity rule makes its identity.
const nordSlug = "nord"

// nordPath is the committed source of the Nord built-in.
var nordPath = builtinPath(nordSlug)

// wantNordTokens is the Nord port's port table — the 19 values Nord ships — in canonical table
// order and in the hex-only value rule's canonical upper case.
//
// It is the deliberate second copy of the shipped file's values, matching both
// Tokyo Night pins: these hexes are what Portal looks like under the first
// genuinely EXTERNAL palette, so changing one is a change to the product and
// has to be made twice.
//
// Five of them are not Nord's own, and the arithmetic is worth reading off the
// table: 13 values come straight from the 16-slot palette, two are contrast
// corrections, three are inventions filling roles no slot covers, and one —
// text.on-selection #FFFFFF — is a functional maximum rather than a palette
// claim. 13 + 2 + 3 + 1 = 19.
var wantNordTokens = []theme.Token{
	{Name: "text.primary", Value: "#ECEFF4"},
	{Name: "text.secondary", Value: "#E5E9F0"},
	{Name: "text.tertiary", Value: "#D8DEE9"},
	{Name: "text.muted", Value: "#939EB2"},
	{Name: "text.subtle", Value: "#73819B"},
	{Name: "text.faint", Value: "#4C566A"},
	{Name: "text.on-selection", Value: "#FFFFFF"},
	{Name: "accent.primary", Value: "#B48EAD"},
	{Name: "accent.key", Value: "#81A1C1"},
	{Name: "accent.mode", Value: "#88C0D0"},
	{Name: "accent.attention", Value: "#EBCB8B"},
	{Name: "state.positive", Value: "#A7C492"},
	{Name: "state.destructive", Value: "#DD8188"},
	{Name: "canvas", Value: "#2E3440"},
	{Name: "bg.selection", Value: "#434C5E"},
	{Name: "bg.attention", Value: "#3D4046"},
	{Name: "bg.subtle", Value: "#3B4252"},
	{Name: "border", Value: "#4C566A"},
	{Name: "text.on-attention", Value: "#ECEFF4"},
}

// TestLoadBuiltin_NordIsValid pins the third built-in as a theme the shared
// loader accepts, and pins the palette it accepts.
//
// The whole 19-token slice is asserted in canonical table order, so a value edited in the
// file, a key wired to the wrong role, or a token quietly dropped surfaces here
// rather than on a Nord terminal.
//
// border and text.faint deliberately carry the SAME hex. Nord's dark end holds
// only three values (nord1/2/3) for Portal's five dark-end roles, so nord3
// serves both — one value for two roles is legitimate, unlike two tokens that
// differ pointlessly, which the border consolidation removed. Asserting
// the pair as shipped is what stops a later reader "fixing" the repetition.
func TestLoadBuiltin_NordIsValid(t *testing.T) {
	got, rejection, found := theme.Loader{}.LoadBuiltin(nordSlug)

	if rejection != nil {
		t.Fatalf("LoadBuiltin(%q) rejected the embedded file: %v", nordSlug, rejection)
	}
	if !found {
		t.Fatalf("LoadBuiltin(%q) reported not found, want the embedded built-in", nordSlug)
	}
	if got.Slug != nordSlug {
		t.Errorf("slug = %q, want %q", got.Slug, nordSlug)
	}
	if tokens := got.Theme.All(); !slices.Equal(tokens, wantNordTokens) {
		t.Errorf("theme = %+v, want %+v", tokens, wantNordTokens)
	}
	if got.Theme.Border.Value != got.Theme.TextFaint.Value {
		t.Errorf("border = %q and text.faint = %q, want both to carry nord3 — the palette's dark end has one value for the two roles",
			got.Theme.Border.Value, got.Theme.TextFaint.Value)
	}
}

// nordCorrections is the port's two contrast corrections, as the record the
// shipped file must carry for each.
//
// A correction has a PUBLISHED SOURCE whose chroma it is supposed to preserve,
// so each row names the source hex, what that source measured, the chroma the
// shipped value retained of it, and the floor it was corrected for. The chroma
// figure is the load-bearing one: it is what makes the value checkable against
// the Nord port's derivation rule if it is ever re-derived, and it is the quantity that
// diagnosed the first, rejected red.
var nordCorrections = []derivationRecord{
	{
		kind: "correction", marker: correctionMarker,
		token: "state.destructive", shipped: "#DD8188",
		figures: []string{"#BF616A", "3.05", "94%", "4.50", "Oklab"},
	},
	{
		kind: "correction", marker: correctionMarker,
		token: "state.positive", shipped: "#A7C492",
		figures: []string{"#A3BE8C", "4.23", "0.018", "100.8%", "4.50", "Oklab"},
	},
}

// nordInventions is the port's three invented values, as the record the shipped
// file must carry for each.
//
// An invention has NO source to preserve, so its record is a derivation rather
// than a chroma figure: where it came from, and what settled it. Nord's greys
// are barrelled at the ends — three bright and three dark with nothing between —
// so the ramp's middle had to be interpolated, and a background warning tint is
// neither a neutral from that dark end nor a foreground accent.
//
// bg.attention additionally records the VISUAL GATE, which is the port's own
// precedent for how an invention is settled: its first arithmetic answer
// (#54524F, a 20% blend) was rejected on sight as far too heavy and as a warm
// grey outside Nord's cool family.
var nordInventions = []derivationRecord{
	{
		kind: "invention", marker: inventionMarker,
		token: "text.muted", shipped: "#939EB2",
		figures: []string{"nord3", "interpolated", "4.62"},
	},
	{
		kind: "invention", marker: inventionMarker,
		token: "text.subtle", shipped: "#73819B",
		figures: []string{"nord3", "interpolated", "3.18"},
	},
	{
		kind: "invention", marker: inventionMarker,
		token: "bg.attention", shipped: "#3D4046",
		figures: []string{"nord13", "8%", "1.20", "#54524F", "visual gate"},
	},
}

// nordPortNotes is the two findings the Nord port calls worth carrying forward — port
// choices that look like oversights and are not.
//
// text.on-attention is COOLER than Portal's other built-ins warm their on-band
// text to be, because Nord's Snow Storm is entirely cool and has no warm light;
// and nord3 serves BOTH border and text.faint, because the palette's dark end
// holds three values for five of Portal's dark-end roles. Written down, each is
// a decision; unwritten, the first reads as a missed warm tint and the second as
// a copy-paste.
//
// The port notes carry NO marker, which is the distinction the guard rests on:
// a marker is a claim that a value is not Nord's own, and text.on-attention and
// border ARE Nord's own (nord6 and nord3) — what they record is why the palette
// was read the way it was, not a value Portal invented or moved.
var nordPortNotes = []derivationRecord{
	{
		kind:  "port note",
		token: "text.on-attention", shipped: "#ECEFF4",
		figures: []string{"nord6", "cool", "9.02"},
	},
	{
		kind:  "port note",
		token: "border", shipped: "#4C566A",
		figures: []string{"nord3", "text.faint"},
	},
}

// correctionMarker and inventionMarker are the phrases the five judged values
// carry, and the discriminators the "no other value claims one" half of the
// guard is asserted on.
const (
	correctionMarker = "Correction"
	inventionMarker  = "Invention"
)

// TestNordFile_CorrectionsAndInventionsCarryComments is the guard on the five
// values in this palette that are not Nord's own.
//
// Thirteen of the 19 are lifted straight from the published palette and need no
// justification — the value is Nord's. The other five are judgements Portal
// made, and a judgement that is not written down is indistinguishable from a
// typo six months later. The lexical rules admit no trailing comments, so the home is a `#`
// block immediately above the value, which the export contract's byte-faithful export carries
// to every user who copies the file.
//
// The assertion is on the FIGURES rather than on the prose, so the record can be
// reworded but not hollowed out. The marker sweep is the half that keeps the
// 13 + 2 + 3 + 1 arithmetic honest: no OTHER value may claim a correction or an
// invention, so a straight palette lift cannot quietly acquire a justification
// it does not need, and text.on-selection's functional maximum cannot be
// mistaken for either.
func TestNordFile_CorrectionsAndInventionsCarryComments(t *testing.T) {
	marked := fmt.Sprintf("the port is 13 values taken directly, %d corrections and %d inventions",
		len(nordCorrections), len(nordInventions))

	assertDerivationRecords(t, readBuiltinFile(t, nordSlug),
		slices.Concat(nordCorrections, nordInventions, nordPortNotes), marked)
}

// TestNordFile_HeaderAttributesThePalette pins the file's header.
//
// The flat format was chosen over JSON precisely so a ported palette could
// carry its attribution, and the export contract exports the file's bytes verbatim, so
// the header is what a user copying Nord actually receives. The file format's own example
// models the two lines asserted here: the palette and its upstream link, and the
// one-line statement that two values are corrected for Portal's floors — which
// is the honest form of shipping adapted values under the palette's own name.
func TestNordFile_HeaderAttributesThePalette(t *testing.T) {
	header := leadingCommentBlock(t, readBuiltinFile(t, nordSlug))

	if first := firstNonBlankLine(header); !strings.HasPrefix(first, "#") {
		t.Fatalf("%s opens with %q, want a # header comment naming the palette and its source", nordPath, first)
	}

	want := []string{"Nord", "https://www.nordtheme.com/", "corrected for Portal's contrast floors"}
	for _, corrected := range nordCorrections {
		want = append(want, corrected.token)
	}

	for _, phrase := range want {
		if !strings.Contains(header, phrase) {
			t.Errorf("the header omits %q:\n%s", phrase, header)
		}
	}
}

// leadingCommentBlock returns the file's opening run of `#` comment lines — the
// attribution header, as distinct from the per-value notes further down, which a
// blank line separates from it.
func leadingCommentBlock(t *testing.T, text string) string {
	t.Helper()

	block := []string{}
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			break
		}
		block = append(block, trimmed)
	}
	if len(block) == 0 {
		t.Fatalf("%s opens with no # header comment", nordPath)
	}
	return strings.Join(block, "\n")
}
