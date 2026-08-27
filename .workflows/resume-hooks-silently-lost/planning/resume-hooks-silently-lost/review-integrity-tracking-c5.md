# Review Tracking: Resume Hooks Silently Lost - Integrity

## Findings

### 1. The test that proves `doctor --fix` repaired something is left asserting a repair that no longer happens

**Severity**: Important
**Plan Reference**: Phase 1, task `resume-hooks-silently-lost-1-1`
**Category**: Acceptance Criteria Quality / Task Self-Containment
**Move**: settled
**Change Type**: add-to-task

**Problem**:
`cmd/doctor_summary_test.go` seeds `hooks.json` with `sessA:0.0` against a live set of `sessB:0.0` and asserts `portal doctor --fix` renders `6 of 7 checks passed` before the repair and `7 checks passed` after it — the only test in the suite that proves the two reports differ *because a repair landed between them*. Once the shape rule retains non-token-shaped keys, `sessA:0.0` stops being stale, the pre-repair check passes, and that assertion fails with the message `pre-repair summary missing or repeated`. The task's fixture inventory names every other seed of this shape — `seedStalePruneFixture`, `TestDoctorStaleHooksCheck`, `cmd/doctor_fix_theme_test.go:104-106` — but not this one, so the executor meets it as an unexplained failure rather than a known re-point. The obvious way out is to relax the expected counts to `7 checks passed` twice, which leaves the test green and no longer testing that `--fix` repairs anything. The same file's `healthyDoctorDeps` also constructs `fakeHookLister{keys: []string{"sessA:0.0"}}`, so the file is a compile blocker again when task 2-1 changes that fake's field to rows.

**Proposal**:
Add `cmd/doctor_summary_test.go` to the task's re-point inventory with the seed named and the wrong fix named alongside it, and add an acceptance criterion pinning the two summary forms so the coverage cannot be dropped by relaxing the counts. This is the same treatment the task already gives `TestCleanStaleRemovesExactlyStaleKeys`, whose failure mode ("passes vacuously") is identical in shape. The re-point rule is unchanged: the seed asserts removal, so it takes a token-shaped literal absent from that fixture's live set.

**Current**:

Do — final bullet, sites list (fragment):

```
`cmd/doctor_fix_theme_test.go:104-106`, `cmd/state_daemon_hook_cleanup_test.go` (`stale:0.0` / `live:0.0` / `a:0.0` / `b:0.0`)
```

Acceptance Criteria — anchor criterion:

```
- [ ] `TestCleanStaleRemovesExactlyStaleKeys` still compares a non-empty removal set against a non-empty prediction — an equality between two empty slices does not satisfy it
```

**Proposed Text**:

Do — final bullet, sites list (fragment):

```
`cmd/doctor_fix_theme_test.go:104-106`, `cmd/doctor_summary_test.go` — both `healthyDoctorDeps`'s live key and `TestDoctorSummary_FixPathRendersTwo`'s `sessA:0.0` seed against a `sessB:0.0` live set, the latter being the only test that proves `--fix` renders two reports *with a repair between them*: it asserts `6 of 7 checks passed` pre-repair, which holds only while the seeded key is judged stale, so the seed is re-pointed at a token-shaped value and the expected counts are left exactly as they are —, `cmd/state_daemon_hook_cleanup_test.go` (`stale:0.0` / `live:0.0` / `a:0.0` / `b:0.0`)
```

Acceptance Criteria — anchor criterion, with the new criterion appended after it:

```
- [ ] `TestCleanStaleRemovesExactlyStaleKeys` still compares a non-empty removal set against a non-empty prediction — an equality between two empty slices does not satisfy it
- [ ] `TestDoctorSummary_FixPathRendersTwo` still asserts `6 of 7 checks passed` before the repair and `7 checks passed` after it — its seeded key is re-pointed at a token-shaped value, never its expected counts relaxed, or the one test proving `doctor --fix` repairs anything stops proving it
```

**Resolution**: Fixed
**Notes**:

---

### 2. Deleting `tmux.HookKey` leaves a test file behind that no longer compiles

**Severity**: Minor
**Plan Reference**: Phase 3, task `resume-hooks-silently-lost-3-4`
**Category**: Task Self-Containment
**Move**: settled
**Change Type**: update-task

**Problem**:
The task directs "delete `TestHookKey` from `internal/tmux/hookkey_test.go`, removing the file if nothing else remains in it". Something does remain: `TestHookKey_DistinctSuffixesUnderOneID` sits beside it and also calls `tmux.HookKey`, so an executor following the instruction literally keeps the file and breaks the build. The `portalIDLiteral = "@portal-id"` const in the same file is the third resident — after Phase 2 re-points its six consumers nothing reads it, and the task's own zero-occurrences grep forbids it in test sources, but no step names its removal either. The condition the instruction offers ("if nothing else remains") reads as a judgement call when the answer is fixed: after task 2-2 removes `TestHookKeyFormatContainsPortalIDLiteral`, everything left in the file dies with `HookKey`.

