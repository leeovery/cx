# Review Tracking: Theming System - Integrity

## Findings

### 1. A constant → adaptive conversion writes the gate's answer without pinning that `startupCanvasHex` must not move

**Severity**: Important
**Plan Reference**: Phase 9, task `theming-system-9-6` ("The newly-live opposite slot loads at commit with `theme: loaded`")
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
Task 9-6's amended "answer half" makes the conversion **write the model's light/dark answer** — the same value the appearance gate sets, and the value task 8-8 and task 8-10 read to select the in-force member. Task 3-3 established that `startupCanvasHex` is captured *at the one place the active member is selected*: "in `syncResolvedMode` (adaptive) and on the constant/colourless construction path". So the existing production entry point for "the answer resolved, select the member" is precisely the entry point that also re-captures the anchor.

An implementer reaching for that entry point — the obvious route, since it already does exactly what 9-6 describes — would silently move `startupCanvasHex` on every conversion. That breaks §11.4's guarantee on the one path the specification singles out as the one where a mistake re-sticks a colour in the user's terminal **after Portal exits**, and it is not covered anywhere:

- Task 4-5's two divergence cases are both driven through `ApplyTheme` (a commit mid-session, an abandoned preview); a conversion is not a swap and inherits none of those assertions.
- Tasks 4-2, 4-5 and 8-9 each assert "`startupCanvasHex` must not move **on a swap**" — 9-6's write is not a swap.
- Task 9-6's own Do, Acceptance Criteria, Tests and Edge Cases never mention `startupCanvasHex` at all.

The plan guards this anchor at four separate sites and leaves the one task that writes the field it is derived from unguarded. Adding the prohibition and its assertion closes the gap without changing any decision.

**Current**:

Task `theming-system-9-6`, **Do** — the answer-half bullet:

- **Resolve the answer half too, from the reply already in hand.** A conversion makes light/dark matter for a user whose launch deliberately never consulted detection (§8.2), so the in-force light/dark answer must be **established here** — never read off the constant path's pre-resolved gate, whose value is the standing dark fallback rather than a classification of the terminal (task 3-2). Classify the OSC 11 background retained at launch and record it as the model's light/dark answer, so task 8-10's close and task 8-8's next open select the in-force member correctly with no change of their own. **No new query, no new race, no new gate** — §9.3's transition dissolves precisely because the answer already arrived, and the single-resolution rule (§8.8) is untouched: a reply arriving after this point still never re-themes. If **no reply has landed** — which requires the panel to have been opened within milliseconds of launch — it falls to **dark**, the same rule as everywhere else. A user who launched on an adaptive pair already has a classified answer, so a conversion never arises for them and nothing is re-derived.

**Proposed**:

Task `theming-system-9-6`, **Do** — replace the answer-half bullet with:

- **Resolve the answer half too, from the reply already in hand.** A conversion makes light/dark matter for a user whose launch deliberately never consulted detection (§8.2), so the in-force light/dark answer must be **established here** — never read off the constant path's pre-resolved gate, whose value is the standing dark fallback rather than a classification of the terminal (task 3-2). Classify the OSC 11 background retained at launch and record it as the model's light/dark answer, so task 8-10's close and task 8-8's next open select the in-force member correctly with no change of their own. **No new query, no new race, no new gate** — §9.3's transition dissolves precisely because the answer already arrived, and the single-resolution rule (§8.8) is untouched: a reply arriving after this point still never re-themes. If **no reply has landed** — which requires the panel to have been opened within milliseconds of launch — it falls to **dark**, the same rule as everywhere else. A user who launched on an adaptive pair already has a classified answer, so a conversion never arises for them and nothing is re-derived. **Record the answer directly and never route through `syncResolvedMode`**: that is where task 3-3 captures `startupCanvasHex`, so reusing it would re-anchor §11.4's echo guard to a canvas the startup window never painted. The anchor is captured once at gate resolution and must survive every conversion — this is the one path where a mistake re-sticks a colour in the user's terminal after Portal exits, and tasks 4-2 / 4-5 / 8-9 guard it only against *swaps*, which a conversion is not.

