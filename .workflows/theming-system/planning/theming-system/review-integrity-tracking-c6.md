# Review Tracking: Theming System - Integrity

## Findings

### 1. The faked `ThemeEnumerator`'s `Resolve` has no specified *palette*, so task 8-8's open-time `ApplyTheme` repaints every panel fixture away from `--theme`

**Severity**: Important
**Plan Reference**: Phase 8, task `theming-system-8-15` ("Panel fixture inputs and the two setting-state frames"); consumes task `theming-system-8-8`; inherited by tasks `theming-system-8-16` and `theming-system-9-12`
**Category**: Task Self-Containment (cross-task handover)
**Change Type**: update-task

**Details**:

Task 8-8 makes the panel's open sequence *apply* a theme: "**Apply it**: if the selected theme differs from `m.activeTheme`, call `Model.ApplyTheme` … never a rebuild, never a direct field assignment", where the selected member comes from the `Resolution` that the seam's `Resolve` returned. Task 8-15 then opens every panel fixture "through the real path" by declaring `captureKeys` as a single `t` press — so that apply runs on every fixture frame, before the `InitialThemeCursor` seed re-anchors the cursor.

Task 8-15 specifies exactly one thing about the fake's `Resolve` return: it "returns a `theme.Resolution` carrying the declared `themeSlots`" (the badge source, per cycle 3's correction). It says nothing about the **palette** that resolution reports. The task's own rule — "The frame's palette is the `--theme` nomination's" — is written against a different seam: it forbids the *cursor seed* from applying a row's `Theme`, and does not reach `Resolve`.

Three outcomes follow, and none of them is loud:

1. **Silently colourless.** A `Resolution` whose nomination is zero-valued resolves through `lipgloss.Color("")`'s no-colour sentinel — the exact hazard task 3-2 names ("silently colourless, with no compile error and no failing assertion") and the reason `New` keeps its dark seed. Declaring only `themeSlots` is the natural reading of the current bullet, and it produces precisely this.
2. **The wrong palette, on the frame that exists to demonstrate coherence.** `theme-panel-constant-previewing` declares raw keys `{Theme: "nord"}` with the cursor on `tokyo-night-day` and is captured with `--theme tokyo-night-day`. Its in-force resolved member is the **constant**, `nord` — so a fake that faithfully reports `nord`'s palette makes the open apply `nord` and paint the frame in it, while the fixture's doc comment, its `--theme` value and task 8-15's coherence rule all say `tokyo-night-day`. The frame that §9.14 makes "the only reference that exists" for the slot half would be self-contradicting.
3. **Silently zero coverage under §13.4's guard.** Task 4-3 drives every enumerated fixture through `ModelAt` with two synthetic themes; task 4-2's `ModelAt` replays `captureKeys`, so `t` opens the panel there too. An open-time apply of anything other than the injected synthetic theme overwrites theme A on the A-render and theme B on the B-render, so the panel fixtures contribute neither an A value nor a B value: assertion 1 (a negative) passes, assertion 2's union balances, and the panel's `bubbles/list` instance — the one §11.2 calls the worst case of the cached-style class, and the reason task 8-16 adds `theme-panel-paginated` at all — is covered by nothing while reading as covered.

The coherent resolution is the one the task already implies but never states: the fake reports **the palette the model was constructed with**, so the open-time apply is a no-op and `--theme` stays the sole palette authority. Task 8-15's own acceptance criterion ("the rendered palette is the `--theme` nomination's … asserted by rendering one fixture under two `--theme` values and diffing") would fail every other implementation, so this is a gap the implementer must close by guesswork rather than one the plan closes for them.

**Current**:

Task `theming-system-8-15`, **Do** — the second bullet:

```markdown
- **The seeded cursor is placement only — it applies no theme.** The frame's palette is the `--theme` nomination's, which is what keeps `capturetool --theme <slug|path>` meaningful on precisely the frames a drop-in author most wants to check; applying a faked row's `Theme` instead would make the flag inert exactly there. Say this in the seed's doc comment.
```

Task `theming-system-8-15`, **Acceptance Criteria** — the palette criterion:

```markdown
- [ ] The seeded cursor changes the highlighted row only — the rendered palette is the `--theme` nomination's in both fixtures (asserted by rendering one fixture under two `--theme` values and diffing).
```

Task `theming-system-8-15`, **Tests** — the palette entry:

```markdown
- `"it takes its palette from the theme flag"` — `TestPanelFixture_PaletteFollowsTheThemeFlag`
```

Task `theming-system-8-15`, **Edge Cases** — the slot-record entry:

```markdown
- The declared slot record reaches the panel through the **faked seam's `Resolve` return**, not a `Deps` field: task 8-8 retired the injected record with its last consumer, so a fixture wired to the old slot would render no badge on any row — and the badges are the entire subject of both setting-state frames, which §9.14 makes the only reference that exists for the slot half.
```

**Proposed**:

Task `theming-system-8-15`, **Do** — the second bullet becomes:

```markdown
- **The palette is the `--theme` nomination's, and the fake must report it.** Two seams can repaint the frame and both must be closed. The **seeded cursor is placement only** — it applies no theme — because applying a faked row's `Theme` would make `capturetool --theme <slug|path>` inert on precisely the frames a drop-in author most wants to check. And task 8-8's open **applies the resolved theme through `ApplyTheme`** before the cursor seed runs, reading it off this fake's `Resolve` return — so the fake is constructed with **the same `theme.Theme` the model's nomination carries** and returns it as the resolution's nomination *and* as every `SlotResolution.Theme`, which makes that apply a no-op. Thread the palette in at the one site where the constant nomination is already built — `cmd/capturetool`'s resolved `--theme` value, and task 4-2's `ModelAt(th, …)` — rather than hard-coding a built-in inside the fixture; the fake must reach no loader and stay I/O-free. Say all of it in the fake's doc comment, because three distinct failures follow from leaving it unstated and none of them is loud: a **zero-valued** resolution renders the whole panel through `lipgloss.Color("")`'s no-colour sentinel — silently colourless, no compile error, no failing assertion, the hazard task 3-2 keeps `New`'s seed to close; a **hard-coded built-in** repaints `theme-panel-constant-previewing` in the persisted constant `nord` while its doc comment and its `--theme` both say `tokyo-night-day`, so the one frame §9.14 calls the only reference that exists for the slot half contradicts the coherence rule this very task states; and inside §13.4's swap-and-diff guard the same apply **overwrites the synthetic theme `ModelAt` was handed**, so every panel fixture contributes neither an A value nor a B value — assertion 1 passes as a vacuous negative, assertion 2's union balances, and the panel's `bubbles/list` instance is covered by nothing while reading as covered, which is the "absence reads as coverage" blind spot task 8-16 exists to prevent.
```

Task `theming-system-8-15`, **Acceptance Criteria** — the existing palette criterion stands and one is added immediately after it:

```markdown
- [ ] The seeded cursor changes the highlighted row only — the rendered palette is the `--theme` nomination's in both fixtures (asserted by rendering one fixture under two `--theme` values and diffing).
- [ ] The fake's `Resolve` reports the **injected** palette, so task 8-8's open-time `ApplyTheme` is a no-op: a panel fixture rendered under a synthetic theme carries that theme's values across the panel's own chrome, no built-in's hex appears anywhere in the frame, and a fixture whose fake reports a zero `Resolution` renders colourless and fails.
```

Task `theming-system-8-15`, **Tests** — the existing palette entry stands and one is added immediately after it:

```markdown
- `"it takes its palette from the theme flag"` — `TestPanelFixture_PaletteFollowsTheThemeFlag`
- `"it reports the injected palette as resolved"` — `TestFakeThemeEnumerator_ResolveReportsTheInjectedPalette` (open-time `ApplyTheme` is a no-op; the panel chrome carries the injected palette; a zero-valued resolution renders colourless)
```

Task `theming-system-8-15`, **Edge Cases** — the existing slot-record entry stands and one is added immediately after it:

```markdown
- The declared slot record reaches the panel through the **faked seam's `Resolve` return**, not a `Deps` field: task 8-8 retired the injected record with its last consumer, so a fixture wired to the old slot would render no badge on any row — and the badges are the entire subject of both setting-state frames, which §9.14 makes the only reference that exists for the slot half.
- The fake's `Resolve` must report the **palette** as well as the slots, because task 8-8's open applies the resolved theme through `ApplyTheme` before the cursor seed runs: a zero-valued resolution paints the panel silently colourless, a hard-coded built-in repaints the constant-previewing frame away from its own `--theme`, and under §13.4's guard either one overwrites the synthetic theme so every panel fixture covers nothing while passing every assertion.
```

**Resolution**: Fixed
**Notes**:

---

### 2. Nothing pins that the swap-and-diff guard's render size clears the panel's entry floor, so its panel fixtures can render panel-less and still pass

**Severity**: Minor
**Plan Reference**: Phase 8, task `theming-system-8-15` ("Panel fixture inputs and the two setting-state frames"); consumes tasks `theming-system-4-2`, `theming-system-4-3`, `theming-system-8-11`, `theming-system-8-13`; inherited by tasks `theming-system-8-16` and `theming-system-9-12`
**Category**: Acceptance Criteria Quality (unasserted cross-phase precondition)
**Change Type**: add-to-task

**Details**:

Task 4-3 renders every enumerated fixture at **one** pinned size — "**Pin the render size** as a named constant wide and tall enough that fixtures render their full chrome rather than a degraded ladder step" — and drives them through task 4-2's `ModelAt`, which replays the fixture's `captureKeys` as its last step. From task 8-15 onward a panel fixture's `captureKeys` is a `t` press, so inside the guard that `t` runs through task 8-13's entry gate exactly as it does under a tape.

That gate refuses below task 8-11's floor and raises a flash instead of opening. If the pinned constant does not clear the floor — width below `themePanelMinWidth`, or height below header(2) + the measured footer + one list row + one message row, plus the extra row `theme-panel-dir-unreadable` forces via `DirUnusable` — the guard renders a **panel-less Sessions frame** with a blocked-`t` flash. Every assertion still passes: assertion 1 is a negative over a frame with no panel in it, assertion 2's union is computed from whatever was rendered, and assertion 3 was already satisfied at Phase 4 without the panel. The panel's third `bubbles/list` instance and its pagination dots go uncovered, which is the exact blind spot §13.3 names ("a missing fixture is a blind spot the guard structurally cannot report … absence reads as coverage") and the reason task 8-16 adds `theme-panel-paginated` at all.

The precondition is almost certainly satisfied in practice — a size wide and tall enough for Portal's full chrome clears a 24-column, ~9-row floor comfortably. It is raised because it is *unasserted*: task 8-16's criterion "§13.4's guard exercises those dots" states the outcome without naming what makes it true, and the failure is silent in exactly the way this plan otherwise names and closes everywhere. One Do line and one criterion turn an assumption into a proof.

**Current**:

Task `theming-system-8-15`, **Do** — the tape bullet (final sentence quoted for anchoring):

```markdown
- **Write one `.tape` per fixture** in the existing idiom (`go run ./cmd/capturetool --fixture <name> --theme <…>`, fixed font/size, `Sleep`, `Screenshot`) — and **the tape types the fixture's declared `captureKeys` sequence itself**, here a single `t`, before the `Screenshot`, exactly as `projects.tape` types its `x` today. […] Then **verify a fresh write before trusting or reviewing each PNG** — confirm the file's hash changed and retry on failure. VHS reports no error when it fails to write, every capture here is a first-time write through a freshly-written tape, and a theme change is visible **only** in the image, so an unverified capture reads as either "the change didn't render" or a false pass.
```

Task `theming-system-8-15`, **Acceptance Criteria** — the guard criterion:

```markdown
- [ ] Both fixtures are picked up by §13.4's swap-and-diff guard with no test edit (the guard enumerates).
```

Task `theming-system-8-15`, **Tests** — the guard entry:

```markdown
- `"it is enumerated by the swap-and-diff guard"` — `TestPanelFixture_UnderTheGuard`
```

Task `theming-system-8-15`, **Edge Cases** — the inheritance entry:

```markdown
- The new fixtures inherit §13.4's three assertions automatically, which is the point of enumerating rather than naming, and §9.1's token table records that the panel fixtures are what cover `accent.mode` and `accent.attention` outside their transient main-screen states.
```

**Proposed**:

Task `theming-system-8-15`, **Do** — the tape bullet stands unchanged and one bullet is added immediately after it:

```markdown
- **Prove the guard's render size clears the entry gate.** §13.4's swap-and-diff guard renders every enumerated fixture at task 4-3's **single pinned size constant** and drives it through task 4-2's `ModelAt`, which replays `captureKeys` — so a panel fixture's `t` passes through task 8-13's entry gate there exactly as it does under a tape. If that constant fails task 8-11's floor (width below `themePanelMinWidth`, or height below header(2) + the measured footer + one list row + one message row, plus the extra row `DirUnusable` forces on `theme-panel-dir-unreadable`), the guard renders a **panel-less** frame carrying a blocked-`t` flash and every assertion still passes: assertion 1 is a negative over a frame with no panel in it, assertion 2's union balances, and assertion 3 was already satisfied without the panel. Assert it rather than assuming it — a panel fixture's A-frame under the guard must contain the panel's left border and its `Themes` header — and raise task 4-3's constant if it does not clear the floor, recording the panel floor as one of the reasons the value is what it is (task 4-3 already requires that comment). This is the blind spot §13.3 names, where absence reads as coverage, at exactly the `bubbles/list` instance task 8-16's paginating frame exists to cover.
```

Task `theming-system-8-15`, **Acceptance Criteria** — the existing guard criterion stands and one is added immediately after it:

```markdown
- [ ] Both fixtures are picked up by §13.4's swap-and-diff guard with no test edit (the guard enumerates).
- [ ] Inside that guard the panel actually opens: a panel fixture's A-frame contains the panel's left border and its `Themes` header, so task 4-3's pinned render size is **proven** to clear task 8-11's entry floor rather than assumed to.
```

Task `theming-system-8-15`, **Tests** — the existing guard entry stands and one is added immediately after it:

```markdown
- `"it is enumerated by the swap-and-diff guard"` — `TestPanelFixture_UnderTheGuard`
- `"it composites the panel under the guard's pinned size"` — `TestPanelFixture_PanelIsCompositedUnderTheGuard` (left border and `Themes` header present in the A-frame)
```

Task `theming-system-8-15`, **Edge Cases** — the existing inheritance entry stands and one is added immediately after it:

```markdown
- The new fixtures inherit §13.4's three assertions automatically, which is the point of enumerating rather than naming, and §9.1's token table records that the panel fixtures are what cover `accent.mode` and `accent.attention` outside their transient main-screen states.
- The guard renders every fixture at task 4-3's **single pinned size** and replays `captureKeys` through `ModelAt`, so a panel fixture's `t` passes task 8-13's entry gate there too — a pinned size below task 8-11's floor makes the guard render a panel-less frame while every assertion still passes, which is the blind spot §13.4 structurally cannot report.
```

**Resolution**: Fixed
**Notes**:
