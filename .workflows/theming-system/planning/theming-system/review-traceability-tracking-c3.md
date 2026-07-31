# Review Tracking: Theming System - Traceability

Cycle 3. Full fresh pass over the whole plan (10 phases, 97 tasks) against
`.workflows/theming-system/specification/theming-system/specification.md`, in both
directions — not a narrowed check of cycles 1–2's fixes.

**Direction 2 (plan → specification) is clean.** Every task's Problem, Solution, Do,
Acceptance Criteria, Tests and Edge Cases traces to a named spec section, and every task
carries a `Spec Reference` plus quoted `Context`. The places the plan resolves a genuine
spec gap are each labelled **Ambiguity flagged** / **Decision recorded** in-task (1-8's
`token` attr cardinality, 2-5's Oklab metric, 3-1's `testdata/vhs/reference/` scoping,
4-2's `captureKeys`, 5-2's control-only persisted value, 6-1's `Field != ""` discriminator,
6-4's absent-file translation no-op, 6-6's migration attrs, 7-1's summary indentation,
7-2's glyph-in-copy ownership, 7-6's advisory-region order, 8-7's `Deps.ThemeSlots`,
8-10's `theme: loaded` panel-path cadence, 8-11's floor without a keep-N-columns rule,
8-13's two-evaluation floor predicate, 9-8's filter-line precedence implementation) — each
naming the gap, resolving it conservatively inside the spec's own reasoning, and recording
the choice. No hallucinated requirement, behaviour, edge case or acceptance criterion was
found. The one class checked hardest, structural guard tests, is clean after cycle 1's fix:
the surviving `TestNoPackageLevelThemeVar` (task 3-1) traces to §3.4's explicit rejection of
package-level mutable theme state, not to §13.4's deliberately-behavioural cached-style
guard.

**Direction 1 (specification → plan)** found two elements with no plan home. Both are
implementer-facing material the spec states deliberately and for a named reason, and both
land on tasks that already exist — so each is an `add-to-task`, not new scope.

Sections verified as fully covered but worth recording because they are thin on explicit
citations: §3.3 (all four consequences land through their forward references — §8's setting
shape, §9.2's mixed-mode flash, §13.5's test-side light/dark table), §4.4/§4.5 (task 1-3 +
task 10-2), §5.3 (task 1-4 + task 10-2), §6.5 (a no-action decision — Portal declines to
police terminal colour capability, so there is nothing to implement and nothing in the plan
invites a capability gate).

## Findings

### 1. §8.7's decline of DEC mode 2031 has no plan home, at the one task that builds the detection gate

**Type**: Missing from plan
**Spec Reference**: §8.7 ("DEC mode 2031 (the OS colour scheme) is deliberately **not** adopted — on semantics, not availability… It is declined because it answers a different question, not because it is out of reach"), §8.8, §9.3
**Plan Reference**: Phase 3, task `theming-system-3-2` (`tui.Build` takes the loaded nomination and the gate selects its member)
**Change Type**: add-to-task

**Details**:

§8.7 decides the **signal** detection uses, and it decides it against a live, fully-plumbed
alternative that the spec goes out of its way to describe in implementation terms:

> DEC mode 2031 (the OS colour scheme) is deliberately **not** adopted — on semantics, not
> availability. It is fully plumbed end-to-end in Portal's stack (`x/ansi` mode constants
> and report parsers, `ultraviolet` decoding DSR `997;1`/`997;2` into typed events, Bubble
> Tea v2 passing them through to `Update`, a one-line `tea.Raw(ansi.SetModeLightDark)`
> opt-in) and tmux 3.6+ supports it. It is declined because it answers a different
> question, not because it is out of reach.

`grep -F "2031"`, `grep -F "ModeLightDark"` and `grep -F "OS colour scheme"` across
`planning.md` and all ten `phase-*-tasks.md` return **nothing at the implementation tasks**.
The only hits are documentation-side: task 10-3 tells `docs/theming.md` to say detection
follows the terminal background "not the OS colour scheme", and task 10-4 rewrites the
README the same way. So the plan tells a *user* which signal is used and never tells the
*implementer* why the other one is refused.

