# Review Tracking: Theming System - Gap Analysis

## Findings

### 1. Four of the six pinned flashes are reachable from the Projects page, which has no notice band or flash surface

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.6 (Opening the panel — `t`, on Sessions and Projects), §9.7 (Blocked-`t` feedback), §9.8 (Geometry — resize-forced close), §9.10 (`NO_COLOR` block), §9.13 (A failed commit write), §14A (Flashes — notice-band precedence), §14.2/§14.3 (Projects footer)

**Details**:
§9.6 binds `t` on **Projects** as well as Sessions, and §14.2 puts `t theme` in the Projects footer. Every failure and refusal path for the panel is then reachable from Projects:

- `t` under `NO_COLOR` → flash (§9.10)
- `t` below the width floor → flash (§9.8, §14A)
- `t` below the height floor → flash (§9.8, §14A)
- resize below either floor with the panel open → forced-close flash (§9.8, §14A)
- closing the panel with a failed commit outstanding → `⚠ theme not saved — see portal.log` (§9.13)

All six are pinned in §14A under the heading **"Flashes (main screen)"**, and §14A resolves their placement solely by reference to the existing notice-band arbiter: *"The band is a single-slot arbiter whose existing order is filter line → burst progress → transient flash → multi-select banner → unsupported banner → no-tags signpost."*

That arbiter is **Sessions-only**. Every contender in that list is a Sessions-page element (multi-select banner, unsupported-terminal banner, no-tags signpost, the sessions filter line), and the shipped implementation describes itself as "the Sessions-page arbiter". The Projects page has no notice band and no transient-flash slot.

So the specification requires flashes on a page that has nowhere to render them, and never says whether Projects gains a band, whether the flashes are suppressed there, or whether the panel simply behaves differently when opened from Projects. An implementer must choose one of three materially different designs:

1. Give Projects a notice band (new surface, new height-recompute, new precedence set — the existing precedence list has no Projects-relevant contenders).
2. Suppress the flashes on Projects — which silently defeats §9.10's proactive-block reasoning ("rather than letting the user walk into a dead end") and §9.13's whole argument that a failed commit must not be a silent revert, on the page where the user may well have opened the panel.
3. Refuse `t` on Projects entirely — contradicting §9.6.

§9.13's case is the sharpest: closing the panel discharges the outstanding state whether or not a flash is actually rendered, so on Projects the report would be destroyed rather than deferred — precisely the failure §9.13 exists to close, reached by a route the section does not consider.

§13.3's mandated fixtures are also all implicitly Sessions-based; no Projects-with-panel fixture is required, so the state would not be seen before release either.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §14A: Projects gains a transient-flash slot — the flash contender alone, not the full Sessions arbiter, since no other contender has a Projects analogue. Both alternatives rejected with reasons recorded: suppressing makes §9.10's proactive block a silent no-op and destroys §9.13's report outright (the close discharges the state whether or not a flash rendered), and refusing `t` contradicts §9.6. §13.3 gains a Projects-with-panel fixture.

---

### 2. The constructor's "raw persisted theme keys, exactly as read" is undefined against the `appearance` translation's in-memory constant

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.4 (Construction timing — the raw persisted keys), §10.5 (Ownership and write-path robustness), §9.5 (badge derivation table), §9.2 (slot-from-constant confirm)

