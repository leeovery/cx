TASK: theming-system-3-2 — `tui.Build` Takes The Loaded Nomination And The Gate Selects Its Member

ACCEPTANCE CRITERIA (from the plan task):
1. `Deps.Appearance` / `WithAppearance` do not exist; a source guard proves it.
2. `New()` with no theme option still yields a themed model (tokyo-night values) and renders truecolor SGRs.
3. Constant nomination: gate resolved + unarmable at construction, `Init` issues no timeout tick, first frame paints the constant's canvas, OSC 11 query still issued and reply captured.
4. `Init` issues the OSC 11 query under every setting shape (constant / adaptive / `NO_COLOR`); a reply at any time populates `originalBg`.
5. After a constant launch on a light terminal the model holds the captured background + the arrival fact, with no light/dark answer in force derived from it.
6. Adaptive nomination: gate armed, nothing painted until it resolves, `activeTheme` unused in the interval.
7. The gate resolves exactly once (a post-timeout reply populates `originalBg`, never re-themes).
8. Under `NO_COLOR` the gate is skipped, both members are loaded and held, the dark member is selected.
9. `"appearance": "dark"` → tokyo-night, `"light"` → tokyo-night-day, `auto` → the pair, with `prefs.json` bytes unchanged (no file created when absent).
10. `prefs.Appearance` / `LoadAppearance` still exist and decode tolerantly (deletion is Phase 5–6).
11. `cmd` binds the `theme` component exactly once (a single package-level `themeLogger`).
12. TUI construction reads no themes directory, no prefs theme keys, enumerates nothing.
13. `portal open <target>` (exec path) constructs no TUI and performs no theme load.
14. `capturetool` passes the constant shape; fixtures render byte-identically.
15. No fallback path exists in this task (§7.6 fatal / §8.5 per-slot fallback stay in Phase 5).

STATUS: complete

SPEC CONTEXT:
§8.4 — "the constructor therefore takes the loaded *nomination*, not a single theme": Constant (one loaded `Theme`, active from frame one, gate never consulted) or Adaptive (both loaded `Theme`s, **no active member yet**, gate selects before first paint). The nomination carries no provisional active member. §8.8 — `prefs.Appearance`'s enum/API and `WithAppearance` die; the detect-or-timeout gate survives but conditional (constant ⇒ immediate paint); the OSC 11 *query* survives unchanged because `restore.go` needs it independent of detection; "the gate resolves exactly once… the reply is still *consumed* but never flips the active theme". §9.10 — under `NO_COLOR` both nominated themes are still loaded, the gate is skipped, the standing dark fallback selects the member. §13.3 — `capturetool` always passes the constant shape (byte-deterministic captures). §12.3 — `cmd` passes a real component logger where a theme is *used*, `log.Discard()` on doctor/export/capturetool.

Amendments by later phases (per the verifier-context note): criteria 9, 10 and 12 were deliberately superseded. Phase 5 replaced the in-memory `appearance` mapping in `cmd/open.go` with resolution from the persisted `theme` / `theme_light` / `theme_dark` keys (`themeResolution` + `theme.Loader.ResolveNomination`, §8.5 fallback, §7.6 fatal), so cmd-side construction now legitimately reads prefs theme keys and may read one or two theme *files* by name (never enumerating). Phase 6 replaced the in-memory-only mapping with the one-shot `appearance` translation and deleted `prefs.Appearance` / `LoadAppearance` (`internal/prefs/appearance_api_guard_test.go` now guards their absence). `Select(dark bool)` became `Select(theme.Member)`. These are intentional supersessions, not drift.

