# Phase 4: Live-swap completeness and the swap-and-diff guard — 5 tasks

## theming-system-4-1

### Task 4.1: Sweep the init-time derived styles and record the residue

**Problem**: §11.2 names **two** offenders — `pagepreview.go`'s package-init `Token` copy and `canvasHexFor`'s built-in reference — and then says in terms that they "are what was *found*, not the boundary of the class": init-time copies of **derived styles** (a style struct built from a token at package scope, rather than the token itself) "were never swept for at all". Both named offenders were fixed in Phase 3 (tasks 3-1 and 3-3), so the *sweep itself* is still unrun. This matters because a derived style assigned once — at package init, or once in a constructor and never re-pointed — cannot see a theme swap: the element it paints silently keeps the previous theme's colours until something else re-renders it. The class is invisible to code reading, because the assignment site and the render site are different files, and §11.2 is explicit that "a sweep that is never run leaves the guard doing work a five-minute grep would have done, and leaves the residue undocumented".

**Solution**: run the sweep as its own act across `internal/tui` (and `internal/capture`), classifying every colour-bearing value assigned outside the per-frame render path into exactly one of three buckets — **fix** (an offender), **leave** (a legitimate colour-free init value), **record** (residue that cannot be fixed) — and prove by assertion that every member of §11.2's hand-maintained list is genuinely re-pointed by the restyle path.

**Outcome**: every colour-bearing value in `internal/tui` is either re-derived per frame or re-pointed by the restyle path, each member of §11.2's list has an assertion proving it, and the residue list exists in a durable, discoverable place — with task 4-3's behavioural guard, per §11.2, as what stops the class returning.

**Do**:
- **Run the package-scope sweep.** Enumerate every package-level `var` in every non-test `.go` file under `internal/tui` (the same package-dir glob idiom `colour_literal_guard_test.go` already uses — `filepath.Glob("./*.go")`, `_test.go` excluded) and classify each initialiser. Colour-bearing means: it calls `.Foreground(` / `.Background(` / `.BorderForeground(` / `.BorderBackground(`, calls `lipgloss.Color(`, or references a `theme.` selector (a `theme.Token`, a `theme.Theme`, or a value derived from one). Known state going in: `nameBase` (`session_item.go`) and `projectNameBase` (`project_item.go`) are `lipgloss.NewStyle().Bold(true)` — **colour-free, legitimate, must not be flagged**; `attachedSlotWidth` (a `lipgloss.Width` measurement), `loadingBlockBannerWidth` (an int), `emptyFooterKeys`, `loadingWordmark`, `labelOrder`, `stepLabelTable` are likewise colour-free; `previewBorderColorToken` is already gone (task 3-1).
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

**Context**:
> §11.2: "**The real risk is completeness.** Threading the theme (§3.4) fixes most of this: anything taking the theme as a parameter re-derives per frame. What remains is the **cached styles Portal does not own** — `bubbles/list`'s help styles, pagination dots, TitleBar, and both filter inputs — which are assigned once. That list is hand-maintained with no guard test… Miss a site and the element silently keeps the previous theme's colours until something else re-renders it."
> §11.2: "**These two are what was *found*, not the boundary of the class.** Init-time copies of *derived styles* (a style struct built from a token at package scope, rather than the token itself) were never swept for at all. Implementation must run that sweep rather than treating the two named fixes as closing the category. The swap-and-diff guard is the safety net that catches whatever the sweep misses — but a sweep that is never run leaves the guard doing work a five-minute grep would have done, and leaves the residue undocumented."
> §11.2: "**The panel introduces a third `bubbles/list` instance, and it is the worst case of this class.**… The `bubbles/list`-owned styles the panel uses (pagination dots, its own help/title styles) are re-pointed by **the same restyle path as the main list**, extended to cover the panel's instance." (Phase 8.)
> §13.4: "This is a **behavioural** guard, not a structural one, deliberately… A structural guard would have to recognise 'this is a cached style' in the AST, which is not mechanically well-defined." — which is why this task's structural guard is scoped to the narrower, mechanically-decidable question (does a package-scope initialiser carry colour?) and does not attempt the cached-style question.
> Phase boundary: Phase 3 task 3-1 removed `pagepreview.go`'s package-scope `Token` copy and task 3-3 made the exit-time restore theme-agnostic. This task owns the **wider** sweep; task 4-3 owns the behavioural guard.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §11.1, §11.2, §13.4

## theming-system-4-2

### Task 4.2: The live theme-swap entry point and the render-swap-render harness seam

**Problem**: After Phase 3 the model *holds* an active `Theme` but nothing can *change* it mid-session — `activeTheme` is set once at gate resolution and never again. Phase 8's panel needs exactly that mutation on every arrow keypress, on open (when a mid-session file edit changed or invalidated the active theme) and on close, and §13.4's completeness guard needs to drive it from a test. Neither half is verifiable alone: a harness seam with no swap path renders nothing new, and a swap path with no seam cannot be driven behaviourally. There is also a trap that makes the guard silently worthless if this task gets it wrong — §13.4: "the caches it exists to catch are assigned *once at construction*, so a test that builds two models — one per theme — assigns every cached style correctly in each and **passes green while live swap is broken**. The fixture harness builds a fresh model per fixture today, which is exactly the shape that would produce that vacuous pass."

**Solution**: add one exported production entry point on `Model` that swaps the active theme through the existing `applyCanvasMode` restyle and style re-point, and one `internal/capture` seam that drives a fixture to its captured state, renders, swaps **the same model** through that entry point, and renders again.

**Outcome**: `Model.ApplyTheme` is the single mid-session swap path (the one Phase 8's arrow-preview, panel-open and panel-close will call), and `internal/capture` can produce a before/after frame pair for any fixture from **one** model — with the before-frame demonstrably populating the caches the guard exists to catch.

**Do**:
- **Add the swap entry point** in `internal/tui/model.go`: `func (m *Model) ApplyTheme(th theme.Theme)` — set `m.activeTheme = th`, then call `m.applyCanvasMode()` (which already re-points both delegates, both lists' `bubbles/list`-owned styles and both filter inputs, and branches on `m.colourless`). Document it as the **production** entry point Phase 8 drives from arrow-preview, panel open and panel close — not a test-only setter — and document the three things it must **not** do: no `rebuildSessionList` (§11.1 — that is the expensive path with the lazy per-session tmux pane reads), no file read, and no write to `startupCanvasHex`.
- **Guard the three prohibitions with assertions**, not comments alone: a swap performs zero calls through the injected `DirReader` seam (which is what `rebuildSessionList`'s lazy dir-resolution pass would use), and `m.startupCanvasHex` is byte-identical before and after any number of swaps.
- **Add the fixture driver** in `internal/capture`: `func (f *Fixture) ModelAt(th theme.Theme, w, h int) tui.Model`. It builds the production model via `tui.Build(f.Deps())` with the **constant** nomination shape carrying `th` (no gate, no wait — the same shape `capturetool` passes), then drives it to the fixture's captured state deterministically **without a tea program**, by feeding `Update` in fixed order: `tea.WindowSizeMsg{w, h}` → `tui.SessionsMsg` built from the fixture's own `Lister` → `tui.ProjectsLoadedMsg` built from the fixture's own project store → each seeded `loadingEvents` entry as a `tui.BootstrapProgressMsg` → the seeded `fatalEvent` when non-zero. Blocking commands (the `LoadingMinDuration` tick, the loading receiver) are never invoked.
- **Declare the post-load key script on the fixture** and replay it as the last step of the driver. Two of today's fixtures reach their captured state through a keypress their tape types rather than through a seed — `projects` presses `x`, `preview-screen` presses `Space` — so without this the driver renders those two as Sessions frames and the guard's coverage claim over the Projects screen and the preview overlay is hollow (see Context for the flag on this). Add a `captureKeys []tea.KeyPressMsg` field alongside the existing `initial*` seeds, populate it for those two fixtures only, and note in its doc comment that it is the same sequence the fixture's tape types, declared once so the two cannot drift.
- **Add the swap harness**: `func (f *Fixture) RenderSwapRender(a, b theme.Theme, w, h int) (before, after string)` — `ModelAt(a, w, h)`, capture `m.View().Content` (**this render is what populates the caches, not optional set-up**), call `m.ApplyTheme(b)` on that same model, capture `m.View().Content` again. Exactly one model is constructed; document that building two is the vacuous-pass shape §13.4 names.
- **Expose the colourless discriminator**: `func (f *Fixture) Colourless() bool` reading `f.Deps().NoColor`, so task 4-3's exclusion is structural rather than a name list. No fixture sets it today; the accessor exists so one that does is excluded automatically.
- **Keep the harness free of config discovery.** The seam adds no XDG lookup, no prefs read and no themes-directory read — the theme arrives as an injected value. Re-run `internal/capture`'s no-real-config import guard and `TestPortalBinaryDoesNotImportCapture`; both must stay green.

**Acceptance Criteria**:
- [ ] `Model.ApplyTheme` exists, sets the active theme and re-points every style through `applyCanvasMode`; it is the only mid-session swap path (no second setter).
- [ ] A swap performs **no** `rebuildSessionList` and no lazy per-session pane read: a model wired with a counting `DirReader` records zero reads across a swap in every grouping mode.
- [ ] A swap performs **no** file read (no loader call, no directory access) — the theme is already in hand.
- [ ] `startupCanvasHex` is unchanged after one swap and after fifty.
- [ ] Swapping to the **identical** theme is a legal no-op (the post-swap frame is byte-identical to the pre-swap frame), and repeated swaps are idempotent per swap (A→B→B renders the same as A→B).
- [ ] A **colourless** model stays colourless across a swap: the post-swap frame contains no `38;2;`/`48;2;` sequence at all — no hue leaks through the re-point.
- [ ] `RenderSwapRender` constructs exactly **one** model: the post-swap frame retains state established before the swap (e.g. the `projects` fixture is still on the Projects page after the swap, which a rebuilt model would lose).
- [ ] The before-frame is non-empty, is not the pre-resolution blank frame, and carries theme A's canvas — proving the A-render happened and populated the caches.
- [ ] `ModelAt` renders every registered `tui.Build`-backed fixture in its **captured** state: `projects` on the Projects page, `preview-screen` with the preview overlay open, `loading-screen` mid-restore, `loading-error` on the fatal frame, and every `sessions-*` fixture with its session rows present (not the empty state).
- [ ] Both delegates and both lists' `bubbles/list`-owned styles re-point together on a swap; no panel/third-list instance is referenced.
- [ ] `internal/capture` still performs no XDG lookup and no prefs read; the two import guards pass.

**Tests**:
- `"it restyles the model without rebuilding the list"` — `TestApplyTheme_RestylesWithoutRebuild` (counting `DirReader` records zero reads)
- `"it reads no file on a swap"` — `TestApplyTheme_PerformsNoFileRead`
- `"it never moves the startup canvas hex"` — `TestApplyTheme_DoesNotMoveStartupCanvasHex` (1 swap and 50)
- `"it treats an identical theme as a no-op"` — `TestApplyTheme_SameThemeIsANoOp`
- `"it is idempotent per swap"` — `TestApplyTheme_RepeatedSwapsAreIdempotent`
- `"it keeps a colourless model colourless"` — `TestApplyTheme_ColourlessStaysColourless`
- `"it drives every fixture to its captured state"` — `TestModelAt_ReachesCapturedState` (table over the `tui.Build`-backed fixtures, asserting the distinguishing content of each captured frame)
- `"it renders before it swaps"` — `TestRenderSwapRender_ARenderPopulatesCachesBeforeSwap`
- `"it mutates one model rather than building two"` — `TestRenderSwapRender_MutatesASingleModel` (page state established pre-swap survives)
- `"it reports a fixture's colourless flag"` — `TestFixtureColourless_ReadsDepsNoColor`
- `"it keeps the harness free of config discovery"` — existing `internal/capture` / `cmd/capturetool` import guards, plus `TestPortalBinaryDoesNotImportCapture`

**Edge Cases**:
- The swap is a live mutation of **one** already-rendered model — a test that builds two models, one per theme, assigns every cached style correctly in each and passes green while live swap is broken.
- The A-render is **not** optional set-up: it is what populates the caches, so a fixture rendered only after the swap passes trivially.
- The fixture harness builds a fresh model per fixture today, which is exactly the shape that produces that vacuous pass.
- The entry point is the one Phase 8's arrow-preview, panel-open and panel-close will drive — **not** a test-only setter and **not** a rebuild.
- It is the `applyCanvasMode` restyle and style re-point, so `rebuildSessionList` and its lazy per-session pane reads must not fire (§11.1), and no file read happens per swap.
- Fixtures reach their captured state through `Init`/`Update` — window size, sessions loaded, loading progress, the terminal fatal event — so the seam must drive them deterministically without a tea program, or most A-renders are a blank or loading frame and the guard's coverage claim is hollow.
- Both delegates and both lists' `bubbles/list`-owned styles re-point together; the panel's third instance does not exist yet.
- A colourless model stays colourless across a swap — no hue leaks through the re-point.
- `startupCanvasHex` must **not** move when the theme swaps: it is the anchor task 4-5 rests on.
- The seam adds no config discovery, so `internal/capture`'s no-real-config import guard and `TestPortalBinaryDoesNotImportCapture` both stay green.
- Swapping to an identical theme is a legal no-op, and repeated swaps are idempotent per swap.

**Context**:
> §13.4: "**The swap must be a live mutation of one already-rendered model, through the production swap path.** This is the guard's whole point and the easiest way to build it wrong… **Render under A first, then swap, then render again.** The A-render is not optional set-up — it is what populates the caches… **The swap goes through the same entry point the panel's arrow-preview uses** (the `applyCanvasMode` restyle and style re-point), not a test-only setter and not a rebuild. **`internal/capture` / `tui.Build` must expose a seam to drive that from a test**, since fixtures are one-shot renders today. Adding it is in scope (§13.3)."
> §11.1: "**Restyle** — `applyCanvasMode` swaps the delegate and re-points the cached style structs `bubbles/list` holds. O(1), no I/O, no list content touched. It performs exactly the mid-session restyle a theme swap needs. **Its production caller changes with this feature**… from here its callers are the panel's **arrow-preview**, its **open**… and its **close**… **`applyCanvasMode` does not call `rebuildSessionList`.** Nothing heavy is on the theme-swap path."
> §11.1: "**Rebuild** — `rebuildSessionList` re-derives the item list and, in grouped modes, runs the lazy dir-resolution pass with its per-session tmux pane reads (the known ~0.5s By-Project cost at ~38 sessions)." — the path a swap must never take.
> §5.8: the panel's enumeration parse is retained for the panel's lifetime "so arrowing previews from values already in hand — no file read per keystroke, which is what keeps the swap the O(1) restyle of §11.1".
> **Ambiguity flagged**: the plan's edge case enumerates the message classes the seam must drive (window size, sessions loaded, loading progress, the terminal fatal event) but the `projects` and `preview-screen` fixtures reach their *captured* state through a keypress their tape types (`x` and `Space` respectively) rather than through a seeded field — verifiable in `testdata/vhs/projects.tape` and `preview-screen.tape` before Phase 3 deletes them. Driving only the four message classes would render those two as Sessions frames, leaving the Projects screen and the preview overlay uncovered by §13.4's guard while the fixture list claims otherwise. The declared `captureKeys` script is the minimal closure of that gap and is called out here rather than folded in silently; it introduces no new capability (keypresses already route through `Update`).

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §11.1, §11.2, §13.3, §13.4

## theming-system-4-3

### Task 4.3: The swap-and-diff guard over the enumerated fixture set

**Problem**: §11.2's cached-style class "cannot reliably be found by reading code", and task 4-1's sweep is a point-in-time act — it says nothing about the screen someone adds next year. §13.4 is the feature's central completeness mechanism and it has three ways to pass while proving nothing: naming fixtures instead of enumerating them (a test naming four or five "satisfies all three assertions today and keeps passing tomorrow, while the next screen anyone adds goes silently uncovered"), using two shipped themes (a hex both palettes set identically "renders identically before and after, so the test cannot tell whether that site updated — it passes either way and the site is uncovered with no signal"), and searching for the wrong representation (assertion 1 is a **negative**, so "searching for the wrong representation passes vacuously and silently"). All three failures are silent, which is why the guard's construction is specified as tightly as its assertions.

**Solution**: a unit-lane test that enumerates the harness's fixture registry, renders each included fixture under synthetic theme A through task 4-2's seam, swaps to synthetic theme B through the production entry point, renders again, and asserts on each token's **rendered SGR form** — no theme-A value survives, and the union of theme-B values across fixtures matches the union observed under A.

**Outcome**: any missed re-point site — today's or one added years later, on any screen — fails a test that names the offending token and the fixture, with no fixture list to maintain.

**Do**:
- **Land the guard** as `internal/capture/theme_swap_guard_test.go`, test package `capture_test` (it must import `internal/capture`, `internal/tui` and `internal/theme`, which no single production package may). Lane **unit** — no tmux server, no daemon, no built binary — and **no `t.Parallel()`**, per the project-wide rule.
- **Enumerate, never name.** Iterate `capture.FixtureNames()`. The single sanctioned skip is `capture.ContrastValidationFixture`, compared against the **exported constant** and never a string literal, because it is a standalone `tea.Model` and not a `tui.Build`-backed `*Fixture`. Every other name must resolve through `capture.FixtureByName` or the test **fails loudly** (`t.Fatalf`) — a silent skip reads as coverage. Add the converse assertion too: `FixtureByName(capture.ContrastValidationFixture)` must return an error, so if the swatch is ever promoted to a real fixture the guard fails and forces the skip to be reconsidered rather than silently dropping a screen.
- **Close the two-list drift.** `FixtureByName`'s `switch` and `FixtureNames()`'s slice are two hand-maintained lists; a fixture present in the switch but absent from the slice is invisible to an enumerating guard. AST-scan `internal/capture/fixtures.go` for `FixtureByName`'s `case` string literals and assert that set equals `FixtureNames()` minus the swatch constant.
- **Exclude colourless fixtures structurally**, via task 4-2's `Fixture.Colourless()` — never by name. "A colourless render contains no theme hexes, so inclusion is meaningless rather than merely redundant." Today's set contains none, so the exclusion is live but empty; do not assert the excluded count (that would fail the day one is added, which is the opposite of the intent).
- **Build two synthetic themes inside the test** — `theme.Theme` is an ordinary struct (§3.2), so they need no loader, no file and no embedded set. All **38** values unique, none repeated within a theme or across the pair. Generate them so every RGB component is in the **100–255** range (always three digits), which makes no rendered triple a prefix of another and removes the substring-prefix false match that decimal SGR parameters otherwise invite. Add a defensive assertion that all 38 differ, and a comment recording why shipped palettes are not used.
- **Compare rendered SGR forms, not hexes.** For each token value derive **both** `\x1b[…38;2;R;G;B…` (foreground) and `…48;2;R;G;B…` (background) parameter runs — `border` renders as a foreground on rules and frames while `canvas` and the `bg.*` tints only ever render as backgrounds — and match each with its terminator (`m` or `;`) so a longer parameter cannot swallow a shorter one. State in a comment that assertion 1 is a negative and would pass vacuously against the wrong representation, which is why the derivation is shared with assertion 2 rather than written twice.
- **Assert a truecolor render.** Under `go test` stdout is not a TTY, so the guard must compare the **`View().Content` string** (lipgloss v2 renders truecolor SGRs in-string; downsampling and `NO_COLOR` suppression happen at the output-writer layer) and never a writer-flushed frame. Add a canary assertion that each A-frame contains at least one `38;2;` run, so a future profile change cannot make the whole guard pass on colourless bytes.
- **Assertion 1 — no theme-A value survives.** For every included fixture, no A token's foreground or background form appears anywhere in the post-swap frame. On failure, name the **token and the fixture** — "a failure must name the offending token and fixture or the guard reports only that something somewhere is stale".
- **Assertion 2 — every theme-B value appears, as a union across fixtures.** Compute per-fixture observed-token sets for both frames (the per-fixture data is what makes the failure message nameable), then assert at the **union** level: the set of tokens observed under A across all included fixtures equals the set observed under B. This catches a site that renders **nothing** rather than merely stale. It is a union, never per fixture, because no single screen renders all 19 roles.
- **Additionally assert the frame's declarative background moves**: `View().BackgroundColor` is set from the canvas token (§11.3's per-frame assignment, which Bubble Tea diffs and emits as OSC 11 on change) and is **not** part of `Content`, so it needs its own one-line check that it equals B's canvas after the swap.
- **Pin the render size** as a named constant wide and tall enough that fixtures render their full chrome rather than a degraded ladder step, with a comment saying why the value was chosen.

**Acceptance Criteria**:
- [ ] The guard enumerates `capture.FixtureNames()` and names no fixture; adding a fixture puts it under the guard with no test edit.
- [ ] The only skip is `capture.ContrastValidationFixture` via the exported constant; any other unresolvable name is a `t.Fatalf`, and `FixtureByName(ContrastValidationFixture)` is asserted to error.
- [ ] `FixtureByName`'s `case` set equals `FixtureNames()` minus the swatch — the two hand-maintained lists cannot drift silently.
- [ ] Colourless fixtures are excluded through `Fixture.Colourless()`, with no name list and no pinned excluded-count.
- [ ] Two synthetic themes are constructed in-test with all 38 values unique, asserted; no shipped palette is used.
- [ ] Both the `38;2;` and `48;2;` forms are searched for every token, terminator-aware.
- [ ] The A-frame carries at least one truecolor run (the canary), and the comparison is against `View().Content`, never writer-flushed output.
- [ ] Assertion 1 passes over every included fixture, and its failure message names token + fixture.
- [ ] Assertion 2 passes as a union across fixtures, and its failure message names the token(s) and where the A form was seen.
- [ ] `View().BackgroundColor` equals theme B's canvas after the swap.
- [ ] Deliberately breaking one re-point site (e.g. reverting `canvasPaginationDots` to a captured style) fails the guard with a message naming that token — verified during development and reverted.
- [ ] Lane is unit; the test uses no `t.Parallel()`; `go test ./...` green.

**Tests**:
- `"it finds no theme-A value after the swap"` — `TestThemeSwapGuard_NoStaleValueSurvives` (subtest per enumerated fixture)
- `"it finds every theme-B value across the fixture union"` — `TestThemeSwapGuard_EveryBValuePresentInUnion`
- `"it enumerates the registry rather than naming fixtures"` — `TestThemeSwapGuard_EnumeratesRegistry` (fails loudly on an unresolvable name; asserts the swatch is the only skip)
- `"it keeps the two fixture lists in agreement"` — `TestFixtureRegistry_ByNameCasesMatchFixtureNames` (AST scan)
- `"it builds two synthetic themes with 38 unique values"` — `TestSyntheticThemes_AllValuesUnique`
- `"it renders truecolor under go test"` — `TestThemeSwapGuard_RenderIsTruecolor` (canary)
- `"it re-points the frame background colour"` — `TestThemeSwapGuard_ViewBackgroundColourFollowsSwap`
- `"it excludes a colourless fixture structurally"` — `TestThemeSwapGuard_ExcludesColourlessFixtures` (drives the exclusion with a locally-constructed colourless fixture, not a registry name)

**Edge Cases**:
- The guard **enumerates** the fixture set and never names a fixture — a test naming four or five satisfies every assertion today and keeps passing tomorrow while the next screen anyone adds goes silently uncovered.
- The enumeration source must be the registry itself, and `FixtureNames()` includes the `contrast-validation` swatch, which is **not** a `tui.Build`-backed fixture — the split must be explicit and an unresolvable name must fail loudly, because a silent skip reads as coverage.
- Two synthetic themes with all **38** values unique, none repeated within a theme or across the pair: a hex both palettes set identically either fails permanently for a non-bug or (worse, silently) renders identically before and after and covers nothing.
- `Theme` is an ordinary struct, so the synthetics need no loader.
- A synthetic value must not collide with non-theme colour on a frame (a fixture's canned scrollback or any raw SGR the harness seeds), or the negative assertion false-fails. Today's canned scrollback carries only 4-bit SGRs, but the generator must still avoid emitting a triple that could appear from anything but a token.
- Decimal SGR parameters are **prefix-ambiguous** (`38;2;1;0;5` is a prefix of `38;2;1;0;55`), so the match must be terminator-aware and the generator must use fixed-width components.
- The comparison is each token's **rendered SGR form**, never its hex, because assertion 1 is a negative that passes vacuously and silently against the wrong representation and assertion 2 is only a backstop.
- Both the foreground and background forms are searched, since `canvas` and the tints only ever render as a background.
- The render is forced to a truecolor profile — under `go test` stdout is not a TTY. In lipgloss v2 `Style.Render` is unconditionally truecolor and stripping happens at the writer layer, so this is satisfied by comparing `View().Content`; the canary assertion is what keeps that satisfaction honest if the library's behaviour changes.
- Assertion 2 is a **union across fixtures**, never per fixture, because no single screen renders all 19 roles.
- Colourless fixtures are excluded (a colourless render contains no theme hexes, so inclusion is meaningless rather than merely redundant), and today's set contains none, so the exclusion is expressed structurally rather than by name.
- Lane is **unit** — no tmux server, no daemon, no built binary — and no `t.Parallel()`.
- A failure must name the offending token and fixture, or the guard reports only that something somewhere is stale.

**Context**:
> §13.4: "**What it is:** render **every fixture** under theme A, switch to theme B, render again, and scan the second output for any colour value belonging to theme A. **The guard enumerates the harness's fixture set; it never names fixtures.** This is the mechanism the guard's claims rest on, not an implementation nicety… **the fixture list *is* the coverage list, and it grows automatically as screens are added**."
> §13.4: "**It uses two synthetic themes constructed inside the test, all 38 values deliberately unique** — none repeated within a theme or across the pair. Using two shipped themes has two failure modes, and both are a matter of time: A hex both palettes happen to set identically survives the swap *legitimately*… Worse and silent: a token with the *same* value in both themes renders identically before and after, so the test cannot tell whether that site updated."
> §13.4: "**The comparison is against each token's *rendered* form, not its hex.** Styled output carries no hex — a truecolor foreground is `ESC[38;2;R;G;B m`, decimal — so the guard converts each theme's token values to their SGR representation and searches for those. Stating it matters because assertion 1 is a **negative**: searching for the wrong representation passes vacuously and silently."
> §13.4: "**The render is forced to a truecolor profile.** Under `go test` stdout is not a TTY, so lipgloss would otherwise strip colour and there would be nothing to diff at all." Codebase note (`internal/tui/pagepreview_view_frame_test.go`): lipgloss v2 moved profile handling to the output-writer layer and `Render` is unconditionally truecolor, so the v1 `SetColorProfile` override no longer exists — comparing the `View().Content` string is what satisfies this requirement here.
> §13.4: "**Lane: unit.** It renders only through the offline harness — no tmux server, no daemon, no built binary." And: "**Colourless fixtures are excluded.**"
> §11.3: "**No per-keystroke churn.** Bubble Tea v2 **diffs** the view's background colour and emits only on change… The declarative per-frame `BackgroundColor` assignment is not a per-frame write." — which is why the frame's `BackgroundColor` field gets its own check alongside the `Content` scan.
> Phase boundary: assertion 3 (every token exercised by at least one fixture) is **task 4-4** — it is a coverage assertion with a different failure mode and can pull fixture work in.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §11.2, §11.3, §13.2, §13.3, §13.4

## theming-system-4-4

### Task 4.4: Guard assertion 3 — every token exercised by at least one fixture

**Problem**: Task 4-3's assertion 2 is a union across fixtures, and §13.4 is explicit that "the union in (2) is only complete if every token appears on *some* fixture". If a token renders nowhere in the fixture set, it is absent from both the A-union and the B-union — so assertion 2 balances perfectly and the guard is **silently blind at precisely the sites it exists to protect**. The at-risk tokens are the transient states, which is exactly where a missed re-point would be least likely to be noticed by eye: `bg.attention` / `text.on-attention` (the warning flash band), `accent.mode`, `state.destructive`, `text.on-selection`, plus `text.subtle` (grouped `··· N` counts and pending loading steps) and `bg.subtle` (the loading bar's empty track), neither of which has a locus on the flat Sessions screen.

**Solution**: assertion 3 — over the included fixtures' post-swap frames, every one of `Theme.All()`'s 19 tokens must be observed at least once — with any gap closed by **adding a fixture**, never by exempting a token.

**Outcome**: a token with no fixture fails the suite and someone adds a fixture; the guard's coverage claim is true rather than assumed, and every fixture added later (including Phase 8's panel set) is enrolled automatically.

**Do**:
- **Add the assertion** to `internal/capture/theme_swap_guard_test.go`, reusing task 4-3's observation helper so the two assertions cannot disagree about what "observed" means: `TestThemeSwapGuard_EveryTokenExercisedByAFixture` computes the union of tokens observed under theme B across the **included** (non-colourless) fixtures and asserts it equals the full set from `theme.Theme.All()` — all 19, no subset.
- **Look for whichever form the token actually takes.** A hit on **either** the `38;2;` foreground form or the `48;2;` background form counts: `border` renders as a foreground on rules and modal frames, while `canvas`, `bg.selection`, `bg.attention` and `bg.subtle` only ever render as backgrounds. Checking one form only would report a covered token as missing.
- **Compute coverage over the included set only.** A token covered solely by an excluded (colourless) fixture must still fail — a colourless render carries no theme hexes, so "covered there" means covered nowhere.
- **Write the failure message as an instruction**: name each uncovered token and state the remedy — add a fixture that renders it; do not exempt it, and do not weaken the assertion to "tokens observed under A" (that is assertion 2, and it is exactly the balance that hides the gap).
- **Run it and close whatever it finds.** Verify rather than assume the loci; §2.5's role table gives the first place to look for each: `bg.attention` / `text.on-attention` / `accent.attention` → `sessions-inline-flash`; `text.subtle` → the grouped `··· N` counts on `sessions-by-project` / `sessions-by-tag` and the loading page's pending steps; `bg.subtle` → the loading bar's empty track on `loading-screen`; `state.destructive` → the `loading-error` fatal frame and `sessions-multi-select-preflight-abort`'s red flag; `accent.mode` → the Sessions header and the preview chrome (`preview-screen`, reachable via task 4-2's key script); `text.on-selection` → the selected row on any sessions fixture.
- **Close a gap with the smallest honest fixture.** Follow the existing seed idiom — a new `Initial*` field on `Deps` mirroring `InitialFlash` / `InitialMultiSelect` / `InitialGoneFlagged`, or a `captureKeys` script from task 4-2 (a modal-only token, for instance, is reachable by a fixture whose script presses the key that opens it, which is what today's modal tapes do over `sessions-flat`). Register any new fixture in **both** `FixtureByName` and `FixtureNames()` — task 4-3's drift check enforces the pair.
- **Record the outcome in the task's commit message**: either "the existing set covers all 19" (a finding, not a non-event) or the fixtures added and the tokens they close.
- **Do not pre-build Phase 8's panel fixtures** or §11.2's paginating panel fixture. They inherit this assertion automatically when they land, which is the point of enumerating rather than naming.

**Acceptance Criteria**:
- [ ] The assertion covers all 19 tokens from `Theme.All()` — no subset, no exemption list, no skip flag.
- [ ] A token uncovered by every included fixture **fails** the test, with a message naming the token and prescribing "add a fixture".
- [ ] Coverage is computed over the **included** fixtures only; a token covered solely by an excluded colourless fixture still fails.
- [ ] Each token is matched on either its foreground or its background rendered form, so `border` (fg) and `canvas` / `bg.*` (bg) all resolve correctly.
- [ ] The assertion passes over the fixture set as it stands after any additions this task makes; every token has at least one named locus recorded.
- [ ] Any fixture added is registered in both `FixtureByName` and `FixtureNames()`, and task 4-3's drift check passes.
- [ ] No token is exempted, and assertion 2 is left unweakened.
- [ ] No panel fixture is created; the assertion is written so Phase 8's set enrols with no test edit.
- [ ] Deliberately removing a token's only locus (a temporary local edit) fails this assertion and not assertion 2 — the two failure modes are distinguishable.

**Tests**:
- `"it fails when a token is exercised by no fixture"` — `TestThemeSwapGuard_EveryTokenExercisedByAFixture` (the assertion itself; a locally-narrowed fixture set proves it fails rather than passing vacuously)
- `"it counts a token rendered only as a background"` — `TestTokenCoverage_MatchesBackgroundForm` (`canvas`, `bg.selection`, `bg.attention`, `bg.subtle`)
- `"it counts a token rendered only as a foreground"` — `TestTokenCoverage_MatchesForegroundForm` (`border`)
- `"it ignores coverage from an excluded colourless fixture"` — `TestTokenCoverage_IgnoresExcludedFixtures`
- `"it covers the transient-state tokens"` — `TestTokenCoverage_TransientStatesHaveALocus` (`bg.attention`, `text.on-attention`, `accent.mode`, `state.destructive`, `text.on-selection`, `text.subtle`, `bg.subtle`, each with the fixture that carries it)

**Edge Cases**:
- A token with no fixture must **fail the test** so someone adds a fixture, rather than the guard being silently blind at precisely the sites it exists to protect.
- The at-risk tokens are the transient states — `bg.attention` / `text.on-attention`, `accent.mode`, `state.destructive`, `text.on-selection`.
- `text.subtle`'s only loci are grouped `··· N` counts and pending loading steps, and `bg.subtle`'s is the loading bar's empty track, so both need their specific fixture rather than the flat Sessions screen.
- A gap is closed by **adding a fixture, never by exempting a token** — an exemption is exactly the permanent render-layer carve-out the guard exists to catch (§9.11).
- Assertion 3 is what makes assertion 2's union complete: without it a token absent everywhere is silently absent from both sides and the balance holds.
- Coverage is computed over the **included** (non-colourless) fixtures only, so a token covered solely by an excluded fixture still fails.
- `border` renders as a foreground on rules and frames while `canvas` and the `bg.*` tints render as backgrounds, so the check must look for whichever form the token actually takes or a covered token reads as missing.
- Phase 8's panel fixtures and §11.2's paginating panel fixture inherit this assertion automatically without anyone remembering to add them, which is the point of enumerating rather than naming.

**Context**:
> §13.4, assertion 3: "**Every token is exercised by at least one fixture.** The union in (2) is only complete if every token appears on *some* fixture, and the at-risk ones are the transient states (`bg.attention` / `text.on-attention`, `accent.mode`, `state.destructive`, `text.on-selection`). Making this an assertion of the guard means a token with no fixture **fails the test** and someone adds a fixture, rather than the guard being silently blind at precisely the sites it exists to protect."
> §13.4: "**The guard enumerates the harness's fixture set; it never names fixtures**… It is also what puts §13.3's four new panel fixtures and §11.2's paginating panel fixture under the guard without anyone remembering to add them."
> §13.3: "**A missing fixture is a blind spot the guard structurally cannot report**: §13.4 enumerates whatever fixtures exist, so absence reads as coverage." — the reason this assertion exists at all.
> §9.11: a surface that deliberately ignores the active theme "is precisely the shape the swap-and-diff guard exists to catch, so the alternative would mean carving out the one test protecting against accidental carve-outs" — which is why a coverage gap is closed with a fixture rather than an exemption.
> §2.5's role table gives each token's illustrative loci (`text.subtle` → group `··· N` counts, pending loading steps; `bg.subtle` → loading-bar empty track; `bg.attention` → warning-flash band; `accent.mode` → Sessions header, Preview chrome; `state.destructive` → kill/delete emphasis, `▲`; `text.on-selection` → the selected row's name).
> §9.1: the panel's own surface-token table "also feed[s] §13.4's third assertion (every token exercised by at least one fixture): the panel fixtures are what cover `accent.mode` and `accent.attention` outside their transient main-screen states" — Phase 8, inherited automatically.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §2.5, §9.11, §13.3, §13.4

## theming-system-4-5

### Task 4.5: `RestoreTerminalBackground`'s two divergence cases through the real swap path

**Problem**: §11.4 is "the one path where a mistake re-sticks a colour in the user's terminal **after Portal exits**", and §13.4's guard "structurally cannot cover it" — the guard scans rendered fixture output, and this is an OSC 11 write that happens after the last render. Phase 3 task 3-3 delivered the anchor (`startupCanvasHex`, captured at gate resolution) and proved it by mutating `activeTheme` directly, because no mid-session swap existed yet. The two cases §11.4 actually names are **divergences** — the active theme differing from the startup one — and neither was reachable: a theme **committed mid-session**, and **quit with an uncommitted preview active** (`Ctrl-C` with the panel open), the second being "the likelier mistake of the two, and the only path on which a colour the user never chose can be left stuck in their terminal after Portal exits". Task 4-2 has now made both reachable through the production swap path.

**Solution**: the named `RestoreTerminalBackground` verification §11.4 requires — a direct unit test, driven without fixtures, that swaps the theme through task 4-2's production entry point and asserts the exit-time comparison stays anchored to the retained startup canvas hex across both divergence cases.

**Outcome**: a mid-session commit or an abandoned preview can never make Portal skip (or emit) the wrong set-back, and the invariant is pinned by a test that drives the same path Phases 8–9 will drive for real.

**Do**:
- **Land the test** as `internal/tui/restore_divergence_test.go`, package `tui` (an in-package test — it reads the unexported `startupCanvasHex` and drives the unexported gate). Extend, do not replace, task 3-3's `TestRestoreTerminalBackground_AnchoredToStartupHex`: that one proved the anchor by direct mutation; this one proves the divergence through `ApplyTheme`.
- **Case 1 — a theme committed mid-session.** Build a model on a **constant** nomination of `tokyo-night` so the gate resolves at construction and `startupCanvasHex` is `#0B0C14`; render once; then `ApplyTheme(tokyo-night-day)` (canvas `#E1E2E7`). Assert: a captured original of `#0b0c14` (the startup canvas echoed back) **skips** the set-back; a captured original of `#E1E2E7` — the *active* theme's canvas, which is exactly what a naive implementation would compare against — **emits** it; and `startupCanvasHex` is unchanged.
- **Case 2 — quit with an uncommitted preview active.** Model the panel-open arrow-preview as a sequence of swaps with **no** commit: `ApplyTheme` across three themes ending on `nord` (canvas `#2E3440`), then run the exit path. Assert: `startupCanvasHex` is still `#0B0C14` after all three; a captured original of `#2E3440` (the previewed theme's canvas — a colour the user never chose) **emits** the set-back; a captured original of `#0b0c14` still **skips**. Name the case in the test so its intent survives Phase 9 wiring the real `Ctrl-C`/close paths onto it.
- **Pin the killer case explicitly**: a swap to a theme whose canvas happens to equal the terminal's **genuine** original must still emit the set-back, because the comparison is anchored to the startup canvas and not the active one. This is the single assertion that distinguishes a correct implementation from the naive one, so give it its own named test rather than folding it into a table.
- **Keep the echo-guard shapes covered** through the swap path: `#0b0c14`, `#0B0C14`, `#0b0c14ff` and `0b0c14` all skip against a canonicalised `#0B0C14` (`sameHexColour` lower-cases both sides, strips a leading `#` and tolerates a trailing alpha pair).
- **Keep the fall-through cases covered**: a non-hex `rgb:0b0b/0c0c/1414` reply still falls through and emits the set-back; an empty capture (`OriginalBackground() == ""`) still writes nothing.
- **Drive every swap through `ApplyTheme`, never by assigning `activeTheme`.** Phase 3-3's direct mutation proved the anchor; this task proves the divergence, and using the production path is what makes it prove anything about Phases 8–9. Do not touch `startupCanvasHex` from the test either — it must be produced by construction/gate resolution, so a regression that starts writing it on swap is caught here.
- **Add no `NO_COLOR` case.** Under `NO_COLOR` the hex is captured as normal from the selected member, but no canvas is painted and no OSC 11 set is issued, so the set-back is a no-op — record that in a comment rather than adding a test that asserts nothing.

**Acceptance Criteria**:
- [ ] Both divergence cases are covered by named tests: a theme **committed mid-session** and **quit with an uncommitted preview active**.
- [ ] Every swap in the test goes through task 4-2's `ApplyTheme`; no test assigns `activeTheme` or `startupCanvasHex` directly.
- [ ] `startupCanvasHex` is captured once at gate resolution and is unchanged after one swap and after a preview run of three.
- [ ] With startup `tokyo-night` and active `tokyo-night-day`: original `#0b0c14` → **no write**; original `#E1E2E7` → **set-back written**.
- [ ] With startup `tokyo-night` and active `nord`: original `#2E3440` → **set-back written** (a colour the user never chose is never left stuck).
- [ ] The echo guard still skips for `#0b0c14`, `#0B0C14`, `#0b0c14ff` and `0b0c14` after any number of swaps.
- [ ] A non-hex (`rgb:`) reply still emits the set-back; an empty capture still writes nothing.
- [ ] `RestoreTerminalBackground`'s body still reads only `m.startupCanvasHex` — task 3-3's source guard passes unchanged.
- [ ] No panel, no commit path and no prefs write is introduced — the preview case is modelled as a swap with no commit.
- [ ] `go test ./...` green; lane unit; no `t.Parallel()`.

**Tests**:
- `"it anchors to the startup canvas after a mid-session commit"` — `TestRestoreBackground_CommittedThemeDivergence`
- `"it anchors to the startup canvas after an abandoned preview"` — `TestRestoreBackground_UncommittedPreviewDivergence` (three swaps, no commit)
- `"it still sets back when the active theme's canvas equals the terminal's original"` — `TestRestoreBackground_ActiveCanvasEqualsOriginalStillSetsBack` (the naive-implementation trap)
- `"it never moves the startup hex across swaps"` — `TestRestoreBackground_StartupHexSurvivesSwaps`
- `"it skips the set-back on every canvas-echo shape"` — `TestRestoreBackground_EchoGuardShapesAfterSwap` (table: exact, uppercase, trailing alpha, no leading `#`)
- `"it still sets back for a non-hex reply"` — `TestRestoreBackground_NonHexReplyAfterSwapStillSetsBack`
- `"it writes nothing without a captured original"` — `TestRestoreTerminalBackground_EmptyWritesNothing` (existing, kept)
- `"it keeps no theme reference in the restore path"` — `TestRestorePath_ReadsNoTheme` (task 3-3's guard, re-run)

**Edge Cases**:
- Both divergence cases are required — a theme **committed mid-session** (the persisted theme differs from the startup one) and **quit with an uncommitted preview active** (`Ctrl-C` with the panel open), the second being the likelier mistake and the only path on which a colour the user never chose can be left stuck in their terminal after Portal exits.
- The swap is driven through task 4-2's **production entry point**, not by mutating `activeTheme` — Phase 3-3's direct mutation proved the anchor, this proves the divergence.
- `startupCanvasHex` is captured once at gate resolution and must not move across any number of swaps.
- The echo guard still skips the set-back when the terminal echoed the startup canvas back, case-insensitively across `#0b0c14` / `#0B0C14` / trailing-alpha / no-leading-`#`.
- A swap to a theme whose canvas equals the terminal's genuine original must **still** emit the set-back, because the comparison is anchored to the startup canvas and not the active one.
- A non-hex (`rgb:`) reply still falls through and emits the set-back, and an empty capture still writes nothing.
- Under `NO_COLOR` the hex is defined but no canvas was painted and no OSC 11 set is issued, so the set-back is a no-op and no case is needed.
- §13.4's guard structurally cannot cover this — it scans rendered fixture output and this is an OSC 11 write after the last render — which is why §11.4 gives it its own named verification.
- The panel does not exist until Phase 8, so "uncommitted preview" is modelled as a swap with no commit, and Phase 9 wires the real close/commit paths onto an anchor that must already hold.

**Context**:
> §11.4: "**Capture and retain the startup canvas hex as model state**, and anchor `RestoreTerminalBackground`'s comparison to it… This is the mechanic carrying an explicit *'do **not** drop this guard'* warning, and the swap-and-diff guard structurally cannot cover it (it scans rendered fixture output, and this is an exit-time OSC 11 write). **It therefore needs its own named verification** — a direct unit test on `RestoreTerminalBackground`, driven without fixtures, asserting it compares against the retained startup canvas and not the active theme's, **across both divergence cases**: **A theme committed mid-session** — the persisted theme differs from the startup one. **Quit with an uncommitted preview active** — `Ctrl-C` with the panel open (§9.7). The model's active theme is the *previewed* one, which the user never persisted and which a naive implementation would compare against. This is the likelier mistake of the two, and the only path on which a colour the user never chose can be left stuck in their terminal after Portal exits. The stakes are why: this is the one path where a mistake re-sticks a colour in the user's terminal **after Portal exits**."
> §11.3: "**The echo guard needs no new race handling.** It exists because the startup OSC 11 *query reply* can race Portal's own canvas set. The query is issued once from `Init`; a later theme switch issues no new query, so it creates no new race. The guard only ever needs to compare against the canvas active during the *startup* window."
> §9.13: "**`Ctrl-C` with a failure outstanding is accepted as an undelivered report.** It is the one exit §9.7 keeps live inside the panel" — the exit that makes the uncommitted-preview case reachable in the first place.
> §9.8: a forced close "takes the `Esc` path exactly… That is also the state §11.4 names as the one where a colour the user never chose can survive Portal's exit."
> §9.10: under `NO_COLOR` "**The startup canvas hex is captured as normal** from the selected member, so `RestoreTerminalBackground` has a defined comparison value — but no canvas is painted and no OSC 11 set is issued, so there is nothing to restore and the set-back is a no-op. The §11.4 anchor test does not need a `NO_COLOR` case: the value is defined and unused."
> Canvas values for the assertions (uppercase-canonical per §4.3): `tokyo-night` `#0B0C14`, `tokyo-night-day` `#E1E2E7`, `nord` `#2E3440`.
> Phase boundary: Phase 8 builds the panel and its arrow-preview; Phase 9 wires `Esc` / forced close / commit. Both land on the anchor this task pins, which must already hold before either is written.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.4, §9.8, §9.10, §11.3, §11.4, §13.4
