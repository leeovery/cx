## Attempt 1

ISSUES:

- `internal/tui/theme_panel.go:120` + `internal/tui/theme_panel_open_test.go:396-423` — **the retention of `themePanel.enumeration` has zero coverage.** Proved by mutation: removing `enumeration: enumeration` from `armThemePanel` passes the entire unit lane (`go test ./...` clean). The discard test only asserts the zero value *after* close, which a never-populated field satisfies trivially, and the field's `//nolint:unused` suppresses the one automated signal that would otherwise notice. This is a Do-list item ("retain **both** returned values") whose regression is silent and consequential: task 8-8's open-time re-resolution and §9.2's commit recompute both call `Reassemble(enumeration, keys)`, so a dropped enumeration would silently produce a union of built-ins only — every drop-in row vanishing from the list after a commit, with nothing failing.

  FIX: add the paired positive assertion. Cheapest correct placement is `TestThemePanelOpen_BoundOnBothPages` (`theme_panel_open_test.go:225-243`), where `recordingThemeEnumerator` already returns `theme.Enumeration{DirPath: "/fixture/themes"}` — assert `m.themePanel.enumeration.DirPath == "/fixture/themes"` alongside the existing union-count assertion.

  ALTERNATIVE: assert it in `TestThemePanelOpen_EnumerationDiscardedOnClose` instead, seeding one drop-in into `dir` **before** the first open and asserting `len(m.themePanel.enumeration.Entries) == 1 && m.themePanel.enumeration.DirPath == dir` at line 402. Stronger (it pins the retained *parse*, which is what task 8-9 arrows off, not just the path) at the cost of restructuring that test's fixture. Recommended if the parse content is what 8-8 will consume; otherwise the first is sufficient.

  CONFIDENCE: high

- `cmd/open_theme_nomination_test.go:58-60` — `newThemeEnumerator` was added to the `allowed` map, but it is **not needed there**: `themeCallSites` records nothing from `theme_enumerator.go` (the methods call `e.loader.Open`, a selector whose `X` is not the `theme` ident, and `newThemeEnumerator` calls only the untracked `themesDirPath`). The `local` entry at line 211 is what actually puts the call under the guard, and it works. Widening `allowed` matches names in *every* file, which the guard's own comment at lines 219-222 explicitly warns against; it also makes the guard blind to a future `l.Enumerate(...)` added inside `newThemeEnumerator`, which is precisely the construction-time sweep §5.7 forbids.

  FIX: drop the `"newThemeEnumerator": true` entry from `allowed` (keep the one in `local`) and re-run `go test ./cmd -run TestOpenExecPath_DoesNoThemeWork` to confirm it still passes.

  CONFIDENCE: medium

NOTES:

MUTATION EVIDENCE (reviewer-run via `go test -overlay` against scratchpad copies; working tree unchanged):

| Mutation | Result |
|---|---|
| remove the pagination-dot theming | **caught** — fails on dark, light AND colourless; fixture genuinely paginates (3 dots at 20 rows / height 16) |
| replace `liveThemeEnumerator` with `e == nil` | **caught** — the typed-nil subtest panics on the keypress; both nil shapes genuinely safe |
| hoist `t` above either `SettingFilter()` guard | **caught** — carve-out test fails on both pages |
| delete the panel input arm | **caught** — swallow test fails on `k`/`x`/`m`/`Esc` |
| partial close (`open = false` only) | **caught** — discard test fails on all three retained values |
| remove `enumeration: enumeration` from `armThemePanel` | **SURVIVES — the entire unit lane stays green** (see ISSUE 1) |

