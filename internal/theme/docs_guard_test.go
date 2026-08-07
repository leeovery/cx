package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// themingDocPath is the user-facing theme documentation, resolved relative to
// this package's directory — a test binary runs in internal/theme, two levels
// under the repo root.
var themingDocPath = filepath.Join("..", "..", "docs", "theming.md")

// tokenTableHeading is the first header cell of every table in the doc that
// declares token roles, and the scope of the row parse below.
//
// A backticked first cell is not a strong enough signal on its own: the doc
// documents other backticked things — filenames, environment variables, slugs —
// and a table about one of those must not be read as a token declaration.
const tokenTableHeading = "Token"

// exampleThemeHeading is the heading whose fenced block holds the doc's
// copy-pasteable theme.
const exampleThemeHeading = "Example theme"

// TestThemingDocGuard_MissingDocFails asserts an absent doc is an error the
// guard reports rather than an empty read it passes over.
//
// The live cases below turn that error into a t.Fatalf and never a skip: a
// skipping guard is indistinguishable from a passing one in the run output, so
// deleting the doc would silently retire the only check keeping the public
// token contract honest.
func TestThemingDocGuard_MissingDocFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "theming.md")

	doc, err := readThemingDoc(missing)
	if err == nil {
		t.Fatalf("readThemingDoc(%q) returned %d bytes and no error for a file that does not exist", missing, len(doc))
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("readThemingDoc(%q) error = %q, want it to name the path it could not read", missing, err)
	}
}

