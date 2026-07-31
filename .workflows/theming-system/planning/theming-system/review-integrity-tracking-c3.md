# Review Tracking: Theming System - Integrity

## Findings

### 1. The panel's open-time width never applies the width ladder, so the degraded band is only reachable by resizing

**Severity**: Important
**Plan Reference**: Phase 8, task `theming-system-8-11` ("Panel geometry — degrade between preferred and minimum, refuse below the floor"); consumed by task `theming-system-8-16`
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
Task 8-7's open sequence builds the panel's list "at task 8-6's preferred width", which is correct at *that* point in the sequence because the ladder does not exist yet — task 8-6 explicitly declares the two constants and defers the choice: "**task 8-11 owns choosing between them and the refusal below the minimum** — this task renders at whatever width it is handed."

Task 8-11 then introduces `themePanelWidthFor(contentW int) (w int, ok bool)` and wires it into exactly **one** caller: the resize path ("**Resize while open — degrade in place.** In the `tea.WindowSizeMsg` path, when the panel is open: re-run the ladder…"). Nothing re-points the **open** path onto it, and no acceptance criterion covers an open-time width.

Read literally, the panel opens at 30 columns on every terminal that clears the floor and shrinks only if the user subsequently resizes. That contradicts task 8-11's own Outcome — "The panel takes its preferred width on a normal terminal, **shrinks toward its minimum as the terminal narrows**, refuses to open (with a flash) below the floor, degrades in place on resize" — where "shrinks as the terminal narrows" and "degrades in place on resize" are stated as two different things.

It also breaks a downstream task outright. Task 8-16's `theme-panel-narrow` frame is "the only observable check on the width ladder between preferred and minimum" and is "captured through a tape whose terminal width lands the panel in the **degraded band**". A fixture is a one-shot render that opens through `captureKeys` (task 8-15: "declare `captureKeys` … as a single `t` press") and never resizes — so under the literal reading the frame renders at 30 columns and its acceptance ("renders the panel at a width strictly between `themePanelMinWidth` and `themePanelPreferredWidth`") is unreachable by construction.

The ladder is already a pure function of `contentW` and task 8-11 already owns "one predicate, two callers" for the floor; this is the same shape applied to the width, and nothing decided changes.

**Current**:

Task `theming-system-8-11`, **Do** — the third and fourth bullets:

- **One predicate, two callers**: `themePanelFloor(contentW, contentH int, dirUnusable bool) (dim themePanelDim, ok bool)` returning which dimension failed (`dimWidth` / `dimHeight`, width checked first) so both the entry flash (task 8-13) and the resize flash below select their copy from the same result. Compute it once; do not let task 8-13 re-derive it.
- **Resize while open — degrade in place.** In the `tea.WindowSizeMsg` path, when the panel is open: re-run the ladder and the body-height arithmetic and `SetSize` the panel's list, exactly as the two page lists are re-sized. The main screen is deliberately **not** re-laid-out to the reduced width (§9.1) — the panel is composited over a page that laid out at full width, so a panel width change never reflows the surface being previewed.

**Proposed**:

Task `theming-system-8-11`, **Do** — insert a bullet between them:

- **One predicate, two callers**: `themePanelFloor(contentW, contentH int, dirUnusable bool) (dim themePanelDim, ok bool)` returning which dimension failed (`dimWidth` / `dimHeight`, width checked first) so both the entry flash (task 8-13) and the resize flash below select their copy from the same result. Compute it once; do not let task 8-13 re-derive it.
- **Apply the ladder at open, not only on resize.** Re-point task 8-7's open sequence — which builds the panel's list "at task 8-6's preferred width" purely because the ladder did not exist yet — onto `themePanelWidthFor(contentW)`, so the width a panel *opens* at and the width it degrades to on resize are the same function of the same input. Without this the panel opens at `themePanelPreferredWidth` on every terminal that clears the floor and narrows only if the user happens to resize afterwards, which contradicts §9.8's *staged* shrink and makes task 8-16's `theme-panel-narrow` frame uncapturable: a fixture opens through `captureKeys` and never resizes, so the degraded band is only ever entered at open. `ok == false` is unreachable at this call site because task 8-13's gate already refused below the floor — clamp to `themePanelMinWidth` defensively rather than branching, the same way the floor predicate's other caller treats an impossible result.
- **Resize while open — degrade in place.** In the `tea.WindowSizeMsg` path, when the panel is open: re-run the ladder and the body-height arithmetic and `SetSize` the panel's list, exactly as the two page lists are re-sized. The main screen is deliberately **not** re-laid-out to the reduced width (§9.1) — the panel is composited over a page that laid out at full width, so a panel width change never reflows the surface being previewed.

