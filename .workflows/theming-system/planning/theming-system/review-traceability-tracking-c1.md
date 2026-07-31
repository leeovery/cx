# Review Tracking: Theming System - Traceability

## Findings

### 1. §9.3's OSC 11 query is not pinned as unconditional, so a constant launch may never capture a reply to convert with

**Type**: Missing from plan
**Spec Reference**: §8.8 ("**The query is issued from `Init` regardless of the setting shape.** That is what makes a mid-session constant → adaptive conversion work without a new query, race or gate."), §9.3, §8.7
**Plan Reference**: Phase 3, task `theming-system-3-2` ("`tui.Build` takes the loaded nomination and the gate selects its member")
**Change Type**: add-to-task

**Details**:
§8.8 pins the OSC 11 *query* as unconditional — issued from `Init` under every setting shape — and names the reason explicitly: it is what makes §9.3's mid-session constant → adaptive conversion work with no new query, race or gate. Task 3-2 owns the gate's construction from the nomination's shape and is the only task that touches this machinery, but it pins the query only indirectly ("the reply must still be **consumed**") while its acceptance criteria pin the *constant* path as "the gate is resolved and unarmable at construction, `Init` issues **no** timeout tick" with nothing said about the query.

That leaves the most natural reading of the task — a constant needs no detection, so construct a pre-resolved gate and skip the detection path entirely — able to satisfy every acceptance criterion while silently dropping the query. Two decided behaviours break if it does: `restore.go`'s original-background capture (§8.8: "Survives unchanged — `restore.go` needs it to capture the original background for restore-on-exit, **independent of detection**"), and §9.3's whole dissolution argument, which rests on the answer having already arrived.

The task also needs to say what the constant path's pre-resolved gate *means*, because finding 2's fix depends on it: its resolved value is the standing dark fallback, not a classification of the terminal, and nothing may read it as one.

**Current**:

From task `theming-system-3-2`, **Do** (third bullet):

- **Leave the single-resolution machinery untouched.** `resolveFromDark` / `resolveDark` already no-op once resolved, and `Update`'s `tea.BackgroundColorMsg` arm already stores `originalBg` *unconditionally* while calling `syncResolvedMode` only when resolution actually happened. Do not change that ordering: the reply must still be **consumed** for `restore.go`'s original-background capture (and for §9.3's mid-session conversion) while never re-theming.

From task `theming-system-3-2`, **Acceptance Criteria** (second bullet):

- [ ] A constant nomination: the gate is resolved and unarmable at construction, `Init` issues **no** timeout tick, and the first frame paints the constant's canvas — no detection wait.

From task `theming-system-3-2`, **Tests** (fourth entry):

- `"it consumes a late reply without re-theming"` — `TestGate_LateReplyCapturesBackgroundButNeverReThemes`

From task `theming-system-3-2`, **Edge Cases** (third bullet):

- The gate resolves **exactly once**: a late OSC 11 reply is still consumed for `restore.go`'s original-background capture but never re-themes, because under split a late flip swaps a whole named theme rather than a variant.

**Proposed**:

Task `theming-system-3-2`, **Do** — replace the third bullet with:

- **Leave the single-resolution machinery untouched, and keep the query unconditional.** `resolveFromDark` / `resolveDark` already no-op once resolved, and `Update`'s `tea.BackgroundColorMsg` arm already stores `originalBg` *unconditionally* while calling `syncResolvedMode` only when resolution actually happened. Do not change that ordering: the reply must still be **consumed** for `restore.go`'s original-background capture (and for §9.3's mid-session conversion) while never re-theming. **The OSC 11 query is issued from `Init` regardless of the setting shape** — a constant nomination skips the *gate*, never the *query*. `restore.go` needs the reply independent of detection, and §9.3's conversion needs it already in hand, so a constant path that skips the query breaks both. Retain the reply in a form a later consumer can classify (the captured background plus whether one ever arrived), and comment that a constant's pre-resolved gate carries **no** detection-derived light/dark answer — its resolved value is the standing dark fallback, and Phase 9 task 9-6 is what classifies the retained reply when a conversion first needs an answer.

Task `theming-system-3-2`, **Acceptance Criteria** — replace the second bullet with these three:

- [ ] A constant nomination: the gate is resolved and unarmable at construction, `Init` issues **no** timeout tick, and the first frame paints the constant's canvas — no detection wait; **the OSC 11 query is still issued** and its reply still captured.
- [ ] `Init` issues the OSC 11 query under **every** setting shape — constant, adaptive pair and `NO_COLOR` — and a reply arriving at any time populates `originalBg`.
- [ ] After a constant launch on a light terminal the model holds the captured background and the fact that a reply arrived, with **no** light/dark answer derived from it — the constant's resolved gate value is the standing dark fallback and is not a classification of the terminal.

Task `theming-system-3-2`, **Tests** — add after the fourth entry:

- `"it issues the query under every setting shape"` — `TestGate_QueryIssuedRegardlessOfSettingShape` (constant, pair, `NO_COLOR`)
- `"it retains a constant launch's reply without classifying it"` — `TestGate_ConstantRetainsReplyWithoutClassifying`

Task `theming-system-3-2`, **Edge Cases** — add after the third bullet:

- **The query is issued from `Init` regardless of the setting shape** — a constant skips the gate, not the query. `restore.go` needs the reply for its original-background capture independent of detection, and §9.3's mid-session conversion needs it in hand; a constant path that skips the query breaks both.
- A constant's pre-resolved gate carries **no** detection-derived light/dark answer — its value is the standing dark fallback and must never be read as "the terminal is dark". Task 9-6 classifies the retained reply when a conversion first needs an answer.

**Resolution**: Pending
**Notes**:

---

### 2. §9.3's "answer half" is unimplemented — a constant → adaptive conversion has no task that classifies the already-arrived background

**Type**: Missing from plan
**Spec Reference**: §9.3 ("Converting to adaptive mid-session starts using an answer that already arrived: no new query, no race, no gate… If the reply has not landed… it falls to **dark**, the same rule as everywhere else."), §8.2, §8.8
**Plan Reference**: Phase 9, task `theming-system-9-6` ("The newly-live opposite slot loads at commit with `theme: loaded`"); consumed by tasks `theming-system-8-8` and `theming-system-8-10`
**Change Type**: add-to-task

**Details**:
§9.3 is the mid-session constant → adaptive transition, and it has two halves. The plan covers one: task 9-6 loads the newly-live opposite slot, quoting the spec's "The transition's other half is a file, not an answer". But the *first* half — the light/dark answer itself — is only "dissolved" because the OSC 11 reply already arrived and can now be **classified and used**. Nothing in the plan classifies it.

The consequence is a concrete wrong behaviour rather than a documentation gap. Under §8.2 a constant means "Detection is never consulted", and task 3-2 constructs that user's gate "already resolved and unarmable" — resolved to the standing dark fallback, since no classification ever ran. Task 8-8 and task 8-10 then both select the in-force member as "the light member iff the gate's resolved light/dark answer is light, else dark". So a user who launched on `"theme": "nord"` on a **light** terminal, converts to adaptive with `l`, and presses `Esc` lands on the **dark** slot — Portal ignoring a terminal-background answer it is holding, which is exactly the outcome §9.3 says dissolves.

The fix belongs on task 9-6 because the converting commit is the keypress that first needs an answer, and because 8-8/8-10 already read "the gate's resolved answer" — establishing it here leaves both consumers correct with no further edit. It depends on finding 1 (the query being unconditional and its reply retained).

**Current**:

From task `theming-system-9-6`, **Do** (sixth and seventh bullets):

- **Join the nomination, not the active member**: the loaded `Theme` becomes the model's other nomination member so the pair is complete; the **active** member does not change (a commit is a write, not a navigation), and `ApplyTheme` is not called. Note that task 8-10's close re-resolves from persisted state regardless, so the join is model consistency rather than a dependency of the close path.
- **Emit nothing where nothing converts**: an already-adaptive `d`/`l`, any `Enter`, and any **failed** write all load nothing and emit nothing — both members are in hand already (or the write did not land), so there is no load to announce.

From task `theming-system-9-6`, **Acceptance Criteria** (ninth bullet):

- [ ] The model's nomination holds both members after a conversion, while the active theme and the composed frame are unchanged.

From task `theming-system-9-6`, **Tests** (tenth entry):

- `"it completes the nomination without changing the active member"` — `TestCommitSlotLoad_ActiveThemeUnchanged`

From task `theming-system-9-6`, **Edge Cases** (ninth bullet):

- The loaded member joins the model's nomination so the pair is complete, but the **active** member does not change.

**Proposed**:

Task `theming-system-9-6`, **Do** — insert this bullet between "Join the nomination, not the active member" and "Emit nothing where nothing converts" (both otherwise unchanged):

- **Resolve the answer half too, from the reply already in hand.** A conversion makes light/dark matter for a user whose launch deliberately never consulted detection (§8.2), so the in-force light/dark answer must be **established here** — never read off the constant path's pre-resolved gate, whose value is the standing dark fallback rather than a classification of the terminal (task 3-2). Classify the OSC 11 background retained at launch and record it as the model's light/dark answer, so task 8-10's close and task 8-8's next open select the in-force member correctly with no change of their own. **No new query, no new race, no new gate** — §9.3's transition dissolves precisely because the answer already arrived, and the single-resolution rule (§8.8) is untouched: a reply arriving after this point still never re-themes. If **no reply has landed** — which requires the panel to have been opened within milliseconds of launch — it falls to **dark**, the same rule as everywhere else. A user who launched on an adaptive pair already has a classified answer, so a conversion never arises for them and nothing is re-derived.

Task `theming-system-9-6`, **Acceptance Criteria** — replace the ninth bullet with these five:

- [ ] The model's nomination holds both members after a conversion, while the active theme and the composed frame are unchanged.
- [ ] After a confirmed conversion on a **light** terminal the model's light/dark answer is light, and the next close (task 8-10) selects the **light** slot; on a dark terminal it selects the dark slot.
- [ ] The conversion issues **no** new OSC 11 query and arms no new gate — the answer is classified from the background captured at launch.
- [ ] A conversion performed before any reply has landed resolves to **dark**, and a reply arriving afterwards still does not re-theme.
- [ ] A user who launched on an adaptive pair is unaffected: their answer is already classified and a `d`/`l` commit does not re-derive it.

Task `theming-system-9-6`, **Tests** — replace the tenth entry with these four:

- `"it completes the nomination without changing the active member"` — `TestCommitSlotLoad_ActiveThemeUnchanged`
- `"it classifies the retained background on conversion"` — `TestCommitSlotLoad_ConversionUsesTheRetainedAnswer` (light terminal, dark terminal)
- `"it issues no new query on conversion"` — `TestCommitSlotLoad_ConversionIssuesNoQuery`
- `"it falls back to dark when no reply landed"` — `TestCommitSlotLoad_ConversionWithNoReplyIsDark`

Task `theming-system-9-6`, **Edge Cases** — replace the ninth bullet with these four:

- The loaded member joins the model's nomination so the pair is complete, but the **active** member does not change.
- §9.3's transition has **two halves** and both land on this keypress: the file half (the opposite slot's load) and the **answer half** — the light/dark classification a constant user's launch deliberately never made. The answer half dissolves only because `restore.go`'s query ran anyway; it does not dissolve into *nothing*, so the classification has to happen somewhere and this is the first keypress that needs it.
- The constant path's gate is **pre-resolved to the standing dark fallback**, not to a classification of the terminal, so reading it as the in-force answer after a conversion puts a light-terminal user on the dark slot — the exact outcome §9.3 exists to prevent.
- **No new query, no race, no gate**, and **dark** when no reply has landed — the same rule as everywhere else.

**Resolution**: Pending
**Notes**:

---

### 3. Task 4-1's structural AST guard is not in the specification

**Type**: Hallucinated content
**Spec Reference**: §11.2 (requires the sweep be *run*, offenders *fixed* and residue *recorded*; names §13.4's guard as "what stops them returning"), §13.4 ("This is a **behavioural** guard, not a structural one, deliberately… A structural guard would have to recognise 'this is a cached style' in the AST, which is not mechanically well-defined.")
**Plan Reference**: Phase 4, task `theming-system-4-1` ("Sweep the init-time derived styles and guard the class")
**Change Type**: update-task

**Details**:
§11.2 asks for three things and no more: run the derived-style sweep as its own act, **fix** what it finds, and **record** residue that cannot be fixed. It names the ongoing protection explicitly and it is the *behavioural* guard — "The swap-and-diff guard is the safety net that catches whatever the sweep misses", and for the two known offenders "the guard is what stops them returning (§13.4)". §13.4 then says in terms that the guard is behavioural "not a structural one, **deliberately**".

Task 4-1 adds a new permanent structural AST guard (`TestNoInitTimeDerivedStyle`) that fails the build on any colour-bearing package-scope `var` in `internal/tui`. Nothing in the specification asks for it, and it introduces a standing rule the spec never set — a future package-scope value that legitimately carries colour and is legitimately re-pointed would fail a test that exists for a class the behavioural guard already covers. The plan's own justification for it ("so the class cannot silently return") is the job §11.2 assigns to §13.4's guard.

The proposal removes the structural guard and keeps everything the spec does require: both sweeps, the fix-never-exempt rule, the per-member assertions that §11.2's hand-maintained list is genuinely re-pointed, and the residue record — re-homed from the deleted guard file to a durable comment beside the restyle path plus the task's commit message, which is what "leaves the residue undocumented" asks for.

The alternative is to keep the guard as an **explicitly approved addition**; if that is preferred, reject this finding rather than applying it.

**Current**:

Task `theming-system-4-1` — title, **Solution**, **Outcome**, **Do**, **Acceptance Criteria**, **Tests** and **Edge Cases** as they stand (its **Problem**, **Context** and **Spec Reference** are unchanged by this finding):

### Task 4.1: Sweep the init-time derived styles and guard the class

**Solution**: run the sweep as its own act across `internal/tui` (and `internal/capture`), classifying every colour-bearing value assigned outside the per-frame render path into exactly one of three buckets — **fix** (an offender), **leave** (a legitimate colour-free init value), **record** (residue that cannot be fixed) — and land a structural guard that discriminates on **colour**, not on package scope, so the class cannot silently return.

**Outcome**: every colour-bearing value in `internal/tui` is either re-derived per frame or re-pointed by the restyle path; the residue list exists in a durable, discoverable place; and a guard test fails the build if a new package-scope colour-bearing value appears.

**Do**:
- **Run the package-scope sweep.** Enumerate every package-level `var` in every non-test `.go` file under `internal/tui` (the same package-dir glob idiom `colour_literal_guard_test.go` already uses — `filepath.Glob("./*.go")`, `_test.go` excluded) and classify each initialiser. Colour-bearing means: it calls `.Foreground(` / `.Background(` / `.BorderForeground(` / `.BorderBackground(`, calls `lipgloss.Color(`, or references a `theme.` selector (a `theme.Token`, a `theme.Theme`, or a value derived from one). Known state going in: `nameBase` (`session_item.go`) and `projectNameBase` (`project_item.go`) are `lipgloss.NewStyle().Bold(true)` — **colour-free, legitimate, must not be flagged**; `attachedSlotWidth` (a `lipgloss.Width` measurement), `loadingBlockBannerWidth` (an int), `emptyFooterKeys`, `loadingWordmark`, `labelOrder`, `stepLabelTable` are likewise colour-free; `previewBorderColorToken` is already gone (task 3-1).
- **Run the construction-time sweep**, which is in scope as well as package-init: any style assigned once in `New` / `newSessionList` / `newProjectList` / a modal-open handler and never re-pointed by `applyCanvasMode` / `applyProjectCanvasMode` / `styleFilterInput`. §11.2's hand-maintained list is the checklist and is the whole point of running the sweep rather than trusting it: the `bubbles/list` help styles (`Styles.HelpStyle`), the pagination dots (`Styles.ActivePaginationDot` / `InactivePaginationDot` **and** the rendered `Paginator.ActiveDot` / `InactiveDot` strings `list.New` reads once at construction), `Styles.TitleBar` and `Styles.Title`, both filter inputs (`FilterInput.SetStyles`, both lists), and both delegates (`SessionDelegate` / `ProjectDelegate`). For each member, prove by assertion that the live value carries the *new* theme's colour after the restyle path runs — do not accept "it looks re-pointed".
- **Fix a found offender; never guard around it.** §11.2: "Two known offenders are fixed outright, not guarded around… Fixing them does not make the guard redundant; the guard is what stops them returning." A package-scope derived style is fixed by inlining its construction at the use site (taking the active `Theme` as every renderer already does); a construction-time style is fixed by adding it to the restyle path.
- **Record residue where it cannot be fixed**, as a comment block at the head of the new guard test file, one line per item with the reason. The first entry is known: `internal/capture`'s contrast-validation swatch (`swatch.go`) takes its theme per invocation and never swaps, so its once-assigned styles are **deliberate recorded residue, not a finding**. Also record the outcome of the sweep in the task's commit message — a sweep that found nothing is a finding, not a non-event.
- **Land the structural guard** in `internal/tui` (test package `tui_test`, alongside the colour-literal guard so the two read as one family): `TestNoInitTimeDerivedStyle` parses each production file with `go/parser`, walks every package-level `var` initialiser, and fails on any colour-bearing one. Allow by **shape** (no `.Foreground`/`.Background`/`lipgloss.Color`/`theme.` in the initialiser), never by a name allowlist — a name list would need editing every time a legitimate init value is added and would rot into an exemption list.
- **Do not pre-build the panel's third `bubbles/list` instance.** §11.2 names it as the worst case of this class, and Phase 8 owns it; the guard is written so the instance inherits coverage when it lands, without this task creating it.

**Acceptance Criteria**:
- [ ] Every package-level `var` in `internal/tui`'s production files is classified, and none is colour-bearing.
- [ ] `nameBase`, `projectNameBase`, `attachedSlotWidth` and `loadingBlockBannerWidth` are **not** flagged — the guard discriminates on colour, not on package scope.
- [ ] Every member of §11.2's hand-maintained list has a passing assertion that it carries the new theme's colour after the restyle path runs: both lists' `HelpStyle`, both lists' `ActivePaginationDot`/`InactivePaginationDot` **and** the rendered `Paginator.ActiveDot`/`InactiveDot` strings, both `TitleBar`s, both `FilterInput` style sets, and both delegates.
- [ ] Any offender found is **fixed**, not exempted; the guard passes with no exemption entries of any kind.
- [ ] The residue list exists at the head of the guard test file, carries a reason per entry, and includes `internal/capture`'s swatch.
- [ ] The guard fails when a colour-bearing package-scope var is deliberately introduced (proven by a temporary local edit during development, reverted).
- [ ] No panel / third `bubbles/list` instance is introduced by this task.
- [ ] `go build ./... && go test ./...` green; `golangci-lint run` clean.

**Tests**:
- `"it flags a package-scope style carrying a colour"` — `TestNoInitTimeDerivedStyle` (AST guard over the production glob)
- `"it does not flag a colour-free init value"` — `TestNoInitTimeDerivedStyle_AllowsColourFreeInitValues` (table: `nameBase`, `projectNameBase`, `attachedSlotWidth`, `loadingBlockBannerWidth`)
- `"it re-points every bubbles/list-owned style on both lists"` — `TestRestylePath_RepointsListOwnedStyles` (help style, both pagination dot styles, the two rendered `Paginator` dot strings, TitleBar, Title — sessions and projects)
- `"it re-points both filter inputs"` — `TestRestylePath_RepointsBothFilterInputs`
- `"it re-points both delegates"` — `TestRestylePath_RepointsBothDelegates`
- `"it leaves the swatch's per-invocation styling alone"` — no test; recorded residue, asserted only by the residue comment block

**Edge Cases**:
- The two named offenders were **already fixed in Phase 3** and are "what was *found*, not the boundary of the class" — this task's value is the sweep for **derived styles**, which was never run.
- The guard discriminates on **colour, not package scope**: `nameBase` / `projectNameBase` (bold only) and `attachedSlotWidth` (a measured width) are legitimate init-time values and must not be flagged.
- **Construction-time derivation is in scope as well as package-init** — a style assigned once in `New` / `newSessionList` and never re-pointed by the restyle path is the same defect with a different assignment site.
- A found offender is **fixed**, not guarded around; fixing it does not make the guard redundant — the guard is what stops it returning.
- Residue that cannot be fixed is **recorded**. `internal/capture`'s swatch takes its theme per invocation and never swaps: deliberate recorded residue, not a finding.
- This structural guard is **not a substitute** for §13.4's behavioural one (task 4-3) — a structural guard cannot recognise "this is a cached style" in the AST, which is exactly why the behavioural guard exists.
- The panel's future third `bubbles/list` instance is **Phase 8's** and must not be pre-built here.

**Proposed**:

### Task 4.1: Sweep the init-time derived styles and record the residue

**Solution**: run the sweep as its own act across `internal/tui` (and `internal/capture`), classifying every colour-bearing value assigned outside the per-frame render path into exactly one of three buckets — **fix** (an offender), **leave** (a legitimate colour-free init value), **record** (residue that cannot be fixed) — and prove by assertion that every member of §11.2's hand-maintained list is genuinely re-pointed by the restyle path.

**Outcome**: every colour-bearing value in `internal/tui` is either re-derived per frame or re-pointed by the restyle path, each member of §11.2's list has an assertion proving it, and the residue list exists in a durable, discoverable place — with task 4-3's behavioural guard, per §11.2, as what stops the class returning.

**Do**:
- **Run the package-scope sweep.** Enumerate every package-level `var` in every non-test `.go` file under `internal/tui` (the same package-dir glob idiom `colour_literal_guard_test.go` already uses — `filepath.Glob("./*.go")`, `_test.go` excluded) and classify each initialiser. Colour-bearing means: it calls `.Foreground(` / `.Background(` / `.BorderForeground(` / `.BorderBackground(`, calls `lipgloss.Color(`, or references a `theme.` selector (a `theme.Token`, a `theme.Theme`, or a value derived from one). Known state going in: `nameBase` (`session_item.go`) and `projectNameBase` (`project_item.go`) are `lipgloss.NewStyle().Bold(true)` — **colour-free and legitimate**; `attachedSlotWidth` (a `lipgloss.Width` measurement), `loadingBlockBannerWidth` (an int), `emptyFooterKeys`, `loadingWordmark`, `labelOrder`, `stepLabelTable` are likewise colour-free; `previewBorderColorToken` is already gone (task 3-1).
- **Run the construction-time sweep**, which is in scope as well as package-init: any style assigned once in `New` / `newSessionList` / `newProjectList` / a modal-open handler and never re-pointed by `applyCanvasMode` / `applyProjectCanvasMode` / `styleFilterInput`. §11.2's hand-maintained list is the checklist and is the whole point of running the sweep rather than trusting it: the `bubbles/list` help styles (`Styles.HelpStyle`), the pagination dots (`Styles.ActivePaginationDot` / `InactivePaginationDot` **and** the rendered `Paginator.ActiveDot` / `InactiveDot` strings `list.New` reads once at construction), `Styles.TitleBar` and `Styles.Title`, both filter inputs (`FilterInput.SetStyles`, both lists), and both delegates (`SessionDelegate` / `ProjectDelegate`). For each member, prove by assertion that the live value carries the *new* theme's colour after the restyle path runs — do not accept "it looks re-pointed".
- **Fix a found offender; never guard around it.** §11.2: "Two known offenders are fixed outright, not guarded around… Fixing them does not make the guard redundant; the guard is what stops them returning." A package-scope derived style is fixed by inlining its construction at the use site (taking the active `Theme` as every renderer already does); a construction-time style is fixed by adding it to the restyle path.
- **Record residue where it cannot be fixed**, as a comment block beside the restyle path (`applyCanvasMode` in `internal/tui/model.go` — where a reader asking "what re-points, and what does not?" lands), one line per item with the reason. The first entry is known: `internal/capture`'s contrast-validation swatch (`swatch.go`) takes its theme per invocation and never swaps, so its once-assigned styles are **deliberate recorded residue, not a finding**. Also record the outcome of the sweep in the task's commit message — a sweep that found nothing is a finding, not a non-event.
- **Add no structural guard.** §11.2's protection against the class returning is task 4-3's **behavioural** swap-and-diff guard, and §13.4 makes that choice deliberate: a structural guard "would have to recognise 'this is a cached style' in the AST, which is not mechanically well-defined". This task's product is the sweep, its fixes, its per-member assertions and its residue record — nothing standing.
- **Do not pre-build the panel's third `bubbles/list` instance.** §11.2 names it as the worst case of this class, and Phase 8 owns it; task 4-3's guard covers the instance when it lands, without this task creating it.

**Acceptance Criteria**:
- [ ] Every package-level `var` in `internal/tui`'s production files is classified, and none is colour-bearing.
- [ ] `nameBase`, `projectNameBase`, `attachedSlotWidth` and `loadingBlockBannerWidth` are classified as legitimate colour-free init values — the sweep discriminates on colour, not on package scope.
- [ ] Every member of §11.2's hand-maintained list has a passing assertion that it carries the new theme's colour after the restyle path runs: both lists' `HelpStyle`, both lists' `ActivePaginationDot`/`InactivePaginationDot` **and** the rendered `Paginator.ActiveDot`/`InactiveDot` strings, both `TitleBar`s, both `FilterInput` style sets, and both delegates.
- [ ] Any offender found is **fixed**, never exempted or worked around.
- [ ] The residue list exists beside the restyle path, carries a reason per entry, and includes `internal/capture`'s swatch.
- [ ] The sweep's outcome — offenders found and fixed, or none — is recorded in the task's commit message.
- [ ] No new standing guard test is introduced by this task; task 4-3's behavioural guard is the protection against the class returning.
- [ ] No panel / third `bubbles/list` instance is introduced by this task.
- [ ] `go build ./... && go test ./...` green; `golangci-lint run` clean.

**Tests**:
- `"it re-points every bubbles/list-owned style on both lists"` — `TestRestylePath_RepointsListOwnedStyles` (help style, both pagination dot styles, the two rendered `Paginator` dot strings, TitleBar, Title — sessions and projects)
- `"it re-points both filter inputs"` — `TestRestylePath_RepointsBothFilterInputs`
- `"it re-points both delegates"` — `TestRestylePath_RepointsBothDelegates`
- `"it re-points a fixed offender"` — one assertion per offender the sweep finds, in the same shape as the three above (none if the sweep finds none)
- `"it leaves the swatch's per-invocation styling alone"` — no test; recorded residue, asserted only by the residue comment block

**Edge Cases**:
- The two named offenders were **already fixed in Phase 3** and are "what was *found*, not the boundary of the class" — this task's value is the sweep for **derived styles**, which was never run.
- The sweep discriminates on **colour, not package scope**: `nameBase` / `projectNameBase` (bold only) and `attachedSlotWidth` (a measured width) are legitimate init-time values.
- **Construction-time derivation is in scope as well as package-init** — a style assigned once in `New` / `newSessionList` and never re-pointed by the restyle path is the same defect with a different assignment site.
- A found offender is **fixed**, not guarded around; fixing it does not make task 4-3's guard redundant — that guard is what stops it returning.
- Residue that cannot be fixed is **recorded**. `internal/capture`'s swatch takes its theme per invocation and never swaps: deliberate recorded residue, not a finding.
- The sweep is a **one-time act** and leaves nothing standing behind it: §11.2 assigns the ongoing protection to §13.4's behavioural guard, and §13.4 rejects a structural one deliberately because "this is a cached style" is not mechanically decidable in the AST.
- The panel's future third `bubbles/list` instance is **Phase 8's** and must not be pre-built here.

**Resolution**: Pending
**Notes**: If the structural guard is wanted as a deliberate addition beyond the specification, reject this finding rather than applying it — the plan otherwise carries an unapproved standing rule.
