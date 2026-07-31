# Review Tracking: Theming System - Integrity

## Findings

### 1. Task 8-10's close re-lays-out the page to "reclaim the panel's frame", contradicting tasks 8-6 and 8-11's non-reflow rule

**Severity**: Important
**Plan Reference**: Phase 8, task `theming-system-8-10` ("`Esc` discards the preview onto the resolved persisted state"); contradicts tasks `theming-system-8-6` and `theming-system-8-11`
**Category**: Dependencies and Ordering (cross-task contract contradiction)
**Change Type**: update-task

**Details**:
Task 8-6 pins the overlay's defining property: the main screen is **deliberately not re-laid-out** while the panel is open. Its acceptance criterion requires "the base view composed at the **unreduced** content width (a footer entry is cut mid-label rather than reflowed)"; task 8-11 repeats it for the resize path ("The main screen is deliberately **not** re-laid-out to the reduced width (§9.1)"); and task 8-16's `theme-panel-projects` frame asserts "the Projects footer beneath cut mid-label by the overlay".

Task 8-10's close step then says the opposite. Its Do step 5 instructs the implementer to "re-sync the active page's layout **so the list reclaims the panel's frame**", and its acceptance criterion asserts "the page list is re-sized so **it reclaims the full frame**". Under 8-6's rule there is no frame to reclaim — the page list was never reduced, because the panel is an opaque layer over a base composed at full content width. The repo's actual helper confirms it: `resyncSessionLayout` (`internal/tui/model.go`) re-applies `applySessionListSize(m.contentWidth(), m.contentHeight())` for the current **notice-band** state and knows nothing of a panel.

The two readings are mutually exclusive and the wrong one is the natural one. An implementer taking 8-10's criterion literally must make the reclaim observable, which requires the panel to have *reduced* the page's frame at open — reintroducing exactly the reflow §9.1 refuses, breaking task 8-6's cut-mid-label criterion and task 8-16's Projects frame, and putting a re-layout on the close path §11.1 names as the one that matters most. Left as written, one of two tasks' acceptance criteria must fail.

**Current**:

Task 8.10 — **Do**, step 5:

```
  5. **Then** discard: `open=false`, zero `enumeration`, `union`, `badges` and `message`, and re-sync the active page's layout so the list reclaims the panel's frame. Order matters — resolution reads the enumeration, so the discard is last.
```

Task 8.10 — **Acceptance Criteria** (one criterion):

```
- [ ] The panel's list, delegate, badges and message are all cleared on close, and the page list is re-sized so it reclaims the full frame.
```

Task 8.10 — **Tests** (tail of the list):

```
- `"it never quits"` — `TestPanelClose_EscDoesNotQuit` (both pages)
- `"it is the single close path"` — `TestPanelClose_ForcedCloseUsesTheSameFunction` (task 8-11's forced close asserted to route here)
```

**Proposed**:

Task 8.10 — **Do**, step 5:

```
  5. **Then** discard: `open=false`, zero `enumeration`, `union`, `badges` and `message`. Order matters — resolution reads the enumeration, so the discard is last. **The page beneath needs no re-layout on close, because it was never reduced on open** — task 8-6 composites the panel over a base view laid out at the **unreduced** content width and task 8-11 keeps that true across a resize, so there is no frame to reclaim. State that in-source as a negative: a reader who "completes" the close with a reclaim step is one step from adding the open-time reduction that would justify it, which reflows the surface being previewed and falsifies task 8-6's cut-mid-label criterion and task 8-16's `theme-panel-projects` frame. If a notice band was raised or cleared while the panel was open, the existing `resyncSessionLayout` / its Projects sibling already handles that on its own path (task 8-12) — closing the panel adds nothing to it.
```

Task 8.10 — **Acceptance Criteria** (the one criterion, replaced by two):

```
- [ ] The panel's list, delegate, badges and message are all cleared on close.
- [ ] Neither open nor close re-lays-out the page beneath: at a fixed terminal size the active page list's width and height are byte-identical before the panel opens, while it is open, and after it closes.
```

Task 8.10 — **Tests** (tail of the list, one entry added):

```
- `"it never quits"` — `TestPanelClose_EscDoesNotQuit` (both pages)
- `"it re-lays-out nothing on the page beneath"` — `TestPanelClose_PageLayoutUnchangedAcrossOpenAndClose` (list size compared before open, while open, and after close)
- `"it is the single close path"` — `TestPanelClose_ForcedCloseUsesTheSameFunction` (task 8-11's forced close asserted to route here)
```

**Resolution**: Pending
**Notes**:

---

### 2. No task wires `overlayThemePanel` into `Model.View()`, so the panel is renderable but never reaches the screen

**Severity**: Minor
**Plan Reference**: Phase 8, task `theming-system-8-6` ("The slide-over surface — overlay, chrome and the pinned directory row")
**Change Type**: add-to-task
**Category**: Task Self-Containment

**Details**:
Task 8-6 declares `renderThemePanel` and `overlayThemePanel` and specifies the compositor call precisely, and task 8-7 sets `open = true` on the keypress — but no task has a Do step or an acceptance criterion for the one line that joins them: `Model.View()` calling the overlay when `p.open` is true. Every acceptance criterion in 8-6 is function-level (`renderThemePanel` returns N lines; `overlayThemePanel` leaves base cells intact), and 8-7's criteria are about directory reads, badges and key swallowing. Both tasks can pass in full with the panel invisible.

The gap is real rather than notional because the plan assigns ownership at exactly this granularity everywhere else — task 8-6 itself pins "the panel's list delegate is built only through `m.themeRowDelegate()`, no second construction site exists", and task 8-5 pins the footer-height helper. It also matters for ordering: from task 8-9 onward the plan asserts against "the composed frame" (8-9's arrow diff, 8-10's byte-compare of the pre-open frame, and every fixture in 8-15 / 8-16 / 9-12), all of which are unwritable until `View` composites. Placing the wiring in 8-6 is what makes 8-6's own Outcome ("overlaid at the right edge it leaves the Sessions/Projects list fully visible behind it") true at the moment the task lands, and it is drivable there by setting `open` directly — the same technique 8-6 already uses for the message slot's height recompute.

**Current**:

Task 8.6 — **Do**, the compositing bullet:

```
- **Composite, do not re-lay-out.** `overlayThemePanel` mirrors `overlayHelpOnPreview` exactly: `lipgloss.NewLayer(base).X(0).Y(0).Z(0)` and `lipgloss.NewLayer(panel).X(contentW - panelWidth).Y(0).Z(1)` through `lipgloss.NewCompositor(...).Render()`. State in-source that the main screen is deliberately **not** re-laid-out while the panel is open — that is what keeps the swap the O(1) restyle of §11.1 and keeps the surface being previewed from reflowing under the user — so the overlay cuts wherever its left border falls, mid-label included (`x proje▏`), which is **not** a §14.4 violation (§14.4 governs how the footer lays *itself* out as the terminal narrows; the panel is an opaque layer over a footer that laid out at full width).
```

Task 8.6 — **Acceptance Criteria** (the overlay criterion):

```
- [ ] `overlayThemePanel` leaves every base cell to the left of the panel byte-identical and replaces every cell under it, with the base view composed at the **unreduced** content width (a footer entry is cut mid-label rather than reflowed).
```

Task 8.6 — **Tests** (the two overlay entries):

```
- `"it composites over the page without reflowing it"` — `TestThemePanel_OverlayDoesNotRelayoutTheBase`
- `"it cuts the covered footer mid-label"` — `TestThemePanel_OverlayCutsMidLabel`
```

**Proposed**:

Task 8.6 — **Do**, the compositing bullet, plus one new bullet immediately after it:

```
- **Composite, do not re-lay-out.** `overlayThemePanel` mirrors `overlayHelpOnPreview` exactly: `lipgloss.NewLayer(base).X(0).Y(0).Z(0)` and `lipgloss.NewLayer(panel).X(contentW - panelWidth).Y(0).Z(1)` through `lipgloss.NewCompositor(...).Render()`. State in-source that the main screen is deliberately **not** re-laid-out while the panel is open — that is what keeps the swap the O(1) restyle of §11.1 and keeps the surface being previewed from reflowing under the user — so the overlay cuts wherever its left border falls, mid-label included (`x proje▏`), which is **not** a §14.4 violation (§14.4 governs how the footer lays *itself* out as the terminal narrows; the panel is an opaque layer over a footer that laid out at full width).
- **Wire it into `Model.View()`.** Gate on `p.open`: render the panel at the current content height, then composite it over the already-composed page view as the **last** layer, after the outer full-terminal canvas fill — otherwise the fill paints over it. Nothing sets `open` in this task (task 8-7's `t` does), so drive it in tests by setting the field directly, exactly as the message slot's height recompute is driven above. Without this step the panel is a block that renders correctly and never reaches the screen, and every later assertion made against "the composed frame" — task 8-9's arrow diff, task 8-10's byte-compare of the pre-open frame, and every fixture in tasks 8-15 / 8-16 / 9-12 — has nothing to assert against.
```

