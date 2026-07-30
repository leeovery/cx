# Review Tracking: Theming System - Input Review

Cycle 7. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. The swap-and-diff guard's scope — the source pins it to *every* fixture, auto-enumerated; the spec carries the property but not the mechanism

**Source**: `discussion/theming-system.md` — "The real risk is completeness — guarded behaviourally" (lines 1748–1753: *"Render **every capture fixture** under theme A, swap to theme B, render again… The offline capture harness already renders every canonical screen deterministically through the shared `tui.Build` constructor with every tmux seam faked, so **the fixture list *is* the coverage list, and it grows automatically as screens are added**."*)
**Category**: Enhancement to existing topic
**Affects**: §13.4 (The swap-and-diff completeness guard); cross-refs §13.2, §13.3, §11.2

**Details**:

The source states the guard's coverage as a *mechanism*: it drives **every** capture fixture, and because the harness's fixture set is the enumeration, the coverage list grows automatically as screens are added. The specification carries the **properties** that mechanism delivers — "it catches *any* missed site — including ones added years later — without anyone having to remember a rule", assertion 2's "union across fixtures", assertion 3's "every token is exercised by at least one fixture", and the carve-out excluding colourless fixtures — but never states the mechanism itself. §13.4 opens with "render **a screen** under theme A" (singular), and nothing anywhere requires the guard to enumerate the fixture registry rather than hold a list.

The gap is not cosmetic, because the spec's own claim depends on it:

- **All three assertions can be satisfied by a hand-picked subset.** Nineteen tokens do not need many fixtures; a test that names four or five fixtures explicitly passes assertion 3 today and keeps passing tomorrow. The next screen anyone adds is then silently uncovered — which is exactly the failure "including ones added years later" asserts immunity from, and the guard exists precisely because the missed sites *cannot* be found by reading code (§13.4's own opening).
- **§13.2's justification for keeping fixtures rests on the same unstated property** — "the swap-and-diff guard drives the fixture renderer and its coverage assertion needs the fixture set to exist" — which is an argument about the *set*, not about whichever fixtures a test happens to name.
- **Two fixtures this feature adds are only guaranteed to be in the guard if the guard enumerates**: §13.3's four new panel fixtures, and §11.2's requirement that one panel fixture carries enough rows to paginate ("Otherwise the guard is blind at exactly the new site this paragraph adds") — a requirement that presumes the guard renders that fixture without saying what puts it there.

One sentence fixes it: the guard enumerates the harness's fixture set (every fixture except the colourless ones), rather than naming fixtures.

**Current**:

> §13.4: **What it is:** render a screen under theme A, switch to theme B, render again, and scan the second output for any colour value belonging to theme A. A survivor means some element never got the new theme — the "assert no stale data survived the invalidation" trick applied to rendered output rather than a cache. It exists because the cached styles `bubbles/list` holds cannot reliably be found by reading code (§11.2).
>
> This is a **behavioural** guard, not a structural one, deliberately. It catches *any* missed site — including ones added years later — without anyone having to remember a rule. […]
>
> 2. **Every expected theme-B value is present** — catching a site that renders *nothing* rather than merely stale. This is a **union across fixtures**, not per fixture: no single screen renders all 19 roles.
> 3. **Every token is exercised by at least one fixture.** […]
>
> **Colourless fixtures are excluded.** A colourless render contains no theme hexes, so there is nothing to diff — inclusion would be meaningless rather than merely redundant.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. What the panel covers omits the footer *entries* the source names — and §14's promotion of `t` and `m` put two more under the overlay, unreconciled with §14.4's no-truncation rule

**Source**: `discussion/theming-system.md` — "Paper spike — two marker/slot treatments" (lines 1610–1613: *"**Non-blanking overlay.** The Sessions list stays fully visible; the panel covers the right column, **which visibly costs the footer's `x projects` and `? help`** — exactly the accepted trade recorded above."*), read against "Footer keymap revision — discoverability" (lines 2154–2173, which appends `t theme · m multi` to the same right end)
**Category**: Enhancement to existing topic
**Affects**: §9.1 (Shape and placement — "What the panel covers"); cross-refs §14.2, §14.3, §14.4

**Details**:

The source's record of what the overlay costs names a footer **entry** — `x projects` — alongside `? help`. The specification's version drops that half: it lists "the footer's right-aligned `? help`, the right-side header hint, and session row meta", and then argues the trade is fine because "the theme is carried almost entirely by the *left* of the screen (session names, cursor bar, group headers, **footer key glyphs**)". That argument was true of the footer as it stood when the frames were built. §14 then changed the footer: `t theme · m multi` are appended to the right end, so the panel now covers **three** entries and their `accent.key` glyphs, not one — and §14.3 measures the Sessions footer as fitting 86 columns "with ~5px spare and no headroom", so a 24–30 column overlay eats roughly a third of a row that is already full.

Two things follow that the spec does not state:

- **The overlay slices a label mid-word.** The panel is opaque and boundary-agnostic, so whatever sits under its left border is cut wherever the cut falls — `x proje▏`. §14.4 rules on exactly this shape for the width ladder: "**never wrap or truncate a label**… a truncated `x proje…` advertises nothing while costing the same space." The two rules are not in conflict (one governs layout, the other an overlay), but nothing says which applies when the panel is open, and an implementer meeting a half-rendered footer entry has a spec sentence that reads like a prohibition against it.
- **The alternative is a real design choice, not a non-question.** Re-laying the main screen out to the reduced width while the panel is open would put the footer through §14.4's right-to-left drop ladder and produce a clean edge — at the cost of reflowing the surface the panel exists to preview, which contradicts §9.1's non-blanking premise and §11.1's O(1)-restyle-only swap path. The source accepted covering; the spec should say so explicitly, because the accepted-trade paragraph is where a reader looks for it and it currently disposes of a smaller loss than the feature now takes.

Worth noting the mild irony that makes it visible: after §14.1 promotes `t` to core, **the panel's own key is one of the entries the panel covers**.

**Current**:

> §9.1: **What the panel covers:** the right-hand column, where the footer's right-aligned `? help`, the right-side header hint, and session row meta live. **Accepted** — the theme is carried almost entirely by the *left* of the screen (session names, cursor bar, group headers, footer key glyphs), while the right edge is metadata. The overlay covers the least theme-informative part of the screen, which is exactly what a preview surface wants.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
