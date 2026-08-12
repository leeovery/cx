TASK: theming-system-6-7 — The WithThemePersister Seam And The cmd-Owned Theme Persister (tick-276f58)

ACCEPTANCE CRITERIA (from the plan task):
1. `CommitTheme("nord")` writes `theme` and clears both slots via `SaveTheme`; `CommitThemeSlot("nord", dark)` writes `theme_dark` and clears the constant via `SaveThemeSlot`.
2. A failed write emits exactly one WARN `theme: commit failed` carrying `slug` + `reason`, plus `slot=dark`/`slot=light` for a slot commit and no `slot` attr for a constant commit.
3. The persister returns the error as well as logging it — asserted explicitly.
4. A successful commit emits nothing.
5. The model emits no `theme` record on any path — one emission site, asserted by a source guard over `internal/tui`.
6. `*prefs.Store` does not satisfy `tui.ThemePersister` (compile-time-style assertion in a test).
7. A nil `Deps.ThemePersister` leaves a nil persister, no option applied, no panic when the seam is exercised.
8. The persister is wired only when `prefsStore != nil` (no typed-nil boxed into a non-nil interface).
9. `capturetool` / fixture harness build models with a nil theme persister; a capture writes no `prefs.json`.
10. The seam is exercised by direct call only — no keypress path exists yet.
11. Per-instance persistence, no file watch, no cross-instance sync: last-write-wins with every other field intact.

STATUS: complete

SPEC CONTEXT:
§8.9 (specification.md:904) pins ownership of the panel's commit write to `cmd`, injected through a `WithThemePersister` option "exactly the shape `WithModePersister` already has", for three reasons: `prefs` is a leaf that must not import `internal/log`, the write needs prefs path resolution, and the `theme` component records the failure. The persister is named as *the* emission site for `theme: commit failed`, which otherwise has none, and the multi-package emission of the `theme` component is explicitly declared legal (bind once per package). §8.9 also fixes the merge inside `prefs` behind field-specific savers (`SaveTheme`/`SaveThemeSlot`), with RMW + strict write-path re-read so a malformed file aborts rather than overwrites — the failure mode this task's log/return path reports. §12.3 (line 1491) specifies the event as WARN carrying `slug`, `slot` (absent for a constant) and `reason`. §9.13 requires the error back as a *value* so Phase 9 can render `⚠ couldn't save theme` while keeping the theme applied in memory. §8.9 also states per-instance persistence with no file watch, mirroring `session_list_mode`.

IMPLEMENTATION:
- Status: Implemented (two mechanisms intentionally superseded by later plan tasks — see below; not drift)
- Location:
  - `cmd/theme_persister.go:1-64` — `themePersister{store, logger}`, `newThemePersister`, `CommitTheme`, `CommitThemeSlot`, the shared `reportCommit` emit-and-return helper, `themeSlotAttr`, `prefsSlotFor`; `var _ tui.ThemePersister = themePersister{}` at :22.
  - `internal/tui/model.go:72-79` — the `ThemePersister` seam declared beside `ModePersister` (:68-70) with the ownership/naming rationale in its doc comment; `WithThemePersister` at :531-535.
  - `internal/tui/build.go:32` (`Deps.ThemePersister`) and :123-125 (`if deps.ThemePersister != nil { … }`) — the nil-guarded option append, line-for-line with `ModePersister` at :120-122.
  - `cmd/open.go:27` (single `log.For("theme")` binding for the package), :460 / :529 (config field → `tui.Deps`), :660-663 — wiring inside the existing `if prefsStore != nil` block beside `cfg.modePersister`, with the typed-nil rationale comment at :657-659. The nil path is genuinely reachable: :601-605 degrades a prefs path-resolution failure to `prefsLoad{}`.
  - `internal/prefs/store.go:220-247` — `SaveTheme` clears both slots, `SaveThemeSlot` clears the constant and rejects an invalid slot; both go through `mutate` → `readFileStrict` (RMW + strict re-read), so the abort-not-overwrite failure the persister reports is real.
- Criteria assessment:
  - AC1 ✅ — `SaveTheme`/`SaveThemeSlot` semantics verified in `internal/prefs/store.go:218-247`; the persister delegates verbatim.
  - AC2 ✅ — `reportCommit` appends `reason` after the identity attrs, so a constant emits `slug,reason` and a slot emits `slug,slot,reason`; message is the catalogue string `"commit failed"` under the `theme` component; WARN level.
  - AC3 ✅ — `reportCommit` returns `err` unwrapped (`cmd/theme_persister.go:49`).
  - AC4 ✅ — `reportCommit` short-circuits on `err == nil` before touching the logger.
  - AC5 ✅ — `internal/tui/theme_panel_commit.go:54-61` applies the failure to state only, with the "No logging: the persister owns the emission" comment; a repo-wide search finds `"commit failed"` in exactly one production file (`cmd/theme_persister.go:12`).
  - AC6 ✅ — method names (`CommitTheme`/`CommitThemeSlot`) deliberately differ from the store's savers.
  - AC7 ✅ — `build.go:123` guard plus `internal/tui/theme_panel_commit.go:31-34`, where a nil persister makes the commit inert rather than failed.
  - AC8 ✅ — `cmd/open.go:660`.
  - AC9 ✅ — no capture/fixture path sets `Deps.ThemePersister` (`internal/capture/fixtures.go` and `cmd/capturetool`).
  - AC10 ✅ as amended — Phase 9 (tick-b0f220 / tick-79eeb1) legitimately added the keypress path; the direct-call verification survives in `cmd/theme_persister_test.go`.
  - AC11 ✅ — two persisters over two stores on one file, each mutating only its own field via `mutate`'s strict re-read.