- **(a) The `themeNomination` → `themeResolution` rename and shared loader check out.** `themeResolution` is the identical evaluation, now returning `Resolution` + `RawKeys`; the nomination handed to the model is `resolution.Nomination`, and the error path still returns before constructing anything. The "one dedup scope per launch" claim is genuine and verified structurally: `theme.Loader.events` is a `*EventLogger` (`internal/theme/load.go:57`), so sharing the `Loader` *value* shares one dedup set. `openTUI` builds it once (`cmd/open.go:990`) and hands the same value to both `themeResolution` and `newThemeEnumerator`. `TestThemeEnumerator_SharesTheConstructionReadsDedupScope` proves it with a real negative control (a second loader emits 2).
- **(b) The typed-nil guard is correct** and task-mandated; nil-able kinds are enumerated so a struct-value seam (the production shape) cannot panic on `IsNil`. `reflect.Interface` in the kind list is unreachable via `reflect.ValueOf` on an interface variable; harmless.
- **(c) The pagination-dot fix is real and verified with a paginating fixture** — the `bubbles` greys `38;2;151;151;151` / `38;2;60;60;60` are asserted absent, which is the right shape given §13.4 is structurally blind to an unchanged value.
- **(d) The negative assertions were verified independently rather than trusted.** Construction-time enumeration would surface as a non-zero `opens` on the real-loader/real-directory seam **and** would break Phase 5's exact-event-list assertions (`assertThemeEvents(t, sink, "INFO loaded slug=drop-alone")` at `cmd/open_theme_construction_test.go:301`, run against a populated `PORTAL_THEMES_DIR`). The exec-path claim holds by both the source guard and the poisoned-directory runtime guard (`assertThemeEvents(t, sink)` with no `want` asserts zero records).
- **SPEC_CONFORMANCE conformant** across §8.4's constructor slots (threaded through `Build` at `build.go:54,98,110`, mapped field-for-field in `buildTUIModel` at `cmd/open.go:851-853`, `Setting` derived not injected), §5.7/§5.8 (the only `Open` call is on the keypress at `theme_panel.go:190`; `newThemeEnumerator` resolves the path and reads nothing), §8.4's prefs asymmetry (commented on both halves at `model.go:262-273`, cross-referenced from `openThemePanel`), §9.6's filter carve-out (`model.go:3816`, `model.go:3014`), and §9.7's key-exclusivity (the panel arm at `model.go:2903` intercepts only `tea.KeyPressMsg`, which is correct — mouse is never enabled, and non-key messages must keep reaching the live list behind the panel).
- **ARCHITECTURE sound**: the enumerator adapter closes over loader + directory exactly like the `ScrollbackReader`/`stateDir` precedent, adds no policy, and keeps union assembly in `internal/theme` so `internal/tui` does not become a fourth `theme`-component emitter. `armThemePanel`'s ordering (width → list → styles) is correct and documented. Concrete types throughout; no `any` escape hatches.
- **CONVENTIONS followed**: no `t.Parallel()`, table-driven subtests with `name` fields, unit-lane-clean (no daemons/binaries), `(&m).mutate()` on a value receiver matching the existing `clearAbortBanner`/`resetBurstState` idiom, seam-shaped 2-method interface, `log.For` binding untouched (`TestThemeComponent_BoundOnceInCmd` still green). `golangci-lint run ./internal/tui/... ./cmd/...` → 0 issues; unit lane and `go vet -tags integration ./...` clean.
- **Test quality is otherwise precise**, with real negative controls — `TestThemePanelOpen_SwallowsPageKeys` proves each key reaches its effect with the panel *closed* before asserting the swallow.
- **Non-blocking, for tasks 8-9/8-11**: `applyThemePanelListStyles` sizes the model's list to `themePanelMinBodyRows` (1) while `renderThemePanel` sizes a *copy* to the real body height. Cursor rendering is unaffected (`bubbles`' `updatePagination` restores `Page` from the index, so the render copy always paints the page the cursor is on), but the model's list carries `PerPage == 1`, so `Paginator.NextPage()`/`PrevPage()` will move by one row rather than one page when §12.2's `Ctrl+↑/↓` is wired. Worth handling when paging lands; `SetWidth` would not have avoided it (it delegates to `SetSize(v, m.height)` with `height == 0`, giving the same `PerPage`).
- `TestThemePanelOpen_ExecPathUntouched` (`cmd/theme_enumerator_test.go:232`) is a near-verbatim copy of `TestOpenExecPath_DoesNoThemeWork/an exec-path open emits no theme record` — same poison, same prefs seed, same `execOpenSession`, same zero-record assertion — differing only in which "loud" fixture proves non-vacuity. The task named the test, so this is defensible, but it is duplicate runtime cost on the same claim.
- `TestThemePanelOpen_WiredThroughBuildTUIModel` asserts the composed frame by substring (`"Themes"`, `"sunset"`, `"●"`). Exact comparison is impractical for a full ANSI frame and row composition is pinned by 8-4's tests, so this is fine as a wiring assertion — noted only because it is the one containment-style assertion in the new set.

## Attempt 2

ISSUES:

- `cmd/theme_enumerator_test.go:151-159` (`TestOpenTUI_BuildsOneThemeLoader`) — **the guard's stated claim exceeds its assertion.** Its own doc comment says "openTUI must construct the loader ONCE **and hand that instance to both**", but it only asserts the first half: it counts calls to `buildThemeLoader`/`newThemeLoader` and never looks at what `newThemeEnumerator` is handed. Proved unguarded — rewriting `cmd/open.go:1049` to `themeEnumerator: newThemeEnumerator(newThemeLoader())` passes the **entire** `./cmd` suite.

  That is the Do-list's "closing over the **same** `theme.Loader`" and §5.5's "one loader per launch is one dedup scope per launch"; under the mutation a user with an unusable themes directory gets every `theme` WARN twice per launch. Production code is correct — this is a guard-completeness gap of the same class as round 1's issue 2 (a guard whose stated claim exceeds its assertion), and it is **the only surviving mutation found across the whole task**.

  FIX: add an AST assertion that every `newThemeEnumerator` call inside `openTUI` is handed an already-bound identifier rather than a fresh construction, and call it from `TestOpenTUI_BuildsOneThemeLoader` beside the existing count loop:

  ```go
  func assertSharedLoaderArg(t *testing.T, n ast.Node) {
      t.Helper()
      ast.Inspect(n, func(node ast.Node) bool {
          call, ok := node.(*ast.CallExpr)
          if !ok { return true }
          ident, isIdent := call.Fun.(*ast.Ident)
          if !isIdent || ident.Name != "newThemeEnumerator" || len(call.Args) != 1 { return true }
          if _, isIdent := call.Args[0].(*ast.Ident); !isIdent {
              t.Errorf("openTUI hands newThemeEnumerator a freshly-constructed loader; it must share the construction-time instance (§5.5)")
          }
          return true
      })
  }
  ```

  Validated in a scratchpad copy: it passes on the current wiring and fails on **both** regression shapes — `newThemeEnumerator(newThemeLoader())` and `newThemeEnumerator(theme.NewLoader(theme.NewEventLogger(log.Discard())))` (the latter also silently drops the real `theme` component logger this task's own acceptance criterion requires).

  ALTERNATIVE: sum the two constructor counts instead of checking each — `total := callCount(fn,"buildThemeLoader") + callCount(fn,"newThemeLoader"); if total > 1 {…}`. One line, also validated (fails on the mutation, passes on the real wiring), but blind to an inline `theme.NewLoader(...)` argument since `callCount` matches `*ast.Ident` only. The arg-ident check is recommended: it is the direct expression of the claim the comment already makes, and it covers both shapes.

  CONFIDENCE: high

NOTES:

MUTATION EVIDENCE (reviewer-run, scratchpad copies / `-overlay`; working tree byte-identical before and after):

| Mutation | Result |
|---|---|
| drop `enumeration: enumeration` from `armThemePanel` | **caught** — 3 assertions, 2 tests (round 1's survivor is closed) |
| drop `union: union` | caught — 6 tests |
| drop `badges: theme.Badges(m.themeSlots)` | caught |
| drop `applyThemePanelListStyles()` (dot theming) | caught — dark, light **and** colourless |
| `liveThemeEnumerator(e)` → `e == nil` | caught — typed-nil subtest panics |
| hoist `t` above the Sessions `SettingFilter()` guard | caught |
| hoist `t` above the Projects `SettingFilter()` guard | caught |
| partial close (`open = false` only) | caught |
| delete the panel input arm in `Update` | caught — `k`/`x`/`m`/`Esc` |
| inject `theme.BuiltinSlugs()` into `newThemeEnumerator` | **caught** (was blind before the fix — counterfactual re-confirmed: restoring the `allowed` entry makes it pass again) |
| `newThemeEnumerator(newThemeLoader())` in `openTUI` | **SURVIVES the whole suite** (see ISSUE) |

- **Round 1's two findings are genuinely closed**, verified independently rather than trusted. The retention mutation now fails 3 assertions across `TestThemePanelOpen_BoundOnBothPages` (both pages, via the shared `fixtureThemesDir` const) and `TestThemePanelOpen_EnumerationDiscardedOnClose` (entries + path). The seeded `aurora.theme` genuinely yields `len(Entries)==1` because `theme.Enumeration.Entries` holds **directory candidates only** — built-ins are not in it (`internal/theme/union.go:149`) — so the assertion is a real parse assertion, not an artefact of the embedded set. The `allowed`-map removal makes the exec-path guard bite on a theme call inside `newThemeEnumerator`, counterfactual confirmed.
- **The fix round touched no production code**, as claimed. Every construct round 1 cited by line still resolves to the same line: `theme_panel.go:120` (the nolint), `:190` (the sole `Open`), `model.go:2903` (the panel arm), `cmd/open.go:851-853`, `:990`, `build.go:54,98,110`.
- `internal/tui/theme_panel.go:120` — the `//nolint:unused` on `enumeration` is now **stale**. Verified: with the directive removed, `golangci-lint run ./internal/tui/...` reports 0 issues, because the new test assertions count as use. Since `nolintlint` is not enabled it costs nothing today, but it is the same live suppression that hid round 1's gap, so the linter would go silent again if those assertions were ever deleted. Worth dropping when task 8-8 lands its readers.
- **SPEC_CONFORMANCE conformant.** §8.4's three constructor slots threaded field-for-field (`build.go:54,98,110` → `Build` at `:245-251` → `cmd/open.go:851-853`), `Setting` derived via `theme.ResolveSetting` rather than injected, `themeResolution` returning nomination + per-slot record + raw keys from one evaluation. §5.7 holds — the only `Open` call is on the keypress (`theme_panel.go:190`); `newThemeEnumerator` resolves a path (`themesDirPath` is pure, no I/O) and reads nothing. §5.8's per-open re-read and lifetime retention/discard implemented and now positively asserted. §8.4's prefs asymmetry commented on both halves (`model.go:262-273`). §9.6's carve-out sits below both `SettingFilter()` guards (`model.go:3015-3018`, `:3813-3816`). §9.7's key-exclusivity intercepts only `tea.KeyPressMsg` — correct, since no `KeyReleaseMsg` handling exists anywhere in the tree and non-key messages must keep reaching the live list beneath. §12.3's real `theme` logger is on this path via the shared `themeLoader`.
- **ARCHITECTURE sound.** The adapter closes over loader + directory exactly like the `ScrollbackReader`/`stateDir` precedent, adds no policy, and keeps union assembly in `internal/theme` so `internal/tui` does not become a fourth `theme`-component emitter. `armThemePanel`'s width → list → styles ordering is correct and documented. `closeThemePanel` zeroes the whole struct, which is why the partial-close mutation is caught. Concrete types throughout; no `any` escape hatches.
- **CONVENTIONS followed.** No `t.Parallel()`; table-driven subtests with real negative controls (`TestThemePanelOpen_SwallowsPageKeys` proves each key reaches its effect with the panel *closed* first); `(&m).mutate()` on a value receiver matching the `clearAbortBanner`/`resetBurstState` idiom; 2-method seam with a compile-time `var _ tui.ThemeEnumerator = themeEnumerator{}`; the typed-nil-in-interface trap handled exactly as `golang-safety` prescribes, with nil-able kinds enumerated so a struct-value seam cannot panic on `IsNil`; one `log.For("theme")` binding. `go vet ./...`, the full unit lane and `golangci-lint run` all clean.
- `applyThemePanelListStyles` sizes the model's list to `themePanelMinBodyRows` (1), so the model's list carries `PerPage == 1` while `renderThemePanel` sizes a per-frame copy. Rendering is unaffected — confirmed `bubbles/v2@v2.1.0` `list.updatePagination` restores `Paginator.Page` from `Index()` — but `Paginator.NextPage()`/`PrevPage()` will move by one row when §12.2's `Ctrl+↑/↓` is wired in 8-11. Carried from round 1; still non-blocking.
- `t` is dispatched on both pages but not yet in `sessionsKeymap()`/`projectsKeymap()`. `keymap_dispatch_guard_test`'s direction-2 is probe-driven, so it does not notice — correctly, since the descriptor entry belongs to the later footer-revision and panel-scope-guard tasks.
- §9.6's "not bound on Preview/Loading" holds structurally (the binding exists only in `updateSessionList`/`updateProjectsPage`) but has no test; likewise `t` while the panel is open is swallowed by `updateThemePanel`'s default arm with no re-enumeration. Both are task 8-13's entry-condition scope.
- `TestThemePanelOpen_ExecPathUntouched` (`cmd/theme_enumerator_test.go:232`) is near-verbatim duplicate runtime cost against `TestOpenExecPath_DoesNoThemeWork/an exec-path open emits no theme record`. The task named the test, so defensible.
