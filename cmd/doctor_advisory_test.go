// Tests in this file mutate package-level Cobra/DI state (doctorDeps, rootCmd)
// and MUST NOT use t.Parallel.
//
// Concern-split from cmd/doctor_test.go (same source file, cmd/doctor.go),
// alongside cmd/doctor_summary_test.go: this file owns the ADVISORY line class —
// the user-content half of the two-class doctor contract — covering its trailing
// block position, its exclusion from the exit code, and the summary's advisory
// suffix. doctor_test.go keeps the check catalog; doctor_summary_test.go keeps
// the <N>/<T> counts. The split is by concern only; all three derive from
// doctor.go.
package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// doctorUnhealthy's sole input is the check catalog, so an advisory cannot reach
// the exit decision even by accident — the structural half of the guarantee
// TestAdvisories_NeverDriveExitCode asserts at runtime. Widening the signature
// stops this file compiling.
var _ func([]checkResult) bool = doctorUnhealthy

// themeAdvisories builds n distinct advisory values whose lines are shaped like
// the theme copy the producers will emit — each already carrying its leading
// "⚠ ", since the producers own the whole string and the renderer only indents.
// The exact wording is the producers' contract, not this file's: these stand-ins
// only need to be distinguishable and glyph-led.
func themeAdvisories(n int) []advisory {
	out := make([]advisory, 0, n)
	for i := range n {
		slug := fmt.Sprintf("theme%d", i)
		out = append(out, advisory{
			line: fmt.Sprintf("⚠ theme %s: bad colour — token text.primary", slug),
			slug: slug,
		})
	}
	return out
}

// renderedLines renders one report and returns its lines, with the trailing
// newline dropped rather than yielding a phantom empty final element — so a line
// index in a test is the index of a real rendered line.
func renderedLines(t *testing.T, results []checkResult, advisories []advisory) []string {
	t.Helper()
	var buf bytes.Buffer
	renderDoctorReport(&buf, results, advisories)
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("report does not end in a newline: %q", out)
	}
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

// mixedCatalog is a small three-check catalog spanning a marked pass, a marked
// fail and the unmarked informational line, so a positional assertion covers
// every marker shape the catalog region can render.
func mixedCatalog() []checkResult {
	return []checkResult{
		{name: "daemon", status: checkPass, detail: "running (pid 1, version v1.0.0)"},
		{name: "saver", status: checkFail, detail: "_portal-saver not running"},
		{name: "host terminal", status: checkInfo, detail: "Ghostty (supported)"},
	}
}

// TestAdvisories_BlockPositionIsFixed pins the report's three regions BY INDEX:
// header, then one line per check in catalog order, then one line per advisory
// in slice order, then the single summary — and nothing else.
func TestAdvisories_BlockPositionIsFixed(t *testing.T) {
	results := mixedCatalog()
	advisories := themeAdvisories(2)

	lines := renderedLines(t, results, advisories)

	want := []string{
		"Portal doctor:",
		"  ✓ daemon: running (pid 1, version v1.0.0)",
		"  ✗ saver: _portal-saver not running",
		"    host terminal: Ghostty (supported)",
		"  ⚠ theme theme0: bad colour — token text.primary",
		"  ⚠ theme theme1: bad colour — token text.primary",
		"  1 of 2 checks passed · 2 advisories",
	}
	if len(lines) != len(want) {
		t.Fatalf("rendered %d lines, want %d:\n%s", len(lines), len(want), strings.Join(lines, "\n"))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line[%d] = %q; want %q", i, lines[i], want[i])
		}
	}
}

// TestAdvisories_NeverInterleave asserts the structural property behind that
// fixed order over a larger report: every advisory line sits after every check
// line, and the advisory lines are contiguous. Interleaving would make a
// fixed-order report vary in position with the contents of a directory.
func TestAdvisories_NeverInterleave(t *testing.T) {
	results := []checkResult{
		{name: "daemon", status: checkPass, detail: "running"},
		{name: "saver", status: checkPass, detail: "_portal-saver up"},
		{name: "hooks", status: checkFail, detail: "hooks not registered on client-attached"},
		{name: "stale hooks", status: checkNotEvaluable, detail: "could not read hooks.json"},
	}
	advisories := themeAdvisories(3)

	lines := renderedLines(t, results, advisories)

	// The catalog occupies indices 1..len(results) (index 0 is the header), so
	// every check line must be found there and nowhere later.
	for i, r := range results {
		want := fmt.Sprintf("  %s %s: %s", checkMarker(r.status), r.name, r.detail)
		if got := lines[1+i]; got != want {
			t.Fatalf("line[%d] = %q; want the check line %q", 1+i, got, want)
		}
	}

	firstAdvisory := len(results) + 1
	for i, a := range advisories {
		if got, want := lines[firstAdvisory+i], "  "+a.line; got != want {
			t.Errorf("line[%d] = %q; want the advisory %q — the block must be contiguous and follow the whole catalog", firstAdvisory+i, got, want)
		}
	}

	for i, line := range lines {
		if !strings.Contains(line, "⚠") {
			continue
		}
		if i < firstAdvisory {
			t.Errorf("advisory line %q rendered at index %d, inside the catalog region (which ends at %d)", line, i, len(results))
		}
	}
}

// TestAdvisories_HostTerminalStaysInCatalog pins the informational host-terminal
// line as the LAST line of the catalog region rather than the first of the
// advisory block: the report has three regions, not two. It runs the real
// diagnosis so the assertion is against the production catalog order.
func TestAdvisories_HostTerminalStaysInCatalog(t *testing.T) {
	deps := healthyDoctorDeps(t)
	results, err := runDoctorDiagnosis(deps)
	if err != nil {
		t.Fatalf("runDoctorDiagnosis: %v", err)
	}
	last := results[len(results)-1]
	if last.name != "host terminal" || last.status != checkInfo {
		t.Fatalf("last catalog check = %+v; want the informational host-terminal line", last)
	}

	advisories := themeAdvisories(2)
	lines := renderedLines(t, results, advisories)

	hostLine := fmt.Sprintf("  %s %s: %s", checkMarker(last.status), last.name, last.detail)
	if got := lines[len(results)]; got != hostLine {
		t.Errorf("line[%d] = %q; want the host-terminal line %q at the end of the catalog", len(results), got, hostLine)
	}
	if got, want := lines[len(results)+1], "  "+advisories[0].line; got != want {
		t.Errorf("line[%d] = %q; want the first advisory %q immediately after the catalog", len(results)+1, got, want)
	}
}

// TestAdvisories_SummarySuffix pins the suffix against BOTH summary forms and
// both grammatical numbers: the advisory count appends to whichever form the
// check counts produced, and never alters those counts.
func TestAdvisories_SummarySuffix(t *testing.T) {
	allPassed := []checkResult{
		{name: "daemon", status: checkPass},
		{name: "saver", status: checkPass},
	}
	someFailed := []checkResult{
		{name: "daemon", status: checkPass},
		{name: "saver", status: checkFail},
	}

	cases := []struct {
		name       string
		results    []checkResult
		advisories int
		want       string
	}{
		{"all passed, one advisory", allPassed, 1, "2 checks passed · 1 advisory"},
		{"all passed, two advisories", allPassed, 2, "2 checks passed · 2 advisories"},
		{"all passed, five advisories", allPassed, 5, "2 checks passed · 5 advisories"},
		{"some failed, one advisory", someFailed, 1, "1 of 2 checks passed · 1 advisory"},
		{"some failed, three advisories", someFailed, 3, "1 of 2 checks passed · 3 advisories"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			advisories := themeAdvisories(tc.advisories)
			if got := doctorSummaryLine(tc.results, advisories); got != tc.want {
				t.Errorf("doctorSummaryLine = %q; want %q", got, tc.want)
			}

			// <M> is the length of the slice the renderer was handed — the same
			// number reaches the rendered line.
			lines := renderedLines(t, tc.results, advisories)
			if got, want := lines[len(lines)-1], "  "+tc.want; got != want {
				t.Errorf("rendered summary = %q; want %q", got, want)
			}
		})
	}
}

// TestAdvisories_SuffixSuppressedAtZero pins the M=0 output byte-identical to the
// summary as it stood before the advisory class existed: no separator, no
// "0 advisories". A clean install's report gains no stray punctuation.
func TestAdvisories_SuffixSuppressedAtZero(t *testing.T) {
	cases := []struct {
		name       string
		advisories []advisory
	}{
		{"nil slice", nil},
		{"empty slice", []advisory{}},
	}

	results := mixedCatalog()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctorSummaryLine(results, tc.advisories)
			if want := "1 of 2 checks passed"; got != want {
				t.Errorf("doctorSummaryLine = %q; want %q", got, want)
			}
			if strings.Contains(got, "·") || strings.Contains(got, "advisor") {
				t.Errorf("doctorSummaryLine = %q; the suffix must be suppressed entirely at zero advisories", got)
			}
		})
	}
}

// TestAdvisories_NeverDriveExitCode asserts the class boundary the contract
// amendment exists for. It has two halves because production supplies an empty
// advisory slice in this phase (the theme scan is the first producer): the
// Execute half proves an all-passing catalog exits 0 through the real command
// body, and the advisory half renders THAT SAME catalog with five advisories and
// re-asks doctorUnhealthy — which also catches a renderer that folded the
// advisories into the results slice it was handed.
func TestAdvisories_NeverDriveExitCode(t *testing.T) {
	deps := healthyDoctorDeps(t)
	if _, _, err := runDoctorCmd(t, deps); err != nil {
		t.Fatalf("Execute err = %v; want nil over an all-passing catalog", err)
	}

	results, err := runDoctorDiagnosis(deps)
	if err != nil {
		t.Fatalf("runDoctorDiagnosis: %v", err)
	}
	if doctorUnhealthy(results) {
		t.Fatalf("precondition: the catalog must be all-passing, got %+v", results)
	}

	lines := renderedLines(t, results, themeAdvisories(5))

	if doctorUnhealthy(results) {
		t.Error("doctorUnhealthy = true after rendering five advisories; an advisory must never reach the exit decision")
	}
	if got, want := lines[len(lines)-1], "  7 checks passed · 5 advisories"; got != want {
		t.Errorf("rendered summary = %q; want %q — five advisories are counted, and the run is still healthy", got, want)
	}
}

// TestAdvisories_FailingCheckAndAdvisoriesShareOneSummary pins ONE summary line
// carrying both counts when the two classes report at once: the check counts
// explain the non-zero exit, the advisory count sits beside them, and no second
// summary line is minted for the second class.
func TestAdvisories_FailingCheckAndAdvisoriesShareOneSummary(t *testing.T) {
	deps := healthyDoctorDeps(t)
	deps.SaverPresent = func() (bool, error) { return false, nil }

	if _, _, err := runDoctorCmd(t, deps); err != ErrDoctorUnhealthy {
		t.Fatalf("Execute err = %v; want ErrDoctorUnhealthy with a failing saver check", err)
	}

	results, err := runDoctorDiagnosis(deps)
	if err != nil {
		t.Fatalf("runDoctorDiagnosis: %v", err)
	}

	lines := renderedLines(t, results, themeAdvisories(2))

	var summaries []string
	for _, line := range lines {
		if strings.Contains(line, "checks passed") {
			summaries = append(summaries, line)
		}
	}
	if len(summaries) != 1 {
		t.Fatalf("summary line count = %d, want 1:\n%s", len(summaries), strings.Join(lines, "\n"))
	}
	if got, want := summaries[0], "  6 of 7 checks passed · 2 advisories"; got != want {
		t.Errorf("summary = %q; want %q", got, want)
	}
}

// TestAdvisories_GlyphBackedNoMarker pins the advisory line's shape: the two-space
// body indent and then the advisory's own string, verbatim — no pass/fail marker,
// no name column, nothing appended. Its text leads with the ⚠ glyph the producers
// bake in, so the class survives a colourless terminal.
func TestAdvisories_GlyphBackedNoMarker(t *testing.T) {
	a := advisory{line: "⚠ themes directory unreadable: /home/u/.config/portal/themes", slug: "", fromPrefs: false}

	lines := renderedLines(t, nil, []advisory{a})
	got := lines[1]

	if want := "  " + a.line; got != want {
		t.Errorf("advisory line = %q; want %q — indent only, nothing prepended or appended", got, want)
	}
	if !strings.HasPrefix(got, "  ⚠") {
		t.Errorf("advisory line = %q; want the glyph immediately after the indent, with no marker column", got)
	}
	for _, marker := range []string{"✓", "✗"} {
		if strings.Contains(got, marker) {
			t.Errorf("advisory line = %q; must carry no pass/fail marker (%q)", got, marker)
		}
	}

	t.Run("the renderer reads only line", func(t *testing.T) {
		// slug and fromPrefs are the producers' dedup identity, not render input:
		// two advisories differing only there must render identically.
		other := advisory{line: a.line, slug: "nord-lee", fromPrefs: true}
		if got, want := renderedLines(t, nil, []advisory{other})[1], "  "+a.line; got != want {
			t.Errorf("advisory line = %q; want %q — slug and fromPrefs must not reach the rendered line", got, want)
		}
	})
}

// TestAdvisories_EmptyBlockRendersNothing pins zero bytes between the last check
// line and the summary when there are no advisories: no blank line, no heading,
// and no difference between a nil and an empty slice.
func TestAdvisories_EmptyBlockRendersNothing(t *testing.T) {
	results := mixedCatalog()

	var nilBuf, emptyBuf bytes.Buffer
	renderDoctorReport(&nilBuf, results, nil)
	renderDoctorReport(&emptyBuf, results, []advisory{})

	want := "Portal doctor:\n" +
		"  ✓ daemon: running (pid 1, version v1.0.0)\n" +
		"  ✗ saver: _portal-saver not running\n" +
		"    host terminal: Ghostty (supported)\n" +
		"  1 of 2 checks passed\n"
	if got := nilBuf.String(); got != want {
		t.Errorf("report with no advisories mismatch\n got: %q\nwant: %q", got, want)
	}
	if nilBuf.String() != emptyBuf.String() {
		t.Errorf("nil and empty advisory slices render differently:\n nil: %q\nempty: %q", nilBuf.String(), emptyBuf.String())
	}
}
