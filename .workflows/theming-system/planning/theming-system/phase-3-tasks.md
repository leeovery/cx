# Phase 3: The render layer on a single threaded palette — 5 tasks

## theming-system-3-1

### Task 3.1: Thread the active `Theme` through every renderer and delete `internal/tui/theme`

**Problem**: The whole render layer is still built on the pre-feature vocabulary. `internal/tui/theme` declares 20 tokens carrying hue names (`accent.violet`, `state.green`) and place names (`border.footer`), each holding a `Light` **and** a `Dark` hex resolved through `ColorFor(mode)`, read from the package-level `theme.MV` at **182 non-test call sites**, with `theme.Mode` threaded as a parameter through ~131 render-helper signatures and both list delegates whose only job is to reach `ColorFor`. Phases 1–2 built the entire replacement — `internal/theme`'s 19-token single-palette vocabulary, the loader, and three embedded `.theme` built-ins — and **nothing consumes it**. The three edits (rename the vocabulary, collapse `Token` to one value, swap the `mode` slot for a `Theme`) all land on the *same lines*, so sequencing them apart would touch all 182 production sites and ~600 test call sites two or three times over. The edit also cannot be sliced by file: `headerStyle` / `headerCanvasBg` in `internal/tui/header.go` are the shared leaf-paint helpers every renderer calls, both list delegates included, so their signature change is instantly global.

**Solution**: One mechanical substitution across `internal/tui` (plus any remaining `internal/tui/theme` consumer): `theme.MV.<OldField>` → `th.<NewField>` on an injected active `theme.Theme` (the new `internal/theme`), `.ColorFor(mode)` → `.Color()`, and every `mode theme.Mode` parameter/struct-field slot → `th theme.Theme`. The model gains an `activeTheme` field sourced **transitionally** from the embedded built-in pair, selected by the appearance gate's light/dark answer (now an unexported `internal/tui` concept). `internal/tui/theme` is then deleted with its MV-shaped tests, and the colour-literal guard loses its subpackage exemption.

