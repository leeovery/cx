package theme_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

const nordSlug = "nord"

var nordPath = builtinPath(nordSlug)

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

const (
	correctionMarker = "Correction"
	inventionMarker  = "Invention"
)

func TestNordFile_CorrectionsAndInventionsCarryComments(t *testing.T) {
	marked := fmt.Sprintf("the port is 13 values taken directly, %d corrections and %d inventions",
		len(nordCorrections), len(nordInventions))

	assertDerivationRecords(t, readBuiltinFile(t, nordSlug),
		slices.Concat(nordCorrections, nordInventions, nordPortNotes), marked)
}

func TestNordFile_HeaderAttributesThePalette(t *testing.T) {
	header := leadingCommentBlock(t, nordPath, readBuiltinFile(t, nordSlug))

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

func leadingCommentBlock(t *testing.T, path, text string) string {
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
		t.Fatalf("%s opens with no # header comment", path)
	}
	return strings.Join(block, "\n")
}
