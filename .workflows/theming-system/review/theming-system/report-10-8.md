TASK: theming-system 10-8 — Amend Portal Doctor's Contract — Two Line Classes and the Closing Summary (tick-e7d649)

ACCEPTANCE CRITERIA:
1. `manifest get cli-verb-surface-redesign status` run and `completed` before any edit.
2. The "0 iff every check passes" sentence corrected in place; old wording does not survive in the body.
3. Both classes named with their markers; `⚠` class stated as glyph-backed and non-exit-code-driving.
4. The reason recorded — no repair path, permanent red, exit code signals the resurrection machinery.
5. Closing summary documented in both forms with ` · <M> advisory|advisories`, M=0 suppression, described as a **new** line on every run.
6. `<N>`/`<T>` count Portal-health checks only; `<M>` counts lines.
7. Host-terminal informational line untouched, beside the new class rather than merged.
8. `--fix` documented as carrying advisories in both passes, exit driven solely by post-repair health checks.
9. The class written to admit later members rather than defined as "theme".
10. Corrigendum block created beneath the title, one entry per correction, dated the day of the edit.
11. Re-index ran against this artifact; commit scoped to `cli-verb-surface-redesign`, separate from 10-7 and 10-9.

STATUS: complete

SPEC CONTEXT:
theming-system spec §12.2 (lines 1450–1473) rules theme lines advisory and non-exit-code-driving, names the two-class table (`Portal-health checks` / `User-content diagnostics` with `⚠`, MV §2.2 glyph-backed), the one-slug-one-line rule, the trailing-block placement, and the `--fix` both-passes clause. §14A (lines 1840–1854) gives the four doctor line frames and the closing-summary forms (`<N> checks passed`, `<N> of <T> checks passed`, ` · <M> advisory|advisories`, M=0 suppressed), plus "`<N>`/`<T>` count Portal-health checks only" and "the summary line is **new**". §15.1 (line 1879) names doctor's contract as one of the three amendments this feature carries. Task's premise confirmed: doctor's contract lives in `.workflows/cli-verb-surface-redesign/…/specification.md`, not the MV spec.

IMPLEMENTATION:
- Status: Implemented (documentation amendment; correction protocol executed correctly)
- Location:
  - `.workflows/cli-verb-surface-redesign/specification/cli-verb-surface-redesign/specification.md:3-11` — five corrigendum entries directly beneath the title, protocol form verbatim (`> **Corrigendum 2026-08-07** (from \`theming-system\`): "{quote}" — corrected: {truth}.`), dated the commit day (f22176e4, 2026-08-07).
  - `:309` — catalog lead-in rewritten to "The authoritative catalog of **Portal-health checks** — the class the exit code is drawn from, its informational host-terminal line excepted".
  - `:320` — new paragraph pointing the catalog at the second class and the closing summary.
  - `:327-333` — the two-class table (`Portal-health checks` / `User-content diagnostics`, `⚠` "glyph-backed, so it survives a colourless terminal", Yes/No exit-code column).
  - `:335` — corrected exit sentence, "0 iff every Portal-health check passes… a report may carry any number of `⚠` lines and still exit 0".
  - `:336` — "Why the second class is exempt" (no repair path → permanently non-zero; exit code signals the resurrection machinery).
  - `:337` — "The class is open-ended" (theme diagnostics the **first** member, not the definition) plus the explicit "distinct from the informational host-terminal line… are not merged".
  - `:338` — trailing-block placement and the no-interleave argument.
  - `:340` — `--fix` clause (both renders, freshly re-collected, exit solely post-repair, no repair step gained).
  - `:342-344` — the closing summary, its two forms, the M=0 suppression, "a line the report **gains**", and the `<N>`/`<T>`/`<M>` counting rules.
  - `:443` — Command Surface Summary row for `portal theme export <slug>` (sweep correction, corrigendum entry 4).
  - `:285` — `uninstall`'s untouched-config list gains `themes/` (sweep correction, corrigendum entry 5).
  - Commit `f22176e4` "specification(cli-verb-surface-redesign): corrigendum from theming-system" — touches only that spec plus `.workflows/.knowledge/{store.msp,metadata.json}` (`last_indexed` advanced), i.e. the re-index rode along as the protocol describes.
