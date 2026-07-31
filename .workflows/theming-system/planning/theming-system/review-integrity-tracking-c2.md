# Review Tracking: Theming System - Integrity

## Findings

### 1. The row→badge lookup key is never defined, and the obvious one double-badges the `reserved name` collision

**Severity**: Important
**Plan Reference**: Phase 8, tasks `theming-system-8-3` ("The `●` badge derivation table") and `theming-system-8-4` ("Row composition and the invalid-row treatment")
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
Task 8-3's Outcome says `Badges(slots)` returns "a `map[string]Badge` **the panel looks each row up in by its badge key**" — but no task defines what a row's badge key *is*. Task 8-4 declares `themeRowItem{Row theme.Row; Badge theme.Badge}` and renders `Badge`; task 8-7 says only "derive `badges` from the injected slots (task 8-3), build the panel's list from the union's rows". The pairing rule is the one link in the chain nobody writes down.

The obvious implementation is `badges[row.Slug]`, and it is wrong in exactly the case the plan singles out as special. Task 8-1 pins a `reserved name` file as "the **one** legitimate two-rows-for-one-slug case", and task 8-2 pins that such a row's slug "is valid, but it is *identical* to the built-in's". So with `"theme": "nord"` persisted and a `nord.theme` drop-in present:

- the built-in `nord` row has `Slug == "nord"`,
- the rejected `nord.theme` row **also** has `Slug == "nord"`,
- `Badges` returns `{"nord": BadgeConstant}`,
- and a slug-keyed lookup paints `●` on **both** rows.

That breaks two decided properties at once. §9.5's `●` means "what is *persisted*", and the rejected file is emphatically not what is persisted — the persisted slug resolved to the **built-in**, which is precisely why task 7-6 gives doctor the mirror rule ("a persisted slug naming a `reserved name` file resolves to the *built-in* and produces no persisted line at all"). And two `●` markers on screen contradict the badge vocabulary's whole premise that the marker says what one setting slot points at.

The gap is also invisible to the plan's tests: task 8-3's cases are all derivation-table cases with no rows in sight, task 8-4's are composition cases with a badge supplied by the test, and task 9-11's behaviour suite covers "the built-in-first tie" and "the three-row badge derivation table" as separate groups but never their intersection. Nothing would catch it.

The fix is one exported function beside the derivation table — the key is the row's identity (the same value `SortKey()` derives from, which task 8-3's existing criterion already relies on for the charset-rejected case) with a single exclusion for `reserved name`.

**Current**:

Task `theming-system-8-3`, **Do** — the first bullet:

- Add `internal/theme/badge.go`:
  ```go
  type Badge int // BadgeNone, BadgeConstant, BadgeLight, BadgeDark, BadgeBoth
  func (b Badge) Text() string // "", "●", "● light", "● dark", "● both"
  func Badges(slots []SlotResolution) map[string]Badge
  ```

**Proposed**:

Task `theming-system-8-3`, **Do** — replace the first bullet with:

- Add `internal/theme/badge.go`:
  ```go
  type Badge int // BadgeNone, BadgeConstant, BadgeLight, BadgeDark, BadgeBoth
  func (b Badge) Text() string // "", "●", "● light", "● dark", "● both"
  func Badges(slots []SlotResolution) map[string]Badge
  // BadgeKey is the value a Row is looked up in Badges' map by. "" means the row
  // can never carry a badge.
  func (r Row) BadgeKey() string
  ```

Task `theming-system-8-3`, **Do** — add after the "**Shapes**" bullet:

- **Define the row's lookup key, and exclude the one collision.** `Row.BadgeKey()` returns the row's identity — `Slug` where one exists, else `Persisted`, else `Filename` — i.e. the same value `SortKey()` derives from, which is what makes task 8-1's charset-rejected persisted row (keyed on its raw string) match its badge. The **one exception is a `reserved name` row, which returns `""` and therefore never carries a badge**: its slug is identical to the built-in's by definition, so a bare identity lookup would paint `●` on *both* rows, and the rejected file is not what is persisted — the persisted slug resolved to the built-in, which is the same discrimination task 7-6 draws for doctor's persisted line. State in-source that this is the only place the union's one legitimate two-rows-for-one-slug case has an observable consequence, and that the panel must look badges up through this method rather than reading `Slug` directly.

