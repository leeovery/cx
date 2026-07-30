# Review Tracking: Theming System - Input Review

Cycle 2. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. The Nord port's verification baseline omits four measured legs and the accent floor

**Source**: `discussion/theming-system.md` — "The Nord port, analysed against the real spec" (lines 811–819, 856–857, 836) and "The unwalked legs — a second correction" (lines 906–917)
**Category**: Enhancement to existing topic
**Affects**: §7.4 (The Nord port) — the "pairing legs the port was verified against" table, and Correction 1

**Details**:

The spec frames §7.4's legs table as *"the port's verification baseline, to be re-checked if any value moves (§7.7)"* — and §7.7 makes it likely that values **will** move (the Oklab re-derivation check can push a value over threshold, and §7.4 itself warns that a failure on an unwalked leg can force re-deriving an invented value). The table is therefore load-bearing as a *complete* rule set, not as a sample. The discussion's own Key Insight #4 is precisely that *"a completeness claim is only as good as the set it was measured against"* — this port was twice found incomplete on exactly that basis.

Four measurements/floors recorded in the source are absent from the spec:

1. **`text.primary` (nord6) on `bg.selection` = 7.49 ≥ 4.50.** Discussion line 817: *"`bg.selection` ← nord2 (fill 1.45 ≥ 1.10, and nord6 on it is 7.49 ≥ 4.50)"*. The spec's table walks `text.on-selection`, `text.secondary` and `text.tertiary` on `bg.selection` but not `text.primary` — the brightest ramp token and the one the selected-row name renders in.
2. **`bg.selection` fill vs canvas = 1.45 ≥ 1.10.** Same source line. The spec's table carries the `bg.subtle` fill leg but not the `bg.selection` fill leg; the 1.45 figure survives only in the Nord token table's ratio column, unlabelled as a floor.
3. **The warning-band pair rule has three legs, of which the spec's table carries one.** Discussion line 856–857: *"The rule set (`TestBgWarningPairRule`) has three legs: text-on-tint ≥ 4.5, the accent bar ≥ 3.0 vs canvas, and the fill ≥ 1.1 vs canvas."* The spec's table has only `text.on-attention` on `bg.attention` (9.02). The `bg.attention` fill leg (1.20 ≥ 1.10) and the `accent.attention` bar leg (8.00 ≥ 3.00) are absent — and `bg.attention` is the invented value most likely to be re-derived, so its own floor being unstated is the sharpest instance.
4. **The accent floor is ≥ 3.00 and is never stated.** Discussion line 816: *"`accent.primary` ← nord15 (4.41 ≥ 3.00)"*. The spec gives ratios for `accent.primary` (4.41) and `accent.key` (4.64) with no floor beside them, so a reader cannot tell whether 4.41 is comfortable or marginal — while `accent.mode` is separately floored at ≥ 4.50 (peek chrome) in the table, which makes the absence read as if no accent floor exists.

Related, in the same section: the spec records that the *rejected* red `#CF888F` lost ~27% of Nord's chroma, but not the *accepted* value's counterpart figure — discussion line 836: `#DD8188` *"retains ~94% of the chroma at the identical ratio"*. §7.4's derivation-method paragraph makes chroma preservation the rule for corrections; the achieved figure is what makes the shipped value checkable against that rule if it is ever re-derived.

**Current**:

> **The pairing legs the port was verified against.** The per-token ratios above are only half the rule set — the second correction was found by walking the *pairing* legs. This is the port's verification baseline, to be re-checked if any value moves (§7.7):
>
> | Leg | Nord | Floor | |
> |---|---|---|---|
> | `bg.subtle` fill vs canvas | 1.24 | ≥ 1.10 | ✓ |
> | `state.positive` on `bg.selection` | 4.23 → **4.50** corrected | ≥ 4.50 | ✗ → ✓ |
> | `text.on-selection` on `bg.selection` | 8.63 | ≥ 4.50 | ✓ |
> | `text.secondary` on `bg.selection` | 7.09 | ≥ 4.50 | ✓ |
> | `text.tertiary` on `bg.selection` | 6.39 | ≥ 4.50 | ✓ |
> | `text.on-attention` on `bg.attention` | 9.02 | ≥ 4.50 | ✓ |
> | `accent.mode` vs canvas (peek chrome) | 6.24 | ≥ 4.50 | ✓ |
> | `text.subtle` band | 3.18 | 3.00–4.49 | ✓ |
> | `text.faint` band | 1.69 | 1.00–2.99 | ✓ |
> | `state.destructive` vs canvas | 4.50 | ≥ 4.50 | ✓ |