**Proposal**:
Say the file goes whole and name what is in it, so the deletion is a fact rather than a condition the executor evaluates. Task 2-2 already flags that `portalIDLiteral` "goes with that re-point"; naming it here closes the loop for whichever of the two lands it.

**Current**:

Do — third bullet:

```
- `internal/tmux/tmux.go`: delete `HookKey` (`:398`) and its doc comment — its only production caller was replaced in 3-1 — and delete `TestHookKey` from `internal/tmux/hookkey_test.go`, removing the file if nothing else remains in it.
```

Edge Cases — anchor bullet:

```
- The `state` row in CLAUDE.md gains what 3-1 and 3-2 built rather than being merely deleted — otherwise the architecture description loses the durable-identity mechanism entirely
```

**Proposed Text**:

Do — third bullet:

```
- `internal/tmux/tmux.go`: delete `HookKey` (`:398`) and its doc comment — its only production caller was replaced in 3-1 — and delete `internal/tmux/hookkey_test.go` **whole**. Nothing in it survives `HookKey`: `TestHookKey` and `TestHookKey_DistinctSuffixesUnderOneID` both call it, and the `portalIDLiteral = "@portal-id"` const beside them lost its last reader when task 2-2 removed `TestHookKeyFormatContainsPortalIDLiteral` and Phase 2 re-pointed the real-tmux files that shared it.
```

Edge Cases — anchor bullet, with the new bullet appended after it:

```
- The `state` row in CLAUDE.md gains what 3-1 and 3-2 built rather than being merely deleted — otherwise the architecture description loses the durable-identity mechanism entirely
- `internal/tmux/hookkey_test.go` holds two `HookKey` tests, not one, plus the `portalIDLiteral` const. Deleting only `TestHookKey` leaves a file that does not compile and a `@portal-id` literal the zero-occurrences grep rejects
```

**Resolution**: Fixed
**Notes**:

---

### 3. The architecture doc still calls `hook` config-file only after it starts writing the state directory

**Severity**: Minor
**Plan Reference**: Phase 2, task `resume-hooks-silently-lost-2-4`
**Category**: Task Self-Containment
**Move**: settled
**Change Type**: add-to-task

**Problem**:
CLAUDE.md's Resume-hook-command bullet ends `Bootstrap-exempt (config-file only)`. This task makes that false: `hook set` resolves the state directory with `state.EnsureDir()` — creating it and its `scrollback` subdirectory on a machine where Portal has never run — and writes `save.requested` inside it. The task's own Edge Cases already say so ("This is the only path `hook` takes outside the config directory holding `hooks.json`"), but no task in the plan corrects the sentence. Four other tasks own CLAUDE.md passages (2-2 the key scheme, 3-4 the `@portal-id` set, 4-2 the removal rule, 5-1 the lock sidecar) and this bullet falls between all of them, so the document is left telling the next reader that a command which now creates a directory and a file outside the config tree touches only a config file — and, since the parenthetical is the only thing on the page justifying the exemption, obscuring what the exemption actually rests on (that `hook` starts no tmux server).

**Proposal**:
Give the correction to this task, since this is the change that makes the claim false, and keep it to the one parenthetical: the exemption itself is unchanged and the rest of the bullet stays. The wording narrows the claim to what remains true — no tmux server is started — and names the state-directory touch as the single write outside the config directory, which is what stops a later reader treating a `save.requested` written by `hook set` as a stray.

**Current**:

Do — final two bullets:

```
- Do not touch `hooksRmCmd` — removal does not touch the dirty flag.
- Cover the happy path and both failure modes in `cmd/hooks_test.go`.
```

Acceptance Criteria — final two criteria:

```
- [ ] `hook rm` touches nothing on either of its paths
- [ ] `hook` starts no tmux server on this path — bootstrap-exemption is unchanged
```

**Proposed Text**:

Do — final two bullets, with the CLAUDE.md bullet inserted between them:

```
- Do not touch `hooksRmCmd` — removal does not touch the dirty flag.
- Correct CLAUDE.md's **Resume-hook command** bullet (the `cmd/hooks.go` entry), whose `Bootstrap-exempt (config-file only)` parenthetical stops being true here: rewrite it to say the exemption holds because `hook` starts no tmux server, and to name the `save.requested` touch in the state directory as its one write outside the config directory. Leave the rest of the bullet — the verb, the permanent `hooks` alias, the pointer to "Resume hooks" — exactly as it is, and touch no other CLAUDE.md passage: the key-scheme passages belong to task 2-2 and the `hooks` row's lock sidecar to Phase 5.
- Cover the happy path and both failure modes in `cmd/hooks_test.go`.
```

Acceptance Criteria — final two criteria, with the new criterion appended after them:

```
- [ ] `hook rm` touches nothing on either of its paths
- [ ] `hook` starts no tmux server on this path — bootstrap-exemption is unchanged
- [ ] CLAUDE.md's Resume-hook-command bullet no longer claims `hook` is config-file only: it rests the exemption on starting no tmux server and names the `save.requested` touch as the one write outside the config directory
```

**Resolution**: Fixed
**Notes**:
