# Phase 9: Panel commits — assignment, confirmation and failure reporting — 12 tasks

## theming-system-9-1

### Task 9.1: `Enter` commits a constant through the persister

**Problem**: The panel lists, previews and closes, but it has never written anything — Phase 8 deliberately left `Enter`/`d`/`l` swallowed so browsing could land as its own checkpoint. The first write is where four decisions that read as small all bite. **`Enter` must not close**: a user who had just set both slots would press it to leave and thereby commit a constant, wiping the pair they just built — so `Esc` stays the only way out, there is no dual-purpose key, and the pair flow needs no special case. It must commit the **cursor's** slug, not the persisted one — the cursor is what is previewed and what the user is looking at, and §9.5 draws that split deliberately (`●` is what is *set*, the cursor is what is *previewed*). It must **not** re-theme: a commit is a write, not a navigation, so the frame is unchanged and committing to a non-active slot (task 9-3) changes nothing on screen. And the in-memory key mutation must apply to the **construction-time snapshot** the panel has used since it opened, never to the merged bytes the persister's read-modify-write just had in hand — re-deriving from those would make the panel jump to another instance's choices at the moment the user presses a key, the cross-instance sync §8.4 explicitly declines. Without the in-memory mutation at all, task 8-10's close re-resolves stale keys and lands the user back on the theme they just replaced.

**Solution**: An `Enter` arm in `updateThemePanel` driving a new `commitConstant` helper that calls Phase 6 task 6-7's `ThemePersister.CommitTheme`, then mirrors §8.2's mutual-exclusion write rule on the model's own `theme.RawKeys` in memory — leaving the applied theme, the cursor and the panel's open state untouched.

**Outcome**: `Enter` on `nord` writes `"theme": "nord"` and clears both slots in one atomic prefs write; the panel stays open; the composed frame is byte-identical across the keypress; and a subsequent `Esc` resolves to `nord` rather than to whatever was persisted at open.

**Do**:
- Add `internal/tui/theme_panel_commit.go` declaring the panel's commit helpers, and wire an `Enter` arm into task 8-7's `updateThemePanel` ahead of its swallow-everything default:
  ```go
  // commitConstant writes theme = slug and clears both slots (§8.2 mutual
  // exclusion, performed on disk by prefs.SaveTheme in ONE atomic write) and
  // mirrors that same rule on the model's construction-time raw keys.
  func (m *Model) commitConstant(slug string) error
  ```
- **Take the target from the panel's selected row**, never from `m.themeKeys`: `p.list.SelectedItem().(themeRowItem).Row`. Guard defensively — if the row is not `Selectable()` or its `Slug` is empty, write **nothing** and return nil. Comment that this is structurally unreachable (arrows skip unselectable rows, task 8-9; the open-time anchor lands selectable, task 8-8) and is a guard rather than a live path.
- **Call the seam**: `m.themePersister.CommitTheme(slug)`. A **nil** persister (a fixture or `capturetool` model, per task 6-7) writes nowhere, mutates nothing and is **inert, not failed** — it is the absence of a writer rather than a failed write, so it raises no message and no outstanding-failure state (task 9-7). Follow the `modePersister` nil-guard precedent and state the distinction in-source so a later reader does not route it into the failure path.
- **On success mutate the raw keys in memory**: `m.themeKeys = theme.RawKeys{Theme: slug}` — a constant clears both slots, mirroring the rule `SaveTheme` just applied on disk. The mutation is applied to the **construction-time snapshot** the panel has been using; do not read back anything the persister's RMW saw. Comment both halves (why mirror rather than re-read, and why the snapshot rather than the merged bytes).
- **On error mutate nothing** — no key change, no recompute, and the `●` therefore cannot move. Return the error up; task 9-7 owns the message, the outstanding state and the fact that the theme stays applied in memory.
- **Assert the three negatives explicitly**: `Enter` does **not** close the panel, does **not** call `Model.ApplyTheme` (the previewed theme is unchanged — a commit is a write, not a navigation), and performs **no** directory read, no prefs read, no enumeration and no tmux write. Each is a named test, not a comment.
- **No confirm on this path.** `Enter` over a pair clears both slots without asking: it visibly does what it says, and the theme is already previewing behind the panel. The asymmetry with `d`/`l` (task 9-5) is the point — the confirm guards the case where the *resolved* theme changes as a side effect of a write the user was told is inert.
- Leave the row/badge recompute to task 9-2 — after this task a successful commit persists correctly while the panel's badges are momentarily stale, which is the deliberate intermediate state the phase's task order accepts.

**Acceptance Criteria**:
- [ ] `Enter` on a valid row calls `CommitTheme` with **that row's** slug — asserted against a fake persister recording its arguments — even when a different slug is persisted.
- [ ] The panel is still open after `Enter`, on both pages.
- [ ] `m.themeKeys` becomes `{Theme: slug, Light: "", Dark: ""}` after a successful commit.
- [ ] `Model.ApplyTheme` is not called and the composed frame is byte-identical before and after the keypress.
- [ ] `Esc` after a commit resolves the **newly** persisted state through task 8-10's close path (a commit of `nord` followed by `Esc` renders `nord`, not the theme persisted at open).
- [ ] A failed commit leaves `m.themeKeys` untouched (no key mutation, so the `●` cannot move) and returns the error.
- [ ] A **nil** persister writes nothing, mutates nothing, does not panic and raises no failure state.
- [ ] Committing the same slug twice is idempotent — two identical `CommitTheme` calls, identical resulting keys, no error and no special retry affordance.
- [ ] `Enter` writes nothing beyond the prefs call: no directory read, no prefs read, no enumeration, no tmux server option, no file.
- [ ] `Enter` on a non-selectable row (constructed directly, bypassing the arrow skip) writes nothing.
- [ ] `Enter` raises no confirm under any setting shape, including an adaptive pair.

**Tests**:
- `"it commits the cursor's slug, not the persisted one"` — `TestPanelEnter_CommitsTheCursorSlug`
- `"it keeps the panel open"` — `TestPanelEnter_DoesNotClose`
- `"it clears both slots in memory"` — `TestPanelEnter_MutatesRawKeysToAConstant`
- `"it does not re-theme"` — `TestPanelEnter_IsAWriteNotANavigation` (frame byte-compared)
- `"it makes Esc resolve to the new constant"` — `TestPanelEnter_EscResolvesTheCommittedTheme`
- `"it mutates nothing on a failed write"` — `TestPanelEnter_FailedWriteLeavesKeysAlone`
- `"it tolerates a nil persister"` — `TestPanelEnter_NilPersisterIsInert`
- `"it is idempotent"` — `TestPanelEnter_RepeatCommitIsIdempotent`
- `"it reads nothing and writes nothing else"` — `TestPanelEnter_NoOtherIO`
- `"it refuses a non-selectable row"` — `TestPanelEnter_UnselectableRowWritesNothing`
- `"it raises no confirm over a pair"` — `TestPanelEnter_NoConfirmOverAPair`

**Edge Cases**:
- `Enter` deliberately **does not close** — a user who had just set both slots would otherwise press `Enter` to exit and wipe the pair they just built, so `Esc` stays the only way out, there is no dual-purpose key, and the pair flow needs no special case.
- The accepted cost is that "pick one and go" is **two keys rather than one**.
- The commit writes the **cursor's** slug, never the persisted one.
- A commit is a **write, not a navigation** — `ApplyTheme` is not called and the panel keeps previewing whatever the cursor is on, so committing to a non-active slot changes nothing on screen.
- Mutual exclusion is Phase 6 task 6-2's (`SaveTheme` clears both slots in one atomic write); the panel **mirrors** it on its in-memory raw keys rather than re-implementing it.
- The in-memory mutation applies to the **construction-time snapshot**, never the bytes the RMW just read.
- `Enter` over a pair needs **no confirm** — it visibly does what it says and the theme is already previewing behind the panel; that asymmetry with `d`/`l` is the point.
- Committing the same slug twice is idempotent, which is what makes "a commit is always re-attemptable" free with no retry affordance and no state to clear first.
- A commit key on a non-selectable row must write nothing — structurally unreachable, so the guard is defensive.
- `Esc` after a commit resolves the **newly** persisted state through task 8-10's single close path, not what was rendering at open.
- Nothing else is written — no tmux option, no file, no directory read.
- A fixture / `capturetool` model carries a **nil** persister, so a commit during a capture writes nowhere and must not panic.

**Context**:
> §9.2: "`Enter` — **Commits a constant** — writes `theme = <selection>`, clears both slots | **stays open**… **`Enter` does not close.** If it did, a user who had just set both slots would press `Enter` to exit and thereby commit a constant, wiping the pair they just built. `Esc` is the only way out — one exit key, no dual-purpose keys, and the pair flow needs no special case. **Cost accepted:** the common case ('pick one and go') is two keys rather than one."
> §9.2: "**Committing to a non-active slot changes nothing on screen.**… A commit is a **write, not a navigation** — the panel keeps previewing whatever the cursor is on; the display resolves from persisted state only on close."
> §9.2: "**The recompute uses the construction-time snapshot plus this instance's own mutation — never the merged bytes the RMW just read.** The commit's read-modify-write (§8.9) necessarily has another instance's writes in hand, and re-deriving from them would make badges and rows jump to another instance's choices at the moment the user presses a key."
> §8.2: "**Mutual exclusion is enforced on write.** Committing a constant clears both slots; assigning a slot clears the constant. Whichever was set last wins."
> §9.13: "**A commit is always re-attemptable.** The commit keys are unconditional writes (§9.2), so pressing `d`/`l`/`Enter` again simply retries — no special retry affordance, and no state to clear first."
> Phase 6 task 6-7 declared `ThemePersister{CommitTheme(slug string) error; CommitThemeSlot(slug string, slot prefs.ThemeSlot) error}`, owned by `cmd`, which is the single emission site for `theme: commit failed` and **returns the error** precisely so this phase can report it.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.2, §8.2, §8.4, §9.13, §8.9

## theming-system-9-2

### Task 9.2: The post-commit recompute — rows, order, badges and the identity-anchored cursor

**Problem**: A commit changes what `prefs.json` says, and the panel is a rendering of exactly that — so leaving the rows alone after a write makes the panel lie about the state the user just created. Badges obviously move, but the row **set** changes too, in both directions: `Enter` clears both slots, so a `not found` or charset-rejected row that existed *only* because a slot named it loses its reason to exist and must **disappear**; `d`/`l` on a constant makes the other slot live, so a row for a slug with no file and no built-in must **appear** — the open-time union never minted one, because a `theme`-wins file's slots are not read at all (§8.2). Three traps sit on that recompute. Re-deriving from the **merged bytes** the commit's RMW just read would import another instance's writes at the moment the user presses a key — the cross-instance sync §8.4 declines, arrived at through the write path instead of the open path. Re-reading the **directory** would break §5.8's pin (enumeration belongs to panel open; a commit changes prefs, not the directory) and would mint a third parse that can disagree with the row on screen. And anchoring the cursor to an **index** silently breaks §9.2's invariant the moment a row is inserted above it: the screen keeps previewing one theme while the cursor sits on another.

**Solution**: A `recomputeThemePanel` step run after every **successful** commit — `Reassemble` the union from the retained enumeration and this instance's mutated keys, re-resolve for badges against that same enumeration, refresh the panel's list items in place, and re-anchor the cursor by the previewed row's **identity** using task 8-8's anchor helper.

**Outcome**: After a commit the panel shows exactly the state this instance just created — rows added or removed, re-sorted into their alphabetical place, badges re-derived (a virgin install's two slot badges visibly collapsing to one bare `●` on `Enter`) — with the cursor still on the row that is painting the screen, no directory read and no theme change.

**Do**:
- Add `func (m *Model) recomputeThemePanel()` to `internal/tui/theme_panel_commit.go`, called by every successful commit path (`commitConstant`, task 9-3's `commitSlot`, task 9-5's confirmed commit) and by nothing else — a **failed** write must not recompute (task 9-7), because the `●` would move and the marker means "what is persisted".
- **Steps, in this order**:
  1. Capture the **identity** of the currently selected row before anything changes — the same key task 8-8's anchor helper matches on (slug where one exists, else filename, else the persisted string).
  2. `union := m.themeEnumerator.Reassemble(p.enumeration, m.themeKeys)` — the retained enumeration plus this instance's **mutated** raw keys. No fresh directory read, no fresh prefs read, and nothing from the persister's RMW.
  3. `res, err := m.themeEnumerator.Resolve(p.enumeration, theme.ResolveSetting(...))` for the **badges** only — `theme.Badges(res.Slots)` (task 8-3). The re-resolution never selects a new active member and never calls `ApplyTheme`. On a **non-nil error** take task 8-8's degrade policy verbatim — the one policy governing all three panel call sites — and **keep the existing badge map** rather than deriving from a zero `Resolution`: `theme.Badges` returns an empty map for an empty slice, so a discarded error would wipe every `●` off the panel at the exact moment the user committed one, the marker lying in the direction §9.13's "a failed commit does not move the `●`" rule exists to forbid. Steps 2, 4 and 5 still run on that path — they read the mutated keys and the retained enumeration, not the resolution — so the rows still re-derive, re-sort and re-anchor.
  4. Refresh the panel's list **items** (`p.list.SetItems(...)`) from the new rows with task 8-4's item type; do **not** construct a new `list.Model`. Its `bubbles/list`-owned styles stay **re-pointed** by task 8-9's restyle path rather than reassigned.
  5. Re-anchor the cursor to the captured identity through task 8-8's helper, inheriting its clamp-to-first-selectable degradation. Comment that an index anchor breaks §9.2's invariant the moment step 2 inserts a row above the cursor.
- **Rely on `Reassemble` for ordering** — task 8-2 sorts inside the assembler, so an inserted row lands in its alphabetical place with no second comparator here and no caller-side sort.
- **State the accepted residue in-source**: after a concurrent commit elsewhere this panel's `●` for the *other* instance's slot shows what this instance knows rather than what is on disk, until relaunch. That is the same per-instance staleness §8.9 already accepts for every prefs field under last-write-wins, and it is confined to a slot the user is not acting on.
- **Emission**: this path emits nothing. Task 8-10 pinned that the shared retained-enumeration resolver emits no `theme: loaded`; `theme: fallback applied` may still fire from it and is deduplicated per process on `slug`+`reason`, so a persistently broken persisted slug does not produce a WARN per keypress. Task 9-6 wires the commit-time `theme: loaded` onto the commit entry point deliberately, not onto this body.
- **A no-change commit still recomputes** — committing the slug that is already the constant must produce an identical row set, identical badges and an identical cursor index, so the recompute is provably idempotent rather than merely usually harmless.

**Acceptance Criteria**:
- [ ] `Enter` on a setting whose slots named a nonexistent slug **removes** that `not found` row from the list.
- [ ] A slot commit that makes an unresolvable opposite slot live **adds** its row (task 9-3/9-5 drive it; this task provides the recompute that renders it).
- [ ] Rows are re-sorted by task 8-2's comparator — an inserted row appears in its alphabetical position, not appended.
- [ ] Badges re-derive: a virgin install (`prefs.json` absent → two shipped-default slot badges) shows exactly one bare `●` on the committed slug after `Enter`, and no `● light` / `● dark` anywhere.
- [ ] The recompute performs **no** directory read (asserted with the themes directory removed after the panel opened) and **no** prefs read.
- [ ] The recompute never calls `ApplyTheme` — the previewed theme and the composed frame's colours are unchanged across the commit.
- [ ] The cursor is re-anchored by identity: a commit that inserts a row **above** the cursor leaves the cursor on the same theme, previewing the same palette.
- [ ] When the previewed row's identity has disappeared from the recomputed union, the cursor clamps to the first selectable row with no panic.
- [ ] A commit of the slug already persisted produces a byte-identical row set, badge map and cursor index.
- [ ] A `Resolve` returning task 5-6's fatal during a recompute leaves the **existing** badge map in place — driven through the seam with an error-returning fake — while the rows still re-derive, re-sort and re-anchor from the mutated keys.
- [ ] A failed commit does **not** recompute (asserted by a fake persister returning an error: rows and badges unchanged).
- [ ] The panel's list instance is the same object after a recompute (items replaced, model not rebuilt), and its pagination dots still render in the previewed theme.

**Tests**:
- `"it removes a row whose only reason to exist was a cleared slot"` — `TestPanelRecompute_RowDisappearsOnConstantCommit`
- `"it inserts a row for a newly-live slot"` — `TestPanelRecompute_RowAppearsForNewlyLiveSlot`
- `"it re-sorts an inserted row into place"` — `TestPanelRecompute_ReSortsThroughTheComparator`
- `"it collapses two slot badges to one bare dot"` — `TestPanelRecompute_VirginInstallBadgeCollapse`
- `"it reads no directory and no prefs"` — `TestPanelRecompute_ReadsNothing`
- `"it never re-themes"` — `TestPanelRecompute_DoesNotApplyTheme`
- `"it anchors the cursor by identity"` — `TestPanelRecompute_CursorAnchoredByIdentity` (row inserted above)
- `"it degrades when the previewed identity is gone"` — `TestPanelRecompute_CursorClampsOnMissingIdentity`
- `"it is idempotent for an unchanged commit"` — `TestPanelRecompute_NoChangeCommitIsStable`
- `"it keeps the badges when the re-resolution errors"` — `TestPanelRecompute_ResolveErrorKeepsBadges`
- `"it does not run on a failed write"` — `TestPanelRecompute_SkippedOnFailedCommit`
- `"it keeps the list instance and its re-pointed styles"` — `TestPanelRecompute_ItemsReplacedNotRebuilt`

**Edge Cases**:
- Badges obviously move, but a commit can **add or remove rows outright** — `Enter` clears both slots, so a `not found` or charset-rejected row that existed *only* because a slot named it **disappears**, while `d`/`l` on a constant makes the other slot live so a row for a slug with no file and no built-in **appears** (the open-time union never minted one, because a `theme`-wins file's slots are not read at all).
- The recompute uses the **construction-time snapshot plus this instance's own mutation**, never the merged bytes the RMW just read — re-deriving from those would make badges and rows jump to another instance's choices at the moment the user presses a key.
- Accepted residue: after a concurrent commit elsewhere this panel's `●` for the *other* instance's slot shows what this instance knows until relaunch — the same per-instance staleness every prefs field already carries, confined to a slot the user is not acting on.
- The recompute calls task 8-1's `Reassemble` with the mutated keys and performs **no fresh directory read**, since §5.8 pins enumeration to panel open and a commit changes prefs, not the directory.
- Rows re-sort through task 8-2's total comparator so an inserted row lands in its alphabetical place.
- Badges re-derive through task 8-3's table off a re-resolution against the **retained** enumeration.
- The badge re-resolution takes task 8-8's degrade policy on a non-nil error and the **existing** badge map stands. Discarding the error and deriving from a zero `Resolution` returns an empty badge map and wipes every `●` at the moment the user committed one; task 8-8 names this recompute as one of the three call sites that single policy governs.
- The cursor **re-anchors to the previewed theme's identity, never to its index** — an index anchor silently breaks §9.2's invariant the moment a row is inserted above the cursor, leaving the screen previewing one theme while the cursor sits on another.
- The recompute must not call `ApplyTheme`; the re-resolution is for badges (and task 9-6's slot load) only, never for selecting a new active member.
- The panel's list is rebuilt from the new rows while its `bubbles/list`-owned styles stay **re-pointed** rather than reassigned.
- A commit that changes nothing (the same slug again) still recomputes and must produce an identical row set and an identical cursor.

**Context**:
> §9.2: "**A successful commit recomputes the panel's full row set, not just the badges.** Badges obviously move — §9.13's 'a failed commit does not move the `●`' only means anything because a successful one does — but a commit can add or remove rows outright: `Enter` clears both slots, so a `not found` or charset-rejected row that existed *only* because a slot named it loses its reason to exist and **disappears**. `d`/`l` on a constant makes the other slot live (§8.2). If that slot names a slug with no file and no built-in, §9.4 requires it to have a row… So a row **appears**."
> §9.2: "So a commit re-derives the union (§9.4), re-sorts it (§9.5), and **re-anchors the cursor to the previewed theme's identity, never to its index**. Anchoring to an index would silently break §9.2's invariant the moment a row is inserted above the cursor… The directory is *not* re-enumerated — §5.8 pins that to panel open, and a commit changes prefs, not the directory."
> §13.3: "§9.2's post-commit re-derivation calls the same assembly with changed prefs state and no fresh directory read, which is why it is one entry point rather than logic inlined in the panel."
> Task 8-1 declared `Reassemble(e Enumeration, keys RawKeys) Union` as a pure function that sorts inside the assembler and emits nothing; task 8-8 added `Resolve` (`ResolveNominationFrom`) over the retained enumeration; task 8-10 pinned that the shared resolver emits `theme: fallback applied` (deduped) and **never** `theme: loaded`.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.2, §9.4, §9.5, §8.2, §8.4, §8.9, §5.8

## theming-system-9-3

### Task 9.3: `d` / `l` commit a slot under an adaptive setting

**Problem**: The panel's genuinely novel half — §9.14 records that assigning a theme to a light/dark slot *from inside a picker* was found in no surveyed tool — still does nothing. Two properties make it more than a copy of `Enter`. A slot save leaves the **other** slot untouched, which is what makes `d` then `l` on one row produce the `● both` badge in two keypresses; that is a likely path, because it is where a user lands wanting "this theme everywhere" without realising `Enter` is the idiom for it. And a slot commit is the sharpest case of "a commit is a write, not a navigation": previewing a **light** theme in a dark terminal and pressing `l` writes the light slot while the resolved-active theme is still the dark slot, so absolutely nothing changes on screen and the only feedback is the badge moving. The third property is a deliberate **hole**: `d`/`l` while a constant is set must not commit here at all — a silent constant-clear is exactly the loss task 9-5's confirm exists to prevent, so the interim behaviour is inert rather than a direct write, and the phase's task order exists so that silent clear is never reachable in an intermediate state.

**Solution**: `d` and `l` arms in `updateThemePanel` driving a `commitSlot` helper over Phase 6 task 6-7's `ThemePersister.CommitThemeSlot` with the **typed** `prefs.ThemeSlot`, followed by task 9-2's recompute — with the constant-set case routed to a no-op placeholder that task 9-5 replaces with the confirm.

**Outcome**: On an adaptive setting, `d` writes `theme_dark = <cursor slug>`, `l` writes `theme_light = <cursor slug>`, each clearing the constant in the same atomic write and leaving the other slot alone; the panel stays open, the frame is unchanged, and the badges move (via task 9-2) — including the two-keypress collapse to a single `● both` row.

**Do**:
- Add `func (m *Model) commitSlot(slug string, slot prefs.ThemeSlot) error` beside `commitConstant`, and wire the `d` / `l` arms into `updateThemePanel` ahead of the swallow-everything default. Take the target row exactly as task 9-1 does (selected row, defensive `Selectable()` guard).
- **Gate on the setting shape first**: when `m.themeKeys.Theme != ""` (a constant is set, per §8.2's `theme`-wins rule) the key must write **nothing** in this task — leave a single named seam (`raiseSlotConfirm`, a no-op returning the model unchanged) that task 9-5 fills with the confirm. Comment that the interim inertness is deliberate: a direct commit here would silently clear the constant, which is precisely the loss the confirm exists to prevent.
- **Otherwise commit**: `m.themePersister.CommitThemeSlot(slug, slot)` — one atomic write that sets the slot and clears the constant (Phase 6 task 6-2), so no partial state is reachable and the panel re-implements no merge. The slot is the **existing typed** `prefs.ThemeSlot` value threaded through the seam, never a caller-supplied key name, so no path can mint a third slot.
- **Mirror the write in memory**: set the committed slot on `m.themeKeys` and clear `Theme`, leaving the **other** slot's raw value exactly as it was — that untouched-other-slot rule is what produces `● both` when the same row is committed to both slots.
- **Then recompute** through task 9-2 (`recomputeThemePanel`) on success only; on error mutate nothing and return the error for task 9-7.
- **Assert the negatives**: the panel stays open, `ApplyTheme` is not called, the previewed theme and the resolved-active theme are both unchanged, and no directory or prefs read happens.
- **Clearing is writing the empty string**, which `omitempty` renders as **key-absent** on disk (Phase 6 task 6-2) — so "an unset slot holds the shipped default" continues to hold and a hand-edited file stays clean. Do not special-case an already-empty constant.
- Do not add a second badge derivation here — the badge movement and the `● both` collapse come from task 9-2's recompute, so there is exactly one derivation site.

**Acceptance Criteria**:
- [ ] On an adaptive setting, `d` calls `CommitThemeSlot(cursorSlug, prefs.SlotDark)` and `l` calls it with `prefs.SlotLight` — asserted against a recording fake.
- [ ] A slot commit leaves the **other** raw slot key untouched, and `d` then `l` on the same row yields a single `● both` badge row after the recompute.
- [ ] The constant is cleared in the same write (one `CommitThemeSlot` call, no second call) and cleared in memory.
- [ ] With a constant set, `d` and `l` write **nothing** in this task and leave the model unchanged (the confirm seam is a no-op until task 9-5).
- [ ] The panel stays open and the previewed theme is unchanged: previewing a light theme in a dark terminal and pressing `l` leaves the composed frame byte-identical.
- [ ] Committing a slot to the row already carrying that slot's badge is idempotent — same call, same resulting keys, no error.
- [ ] The slot argument is the typed `prefs.ThemeSlot`; no code path constructs a slot from a string.
- [ ] A failed slot commit mutates nothing and returns the error.
- [ ] A nil persister is inert (no write, no mutation, no failure state), as in task 9-1.
- [ ] No directory read, no prefs read, no enumeration, no tmux write on either key.
- [ ] `Enter` after a slot commit clears both slots again and still raises no confirm.

**Tests**:
- `"it writes the dark slot"` — `TestPanelSlotCommit_DarkWritesTheDarkSlot`
- `"it writes the light slot"` — `TestPanelSlotCommit_LightWritesTheLightSlot`
- `"it leaves the other slot untouched"` — `TestPanelSlotCommit_OtherSlotSurvives`
- `"it produces the both badge in two keypresses"` — `TestPanelSlotCommit_DThenLYieldsBoth`
- `"it clears the constant in the same write"` — `TestPanelSlotCommit_ClearsTheConstantAtomically`
- `"it writes nothing while a constant is set"` — `TestPanelSlotCommit_InertOverAConstant`
- `"it changes nothing on screen"` — `TestPanelSlotCommit_NonActiveSlotIsVisuallyInert`
- `"it is idempotent"` — `TestPanelSlotCommit_RepeatIsIdempotent`
- `"it cannot mint a third slot"` — `TestPanelSlotCommit_TypedSlotOnly`
- `"it mutates nothing on failure"` — `TestPanelSlotCommit_FailedWriteLeavesKeysAlone`
- `"it tolerates a nil persister"` — `TestPanelSlotCommit_NilPersisterIsInert`
- `"it needs no confirm for a following Enter"` — `TestPanelSlotCommit_EnterAfterSlotNeedsNoConfirm`

**Edge Cases**:
- `d`/`l` write the slot and clear the constant in **one atomic write** through `SaveThemeSlot`, so no partial state is reachable and the panel re-implements no merge.
- A slot save leaves the **other** slot untouched, which is what makes `d` then `l` on one row produce `● both` in two keypresses — the likely path for a user wanting "this theme everywhere" without realising `Enter` is the idiom for it.
- The panel **stays open** and the previewed theme does not change — previewing a light theme in a dark terminal and pressing `l` writes the light slot while the resolved-active theme is still the dark slot, because a commit is a write, not a navigation.
- The slot is the **typed** value (light/dark) carried through from `prefs`, never a caller-supplied key name, so the panel cannot mint a third slot.
- `d`/`l` **while a constant is set** is deferred to task 9-5's confirm and must write nothing here — a silent constant-clear is the exact loss the confirm exists to prevent, so the interim behaviour is inert rather than a direct commit.
- Committing a slot to the row already carrying that slot's badge is idempotent.
- Clearing a key is writing the empty string, which `omitempty` renders as **key-absent**, so "an unset slot holds the shipped default" continues to hold on disk.
- The badge movement and the `● both` collapse come from task 9-2's recompute rather than a second derivation here.
- `Enter` after a slot commit clears both slots again and still needs no confirm.

**Context**:
> §9.2: "`d` — **Commits the dark slot** — writes `theme_dark = <selection>`, clears the constant | stays open. `l` — **Commits the light slot** — writes `theme_light = <selection>`, clears the constant | stays open."
> §9.5: "**When both slots name the same slug, that one row carries `● both`.** This is reachable in two keypresses (`d` then `l` on one row) and is a likely path — it is where a user lands wanting 'this theme everywhere' without realising `Enter` is the idiom for it."
> §9.2: "**Committing to a non-active slot changes nothing on screen.** Previewing a light theme in a dark terminal and pressing `l` writes the light slot, but the resolved-active theme is still the dark slot."
> §9.2: "**Assigning a slot while a constant is set asks for confirmation first.** This is the one place a keypress described as inert can silently cost the user a setting they chose."
> Phase 6 task 6-2 pinned that `SaveThemeSlot` writes the slot and clears the constant in **one** atomic write, leaves the other slot untouched, and renders a cleared key as absent via `omitempty`.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.2, §9.5, §8.2, §8.3, §9.14

## theming-system-9-4

### Task 9.4: The panel message slot and the confirm's nested keymap scope

**Problem**: Phase 8 left the message slot budgeted and permanently empty, and both of its contenders are commit-path states that now exist or are about to: the slot-from-constant confirm (task 9-5) and the failed-commit line (task 9-7). The slot is a **single-slot arbiter with exactly two contenders**, and its exclusion is not something to resolve with an invented precedence rule — a confirm resolves *before* any write happens, so the two can never be live at once and the correct implementation asserts that rather than ranking them. The confirm also cannot advertise the standing footer while it is live: `⏎ set theme` / `d set as dark` / `l set as light` / `esc close` lists four keys of which **none** would act, and §14.3 is firm that advertising a key that will not act is the dead end a proactive block exists to prevent. That forces the confirm's keys into the descriptor as a **nested scope** — otherwise its substituted footer becomes exactly the second, unguarded place a key label can go stale that `keymap_dispatch_guard_test` exists to prevent, and task 9-10 would have nothing to probe against.

**Solution**: A two-contender message value on the panel with a pinned renderer for each, plus `themePanelConfirmKeymap()` as a nested scope beneath the panel scope, substituted into task 8-5's already-parameterised footer renderer while the confirm is live.

**Outcome**: The panel's message slot renders exactly one of two pinned messages — `clear constant <slug>?  y / n` in `text.secondary` with no band, or `⚠ couldn't save theme` with `⚠` and text in `accent.attention` and no band — the footer reads `y confirm` / `n cancel` while the confirm is live and reverts when it resolves, and the slot still costs a row only when it is non-empty.

**Do**:
- **Replace task 8-6's `message string` field** with a two-contender value in `internal/tui/theme_panel.go`:
  ```go
  type themePanelMessageKind int // themeMessageNone, themeMessageConfirm, themeMessageCommitFailed

  type themePanelMessage struct {
      Kind themePanelMessageKind
      Slug string // the persisted CONSTANT being cleared — confirm only
  }
  ```
  The message carries **no free text**: each contender renders its own pinned copy, so no call site can paraphrase it and §14A stays single-sourced. Assert the exclusion directly — a helper that sets one contender clears the other, and a test proves both can never be non-`None` at once (a confirm resolves before any write happens, so the state is unreachable by construction).
- **Pin the copy** as constants beside the renderer, verbatim from §14A: the confirm is `clear constant <slug>?  y / n` (two spaces before `y`, exactly as pinned) with the slug truncated by §9.5's rule when it does not fit; the failed-commit line is `⚠ couldn't save theme`.
- **Render per §9.1's token table** in `renderThemePanelMessage`: the confirm in **`text.secondary`, no band**; the failed-commit line's `⚠` and text in **`accent.attention`**, and explicitly **no `bg.attention` band** — the warning band is a full-width main-screen flash treatment and would read as heavy inside a 24–30 column panel. Both are glyph-backed and canvas-painted; `colourless` drops hue exactly as the rest of the panel does.
- **The confirm reads the raw keys, not the nomination**: `m.themeKeys.Theme` is the persisted constant, which may be the very slug that failed to load, so the message must name it even when nothing loaded from it.
- **Degrade per dimension** (task 8-11 pinned the rule; this task supplies the content): at the minimum **height** the message truncates to one line; otherwise it wraps (it is not a list delegate, so wrapping costs pagination nothing). The slot's height is **measured** off the rendered block exactly as task 8-6 established, so a wrapped message shrinks the list by two rows rather than one.
- **Add `themePanelConfirmKeymap()`** to `internal/tui/keymap.go`, immediately beneath `themePanelKeymap()`, as a **nested confirm scope**:
  ```go
  func themePanelConfirmKeymap() []keymapEntry {
      return []keymapEntry{
          {Key: "y", Action: "confirm", HelpAction: "Clear the constant and set the slot", Core: true},
          {Key: "n", Action: "cancel",  HelpAction: "Keep the constant", Core: true},
      }
  }
  ```
  Document that §9.12's "all six" is the **panel scope's own membership** — the confirm is a *second* scope, not a sixth-plus-four list — that task 9-10's guard consumes it (including the uppercase `Y`/`N` dispatch), and that it has no `RightAligned` and no `?` entry for the same reasons the panel scope has none.
- **Substitute, do not fork**: the panel's render path passes `themePanelConfirmKeymap()` to task 8-5's `renderThemePanelFooter(entries, …)` while the confirm is live and `themePanelKeymap()` otherwise — one renderer, one height helper (`themePanelFooterHeight(entries)`), no second footer implementation and no special case.
- **Keep the floor computed from the standing scope.** Task 8-11's `themePanelMinHeight(entries, dirUnusable)` continues to take `themePanelKeymap()`, so the floor is the *taller* footer plus one message row; the confirm's two-row footer is strictly shorter, so a confirm can never need more room than the floor already guarantees and no row is added to the floor. Record that reasoning in-source, because it is the non-obvious half of "the slot's height feeds the floor arithmetic unchanged".

**Acceptance Criteria**:
- [ ] The confirm renders exactly `clear constant nord?  y / n` (byte-compared against the §14A string, including the double space) in `text.secondary`, with **no** background tint anywhere on the row.
- [ ] The failed-commit line renders exactly `⚠ couldn't save theme` with the `⚠` and the text in `accent.attention` and **no** `bg.attention` band.
- [ ] Setting either contender clears the other; a test asserts both cannot be live simultaneously.
- [ ] With `themeMessageNone` the slot contributes zero rows and the list body is one row taller (task 8-6's unreserved-when-empty rule is unchanged).
- [ ] A message that wraps to two rows shrinks the list body by two — the slot's height is measured, never assumed.
- [ ] At the minimum panel height the message renders on exactly **one** line (truncated), while at the minimum width above the floor the same message may occupy two.
- [ ] A long persisted constant slug is truncated inside the confirm copy by §9.5's rule, and the row still renders at the minimum width.
- [ ] The confirm names `m.themeKeys.Theme` even when that slug failed to load (asserted with a persisted constant that resolves to a fallback).
- [ ] While the confirm is live the footer renders exactly two rows — `y confirm`, `n cancel` — and reverts to the four standing rows when it resolves, through the *same* renderer.
- [ ] `themePanelFooterHeight` reports the substituted footer's height, and the panel's layout absorbs the difference in the list body.
- [ ] `themePanelMinHeight` is unchanged by the confirm (still computed from the standing scope) and no row is added to the floor.
- [ ] The confirm scope carries no `RightAligned` and no `?` entry, and does not leak into either page footer or either help body.
- [ ] Under `colourless` both messages drop hue and keep their glyphs.

**Tests**:
- `"it renders the pinned confirm copy"` — `TestPanelMessage_ConfirmPinnedCopy`
- `"it renders the pinned failed-commit copy"` — `TestPanelMessage_CommitFailedPinnedCopy`
- `"it keeps the two contenders mutually exclusive"` — `TestPanelMessage_SingleSlotExclusion`
- `"it costs no row when empty"` — `TestPanelMessage_UnreservedWhenEmpty`
- `"it measures a wrapped message"` — `TestPanelMessage_WrappedMessageCostsTwoRows`
- `"it truncates at the minimum height"` — `TestPanelMessage_TruncatesAtFloorHeight`
- `"it truncates a long constant slug"` — `TestPanelMessage_ConfirmSlugTruncation`
- `"it names the persisted constant even when it failed to load"` — `TestPanelMessage_ConfirmReadsRawKeys`
- `"it uses text.secondary with no band"` — `TestPanelMessage_ConfirmTokens`
- `"it uses accent.attention with no band"` — `TestPanelMessage_CommitFailedTokens`
- `"it substitutes the confirm footer"` — `TestPanelFooter_ConfirmScopeSubstitution`
- `"it restores the standing footer on resolve"` — `TestPanelFooter_RevertsAfterConfirm`
- `"it leaves the height floor unchanged"` — `TestPanelMessage_FloorUsesStandingScope`
- `"its scope does not leak"` — `TestThemeConfirmKeymap_DoesNotLeakIntoPageSurfaces`
- `"it drops hue under colourless"` — `TestPanelMessage_Colourless`

**Edge Cases**:
- The slot is a **single-slot arbiter with exactly two contenders** that can never be live at once, because a confirm resolves before any write happens — the exclusion is asserted rather than resolved by inventing a precedence rule.
- Phase 8 left the slot budgeted and always empty, so this task fills it without changing the height accounting — it appears and the list shrinks by one, the way the main screen's notice band recomputes list height.
- The confirm copy is pinned verbatim — `clear constant <slug>?  y / n` — with the slug truncated by §9.5's rule if needed.
- It renders in **`text.secondary` with no band**, because the warning band is a full-width main-screen flash treatment and would read as heavy inside a 24–30 column panel.
- At the minimum **width** the message may wrap to two rows (it is not a list delegate, so wrapping costs pagination nothing) while at the minimum **height** it truncates to one line instead, since §9.8's floor counts exactly one message row and both contenders are non-suppressible — truncation degrades the message rather than the row the user is being asked about.
- The **confirm scope** lives in the descriptor nested beneath the panel scope, so its footer renders from the descriptor exactly as the panel's does and task 9-10's guard can cover `y`/`Y`/`n`/`N`.
- §9.12's "all six" is the panel scope's **own** membership — the confirm is a second scope, not a sixth-plus-four list.
- The footer **swaps to `y confirm` / `n cancel` while the confirm is live and swaps back when it resolves**, because the standing footer advertises four keys of which none would act.
- Task 8-5's renderer already takes its entries as a parameter, so the substitution needs no second renderer and no special case.
- The confirm is **inline, not a modal** — the panel does not blank, and stacking a modal over a non-blanking overlay is the shape §9.6 rejects.
- The confirm renders the **persisted** constant, which may be the slug that failed to load, so it reads the raw keys rather than the nomination.
- The slot's height feeds task 8-11's floor arithmetic unchanged — no row is added to the floor, and the confirm's shorter footer can only free rows.

**Context**:
> §9.1: "**Message slot.** A single-row region directly above the vertical keymap footer, **not reserved when empty**… It is a **single-slot arbiter** with two contenders, which can never be live at once because a confirm resolves before any write happens: 1. The **slot-from-constant confirm** (§9.2). 2. A **failed commit write** (§9.13)… **It does cost a row of vertical budget, so at the minimum *height* the message is truncated to one line rather than wrapped.**"
> §9.1 (token table): "Message slot — confirm → `text.secondary`, no band | Message slot — failed commit → `⚠` and text in `accent.attention`, **no `bg.attention` band** — the warning band is a full-width main-screen flash treatment and would read as heavy inside a 24–30 column panel."
> §9.2: "**The panel footer switches to the confirm's own keys while it is live** — `y confirm` / `n cancel` — and switches back when it resolves. The standing footer advertises four keys of which none would act during a confirm, and §14.3 is firm that advertising a key that will not act is the dead end a proactive block exists to prevent. **The confirm's keys live in the descriptor as a nested confirm scope** under the panel scope (§9.12), so its footer renders from the descriptor like the panel's and `keymap_dispatch_guard_test` covers `y`/`Y`/`n`/`N` too. §9.12's 'all six' is the panel scope's own membership; the confirm is a second scope, not a sixth-plus-four list."
> §14A: "Slot-from-constant confirm → `clear constant <slug>?  y / n` — the slug truncated by §9.5's rule if needed. Failed commit write → `⚠ couldn't save theme`."
> §8.4: "**§14A's confirm renders the persisted constant**, on a path where that constant may be the one that failed to load."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.1, §9.2, §9.12, §9.13, §9.8, §14A

## theming-system-9-5

### Task 9.5: The slot-from-constant confirm's three-input resolution and atomic commit

**Problem**: `d`/`l` are described to the user as inert — they assign a slot, and §9.2 states plainly that committing to a non-active slot changes nothing on screen. On a **constant** that description is false in a way the user cannot see coming: on `"theme": "nord"`, pressing `l` clears the constant, the untouched dark slot falls back to the shipped default, and `Esc` in a dark terminal lands on `tokyo-night` rather than `nord`. That is the one place a keypress the user was told is inert can silently cost them a setting they chose — and task 9-3 deliberately left the path inert rather than committing, precisely so the silent clear is never reachable. Closing it needs more than a prompt: the confirm has to be genuinely **key-exclusive within the panel** (an arrow that moved the cursor mid-question would re-theme the screen while the user is answering about a row that has just stopped being the previewed one), it has to resolve on exactly three inputs, and `Esc` has to mean *cancel the confirm* rather than *close the panel*, because the innermost thing resolves first.

**Solution**: Fill task 9-3's `raiseSlotConfirm` seam — `d`/`l` over a constant raise task 9-4's confirm message and swap the footer; while it is live a dedicated confirm arm at the very top of `updateThemePanel` resolves it on `y`/`Y` (commit), `n`/`N`/`Esc` (cancel) or `Ctrl-C` (quit) and swallows every other key.

**Outcome**: On a constant, `l` asks `clear constant nord?  y / n` with the footer reading `y confirm` / `n cancel`; `y` clears the constant and writes the light slot in one atomic prefs write and recomputes; `n` or `Esc` leaves the panel exactly as it was with nothing written; every other key does nothing; and a forced close cancels it silently.

**Do**:
- **Raise it**: `raiseSlotConfirm(slug string, slot prefs.ThemeSlot)` records the pending slug + typed slot on the panel and sets task 9-4's confirm message with `Slug: m.themeKeys.Theme` (the persisted constant being cleared). Nothing is written at this point.
- **Route it first**: add a confirm arm at the **top** of `updateThemePanel`, ahead of the arrow arm, the commit arms and the `Esc` close, so while the confirm is live it owns input:
  - **`y` / `Y`** — resolve to a commit: `m.themePersister.CommitThemeSlot(pendingSlug, pendingSlot)`, which clears the constant and writes the slot in **one** atomic write (Phase 6 task 6-2). On success mirror it in memory (slot set, `Theme` cleared, other slot untouched), run task 9-6's newly-live opposite-slot load, then task 9-2's recompute; clear the confirm and restore the standing footer either way. On failure hand the error to task 9-7 — the constant is **not** cleared in memory, so the badges still show it.
  - **`n` / `N` / `Esc`** — cancel: clear the confirm, restore the footer, write nothing, and leave the preview, the cursor, the badges and the row set **exactly** as they were. `Esc` must not reach task 8-10's close: the innermost thing resolves first, the same nesting rule the panel already applies over multi-select.
  - **`Ctrl-C`** — quits Portal. It is not a cancel; it stays live everywhere, because swallowing it would take away the exit key inside a settings surface.
- **Swallow everything else** — arrows, `Ctrl+↑/↓`, `Enter`, the other slot key, a *second* `d`/`l` (swallowed, never treated as a re-raise), `t`, `?`, `k`, `x`, `m`, `/`. The confirm persists until one of the three resolves it; nothing has been written, so there is no partial state to leave behind.
- **Wire the forced close**: task 8-11's below-floor resize path silently cancels a live confirm (clear the pending state, no write, no flash of its own) before calling `closeThemePanel`. State in-source why this is called out at all — the confirm is otherwise specified as resolvable only by a keypress.
- **Do not raise it in the reverse direction.** `Enter` over a pair raises no confirm (task 9-1); the confirm guards only the case where the *resolved* theme changes as a side effect of a write the user was told is inert.
- **Handle the hand-edited `theme`-wins file explicitly**: on such a file the confirm names the **constant** being cleared — the change the user initiated — and the stale opposite slot becoming live is then plainly visible in the badges the moment the confirm resolves (task 9-2's recompute renders it, task 9-6 loads it). Do not extend the copy to mention the stale slot.

**Acceptance Criteria**:
- [ ] `d` or `l` with a constant set raises the confirm, writes nothing, and swaps the footer to `y confirm` / `n cancel`.
- [ ] `y` and `Y` both commit: exactly one `CommitThemeSlot` call with the pending slug and typed slot, the constant cleared in the same write, the confirm cleared and the footer restored.
- [ ] `n`, `N` and `Esc` all cancel: no persister call, `m.themeKeys` unchanged, the confirm cleared, the footer restored, and the panel **still open**.
- [ ] `Esc` during a live confirm does **not** close the panel (asserted against the close path's own observable effects — the enumeration is still retained).
- [ ] `Ctrl-C` during a live confirm quits.
- [ ] Every other key is swallowed with the confirm still live: arrows do not move the cursor, `Enter` does not commit, a second `d`/`l` does not re-raise or commit, and the previewed theme is unchanged across all of them.
- [ ] After a cancel the preview, cursor index, badge map and row set are byte-identical to their pre-raise values.
- [ ] After a confirmed commit the badges reflect the new slot and the constant's bare `●` is gone (via task 9-2).
- [ ] A confirmed commit that **fails** leaves the constant in the in-memory keys (badges still show the bare `●`) and routes to task 9-7's report.
- [ ] A forced close (below-floor resize) with a live confirm cancels it silently: nothing written, no confirm-specific flash, and the close takes task 8-10's path exactly.
- [ ] `Enter` over a pair raises no confirm.
- [ ] On a hand-edited file carrying both a constant and slots, the confirm names the constant, and after `y` the stale opposite slot's badge is visible.

**Tests**:
- `"it raises the confirm on a constant"` — `TestSlotConfirm_RaisedByDAndLOverAConstant`
- `"it commits on y and Y"` — `TestSlotConfirm_ConfirmsOnEitherCase`
- `"it cancels on n, N and Esc"` — `TestSlotConfirm_CancelsOnThreeInputs`
- `"it does not close the panel on Esc"` — `TestSlotConfirm_EscCancelsNotCloses`
- `"it keeps Ctrl-C live"` — `TestSlotConfirm_CtrlCQuits`
- `"it swallows every other key"` — `TestSlotConfirm_SwallowsEverythingElse` (table incl. a second `d`/`l`)
- `"it leaves everything untouched on cancel"` — `TestSlotConfirm_CancelIsInert`
- `"it clears the constant and writes the slot atomically"` — `TestSlotConfirm_AtomicConstantClearPlusSlot`
- `"it keeps the constant in memory when the write fails"` — `TestSlotConfirm_FailedCommitKeepsTheConstant`
- `"it is cancelled silently by a forced close"` — `TestSlotConfirm_ForcedCloseCancels`
- `"it is not raised by Enter"` — `TestSlotConfirm_NotRaisedByEnter`
- `"it names the constant on a theme-wins file"` — `TestSlotConfirm_HandEditedFileNamesTheConstant`

**Edge Cases**:
- Raised only by `d`/`l` **while a constant is set** — the one place a keypress described as inert can silently cost the user a setting they chose: on `"theme": "nord"`, pressing `l` clears the constant, the untouched dark slot falls back to the shipped default, and `Esc` in a dark terminal lands on `tokyo-night` rather than `nord`.
- While live it is **key-exclusive within the panel** and resolves on exactly three inputs — `y`/`Y` confirms (constant cleared and slot written in **one** atomic write), `n`/`N`/`Esc` cancels with the panel open and nothing written, `Ctrl-C` quits Portal.
- `Esc` **cancels the confirm rather than closing the panel**, because the innermost thing resolves first — the same nesting rule the panel already applies over multi-select.
- **Every other key is swallowed** — arrows, `Enter`, the other slot key, all of it — and the confirm persists until one of the three resolves it, with nothing written so there is no partial state to leave behind.
- A second `d`/`l` while it is live is swallowed, not treated as a re-raise.
- `Ctrl-C` is **not** a cancel — it stays live everywhere, including inside the confirm, because swallowing it would take away the exit key inside a settings surface.
- A **forced close silently cancels** it (task 8-11's below-floor resize), stated because the confirm is otherwise specified as resolvable only by a keypress and because nothing has been written at that point.
- The reverse direction (`Enter` over a pair) deliberately raises no confirm — the confirm guards the case where the *resolved* theme changes as a side effect of a write the user was told is inert.
- On a hand-edited `theme`-wins file the confirm names the constant being cleared, which is the change the user initiated; the stale opposite slot surfacing is then plainly visible in the badges the moment the confirm resolves.
- Cancelling leaves the preview, the cursor, the badges and the row set exactly as they were.

**Context**:
> §9.2: "**Assigning a slot while a constant is set asks for confirmation first.** This is the one place a keypress described as inert can silently cost the user a setting they chose: on `"theme": "nord"`, pressing `l` clears the constant, the untouched dark slot falls back to the shipped default, and `Esc` in a dark terminal lands on `tokyo-night` rather than `nord`."
> §9.2: "While the confirm is live it is **key-exclusive within the panel** and resolves on exactly three inputs: **`y` or `Y` confirms** — the constant is cleared and the slot written, in one atomic prefs write. **`n`, `N` or `Esc` cancels**, leaving the panel open and nothing written. `Esc` cancels the confirm rather than closing the panel, because the innermost thing resolves first… **`Ctrl-C` quits Portal**, per §9.7. It is not a cancel; it stays live everywhere. **Every other key is swallowed** — arrows, `Enter`, the other slot key, all of it."
> §9.2: "**The reverse direction needs no confirm.**… The asymmetry is the point: the confirm guards the case where the *resolved* theme changes as a side effect of a write the user was told is inert."
> §8.2: "The one visible consequence: on such a file, `d`/`l` clears the constant and the *other* stale hand-edited slot becomes live in the same keypress. The §9.2 confirm names the constant being cleared, which is the change the user initiated; the stale slot surfacing is then plainly visible in the panel's badges the moment the confirm resolves."
> §9.8: "**A live slot-from-constant confirm is silently cancelled** by a forced close. Nothing has been written at that point (§9.2), so there is no partial state to leave behind — but it is stated because the confirm is otherwise specified as resolvable only by a keypress."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.2, §9.7, §9.8, §8.2, §14A

## theming-system-9-6

### Task 9.6: The newly-live opposite slot loads at commit with `theme: loaded`

**Problem**: Converting a constant to adaptive makes a **second** slot live that construction never loaded — a constant nominates one theme, so the other member simply does not exist in the model's nomination at the moment the confirm resolves. This is the one theme load that happens outside construction, and it is why §12.3's cadence column gives `theme: loaded` a commit-time entry at all. Two mistakes are easy here. Loading the slot the user just **assigned** would be redundant and would re-parse a file the panel already holds — the retained enumeration is what keeps arrowing an O(1) restyle, and the assigned slot's parse is in hand. And issuing a **commit-time directory read** for the opposite slot would produce a *third* parse of the same slug — neither construction's nor the panel's — that can disagree with the row the user is looking at, reintroducing exactly the staleness split §5.8 exists to close. There is also a logging trap: task 8-10 deliberately pinned the shared retained-enumeration resolver to emit **no** `theme: loaded`, so if the commit path simply reuses it the catalogued commit-time event never fires and a `grep "theme:"` on a converted install cannot answer which palette the new slot resolved to.

**Solution**: A commit-time seam method that resolves the **opposite** slot alone against the retained enumeration and emits `theme: loaded` (and the fallback's own `loaded` line when it falls back), called only on a commit that actually converts a constant to adaptive.

**Outcome**: `y` on a slot-from-constant confirm makes the other slot live from values already in hand — the shipped default from the embedded set for an untouched slot, the panel's own parse for a stale hand-edited one, §8.5's per-slot fallback when neither resolves — with exactly one `theme: loaded` line carrying `slug` and `slot`, no directory read, and no change to the active theme.

**Do**:
- **Extend the `ThemeEnumerator` seam** (tasks 8-1 / 8-8) with the commit-time entry point, and wire the production adapter and the fixture fake to it:
  ```go
  // ResolveSlot resolves ONE slot against a retained enumeration and emits the
  // commit-time `theme: loaded` (§12.3's one load outside construction). It shares
  // its rule body with ResolveNominationFrom — embedded set first, then the
  // enumeration's entries, then §8.5's per-slot fallback — and reads no directory.
  ResolveSlot(e theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error)
  ```
  Factor it over the same body task 8-8 already shares, so the badge path and the load path can never disagree about the same slug.
- **Call it only on a converting commit**: after a **successful** slot commit whose *pre-commit* keys carried a non-empty `Theme`. Resolve the **opposite** slot (the one the user did not assign) using its raw persisted value from the pre-commit keys — empty for an untouched slot, the stale hand-edited slug otherwise.
- **Resolution order, from values already in hand**: an untouched slot holds a shipped default, which resolves from the **embedded set** and never touches the themes directory (cheap and infallible); a stale hand-edited slot resolves from the panel's **retained enumeration**, and only a slug the enumeration has no entry for falls through to the embedded set; in neither, it is unresolvable and takes §8.5's per-slot mode-matched fallback. **No commit-time directory read on any branch** — assert it with the themes directory removed after the panel opened.
- **Do not re-read the assigned slot** — its parse is already retained. State that in-source, since it is the half a reader is most likely to add "for symmetry".
- **Emit exactly one `theme: loaded`** per converting commit: INFO, **undeduplicated**, carrying `slug` and `slot`, and carrying the **fallback's** slug when the newly-live slot was unloadable — otherwise `theme: fallback applied` and `theme: loaded` both name the slug that failed and a grep on a broken install cannot answer which palette is actually rendering. Wire the emission onto **this commit entry point only**; task 8-10 pinned that the shared open/close resolver emits none, so this is added deliberately rather than inherited.
- **Join the nomination, not the active member**: the loaded `Theme` becomes the model's other nomination member so the pair is complete; the **active** member does not change (a commit is a write, not a navigation), and `ApplyTheme` is not called. Note that task 8-10's close re-resolves from persisted state regardless, so the join is model consistency rather than a dependency of the close path.
- **Resolve the answer half too, from the reply already in hand.** A conversion makes light/dark matter for a user whose launch deliberately never consulted detection (§8.2), so the in-force light/dark answer must be **established here** — never read off the constant path's pre-resolved gate, whose value is the standing dark fallback rather than a classification of the terminal (task 3-2). Classify the OSC 11 background retained at launch and record it as the model's light/dark answer, so task 8-10's close and task 8-8's next open select the in-force member correctly with no change of their own. **No new query, no new race, no new gate** — §9.3's transition dissolves precisely because the answer already arrived, and the single-resolution rule (§8.8) is untouched: a reply arriving after this point still never re-themes. If **no reply has landed** — which requires the panel to have been opened within milliseconds of launch — it falls to **dark**, the same rule as everywhere else. A user who launched on an adaptive pair already has a classified answer, so a conversion never arises for them and nothing is re-derived. **Record the answer directly and never route through `syncResolvedMode`**: that is where task 3-3 captures `startupCanvasHex`, so reusing it would re-anchor §11.4's echo guard to a canvas the startup window never painted. The anchor is captured once at gate resolution and must survive every conversion — this is the one path where a mistake re-sticks a colour in the user's terminal after Portal exits, and tasks 4-2 / 4-5 / 8-9 guard it only against *swaps*, which a conversion is not.
- **Emit nothing where nothing converts**: an already-adaptive `d`/`l`, any `Enter`, and any **failed** write all load nothing and emit nothing — both members are in hand already (or the write did not land), so there is no load to announce.

**Acceptance Criteria**:
- [ ] A confirmed `l` on a constant loads the **dark** slot (and a confirmed `d` loads the **light** slot) — the opposite one, never the assigned one.
- [ ] An untouched opposite slot resolves the shipped default from the embedded set with **zero** directory access.
- [ ] A stale hand-edited opposite slot resolves from the panel's retained enumeration; a slug absent from the enumeration falls through to the embedded set; a slug in neither takes §8.5's per-slot fallback (`theme_dark` → `tokyo-night`, `theme_light` → `tokyo-night-day`).
- [ ] No commit-time directory read on any branch (asserted with the directory removed after the panel opened).
- [ ] Exactly one `theme: loaded` per converting commit, carrying `slug` and `slot`; two conversions in one panel session emit two lines (it is not deduplicated).
- [ ] When the newly-live slot is unloadable, `theme: loaded` carries the **fallback's** slug and `theme: fallback applied` carries the failed nomination's — the two name different slugs on the same commit.
- [ ] An already-adaptive `d`/`l` and any `Enter` emit **no** `theme: loaded` and perform no resolution of a new member.
- [ ] A failed write emits no `theme: loaded`.
- [ ] The model's nomination holds both members after a conversion, while the active theme and the composed frame are unchanged.
- [ ] After a confirmed conversion on a **light** terminal the model's light/dark answer is light, and the next close (task 8-10) selects the **light** slot; on a dark terminal it selects the dark slot.
- [ ] The conversion issues **no** new OSC 11 query and arms no new gate — the answer is classified from the background captured at launch.
- [ ] A conversion leaves `startupCanvasHex` **byte-identical** — before and after a confirmed `d`/`l` on a constant, on a light terminal and a dark one — so task 3-3's anchor and task 4-5's divergence guarantee both still hold.
- [ ] The classification does not route through `syncResolvedMode` (asserted structurally, since that is the function that captures the anchor).
- [ ] A conversion performed before any reply has landed resolves to **dark**, and a reply arriving afterwards still does not re-theme.
- [ ] A user who launched on an adaptive pair is unaffected: their answer is already classified and a `d`/`l` commit does not re-derive it.
- [ ] `ResolveSlot` and the badge path's `Resolve` return the same slug for the same input (shared rule body).
- [ ] A `log.Discard`-backed loader emits nothing on this path.

**Tests**:
- `"it loads the opposite slot, not the assigned one"` — `TestCommitSlotLoad_LoadsTheOppositeSlot`
- `"it resolves an untouched slot from the embedded set"` — `TestCommitSlotLoad_UntouchedSlotIsTheShippedDefault`
- `"it resolves a stale slot from the retained enumeration"` — `TestCommitSlotLoad_StaleSlotFromEnumeration`
- `"it falls back per slot when unresolvable"` — `TestCommitSlotLoad_UnresolvableTakesTheModeMatchedFallback`
- `"it reads no directory at commit"` — `TestCommitSlotLoad_NoDirectoryRead`
- `"it emits one undeduplicated loaded line"` — `TestCommitSlotLoad_EmitsLoadedOncePerConversion`
- `"it names the fallback in loaded and the nomination in fallback applied"` — `TestCommitSlotLoad_LoadedNamesTheFallbackSlug`
- `"it emits nothing when nothing converts"` — `TestCommitSlotLoad_NonConvertingCommitIsSilent` (adaptive `d`/`l` and `Enter`)
- `"it emits nothing on a failed write"` — `TestCommitSlotLoad_FailedCommitLoadsNothing`
- `"it completes the nomination without changing the active member"` — `TestCommitSlotLoad_ActiveThemeUnchanged`
- `"it classifies the retained background on conversion"` — `TestCommitSlotLoad_ConversionUsesTheRetainedAnswer` (light terminal, dark terminal)
- `"it issues no new query on conversion"` — `TestCommitSlotLoad_ConversionIssuesNoQuery`
- `"it falls back to dark when no reply landed"` — `TestCommitSlotLoad_ConversionWithNoReplyIsDark`
- `"it never moves the startup canvas hex on a conversion"` — `TestCommitSlotLoad_ConversionDoesNotMoveStartupCanvasHex` (light terminal, dark terminal, no-reply)
- `"it agrees with the badge resolution"` — `TestCommitSlotLoad_SharesTheResolverBody`
- `"it is silent on the discard logger"` — `TestCommitSlotLoad_DiscardSilencesLoaded`

**Edge Cases**:
- This is **the one theme load that happens outside construction**, which is why §12.3's cadence column gives `theme: loaded` a commit-time entry at all.
- The slot the user just **assigned** needs no read — the panel's retained enumeration already holds its parse, which is what keeps arrowing the O(1) restyle — and the read that is needed is the **opposite** one.
- An **untouched** opposite slot holds a shipped default, so it resolves from the embedded set and never touches the themes directory (cheap and infallible).
- A **stale hand-edited** slot resolves from the panel's **retained enumeration**, and only a slug the enumeration has no entry for falls through to the embedded set — in neither, it is unresolvable and takes §8.5's per-slot mode-matched fallback.
- **No commit-time directory read** — issuing one would produce a *third* parse of the same slug, neither construction's nor the panel's, that can disagree with the row the user is looking at, reintroducing exactly the staleness split §5.8 exists to close.
- `theme: loaded` is **INFO, undeduplicated, one line per load**, carrying `slug` and `slot`, and fires **for the fallback too** when the newly-live slot is unloadable, or a `grep "theme:"` on a broken install cannot answer which palette is actually rendering.
- The event is wired onto the **commit entry point only** — task 8-10 pinned that the shared retained-enumeration resolver emits no `loaded` on open or close, so this is added deliberately rather than inherited from the shared body.
- A commit that converts nothing (an already-adaptive `d`/`l`, or any `Enter`) loads nothing and emits nothing, both members being in hand already.
- The loaded member joins the model's nomination so the pair is complete, but the **active** member does not change.
- §9.3's transition has **two halves** and both land on this keypress: the file half (the opposite slot's load) and the **answer half** — the light/dark classification a constant user's launch deliberately never made. The answer half dissolves only because `restore.go`'s query ran anyway; it does not dissolve into *nothing*, so the classification has to happen somewhere and this is the first keypress that needs it.
- The constant path's gate is **pre-resolved to the standing dark fallback**, not to a classification of the terminal, so reading it as the in-force answer after a conversion puts a light-terminal user on the dark slot — the exact outcome §9.3 exists to prevent.
- **No new query, no race, no gate**, and **dark** when no reply has landed — the same rule as everywhere else.
- The conversion writes the model's light/dark answer, which is the **same value the gate's resolution sets** — so it must record the answer directly and never route through `syncResolvedMode`, where task 3-3 captures `startupCanvasHex`. Re-capturing the hex would re-anchor §11.4's echo guard to a canvas the startup window never painted, on the one path where a mistake re-sticks a colour in the user's terminal after Portal exits. Tasks 4-2, 4-5 and 8-9 assert the anchor is stable across a **swap**; a conversion is not a swap and inherits none of those assertions, which is why this task needs its own.
- A failed write converts nothing, so it loads nothing and emits nothing.

**Context**:
> §8.4: "**Mid-session slot assignment loads the *other* slot at commit time.** A constant nominates one theme, so converting to adaptive makes a second slot live that construction never loaded. The slot the user just assigned needs no read — §5.8's enumeration already holds its parse… **An untouched slot holds a shipped default**, so it resolves from the embedded set and never touches the themes directory… **A stale hand-edited slot** resolves from the panel's retained enumeration… **No commit-time directory read.** Issuing one would produce a *third* parse of the same slug… **This is the one theme load that happens outside construction**, so it emits `theme: loaded` at commit rather than at construction."
> §12.3 (`theme: loaded`): "**Also fires at commit time** for the one load that happens outside construction: the newly-live opposite slot on a constant → adaptive conversion (§8.4). **When a nomination is unloadable it fires for the fallback too**, carrying the fallback's slug — otherwise `theme: fallback applied` and `theme: loaded` both name the slug that *failed*, and a `grep "theme:"` on a broken install cannot answer which palette is actually rendering."
> §9.3: "**The transition's other half is a file, not an answer**, and it does not dissolve: the slot the user did *not* assign was never loaded at construction and becomes live on the same keypress. §8.4 specifies that load."
> Task 8-10 pinned the emission policy of the retained-enumeration resolver: `theme: fallback applied` fires (deduped per process), `theme: loaded` does **not** — "wire the emission onto the construction and commit entry points, not onto the shared body".

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.4, §8.5, §9.3, §12.3, §5.8

## theming-system-9-7

### Task 9.7: A failed commit write and the outstanding-failure state

**Problem**: Phase 6 task 6-7 made the persister **return** its error precisely so this phase could report it, and so far nothing consumes it. A failed write is the picker idiom's one genuinely dangerous state — "applied but not persisted", the state the whole explicit-keypress design was buying protection from — and §9.13 turns it from silent into *reported* with four rules that are easy to get individually right and collectively wrong. The theme stays applied in memory. The `●` must **not** move, because the marker means "what is persisted" and would be lying if it moved — which forces the failure path to skip the key mutation *and* the recompute. The message persists **until the next keypress** rather than timing out like a transient flash, because it reports a state the user must act on and a message that vanishes on its own can be missed in a surface whose only other feedback is the `●` deliberately *not* moving. And the subtlest rule: **"outstanding" is a state, not a message.** Arrowing away dismisses the message while leaving the state, which is what stops the very next `Esc` reinstating the silent revert §9.13 exists to close — without that split the "reported rather than silent" property holds for exactly one keystroke.

**Solution**: A single commit-result handler that, on error, sets task 9-4's failed-commit message contender and an `themeCommitFailed` outstanding flag while skipping every mutation — with the message cleared by the next keypress and the **state** cleared only by a subsequent successful commit.

**Outcome**: A failed `d` leaves the previewed theme painting the screen, the `●` where it was, and `⚠ couldn't save theme` in the panel's message slot until the next key; arrowing dismisses the message but leaves the failure outstanding for task 9-9's close-time report; and a later successful commit clears both, so a `d` that fails followed by an `l` that succeeds raises nothing.

**Do**:
- **Route every commit through one handler** in `internal/tui/theme_panel_commit.go` — `func (m *Model) applyCommitResult(err error)` — called by `commitConstant`, `commitSlot` and the confirm's `y`. There is exactly one place the failure semantics live.
- **On error**: set the message to `themeMessageCommitFailed` (task 9-4 renders the pinned `⚠ couldn't save theme`) and set `m.themeCommitFailed = true`. Perform **no** key mutation, **no** recompute and **no** `ApplyTheme` — the theme stays applied in memory and the `●` cannot move, because nothing it derives from changed.
- **On success**: clear a live failed-commit message and clear `m.themeCommitFailed`. A `d` that fails followed by an `l` that succeeds must leave nothing outstanding, so task 9-9 raises **no** flash — the user is not told a theme was not saved when it was.
- **Message lifetime — until the next keypress, not a timer.** At the top of `updateThemePanel`'s key arm, clear a live failed-commit message and then **fall through** to normal dispatch (one key, one intent — the same shape `updateSessionList`'s actionable-key clear already uses). The keypress that *raises* the message is unaffected, because the clear runs before dispatch and the raise happens inside it. Do **not** use the flash lifecycle: no `flashTickCmd`, no 3s auto-clear, no generation counter — this message is not a transient flash.
- **State lifetime — until a commit succeeds.** `m.themeCommitFailed` survives arrows, paging, a confirm raised and cancelled, and the message being dismissed. Nothing else clears it; task 9-9 discharges it by raising the close-time report.
- **Do not log.** The persister is the **single** emission site for `theme: commit failed` (Phase 6 task 6-7) — the panel logging too would double the event and would make `internal/tui` a fourth emitter of the `theme` component, which §8.9 closes at three. Add a test that the panel emits no `theme` record on the failure path.
- **Confirm-driven failures land identically**: the confirm has already resolved (it is consumed before the write), so the two message contenders are never live together; the constant is not cleared in memory, so the badges still show its bare `●`; and the footer has already reverted to the standing scope.
- **A nil persister is not a failure** (task 9-1) — no message, no outstanding state. Keep that discrimination explicit so a capture or fixture model can never enter the reported-failure state.
- **Retry is free**: the commit keys are unconditional writes, so pressing `d`/`l`/`Enter` again simply retries. Add no retry affordance, no "press r to retry", and no state to clear first.

**Acceptance Criteria**:
- [ ] A failed commit leaves the previewed theme applied and the composed frame's colours unchanged.
- [ ] A failed commit performs **no** key mutation and **no** recompute — the `●` is on the same row before and after, asserted on the rendered rows.
- [ ] The message renders exactly `⚠ couldn't save theme` (task 9-4's pinned copy) in the panel's message slot.
- [ ] The message survives an intervening non-key event (a `tea.WindowSizeMsg` above the floor) and is cleared by the **next keypress**, which still performs its own action (an arrow both clears the message and moves the cursor).
- [ ] The message does not auto-clear: advancing the clock past `flashAutoClearDuration` with no keypress leaves it rendered, and no `flashTickCmd` is issued by the failure path.
- [ ] `m.themeCommitFailed` remains true after the message is dismissed by an arrow, after paging, and after a confirm is raised and cancelled.
- [ ] A subsequent **successful** commit clears both the message and the outstanding state.
- [ ] The panel emits no `theme` log record on the failure path (asserted with a `logtest.Sink`); the persister's single `theme: commit failed` WARN is the only record.
- [ ] A failed confirm-driven commit leaves the constant in the in-memory keys, shows the bare `●` still on it, restores the standing footer, and renders the failed-commit message with no confirm live.
- [ ] The confirm and the failed-commit message are never live simultaneously (task 9-4's exclusion, asserted through this path).
- [ ] A nil persister raises neither the message nor the outstanding state.
- [ ] Pressing the same commit key again after a failure retries: a second persister call is made, and a success clears everything.

**Tests**:
- `"it keeps the theme applied"` — `TestCommitFailure_ThemeStaysApplied`
- `"it does not move the marker"` — `TestCommitFailure_BadgeDoesNotMove`
- `"it renders the pinned message"` — `TestCommitFailure_MessageCopy`
- `"it persists until the next keypress"` — `TestCommitFailure_MessageClearsOnNextKeyAndFallsThrough`
- `"it survives a non-key event"` — `TestCommitFailure_MessageSurvivesWindowSize`
- `"it does not auto-clear on a timer"` — `TestCommitFailure_MessageHasNoTickLifecycle`
- `"it keeps the failure outstanding after the message is dismissed"` — `TestCommitFailure_StateOutlivesTheMessage`
- `"it clears the state on a later successful commit"` — `TestCommitFailure_SuccessDischargesTheState`
- `"it logs nothing from the panel"` — `TestCommitFailure_PanelEmitsNoThemeRecord`
- `"it lands identically on a confirmed commit"` — `TestCommitFailure_ConfirmDrivenFailure`
- `"it never co-renders with the confirm"` — `TestCommitFailure_NeverLiveWithTheConfirm`
- `"it treats a nil persister as inert"` — `TestCommitFailure_NilPersisterRaisesNothing`
- `"it retries on the same key"` — `TestCommitFailure_RetryIsJustPressingAgain`

**Edge Cases**:
- The theme **stays applied in memory** — this recreates "applied but not persisted", but as a *reported* state rather than a silent one, which is the distinction the picker idiom was buying.
- The `●` **does not move**, because the marker means "what is persisted" and would be lying if it moved — so a failed write performs no key mutation and no recompute.
- The message **persists until the next keypress** rather than timing out like a transient flash, since it reports a state the user must act on and a message that vanishes on its own can be missed in the surface where the only other feedback is the `●` deliberately *not* moving — it therefore does not use the 3s tick / generation lifecycle.
- The copy is pinned verbatim — `⚠ couldn't save theme` — glyph-backed, with `⚠` and text in `accent.attention` and **no `bg.attention` band**.
- **"Outstanding" is a state, not a message**: it runs from the moment a write fails until a **subsequent commit succeeds**, and nothing else clears it — arrowing away dismisses the *message* while leaving the state, which is what stops the very next `Esc` reinstating the silent revert.
- A `d` that fails followed by an `l` that succeeds clears the state and raises **no** flash, so the user is not told a theme was not saved when it was.
- A commit is always re-attemptable — the commit keys are unconditional writes, so pressing the key again simply retries, with no retry affordance and no state to clear first.
- The error arrives from the persister, which is also the **single** emission site for `theme: commit failed` (Phase 6 task 6-7), so the panel must not log or the event doubles.
- The confirm and the failed-commit line cannot be live together because a confirm resolves before any write happens.
- A failure on a confirm-driven commit lands identically — the constant is not cleared in memory, so the badges still show it.

**Context**:
> §9.13: "A failed write on `Enter`/`d`/`l`: **Reports in the panel's message slot** (§9.1) — `⚠` plus a terse statement that the theme could not be saved, glyph-backed per Portal's convention. It **persists until the next keypress** rather than timing out like a transient flash: it reports a state the user must act on, and a message that vanishes on its own can be missed in the surface where the only other feedback is the `●` deliberately *not* moving. **Keeps the theme applied in memory.** **Does not move the `●`** — the marker means 'what is persisted' and would be lying if it moved."
> §9.13: "**'Outstanding' is a state, not a message.** A commit failure is outstanding from the moment a write fails until a **subsequent commit succeeds** — nothing else clears it. In particular arrowing away does not: that dismisses the *message* (which persists only until the next keypress) while leaving the state, which is what stops the very next `Esc` reinstating the silent revert this section exists to close. And because a successful retry clears it, a `d` that fails followed by an `l` that succeeds raises no flash."
> §12.3: "`theme: commit failed` | WARN | Per failed write. Carries `slug`, `slot` (absent when committing a constant), and `reason`." — emitted by the `cmd`-owned persister, which is its single site (§8.9).
> Phase 6 task 6-7: "The persister **returns the error as well as logging it**, because Phase 9's outstanding-failure state machine and its `⚠ couldn't save theme` message need it — logging alone would leave the panel unable to report."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.13, §9.1, §9.2, §12.3, §8.9

## theming-system-9-8

### Task 9.8: Theme flashes outrank the filter line on both pages

**Problem**: All six theme signals — the three blocked-`t` flashes (task 8-13), the two forced-close flashes (task 8-11) and task 9-9's `⚠ theme not saved — see portal.log` — route through the notice band's **transient flash** slot, and the band's stated precedence puts the **filter line** above it. The filter line is the one contender above flash that can be live throughout a panel open, use and close: a filter can sit applied-but-unfocused on the Sessions list the whole time. Under that order two decided guarantees fail **silently**. §9.13's failed-commit report would never reach the band — and because raising the flash **discharges** the outstanding state, the report would be *destroyed rather than deferred*, which is precisely the silent revert §9.13 exists to close. And §9.10's proactive `NO_COLOR` block would produce nothing at all — the walkable dead end it exists to prevent, reached by another route. The justification for reversing them is asymmetric by design: a filter line is a persistent restatement of a state the user can already see in their own list, while each theme flash reports a one-time event with no other surface.

**Solution**: An origin discriminator carried **on the flash itself** (`setThemeFlash`), plus an explicit, documented and tested precedence rule in both band arbiters — the Sessions `activeNoticeBand` and task 8-12's `activeProjectNoticeBand` — under which a theme-origin flash owns the notice slot while a filter line is live, with non-theme flashes keeping today's order byte-for-byte.

**Outcome**: With a filter applied (or the filter input focused) on either page, every one of the six theme flashes renders in the notice band; a non-theme flash behaves exactly as it does today; and the discrimination is a property of the flash, so no future contender can quietly re-order theme flashes below the filter line.

**Do**:
- **Add the origin to the flash state** in `internal/tui/model.go` / `sessions_flash.go`: a `flashOrigin` value (`flashOriginDefault`, `flashOriginTheme`) beside `flashKind`, plus `func (m *Model) setThemeFlash(text string)` — bump `flashGen`, set the text, keep `flashWarning` as the kind, set the theme origin, and re-sync the active page's layout exactly as `setFlash` does. `setFlash` / `setSuccessFlash` reset the origin to default, so an ordinary flash can never inherit theme precedence.
- **Route all six theme flashes through it**: task 8-13's `theme picker needs colour — NO_COLOR is set`, `terminal too narrow for the theme picker`, `terminal too short for the theme picker`; task 8-11's `terminal too narrow — theme picker closed`, `terminal too short — theme picker closed`; and task 9-9's `⚠ theme not saved — see portal.log`. Re-point the earlier tasks' call sites rather than adding a parallel setter.
- **Pin the precedence in both arbiters.** In `activeNoticeBand` and `activeProjectNoticeBand`, state the rule in a doc comment and encode it in the ordering: *a theme-origin flash claims the notice slot even while the filter line is live; every other contender keeps today's order.* Name the two guarantees it protects (the failed-commit report, which is destroyed rather than deferred if it cannot claim the slot; and the proactive `NO_COLOR` block) so the reason survives a later refactor.
- **Prove the guarantee behaviourally, on both pages and both filter states**: a theme flash raised while the list is `Filtering` (input focused) and while it is `FilterApplied` (locked query) renders in the band, and the filter's own row is unaffected. This is the regression guard the change exists to install — the two contenders occupy different physical rows today, so the guarantee currently holds by construction, and the test is what stops a future single-row arbiter or a filter-state suppression silently destroying it.
- **Scope it to the six.** A non-theme flash keeps today's behaviour, asserted byte-identically (the burst partial-failure flash and the externally-killed-session bail are the live examples). The **flash carries the discrimination**; the band never infers it from the message text.
- **Leave the lifecycle alone**: same generation counter, same `flashTickMsg` auto-clear, same clear-on-next-actionable-key. No new band role, no seventh contender, no permanent entry.
- **Burst progress needs no answer** — §9.7 swallows `t` during a pending burst, so a theme flash and the `Opening n/N…` band are not simultaneously reachable. Record that as the reason rather than adding a rule.
- Keep composition below flash intact: a theme flash still outranks the multi-select banner, so the nesting §9.7 allows (panel over multi-select, `Esc` innermost-first) keeps working.

**Acceptance Criteria**:
- [ ] `setThemeFlash` sets the theme origin; `setFlash` / `setSuccessFlash` reset it to default.
- [ ] All six theme flashes are raised through `setThemeFlash` — asserted by a source-level or call-site test that no theme copy reaches `setFlash` directly.
- [ ] With `FilterState == FilterApplied` on Sessions, a theme flash renders in the notice band and the locked-query header still renders on its own row.
- [ ] With `FilterState == Filtering` on Sessions, a theme flash renders in the notice band and the live filter input still owns the section-header row.
- [ ] Both filter states behave identically on Projects, through task 8-12's flash slot.
- [ ] A non-theme flash's rendering, precedence and lifecycle are byte-identical to today under both filter states (regression assertion).
- [ ] The band still holds at most one contender — no double band on any page in any of the above.
- [ ] The theme flash still outranks the multi-select banner and still co-renders per the documented Sessions exception.
- [ ] The flash lifecycle is unchanged: the auto-clear tick with its generation guard, and the clear-on-next-actionable-key, both apply to a theme flash.
- [ ] No new notice-band role, contender or permanent entry is added, and the six route through the single transient-flash slot.
- [ ] `t` during a pending burst is still swallowed, so the theme flash and the `Opening n/N…` band are never both due.

**Tests**:
- `"it tags a theme flash with its origin"` — `TestThemeFlash_OriginDiscriminator`
- `"it routes every theme copy through the theme setter"` — `TestThemeFlash_AllSixUseSetThemeFlash`
- `"it renders with a filter applied on Sessions"` — `TestThemeFlash_OutranksAppliedFilterOnSessions`
- `"it renders with the filter input focused on Sessions"` — `TestThemeFlash_OutranksLiveFilterOnSessions`
- `"it renders with a filter applied on Projects"` — `TestThemeFlash_OutranksAppliedFilterOnProjects`
- `"it leaves a non-theme flash's order unchanged"` — `TestThemeFlash_NonThemeFlashUnchanged`
- `"it never renders two bands"` — `TestThemeFlash_SingleSlotHolds`
- `"it still outranks the multi-select banner"` — `TestThemeFlash_ComposesWithMultiSelect`
- `"it keeps the existing lifecycle"` — `TestThemeFlash_LifecycleUntouched` (tick + generation guard + actionable-key clear)
- `"it adds no contender"` — `TestThemeFlash_NoNewBandRole`

**Edge Cases**:
- The band's existing order is *filter line → burst progress → transient flash → multi-select banner → unsupported banner → no-tags signpost*, and the filter line is the **one contender above flash that can be live throughout a panel open, use and close** — a filter can sit applied-but-unfocused on the Sessions list the whole time.
- Under that order two decided guarantees fail **silently** — §9.13's failed-commit report would never reach the band, and because raising the flash **discharges** the outstanding state the report would be *destroyed rather than deferred*; and §9.10's proactive `NO_COLOR` block would produce nothing at all, the walkable dead end it exists to prevent reached by another route.
- The justification is asymmetric by design — a filter line is a persistent restatement of a state the user can already see in their own list, while each theme flash reports a one-time event with no other surface.
- The change is **scoped to the six theme flashes**, not to flashes generally, so a non-theme flash keeps today's order — which means the flash itself must carry the discrimination rather than the band inferring it.
- It applies on **Sessions and Projects alike**, the Projects slot being task 8-12's flash-only contender.
- Burst progress needs no answer, since §9.7 swallows `t` during a pending burst.
- The flash still composes correctly with everything below it — it outranks the multi-select banner, so the nesting §9.7 allows keeps working.
- The existing flash lifecycle is untouched (same generation counter, same tick, same clear-on-next-actionable-key) and all six flashes route through the one transient-flash slot, so no new band role, no seventh contender and no permanent entry is added.

**Context**:
> §14A: "**The filter line is the one contender above flash that can be live throughout a panel open/use/close, and the theme flashes take precedence over it.** A filter can sit applied-but-unfocused on the Sessions list the whole time, and under the existing order two decided guarantees would fail silently: §9.13's failed-commit report would never reach the band — and because raising the flash **discharges** the outstanding state, the report would be destroyed rather than deferred… §9.10's proactive `NO_COLOR` block would produce nothing at all… **This is a change to the band's precedence, scoped to these flashes**."
> §14A: "**Notice-band precedence for these flashes.** The band is a single-slot arbiter whose existing order is *filter line → burst progress → transient flash → multi-select banner → unsupported banner → no-tags signpost*. All six theme signals route through the **transient flash** slot, which composes correctly with everything below it… and needs no answer for burst progress, since §9.7 swallows `t` during a pending burst."
> **Ambiguity flagged and resolved conservatively**: the spec models one arbiter whose contender list spans the filter line and the band, while Portal's implementation splits those contenders across two *physical* rows — the filter line is a section-header claimant (`applySectionHeader` / `applyProjectsSectionHeader`), the flash is the separate notice-band row — so no filter-state suppression exists today and the guarantee currently holds by construction. This task therefore implements the precedence as an **explicit, tested rule plus the origin discriminator** rather than inventing a suppression the code does not have; if an implementer finds any path where a filter-line contender does pre-empt the band, the theme-origin flash must win there.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §14A, §9.13, §9.10, §9.7, §9.8

## theming-system-9-9

### Task 9.9: Closing with a failure outstanding raises and discharges the report

**Problem**: The failed-commit report has to **survive the panel closing**, and composed naively it does not. `Esc` is the only way out and it re-resolves from persisted state (task 8-10), so the very next keypress both clears task 9-7's message *and* drops the theme the user chose — with no `●` movement to signal it (§9.13 correctly forbids that) and nothing on the main screen. The "reported rather than silent" property would hold for exactly one keystroke, and the user would be left rendering the old theme with no explanation. Two further edges have decided answers that a naive implementation gets wrong in opposite directions: on a **forced close** (task 8-11) both the geometry flash and the commit flash are due at once and the band has one slot — and losing the commit flash on the one path where the user cannot reopen the panel to retry is exactly the failure being closed; while re-firing the flash on **every** close for the life of the process, about a failure already reported, is what happens if raising it does not **discharge** the state.

**Solution**: A post-close step on task 8-10's single `closeThemePanel` — the named hook it left for exactly this — that raises the pinned theme flash when a failure is outstanding and clears the state in the same act, with the forced-close path preferring it over its geometry copy.

**Outcome**: Closing the panel with a failed commit outstanding leaves the user on the main screen reading `⚠ theme not saved — see portal.log`, on whichever page they were on; the state is discharged so a later close is silent; a forced close raises the commit flash instead of its geometry flash; and `Ctrl-C` accepts the report as undelivered with the log as the record.

**Do**:
- **Pin the copy** as a constant beside task 8-13's entry strings: `⚠ theme not saved — see portal.log`, asserted verbatim against §14A.
- **Fill the post-close hook.** Change `closeThemePanel` to return a `tea.Cmd` and have its single post-close step do: if `m.themeCommitFailed` → `m.setThemeFlash(themeNotSavedFlash)` (task 9-8's theme-origin setter), clear `m.themeCommitFailed` (**the discharge**), and return `flashTickCmd(m.flashGen)` so the report inherits the standard auto-clear; otherwise return nil. Both callers (`Esc` and the forced close) propagate the cmd out of `Update`. There is exactly **one** close path and one hook — do not fork a second close.
- **Forced close: the commit flash wins, and the flag is read *before* the close.** In task 8-11's below-floor resize path, capture `m.themeCommitFailed` **before** calling `closeThemePanel`, and raise the geometry flash only when that captured value was false; when it was true the close's own hook has already raised the report and discharged the state, so the resize path raises nothing further. Pin the ordering in-source, because the naive shape is wrong in a way no reading of the sentence catches: the hook discharges the flag *as part of* raising the report, so a post-close `if !m.themeCommitFailed { raise geometry }` always sees false and overwrites the report the hook just placed in the single-slot band. Record the reasoning too: the band has one slot and the two report different things — a geometry event the user can see for themselves (their terminal just got smaller and the panel vanished) versus an unsaved setting they must act on. Losing the geometry flash costs nothing; losing the commit flash on the one path where the user cannot reopen the panel to retry is exactly the failure §9.13 closes.
- **`Ctrl-C` is an accepted undelivered report.** It is the one exit §9.7 keeps live inside the panel, the main screen is going away, so raise nothing, write nothing to stderr, and leave the state undischarged — `theme: commit failed` is already in the log and is the record. Note the rejected alternative in-source: a post-TUI stderr warning would put a message about a colour preference on the channel Portal reserves for bootstrap failures.
- **The revert itself stays.** The write did not land, so the theme is not persisted and `Esc` resolving to persisted state is right — the user is told, on the surface they are left looking at. Do not "fix" the revert by keeping the unsaved theme applied after close.
- **Both pages.** The report lands in task 8-12's Projects flash slot as readily as the Sessions band, since closing is reachable from both — assert it on both.
- **No re-fire.** After a discharge, re-opening the panel and pressing `Esc` raises nothing; only a *new* failed commit can arm it again.

**Acceptance Criteria**:
- [ ] `Esc` with a failure outstanding raises exactly `⚠ theme not saved — see portal.log` in the active page's flash slot and returns the standard flash tick cmd.
- [ ] Raising it **discharges** the state: a second open + `Esc` with no new failure raises nothing.
- [ ] `Esc` with nothing outstanding raises nothing.
- [ ] A **forced close** with a failure outstanding raises the commit flash and **not** the geometry flash, and discharges the state.
- [ ] A forced close with nothing outstanding raises the pinned geometry flash (task 8-11's behaviour, unchanged).
- [ ] `Ctrl-C` with a failure outstanding quits, raises no flash, writes nothing to stderr, and leaves the log as the only record.
- [ ] The close still re-resolves persisted state — the unsaved theme is reverted, and the flash is the only thing that changed about that path.
- [ ] The report renders on **Projects** as well as Sessions.
- [ ] The flash is theme-origin, so it claims the band with a filter applied (task 9-8).
- [ ] There is exactly one close implementation: the forced close and `Esc` produce identical model state apart from which flash is raised.
- [ ] A failed commit followed by a **successful** commit and then `Esc` raises nothing (the state was already cleared by task 9-7).

**Tests**:
- `"it raises the pinned report on close"` — `TestCloseReport_RaisesTheFlash`
- `"it discharges the state"` — `TestCloseReport_DischargedOnRaise` (second close silent)
- `"it raises nothing with no failure"` — `TestCloseReport_SilentWhenNothingOutstanding`
- `"it wins over the geometry flash on a forced close"` — `TestCloseReport_ForcedCloseCommitFlashWins`
- `"it keeps the geometry flash when nothing is outstanding"` — `TestCloseReport_ForcedCloseGeometryFlashSurvives`
- `"it delivers nothing on Ctrl-C"` — `TestCloseReport_CtrlCIsAnUndeliveredReport`
- `"it still reverts to persisted state"` — `TestCloseReport_RevertStands`
- `"it reports on Projects too"` — `TestCloseReport_ProjectsFlashSlot`
- `"it claims the band with a filter applied"` — `TestCloseReport_OutranksFilterLine`
- `"it uses the one close path"` — `TestCloseReport_SingleClosePath`
- `"it raises nothing after a successful retry"` — `TestCloseReport_SuccessfulRetryIsSilent`

**Edge Cases**:
- The report **must survive the panel closing**: `Esc` is the only way out and it re-resolves from persisted state, so composed naively the very next keypress both clears the message and drops the theme the user chose — with no `●` movement to signal it and nothing on the main screen — and the "reported rather than silent" property would hold for exactly one keystroke.
- The copy is pinned verbatim — `⚠ theme not saved — see portal.log`.
- **Raising the flash discharges the state**, because it is the report the state exists to produce — without the discharge, reopening the panel and pressing `Esc` would re-fire a flash about a failure already reported, on every close for the life of the process.
- On a **forced close** both flashes are due at once and the **failed-commit flash wins**: the notice band has one slot, and the two report different things — a geometry event the user can see for themselves versus an unsaved setting they must act on — and the state is discharged, because the report was made.
- Losing the geometry flash costs nothing, while losing the commit flash on the one path where the user cannot reopen the panel to retry is exactly the failure being closed.
- The revert itself is correct and **stays** — the write did not land, so the theme is not persisted and `Esc` resolving to persisted state is right — but the user is told, on the surface they are left looking at.
- **`Ctrl-C` with a failure outstanding is an accepted undelivered report** — it is the one exit §9.7 keeps live inside the panel, the main screen is going away so there is nowhere to raise a flash, and `theme: commit failed` is already written; the alternative (a post-TUI stderr warning) would put a message about a colour preference on the channel Portal reserves for bootstrap failures.
- The flash attaches to task 8-10's **single** close path through its named post-close hook, never a forked second close.
- With nothing outstanding a close raises nothing and a forced close keeps its own geometry flash.
- The report lands in the Projects flash slot as readily as the Sessions band, since closing is reachable from both pages.

**Context**:
> §9.13: "**The report must survive the panel closing.** `Esc` is the only way out and it re-resolves from persisted state (§9.2) — so composed naively, the very next keypress both clears the message and drops the theme the user chose, with no `●` movement to signal it… and nothing on the main screen. **So closing the panel with a failed commit outstanding raises a main-screen flash**: `⚠ theme not saved — see portal.log`. **Raising the flash discharges the state** — it is the report the state exists to produce, so once made the state has done its job."
> §9.13: "**On a forced close (§9.8) both flashes are due at once, and the failed-commit flash wins.**… **The state is discharged**, because the report was made. Losing the geometry flash costs nothing; losing the commit flash on the one path where the user cannot reopen the panel to retry is exactly the failure this section closes. The revert itself is correct and stays… but the user is told, on the surface they are left looking at."
> §9.13: "**`Ctrl-C` with a failure outstanding is accepted as an undelivered report.**… **The log is the record** — `theme: commit failed` is already written (§12.3) — and the alternative, a post-TUI stderr warning, would put a message about a colour preference on the same channel Portal reserves for bootstrap failures."
> Task 8-10 left "a named hook point (a single post-close step) rather than letting Phase 9 fork the path"; task 8-11's forced close calls the same `closeThemePanel` and then raises its geometry flash.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.13, §9.8, §9.7, §9.2, §14A

## theming-system-9-10

### Task 9.10: `keymap_dispatch_guard_test` covers the panel and confirm scopes

**Problem**: The panel and its confirm now render **two** bespoke vertical footers from descriptors that live outside the page keymaps — exactly the second and third place a key label can go stale, which is the drift class `keymap_dispatch_guard_test` exists to make a test failure rather than a bug report. Task 8-5 deliberately authored the panel scope as all six keys (not the four visible commits) *because* the guard's contract is parity in **both** directions: a descriptor key with no probe fails, and a probed key absent from the descriptor fails — so omitting arrows and paging from the descriptor is what breaks parity rather than what satisfies it. The guard also has a way of passing vacuously here that it does not have on the pages: the panel is key-exclusive, so a probe asserting only that the key was *consumed* passes for every key in the alphabet. And `Esc` is genuinely overloaded — it cancels the confirm when one is live and closes the panel otherwise — so a probe that does not distinguish the two proves nothing about either.

**Solution**: Two new parity suites over the shared `assertDescriptorDispatchParity` helper — one for `themePanelKeymap()` with six probes, one for `themePanelConfirmKeymap()` with `y`/`Y` and `n`/`N` probes — each driving the live `updateThemePanel` from a seeded open-panel model with a faked enumerator and a recording fake persister, and each asserting the **bound effect**.

**Outcome**: Deleting any panel or confirm dispatch arm, or adding a descriptor entry with no dispatch, fails the guard; and the three page descriptors and their probe sets are untouched.

**Do**:
- **Seed a panel model helper** in `internal/tui/keymap_dispatch_guard_test.go` (or a sibling test file) mirroring `sessionsGuardModel`: a Sessions model with a **faked `ThemeEnumerator`** returning a small union with at least two selectable rows and one invalid row, a **recording fake `ThemePersister`** that writes nowhere, the panel already open via a real `t` press, and dimensions above the floor with `colourless` false so neither entry block is armed.
- **Panel scope — six probes, each asserting the bound effect**:
  - `↑↓` — the panel list's index moves.
  - `^↑/↓` — the panel list's `NextPage`/`PrevPage` bindings are live and paging moves the page.
  - `⏎` — the fake persister records a `CommitTheme` call with the cursor's slug **and** the panel is still open (both halves, since "did not close" is half of what §9.2 pins).
  - `d` / `l` — the fake records `CommitThemeSlot` with the matching typed slot (seeded from an adaptive setting so no confirm intervenes).
  - `esc` — the panel closes (open false, enumeration discarded), with **no** confirm live.
- **Confirm scope — a second suite**: seed a **constant** setting, press `l` to raise the confirm, then probe `y` (a `CommitThemeSlot` call recorded and the confirm cleared) and `n` (no persister call, confirm cleared, panel still open). Add companion assertions that **`Y`** and **`N`** reach the same arms, so the uppercase dispatch cannot drift from the lowercase descriptor entry.
- **Distinguish the two `Esc` meanings**: the panel suite's `esc` probe asserts a *close*; the confirm suite adds an `Esc`-cancels assertion (confirm cleared, panel still open, nothing written). These must be separate assertions, since the same key resolves the innermost thing first.
- **Never assert mere consumption.** Document in the suites that key-exclusive swallowing makes "the key was consumed" trivially true for every key, so each probe asserts a state change the dispatch is supposed to cause.
- **Guard the negatives**: the panel scope carries **no `RightAligned` entry**, so the existing `?`-help allow-list has nothing to exclude here and must not be widened; `?` has **no** descriptor entry and **no** probe (it is swallowed — there is no panel help modal); and `Ctrl-C` is not a descriptor entry, being the global quit that stays live everywhere.
- **Leave the pages alone**: assert that `sessionsKeymap()`, `projectsKeymap()`, `previewKeymap()` and their existing probe maps are byte-unchanged by this task.
- Keep both suites in the **unit** lane with no `t.Parallel()`, and let the fakes ensure no directory, no `prefs.json` and no real persister is touched.

**Acceptance Criteria**:
- [ ] `assertDescriptorDispatchParity` runs over `themePanelKeymap()` with a probe for every one of the six entries, all honouring their bound effect.
- [ ] Removing any panel dispatch arm (arrows, paging, `Enter`, `d`, `l`, `Esc`) makes the corresponding probe fail.
- [ ] Adding a seventh descriptor entry with no probe fails the descriptor-coverage direction.
- [ ] The confirm suite probes `y` and `n` and additionally asserts `Y` and `N` reach the same arms.
- [ ] The panel `esc` probe asserts a **close**; the confirm `Esc` assertion asserts a **cancel** with the panel still open.
- [ ] Every probe asserts a state change, not consumption — a probe that merely swallowed the key would fail (asserted by construction and stated in the suite's doc comment).
- [ ] No probe writes to a real prefs file or reads a real themes directory (fake persister, faked enumerator).
- [ ] The panel scope has no `RightAligned` entry and the `?`-help allow-list is unchanged.
- [ ] `?` has no descriptor entry and no probe in either scope.
- [ ] `Ctrl-C` is not a descriptor entry in either scope.
- [ ] The three page descriptors and their probe maps are byte-identical before and after this task.
- [ ] Both suites are unit-lane and carry no `t.Parallel()`.

**Tests**:
- `"it keeps the panel descriptor and dispatch in parity"` — `TestThemePanelDescriptorDispatchParity`
- `"it keeps the confirm descriptor and dispatch in parity"` — `TestThemeConfirmDescriptorDispatchParity`
- `"it honours both cases of the confirm keys"` — `TestThemeConfirmDispatch_UppercaseReachesTheSameArm`
- `"it distinguishes panel close from confirm cancel"` — `TestThemePanelDispatch_EscMeansInnermostFirst`
- `"it fails on a missing dispatch arm"` — `TestThemePanelParity_DetectsARemovedArm` (guard-of-the-guard, table-driven over a stubbed dispatch)
- `"it probes a bound effect rather than consumption"` — `TestThemePanelParity_ProbesAssertEffects`
- `"it adds no right-aligned entry"` — `TestThemePanelKeymap_NoRightAlignedEntry`
- `"it binds no help key"` — `TestThemePanelKeymap_NoHelpEntry`
- `"it leaves the page descriptors untouched"` — `TestKeymapGuard_PageDescriptorsUnchanged`

**Edge Cases**:
- The guard's contract is descriptor↔dispatch **parity in both directions** — a descriptor key with no probe fails ("advertises a key the guard cannot tie to dispatch") and a probed key absent from the descriptor fails ("dispatch binds a key the descriptor omits").
- All **six** panel keys need a probe driving the live `updateThemePanel`, arrows and paging included, which is exactly why task 8-5 authored the scope complete rather than as the four visible commits — omitting them is what breaks parity rather than what satisfies it.
- The confirm is a **second scope** with its own probes for `y`/`Y` and `n`/`N`, so its substituted footer and its dispatch cannot drift either.
- The panel scope carries **no `RightAligned` entry**, so the existing `?`-help allow-list has nothing to exclude here and must not be widened to admit one.
- **`?` is swallowed inside the panel** and must have no descriptor entry and no probe — there is no panel help modal and the scope exists to drive the footer and the guard, not a help body.
- A probe must assert the **bound effect**, not merely that the key was consumed, or key-exclusive swallowing would make every probe pass trivially.
- The commit probes need a fake persister so a probe writes nowhere and a faked seam so none reaches a directory.
- The `Esc` probe must distinguish the panel's **close** from the confirm's **cancel**, since the same key resolves the innermost thing first.
- `Ctrl-C` is not a descriptor entry — it is the global quit that stays live everywhere.
- The three page descriptors and their probe sets must stay byte-unchanged by this task.

**Context**:
> §9.12: "**The panel's keys live in the keymap descriptor as a panel scope** — **all six**… The descriptor must be complete or the dispatch guard's descriptor↔dispatch parity is what breaks… **`keymap_dispatch_guard_test` covers them.**… **`?` does nothing inside the panel.**"
> §9.2: "**The confirm's keys live in the descriptor as a nested confirm scope** under the panel scope (§9.12), so its footer renders from the descriptor like the panel's and `keymap_dispatch_guard_test` covers `y`/`Y`/`n`/`N` too."
> §13.6: "`keymap_dispatch_guard_test` | Extended to cover the panel scope (§9.12)."
> `internal/tui/keymap_dispatch_guard_test.go` (existing): "for each page they assert a two-way correspondence… Every non-help descriptor Key has a probe that drives the page's LIVE Update and asserts the dispatch HONOURS that key (produces the documented bound effect, not a passthrough no-op)… The `?` help self-entry is the only allow-listed exception… derived from the descriptor's own RightAligned flag rather than a hand-listed glyph so it cannot silently widen."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.12, §9.2, §13.6

## theming-system-9-11

### Task 9.11: The panel behaviour test through the `ThemeEnumerator` seam

**Problem**: §13.6 names this a **new** test because the panel carries a large body of exactly-specified, purely deterministic behaviour that nothing else covers — and the parts most likely to be got wrong are precisely the parts with no other test home. `keymap_dispatch_guard_test` covers the descriptor and **none** of this. Phase 8's tasks proved the pure functions *inside* `internal/theme` (the union, the comparator, the badge table), but the panel is where they are wired together, and a wiring mistake — badges derived from the nomination instead of the resolution record, the union re-derived from the wrong keys after a commit, the cursor anchored by index — renders a plausible-looking panel that is quietly wrong. All of it is a pure function of injected state, so it is cheap to cover and expensive to leave to inspection; and the seam §13.3 requires as an **architectural commitment rather than a convenience** is what makes it reachable at all, since the harness must render an invalid-theme row with no real themes directory and §7.1's import guard forbids `internal/capture` reaching config.

**Solution**: A single `internal/tui/theme_panel_behaviour_test.go` driving the panel through a wholesale-faked `ThemeEnumerator` and a fake persister, covering §13.6's named list end to end at the panel level.

**Outcome**: Every named behaviour has a test that exercises it *through the panel* — the rows the panel lists, the order it lists them in, the badges it marks, what a commit does to all three, how the confirm resolves, and how the outstanding-failure state machine behaves — with the suite touching no directory, no `prefs.json` and no real persister.

**Do**:
- **Fake the seam wholesale**: a test `ThemeEnumerator` returning declared `Enumeration` / `Union` / `Resolution` values with no I/O on any of its four methods (`Open`, `Reassemble`, `Resolve`, task 9-6's `ResolveSlot`), plus a recording fake `ThemePersister` whose error is settable per test. Both live in the test file — no production seam gains a test-only shape.
- **Cover the named set, exercised through the panel**, each as its own sub-test group:
  1. **§9.4's union and one-slug-one-row** — a persisted built-in slug is that built-in's row, an existing-but-invalid persisted file is that file's row, and a `reserved name` file is the one legitimate two-rows-for-one-slug case.
  2. **§9.5's sort key rules** — sort key versus display label, the case-insensitive comparison with its byte-wise tie-break, and the tie guaranteed by construction between a `reserved name` row and its built-in with the **built-in first**.
  3. **The four-element row-composition priority and its three-character truncation floor** — including the persisted-and-invalid row where the badge competes and the reason is the first element dropped.
  4. **The three-row badge derivation table**, including the **shipped-default row that is the most common install** (a virgin `prefs.json`), and the fallback case where the badge stays on the persisted slug.
  5. **§9.2's commit recompute and identity-anchored cursor** — rows appearing and disappearing, re-sorting, badges moving, and the cursor staying on the previewed theme when a row is inserted above it.
  6. **The confirm's three-input resolution and swallow-everything-else rule.**
  7. **§9.13's outstanding-failure state machine** — message until the next keypress, state until a successful commit, the close-time raise-and-discharge, and the forced-close precedence.
- **State the added value in the suite's doc comment**: Phase 8 proved the pure functions inside `internal/theme`; this proves the panel **wires** them — so a test here asserts panel-observable behaviour (rendered rows, cursor index, badge placement, persister calls) rather than re-asserting `internal/theme`'s own unit assertions.
- **Be complete against the named list rather than sampling it.** A gap here is invisible — nothing else covers this behaviour — so add a checklist comment enumerating §13.6's seven items and keep one test group per item.
- **Lane and hygiene**: unit lane, no `t.Parallel()` (the project rule), no `tea.Program`, no real tmux, no built binary, no config access. Drive the model through `Update` with real key messages so the assertions are about production dispatch, not helper functions.

**Acceptance Criteria**:
- [ ] The suite drives the panel through the faked seam and a fake persister only — it opens no directory, reads no `prefs.json` and constructs no real `prefs.Store`.
- [ ] Each of §13.6's seven named behaviours has its own test group, named to match, with a checklist comment mapping group → item.
- [ ] The union group asserts one-slug-one-row for a persisted built-in, a persisted invalid file, and the `reserved name` two-row case.
- [ ] The ordering group asserts the built-in-first tie resolution and the case-insensitive/byte-wise comparison through the rendered row order.
- [ ] The composition group asserts one delegate line per row and the reason-dropped-before-badge priority at the panel's minimum width.
- [ ] The badge group asserts all three derivation rows, including the virgin-install shipped-default case.
- [ ] The commit group asserts a row appearing, a row disappearing, a re-sort, a badge move and the identity-anchored cursor.
- [ ] The confirm group asserts `y`/`Y`, `n`/`N`/`Esc` and the swallow-everything-else rule.
- [ ] The failure group asserts the message-versus-state split, the discharge on close, and the forced-close precedence.
- [ ] Assertions are panel-observable (rendered output, cursor index, recorded persister calls) rather than re-assertions of `internal/theme` internals.
- [ ] The suite is unit-lane, carries no `t.Parallel()`, and passes with `-race`.

**Tests**:
- `"it lists one row per slug"` — `TestThemePanelBehaviour_Union`
- `"it orders rows and resolves the guaranteed tie"` — `TestThemePanelBehaviour_Ordering`
- `"it composes a row within the width budget"` — `TestThemePanelBehaviour_RowComposition`
- `"it derives every badge row"` — `TestThemePanelBehaviour_Badges`
- `"it recomputes and re-anchors after a commit"` — `TestThemePanelBehaviour_CommitRecompute`
- `"it resolves the confirm on three inputs"` — `TestThemePanelBehaviour_Confirm`
- `"it runs the outstanding-failure state machine"` — `TestThemePanelBehaviour_FailureStateMachine`
- `"it touches no config"` — `TestThemePanelBehaviour_NoConfigAccess`

**Edge Cases**:
- §13.6 names it a **new** test because the panel carries a large body of exactly-specified, purely deterministic behaviour that nothing else covers — all pure functions of injected state, cheap to cover and expensive to leave to inspection.
- It is driven through the seam, which §13.3 requires as an **architectural commitment rather than a convenience**: the harness must render an invalid-theme row with no real themes directory, and §7.1's no-real-config import guard forbids `internal/capture` reaching config at all.
- Coverage is the whole named set — §9.4's union and its one-slug-one-row rule; §9.5's sort key rules including the tie guaranteed by construction between a `reserved name` row and its built-in and the built-in-first resolution that settles it; the four-element row-composition priority and its three-character truncation floor; the three-row badge derivation table **including the shipped-default row that is the most common install**; §9.2's commit recompute and identity-anchored cursor; the confirm's three-input resolution and swallow-everything-else rule; and §9.13's outstanding-failure state machine.
- It exercises them **through the panel**, where Phase 8's tasks proved the pure functions inside `internal/theme` — the added value is that the panel wires them, not a second copy of the same assertions.
- `keymap_dispatch_guard_test` covers the descriptor and **none** of this.
- The seam is faked wholesale so the suite touches no directory, no `prefs.json` and no persister.
- Lane is **unit**, with no `t.Parallel()` per the project rule.
- A gap here is invisible, so the suite must be complete against the named list rather than sampling it.

**Context**:
> §13.6 (Panel behaviour test): "**New**, driven through the `ThemeEnumerator` seam (§13.3), which §13.3 requires as an architectural commitment rather than a convenience. The panel carries a large body of exactly-specified, purely deterministic behaviour that nothing else covers: §9.5's sort key rules including the guaranteed `reserved name`/built-in tie and its built-in-first resolution; the four-element row-composition priority and its truncation floor; the three-row badge derivation table, including the shipped-default row that is the most common install; §9.4's union and its one-slug-one-row rule; §9.2's commit recompute and identity-anchored cursor; the confirm's three-input resolution and swallow-everything-else rule; and §9.13's outstanding-failure state machine. All pure functions of injected state, so cheap to cover and expensive to leave to inspection. (`keymap_dispatch_guard_test` covers the descriptor, not any of this.)"
> §13.3: "**The panel's theme enumeration is behind an injectable seam.**… This is an architectural requirement, not a convenience — the harness must render an invalid-theme row without a real themes directory, and §7.1's no-real-config import guard forbids `internal/capture` reaching config at all. It is also what makes the panel unit-testable (row composition, ordering, truncation, the invalid-row skip), none of which otherwise has a test home."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §13.6, §13.3, §9.2, §9.4, §9.5, §9.13

## theming-system-9-12

### Task 9.12: The confirm, failed-commit and minimum-height-with-message fixtures

**Problem**: Three specified surfaces still have **no way to be seen before release**, and §13.4's guard structurally cannot report their absence — it enumerates whatever fixtures exist, so a missing fixture reads as coverage. Each carries something no other frame does. §9.1 warns the message-slot copy may **wrap** at the minimum width and §14A calls panel wording *"a layout constraint as much as a copy choice"*, so the one copy the spec says might not fit must be capturable. The **confirm** frame is the only visual proof of task 9-4's footer substitution — a reviewer cannot otherwise see that the standing four keys, none of which would act, have been replaced by `y confirm` / `n cancel`. The **failed-commit** frame is the only place the deliberate absence of a `bg.attention` band is checkable (the whole point of the token decision is that the main screen's warning band would read as heavy at 24–30 columns, which is a judgement only an image settles). And the **minimum-height-with-a-message** frame is the only surface on which §9.8's floor arithmetic — header + footer + one list row + one message row — is observable at all, and the only check that the message **truncates to one line rather than wrapping** there.

**Solution**: Three more fixtures on task 8-15's four inputs plus two capture-only message seeds, each declaring its state directly (fixtures are one-shot renders), registered in both registry lists with a `.tape` apiece.

**Outcome**: `capturetool --fixture theme-panel-confirm`, `--fixture theme-panel-commit-failed` and `--fixture theme-panel-min-height-message` render the panel's three remaining specified states offline, so the message slot's copy, tokens, footer substitution and floor arithmetic can all be reviewed as images before release.

**Do**:
- **Add two capture seeds** beside task 8-15's `InitialThemeCursor`, applied after the panel's real `t`-press open: `Deps.InitialThemeConfirm bool` (raises task 9-5's confirm against the persisted constant, with the pending slot immaterial to the frame — default light) and `Deps.InitialThemeCommitFailed bool` (raises task 9-7's message and sets the outstanding state). Both declare **state**, never text: the copy is rendered from the panel's own pinned constants, so a fixture can never ship a paraphrase of §14A. Fixtures are one-shot renders, so the state must be declarable directly rather than reached by pressing keys — exactly as task 8-6 drove the empty slot's height recompute by setting the field.
- **`theme-panel-confirm`** — raw keys `{Theme: "nord"}` (a constant, so the confirm is the state a real `l` would produce), the adaptive-pair frame's union, cursor on a row **other** than `nord`, `InitialThemeConfirm` set. Captured at the panel's **minimum width** so §9.1's wrap case is visible, and captured with `--theme` naming the theme under the cursor. The frame must show `clear constant nord?  y / n` in `text.secondary` with **no band**, and the footer **swapped** to `y confirm` / `n cancel`.
- **`theme-panel-commit-failed`** — the adaptive-pair keys and union, `InitialThemeCommitFailed` set, captured at the preferred width. The frame must show `⚠ couldn't save theme` with `⚠` and text in `accent.attention` and **no `bg.attention` band** behind it, with the badges unmoved (the marker means "what is persisted") and the standing four-row footer still in force.
- **`theme-panel-min-height-message`** — the same keys and union, `InitialThemeCommitFailed` set (so the **standing** footer is in force and the floor is the predicate's own arithmetic), captured through a tape whose terminal height lands the panel at `themePanelMinHeight(themePanelKeymap(), false)` and whose width is the panel minimum. The frame must show header(2) + the standing footer + **exactly one** list row + the message on **exactly one** line, truncated rather than wrapped. Record the intended row/column count in the tape's comment, since VHS sets the terminal in pixels via `Set Width` / `Set Height` / `Set FontSize`.
- **Coherence rule applies unchanged** (task 8-15): `--theme` must name the theme **under the cursor**, because §9.2's invariant is that the cursor's row is always what is painted behind the panel. Write the pairing into each fixture's doc comment and note that the harness cannot check it — an incoherent frame is indistinguishable from a correct one to a reviewer, and §13.4's guard enumerates and diffs colours, so it passes either way.
- **Register all three in both `FixtureByName` and `FixtureNames()`** or Phase 4 task 4-3's registry drift check fails; each inherits §13.4's three assertions automatically, which is the point of enumerating rather than naming.
- **Write a `.tape` per fixture** in the existing idiom and **verify a fresh write** — confirm the file's hash changed and retry on failure. VHS reports no error when it fails to write; a message state is visible **only** in the image and there is no functional assertion that would catch a capture that never landed, so an unverified capture reads either as "the change didn't render" or as a false pass.
- **Retention**: tapes and images are scaffolding under §13.2 — created as work proceeds, cleared at sign-off in Phase 10 — while the Go fixture definitions in `internal/capture` are **permanent** (§13.4's guard drives them).
- **Reach no config**: the union is faked wholesale and the seeds are fixture data, not config discovery, so §7.1's import guard is untouched — no themes directory, no `prefs.json`, no XDG lookup, and a **nil** persister so the seeded failure state writes nowhere.

**Acceptance Criteria**:
- [ ] `theme-panel-confirm` renders `clear constant nord?  y / n` verbatim in `text.secondary` with no background tint, and a footer of exactly `y confirm` / `n cancel`.
- [ ] At the captured minimum width the confirm message occupies two rows and the list body has shrunk by two — the wrap case §9.1 names.
- [ ] `theme-panel-commit-failed` renders `⚠ couldn't save theme` with `⚠` and text in `accent.attention`, **no** `bg.attention` band, the standing four-row footer, and the `●` in its pre-failure position.
- [ ] `theme-panel-min-height-message` renders exactly one list row and a **one-line** (truncated, not wrapped) message at the floor height, with the standing footer.
- [ ] Each fixture's `--theme` names the theme under its cursor, stated in the doc comment.
- [ ] All three appear in `FixtureByName` **and** `FixtureNames()`, and task 4-3's drift check passes.
- [ ] All three are enumerated by §13.4's swap-and-diff guard with no test edit.
- [ ] The seeds declare state only — no fixture supplies message text, and the rendered copy comes from the panel's pinned constants.
- [ ] No fixture reaches a themes directory, `prefs.json` or an XDG lookup; both import guards stay green and the seeded failure writes nowhere (nil persister).
- [ ] A `.tape` exists per fixture and each captured PNG is verified as a **fresh** write (hash changed) before review.
- [ ] The tapes record their intended column/row counts in comments.

**Tests**:
- `"it renders the confirm with its substituted footer"` — `TestPanelFixture_ConfirmFrame`
- `"it wraps the confirm at the minimum width"` — `TestPanelFixture_ConfirmWrapsAtMinWidth`
- `"it renders the failed-commit line with no band"` — `TestPanelFixture_CommitFailedFrame`
- `"it leaves the badge in place on a failure"` — `TestPanelFixture_CommitFailedBadgeUnmoved`
- `"it renders the floor arithmetic with a message"` — `TestPanelFixture_MinHeightMessageFrame`
- `"it truncates rather than wraps at the floor height"` — `TestPanelFixture_MinHeightMessageTruncates`
- `"it seeds state, never text"` — `TestPanelFixture_MessageSeedsAreStateOnly`
- `"it registers all three in both lists"` — `TestPanelFixture_MessageFramesRegistered`
- `"it reaches no config and writes nothing"` — `TestPanelFixture_MessageFramesNoConfigAccess`
- `"it is enumerated by the swap-and-diff guard"` — `TestPanelFixture_MessageFramesUnderTheGuard`

**Edge Cases**:
- §9.1 warns these messages may wrap at the minimum width and §14A calls panel wording a **layout constraint as much as a copy choice**, so the one copy the spec says might not fit must be capturable.
- The **minimum-height-with-a-message** frame is the only surface on which §9.8's floor arithmetic (header + footer + one list row + one message row) is observable at all, and the only check that the message **truncates to one line rather than wrapping** there.
- The **confirm** frame must show the footer **swapped** to `y confirm` / `n cancel`, which is the only visual proof of task 9-4's substitution, plus the pinned `clear constant <slug>?  y / n` in `text.secondary` with no band.
- The **failed-commit** frame must show `⚠ couldn't save theme` with `⚠` and text in `accent.attention` and **no `bg.attention` band** behind it.
- Each frame declares its four inputs coherently under task 8-15's rule — palette, raw persisted keys, faked union and cursor — and `--theme` must name the theme **under the cursor**, an authoring rule the harness cannot check since an incoherent frame is indistinguishable from a correct one to a reviewer and §13.4's guard passes either way.
- Fixtures are **one-shot renders**, so a message state must be declarable directly rather than reached by pressing keys, exactly as task 8-6 drove the empty slot's height recompute by setting the field.
- Every fixture is registered in **both** `FixtureByName` and `FixtureNames()` or Phase 4 task 4-3's drift check fails, and each inherits §13.4's three assertions automatically — the point of enumerating rather than naming.
- **VHS fails silently on write**, so verify the file's hash changed and retry before trusting or reviewing a capture, since a message state is visible only in the image and there is no functional assertion that would catch a capture that never landed.
- Tapes and images are scaffolding created as work proceeds and cleared at sign-off in Phase 10, while the Go fixture definitions in `internal/capture` are permanent.
- A missing fixture is a blind spot the guard structurally cannot report, since it enumerates whatever exists and absence reads as coverage.

**Context**:
> §13.3: "**The message slot in both states** — the confirm and the failed-commit line. §9.1 warns these may wrap at the minimum width and §14A calls panel wording a layout constraint, so the one copy the spec says might not fit must be capturable. **The narrow degraded panel**, and **the panel at its minimum height with a message live** — §9.8's floor is defined as header + footer + one row + one message row, and that arithmetic is only observable on a frame that renders it."
> §9.1: "At the minimum panel width the slot may wrap to two rows — it is not a list delegate, so wrapping costs nothing to pagination. **It does cost a row of vertical budget, so at the minimum *height* the message is truncated to one line rather than wrapped.**"
> §9.1 (token table): "Message slot — confirm → `text.secondary`, no band | Message slot — failed commit → `⚠` and text in `accent.attention`, **no `bg.attention` band**."
> §13.3: "**A fixture declares its own raw persisted theme keys, independently of `--theme`.**… **The coherence rule, stated generally: `--theme` must name the theme under the cursor.**… **The harness is known to fail silently on write**… **verify a fresh write before trusting or reviewing a capture** — confirm the file's hash changed — and retry on failure."
> §13.2: "**From this feature forward, captures and the tapes that produce them are created as work proceeds, committed while they are being collaborated on, and cleared out after sign-off**… **The deletion covers images and tapes, NOT fixtures.**"

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §13.3, §13.4, §13.2, §9.1, §9.8, §9.13, §14A
