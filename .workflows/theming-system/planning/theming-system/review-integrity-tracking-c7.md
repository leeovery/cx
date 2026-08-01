# Review Tracking: Theming System - Integrity

## Findings

### 1. CLAUDE.md's "Modern Vivid TUI" section still documents the deleted `appearance` pref and two hardcoded canvases, and no task owns it

**Severity**: Important
**Plan Reference**: Phase 10, task `theming-system-10-6` ("CLAUDE.md's five remaining stale entries"); boundary with Phase 3 tasks `theming-system-3-1` and `theming-system-3-3`
**Category**: Task Self-Containment (unowned deliverable in a named artifact)
**Change Type**: add-to-task

**Details**:

The plan divides CLAUDE.md's corrections across three tasks and counts them explicitly. Task 3-1 owns the **`tui/theme` row**; task 3-3 owns the **`tui` row**; task 10-6 owns **five** more — config path resolution, the bootstrap-exempt set, the `prefs` row, the logging section, the capture-harness section — and states the total as "seven of its entries describe the pre-feature world". Each of the three tasks names its scope and explicitly defers the rest ("the config, bootstrap-exempt, prefs, log-count and capture-harness entries are **Phase 10's**").

Two clauses in CLAUDE.md's **"Modern Vivid TUI"** prose section fall outside all seven, and both are falsified by this feature:

- **"Owned mode-matched canvas."** — *"Portal paints its own opaque backdrop on every cell — near-black `#0b0c14` (dark) / near-white `#e1e2e7` (light)"*. After §3.1's split the canvas is the **active theme's `canvas` token**, three built-ins ship and a drop-in supplies its own — so naming two hexes as *the* canvas, and the "mode-matched" framing itself, are the exact claims task 10-7 corrects in the MV spec (§2.9's "two hardcoded canvases" retirement) and task 2-3 already removed from the floor tests.
- **"Light/dark detection (`appearance_gate.go`)."** — *"The `appearance` pref (`auto|light|dark`) overrides — `light`/`dark` pin the canvas and skip both detection and the wait."* Task 5-7 **deletes** `prefs.Appearance`, `parseAppearance`, `LoadAppearance` and `SaveAppearance` with their last caller. The replacement rule is §8.4's: a **constant** nomination skips the gate entirely, a **pair** resolves it once. So this sentence points the next implementing agent at a deleted API as the live canvas control.

The gap is invisible to task 10-6's own acceptance: its grep is `grep -n "WithAppearance\|17 component\|--appearance" CLAUDE.md`, and neither clause matches any of those three strings (they read "The \`appearance\` pref" and a bare hex pair). Verified against the shipped file — CLAUDE.md line 168 carries the canvas bullet and line 170 the detection bullet, and neither is inside the `tui/theme` row, the `tui` row, or any of task 10-6's five sections.

This matters more than ordinary doc drift because the plan itself makes the argument: task 10-6's Problem states *"CLAUDE.md is the file an implementing agent reads first, which is why a stale clause here actively misdescribes the subsystem"*, and §12.6 singles out the `tui` row as "the entry whose staleness is most dangerous" for precisely this reason. The remedy is also already precedented **inside this plan**: task 10-7 instructs "**Sweep the rest of the file for the same falsified facts before editing**, because … §15.2 names the **minimum** set, not a ceiling" for the MV spec. Task 10-6 carries no equivalent clause. No new decision is invented by fixing this — the corrected content is entirely §3.1's per-theme canvas and §8.4's nomination, both already decided and already implemented by Phases 3 and 5.

**Current**:

`planning.md`, Phase 10 **Acceptance** — the CLAUDE.md bullet:

```markdown
- [ ] CLAUDE.md's remaining stale entries are corrected: config path resolution, the bootstrap-exempt set, the `prefs` row, the log component count, and the capture harness section
```

`planning.md`, Phase 10 task table — the `theming-system-10-6` row:

```markdown
| theming-system-10-6 | CLAUDE.md's five remaining stale entries | Phase 3 already corrected the **`tui/theme` row (task 3-1)** and the **`tui` row (task 3-3)**, so this task must not re-edit or contradict either — the `tui` row now carries the standing *"do not drop this guard"* warning re-anchored to the retained **startup** canvas hex; **config path resolution**: `themesDirPath` resolves a *directory*, so it is explicitly **not** a `configFilePath` member and has no one-shot macOS Application Support migration, `PORTAL_THEMES_DIR` joins the env-var set, `cmd/config.go` exposes the **non-migrating** prefs read for doctor, `WithThemePersister` joins the TUI wiring, and `WithAppearance` is **gone**, replaced by the loaded nomination; **bootstrap-exempt set**: `theme` joins the `skipTmuxCheck` list, which is quoted verbatim in that section and therefore goes stale silently; **`prefs` row**: `theme` / `theme_light` / `theme_dark` / `theme_migrated` replace the `Appearance` enum and its tolerant decode, and the row must record that **`appearance` survives on disk as a preserved raw string that is read but never parsed** — omit that and the next implementer deletes the field and erases every user's pin on the first `s` keypress; **logging section**: the count moves **17 → 18** with the `theme` component and its attr keys (`slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected`), stated exactly the way `spawn` and `resolve` are, and noting it is legally emitted from three packages under bind-once-*per-package*; **capture harness section**: `testdata/vhs/` PNGs are **scaffolding, not a durable asset** — there is no visual-regression obligation — while the Go fixture definitions and the harness are permanent, and `capturetool`'s flag is `--theme <slug\|path>` (default `tokyo-night`), not `--appearance`; CLAUDE.md is the file an implementing agent reads first, which is why a stale clause here actively misdescribes the subsystem; it is a **repo file, not a `.workflows/` artifact**, so the completed-unit correction protocol does **not** apply — no corrigendum, no re-index |
```

`phase-10-tasks.md`, task `theming-system-10-6` — heading:

```markdown
### Task 10.6: CLAUDE.md's five remaining stale entries
```

`phase-10-tasks.md`, task `theming-system-10-6` — **Problem** (opening two sentences):

```markdown
**Problem**: §12.6 opens with the reason this matters: "CLAUDE.md is what an implementing agent reads first", and seven of its entries describe the pre-feature world. Phase 3 corrected the two most dangerous ones as it changed the code they describe — the `tui/theme` row (task 3-1) and the `tui` row with its standing *"do not drop this guard"* warning (task 3-3). **Five remain**, and each is a specific hazard rather than untidiness:
```

`phase-10-tasks.md`, task `theming-system-10-6` — **Outcome**:

```markdown
**Outcome**: An agent reading CLAUDE.md first finds the themes directory, the non-migrating prefs read, `WithThemePersister`, `theme` in the exempt set, the prefs schema including the preserved raw `appearance`, an 18-component taxonomy, and a capture harness described as scaffolding with a `--theme` flag.
```

`phase-10-tasks.md`, task `theming-system-10-6` — **Do**, the capture-harness bullet (the last content bullet, immediately before the cross-check bullet):

```markdown
- **Visual capture harness section**: state that `testdata/vhs/` PNGs and tapes are **scaffolding, not a durable asset** — there is no visual-regression obligation — while the Go fixture definitions in `internal/capture` and the harness itself are **permanent** (the swap-and-diff guard drives the fixture renderer and its coverage assertion needs the fixture set to exist). Change the documented flag to **`--theme <slug|path>` (default `tokyo-night`)** in place of `--appearance dark|light`.
```

`phase-10-tasks.md`, task `theming-system-10-6` — **Acceptance Criteria**, the grep criterion:

```markdown
- [ ] `grep -n "WithAppearance\|17 component\|--appearance" CLAUDE.md` returns nothing.
```

**Proposed**:

`planning.md`, Phase 10 **Acceptance** — replace the CLAUDE.md bullet with:

```markdown
- [ ] CLAUDE.md's remaining stale entries are corrected: config path resolution, the bootstrap-exempt set, the `prefs` row, the log component count, the capture harness section, and the "Modern Vivid TUI" section's owned-canvas and light/dark-detection bullets
```

`planning.md`, Phase 10 task table — replace the `theming-system-10-6` row with:

```markdown
| theming-system-10-6 | CLAUDE.md's six remaining stale entries | Phase 3 already corrected the **`tui/theme` row (task 3-1)** and the **`tui` row (task 3-3)**, so this task must not re-edit or contradict either — the `tui` row now carries the standing *"do not drop this guard"* warning re-anchored to the retained **startup** canvas hex; **config path resolution**: `themesDirPath` resolves a *directory*, so it is explicitly **not** a `configFilePath` member and has no one-shot macOS Application Support migration, `PORTAL_THEMES_DIR` joins the env-var set, `cmd/config.go` exposes the **non-migrating** prefs read for doctor, `WithThemePersister` joins the TUI wiring, and `WithAppearance` is **gone**, replaced by the loaded nomination; **bootstrap-exempt set**: `theme` joins the `skipTmuxCheck` list, which is quoted verbatim in that section and therefore goes stale silently; **`prefs` row**: `theme` / `theme_light` / `theme_dark` / `theme_migrated` replace the `Appearance` enum and its tolerant decode, and the row must record that **`appearance` survives on disk as a preserved raw string that is read but never parsed** — omit that and the next implementer deletes the field and erases every user's pin on the first `s` keypress; **logging section**: the count moves **17 → 18** with the `theme` component and its attr keys (`slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected`), stated exactly the way `spawn` and `resolve` are, and noting it is legally emitted from three packages under bind-once-*per-package*; **capture harness section**: `testdata/vhs/` PNGs are **scaffolding, not a durable asset** — there is no visual-regression obligation — while the Go fixture definitions and the harness are permanent, and `capturetool`'s flag is `--theme <slug\|path>` (default `tokyo-night`), not `--appearance`; **"Modern Vivid TUI" section**: the owned-canvas bullet stops naming `#0b0c14`/`#e1e2e7` as *the* canvas (it is the active theme's `canvas` token) and the light/dark-detection bullet stops naming the deleted `appearance` pref as the override (a **constant** nomination skips the gate, a **pair** resolves it once) — this pair is outside all five table-row and section entries above and outside Phase 3's two rows, and task 10-6's `WithAppearance` / `--appearance` grep does not reach it; CLAUDE.md is the file an implementing agent reads first, which is why a stale clause here actively misdescribes the subsystem; it is a **repo file, not a `.workflows/` artifact**, so the completed-unit correction protocol does **not** apply — no corrigendum, no re-index |
```

`phase-10-tasks.md`, task `theming-system-10-6` — replace the heading with:

```markdown
### Task 10.6: CLAUDE.md's six remaining stale entries
```

`phase-10-tasks.md`, task `theming-system-10-6` — replace the **Problem**'s opening two sentences with:

```markdown
**Problem**: §12.6 opens with the reason this matters: "CLAUDE.md is what an implementing agent reads first", and eight of its entries describe the pre-feature world. Phase 3 corrected the two most dangerous ones as it changed the code they describe — the `tui/theme` row (task 3-1) and the `tui` row with its standing *"do not drop this guard"* warning (task 3-3). **Six remain**, and each is a specific hazard rather than untidiness:
```

`phase-10-tasks.md`, task `theming-system-10-6` — replace the **Outcome** with:

```markdown
**Outcome**: An agent reading CLAUDE.md first finds the themes directory, the non-migrating prefs read, `WithThemePersister`, `theme` in the exempt set, the prefs schema including the preserved raw `appearance`, an 18-component taxonomy, a capture harness described as scaffolding with a `--theme` flag, and a "Modern Vivid TUI" section whose canvas comes from the active theme rather than from two hardcoded hexes and a deleted pref.
```

`phase-10-tasks.md`, task `theming-system-10-6` — **Do**: the capture-harness bullet stands unchanged and these two bullets are added immediately after it (before the cross-check bullet):

```markdown
- **"Modern Vivid TUI" section** — two prose bullets that no table row covers and that this feature falsifies outright:
  - **Owned mode-matched canvas.** It currently reads *"Portal paints its own opaque backdrop on every cell — near-black `#0b0c14` (dark) / near-white `#e1e2e7` (light)"*. After §3.1's split the backdrop is the **active theme's `canvas` token** — three built-ins ship and a drop-in supplies its own — so the two hexes stop being *the* canvas and the "mode-matched" framing goes with them (it is the same claim task 10-7 retires from the MV spec's §2.9). The two-layer mechanism, the outer full-terminal fill and its height-budget note are unchanged and stay.
  - **Light/dark detection (`appearance_gate.go`).** It currently reads *"The `appearance` pref (`auto|light|dark`) overrides — `light`/`dark` pin the canvas and skip both detection and the wait."* That pref does not exist after task 5-7. Replace it with §8.4's rule: the loaded **nomination** decides — a **constant** skips the gate entirely and paints from frame one, an **adaptive pair** arms the gate and it resolves exactly once before anything is painted. The detect-or-timeout race, the single-resolution rule, the dark no-answer fallback and the `NO_COLOR` carve-out are unchanged and stay.
- **Sweep the section for the same class before editing.** These two are what was *found*, not necessarily the boundary: task 10-7 applies the same discipline to the MV spec on the grounds that §15.2 names the minimum set rather than a ceiling, and the same holds here. Read the "Modern Vivid TUI" and "Cold-path startup flip" sections and the Architecture opener end to end against the shipped behaviour, surface anything else falsified (the opener's "owned mode-matched canvas" phrasing and the cold-path section's "detect-or-timeout appearance gate" sentence are the obvious candidates), and correct or leave each by decision rather than by omission. **Do not** widen the sweep into the two rows Phase 3 owns.
```

`phase-10-tasks.md`, task `theming-system-10-6` — **Acceptance Criteria**: replace the grep criterion with these three:

```markdown
- [ ] `grep -n "WithAppearance\|17 component\|--appearance" CLAUDE.md` returns nothing.
- [ ] The "Modern Vivid TUI" section's owned-canvas bullet no longer presents `#0b0c14` / `#e1e2e7` as *the* canvas and no longer calls it "mode-matched"; the two-layer fill mechanism and its height-budget note are byte-unchanged.
- [ ] The "Modern Vivid TUI" section's light/dark-detection bullet names the loaded nomination (constant skips the gate, pair resolves it once) and mentions no `appearance` pref; the detect-or-timeout race, the single-resolution rule, the dark fallback and the `NO_COLOR` carve-out are byte-unchanged.
- [ ] The sweep of the "Modern Vivid TUI" and "Cold-path startup flip" sections and the Architecture opener was run, and every further falsified clause it found was corrected or left by an explicit decision.
```

`phase-10-tasks.md`, task `theming-system-10-6` — **Tests**: add after `"no stale appearance wiring survives"`:

```markdown
- `"the MV section no longer names a hardcoded canvas or the appearance pref"` — `grep -n "mode-matched canvas\|#0b0c14\|appearance\` pref" CLAUDE.md` returns nothing inside the "Modern Vivid TUI" section.
```

`phase-10-tasks.md`, task `theming-system-10-6` — **Edge Cases**: add after the "The capture PNGs are **scaffolding, not a durable asset**…" entry:

```markdown
- The **"Modern Vivid TUI" section** is prose rather than a table row, which is why it fell outside both Phase 3's two rows and §12.6's five named entries — and its light/dark-detection bullet names the deleted `appearance` pref as the live canvas control, which is exactly the "actively misdescribes the subsystem" hazard this task exists to close. Task 10-6's `WithAppearance` / `--appearance` grep does not match either clause, so the criteria must name them directly.
- The section is swept the way task 10-7 sweeps the MV spec: §12.6 names the minimum set, not a ceiling, so anything else falsified in the same section is corrected or left by decision rather than by omission — while Phase 3's two rows stay untouched.
```

**Resolution**: Fixed
**Notes**:

---

### 2. `SlotResolution.WasSet` cannot be computed from `ResolveNomination`'s declared inputs, and no consumer reads it

**Severity**: Minor
**Plan Reference**: Phase 5, task `theming-system-5-4` ("Per-slot mode-matched fallback for an unloadable nomination"); consumed (nominally) by Phase 8 task `theming-system-8-3`
**Category**: Acceptance Criteria Quality (a criterion the declared signature cannot satisfy)
**Change Type**: update-task

**Details**:

Task 5-4 declares `SlotResolution.WasSet bool // true when Requested came from prefs rather than the shipped default`, pins it in a Do bullet ("the slot resolves normally with `WasSet=false`") and in an acceptance criterion ("An **unset** slot yields `WasSet=false`, `FellBack=false` and the shipped default's theme").

The function that must populate it takes only a `Setting`:

```go
func (l Loader) ResolveNomination(s Setting, themesDir string) (Resolution, error)
```

and task 5-2 has already **substituted the shipped defaults into `Setting`** before it is handed over ("`("","","")` → pair `{tokyo-night-day, tokyo-night}`"). So `Setting{Light: "tokyo-night-day"}` is byte-identical whether the slot was unset or explicitly set to `tokyo-night-day`, and `ResolveNomination` has no other input — task 5-7 keeps `rawKeys` local at the call site and explicitly does not thread it further. The set-ness is recoverable only from `RawKeys`, which the function never sees. An implementer reaches the assignment, finds nothing to assign from, and must invent a policy: thread `RawKeys` through (rippling into task 8-8's `ResolveNominationFrom`, the `ThemeEnumerator.Resolve` seam, task 8-15's fake and task 9-2's recompute), compare against `DefaultLightSlug` (wrong for a user who set the default explicitly), or drop the field.

Nothing reads it. Task 8-3 — the task the field was carried for — keys the whole badge table on `Requested` alone and says so: "**Key every badge on `SlotResolution.Requested`** … One field, three rows, no branching on `FellBack`." Doctor (task 7-5) reads the raw keys directly, never a `SlotResolution`. Task 9-6's `ResolveSlot` takes the raw slug as a parameter and so needs no flag. The two remaining mentions in task 8-3 are descriptions of an *input state* ("`prefs.json` absent → `WasSet=false` on both slots"), not reads.

Dropping the field is therefore the proportionate fix and is consistent with the plan's own standing discipline — task 3-2 removes `Deps.Appearance` "rather than left alongside", task 5-7 deletes `LoadAppearance` with its last caller, and task 8-8 retires `Deps.ThemeSlots` "with its last consumer" — each on the grounds that an unread field is a second source of truth. This is smaller than the alternative and changes no decided behaviour: `Requested` already carries §9.5's three rows in full.

The path is unreachable as a *bug* (the flag has no reader, so a wrong value harms nothing), which is why this is Minor rather than Important. It is raised because the acceptance criterion is unsatisfiable as written, and an implementer must stop and decide.

**Current**:

Task `theming-system-5-4`, **Do** — the declaration bullet:

```markdown
- Declare in `internal/theme`:
  ```go
  type Slot int // SlotConstant, SlotLight, SlotDark
  type SlotResolution struct {
      Slot      Slot
      Requested string   // the slug that was nominated (shipped default when the slot was unset)
      WasSet    bool     // true when Requested came from prefs rather than the shipped default
      Resolved  string   // the slug actually loaded
      FellBack  bool
      Reason    Reason   // populated iff FellBack
      Theme     Theme
  }
  type Resolution struct {
      Nomination Nomination
      Slots      []SlotResolution // exactly 1 under a constant, exactly 2 (light, dark) under a pair
  }
  func (l Loader) ResolveNomination(s Setting, themesDir string) (Resolution, error)
  ```
```

Task `theming-system-5-4`, **Do** — the unset-slot bullet:

```markdown
- Treat an **unset** slot as ordinary resolution, not a fallback: task 5-2 already substituted the shipped default into `Setting`, so the slot resolves normally with `WasSet=false` and `FellBack=false`. State in-source that this is why unset and unloadable converge with no second mechanism — one rule ("an unset slot holds the shipped default") applied to a slot that is *set but unloadable*.
```

Task `theming-system-5-4`, **Acceptance Criteria** — the unset-slot criterion:

```markdown
- [ ] An **unset** slot yields `WasSet=false`, `FellBack=false` and the shipped default's theme — a virgin install produces **zero** fallbacks.
```

Task `theming-system-5-4`, **Context** — the badge-table line:

```markdown
> §9.5's badge table needs exactly this record: a slot "Set but unloadable" keeps the badge on the **persisted** slug while "the fallback's own row carries no badge", and a "Never set" slot badges the **shipped default's** slug — which is why `WasSet` is carried separately from `Requested`.
```

Task `theming-system-8-3`, **Do** — the badge-key bullet:

```markdown
- **Key every badge on `SlotResolution.Requested`** and comment that this single field *is* §9.5's three-row table: `Requested` is the persisted slug when the slot was set (`WasSet=true`) whether or not it loaded, and the shipped default's slug when it was not (`WasSet=false`). One field, three rows, no branching on `FellBack` — and state explicitly that reading `Resolved` instead would move the badge onto a fallback, which is the bug this task exists to prevent.
```

Task `theming-system-8-3`, **Acceptance Criteria** — the never-set criterion:

```markdown
- [ ] A **never-set** slot badges the shipped default's slug, so a virgin install (`prefs.json` absent → `WasSet=false` on both slots) yields `tokyo-night-day` `● light` and `tokyo-night` `● dark`.
```

**Proposed**:

Task `theming-system-5-4`, **Do** — replace the declaration bullet with:

```markdown
- Declare in `internal/theme`:
  ```go
  type Slot int // SlotConstant, SlotLight, SlotDark
  type SlotResolution struct {
      Slot      Slot
      Requested string   // the slug that was nominated (shipped default when the slot was unset)
      Resolved  string   // the slug actually loaded
      FellBack  bool
      Reason    Reason   // populated iff FellBack
      Theme     Theme
  }
  type Resolution struct {
      Nomination Nomination
      Slots      []SlotResolution // exactly 1 under a constant, exactly 2 (light, dark) under a pair
  }
  func (l Loader) ResolveNomination(s Setting, themesDir string) (Resolution, error)
  ```
  **Carry no "was this slot set?" flag**, and record why in a doc comment on `Requested`. Task 5-2 substitutes the shipped default into `Setting` *before* this function sees it, so `Setting{Light: "tokyo-night-day"}` is identical whether the slot was unset or explicitly set to that slug — the distinction lives only in `RawKeys`, which this function is deliberately not given (task 5-7 keeps the raw keys local). And nothing needs it: task 8-3 keys the entire §9.5 badge table on `Requested` alone ("one field, three rows"), doctor (task 7-5) reads the raw keys directly, and task 9-6's `ResolveSlot` is handed the raw slug as a parameter. A flag that cannot be computed here and that no consumer reads would be a second, wrong source of truth for which slug carries the `●` — the same reason task 3-2 removed `Deps.Appearance` and task 8-8 retires `Deps.ThemeSlots` rather than leaving them alongside.
```

Task `theming-system-5-4`, **Do** — replace the unset-slot bullet with:

```markdown
- Treat an **unset** slot as ordinary resolution, not a fallback: task 5-2 already substituted the shipped default into `Setting`, so the slot resolves normally with `FellBack=false` and its `Requested` is the shipped default's slug. State in-source that this is why unset and unloadable converge with no second mechanism — one rule ("an unset slot holds the shipped default") applied to a slot that is *set but unloadable* — and that it is also why this function needs no set-ness flag: both states resolve identically and `Requested` already says which slug the badge sits on.
```

Task `theming-system-5-4`, **Acceptance Criteria** — replace the unset-slot criterion with:

```markdown
- [ ] An **unset** slot yields `FellBack=false`, a `Requested` equal to the shipped default's slug and that theme — a virgin install produces **zero** fallbacks.
- [ ] `SlotResolution` declares no set-ness flag: an unset slot and a slot explicitly set to the same shipped-default slug produce **identical** `SlotResolution` values, which is what makes the record computable from `Setting` alone.
```

Task `theming-system-5-4`, **Tests** — add after `"it treats an unset slot as a default, not a fallback"`:

```markdown
- `"it produces the same record whether a default slot was set or unset"` — `TestResolveNomination_SetAndUnsetDefaultsAreIndistinguishable`
```

Task `theming-system-5-4`, **Edge Cases** — add after "An **unset** slot is not a fallback at all but the same default rule, so unset and unloadable converge with no second mechanism.":

```markdown
- The record carries **no set-ness flag**. `Setting` already has the shipped defaults substituted in (task 5-2), so "was this slot set?" is not derivable here, and nothing consumes it — task 8-3's badge table keys on `Requested` alone, doctor reads the raw keys directly, and task 9-6's `ResolveSlot` takes the raw slug as a parameter. Adding a flag that cannot be computed and that no consumer reads would be a second, wrong source of truth for which slug carries the `●`.
```

Task `theming-system-5-4`, **Context** — replace the badge-table line with:

```markdown
> §9.5's badge table needs exactly this record: a slot "Set but unloadable" keeps the badge on the **persisted** slug while "the fallback's own row carries no badge", and a "Never set" slot badges the **shipped default's** slug — which is why `Requested` (the pre-fallback slug) is carried separately from `Resolved`, and why one field covers all three rows with no set-ness flag.
```

Task `theming-system-8-3`, **Do** — replace the badge-key bullet with:

```markdown
- **Key every badge on `SlotResolution.Requested`** and comment that this single field *is* §9.5's three-row table: `Requested` is the persisted slug when the slot was set — whether or not it loaded — and the shipped default's slug when it was not, because task 5-2 substitutes the default into the `Setting` before resolution. One field, three rows, no branching on `FellBack` and no set-ness flag to consult — and state explicitly that reading `Resolved` instead would move the badge onto a fallback, which is the bug this task exists to prevent.
```

Task `theming-system-8-3`, **Acceptance Criteria** — replace the never-set criterion with:

```markdown
- [ ] A **never-set** slot badges the shipped default's slug, so a virgin install (`prefs.json` absent → both slots' `Requested` are the shipped defaults) yields `tokyo-night-day` `● light` and `tokyo-night` `● dark`.
```

**Resolution**: Fixed
**Notes**:
