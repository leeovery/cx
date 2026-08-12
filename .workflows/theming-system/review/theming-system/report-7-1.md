TASK: theming-system-7-1 — Doctor's Closing Summary — Portal-Health Checks Counted Apart

ACCEPTANCE CRITERIA:
- All seven health checks pass + host-terminal info line present → report ends with `  7 checks passed` (info line in neither count)
- One `checkFail` among seven → `  6 of 7 checks passed`, still exits non-zero
- One `checkNotEvaluable` among seven → `  6 checks passed` (N == T == 6), exits 0
- A `checkUnknown` result yields N < T and `doctorUnhealthy == true`; the two agree
- `doctorCheckCounts` and `doctorUnhealthy` agree on every table row: `passed == total` iff healthy
- Summary is the final line of `renderDoctorReport`'s output; no trailing blank line, no blank line before it
- `portal doctor --fix` prints exactly two summary lines, one per report render, each reflecting its own pass
- `ErrDoctorUnhealthy`, `doctorUnhealthy`, `checkMarker`, `runDoctorDiagnosis`, catalog order byte-unchanged; exit codes unchanged
- Summary copy single-sourced in `doctorSummaryLine` — no format string for it at any other site

STATUS: complete

SPEC CONTEXT:
§14A (specification.md:1850–1853) pins two forms for the closing summary — `<N> checks passed` when all pass, `<N> of <T> checks passed` otherwise — and states that `<N>`/`<T>` count **Portal-health checks only**, the class that drives the exit code, with advisories counted separately by `<M>` and never folded in. §14A explicitly names the summary line as **new** ("today's report is a header plus one line per check with no trailing summary, so every run gains a line — that is the amendment §15.1 names, not a regression"), which is why every pre-existing doctor output assertion legitimately changes. §12.2 (specification.md:1465) states the summary must distinguish the two counts so the exit code's meaning is legible without reading the contract. The spec deliberately does **not** pin the summary's indentation or whether a blank line separates it from the catalog — the task flagged that ambiguity and chose two-space indent with no separating blank line.

IMPLEMENTATION:
- Status: Implemented (signature intentionally extended by the follow-on task 7-2)
- Location:
  - `cmd/doctor.go:485-500` — `doctorCheckCounts(results []checkResult) (passed, total int)`, sited immediately after `doctorUnhealthy` (`cmd/doctor.go:476-483`) as the task specified. Membership is exactly as pinned: `checkPass` → both counters; `checkFail, checkUnknown` → `total` only; `checkInfo, checkNotEvaluable` → an explicit empty arm counted by neither, carrying the justification comment.
  - `cmd/doctor.go:502-514` — `doctorSummaryLine`, producing `"%d checks passed"` when `passed == total` and `"%d of %d checks passed"` otherwise. No singular carve-out, and the checks count is deliberately not routed through `pluralCount` (`cmd/doctor.go:399`) — the doc comment states the asymmetry against the advisory count, which does take it.
  - `cmd/doctor.go:458` — rendered from `renderDoctorReport` as the last line written, `"  %s\n"`: two-space indent, no marker column, no name column, no preceding blank line, no trailing blank line.
- Notes:
  - **Amended by task 7-2, not drift.** Both helpers now take a second `advisories []advisory` parameter and `doctorSummaryLine` appends the ` · <M> advisory|advisories` suffix (`cmd/doctor.go:510-512`), and the advisory block renders between the catalog and the summary (`cmd/doctor.go:453-457`). That is exactly the phase boundary this task's Context describes ("task 7-2 … appends to whichever form this task produces"). The summary remains the last line in both shapes.
  - The counting class is uncontaminated by the theme feature: `runDoctorDiagnosis` (`cmd/doctor.go`) still returns the same seven health checks plus the conditional `checkHostTerminal` info line; theme findings land in the separate advisory slice, so they cannot move `<N>` or `<T>`.
  - Byte-unchanged claim holds: commit `6d4b7200` touches `cmd/doctor.go` with 61 insertions and zero deletions — `ErrDoctorUnhealthy`, `doctorUnhealthy`, `checkMarker`, `runDoctorDiagnosis` and the catalog order are untouched by this task.
  - Single-sourcing holds: `grep` over all non-test Go sources finds the `checks passed` format strings only at `cmd/doctor.go:506` and `:508`. Both `renderDoctorReport` call sites (`cmd/doctor.go:155` diagnose, `:176` post-repair re-diagnose) route through the one function, so `--fix` emits exactly two summaries structurally rather than by convention.
  - The rich doc comments this task shipped (the per-arm justification block on `doctorCheckCounts`, the "explains the exit code and never computes it" boundary, and the framing-is-a-local-choice note on `renderDoctorReport`) were later condensed by the repo-wide comment-standard sweeps (`e30939b2` task 11-3, then the `chore(comments)` commits `a4bc7bd5` / `915e7fcb`). The surviving comments are accurate and carry the load-bearing claims; only the whitespace-framing record was lost entirely (see the note below).

TESTS:
- Status: Adequate
- Coverage: `cmd/doctor_summary_test.go` implements all nine named micro-acceptance tests, and each maps to a criterion:
  - `TestDoctorSummary_AllChecksPassed` (:23) — end-to-end through `rootCmd.Execute()`, asserts the output ends with `"\n  7 checks passed\n"`. `healthyDoctorDeps` → `staleDeps` → `withHealthyRuntime` supplies `Detector`/`Resolve`, so the host-terminal info line *is* present in the report while sitting outside the count — the criterion's exact condition, not a weaker variant.
  - `TestDoctorSummary_PartialForm` (:35) — a failing saver check; asserts both `ErrDoctorUnhealthy` and the `"6 of 7"` suffix, so form and exit code are pinned together.
  - `TestDoctorSummary_InfoLineOutsideCounts` (:50) — differential: counts the same catalog with and without the info line and asserts identical `(passed, total)`. Stronger than asserting a literal, since it cannot pass by coincidence of totals.
  - `TestDoctorSummary_NotEvaluableOutsideCounts` (:67) — drives not-evaluable via a transient `SaverPresent` error and asserts `err == nil` alongside `"6 checks passed"` (N == T), directly refuting the `6 of 7`-beside-exit-0 failure mode the task named.
  - `TestDoctorSummary_UnknownCountsTowardTotalOnly` (:82) — uses a literal zero-value `checkResult{name: "forgotten"}`, so it pins the iota-0 sentinel rather than a named constant, and asserts `doctorUnhealthy` agreement in the same test.
  - `TestDoctorSummary_MatchesDoctorUnhealthy` (:100) — the equivalence asserted as a property over 12 rows (empty catalog, pass-only, one fail, one unknown, one info, one not-evaluable, info+not-evaluable with all-pass, info+not-evaluable with a fail, fail+unknown, every status at once, info-only, not-evaluable-only). It computes `(passed == total) != healthy` rather than hardcoding expectations, so it genuinely tests the property. Covers the task's required combinations and then some.
  - `TestDoctorSummary_IsTheLastLine` (:146) — exact full-string equality on `renderDoctorReport`'s buffer, which is the only assertion shape that can prove *no blank line before* and *no trailing blank line* simultaneously. `HasSuffix` alone could not.
  - `TestDoctorSummary_FixPathRendersTwo` (:168) — asserts count == 2 plus each render's own form (`6 of 7` pre-repair exactly once, `7 checks passed` as the suffix), so it proves the two summaries reflect *their own* pass rather than merely appearing twice.
  - `TestDoctorSummary_NoSingularCarveOut` (:192) — three rows placing `1` at each singular-risk position (N=1 all-pass, T=1, N=1 in the partial form) and additionally asserts the substring `"check passed"` never renders. Not redundant: each row exercises a different slot.
- Notes:
  - Existing assertions were updated, not deleted: `TestDoctorAllStateChecksPassExitsZero` (`cmd/doctor_test.go:165`), `TestDoctorFreshInstallReportedHonestly` (:223), `TestDoctorExecuteStaleEntryReturnsUnhealthy` (:1254-1257), `TestDoctorFixPrunesStaleEntriesThenRediagnosesClean` (:1282-1285), `TestDoctorFixDownServerPrunesProjectsButNotHooks` (:1337-1343), `TestDoctorFixLogSweepNeverDrivesExit` (:1372). Each carries a comment explaining why its count is 6 rather than 7 (the stale-hooks check reports not-evaluable against `TestMain`'s poisoned tmux socket) — the arithmetic is justified in place rather than left as a magic number, which is what stops a future reader from "fixing" it to 7.
  - No over-testing detected: the command-level tests (`AllChecksPassed`, `PartialForm`, `NotEvaluableOutsideCounts`, `FixPathRendersTwo`) verify wiring through the real Cobra body; the unit-level tests verify counting membership. Neither layer duplicates the other's subject.
  - Not-run note: per the verifier contract I assessed these by reading, not by executing the suite.
  - `healthyDoctorDeps` (`cmd/doctor_summary_test.go:14`), introduced by this task, has since become the shared fixture for 14 call sites across `doctor_advisory_test.go`, `doctor_theme_test.go`, `doctor_theme_union_test.go` and `doctor_fix_theme_test.go` — the helper generalised well rather than becoming dead weight.

CODE QUALITY:
- Project conventions: Followed. No `t.Parallel()`; the cmd `*Deps` seam is injected via `runDoctorCmd`/`runDoctorFixCmd` with `t.Cleanup` restore, so no test reaches a real tmux server; tests stay in the unit lane (no daemon spawn, no built binary), which is correct for a pure-rendering change.
- SOLID principles: Good. Counting is separated from formatting is separated from rendering, so each has one reason to change; `doctorUnhealthy` remains the sole authority behind the exit code and the summary is strictly downstream of it.
- Complexity: Low. `doctorCheckCounts` is a single switch over five statuses; `doctorSummaryLine` is one branch plus an optional suffix.
- Modern idioms: Yes. Multi-value `case checkFail, checkUnknown`, an explicit empty arm for the excluded statuses (documents intent and makes the exclusion visible in the switch rather than implicit in a `default`), named results used only as documentation.
- Readability: Good. The absence of a `default` arm is the right call here — a hypothetically-added status would be counted by neither, which is precisely how `doctorUnhealthy` would also treat it, so the equivalence survives by construction rather than by a catch-all guess.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `cmd/doctor.go:458` — the record of the summary's whitespace framing is gone. The spec pins the copy but not the indentation or the blank-line question, and this task deliberately chose two-space indent with no separating blank line and required the choice be documented so a later change has one home; the shipped comment was removed by the repo-wide comment-standard sweep (`a4bc7bd5`) along with its spec citation. Restore a citation-free one-liner above the summary `Fprintf`: `// The copy is fixed; this framing is not — the two-space indent, the absent marker and name columns, and the lack of a preceding blank line are a local choice for continuity with the body lines.` (If the reviewer judges the sweep's removal to have been deliberate for this comment specifically rather than collateral, drop this note — it is the only casualty of the sweep that carried information not recoverable from the code.)