Task `theming-system-8-3`, **Acceptance Criteria** — add after "A charset-rejected persisted value is badged on that raw value, so it matches the union row keyed by the same string (task 8-1) and the badge is not lost.":

- [ ] `Row.BadgeKey()` returns the slug for a built-in, a valid file and a `not found` persisted row; the raw persisted string for a charset-rejected row; the filename for a `bad name` row; and **`""` for a `reserved name` row**.
- [ ] With `"theme": "nord"` persisted and a `nord.theme` drop-in present, exactly **one** row's `BadgeKey()` matches the badge map — the built-in's — so only one `●` can render.

Task `theming-system-8-3`, **Tests** — add after `"it badges a charset-rejected persisted value on that value"`:

- `"it gives a reserved-name row no badge key"` — `TestBadgeKey_ReservedNameRowHasNone`
- `"it keys every other row on its identity"` — `TestBadgeKey_MatchesRowIdentity` (table: built-in, valid file, `not found` persisted, charset-rejected persisted, `bad name` file)

Task `theming-system-8-3`, **Edge Cases** — add after "The badge and the cursor are independent signals (`●` is what is *set*, `▌` + tint is what is *previewed*), so a badge legitimately sits on an unselectable row.":

- The row→badge lookup key is `Row.BadgeKey()`, never `Row.Slug` read directly — because a **`reserved name`** row shares its slug with the built-in it collides with by definition, so a bare slug lookup paints `●` on both rows. The rejected file is not what is persisted (the slug resolved to the built-in), so `BadgeKey()` returns `""` for it — the only observable consequence of §9.4's one legitimate two-rows-for-one-slug case.

Task `theming-system-8-4`, **Acceptance Criteria** — add after "A `bad name` row is labelled by its **filename** and a `reserved name` row likewise, both via `Row.Label()` with no second derivation in the delegate.":

- [ ] A `reserved name` row renders **no** `●` even when its slug is the persisted one, while the built-in it collides with renders the badge — asserted on the two adjacent rows of the same union.

Task `theming-system-8-4`, **Tests** — add after `"it labels bad-name and reserved-name rows by filename"`:

- `"it badges the built-in and not its reserved-name collider"` — `TestThemeRow_ReservedNameRowCarriesNoBadge`

**Resolution**: Fixed
**Notes**:

---

### 2. The `t` entry gate is specified to read a `DirUnusable` state that does not exist until after the panel opens

**Severity**: Important
**Plan Reference**: Phase 8, task `theming-system-8-13` ("Entry conditions, blocked-`t` flashes and key-exclusive routing")
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
Task 8-11's height floor is **conditional on the directory's state**: "header (two rows) + footer + one list row + one message row, plus the `⚠ dir unreadable` chrome row when the directory is unusable — otherwise the warning would consume the single list row while §9.5 simultaneously requires rows beneath it." Its predicate signature is `themePanelFloor(contentW, contentH int, dirUnusable bool)`.

Task 8-13 then wires that predicate into the entry gate "on the current content dimensions and **the panel's would-be `DirUnusable` state**" — but that state is a product of the enumeration, and task 8-7 pins the enumeration to the keypress *after* the gate ("The directory read happens **here**, on the keypress — never at construction"; the gate runs first and "does nothing else"). At entry time there is no `DirUnusable` to read, and "would-be" names the problem without resolving it.

The implementer has to pick, and both obvious picks have a visible defect:

- **Assume `false`** — a user with an unreadable directory on a terminal exactly at the non-directory floor passes entry, then task 8-6's layout allocates header(2) + dir row(1) + footer + message budget and leaves the list body at **zero rows**, violating both task 8-6's "list body (the remainder, ≥ 1)" and §9.5's requirement that built-in and persisted rows still render beneath the pinned warning — the exact state that row exists to prevent.
- **Assume `true`** — the gate reserves a row that usually is not needed, refusing `t` on terminals that would have rendered a perfectly good panel, contradicting §9.8's degrade-don't-refuse and the floor §9.8 actually defines.

This is the one place in the panel's geometry where the plan's otherwise-complete floor arithmetic has no input. Pinning the evaluation order closes it without changing any decided property: one predicate (task 8-11's, unchanged), evaluated against the real flag as soon as the flag exists.

**Current**:

Task `theming-system-8-13`, **Do** — the first bullet:

- **Add the gate** `func (m Model) themePanelEntry() (blockedFlash string, ok bool)` in `internal/tui/theme_panel.go`, evaluated in this order: `m.colourless` (the `NO_COLOR` carve-out) → task 8-11's `themePanelFloor(...)` on the current content dimensions and the panel's would-be `DirUnusable` state → otherwise open. It returns the pinned copy for the blocked cases and is the **only** place the decision is made; `t`'s dispatch on both pages consults it and does nothing else.

**Proposed**:

Task `theming-system-8-13`, **Do** — replace the first bullet with:

- **Add the gate** `func (m Model) themePanelEntry() (blockedFlash string, ok bool)` in `internal/tui/theme_panel.go`, evaluated in this order: `m.colourless` (the `NO_COLOR` carve-out) → task 8-11's `themePanelFloor(contentW, contentH, false)` on the current content dimensions → otherwise open. It returns the pinned copy for the blocked cases and is the **only** place the pre-read decision is made; `t`'s dispatch on both pages consults it and does nothing else.
- **Re-evaluate the floor once the directory's state is known.** The `⚠ dir unreadable` row raises the height floor by one (task 8-11) but `Union.DirUnusable` is a product of the enumeration, which task 8-7 pins to the keypress *after* this gate — so the pre-read evaluation passes `dirUnusable = false` and task 8-7's open sequence re-applies **the same predicate** with the real flag as soon as `Open` returns. If the real flag now fails the floor, discard the enumeration, do **not** open, and raise the same pinned height flash (`terminal too short for the theme picker`). One predicate, two evaluations — task 8-11's "compute it once" is about not re-deriving the arithmetic, not about evaluating it once. Record why neither shortcut is taken: assuming `true` at entry refuses terminals that would have rendered a good panel, contradicting §9.8's degrade-don't-refuse; assuming `false` and never re-checking opens a panel whose list body is **zero rows**, which is the "completely in the dark" state §9.5's pinned row exists to prevent. Accept, and state in-source, that a blocked open in this rare case has already performed its directory read and emitted `theme: enumerated` — the enumeration genuinely happened, and splitting the read from its emission would fork task 8-1's seam for one edge.

Task `theming-system-8-13`, **Acceptance Criteria** — add after "The entry gate and the resize path read the **same** floor predicate — a size that blocks entry also force-closes an open panel, asserted across one table.":

- [ ] With a **usable** directory, a terminal one row above the non-directory floor opens the panel — the gate does not reserve the `⚠ dir unreadable` row speculatively.
- [ ] With an **unusable** directory at that same height, the panel does **not** open: the enumeration is discarded, `terminal too short for the theme picker` is raised, and no panel state survives.
- [ ] A panel that opens with an unusable directory always renders **at least one list row** beneath the pinned `⚠ dir unreadable` row — asserted at the directory-inclusive floor exactly.

Task `theming-system-8-13`, **Tests** — add after `"it blocks below each render-floor dimension"`:

- `"it does not reserve the directory row before it knows"` — `TestPanelEntry_UsableDirectoryOpensAtTheNonDirFloor`
- `"it blocks after the read when the directory row raises the floor"` — `TestPanelEntry_UnusableDirectoryBlocksOnTheReEvaluation` (enumeration discarded, pinned short flash, panel closed)

Task `theming-system-8-13`, **Edge Cases** — add after "Below-the-floor is an **entry** condition as well as §9.8's resize condition, and both read the one predicate.":

- The floor is **conditional on `DirUnusable`**, which does not exist until the enumeration runs on the keypress — so the predicate is evaluated twice against the same function: once before the read with `dirUnusable = false`, once immediately after `Open` returns with the real flag. Assuming `true` up front would refuse terminals that fit; assuming `false` and never re-checking would open a panel with **zero** list rows beneath the pinned warning row, the state §9.5 requires rows beneath it precisely to prevent. A blocked open on the re-evaluation has already read the directory and emitted `theme: enumerated`, which is accepted rather than worked around.

**Resolution**: Fixed
**Notes**:

---

### 3. Task 8-13 lists `d` among the keys the panel swallows, contradicting its own "the panel owns `d`" and falsifying its test in Phase 9

**Severity**: Minor
**Plan Reference**: Phase 8, task `theming-system-8-13` ("Entry conditions, blocked-`t` flashes and key-exclusive routing")
**Category**: Acceptance Criteria Quality
**Change Type**: update-task

**Details**:
Task 8-13's Edge Cases state the rule correctly — "The panel is **key-exclusive** — it owns arrows, `Enter`, `d`, `l` and `Esc` and swallows everything else" — but its Do, its acceptance criterion and its test table all list `d` among the **swallowed** keys, and omit `l`. The asymmetry is the tell: `d` and `l` are the same kind of key (task 8-5's descriptor marks both `Core`; task 9-3 makes both commit keys), so listing one and not the other is an oversight rather than a statement that `d` is inert this phase.

