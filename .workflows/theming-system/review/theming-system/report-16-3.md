TASK: theming-system-16-3 — Share The doctor --fix Down-Server Deferral Fixture Between The Two Tests That Claim It

ACCEPTANCE CRITERIA:
- Neither test declares its own down-server `DoctorDeps` literal or its own hooks/projects seeding.
- The theme subtest's only additions over the shared helper are `ThemesDir` and the advisory-suffix assertions.
- The "why hooks are not pruned on a down server" reason appears once.
- Both test names are unchanged and both still fail if the down-server contract changes.
- `go test ./cmd` passes.

STATUS: complete

SPEC CONTEXT:
The contract under test straddles two things. The pre-existing half is doctor's `--fix` repair split: the stale-hook prune is protected by `runHookStaleCleanup`'s mass-deletion hazard guard (cmd/doctor.go:294, `len(live) == 0` → defer), because a down server's empty `ListAllPaneHookKeys` enumeration is indistinguishable from "every pane is gone" and hooks are non-reconstructable user commands; the stale-project prune is filesystem-only (`ProjectStore.CleanStale()`, cmd/doctor.go:206-217) and has no such ambiguity, so it still runs. The theming half is spec §12.2 / §14A (specification.md:1456, :1852): theme findings are **advisory** and must NOT drive the exit code, surfacing only as `⚠` lines plus a ` · <M> advisory` suffix on the closing summary. The theme subtest's job is therefore to prove the theme feature added the advisory suffix *without* perturbing the pass counts or the repair split — which is exactly why it must assert the same `2 of 6` / `3 of 6` counts as its non-theme twin, and exactly why sharing one fixture between them is the right call.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `/Users/leeovery/Code/portal/cmd/doctor_test.go:896-917` — `downServerDeferFixture(t, stateDir) (deps, hooksPath, projectsPath, goneDir)`, placed directly beside `seedStalePruneFixture` (:850) as instructed, owning the hooks/projects seeding and the full `DoctorDeps` literal including `ServerRunning: func() bool { return false }`.
  - `/Users/leeovery/Code/portal/cmd/doctor_test.go:919-941` — `assertDownServerDeferral(t, hooksBefore, hooksPath, projectsPath, goneDir, err)`, beside `assertStalePrunesApplied` (:866), owning all three assertions (hooks byte-identical, `projects.json` no longer carrying `goneDir`, `ErrDoctorUnhealthy`).
  - `/Users/leeovery/Code/portal/cmd/doctor_test.go:1323-1331` — `TestDoctorFixDownServerPrunesProjectsButNotHooks` re-pointed at both helpers.
  - `/Users/leeovery/Code/portal/cmd/doctor_fix_theme_test.go:412-433` — the "the hazard guard still defers on a down server" subtest re-pointed at both helpers.
  - Commit `c090ddb4`; the doc comments were later trimmed by the plan's comment-hygiene commits `a4bc7bd5` / `915e7fcb` (intentional later revision, not drift).
- Notes:
  - All six `Do` steps landed. Verified against the commit diff that no assertion changed meaning: the `ErrDoctorUnhealthy` check, the `bytes.Equal` hooks comparison and the `strings.Contains(projectsAfter, goneDir)` check are moved verbatim. Where the two copies' failure strings differed the *stronger/clearer* form was taken, as step 6 required — the helper uses the theme copy's `"the filesystem-only stale-project prune did not run on a down server"` over the older `"filesystem-only stale-project prune did not run..."`. No behavioural difference; both callers pass.
  - Step 5 is satisfied: the "why hooks survive a down server" rationale exists in exactly one place (`doctor_test.go:896-899`). Grepped the whole `cmd` package — the only other hit is an unrelated comment in `state_hydrate_replayed_log_test.go:85`. The inline `// A down server yields an empty live-pane enumeration.` that was duplicated in both copies is gone from both.
  - No third copy was missed. The other two `ServerRunning: func() bool { return false }` literals in the package (`doctor_test.go:453`, `:1436`) are genuinely different scenarios — read-only `runDoctorDiagnosis` checks, not the `--fix` deferral contract — and correctly left alone. Folding them in would have conflated a diagnosis assertion with a repair assertion.
  - The `stateDir` parameter is not read by either caller beyond `t.TempDir()`, unlike the sibling `seedStalePruneFixture` (whose theme caller needs `dir` to seed a stale rotated log). Keeping it preserves signature symmetry with the sibling it sits beside and matches the task's prescribed signature — the right call, not flagged.
  - Correctly does NOT call `seedHealthyStateDir`, matching both pre-refactor copies: the point of this fixture is an unhealthy runtime.

TESTS:
- Status: Adequate
- Coverage: This task *is* test-surface work, so the relevant question is whether the shared helpers still bite. They do, and non-vacuously:
  - The hooks assertion cannot pass by accident. The fixture seeds a real entry (`sessA:0.0`) against `fakeHookLister{keys: []string{}}`, so the entry genuinely *is* stale — an unguarded prune would delete it and the `bytes.Equal` check would fail. Had the fixture seeded an empty or already-live hook set, the assertion would have been decorative; it does not.
  - The projects assertion likewise seeds exactly one record whose dir is absent, so skipping the filesystem-only prune fails the `strings.Contains` check.
  - `ErrDoctorUnhealthy` is asserted with `t.Fatalf` (fail-fast before the file re-reads), preserving the original ordering semantics.
  - Both mutation experiments in the task's Tests section would fail *both* callers, because both now route every one of the three claims through the single helper — which is precisely the divergence the task set out to make impossible.
- Notes:
  - Not over-tested. The theme subtest adds only what is genuinely its own: `deps.ThemesDir` (:414-416) and the advisory-suffix expectations (:425-432). Critically it retains the same `2 of 6` / `3 of 6` pass counts as its non-theme twin (`doctor_test.go:1340`, :1343), differing only by the ` · 1 advisory` suffix — so the pair jointly pins spec §12.2's "advisories do not move the exit-code arithmetic". Sharing the fixture is what makes that comparison trustworthy; with two independent `DoctorDeps` literals a seam could drift and the count comparison would silently stop meaning anything.
  - Compilation holds after the extraction: `bytes` is still used in `doctor_fix_theme_test.go` (:210) and `filepath`/`strings` remain used there, so no import became orphaned by the code removal. No unused locals left in either caller.
  - Both test names are byte-identical to their pre-refactor forms, per AC and step 6.

CODE QUALITY:
- Project conventions: Followed. Helpers are `*testing.T`-first with `t.Helper()`, kept in `cmd/doctor_test.go` beside their siblings rather than in a new file (step 7), named in the same `seed*`/`assert*` register as `seedStalePruneFixture` / `assertStalePrunesApplied`, and use the project's `t.Errorf` vs `t.Fatalf` discipline (fatal for setup/IO and for the guard condition that makes later checks meaningless; error for independent claims, so one failure does not mask the others). No `t.Parallel()`, per the package rule. No real tmux or filesystem reach outside `t.TempDir()`.
- SOLID principles: Good. Clean separation between arrange (`downServerDeferFixture`) and assert (`assertDownServerDeferral`), mirroring the sibling pair — a caller can reuse the fixture with different assertions, or the assertions after a differently-shaped run, without either helper knowing about the other.
- Complexity: Low. Both helpers are straight-line; no branching, no options struct, no test-table indirection.
- Modern idioms: Yes. Named return values are used purely as documentation for a 4-value return (a legitimate Go use), with an explicit `return` listing every value rather than a naked return — matching the sibling helper.
- Readability: Good. The one point worth calling out: `assertDownServerDeferral` shadows the outer `err` parameter with `readErr` for its own IO, which is correct and necessary — reusing `err` would clobber the run error the helper was handed. Easy to get wrong; it was got right.
- Comment accuracy: The fixture's doc comment holds against the code. "wires a down server" matches `ServerRunning: func() bool { return false }`; "empty live-pane enumeration" matches `fakeHookLister{keys: []string{}}`; "the mass-deletion hazard guard defers the stale-hook prune" matches `cmd/doctor.go:294`; "the filesystem-only project prune has no such ambiguity and still runs" matches `pruneDoctorStaleProjects` (cmd/doctor.go:206-217), which touches no tmux seam. It explains *why*, not *what*, and carries no task ids or spec-section references.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] /Users/leeovery/Code/portal/cmd/doctor_test.go:900 — The one piece of the down-server scenario still duplicated between the two callers is the pre-state snapshot: both `TestDoctorFixDownServerPrunesProjectsButNotHooks` (doctor_test.go:1325-1328) and the theme subtest (doctor_fix_theme_test.go:417-420) open with the identical five-line `hooksBefore, err := os.ReadFile(hooksPath)` + `t.Fatalf("read hooks.json: %v", err)`. Have `downServerDeferFixture` return `hooksBefore []byte` as a fifth value (it seeds the file and nothing mutates it before the run, so the fixture is the natural owner of the snapshot) and delete the read from both callers; `assertDownServerDeferral` keeps its `hooksBefore` parameter unchanged. Lower stakes than the deps literal this task removed — a snapshot cannot state a contradictory contract the way a seam can — which is why it is a note rather than a finding.
- [do-now] /Users/leeovery/Code/portal/cmd/doctor_test.go:897 — The comment reads `... indistinguishable from "every pane is gone" - so the mass-deletion hazard guard ...`, using a spaced hyphen. It is the only spaced-hyphen dash in the file's comments; the convention here (including the guard's own comment at cmd/doctor.go:196) is an em dash. Replace ` - ` with ` — ` so the line reads `// enumeration is indistinguishable from "every pane is gone" — so the`.
