# Review Tracking: Resume Hooks Silently Lost - Integrity

## Findings

### 1. The shipped user documentation still teaches the retired key form

**Severity**: Important
**Plan Reference**: Phase 4, task `resume-hooks-silently-lost-4-2` (and, by omission, the whole plan)
**Category**: Task Self-Containment / Acceptance Criteria Quality
**Move**: settled
**Change Type**: add-to-task

**Problem**:
After this work unit a hook key is a six-character pane token, and `hook rm` exits non-zero when it removes nothing. Two user-facing texts still teach the opposite, and no task in any phase touches either:

- `README.md`'s `### xctl hook` section shows `xctl hook rm --on-resume --pane-key 'sess:0.1'` as the worked example for removing a specific entry. `sess:0.1` is a key shape nothing can write after Phase 2 and nothing can resolve after Phase 3, so a user copying the documented example gets a non-zero exit and the message `no resume hook registered for sess:0.1` — from the one command the README points them at for cleaning up entries whose pane is gone. The same section never mentions that `hook rm` now fails when it removes nothing, which is a breaking change for anyone scripting it.
- `hooksRmCmd`'s `--pane-key` flag help reads `Structural key of the pane whose hook should be removed (defaults to the current pane)`. "Structural key" names the retired positional form, and it is printed by `portal hook rm --help`.

The plan is otherwise meticulous about stale text — it rewrites CLAUDE.md's hook passages across three phases and explicitly corrects `portal state hydrate --hook-key`'s help string in task 3-3 for exactly this reason. A grep for `README` across the plan returns nothing, so the omission reads as an oversight rather than a scope call.

**Proposal**:
Fold both corrections into task 4-2. It is the last task that changes `hook`'s user-facing contract, it already owns a documentation edit (the CLAUDE.md Resume-hooks paragraph), and doing them in one pass avoids a partial README that describes the token key before the exit rule exists. The `hook list` fourth column needs no README change — the README documents no output format for it — and the README's rename guarantee stays true (a token key widens it rather than narrowing it), so no other passage is touched.

**Current**:
```markdown
- Re-point `cmd/hooks_test.go`: the `"silent no-op when no hook exists for pane"` subtest at `:491` asserts exit 0 for exactly the case that must now fail — invert it and rename it; check the surrounding `--pane-key` subtests seed keys that the removal actually finds; then add coverage for the three routes and for both success paths. Add the exit-0-iff-removed rule to CLAUDE.md's Resume-hooks paragraph, beside the existing `--pane-key` sentence (which stays true), touching none of the key-scheme passages Phases 2 and 3 rewrote.
```

and, in the same task's Acceptance Criteria:

```markdown
- [ ] CLAUDE.md's Resume-hooks paragraph states that `hook rm` exits 0 only when an entry was removed
```