Task 8.6 — **Acceptance Criteria** (the overlay criterion, plus two added):

```
- [ ] `overlayThemePanel` leaves every base cell to the left of the panel byte-identical and replaces every cell under it, with the base view composed at the **unreduced** content width (a footer entry is cut mid-label rather than reflowed).
- [ ] `Model.View()` composites the panel when `open` is true (set directly in the test) and is byte-identical to the pre-panel view when it is false.
- [ ] The panel is the last layer composed, so the outer full-terminal canvas fill never paints over any panel cell.
```

Task 8.6 — **Tests** (the two overlay entries, plus one added):

```
- `"it composites over the page without reflowing it"` — `TestThemePanel_OverlayDoesNotRelayoutTheBase`
- `"it cuts the covered footer mid-label"` — `TestThemePanel_OverlayCutsMidLabel`
- `"it composites the panel into the model's view when open"` — `TestThemePanel_ViewCompositesWhenOpen` (open set directly; the closed frame byte-identical to the pre-panel view)
```

**Resolution**: Pending
**Notes**:

---

### 3. Task 9-9's forced-close rule reads the outstanding flag after the close has already discharged it

**Severity**: Minor
**Plan Reference**: Phase 9, task `theming-system-9-9` ("Closing with a failure outstanding raises and discharges the report")
**Change Type**: update-task
**Category**: Acceptance Criteria Quality (unstated ordering hazard)

**Details**:
Task 9-9 gives `closeThemePanel` a post-close hook that raises the report **and clears `m.themeCommitFailed`** — the discharge is part of raising. It then instructs the forced-close path to "raise the geometry flash **only when nothing is outstanding**". Composed in the order the sentence implies (close, then check the flag), the check always reads `false`, because the hook it just ran cleared it — so the geometry flash is raised unconditionally and overwrites the report the hook put in the band a moment earlier. The single-slot band then shows the geometry copy and drops the commit report, which is precisely the outcome the task exists to prevent and on the one path where the user cannot reopen the panel to retry.

The task's acceptance criterion pins the correct *outcome*, so `TestCloseReport_ForcedCloseCommitFlashWins` would eventually catch it — but the Do step as written describes the failing shape, and this plan's standing practice is to name a naive implementation where one exists (task 8-10 does exactly that for the snapshot-at-open close, task 9-2 for the index-anchored cursor). Naming the ordering costs one clause and removes the trap.

**Current**:

Task 9.9 — **Do**, the forced-close bullet:

```
- **Forced close: the commit flash wins.** In task 8-11's below-floor resize path, raise the geometry flash **only when nothing is outstanding**; when a failure is outstanding the close's own report is raised instead and the state is discharged. Record the reasoning in-source: the band has one slot and the two report different things — a geometry event the user can see for themselves (their terminal just got smaller and the panel vanished) versus an unsaved setting they must act on. Losing the geometry flash costs nothing; losing the commit flash on the one path where the user cannot reopen the panel to retry is exactly the failure §9.13 closes.
```

**Proposed**:

Task 9.9 — **Do**, the forced-close bullet:

```
- **Forced close: the commit flash wins, and the flag is read *before* the close.** In task 8-11's below-floor resize path, capture `m.themeCommitFailed` **before** calling `closeThemePanel`, and raise the geometry flash only when that captured value was false; when it was true the close's own hook has already raised the report and discharged the state, so the resize path raises nothing further. Pin the ordering in-source, because the naive shape is wrong in a way no reading of the sentence catches: the hook discharges the flag *as part of* raising the report, so a post-close `if !m.themeCommitFailed { raise geometry }` always sees false and overwrites the report the hook just placed in the single-slot band. Record the reasoning too: the band has one slot and the two report different things — a geometry event the user can see for themselves (their terminal just got smaller and the panel vanished) versus an unsaved setting they must act on. Losing the geometry flash costs nothing; losing the commit flash on the one path where the user cannot reopen the panel to retry is exactly the failure §9.13 closes.
```

**Resolution**: Pending
**Notes**:
