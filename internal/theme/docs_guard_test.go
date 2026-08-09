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

// TestThemingDocExampleThemeIsTheDarkBuiltin is the third live case: the doc's
// copy-pasteable theme IS the dark built-in it says it is.
//
// Validity is not enough here. The doc states the identity in prose — a reader
// is told these are the shipped values and may reasonably treat the block as the
// palette to diff their own against — so an example that merely parses can go on
// claiming to be the built-in while showing colours the binary no longer paints.
// Re-deriving those values is an ordinary change, and this is what makes the doc
// move with them.
func TestThemingDocExampleThemeIsTheDarkBuiltin(t *testing.T) {
	doc, err := readThemingDoc(themingDocPath)
	if err != nil {
		t.Fatalf("%v", err)
	}

	for _, problem := range auditDocExampleMatchesBuiltin(doc, requireDarkBuiltinSource(t)) {
		t.Errorf("%s: %s", themingDocPath, problem)
	}
}

// TestThemingDocGuard_ExampleHeaderCommentMayDiffer pins the one difference the
// comparison forgives, and pins it from the built-in's own body so the tolerance
// is exercised even if the two headers were ever made identical.
//
// The doc's header addresses someone about to copy the block; the file's
// addresses whoever maintains the palette. They say different things on purpose.
func TestThemingDocGuard_ExampleHeaderCommentMayDiffer(t *testing.T) {
	builtin := requireDarkBuiltinSource(t)

	problems := auditDocExampleMatchesBuiltin(exampleFromBody(bodyAfterHeaderComment(builtin)), builtin)

	if len(problems) != 0 {
		t.Errorf("auditDocExampleMatchesBuiltin() reported %v for the built-in's own body under a restated header, want no problems", problems)
	}
}

// TestThemingDocGuard_ExampleValueDivergingFromBuiltinFails is the drift the
// guard exists for: a palette value that moved in the .theme file and not in the
// doc.
func TestThemingDocGuard_ExampleValueDivergingFromBuiltinFails(t *testing.T) {
	builtin := requireDarkBuiltinSource(t)
	body, moved := rewriteFirstMatchingLine(t, bodyAfterHeaderComment(builtin), movedValue)

	problems := auditDocExampleMatchesBuiltin(exampleFromBody(body), builtin)

	requireProblemNaming(t, problems, moved)
}

// TestThemingDocGuard_ExampleSectionCommentDivergingFromBuiltinFails pins the
// other half of the tolerance rule: the LEADING comment is forgiven, and no
// other comment is.
//
// The section comments group the ramp and the surfaces, so they carry the same
// "this is the file, verbatim" claim the values do — forgiving every comment
// would let the doc quietly re-annotate a block it presents as a copy.
func TestThemingDocGuard_ExampleSectionCommentDivergingFromBuiltinFails(t *testing.T) {
	builtin := requireDarkBuiltinSource(t)
	body, restated := rewriteFirstMatchingLine(t, bodyAfterHeaderComment(builtin), restatedComment)

	problems := auditDocExampleMatchesBuiltin(exampleFromBody(body), builtin)

	requireProblemNaming(t, problems, restated)
}

// TestThemingDocGuard_ExampleWithNoBodyFailsLoudly refuses the vacuous
// comparison: two sources that are nothing but a header agree on every line they
// have, and a guard that reports agreement there is a guard that has stopped
// reading its subject.
func TestThemingDocGuard_ExampleWithNoBodyFailsLoudly(t *testing.T) {
	problems := auditDocExampleMatchesBuiltin([]byte(docWithExampleTheme(restatedHeader)), []byte(restatedHeader))

	if len(problems) != 1 {
		t.Fatalf("auditDocExampleMatchesBuiltin() reported %d problems for two header-only sources, want exactly the vacuous-comparison one: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "no theme lines") {
		t.Errorf("problem = %q, want it to say the comparison found no theme lines to compare", problems[0])
	}
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

// auditDocExampleMatchesBuiltin compares the doc's fenced example against the
// embedded source of the built-in it claims to reproduce, returning one problem
// per diverging line and nothing when the two agree.
//
// Everything below each source's leading comment is compared verbatim and in
// order — blank lines, section comments and key lines alike — so the only
// difference the doc is allowed is the header it opens with. A looser rule would
// have to decide which parts of a block presented as a copy may stop being one.
//
// Two bodies that are both empty short-circuit: they agree on every line they
// have, which is the one way this comparison can report success without having
// read a palette.
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

// bodyAfterHeaderComment returns src's lines from the first that is not part of
// the leading comment block, with a trailing newline normalised away.
//
// The header is the run of `#` lines the file OPENS with, and nothing further: a
// comment below the first key or blank line is body. The prefix is matched
// against the raw line rather than a trimmed one, which is stricter than the
// loader's own comment rule: the loader forgives whitespace before a `#`, so an
// indented header line is a comment to it and body here.
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

// requireDarkBuiltinSource returns the embedded source of the built-in the doc
// reproduces, failing rather than skipping when it resolves to nothing.
func requireDarkBuiltinSource(t *testing.T) []byte {
	t.Helper()

	source, found := BuiltinBytes(DefaultDarkSlug)
	if !found {
		t.Fatalf("the dark built-in %q resolves to no embedded source", DefaultDarkSlug)
	}
	return source
}

// restatedHeader stands in for the doc's own header comment, which says
// something different from the file's by design.
const restatedHeader = "# A header the file does not carry.\n"

// exampleFromBody renders a minimal doc whose fenced example carries body under
// a header of its own — the real doc's shape, with the body under test.
func exampleFromBody(body []string) []byte {
	return []byte(docWithExampleTheme(restatedHeader + strings.Join(body, "\n") + "\n"))
}

// rewriteFirstMatchingLine returns body with the first line rewrite accepts
// replaced, alongside the replacement it wrote.
//
// The line is FOUND rather than named, so a negative case never pins itself to a
// palette value: naming one would make the very drift this guard catches break
// its own test first. A rewrite that changes nothing is a failure — a divergence
// that is not a divergence would leave the case asserting against agreement.
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

// movedValue rewrites a `key = value` line to stand for a palette value that
// was re-derived in the .theme file, keeping the key so the divergence is the
// colour and nothing else.
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

// restatedComment rewrites a comment line to stand for the doc re-annotating a
// block it presents as a verbatim copy.
func restatedComment(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	return "# A remark the file does not make.", true
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
