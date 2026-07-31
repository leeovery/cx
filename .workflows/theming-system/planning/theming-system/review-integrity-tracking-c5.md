# Review Tracking: Theming System - Integrity

## Findings

### 1. Doctor's unresolved-themes-directory early return suppresses the persisted-slug advisory, contradicting task 7-5

**Severity**: Important
**Plan Reference**: Phase 7, task `theming-system-7-3` ("Scan the themes directory into per-file advisories"); contradicts task `theming-system-7-5` ("The unresolvable-persisted-theme line off the non-migrating prefs read")
**Category**: Dependencies and Ordering (cross-task contract contradiction)
**Change Type**: update-task

**Details**:

Task 7-3 instructs `collectThemeAdvisories` to `return early with nil when deps.ThemesDir == ""`, and pins that behaviour with the acceptance criterion "A `themesDirPath()` failure yields zero advisories".

Task 7-5 then declares its producer is "called from `collectThemeAdvisories`", and states in its edge cases that "An **empty** `deps.ThemesDir` (unresolved path) still resolves built-ins from the embedded set and returns `not found` for a drop-in slug, composing no path." Task 7-6 restructures `collectThemeAdvisories` into the three-region assembly and never revokes the early return.

So the two tasks disagree about the same function. Following 7-3 as written, an unresolved themes directory short-circuits the whole assembly and the persisted-slug producer never runs — which silently removes the line for **the failure the user is most likely to hit** (§12.2: "the failure a user is most likely to hit is not a stray file in `themes/` — it is *the theme they chose no longer applying*"). The persisted-slug resolution needs no directory at all: `ResolveByName` charset-checks, then resolves the embedded set, and 7-5 explicitly requires an empty directory string to compose no path and yield `not found`.

The fix is to scope the skip to the **directory scan** rather than to `collectThemeAdvisories`, and to re-word 7-3's acceptance criterion so it pins the scan's silence rather than the whole block's — with a matching criterion on 7-5 so the boundary is asserted from both sides rather than only implied.

**Current**:

In `theming-system-7-3`, **Do** (second bullet):

```markdown
- Create `cmd/doctor_theme.go` with `func collectThemeAdvisories(deps *DoctorDeps) []advisory`, the single entry point tasks 7-4/7-5/7-6 extend. Return early with `nil` when `deps.ThemesDir == ""`.
```

In `theming-system-7-3`, **Acceptance Criteria**:

```markdown
- [ ] A `themesDirPath()` failure yields zero advisories and a diagnosis that still renders every check and its summary.
```

In `theming-system-7-3`, **Edge Cases**:

```markdown
- A `themesDirPath` resolution failure must degrade rather than abort the diagnosis, matching doctor's existing best-effort store construction where a nil store yields a not-evaluable check.
```

**Proposed**:

In `theming-system-7-3`, **Do** (second bullet) becomes:

```markdown
- Create `cmd/doctor_theme.go` with `func collectThemeAdvisories(deps *DoctorDeps) []advisory`, the single entry point tasks 7-4/7-5/7-6 extend. When `deps.ThemesDir == ""` (an unresolved path) **skip the directory scan only** — no `Enumerate` call, no directory line, no per-file line — and carry on with the rest of the assembly. Do **not** return early from `collectThemeAdvisories` itself: task 7-5's persisted-slug producer resolves built-ins from the embedded set with **no path at all** and yields `not found` for a drop-in slug, so an unresolved directory must never suppress the line for the failure a user is most likely to hit (§12.2). Task 7-6 assembles the three regions inside this same function, so the early return would silently remove two of them.
```

In `theming-system-7-3`, **Acceptance Criteria** becomes:

```markdown
- [ ] A `themesDirPath()` failure yields zero **theme-file** advisories — no directory line and no per-file line — and a diagnosis that still renders every check and its summary; the skip is scoped to the scan, so `collectThemeAdvisories` still returns and task 7-5's producer still runs from it.
```

In `theming-system-7-3`, **Edge Cases** becomes:

```markdown
- A `themesDirPath` resolution failure must degrade rather than abort the diagnosis, matching doctor's existing best-effort store construction where a nil store yields a not-evaluable check — and the degradation is **scoped to the directory scan**, never to `collectThemeAdvisories` itself, because the persisted-slug producer (task 7-5) resolves built-ins with no path at all and must still report an unresolvable persisted slug.
```

In `theming-system-7-5`, **Acceptance Criteria** gains one criterion, placed immediately after the `0000`-mode-directory criterion:

```markdown
- [ ] With `deps.ThemesDir` **empty** (an unresolved path), `{"theme":"nord-lee"}` still yields `⚠ theme nord-lee does not resolve: not found` — the unresolved directory suppresses the file scan (task 7-3) but never this producer, and no path is composed.
```

In `theming-system-7-5`, **Tests** gains one entry, placed immediately after the `"it distinguishes not found from unreadable"` entry:

```markdown
- `"it still reports a persisted slug with no themes directory"` — `TestPersistedThemeAdvisory_UnresolvedThemesDirStillReports`
```

**Resolution**: Fixed
**Notes**:

---

### 2. The panel's row delegate carries its own `Width`, and no task re-points it on a resize

**Severity**: Important
**Plan Reference**: Phase 8, task `theming-system-8-11` ("Panel geometry — degrade between preferred and minimum, refuse below the floor"); consumes tasks `theming-system-8-4`, `theming-system-8-6`, `theming-system-8-9`
**Category**: Task Self-Containment
**Change Type**: update-task

**Details**:

Task 8-4 declares the panel's delegate with an explicit width field — `themeRowDelegate{ Theme; Colourless; Width int }`, where `Width` is "the panel's inner content width" and is the budget the four-element composition priority and the three-character truncation floor are measured against. This is a deliberate departure from the existing `SessionDelegate`, which reads the width off the `list.Model` handed to `Render`.

Task 8-6 names the hazard that departure creates: the delegate must be built through exactly one helper, `m.themeRowDelegate()`, because "two construction sites can disagree about width or colourlessness, and that disagreement is invisible **until a resize during a live preview**".

But only one task re-invokes that helper: task 8-9 adds `p.list.SetDelegate(m.themeRowDelegate())` to `applyCanvasMode`, which runs on a **theme swap**. Task 8-11's resize path re-runs the width ladder and `SetSize`s the panel's list, and says nothing about the delegate. A `tea.WindowSizeMsg` does not call `ApplyTheme`, so after a resize the delegate keeps the pre-resize `Width` until the user happens to press an arrow.

The observable defect is exactly the one 8-6 predicted: narrow the terminal with the panel open and rows keep composing against the old, wider budget — labels that should truncate do not, the right-aligned badge and reason land past the inner edge, and the panel block either overflows its declared width or silently cuts the composed row. Task 8-11's existing criterion ("every row still equals the panel width") gestures at it, but the Do section never instructs the re-point, so the implementer is left to rediscover the hazard the plan already identified.

**Current**:

In `theming-system-8-11`, **Do**:

```markdown
- **Resize while open — degrade in place.** In the `tea.WindowSizeMsg` path, when the panel is open: re-run the ladder and the body-height arithmetic and `SetSize` the panel's list, exactly as the two page lists are re-sized. The main screen is deliberately **not** re-laid-out to the reduced width (§9.1) — the panel is composited over a page that laid out at full width, so a panel width change never reflows the surface being previewed.
```

In `theming-system-8-11`, **Acceptance Criteria**:

```markdown
- [ ] A resize that stays above the floor keeps the panel open and re-sizes its list; the panel's rendered height still equals the content height and every row still equals the panel width.
```

In `theming-system-8-11`, **Tests**:

```markdown
- `"it degrades in place on a resize"` — `TestPanelGeometry_ResizeDegradesInPlace`
```

**Proposed**:

In `theming-system-8-11`, **Do** becomes:

```markdown
- **Resize while open — degrade in place.** In the `tea.WindowSizeMsg` path, when the panel is open: re-run the ladder and the body-height arithmetic, `SetSize` the panel's list exactly as the two page lists are re-sized, **and re-point the panel's delegate through `m.themeRowDelegate()`** so task 8-4's composition budget follows the new inner width. The re-point is load-bearing rather than tidiness: task 8-4's delegate holds `Width` as a **field** (unlike `SessionDelegate`, which reads `m.Width()` off the list), and the only other caller of the helper is task 8-9's `applyCanvasMode`, which runs on a **theme swap** — a `tea.WindowSizeMsg` does not call `ApplyTheme`. Without it, a resize that is not followed by an arrow leaves every row composing against the pre-resize budget, which is precisely the disagreement task 8-6 introduced the single construction point to prevent and the failure mode it names, "invisible until a resize during a live preview". The main screen is deliberately **not** re-laid-out to the reduced width (§9.1) — the panel is composited over a page that laid out at full width, so a panel width change never reflows the surface being previewed.
```

In `theming-system-8-11`, **Acceptance Criteria** — the existing criterion stands and one is added immediately after it:

```markdown
- [ ] A resize that stays above the floor keeps the panel open and re-sizes its list; the panel's rendered height still equals the content height and every row still equals the panel width.
- [ ] A resize re-points the panel's delegate: narrowing from the preferred width into the degraded band **with no intervening arrow keypress** re-composes the rows against the new budget — a label that needed no truncation at the preferred width is truncated with `…`, the `●` badge stays inside the inner edge, and no row exceeds the panel's inner width.
```

In `theming-system-8-11`, **Tests** — the existing entry stands and one is added immediately after it:

```markdown
- `"it degrades in place on a resize"` — `TestPanelGeometry_ResizeDegradesInPlace`
- `"it re-points the delegate on a resize"` — `TestPanelGeometry_ResizeRepointsTheDelegate` (narrow with **no** intervening arrow; row composition follows the new width)
```

In `theming-system-8-11`, **Edge Cases** gains one entry, placed immediately after the "Resize while open degrades **in place**…" entry:

```markdown
- The resize path must **re-point the delegate**, not only `SetSize` the list: task 8-4's delegate holds `Width` as a field and task 8-9's restyle path re-invokes `m.themeRowDelegate()` only on a theme swap, so a resize with no following arrow would otherwise compose every row against the pre-resize budget — the exact disagreement task 8-6's single construction point exists to prevent.
```

**Resolution**: Fixed
**Notes**:

---

### 3. The panel fixtures' tapes are specified without the keypress that opens the panel

**Severity**: Minor
**Plan Reference**: Phase 8, task `theming-system-8-15` ("Panel fixture inputs and the two setting-state frames"); inherited by tasks `theming-system-8-16` and `theming-system-9-12`
**Category**: Task Self-Containment
**Change Type**: update-task

**Details**:

Task 4-2 introduced `captureKeys` as "the same sequence the fixture's tape types, declared once so the two cannot drift" — i.e. the **tape** types the keys for the live VHS capture, and the field is the offline mirror consumed by `ModelAt` (which the swap-and-diff guard drives). The shipped `testdata/vhs/projects.tape` confirms the idiom: it runs `capturetool --fixture projects`, then `Type "x"`, then `Screenshot`. Nothing in `cmd/capturetool` replays a fixture's keys.

Task 8-15 opens the panel "through the real path" by declaring `captureKeys` as a single `t` press, then specifies the tape as "the existing idiom (`go run ./cmd/capturetool --fixture <name> --theme <…>`, fixed font/size, `Sleep`, `Screenshot`)" — with no `Type` step. Tasks 8-16 and 9-12 repeat "the existing idiom" and frame their extra keypresses as `captureKeys` too (`theme-panel-dir-unreadable`'s page-2 `Ctrl+↓`, `theme-panel-projects`'s `x` then `t`).

Read literally, every panel tape screenshots a panel-less Sessions frame — seven captures whose entire subject is missing, on the surface §13.1 makes the *only* route to seeing a visual change before release. The alternative reading (make `capturetool` replay `captureKeys`) is a design decision the plan does not make, and taking it would double-apply the rewritten `projects` tape's `x` — which toggles Sessions⟷Projects in **both** directions, so the frame would land back on Sessions.

The gate would catch a blank capture, so this is polish rather than a blocker — but it is one sentence, and it removes a fork an implementer would otherwise have to resolve across three tasks.

**Current**:

In `theming-system-8-15`, **Do**:

```markdown
- **Write one `.tape` per fixture** in the existing idiom (`go run ./cmd/capturetool --fixture <name> --theme <…>`, fixed font/size, `Sleep`, `Screenshot`), and **verify a fresh write before trusting or reviewing each PNG** — confirm the file's hash changed and retry on failure. VHS reports no error when it fails to write, every capture here is a first-time write through a freshly-written tape, and a theme change is visible **only** in the image, so an unverified capture reads as either "the change didn't render" or a false pass.
```

**Proposed**:

In `theming-system-8-15`, **Do** becomes:

```markdown
- **Write one `.tape` per fixture** in the existing idiom (`go run ./cmd/capturetool --fixture <name> --theme <…>`, fixed font/size, `Sleep`, `Screenshot`) — and **the tape types the fixture's declared `captureKeys` sequence itself**, here a single `t`, before the `Screenshot`, exactly as `projects.tape` types its `x` today. `capturetool` runs the live program and replays **no** keys of its own: `captureKeys` is the offline mirror task 4-2 added for `ModelAt`, "declared once so the two cannot drift", and making `capturetool` replay it instead would double-apply the rewritten `projects` tape's `x`, which toggles Sessions⟷Projects in both directions. A panel tape that omits the keypress screenshots a panel-less Sessions frame — the whole subject of the capture missing. This is the tape idiom tasks 8-16 and 9-12 inherit, including `theme-panel-dir-unreadable`'s page-2 `Ctrl+↓` and `theme-panel-projects`'s `x` then `t`. Then **verify a fresh write before trusting or reviewing each PNG** — confirm the file's hash changed and retry on failure. VHS reports no error when it fails to write, every capture here is a first-time write through a freshly-written tape, and a theme change is visible **only** in the image, so an unverified capture reads as either "the change didn't render" or a false pass.
```

In `theming-system-8-15`, **Acceptance Criteria** — the existing tape criterion stands and one is added immediately after it:

```markdown
- [ ] A `.tape` exists per fixture and each captured PNG is verified as a **fresh** write (hash changed) before review.
- [ ] Each `.tape` types the same key sequence its fixture declares in `captureKeys` before its `Screenshot`, and `capturetool` replays no keys of its own — a capture whose frame shows no panel is a tape defect, not a fixture defect.
```

In `theming-system-8-15`, **Edge Cases** gains one entry, placed immediately after the "**VHS fails silently on write**…" entry:

```markdown
- The **tape** types the fixture's `captureKeys` sequence; `capturetool` does not replay it (that field is task 4-2's offline mirror for `ModelAt`), so a panel tape without its `t` screenshots a panel-less Sessions frame — and making `capturetool` replay it instead would double-apply the `projects` tape's `x`, which toggles pages in both directions.
```

**Resolution**: Fixed
**Notes**:

---
