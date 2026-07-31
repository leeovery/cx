# Review Tracking: Theming System - Traceability

Cycle 4. Full fresh pass over the whole plan (10 phases, 97 tasks) against
`.workflows/theming-system/specification/theming-system/specification.md`, in both
directions — not a narrowed check of cycles 1–3's fixes.

**Direction 2 (plan → specification) is clean.** Every task's Problem, Solution, Do,
Acceptance Criteria, Tests and Edge Cases traces to a named spec section, and every task
carries a `Spec Reference` plus quoted `Context`. The places the plan resolves a genuine
spec gap remain labelled **Ambiguity flagged** / **Decision recorded** in-task (1-8's
`token` attr cardinality, 2-5's Oklab metric, 3-1's `testdata/vhs/reference/` scoping,
4-2's `captureKeys`, 5-2's control-only persisted value, 6-1's `Field != ""`
discriminator, 6-4's absent-file translation no-op, 6-6's migration attrs, 7-1's summary
indentation, 7-2's glyph-in-copy ownership, 7-6's advisory-region order, 8-7's
`Deps.ThemeSlots`, 8-10's `theme: loaded` panel-path cadence, 8-11's floor without a
keep-N-columns rule, 8-13's two-evaluation floor predicate, 9-8's filter-line precedence).
No hallucinated requirement, behaviour, edge case or acceptance criterion was found.

Two constructions were checked hardest this cycle and are **correct, not deviations**:
task 3-3 **deletes** `canvasHexFor` where §11.4 says "make `canvasHexFor` theme-agnostic"
— a surviving theme-agnostic version would need a hardcoded canvas hex in `internal/tui`,
which §7.1 and the colour-literal guard forbid, so deletion is the only coherent reading of
the stated property; and task 8-4 adds a two-cell **cursor column** ahead of §9.5's
four-element priority, which is the mechanical consequence of §9.1's cursor-row selection
treatment rather than a fifth invented element.

**Direction 1 (specification → plan)** found two elements with no plan home. Both land on
tasks that already exist — so each is an `add-to-task`, not new scope.

Sections verified as fully covered but recorded because they are thin on explicit
citations: §1.4's deferred items (each either has a task or is a pure no-op — the
one-shot detection seed and the general settings panel implement nothing, and no task
invites either), §6.5 (a no-action decision — nothing in the plan invites a terminal
capability gate), §8.3's ecosystem check and §9.14's picker-half prior art (design
rationale with no implementation consequence), §15.4's rejected `Theme slide-over — B`
frame (retained as a record, correctly absent from every task). Enumerated sets were
counted one by one and all match: §2.4's 19 tokens, §6.2's seven reasons and six-rung
ladder, §8.5's three fallback rows, §9.1's twelve panel-surface tokens, §9.5's three badge
rows, §10.2's four mapping rows, §12.3's seven events and seven attr keys, §12.5's four
README sites, §12.6's seven CLAUDE.md entries, §13.3's ten panel fixtures, §13.5's floor
rule set, §13.6's sixteen test-reshape rows, §14.2's two footers, §15.1's three amendments.

## Findings

### 1. §5.8's mirror case — a repaired theme taking effect on the next open — is asserted by no task

**Type**: Incomplete coverage
**Spec Reference**: §5.8 ("**The mirror case works for the same reason**: fixing a previously-invalid theme takes effect on the next panel open, without relaunching. That symmetry is what §5.8 exists to buy."), §9.2 (the two edited-file cases), §8.5
**Plan Reference**: Phase 8, task `theming-system-8-8` ("Opening lands the cursor on the theme actually rendering")
**Change Type**: add-to-task

**Details**:

§5.8 states two consequences of "the panel's parse supersedes the construction-time parse
for the same slug", and presents them as a matched pair:

> - **`Esc` resolves persisted state against the panel's enumeration**, not against what
>   construction loaded. If the user edited their active theme's file and broke it, `Esc`
>   lands on the §8.5 fallback…
> - **The mirror case works for the same reason**: fixing a previously-invalid theme takes
>   effect on the next panel open, without relaunching. That symmetry is what §5.8 exists
>   to buy.

