package theme_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// nordSlug is the third built-in's slug — the stem of its committed filename,
// which §5.1 makes its identity.
const nordSlug = "nord"

// nordPath is the committed source of the Nord built-in.
var nordPath = filepath.Join(builtinsDir, nordSlug+".theme")

// wantNordTokens is §7.4's port table — the 19 values Nord ships — in §2.4
// order and in §4.3's canonical upper case.
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
// The whole 19-token slice is asserted in §2.4 order, so a value edited in the
// file, a key wired to the wrong role, or a token quietly dropped surfaces here
// rather than on a Nord terminal.
//
// border and text.faint deliberately carry the SAME hex. Nord's dark end holds
// only three values (nord1/2/3) for Portal's five dark-end roles, so nord3
// serves both — one value for two roles is legitimate, unlike two tokens that
// differ pointlessly, which the §2.2 border consolidation removed. Asserting
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

// TestNord_IsEnrolledInFloorChecks pins the Nord built-in into §13.5's
// auto-enumerated floor set.
//
// Enrolment is the whole shape of this port's verification: no test names Nord,
// no floor is relaxed for it, and every §13.5 leg is measured against Nord's own
// canvas #2E3440 by the same code that measures the other built-ins. That
// matters more here than for either Tokyo Night, because #2E3440 is a MID-dark
// rather than a near-black — the headroom is materially tighter and several legs
// clear by hundredths, so a palette that quietly stopped being measured would
// look fine and read badly.
//
// The set is derived from BuiltinSlugs, so this also pins that the file is
// embedded, enumerated and reserved with no Go edit (§7.1, §5.4).
func TestNord_IsEnrolledInFloorChecks(t *testing.T) {
	enrolled := slices.Sorted(maps.Keys(embeddedThemes(t)))

	if !slices.Contains(enrolled, nordSlug) {
		t.Errorf("the floor tests enrol %v, want %q among them", enrolled, nordSlug)
	}
}

// nordJudgement is one value the port did not lift straight from the palette,
// together with the figures its `#` comment must carry.
//
// The figures rather than the prose are what the guard asserts, so the record
// can be reworded but not hollowed out.
type nordJudgement struct {
	token, shipped string
	figures        []string
}