(And, in Correction 1: *"Shipped corrected as `#DD8188` (4.50)."*)

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. The slide-over's own body background and left border are not bound to tokens

**Source**: `discussion/theming-system.md` — "Paper spike — two marker/slot treatments" (lines 1596–1604: frames built with *"panel chrome `#0C0C16` on `#2B3050` border"*) and "The panel re-themes too" (lines 1508–1539)
**Category**: Gap/Ambiguity
**Affects**: §9.1 (Shape and placement), §9.11 (Everything re-themes, panel included), §2.5 (Role meanings)

**Details**:

§9.1 names a token for three panel elements — the header label (`accent.mode`), the header rule (`border`), and the cursor row (the shipped selection treatment) — but never says what paints the two largest surfaces the panel introduces: **its body background** and **its left border**.

This is not a free implementation choice, because three other decisions close around it:

- §9.11 says the panel re-themes *"No exceptions"*, and §13.4's swap-and-diff guard will fail on any panel element that is not derived from the active theme.
- §2.1's colour-literal guard forbids raw hex at call sites, so every panel surface must resolve to one of the 19 tokens.
- The reference frames the discussion built the panel against use **`#0C0C16` chrome on a `#2B3050` border** — neither of which is a token value (`canvas` is `#0b0c14`, `border` is `#292E42`). The frame's panel body is deliberately a *slightly different* near-black from the canvas, and its border is a lighter blue-violet than `border`. Under the closed 19-token vocabulary that distinction cannot be expressed.

So one of two things is true and the spec should say which: either the panel body is `canvas` and the left border is `border` (the frame's chrome/border distinction is a per-frame literal and is dropped — consistent with §9.14's *"the frames are reference, never truth"* caution), or the design wants a distinct panel tone, which would need a 20th token and would reopen §2.1's count and §4.6's vocabulary-evolution path. Left unstated, an implementer meets the guard failure at build time with no decided answer, and the §9.14 caution is exactly the sort of note that makes silently diverging from the frame look like a mistake.

Note this also decides whether the panel's non-blanking premise reads correctly: if the panel body is `canvas`, the panel is distinguished from the list behind it by its left border alone.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. `CLAUDE.md`'s stale entries beyond the `testdata/vhs/` description

**Source**: `discussion/theming-system.md` — "The premise correction — committed PNGs were scaffolding, not an asset" (lines 2349–2351: *"Note this contradicts how `CLAUDE.md` currently describes `testdata/vhs/` … Worth correcting when the docs are updated"*)
**Category**: Enhancement to existing topic
**Affects**: §12.5 (README and CHANGELOG)

**Details**:

The source flags `CLAUDE.md` as needing correction **at doc-update time**, giving the `testdata/vhs/` description as the instance it noticed. The spec carries that one instance and stops there — but this feature's own decisions invalidate three further `CLAUDE.md` entries, all of which are load-bearing agent-facing architecture documentation:

- **The `tui/theme` row** describes *"~20 named semantic role tokens … each with a **Light and Dark** variant; `Token.ColorFor(mode)` resolves per the owned-canvas mode, `theme.MV` is the single built-in theme, and `Mode`'s zero value (`Dark`) is the no-answer fallback"*, plus `contrast_test.go` measuring against two hardcoded canvases. §2.1, §3.2 and §13.5 delete every clause of that.
- **The `prefs` row** documents the `appearance` override, the `Appearance` enum (`auto|light|dark`) and its tolerant decode, and `cmd/open.go`'s `WithAppearance` wiring. §8.1 and §8.8 delete all of it and replace it with `theme` / `theme_light` / `theme_dark` / `theme_migrated`.
- **The logging section** pins the taxonomy at *"17 component names"* with the `spawn` and `resolve` additions named as the precedent for amendment. §12.3 adds an 18th (`theme`) with its own attr keys — the same shape of amendment those two carried, which is why the count is stated at all.

§12.5 is the spec's only home for doc consequences, and it already reaches into `CLAUDE.md` for one item, so the omission reads as "these were not noticed" rather than "these were judged out of scope". It also matters more than usual here: `CLAUDE.md` is what an implementing agent reads first, and three of its entries would actively describe the pre-feature world while the work is under way.

**Current**:

> **`CLAUDE.md` needs correcting too:** it currently describes `testdata/vhs/` as committed reference PNGs forming a visual-verification harness, which reads as a durable asset. It is not (§13.2).

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