Task `theming-system-9-6`, **Acceptance Criteria** — add after "The conversion issues **no** new OSC 11 query and arms no new gate — the answer is classified from the background captured at launch.":

- [ ] A conversion leaves `startupCanvasHex` **byte-identical** — before and after a confirmed `d`/`l` on a constant, on a light terminal and a dark one — so task 3-3's anchor and task 4-5's divergence guarantee both still hold.
- [ ] The classification does not route through `syncResolvedMode` (asserted structurally, since that is the function that captures the anchor).

Task `theming-system-9-6`, **Tests** — add after `"it falls back to dark when no reply landed"`:

- `"it never moves the startup canvas hex on a conversion"` — `TestCommitSlotLoad_ConversionDoesNotMoveStartupCanvasHex` (light terminal, dark terminal, no-reply)

Task `theming-system-9-6`, **Edge Cases** — add after "**No new query, no race, no gate**, and **dark** when no reply has landed — the same rule as everywhere else.":

- The conversion writes the model's light/dark answer, which is the **same value the gate's resolution sets** — so it must record the answer directly and never route through `syncResolvedMode`, where task 3-3 captures `startupCanvasHex`. Re-capturing the hex would re-anchor §11.4's echo guard to a canvas the startup window never painted, on the one path where a mistake re-sticks a colour in the user's terminal after Portal exits. Tasks 4-2, 4-5 and 8-9 assert the anchor is stable across a **swap**; a conversion is not a swap and inherits none of those assertions, which is why this task needs its own.

**Resolution**: Fixed
**Notes**:

---

### 2. `Resolve`'s error return has no stated policy at any of its three panel call sites

**Severity**: Minor
**Plan Reference**: Phase 8, tasks `theming-system-8-8` and `theming-system-8-10` (consumed also by Phase 9 task `theming-system-9-2`)
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
Task 8-8 declares `Resolve(e theme.Enumeration, s theme.Setting) (theme.Resolution, error)` and states that a failing *fallback* returns task 5-6's fatal (`built-in theme <slug> is missing or invalid — this binary is broken`). Three tasks call it, and none of the three agree about the error:

- **8-8 (open)**: "call `Resolve`, and use the returned `Resolution` for three things" — no error branch at all.
- **8-10 (close)**: "Call the seam's `Resolve(p.enumeration, setting)`" — no error branch at all.
- **9-2 (post-commit recompute)**: `res, _ := m.themeEnumerator.Resolve(...)` — explicitly discarded.

Left as written, an implementer at 8-8 must invent a policy, and the three sites can diverge: one could surface the fatal up through `Update` (so pressing `t` on a broken binary quits Portal mid-session), one could discard, one could panic on a nil `Resolution`. The path is genuinely unreachable in a correctly built binary — Phase 2 task 2-8's build-time guarantee is what makes it so — which is why the fix is to *state* the degrade policy once rather than to design a failure surface for it. Task 9-2's `_` already matches the proposed policy and needs no edit.

**Current**:

Task `theming-system-8-8`, **Do** — the seam-extension bullet:

- **Extend the seam** (task 8-1) with `Resolve(e theme.Enumeration, s theme.Setting) (theme.Resolution, error)` and wire the production adapter and the fixture fake to it.

Task `theming-system-8-10`, **Do** — step 2 of `closeThemePanel`:

  2. Call the seam's `Resolve(p.enumeration, setting)` (task 8-8's `ResolveNominationFrom`) — resolution runs **against the retained enumeration**, never the filesystem, so it agrees with the rows the user was just looking at and issues no third parse.

**Proposed**:

Task `theming-system-8-8`, **Do** — replace the seam-extension bullet with these two:

- **Extend the seam** (task 8-1) with `Resolve(e theme.Enumeration, s theme.Setting) (theme.Resolution, error)` and wire the production adapter and the fixture fake to it.
- **Pin the error policy once, here.** `Resolve` returns task 5-6's fatal only when a *fallback* cannot resolve within the embedded set, which Phase 2 task 2-8's build-time guarantee makes unreachable in a correctly built binary. The panel therefore **degrades rather than escalating**: on a non-nil error leave the badges, the active theme and the cursor exactly as they were, carry on with the union already in hand, and write nothing — a settings surface must not become the route by which a broken binary quits Portal mid-session, and §7.6 puts the fatal on the *startup* path deliberately. State in-source that this one policy governs **every** panel call site of `Resolve` — this task's open, task 8-10's close and task 9-2's recompute — so the three cannot each invent their own.

Task `theming-system-8-8`, **Acceptance Criteria** — add after "With an identity absent from the union the cursor clamps to the first selectable row with no panic and no out-of-range index.":

- [ ] A `Resolve` returning task 5-6's fatal leaves the badges, the active theme and the cursor unchanged, still opens the panel on the union already in hand, writes nothing and does not quit Portal — driven through the seam with an error-returning fake.

Task `theming-system-8-8`, **Tests** — add after `"it degrades rather than indexing out of range"`:

- `"it degrades rather than escalating an unresolvable fallback"` — `TestPanelOpen_ResolveErrorDegrades`

Task `theming-system-8-8`, **Edge Cases** — add after "Built-ins are always valid so a fully-unselectable list is unreachable, but the anchor must degrade rather than index out of range.":

- `Resolve` can return task 5-6's fatal, which the build-time guarantee makes unreachable in a correctly built binary — the panel **degrades** (badges, active theme and cursor untouched, nothing written) rather than escalating, because a settings surface must not be the route by which a broken binary quits mid-session. One policy governs all three panel call sites: this task's open, task 8-10's close and task 9-2's recompute.

Task `theming-system-8-10`, **Do** — replace step 2 with:

  2. Call the seam's `Resolve(p.enumeration, setting)` (task 8-8's `ResolveNominationFrom`) — resolution runs **against the retained enumeration**, never the filesystem, so it agrees with the rows the user was just looking at and issues no third parse. A non-nil error takes task 8-8's degrade policy: skip steps 3 and 4, leaving the active theme exactly as it is, and fall through to step 5 so the panel still closes.

**Resolution**: Fixed
**Notes**:

---

### 3. Doctor's file-line producers are never told to populate the advisory identity fields the union dedups on

**Severity**: Minor
**Plan Reference**: Phase 7, tasks `theming-system-7-3` and `theming-system-7-4`
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
Task 7-2 declares `advisory{line, slug, fromPrefs}` and documents that "`slug` and `fromPrefs` are the identity task 7-6's one-slug-one-line union dedups on". Task 7-5 explicitly instructs its producer to "Mark each of these advisories `fromPrefs: true` and carry its `slug`". Tasks 7-3 and 7-4 — the two *file*-line producers — describe only the `line` each frame renders and never mention the identity fields.

Task 7-6 then drops "every file line whose `slug` is non-empty and a member of" the persisted set. If 7-3 lands with `line` alone, the dedup silently does nothing and `<M>` counts detections rather than problems — the exact property 7-6 exists to protect — and the failure is invisible until 7-6 is written and its tests fail. 7-4's case is the mirror: its `bad name` rows must carry an **empty** slug, which is what makes 7-6's "a `bad name` file can never collide" claim structural rather than incidental.

Both are one-line additions to instructions that are otherwise exhaustive about what each producer emits.

**Current**:

Task `theming-system-7-3`, **Do** — the generic-frame bullet:

- For each entry with a non-nil `Rejection` whose reason is one of the four this task owns — `missing tokens`, `bad colour`, `bad syntax`, `unreadable` — emit the generic frame `⚠ theme <slug>: <reason> — <detail>`, where `<slug>` is `Entry.Slug` and `<reason>` is the `Reason`'s string value (the terse §6.2 label verbatim). The filename reasons (`bad name`, `reserved name`) are task 7-4 and are skipped here — leave an explicit `default:` arm so the compiler-level exhaustiveness is visible rather than silently dropping them.

Task `theming-system-7-4`, **Do** — the frames bullet:

- Extend `cmd/doctor_theme.go`'s per-entry mapping with the two filename reasons, using `Entry.Filename` (the base name as enumerated, never the full path — §14A's placeholder is `<filename>`):
  - `bad name` with `BadNameCause == BadNameSlug` → `⚠ theme file <filename>: slug must be lowercase letters, digits and hyphens`
  - `bad name` with `BadNameCause == BadNameExtension` → `⚠ theme file <filename>: extension must be lowercase .theme`
  - `reserved name` → `⚠ theme file <filename>: <slug> is a built-in — rename it (e.g. <slug>-mine.theme)`, where `<slug>` is `Entry.Slug` (a `reserved name` entry has a valid slug — that is what collided).

**Proposed**:

Task `theming-system-7-3`, **Do** — replace the generic-frame bullet with:

- For each entry with a non-nil `Rejection` whose reason is one of the four this task owns — `missing tokens`, `bad colour`, `bad syntax`, `unreadable` — emit the generic frame `⚠ theme <slug>: <reason> — <detail>`, where `<slug>` is `Entry.Slug` and `<reason>` is the `Reason`'s string value (the terse §6.2 label verbatim). The filename reasons (`bad name`, `reserved name`) are task 7-4 and are skipped here — leave an explicit `default:` arm so the compiler-level exhaustiveness is visible rather than silently dropping them. **Populate the advisory's identity fields as well as its line** — `slug: Entry.Slug`, `fromPrefs: false` — because task 7-6's one-slug-one-line union drops a file line only when its `slug` is non-empty and matches a persisted line's. A producer that sets `line` alone silently defeats the dedup and makes `<M>` count detections rather than problems.

Task `theming-system-7-3`, **Acceptance Criteria** — add after "A valid `.theme` file produces no line; a non-`.theme` file produces no line.":

- [ ] Every advisory this task emits carries `slug == Entry.Slug` and `fromPrefs == false`, so task 7-6's union has an identity to dedup on.

Task `theming-system-7-3`, **Tests** — add after `"it reports exactly one reason per file"`:

- `"it carries the slug identity for the union"` — `TestThemeAdvisories_FileLinesCarryTheirSlug`

Task `theming-system-7-3`, **Edge Cases** — add after "Doctor enumerates **within** the reason and never across, so a file is never reported as both `bad colour` and `missing tokens`.":

- Each advisory carries `slug` and `fromPrefs: false` alongside its line — task 7-6's dedup keys on exactly those, so a producer emitting the line alone silently disables the one-slug-one-line rule.

Task `theming-system-7-4`, **Do** — replace the frames bullet with:

- Extend `cmd/doctor_theme.go`'s per-entry mapping with the two filename reasons, using `Entry.Filename` (the base name as enumerated, never the full path — §14A's placeholder is `<filename>`):
  - `bad name` with `BadNameCause == BadNameSlug` → `⚠ theme file <filename>: slug must be lowercase letters, digits and hyphens`
  - `bad name` with `BadNameCause == BadNameExtension` → `⚠ theme file <filename>: extension must be lowercase .theme`
  - `reserved name` → `⚠ theme file <filename>: <slug> is a built-in — rename it (e.g. <slug>-mine.theme)`, where `<slug>` is `Entry.Slug` (a `reserved name` entry has a valid slug — that is what collided).

  Set the identity fields alongside each line: `fromPrefs: false` in both cases; `slug` **empty** for a `bad name` row (`Entry.Slug` is empty exactly when the rejection is `bad name`, which is what makes task 7-6's "a `bad name` file can never collide with a persisted slug" structural rather than incidental) and `Entry.Slug` for a `reserved name` row.

Task `theming-system-7-4`, **Acceptance Criteria** — add after "A `reserved name` file whose contents are perfectly valid still produces its line — the reason is decided from the slug alone, before any read.":

- [ ] A `bad name` advisory carries an **empty** `slug`; a `reserved name` advisory carries `Entry.Slug`; both carry `fromPrefs == false`.

Task `theming-system-7-4`, **Tests** — add after `"it never reports a bad-name file's contents"`:

- `"it leaves a bad-name advisory without a slug"` — `TestThemeAdvisories_BadNameCarriesNoSlug`

Task `theming-system-7-4`, **Edge Cases** — add after "A `bad name` file can never also report `unreadable` or any content reason, the filename being decided before the file is opened.":

- A `bad name` advisory carries an **empty** `slug` and a `reserved name` advisory carries its `Entry.Slug`; both carry `fromPrefs: false`. That is what makes task 7-6's non-collision rule structural rather than incidental.

**Resolution**: Fixed
**Notes**:

---

### 4. The `theme` component logger's binding site in `cmd` is unassigned until task 6-6 resolves it conditionally

**Severity**: Minor
**Plan Reference**: Phase 3 task `theming-system-3-2`, Phase 5 task `theming-system-5-7`, Phase 6 task `theming-system-6-6`, Phase 8 task `theming-system-8-7`
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
CLAUDE.md's rule is explicit: "bind a component logger once per package via `var logger = log.For("<component>")`". Four `cmd`-side tasks need the `theme` logger and none of the first three establishes the package binding:

- **3-2**: "Construct the loader with `theme.NewEventLogger(log.For("theme"))`" — inline call.
- **5-7**: "`loader` is constructed with `theme.NewEventLogger(log.For("theme"))`" — a second inline call.
- **6-6**: "Bind the component logger once for the package **if Phase 5 task 5-7 has not already**: `var themeLogger = log.For("theme")`" — a conditional instruction that asks the implementer to guess at the state 5-7 left behind (5-7 does not bind a package var, so the condition is always true, but nothing says so).
- **8-7**: "with the **real** `log.For("theme")` component logger" — a third inline call.

Tasks 10-6 and 10-9 both go on to *document* bind-once-per-package as a rule this feature honours, so leaving `cmd` with two or three `log.For("theme")` call sites contradicts the plan's own closing documentation. Assigning the binding to 3-2 — the first `cmd`-side use — and having the other three reuse it removes the conditional and the drift in four one-line edits.

**Current**:

Task `theming-system-3-2`, **Do** — the legacy-mapping bullet (relevant clause):

- **Map the legacy `appearance` pref in `cmd/open.go`**, in memory only, where `prefsStore.LoadAppearance()` is read today: `auto` → `AdaptivePair(LoadBuiltin(theme.DefaultLightSlug), LoadBuiltin(theme.DefaultDarkSlug))`; `light` → `ConstantNomination(LoadBuiltin(theme.DefaultLightSlug))`; `dark` → `ConstantNomination(LoadBuiltin(theme.DefaultDarkSlug))`. Construct the loader with `theme.NewEventLogger(log.For("theme"))` — the §12.3 assignment for a path where a theme is *used* (no event is defined for this path until Phase 5's `theme: loaded`). **Nothing is written to `prefs.json`** and `prefs.Appearance` / `LoadAppearance` are left in place; Phase 6 owns the persisted translation.

Task `theming-system-5-7`, **Do** — the derive-and-resolve bullet:

- Derive and resolve: `setting, rawKeys := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)`, then `res, err := loader.ResolveNomination(setting, themesDir)` where `loader` is constructed with `theme.NewEventLogger(log.For("theme"))` — the §12.3 assignment for a path where a theme is *used* — and `themesDir` comes from `themesDirPath()`.

Task `theming-system-6-6`, **Do** — the binding bullet:

- Bind the component logger once for the package if Phase 5 task 5-7 has not already: `var themeLogger = log.For("theme")` at `cmd` package scope, reused by this task and task 6-7. CLAUDE.md's rule is bind once *per package*, and §8.9 explicitly legitimises the `theme` component being emitted from three packages (the loader, this translation, the persister).

Task `theming-system-8-7`, **Do** — the wire-production bullet (relevant clause):

- **Wire production** in `cmd/open.go`: a small adapter closing over the same `theme.Loader` Phase 5 task 5-7 already constructs (with the **real** `log.For("theme")` component logger — the panel is a path where a theme is *used*) and over `themesDirPath()`, mirroring the `ScrollbackReader` adapter that closes over `stateDir` at TUI construction. Pass the `RawKeys` and `[]SlotResolution` the construction-time resolution already produced — replacing task 5-7's `_`-plus-comment placeholder. Doctor, `portal theme export` and `capturetool` keep `log.Discard`.

**Proposed**:

Task `theming-system-3-2`, **Do** — replace the legacy-mapping bullet with:

- **Map the legacy `appearance` pref in `cmd/open.go`**, in memory only, where `prefsStore.LoadAppearance()` is read today: `auto` → `AdaptivePair(LoadBuiltin(theme.DefaultLightSlug), LoadBuiltin(theme.DefaultDarkSlug))`; `light` → `ConstantNomination(LoadBuiltin(theme.DefaultLightSlug))`; `dark` → `ConstantNomination(LoadBuiltin(theme.DefaultDarkSlug))`. **Bind the `theme` component once for the `cmd` package here** — `var themeLogger = log.For("theme")` at package scope, per CLAUDE.md's bind-once-*per-package* rule — and construct the loader with `theme.NewEventLogger(themeLogger)`; this is the §12.3 assignment for a path where a theme is *used* (no event is defined for this path until Phase 5's `theme: loaded`), and tasks 5-7, 6-6, 6-7 and 8-7 all reuse this var rather than calling `log.For("theme")` again. **Nothing is written to `prefs.json`** and `prefs.Appearance` / `LoadAppearance` are left in place; Phase 6 owns the persisted translation.

Task `theming-system-3-2`, **Acceptance Criteria** — add after "`prefs.Appearance` and `prefsStore.LoadAppearance()` still exist and still decode tolerantly — their deletion is Phase 5–6.":

- [ ] `cmd` binds the `theme` component exactly once — a single package-level `themeLogger` — and no other `log.For("theme")` call exists in the package.

Task `theming-system-5-7`, **Do** — replace the derive-and-resolve bullet with:

- Derive and resolve: `setting, rawKeys := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)`, then `res, err := loader.ResolveNomination(setting, themesDir)` where `loader` is constructed with `theme.NewEventLogger(themeLogger)` — task 3-2's package-level `cmd` binding, never a second `log.For("theme")` call — and `themesDir` comes from `themesDirPath()`.

Task `theming-system-6-6`, **Do** — replace the binding bullet with:

- Reuse `cmd`'s package-level `themeLogger`, bound by Phase 3 task 3-2 and shared by this task and task 6-7. Do **not** add a second `log.For("theme")` call: CLAUDE.md's rule is bind once *per package*, and §8.9 explicitly legitimises the `theme` component being emitted from three packages (the loader, this translation, the persister) — which is a per-package rule, not a per-call-site licence.

Task `theming-system-8-7`, **Do** — replace the wire-production bullet with:

- **Wire production** in `cmd/open.go`: a small adapter closing over the same `theme.Loader` Phase 5 task 5-7 already constructs (built on `cmd`'s package-level `themeLogger` from task 3-2 — the **real** component logger, because the panel is a path where a theme is *used*) and over `themesDirPath()`, mirroring the `ScrollbackReader` adapter that closes over `stateDir` at TUI construction. Pass the `RawKeys` and `[]SlotResolution` the construction-time resolution already produced — replacing task 5-7's `_`-plus-comment placeholder. Doctor, `portal theme export` and `capturetool` keep `log.Discard`.

**Resolution**: Fixed
**Notes**:

---

### 5. The panel row delegate's construction point is used by task 8-9 but declared by no task

**Severity**: Minor
**Plan Reference**: Phase 8, task `theming-system-8-6` ("The slide-over surface — overlay, chrome and the pinned directory row")
**Category**: Task Self-Containment
**Change Type**: add-to-task

**Details**:
Task 8-4 declares `themeRowDelegate{Theme, Colourless, Width}` and its `Render` method. Task 8-6 builds the panel's `bubbles/list` instance and sizes it, but never says where the delegate is constructed or where its three inputs come from. Task 8-9 then writes `p.list.SetDelegate(m.themeRowDelegate())` — a named helper that no task's **Do** creates.

An implementer landing 8-6 would most naturally build the delegate inline in the render path; 8-9 would then need a model method that does not exist, and the two sites could disagree about the inner content width (which task 8-11's ladder makes variable) or about `Colourless`. A width disagreement between construction and restyle is invisible until a resize happens during a live preview — precisely the surface §11.2 calls the worst case of the cached-style class.

One `Do` bullet in 8-6 naming the helper and its three inputs closes it, and 8-9's existing reference then resolves.

**Current**:

Task `theming-system-8-6`, **Do** — the layout bullet:

- **Lay out the panel** as: header (2) + directory row (0 or 1) + list body (the remainder, ≥ 1) + message slot (0, always, this phase) + footer (task 8-5's measured height). Size the panel's `list.Model` to `(innerWidth, bodyRows)` through the same `SetSize` discipline the other two lists use, and construct it with `SetFilteringEnabled(false)`, `SetShowTitle(false)`, `SetShowStatusBar(false)` and `SetShowHelp(false)` — the panel's own chrome supplies all of that. Declare `themePanelPreferredWidth = 30` and `themePanelMinWidth = 24` here as the two ends of the ladder; **task 8-11 owns choosing between them and the refusal below the minimum** — this task renders at whatever width it is handed.

**Proposed**:

Task `theming-system-8-6`, **Do** — replace the layout bullet with these two:

- **Lay out the panel** as: header (2) + directory row (0 or 1) + list body (the remainder, ≥ 1) + message slot (0, always, this phase) + footer (task 8-5's measured height). Size the panel's `list.Model` to `(innerWidth, bodyRows)` through the same `SetSize` discipline the other two lists use, and construct it with `SetFilteringEnabled(false)`, `SetShowTitle(false)`, `SetShowStatusBar(false)` and `SetShowHelp(false)` — the panel's own chrome supplies all of that. Declare `themePanelPreferredWidth = 30` and `themePanelMinWidth = 24` here as the two ends of the ladder; **task 8-11 owns choosing between them and the refusal below the minimum** — this task renders at whatever width it is handed.
- **Declare the delegate's single construction point.** Add `func (m Model) themeRowDelegate() themeRowDelegate` returning task 8-4's delegate built from the **previewed** theme (`m.activeTheme`), `m.colourless`, and the panel's current inner content width — and build the list's delegate only through it, here at construction and (task 8-9) on every restyle. There must be exactly one place the three inputs are assembled: two construction sites can disagree about width or colourlessness, and that disagreement is invisible until a resize during a live preview, on the surface §11.2 calls the worst case of the cached-style class.

Task `theming-system-8-6`, **Acceptance Criteria** — add after "The panel's list is constructed with filtering, title, status bar and help all disabled, and `SetSize` is fed the inner width and the computed body height.":

- [ ] The panel's list delegate is built only through `m.themeRowDelegate()`, which takes its `Theme`, `Colourless` and `Width` from the model and the panel's current inner width — no second construction site exists.

Task `theming-system-8-6`, **Tests** — add after `"it uses only existing tokens"`:

- `"it builds its delegate from one place"` — `TestThemePanel_DelegateHasASingleConstructionPoint`

Task `theming-system-8-6`, **Edge Cases** — add after "The panel introduces a **third `bubbles/list` instance**, §11.2's worst case of the cached-style class.":

- The delegate's three inputs (previewed `Theme`, `Colourless`, inner content width) are assembled in exactly one helper — `m.themeRowDelegate()` — which task 8-9's restyle path re-invokes. Two construction sites can disagree about width or colourlessness, and the disagreement is invisible until a resize during a preview.

**Resolution**: Fixed
**Notes**:
