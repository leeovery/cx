# Review Tracking: Theming System - Input Review

Cycle 11. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. §2.3's naming principle drops the ecosystem evidence on both sides — use-site naming is the surveyed *norm*, and the pairing name has named precedent

**Source**: `discussion/theming-system.md` — "The distinction that defines the target" (lines 686–692: *"Crucially this does **not** mean everything becomes weight-based: research (A7) found use-site naming is the ecosystem **norm** (Helix names essentially its whole UI half that way)"*) and "The scheme (decided)" (line 723: `text.on-selection` — *"unchanged — contrast pairing (Crush `onPrimary` convention)"*)
**Category**: Enhancement to existing topic
**Affects**: §2.3 (Naming principle), §2.4 (row 7); cross-refs §2.6, §12.4

**Details**:

§2.3 is the stated naming principle for a **public contract** — §12.4 makes `docs/theming.md` its source of truth, and §4.6 makes every name expensive to change (a rename fails every drop-in using the old key). It is therefore the section a theme author reads to understand why the vocabulary is shaped this way, and the one place external precedent carries weight. The specification cites precedent freely everywhere else it decides something — Helix in §3.1, btop in §4.1, *"Every comparable tool lists slugs"* in §5.1, Ghostty/kitty in §7.1, Ghostty's untrusted-theme caveat in §4.4, Helix's `is_16_color()` in §6.5. §2.3 is the one naming section with none, and the source has it on **both** of the section's two open edges:

1. **Use-site naming is the ecosystem norm, and that is *why* the rule is bounded.** The source states the A7 finding at the exact point the rule is scoped — Helix names essentially its whole UI half by use-site. The specification keeps the conclusion (*"This does **not** make everything weight-based"*) and drops the finding that produced it, so the sentence now floats as an assertion with no stated cause. It also leaves the table's flat verdict — a **place** is *"Wrong"* — reading as uncontested, when the source records that the thing being called wrong is what most surveyed tools do. That matters to a theme author arriving from Helix, and it matters to whoever later argues the ramp should go fully positional: the counter-argument on record is not just "it strips meaning from ~20 files" (§2.6.1) but "the ecosystem does not do this either".

2. **The fourth naming kind is argued from first principles alone.** §2.3's *pairing* kind (added in cycle 10) reasons entirely internally — the roles are *"genuinely relational"* and §13.5 floors them as pairings — while §2.4 row 7's own source entry names an existing convention for exactly this shape: **Crush's `onPrimary`**. A deliberately-kept fourth kind that departs from the section's own stated principle is the one that most benefits from showing it is a recognised convention rather than a Portal invention.

Neither is a new decision — both are supporting evidence the source recorded for calls the specification already makes.

**Current**:

> §2.3: Four naming kinds are in play. Three are covered by the table below — two of them failures — and a fourth is set out beneath it:
>
> | Kind | Example | Verdict |
> |---|---|---|
> | A **place** | `border.footer` | Wrong — goes stale as other surfaces reuse the token. |
> | A **hue** | `accent.violet` | Wrong — lies in every port. […] |
> | A **meaning** | `state.destructive` | Right — stays true regardless of palette or where it is drawn. |
>
> This does **not** make everything weight-based. The text ramp and the border want intrinsic-**weight** names because their role genuinely is "how prominent". The accents want **meaning** names because a theme author needs to know what a colour signifies in order to choose one.
>
> **A fourth kind is deliberately kept: a *pairing* name.** `text.on-selection` and `text.on-attention` name **another token** rather than a place, a hue, a meaning or a weight — and that is correct here, because their role genuinely *is* relational […]
>
> §2.4: | 7 | `text.on-selection` | `text.on-selection` | `TextOnSelection` | unchanged — contrast pairing |

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. The Lipgloss v2 reading that looks like evidence *for* split is recorded in the source as a misreading, and the specification carries neither the reading nor the correction

**Source**: `discussion/theming-system.md` — "Theme model — split, not paired / Journey" (lines 2714–2718: *"**Charm's direction is easy to misread.** Lipgloss v2 moved `AdaptiveColor` into `compat`, but the recommended replacement (`lightDark := lipgloss.LightDark(hasDarkBG)`) **keeps paired values** and makes detection explicit. Charm de-recommended implicit detection, not paired colours — so it is not evidence for split."*)
**Category**: Enhancement to existing topic
**Affects**: §3.1 (Decisive reasons); cross-refs §3.2, §3.4, §8.8

**Details**:

Split is the largest mechanical decision in the feature — §3.4 puts it at ~182 call sites, §3.2 deletes `Token.ColorFor` and `theme.Mode`, and §13.6 records that ten existing tests do not compile after it. §3.1 defends it with four reasons, all of which are about authoring burden, MV's own values, detection/pairing independence, and the ecosystem's dominant shape.

The one external check a reader is most likely to reach for is absent: **Portal's styling library ships a paired-value mechanism and recommends it.** Lipgloss v2 moved `AdaptiveColor` into `compat`, which reads at a glance as "Charm deprecated paired colours" — i.e. as independent support for split. The source records that this reading is wrong, and does so explicitly: the recommended replacement, `lipgloss.LightDark(hasDarkBG)`, **keeps paired values** and merely makes the detection explicit. What Charm de-recommended is implicit detection, not pairing.

Two reasons this is worth carrying rather than dropping as a false path:

- **It is a standing fact about a live dependency, not a discarded option.** `compat.AdaptiveColor` and `lipgloss.LightDark` are both in the tree Portal builds against. An implementer working through §3.2's collapse of `Token` to `{Name, Value}`, or through §8.8's surviving detect-or-timeout gate, will meet them — and the natural question ("why is Portal hand-rolling the light/dark decision the library has an API for?") has no answer in the document.
- **The correction cuts the other way from the decision it sits beside.** Left unstated, a future reader re-deriving §3.1's case can pick up the misreading as a fifth supporting reason, and it is one that does not hold. The source flagged it precisely because it is easy to misread.

Note this does not reopen split. The source's own conclusion is that Charm's direction is **neutral** on the question — which is exactly the sentence the specification is missing.

**Current**:

> §3.1: Decisive reasons:
>
> - **Authoring burden under the contribution routes.** […]
> - **The pairing MV implies isn't real.** […]
> - **Detection and pairing are independent axes.** Auto-detection with single-palette themes — where detection picks between two *named themes* rather than two variants — is a shipping design (Helix's). Wanting detection does not commit Portal to paired.
> - Single-palette is the overwhelmingly dominant ecosystem shape.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. §8.3 ships detection **on** for every new user with no external support, and the source records the evidence that pointed the other way as refuted

**Source**: `discussion/theming-system.md` — "Decision — ship the adaptive pair (option b)" (lines 1015–1026: *"the orchestrator's earlier claim that 'every surveyed application ships a hardcoded default and starts rendering' came from a research paragraph research itself marked **SUPERSEDED and explicitly refuted**. The surviving claim is narrower — nobody **prompts** on first run (A4). On detection, A1 is the opposite: `bat` (`--theme` defaults `auto`), `delta` (`--detect-dark-light` defaults `auto`), Neovim (`background` auto-set by the TUI on startup and re-detected when a UI attaches) and `yazi` all detect **by default**."*), read against "The one-shot seed job — not shipped" (lines 610–613, which still carries the refuted phrasing)
**Category**: Enhancement to existing topic
**Affects**: §8.3 (The shipped default is the adaptive pair); cross-ref §8.7

**Details**:

§8.3 is the decision every brand-new install lands on: Portal queries the terminal and picks a theme, rather than shipping a fixed one. It is defended by three reasons, all internal to Portal — the timeout is not a price, it degrades to the alternative, the escape is asymmetric. The source has a fourth that is external and that the specification does not carry: **four surveyed tools detect by default** (`bat`, `delta`, Neovim, `yazi`), which is the ecosystem answer to "should a terminal tool detect out of the box".

What makes the omission worth closing rather than shrugging at is the *shape* of the source record. The claim that appears to argue against detection-by-default — *"every surveyed application ships a hardcoded default and starts rendering"* — is recorded in the source as **refuted**, from a research paragraph research itself marked SUPERSEDED. Only the narrower A4 claim survives: nobody *prompts* on first run.

Three consequences:

- The specification correctly avoided importing the refuted claim (§8.7's seed paragraph rests only on *"There is no unconfigured case left to seed"*), but it also dropped the correction — so a future reader who re-derives the question from the same research material can pick the refuted version straight back up, and it argues against the decision §8.3 makes.
- The A4 half that *did* survive is the standing precedent for **not** seeding and **not** prompting, which is §8.7's own position; it is currently unsupported there too.
- §8.3 names a real risk immediately below the reasons (*"a terminal that answers OSC 11 inconsistently makes Portal flip between launches"*). A decision that ships a named risk on every install is the one that most wants the record that the ecosystem judged the same trade the same way.

**Current**:

> §8.3: Reasons over shipping a constant dark default:
>
> - **The 50ms is a timeout, not a price** — terminals that answer do so in single-digit ms — and it applies only to TUI launches, since `portal open <target>` execs without painting.
> - **It degrades to the alternative**: no answer resolves to dark, so the adaptive pair is a superset of a constant dark default with a bounded downside.
> - **Asymmetric escape.** Pinning is one line and is the *simpler* config (`"theme": "tokyo-night"`), so an annoyed user has an obvious remedy. […]

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. §8.3's "degrades to the alternative" argument depends on §8.5's fallback values, and the source records that dependency where the specification states only the property

**Source**: `discussion/theming-system.md` — "Which built-in is 'the default built-in' (the review's F2 / F15)" (lines 2286–2289: *"It also disposes of the review's F15: because the shipped adaptive default and the fallback default are then **the same values**, the earlier argument that shipping the pair 'degrades to shipping a constant dark' stays true rather than quietly resting on two different notions of 'default'."*)
**Category**: Enhancement to existing topic
**Affects**: §8.5 (Fallback — per-slot and mode-matched); cross-ref §8.3

**Details**:

The source states this as a *dependency between two decisions*, discovered by a review: §8.3's second reason — the adaptive pair *"degrades to the alternative … a superset of a constant dark default"* — is only true because the per-slot mode-matched fallback resolves to the same values the shipped default nominates. Before the fallback was pinned, the argument was resting on two different notions of "default" and nobody had noticed.

The specification carries the *property* (§8.5: *"it makes the shipped adaptive default and the fallback default **the same values**"*) but not what the property is load-bearing for. Two things follow from leaving the link undrawn:

- **§8.3's argument becomes uncheckable in place.** A reader assessing "does the adaptive pair really degrade to a constant dark default?" has to independently notice that §8.5 exists and that its values coincide. The source's own record is that this is exactly the check that was missed once.
- **The rejected alternative in §8.5 is the thing that would break it.** §8.5 explicitly rejects *"a single fixed fallback regardless of mode"* on a different ground (a light-terminal user thrown to dark). That alternative — or any later change to the fallback values — would silently invalidate §8.3's second reason, and nothing in either section says so. This is the same class of cross-decision link §7.7 flags for the built-in set (*"The built-in-set decision is conditional on this check"*) and §8.4 flags for no-shadowing.

One sentence in §8.5 closes it, and it is the sentence the source wrote.

**Current**:

> §8.5: This introduces **no new mechanism** — it is the already-decided "an unset slot holds the shipped default" rule applied to a slot that is *set but unloadable* rather than unset. One rule covers both cases, and it makes the shipped adaptive default and the fallback default **the same values**.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:
