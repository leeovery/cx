// Tests in this file mutate package-level Cobra/DI state (doctorDeps, rootCmd)
// and MUST NOT use t.Parallel.
//
// Concern-split from cmd/doctor_test.go (same source file, cmd/doctor.go): this
// file owns the closing-summary line — doctorCheckCounts, doctorSummaryLine and
// renderDoctorReport's trailing line — while doctor_test.go keeps the check
// catalog itself. The split is by concern only; both are derived from doctor.go.
package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// healthyDoctorDeps builds a DoctorDeps whose entire seven-check catalog passes:
// a seeded state dir (live daemon.pid, version marker, valid sessions.json), the
// three healthy runtime probes from withHealthyRuntime, an empty hooks.json over
// a NON-empty live-pane set (so the mass-deletion hazard guard does not defer to
// not-evaluable), and a projects.json naming only an extant directory. The
// informational host-terminal line rides along from withHealthyRuntime's stubbed
// detector, which is exactly what the counts must ignore.
func healthyDoctorDeps(t *testing.T) *DoctorDeps {
	t.Helper()
	dir := t.TempDir()
	seedHealthyStateDir(t, dir)
	hookStore, _ := seedHooksJSON(t)
	projectStore, _ := seedProjectsJSON(t, t.TempDir())
	return staleDeps(dir, fakeHookLister{keys: []string{"sessA:0.0"}}, hookStore, projectStore)
}

// TestDoctorSummary_AllChecksPassed drives the real command body over a fully
// healthy catalog: all seven Portal-health checks pass and the informational
// host-terminal line is present, so the report ends with the all-passed form
// counting seven — the info line in neither count.
func TestDoctorSummary_AllChecksPassed(t *testing.T) {
	outBuf, _, err := runDoctorCmd(t, healthyDoctorDeps(t))
	if err != nil {
		t.Fatalf("Execute err = %v; want nil when every check passes", err)
	}

	const want = "\n  7 checks passed\n"
	if out := outBuf.String(); !strings.HasSuffix(out, want) {
		t.Errorf("report does not end with %q:\n%s", want, out)
	}
}

// TestDoctorSummary_PartialForm drives the real command body with exactly one of
// the seven health checks failing (the saver probe reports absent): the summary
// switches to the "<N> of <T>" form — the case it exists for — and the exit stays
// non-zero, since the summary explains the exit code and never computes it.
func TestDoctorSummary_PartialForm(t *testing.T) {
	deps := healthyDoctorDeps(t)
	deps.SaverPresent = func() (bool, error) { return false, nil }

	outBuf, _, err := runDoctorCmd(t, deps)
	if err != ErrDoctorUnhealthy {
		t.Fatalf("Execute err = %v; want ErrDoctorUnhealthy with a failing saver check", err)
	}

	const want = "\n  6 of 7 checks passed\n"
	if out := outBuf.String(); !strings.HasSuffix(out, want) {
		t.Errorf("report does not end with %q:\n%s", want, out)
	}
}

// TestDoctorSummary_InfoLineOutsideCounts pins the informational host-terminal
// line out of BOTH counts: appending it to a catalog must leave passed and total
// untouched. Counting it in the total alone would render "2 of 3 checks passed"
// beside exit 0 — a healthy run reading as partial.
func TestDoctorSummary_InfoLineOutsideCounts(t *testing.T) {
	catalog := []checkResult{
		{name: "daemon", status: checkPass, detail: "running"},
		{name: "saver", status: checkPass, detail: "_portal-saver up"},
	}
	withInfo := append(slices.Clone(catalog), checkResult{name: "host terminal", status: checkInfo, detail: "Ghostty (supported)"})

	wantPassed, wantTotal := doctorCheckCounts(catalog)
	gotPassed, gotTotal := doctorCheckCounts(withInfo)
	if gotPassed != wantPassed || gotTotal != wantTotal {
		t.Errorf("doctorCheckCounts(with info line) = (%d, %d); want (%d, %d) — the info line must count for neither", gotPassed, gotTotal, wantPassed, wantTotal)
	}
	if got := doctorSummaryLine(withInfo, nil); got != "2 checks passed" {
		t.Errorf("doctorSummaryLine(with info line) = %q; want %q", got, "2 checks passed")
	}
}

