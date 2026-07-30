# Review Tracking: Theming System - Input Review

Cycle 10. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. The reason the panel is not a modal — the source's founding constraint — is absent, and three sections each justify the shape on weaker grounds instead

**Source**: `discussion/theming-system.md` — Context, deliverable 3 (lines 16–20: *"Lee's picture is a TUI 'slide-over' overlaying the right-hand strip, arrowing the list re-theming the main view live behind it (**a modal blanks the screen to the canvas, so a modal picker has nothing to preview against**)"*)
**Category**: Enhancement to existing topic
**Affects**: §9.1 (Shape and placement); cross-refs §9.2 (the inline confirm), §9.6 (Preview page), §9.11, §1.1

**Details**:

The source states the panel's shape as a *forced* choice, in the sentence that first describes the feature: Portal's modals blank the page to the canvas (`modal.go` clears to canvas + `placeModalOnClearedCanvas`), so a modal theme picker would render canvas plus its own frame and **preview nothing**. Live preview is the feature; a modal cannot deliver it. That is the whole reason the slide-over exists.

The specification never says it. §9.1 opens on the shape and gives an *aesthetic* reason — *"deliberately not an inset bordered panel like the modals, so it reads as a slide-over rather than a floating dialog"* — which is a statement about how it reads, not about what a modal would cost. The consequence is that the three places resting on the constraint each reach for a different, weaker justification:

- **§9.2's inline confirm** justifies itself on the *stacking* rule (*"stacking a modal over an overlay is the shape §9.6 rejects for the Preview page"*) rather than on the blanking that makes a modal wrong here in the first place. A reader could conclude the confirm could have been a modal if only the panel weren't an overlay.
- **§9.6 rejects the Preview page** on two grounds, one of which is overlay-on-overlay — again stacking, not blanking.
- **§9.11 ("everything re-themes")** argues from honest preview and the guard carve-out. The blanking constraint is the same argument one level down: the panel is the only chrome on screen that a modal *would* have shown, which is why fixing it would be visible.

It also leaves §9.1's largest accepted cost under-defended. §9.1 spends a paragraph accepting that the overlay covers three footer entries and cuts a label mid-word; a reader who does not know a modal is disqualified can reasonably ask why a centred modal — which costs no footer at all — was not used. The answer is that it costs the entire preview, and it is not in the document.

Downstream, the same constraint is what produces the panel's ~24–30 column budget, and that budget is load-bearing in four places: §9.5's four-element row-composition priority and its truncation floor, §9.8's degrade-then-refuse ladder, §9.1's message-slot truncation rule, and §14A's *"in the panel the wording is a **layout constraint** as much as a copy choice"*. All of it follows from "it must not blank", which is stated nowhere.

**Current**:

> §9.1: A **full-height, right-edge, non-blanking overlay** with a **left border only** — deliberately *not* an inset bordered panel like the modals, so it reads as a slide-over rather than a floating dialog.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §9.1 now states the founding constraint: Portal's modals blank the page to canvas before drawing, so a modal picker would render canvas plus its own frame and preview nothing — non-blanking is the only shape that can do the job, not a preference. Recorded what inherits from it (the column budget, row composition, message truncation, §14A's layout-constraint framing, the accepted footer cost) and that it is the deeper reason behind §9.2's inline confirm and §9.6's Preview refusal, both of which had been resting on the weaker stacking argument.

---

### 2. Research flagged three naming issues; the spec disposes of two and leaves the `text.on-*` pairing names undisposed, outside its own naming taxonomy

**Source**: `discussion/theming-system.md` — "Scope — full audit of all 19, including the hue-named accents" (lines 660–664: *"Research's audit flagged three issues (`bg.track` is use-site-derived; **the `text.on-*` tokens are pairing names coupling to other tokens**; the text ramp encodes no ordering). It did not flag the largest one…"*) and the decision it produced (lines 699–703: *"**Full audit of all 19 tokens**"*)
**Category**: Enhancement to existing topic
**Affects**: §2.3 (Naming principle), §2.6 (Accepted ambiguities); cross-refs §2.4 (row 19), §2.5, §4.6

**Details**:

The source names exactly three research-flagged naming issues and then decides on a full audit of all 19 tokens. Two of the three are visibly disposed of in the specification:

- `bg.track` is use-site-derived → §2.4 row 17 renames it `bg.subtle`, *"use-site → intrinsic weight"*.
- The text ramp encodes no ordering → §2.4 rows 2/3/5 introduce ordinals, §2.6.1 records the residual ambiguity at the ramp's middle join, and §2.7 puts the ordering in `docs/theming.md`.

The third — **the `text.on-*` tokens are pairing names that couple to other tokens** — gets no disposition anywhere. Both names survive the audit unchanged in kind, and §2.4 row 19's stated reason is literally the coupling (*"lockstep with `bg.attention`"*).

That matters for three reasons, all of which are the specification's own:

- **§2.3 is the public contract's stated naming principle and it does not classify these two tokens.** Its table admits three kinds — a place (wrong), a hue (wrong), a meaning (right) — with a carve-out that the ramp and border want *weight*. `text.on-selection` and `text.on-attention` are none of these: they name **another token**. So the two tokens that most obviously depart from the principle read as unexamined rather than accepted, in the section a theme author reads to understand why the vocabulary is shaped the way it is.
- **§2.6 is exactly the place the spec records this class of call**, and it carries three entries. A fourth belongs there if pairing names are accepted, which the rename table shows they are.
- **The coupling is a real constraint inside a public contract, and it is one-directional.** Renaming `bg.attention` forces renaming `text.on-attention` — §2.4 row 19 already records the lockstep as a fact — and under §4.6 a rename means every drop-in using the old key fails `missing tokens`. This is the one place in the 19 where a single rename is necessarily two breaking changes, which is worth stating in a vocabulary whose entire justification (§1.3) is that renaming is cheap for built-ins and breaking for users.

The disposition itself is not in doubt — the names are kept and §2.5 already documents the pairing in each role's meaning (*"pairs against `bg.selection`"* / *"pairs against `bg.attention`"*), and §13.5's foreground-on-tint rules are what the names announce. What is missing is the record that this was one of the flagged issues, that a pairing name is a deliberate third kind rather than an oversight, and what it costs.

**Current**:

> §2.3: Three naming failures are in play; two are failures:
>
> | Kind | Example | Verdict |
> |---|---|---|
> | A **place** | `border.footer` | Wrong — goes stale as other surfaces reuse the token. |
> | A **hue** | `accent.violet` | Wrong — lies in every port. […] |
> | A **meaning** | `state.destructive` | Right — stays true regardless of palette or where it is drawn. |
>
> This does **not** make everything weight-based. The text ramp and the border want intrinsic-**weight** names because their role genuinely is "how prominent". The accents want **meaning** names because a theme author needs to know what a colour signifies in order to choose one.
>
> §2.6: Three spots were flagged as genuinely arguable and resolved to the values above: 1. The ramp's middle join. […] 2. **`accent.key`** […] 3. **`bg.subtle`** […]

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §2.3 now admits a deliberate fourth naming kind — a *pairing* name — with the reasoning that the `text.on-*` roles are genuinely relational (they exist only to be legible on a specific tint, and §13.5 floors them as pairings) and the cost stated: renaming `bg.attention` forces renaming `text.on-attention` in lockstep, the only place in the 19 where one rename is necessarily two breaking changes. §2.6 gains a fourth accepted-ambiguity entry, which is the third of research's three flagged naming issues finally disposed of.

---

### 3. The Nord port's *input* — the measured source palette — is not carried, and two slots the port declined are near-misses that never appear

**Source**: `discussion/theming-system.md` — "The Nord port, analysed against the real spec" (lines 798–809: the sixteen-row table, *"Every Nord colour measured against Nord's own canvas (`nord0 #2E3440`)"*)
**Category**: Enhancement to existing topic
**Affects**: §7.4 (The Nord port)

**Details**:

The source's port rests on one measured artefact: all sixteen Nord colours with their contrast ratio against Nord's own canvas. §7.4 carries the port's *output* — a token-by-token table of what was chosen — but not its input. The slots the port used appear (with their ratios); the slots it declined mostly do not.

Two never appear anywhere in the specification: **nord10 `#5E81AC` at 3.10** and **nord12 `#D08770` at 4.39**. (nord7 survives only because §7.4 records it as the alternative `accent.mode` lost to.)

This is load-bearing for the same reason §7.4's legs table is, and the spec says so itself: it frames that table as *"the port's verification baseline, to be re-checked if any value moves (§7.7)"*, and states that a failure on an unwalked leg *"can force re-deriving an **invented** value"* while the floor test auto-enumerating the embedded set means *"a missed leg surfaces at implementation rather than shipping"*. When a leg fails on a value **taken directly** from Nord, the remedy is not Oklab re-derivation — there is nothing to correct, the value is the palette's own. The remedy is "is another slot in this palette a better fit?", which is precisely the move the port already made once and recorded (nord8 over nord7). Answering it needs the measured palette, and the specification does not have it.

The two absent slots are exactly the ones that make the question non-obvious rather than trivial:

- **nord12 at 4.39** sits just under the 4.50 foreground floor — so it is a plausible-looking candidate that fails, and nothing in §7.4 records that it was measured and excluded.
- **nord10 at 3.10** clears the 3.00 UI floor and fails 4.50 — so under §13.5's rule set it is legal for `accent.primary` and illegal for every other accent, which is the discrimination §7.4's own read-off paragraph goes out of its way to make explicit.

It is also the only place the port's central structural claim is checkable in full. §7.4 asserts Nord's greys are *"barrelled at the ends"* and cites the six grey ratios, but the assertion is about the palette as a whole; the sixteen-row measurement is what supports it, and it is the same completeness class the source's own Key Insight #4 warns about — this port having been found incomplete twice on exactly that basis.

**Current**:

> §7.4: Nord is a 16-slot ANSI palette (Polar Night `nord0–3`, Snow Storm `nord4–6`, Frost `nord7–10`, Aurora `nord11–15`). Portal's 19-token vocabulary is meaningfully wider than 16 slots **at the dark end**, so the port takes 14 values directly, **corrects two**, and **invents three**.
>
> **`nord.theme`:**
>
> | Token | Value | Source | Ratio vs canvas |
> |---|---|---|---|
> | `canvas` | `#2E3440` | nord0 | — |
> | … | | | |

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §7.4 now carries the port's measured *input* — all sixteen Nord colours against nord0 — alongside its output, with the reasoning that a leg failing on a directly-taken value has no Oklab correction available (the value is Nord's own) so the remedy is slot substitution, which needs the palette. The two absent near-misses read off explicitly: nord12 at 4.39 just under the foreground floor, nord10 at 3.10 legal for accent.primary and illegal for every other accent.