Two consequences. Within the task, the Do contradicts the Edge Cases about the same key. Across the plan, an implementer encodes `d` into `TestPanelRouting_KeyExclusive`'s table asserting "the model state is unchanged", and task 9-3 then makes `d` write the dark slot and mutate `m.themeKeys` — so the test becomes false and nothing in task 9-3 tells anyone to edit it.

The intent behind listing `d` is real and worth keeping: on Projects, `d` is the delete key, so it must not open the delete modal from behind the panel. That is what the criterion should assert — a statement that stays true after task 9-3.

**Current**:

Task `theming-system-8-13`, **Do** — the "Key-exclusive routing" bullet:

- **Key-exclusive routing**: task 8-7 already swallows non-panel keys; here pin it as tested behaviour with the reasoning attached — `k` does not kill, `x` does not switch page, `m` does not enter multi-select, `/` does not filter, `?` does nothing (there is no panel help modal), `s`/`e`/`d`/`n`/`r`/`q` are inert — while **`Ctrl-C` stays live** and quits. State in-source that non-blanking and key-exclusive are not in tension: seeing the list without being able to drive it *is* the live-preview premise.

Task `theming-system-8-13`, **Acceptance Criteria** — the routing criterion:

- [ ] While the panel is open: `k`, `x`, `m`, `/`, `?`, `s`, `e`, `d`, `n`, `r`, `q` each leave the model state unchanged (no kill, no page switch, no mode entry, no filter, no modal).

Task `theming-system-8-13`, **Tests** — the routing test:

- `"it swallows every page key while open"` — `TestPanelRouting_KeyExclusive` (table over k/x/m//,?,s,e,d,n,r,q)

Task `theming-system-8-13`, **Edge Cases** — the key-exclusive bullet:

- The panel is **key-exclusive** — it owns arrows, `Enter`, `d`, `l` and `Esc` and swallows everything else, because `k` would kill the highlighted session while you pick a theme, `x` would swap pages behind it and `m` would start a multi-select — but **`Ctrl-C` stays live**, since swallowing it would take away the exit key inside a settings surface.

**Proposed**:

Task `theming-system-8-13`, **Do** — replace the "Key-exclusive routing" bullet with:

- **Key-exclusive routing**: task 8-7 already swallows non-panel keys; here pin it as tested behaviour with the reasoning attached — `k` does not kill, `x` does not switch page, `m` does not enter multi-select, `/` does not filter, `?` does nothing (there is no panel help modal), `s`/`e`/`n`/`r`/`q` are inert — while **`Ctrl-C` stays live** and quits. `d` and `l` are **panel-owned** keys, not swallowed ones: they are inert this phase and become commit keys in task 9-3, so assert them as "the page's own binding never fires" (on Projects, `d` must not open the delete modal) rather than as "the model is unchanged", which task 9-3 would falsify. State in-source that non-blanking and key-exclusive are not in tension: seeing the list without being able to drive it *is* the live-preview premise.

Task `theming-system-8-13`, **Acceptance Criteria** — replace the routing criterion with:

- [ ] While the panel is open: `k`, `x`, `m`, `/`, `?`, `s`, `e`, `n`, `r`, `q` each leave the model state unchanged (no kill, no page switch, no mode entry, no filter, no modal).
- [ ] `d` and `l` never reach the page beneath while the panel is open — on Projects, `d` opens **no** delete modal — asserted as an absence of the page's effect, so the criterion still holds once task 9-3 makes them commit keys.

Task `theming-system-8-13`, **Tests** — replace the routing test with:

- `"it swallows every page key while open"` — `TestPanelRouting_KeyExclusive` (table over k/x/m//,?,s,e,n,r,q)
- `"it keeps the panel's own keys off the page beneath"` — `TestPanelRouting_PanelOwnedKeysNeverReachThePage` (`d` on Projects opens no delete modal; `l` reaches no page binding)

Task `theming-system-8-13`, **Edge Cases** — replace the key-exclusive bullet with:

- The panel is **key-exclusive** — it owns arrows, `Enter`, `d`, `l` and `Esc` and swallows everything else, because `k` would kill the highlighted session while you pick a theme, `x` would swap pages behind it and `m` would start a multi-select — but **`Ctrl-C` stays live**, since swallowing it would take away the exit key inside a settings surface. `d`/`l` are owned rather than swallowed, so they are asserted as never reaching the page beneath (no Projects delete modal) rather than as leaving the model unchanged — task 9-3 makes them write.

**Resolution**: Fixed
**Notes**:

---

### 4. Task 1-7's Do instructs wiring the event-logger seam that task 1-8 introduces

**Severity**: Minor
**Plan Reference**: Phase 1, task `theming-system-1-7` ("Enumerate the themes directory into classified entries")
**Category**: Dependencies and Ordering
**Change Type**: update-task

**Details**:
Task 1-7's final **Do** bullet instructs the implementer to "Wire the event-logger seam from task 1.8 at this call site" and enumerates the three emission cases. Task 1-8 — the *next* task — then owns exactly that work with the same three cases ("Thread the `*EventLogger` onto the `Loader` (constructor parameter) and emit from `Enumerate`: one `Rejected` per rejected entry, one `DirectoryUnusable` for the unusable-directory verdict, and **nothing whatsoever** for an absent directory").

`EventLogger` does not exist at task 1-7, so the bullet is either a compile error or dead instruction, and the two tasks duplicate one deliverable. Everywhere else in this plan a forward reference is written as a phase-boundary note (task 1-7's own **Context** does exactly that: "a `theme: directory unusable` log entry (task 1.8)"), so this reads as a slip rather than an intended overlap. Task 1-7's acceptance criteria and tests carry no emission assertion, so nothing else has to move.

**Current**:

Task `theming-system-1-7`, **Do** — the final bullet:

- Wire the event-logger seam from task 1.8 at this call site: one `theme: rejected` per rejected entry, one `theme: directory unusable` for the unusable-directory verdict, and **nothing** for an absent directory.

**Proposed**:

Task `theming-system-1-7`, **Do** — replace the final bullet with:

- Leave the `Loader` **without** an event-logger seam here: `Enumerate` returns its entries and its directory verdict and emits nothing. Task 1-8 introduces the seam, threads it onto the `Loader` and adds the three emissions at these exact call sites (one `theme: rejected` per rejected entry, one `theme: directory unusable` for the unusable-directory verdict, and nothing for an absent directory) — so structure `Enumerate` so those three points are distinguishable, but do not build the logger here.

**Resolution**: Fixed
**Notes**:

---

### 5. Task 3-2 removes task 3-1's transitional theme source without saying whether `New`'s default seed survives

**Severity**: Minor
**Plan Reference**: Phase 3, task `theming-system-3-2` ("`tui.Build` takes the loaded nomination and the gate selects its member")
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
Task 3-1 adds two theme sources to `internal/tui`: an unexported pair holder seeded in `Build`, **and** a seed in `New` — "`New` seeds `activeTheme` from the dark built-in before options apply so a model constructed without `Build` is still themed". The second exists to close a hazard 3-1 names explicitly: a model with an empty `Theme` renders through `lipgloss.Color("")`'s no-colour sentinel, which is "a silent colourless render, not a compile error".

Task 3-2 then says "drop the transitional built-in-pair holder task 3-1 added — the nomination is now the source", and its Edge Cases add that a zero `Nomination`'s `Select`/`Constant` "return a zero `Theme` rather than panic". Whether `New`'s seed is part of "the transitional holder" is unstated, and the two readings diverge materially: if it goes, every `New(...)` constructed without `WithThemeNomination` — in-package tests, and any later fixture path that forgets the option — renders silently colourless, reinstating exactly the hazard 3-1 mitigated, with no compile signal and no test failure. Tasks in Phases 4, 8 and 9 build models directly (task 8-12 explicitly relies on "a bare `Model{}`" for the flash primitives), so the answer is consumed repeatedly and should be stated once.

**Current**:

Task `theming-system-3-2`, **Do** — the "Replace the injection" bullet:

- **Replace the injection in `internal/tui`**: delete `Deps.Appearance` and the `WithAppearance` option, add `Deps.Theme theme.Nomination` and a `WithThemeNomination` option, and drop the transitional built-in-pair holder task 3-1 added — the nomination is now the source. `Build` always injects it (a zero-value nomination is not a valid state; see Edge Cases).

**Proposed**:

Task `theming-system-3-2`, **Do** — replace the "Replace the injection" bullet with:

- **Replace the injection in `internal/tui`**: delete `Deps.Appearance` and the `WithAppearance` option, add `Deps.Theme theme.Nomination` and a `WithThemeNomination` option, and drop the transitional built-in-pair holder task 3-1 added — the nomination is now the source. `Build` always injects it (a zero-value nomination is not a valid state; see Edge Cases). **`New`'s dark-built-in seed of `activeTheme` stays**, and is not part of the holder being dropped: a model constructed without `Build` (or with `New` plus options but no nomination) must still be themed, because an empty `Theme` resolves through `lipgloss.Color("")`'s no-colour sentinel — a silent colourless render with no compile error and no failing assertion, which is the hazard task 3-1 added the seed to close and which Phases 4, 8 and 9 all build models against. Applying a nomination through `WithThemeNomination` (or `Build`) overwrites it.

Task `theming-system-3-2`, **Acceptance Criteria** — add after "`Deps.Appearance` and `WithAppearance` do not exist; a source guard proves `internal/tui` declares neither (they are **removed rather than left alongside**).":

- [ ] `New()` with no theme option still yields a themed model — `activeTheme` carries `tokyo-night`'s values, not a zero `Theme` — and a render from it emits truecolor SGRs rather than a silently colourless frame.

Task `theming-system-3-2`, **Tests** — add after `"it removes the appearance injection outright"`:

- `"it still themes a model built without a nomination"` — `TestNew_SeedsTheDarkBuiltinWhenNoNominationIsGiven`

Task `theming-system-3-2`, **Edge Cases** — add after "The zero value of `Nomination` is neither state. `Build` always injects one; give the type a constructor-only contract (unexported fields) so a zero value cannot be constructed accidentally, and make `Select`/`Constant` on a zero value return a zero `Theme` rather than panic.":

- `New`'s dark-built-in seed of `activeTheme` **survives** this task — only `Build`'s transitional pair holder is dropped. Without the seed, a model constructed without a nomination renders through `lipgloss.Color("")`'s no-colour sentinel: silently colourless, with no compile error and no failing assertion, which is precisely why task 3-1 added it.

**Resolution**: Fixed
**Notes**:
