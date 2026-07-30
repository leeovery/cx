# Review Tracking: Theming System - Input Review

Cycle 4. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. Mid-session constant → adaptive names the wrong slot as the one needing a read, and the slot that genuinely was never loaded is unspecified

**Source**: `discussion/theming-system.md` — "Construction timing under the adaptive default (the review's F1)" (lines 1301–1305: *"that dissolved on the grounds that the OSC 11 **answer** is already in hand, but **the other slot's *file* would not have been read at construction** (a constant nominates one theme). Assigning a slot therefore reads that slot's file at commit time"*) and "Two undefined transitions" (lines 1463–1478)
**Category**: Enhancement to existing topic
**Affects**: §8.4 (Construction timing — the mid-session paragraph), §9.3 (Mid-session constant → adaptive); cross-refs §8.2, §5.8, §12.3

**Details**:

The source records the constant→adaptive conversion as having **two** halves, and names them separately: the OSC 11 *answer* (already in hand, so the transition "dissolves") and the *file* — *"the other slot's file would not have been read at construction"*. The specification carries the first half in §9.3 (which is entirely about the OSC 11 answer and mentions no file read at all) and the second half in one sentence of §8.4 — but that sentence loses the source's word *other*, and with it the actual mechanic.

As written, §8.4 says assigning a slot *"reads **that slot's** file at keypress time"* — the slot the cursor is on. Under §5.8 the panel already holds that theme's parse result for the panel's lifetime (that is precisely what makes arrowing the O(1) restyle of §11.1), so the read §8.4 specifies is the one read that is **not** needed. Meanwhile the read that *is* needed is unstated:

- User is on `"theme": "nord"` — construction loaded exactly one theme (§8.4, §5.7).
- They press `l` on `tokyo-night-day`. §8.2's mutual exclusion clears the constant; the user is now adaptive, with `theme_dark` **unset and therefore holding the shipped default** `tokyo-night` (§8.3).
- That dark slot's theme was never loaded. In a dark terminal, `Esc` resolves persisted state (§9.2) onto exactly that theme.

The spec already recognises the *visible* form of this in §8.2 — *"`d`/`l` clears the constant and the **other** stale hand-edited slot becomes live in the same keypress"* — where the other slot can name an arbitrary drop-in that construction never touched, and which may itself be unloadable and require the §8.5 fallback. So the newly-live slot needs a load (and possibly a fallback resolution) on a path the spec does not describe.

Two consequences follow that are also unstated:

- **Where the load comes from.** For an untouched slot the value is a built-in, so it resolves from the embedded set and never touches the themes directory (§8.4's ordering rule) — cheap and infallible. For a hand-edited stale slug it is a by-name directory read that can fail into §8.5. Those are materially different costs on a keypress path the spec elsewhere budgets carefully.
- **§12.3's closed event catalogue has no home for it.** `theme: loaded` is declared as firing *"At TUI construction"*. A theme loaded at commit time emits nothing, so the one load that happens outside construction falls outside a catalogue the spec declares closed and spec-governed.

**Current**:

> §8.4: **Mid-session slot assignment reads at commit time.** A constant nominates one theme, so assigning a slot (converting the user to adaptive in-session) reads that slot's file at keypress time — already the panel's cost model.

> §9.3: Assigning a slot converts a constant-theme user to adaptive in-session, which needs a light/dark answer their launch deliberately never waited for.
>
> **This dissolves.** `restore.go` issues the OSC 11 query from `Init` **regardless** — it needs the original background to restore on exit, independent of detection. […]

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. §7.7's re-derivation check drops the chroma measurement that is the whole point of the check

**Source**: `discussion/theming-system.md` — "MV's own erratum values — the check has an owner (the review's F10)" (lines 2091–2093: *"Re-derive the six corrected light values in Oklab, **measure chroma loss**, and give a fresh visual gate to any that moved materially"*), against the method note it derives from (lines 830–842: the rejected `#CF888F` *"lost ~27% of Nord's red saturation"*; `#DD8188` *"retains ~94% of the chroma at the identical ratio"*)
**Category**: Enhancement to existing topic
**Affects**: §7.7 (the owned check and its acceptance criteria); cross-ref §7.4 (derivation method)

**Details**:

The source gives the check three steps — re-derive in Oklab, **measure chroma loss**, gate anything that moved materially. The specification carries the first and third and drops the second, substituting a single Oklab ΔE threshold for it.

That matters because chroma is the quantity the check exists to interrogate, not a synonym for ΔE. §7.7's own opening states the suspicion in chroma terms — MV's values are described in-source as *"darkened, hue-preserved"*, which *"may carry the same chroma flaw as the rejected Nord red"* — and the Nord precedent is explicit that the flaw is invisible to a naive derivation and was diagnosed **by measuring chroma against the source value** (~27% lost, rejected; ~94% retained, shipped). §7.4 then makes chroma preservation the standing rule for corrections and records the ~94% figure precisely because it is *"the figure that makes the shipped value checkable against the derivation rule … if it is ever re-derived."*

Under the spec as written, the six MV corrections get no such figure. ΔE(shipped, re-derivation) answers "did the re-derivation land somewhere else"; it does not answer "how much chroma did the shipped value give up against its original" — the two can disagree in both directions (a shipped value can sit within ΔE 0.05 of the re-derivation while both have shed chroma against the original, and a value can exceed the threshold on lightness alone with chroma intact). The distinction is exactly the one §7.4's derivation-method paragraph draws.

It also leaves §7.7's recording obligation thin: the spec requires that *"a passing check is a finding, not a non-event"* and must be recorded — but with no chroma figure, what is recorded is one delta per value, and the six MV light corrections remain the only corrections in the built-in set with no chroma record beside them, unlike Nord's two.

**Current**:

> **Owned by this feature's implementation, before MV's values are frozen into theme files:** re-derive the six corrected light values in Oklab — the minimal-Oklab-distance colour that clears the same floor — and compare each against the shipped value.
>
> **Threshold: Oklab ΔE ≥ 0.05 is "moved materially".** The Nord port anchors the scale at the other end (ΔE 0.018, cited as essentially imperceptible), and 0.05 is comfortably above that while still well below a difference anyone would describe as a colour change. Under it, nothing happens.
>
> **Acceptance criteria, so the check has a determinate outcome either way:**
>
> - **Every value under threshold** → the check passes, `§7.3`'s tables stand, nothing moves, and the result is recorded (a passing check is a finding, not a non-event).

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