- Notes:
  - Criterion 1: `.workflows/cli-verb-surface-redesign/manifest.json:4` is `"status": "completed"` and the unit's manifest is untouched by the commit, so the protocol's `completed` branch was the correct one and was followed. The literal "the command was run first" is not recoverable from artifacts; the substantive condition holds.
  - Criterion 2: `grep "0 iff every check passes"` matches only line 3 (inside the corrigendum quote). Old wording does not survive in the body.
  - Criterion 7: the host-terminal bullet (`:341`) appears as unchanged context in the commit diff — no hunk. The new class sits beside it and `:337` explicitly refuses the merge.
  - Criterion 11: separate commits confirmed — `3ffa63ab` (spectrum-tui-design corrigendum) and `5bcf3699` (10-7) precede it, `b01a623e` (portal-observability-layer corrigendum) and `cc1ec21b` (10-9) follow it. Nothing folded.
  - **Spec↔code fidelity checked, all clean.** Every claim the amendment adds is true of the shipped `cmd/doctor.go`: two-class rendering and trailing block (`renderDoctorReport`, :448-459 — catalog, then advisories, then summary); summary forms (`doctorSummaryLine`, :504-514 — `%d checks passed` when passed==total else `%d of %d`); ` · <M> advisory|advisories` with M=0 suppressed (:510-512); `<M>` = `len(advisories)`, i.e. lines after the one-slug-one-line union; `--fix` re-collects advisories for the second render (`collectThemeAdvisories(deps)` called twice, :155 and :176) with the exit driven solely by `doctorUnhealthy(postResults)` (:177); host-terminal returns `checkInfo` (:266-276) and `doctorCheckCounts` (:487-500) excludes `checkInfo` and `checkNotEvaluable` from **both** `<N>` and `<T>`.
  - The one place the amendment goes beyond its source (`:343`, "any check that could not be evaluated count toward neither") is **not** drift: theming §14A is silent on not-evaluable checks rather than in conflict, and the sentence documents the shipped `doctorCheckCounts` behaviour exactly, with its illustrative "6 of 7 checks passed beside exit 0" being precisely the state the code avoids.
  - The two sweep corrections beyond the task's named scope are accurate and correctly co-located in the same owning-unit commit: `cmd/theme.go:11-16,78-79` shows `theme` with `export` as its only subcommand and no `Hidden`, `cmd/root.go:31` shows `"theme": true` in `skipTmuxCheck`, and `cmd/uninstall.go` performs no filesystem writes while `themesDirPath()` resolves under the same config base.
  - A prior implementation-review round caught and fixed a self-inflicted falsification in this very edit (`.workflows/theming-system/implementation/theming-system/fix-tracking-theming-system-10-8.md:5`) — the catalog lead-in briefly claimed the whole list drives the exit code while its last member is informational. The remedy is present in the file at `:309` and `:332`. No residue.

TESTS:
- Status: Adequate (verification-based; no Go code changed by this task)
- Coverage: The task's "tests" are artifact assertions and all hold — old claim confined to the corrigendum quote; `grep "advisor"` returns the class, the summary suffix and the `--fix` clause; the host-terminal bullet has no diff hunk; the corrigendum block is beneath the title and dated the edit day; the commit is scoped and separate; the re-index rode the same commit (`metadata.json` `last_indexed` advanced, `store.msp` rewritten).
- Notes: The *behaviour* the amendment documents is separately pinned by phase-7 tests (`TestDoctorSummary_InfoLineOutsideCounts`, `_NotEvaluableOutsideCounts`, `_NoSingularCarveOut`, `_IsTheLastLine`, `TestDoctorFix_AdvisoriesInBothPasses`), which is the right split — the spec edit is not the place to add coverage, and none was expected. Not over-tested: no redundant guard was added for a prose file.

CODE QUALITY:
- Project conventions: Followed — the four-step correction protocol (`.claude/skills/workflow-shared/references/correcting-historical-artifacts.md`) is executed as written, corrigendum form byte-for-byte, no manifest touch, edit-in-place with no "was X, now Y" residue in any section body.
- SOLID principles: N/A (documentation artifact)
- Complexity: Low — the contract now reads as a table plus six bullets in a single section; the exit-code question is answerable from `:330-335` alone.
- Modern idioms: N/A
- Readability: Good — the reason bullet, the open-endedness bullet and the counting sub-bullets each state one idea, and the summary's two forms are given literally rather than described.
- Issues: One phrasing residue on the `--fix` bullet (see non-blocking).

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `.workflows/cli-verb-surface-redesign/specification/cli-verb-surface-redesign/specification.md:340` — the bullet retains the pre-amendment absolutes "exits **0 iff everything is healthy post-repair, non-zero if anything remains unhealthy or unfixable**" and scopes them only by the trailing "— driven **solely** by the post-repair Portal-health checks". Corrigendum entry 2 (`:5`) quotes that exact phrase as the claim being corrected, so the wording it refutes still stands in the body. Replace with "exits **0 iff every Portal-health check passes post-repair, non-zero if any Portal-health check remains unhealthy or unfixable**" and drop the now-redundant "— driven **solely** by the post-repair Portal-health checks" clause opener to "The user-content scan is read-only, so it runs on the `--fix` path too: …".
