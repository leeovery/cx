TASK: theming-system-10-9 — Amend the Log-Component Vocabulary With the Theme Component (tick-275a80)

ACCEPTANCE CRITERIA:
1. `manifest get portal-observability-layer status` run and returned `completed` before any edit.
2. `theme` appears in both the fenced component block and the "Component | Owns" table, with its three emitting sites named.
3. The stated component count is true of the listed membership and agrees with CLAUDE.md's 18 and `internal/log`'s constants.
4. `spawn` and `resolve` are present, with the corrigendum recording that their amendments never landed.
5. The seven theme attr keys are reflected without duplicating `reason` or `path`; stated key count matches the deduplicated set.
6. The full seven-event catalogue is recorded with each event's level and cadence, including every dedup key.
7. Multi-package emission is stated as legal under bind-once-per-package; the binding rule no longer reads as one package per component.
8. The spec records: rejections are WARN; the component never records diagnosis (doctor/export emit nothing); `token`'s only consumer is `theme: rejected`; dedup state lives on the injected logger.
9. `prefs` and `terminals` remain documented as deliberately outside the vocabulary.
10. A corrigendum block is created with one entry per correction, dated the day of the edit.
11. Re-index ran against this artifact; the commit is scoped to `portal-observability-layer`, separate from tasks 10-7 and 10-8.
12. The `spawn`/`resolve` attr-key gap was surfaced explicitly and resolved by decision, not by silence.

STATUS: complete

SPEC CONTEXT:
theming-system §12.3 ("A new `theme` log component") is the governing text: it declares the component, its seven-row event catalogue with levels and cadences, the attr keys `slug`/`slot`/`reason`/`path`/`token`/`count`/`rejected`, the injected-logger emission-control mechanism, the per-process dedup contract, the WARN-not-INFO rule for rejections, and the used-not-diagnosed rule (doctor, `theme export` and `capturetool` emit nothing). §8.9 states the multi-package emission is legal under bind-once-per-package. §15.1 lists the log-component vocabulary as a spec amendment this feature owes. The `portal-observability-layer` specification owns the closed taxonomy and its own extension policy requires amendment of THAT file, which is what this task discharges via the completed-unit correction protocol (`.claude/skills/workflow-shared/references/correcting-historical-artifacts.md`).

IMPLEMENTATION:
- Status: Implemented — and unusually precise. Every amended claim was verified true of the as-built code, not merely transcribed from the plan text.
- Location: `.workflows/portal-observability-layer/specification/portal-observability-layer/specification.md` (commit `b01a623e`, "specification(portal-observability-layer): corrigendum from theming-system", 2026-08-07).
  - Corrigendum block: lines 3-7 (five entries, directly beneath the title, protocol format exact, dated the commit day).
  - Fenced component block + count: line 160 ("Closed component value space (18 total)"), lines 162-166.
  - Owns-table rows added: `spawn` (187), `resolve` (188), `theme` (189).
  - `prefs`/`terminals` exclusion + the "what earns `theme` its place" argument: line 193.
  - Attr-key space: line 195 ("64 keys"); Spawn group 224-236, Resolve 239, Theme 241-251.
  - Bind-once restatement: lines 121-127.
  - New section "The `theme` component event catalog": lines 973-1015; roadmap entry added at line 40 with the Hook-firing section renumbered to 15.