This is the same class as cycle 2's finding 1 (§3.1's Lipgloss standing fact), which was
accepted and applied to this same task — and the temptation here is sharper, because §8.7
names an **accepted cost** an implementer would reasonably try to remove:

> **Accepted cost: OSC 11 is query-only; 2031 pushes on change.** Portal gets
> *correct-at-startup*, not *live-following* — a terminal that flips mid-session is not
> noticed until the next launch.

Three tasks put an implementer directly in front of that cost with the fix one line away.
Task 3-2 rebuilds the gate (`newAppearanceGate` → a nomination-shaped gate with the ~50ms
race and the dark no-answer fallback). Task 5-7 re-points that gate at the persisted
setting. Task 9-6 hand-classifies the OSC 11 background retained at launch, precisely
because no signal pushed. At any of the three, adding `tea.Raw(ansi.SetModeLightDark)` and
consuming the typed event reads as a strict improvement — and it silently inverts the
decision: Portal would follow the *OS scheme* rather than the *terminal background*, which
breaks §8.7's entire payoff ("Detection's entire payoff is therefore **aesthetic blending
with the surrounding terminal**"), its transition-dominance argument, and its
forward-compatibility-with-transparency argument (a transparent theme *must* follow the
terminal background). Nothing in the plan or in a test would catch it: both signals resolve
light/dark, so every gate assertion still passes.

Task 3-2 is the single correct home — it owns the gate's construction, it lands before both
later consumers, and it already carries the sibling standing fact in exactly this shape.

**Current** (task `theming-system-3-2`, closing bullets of **Edge Cases**):

```markdown
- `New`'s dark-built-in seed of `activeTheme` **survives** this task — only `Build`'s transitional pair holder is dropped. Without the seed, a model constructed without a nomination renders through `lipgloss.Color("")`'s no-colour sentinel: silently colourless, with no compile error and no failing assertion, which is precisely why task 3-1 added it.
- **Do not reach for `lipgloss.LightDark` or `compat.AdaptiveColor`** — both are in the tree Portal builds against, and either the `Token` collapse (task 3-1) or this gate is where an implementer meets them and reasonably asks why Portal hand-rolls a light/dark decision the library has an API for. Lipgloss v2 moving `AdaptiveColor` into `compat` reads at a glance as Charm deprecating paired colours, but the recommended replacement `lipgloss.LightDark(hasDarkBG)` **keeps paired values** and merely makes the detection explicit: what Charm de-recommended is *implicit detection*, not pairing. So the library's direction is **neutral on split, not supporting evidence for it**, and neither API can serve this gate — Portal selects between two *named themes*, which `LightDark` does not model. Hand-rolling the gate is the decision, not an oversight.
```

**Proposed** (task `theming-system-3-2`, closing bullets of **Edge Cases**):

```markdown
- `New`'s dark-built-in seed of `activeTheme` **survives** this task — only `Build`'s transitional pair holder is dropped. Without the seed, a model constructed without a nomination renders through `lipgloss.Color("")`'s no-colour sentinel: silently colourless, with no compile error and no failing assertion, which is precisely why task 3-1 added it.
- **Do not reach for `lipgloss.LightDark` or `compat.AdaptiveColor`** — both are in the tree Portal builds against, and either the `Token` collapse (task 3-1) or this gate is where an implementer meets them and reasonably asks why Portal hand-rolls a light/dark decision the library has an API for. Lipgloss v2 moving `AdaptiveColor` into `compat` reads at a glance as Charm deprecating paired colours, but the recommended replacement `lipgloss.LightDark(hasDarkBG)` **keeps paired values** and merely makes the detection explicit: what Charm de-recommended is *implicit detection*, not pairing. So the library's direction is **neutral on split, not supporting evidence for it**, and neither API can serve this gate — Portal selects between two *named themes*, which `LightDark` does not model. Hand-rolling the gate is the decision, not an oversight.
- **Do not adopt DEC mode 2031 (`ModeLightDark`) here, in task 5-7, or in task 9-6's conversion.** It is the other live temptation in this tree and it is one line away — the mode is fully plumbed end-to-end (`x/ansi` constants and report parsers, `ultraviolet` decoding DSR `997;1`/`997;2` into typed events, Bubble Tea v2 passing them to `Update`, a `tea.Raw(ansi.SetModeLightDark)` opt-in) and tmux 3.6+ supports it. It is declined on **semantics, not availability**: `ModeLightDark` reports *the operating system's* colour-scheme preference while OSC 11 reports *what colour the terminal's background is*, and the two routinely disagree (a terminal pinned dark on a light OS is the canonical case; on terminals without 2031 tmux synthesises the answer from the background anyway). Portal's detection exists for **aesthetic blending with the surrounding terminal**, so the terminal's background is the question it must ask — and following the terminal is also what keeps the deferred transparent-theme route purely additive.
- **The named accepted cost is not a defect to fix.** OSC 11 is query-only where 2031 pushes on change, so Portal is **correct-at-startup, not live-following**: a terminal that flips its background mid-session is not noticed until the next launch. That was judged thin and accepted. Adding a pushing signal to "improve" it silently swaps the question detection answers, and no gate assertion would catch it — both signals resolve to light/dark, so every test in this task still passes while the product now follows the OS scheme.
```

