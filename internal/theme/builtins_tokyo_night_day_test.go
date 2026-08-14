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

const tokyoNightDaySlug = "tokyo-night-day"

var tokyoNightDayPath = builtinPath(tokyoNightDaySlug)

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

var sevenCheckedValues = []derivationRecord{
	{kind: "re-derivation", token: "text.tertiary", shipped: "#4C5478", figures: []string{"#515A80", "95.6%", "#4C557B", "0.0051", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "text.muted", shipped: "#586093", figures: []string{"#5A6296", "98.5%", "#596295", "0.0063", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "text.subtle", shipped: "#767DA2", figures: []string{"#7C84AA", "98.0%", "#7A7FA5", "0.0091", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "accent.key", shipped: "#2D5CCA", figures: []string{"#2E5FD0", "97.8%", "#2D5ECE", "0.0078", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "accent.mode", shipped: "#0D6C87", figures: []string{"#0E7490", "95.5%", "#036E8B", "0.0075", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "state.positive", shipped: "#3B5E18", figures: []string{"#4C7A1F", "81.2%", "#406000", "0.0162", "under the 0.05 threshold"}},
	{kind: "re-derivation", token: "state.destructive", shipped: "#BD2545", figures: []string{"#C32647", "97.5%", "#C12445", "0.0080", "under the 0.05 threshold"}},
}

func TestTokyoNightDayFile_SevenValuesCarryDerivationComments(t *testing.T) {
	assertDerivationRecords(t, readBuiltinFile(t, tokyoNightDaySlug), sevenCheckedValues, "")
}

var pinnedTints = []derivationRecord{
	{kind: "eyeball pin", marker: eyeballMarker, token: "bg.selection", shipped: "#D0C6F0", figures: []string{"#28243a"}},
	{kind: "eyeball pin", marker: eyeballMarker, token: "bg.attention", shipped: "#E8D6A8", figures: []string{"#241B10"}},
	{kind: "eyeball pin", marker: eyeballMarker, token: "bg.subtle", shipped: "#D2D4DE", figures: []string{"#26283A"}},
	{kind: "eyeball pin", marker: eyeballMarker, token: "border", shipped: "#C9CDDB", figures: []string{"#292E42"}},
}

const eyeballMarker = "eyeball-confirmed"

func TestTokyoNightDayFile_PinnedTintsCarryDerivationComments(t *testing.T) {
	marked := fmt.Sprintf("the eyeball-pinned set is the four surface tints %v", recordTokens(pinnedTints))

	assertDerivationRecords(t, readBuiltinFile(t, tokyoNightDaySlug), pinnedTints, marked)
}

// The header is where the re-derivation metric, the surfaces the ratios are
// measured against and the count of note kinds are stated once. Nothing else
// reads it, so without this the whole header could be deleted with the suite
// green — and a later re-derivation would have no comparable scale.
func TestTokyoNightDayFile_HeaderStatesTheMetric(t *testing.T) {
	header := leadingCommentBlock(t, tokyoNightDayPath, readBuiltinFile(t, tokyoNightDaySlug))

	if first := firstNonBlankLine(header); !strings.HasPrefix(first, "#") {
		t.Fatalf("%s opens with %q, want a # header comment naming the palette and its source", tokyoNightDayPath, first)
	}

	want := []string{
		"Tokyo Night Day",
		"https://github.com/tokyo-night/tokyo-night-vscode-theme",
		// The canvas every other ratio is measured against, and the two
		// text-carrying bands the header names as the exceptions.
		"#e1e2e7", "#D0C6F0", "#E8D6A8",
		// The metric itself, and the threshold the seven came in under.
		"Oklab", "OkLch", "0.05",
		// The two corrections bound by the selected-row leg rather than the canvas.
		"text.tertiary", "state.positive",
		// The three kinds of note, including the out-of-scope one.
		"Three kinds of note", "out of scope",
	}

	for _, phrase := range want {
		if !strings.Contains(header, phrase) {
			t.Errorf("the header omits %q:\n%s", phrase, header)
		}
	}
}

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

func builtinPath(slug string) string {
	return filepath.Join(builtinsDir, slug+theme.FileExtension)
}

func readBuiltinFile(t *testing.T, slug string) string {
	t.Helper()

	path := builtinPath(slug)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

type derivationRecord struct {
	kind    string
	token   string
	shipped string
	figures []string
	marker  string
}

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

func recordTokens(records []derivationRecord) []string {
	tokens := make([]string, 0, len(records))
	for _, record := range records {
		tokens = append(tokens, record.token)
	}
	return tokens
}

func declaresKey(line, key string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return false
	}
	name, _, separated := strings.Cut(trimmed, "=")
	return separated && strings.TrimSpace(name) == key
}

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
