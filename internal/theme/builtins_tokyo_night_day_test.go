package theme_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// tokyoNightDaySlug is the light built-in's slug — the stem of its committed
// filename, which the filename-is-identity rule makes its identity.
const tokyoNightDaySlug = "tokyo-night-day"

// wantTokyoNightDayTokens is the shipped Tokyo Night light table — the values MV carried as
// its Light variants — in canonical table order and in the hex-only value rule's canonical
// upper case.
//
// It is the deliberate second copy of the shipped file's values, matching the
// dark built-in's pin: these 19 hexes are what Portal looks like on a light
// terminal, so changing one is a change to the product and has to be made twice.
// `canvas` is written lower case in the file and appears here upper case, which
// is the canonicalisation the OSC 11 re-emission rule's background diffing and the exit-time
// restore rule's retained startup canvas hex both compare against.
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
// The whole 19-token slice is asserted in canonical table order, so a value edited in the
// file, a key wired to the wrong role, or a token quietly dropped surfaces here
// rather than on a light terminal. The case half matters for the same reason it
// does on the dark side: the file writes `canvas` lower case and the parser
// canonicalises to upper, which is what lets the OSC 11 re-emission rule and the exit-time
// restore rule compare hex strings at all.
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

// sevenCheckedValues is the erratum re-derivation check, as the record the shipped
// file must carry for each value it covers.
//
// Six are the erratum contrast corrections; the seventh, text.tertiary, is
// a darkening for the bg.selection pairing floor rather than an erratum, and
// carries the same chroma risk. Each row names the FOUR figures the erratum re-derivation
// requires — the original, the chroma the shipped value retained of it, the Oklab
// re-derivation, and ΔE(shipped, re-derivation) — plus the verdict that ΔE
// produced, because a passing check is a finding and has to be recorded as one.
//
// state.positive's original is #4C7A1F rather than the intermediate #456E1C: it
// was darkened twice, so measuring against the intermediate would understate the
// loss.
var sevenCheckedValues = []derivationRecord{
	{kind: "re-derivation", token: "text.tertiary", shipped: "#4C5478", figures: []string{"#515A80", "95.6%", "#4C557B", "0.0051", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "text.muted", shipped: "#586093", figures: []string{"#5A6296", "98.5%", "#596295", "0.0063", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "text.subtle", shipped: "#767DA2", figures: []string{"#7C84AA", "98.0%", "#7A7FA5", "0.0091", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "accent.key", shipped: "#2D5CCA", figures: []string{"#2E5FD0", "97.8%", "#2D5ECE", "0.0078", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "accent.mode", shipped: "#0D6C87", figures: []string{"#0E7490", "95.5%", "#036E8B", "0.0075", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "state.positive", shipped: "#3B5E18", figures: []string{"#4C7A1F", "81.2%", "#406000", "0.0162", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "state.destructive", shipped: "#BD2545", figures: []string{"#C32647", "97.5%", "#C12445", "0.0080", "under the 0.05 threshold"}},
}

// TestTokyoNightDayFile_SevenValuesCarryDerivationComments is the guard on
// the erratum re-derivation's durable record.
//
// The check's whole output — chroma loss against the original and ΔE against
// the Oklab re-derivation — has exactly one home: a `#` comment beside the value
// in this file. A commit message would be gone in a year and docs/theming.md
// documents roles rather than derivations, so if the comment is not here the
// figures do not exist anywhere. The file is exported byte-faithfully,
// which is what makes the comment travel with the value it describes.
//
// The assertion is on the FIGURES rather than on the prose, so the record can be
// reworded but not hollowed out: drop the re-derivation or the chroma
// percentage and the test names which one went.
func TestTokyoNightDayFile_SevenValuesCarryDerivationComments(t *testing.T) {
	assertDerivationRecords(t, readBuiltinFile(t, tokyoNightDaySlug), sevenCheckedValues, "")
}

// pinnedTints is the contrast gate's four eyeball-pinned light surface tints,
// each carrying as its required figure the dark anchor it was lifted from.
//
// The count of four is load-bearing: it is what decides which notes move
// into the theme file, and it is four rather than three because the border
// consolidation collapsed border.separator and border.footer into one token that
// keeps its pin.
var pinnedTints = []derivationRecord{
	{kind: "eyeball pin", marker: eyeballMarker, token: "bg.selection", shipped: "#D0C6F0", figures: []string{"#28243a"}},
	{kind: "eyeball pin", marker: eyeballMarker, token: "bg.attention", shipped: "#E8D6A8", figures: []string{"#241B10"}},
	{kind: "eyeball pin", marker: eyeballMarker, token: "bg.subtle", shipped: "#D2D4DE", figures: []string{"#26283A"}},
	{kind: "eyeball pin", marker: eyeballMarker, token: "border", shipped: "#C9CDDB", figures: []string{"#292E42"}},
}

// eyeballMarker is the phrase the four pins carry and nothing else does. It is
// the discriminator the "no other value" half of the pin rule is asserted on.
const eyeballMarker = "eyeball-confirmed"

// TestTokyoNightDayFile_PinnedTintsCarryDerivationComments is the guard on the
// one judgement in this file that is not numerically recoverable.
//
// A light tint on a light canvas is numeric-insufficient — the fill legs
// bound it from below but nothing decides between two values that both clear
// 1.10 — so each of the four was settled by human eye against #e1e2e7, from a
// dark anchor. That derivation is the reason the value is what it is, and the
// built-in-is-a-file rule makes the theme file its home now that MV's inline comments are
// going.
//
// The second half is the one that keeps the record honest: NO OTHER value
// carries the marker. An eyeball pin claimed for a value that was in fact
// derived numerically would make the four-token count — which decides how wide
// the light-only carve-out has to be — a guess.
func TestTokyoNightDayFile_PinnedTintsCarryDerivationComments(t *testing.T) {
	marked := fmt.Sprintf("the eyeball-pinned set is the four surface tints %v", recordTokens(pinnedTints))

	assertDerivationRecords(t, readBuiltinFile(t, tokyoNightDaySlug), pinnedTints, marked)
}

// TestTokyoNightDayFile_AccentPrimaryUnchangedAndMarkedOutOfScope pins the one
// light value the erratum re-derivation's check deliberately did not touch.
//
// accent.primary renders bars and glyphs rather than body text, so it carries
// the 3.00 large/UI floor and cleared it unremedied — it was never darkened, so
// there is no correction to measure chroma loss against. Recording that is not
// bookkeeping: the alternative is a value sitting silently among six corrected
// neighbours, indistinguishable from one whose record was forgotten.
func TestTokyoNightDayFile_AccentPrimaryUnchangedAndMarkedOutOfScope(t *testing.T) {
	text := readBuiltinFile(t, tokyoNightDaySlug)

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
// parser reads `#` at line start as a comment and never as the start of a value,
// which is what lets the format carry the record at all.
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

	text := readBuiltinFile(t, tokyoNightDaySlug)
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

// builtinPath is the path of the built-in's committed source, derived from its
// slug by the filename-is-identity rule.
func builtinPath(slug string) string {
	return filepath.Join(builtinsDir, slug+theme.FileExtension)
}

// readBuiltinFile reads the committed source of the built-in named slug.
//
// The comment guards read the FILE rather than Result.Source so they are
// assertions about what is committed: the embedding is pinned to the committed
// bytes separately, and a guard that read through the embed could not tell a
// missing comment from a stale build.
//
// It is keyed by slug rather than written once per palette, so a built-in added
// to builtins/ is readable here with no new helper.
func readBuiltinFile(t *testing.T, slug string) string {
	t.Helper()

	path := builtinPath(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// derivationRecord is one value whose shipped hex the file must justify: the
// figures its `#` comment carries, and — where the value is not the palette's
// own — the marker by which the comment names the kind of judgement made.
//
// kind labels the record in subtest names and failure messages. A record with
// no marker still requires its figures; what it does not do is grant its token
// the right to carry one, so a note about why a palette was read a certain way is
// not confused with a claim that Portal moved the value.
type derivationRecord struct {
	kind    string
	token   string
	shipped string
	figures []string
	marker  string
}

// assertDerivationRecords asserts each record's shipped value and the figures
// its comment block carries, then sweeps every token for a marker no
// record grants it.
//
// The sweep is what keeps a marked set honest: a value lifted straight from a
// palette cannot quietly acquire a justification it does not need, and the count
// of marked values stays a fact rather than a guess. markedSet states what the
// marked set is and is quoted on a false claim.
//
// The markers swept are derived from the records, so deleting every record
// carrying a marker retires that marker's sweep silently — a record slice
// carrying no markers sweeps nothing and never reads markedSet.
//
// The figures rather than the prose are asserted, so a record can be reworded
// but not hollowed out.
func assertDerivationRecords(t *testing.T, text string, records []derivationRecord, markedSet string) {
	t.Helper()

	claimed := map[string]string{}
	markers := []string{}
	for _, record := range records {
		if record.marker != "" {
			claimed[record.token] = record.marker
			if !slices.Contains(markers, record.marker) {
				markers = append(markers, record.marker)
			}
		}

		t.Run(record.kind+"/"+record.token, func(t *testing.T) {
			if got := valueFor(t, text, record.token); got != record.shipped {
				t.Errorf("%s = %s, want the shipped %s", record.token, got, record.shipped)
			}

			block := commentBlockAbove(t, text, record.token)
			if block == "" {
				t.Fatalf("%s carries no # comment — its %s has nowhere else to live", record.token, record.kind)
			}
			if record.marker != "" && !strings.Contains(block, record.marker) {
				t.Errorf("%s's comment omits the %s marker %q — the value's whole justification:\n%s",
					record.token, record.kind, record.marker, block)
			}
			for _, figure := range record.figures {
				if !strings.Contains(block, figure) {
					t.Errorf("%s's comment omits %q:\n%s", record.token, figure, block)
				}
			}
		})
	}

	for _, token := range theme.TokenNames() {
		block := commentBlockAbove(t, text, token)
		for _, marker := range markers {
			if strings.Contains(block, marker) && claimed[token] != marker {
				t.Errorf("%s claims %q, but %s", token, marker, markedSet)
			}
		}
	}
}

// recordTokens returns the token of each record, in record order.
func recordTokens(records []derivationRecord) []string {
	tokens := make([]string, 0, len(records))
	for _, record := range records {
		tokens = append(tokens, record.token)
	}
	return tokens
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
// key's line, joined — the "beside the value" the erratum re-derivation and the
// built-in-is-a-file rule both mean, given the lexical rules admit no trailing comments.
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