**Outcome**: `internal/tui/theme` no longer exists, `go build ./... && go test ./...` is green, the colour-literal guard still passes with zero hex literals in `internal/tui`, and every captured screen renders exactly as it does today apart from the two intended deltas (the dark footer rule, and any light value Phase 2's §7.7 re-derivation moved) — with the existing `appearance` pin still selecting the equivalent built-in.

**Do**:
- **Apply the §2.4 rename map at every site.** Old Go field → new Go field: `TextStrong`→`TextSecondary`, `TextMutedBright`→`TextTertiary`, `TextDetail`→`TextMuted`, `TextDim`→`TextSubtle`, `AccentViolet`→`AccentPrimary`, `AccentBlue`→`AccentKey`, `AccentCyan`→`AccentMode`, `AccentOrange`→`AccentAttention`, `StateGreen`→`StatePositive`, `StateRed`→`StateDestructive`, `BgWarning`→`BgAttention`, `BgTrack`→`BgSubtle`, `BorderSeparator`→`Border`, `TextOnWarning`→`TextOnAttention`; `TextPrimary`, `TextFaint`, `TextOnSelection`, `Canvas`, `BgSelection` unchanged. **`BorderFooter` has no successor** — its sole production consumer (`footerTopRule` in `internal/tui/footer.go`) takes `Border`, the same token as the title rule (§2.2).
- **Swap the parameter slot, don't add one.** Every `mode theme.Mode` in a signature becomes `th theme.Theme` in the same position — `headerStyle(fg theme.Token, th theme.Theme, colourless bool)`, `renderHeaderBlock(width int, th theme.Theme, colourless bool)`, and the ~131 siblings across `header.go`, `footer.go`, `filter_footer.go`, `section_header.go`, `notice_band.go`, `modal.go`, `modal_footer.go`, `panel.go`, `destructive_confirm.go`, `kill_modal.go`, `delete_modal.go`, `edit_modal.go`, `help_modal.go`, `rename_modal.go`, `empty_states.go`, `filtering_no_matches.go`, `loading_view.go`, `pagepreview.go`, `sessions_flash.go`, `session_item.go`, `project_item.go`, `model.go`. Inside each body, `theme.MV.X.ColorFor(mode)` becomes `th.X.Color()` and the canvas background becomes `th.Canvas.Color()`. `SessionDelegate.Mode` / `ProjectDelegate.Mode` become `Theme theme.Theme`; `previewModel.mode` becomes `previewModel.th`.
- **Delete `pagepreview.go`'s package-scope `Token` copy.** `var previewBorderColorToken = theme.MV.AccentCyan` (the §11.2 named offender) cannot survive `theme.MV`'s deletion — inline `th.AccentMode` at its use sites. Phase 4 owns the *wider* init-time derived-style sweep and the swap-and-diff guard; do not attempt either here.
- **Make the gate's answer an unexported `internal/tui` concept.** The token layer carries no variant (§4.7), so declare an unexported enum in `appearance_gate.go` — `type canvasAppearance int` with `appearanceDarkCanvas canvasAppearance = iota` first — and replace `theme.Mode` in `appearanceGate`, `resolve`, `resolveDark`, `resolveFromDark` and `Model.canvasMode` with it. **The zero value must remain dark** (the no-answer fallback); a `bool` field would invert it silently.
- **Hold the active `Theme` on the model, with a transitional source.** Add `activeTheme theme.Theme` to `Model` and an unexported holder for the embedded pair (light + dark `theme.Theme`), seeded in `Build` by loading `theme.DefaultLightSlug` / `theme.DefaultDarkSlug` through `Loader.LoadBuiltin`; `New` seeds `activeTheme` from the dark built-in before options apply so a model constructed without `Build` is still themed. `syncResolvedMode` sets `m.activeTheme` from the gate's resolved appearance and then calls `applyCanvasMode` exactly as today. **No package-level mutable theme state** — no `theme.Active` var, no setter. Task 3-2 replaces the pair holder with the injected nomination.
- **Re-point the `bubbles/list`-owned cached styles from the theme**, not a mode: `canvasHelpStyles`, `canvasPaginationDots`, `colourlessHelpStyles`/`colourlessPaginationDots` (unchanged in behaviour), the TitleBar canvas background, both filter inputs (`styleFilterInput`), `centrePaginationRow`'s canvas fill, and `View`'s `v.BackgroundColor` / `fillCanvas`. Both delegates are rebuilt through the existing `sessionDelegate()` / `applyProjectCanvasMode` restyle path carrying the `Theme`.
- **Delete `internal/tui/theme` entirely** — `theme.go`, `theme_test.go` (`TestMVDarkVariantsPinned`, `TestMVTokenCount`, `TestEachTokenCarriesLightVariant`) and `contrast_test.go` (`TestEveryTokenHasLightVariant` plus the ten mode-paired floor tests and the two light-pin tests). Phase 2 already re-homed the floors, the pins and the enrolment table into `internal/theme`; nothing here is lost. Update the colour-literal guard's doc comment in `internal/tui/colour_literal_guard_test.go` to drop the `theme/` subpackage exemption wording — **the glob is unchanged** — and add a cheap source guard asserting no file in the repo imports `github.com/leeovery/portal/internal/tui/theme`.
- **Capture the before/after, then delete the image set and its tapes.** Before starting, re-capture (or confirm) today's committed `testdata/vhs/*.png` as the before state; after the substitution re-run each tape and diff — this is the last useful before/after for a substitution that is supposed to be visually inert. Then delete `testdata/vhs/*.png` and `testdata/vhs/*.tape` in the same act per §13.2. **Keep** the Go fixture definitions in `internal/capture`, the harness itself, `testdata/vhs/README.md` and `testdata/vhs/reference/` (see Context).
- **Correct CLAUDE.md's `tui/theme` row**: it moves out of the TUI subtree into the internal-packages inventory as `internal/theme`, describing the 19 single-palette tokens, `Token.Color()`, `Theme` as a parse result rather than a built-in var, the embedded `.theme` files, and `contrast_test.go` auto-enumerating the embedded set against each theme's own canvas. The `tui` row is **task 3-3's**; the config, bootstrap-exempt, prefs, log-count and capture-harness entries are **Phase 10's**.

**Acceptance Criteria**:
- [ ] `internal/tui/theme` does not exist; no file in the repo imports it; `grep -r "theme.MV\|ColorFor\|theme.Mode" --include="*.go" .` returns nothing.
- [ ] Exactly **two** classes of intended render delta: the dark footer rule moves from `#20232E` to `#292E42` (`border.footer` → `border`), plus any light value Phase 2's §7.7 re-derivation moved. Every other rendered byte is unchanged — the byte-exact golden tests (`edit_modal_render_byte_exact_test.go`, the row-anatomy and reskin suites) pass without their expectations being edited except for those two classes.
- [ ] The dark palette's SGR output is byte-identical to today's despite the uppercase canonicalisation — `lipgloss.Color("#0b0c14")` and `lipgloss.Color("#0B0C14")` resolve to the same RGB.
- [ ] An existing `"appearance": "light"` / `"dark"` pin still paints the equivalent built-in from frame one, and `auto` still runs the detect-or-timeout gate with a dark no-answer fallback — `WithAppearance` and `prefs.Appearance` are untouched by this task.
- [ ] `NO_COLOR` renders colourless and glyph-backed exactly as today: the colourless flag is read **before** any token resolves, so no `Color()` call reaches the writer.
- [ ] The model holds the active `Theme`; `internal/tui` declares no package-level `theme.Theme` or `theme.Token` var (guard test), and `pagepreview.go`'s `previewBorderColorToken` is gone.
- [ ] `TestNoRawColourLiteralAtCentralisedSites` passes over the unchanged glob with zero hex literals in `internal/tui`, and its exemption wording is deleted rather than re-pointed.
- [ ] `testdata/vhs/*.png` and `testdata/vhs/*.tape` are deleted; `internal/capture`'s fixture definitions, `FixtureNames()`, `testdata/vhs/README.md` and `testdata/vhs/reference/` remain.
- [ ] `go build ./... && go test ./...` green, `go test -tags integration -p 1 ./...` green, `golangci-lint run` clean.
- [ ] CLAUDE.md's `tui/theme` row is corrected and relocated; the `tui` row is untouched (3-3 owns it).

**Tests**:
- `"it sources the active theme from the embedded built-in the gate selected"` — `TestActiveTheme_SelectedByGateAppearance` (dark gate → `tokyo-night`'s canvas value; light gate → `tokyo-night-day`'s)
- `"it keeps the dark canvas zero value on the gate"` — `TestCanvasAppearance_ZeroValueIsDark`
- `"it renders the footer rule with the consolidated border token"` — `TestFooterTopRule_UsesBorderToken` (asserts `#292E42` in the dark frame, and that `#20232E` appears nowhere)
- `"it declares no package-level theme state in internal/tui"` — `TestNoPackageLevelThemeVar` (AST scan for a package-scope var of type `theme.Theme`/`theme.Token`, catching a returning `previewBorderColorToken`)
- `"it no longer imports the old theme subpackage"` — `TestOldThemeSubpackageIsGone`
- `"it suppresses every colour under NO_COLOR"` — `TestColourless_NoTokenReachesTheWriter` (existing `colourless_nocolor_test.go` cases, migrated)
- `"it still honours a pinned appearance from frame one"` — `TestPinnedAppearance_SelectsEquivalentBuiltin`
- `"it renders every fixture identically apart from the intended deltas"` — the migrated byte-exact and reskin suites (`session_row_anatomy_test.go`, `project_row_anatomy_test.go`, `sessions_grouped_reskin_test.go`, `edit_modal_render_byte_exact_test.go`, `pagepreview_sgr_test.go`)
- `"it keeps no raw colour literal at a call site"` — `TestNoRawColourLiteralAtCentralisedSites` (existing, exemption wording removed)

**Edge Cases**:
- **Exactly two intended render deltas** — the dark footer rule `#20232E` → `#292E42`, plus any light value §7.7 moved. Anything else is a bug in the substitution, not a design change.
- MV's lowercase hexes are uppercase-canonical in the `.theme` files, so `lipgloss.Color` yields identical SGR and the byte-exact golden tests hold with no expectation edits.
- `NO_COLOR` still suppresses every colour and stays glyph-backed — the colourless flag is read before any token resolves, so the substitution cannot leak a hue into the colourless path.
- The gate's light/dark answer becomes an **unexported `internal/tui` concept** because the token layer carries no variant (§4.7). Its zero value must stay dark; a bare `bool` inverts the no-answer fallback silently.
- The model's active `Theme` is sourced **transitionally** from the embedded built-ins by that answer; task 3-2 replaces the source with the injected nomination.
- **No package-level mutable theme state** replaces `theme.MV` — no `Active` var, no setter. §3.4's reason is that it would put order-dependent mutable state on the render path in a suite that already forbids `t.Parallel()`.
- `pagepreview.go`'s package-scope `Token` copy goes here because it cannot survive `theme.MV`'s deletion. **Phase 4 owns the wider init-time derived-style sweep and the swap-and-diff guard** — do not build either here.
- The `bubbles/list`-owned cached styles (help, pagination dots, both filter inputs, the canvas paint, the centred pagination row) re-point **from the theme** rather than a mode; both delegates carry the `Theme` and are rebuilt through the existing restyle path.
- The colour-literal guard's `theme`-subpackage exemption is **deleted, not re-pointed** — §3.2 moves the package to a sibling, and widening the globs to reach it purely to exempt it would be a mechanism change. The exemption has also lost its reason: after §7.1 the new package holds no hex values at all.
- `TestMVDarkVariantsPinned`, `TestMVTokenCount`, `TestEachTokenCarriesLightVariant`, `TestEveryTokenHasLightVariant` and the ten mode-paired floor tests **die with the package** — Phase 2 re-homed the floors, pins and enrolment into `internal/theme`.
- ~600 test call sites pass `theme.Dark` / `theme.Light` to render helpers. They take a loaded built-in instead; add one in-package helper (e.g. `testDarkTheme(t)` / `testLightTheme(t)` over `LoadBuiltin`) rather than 600 ad-hoc loads.
- **Zero-value `Model{}` literals in in-package tests** (25 of them) would render with an all-empty `Theme`, which `lipgloss.Color("")` turns into the no-colour sentinel — a silent colourless render, not a compile error. Any such literal that renders must seed `activeTheme` from the test helper.
- Today's committed PNGs are the last useful before/after for this substitution: **compare, then delete** the image set and its tapes in the same act per §13.2, keeping the Go fixture definitions and the harness, which are permanent.
- CLAUDE.md's `tui/theme` row is corrected here; the `tui` row is task 3-3's.

**Context**:
> §3.1: "**A theme becomes one palette of 19 values, and is itself light or dark.** MV splits into two built-ins carrying the existing values." §3.2: `Token.ColorFor` is removed in favour of a no-argument `Token.Color()`; `theme.Mode` is removed "along with its threading through the render layer"; `theme.MV` as an exported package-level value "ceases to exist".
> §3.4: "`theme.MV` is currently a package-level global read directly at ~182 call sites. Making the active theme switchable is a straight substitution rather than a new mechanism, because **split removes the `mode` parameter** from every one of those sites — so all 182 are being edited regardless, and a parameter slot is freed at exactly the same moment. **The model holds the active `Theme` and passes it where `mode` is passed today.**" Mutable package state was rejected: "its entire advantage was avoiding churn Portal is now paying anyway".
> §2.2: `border.separator` and `border.footer` are one role. "**`border.footer` is dropped.** The footer rule renders with the same token as the title rule. **Accepted visual change:** in dark themes the footer rule becomes marginally more prominent (`#292E42` rather than `#20232E`)."
> §11.2: "**Two known offenders are fixed outright**, not guarded around: `pagepreview.go` copies a `Token` at **package init**, so it would never see a swap… Fixing them does not make the guard redundant; **the guard is what stops them returning**."
> §13.6: the colour-literal guard is "**Unchanged in mechanism and unchanged in scope**… **Its `theme`-subpackage exemption is deleted rather than re-pointed**."
> §13.2: "**Everything that exists today as an image or tape is deleted** — the committed reference PNGs and the VHS tapes that produce them… **The deletion covers images and tapes, NOT fixtures.** The Go fixture *definitions* in `internal/capture` and the harness itself are **permanent**."
> **Ambiguity flagged**: §13.2's "everything that exists today as an image or tape" is written against CLAUDE.md's description of `testdata/vhs/` as "one `.tape` per canonical screen plus committed reference PNGs" — i.e. the VHS capture outputs. `testdata/vhs/reference/` holds the **Paper-exported MV design frames**, which §15.4 keeps as historical reference and which `internal/capture/fixtures.go` comments point at by name. Scope the deletion to `testdata/vhs/*.png` and `testdata/vhs/*.tape` and leave `reference/` in place; if the human gate wants `reference/` gone too, that is a one-line follow-up, whereas deleting it here would destroy the MV design record and strand a dozen source comments.
> Phase boundary: `WithAppearance`, `Deps.Appearance` and `prefs.Appearance` all survive this task — the input path deliberately does not move until 3-2, so "rendering is unchanged" is provable against the *unchanged* input.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §2.2, §2.4, §3.1–§3.4, §11.2, §12.6, §13.2, §13.6

## theming-system-3-2

### Task 3.2: `tui.Build` takes the loaded nomination and the gate selects its member

**Problem**: Task 3-1 left the render layer on a threaded `Theme` but the *input* is still `prefs.Appearance`: `tui.Deps.Appearance` / `WithAppearance` inject a light/dark mode, and the model's active theme is sourced from a transitional bridge that hardcodes the two built-ins. That is the exact injection §13.3 removes — "a theme *is* the mode", so there is no mode left to pin — and it blocks two later phases outright: Phase 5 cannot hand the model a theme resolved from `prefs.json`, and Phase 8's panel cannot swap one. It also mis-states the startup contract: a user on a **constant** theme needs no detection at all (their first paint should be immediate), while a user on the **adaptive** pair still needs light/dark resolved before anything is painted or Portal paints one theme and flips — under split that flip swaps a whole named theme, not a variant.

**Solution**: Introduce a two-state loaded **nomination** in `internal/theme` (constant → one `Theme`; adaptive → light and dark `Theme`s with no active member), have `tui.Build` take it where it takes `prefs.Appearance` today, and build the appearance gate from its shape. `cmd/open.go` maps the surviving legacy `appearance` pref onto that shape **in memory only** per §10.2, so an existing pin keeps working with nothing written to `prefs.json`.

**Outcome**: `Deps.Appearance` and `WithAppearance` are gone; a constant nomination paints from frame one with no detection and no wait, an adaptive nomination resolves exactly once before first paint, `portal open <target>` still does zero theme work, and an existing `"appearance": "dark"` pin renders `tokyo-night` with `prefs.json` byte-unchanged.

**Do**:
- **Declare the nomination in `internal/theme`** (it is where by-name resolution lives, §3.2, and Phase 5 populates it from prefs): a `Nomination` value with constructors `ConstantNomination(t Theme)` and `AdaptivePair(light, dark Theme)`, plus `IsConstant() bool`, `Constant() Theme` and `Select(dark bool) Theme`. Document that it carries **no provisional active member under adaptive** — the gate resolves before anything is painted, so there is no frame to render in the interval and no second resolution to reconcile (§8.4).
- **Replace the injection in `internal/tui`**: delete `Deps.Appearance` and the `WithAppearance` option, add `Deps.Theme theme.Nomination` and a `WithThemeNomination` option, and drop the transitional built-in-pair holder task 3-1 added — the nomination is now the source. `Build` always injects it (a zero-value nomination is not a valid state; see Edge Cases). **`New`'s dark-built-in seed of `activeTheme` stays**, and is not part of the holder being dropped: a model constructed without `Build` (or with `New` plus options but no nomination) must still be themed, because an empty `Theme` resolves through `lipgloss.Color("")`'s no-colour sentinel — a silent colourless render with no compile error and no failing assertion, which is the hazard task 3-1 added the seed to close and which Phases 4, 8 and 9 all build models against. Applying a nomination through `WithThemeNomination` (or `Build`) overwrites it.
- **Build the gate from the nomination's shape**, replacing `newAppearanceGate(prefs.Appearance)`: a **constant** constructs the gate already resolved and unarmable and sets `activeTheme` to the constant immediately; an **adaptive pair** constructs an armable gate (dark zero value = the no-answer fallback) and sets `activeTheme` only when the gate resolves, through the existing `syncResolvedMode` path. `newColourlessGate` keeps its behaviour: resolved, unarmable, and the standing dark fallback selects the member.
- **Leave the single-resolution machinery untouched, and keep the query unconditional.** `resolveFromDark` / `resolveDark` already no-op once resolved, and `Update`'s `tea.BackgroundColorMsg` arm already stores `originalBg` *unconditionally* while calling `syncResolvedMode` only when resolution actually happened. Do not change that ordering: the reply must still be **consumed** for `restore.go`'s original-background capture (and for §9.3's mid-session conversion) while never re-theming. **The OSC 11 query is issued from `Init` regardless of the setting shape** — a constant nomination skips the *gate*, never the *query*. `restore.go` needs the reply independent of detection, and §9.3's conversion needs it already in hand, so a constant path that skips the query breaks both. Retain the reply in a form a later consumer can classify (the captured background plus whether one ever arrived), and comment that a constant's pre-resolved gate carries **no** detection-derived light/dark answer — its resolved value is the standing dark fallback, and Phase 9 task 9-6 is what classifies the retained reply when a conversion first needs an answer.
- **Map the legacy `appearance` pref in `cmd/open.go`**, in memory only, where `prefsStore.LoadAppearance()` is read today: `auto` → `AdaptivePair(LoadBuiltin(theme.DefaultLightSlug), LoadBuiltin(theme.DefaultDarkSlug))`; `light` → `ConstantNomination(LoadBuiltin(theme.DefaultLightSlug))`; `dark` → `ConstantNomination(LoadBuiltin(theme.DefaultDarkSlug))`. **Bind the `theme` component once for the `cmd` package here** — `var themeLogger = log.For("theme")` at package scope, per CLAUDE.md's bind-once-*per-package* rule — and construct the loader with `theme.NewEventLogger(themeLogger)`; this is the §12.3 assignment for a path where a theme is *used* (no event is defined for this path until Phase 5's `theme: loaded`), and tasks 5-7, 6-6, 6-7 and 8-7 all reuse this var rather than calling `log.For("theme")` again. **Nothing is written to `prefs.json`** and `prefs.Appearance` / `LoadAppearance` are left in place; Phase 6 owns the persisted translation.
- **Re-point `cmd/capturetool`** to pass the **constant shape** (`ConstantNomination`) built from the `--appearance` pin's equivalent built-in, so its frames stay un-gated and byte-deterministic. `--appearance` itself survives one more task — task 3-4 replaces it with `--theme` and widens the resolution.
- Delete `tuiConfig.appearance` and its `Deps` mapping in `cmd/open.go`'s `buildTUIModel`, replacing them with the nomination field, and update `internal/capture`'s fixture `Deps()` plumbing accordingly.

**Acceptance Criteria**:
- [ ] `Deps.Appearance` and `WithAppearance` do not exist; a source guard proves `internal/tui` declares neither (they are **removed rather than left alongside**).
- [ ] `New()` with no theme option still yields a themed model — `activeTheme` carries `tokyo-night`'s values, not a zero `Theme` — and a render from it emits truecolor SGRs rather than a silently colourless frame.
- [ ] A constant nomination: the gate is resolved and unarmable at construction, `Init` issues **no** timeout tick, and the first frame paints the constant's canvas — no detection wait; **the OSC 11 query is still issued** and its reply still captured.
- [ ] `Init` issues the OSC 11 query under **every** setting shape — constant, adaptive pair and `NO_COLOR` — and a reply arriving at any time populates `originalBg`.
- [ ] After a constant launch on a light terminal the model holds the captured background and the fact that a reply arrived, with **no** light/dark answer derived from it — the constant's resolved gate value is the standing dark fallback and is not a classification of the terminal.
- [ ] An adaptive nomination: the gate is armed, the model paints nothing until it resolves, and `activeTheme` is unset-but-unused in the interval (no frame exists to render).
- [ ] The gate resolves exactly once: a `tea.BackgroundColorMsg` arriving **after** the timeout resolved it still populates `originalBg` and does **not** change `activeTheme`.
- [ ] Under `NO_COLOR` the gate is skipped, **both** members of an adaptive nomination are still loaded and held, and the dark member is selected.
- [ ] An `"appearance": "dark"` pin renders `tokyo-night`, `"light"` renders `tokyo-night-day`, and `auto` renders the pair — with `prefs.json`'s bytes unchanged after the run (and no file created when absent).
- [ ] `prefs.Appearance` and `prefsStore.LoadAppearance()` still exist and still decode tolerantly — their deletion is Phase 5–6.
- [ ] `cmd` binds the `theme` component exactly once — a single package-level `themeLogger` — and no other `log.For("theme")` call exists in the package.
- [ ] TUI construction reads **no** themes directory, no prefs theme keys, and enumerates nothing: with `PORTAL_THEMES_DIR` pointing at an unreadable path, construction succeeds and emits no `theme` event.
- [ ] `portal open <target>` (the exec path) constructs no TUI and performs no theme load — asserted by a test that runs the direct-path body with a `logtest.Sink` and a poisoned themes dir.
- [ ] `capturetool` passes the constant shape; every fixture still renders byte-identically to its task 3-1 output.
- [ ] No fallback path exists in this task — an unloadable nomination cannot arise because the source is the embedded set (§7.6's fatal message and §8.5's per-slot fallback stay in Phase 5).

**Tests**:
- `"it paints a constant nomination from frame one"` — `TestNomination_ConstantSkipsDetectionAndWait`
- `"it arms the gate for an adaptive nomination"` — `TestNomination_AdaptiveArmsTheGate`
- `"it selects the pair's member when the gate resolves"` — `TestNomination_GateSelectsMember` (dark reply → dark member, light reply → light member, timeout → dark member)
- `"it consumes a late reply without re-theming"` — `TestGate_LateReplyCapturesBackgroundButNeverReThemes`
- `"it issues the query under every setting shape"` — `TestGate_QueryIssuedRegardlessOfSettingShape` (constant, pair, `NO_COLOR`)
- `"it retains a constant launch's reply without classifying it"` — `TestGate_ConstantRetainsReplyWithoutClassifying`
- `"it loads both members and selects dark under NO_COLOR"` — `TestNoColor_LoadsBothAndSelectsDark`
- `"it maps a legacy appearance pin to the equivalent constant"` — `TestAppearanceMapping_PinToConstant` (table: dark, light, auto)
- `"it writes nothing to prefs.json while mapping"` — `TestAppearanceMapping_IsInMemoryOnly` (absent file stays absent; existing bytes unchanged)
- `"it reads no themes directory at construction"` — `TestConstruction_ReadsNoThemesDirectory`
- `"it does no theme work on the exec path"` — `TestOpenExecPath_DoesNoThemeWork`
- `"it removes the appearance injection outright"` — `TestDeps_HasNoAppearanceField`
- `"it still themes a model built without a nomination"` — `TestNew_SeedsTheDarkBuiltinWhenNoNominationIsGiven`

**Edge Cases**:
- A constant nomination skips detection and the first-paint wait **entirely** and is active from frame one — the real startup win §8.8 records.
- A pair carries **no provisional active member**, and the gate resolves before anything is painted, so there is no frame to render in the interval.
- The gate resolves **exactly once**: a late OSC 11 reply is still consumed for `restore.go`'s original-background capture but never re-themes, because under split a late flip swaps a whole named theme rather than a variant.
- **The query is issued from `Init` regardless of the setting shape** — a constant skips the gate, not the query. `restore.go` needs the reply for its original-background capture independent of detection, and §9.3's mid-session conversion needs it in hand; a constant path that skips the query breaks both.
- A constant's pre-resolved gate carries **no** detection-derived light/dark answer — its value is the standing dark fallback and must never be read as "the terminal is dark". Task 9-6 classifies the retained reply when a conversion first needs an answer.
- `NO_COLOR` skips the gate, still loads both nominations (so a later commit has something to persist against), and the standing dark no-answer fallback selects the member.
- The legacy `appearance` mapping is **in-memory only** — `auto` → the built-in pair, `light`/`dark` → the equivalent constant — and writes nothing to `prefs.json`.
- `prefs.Appearance` and `LoadAppearance` **survive this phase**; their deletion and the persisted `theme` keys are Phases 5–6.
- Construction reads no themes directory, no prefs theme keys and enumerates nothing, so `portal open <target>` still does zero theme work.
- An unloadable nomination cannot arise because the source is the embedded set — §8.5's per-slot fallback and §7.6's fatal startup message stay in Phase 5. Do not invent an interim fallback here.
- `capturetool` passes the constant shape so its frames stay un-gated and byte-deterministic.
- `Deps.Appearance` and `WithAppearance` are **removed rather than left alongside** — a dead option is a second injection path the harness and production could diverge on.
- The zero value of `Nomination` is neither state. `Build` always injects one; give the type a constructor-only contract (unexported fields) so a zero value cannot be constructed accidentally, and make `Select`/`Constant` on a zero value return a zero `Theme` rather than panic.
- `New`'s dark-built-in seed of `activeTheme` **survives** this task — only `Build`'s transitional pair holder is dropped. Without the seed, a model constructed without a nomination renders through `lipgloss.Color("")`'s no-colour sentinel: silently colourless, with no compile error and no failing assertion, which is precisely why task 3-1 added it.
- **Do not reach for `lipgloss.LightDark` or `compat.AdaptiveColor`** — both are in the tree Portal builds against, and either the `Token` collapse (task 3-1) or this gate is where an implementer meets them and reasonably asks why Portal hand-rolls a light/dark decision the library has an API for. Lipgloss v2 moving `AdaptiveColor` into `compat` reads at a glance as Charm deprecating paired colours, but the recommended replacement `lipgloss.LightDark(hasDarkBG)` **keeps paired values** and merely makes the detection explicit: what Charm de-recommended is *implicit detection*, not pairing. So the library's direction is **neutral on split, not supporting evidence for it**, and neither API can serve this gate — Portal selects between two *named themes*, which `LightDark` does not model. Hand-rolling the gate is the decision, not an oversight.

**Context**:
> §8.4: "**At construction Portal loads every *nominated* theme — at most two.** The light/dark gate then only **selects** between values already in hand… **The constructor therefore takes the loaded *nomination*, not a single theme.** One value covering both states: **Constant** — one loaded `Theme`, active from frame one; the gate is never consulted. **Adaptive** — both loaded `Theme`s, light and dark, and **no active member yet**."
> §8.8: what survives the appearance gate — "The `prefs.Appearance` **enum and its API**" dies; "the detect-or-timeout first-paint gate" survives but conditional; "the OSC 11 *query* itself" survives unchanged because `restore.go` needs it independent of detection. "**The gate resolves exactly once. A reply that arrives after the timeout has resolved it does not re-resolve it.** The reply is still *consumed*… but it never flips the active theme."
> §9.10: "**Under `NO_COLOR` the theme machinery still runs normally, unchanged.**… **Both nominated themes are still loaded** at construction… **The gate is skipped**, so the standing **dark** no-answer fallback selects the active member."
> §13.3: "**`tui.Build` takes the loaded *nomination* where it takes a `prefs.Appearance` today** — the exact injection mechanism this work removes… `capturetool` always passes the **constant shape**: a single pinned theme, no gate, no wait — which is what keeps captures byte-deterministic."
> §10.2's mapping (applied here in memory only, per the plan's phase note): `dark` → `tokyo-night`, `light` → `tokyo-night-day`, `auto` → nothing (the adaptive default is what `auto` meant).
> §12.3: emission is controlled by an injected logger — `cmd` passes a **real** component logger on the paths where a theme is used (TUI construction) and `log.Discard` on doctor, export and `capturetool`.
> §3.1: "**Lipgloss v2's direction is neutral on this decision, not supporting evidence.** Lipgloss v2 moved `AdaptiveColor` into `compat`, which reads at a glance as Charm deprecating paired colours — i.e. as independent support for split. It is not: the recommended replacement, `lipgloss.LightDark(hasDarkBG)`, **keeps paired values** and merely makes the detection explicit. What Charm de-recommended is *implicit detection*, not pairing, so **its direction is neutral on this decision.** This is a standing fact about a live dependency rather than a discarded option — both APIs are in the tree Portal builds against, and an implementer working through §3.2's collapse of `Token` or §8.8's surviving detect-or-timeout gate will meet them and reasonably ask why Portal hand-rolls a light/dark decision the library has an API for. The answer is that Portal's gate selects between two *named themes*, which `LightDark` does not model."
> Phase boundary: Phase 5 replaces this mapping with the persisted `theme` / `theme_light` / `theme_dark` keys, adds §8.5's fallback and §7.6's fatal message; Phase 6 adds the one-shot `appearance` translation and deletes `prefs.Appearance`.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §8.2, §8.4, §8.8, §9.10, §10.2, §12.3, §13.3

## theming-system-3-3

### Task 3.3: Retain the startup canvas hex and re-anchor the exit-time background restore

**Problem**: `RestoreTerminalBackground` derives its comparison value **at exit** from `m.canvasMode` via `canvasHexFor`, which reads the built-in canvas directly. Under a switchable theme that is wrong twice over: after task 3-2 there is no `theme.MV` to read, and comparing against whatever theme is *active at exit* would compare against a canvas the startup window never painted. This is the canvas-echo guard — the mechanic carrying CLAUDE.md's standing *"do not drop this guard"* warning — and it is the **one path where a mistake re-sticks a colour in the user's terminal after Portal exits**. §13.4's swap-and-diff guard structurally cannot cover it: the guard scans rendered fixture output, and this is an OSC 11 write that happens after the last render.

**Solution**: Capture the selected theme's canvas hex onto the model as `startupCanvasHex` at the moment the gate resolves, anchor `RestoreTerminalBackground`'s echo guard to that retained value, and delete `canvasHexFor` outright so nothing can re-derive the comparison from a theme.

**Outcome**: `RestoreTerminalBackground` compares against a hex captured once at gate resolution and never reads the active theme; the canvas-echo guard still skips the set-back for a terminal that echoes the canvas back; `canvasHexFor` no longer exists; and CLAUDE.md's `tui` row carries the warning forward re-anchored to the retained hex.

**Do**:
- Add `startupCanvasHex string` to `Model`, documented as: the canvas hex of the theme **the gate selected**, captured at the single moment the gate resolves — which is also the moment the first frame is composed — so it is defined for every frame that exists.
- Set it at the one place the active member is selected: in `syncResolvedMode` (adaptive) and on the constant/colourless construction path where the gate is built already resolved (task 3-2's shapes). Take it from `m.activeTheme.Canvas.Value` — the parsed, uppercase-canonical value — not from a re-read of the nomination.
- Re-anchor `RestoreTerminalBackground` (`internal/tui/restore.go`): replace `sameHexColour(original, canvasHexFor(m.canvasMode))` with `sameHexColour(original, m.startupCanvasHex)` and **delete `canvasHexFor`**. Leave `sameHexColour` / `normaliseHex6` / `isHexDigit` untouched — they already lower-case both sides and already return false for a non-hex (`rgb:`) reply so the caller falls through to emitting the set-back.
- Keep the doc comment's canvas-echo rationale and extend it with the new anchor: the comparison is against the canvas in force during the **startup** window, never the active theme, because a mid-session commit or an uncommitted preview would otherwise make the guard skip (or emit) the wrong set-back.
- Add the **named verification** §11.4 requires — a direct unit test on `RestoreTerminalBackground`, driven without fixtures — covering: the retained hex anchors the skip; mutating `activeTheme` after capture does **not** move the comparison; the case-insensitive and trailing-alpha reply shapes; and the non-hex reply still emitting the set-back. Migrate `TestRestoreTerminalBackground_CanvasEchoGuard` off `theme.MV.Canvas.Dark/Light` onto loaded built-ins plus a seeded `startupCanvasHex`.
- Assert both program-launch sites still restore identically — `cmd/open.go`'s post-`p.Run()` call and `cmd/capturetool`'s — with no behavioural difference between them.
- **Correct CLAUDE.md's `tui` row**: `restore.go` no longer paints a "mode-matched" canvas and the canvas-echo guard no longer compares "against the canvas hex" but against the **retained startup canvas hex**; `canvasHexFor` is gone. Carry the standing *"do not drop this guard"* warning forward verbatim in force, re-anchored to the retained hex. (§12.6 names this row as the one whose staleness is most dangerous, because it is what an implementer reads immediately before touching this code.)

**Acceptance Criteria**:
- [ ] `canvasHexFor` does not exist anywhere in the tree, and `RestoreTerminalBackground`'s body references no theme token or nomination — a source guard proves the comparison reads only `m.startupCanvasHex`.
- [ ] Under an adaptive nomination the hex is captured when the gate resolves and equals the **selected** member's canvas (dark reply → `tokyo-night`'s `#0B0C14`; light reply → `tokyo-night-day`'s `#E1E2E7`).
- [ ] Under a constant nomination it is captured at construction, before any frame.
- [ ] Mutating `m.activeTheme` after capture (the shape a mid-session commit or an uncommitted preview will take in Phases 8–9) does **not** change what `RestoreTerminalBackground` compares against.
- [ ] The canvas-echo guard still skips the set-back for `#0b0c14`, `#0B0C14`, `#0b0c14ff` and `0b0c14` against a canonical `#0B0C14` — `sameHexColour` is case-insensitive, so a canonicalised hex still matches a terminal's lower-case echo.
- [ ] A non-hex reply (`rgb:0b0b/0c0c/1414`) still falls through and emits the set-back unchanged.
- [ ] An empty capture (`OriginalBackground() == ""`) still writes nothing.
- [ ] Under `NO_COLOR` the hex is captured as normal from the selected member, no canvas is painted and no OSC 11 set is issued, so the set-back is a no-op — the anchor test needs no `NO_COLOR` case.
- [ ] Both launch sites (`cmd/open.go`, `cmd/capturetool/main.go`) call `RestoreTerminalBackground` with the program's output writer and behave identically.
- [ ] CLAUDE.md's `tui` row is corrected and still carries the "do not drop this guard" warning.

**Tests**:
- `"it captures the startup canvas hex when the gate resolves"` — `TestStartupCanvasHex_CapturedAtGateResolution` (dark reply / light reply / timeout)
- `"it captures the hex at construction under a constant"` — `TestStartupCanvasHex_ConstantCapturedAtConstruction`
- `"it compares against the retained startup hex, never the active theme"` — `TestRestoreTerminalBackground_AnchoredToStartupHex` (capture, then swap `activeTheme` to another built-in, then assert the skip/emit decision is unchanged)
- `"it skips the set-back on a canvas echo"` — `TestRestoreTerminalBackground_CanvasEchoGuard` (migrated: exact, uppercase, trailing alpha, no leading `#`)
- `"it still sets back for a non-hex reply"` — `TestRestoreTerminalBackground_NonHexReplyStillSetsBack`
- `"it writes nothing without a captured original"` — `TestRestoreTerminalBackground_EmptyWritesNothing` (existing, kept)
- `"it defines the hex but issues no set under NO_COLOR"` — `TestNoColor_HexCapturedAndSetBackIsANoOp`
- `"it keeps no theme reference in the restore path"` — `TestRestorePath_ReadsNoTheme`
- `"it restores identically from both launch sites"` — `TestLaunchSites_RestoreIdentically`

**Edge Cases**:
- The hex is captured **at gate resolution**, the same moment the first frame is composed, so it is defined for every frame that exists.
- If Portal dies before the gate resolves, nothing was painted and there is nothing to restore — the field is empty, `sameHexColour` returns false, and the set-back is emitted to the terminal's own original, which is a harmless no-op write.
- Under `NO_COLOR` the hex is captured as normal from the selected member but no canvas is painted and no OSC 11 set is issued, so the set-back is a no-op and the anchor needs no `NO_COLOR` case.
- The comparison must **never** read the *active* theme. Phase 4's named test covers the two divergence cases (a theme committed mid-session, and quit with an uncommitted preview active), neither reachable yet — **this phase proves the anchor, not the divergence** — so simulate a later swap by mutating `activeTheme` directly in the test.
- `sameHexColour` is case-insensitive, so a canonicalised `#0B0C14` still matches a terminal's `#0b0c14` echo. This is exactly why §4.3 canonicalises at parse.
- A non-hex OSC 11 reply (an `rgb:` form) still falls through to emitting the set-back unchanged — never worse than before.
- The swap-and-diff guard structurally cannot cover this (it scans rendered output; this is an exit-time OSC 11 write), which is why it keeps its own named verification.
- Both program-launch sites (`cmd/open.go` and `cmd/capturetool`) restore identically.
- CLAUDE.md's `tui` row is re-anchored here, carrying the standing "do not drop this guard" warning forward to the retained startup hex.

**Context**:
> §11.4: "`RestoreTerminalBackground` currently derives its comparison value *at exit* from `m.canvasMode` via `canvasHexFor`, which reads `theme.MV.Canvas` directly. Under a switchable theme that is wrong: it would compare against the *active* theme's canvas rather than the one in force during the startup window. **Required change:** **Capture and retain the startup canvas hex as model state**, and anchor `RestoreTerminalBackground`'s comparison to it. **Make `canvasHexFor` theme-agnostic** — no `theme.MV` reference. This is the mechanic carrying an explicit *'do **not** drop this guard'* warning, and the swap-and-diff guard structurally cannot cover it… **It therefore needs its own named verification.**… The stakes are why: this is the one path where a mistake re-sticks a colour in the user's terminal **after Portal exits**."
> §8.4: "**The retained startup canvas hex (§11.4) is captured from the theme the gate *selected***, not from what the constructor was handed — under adaptive those differ until the gate resolves." And: "§11.4's retained startup canvas hex is captured **when the gate resolves**, which is the same moment the first frame is composed — so it is defined for every frame that exists, and if Portal dies before then nothing was painted and there is nothing to restore."
> §11.3: "**The echo guard needs no new race handling.** It exists because the startup OSC 11 *query reply* can race Portal's own canvas set. The query is issued once from `Init`; a later theme switch issues no new query, so it creates no new race. The guard only ever needs to compare against the canvas active during the *startup* window."
> §12.6 on the `tui` row: "**This is the entry whose staleness is most dangerous** — it is the warning an implementer reads immediately before touching the exact code §11.4 changes."
> Phase boundary: Phase 4 owns the two divergence cases (mid-session commit; quit with an uncommitted preview), which need the panel to exist. This task delivers the anchor and its direct verification.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §11.3, §11.4, §8.4, §9.10, §12.6, §13.4

## theming-system-3-4

### Task 3.4: `capturetool --theme <slug|path>` replaces `--appearance`

**Problem**: `capturetool` still pins the canvas with `--appearance dark|light`, and its entire backing mechanism is gone: §8.8 deleted `prefs.Appearance` from the injection path and task 3-2 replaced it with a nomination, so there is no mode left to pin — **a theme *is* the mode**. Worse, the harness is the *only* viable route to seeing a visual change before release (§13.1: Portal cannot be run from a temporary build), and it is the only visual-verification route a **drop-in author** has — but today it can only ever render the compiled-in default, so somebody writing a `.theme` file has no way to look at it. Phase 2 re-pointed the standalone swatch branch to a `--theme <slug>`, deliberately leaving `--appearance` driving the `tui.Build` fixture path for exactly one phase; that coexistence ends here.

**Solution**: One `--theme` flag (default `tokyo-night`) driving **both** capturetool branches, accepting a built-in slug **or an explicit path to a real theme file**, with slug-versus-path discriminated by a path separator or the `.theme` suffix, invalid content a hard error, and the two filename reasons emitted as non-blocking stderr warnings on the path form only.

**Outcome**: `go run ./cmd/capturetool --fixture <name> --theme <slug|path>` renders any built-in or any hand-authored file; an invalid file fails loudly at exit 1 with its §6.2 reason rather than silently rendering the wrong colours at a visual gate; and `--appearance` no longer exists.

**Do**:
- **Add the explicit-path loader entry to `internal/theme`**: `func (l Loader) LoadPath(path string) (Result, *Rejection)` — read the bytes and run the **content** rungs only (`unreadable` → `bad syntax` → `bad colour` → `missing tokens`) via the shared `parseThemeBytes`, deriving **no** slug and running neither filename rung. Document why: an explicit path is an *input*, not a directory entry, and §3.2 gives `Theme` no identity field, so a theme loaded from a path has no slug. `LoadFile` stays the directory-entry path that runs the full §6.2 ladder.
- **Replace the flag** in `cmd/capturetool/main.go`: `--theme` (default `tokyo-night`) in place of `--appearance`; delete `resolveAppearance`. Thread the resolved value through `resolveProgram` so it drives **both** branches — `resolveModel`'s `tui.Build` fixture path (as the **constant** nomination shape: no gate, no wait, byte-deterministic) and the `capture.NewContrastValidationModel` swatch branch Phase 2 already re-pointed.
- **Discriminate slug from path**: the argument is a **path** if it contains a path separator **or** ends in `.theme`; otherwise a **slug**. So `nord` is a slug while `nord.theme`, `./nord.theme`, `/abs/nord.theme`, `sub/dir/x` and `./mytheme.txt` are all paths. Record in a comment why the separator half is load-bearing: without it a real file with an unexpected extension classifies as a slug and is rejected as an unknown built-in — an error naming the wrong problem for a file that plainly exists.
- **Slug branch**: `LoadBuiltin`; `found == false` or a rejection is a hard error naming the slug (and the §6.2 reason where present), exit 1. Emit **no** filename warnings — a slug argument names a built-in by design, so a `reserved name` warning would fire on the normal documented invocation.
- **Path branch**: `LoadPath`; any rejection is a hard error carrying the §6.2 reason, exit 1, **never a fallback**. Then derive a **candidate slug** from `filepath.Base` via `SlugFromFilename`, used **solely** for warnings and never as identity: a `bad name` (either cause) and a `reserved name` (candidate slug present in `theme.BuiltinSlugs()`) each print one warning line to **stderr** and do **not** block. Emit them before the Bubble Tea program starts, so they land on the primary screen and are still visible after the alt screen is left.
- Construct the loader with `theme.NewEventLogger(log.Discard())` so `capturetool` emits no `theme` events (§12.3's fifth caller).
- Keep the `NO_COLOR` env carve-out on the fixture path exactly as it is — it is read after the theme resolves and **wins** over the pinned theme (no canvas to select).
- Update `cmd/capturetool`'s existing tests (`main_test.go`, `swatch_test.go`, `shared_constructor_test.go`) to the new flag, and re-confirm the two guards: `internal/capture` still reaches no `internal/xdg` / `cmd` / prefs, and the portal binary still does not import `internal/capture`. Any tape written from here on uses `--theme`.

**Acceptance Criteria**:
- [ ] `--appearance` no longer exists on `capturetool`, and no code path resolves a `prefs.Appearance`.
- [ ] Omitting `--theme` renders `tokyo-night` — the shipped dark default every flagless capture depends on.
- [ ] Slug/path discrimination is exactly as tabled: `nord` → slug; `nord.theme`, `./nord.theme`, `/abs/nord.theme`, `sub/x`, `./mytheme.txt` → path.
- [ ] `--theme ./broken.theme` with a duplicate key exits non-zero with `bad syntax`; with a bad hex, `bad colour`; missing a token, `missing tokens`; unreadable, `unreadable`. **Nothing renders on any of them.**
- [ ] `--theme not-a-theme` exits non-zero naming the slug; no fallback render occurs on any failure path.
- [ ] `--theme ./Nord.THEME` (a valid-content file with a fatal filename) **renders**, with a `bad name` warning on stderr; `--theme ./nord.theme` renders with a `reserved name` warning; `--theme nord` renders with **no** warning.
- [ ] A theme loaded from a path carries **no slug** — `Result.Slug` is empty and nothing downstream derives identity from the filename.
- [ ] Both branches are driven by the flag: the fixture path receives the **constant** nomination shape (no gate, no wait), and the swatch renders the same resolved theme.
- [ ] `NO_COLOR=1` still renders the colourless native-bg frame and wins over the pinned theme.
- [ ] Resolving a theme in `capturetool` produces zero records through a `logtest.Sink`.
- [ ] `internal/capture` performs no XDG lookup and no prefs read; `cmd/capturetool`'s import guard and `TestPortalBinaryDoesNotImportCapture` both pass.

**Tests**:
- `"it defaults to tokyo-night"` — `TestResolveTheme_DefaultsToTokyoNight`
- `"it discriminates a slug from a path"` — `TestThemeArg_SlugVersusPath` (table: `nord`, `nord.theme`, `./nord.theme`, `/abs/nord.theme`, `sub/dir/x`, `./mytheme.txt`)
- `"it hard-errors on an unknown slug"` — `TestResolveTheme_UnknownSlugIsAnError`
- `"it hard-errors on every content reason for a path"` — `TestResolveTheme_PathContentReasonsAreHardErrors` (table: `bad syntax`, `bad colour`, `missing tokens`, `unreadable`)
- `"it never falls back to another theme on failure"` — `TestResolveTheme_NoFallbackOnFailure`
- `"it warns but renders for a fatal filename"` — `TestResolveTheme_PathBadNameWarnsWithoutBlocking`
- `"it warns but renders for a reserved candidate slug"` — `TestResolveTheme_PathReservedNameWarnsWithoutBlocking`
- `"it never warns for a slug argument"` — `TestResolveTheme_SlugFormNeverWarns`
- `"it loads a path with no slug"` — `TestLoadPath_DerivesNoSlugAndRunsNoFilenameRung`
- `"it drives the fixture path as a constant nomination"` — `TestResolveModel_PassesConstantNomination`
- `"it lets NO_COLOR win over the pinned theme"` — `TestResolveModel_NoColorWinsOverTheme`
- `"it emits no theme log records"` — `TestCaptureTool_ThemeResolutionIsSilent`
- `"it keeps the harness free of config discovery"` — existing `internal/capture` / `cmd/capturetool` import guards

**Edge Cases**:
- Default `tokyo-night` when the flag is omitted, on which every capture taken without it depends.
- Slug versus path is discriminated by a path separator **or** the `.theme` suffix; without the separator half a real file with an unexpected extension would be classified as a slug and rejected as an unknown built-in, naming the wrong problem for a file that plainly exists.
- Only the **content** reasons apply to a path — `bad syntax`, `bad colour`, `missing tokens`, `unreadable`.
- Invalid input is a **hard error** carrying the §6.2 reason at non-zero exit, never a fallback, because silently rendering the wrong theme at a visual gate is the failure the tool exists to prevent.
- A candidate slug is derived from the basename **solely** to produce the filename warnings and is never used as identity — a theme loaded from a path has no slug.
- `bad name` and `reserved name` warn on stderr **without blocking**, and apply to the **path form only**: a slug argument names a built-in by design, and blocking would break §12.1's published export-then-rename workflow (an exported built-in is a reserved slug until the user renames it).
- The flag drives **both** branches — the `tui.Build` fixture path as the constant nomination shape, and the swatch branch Phase 2 already re-pointed.
- `--appearance` is **removed, not kept alongside**, because a theme *is* the mode.
- The `NO_COLOR` env carve-out still applies to the fixture path and wins over the pinned theme.
- The loader is handed `log.Discard` so `capturetool` emits no `theme` events.
- An explicit path is an **input rather than config discovery**, so `internal/capture`'s no-real-config import guard is untouched — no XDG lookup, no prefs read.

**Context**:
> §13.3: "**`capturetool` gains a `--theme` flag, replacing `--appearance`.** `--theme` accepts a built-in slug **and an explicit path to a real theme file**. An explicit path from a flag is an **input, not config discovery**, so the `internal/capture` no-real-config import guard's invariant is preserved… This matters disproportionately: it is the only visual-verification route for someone authoring a drop-in."
> §13.3's sub-rules, verbatim in substance: default `tokyo-night`; "**Slug versus path is discriminated by a path separator *or* the `.theme` suffix**"; "**Only the content reasons apply to a path**"; "**Invalid input is a hard error**… never a fallback"; "**A candidate slug *is* derived from the basename, solely to produce the warnings below**, and never used as identity"; "**The filename reasons — `bad name` and `reserved name` — warn on stderr but do not block, and apply to the path form only.**"
> §13.3: "**`--appearance` is removed, not kept alongside.** It exists today (`dark|light`, resolving to a pinned `prefs.Appearance`), and its entire backing mechanism — `prefs.Appearance` and `WithAppearance` — is deleted by §8.8. There is no mode left to pin; a theme *is* the mode."
> §13.1: "**Portal cannot be run from a temporary build to check a visual change.**… So `capturetool` is not a convenience; it is the **only viable route** to seeing a visual change before release."
> §12.3: `capturetool` is the fifth caller of the loader and "neither uses nor diagnoses a theme — it is an offline renderer whose output is a frame, so emission would be noise, and `Discard` also leaves its per-process dedup state owned rather than dangling."
> Phase boundary: Phase 2 re-pointed the swatch branch and deliberately left `--appearance` driving `tui.Build` for one phase; this task ends that coexistence. The panel fixtures, their four inputs and the `ThemeEnumerator` seam are **Phase 8**.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §13.3, §13.1, §12.1, §12.3, §6.2, §8.8

## theming-system-3-5

### Task 3.5: Nord's grouped capture and the outstanding `text.subtle` visual gate

**Problem**: §7.4 closes the Nord port with **one gate still open**: `text.subtle` has no locus on any captured Nord frame. It renders the group `··· N` counts and the loading page's pending steps, neither of which appears on a flat Sessions screen — so the port shipped in Phase 2 with its `text.subtle` checked only numerically. That value is an **invention**, not a palette lift: Nord's greys are barrelled at the ends (1.24 / 1.45 / 1.69 dark, 9.25 / 10.26 / 10.84 bright, nothing between), so `#73819B` was interpolated on nord3's hue and saturation and measures **3.177** against `#2E3440` — inside the 3.00–4.49 band, but with 0.177 of headroom on the palette whose canvas is a mid-dark and whose contrast headroom is the tightest in the shipped set. Numbers cannot answer whether it reads as *de-emphasised but still readable*; only an eyeball on a grouped frame can, and until this phase the render layer could not take a `Theme` at all, so the capture was impossible to produce.

**Solution**: Produce a grouped Nord capture through `capturetool --theme nord`, verify the file was actually written, put it in front of the human gate with its live-view command and the Nord Paper frame, and record the outcome — re-deriving the invention at a fresh visual gate (never by arithmetic) if it is rejected.

**Outcome**: The last outstanding judgement from the Nord port is settled with a recorded finding, and either `nord.theme` stands unchanged or its `text.subtle` carries a new eyeball-settled value inside the 3.00–4.49 band with Phase 2's auto-enumerated floors re-run green.

**Do**:
- **Write the tape.** Add `testdata/vhs/sessions-by-project-nord.tape` driving `go run ./cmd/capturetool --fixture sessions-by-project --theme nord` (task 3-4's flag). Per §13.2 the tape and its PNG are **scaffolding** — created as work proceeds, committed while being collaborated on, cleared at sign-off in Phase 10 — not durable assets.
- **Confirm the locus is actually on the frame before capturing.** `text.subtle` renders the group header's `··· N` count (`HeaderItem`'s `countText()` in `internal/tui/session_item.go`). Add a cheap unit assertion that the grouped render contains that segment carrying `nord`'s `text.subtle` value, so the gate can never be taken on a frame where the token is invisible. If `sessions-by-project` renders no counts for the fixture data, use `sessions-by-tag`; the loading screen's pending steps are the secondary locus if a second frame helps the judgement.
- **Capture, and verify a fresh write.** VHS is known to fail silently on write — it runs the tape, reports no error, and does not produce the PNG. Hash the target file before and after, confirm the hash changed, and retry on failure. **Never pixel-check or review a capture whose write was not verified** — a stale image reads as a false pass and this task's entire signal is the image.
- **Take the human visual gate.** Present the capture inline, alongside the live-view command `go run ./cmd/capturetool --fixture sessions-by-project --theme nord` (unprompted) and the `Sessions — Nord (port)` Paper frame as the reference. The question is narrow: does `#73819B` read as de-emphasised but still readable against `#2E3440` at the group counts, sitting correctly *below* `text.muted` (`#939EB2`, already seen on the same frame) and *above* `text.faint` (`#4C566A`)?
- **Record the outcome either way** — a passing gate is a finding, not a non-event. Note it in the task's commit message.
- **If the gate rejects it**: re-derive as an **invention** — settled visually, not by arithmetic (its only constraints are landing in the right band and looking right, which is exactly how `bg.attention` was settled). Land the new value in `internal/theme/builtins/nord.theme` with its `#` comment updated to record the derivation and the fresh gate, keep it inside the **3.00–4.49** band against `#2E3440`, then re-run Phase 2's auto-enumerated floor suite over the whole embedded set and re-capture + re-gate. **No floor is relaxed and no carve-out is granted for a named palette.**

**Acceptance Criteria**:
- [ ] The capture is produced through `capturetool --theme nord` on a **grouped** fixture — task 3-4 must have landed, since `--appearance` cannot express a theme.
- [ ] The captured frame demonstrably contains a `text.subtle` locus (a group `··· N` count), proven by the render assertion rather than by eye alone.
- [ ] The PNG's hash is verified to have changed on this run; a silent VHS non-write is retried, never reviewed.
- [ ] The human gate was taken with the capture shown inline, the `capturetool --fixture … --theme nord` command given, and the Nord Paper frame available as reference.
- [ ] The outcome is recorded in the task's commit message — pass or re-derive.
- [ ] If the value moved: `nord.theme` carries the new hex with an updated `#` comment recording the derivation and the fresh gate; the value measures ≥ 3.00 and < 4.50 against `#2E3440`; Phase 2's floor suite passes over all three built-ins with nothing relaxed.
- [ ] `text.muted` is untouched — it was already seen on `Sessions — Nord (port)` and is out of scope.
- [ ] `nord`'s enrolment (dark) and the four light pins are unaffected — `text.subtle` is neither a pinned tint nor light-theme scoped.
- [ ] The tape and PNG are committed as scaffolding with a note that they clear at sign-off in Phase 10; the Go fixture definitions stay untouched.

**Tests**:
- `"it renders the subtle-count locus on a grouped frame"` — `TestGroupedRender_CarriesTextSubtleCountLocus` (grouped render under `nord` contains the `···` count segment styled with `text.subtle`'s value)
- `"it holds text.subtle inside the 3.00–4.49 band"` — `TestTextSubtleBand` (Phase 2's auto-enumerated floor, re-run; must stay green if the value moves)
- `"it clears every floor rule for every built-in"` — Phase 2's `internal/theme` floor suite, re-run after any change
- `"it records the derivation beside the value"` — `TestNordFile_CorrectionsAndInventionsCarryComments` (Phase 2's file-comment assertion, updated if the value moves)

**Edge Cases**:
- `text.subtle` has **no locus on a flat Sessions frame** — it renders group `··· N` counts and pending loading steps — so a grouped capture is the only surface that can carry the gate.
- `text.muted` has already been seen on `Sessions — Nord (port)` and is out of scope for this gate.
- A failure re-derives an **invention**, so the new value is settled at a fresh visual gate rather than by arithmetic, lands back in `nord.theme` with its `#` comment, and must stay inside the **3.00–4.49** band against `#2E3440` (3.177 today).
- A moved value re-runs Phase 2's auto-enumerated floors rather than relaxing any, and **no carve-out is granted for a named palette** — this being the first external port, a carve-out sets the precedent for every PR theme after it.
- VHS is known to **fail silently on write**, so verify the file's hash changed and retry before trusting or reviewing the capture; a stale or absent image reads as a false pass, and there is no functional assertion that would catch it.
- The tape and image are scaffolding under §13.2's retention rule — created as work proceeds and cleared at sign-off in Phase 10, not committed assets.
- The capture is taken through `capturetool --theme nord`, so task 3-4 must have landed.
- The human gate is the deciding instrument, and **a passing gate is still a finding to record**.

**Context**:
> §7.4: "**Outstanding visual gate:** `text.subtle` has no locus on any captured Nord frame — it renders group `··· N` counts and pending loading steps, neither of which appears on the flat Sessions frame. **It needs a visual gate at implementation, on a grouped Nord capture.** (`text.muted` has already been seen — it is the 'N window(s)' text on `Sessions — Nord (port)`.)"
> §7.4 on inventions: "**Invention 1 & 2 — the ramp's middle.** Nord's greys are barrelled at the ends… Portal needs `text.muted` ≥ 4.5 and `text.subtle` in the 3.0–4.5 band, so both are interpolated on nord3's hue and saturation." And on derivation method: "An **invented** value has no source to preserve; its constraints are landing in the right band and looking right, which is why `bg.attention` was settled at a visual gate rather than by arithmetic."
> §7.4: "**A failure on an unwalked leg can force re-deriving an *invented* value — which then needs a fresh visual gate.**… if it lands on `text.muted`, `text.subtle` or `bg.attention`, the new value is an *invention*, and this port's own precedent is that inventions are settled at a visual gate rather than by arithmetic."
> §13.5: `text.subtle` carries a band, not a floor — "**3.00–4.49** — it must clear the UI floor *and* stay below normal text, or it is not de-emphasised."
> §13.1: the harness is two mechanisms for two audiences — a producible PNG per fixture for the agent ("**Without a producible PNG the agent cannot see what it built**") and `capturetool --fixture` in a real terminal for the human, "loaded in a real terminal at the human-in-the-loop gate and judged as the real thing".
> §13.3: "**The harness is known to fail silently on write, and this feature is unusually exposed to it.**… **Mitigation, procedural and mandatory: verify a fresh write before trusting or reviewing a capture** — confirm the file's hash changed — and retry on failure."
> §13.2: "**From this feature forward, captures and the tapes that produce them are created as work proceeds, committed while they are being collaborated on, and cleared out after sign-off**… A tape is scaffolding on the same terms as the image it renders."
> Phase boundary: the plan places this gate in Phase 3 rather than Phase 2 because a grouped Nord capture needs the render layer to take a `Theme` first; Phase 2's swatch gate covered Nord's tints, corrections and the `bg.attention` invention. `docs/theming.md`'s Nord attribution and correction record is **Phase 10**.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §7.4, §13.1, §13.2, §13.3, §13.5
