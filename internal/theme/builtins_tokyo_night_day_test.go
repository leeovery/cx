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

// tokyoNightDaySlug is the light built-in's slug — the stem of its committed
// filename, which §5.1 makes its identity.
const tokyoNightDaySlug = "tokyo-night-day"

// tokyoNightDayPath is the committed source of the light built-in.
var tokyoNightDayPath = filepath.Join(builtinsDir, tokyoNightDaySlug+".theme")

// wantTokyoNightDayTokens is §7.3's light table — the values MV carried as its
// Light variants — in §2.4 order and in §4.3's canonical upper case.
//
// It is the deliberate second copy of the shipped file's values, matching the
// dark built-in's pin: these 19 hexes are what Portal looks like on a light
// terminal, so changing one is a change to the product and has to be made twice.
// `canvas` is written lower case in the file and appears here upper case, which
// is the canonicalisation §11.3's background diffing and §11.4's retained
// startup canvas hex both compare against.
var wantTokyoNightDayTokens = []theme.Token{
	{Name: "text.primary", Value: "#2E3C64"},
	{Name: "text.secondary", Value: "#3F4760"},
	{Name: "text.tertiary", Value: "#4C5478"},
	{Name: "text.muted", Value: "#586093"},
	{Name: "text.subtle", Value: "#767DA2"},
	{Name: "text.faint", Value: "#AEB2C6"},
	{Name: "text.on-selection", Value: "#1A1B2E"},
	{Name: "accent.primary", Value: "#8A3FD1"},
	{Name: "accent.key", Value: "#2D5CCA"},
	{Name: "accent.mode", Value: "#0D6C87"},
	{Name: "accent.attention", Value: "#9A5200"},
	{Name: "state.positive", Value: "#3B5E18"},
	{Name: "state.destructive", Value: "#BD2545"},
	{Name: "canvas", Value: "#E1E2E7"},
	{Name: "bg.selection", Value: "#D0C6F0"},
	{Name: "bg.attention", Value: "#E8D6A8"},
	{Name: "bg.subtle", Value: "#D2D4DE"},
	{Name: "border", Value: "#C9CDDB"},
	{Name: "text.on-attention", Value: "#7A4B12"},
}

// TestLoadBuiltin_TokyoNightDayIsValid pins the light built-in as a theme the
// shared loader accepts, and pins the palette it accepts.
//
// The whole 19-token slice is asserted in §2.4 order, so a value edited in the
// file, a key wired to the wrong role, or a token quietly dropped surfaces here
// rather than on a light terminal. The case half matters for the same reason it
// does on the dark side: the file writes `canvas` lower case and the parser
// canonicalises to upper (§4.3), which is what lets §11.3 and §11.4 compare hex
// strings at all.
func TestLoadBuiltin_TokyoNightDayIsValid(t *testing.T) {
	got, rejection, found := theme.Loader{}.LoadBuiltin(tokyoNightDaySlug)

	if rejection != nil {
		t.Fatalf("LoadBuiltin(%q) rejected the embedded file: %v", tokyoNightDaySlug, rejection)
	}
	if !found {
		t.Fatalf("LoadBuiltin(%q) reported not found, want the embedded built-in", tokyoNightDaySlug)
	}
	if got.Slug != tokyoNightDaySlug {
		t.Errorf("slug = %q, want %q", got.Slug, tokyoNightDaySlug)
	}
	if tokens := got.Theme.All(); !slices.Equal(tokens, wantTokyoNightDayTokens) {
		t.Errorf("theme = %+v, want %+v", tokens, wantTokyoNightDayTokens)
	}
	if want := "#E1E2E7"; got.Theme.Canvas.Value != want {
		t.Errorf("Canvas.Value = %q, want the upper-case canonical %q", got.Theme.Canvas.Value, want)
	}
}

// TestTokyoNightDay_IsEnrolledInFloorChecks pins the light built-in into
// §13.5's auto-enumerated floor set.
//
// Enrolment is what makes every floor in contrast_test.go a statement about
// THIS palette rather than about the dark one alone — and it is the property
// that has to hold for a light theme in particular, since its floors are
// measured against its own near-white canvas rather than a dark reference. The
// enumeration is derived, so this cannot fail without the file having gone
// missing from the embedded set; it is asserted because a light built-in that
// silently stopped being measured is exactly the failure §6.4's bundled tier
// exists to prevent.
func TestTokyoNightDay_IsEnrolledInFloorChecks(t *testing.T) {
	enrolled := slices.Sorted(maps.Keys(embeddedThemes(t)))

	if !slices.Contains(enrolled, tokyoNightDaySlug) {
		t.Errorf("the floor tests enrol %v, want %q among them", enrolled, tokyoNightDaySlug)
	}
}

// sevenCheckedValues is §7.7's re-derivation check, as the record the shipped
// file must carry for each value it covers.
//
// Six are the §2.9-erratum contrast corrections; the seventh, text.tertiary, is
// a darkening for the bg.selection pairing floor rather than an erratum, and
// carries the same chroma risk. Each row names the FOUR figures §7.7 requires —
// the original, the chroma the shipped value retained of it, the Oklab
// re-derivation, and ΔE(shipped, re-derivation) — plus the verdict that ΔE
// produced, because a passing check is a finding and has to be recorded as one.
//
// state.positive's original is #4C7A1F rather than the intermediate #456E1C: it
// was darkened twice, so measuring against the intermediate would understate the
// loss.
var sevenCheckedValues = []struct {
	token, shipped string
	figures        []string
}{
	{"text.tertiary", "#4C5478", []string{"#515A80", "95.6%", "#4C557B", "0.0051", "under the 0.05 threshold"}},
	{"text.muted", "#586093", []string{"#5A6296", "98.5%", "#596295", "0.0063", "under the 0.05 threshold"}},
	{"text.subtle", "#767DA2", []string{"#7C84AA", "98.0%", "#7A7FA5", "0.0091", "under the 0.05 threshold"}},
	{"accent.key", "#2D5CCA", []string{"#2E5FD0", "97.8%", "#2D5ECE", "0.0078", "under the 0.05 threshold"}},
	{"accent.mode", "#0D6C87", []string{"#0E7490", "95.5%", "#036E8B", "0.0075", "under the 0.05 threshold"}},
	{"state.positive", "#3B5E18", []string{"#4C7A1F", "81.2%", "#406000", "0.0162", "under the 0.05 threshold"}},
	{"state.destructive", "#BD2545", []string{"#C32647", "97.5%", "#C12445", "0.0080", "under the 0.05 threshold"}},
}

// TestTokyoNightDayFile_SevenValuesCarryDerivationComments is the guard on
// §7.7's durable record.
//
// The check's whole output — chroma loss against the original and ΔE against
// the Oklab re-derivation — has exactly one home: a `#` comment beside the value
// in this file. A commit message would be gone in a year and docs/theming.md
// documents roles rather than derivations, so if the comment is not here the
// figures do not exist anywhere. The file is exported byte-faithfully (§12.1),
// which is what makes the comment travel with the value it describes.
//
// The assertion is on the FIGURES rather than on the prose, so the record can be
// reworded but not hollowed out: drop the re-derivation or the chroma
// percentage and the test names which one went.
func TestTokyoNightDayFile_SevenValuesCarryDerivationComments(t *testing.T) {
	text := readTokyoNightDay(t)

	for _, checked := range sevenCheckedValues {
		t.Run(checked.token, func(t *testing.T) {
			if got := valueFor(t, text, checked.token); got != checked.shipped {
				t.Errorf("%s = %s, want the shipped %s", checked.token, got, checked.shipped)
			}

			block := commentBlockAbove(t, text, checked.token)
			if block == "" {
				t.Fatalf("%s carries no # comment — §7.7's figures have nowhere else to live", checked.token)
			}
			for _, figure := range checked.figures {
				if !strings.Contains(block, figure) {
					t.Errorf("%s's comment omits %q:\n%s", checked.token, figure, block)
				}
			}
		})
	}
}

// pinnedTints is §13.5's four eyeball-pinned light surface tints, with the dark
// anchor each was lifted from.
//
// The count of four is load-bearing (§13.5): it is what decides which notes move
// into the theme file, and it is four rather than three because the §2.2 border
// consolidation collapsed border.separator and border.footer into one token that
// keeps its pin.
var pinnedTints = []struct {
	token, value, anchor string
}{
	{"bg.selection", "#D0C6F0", "#28243a"},
	{"bg.attention", "#E8D6A8", "#241B10"},
	{"bg.subtle", "#D2D4DE", "#26283A"},
	{"border", "#C9CDDB", "#292E42"},
}

// eyeballMarker is the phrase the four pins carry and nothing else does. It is
// the discriminator the "no other value" half of the pin rule is asserted on.
const eyeballMarker = "eyeball-confirmed"

// TestTokyoNightDayFile_PinnedTintsCarryDerivationComments is the guard on the
// one judgement in this file that is not numerically recoverable.
//
// A light tint on a light canvas is numeric-insufficient (§13.5) — the fill legs
// bound it from below but nothing decides between two values that both clear
// 1.10 — so each of the four was settled by human eye against #e1e2e7, from a
// dark anchor. That derivation is the reason the value is what it is, and §7.1
// makes the theme file its home now that MV's inline comments are going.
//
// The second half is the one that keeps the record honest: NO OTHER value
// carries the marker. An eyeball pin claimed for a value that was in fact
// derived numerically would make the four-token count — which decides how wide
// the light-only carve-out has to be — a guess.
func TestTokyoNightDayFile_PinnedTintsCarryDerivationComments(t *testing.T) {
	text := readTokyoNightDay(t)

	pinned := map[string]struct{}{}
	for _, pin := range pinnedTints {
		pinned[pin.token] = struct{}{}

		t.Run(pin.token, func(t *testing.T) {
			if got := valueFor(t, text, pin.token); got != pin.value {
				t.Errorf("%s = %s, want the pinned %s", pin.token, got, pin.value)
			}

			block := commentBlockAbove(t, text, pin.token)
			if !strings.Contains(block, pin.anchor) {
				t.Errorf("%s's comment omits its dark anchor %s:\n%s", pin.token, pin.anchor, block)
			}
			if !strings.Contains(block, eyeballMarker) {
				t.Errorf("%s's comment omits %q — the pin's whole justification:\n%s", pin.token, eyeballMarker, block)
			}
		})
	}

	for _, token := range theme.TokenNames() {
		if _, isPin := pinned[token]; isPin {
			continue
		}
		if strings.Contains(commentBlockAbove(t, text, token), eyeballMarker) {
			t.Errorf("%s claims %q, but the eyeball-pinned set is the four surface tints %v", token, eyeballMarker, pinned)
		}
	}
}

// TestTokyoNightDayFile_AccentPrimaryUnchangedAndMarkedOutOfScope pins the one
// light value §7.7's check deliberately did not touch.
//
// accent.primary renders bars and glyphs rather than body text, so it carries
// the 3.00 large/UI floor and cleared it unremedied — it was never darkened, so
// there is no correction to measure chroma loss against. Recording that is not
// bookkeeping: the alternative is a value sitting silently among six corrected
// neighbours, indistinguishable from one whose record was forgotten.
func TestTokyoNightDayFile_AccentPrimaryUnchangedAndMarkedOutOfScope(t *testing.T) {
	text := readTokyoNightDay(t)

	if got, want := valueFor(t, text, "accent.primary"), "#8A3FD1"; got != want {
		t.Errorf("accent.primary = %s, want the unchanged %s", got, want)
	}

	block := commentBlockAbove(t, text, "accent.primary")
	if !strings.Contains(block, "out of scope") {
		t.Errorf("accent.primary's comment does not mark it out of scope:\n%s", block)
	}
	if !strings.Contains(block, "never darkened") {
		t.Errorf("accent.primary's comment omits why it is out of scope — it was never darkened:\n%s", block)
	}
}

// TestLoadBuiltin_CommentsDoNotAffectParse proves the derivation record is inert.
//
// This file carries more comment than value, and every one of those lines is a
// judgement that cannot be reconstructed — so the cost of keeping them has to be
// exactly zero. Stripping every comment must yield the identical palette: the
// parser reads `#` at line start as a comment and never as the start of a value
// (§4.2), which is what lets the format carry the record at all.
//
// The stripped copy is loaded through LoadFile rather than compared textually,
// so the assertion is about the PALETTE the loader builds rather than about the
// bytes — a comment that somehow reached a value would show up as a rejection or
// a changed token, which is the failure worth catching.
func TestLoadBuiltin_CommentsDoNotAffectParse(t *testing.T) {
	withComments, rejection, found := theme.Loader{}.LoadBuiltin(tokyoNightDaySlug)
	if rejection != nil || !found {
		t.Fatalf("LoadBuiltin(%q) = (rejection %v, found %t), want the embedded built-in", tokyoNightDaySlug, rejection, found)
	}

	text := readTokyoNightDay(t)
	kept := []string{}
	for line := range strings.SplitSeq(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) != len(theme.TokenNames()) {
		t.Fatalf("stripping comments left %d lines, want the %d token pairs", len(kept), len(theme.TokenNames()))
	}

	strippedPath := filepath.Join(t.TempDir(), "stripped.theme")
	if err := os.WriteFile(strippedPath, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", strippedPath, err)
	}

	stripped, rejection := theme.Loader{}.LoadFile(strippedPath)
	if rejection != nil {
		t.Fatalf("LoadFile(%q) rejected the comment-stripped copy: %v", strippedPath, rejection)
	}
	if !slices.Equal(stripped.Theme.All(), withComments.Theme.All()) {
		t.Errorf("the comment-stripped copy parsed %+v, want the shipped %+v", stripped.Theme.All(), withComments.Theme.All())
	}
}

// readTokyoNightDay reads the committed light built-in's source.
//
// The comment guards read the FILE rather than Result.Source so they are
// assertions about what is committed: the embedding is pinned to the committed
// bytes separately, and a guard that read through the embed could not tell a
// missing comment from a stale build.
func readTokyoNightDay(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(tokyoNightDayPath)
	if err != nil {
		t.Fatalf("read %s: %v", tokyoNightDayPath, err)
	}
	return string(data)
}

// declaresKey reports whether one line of the file is the pair declaring key.
//
// Comment lines are excluded before the split, which matters in a file this
// dense in notes: a comment is free prose and may well contain an `=`, and one
// mentioning a token name would otherwise be read as that token's declaration.
func declaresKey(line, key string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return false
	}
	name, _, separated := strings.Cut(trimmed, "=")
	return separated && strings.TrimSpace(name) == key
}

// valueFor returns the value the file declares for key, failing if it declares
// none.
//
// It takes the file's TEXT rather than its path so every built-in's comment
// guard reads the same helper — the theme file format is one format, and a
// second copy of this walk per palette is how two files' guards drift apart.
func valueFor(t *testing.T, text, key string) string {
	t.Helper()

	for line := range strings.SplitSeq(text, "\n") {
		if declaresKey(line, key) {
			_, value, _ := strings.Cut(line, "=")
			return strings.TrimSpace(value)
		}
	}

	t.Fatalf("the theme file declares no %s", key)
	return ""
}

// commentBlockAbove returns the unbroken run of `#` comment lines directly above
// key's line, joined — the "beside the value" §7.7 and §7.1 both mean, given
// §4.2 admits no trailing comments.
//
// A blank line ends the block, so a note belonging to one value cannot be
// counted as another's: the header and each group's introduction are separated
// from the pairs by a blank line, exactly as the dark built-in writes them.
func commentBlockAbove(t *testing.T, text, key string) string {
	t.Helper()

	lines := strings.Split(text, "\n")
	index := slices.IndexFunc(lines, func(line string) bool { return declaresKey(line, key) })
	if index < 0 {
		t.Fatalf("the theme file declares no %s", key)
	}

	block := []string{}
	for above := index - 1; above >= 0; above-- {
		line := strings.TrimSpace(lines[above])
		if !strings.HasPrefix(line, "#") {
			break
		}
		block = append(block, line)
	}
	slices.Reverse(block)
	return strings.Join(block, "\n")
}