**Details**:
§8.4 pins what the model receives: *"The constructor also takes the raw persisted theme keys — `theme`, `theme_light`, `theme_dark` exactly as read."* Those keys drive three things the panel cannot derive from anything else: the `●` badge placement (§9.5's three-row table), the `not found` / charset-rejected rows (§9.4), and whether `d`/`l` raises the slot-from-constant confirm (§9.2).

§10.5 specifies the `appearance` translation as **compute-then-persist, with the persist best-effort and non-blocking**: *"read `appearance`, compute the translated theme, and **use it in memory immediately**; the write is **best-effort and non-blocking**."* So on a migrating launch there are two candidate values for the theme keys, and the specification never says which one is "as read":

- **The on-disk bytes** — which, when the write fails or has not yet happened, hold *empty* theme keys.
- **The post-translation prefs value** — holding the computed `theme = tokyo-night` (or `tokyo-night-day`).

The two produce different panels for the same launch. Under the disk reading, a migrated user whose write failed sees the panel render `● light` on `tokyo-night-day` and `● dark` on `tokyo-night` (§9.5's "never set → shipped default" row) while a **constant** is actually in force and painting the screen — the badges misreport the setting on the panel's primary display. In the same state `d`/`l` would **not** raise §9.2's confirm, because the model does not believe a constant is set, so the keypress silently does exactly what the confirm exists to prevent (§9.2: *"the one place a keypress described as inert can silently cost the user a setting they chose"*).

The ambiguity is not confined to the failed-write case. Even on a successful translation, "exactly as read" invites an implementation that captures the raw keys *before* `loadPrefsStore` runs the translation — which produces the same divergence for **every** migrated user on their first post-upgrade launch, the exact population §10.1 identifies as the one that must not be surprised.

§8.9's *"once a mid-session commit supersedes the translated value, the commit is the model's active theme"* settles the **active theme** question and explicitly frames the translated value as "the starting value for a launch nobody had chosen a theme in" — but says nothing about the raw keys the panel reads.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §8.4: "as read" means the post-translation in-memory prefs value, not the on-disk bytes — which is the point of §10.5 computing and using immediately. Recorded what the disk reading would cost: a migrated user's badges claiming two shipped defaults while a constant paints the screen, and `d`/`l` not raising §9.2's confirm, to exactly the population §10.1 protects.

---

### 3. The mandated "constant set, previewing another" fixture depicts a cursor state no fixture input can produce

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.3 (Harness changes required — new fixtures, fixture-input coherence), §9.2 (opening state), §9.14 (reference frames)

**Details**:
§13.3 mandates a fixture for **"The constant-while-previewing state (a bare `●` while the cursor sits elsewhere)"**, and §9.14 identifies precisely this frame as completing the panel's specification because the two setting states never coexist on screen. §13.1 makes the harness the only route to seeing any of it before release.

But §9.2 pins the panel's opening state absolutely: *"the cursor lands on the theme that is actually rendering, and opening previews nothing"* — under a constant, that is the constant's row. A cursor sitting on a *different* row is only reachable by arrowing, i.e. after at least one keypress. Fixtures are one-shot renders (§13.4 says so explicitly: *"fixtures are one-shot renders today"*, which is why the swap-and-diff guard needs a new seam). Nothing in §13.3 gives a fixture a way to declare a cursor position or to drive a keypress, so as specified this fixture cannot be built.

Two further loose ends in the same paragraph:

- **"A fixture's three inputs"** is stated without the three ever being enumerated. Two are named in the surrounding text (`--theme`, the raw persisted theme keys); the third is left to inference — plausibly the faked `ThemeEnumerator` row set, which would leave the cursor as an unaddressed fourth.
- **The coherence rule is written only for the adaptive shape**: *"`capturetool` runs no gate, so the active slot is the **dark** slot by the standing no-answer fallback — therefore `--theme` must name the palette of the theme the dark slot declares."* Under the constant fixture there is no dark slot at all, and by §9.2's invariant (*"the cursor is always on a selectable row, and that row is always what is painted behind the panel"*) `--theme` must name the **previewed** theme, not the persisted constant. The rule as written gives no answer for the one fixture whose whole point is that those two differ, and §13.3 itself warns that an incoherent frame is *"indistinguishable from a correct one to a reviewer"*.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §13.3: a panel fixture has *four* inputs, the fourth being the cursor position — previously unstated and required, since the mandated constant-while-previewing frame is otherwise unreachable in a one-shot render. The coherence rule generalised from the adaptive case to `--theme` must name the theme *under the cursor*, which resolves to the dark slot at open for the adaptive fixture and to the previewed (not marked) theme for the constant fixture.

---

### 4. `Token.ColorFor` is removed with no replacement accessor named, at ~182 call sites and in the guard

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §3.2 (Go-side data shape), §3.4 (plumbing), §13.4 (swap-and-diff guard — SGR comparison)

**Details**:
§3.2 is otherwise exhaustive about the new Go-side shape — it pins the struct fields, the `All()` accessor, the stable order, the package home, the absence of an identity field. It states two removals without a replacement: *"`Token` becomes `{Name, Value string}`"* and *"`Token.ColorFor` is **removed**."*

`ColorFor` is today the only thing that turns a token into a renderable colour (it returns a `color.Color` via `lipgloss.Color`). After the collapse, `Token` exposes only a `string`, and every one of the ~182 call sites §3.4 counts needs a colour value. §13.4's guard needs the same conversion from the other direction — it *"converts each theme's token values to their SGR representation"*.

The specification does not say whether the replacement is a new accessor on `Token` (e.g. a `Color()` method) or an inline `lipgloss.Color(tok.Value)` at each site. It is a mechanical choice with no downstream consequence, but it is a design decision the spec leaves open at the single largest edit surface in the feature, in a section that otherwise pins the data shape precisely.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §3.2 now names the replacement: a no-argument `Token.Color()` returning a color.Color via lipgloss.Color(t.Value). An accessor rather than inline conversion — it keeps the ~182 call sites reading as they do, gives §13.4's guard one derivation point, and leaves a single seam if the value domain widens for §4.1's deferred transparent keyword.

---

### 5. §10.5 requires a combined theme-key + marker write, which none of §8.9's three named save methods performs

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §8.9 (Concurrent instances and prefs writes — the save-method API), §10.5 (the combined save)

**Details**:
§8.9 enumerates the `prefs` write API as three field-specific methods and presents that list as the whole surface (the alternative — *"exporting a whole-record type with `Load`/`Save`"* — is explicitly rejected, so callers have only what this list gives them).

§10.5 then requires a write that none of the three can perform: *"The theme key and the marker land in one write... The migration therefore uses a combined save rather than two calls."* The reasoning is load-bearing — a split write leaves a state where the theme key persists with the marker unset, and the next launch *"writes only the marker, and therefore never emits the event: the translation succeeded while the log says it failed."*

So a fourth method is required by §10.5 and absent from §8.9's enumerated API. An implementer reading §8.9 as the API surface would reach for `SaveTheme` + `SaveMigrationMarker` — exactly the two calls §10.5 forbids.

**Current**:
> **The merge itself lives inside `prefs`, behind field-specific save methods** — `SaveTheme`, `SaveThemeSlot`, `SaveMigrationMarker` — matching `SaveSessionListMode`, which already performs its own internal read-modify-write.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §8.9's API list gains `SaveTranslation` (theme key plus marker in one write), which §10.5 requires and the three field-specific methods cannot compose without leaving the window §10.5 forbids.

---

### 6. `capturetool` runs the loader but is not assigned a logger under §12.3's injected-logger rule

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §12.3 (A new `theme` log component — emission is controlled by an injected logger), §13.3 (`capturetool --theme`)

**Details**:
§12.3 makes the logger a mandatory constructor input for the loader and enumerates who passes what: *"The loader takes a logger seam; `cmd` passes a **real** component logger on the paths where a theme is used — TUI construction, the panel, the theme persister — and **`log.Discard`** on `portal doctor` and `portal theme export`."*

`capturetool` is a fifth loader caller. §13.3 has it parse a built-in slug or an explicit `.theme` path, hit every content reason class (`bad syntax`, `bad colour`, `missing tokens`, `unreadable`), and derive a candidate slug to raise `bad name` / `reserved name` warnings — i.e. it exercises exactly the outcomes the component records. It is not in either list.

The gap matters slightly more than an omission from a list, because §12.3's justification for the Discard cases is a *principle* — *"the component records where a theme is **used**, never where one is **diagnosed**"* — and `capturetool` is neither: it is an offline renderer whose output is a frame, where log emission would be noise. The rule as stated does not decide it, and §12.3 also notes the injected logger is where the per-process dedup state lives, so leaving the caller unassigned leaves that state unowned for this process too.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §12.3 assigns `capturetool` `log.Discard` — a fifth caller that neither uses nor diagnoses a theme, being an offline renderer whose output is a frame, so emission would be noise and the per-process dedup state stays owned.

---

### 7. `bad name` is defined as a filename-only class with exactly two causes, but is assigned to persisted slugs and CLI arguments

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §6.2 (The reason vocabulary), §9.4 (charset-rejected persisted slug), §12.1 (`portal theme export` — charset failure), §13.3 (`capturetool --theme` warnings), §14A (doctor detail formats)

**Details**:
§6.2 defines `bad name` narrowly and closes the definition: it is about **a filename**, and it has **two causes**, both derived from a directory entry.

Three other sections then apply the same reason to inputs that are not filenames:

- §9.4: *"A persisted slug **rejected by the charset check** (§8.6) before any file is sought gets a row with reason **`bad name`**."*
- §12.1: *"A slug failing the charset check | Refused with reason **`bad name`**"* — a CLI argument.
- §14A pins export's copy for it: `theme <slug> is not valid: bad name`.

For those inputs neither of §6.2's two causes applies (there is no extension, and no filename), and the user-facing fact §6.2 gives as the reason for collapsing the causes — *"this filename is not usable"* — is wrong: the thing that is not usable is a setting value or an argument. §14A's doctor copy reflects the split correctly (the persisted case routes to `⚠ theme <slug> (<slot>) does not resolve: <reason>`, not to the `⚠ theme file <filename>: …` line), so the intent is clear — but §6.2, which is the definitional home an implementer builds the reason enum and the table-driven loader test from (§13.6), does not admit it.

**Current**:
> | `bad name` | The filename is not a valid theme filename — **two causes**: the slug does not match `^[a-z0-9][a-z0-9-]*$` (§5.2), or the extension is not exactly lowercase `.theme` (§5.6). One reason class because the user-facing fact is the same (*this filename is not usable*) and the panel row has no width to discriminate; doctor's detail names which (§14A). |

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §6.2's `bad name` widened from a filename-only class with two causes to three causes across two input classes: directory entries (bad slug or bad extension casing) and non-file inputs (a persisted slug or CLI argument failing the same charset rule, with no extension involved). Noted that the differing line frames in §14A are what carry the input class.

---

### 8. §13.6's panel-behaviour-test row states the `ThemeEnumerator` seam is only a possibility, contradicting §13.3

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §13.6 (Guard-test reshape — Panel behaviour test), §13.3 (the `ThemeEnumerator` seam)

**Details**:
§13.3 commits to the seam in unambiguous terms: *"**The panel's theme enumeration is behind an injectable seam.** ... This is an architectural requirement, not a convenience."*

§13.6's row for the new panel behaviour test then says the seam *"currently describes a possibility rather than a commitment"*. Read against §13.3 that is simply false, and the clause reads as leftover decision-process narration from the cycle that introduced the test rather than a statement about what is being built. An implementer reading §13.6 first has reason to treat the seam as optional — which is exactly the reading §13.3 forecloses, and on which both the panel behaviour test and §13.3's mandated invalid-row fixtures depend.

**Current**:
> | **Panel behaviour test** | **New**, driven through the `ThemeEnumerator` seam (§13.3) — which is what makes it possible, and currently describes a possibility rather than a commitment. The panel carries a large body of exactly-specified, purely deterministic behaviour that nothing else covers: …

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.6's stale clause removed — the seam is an architectural commitment per §13.3, not a possibility; the wording was leftover decision-process narration from the cycle that introduced the test.

---

### 9. §2.3 says four naming kinds are in play and then describes five

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §2.3 (Naming principle — meaning and weight, never hue or place)

**Details**:
§2.3 opens by fixing the count: *"Four naming kinds are in play. Three are covered by the table below — two of them failures — and a fourth is set out beneath it."* The table covers **place** (wrong), **hue** (wrong) and **meaning** (right); the fourth set out beneath is the **pairing** name.

But the paragraph between them introduces a further kind and gives it to six of the nineteen tokens: *"the text ramp and the border want intrinsic-**weight** names because their role genuinely is 'how prominent'. The accents want **meaning** names."* Weight is a distinct kind, not a sub-case of meaning — §2.4's "Why" column uses it as such throughout (`use-site → intrinsic weight`, `ordinal makes ramp position explicit`), and the section's own title names them as two things ("meaning **and** weight").

So the stated count is four and the described count is five. The section is the vocabulary's rationale and the reference a theme author is pointed at for why a name is what it is (§12.4), so a self-inconsistent enumeration in it is worth closing.

**Current**:
> Four naming kinds are in play. Three are covered by the table below — two of them failures — and a fourth is set out beneath it:

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §2.3's count corrected from four kinds to five: place, hue and meaning in the table, plus weight and pairing beneath it. Weight was already being applied to six of the nineteen tokens and used as a distinct kind throughout §2.4's Why column. The pairing kind renumbered to fifth in both §2.3 and §2.6.

---

### 10. §7.4's "14 directly / corrects two / invents three" does not reconcile with its own value table

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §7.4 (The Nord port)

**Details**:
§7.4 summarises the port arithmetically, and the figure is used as carry-forward guidance for future ports (alongside *"every port should expect to invent at the dark end"*).

The table beneath it does not match. Counting its **Source** column: **13** values are taken from a Nord slot (`nord0`, `nord6`, `nord5`, `nord4`, `nord3` twice — for `text.faint` and `border` — `nord15`, `nord9`, `nord8`, `nord13`, `nord2`, `nord1`, `nord6` again for `text.on-attention`), **2** are corrected (`state.positive`, `state.destructive`), **3** are invented (`text.muted`, `text.subtle`, `bg.attention`), and **1** — `text.on-selection = #FFFFFF` — is sourced as *"functional maximum"*, which is neither a Nord slot nor one of the three named inventions. 13 + 1 + 2 + 3 = 19, so the table is complete and internally consistent; the summary's "14 directly" is what does not hold.

Low impact on implementation — every value is pinned in the table — but the count is quoted as a structural finding for future ports, and §7.4 is a section the specification has twice recorded as having been *"found incomplete"* with a "completeness claim plausible enough to pass unexamined".

**Current**:
> Nord is a 16-slot ANSI palette (Polar Night `nord0–3`, Snow Storm `nord4–6`, Frost `nord7–10`, Aurora `nord11–15`). Portal's 19-token vocabulary is meaningfully wider than 16 slots **at the dark end**, so the port takes 14 values directly, **corrects two**, and **invents three**.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §7.4's summary corrected to 13 taken directly, 2 corrected, 3 invented, 1 functional maximum (`text.on-selection` = #FFFFFF, a contrast choice rather than a palette claim), with the arithmetic shown. The figure is quoted as carry-forward guidance for future ports, and §7.4 is a section already twice found incomplete on a plausible-looking count.