// TestDoctorSummary_NotEvaluableOutsideCounts drives the real command body with
// one of the seven checks not-evaluable (a transient saver-probe error): the
// summary reads the all-passed form over the remaining six (N == T == 6) and the
// run still exits 0, because a not-evaluable check never drives the exit code.
func TestDoctorSummary_NotEvaluableOutsideCounts(t *testing.T) {
	deps := healthyDoctorDeps(t)
	deps.SaverPresent = func() (bool, error) { return false, errors.New("tmux transient") }

	outBuf, _, err := runDoctorCmd(t, deps)
	if err != nil {
		t.Fatalf("Execute err = %v; want nil — a not-evaluable check must not drive the exit code", err)
	}

	const want = "\n  6 checks passed\n"
	if out := outBuf.String(); !strings.HasSuffix(out, want) {
		t.Errorf("report does not end with %q:\n%s", want, out)
	}
}

// TestDoctorSummary_UnknownCountsTowardTotalOnly pins the iota-0 sentinel into
// the total but not the passed count. checkUnknown already reads as unhealthy in
// doctorUnhealthy, so counting it anywhere else here would make the summary and
// the exit code disagree about the same run.
func TestDoctorSummary_UnknownCountsTowardTotalOnly(t *testing.T) {
	results := []checkResult{
		{name: "daemon", status: checkPass, detail: "running"},
		{name: "forgotten"}, // zero-value status — a missed assignment
	}

	passed, total := doctorCheckCounts(results)
	if passed != 1 || total != 2 {
		t.Errorf("doctorCheckCounts = (%d, %d); want (1, 2) — checkUnknown counts toward the total only", passed, total)
	}
	if !doctorUnhealthy(results) {
		t.Error("doctorUnhealthy = false; the iota-0 sentinel must read as unhealthy")
	}
	if got := doctorSummaryLine(results, nil); got != "1 of 2 checks passed" {
		t.Errorf("doctorSummaryLine = %q; want %q", got, "1 of 2 checks passed")
	}
}

// TestDoctorSummary_MatchesDoctorUnhealthy asserts the equivalence the summary
// rests on as a PROPERTY rather than assuming it: passed == total exactly when
// the run is healthy, across every status and their mixtures. The rendered form
// is checked alongside it, so the "<N> of <T>" wording appears precisely when the
// exit is non-zero.
func TestDoctorSummary_MatchesDoctorUnhealthy(t *testing.T) {
	pass := checkResult{name: "pass", status: checkPass, detail: "ok"}
	fail := checkResult{name: "fail", status: checkFail, detail: "broken"}
	unknown := checkResult{name: "forgotten"} // zero-value status — checkUnknown
	info := checkResult{name: "host terminal", status: checkInfo, detail: "Ghostty (supported)"}
	notEval := checkResult{name: "stale hooks", status: checkNotEvaluable, detail: "not evaluable"}

	cases := []struct {
		name    string
		results []checkResult
	}{
		{"empty catalog", nil},
		{"pass only", []checkResult{pass, pass, pass}},
		{"one fail", []checkResult{pass, fail, pass}},
		{"one unknown", []checkResult{pass, unknown, pass}},
		{"one info", []checkResult{pass, pass, info}},
		{"one not-evaluable", []checkResult{pass, pass, notEval}},
		{"info and not-evaluable alongside all-pass", []checkResult{pass, pass, info, notEval}},
		{"info and not-evaluable alongside a fail", []checkResult{pass, fail, info, notEval}},
		{"fail and unknown together", []checkResult{fail, unknown}},
		{"every status at once", []checkResult{pass, fail, unknown, info, notEval}},
		{"info only", []checkResult{info}},
		{"not-evaluable only", []checkResult{notEval}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			passed, total := doctorCheckCounts(tc.results)
			healthy := !doctorUnhealthy(tc.results)

			if (passed == total) != healthy {
				t.Errorf("doctorCheckCounts = (%d, %d) so passed==total is %t, but doctorUnhealthy reports healthy=%t — the summary and the exit code disagree about the same run",
					passed, total, passed == total, healthy)
			}

			want := fmt.Sprintf("%d of %d checks passed", passed, total)
			if healthy {
				want = fmt.Sprintf("%d checks passed", passed)
			}
			if got := doctorSummaryLine(tc.results, nil); got != want {
				t.Errorf("doctorSummaryLine = %q; want %q", got, want)
			}
		})
	}
}