**Current** (task `theming-system-3-2`, closing lines of **Context**):

```markdown
> §3.1: "**Lipgloss v2's direction is neutral on this decision, not supporting evidence.** Lipgloss v2 moved `AdaptiveColor` into `compat`, which reads at a glance as Charm deprecating paired colours — i.e. as independent support for split. It is not: the recommended replacement, `lipgloss.LightDark(hasDarkBG)`, **keeps paired values** and merely makes the detection explicit. What Charm de-recommended is *implicit detection*, not pairing, so **its direction is neutral on this decision.** This is a standing fact about a live dependency rather than a discarded option — both APIs are in the tree Portal builds against, and an implementer working through §3.2's collapse of `Token` or §8.8's surviving detect-or-timeout gate will meet them and reasonably ask why Portal hand-rolls a light/dark decision the library has an API for. The answer is that Portal's gate selects between two *named themes*, which `LightDark` does not model."
> Phase boundary: Phase 5 replaces this mapping with the persisted `theme` / `theme_light` / `theme_dark` keys, adds §8.5's fallback and §7.6's fatal message; Phase 6 adds the one-shot `appearance` translation and deletes `prefs.Appearance`.
```

**Proposed** (task `theming-system-3-2`, closing lines of **Context**):

```markdown
> §3.1: "**Lipgloss v2's direction is neutral on this decision, not supporting evidence.** Lipgloss v2 moved `AdaptiveColor` into `compat`, which reads at a glance as Charm deprecating paired colours — i.e. as independent support for split. It is not: the recommended replacement, `lipgloss.LightDark(hasDarkBG)`, **keeps paired values** and merely makes the detection explicit. What Charm de-recommended is *implicit detection*, not pairing, so **its direction is neutral on this decision.** This is a standing fact about a live dependency rather than a discarded option — both APIs are in the tree Portal builds against, and an implementer working through §3.2's collapse of `Token` or §8.8's surviving detect-or-timeout gate will meet them and reasonably ask why Portal hand-rolls a light/dark decision the library has an API for. The answer is that Portal's gate selects between two *named themes*, which `LightDark` does not model."
> §8.7: "**Detection ships. The signal is the terminal background via OSC 11.** DEC mode 2031 (the OS colour scheme) is deliberately **not** adopted — on semantics, not availability. It is fully plumbed end-to-end in Portal's stack (`x/ansi` mode constants and report parsers, `ultraviolet` decoding DSR `997;1`/`997;2` into typed events, Bubble Tea v2 passing them through to `Update`, a one-line `tea.Raw(ansi.SetModeLightDark)` opt-in) and tmux 3.6+ supports it. It is declined because it answers a different question, not because it is out of reach. The two answer different questions: `ModeLightDark` reports *the operating system's* colour-scheme preference; OSC 11 reports *what colour the terminal's background is*. They routinely disagree — a terminal pinned dark on a light OS is the canonical case. On terminals that don't support 2031, tmux *synthesises* the answer by guessing from the background colour anyway. **What detection is for discriminates the signals.** Because Portal owns an opaque canvas and guarantees its contrast floors against that canvas, a mode mismatch is *jarring, never illegible*. Detection's entire payoff is therefore **aesthetic blending with the surrounding terminal** — which wants the terminal's background, OSC 11's question."
> §8.7's accepted cost: "**OSC 11 is query-only; 2031 pushes on change.** Portal gets *correct-at-startup*, not *live-following* — a terminal that flips mid-session is not noticed until the next launch. Judged thin: terminal backgrounds rarely change mid-session, and when they do it is usually because the terminal is itself following the OS." Also §8.7's third argument for the terminal signal: "**Forward compatibility with transparency** (deferred, not rejected). A transparent theme *must* follow the terminal background, so choosing terminal now makes adding transparency later purely additive."
> Phase boundary: Phase 5 replaces this mapping with the persisted `theme` / `theme_light` / `theme_dark` keys, adds §8.5's fallback and §7.6's fatal message; Phase 6 adds the one-shot `appearance` translation and deletes `prefs.Appearance`.
```

**Resolution**: Pending
**Notes**: `planning.md`'s Phase 3 task table carries a condensed Edge Cases cell for
`theming-system-3-2`; if the fix is approved it should gain the matching condensed clause so
the table and the detail file do not drift — suggested text: `do not adopt DEC mode 2031`
`(ModeLightDark) here or in tasks 5-7 / 9-6 — it is one line away and fully plumbed, but it`
`reports the OS colour scheme where OSC 11 reports the terminal's background, and detection`
`exists for aesthetic blending with the surrounding terminal; correct-at-startup rather than`
`live-following is the named accepted cost, not a defect to fix`. No task file other than
`phase-3-tasks.md` changes.

---

### 2. §7.4's measured-input table and the remedy for a failing **directly-taken** Nord value have no plan home

**Type**: Incomplete coverage
**Spec Reference**: §7.4 (the measured-input table and its stated purpose: "This is the port's *source* material, kept alongside its output because a leg that fails on a value taken **directly** from the palette has no Oklab correction available — the value is Nord's own — and the remedy is instead 'is another slot a better fit?', the move this port already made once (nord8 over nord7)"), §7.4's two declined near-misses
**Plan Reference**: Phase 2, task `theming-system-2-7` (Port Nord — the first genuinely external palette)
**Change Type**: add-to-task

**Details**:

§7.4 keeps a full measured-input table — every Nord colour against Nord's own canvas — and
states in terms why it is kept rather than being working-out discarded once the output
table exists:

> This is the port's *source* material, kept alongside its output because a leg that fails
> on a value taken **directly** from the palette has no Oklab correction available — the
> value is Nord's own — and the remedy is instead "is another slot a better fit?", the move
> this port already made once (nord8 over nord7).
>
> Two declined slots are worth reading off it, because they are the near-misses that make a
> substitution question non-trivial: **nord12 at 4.39** sits just under the 4.50 foreground
> floor — a plausible candidate that fails — and **nord10 at 3.10** clears the 3.00 UI floor
> while failing 4.50, so under §13.5 it is legal for `accent.primary` and illegal for every
> other accent.

None of it is in the plan. `grep -F` for `nord12`, `nord10`, `nord7`, `4.39` and
`better fit` across `planning.md` and all ten `phase-*-tasks.md` returns nothing.

The consequence is a concrete hole in task 2-7's failure handling, not a missing quotation.
Nord's 19 values split into **13 taken directly**, 2 corrected, 3 invented and 1 functional
maximum, and task 2-7's remedy bullet covers only the last two categories:

> If any leg misses, **re-derive rather than relax**: a correction has a published source
> whose chroma must be preserved (Oklab, never HSL lightness); an invention has no source,
> so a new value is settled at a fresh visual gate rather than by arithmetic.

Its Edge Cases repeat the same two-way split ("if it lands on `text.muted`, `text.subtle` or
`bg.attention` the new value is an **invention**"). So a failing **directly-taken** value —
13 of the 19, including every value whose floor an implementer will actually re-measure —
falls into neither bucket. The likely reading is "treat it as a correction", which is
exactly what §7.4 forecloses: there is nothing to correct *toward*, because the value is
Nord's own published colour, and Oklab-shifting it would ship a value under Nord's name that
Nord does not contain, for no floor reason. The correct move is a **slot substitution**, and
the measured table is the only thing that makes it decidable — nord12 and nord10 being
precisely the two candidates that look plausible and are not (nord10 is legal for
`accent.primary` alone).

This is live rather than theoretical. Task 2-3's own edge cases record that two of Nord's
legs clear by under 0.003 (`state.destructive` 4.502234, `state.positive` on `bg.selection`
4.500345) and warn that a different luminance implementation can flip them; both of those
are corrections, so they are covered — but `accent.key` (nord9, 4.640, taken directly) and
`accent.primary` (nord15, 4.409 against a 3.00 floor, taken directly) sit on the same
knife-edge machinery with no remedy written down. §7.4 also uses the table to support the
barrelled-greys claim "as a statement about the whole palette rather than the six values
quoted there", which task 2-7 asserts without the evidence.

Task 2-7 is the only home: it authors the file, walks the floors and owns the visual gate.

**Current** (task `theming-system-2-7`, the final **Do** bullet):

```markdown
- If any leg misses, **re-derive rather than relax**: a correction has a published source whose chroma must be preserved (Oklab, never HSL lightness); an invention has no source, so a new value is settled at a fresh visual gate rather than by arithmetic.
```

**Proposed** (task `theming-system-2-7`, the final **Do** bullet):

```markdown
- If any leg misses, **re-derive rather than relax**, and pick the remedy by how the value was obtained — there are **three** cases, not two. A **correction** has a published source whose chroma must be preserved (Oklab, never HSL lightness). An **invention** has no source, so a new value is settled at a fresh visual gate rather than by arithmetic. A value taken **directly** from the palette (13 of the 19) has **no Oklab correction available at all** — the value is Nord's own, so shifting it would ship a colour under Nord's name that Nord does not contain — and the remedy is instead **"is another slot a better fit?"**, the move this port already made once when `accent.mode` took nord8 `#88C0D0` (6.24) over nord7 `#8FBCBB` (5.99) as Nord's own primary UI accent. §7.4's measured-input table (quoted in Context) is what makes that substitution decidable, and its two declined near-misses are what make it non-trivial: **nord12 `#D08770` at 4.39** sits just under the 4.50 foreground floor — a plausible candidate that fails — and **nord10 `#5E81AC` at 3.10** clears the 3.00 large/UI floor while failing 4.50, so it is legal for `accent.primary` and illegal for every other accent. Only if no slot fits does the value become an invention, and it then takes an invention's fresh visual gate rather than arithmetic.
```

**Current** (task `theming-system-2-7`, the **Edge Cases** bullet on unwalked legs):

```markdown
- A failure on a leg §7.4 did not walk forces a re-derivation, and if it lands on `text.muted`, `text.subtle` or `bg.attention` the new value is an **invention** and needs a fresh visual gate rather than arithmetic.
```

**Proposed** (task `theming-system-2-7`, the **Edge Cases** bullet on unwalked legs, replaced by three):

```markdown
- A failure on a leg §7.4 did not walk forces a re-derivation, and the remedy depends on how the value was obtained: if it lands on `text.muted`, `text.subtle` or `bg.attention` the new value is an **invention** and needs a fresh visual gate rather than arithmetic; if it lands on one of the two **corrections** its published source's chroma must be preserved in Oklab; if it lands on any of the 13 values taken **directly** from the palette there is **no correction available** — the value is Nord's own — and the remedy is a **slot substitution**.
- The precedent for a substitution is already in the port: `accent.mode` took **nord8 over nord7** (6.24 versus 5.99) as Nord's own primary UI accent. §7.4's measured-input table is kept as the port's **source material** for exactly this, and its two declined slots are the near-misses that make the question non-trivial — **nord12 at 4.39** fails the 4.50 foreground floor by a hair, and **nord10 at 3.10** clears the 3.00 large/UI floor while failing 4.50, so it is legal for `accent.primary` and illegal for every other accent.
- The measured table is also what supports the **barrelled-greys** claim as a statement about the whole 16-slot palette rather than about the six values quoted for it — three bright (9.25 / 10.26 / 10.84), three dark (1.24 / 1.45 / 1.69) and nothing between, which is why `text.muted` and `text.subtle` had to be invented at all.
```

**Current** (task `theming-system-2-7`, two adjacent **Context** lines):

```markdown
> §7.4's derivation method: "Contrast **corrections** must be computed in a **perceptual space (Oklab), never by moving HSL lightness**… A correction has a published source value whose chroma must be preserved. An **invented** value has no source to preserve; its constraints are landing in the right band and looking right, which is why `bg.attention` was settled at a visual gate rather than by arithmetic."
> §7.4: "**A failure on an unwalked leg can force re-deriving an *invented* value — which then needs a fresh visual gate.**  The port was twice found incomplete… and each time the completeness claim was plausible enough to pass unexamined. The floor test auto-enumerating the embedded set means a missed leg surfaces at implementation rather than shipping."
```

**Proposed** (task `theming-system-2-7`, the same two **Context** lines with the source table inserted between them):

```markdown
> §7.4's derivation method: "Contrast **corrections** must be computed in a **perceptual space (Oklab), never by moving HSL lightness**… A correction has a published source value whose chroma must be preserved. An **invented** value has no source to preserve; its constraints are landing in the right band and looking right, which is why `bg.attention` was settled at a visual gate rather than by arithmetic."
> §7.4's **measured input** — every Nord colour against Nord's own canvas `nord0 #2E3440` — kept "because a leg that fails on a value taken **directly** from the palette has no Oklab correction available — the value is Nord's own — and the remedy is instead 'is another slot a better fit?', the move this port already made once (nord8 over nord7)":
>
> | slot | ratio | slot | ratio |
> |---|---|---|---|
> | nord1 `#3B4252` | 1.24 | nord9 `#81A1C1` | 4.64 |
> | nord2 `#434C5E` | 1.45 | nord10 `#5E81AC` | 3.10 |
> | nord3 `#4C566A` | 1.69 | nord11 `#BF616A` | 3.05 |
> | nord4 `#D8DEE9` | 9.25 | nord12 `#D08770` | 4.39 |
> | nord5 `#E5E9F0` | 10.26 | nord13 `#EBCB8B` | 8.00 |
> | nord6 `#ECEFF4` | 10.84 | nord14 `#A3BE8C` | 6.13 |
> | nord7 `#8FBCBB` | 5.99 | nord15 `#B48EAD` | 4.41 |
> | nord8 `#88C0D0` | 6.24 | | |
>
> §7.4 on the two declined slots: "**nord12 at 4.39** sits just under the 4.50 foreground floor — a plausible candidate that fails — and **nord10 at 3.10** clears the 3.00 UI floor while failing 4.50, so under §13.5 it is legal for `accent.primary` and illegal for every other accent. This table is also what supports the barrelled-greys claim below as a statement about the whole palette rather than the six values quoted there."
> §7.4: "**A failure on an unwalked leg can force re-deriving an *invented* value — which then needs a fresh visual gate.**  The port was twice found incomplete… and each time the completeness claim was plausible enough to pass unexamined. The floor test auto-enumerating the embedded set means a missed leg surfaces at implementation rather than shipping."
```

**Resolution**: Pending
**Notes**: `planning.md`'s Phase 2 task table carries a condensed Edge Cases cell for
`theming-system-2-7`; if the fix is approved it should gain the matching condensed clause —
suggested text: `a failing directly-taken value has no Oklab correction available (the value`
`is Nord's own), so the remedy is a slot substitution — the nord8-over-nord7 move — read off`
`§7.4's measured-input table, whose declined near-misses are nord12 at 4.39 and nord10 at`
`3.10 (legal for accent.primary only)`. No task file other than `phase-2-tasks.md` changes.
Task 2-7's per-value `#` comment rule ("the five judgements only") is deliberately left
unchanged by this fix — the substitution precedent belongs in the task, not as a fourteenth
comment in the shipped file.