- Later-task supersessions (verified as intentional, not drift):
  - Seam signature is `CommitThemeSlot(slug string, member theme.Member)`, not the task's `prefs.ThemeSlot`. Superseded by plan task tick-a29b0e "Hold The Domain Light/Dark Type In The Panel And Convert Once At The Persister Seam"; `prefsSlotFor` (`cmd/theme_persister.go:59-64`) is that single conversion point, and `internal/tui/theme_panel_commit_slot_test.go:416-436` now guards the signature and bans `prefs.Slot*` from `internal/tui`. The task's own edge case ("the slot is the existing typed value, not a new string, so the seam cannot mint a third slot") is honoured more strictly, not less.
  - The seam is held at `m.themeState.persister` rather than a bare `m.themePersister` field (`internal/tui/theme_state.go:47`), from the theme-state consolidation (tick-3e59e9). Behaviourally identical.
  - The `"slot" → "light"/"dark"` helper single-sources off `theme.Slot.AttrName()` (`internal/theme/resolution.go:19-28`) rather than a persister-local string, per tick-60a015 — a stronger version of the task's "single-source the rendering" instruction.
- Notes: `themeSlotAttr` discards `AttrName`'s `ok`. Structurally safe — `Member` has exactly two values and `Member.Slot()` maps them to `SlotLight`/`SlotDark`, never `SlotConstant` — but the discard carries no rationale in a file that comments every other decision.

