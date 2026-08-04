## Attempt 1

ISSUES:

- `internal/tui/theme_panel.go:691` — **the reversal counter-example is off by one.** The comment states: on `[V,V,I,I]` at `PerPage=2`, `Ctrl+↓` from index 0 "lands on 3, reverses, and settles on 1". Measured against the real fixture (`newArrowPanelModelAt(..., arrowPagingTermH)`, `PerPage=2`), the unskipped `Ctrl+↓` lands on **2** — the first row of page 1. Index 3 is where the *skip walk* reaches (2 → 3 → boundary → 2 → 1) before turning round. The settle (1) and the conclusion ("one row FORWARD of the start") are both correct; only the landing index is wrong.

  This matters because it is **the sentence the carried review note was raised about**, and the replacement re-states a false step of the same class.

  FIX: change "lands on 3" to "lands on 2" and make the walk explicit, e.g. "`Ctrl+↓` from index 0 lands on 2, walks down to 3, reverses, and settles on 1, one row FORWARD of the start."

  While there, discharge the same claim's **surviving twins**: `internal/tui/theme_panel_arrow_test.go:209` ("A reversal necessarily ends where it began…") and `:450` ("A REVERSAL NECESSARILY ENDS WHERE IT BEGAN") both state the generalisation the production comment now explicitly contradicts. `:209` is the more important one — it documents `requireArrowUnskippedLandingAt`, which the paging test also calls. Scope both to the single-row step they are actually true of.

  CONFIDENCE: high

NOTES:

MUTATION EVIDENCE (reviewer-run in an isolated copy; the working tree was not touched):

| Mutation | Caught by |
|---|---|
| A — bare discard, no re-resolution (the provisional body) | `DiscardsThePreview`, `ResolvesEditedValues`, `ResolvesToFallback`, `ReadsNothing` |
| B1 — snapshot taken **before** the open's resolution, restored on close (the true naive snapshot) | `DiscardsThePreview`, `ResolvesEditedValues`, `ResolvesToFallback` |
| B2 — snapshot taken **after** the open's resolution, restored on close | `DiscardsThePreview` **only** (the `Resolve` call-count) |
| C — discard before resolve (order inverted) | `ResolvesEditedValues`, `ReadsNothing`, `EventCadence` |
| D — partial clear instead of zeroing | `EnumerationDiscarded`, `ForcedCloseUsesTheSameFunction` |
| E — `Esc` closes but leaks to the page beneath | `DiscardsThePreview`, `WritesNothing`, `DoesNotClearTheFilter`, `NestsOverMultiSelect`, `EscDoesNotQuit` |
| F1/F2 — panel pass emits `loaded` / fallback dedup removed | `EventCadence` (20 `loaded`, 20 `fallback applied` respectively) |

- **(a) The executor under-sold its own tests.** The *naive* snapshot (B1 — "the palette held when the panel opened") **is** caught behaviourally by both mid-session-edit tests. Only the subtler B2 variant survives everything except the call count, so the structural assertion is genuine belt-and-braces rather than the sole load-bearer.
- **(b) The `applyInForceTheme` extraction is the right call**, and 8-8's open behaviour is preserved exactly. Badge assignment moved after `ApplyTheme`, but `applyThemePanelCanvasMode` early-returns while `themePanel.open` is false (which it is throughout `armThemePanel`), so the two orderings are indistinguishable; the degrade policy returns `""` on both the error and the no-slot path with badges/cursor/theme untouched, identical to before. The close inherits nothing open-specific — no badge refresh, no cursor anchoring. The whole cursor/open/arrow suite stays green.
- **(c) The event-cadence claim verified against §12.3 directly**, not against the task summary. The panel path routes `ResolveNominationFrom` → `enumerationPass` → `reportFallback`, which never calls `events.Loaded`; `FallbackApplied` dedups on `{event, slug, reason}`. Both halves of the 10-cycle test bite. The mutation output (`20 fallback applied`) independently confirms the close really re-resolves — 2 per cycle.
- **(d) Negatives verified.** The page-layout mutation (reclaim on close) fails `PageLayoutUnchangedAcrossOpenAndClose` on both pages. `gofmt`/`go vet`/`golangci-lint` clean, full unit lane green.
- **"No tmux option set" has no direct observation, and the reachability argument is sound.** `internal/tui`'s only tmux-option writer is the burst ack channel (`m.ackChannel`, `burst_progress.go:172/201`), and `closeThemePanel` reaches only the enumerator seam's `Resolve` and `ApplyTheme` (which fans to leaf-style re-points, no I/O). Recorded in the test doc. Accepted. If closing it cheaply later is wanted, `WithAckChannel` accepts a fake and a zero-call assertion would give the third seam a direct observation — but nothing in the close path can reach it today, so this is optional.
- **TEST_COVERAGE adequate** — all 12 named tests present, every one demonstrated to bite under mutation. Positive controls are real: the filter/multi-select/quit cases each re-press `Esc` on the closed picker and require the opposite outcome; `WritesNothing` proves the persister seam is live via an `s` press; `EventCadence` proves the sink is wired via `enumerated` and that `loaded` is reachable through the same loader.
- **CONVENTIONS followed** — no `t.Parallel()` (stated in the file header per CLAUDE.md), per-concern panel test file matching the established `theme_panel_{open,cursor,arrow}_test.go` split, named subtests, seam fakes over concrete types, AST source guard consistent with the package's existing guard idiom (`parsePackageFilesByName` scans production sources only, so a test cannot trip it).
- `applyInForceTheme` dereferences `m.themeEnumerator` with **no nil guard**. Structurally safe today (`open` is only set by `armThemePanel`, which runs behind `openThemePanel`'s nil guard) and the comment says so. Worth remembering when 8-11's forced close and Phase 9's hooks attach: a caller that invokes `closeThemePanel` on an already-closed panel in a fixture with no enumerator wired would panic. Not a change to make now.
- `TestPanelClose_EventCadence` deliberately gives construction a silent loader and the panel a sink-backed one, so the "exactly 1" is the panel path's own emission rather than a delta against construction's. That is a stronger statement than the production shape (where construction emits the 1 and every open/close dedups), and the fixture doc says why. Good call.
- Comment-only edits to `internal/theme/resolution.go` and `resolution_test.go` de-stale the "task 8-10" forward references; both are accurate against the shipped behaviour.
- **Orchestrator note, resolved:** the reviewer flagged a stray 56 KB `package cmd` file named `open_leak.go` seen at the repo root mid-review. Verified after the review: it does not exist, `git status` is clean of it, and `go build ./...` succeeds. Nothing to do.

## Attempt 2

ISSUES:

- `internal/tui/theme_panel_open_test.go:165` — the helper **the entire close suite drives `Esc` through** is still documented as *"drives the **provisional** `Esc` close through Update"*. The close stopped being provisional in this task. This is the same class of stale forward reference the executor de-staled in `resolution.go`/`resolution_test.go`, missed by the sweep — the **fourth** instance of the comment-accuracy defect this task has been blocked on twice.

  FIX: reword to name the real path, e.g. `// closeThemePanelForTest drives the Esc close (closeThemePanel) through Update.` Comment-only, one line.

  CONFIDENCE: high

- `internal/tui/theme_panel_cursor_test.go:439-441` — **a forward-looking design claim that contradicts the spec.** The sentence added in this fix round asserts the policy "is stated once on `applyInForceTheme`, **the shared body all three route through**", where the third is task 9-2's commit recompute.

  `applyInForceTheme` *applies* the in-force theme through `ApplyTheme`. §11.1 is explicit that "**A *commit* is not a caller**: it recomputes rows and badges, never the rendered theme", and §9.2 that "committing to a non-active slot changes nothing on screen" — routing the recompute through this body would make `d`/`l` on the non-active slot **swap the screen off the user's preview**. `ApplyTheme`'s own doc (`model.go:1514-1521`) lists exactly three callers and a commit is not among them.

  The claim could steer the Phase 9 executor straight into the violation.

  FIX: scope the sentence to what is true — the *policy* is stated once on `applyInForceTheme` and governs all three `Resolve` call sites, while only the open and the close route through that body (a commit recomputes rows and badges without applying). While there, consider the same scoping on `theme_panel.go:256-260`, whose "IT IS SHARED RATHER THAN RESTATED at each site" sits directly beside the same three-site list and reads the same way.

  ALTERNATIVE: drop the appositive entirely ("It is stated once on `applyInForceTheme`.") — smaller edit, loses the useful note about *which* callers share the body. The scoped version is recommended.

  CONFIDENCE: medium

NOTES:

VERIFICATION METHOD (reviewer-run in a throwaway copy under the scratchpad; the working tree was never touched and `git status` is byte-identical):

- **Skip-loop counter-example** (`theme_panel.go:691`) measured on the real fixture: `PerPage=2`, start index 0 → unskipped `Ctrl+↓` lands on **2** (`ccc`, unselectable) → skip walk **`[2 3 2 1]`** → production settles on **1**, one row forward of the start. The comment now says exactly that.
- **`previewSelectedThemeRow`'s new page-reversal claim** measured: the same reversal moves `#AA8ED6 → #BB8ED6`, i.e. it *does* restyle. True.
- **Mutation battery** (8 mutations): drop the re-resolution; invert the order (discard first); partial clear; schedule a `tea.Cmd` on close; `Esc` closes but leaks to the page beneath; re-layout the page on close; add a second discard site; make the panel pass emit `theme: loaded`. Every one of the 12 named tests bites under at least one; nothing is vacuous.
- `go test ./...`, `go vet`, `golangci-lint run` on both packages: clean.

- **Everything the fix round was briefed on is discharged and measured-correct**: the corrected landing index and explicit walk (`theme_panel.go:684-694`), both test-side twins scoped to the single-row step (`theme_panel_arrow_test.go:210-214`, `:453-457`), and the self-found third site (`theme_panel.go:730-737`). The untouched claims were re-verified too: the IT LOOPS bullet, the 2×N bound, `skipHeaderRow`'s "flips only at index 0" (`model.go:2159`), the `Resolve`-only-error-is-§7.6's-fatal claim (`resolution.go:309-322`), `ApplyTheme` idempotence, and "the seam is always live here" (`WithThemeEnumerator` rejects both nil shapes; `open` is only ever set behind `openThemePanel`'s guard). All true.
- **(c) holds as far as it is checkable**: the tracked-file diffs for `theme_panel_arrow_test.go`, `theme_panel_cursor_test.go`, `resolution.go` and `resolution_test.go` are **comment-only** (every `+` line is a `//` line), and `theme_panel.go`'s non-comment content is the implementation round 1 approved. No assertion or production statement changed.
- **SPEC_CONFORMANCE conformant.** `closeThemePanel` runs §5.8's re-resolution against the retained enumeration, selects the in-force member exactly as the open does (`inForceSlot`, no re-detection), applies through `Model.ApplyTheme`, then discards last. Degrade policy matches 8-8. `ResolveNominationFrom`'s `enumerationPass`/`reportFallback` gives §12.3's cadence (fallback WARN deduped, no `loaded`). No write, one frame, no re-layout.
- **ACCEPTANCE_CRITERIA all met** — each of the 12 verified against code or measurement, not self-report: the byte-compare, the edited/invalidated mid-session cases, the deleted-directory case, the enumeration discard (`opens == 2`), the 10-cycle 1-WARN/0-INFO cadence, filter/multi-select/quit exclusivity, full-struct clear, unchanged layout on both pages, nil `tea.Cmd`.
- **TEST_COVERAGE adequate.** Positive controls are real: `s` proves the mode persister is live; `enumerated`×10 plus a by-name resolution prove the sink records and that `loaded` is reachable through the *same* loader; each exclusivity case re-presses `Esc` on the closed picker and requires the opposite outcome. `panelDiscardSites` scans production sources only (`parsePackageFilesByName` skips `_test.go`), so the structural guard cannot be tripped by a test.
- **ARCHITECTURE sound.** The `applyInForceTheme` extraction is the right seam: one evaluation feeds both the applied theme and the anchored row, the degrade policy is stated once, and the open keeps only what is open-specific (badges, cursor identity). One close path, hook point named for §9.8 and Phase 9.
- `closeThemePanel`'s "already handled by `resyncSessionLayout` (**and its Projects sibling**)" names a function that does not exist — the Projects page has no `resyncProjectLayout`; it reserves via `projectBandHeight` inside `applyProjectListSize` at its own call sites. The conclusion ("closing adds nothing to it") is still correct. Non-blocking imprecision.
- `applyInForceTheme` runs while `themePanel.open` is still `true`, so a close that *changes* the theme re-points the delegate/styles of a list one statement from being discarded. Harmless (the order is load-bearing for the enumeration read), noted only so 8-11 does not "optimise" it by moving the discard up.
- **Nil-seam residue for task 8-11**: a future caller invoking `closeThemePanel` on an already-*closed* panel in a model with no enumerator wired would panic on `Resolve`. The in-source comment names the invariant; worth a guard only if the forced close can fire outside the open state.

## Attempt 3

ISSUES:

- `internal/tui/theme_seams.go:44-46` — **a false in-source pointer created by this task's own refactor.** The `Resolve` godoc still says "see `Model.applyThemePanelResolution`, which states that policy once for all three panel call sites". This task **moved** that policy: the `THE ERROR POLICY` paragraph and the "governs EVERY panel call site" sentence now live on `applyInForceTheme` (`theme_panel.go:256-281`); `applyThemePanelResolution` states only the consequence (empty-slug identity, badges untouched).

  This sits on the godoc of **the interface every panel call site — Phase 9 included — is written against.**

  FIX: change the reference to `Model.applyInForceTheme`, e.g. "…see `Model.applyInForceTheme`, which states that policy once for all three panel call sites." One identifier.

  While there, consider the same rescope on `theme_panel.go:394` ("`applyThemePanelResolution`'s error policy leaves the cursor exactly where it was") — that one is still defensible, since the degrade→empty-string→cursor-untouched mapping genuinely is `applyThemePanelResolution`'s, so change it only if you want the phrase "error policy" reserved to its single site.

  CONFIDENCE: high

NOTES:

VERIFICATION METHOD (all in-place mutations reverted; the working tree is byte-identical to how the reviewer found it — verified against the opening `git status`/`git diff --stat`):

- Read §5.7, §5.8, §6.x, §8.4, §8.5, §9.2, §9.7, §9.8, §9.10–§9.13, §11.1–§11.4, §12.3 directly, and checked each comment claim against the spec text rather than against the task summary.
- **Independent 16-mutation battery** against the live tree: drop the re-resolution; discard-before-resolve; capture-then-discard-then-apply; partial clear; fresh directory read; snapshot-at-open-and-restore (the true naive B1); `Esc` quits; `Esc` schedules a `tea.Cmd`; close clears the filter; close exits multi-select; close re-lays-out both pages; drop the `err` guard; drop the no-slot guard; a second discard site; panel pass emits `theme: loaded`; fallback dedup removed; close refuses to close on degrade. **All 12 named tests bite.** The only surviving mutant is the deliberate equivalence one (dropping the `inForce.Theme != m.activeTheme` skip), which the comment correctly labels "explicitness rather than necessity".
- Re-measured the two claims earlier rounds corrected, from scratch: `[V,V,I,I]` @ `PerPage=2`, `Ctrl+↓` from 0 → **unskipped landing 2**, skip walk `[3 2 1]`, **settles on 1**; and the preview canvas moves `#AA8ED6 → #BB8ED6`, so `previewSelectedThemeRow`'s page-reversal-is-an-ordinary-swap claim is true. Both cases in `TestPanelArrow_SkipReversesAtTheBoundary` do take single-row steps.
- `gofmt`, `go vet`, `golangci-lint run` clean; full unit lane green.

- **(a) No fifth false claim among the reworded comments.** Verified individually: the degrade policy's three-site scope; "THE BODY IS SHARED BY THE OPEN AND THE CLOSE ONLY … a commit recomputes rows and badges and never the rendered theme" (§11.1 verbatim: "A *commit* is not a caller"); "resolves against the retained parse and never the filesystem" (`cmd/theme_enumerator.go:77` takes the enumeration and reads no directory); "the light/dark answer is READ OFF THE MODEL" (`inForceSlot(m.canvasMode)`); "THE SEAM IS ALWAYS LIVE HERE" (`WithThemeEnumerator`'s `liveThemeEnumerator` rejects both nil shapes and `open` is only ever set behind `openThemePanel`'s guard); "a theme already painting the screen is skipped … explicitness rather than necessity" (the equivalence mutant passes the whole lane).
- **(b) The three beyond-brief rewrites are true.** "A Phase 9 commit writes prefs and leaves the panel OPEN" is §9.2's table verbatim (`Enter`/`d`/`l` all "stays open") plus "Commit slots and `Esc` lands on the newly-resolved theme, which is correct". `closeThemePanel`'s FORWARDS bullet, and `theme_panel_close_test.go:25`/`:136`, all now state that correctly — and `:136`'s "a theme snapshotted at open would produce this same frame" is properly scoped to that test, where nothing is edited.
- **(c) Confirmed as far as it is checkable:** every changed line in `theme_panel_arrow_test.go`, `theme_panel_cursor_test.go`, `theme_panel_open_test.go`, `resolution.go` and `resolution_test.go` is a `//` line, and `theme_panel.go`'s non-comment delta is the implementation round 1 approved, re-proved by the battery. No assertion or production statement moved.
- **SPEC_CONFORMANCE conformant.** `closeThemePanel` runs `ResolveSetting` → seam `Resolve` against the RETAINED enumeration → `inForceSlot` off `m.canvasMode` (no re-detection) → `Model.ApplyTheme` → discard last (§5.8, §8.4, §8.8, §9.2). Degrade policy matches 8-8. `enumerationPass`/`reportFallback` gives §12.3's cadence. No write, one frame, no re-layout of the page beneath.
- **ACCEPTANCE_CRITERIA all met** — each verified by code reading or measurement: the byte-compare, edited-but-valid, invalidated→§8.5 fallback, deleted-directory (`opens == 1`), enumeration discard (`opens == 2` + the new file appearing), zero writes on three seams + prefs bytes + directory entries, 10-cycle 1×WARN/0×INFO, filter/multi-select/quit exclusivity on both pages, full-struct clear (list, size, keymap-as-delegate-proxy, badges, message, width), unchanged list geometry across open and close, nil `tea.Cmd`.
- **TEST_COVERAGE adequate.** Positive controls are real: a second `Esc` on the closed picker inverts each exclusivity case; `s` proves the mode persister is live; `enumerated`×10 plus a by-name resolution prove the sink records and that `loaded` is reachable through the same loader. The "still close on a degrade" half is covered incidentally by the stub-backed fixtures (mutating the close to bail on `!ok` fails three tests).
- **ARCHITECTURE sound.** `applyInForceTheme` is the right seam: one evaluation feeds both the applied theme and the anchored row, the degrade policy is stated once, and the open retains only what is open-specific (badges, cursor identity). One close path with a named hook point for §9.8 and Phase 9; the AST guard makes "no second close" structural.
- The file header of `theme_panel_close_test.go:29` still says "Behaviourally the two are indistinguishable in Phase 8 — nothing commits yet". Measured: the naive snapshot-at-open (B1) is caught behaviourally by `TestPanelClose_ResolvesEditedValues` AND `TestPanelClose_ResolvesToFallback`; only the B2 variant (snapshot taken after the open's re-resolution) survives everything but the `Resolve` call count. Round 1 already recorded this as an under-sell rather than an error, and it errs toward modesty, so non-blocking — but the sentence would be truer as "only a snapshot taken after the open's own re-resolution is behaviourally indistinguishable in Phase 8".
- `resolution.go:380-386`'s "Phase 9's recompute … nothing was loaded that construction did not already report" is pre-existing (task 8-8) and out of this task's scope, but **Phase 9 should be careful with it**: §8.4/§12.3 require a `theme: loaded` line for the newly-live opposite slot at commit, and `ResolveNominationFrom` emits none — so that load's emission has to come from the persister, not from the recompute call.
- `closeThemePanel` applies while `themePanel.open` is still true, so a close that *changes* the theme re-points the delegate/styles of a list one statement from being discarded. Harmless, and the order is load-bearing for the enumeration read — noted only so 8-11 does not "optimise" it by hoisting the discard.
- **Nil-seam residue for 8-11/Phase 9**: `applyInForceTheme` dereferences `m.themeEnumerator` with no guard. Safe on every path that exists today (the in-source comment names the invariant), but a forced close invoked outside the open state in a fixture with no enumerator wired would panic.