The plan covers the first half thoroughly (task 8-10) and covers the *breaking* direction
at open thoroughly (task 8-8: "Editing the active theme's file to make it **invalid** and
then opening flips the screen to the §8.5 fallback **on open**"). The **repairing**
direction — the launch rendered the fallback because the persisted theme was already broken
at construction, the user fixes the file, and the next open must apply their own theme — is
named in exactly one place in the whole plan: an Edge Cases line in task **8-10**, the
*close* task ("The mirror case is the whole point of re-reading — fixing a previously-invalid
theme takes effect on the next open with no relaunch"). Neither 8-8 nor 8-10 carries an
acceptance criterion or a test for it, and the behaviour the line describes happens at
**open**, which is task 8-8's territory, not 8-10's.

That leaves the one behaviour §5.8 names as its own payoff with no verification anywhere,
and it is not a free consequence of the cases that *are* covered. The two covered cases
both move the cursor *away* from the persisted slug or leave it where it was; the repair
case is the only one that moves the cursor **onto** a row that was unselectable at
construction, while the `●` — which never moved off the persisted slug (§9.5's badge table)
— stops sitting on an unselectable row. An implementation that resolved only against the
construction-time record for a slug it had already classified as broken would satisfy every
existing criterion in 8-8 and still leave the user staring at the fallback after fixing
their file, which is precisely the state §5.8 exists to prevent.

Task 8-8 is the single correct home: it owns open-time re-resolution, it already carries the
other three edited-file outcomes, and its `Resolve`-against-the-retained-enumeration step is
the mechanism the mirror case rides on.

**Current** (task `theming-system-8-8`, the two adjacent **Acceptance Criteria** bullets):

```markdown
- [ ] Editing the active theme's file to new but **valid** values and then opening re-renders the same slug with the new values, with no arrowing required.
- [ ] Editing the active theme's file to make it **invalid** and then opening flips the screen to the §8.5 fallback **on open**, with the persisted row unselectable and reasoned.
```

**Proposed** (task `theming-system-8-8`, the same two bullets plus a third):

```markdown
- [ ] Editing the active theme's file to new but **valid** values and then opening re-renders the same slug with the new values, with no arrowing required.
- [ ] Editing the active theme's file to make it **invalid** and then opening flips the screen to the §8.5 fallback **on open**, with the persisted row unselectable and reasoned.
- [ ] **Repairing** a theme that was already invalid at construction — so the launch is rendering the §8.5 fallback — and then opening applies the **persisted** theme on that open: the cursor moves from the fallback's row onto the persisted slug's now-selectable row, the `●` (which never left it) is on that same row, the fallback's row carries no badge, and **no relaunch is required**. This is §5.8's mirror case and the payoff re-reading exists to buy, so it is asserted rather than inferred from the breaking direction.
```

**Current** (task `theming-system-8-8`, the two adjacent **Tests** entries):

```markdown
- `"it applies an edited-but-valid active theme on open"` — `TestPanelOpen_AppliesMidSessionEdit`
- `"it flips to the fallback on open when the active theme is broken"` — `TestPanelOpen_InvalidatedActiveThemeFlipsOnOpen`
```

**Proposed** (task `theming-system-8-8`, the same two entries plus a third):

```markdown
- `"it applies an edited-but-valid active theme on open"` — `TestPanelOpen_AppliesMidSessionEdit`
- `"it flips to the fallback on open when the active theme is broken"` — `TestPanelOpen_InvalidatedActiveThemeFlipsOnOpen`
- `"it applies a repaired theme on open with no relaunch"` — `TestPanelOpen_RepairedThemeAppliesOnOpen` (constructed on a broken persisted slug so the model starts on the fallback; the enumeration handed to `Open` carries the repaired file)
```

**Current** (task `theming-system-8-8`, the **Edge Cases** bullet on the invalidating edit):

```markdown
- An edit that **invalidates** the active theme resolves §8.5's fallback and flips on **open**, never deferred to `Esc`, because deferring would leave the panel listing a theme as invalid while the screen still renders it.
```

**Proposed** (task `theming-system-8-8`, that bullet plus its mirror):

```markdown
- An edit that **invalidates** the active theme resolves §8.5's fallback and flips on **open**, never deferred to `Esc`, because deferring would leave the panel listing a theme as invalid while the screen still renders it.
- The **mirror case lands on the same open** and is the payoff §5.8 exists to buy: a theme that was invalid at construction — so the launch is rendering the fallback — becomes loadable once the user fixes the file, and the next open applies **their** theme, moving the cursor off the fallback's row onto the persisted slug's now-selectable one. It is not a free consequence of the breaking direction: it is the only case that moves the cursor *onto* a row that was unselectable at construction, and an implementation that trusted the construction-time classification of an already-broken slug would satisfy every other criterion here while leaving the user on the fallback after they fixed their file.
```

**Current** (task `theming-system-8-8`, closing lines of **Context**):

```markdown
> §8.4: "**A stale hand-edited slot** resolves from the panel's retained enumeration (§5.8), which already parsed and classified every file in the directory when the panel opened… **No commit-time directory read.** Issuing one would produce a *third* parse of the same slug — neither construction's nor the panel's — that can disagree with the row the user is looking at."
> §9.2: "…**re-anchors the cursor to the previewed theme's identity, never to its index**. Anchoring to an index would silently break §9.2's invariant the moment a row is inserted above the cursor."
```

**Proposed** (task `theming-system-8-8`, closing lines of **Context**):

```markdown
> §8.4: "**A stale hand-edited slot** resolves from the panel's retained enumeration (§5.8), which already parsed and classified every file in the directory when the panel opened… **No commit-time directory read.** Issuing one would produce a *third* parse of the same slug — neither construction's nor the panel's — that can disagree with the row the user is looking at."
> §9.2: "…**re-anchors the cursor to the previewed theme's identity, never to its index**. Anchoring to an index would silently break §9.2's invariant the moment a row is inserted above the cursor."
> §5.8: "**The panel's parse supersedes the construction-time parse for the same slug.** After a mid-session edit the panel holds the fresher truth, and that is the entire point of re-reading. Two consequences, both following from the same rule: **`Esc` resolves persisted state against the panel's enumeration**, not against what construction loaded… **The mirror case works for the same reason**: fixing a previously-invalid theme takes effect on the next panel open, without relaunching. That symmetry is what §5.8 exists to buy."
```

**Resolution**: Pending
**Notes**: `planning.md`'s Phase 8 task table carries a condensed Edge Cases cell for
`theming-system-8-8`; if the fix is approved it should gain the matching condensed clause so
the table and the detail file do not drift — suggested text: `the mirror case lands on the
same open — a theme that was invalid at construction becomes loadable once the user fixes
the file, and the next open applies theirs, moving the cursor off the fallback's row onto
the persisted slug's now-selectable one, with no relaunch`. Task 8-10's Edge Cases already
carry the mirror case in prose and are deliberately left unchanged — the *close* re-resolves
from the same record, so it inherits the behaviour, but the flip is an **open**-time event
and belongs to 8-8. No task file other than `phase-8-tasks.md` changes.

---

### 2. Task 8-4's reason-vocabulary test names six labels where §6.2 declares seven

**Type**: Incomplete coverage
**Spec Reference**: §6.2 (the seven reject classes: `missing tokens`, `bad colour`, `bad syntax`, `bad name`, `reserved name`, `unreadable`, `not found`), §9.4 (`not found` and `bad name` persisted-slug rows), §14A ("**Panel — rows (§9.5):** §6.2's reason labels verbatim, each prefixed `⚠ `")
**Plan Reference**: Phase 8, task `theming-system-8-4` ("Row composition and the invalid-row treatment")
**Change Type**: update-task

**Details**:

§6.2 declares **seven** reject classes and §14A pins that the panel row renders "§6.2's
reason labels verbatim, each prefixed `⚠ `". Task 8-4's **Do** section is correct and
enumerates all seven:

> **Reason text is §6.2's terse vocabulary verbatim, prefixed `⚠ `** — the `Reason`
> constants' own string values (`missing tokens`, `bad colour`, `bad syntax`, `bad name`,
> `reserved name`, `unreadable`, `not found`).

Its verification surface does not follow: the task's only reason-vocabulary test is named
`TestThemeRow_ReasonLabelsAreTheSixTerseStrings`, and no acceptance criterion pins the
count. A table-driven test written to its own name covers six of the seven and passes,
leaving one label unverified on the surface §6.3 makes solely responsible for telling a user
their file did not work.

The undercount is not cosmetic, because the seventh label is the one with the least cover
elsewhere. All seven are reachable on a panel row — task 8-1's union mints `not found` and
`bad name` rows for persisted slugs, `reserved name` for a colliding file, and the four
content reasons come back from `LoadFile` — but **`not found` is the one reason §6.2 places
deliberately outside the ladder** ("`not found` is not in this ladder — it applies only to a
persisted slug with no file"), so task 1-5 asserts it is *never* produced
(`TestLoadFile_NotFoundIsOutsideTheLadder`) and no loader-side test renders it. If the
dropped case is that one, the label that only ever reaches a user through the panel is the
one nothing renders in a test.

The fix is a corrected test name plus one acceptance criterion, so the count is pinned
rather than inferred from a name. Task 8-4's **Do** section already carries the right list
and needs no edit.

**Current** (task `theming-system-8-4`, the final **Tests** entry):

```markdown
- `"it renders the terse reason vocabulary verbatim"` — `TestThemeRow_ReasonLabelsAreTheSixTerseStrings`
```

**Proposed** (task `theming-system-8-4`, the final **Tests** entry):

```markdown
- `"it renders the terse reason vocabulary verbatim"` — `TestThemeRow_ReasonLabelsAreTheSevenTerseStrings` (table over **all seven** §6.2 labels — `missing tokens`, `bad colour`, `bad syntax`, `bad name`, `reserved name`, `unreadable`, `not found` — each asserted with its `⚠ ` prefix)
```

**Current** (task `theming-system-8-4`, the **Acceptance Criteria** bullet on `text.faint`):

```markdown
- [ ] `text.faint` appears nowhere in a panel row's output.
```

**Proposed** (task `theming-system-8-4`, that bullet plus the count criterion beneath it):

```markdown
- [ ] `text.faint` appears nowhere in a panel row's output.
- [ ] All **seven** §6.2 reason labels render verbatim, each prefixed `⚠ ` — `missing tokens`, `bad colour`, `bad syntax`, `bad name`, `reserved name`, `unreadable` and `not found`. The count is asserted rather than left to the test's name: `not found` is the one reason §6.2 keeps **outside** the ladder, so task 1-5 pins that `LoadFile` never produces it and it reaches a row only through task 8-1's union — making the panel the sole surface that renders it.
```

**Resolution**: Pending
**Notes**: Task 8-4's **Do** section already enumerates all seven and is unchanged by this
fix. `planning.md`'s Phase 8 cell for `theming-system-8-4` states the rule without a count
("reason labels are §6.2's terse vocabulary verbatim, each prefixed `⚠ `, with full detail
staying in doctor"), so it carries no drift and needs no edit. No task file other than
`phase-8-tasks.md` changes.