TESTS:
- Status: Adequate (one named test in the plan's list is absent, deliberately superseded — see below)
- Coverage:
  - `cmd/theme_persister_test.go:25-38` `TestThemePersister_CommitTheme` — constant round-trip through the real store to a real temp `prefs.json`; the struct-equality assert proves both slots were cleared and `session_list_mode`/`appearance` survived (AC1, AC11 partial).
  - `:40-61` `TestThemePersister_CommitThemeSlot` — named light/dark subtests, each asserting the constant cleared and the other slot untouched (AC1).
  - `:77-100` `TestThemePersister_FailedCommitLogsAndReturns` — malformed prefs; asserts non-nil return *first* with a message explaining why the value matters, the file unchanged, exactly one record (`OnlyRecord`), WARN, verbatim message, and `reason == err.Error()` (AC2, AC3).
  - `:104-176` `TestThemePersister_CommitFailedAttrs` — exact attr-key sequence per commit shape, so the *absence* of `slot` on a constant is asserted rather than assumed; plus a closed-vocabulary check against the `theme` component's spec-governed key set, plus a parity subtest that reads the loader's own slot rendering off a live emission instead of restating the literals (AC2, and the anti-drift edge case).
  - `:179-202` `TestThemePersister_SuccessIsSilent` — two successful commits emit nothing, then a deliberately failing control commit proves the sink was live (AC4). The positive control is exactly what a silence assertion needs.
  - `:204-223` `TestThemePersister_PerInstanceLastWriteWins` — two persisters over two stores on one file; asserts last-write-wins with every other field intact (AC11).
  - `internal/tui/theme_persister_seam_test.go:44-90` `TestBuild_NilThemePersisterIsTolerated` — nil dep applies no option, the nil-persister model drives through resize/sessions/`s`/`x`/`?` without panicking and still paints (a non-vacuous no-panic assertion), and an injected persister is the one the model holds (AC7).
  - `:94-103` `TestPrefsStore_DoesNotSatisfyThemePersister` — negative assertion boxed through `any` (a direct `var _` cannot express a negative), with a `ModePersister` positive control so the negative cannot pass vacuously (AC6).
  - `:105-142` `TestCommitFailed_SingleEmissionSite` — AST guard over `internal/tui` production files only (`sourceguardtest.PackageGoFiles(".", false)`, which errors on an empty match), banning both a `log.For("theme")` binding and the message literal; the expected string is deliberately a test-side constant so importing it cannot defeat the guard (AC5).
  - `cmd/capturetool/theme_persister_test.go:14-39` `TestCapturetool_NoThemePersister` — every registered fixture (bar the non-model contrast swatch) wires neither persister; `:41-67` additionally proves a capture writes no file into an isolated config dir (AC9).
  - `internal/capture/theme_panel_message_fixtures_test.go:334-343` — the seeded commit-failure frames wire no persister either.
  - Phase 9 adds the end-to-end proof this seam exists for: `cmd/open_theme_commit_test.go` drives keypress → persister → real `prefs.json` → relaunch resolution, including `TestThemePanelCommit_NoPrefsStoreWritesNothing` (:330-350) for the unwired case.
- Notes:
  - `TestOpenTUI_ThemePersisterWiredOnlyWithAStore` (AC8's named test) does not exist. This is a deliberate later supersession: plan task tick-452a30 "Add The Missing Panel → Persister → prefs.json Commit Round Trip And Drop The AST Wiring Guards" replaced the AST wiring guards with the behavioural round trip. The residual is that `cmd/open.go:660`'s guard itself has no assertion — `themeRoundTripConfig` (`cmd/open_theme_commit_test.go:89-91`) re-implements the guard test-side rather than observing production's. Same standing as the `modePersister` precedent, which is equally unasserted, and the blast radius is small (a lost guard panics only where prefs path resolution fails). Noted, not blocking.
  - Mild over-test: `TestThemePersister_MemberToPrefsSlot` (`:63-75`) asserts the `Member → prefs.ThemeSlot` mapping that `TestThemePersister_CommitThemeSlot` already proves end-to-end through the file for both halves.
  - Test isolation is correct throughout: every case pins `PORTAL_PREFS_FILE` into a `t.TempDir` via `setPrefsFile`, the failure fixtures use a malformed file or an unwritable dir (mode restored by a cleanup registered after `t.TempDir`'s), and `prefsStoreForTest` uses the *non-migrating* loader with a stated reason — the migrating loader would dispatch a write of its own and make the negative log assertions pass for the wrong reason. No `t.Parallel()`, per CLAUDE.md. Nothing here spawns a daemon or touches tmux, so the unit lane is correct.

CODE QUALITY:
- Project conventions: Followed. The `theme` component is bound once per package (`cmd/open.go:27`) and reused by the persister rather than re-bound or passed as a parameter, with that reasoning stated at `cmd/theme_persister.go:24-25`; `internal/prefs` remains a leaf importing only `fileutil`; the attr vocabulary (`slug`/`slot`/`reason`) is the spec-governed set; the test file is named after its source file; DI follows the established small-interface + `Deps` + `Option` pattern, mirroring `ModePersister` exactly as §8.9 requires.
- SOLID: Good. Single responsibility is tight — the persister converts, delegates, reports and returns; it decides nothing about paths, slugs or panel state. Interface segregation is deliberate and load-bearing: the two-method seam's names diverge from the store's precisely so `*prefs.Store` cannot silently satisfy it and bypass the emission site. Dependency inversion holds — `internal/tui` depends on the seam, `cmd` supplies the implementation.
- DRY: Good. `reportCommit` is the single emit-and-return path (the alternative — `reason` appended at both call sites — would let attr order drift), and `themeSlotAttr` defers to `theme.Slot.AttrName()` so the persister's `light`/`dark` strings cannot drift from the loader's.
- Complexity: Low. Two one-line public methods, one three-line reporter, two trivial mappers; no branching beyond the nil-error short-circuit.
- Modern idioms: Yes. Variadic `...any` slog attrs, a compile-time interface assertion, `log.OrDiscard` for the zero-value receiver, value receivers on an immutable two-field struct.
- Readability: Good. Every non-obvious decision carries a why-comment (why `cmd` owns it, why the logger is not a parameter, why `reason` is appended centrally, why the two slot vocabularies stay separate types). Comments were checked against the code they describe and all hold: `prefs` production sources import only `internal/fileutil`, so the "prefs must not import internal/theme" justification at :57-58 is accurate; the model does emit nothing; the seam's doc comment at `model.go:72-75` correctly describes both the ownership and the deliberate method-name divergence. No comment references a task id, phase or spec section.
- Security: N/A — no untrusted input, no shelling out; the write path is `fileutil.AtomicWrite` via the store.
- Performance: N/A — one file RMW per explicit user commit.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] cmd/theme_persister.go:52-55 — `themeSlotAttr` silently discards `AttrName`'s `ok`, the only unexplained decision in an otherwise fully-reasoned file. Add above the function: `// The ok is discarded rather than handled: Member.Slot() yields only SlotLight or SlotDark, so the constant's empty name is unreachable here.`
- [quickfix] cmd/theme_persister_test.go:63-75 — fold `TestThemePersister_MemberToPrefsSlot` away: `TestThemePersister_CommitThemeSlot` already proves both `Member → prefs.ThemeSlot` mappings through the file (dark writes `theme_dark`, light writes `theme_light`). If it is kept for failure localisation, wrap each case in a named `t.Run` — it is currently the one table in the file that loops without named subtests, against the project testing skill's first rule.
- [idea] cmd/open.go:660-663 — the typed-nil wiring guard has no test observing production: `themeRoundTripConfig` (cmd/open_theme_commit_test.go:89-91) re-implements the same `if store != nil` check test-side, so a regression that dropped the production guard would not fail anything. Decide whether to extract the wiring into a small assertable helper (e.g. `themePersisterFor(*prefs.Store) tui.ThemePersister` returning a nil interface for a nil store, which also makes the guard unforgettable at the call site) or to accept the gap as the equally-unasserted `modePersister` precedent does.