IMPLEMENTATION:
- Status: Implemented (as amended by Phases 5–9)
- Location:
  - `internal/theme/nomination.go:1-66` — two-state `Nomination`, constructor-only (unexported fields), `ConstantNomination` / `AdaptivePair(light, dark)` / `IsConstant` / `Constant` / `Select(Member)`; zero value is neither state and answers with a zero `Theme` rather than panicking; separate `constant` field documented so `Constant()` on a pair cannot return the dark member.
  - `internal/tui/build.go:16-63,118` — `Deps.Appearance` gone, `Deps.Theme theme.Nomination` present, `Build` always applies `WithThemeNomination`; `armAppearanceDetection()` at `build.go:165`.
  - `internal/tui/model.go:551-557` (`WithThemeNomination`), `813-841` (`New` seeds `themeState.active` with the dark built-in **before** options, then builds the gate from the shape — colourless first, `WithCanvasMode` override preserved), `843-846` (`armAppearanceDetection`), `856-875` (`syncResolvedMode` / `captureStartupCanvasHex`).
  - `internal/tui/appearance_gate.go:25-71` — `newNominationGate` returns a pinned (resolved, unarmable) gate for a constant *and* for the zero nomination; an adaptive pair gets an armable gate whose zero answer is the dark no-answer fallback; `resolve` is first-call-only.
  - `internal/tui/model.go:1423-1464` (`Init`) — the OSC 11 query is unconditional under every shape; the timeout tick is nil for an already-resolved gate. `model.go:1485-1502` (`Update`) — the reply is retained *before* the gate is offered it, `originalBg` set under a nil-Color guard, `syncResolvedMode` only when resolution actually happened.
  - `internal/tui/theme_state.go:11-31,85-98` — `terminalReply{arrived, member}` retains the reply for §9.3's later conversion (`adoptRetainedReply`), separate from the answer in force (`canvasMode` / `adoptGateAnswer`).
  - `internal/tui/model.go:2697-2705` + `2722-2730` — unresolved ⇒ unstyled blank frame with no `BackgroundColor` set (no paint-then-flip).
  - `internal/tui/builtin_themes.go:1-16` — the transitional built-in *pair* holder is gone; only `defaultDarkTheme()` (the `New` seed, loaded through the silent loader) survives, with no fallback on a miss.
  - `cmd/open.go:27` — the single package-level `var themeLogger = log.For("theme")`; `open.go:483-512` — `newThemeLoader` / `buildThemeLoader` / `themeResolution`; `open.go:442-474,532` — `tuiConfig.appearance` replaced by `theme theme.Nomination` (+ `themeKeys`).
  - `internal/capture/fixtures.go:70-107` — fixture `Deps()` wraps the pinned palette in `theme.ConstantNomination`; `cmd/capturetool/main.go:87-166` resolves `--theme` to one `theme.Theme` and hands it in (un-gated, byte-deterministic).
  - `internal/tui/restore.go:20-32` — the colourless early return flagged in the task's commit message (owned by 3-3) is present, so the now-unconditional query does not produce a set-back under `NO_COLOR`.
- Notes: criterion 6's literal wording ("`activeTheme` is unset in the interval") is met in substance rather than by leaving the field empty: `syncResolvedMode` pre-selects the dark member while the gate is open, but `View()` gates on `modeResolved()` and paints an unstyled blank frame with no `BackgroundColor`, and `captureStartupCanvasHex` deliberately leaves `startupCanvasHex` empty until resolution. The *`Nomination`* itself carries no active member, which is what §8.4 requires. `cmd/open_theme_construction_test.go:85` pins the observable half (`View().BackgroundColor == nil` before resolution).

TESTS:
- Status: Adequate
- Coverage:
  - `internal/tui/nomination_test.go` carries the task's named tests, all present and behavioural: `TestDeps_HasNoAppearanceField` (reflect for the field + AST scan of exported funcs for `WithAppearance`), `TestNew_SeedsTheDarkBuiltinWhenNoNominationIsGiven` (asserts both the value *and* the rendered SGR), `TestNomination_ConstantSkipsDetectionAndWait` (resolved, re-arm is a no-op, no timeout tick, query still issued, constant canvas painted), `TestNomination_AdaptiveArmsTheGate`, `TestNomination_GateSelectsMember` (dark reply / light reply / timeout table), `TestGate_LateReplyCapturesBackgroundButNeverReThemes`, `TestGate_QueryIssuedRegardlessOfSettingShape` (constant / pair / NO_COLOR), `TestGate_ConstantRetainsReplyWithoutClassifying` (background + arrival retained, `inForceMode()` stays the standing dark fallback), `TestNoColor_LoadsBothAndSelectsDark` (asserts the *light* member is still reachable via `Select`), `TestConstruction_ReadsNoThemesDirectory` (AST guard banning `os.ReadDir/ReadFile/Open/Stat/Getenv/LookupEnv`, `filepath.Walk*` and any `.Enumerate` across the whole `internal/tui` package).
  - `internal/theme/nomination_test.go` covers the type contract itself: constant holds one theme for both answers, pair holds both with no constant, argument order is light-then-dark (a swapped construction is observably different), both members filled, zero value is neither state *and* is distinguishable from `ConstantNomination(Theme{})` / `AdaptivePair(Theme{}, Theme{})`.
  - `internal/tui/appearance_detection_test.go` covers the surviving gate semantics (unresolved carries the dark fallback, dark/light detection, no-paint-then-flip, timeout fallback, COLORFGBG never overriding OSC 11, `TestAdaptiveArmsTimeoutTick` as the positive counterpart to `assertNoTimeoutTick`).
  - `cmd/open_theme_nomination_test.go` covers criteria 11 and 13: `TestThemeComponent_BoundOnceInCmd` (AST, package-level var by name) and `TestOpenExecPath_DoesNoThemeWork` (a source guard over every `theme.*` call site outside the sanctioned construction helpers **plus** a runtime half that first proves the poisoned-themes-dir assertion is non-vacuous, then asserts zero `theme` records across a real exec-path `portal open <target>`).
  - Criterion 9 is now covered at its amended home: `cmd/prefs_translation_test.go:62-73` (`dark`/`light` pin renders the equivalent constant), `:77-92` (`auto`/absent/empty/unrecognised translate to nothing ⇒ the shipped pair), `:147-174` (translating launch leaves the file byte-identical; an absent `prefs.json` stays absent).
  - Criterion 10's amended form is guarded by `internal/prefs/appearance_api_guard_test.go` (the enum and its API must be *gone* while the raw on-disk field round-trips — `internal/prefs/theme_keys_test.go:202`).
  - Nil-`Color` replies (the one edge the criteria leave implicit) are covered by `internal/tui/theme_answer_test.go:10-25` and `internal/tui/background_restore_test.go:99`; `BackgroundColorMsg.IsDark()` is genuinely nil-safe (ultraviolet `isDarkColor` returns true for nil), so the "nil-safe" comment at `theme_state.go:18` holds.
  - Criterion 14's byte-identity cannot be re-derived at review time (tapes/PNGs are scaffolding), but the fixture plumbing is pinned by `internal/capture` and the swap-and-diff completeness guard.
- Notes: three tests at the same layer assert the resolve-once contract over near-identical fixtures — `TestNoPaintThenFlip` (appearance_detection_test.go:107), `TestGate_LateReplyCapturesBackgroundButNeverReThemes` (nomination_test.go:102) and `TestInForceMode_LateReplyIsRecordedButNeverReThemes` (theme_answer_test.go:57). The last two share the same setup and both assert `active` unchanged, differing only in which retained-reply accessor they read. Mild redundancy, noted below; not enough to call the task over-tested.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays leaf-and-log-free (the loader takes an injected `EventLogger`); `cmd` binds the `theme` component once per package per CLAUDE.md's bind-once rule; `internal/tui` reads no env and no files (guarded); no `t.Parallel()`; tests inject the `*Deps` seams before Executing a command body (`execOpenSession` injects `bootstrapDeps` + `openDeps` and stubs `openSessionFunc`); no `lipgloss.LightDark` / `compat.AdaptiveColor` and no DEC 2031 adoption, as the task explicitly required.
- SOLID principles: Good. `Nomination` is a single-responsibility value with a constructor-only contract; the gate depends on the nomination's *shape* only (`IsConstant`), not on prefs; the retained reply is modelled separately from the answer in force, which is what lets Phase 9's conversion reuse it without a second query.
- Complexity: Low. `newNominationGate` is two branches; `syncResolvedMode` is a four-line chokepoint; `Init`'s batching is unchanged apart from the unconditional query.
- Modern idioms: Yes — `reflect.TypeFor[Deps]()` in the guard, a small comparable value type, `switch` with no fallthrough in `Select`.
- Readability: Good. The comments carry the *why* (why constant and dark are separate fields, why the zero gate means resolved, why the reply is retained before the gate is offered it, why `startupCanvasHex` must not move with `active`) and none reference task ids, phases or spec sections. All comments I checked hold true against the code, including the nil-safety claim on `IsDark`.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_answer_test.go:57 — `TestInForceMode_LateReplyIsRecordedButNeverReThemes` duplicates the fixture and the "active unchanged" assertion of `TestGate_LateReplyCapturesBackgroundButNeverReThemes` (internal/tui/nomination_test.go:102). Move its two unique assertions (`reply.answer() == MemberLight`, `inForceMode() == MemberDark`) into the nomination_test.go case and delete the duplicate.
- [quickfix] internal/tui/appearance_gate.go:32 — the zero-nomination check is spelled as a raw struct comparison (`n == (theme.Nomination{})`) here and again in `Model.hasNomination` (internal/tui/model.go:852-854). Add a `func (n Nomination) IsSet() bool` (or `IsZero`) to internal/theme/nomination.go beside the documented zero-value contract and route both call sites through it, so the "zero value is neither state" rule lives in one place.
