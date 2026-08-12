TASK: theming-system-12-8 — Strip Workflow Vocabulary From The Topic's Test Comments (tick-1bea5e, severity low, source: standards)

ACCEPTANCE CRITERIA:
- No `§`, `Phase N` or `task N-M` reference remains in the topic's test files.
- Every replacement names a rule or behaviour that is checkable against the code, not a document.
- No test name, assertion or non-comment line changed.

STATUS: complete

SPEC CONTEXT:
This is a standards-sourced task, not a spec-behaviour task. The governing rule is
`.claude/skills/workflow-implementation-process/references/code-quality.md:50` — "Never in a comment:
links, URLs, issue ids, or any workflow vocabulary — task ids, phase numbers, spec-section citations.
The comment must hold true for a reader with no knowledge of the process that produced the code, long
after its artifacts are archived." Cycle 1 swept the topic's production files; this task extends the
sweep to the topic's test files (the spec these citations pointed at is archived at sign-off, so `§9.5`
would name nothing a future reader can open). No specification section is implicated.

IMPLEMENTATION:
- Status: Implemented
- Location: commit `0a471f95` ("Ttheming-system-12-8 — strip workflow vocabulary from the topic's test
  comments") — 71 files, all `*_test.go` under `internal/theme/`, `internal/tui/theme_*`,
  `internal/capture/theme_*`, `cmd/*theme*`, `cmd/capturetool/main_test.go`, plus `.tick/tasks.jsonl`
  and the workflow manifest. No production file touched.
- Verification against the CURRENT tree (phases 13–17 rewrote or deleted many of these comment blocks;
  outcome judged as amended):
  - `grep -rn "§" --include="*.go" .` → 0 hits repo-wide.
  - `grep -rniE "\bphase [0-9]" --include="*_test.go" .` → 1 hit, `internal/spawn/detect_test.go:162`,
    a pre-existing unrelated (spawn) file explicitly out of scope per Do step 5.
  - `grep -rniE "task [0-9]" --include="*.go" .` → 1 hit, in a topic file (see NON-BLOCKING NOTES).
  - Scope was respected: the commit swept only theme-topic test files; the pre-existing
    `cmd/bootstrap/phase*_test.go`, `internal/restore/integration*_test.go` and
    `internal/spawn/detect_test.go` occurrences were correctly left alone.
- Substitution quality: the sweep used a consistent one-to-one mapping from section numbers to named
  rules — `§9.4 → "the union rule"`, `§9.5 → "the row-rendering rule"`, `§9.8 → "the geometry rule"`,
  `§6.2 → "the reason vocabulary"`, `§5.4 → "the reserved-slug rule"`, `§13.3 → "the harness contract"`,
  `§14A → "the pinned copy"`, `§9.2 → "the picker idiom"`, etc. Each names a behaviour a reader can
  check against the code rather than a document, satisfying AC 2. The weakest labels were the two
  highest-frequency generics ("the picker idiom" ×97, "the pinned copy" ×92) and one that itself read
  as workflow vocabulary ("the guard-test reshape's named set", replacing §13.6) — all of these have
  since been rewritten or removed by the later comment-audit and phase 13–17 commits; only three
  "pinned copy" occurrences survive today, all in `t.Errorf` message text where they read correctly.
  Nothing actionable remains from this.
- The three task-id comment references called out in the problem statement ("Enumeration (task 1.7)",
  "from task 1.8", "from task 1.3") were deleted outright, and the surrounding sentences were left
  standing and coherent (e.g. `internal/theme/enumerate_test.go` "no entry, no reason, and no log line").
- Notes: no mechanical-replacement artifacts ("the the", "rule's rule", stray possessives) and no
  comment line over 100 columns anywhere in the topic's test files after the sweep.

TESTS:
- Status: Adequate (comment-only change; the existing suite is the test)
- Coverage: The change is comment-only, so the correct verification is (a) the suite still builds and
  passes and (b) a grep returns nothing. The diff, filtered to non-comment lines, contains exactly
  three changed lines — `internal/theme/contrast_test.go`'s `floorNormal = 4.5`, `floorLargeUI = 3.0`
  and `floorFillPerceptible = 1.10`, where only the TRAILING comment changed (`// §13.5 normal text` →
  `// normal text`); the code text and all three values are byte-identical. No test name, no assertion,
  no condition, no import and no build tag changed, so AC 3 holds. No code line was commented out, so
  the change cannot have broken the build — corroborated by the ~60 subsequent commits on top of it.
- Notes: judged by reading, per the review protocol; the suite was not executed.

CODE QUALITY:
- Project conventions: Followed. The change is confined to `*_test.go` files in the topic, respects the
  no-unrelated-sweep constraint, and leaves the lane split (`//go:build integration`) untouched.
- SOLID principles: N/A (comment-only).
- Complexity: Low.
- Modern idioms: N/A.
- Readability: Good. Replacements name a rule rather than pointing at an archived section, which is
  exactly the standard's stated intent; the surrounding prose was re-wrapped rather than left ragged.
- Issues: One residual workflow reference survives outside a comment — see below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/capture/theme_panel_fixture_render_test.go:256 — the `t.Errorf` message still reads
  "the pinned render size is below task 8-11's floor". It predates this task (introduced by task 8-15 in
  `5baac223`), so it was in scope for AC 1's literal wording ("no `task N-M` reference remains in the
  topic's test files") but out of scope for Do step 6 ("change no non-comment line") — the two clauses
  conflict, and the sweep resolved the conflict in favour of step 6. The standard's rationale applies
  the same way to a failure message a future reader will actually see, so replace the citation with the
  rule it names: `t.Errorf("the A-frame carries the blocked-entry flash %q; the pinned render size is
  below the panel's entry-gate floor", refusal)` — "entry gate" is already the wording used two
  assertions above at line 249, and `panelEntryRefusalCopy` at line 254 makes it checkable against code.