// nordCorrections is the port's two contrast corrections, as the record the
// shipped file must carry for each.
//
// A correction has a PUBLISHED SOURCE whose chroma it is supposed to preserve,
// so each row names the source hex, what that source measured, the chroma the
// shipped value retained of it, and the floor it was corrected for. The chroma
// figure is the load-bearing one: it is what makes the value checkable against
// §7.4's derivation rule if it is ever re-derived, and it is the quantity that
// diagnosed the first, rejected red.
var nordCorrections = []nordJudgement{
	{"state.destructive", "#DD8188", []string{"#BF616A", "3.05", "94%", "4.50", "Oklab"}},
	{"state.positive", "#A7C492", []string{"#A3BE8C", "4.23", "0.018", "100.8%", "4.50", "Oklab"}},
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
var nordInventions = []nordJudgement{
	{"text.muted", "#939EB2", []string{"nord3", "interpolated", "4.62"}},
	{"text.subtle", "#73819B", []string{"nord3", "interpolated", "3.18"}},
	{"bg.attention", "#3D4046", []string{"nord13", "8%", "1.20", "#54524F", "visual gate"}},
}

// nordPortNotes is the two findings §7.4 calls worth carrying forward — port
// choices that look like oversights and are not.
//
// text.on-attention is COOLER than Portal's other built-ins warm their on-band
// text to be, because Nord's Snow Storm is entirely cool and has no warm light;
// and nord3 serves BOTH border and text.faint, because the palette's dark end
// holds three values for five of Portal's dark-end roles. Written down, each is
// a decision; unwritten, the first reads as a missed warm tint and the second as
// a copy-paste.
var nordPortNotes = []nordJudgement{
	{"text.on-attention", "#ECEFF4", []string{"nord6", "cool", "9.02"}},
	{"border", "#4C566A", []string{"nord3", "text.faint"}},
}

// nordJudgementGroups is the three kinds of note the file carries, each with the
// marker its members must name themselves by.
//
// The port notes carry NO marker, which is the distinction the guard rests on:
// a marker is a claim that a value is not Nord's own, and text.on-attention and
// border ARE Nord's own (nord6 and nord3) — what they record is why the palette
// was read the way it was, not a value Portal invented or moved.
var nordJudgementGroups = []struct {
	kind    string
	marker  string
	members []nordJudgement
}{
	{"correction", correctionMarker, nordCorrections},
	{"invention", inventionMarker, nordInventions},
	{"port note", "", nordPortNotes},
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
// typo six months later. §4.2 admits no trailing comments, so the home is a `#`
// block immediately above the value, which §12.1's byte-faithful export carries
// to every user who copies the file.
//
// The assertion is on the FIGURES rather than on the prose, so the record can be
// reworded but not hollowed out. The final loop is the half that keeps the
// 13 + 2 + 3 + 1 arithmetic honest: no OTHER value may claim a correction or an
// invention, so a straight palette lift cannot quietly acquire a justification
// it does not need, and text.on-selection's functional maximum cannot be
// mistaken for either.
func TestNordFile_CorrectionsAndInventionsCarryComments(t *testing.T) {
	text := readNord(t)

	judged := map[string]string{}
	for _, group := range nordJudgementGroups {
		for _, member := range group.members {
			if group.marker != "" {
				judged[member.token] = group.marker
			}

			t.Run(group.kind+"/"+member.token, func(t *testing.T) {
				if got := valueFor(t, text, member.token); got != member.shipped {
					t.Errorf("%s = %s, want the shipped %s", member.token, got, member.shipped)
				}

				block := commentBlockAbove(t, text, member.token)
				if block == "" {
					t.Fatalf("%s carries no # comment — its %s has nowhere else to live", member.token, group.kind)
				}
				if group.marker != "" && !strings.Contains(block, group.marker) {
					t.Errorf("%s's comment does not name itself a %s (want %q):\n%s", member.token, group.kind, group.marker, block)
				}
				for _, figure := range member.figures {
					if !strings.Contains(block, figure) {
						t.Errorf("%s's comment omits %q:\n%s", member.token, figure, block)
					}
				}
			})
		}
	}

	for _, token := range theme.TokenNames() {
		block := commentBlockAbove(t, text, token)
		for _, marker := range []string{correctionMarker, inventionMarker} {
			if strings.Contains(block, marker) && judged[token] != marker {
				t.Errorf("%s claims %q, but the port is 13 values taken directly, %d corrections and %d inventions",
					token, marker, len(nordCorrections), len(nordInventions))
			}
		}
	}
}

// TestNordFile_HeaderAttributesThePalette pins the file's header.
//
// The flat format was chosen over JSON precisely so a ported palette could
// carry its attribution (§4.1), and §12.1 exports the file's bytes verbatim, so
// the header is what a user copying Nord actually receives. §4.1's own example
// models the two lines asserted here: the palette and its upstream link, and the
// one-line statement that two values are corrected for Portal's floors — which
// is the honest form of shipping adapted values under the palette's own name.
func TestNordFile_HeaderAttributesThePalette(t *testing.T) {
	header := leadingCommentBlock(t, readNord(t))

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

// readNord reads the committed Nord built-in's source.
//
// The comment guards read the FILE rather than Result.Source, for the reason the
// light built-in's do: the embedding is pinned to the committed bytes
// separately, and a guard that read through the embed could not tell a missing
// comment from a stale build.
func readNord(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(nordPath)
	if err != nil {
		t.Fatalf("read %s: %v", nordPath, err)
	}
	return string(data)
}
