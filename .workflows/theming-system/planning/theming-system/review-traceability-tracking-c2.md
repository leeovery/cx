# Review Tracking: Theming System - Traceability

Cycle 2. Full fresh pass over the whole plan (10 phases, 97 tasks) against
`.workflows/theming-system/specification/theming-system/specification.md`, in both
directions.

**Direction 2 (plan → specification) is clean.** Every task's Problem, Solution, Do,
Acceptance Criteria, Tests and Edge Cases traces to a named spec section, and every task
carries a `Spec Reference` plus quoted `Context`. The nine places the plan resolves a
genuine spec gap are each explicitly labelled **Ambiguity flagged** or **Decision
recorded** in-task (1-8's `token` attr cardinality, 2-5's Oklab metric implementation,
3-1's `testdata/vhs/reference/` scoping, 4-2's `captureKeys` fixture script, 5-2's
control-only persisted value, 6-4's absent-file translation no-op, 7-1's summary
indentation, 7-2's glyph-in-copy ownership, 7-6's advisory-region order, 8-7's
`Deps.ThemeSlots` injection, 8-10's `theme: loaded` panel-path cadence, 8-11's floor
without a keep-N-columns rule, 9-8's filter-line precedence implementation) — each names
the gap, resolves it conservatively within the spec's own reasoning, and records the
choice for later revisiting. No hallucinated requirement, behaviour, edge case or
acceptance criterion was found.

**Direction 1 (specification → plan)** found two elements with no plan home. Both are
implementer-facing reference material the spec states deliberately, and both land on
tasks that already exist — so each is an `add-to-task`, not new scope.

## Findings

### 1. §3.1's Lipgloss v2 standing fact has no plan home

**Type**: Missing from plan
**Spec Reference**: §3.1 (final two paragraphs — "Lipgloss v2's direction is neutral on this decision, not supporting evidence")
**Plan Reference**: Phase 3, task `theming-system-3-2` (`tui.Build` takes the loaded nomination and the gate selects its member)
**Change Type**: add-to-task

**Details**:

§3.1 closes with a paragraph the spec explicitly frames as **implementer-facing**, not as
design rationale:

> This is a standing fact about a live dependency rather than a discarded option — both
> APIs are in the tree Portal builds against, and an implementer working through §3.2's
> collapse of `Token` or §8.8's surviving detect-or-timeout gate will meet them and
> reasonably ask why Portal hand-rolls a light/dark decision the library has an API for.
> The answer is that Portal's gate selects between two *named themes*, which `LightDark`
> does not model.

The two sites the spec names are exactly tasks 3-1 (the `Token` collapse) and 3-2 (the
surviving gate). Neither carries it: `grep -F "LightDark"` and `grep -F "AdaptiveColor"`
across `planning.md` and all ten `phase-*-tasks.md` return nothing.

The consequence is concrete rather than cosmetic. Task 3-2 is where an implementer builds
a light/dark decision from scratch — `newAppearanceGate` replaced by a nomination-shaped
gate with a ~50ms race and a dark no-answer fallback — while `lipgloss.LightDark(hasDarkBG)`
sits in the dependency tree offering precisely that shape, and `compat.AdaptiveColor`
offers the paired-value `Token` §3.2 is collapsing. Reaching for either would silently
reintroduce pairing, which is the decision §3.1 exists to settle. The spec anticipated the
question and pre-answered it; the plan drops the answer at the one task where the question
arises.

Task 3-2 is the single correct home: it owns the gate, it lands after 3-1's collapse, and
the proposed wording covers both sites the spec names.

**Current** (task `theming-system-3-2`, closing bullets of **Edge Cases**):

```markdown
- `Deps.Appearance` and `WithAppearance` are **removed rather than left alongside** — a dead option is a second injection path the harness and production could diverge on.
- The zero value of `Nomination` is neither state. `Build` always injects one; give the type a constructor-only contract (unexported fields) so a zero value cannot be constructed accidentally, and make `Select`/`Constant` on a zero value return a zero `Theme` rather than panic.
```

**Proposed** (task `theming-system-3-2`, closing bullets of **Edge Cases**):

```markdown
- `Deps.Appearance` and `WithAppearance` are **removed rather than left alongside** — a dead option is a second injection path the harness and production could diverge on.
- The zero value of `Nomination` is neither state. `Build` always injects one; give the type a constructor-only contract (unexported fields) so a zero value cannot be constructed accidentally, and make `Select`/`Constant` on a zero value return a zero `Theme` rather than panic.
- **Do not reach for `lipgloss.LightDark` or `compat.AdaptiveColor`** — both are in the tree Portal builds against, and either the `Token` collapse (task 3-1) or this gate is where an implementer meets them and reasonably asks why Portal hand-rolls a light/dark decision the library has an API for. Lipgloss v2 moving `AdaptiveColor` into `compat` reads at a glance as Charm deprecating paired colours, but the recommended replacement `lipgloss.LightDark(hasDarkBG)` **keeps paired values** and merely makes the detection explicit: what Charm de-recommended is *implicit detection*, not pairing. So the library's direction is **neutral on split, not supporting evidence for it**, and neither API can serve this gate — Portal selects between two *named themes*, which `LightDark` does not model. Hand-rolling the gate is the decision, not an oversight.
```

**Current** (task `theming-system-3-2`, closing lines of **Context**):

```markdown
> §12.3: emission is controlled by an injected logger — `cmd` passes a **real** component logger on the paths where a theme is used (TUI construction) and `log.Discard` on doctor, export and `capturetool`.
> Phase boundary: Phase 5 replaces this mapping with the persisted `theme` / `theme_light` / `theme_dark` keys, adds §8.5's fallback and §7.6's fatal message; Phase 6 adds the one-shot `appearance` translation and deletes `prefs.Appearance`.
```

**Proposed** (task `theming-system-3-2`, closing lines of **Context**):

```markdown
> §12.3: emission is controlled by an injected logger — `cmd` passes a **real** component logger on the paths where a theme is used (TUI construction) and `log.Discard` on doctor, export and `capturetool`.
> §3.1: "**Lipgloss v2's direction is neutral on this decision, not supporting evidence.** Lipgloss v2 moved `AdaptiveColor` into `compat`, which reads at a glance as Charm deprecating paired colours — i.e. as independent support for split. It is not: the recommended replacement, `lipgloss.LightDark(hasDarkBG)`, **keeps paired values** and merely makes the detection explicit. What Charm de-recommended is *implicit detection*, not pairing, so **its direction is neutral on this decision.** This is a standing fact about a live dependency rather than a discarded option — both APIs are in the tree Portal builds against, and an implementer working through §3.2's collapse of `Token` or §8.8's surviving detect-or-timeout gate will meet them and reasonably ask why Portal hand-rolls a light/dark decision the library has an API for. The answer is that Portal's gate selects between two *named themes*, which `LightDark` does not model."
> Phase boundary: Phase 5 replaces this mapping with the persisted `theme` / `theme_light` / `theme_dark` keys, adds §8.5's fallback and §7.6's fatal message; Phase 6 adds the one-shot `appearance` translation and deletes `prefs.Appearance`.
```

**Resolution**: Fixed
**Notes**: `planning.md`'s Phase 3 task table carries a condensed Edge Cases cell for
`theming-system-3-2`; if the fix is approved it should gain the matching condensed clause
so the table and the detail file do not drift — suggested text: `neither`
`lipgloss.LightDark` `nor` `compat.AdaptiveColor` `serves this gate — v2's move of`
`AdaptiveColor` `into` `compat` `de-recommends implicit detection, not pairing, and`
`LightDark` `keeps paired values and cannot model a selection between two named themes,`
`so the hand-rolled gate is the decision rather than an oversight`.

---

### 2. Two of §15.4's forward-looking Nord Paper frames are named by no task

**Type**: Incomplete coverage
**Spec Reference**: §15.4 (the six new forward-looking frames), §7.4 (the two corrections and the `bg.attention` invention, both settled visually), §9.14 (frames are reference, never truth)
**Plan Reference**: Phase 2, task `theming-system-2-7` (Port Nord — the first genuinely external palette)
**Change Type**: add-to-task

**Details**:

§15.4 names six Paper frames as **forward-looking reference material** on the explicit
ground that they "describe surfaces that do not exist yet". The plan surfaces four of them:

- `Theme slide-over — A (inline slot badges)` → task 8-15 ✓
- `Theme slide-over — A (constant set, previewing another)` → task 8-15 ✓
- `Theme slide-over — B (assignment header)` → the rejected treatment, retained by §9.14 as
  the record of what was weighed; no implementation consumer, correctly absent
- `Sessions — Nord (port)` → task 3-5 ✓
- `Kill Modal — Nord (state.destructive #DD8188)` → **named by no task**
- `Sessions — Nord inline flash (bg.attention #3D4046)` → **named by no task**

Confirmed by `grep -F "Kill Modal"` and `grep -F "Nord inline flash"` across `planning.md`
and all ten `phase-*-tasks.md`: zero hits.

The two missing frames are not incidental — they depict exactly the two values task 2-7's
human visual gate must judge. §7.4 records that the red correction was "judged **visually**
(both reds mocked side by side in a Nord kill modal)" and that `bg.attention`'s first
arithmetic answer "was rejected at a visual gate as far too heavy and pushed into a warm
grey outside Nord's cool family". Task 2-7's gate step points only at the standalone swatch
(`capturetool --fixture contrast-validation --theme nord`), which shows the tints as labelled
bands but not the shipped surfaces the spec's own frames render — so the gate is taken
without the reference the spec provides for it, and the reviewer has nothing to check the
shipped values against beyond a swatch.

§9.14's caution travels with them and must be stated wherever a frame is named: the mocks
use per-frame literal hexes, so they are reference, never truth.

**Current** (task `theming-system-2-7`, the **Do** section's visual-gate bullet):

```markdown
- **Human visual gate** on the corrections and inventions: `go run ./cmd/capturetool --fixture contrast-validation --theme nord` (task 2-4's surface), judging the two corrected values and the three invented ones — particularly `bg.attention`, whose first arithmetic answer was rejected at a gate as far too heavy and warm for Nord's cool family.
```

**Proposed** (task `theming-system-2-7`, the **Do** section's visual-gate bullet):

```markdown
- **Human visual gate** on the corrections and inventions: `go run ./cmd/capturetool --fixture contrast-validation --theme nord` (task 2-4's surface), judging the two corrected values and the three invented ones — particularly `bg.attention`, whose first arithmetic answer was rejected at a gate as far too heavy and warm for Nord's cool family. **Present §15.4's two Nord Paper frames alongside the swatch as the gate's reference** — `Kill Modal — Nord (state.destructive #DD8188)` for the corrected red and `Sessions — Nord inline flash (bg.attention #3D4046)` for the invented band. They are the only rendering of those two values on the surfaces they actually paint, and both judgements were originally settled against them (§7.4: the two reds "mocked side by side in a Nord kill modal"; the first `bg.attention` answer "rejected at a visual gate"). Read them **as reference, never truth** — §9.14 records that the mocks use per-frame literal hexes, so a frame's hex may differ from the shipped token's and the `.theme` file is the authority.
```

**Current** (task `theming-system-2-7`, closing lines of **Context**):

```markdown
> §7.4: "**Fidelity versus floors — resolved.** The floors win, and the corrected values ship under the palette's own name. No application maps a 16-slot ANSI palette 1:1 onto its own semantic roles; every Nord port in the wild adapts."
> Phase boundary: Nord's outstanding `text.subtle` gate lands in **Phase 3** on a grouped capture; `docs/theming.md`'s attribution and correction record lands in **Phase 10**.
```

**Proposed** (task `theming-system-2-7`, closing lines of **Context**):

```markdown
> §7.4: "**Fidelity versus floors — resolved.** The floors win, and the corrected values ship under the palette's own name. No application maps a 16-slot ANSI palette 1:1 onto its own semantic roles; every Nord port in the wild adapts."
> §15.4 lists the forward-looking reference frames for surfaces that do not exist yet; three are Nord's — `Sessions — Nord (port)` (task 3-5's gate), `Kill Modal — Nord (state.destructive #DD8188)` and `Sessions — Nord inline flash (bg.attention #3D4046)` (this task's gate). "**And even those are reference, never truth:** the Paper mocks use per-frame literal hexes, so the same token can carry different values across frames. That is exactly the drift the token layer prevents in code."
> Phase boundary: Nord's outstanding `text.subtle` gate lands in **Phase 3** on a grouped capture; `docs/theming.md`'s attribution and correction record lands in **Phase 10**.
```

**Resolution**: Fixed
**Notes**: `planning.md`'s Phase 2 task table carries a condensed Edge Cases cell for
`theming-system-2-7`; if the fix is approved it should gain the matching condensed clause —
suggested text: `§15.4's` `Kill Modal — Nord (state.destructive #DD8188)` `and`
`Sessions — Nord inline flash (bg.attention #3D4046)` `frames are the gate's reference for`
`the corrected red and the invented band, read as reference and never truth because the`
`mocks use per-frame literal hexes`. No task file other than `phase-2-tasks.md` changes.
