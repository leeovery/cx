package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var themingDocPath = filepath.Join("..", "..", "docs", "theming.md")

const tokenTableHeading = "Token"

const exampleThemeHeading = "Example theme"

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

func TestThemingDocGuard_TokenAbsentFromTableFails(t *testing.T) {
	names := TokenNames()
	undocumented := names[len(names)-1]
	doc := docWithTokenTable(names[:len(names)-1])

	problems := auditDocTokenTable([]byte(doc), names)

	requireProblemNaming(t, problems, undocumented)
}

func TestThemingDocGuard_UnknownTableRowFails(t *testing.T) {
	const retired = "border.footer"
	names := TokenNames()
	doc := docWithTokenTable(append(slices.Clone(names), retired))

	problems := auditDocTokenTable([]byte(doc), names)

	requireProblemNaming(t, problems, retired)
}

func TestThemingDocGuard_RowOrderIsNotAsserted(t *testing.T) {
	names := TokenNames()
	reversed := slices.Clone(names)
	slices.Reverse(reversed)

	problems := auditDocTokenTable([]byte(docWithTokenTable(reversed)), names)

	if len(problems) != 0 {
		t.Errorf("auditDocTokenTable() reported %v for a table carrying every token in reverse order, want no problems", problems)
	}
}

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

func TestThemingDocGuard_ExampleMissingTokenFails(t *testing.T) {
	names := TokenNames()
	dropped := names[0]
	doc := docWithExampleTheme(exampleThemeText(names[1:]))

	_, problems := auditDocExampleTheme([]byte(doc))

	requireProblemNaming(t, problems, dropped)
}

func TestThemingDocExampleThemeIsTheDarkBuiltin(t *testing.T) {
	doc, err := readThemingDoc(themingDocPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	for _, problem := range auditDocExampleMatchesBuiltin(doc, requireDarkBuiltinSource(t)) {
		t.Errorf("%s: %s", themingDocPath, problem)
	}
}

func TestThemingDocGuard_ExampleHeaderCommentMayDiffer(t *testing.T) {
	builtin := requireDarkBuiltinSource(t)

	problems := auditDocExampleMatchesBuiltin(exampleFromBody(bodyAfterHeaderComment(builtin)), builtin)

	if len(problems) != 0 {
		t.Errorf("auditDocExampleMatchesBuiltin() reported %v for the built-in's own body under a restated header, want no problems", problems)
	}
}

func TestThemingDocGuard_ExampleValueDivergingFromBuiltinFails(t *testing.T) {
	builtin := requireDarkBuiltinSource(t)
	body, moved := rewriteFirstMatchingLine(t, bodyAfterHeaderComment(builtin), movedValue)

	problems := auditDocExampleMatchesBuiltin(exampleFromBody(body), builtin)

	requireProblemNaming(t, problems, moved)
}

func TestThemingDocGuard_ExampleSectionCommentDivergingFromBuiltinFails(t *testing.T) {
	builtin := requireDarkBuiltinSource(t)
	body, restated := rewriteFirstMatchingLine(t, bodyAfterHeaderComment(builtin), restatedComment)

	problems := auditDocExampleMatchesBuiltin(exampleFromBody(body), builtin)

	requireProblemNaming(t, problems, restated)
}

func TestThemingDocGuard_ExampleWithNoBodyFailsLoudly(t *testing.T) {
	problems := auditDocExampleMatchesBuiltin([]byte(docWithExampleTheme(restatedHeader)), []byte(restatedHeader))

	if len(problems) != 1 {
		t.Fatalf("auditDocExampleMatchesBuiltin() reported %d problems for two header-only sources, want exactly the vacuous-comparison one: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "no theme lines") {
		t.Errorf("problem = %q, want it to say the comparison found no theme lines to compare", problems[0])
	}
}

func readThemingDoc(path string) ([]byte, error) {
	doc, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the theme documentation at %s: %w", path, err)
	}
	return doc, nil
}

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

func indexOfFence(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			return i
		}
	}
	return -1
}

func auditDocExampleMatchesBuiltin(doc, builtin []byte) []string {
	example, found := extractDocExampleTheme(doc)
	if !found {
		return []string{fmt.Sprintf("no fenced block under the %q heading", exampleThemeHeading)}
	}

	documented := bodyAfterHeaderComment(example)
	shipped := bodyAfterHeaderComment(builtin)
	if len(documented) == 0 && len(shipped) == 0 {
		return []string{"the example and the built-in hold no theme lines below their header comments: every comparison below would hold vacuously"}
	}

	var problems []string
	for i := range max(len(documented), len(shipped)) {
		switch {
		case i >= len(documented):
			problems = append(problems, fmt.Sprintf("the example stops short of the built-in, which continues %q", shipped[i]))
		case i >= len(shipped):
			problems = append(problems, fmt.Sprintf("the example runs past the built-in, carrying %q", documented[i]))
		case documented[i] != shipped[i]:
			problems = append(problems, fmt.Sprintf("the example reads %q where the built-in reads %q", documented[i], shipped[i]))
		}
	}
	return problems
}

func bodyAfterHeaderComment(src []byte) []string {
	text := strings.TrimSuffix(string(src), "\n")
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "#") {
			return lines[i:]
		}
	}
	return nil
}

func requireDarkBuiltinSource(t *testing.T) []byte {
	t.Helper()

	source, found := BuiltinBytes(DefaultDarkSlug)
	if !found {
		t.Fatalf("the dark built-in %q resolves to no embedded source", DefaultDarkSlug)
	}
	return source
}

const restatedHeader = "# A header the file does not carry.\n"

func exampleFromBody(body []string) []byte {
	return []byte(docWithExampleTheme(restatedHeader + strings.Join(body, "\n") + "\n"))
}

func rewriteFirstMatchingLine(t *testing.T, body []string, rewrite func(string) (string, bool)) ([]string, string) {
	t.Helper()

	for i, line := range body {
		replacement, ok := rewrite(line)
		if !ok {
			continue
		}
		if replacement == line {
			t.Fatalf("rewriting %q produced the same line — the divergence under test was never staged", line)
		}

		mutated := slices.Clone(body)
		mutated[i] = replacement
		return mutated, replacement
	}

	t.Fatalf("no line of %d matched the rewrite — the divergence under test was never staged", len(body))
	return nil, ""
}

func movedValue(line string) (string, bool) {
	if strings.HasPrefix(line, "#") {
		return "", false
	}
	key, _, ok := strings.Cut(line, " = ")
	if !ok {
		return "", false
	}
	return key + " = #010203", true
}

func restatedComment(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	return "# A remark the file does not make.", true
}

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

func docWithTokenTable(names []string) string {
	var doc strings.Builder
	doc.WriteString("# Themes\n\nProse that is not a table.\n\n")
	doc.WriteString("| " + tokenTableHeading + " | Role |\n|---|---|\n")
	for _, name := range names {
		fmt.Fprintf(&doc, "| `%s` | A role. |\n", name)
	}
	return doc.String()
}

func docWithExampleTheme(theme string) string {
	return "# Themes\n\n## " + exampleThemeHeading + "\n\n```ini\n" + theme + "```\n"
}

func exampleThemeText(names []string) string {
	var theme strings.Builder
	for _, name := range names {
		fmt.Fprintf(&theme, "%s = #123456\n", name)
	}
	return theme.String()
}