// TestThemingDocGuard_ZeroRowsFailsLoudly asserts a parse matching no rows is
// itself a failure, reported differently from a name mismatch.
//
// Every comparison this guard makes is against what the parse found, so a parse
// that finds nothing agrees with everything. The row count is therefore asserted
// rather than inferred, and the empty case carries its own message so a reader
// can tell "the doc lost a token" from "the guard stopped recognising the doc".
func TestThemingDocGuard_ZeroRowsFailsLoudly(t *testing.T) {
	doc := "# Themes\n\nProse with no role table in it at all.\n"

	problems := auditDocTokenTable([]byte(doc), TokenNames())

	if len(problems) != 1 {
		t.Fatalf("auditDocTokenTable() reported %d problems for a doc with no table, want exactly the vacuous-parse one: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "no token rows") {
		t.Errorf("problem = %q, want it to say the parse matched no token rows", problems[0])
	}
	for _, name := range TokenNames() {
		if strings.Contains(problems[0], name) {
			t.Fatalf("problem = %q names the token %q, want a message distinct from a name mismatch", problems[0], name)
		}
	}
}

// TestThemingDocTokenTableMatchesAllTokens is the live case: the doc's token
// table declares exactly the tokens the vocabulary holds.
//
// The doc is the source of truth for the public contract — a drop-in author
// writes these keys — so a token added, removed or renamed without the doc
// moving with it fails here.
func TestThemingDocTokenTableMatchesAllTokens(t *testing.T) {
	doc, err := readThemingDoc(themingDocPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	all := (Theme{}).All()
	want := make([]string, 0, len(all))
	for _, token := range all {
		want = append(want, token.Name)
	}

	for _, problem := range auditDocTokenTable(doc, want) {
		t.Errorf("%s: %s", themingDocPath, problem)
	}
}

// TestThemingDocGuard_TokenAbsentFromTableFails pins the first drift direction:
// a token the vocabulary holds and the doc never documents.
func TestThemingDocGuard_TokenAbsentFromTableFails(t *testing.T) {
	names := TokenNames()
	undocumented := names[len(names)-1]
	doc := docWithTokenTable(names[:len(names)-1])

	problems := auditDocTokenTable([]byte(doc), names)

	requireProblemNaming(t, problems, undocumented)
}

// TestThemingDocGuard_UnknownTableRowFails pins the other drift direction: a
// doc row naming a token that no longer exists.
func TestThemingDocGuard_UnknownTableRowFails(t *testing.T) {
	const retired = "border.footer"
	names := TokenNames()
	doc := docWithTokenTable(append(slices.Clone(names), retired))

	problems := auditDocTokenTable([]byte(doc), names)

	requireProblemNaming(t, problems, retired)
}

// TestThemingDocGuard_RowOrderIsNotAsserted asserts the guard compares a name
// set rather than a sequence.
//
// The order keys appear in carries no meaning and is not enforced anywhere, so
// a doc that reorders its rows — or regroups them — must stay green. The ramp's
// weight ordering is prose this guard deliberately does not check.
func TestThemingDocGuard_RowOrderIsNotAsserted(t *testing.T) {
	names := TokenNames()
	reversed := slices.Clone(names)
	slices.Reverse(reversed)

	problems := auditDocTokenTable([]byte(docWithTokenTable(reversed)), names)

	if len(problems) != 0 {
		t.Errorf("auditDocTokenTable() reported %v for a table carrying every token in reverse order, want no problems", problems)
	}
}

// TestThemingDocExampleThemeIsValid is the second live case: the doc's
// copy-pasteable theme parses as a complete palette.
//
// It goes through the same content ladder every theme file goes through, which
// is what stops the example being an unguarded copy of the vocabulary — a
// missing key or a mistyped hex fails here rather than in the terminal of the
// first person who pastes it.
func TestThemingDocExampleThemeIsValid(t *testing.T) {
	doc, err := readThemingDoc(themingDocPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	built, problems := auditDocExampleTheme(doc)
	if len(problems) > 0 {
		for _, problem := range problems {
			t.Errorf("%s: %s", themingDocPath, problem)
		}
		return
	}

	for _, token := range built.All() {
		if token.Value == "" {
			t.Errorf("%s: the example theme parsed but left %q empty", themingDocPath, token.Name)
		}
	}
}

// TestThemingDocGuard_ExampleMissingTokenFails asserts the example is judged by
// the validity rule rather than by looking like a theme: all 19 keys present,
// every value a well-formed hex.
func TestThemingDocGuard_ExampleMissingTokenFails(t *testing.T) {
	names := TokenNames()
	dropped := names[0]
	doc := docWithExampleTheme(exampleThemeText(names[1:]))

	_, problems := auditDocExampleTheme([]byte(doc))

	requireProblemNaming(t, problems, dropped)
}

// readThemingDoc reads the theme documentation, framing a failed read as the
// guard's own error so an absent doc reads as "the guard could not find what it
// checks" rather than as a bare OS message.
func readThemingDoc(path string) ([]byte, error) {
	doc, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the theme documentation at %s: %w", path, err)
	}
	return doc, nil
}

// auditDocTokenTable compares the token names the doc's role tables declare
// against want, returning one problem per drift and nothing when the two agree.
//
// A parse matching nothing short-circuits: with no rows every comparison below
// holds vacuously, so an unrecognised doc would otherwise pass as a correct one.
// A row count that disagrees does NOT short-circuit — the count is what proves
// the parse saw the whole table, but the offending names are what a reader needs
// to act on, so both are reported.
func auditDocTokenTable(doc []byte, want []string) []string {
	rows := parseDocTokenRows(doc)
	if len(rows) == 0 {
		return []string{fmt.Sprintf(
			"parsed no token rows: the role tables must open with a %q header cell and each row with a backticked token name",
			tokenTableHeading,
		)}
	}

	var problems []string
	if len(rows) != len(want) {
		problems = append(problems, fmt.Sprintf("token table declares %d rows, want %d", len(rows), len(want)))
	}

	documented := make(map[string]bool, len(rows))
	for _, name := range rows {
		documented[name] = true
	}
	known := make(map[string]bool, len(want))
	for _, name := range want {
		known[name] = true
	}

	for _, name := range want {
		if !documented[name] {
			problems = append(problems, fmt.Sprintf("token %q has no row in the doc", name))
		}
	}
	for _, name := range rows {
		if !known[name] {
			problems = append(problems, fmt.Sprintf("doc row %q names a token the vocabulary does not hold", name))
		}
	}
	return problems
}

// parseDocTokenRows returns the token names the doc's role tables declare, in
// document order.
//
// A row counts when it sits under a header row whose first cell is
// tokenTableHeading and its own first cell is a single backticked value.
// Everything else — prose, the delimiter row, tables about anything else — is
// skipped.
func parseDocTokenRows(doc []byte) []string {
	var names []string
	inTokenTable := false

	for line := range strings.SplitSeq(string(doc), "\n") {
		cells, isRow := markdownTableCells(line)
		if !isRow {
			inTokenTable = false
			continue
		}
		if len(cells) == 0 {
			continue
		}
		if cells[0] == tokenTableHeading {
			inTokenTable = true
			continue
		}
		if !inTokenTable {
			continue
		}
		if name, ok := backtickedValue(cells[0]); ok {
			names = append(names, name)
		}
	}

	return names
}

// markdownTableCells splits one markdown table row into its trimmed cells,
// reporting whether the line is a table row at all.
func markdownTableCells(line string) ([]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return nil, false
	}

	cells := strings.Split(strings.Trim(trimmed, "|"), "|")
	for i, cell := range cells {
		cells[i] = strings.TrimSpace(cell)
	}
	return cells, true
}

// backtickedValue returns the contents of a cell that is exactly one backticked
// value, and reports whether it was one. A delimiter cell, a prose cell or a
// cell carrying two code spans is not.
func backtickedValue(cell string) (string, bool) {
	if len(cell) < 3 || !strings.HasPrefix(cell, "`") || !strings.HasSuffix(cell, "`") {
		return "", false
	}
	inner := cell[1 : len(cell)-1]
	if inner == "" || strings.Contains(inner, "`") {
		return "", false
	}
	return inner, true
}

// auditDocExampleTheme parses the doc's example theme through the same parse and
// validation every theme file goes through, returning the palette it describes
// alongside one problem per failure.
func auditDocExampleTheme(doc []byte) (Theme, []string) {
	example, found := extractDocExampleTheme(doc)
	if !found {
		return Theme{}, []string{fmt.Sprintf("no fenced block under the %q heading", exampleThemeHeading)}
	}

	built, rejection := parseThemeBytes(example)
	if rejection != nil {
		return Theme{}, []string{fmt.Sprintf("the example theme is rejected as %q: %s", rejection.Reason, rejection.Detail)}
	}
	return built, nil
}

// extractDocExampleTheme returns the bytes of the first fenced block under the
// doc's example-theme heading.
func extractDocExampleTheme(doc []byte) ([]byte, bool) {
	lines := strings.Split(string(doc), "\n")

	start := indexOfHeading(lines, exampleThemeHeading)
	if start < 0 {
		return nil, false
	}

	opening := indexOfFence(lines, start)
	if opening < 0 {
		return nil, false
	}
	closing := indexOfFence(lines, opening+1)
	if closing < 0 {
		return nil, false
	}

	return []byte(strings.Join(lines[opening+1:closing], "\n") + "\n"), true
}

// indexOfHeading returns the index of the markdown heading whose text is
// exactly heading, or -1.
func indexOfHeading(lines []string, heading string) int {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.TrimSpace(strings.TrimLeft(trimmed, "#")) == heading {
			return i
		}
	}
	return -1
}

// indexOfFence returns the index of the first code-fence line at or after from,
// or -1.
func indexOfFence(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			return i
		}
	}
	return -1
}

// requireProblemNaming fails unless exactly one reported problem names the
// offender, so a synthetic drift is pinned to the message a reader would act on
// rather than to "something went wrong".
func requireProblemNaming(t *testing.T, problems []string, offender string) {
	t.Helper()

	var naming []string
	for _, problem := range problems {
		if strings.Contains(problem, offender) {
			naming = append(naming, problem)
		}
	}
	if len(naming) != 1 {
		t.Fatalf("problems = %v, want exactly one naming %q", problems, offender)
	}
}

// docWithTokenTable renders a minimal doc whose one role table declares names,
// so drift is exercised against synthetic content while the real doc stays the
// live case.
func docWithTokenTable(names []string) string {
	var doc strings.Builder
	doc.WriteString("# Themes\n\nProse that is not a table.\n\n")
	doc.WriteString("| " + tokenTableHeading + " | Role |\n|---|---|\n")
	for _, name := range names {
		fmt.Fprintf(&doc, "| `%s` | A role. |\n", name)
	}
	return doc.String()
}

// docWithExampleTheme renders a minimal doc carrying theme as its fenced
// example.
func docWithExampleTheme(theme string) string {
	return "# Themes\n\n## " + exampleThemeHeading + "\n\n```ini\n" + theme + "```\n"
}

// exampleThemeText renders names as a theme file, every value the same
// well-formed hex — validity here is about which keys are present, and the
// values are the other guard's subject.
func exampleThemeText(names []string) string {
	var theme strings.Builder
	for _, name := range names {
		fmt.Fprintf(&theme, "%s = #123456\n", name)
	}
	return theme.String()
}
