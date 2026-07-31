# Review Tracking: Theming System - Input Review

Cycle 12. Full fresh pass over the whole specification against the whole discussion source, with particular attention to whether the consolidation pass lost anything decided.

**Consolidation-loss check result:** no decided value, threshold, reason class, log event, test, pinned string, or rejected alternative from the source was found missing. Every Lee-attributed decision in the source (26 quoted positions), all eleven research leans, all seven reason classes, all three floors and both bands, the full Nord value/leg tables, the six-plus-one erratum set, the seven-event log catalogue, the ten rewritten floor tests, the four deferred-by-decision clusters and all three MV spec amendments are present and findable. The findings below are three pieces of **supporting evidence for shipped decisions** that exist in the source and have no home in the specification — not lost decisions.

## Findings

### 1. DEC mode 2031 is rejected on semantics, but the source's finding that it is fully plumbed end-to-end is absent — so the rejection reads as possibly a feasibility one

**Source**: Discussion, *Light/dark detection* → Journey step 3 ("DEC mode 2031 was found to be plumbed end-to-end in Portal's stack") and step 4 ("FALSE PATH — 2031 answers a different question")

**Category**: Enhancement to existing topic
**Affects**: §8.7 (Light/dark detection)

**Details**:
§8.7 states 2031 is "deliberately **not** adopted" and gives the semantic argument (`ModeLightDark` reports the OS preference, OSC 11 reports the terminal background; they routinely disagree). What it does not record is the source's finding that **2031 is available and cheap**: `x/ansi` carries the mode constants and report parsers, `ultraviolet` decodes DSR `997;1`/`997;2` into typed events, Bubble Tea v2 passes them to `Update` verbatim (`type Msg = uv.Event` is a type *alias*, and `translateInputEvent` returns unhandled events bare), Portal opts in with a single `tea.Raw(ansi.SetModeLightDark)`, tmux 3.6+ supports it and the installed tmux is 3.7b.

That finding is load-bearing in two directions the specification currently leaves open:

- It establishes the rejection is **purely about which question the signal answers**, not about availability or cost. Without it a reader meeting §8.7 can reasonably conclude 2031 was declined because it is not reachable from Portal's stack — and re-open it on discovering that it is.
- It is the same class of fact §3.1 deliberately keeps for Lipgloss v2's `LightDark`: *"a standing fact about a live dependency rather than a discarded option — both APIs are in the tree Portal builds against, and an implementer … will meet them and reasonably ask why Portal hand-rolls a light/dark decision the library has an API for."* 2031 is exactly that shape — the plumbing is in the tree, and §8.8 requires the implementer to work in the gate that would consume it.

The source also records the *misreading* explicitly (`FALSE PATH — 2031 answers a different question`), and the specification carries the correction without the thing that made it a false path.

**Current** (§8.7, second paragraph):

> The two answer different questions: `ModeLightDark` reports *the operating system's* colour-scheme preference; OSC 11 reports *what colour the terminal's background is*. They routinely disagree — a terminal pinned dark on a light OS is the canonical case. On terminals that don't support 2031, tmux *synthesises* the answer by guessing from the background colour anyway.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. The **dark** no-answer fallback is asserted in five places with no recorded grounding, and the source carries both the ecosystem universality and the fact that Helix models it as an explicit configurable value

**Source**: Discussion, *Light/dark detection* → Decision → Config shape, third bullet: *"A no-answer resolves to the **dark** slot (Helix carries an explicit third `fallback` value for exactly this, defaulting to dark; Helix, Neovim, delta and Glamour v2 all use dark as the universal no-answer fallback)."*

**Category**: Enhancement to existing topic
**Affects**: §8.8 (What survives and what dies in the appearance gate) — primary home; referenced by §8.3, §9.3, §9.10, §13.3

**Details**:
"Dark" as the no-answer resolution is stated as a standing rule in five places — §8.3 (*"no answer resolves to dark, so the adaptive pair is a superset of a constant dark default"*), §8.8 (*"**dark** no-answer fallback still apply"*), §9.3 (*"it falls to **dark**, the same rule as everywhere else"*), §9.10 (*"the standing **dark** no-answer fallback selects the active member"*), and §13.3's fixture coherence rule (*"`capturetool` runs no gate, so the standing no-answer fallback selects dark"*). Nowhere does the specification say **why dark**.

Two things in the source fill that, and both matter more under split than they did before:

- **It is the ecosystem's universal choice** — Helix, Neovim, delta and Glamour v2 all use dark as the no-answer fallback. §8.3 already carries ecosystem grounding for the *adjacent* decision (detect-by-default, citing `bat`/`delta`/Neovim/`yazi`) precisely because that decision "ships a named risk to every install"; the no-answer resolution is the other half of the same mechanism and currently ships ungrounded.
- **Helix carries an explicit third `fallback` setting for exactly this, defaulting to dark.** Portal hardcodes it. That is a real shape difference from the tool whose two-slot form Portal otherwise adopts wholesale, and recording it is what stops a future reader assuming the hardcoding was an oversight rather than a choice.

The stake is higher post-split for the reason §8.8 itself gives: the fallback no longer selects a variant of one palette, it selects an entire named theme (potentially Nord over Tokyo Night Day). A rule with that reach carrying no recorded justification is the gap.

**Current** (§8.8, "Survives, but conditional" row):

> | **Survives, but conditional** | The detect-or-timeout first-paint gate. A user on a **constant** theme needs no detection, so their first paint is immediate — a real startup win. A user on the **adaptive** form still needs light/dark resolved *before* first paint or Portal paints one theme and flips, so the same race, ~50ms timeout and **dark** no-answer fallback still apply. |

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. §7.4's fidelity-versus-floors argument rests on an unsupported universal claim; the source names the three shipping Nord ports that make it checkable

**Source**: Discussion, *Fidelity versus floors — resolved*: *"Nord is a 16-slot ANSI palette; no application maps it 1:1 onto its own semantic roles, so every Nord port in the wild adapts (Ghostty, Zellij and k9s all ship one)."*

**Category**: Enhancement to existing topic
**Affects**: §7.4 (The Nord port) — the "Fidelity versus floors — resolved" paragraph

**Details**:
§7.4 ships two colours under Nord's own name that are not Nord's published values (`#DD8188`, `#A7C492`), and the sole argument that this is legitimate rather than misattribution is *"every Nord port in the wild adapts"* — currently a bare universal claim with nothing behind it. The source names three shipping counterexample-free instances (Ghostty, Zellij, k9s), which turns an assertion into something a reviewer or a future porter can verify.

This carries weight beyond tidiness because §6.4 makes the same argument load-bearing for **every** future bundled port ("porting is not free … a straight palette lift may not clear the floors unmodified"), and §7.4 explicitly refuses the carve-out route on precedent grounds — *"this being the first external palette, a carve-out granted here would set the precedent for every PR theme after it."* The evidence that adaptation is the norm is what makes refusing the carve-out defensible rather than merely strict, and it is the one piece of external support the paragraph has.

**Current** (§7.4, "Fidelity versus floors — resolved"):

> **Fidelity versus floors — resolved.** The floors win, and the corrected values ship under the palette's own name. No application maps a 16-slot ANSI palette 1:1 onto its own semantic roles; every Nord port in the wild adapts. The corrections are minimal and perceptually close, judged **visually** (both reds mocked side by side in a Nord kill modal), and `docs/theming.md` records them alongside the attribution.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