- Verification performed (each independently, against code rather than against either document):
  - **Count 18 is true of the list**: the fenced block names exactly 18. Production `log.For` bindings enumerate 17 distinct components plus `log-rotate` (`internal/log/rotate.go:10`) = 18. Matches CLAUDE.md's 18. `internal/log` holds only two component *constants* (`processComponent`, `bootstrapComponent`), so bindings are the correct oracle and the corrigendum says exactly that ("verified against `internal/log`'s bindings") rather than over-claiming a constants table.
  - **Attr count 64 is arithmetic-true**: 14 Contextual + 14 Cycle-summary + 7 Lifecycle + 3 Hydrate + 7 Process + 8 Spawn + 2 Resolve + 5 Theme + 4 Baseline = 64. Each group's enumeration was counted row-by-row and matches its stated number. No double-count: `reason` appears once (Lifecycle) and is cross-listed to Process and Theme; `path` once (Contextual), cross-listed to Theme; `session` once (Contextual), cross-listed to Spawn; `target` once (Process), cross-listed to Resolve. The prior 49 reconciles as 49 + 8 + 2 + 5 = 64.
  - **Corrigendum quotes are verbatim**: `git show b01a623e` confirms the removed lines read "(15 components)", "Closed component value space (15 total)" and "Closed attr-key value space (49 keys)" exactly as quoted.
  - **Three emitting sites are real**: injected loader seam at `cmd/open.go:486` (`theme.NewLoader(theme.NewEventLogger(themeLogger))`), `cmd/config.go:170` (`appearance migrated`), `cmd/theme_persister.go:27,48` (`commit failed`). `internal/theme` contains no `log.For` call, so "binds nothing" holds; the single `log.For("theme")` is `cmd/open.go:27`.
  - **The framing is more accurate than the task text**: the plan's edge-case wording says "emitted from three packages"; the spec correctly says "three sites across two packages" (`cmd/config.go` and the persister are both package `cmd`), which is what §8.9's "more than one package" actually supports. The executor corrected a loose input rather than transcribing it.
  - **Catalogue matches code, event by event**: levels/attrs at `internal/theme/events.go` (`Loaded` INFO, `Enumerated` INFO, `FallbackApplied`/`Rejected`/`DirectoryUnusable` WARN); dedup key is `(event, identity, reason)` at `events.go:41-45` with the slug-else-path identity swap at 96-99, matching the spec's stated dedup keys exactly; dedup state on the instance (`events.go:32-37`), not package state; check-and-record is one critical section (`firstSighting`, 128-137) as claimed.
  - **Two load-bearing orderings verified**: `internal/theme/resolution.go:193-196` calls `reportFallback` before `events.Loaded`, and `Loaded` carries `r.Resolved` (the fallback's slug on a fallback) — both exactly as the spec's closing paragraph asserts.
  - **The size-floor claim is verified as-built**: `internal/tui/theme_panel.go:128-129` calls `source.Open(...)` (which emits `enumerated` at `internal/theme/union.go:121`) *before* the floor check that can refuse the open. This detail is not in §12.3; it was verified against code and added correctly.
  - **Used-not-diagnosed holds**: `theme.NewSilentLoader()` (`internal/theme/load.go:43-45`, `NewEventLogger(log.Discard())`) is used by `cmd/theme.go:54` (export), `cmd/doctor_theme.go:52` and `cmd/capturetool/main.go:87`. `NewLoader` panics on a nil seam, so silence is never accidental.
  - **Migration path is genuinely single-sited**: `runTranslationPersist` (`cmd/config.go`) returns silently on `err != nil || !persisted`, so `commit failed` has exactly one site and `appearance migrated` fires only on a persist that wrote a theme key — both as the catalogue states.
- Notes: Owning unit status is `completed` (`.workflows/portal-observability-layer/manifest.json`), so the correction protocol's `completed` branch is the right one. Roadmap renumbering is clean — cross-references in the file address sections by italic name, not number, so nothing broke.

TESTS:
- Status: Adequate. This is a documentation-artifact correction in `.workflows/`, not source; no Go code changed and no new test is owed. No guard test anywhere in the repo pins the observability spec's component list, so nothing existed to update or to break.
- Coverage: The task's eight "Tests" are verification steps rather than automated tests. All eight were re-performed independently by reading:
  1. Owning unit `completed` — confirmed in the unit manifest.
  2. Component declared in both the fenced block and the owns table — confirmed (lines 165, 189).
  3. Count true of the list — confirmed by counting names (18) and by enumerating production bindings (18).
  4. No attr key double-counted — confirmed; `reason` and `path` each appear once, groups sum to 64.
  5. Catalogue complete — all seven events present with level, cadence and dedup key.
  6. Corrigendum records the corrections — five entries, a superset of the two the Do-list named.
  7. Re-index replaced the chunks — `.workflows/.knowledge/store.msp` carries "Closed component value space (18 total)" and the new fence "process  spawn  resolve  theme"; the old fence form is absent. The two residual "(15 total)" strings in the store are the corrigendum's own quotation and the historical planning artifact `review-traceability-tracking-c1.md`, both expected.
  8. Commit scoped and separate — `b01a623e` touches only the observability spec + the two knowledge-store files; siblings `3ffa63ab` (spectrum-tui-design, 10-7) and `f22176e4` (cli-verb-surface-redesign, 10-8) are distinct commits with the same shape.
- Notes: No over- or under-testing. Adding a Go guard that pins a `.workflows` spec's prose count against `internal/log` bindings would be a new coupling this task neither needs nor was asked for.

CODE QUALITY:
- Project conventions: Followed. The correction protocol was executed to the letter — edit in place (no wrong content left in the body for posterity), corrigendum beneath the title in the exact `> **Corrigendum {YYYY-MM-DD}** (from `{unit}`): {quote} — corrected: {truth}.` shape, re-index, scoped commit with the protocol's prescribed message, and the owning manifest untouched (still `completed`, no reopen).
- SOLID principles: N/A (documentation).
- Complexity: Low.
- Modern idioms: N/A.
- Readability: Good. The new catalog section mirrors the existing saver/daemon catalog's structure (Decision → Mechanical rule → Calling code locations), so it reads as native to the file rather than bolted on. The "Two rules are part of the amendment rather than call-site detail" framing correctly separates policy from implementation detail.
- Comment accuracy: N/A for source; the amendment's every factual claim was checked against code above and none is falsified.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [bug] `.workflows/portal-observability-layer/specification/portal-observability-layer/specification.md:173` — the `bootstrap` owns-row reads "The 11-step bootstrap orchestrator"; as-built the orchestrator is ten steps (CLAUDE.md:126; former step 11 `CleanStale` was re-homed onto the `_portal-saver` daemon). Line 869 repeats "the 11-step bootstrap orchestrator". Change both to "ten-step". This stale number sits in the very table this amendment edited and, unlike the `clean` row, was not surfaced anywhere — so a file that now carries a corrigendum block asserting its counts are reconciled still serves one wrong count to the knowledge base.
- [idea] `.workflows/portal-observability-layer/specification/portal-observability-layer/specification.md:183` — the `clean` owns-row reads "`portal clean` command path", naming a command `cli-verb-surface-redesign` retired (the component itself is still live, bound at `cmd/bootstrap/bootstrap.go:16` and `internal/state/fifo_sweep.go:13`, so the count of 18 is unaffected). The task deliberately scoped this out ("surface it, do not sweep it"), so leaving it is compliance, not drift. Decide whether a follow-up correction cycle re-invokes the protocol on this file to fix this row together with the `11-step` note above, rather than paying the gate twice.