**Proposed Text**:
```markdown
- Re-point `cmd/hooks_test.go`: the `"silent no-op when no hook exists for pane"` subtest at `:491` asserts exit 0 for exactly the case that must now fail — invert it and rename it; check the surrounding `--pane-key` subtests seed keys that the removal actually finds; then add coverage for the three routes and for both success paths. Add the exit-0-iff-removed rule to CLAUDE.md's Resume-hooks paragraph, beside the existing `--pane-key` sentence (which stays true), touching none of the key-scheme passages Phases 2 and 3 rewrote.
- Correct the two user-facing texts that still teach the retired key form, both here because this is the last change to `hook`'s CLI contract. `hooksRmCmd`'s `--pane-key` flag help (`cmd/hooks.go`) currently reads `Structural key of the pane whose hook should be removed (defaults to the current pane)`; rewrite it to name the pane token, keeping the defaults-to-the-current-pane clause. In `README.md`'s `### xctl hook` section, the example `xctl hook rm --on-resume --pane-key 'sess:0.1'` shows a key shape nothing can write after Phase 2 and nothing can resolve after Phase 3 — re-point it at a token-shaped value, and state in the surrounding paragraph (the one that already describes `--pane-key`) that `hook rm` exits non-zero when it removes nothing, so a scripted caller is warned rather than surprised. Touch no other README passage: the rename guarantee it documents still holds and widens under a token key, and `hook list`'s appended column has no documented output format there.
```

and, in the same task's Acceptance Criteria:

```markdown
- [ ] CLAUDE.md's Resume-hooks paragraph states that `hook rm` exits 0 only when an entry was removed
- [ ] `hook rm --pane-key`'s flag help describes the pane token and no longer says `Structural key`
- [ ] README's `xctl hook` section carries no positional `--pane-key` example, and states that `hook rm` exits non-zero when it removes nothing
- [ ] No other README passage is edited — the rename guarantee and the hook-firing explanation are unchanged
```

**Resolution**: Fixed
**Notes**:

---

### 2. A machine with no tmux server is told a restore is in progress

**Severity**: Important
**Plan Reference**: Phase 1, tasks `resume-hooks-silently-lost-1-3`, `resume-hooks-silently-lost-1-4`, `resume-hooks-silently-lost-1-5`
**Category**: Acceptance Criteria Quality / Task Self-Containment
**Move**: choice
**Change Type**: update-task

**Problem**:
Task 1-3 fixes "a failed `@portal-restoring` read counts as set", and tasks 1-4 and 1-5 then route that outcome onto two user-facing surfaces. With no tmux server running, `state.IsRestoringSet` propagates the tmux error (a dead socket is not `ErrOptionNotFound`), so the failed-read branch fires — and the user is told, in words, that a restore is in progress when nothing is running at all:

- `portal doctor --fix` prints `Skipped stale hook prune: restore in progress` (task 1-4's reason→phrase table). Today it prints nothing on a down server.
- `portal doctor` reports the stale-hooks check as `restore in progress (not evaluable)` (task 1-5's detail string). Today it reports `could not enumerate live panes`.

`portal doctor` is bootstrap-exempt precisely so it works on a broken install, and the README states its contract as "It starts nothing (a down runtime is reported honestly, not silently started)". Running `portal doctor` or `portal doctor --fix` with no server up is therefore a normal, supported path, not a corner — and it is the exact path a user takes when they suspect something is wrong. A work unit whose premise is that Portal must not report things that did not happen would be shipping a new false statement on that path.

Task 1-3's Context anticipates the consequence for the *log* value (`reason=restoring` rather than naming the server, "accepted by the specification"), but neither 1-4 nor 1-5 carries it into the prose they introduce — the printed `--fix` line and the check detail — and neither task's Edge Cases mention a down server producing that wording. The check detail in particular is named in 1-5's own Context as "a decision made in this task, not in the specification".

**Options**:
- Widen only the plan-owned wording so it is true of both readings — the `restoring` entry in task 1-4's phrase table and task 1-5's check detail become something like `cannot confirm no restore is in progress` — leaving the logged `reason=restoring` value and the closed three-reason vocabulary untouched (recommended)
- Accept the specification's fixed phrase as-is and record the down-server case explicitly in 1-4's and 1-5's Edge Cases, so it lands as a known accepted outcome rather than as a later bug report
- Keep the check's existing `could not enumerate live panes` branch ahead of the marker read in 1-5, so a down server keeps today's honest detail, accepting that a genuine restore window with an unreadable server is reported as an enumeration failure and that 1-5's ordering acceptance criterion changes

**Resolution**: Pending
**Notes**:

---

### 3. Two tasks state different orderings inside `checkStaleHooks`

**Severity**: Minor
**Plan Reference**: Phase 5, task `resume-hooks-silently-lost-5-2` (against Phase 1, task `resume-hooks-silently-lost-1-5`)
**Category**: Task Self-Containment
**Move**: settled
**Change Type**: update-task

**Problem**:
Task 1-5 fixes the order inside `checkStaleHooks` explicitly and pins it with acceptance criteria and tests: the `store == nil` and `store.Load` guards first, then the `@portal-restoring` read, then the live enumeration. Task 5-2's Edge Cases then say the check "keeps its restore-marker read and its not-evaluable branches ahead of everything" — which, read literally, puts the marker read ahead of the load and contradicts the order 1-5 built.

The two readings differ in observable behaviour once 5-2 puts a shared lock on `Load`: under 1-5's order a `portal doctor` run inside a restore window acquires (or waits out and degrades past) the sidecar lock and may emit an `op=load-unlocked` DEBUG before discarding the result and reporting `restore in progress (not evaluable)`. An implementer reading both tasks has to decide which one governs, and the wrong pick either reorders a tested branch or adds a lock acquisition the plan never sanctioned.

**Proposal**:
Correct 5-2's prose to name 1-5's order rather than restating it loosely. 1-5's ordering is the one carrying acceptance criteria and named tests, so it is what governs; 5-2's bullet is an unpinned aside whose real point is that a degraded read must not change the check's status or the exit code. Stating the concrete order also makes the lock-acquisition consequence visible where it is introduced.

**Current**:
```markdown
- `checkStaleHooks` keeps its restore-marker read and its not-evaluable branches ahead of everything, and a degraded read must not change the check's status or `portal doctor`'s exit code
```

**Proposed Text**:
```markdown
- `checkStaleHooks` keeps the order task 1-5 fixed — the `store == nil` and load guards, then the restore-marker read, then the enumeration — so a restore window reports its detail after the shared acquire has already resolved one way or the other, and a degraded read must not change the check's status, its detail or `portal doctor`'s exit code
```

**Resolution**: Pending
**Notes**:

---