// TestDoctorSummary_IsTheLastLine pins renderDoctorReport's whole output byte
// for byte over a mixed catalog: the summary is the final line, there is no
// blank line between it and the last check line, and nothing trails it.
func TestDoctorSummary_IsTheLastLine(t *testing.T) {
	results := []checkResult{
		{name: "daemon", status: checkPass, detail: "running (pid 1, version v1.0.0)"},
		{name: "saver", status: checkFail, detail: "_portal-saver not running"},
		{name: "stale hooks", status: checkNotEvaluable, detail: "could not read hooks.json"},
		{name: "host terminal", status: checkInfo, detail: "Ghostty (supported)"},
	}

	var buf bytes.Buffer
	renderDoctorReport(&buf, results, nil)

	want := "Portal doctor:\n" +
		"  ✓ daemon: running (pid 1, version v1.0.0)\n" +
		"  ✗ saver: _portal-saver not running\n" +
		"  · stale hooks: could not read hooks.json\n" +
		"    host terminal: Ghostty (supported)\n" +
		"  1 of 2 checks passed\n"
	if got := buf.String(); got != want {
		t.Errorf("renderDoctorReport output mismatch\n got: %q\nwant: %q", got, want)
	}
}

// TestDoctorSummary_FixPathRendersTwo pins one summary per report render: the
// --fix path renders twice, and each summary reflects its OWN pass — the
// pre-repair report counts the stale-hook failure, the post-repair report counts
// the pruned catalog clean.
func TestDoctorSummary_FixPathRendersTwo(t *testing.T) {
	dir := t.TempDir()
	seedHealthyStateDir(t, dir)
	// A persisted key absent from a NON-empty live set is genuinely stale (the
	// hazard guard does not defer), so the first pass fails the stale-hooks check
	// and --fix prunes it before the second.
	hookStore, _ := seedHooksJSON(t, "sessA:0.0")
	projectStore, _ := seedProjectsJSON(t, t.TempDir())
	deps := staleDeps(dir, fakeHookLister{keys: []string{"sessB:0.0"}}, hookStore, projectStore)

	outBuf, _, err := runDoctorFixCmd(t, deps)
	if err != nil {
		t.Fatalf("Execute err = %v; want nil (healthy post-repair)", err)
	}

	out := outBuf.String()
	if n := strings.Count(out, " checks passed\n"); n != 2 {
		t.Errorf("summary line count = %d; want 2 (one per renderDoctorReport call):\n%s", n, out)
	}
	if n := strings.Count(out, "\n  6 of 7 checks passed\n"); n != 1 {
		t.Errorf("pre-repair summary missing or repeated (want exactly one \"6 of 7 checks passed\"):\n%s", out)
	}
	if !strings.HasSuffix(out, "\n  7 checks passed\n") {
		t.Errorf("post-repair report does not end with the all-passed summary:\n%s", out)
	}
}

// TestDoctorSummary_NoSingularCarveOut pins the single form for the checks
// count: one passing check reads "1 checks passed", never the grammatical
// "1 check passed". The count deliberately does NOT route through pluralCount.
func TestDoctorSummary_NoSingularCarveOut(t *testing.T) {
	cases := []struct {
		name    string
		results []checkResult
		want    string
	}{
		{"one passing check", []checkResult{{name: "daemon", status: checkPass}}, "1 checks passed"},
		{"one failing check", []checkResult{{name: "daemon", status: checkFail}}, "0 of 1 checks passed"},
		{"one passing beside one failing", []checkResult{{name: "daemon", status: checkPass}, {name: "saver", status: checkFail}}, "1 of 2 checks passed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctorSummaryLine(tc.results, nil)
			if got != tc.want {
				t.Errorf("doctorSummaryLine = %q; want %q — the checks count has exactly one form", got, tc.want)
			}
			if strings.Contains(got, "check passed") {
				t.Errorf("doctorSummaryLine = %q; the singular \"check passed\" must never render", got)
			}
		})
	}
}