Task `theming-system-8-11`, **Acceptance Criteria** — add after "`themePanelWidthFor` returns the preferred width on a wide terminal, a value strictly between minimum and preferred across a band of narrowing widths (monotone non-increasing), the minimum at the bottom of the band, and `ok=false` below it.":

- [ ] The panel **opens** at `themePanelWidthFor(contentW)`: on a terminal inside the degraded band the very first rendered frame is already narrower than `themePanelPreferredWidth`, with no `tea.WindowSizeMsg` resize involved.
- [ ] The open width and the post-resize width agree for the same content width — one function, two callers, asserted across a single table.

Task `theming-system-8-11`, **Tests** — add after `"it refuses below the minimum width"`:

- `"it opens at the ladder width"` — `TestPanelGeometry_OpenUsesTheWidthLadder` (degraded-band terminal; first frame narrower than preferred; no resize)
- `"it agrees between open and resize"` — `TestPanelGeometry_OpenAndResizeWidthsAgree`

Task `theming-system-8-11`, **Edge Cases** — add after "The multi-select precedent (proactive block at entry) deliberately does **not** transfer, because that is a capability *absence* and this is a space *shortage*.":

- The ladder applies at **open** as well as on resize. Task 8-7 opened at the preferred width only because the ladder did not exist yet, and leaving it there would mean the panel never renders in the degraded band except after a resize — a state no fixture can reach, since task 8-16's narrow frame opens through `captureKeys` and never resizes.

**Resolution**: Fixed
**Notes**:

---

### 2. The badge source moves to `Resolve` at task 8-8, but the injected slot record is left alive and the panel fixtures are wired to it

**Severity**: Important
**Plan Reference**: Phase 8, tasks `theming-system-8-8` ("Opening lands the cursor on the theme actually rendering") and `theming-system-8-15` ("Panel fixture inputs and the two setting-state frames")
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
Task 8-7 injects `Deps.ThemeSlots []theme.SlotResolution` as the panel's badge source and pins it in acceptance: "Badges render on the first open from the injected `[]SlotResolution` with no additional resolution work." Its own ambiguity note anticipates the handover — "Injecting `Deps.ThemeSlots` is the minimal way to satisfy that at the *first* open, before task 8-8's open-time re-resolution exists to produce a fresher record. Record the choice in a source comment so task 8-8 **replaces** rather than duplicates it."

Task 8-8 then performs the replacement: "use the returned `Resolution` for three things — refresh `badges` from its `Slots` (task 8-3), select the in-force member, and anchor the cursor. Replace task 8-7's injected `[]SlotResolution` as the panel's badge source from this point on (the injected value remains the pre-open truth for construction)."

The parenthetical does not hold. Badges exist only while the panel is open, and the panel only opens through 8-8's sequence — there is no construction-time badge to be the "pre-open truth" of. After 8-8, `Deps.ThemeSlots` and `WithThemeSlots` have **no reader at all**, and no task states their fate. The plan is deliberate about exactly this class everywhere else: task 3-2 removes `Deps.Appearance` / `WithAppearance` "rather than left alongside, because a dead option is a second injection path the harness and production could diverge on", and task 5-7 deletes `LoadAppearance` with its last caller because "a dead read path is a second source of truth".

The consequence is not cosmetic, because task 8-15 authors the fixture half against the dead field: "`themeUnion theme.Union` + `themeEnumeration theme.Enumeration` fed to a **`fakeThemeEnumerator`** implementing the seam's `Open` / `Reassemble` / `Resolve` from the declared values with **no I/O**; `themeSlots []theme.SlotResolution` → `Deps.ThemeSlots` (task 8-3's badge source)". Only the union and the enumeration are named as the fake's inputs, so the fake's `Resolve` has nothing to populate `Resolution.Slots` from — and under 8-8's rule that is precisely what the badges derive from. Followed literally, `theme-panel-adaptive-pair` renders **no badges**, failing its own acceptance ("renders two badge rows — `● light` on `tokyo-night-day`, `● dark` on `nord`"), and `theme-panel-constant-previewing` loses the bare `●` that is the entire content of the frame.

That matters more than a normal fixture defect: §9.14 records that assigning a theme to a light/dark slot from inside a picker was found in no surveyed tool, so these two images are "the only reference that exists" for the slot half of the vocabulary.

**Current**:

Task `theming-system-8-8`, **Do** — the fourth bullet:

- **On open, after the enumeration**: derive `Setting` from the raw keys, call `Resolve`, and use the returned `Resolution` for three things — refresh `badges` from its `Slots` (task 8-3), select the in-force member, and anchor the cursor. Replace task 8-7's injected `[]SlotResolution` as the panel's badge source from this point on (the injected value remains the pre-open truth for construction).

Task `theming-system-8-15`, **Do** — the first bullet:

- **Add the four fixture inputs** to `internal/capture/fixtures.go`, following the existing `initial*` seed idiom: `themeKeys theme.RawKeys` → `Deps.ThemeKeys` (task 8-7's constructor slot); `themeUnion theme.Union` + `themeEnumeration theme.Enumeration` fed to a **`fakeThemeEnumerator`** implementing the seam's `Open` / `Reassemble` / `Resolve` from the declared values with **no I/O**; `themeSlots []theme.SlotResolution` → `Deps.ThemeSlots` (task 8-3's badge source); and `initialThemeCursor string` → `Deps.InitialThemeCursor`, the row identity the panel's cursor lands on.

**Proposed**:

Task `theming-system-8-8`, **Do** — replace the fourth bullet with these two:

- **On open, after the enumeration**: derive `Setting` from the raw keys, call `Resolve`, and use the returned `Resolution` for three things — refresh `badges` from its `Slots` (task 8-3), select the in-force member, and anchor the cursor.
- **Retire task 8-7's injected slot record with its last consumer.** `Resolve` supersedes it at the only moment badges exist — the panel is closed until this sequence runs, so there is no construction-time badge for the injected value to be the truth of — which leaves `Deps.ThemeSlots` and `WithThemeSlots` with no reader. **Delete both here rather than leaving them alongside**, the same discipline task 3-2 applied to `Deps.Appearance` / `WithAppearance` and task 5-7 to `LoadAppearance`: a dead injection is a second source of truth for which slug carries the `●`, and it is the one the fixtures would otherwise be authored against. `cmd/open.go` stops passing the record (task 8-7's `RawKeys` slot is unaffected and stays); the production adapter's `Resolve` is where construction's resolution rules now reach the panel. A fixture declares its slot record through the faked seam's `Resolve` return instead — task 8-15 owns that wiring.

Task `theming-system-8-8`, **Acceptance Criteria** — add after "The cursor is anchored by identity: inserting a row above the target before the anchor runs still lands the cursor on the target.":

- [ ] `Deps.ThemeSlots` and `WithThemeSlots` no longer exist — a source guard proves `internal/tui` declares neither — and every `●` rendered on any open derives from a `Resolve` result rather than an injected record.

Task `theming-system-8-8`, **Edge Cases** — add after "The `●` **stays on the persisted slug** while the cursor sits on the fallback — exactly the split §9.5 draws, `●` being what is *set* and the cursor what is *previewed*.":

- Task 8-7's injected `[]SlotResolution` is **retired here**, not kept alongside: badges exist only while the panel is open and the panel only opens through this sequence, so `Resolve` is the sole badge source from this point and a surviving `Deps` field would be a second, staler one — and the one a fixture author would reach for.

Task `theming-system-8-15`, **Do** — replace the first bullet with:

- **Add the four fixture inputs** to `internal/capture/fixtures.go`, following the existing `initial*` seed idiom: `themeKeys theme.RawKeys` → `Deps.ThemeKeys` (task 8-7's constructor slot); `themeUnion theme.Union` + `themeEnumeration theme.Enumeration` + `themeSlots []theme.SlotResolution` **all** fed to a **`fakeThemeEnumerator`** implementing the seam's `Open` / `Reassemble` / `Resolve` from the declared values with **no I/O**, where `Resolve` returns a `theme.Resolution` carrying the declared `themeSlots` — that return **is** the panel's badge source after task 8-8 retired the injected record, so a fixture that declared its slots anywhere else renders no `●` on any row and the adaptive-pair frame loses its entire subject; and `initialThemeCursor string` → `Deps.InitialThemeCursor`, the row identity the panel's cursor lands on.

Task `theming-system-8-15`, **Acceptance Criteria** — add after "A fixture can declare all four inputs and render the panel with no real themes directory and no prefs file.":

- [ ] The fake enumerator's `Resolve` returns the fixture's declared `themeSlots`, and a fixture that declares none renders no badge on any row — asserted directly, so the badge path cannot silently go dark on the two frames whose subject it is.

Task `theming-system-8-15`, **Edge Cases** — add after "The raw keys are declared **independently of `--theme`** because `capturetool` always passes the constant shape and §8.2 makes a non-empty `theme` render a bare `●` with no slot badges — a fixture built from the nomination alone could only ever produce that one state, leaving the adaptive-pair frame unreachable.":

- The declared slot record reaches the panel through the **faked seam's `Resolve` return**, not a `Deps` field: task 8-8 retired the injected record with its last consumer, so a fixture wired to the old slot would render no badge on any row — and the badges are the entire subject of both setting-state frames, which §9.14 makes the only reference that exists for the slot half.

**Resolution**: Fixed
**Notes**:

---

### 3. The post-commit recompute discards `Resolve`'s error, contradicting the degrade policy that names it

**Severity**: Minor
**Plan Reference**: Phase 9, task `theming-system-9-2` ("The post-commit recompute — rows, order, badges and the identity-anchored cursor")
**Category**: Acceptance Criteria Quality
**Change Type**: update-task

**Details**:
Task 8-8 pins the `Resolve` error policy exactly once and names its three consumers so they cannot diverge: "on a non-nil error leave the badges, the active theme and the cursor exactly as they were, carry on with the union already in hand, and write nothing… State in-source that this one policy governs **every** panel call site of `Resolve` — this task's open, task 8-10's close and **task 9-2's recompute** — so the three cannot each invent their own."

Task 8-10 honours it explicitly ("A non-nil error takes task 8-8's degrade policy: skip steps 3 and 4, leaving the active theme exactly as it is, and fall through to step 5 so the panel still closes"). Task 9-2 does not mention it at all, and its code sketch actively contradicts it: step 3 reads `res, _ := m.themeEnumerator.Resolve(…)` and hands `res.Slots` straight to `theme.Badges`.

Followed literally, an error yields a zero `Resolution`, and task 8-3 pins `Badges`'s behaviour for that input — "A nil or empty slice returns an empty map with no panic" — so **every `●` disappears from the panel at the moment the user committed one**. That is the inverse of "leave the badges exactly as they were", and it is the marker lying in the one direction §9.13 spends a whole rule preventing: task 9-7's "the `●` **does not move**, because the marker means 'what is persisted' and would be lying if it moved" only means anything while the marker is otherwise trustworthy.

The path is unreachable in a correctly built binary — task 8-8 notes the fatal is "what Phase 2 task 2-8's build-time guarantee makes unreachable" — so this is polish rather than a live defect. It is raised because it is a *contradiction* between two tasks rather than an omission, and the contradicting text is the one line an implementer copies verbatim.

**Current**:

Task `theming-system-9-2`, **Do** — step 3 of the ordered steps:

  3. `res, _ := m.themeEnumerator.Resolve(p.enumeration, theme.ResolveSetting(...))` for the **badges** only — `theme.Badges(res.Slots)` (task 8-3). The re-resolution never selects a new active member and never calls `ApplyTheme`.

**Proposed**:

Task `theming-system-9-2`, **Do** — replace step 3 with:

  3. `res, err := m.themeEnumerator.Resolve(p.enumeration, theme.ResolveSetting(...))` for the **badges** only — `theme.Badges(res.Slots)` (task 8-3). The re-resolution never selects a new active member and never calls `ApplyTheme`. On a **non-nil error** take task 8-8's degrade policy verbatim — the one policy governing all three panel call sites — and **keep the existing badge map** rather than deriving from a zero `Resolution`: `theme.Badges` returns an empty map for an empty slice, so a discarded error would wipe every `●` off the panel at the exact moment the user committed one, the marker lying in the direction §9.13's "a failed commit does not move the `●`" rule exists to forbid. Steps 2, 4 and 5 still run on that path — they read the mutated keys and the retained enumeration, not the resolution — so the rows still re-derive, re-sort and re-anchor.

Task `theming-system-9-2`, **Acceptance Criteria** — add after "A commit of the slug already persisted produces a byte-identical row set, badge map and cursor index.":

- [ ] A `Resolve` returning task 5-6's fatal during a recompute leaves the **existing** badge map in place — driven through the seam with an error-returning fake — while the rows still re-derive, re-sort and re-anchor from the mutated keys.

Task `theming-system-9-2`, **Tests** — add after `"it is idempotent for an unchanged commit"`:

- `"it keeps the badges when the re-resolution errors"` — `TestPanelRecompute_ResolveErrorKeepsBadges`

Task `theming-system-9-2`, **Edge Cases** — add after "Badges re-derive through task 8-3's table off a re-resolution against the **retained** enumeration.":

- The badge re-resolution takes task 8-8's degrade policy on a non-nil error and the **existing** badge map stands. Discarding the error and deriving from a zero `Resolution` returns an empty badge map and wipes every `●` at the moment the user committed one; task 8-8 names this recompute as one of the three call sites that single policy governs.

**Resolution**: Fixed
**Notes**:

---
