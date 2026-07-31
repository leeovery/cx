# Phase 2: The three built-in themes and `portal theme export` — 10 tasks

## theming-system-2-1

### Task 2.1: Embed the built-in theme set and ship `tokyo-night.theme`

**Problem**: Phase 1 built a loader with no content. Portal's dark palette still lives in Go as `theme.MV`'s `Dark` fields, which means a built-in is a *struct* and a user's drop-in is a *file* — two shapes, two code paths, and a format nobody dogfoods. §7.1 decides the opposite: **a built-in *is* a `.theme` file**, embedded via `go:embed` and parsed by the same loader as a stranger's, so "copy a built-in and edit it" is a first-class workflow, a PR is "add a file", and a bad format is the maintainer's problem on day one. The consequence that has to be handled in the same breath is that parse failures move from compile time to load time, and the built-in lookup must stay reachable **with no path at all** or `internal/capture`'s no-real-config import guard and `capturetool`'s offline render both break.

**Solution**: An `internal/theme/builtins/` directory of `.theme` files embedded through a single `go:embed` directive, with `BuiltinBytes` / `BuiltinSlugs` / `Loader.LoadBuiltin` reusing Phase 1's lex-and-validate path verbatim, and the first file — `tokyo-night.theme` — carrying §7.3's dark table as 19 keys.

**Outcome**: `LoadBuiltin("tokyo-night")` returns the same shape `LoadFile` returns for a drop-in, built from bytes that are byte-identical to the committed file, with no filesystem access and no directory argument anywhere on the path.

**Do**:
- Refactor Phase 1's `LoadFile` so rungs 4–6 live in one unexported `parseThemeBytes(data []byte) (Theme, *Rejection)` — lex, then `themeFromPairs`. `LoadFile` calls it after its `os.ReadFile`; `LoadBuiltin` calls it on the embedded bytes. **No second parse path exists**, and a test may not be able to tell the two callers apart by behaviour.
- Create `internal/theme/builtins.go`: `//go:embed builtins/*.theme` over `var builtinFS embed.FS`; `func BuiltinBytes(slug string) ([]byte, bool)` reading `builtins/<slug>.theme` from that FS (returning a copy, never the shared slice); `func BuiltinSlugs() []string` reading the embedded directory and returning slugs sorted, derived from the filenames rather than a hand-written list.
- Add `func (l Loader) LoadBuiltin(slug string) (Result, *Rejection, bool)` — the third return is *found*, so an unknown slug is a clean "not a built-in" outcome (nil rejection) that Phase 5's by-name resolution falls through on. Extend Phase 1's `Result` with `Source []byte` — the exact bytes parsed — populated by both `LoadFile` and `LoadBuiltin`, documented as existing so `portal theme export` (task 2-9) prints what it validated without a second read.
- Author `internal/theme/builtins/tokyo-night.theme`: a `#` header block naming the palette and its upstream source link (take the real URL from the upstream project at implementation — do not invent one), then the **19** keys of §7.3's dark table, values written verbatim as the spec lists them (MV's mixed case included — the parser is what canonicalises). `border = #292E42` (the former `border.separator` value); **no `border.footer` key**.
- Add two source guards to the package's existing guard test file: `internal/theme`'s non-test `.go` files declare **no `#RRGGBB` literal** (values live only in `.theme` files — this is what retires the colour-literal guard's exemption in Phase 3), and the package declares **no `func init()`** and no package-level var whose initialiser parses (nothing walks the embedded set at init, §7.6).

**Acceptance Criteria**:
- [ ] `LoadBuiltin("tokyo-night")` returns no rejection, `found == true`, and a `Theme` whose 19 tokens carry §7.3's dark values **uppercase-canonical** — `Canvas.Value == "#0B0C14"`, `BgSelection.Value == "#28243A"`, `BgSubtle.Value == "#26283A"`.
- [ ] `LoadBuiltin("no-such-theme")` returns `found == false` with a nil rejection and touches no filesystem — the signature takes one string and no directory, proven at compile level and by running the test with `PORTAL_THEMES_DIR` pointing at a path that does not exist.
- [ ] `BuiltinBytes("tokyo-night")` is byte-identical to `os.ReadFile("internal/theme/builtins/tokyo-night.theme")`, comments and trailing newline included.
- [ ] `BuiltinSlugs()` derives from the embedded filenames (a new file appears without editing Go), returns them sorted, and contains `tokyo-night`.
- [ ] `parseThemeBytes` on deliberately broken bytes returns an ordinary `*Rejection` — never a panic — for each of `bad syntax`, `bad colour` and `missing tokens`.
- [ ] `tokyo-night.theme` contains exactly 19 `key = value` lines, no `border.footer`, and a header comment; it parses through `LoadFile` too (written to a temp dir under a legal filename) with an identical `Theme`.
- [ ] The source guards pass: no hex literal in non-test `internal/theme` Go source, no `func init()`.
- [ ] `internal/tui/theme` is untouched, `go build ./...` and `go test ./...` are green, and `cmd/capturetool`'s import guard still passes.

**Tests**:
- `"it loads an embedded built-in through the same parse path as a file"` — `TestLoadBuiltin_UsesTheSharedParsePath` (asserts `LoadFile` on a copy of the file and `LoadBuiltin` produce equal `Theme`s)
- `"it returns not-found for an unknown slug without touching the filesystem"` — `TestLoadBuiltin_UnknownSlugIsNotFound`
- `"it returns the embedded bytes verbatim"` — `TestBuiltinBytes_MatchesCommittedFile`
- `"it derives the built-in slugs from the embedded filenames"` — `TestBuiltinSlugs_DerivedAndSorted`
- `"it canonicalises the dark palette's lowercase hexes"` — `TestLoadBuiltin_TokyoNightValuesAreUppercaseCanonical`
- `"it ships nineteen keys with no border.footer"` — `TestTokyoNightFile_HasNineteenKeysAndNoBorderFooter`
- `"it returns an ordinary rejection for broken embedded-shaped bytes"` — `TestParseThemeBytes_BrokenInputIsARejectionNotAPanic`
- `"it declares no hex literal in Go source"` — `TestThemePackage_DeclaresNoHexLiterals`
- `"it walks nothing at package init"` — `TestThemePackage_HasNoInitFunction`

**Edge Cases**:
- Embedded files parse through the **same** loader as a drop-in — no second code path, so a format bug cannot hide behind a Go-side built-in.
- An embedded parse failure returns an ordinary error, never a panic; §7.6's escalation happens where a fallback is *needed* (Phase 5), not here.
- Nothing walks the embedded set at init — no startup-eager validation; task 2-8's test is what proves the set at build time.
- An unknown slug returns a not-a-built-in outcome without touching the filesystem, which is what lets Phase 5 fall through to the themes directory cleanly.
- The built-in lookup takes **no directory path at all**, so `internal/capture`'s no-real-config import guard stays satisfied — `go:embed` is not config discovery.
- `internal/theme` declares no hex values of its own; every value lives in a `.theme` file.
- `border` carries the former `border.separator` value and `border.footer` is dropped, so the file is 19 keys, not 20.
- MV's lowercase dark hexes (`#0b0c14`, `#28243a`) canonicalise to uppercase at parse, so §11.3's background diffing and §11.4's retained startup canvas hex compare equal later.

**Context**:
> §7.1: "Built-in themes are `.theme` files embedded via `go:embed` and parsed by the **same loader** as a user's drop-in. They are not Go structs." One code path, one format, one validity rule. Prior art: Ghostty and kitty avoid inventing a theme format at all — a theme *is* a config file. Consequences named in the same section: parse failures move to load time (hence §7.6), and `internal/capture`'s import guard is preserved because `go:embed` is not config discovery.
> §7.3's dark table, verbatim: `canvas #0b0c14`, `text.primary #C0CAF5`, `text.secondary #A9B1D6`, `text.tertiary #828BB8`, `text.muted #737AA2`, `text.subtle #535C86`, `text.faint #3B4261`, `text.on-selection #FFFFFF`, `border #292E42`, `text.on-attention #E8C9A0`, `accent.primary #BB9AF7`, `accent.key #7AA2F7`, `accent.mode #7DCFFF`, `accent.attention #FF9E64`, `state.positive #9ECE6A`, `state.destructive #F7768E`, `bg.selection #28243a`, `bg.attention #241B10`, `bg.subtle #26283A`.
> §7.1 also decides that **MV's inline erratum comments are deleted, not ported** — `contrast_test.go` enforces the corrected values numerically, so a comment recording why a hex differs from upstream is duplicated history. The one class of judgement that is *not* numerically recoverable (the four eyeball-pinned **light** tints) moves into the theme file as a `#` comment — that is task 2-5's file, not this one. This dark file therefore carries attribution only.
> Phase boundary: `internal/tui/theme` and `theme.MV` stay in place and in use — the render-layer rename and the old package's deletion are **Phase 3**.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §7.1, §7.2, §7.3, §7.6, §2.2, §3.2

## theming-system-2-2

### Task 2.2: Reserve the built-in slugs — the no-shadowing safety property

**Problem**: An invalid theme falls back to a built-in (§8.5). If a user file could **shadow** the built-in that is the fallback, the fallback itself could be broken — drop `tokyo-night.theme` into `themes/` with a typo'd hex and the thing Portal falls back to is the same broken file. That must be impossible. Phase 1 built the `reserved name` rung against an **injected, empty** slug set precisely so this task could populate it from real content; until it is populated the rung is inert and the safety property does not exist.

**Solution**: Derive the loader's reserved slug set from `BuiltinSlugs()` — the embedded set itself, never a hand-maintained list — so the rung decides from the slug alone before any read, and a built-in added by a later PR reserves its own slug automatically.

**Outcome**: A drop-in whose slug collides with a built-in is rejected `reserved name` without the file ever being opened, `Nord.theme` still fails `bad name` at rung 1 and never reaches the reserved rung, and a deliberately broken `themes/tokyo-night.theme` cannot change what `LoadBuiltin("tokyo-night")` returns.

**Do**:
- Populate the `Loader`'s reserved-slug source in its production constructor from `BuiltinSlugs()` (build the set once per loader, not per file). Keep the field injectable exactly as Phase 1 left it — this task **populates the seam, it does not replace it** — so a test can still drive the rung with a synthetic set and Phase 1's empty-set behaviour stays reachable.
- Document in-source that the set is *derived*: a future built-in file reserves its slug with no Go edit, which is the property that keeps §5.4's guarantee true as the set grows.
- Document in-source the property being protected, in one sentence: the fallback built-in can never be the user's broken file.
- Confirm rung ordering is unchanged — `bad name` (rung 1) still precedes `reserved name` (rung 2), so a filename whose slug is illegal never reaches the collision check and cannot be normalised into one.
- Add the safety-property test as an explicit named case rather than leaving it implied by the rung test (below).

**Acceptance Criteria**:
- [ ] A `themes/tokyo-night.theme` whose contents are **perfectly valid** is rejected `reserved name`.
- [ ] The same is true when the file is unreadable (mode `0000`) or absent — the rejection is decided from the slug alone, before any read, so it cannot report `unreadable`.
- [ ] `Nord.theme` beside the built-in `nord` reports `bad name`, never `reserved name`, and never yields the slug `nord` — safe on a case-insensitive macOS filesystem.
- [ ] The reserved check is **exact string equality**; no `strings.EqualFold`, no lowercasing, anywhere on the path.
- [ ] Every member of `BuiltinSlugs()` reserves — the test loops the enumerated set and names no theme, so a built-in added later is covered without editing the test.
- [ ] Safety property: with a directory containing a `tokyo-night.theme` that has a broken hex, `Enumerate` reports that entry as `reserved name` **and** `LoadBuiltin("tokyo-night")` still returns the embedded, valid theme.
- [ ] `nord-lee.theme` (§5.4's published workaround) loads normally — the escape hatch works.
- [ ] A `Loader` constructed with an explicitly empty injected set still never rejects as reserved (Phase 1's behaviour is preserved for tests).

**Tests**:
- `"it rejects a drop-in whose slug collides with a built-in"` — `TestLoadFile_ReservedSlugRejected`
- `"it decides reserved name without opening the file"` — `TestLoadFile_ReservedDecidedBeforeRead` (unreadable and absent variants)
- `"it never lowercases a filename into a collision"` — `TestLoadFile_MixedCaseFilenameIsBadNameNotReserved`
- `"it reserves every embedded slug"` — `TestReservedSet_CoversEveryBuiltinSlug` (loops `BuiltinSlugs()`)
- `"it cannot have its fallback shadowed by a broken drop-in"` — `TestNoShadowing_BrokenDropInCannotReplaceBuiltin`
- `"it accepts the renamed workaround file"` — `TestLoadFile_RenamedCopyIsAccepted`
- `"it still never rejects when the injected set is empty"` — `TestLoadFile_EmptyInjectedReservedSetNeverRejects`

**Edge Cases**:
- Decided from the slug alone before any read, so a colliding file is never opened.
- Exact string equality, never case-insensitive — `Nord.theme` fails `bad name` at rung 1 and never reaches the reserved rung, so it cannot shadow `nord` on a case-insensitive macOS filesystem.
- A `bad name` file has no slug, so `reserved name` is structurally unreachable for it.
- The reserved set derives from the embedded set itself, not a hand-maintained list, so a future built-in reserves its slug automatically.
- The property being protected is that the fallback built-in can never be the user's broken file.
- Phase 1's injected empty slug source is the seam being **populated, not replaced** — the injection point survives for tests.

**Context**:
> §5.4: "**A user file whose slug collides with a built-in is rejected**, with reason `reserved name`, through the same channel as any other invalid theme. This exists because of a hard constraint: an invalid theme falls back to a built-in, so **if a user file could shadow the built-in that is the fallback, the fallback itself could be broken.**" Rejected alternatives: user-dir-shadows-built-ins (needs a reserved-name special case, a precedence chain and "which `nord` am I looking at?" ambiguity) and built-ins-always-win-silently (you edit a file and nothing happens, with no signal at all).
> §5.2: reject, never normalise — "This removes the case question outright rather than defining case-insensitive matching, so the reserved-name check stays **exact string equality**."
> §5.4's accepted consequence: because built-in rows are deliberately indistinguishable from drop-in rows in the panel, the reserved-slug set is not discoverable from the UI — a user learns a slug is reserved by having their file rejected. `portal theme export` (tasks 2-9/2-10) and `docs/theming.md` make the set discoverable outside the panel.
> §8.4 is the other half of the same guarantee and lands in **Phase 5**: construction resolves the embedded set *before* the themes directory, so on the non-enumerating by-name path the safety property comes from ordering rather than collision detection. This task covers the enumerating path.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §5.4, §5.2, §6.2, §8.4, §8.5

## theming-system-2-3

### Task 2.3: Auto-enumerate §13.5's contrast floors over the embedded set

**Problem**: The bundled tier's whole promise is that what Portal *ships* is valid **and good** (§6.4) — floors, bands and pairing legs checked numerically, with no carve-out for a named palette. Today's floor tests cannot serve that: they read `theme.MV.<Field>`, address `.Dark`/`.Light`, run a `/dark` + `/light` subtest pair per token, and measure against the `canvasDark`/`canvasLight` constants — four things this feature removes. Worse, they **name** the one palette they check, so the two palettes landing later in this phase would ship unchecked unless someone remembered to add them.

**Solution**: A new `internal/theme/contrast_test.go` implementing §13.5's canonical rule set as a loop over `BuiltinSlugs()` × tokens, resolving every reference background from **the theme's own `canvas`**, so a new built-in enrols by default and no test names a theme.

**Outcome**: `go test ./internal/theme` measures every embedded palette against every §13.5 rule and fails loudly if any leg misses, with `tokyo-night-day` and `nord` (tasks 2-5, 2-7) landing already floor-checked the moment their files appear.

**Do**:
- Create `internal/theme/contrast_test.go` in package `theme_test`. Port `relativeLuminance`, `contrastRatio` and `TestContrastMath` from `internal/tui/theme/contrast_test.go` **unchanged** — the WCAG math is the one member of that file §13.6 leaves genuinely untouched, and it is the anchor that stops every floor assertion passing vacuously. Keep `go-colorful`'s `LinearRgb()` as the linearisation (see Edge Cases: several shipped legs clear by less than 0.01).
- Add one enumeration helper — `embeddedThemes(t *testing.T) map[string]theme.Theme` — loading every `BuiltinSlugs()` entry through `LoadBuiltin` with a `log.Discard()`-backed event logger, failing the test on any rejection, and **failing if the set is empty** so the suite can never pass vacuously.
- Declare the three floors as constants: `floorNormal = 4.5`, `floorLargeUI = 3.0`, `floorFillPerceptible = 1.10`.
- Write the ten rule carriers, each `t.Run(slug + "/" + token)` inside the theme loop. Names map one-to-one onto §13.6's ten so Phase 3's deletion of the old file is a clean swap: `TestForegroundFloorAgainstOwnCanvas`; `TestTextSubtleBand` (was `TestTextDimHeldToThreeToOneFloor`); `TestTextFaintDecorativeBand`; `TestBgSelectionPairRule`; `TestBgAttentionPairRule` (was `TestBgWarningPairRule`); `TestInlineFlashAttentionPairClearsFloor`; `TestPreviewPeekChromeClearsFloorAgainstCanvas`; `TestBgSubtlePairRule` (was `TestBgTrackPairRule`); `TestForegroundOnTintPairings`; `TestStatePositiveClearsCanvasAndSelection`.
- Encode the rule set exactly: **≥ 4.50** for `text.primary`, `text.secondary`, `text.tertiary`, `text.muted`, `accent.key`, `accent.mode`, `accent.attention`, `state.positive`, `state.destructive`; **`text.subtle` in the 3.00–4.49 band** (≥ 3.00 and < 4.50); **`text.faint` in the > 1.00 and < 3.00 band**; **`accent.primary` ≥ 3.00**; `border` and `canvas` carry **no numeric floor**. Tints: `bg.selection` three legs (`text.on-selection` on tint ≥ 4.50 · `accent.primary` vs canvas ≥ 3.00 · fill vs canvas ≥ 1.10), `bg.attention` three legs (`text.on-attention` on tint ≥ 4.50 · `accent.attention` vs canvas ≥ 3.00 · fill vs canvas ≥ 1.10), `bg.subtle` fill only. Foreground-on-tint ≥ 4.50: `text.on-selection`, `text.secondary`, `text.tertiary`, `state.positive` on `bg.selection`, and `text.on-attention` on `bg.attention`. `state.positive` additionally clears **both** canvas and `bg.selection`.
- Do **not** add a `text.primary` on `bg.selection` leg — §7.4 walked it as free information and §13.5 deliberately omits it (the selected row's name renders in `text.on-selection`).
- Leave `internal/tui/theme/contrast_test.go` untouched — it still guards the shipping render layer until Phase 3.

**Acceptance Criteria**:
- [ ] Every embedded theme passes every rule; the failure message names the slug, the token, the measured ratio and the floor.
- [ ] No `canvasDark` / `canvasLight` constant exists in the new file — every reference background is `th.Canvas.Value`.
- [ ] No test names a theme; adding a `.theme` file to `builtins/` enrols it with no test edit (proven by the loop over `BuiltinSlugs()`).
- [ ] The enumeration helper fails when the embedded set is empty, so the suite cannot pass vacuously.
- [ ] `TestContrastMath` still anchors black-on-white at 21.00 and self-contrast at 1.00.
- [ ] Comparisons are plain `>=` / `<` against the exact floors — no epsilon, no rounding to two decimals before comparing.
- [ ] `internal/tui/theme`'s tests still compile and pass unchanged.
- [ ] The whole file runs in the unit lane with no tmux, no daemon and no built binary.

**Tests**:
- `"it anchors the WCAG math"` — `TestContrastMath`
- `"it holds every foreground to its floor against its own canvas"` — `TestForegroundFloorAgainstOwnCanvas`
- `"it holds text.subtle inside the 3.00–4.49 band"` — `TestTextSubtleBand`
- `"it holds text.faint inside the decorative band"` — `TestTextFaintDecorativeBand`
- `"it clears all three bg.selection legs"` — `TestBgSelectionPairRule`
- `"it clears all three bg.attention legs"` — `TestBgAttentionPairRule`
- `"it clears the inline flash's attention pairing"` — `TestInlineFlashAttentionPairClearsFloor`
- `"it clears the preview peek chrome against the canvas"` — `TestPreviewPeekChromeClearsFloorAgainstCanvas`
- `"it keeps bg.subtle's fill perceptible"` — `TestBgSubtlePairRule`
- `"it clears every foreground-on-tint pairing"` — `TestForegroundOnTintPairings`
- `"it clears state.positive on both canvas and selection"` — `TestStatePositiveClearsCanvasAndSelection`
- `"it enrols every embedded theme without naming one"` — `TestFloorsEnumerateTheEmbeddedSet` (asserts the enumerated slug set equals `BuiltinSlugs()` and is non-empty)

**Edge Cases**:
- Every ratio is measured against **the theme's own `canvas`**, never a `canvasDark`/`canvasLight` constant — that is what makes enrolment automatic and mode-free.
- `text.subtle`'s band has a **ceiling** as well as a floor: it must clear the UI floor *and* stay below normal text, or it is not de-emphasised. This generalises what ships today as a light-only ceiling.
- `text.faint` reaching the UI floor is a **failure**, not a pass — the band is `> 1.00` and `< 3.00`.
- `accent.primary` sits on the 3.00 large/UI floor while every other accent takes 4.50; `border` and `canvas` carry no numeric floor at all.
- `state.positive`'s dual clearance (canvas **and** `bg.selection`) is the leg that caught the Nord green and the one MV itself solved by darkening — it must be its own named test, not folded into the pairing loop.
- **Several shipped legs clear by hair's-breadth margins**, measured during planning with `go-colorful` `LinearRgb()`: `nord` `state.positive` on `bg.selection` = **4.500345**, `nord` `state.destructive` vs canvas = **4.502234**, `tokyo-night` `text.subtle` = **3.011278**, `tokyo-night-day` `bg.attention` fill = **1.110757**, `tokyo-night` `text.tertiary` on `bg.selection` = **4.512702**. A different luminance implementation, or comparing a value rounded to two decimals, can flip these from pass to fail — keep the existing math and compare the raw float.
- Enumeration over the embedded set means a new built-in enrols by default and no test names a theme.
- This lands in `internal/theme`; the MV-shaped tests under `internal/tui/theme` stay until Phase 3.

**Context**:
> §13.5 states the canonical rule set "because 'auto-enumerates' only means anything against a complete and theme-independent list. §7.4's table is the *Nord port's* verification record — a walk of these rules for one palette — not the rules themselves. Every ratio is measured against **the theme's own `canvas`**, never a constant. Three floors carry the whole set: **4.50** normal text, **3.00** large/UI, **1.10** fill-perceptible."
> §13.5: "**Floor-check enrolment is automatic.** The floor tests **auto-enumerate the embedded set**, so a new built-in is checked by default."
> §13.6: the ten floor tests "**do not compile after §3.2**, and together they are the single largest mechanical surface in the reshape… Two of them are the named carriers of rules §13.5 states canonically: `TestBgWarningPairRule` is the three-leg warning band, `TestStateGreenClearsCanvasAndSelection` is the dual clearance that caught the Nord green. (`TestContrastMath` is pure ratio math and is genuinely untouched — the one member of the file that is.)"
> §6.4: contrast floors apply to what Portal **ships**; syntactic validity applies to what users write. "Relaxing a floor for a named port was the one option ruled out, because it would break the guarantee that is the entire point of having tiers."
> §7.5 records the light-theme asymmetry this task's automation does *not* cover: a dark theme is near-free because these checks auto-enumerate, whereas a light theme additionally needs the eyeball pins of task 2-6.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §13.5, §13.6, §6.4, §7.4, §7.5

## theming-system-2-4

### Task 2.4: Re-point the contrast swatch surface at a loaded `Theme` via `--theme`

**Problem**: Two tasks in this phase carry **human visual gates** — the §7.7 re-derivation (2-5) and Nord's inventions and corrections (2-7) — and §13.1 is categorical that `capturetool` is the *only* viable route to seeing a colour change before release, because Portal cannot be run from a temporary build. The instrument for exactly this judgement already exists: the standalone labelled-tint swatch. But it is driven by `--appearance` and reads `theme.MV` directly, so it can only ever render the one compiled-in palette in one of two modes — it cannot show a *theme*. It is also the one capture surface that **deliberately does not route through `tui.Build`**, which is precisely why it can be re-pointed now, three phases before the render layer takes a `Theme`.

**Solution**: `internal/capture`'s swatch takes an `internal/theme.Theme` instead of a `prefs.Appearance`, rendering every band from that theme's own tokens against its own canvas; `capturetool` gains a `--theme <slug>` flag resolving against the embedded set, defaulting to `tokyo-night`, which drives the swatch branch only — `--appearance` keeps driving the `tui.Build` fixture path for this one phase.

**Outcome**: `go run ./cmd/capturetool --fixture contrast-validation --theme <slug>` renders any embedded palette's tints, pairings and border on that palette's own canvas, and an unknown or invalid `--theme` is a hard, non-zero-exit error rather than a silent fallback to the wrong colours at a gate.

**Do**:
- Change `capture.NewContrastValidationModel(appearance prefs.Appearance)` to `NewContrastValidationModel(th theme.Theme)` (the new `internal/theme`), and delete `modeFromAppearance`. `swatchModel` holds the `Theme` in place of `mode`; every `theme.MV.X.ColorFor(mode)` becomes `s.th.X.Color()`, and `tokenHex(tok, mode)` becomes `tok.Value`. The `prefs` import goes from `swatch.go`.
- Re-point the bands onto the renamed vocabulary: `bg.selection` (with `text.on-selection` · `text.secondary` · `state.positive` on the tint), `bg.attention` (with `text.on-attention`), `bg.subtle` (with the `accent.primary` bar over it), and a single `border` rule — the fourth band label collapses from `border.separator / border.footer` to `border`, since the tokens consolidated. Keep the title line's shape but source the canvas hex from `th.Canvas.Value`.
- Add `--theme` to `cmd/capturetool/main.go`, default `tokyo-night`, resolving through `theme.NewLoader(theme.NewEventLogger(log.Discard()))` + `LoadBuiltin`. `found == false` or a rejection is an error returned to `main` → exit 1, with the slug and (where present) the reason in the message. **No fallback ever.**
- Keep `--appearance` wired to `resolveModel` (the `tui.Build` fixture path) unchanged, and add a source comment stating the coexistence is deliberate and lasts exactly one phase — Phase 3 deletes `--appearance` and widens `--theme` to the slug-or-path form with its filename-reason stderr warnings.
- Update `internal/capture/swatch_test.go` and `cmd/capturetool/swatch_test.go` to the new signature; add a `cmd/capturetool` test asserting the swatch path emits **zero** log records, using `log.SetTestHandler` with a `logtest.Sink` around a resolve call.
- Verify the shipped guards still hold: `internal/capture` gains an `internal/theme` import (an embedded lookup, not config discovery) and must still not reach `internal/xdg` or `cmd`; the portal binary must still not import `internal/capture`.

**Acceptance Criteria**:
- [ ] `capturetool --fixture contrast-validation` with no `--theme` renders `tokyo-night` — the default is the shipped dark built-in and every capture taken without the flag depends on it.
- [ ] `--theme <slug>` renders that built-in's tints on **its own** canvas; a light theme is therefore judged against `#e1e2e7`, not a dark reference.
- [ ] `--theme not-a-theme` exits non-zero with a message naming the slug, and renders nothing.
- [ ] A `--theme` naming an embedded file that fails validation exits non-zero carrying the §6.2 reason (unreachable today by construction — assert through an injected loader or a synthetic built-in FS rather than by breaking a shipped file).
- [ ] The swatch's bands cover all four pinned tints (`bg.selection`, `bg.attention`, `bg.subtle`, `border`) and every foreground-on-tint pairing §13.5 lists, including `state.positive` on `bg.selection`.
- [ ] `swatch.go` imports neither `internal/prefs` nor `internal/tui/theme`.
- [ ] Resolving a theme in `capturetool` produces zero records through a `logtest.Sink` — `log.Discard` is what the loader is handed.
- [ ] `internal/capture`'s no-real-config invariant holds (no `internal/xdg`, no `cmd`, no prefs read) and `cmd/capturetool`'s import guard still passes.
- [ ] Every other fixture still renders exactly as before through `--appearance`.

**Tests**:
- `"it renders the swatch from an injected theme"` — `TestSwatch_RendersFromInjectedTheme`
- `"it paints every band on the theme's own canvas"` — `TestSwatch_UsesThemeOwnCanvas` (dark and light themes)
- `"it covers the four pinned tints"` — `TestSwatchBandsCoverEveryPinnedTint`
- `"it covers the foreground-on-tint pairings"` — `TestSwatchCoversForegroundOnTintPairings`
- `"it defaults to tokyo-night"` — `TestResolveTheme_DefaultsToTokyoNight`
- `"it hard-errors on an unknown theme"` — `TestResolveTheme_UnknownSlugIsAnError`
- `"it hard-errors on an invalid theme rather than falling back"` — `TestResolveTheme_InvalidThemeIsAnErrorNotAFallback`
- `"it emits no theme log records"` — `TestCaptureTool_ThemeResolutionIsSilent`
- `"it still resolves the tui.Build fixtures through --appearance"` — `TestResolveProgramSessionsFixture` (existing test, kept green)

**Edge Cases**:
- The swatch deliberately does not route through `tui.Build`, so it is drivable **before** the render layer takes a `Theme` in Phase 3 — that is the whole reason this task sits in this phase.
- `--appearance` still drives the `tui.Build` fixture path this phase; the two flags coexist for exactly one phase and the coexistence is documented in-source.
- An unknown or invalid theme is a hard error at non-zero exit, never a silent fallback — rendering the wrong theme at a visual gate is the failure the tool exists to prevent.
- Default `tokyo-night`, matching the shipped dark default.
- The loader is handed `log.Discard` so `capturetool` emits no `theme` events (§12.3 lists it as the fifth caller, neither using nor diagnosing a theme).
- Each tint is labelled against the theme's own canvas so a light theme is judged on its own background — the light-tint-on-light-canvas case is numeric-insufficient, which is exactly why the human looks at it.
- Slug-versus-path discrimination (`--theme ./mine.theme`) and the `bad name` / `reserved name` stderr warnings are **Phase 3**; this phase resolves slugs only.

**Context**:
> §13.3: "**The contrast-validation swatch fixture is re-pointed to `--theme` too.** `capturetool` carries a standalone labelled-tint swatch branch (the MV spec §16.5 lock-in/bail surface) which deliberately does not route through `tui.Build` and is driven by `--appearance` today. It is the surface that satisfies the human eyeball gate §7.5 and §13.5 require for a new light theme's pinned tints, so it must take a theme like everything else."
> §13.3 on the flag: "**Default: `tokyo-night`** when the flag is omitted, matching the shipped dark default. Every capture taken without the flag depends on it." And: "**Invalid input is a hard error** with the §6.2 reason and a non-zero exit, never a fallback: silently rendering the wrong theme at a visual gate is precisely the failure this tool exists to prevent."
> §13.1: "**Portal cannot be run from a temporary build to check a visual change.**… So `capturetool` is not a convenience; it is the **only viable route** to seeing a visual change before release." Two mechanisms, two audiences: a producible PNG per fixture for the agent, `capturetool --fixture` in a real terminal for the human.
> §13.3's VHS caveat applies to every capture this phase produces: the harness "is known to fail silently on write… **verify a fresh write before trusting or reviewing a capture** — confirm the file's hash changed — and retry on failure."
> Phase boundary: `tui.Build` still takes `prefs.Appearance` until Phase 3, which is why `--appearance` survives here. Task 2-1's guard already keeps `internal/theme` free of path resolution, so importing it from `internal/capture` cannot smuggle config discovery in.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §13.3, §13.1, §13.5, §12.3, §7.5

## theming-system-2-5

### Task 2.5: Ship `tokyo-night-day.theme` and run the §7.7 Oklab re-derivation check

**Problem**: MV's corrected light values are described in-source as *"darkened, hue-preserved"* — a claim about **chroma** that has never been measured. The Nord port showed exactly how that claim can be false: its first red correction (`#CF888F`) cleared the floor while shedding ~27% of the source's chroma and read washed-out and pink on sight. Seven MV light values carry the same risk in the opposite direction, and this is the last moment to check them: once the values are frozen into a `.theme` file they are shipped colours, and the inline erratum comments that identify *which* values are in scope are deleted in Phase 3. §7.6 also makes the built-in-set decision **conditional on this check**.

**Solution**: Re-derive each of the seven in Oklab, measure the shipped value's chroma loss against its **original**, compare the shipped value against the re-derivation with Oklab ΔE at a 0.05 threshold, gate anything that moved — then author `tokyo-night-day.theme` with §7.3's light table (as amended by the check) and every figure recorded as a `#` comment beside its value.

**Outcome**: `tokyo-night-day.theme` ships, auto-enrols in task 2-3's floors and passes, and each of the seven values carries a durable, exported-with-the-file record of what was measured and what was decided — including when nothing moved, because a passing check is a finding.

**Do**:
- **Recover the seven values from §7.7's table, not from source.** Six `§2.9 erratum` corrections, original → shipped: `text.muted` `#5A6296` → `#586093`; `text.subtle` `#7C84AA` → `#767DA2`; `accent.key` `#2E5FD0` → `#2D5CCA`; `accent.mode` `#0E7490` → `#0D6C87`; `state.positive` `#4C7A1F` → `#456E1C` → `#3B5E18` (darkened twice, so the original is `#4C7A1F`); `state.destructive` `#C32647` → `#BD2545`. Plus the seventh, `text.tertiary` `#515A80` → `#4C5478` — not an erratum but a darkening for the `bg.selection` pairing floor, carrying the same chroma risk. `accent.primary` `#8A3FD1` is **explicitly out of scope**: it cleared its floor unremedied and was never darkened.
- **Compute, with `go-colorful` v1.4.0** (already a dependency): chroma via `OkLch()`'s `c`; ΔE as Euclidean distance in `OkLab()` space. Pin the metric by reproducing §7.4's published Nord figures first — ΔE(`#A3BE8C`,`#A7C492`) ≈ 0.018 and `#DD8188` retaining ~94% of `#BF616A`'s chroma — which the planning pass verified this exact metric reproduces (0.0176 and 94.3%). Do the work in a throwaway program; the durable record is the file comments.
- For each of the seven: (1) **re-derive** the minimal-Oklab-distance colour that clears the *same* floor against `#e1e2e7`; (2) **measure chroma loss** of shipped versus original; (3) compute **ΔE(shipped, re-derivation)** and compare against **0.05**.
- **Gate anything at or over threshold**: replace the value with the re-derivation and take a fresh human visual gate through `go run ./cmd/capturetool --fixture contrast-validation --theme tokyo-night-day` (task 2-4). If the re-derived value is rejected at that gate, the shipped value stands and the comment records "measured, moved, judged worse".
- Author `internal/theme/builtins/tokyo-night-day.theme`: header comment (palette, upstream link, and one line stating the light half is independently tuned against the `#e1e2e7` canvas), then §7.3's light table. Beside each of the seven, a `#` comment carrying original → shipped, chroma retained (%), the re-derived hex, the ΔE, and the verdict. Beside `accent.primary`, a one-line comment recording it was out of scope and why.
- Move the four **eyeball-pin derivation notes** into this file as `#` comments beside `bg.selection`, `bg.attention`, `bg.subtle` and `border` — the dark-anchor derivation plus "eyeball-confirmed", carried across from `theme.go`'s existing `pinned — derivation …` comments. §7.1 makes the theme file their new home because they are the one judgement that is not numerically recoverable.
- If any value moved, add one line to the file's header noting that §7.3's table is superseded by this file for the moved values. Do **not** rewrite the specification.

**Acceptance Criteria**:
- [ ] `internal/theme/builtins/tokyo-night-day.theme` parses through `LoadBuiltin` with no rejection and 19 uppercase-canonical tokens.
- [ ] It auto-enrols in task 2-3's floor tests and passes every rule against `#e1e2e7` (planning-measured: `text.muted` 4.626, `text.subtle` 3.107, `text.tertiary` 5.703, `accent.key` 4.645, `accent.mode` 4.622, `accent.attention` 4.533, `state.positive` 5.797 canvas / 4.649 selection, `state.destructive` 4.619, `accent.primary` 4.373, fills 1.247 / 1.111 / 1.142).
- [ ] Each of the seven values carries a `#` comment with **all four** figures — original, chroma retained, re-derivation, ΔE — and its verdict, whether or not it moved.
- [ ] `accent.primary` carries an out-of-scope comment and its value is unchanged at `#8A3FD1`.
- [ ] The four pinned tints carry their derivation + `eyeball-confirmed` comments; no other value carries a derivation comment.
- [ ] The metric is stated once in the file header (Oklab Euclidean ΔE, OkLch chroma) so a future re-derivation is comparable.
- [ ] Any value at or over ΔE 0.05 was replaced and visually gated, and the gate's outcome is recorded in its comment; any value under threshold is unchanged and its figure is still recorded.
- [ ] `portal theme export tokyo-night-day` (task 2-9) reproduces the comments verbatim — they are in the file, not in Go.
- [ ] `internal/tui/theme/theme.go`'s inline erratum comments are **still present and unmodified** — their deletion is Phase 3, after this check has consumed them.

**Tests**:
- `"it ships a valid light built-in"` — `TestLoadBuiltin_TokyoNightDayIsValid`
- `"it clears every floor against its own canvas"` — covered by task 2-3's auto-enumeration; add `TestTokyoNightDay_IsEnrolledInFloorChecks` asserting the slug appears in the enumerated set
- `"it records a derivation figure for each of the seven checked values"` — `TestTokyoNightDayFile_SevenValuesCarryDerivationComments` (parses the file's comment lines and asserts one appears adjacent to each of the seven keys)
- `"it records the four eyeball pins' derivations"` — `TestTokyoNightDayFile_PinnedTintsCarryDerivationComments`
- `"it leaves accent.primary out of scope"` — `TestTokyoNightDayFile_AccentPrimaryUnchangedAndMarkedOutOfScope`
- `"it keeps comments through the loader"` — `TestLoadBuiltin_CommentsDoNotAffectParse` (the file with its comments parses to the same `Theme` as a comment-stripped copy)

**Edge Cases**:
- The seven values are recovered from **§7.7** before the inline erratum comments are deleted — that ordering trap is explicitly named in the spec (delete first and the check's input set is gone).
- `accent.primary` `#8A3FD1` is out of scope: its in-source note records it cleared its floor unremedied, so it was never darkened.
- `state.positive`'s original is `#4C7A1F` — it was darkened **twice**, so measuring against the intermediate `#456E1C` would understate the loss. (Planning measurement: shipped retains ~81% of the original's chroma, by far the largest loss of the seven — expect this one to be the finding.)
- Chroma loss is **shipped versus original** while ΔE is **shipped versus re-derivation** — they answer different questions and disagree in both directions: a shipped value can sit within threshold of the re-derivation while both have shed chroma against the original, and a value can exceed the threshold on lightness alone with chroma intact.
- Corrections are computed in Oklab, never by moving HSL lightness — raising lightness at fixed HSL saturation *drops* actual chroma, which is what produced the rejected Nord red.
- ΔE ≥ 0.05 replaces and takes a fresh visual gate; under threshold nothing moves but the figure is still recorded (a passing check is a finding, not a non-event).
- A re-derived value rejected at its gate leaves the shipped value standing, recorded as "measured, moved, judged worse" — the check surfaces a possible flaw, it does not mandate a change.
- Every figure lands as a `#` comment beside its value: the only durable home, since export is byte-faithful, the comment travels with the value, and it survives a re-derivation that supersedes §7.3's tables.
- A moved value that is one of the four pinned tints moves its **pin** too — hand the new value to task 2-6.
- If anything moves, §7.3's value table is superseded by the file rather than rewritten in the spec.
- Do not touch `bg.attention`'s light value while working nearby: its fill measures **1.110757** against `#e1e2e7`, 0.011 above the floor, and it is a pin rather than one of the seven.

**Context**:
> §7.7: "MV's corrected light values are described in-source as *'darkened, hue-preserved'*, which may carry the same chroma flaw as the rejected Nord red — in the opposite direction… **Owned by this feature's implementation, before MV's values are frozen into theme files.** Three steps, and the middle one is the point: re-derive each value in Oklab; measure chroma loss of the *shipped* value against its *original*; gate anything that moved materially."
> §7.7's threshold: "**Oklab ΔE ≥ 0.05 is 'moved materially'.** The Nord port anchors the scale at the other end (ΔE 0.018, cited as essentially imperceptible), and 0.05 is comfortably above that while still well below a difference anyone would describe as a colour change. Under it, nothing happens."
> §7.7's acceptance criteria "so the check has a determinate outcome either way", and its flagged consequence: "if the check finds anything, shipped colours change, `TestLightSurfaceTintsPinned`'s eyeball-established pins move, and 'Tokyo Night Dark/Light are just the existing values' stops holding exactly. **The built-in-set decision is conditional on this check.**"
> §7.7 on the home for the figures: "**Its home is a `#` comment beside the value in `tokyo-night-day.theme`** — the same home §7.1 gives the other judgement that is not numerically recoverable, the eyeball pins. It is the only durable option: the theme file is exported byte-faithfully to users (§12.1), it travels with the value it describes, and it survives a re-derivation that supersedes §7.3's tables."
> §7.3's light table, verbatim: `canvas #e1e2e7`, `text.primary #2E3C64`, `text.secondary #3F4760`, `text.tertiary #4C5478`, `text.muted #586093`, `text.subtle #767DA2`, `text.faint #AEB2C6`, `text.on-selection #1A1B2E`, `border #C9CDDB`, `text.on-attention #7A4B12`, `accent.primary #8A3FD1`, `accent.key #2D5CCA`, `accent.mode #0D6C87`, `accent.attention #9A5200`, `state.positive #3B5E18`, `state.destructive #BD2545`, `bg.selection #D0C6F0`, `bg.attention #E8D6A8`, `bg.subtle #D2D4DE`.
> **Ambiguity flagged**: §7.7 fixes the threshold and the two quantities but not the exact metric implementation. Use `go-colorful`'s `OkLab()` (Euclidean distance) and `OkLch()` (chroma), verified against §7.4's own published Nord figures, and state the choice in the file header so a later re-derivation is measured the same way.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §7.7, §7.3, §7.1, §13.5, §13.6

## theming-system-2-6

### Task 2.6: The light-theme enrolment table and the four eyeball pins

**Problem**: A light tint on a light canvas is **numerically insufficient** — the floor math cannot tell a tasteful `#D0C6F0` from a washed-out one, which is why four values were pinned by human eyeball at MV's validation gate and why an exact-value pin is the only guard they can have. Two things then have to be true at once: the pins must apply to *light* themes only (measuring a light tint against a dark reference is meaningless), and the product must keep **no variant concept at all** (§4.7). And the failure mode of getting the membership wrong is silent — a light theme with no entry either ships unchecked or is measured against the wrong reference.

**Solution**: A test-side light/dark enrolment table keyed by slug, carrying an assertion that **every** embedded theme appears in it, driving the two per-light-theme tests — `TestLightSurfaceTintsPinned` (four exact values) and `TestLightTintFillsArePerceptible` (the same four against the theme's own canvas).

**Outcome**: `tokyo-night-day`'s four pinned tints are guarded at their exact hexes, and any embedded theme that is added without being enrolled fails the suite rather than shipping unexamined — while `internal/theme`'s production surface still knows nothing about light or dark.

**Do**:
- Create `internal/theme/light_pins_test.go` (package `theme_test`) declaring the enrolment table as a `map[string]bool` slug → isLight, with a comment stating it is **test-side knowledge only** and why the runtime deliberately has none (§4.7: the slot classifies the theme; Portal never inspects a palette).
- Add `TestThemeAppearanceTableCoversEveryEmbeddedTheme`: every `BuiltinSlugs()` entry has an entry, **and** every table entry names a real embedded slug (so a deleted or renamed built-in leaves no stale row).
- Add `TestLightSurfaceTintsPinned`: for each **light** theme, the four pinned tokens equal their pinned hexes. The set is exactly four after the border consolidation — `bg.selection`, `bg.attention`, `bg.subtle`, `border` — and for `tokyo-night-day` those are `#D0C6F0`, `#E8D6A8`, `#D2D4DE`, `#C9CDDB`, uppercase-canonical, **unless task 2-5's re-derivation moved one**, in which case the pin takes the value from that gate. Structure the pins as a per-slug table so a future light theme adds a row.
- Add `TestLightTintFillsArePerceptible`: the same four tokens against **the theme's own canvas** (`th.Canvas.Value`, never a `canvasLight` constant), floor 1.10.
- Comment both tests with the scope rule: the light-only carve-out is **enrolment, not relaxation** — every §13.5 rule in task 2-3 still applies to a light theme, including the three ≥ 1.10 fill legs; these two tests are *additional*.
- Comment that pins for any future light theme are established by human eyeball at a `capturetool` visual gate (task 2-4's surface) and that there is no automatic path — this is the whole reason §7.5 defers a second light theme to separate work.

**Acceptance Criteria**:
- [ ] Adding a `.theme` file to `builtins/` without a table entry **fails** `TestThemeAppearanceTableCoversEveryEmbeddedTheme`; so does a table row naming a slug that no longer exists.
- [ ] The pinned set is exactly **four** tokens — not three, not the five rows the pre-consolidation test carried.
- [ ] `border` participates in the pins and in **no** numeric floor (task 2-3 gives it none).
- [ ] The fill test resolves its reference background from the theme, and no `canvasLight` constant appears in the file.
- [ ] Dark themes are not pin-checked and not fill-checked here; they are fully covered by task 2-3.
- [ ] The table and both tests live only in `_test.go` files; `internal/theme`'s production surface exposes no light/dark type, field or method (the Phase 1 vocabulary guard already asserts no `Mode`).
- [ ] With `tokyo-night-day` enrolled, both tests pass at the values task 2-5 shipped.
- [ ] Task 2-7 enrolling `nord` as dark is what keeps the coverage assertion green once it lands — the assertion is what forces it.

**Tests**:
- `"it requires every embedded theme to be enrolled"` — `TestThemeAppearanceTableCoversEveryEmbeddedTheme`
- `"it rejects a stale enrolment row"` — `TestThemeAppearanceTableHasNoStaleEntries`
- `"it pins the four light surface tints"` — `TestLightSurfaceTintsPinned`
- `"it keeps every light tint's fill perceptible against its own canvas"` — `TestLightTintFillsArePerceptible`
- `"it pins exactly four tokens"` — `TestLightPins_AreExactlyFourTokens`
- `"it does not pin a dark theme"` — `TestLightPins_SkipDarkThemes`

**Edge Cases**:
- The pinned set is four tokens after the border consolidation — `bg.selection`, `bg.attention`, `bg.subtle`, `border` — where the pre-consolidation test listed five entries (`border.separator` and `border.footer` shared a light hex).
- `border` participates in the pins and in no numeric floor.
- The table asserts every embedded theme appears in it, so an unenrolled theme fails the suite rather than shipping unchecked or being measured against the wrong reference.
- The table is **test-side knowledge only**; the product keeps no variant concept — a test is allowed to know things the runtime does not.
- The light-only scope is **enrolment, not relaxation**: every §13.5 rule still applies to a light theme, including the three ≥ 1.10 fill legs in task 2-3.
- The fill test resolves its reference background from the theme, not the hardcoded light canvas.
- A pin moved by task 2-5's re-derivation takes its value from that gate — the pin follows the shipped value, it does not contradict it.
- Pins for any future light theme are established by human eyeball with no automatic path.

**Context**:
> §13.5: "*Light themes only:* the four eyeball-pinned tints carry **additional** exact-value pins. **Nothing above is relaxed** — every rule in this section applies to every bundled theme regardless of light or dark, including the ≥ 1.10 fill legs on all three tints. The light/dark table's sole job is **enrolment**: it names which built-ins are light so `TestLightSurfaceTintsPinned` and `TestLightTintFillsArePerceptible` know which themes to run against. 'Carve-out' describes the *enrolment*, not a relaxation."
> §13.5: "**The eyeball-pinned set is four tokens, not three.** `TestLightSurfaceTintsPinned` today pins five entries — `bg.selection`, `bg.warning`, `bg.track`, `border.separator`, `border.footer` — which is **four distinct tokens after the §2.2 border consolidation**… The count is load-bearing: it determines which pin notes move into the theme files as `#` comments (§7.1), and how wide the light-only carve-out has to be. **All four.**"
> §13.5: "**It is the *test* that needs to know, not the product.**… **The table carries an assertion that every embedded theme appears in it.** A forgotten entry fails the suite rather than silently shipping a Portal-endorsed theme nobody checked — or measuring a light theme against a dark reference."
> §4.7: "**Portal has no notion of a theme being 'light' or 'dark'.** It is neither declared in the file nor derived from canvas luminance… **The one exception is test-side, not product-side.**"
> §13.6: both tests "**survive, and become per-light-theme**"; the fill test's "≥1.1 fill floor resolves its reference background from the theme rather than the hardcoded light canvas."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §13.5, §13.6, §4.7, §7.5, §2.2

## theming-system-2-7

### Task 2.7: Port Nord — the first genuinely external palette

**Problem**: The 19-token vocabulary has only ever been exercised by the palette it was designed for, so **porting one genuinely external palette is the first real test of whether the roles map cleanly — and that test must happen before the names become a public contract.** Nord makes the test unusually sharp: its canvas `#2E3440` is a mid-dark rather than a near-black, so contrast headroom is materially tighter than MV's, and its 16 slots are barrelled at the ends with nothing in the middle of the grey ramp. The port was twice found incomplete during specification — first covering 16 of 19 tokens, then roughly half the rule set — each time with a plausible-looking completeness claim, which is exactly why this task ships against an auto-enumerating floor test rather than a hand-walked table.

**Solution**: Author `nord.theme` from §7.4's table — 13 values taken directly, two corrected, three invented, one functional maximum — enrol it as dark in task 2-6's table, let task 2-3's auto-enumeration prove every §13.5 leg, and take a human visual gate on the corrections and inventions through the swatch.

**Outcome**: Portal ships a third built-in that clears every floor with no carve-out, whose corrections and inventions are recorded beside their values, and whose port has been checked by machine on the numbers and by eye on the judgements.

**Do**:
- Author `internal/theme/builtins/nord.theme` with §7.4's values: `canvas #2E3440` (nord0), `text.primary #ECEFF4` (nord6), `text.secondary #E5E9F0` (nord5), `text.tertiary #D8DEE9` (nord4), `text.muted #939EB2` (**invented**), `text.subtle #73819B` (**invented**), `text.faint #4C566A` (nord3), `text.on-selection #FFFFFF` (functional maximum), `accent.primary #B48EAD` (nord15), `accent.key #81A1C1` (nord9), `accent.mode #88C0D0` (nord8), `accent.attention #EBCB8B` (nord13), `state.positive #A7C492` (**corrected** from nord14 `#A3BE8C`), `state.destructive #DD8188` (**corrected** from nord11 `#BF616A`), `bg.selection #434C5E` (nord2), `bg.subtle #3B4252` (nord1), `bg.attention #3D4046` (**invented**), `border #4C566A` (nord3), `text.on-attention #ECEFF4` (nord6).
- Header comment: palette name, upstream link (`https://www.nordtheme.com/`), and the one-line statement §4.1's own example models — that `state.destructive` and `state.positive` are corrected for Portal's contrast floors.
- Per-value `#` comments for the five judgements only: each correction carries source hex → shipped hex, the chroma figure (`#DD8188` retains ~94% of `#BF616A`; `#A7C492` is ΔE 0.018 from `#A3BE8C` with chroma marginally *above* the original) and the floor it was corrected for; each invention carries how it was derived (`text.muted`/`text.subtle` interpolated on nord3's hue and saturation to fill the barrelled middle; `bg.attention` a ~8% nord13-into-canvas blend settled at a visual gate). Add the two port notes worth carrying: `text.on-attention` uses nord6 because Nord's Snow Storm is entirely cool and has no warm light (a deliberate divergence from MV's warmed treatment), and `nord3` legitimately serves both `border` and `text.faint`. No TODOs — the file is exported verbatim to users.
- Enrol `nord` as **dark** in task 2-6's table; the coverage assertion fails the suite until this is done.
- Run task 2-3's floors and confirm every leg. Planning-measured against `#2E3440`: `text.primary` 10.836, `text.secondary` 10.257, `text.tertiary` 9.245, `text.muted` 4.624, `text.subtle` 3.177, `text.faint` 1.693, `accent.primary` 4.409, `accent.key` 4.640, `accent.mode` 6.243, `accent.attention` 7.998, `state.positive` 6.515, `state.destructive` 4.502; fills `bg.selection` 1.448, `bg.attention` 1.202, `bg.subtle` 1.241; on-selection `text.on-selection` 8.628, `text.secondary` 7.085, `text.tertiary` 6.386, `state.positive` 4.500; on-attention `text.on-attention` 9.019.
- **Human visual gate** on the corrections and inventions: `go run ./cmd/capturetool --fixture contrast-validation --theme nord` (task 2-4's surface), judging the two corrected values and the three invented ones — particularly `bg.attention`, whose first arithmetic answer was rejected at a gate as far too heavy and warm for Nord's cool family. **Present §15.4's two Nord Paper frames alongside the swatch as the gate's reference** — `Kill Modal — Nord (state.destructive #DD8188)` for the corrected red and `Sessions — Nord inline flash (bg.attention #3D4046)` for the invented band. They are the only rendering of those two values on the surfaces they actually paint, and both judgements were originally settled against them (§7.4: the two reds "mocked side by side in a Nord kill modal"; the first `bg.attention` answer "rejected at a visual gate"). Read them **as reference, never truth** — §9.14 records that the mocks use per-frame literal hexes, so a frame's hex may differ from the shipped token's and the `.theme` file is the authority.
- If any leg misses, **re-derive rather than relax**: a correction has a published source whose chroma must be preserved (Oklab, never HSL lightness); an invention has no source, so a new value is settled at a fresh visual gate rather than by arithmetic.

**Acceptance Criteria**:
- [ ] `nord.theme` parses through `LoadBuiltin` with no rejection and 19 uppercase-canonical tokens.
- [ ] It is enrolled as dark in task 2-6's table; the enrolment-coverage assertion passes.
- [ ] It clears **every** §13.5 rule through task 2-3's auto-enumeration — no test names it, and no floor is relaxed for it.
- [ ] `state.positive` clears both canvas (6.515) and `bg.selection` (4.500345) — the dual-clearance leg that caught the uncorrected green.
- [ ] The two corrections and three inventions each carry a `#` comment recording their derivation; the two port notes (`text.on-attention` cool, `nord3` serving two roles) are recorded.
- [ ] `BuiltinSlugs()` now contains three slugs and `nord` reserves its slug automatically through task 2-2 (no Go edit needed).
- [ ] The visual gate was taken and its outcome recorded in the task's commit; anything the gate rejected was re-derived, not shipped.
- [ ] Nothing in `internal/theme`'s Go source mentions Nord — the palette is entirely file-resident.

**Tests**:
- `"it ships a valid third built-in"` — `TestLoadBuiltin_NordIsValid`
- `"it is enrolled as dark"` — `TestThemeAppearanceTableCoversEveryEmbeddedTheme` (task 2-6's assertion, now covering three themes)
- `"it clears every floor rule"` — task 2-3's auto-enumerated suite (add `TestNord_IsEnrolledInFloorChecks` asserting the slug is present in the enumerated set, so the coverage is not merely assumed)
- `"it clears state.positive on both canvas and selection"` — `TestStatePositiveClearsCanvasAndSelection` (task 2-3, now covering nord)
- `"it records its corrections and inventions"` — `TestNordFile_CorrectionsAndInventionsCarryComments`
- `"it reserves its own slug"` — `TestReservedSet_CoversEveryBuiltinSlug` (task 2-2, now covering nord)

**Edge Cases**:
- Canvas `#2E3440` is a mid-dark, so headroom is materially tighter than MV's — several legs clear by hundredths, and nothing has slack to spare.
- **Two legs sit within 0.003 of their floor**: `state.destructive` vs canvas = 4.502234 and `state.positive` on `bg.selection` = 4.500345. Verify these with the same WCAG implementation task 2-3 uses; if either lands microscopically under 4.50 in the shipped math, re-derive in Oklab preserving the source chroma — **do not** relax the floor and do not round before comparing.
- Two corrections (`state.destructive` `#DD8188` retaining ~94% chroma, `state.positive` `#A7C492` at ΔE 0.018), three inventions (`text.muted`, `text.subtle`, `bg.attention`) and one functional maximum (`text.on-selection` `#FFFFFF`) — 13 + 2 + 3 + 1 = 19.
- `state.positive` must clear **both** canvas and `bg.selection` — the single-token dual clearance is the leg that caught the green at 4.23.
- `nord3` serves both `border` and `text.faint`, which is legitimate (unlike two tokens that differ pointlessly, which the border consolidation removed); every port should expect to invent at the dark end, where Nord holds three values for Portal's five dark-end roles.
- `text.on-attention` is nord6, cooler than MV's warmed treatment — a deliberate port choice, not an oversight.
- **No floor carve-out for a named palette**: this being the first external theme, a carve-out granted here would set the precedent for every PR theme after it.
- A failure on a leg §7.4 did not walk forces a re-derivation, and if it lands on `text.muted`, `text.subtle` or `bg.attention` the new value is an **invention** and needs a fresh visual gate rather than arithmetic.
- `text.subtle` has **no locus on a flat Sessions frame** — it renders group `··· N` counts and pending loading steps — so its outstanding visual gate (§7.4) is deferred to **Phase 3**'s grouped Nord capture, which needs the render layer to take a `Theme` first. The swatch gate here covers the tints and inventions, not that one.
- Nord must be enrolled as dark in task 2-6's table or the suite fails — that is the assertion doing its job, not an obstacle.

**Context**:
> §7.2: "The deciding argument was **risk, not scope**: the 19-token vocabulary has only ever been exercised by the palette it was designed for, so porting one genuinely external palette is the first real test of whether the roles map cleanly — and that test must happen *before* the names become a public contract. Nord makes the test unusually sharp because its canvas is `#2E3440`, a mid-dark rather than a near-black, so its contrast headroom is materially tighter than MV's."
> §7.4's corrections: "**Correction 1 — the red.** `state.destructive` carries the 4.5 normal floor; Nord's published red `#BF616A` measures **3.05** against Nord's own canvas. Shipped corrected as `#DD8188` (4.50), **retaining ~94% of Nord's red chroma**… The floor holds with no carve-out — this being the *first* external palette, a carve-out granted here would set the precedent for every PR theme after it. **Correction 2 — the green.** The single `state.positive` token must clear **both** the canvas and the selection tint… Corrected to `#A7C492` (Oklab ΔE 0.018 — essentially imperceptible)."
> §7.4's inventions: "**Invention 1 & 2 — the ramp's middle.** Nord's greys are barrelled at the ends… both are interpolated on nord3's hue and saturation. **Invention 3 — the warning band.**… Settled at `#3D4046` (~8% nord13-into-canvas blend, fill 1.20), matching MV's own proportion… A first arithmetic answer (`#54524F`, a 20% blend at fill 1.60) was rejected at a visual gate as far too heavy and pushed into a warm grey outside Nord's cool family."
> §7.4's derivation method: "Contrast **corrections** must be computed in a **perceptual space (Oklab), never by moving HSL lightness**… A correction has a published source value whose chroma must be preserved. An **invented** value has no source to preserve; its constraints are landing in the right band and looking right, which is why `bg.attention` was settled at a visual gate rather than by arithmetic."
> §7.4: "**A failure on an unwalked leg can force re-deriving an *invented* value — which then needs a fresh visual gate.** The port was twice found incomplete… and each time the completeness claim was plausible enough to pass unexamined. The floor test auto-enumerating the embedded set means a missed leg surfaces at implementation rather than shipping."
> §7.4: "**Fidelity versus floors — resolved.** The floors win, and the corrected values ship under the palette's own name. No application maps a 16-slot ANSI palette 1:1 onto its own semantic roles; every Nord port in the wild adapts."
> §15.4 lists the forward-looking reference frames for surfaces that do not exist yet; three are Nord's — `Sessions — Nord (port)` (task 3-5's gate), `Kill Modal — Nord (state.destructive #DD8188)` and `Sessions — Nord inline flash (bg.attention #3D4046)` (this task's gate). "**And even those are reference, never truth:** the Paper mocks use per-frame literal hexes, so the same token can carry different values across frames. That is exactly the drift the token layer prevents in code."
> Phase boundary: Nord's outstanding `text.subtle` gate lands in **Phase 3** on a grouped capture; `docs/theming.md`'s attribution and correction record lands in **Phase 10**.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §7.4, §7.2, §13.5, §6.4, §12.4

## theming-system-2-8

### Task 2.8: The build-time guarantee — embedded-set validity and fallback-slug resolution

**Problem**: Making built-ins files moved parse failures from compile time to load time, and §7.6's answer is deliberately **not** a runtime crutch: there is no hardcoded last-resort palette beneath the fallback, so the situation is made impossible at build time instead. One half alone is not enough, and the reason is specific: validating the *files* proves the files are good, but the fallback is **hardcoded slug constants resolving into that set** — rename a built-in file in a later PR, or typo a constant, and every embedded theme still validates while **every fallback path becomes unresolvable**. Nothing else in the feature would catch that.

**Solution**: Declare the shipped-default/fallback slugs as production constants in `internal/theme`, and add one test file carrying both halves — every embedded theme parses and validates, **and** every fallback slug plus the shipped default pair resolves within that set.

**Outcome**: A binary cannot ship with a broken or unresolvable default; the failure surfaces in `go test ./internal/theme` at the moment it is introduced, not at a user's launch.

**Do**:
- Declare in `internal/theme`: `const DefaultDarkSlug = "tokyo-night"` and `const DefaultLightSlug = "tokyo-night-day"`, with a doc comment recording that they serve **both** §8.3's shipped adaptive pair and §8.5's per-slot fallback, and §8.5's warning verbatim in substance — "changing these values, or adopting a single fixed fallback, silently invalidates §8.3's degrades-to-a-constant-dark-default argument".
- Create `internal/theme/embedded_test.go` with half one: loop `BuiltinSlugs()`, load each through `LoadBuiltin`, assert no rejection, 19 populated tokens, every value uppercase-canonical `#RRGGBB`. Names no theme; enumerates.
- Half two: assert `LoadBuiltin(DefaultDarkSlug)` and `LoadBuiltin(DefaultLightSlug)` each return `found == true` with no rejection — the resolution assertion, distinct from the validity one.
- Assert the embedded set is non-empty, so neither half can pass vacuously against an empty `builtins/`.
- Add an in-package (`package theme`) test proving an embedded-shaped parse failure is an **ordinary error**: drive `parseThemeBytes` with each of a duplicate-keyed, a bad-hex and a short file and assert a `*Rejection` comes back with no panic. §7.6's fatal escalation belongs where a fallback is *needed* — Phase 5 — and must not be added here.
- Note in the file's header comment that validation is deliberately **not** startup-eager: nothing walks the embedded set at init because this test already proves it at build time, and re-proving it on every launch buys nothing on a cold path this feature otherwise adds no cost to. Task 2-1's no-`init()` guard is the structural half of that claim.

**Acceptance Criteria**:
- [ ] Every embedded theme parses and validates; the failure message names the slug and the §6.2 reason with its detail.
- [ ] `DefaultDarkSlug` and `DefaultLightSlug` both resolve within the embedded set; renaming a built-in file without updating a constant fails this test (verifiable by temporarily renaming one in a scratch checkout).
- [ ] The test enumerates and names no theme, so a built-in added by a later PR is enrolled automatically.
- [ ] The suite fails if `builtins/` is empty — no vacuous pass.
- [ ] An embedded-shaped parse failure returns an ordinary `*Rejection`; no panic, and no fatal-message path exists in `internal/theme`.
- [ ] Nothing walks the embedded set at package init (task 2-1's guard), and no exported eager-validation helper exists.
- [ ] The constants are production symbols in `internal/theme`, not test-file literals — Phase 5 consumes them for both the shipped pair and the fallback.

**Tests**:
- `"it validates every embedded theme"` — `TestEveryEmbeddedThemeIsValid`
- `"it resolves every fallback slug within the embedded set"` — `TestFallbackSlugsResolveWithinEmbeddedSet`
- `"it resolves the shipped default pair"` — `TestShippedDefaultPairResolves`
- `"it refuses to pass against an empty embedded set"` — `TestEmbeddedSetIsNonEmpty`
- `"it returns an ordinary rejection for a broken embedded parse"` — `TestEmbeddedParseFailureIsAnOrdinaryError`
- `"it enrols a new built-in without naming one"` — `TestEmbeddedValidity_EnumeratesRatherThanNames`

**Edge Cases**:
- Both halves are load-bearing — validating the files alone stays green while every fallback path is unresolvable after a renamed file or a typo'd constant.
- The fallback slugs and the shipped default pair are **constants resolving into the embedded set**, not free-floating strings; they are the same two values by §8.5's design, and that coincidence is what §8.3's argument rests on.
- An embedded parse failure is an ordinary error, never a panic — `main.go`'s panic-recovering exit stays the backstop for a genuine programming fault, not the designed route.
- Validation is **not startup-eager**, so nothing re-proves it on the cold path.
- The fatal one-line startup message (`built-in theme <slug> is missing or invalid — this binary is broken`) is raised where a fallback is *needed*, which is **Phase 5**, not here.
- A built-in added by a later PR is enrolled automatically because nothing names a theme.

**Context**:
> §7.6: "There is **no runtime fallback to hardcoded values** beneath the built-in fallback. Instead the situation is made impossible at build time. **A unit test must:** 1. **Parse and validate every embedded built-in** against the full validity rule (§6.1). 2. **Assert that every fallback slug and the shipped default pair resolve within that set.** Both halves are load-bearing… rename a built-in file in a later PR, or typo a constant, and every embedded theme still validates while **every fallback path becomes unresolvable.**"
> §7.6's mechanism: "the loader returns an ordinary error for an embedded parse failure — it does not panic. The escalation happens where the fallback is *needed*: a fallback that cannot resolve is a fatal error returned up the normal path, so the user sees a one-line message rather than a Go panic trace… **Validation is not startup-eager** — nothing walks the embedded set at init."
> §8.5: fallback is per-slot and mode-matched — `theme_dark` → `tokyo-night`, `theme_light` → `tokyo-night-day`, constant → `tokyo-night`. "**§8.3's second reason depends on that coincidence.**… So **changing these values, or adopting the single-fixed-fallback alternative rejected below, silently invalidates §8.3.**"
> §13.6 lists this as its own new test — "**Embedded-set validity + fallback-slug resolution**" — distinct from Phase 5's nomination-resolution + fallback test, which covers the resolution *path* rather than the build-time guarantee.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §7.6, §8.3, §8.5, §13.6, §14A

## theming-system-2-9

### Task 2.9: `portal theme export <slug>` — by-name resolution and byte-verbatim stdout

**Problem**: *"Copy a built-in and edit it"* carries **two** decisions — it is the pro that justified `go:embed` and the deciding factor that rejected merge-over-a-base — but built-ins live inside the binary, `portal theme list` and `--theme` are ruled out, and the themes directory is deliberately never created or seeded. Without an export verb the only route to a built-in's file is finding it on GitHub, which was never named as the workflow and is unavailable offline. Re-serialising the parsed `Theme` would not fix it either: that drops every `#` comment, and the comments are the attribution header and the derivation notes tasks 2-5 and 2-7 just spent their whole effort recording.

**Solution**: A bootstrap-exempt `portal theme export <slug>` that resolves the embedded set then the themes directory, parses and validates what it found, and writes **the source file's bytes** to stdout.

**Outcome**: `portal theme export nord > ~/.config/portal/themes/nord-lee.theme` produces a byte-faithful copy of the shipped file, comments included, having started no tmux server and read no prefs.

**Do**:
- Create `cmd/theme.go`: a `themeCmd` (`Use: "theme"`, `Short: "Manage themes"`) with one child, `themeExportCmd` (`Use: "export <slug>"`, `Args: cobra.ExactArgs(1)`), registered on `rootCmd`. Note in a comment that the group having exactly one member is deliberate.
- Add `"theme"` to `skipTmuxCheck` in `cmd/root.go`, with a comment in the existing style: printing a file must not start a tmux server, ensure the saver or run restore.
- Resolution, in `RunE`: validate the argument with `theme.ValidSlug` (Phase 1's rule, no extension involved); then try the **embedded set first** via `LoadBuiltin`; only on `found == false` compose `filepath.Join(themesDir, slug+".theme")` from `themesDirPath()` and call `LoadFile`. A built-in therefore never reads the themes directory — the same ordering Phase 5 reuses at construction.
- Construct the loader with `theme.NewEventLogger(log.Discard())` so export emits **no** `theme` events (§12.3: the component records where a theme is *used*, never where one is *diagnosed*).
- Write `result.Source` (task 2-1's field — the exact bytes parsed) to `cmd.OutOrStdout()` with no re-encoding, no added or stripped trailing newline, and no key reordering. Read once: the bytes printed are the bytes validated.
- Never touch `prefs.json` on any path — the argument is a slug, so the theme setting never enters and side-effect-freedom is by construction rather than by carve-out.
- Return failure paths as errors (`main.classify` prints them to stderr and exits 1); their exact frames and the control-stripping are **task 2-10**. Do not write anything to stdout on a failure path.

**Acceptance Criteria**:
- [ ] `portal theme export tokyo-night` writes bytes byte-identical to `BuiltinBytes("tokyo-night")` — header comment, derivation comments, blank lines and trailing newline included.
- [ ] `portal theme export nord-lee` with a valid `<themesDir>/nord-lee.theme` writes that file's bytes verbatim.
- [ ] The output is **not** a re-serialisation: a file whose keys are in a scrambled order with interleaved comments round-trips unchanged.
- [ ] A built-in slug succeeds even when `PORTAL_THEMES_DIR` points at a directory with mode `0000` — the embedded set resolves first and the directory is never read.
- [ ] `skipTmuxCheck["theme"]` is true, and executing the command with a recording `bootstrapDeps.Orchestrator` proves `Run` is never called.
- [ ] Zero and two arguments are rejected by `ExactArgs(1)`.
- [ ] No prefs read occurs (the `cmd` package's `TestMain` poison makes a stray read fail loudly), and no file is written anywhere.
- [ ] Exporting produces no `theme` log records through a `logtest.Sink`.
- [ ] Output goes through `cmd.OutOrStdout()` so a test can capture it.

**Tests**:
- `"it writes a built-in's bytes verbatim"` — `TestThemeExport_BuiltinBytesAreVerbatim`
- `"it writes a drop-in's bytes verbatim"` — `TestThemeExport_DropInBytesAreVerbatim`
- `"it preserves comments and key order"` — `TestThemeExport_IsNotAReserialisation`
- `"it resolves the embedded set before the themes directory"` — `TestThemeExport_BuiltinNeverReadsThemesDirectory` (unreadable directory, built-in slug, still succeeds)
- `"it is bootstrap-exempt"` — `TestThemeExport_IsBootstrapExempt`
- `"it takes exactly one slug"` — `TestThemeExport_ExactArgsOne`
- `"it never reads prefs"` — `TestThemeExport_ReadsNoPrefs`
- `"it emits no theme log events"` — `TestThemeExport_EmitsNoThemeEvents`

**Edge Cases**:
- Output is the file's bytes with comments included, never a re-serialisation of the parsed `Theme`, which would strip the attribution header and the derivation notes.
- The theme is still parsed and validated first even though the source file is what is written — that is what refuses an invalid drop-in and an unknown slug (task 2-10).
- Resolution is the embedded set then `<themes dir>/<slug>.theme`, so a built-in never reads the directory — the same ordering Phase 5 reuses at construction, and the same ordering that carries the no-shadowing guarantee.
- Bootstrap-exempt via `skipTmuxCheck` so printing a file starts no server, ensures no saver and runs no restore.
- `ExactArgs(1)`, so zero or several slugs is a Cobra usage error — it is not one of §14A's four refusal frames and inherits Portal's existing behaviour for arg-count errors.
- The `theme` verb group has exactly one member, deliberately.
- `prefs.json` is never read, making export side-effect-free by construction.
- The loader is handed `log.Discard` so export emits no `theme` events.
- Key ordering and trailing newline are non-questions because the shipped file already parses.

**Context**:
> §12.1: "Writes the named theme to **stdout** in canonical form, so the full drop-in workflow is `mkdir -p ~/.config/portal/themes` then `portal theme export nord > ~/.config/portal/themes/nord-lee.theme`. The `mkdir -p` is part of the published workflow, not an omission: Portal deliberately never creates or seeds the themes directory, and a shell redirect will not create it either."
> §12.1: "**Output is the file's bytes, comments included** — not a re-serialisation of the parsed `Theme`. The theme is still parsed and validated first… but what is written is the source file. Re-serialising would **drop every `#` comment**, and comments are not decoration here: they carry the attribution header the file format was chosen for and the eyeball-pin derivation notes that are the only surviving record of a non-numeric judgement."
> §12.1's command surface: bootstrap-exempt (added to `skipTmuxCheck`); slug domain is **built-ins *and* drop-ins**, which "makes export a diagnosis tool — 'show me what Portal parsed' — not just an on-ramp"; exactly one slug via `ExactArgs(1)`; the `theme` group has only `export`, "a one-member group, noted deliberately".
> §10.5: "**`portal theme export` does not read `prefs.json` at all.** Its argument is a slug, which resolves by name against the embedded set and then the themes directory (§8.4's ordering) — the theme setting never enters. That keeps it side-effect-free by construction rather than by carve-out."
> §12.3: the `theme` component "records where a theme is *used*, never where one is *diagnosed*" — doctor and export emit nothing, on three compounding grounds.
> Phase boundary: refusal copy, control-stripping and the `unreadable`-versus-`not found` discrimination are **task 2-10**; `docs/theming.md`'s two-line workflow is **Phase 10**.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.1, §10.5, §12.3, §8.4, §5.4

## theming-system-2-10

### Task 2.10: Export refusals — §14A's four stderr frames at exit 1

**Problem**: Export is also a diagnosis tool — "show me what Portal parsed" — so its failure messages are the whole answer a user gets, and each of the four failure classes sends them somewhere different. Getting the discrimination wrong is worse than terse: `portal theme export nord-lee` against a `themes/` directory the user cannot read must not print *"no theme named nord-lee"* about a file that plainly exists, and a charset failure must not be reported as a missing file, or the user goes looking in the wrong place. And because export is a pipe-into-a-file tool, a message on the wrong stream lands **inside the theme file** the user just created.

**Solution**: Map each failure class onto §14A's pinned frame verbatim, control-strip the argument where it is read, and return every failure as an error so `main.classify` prints it to stderr and exits 1 with stdout untouched.

**Outcome**: Each of unknown slug, invalid drop-in, charset-failing slug and unreadable produces its exact pinned sentence on stderr at exit 1, and a redirect never captures an error.

**Do**:
- Add `func StripControl(s string) string` to `internal/theme` — strips control characters and ANSI escapes from a value that will be echoed back — and apply it to the argument at the point `RunE` reads it. Document the reuse contract: §9.5 requires the same treatment for a slug read from `prefs.json` (Phase 5), and export needs its own site because it never reads prefs.
- Map the four classes to §14A's copy, verbatim:
  - **Unknown slug** (not a built-in; `<themesDir>/<slug>.theme` does not exist) → `no theme named <slug>`
  - **Invalid drop-in** (any content reason) → `theme <slug> is not valid: <reason>` where `<reason>` is §6.2's terse label verbatim (`bad syntax` / `bad colour` / `missing tokens`)
  - **Charset failure** → `theme <slug> is not valid: bad name`
  - **Unreadable** (the file or the directory could not be read) → `theme <slug> could not be read: <OS error>`
- Keep the `unreadable` frame **separate**, not folded into "is not valid": nothing was read, so "is not valid" would describe a judgement that was never made.
- Discriminate absent from unusable exactly as §5.5 does elsewhere: `os.IsNotExist` on the composed path is `no theme named`; every other read failure — permission denial, a dangling symlink, an unreadable *directory*, an I/O error — is `unreadable` carrying the OS error verbatim.
- Return each as a plain error (not a `*cmd.UsageError`, not a silent-exit sentinel) so `main.classify` prints it once to stderr and exits **1** for every class; the reason string is what discriminates, not the code.
- Guarantee stdout is untouched on every refusal path — nothing is written before validation succeeds.

**Acceptance Criteria**:
- [ ] Each of the four frames is produced **verbatim**, asserted against the literal strings from §14A rather than a paraphrase.
- [ ] `export nope` with an absent themes directory → `no theme named nope`; with an *unreadable* themes directory → `theme nope could not be read: <OS error>`.
- [ ] A `<themesDir>/mine.theme` with a duplicate key → `theme mine is not valid: bad syntax`; with a bad hex → `… bad colour`; missing a token → `… missing tokens`.
- [ ] `export ../evil`, `export -nord`, `export Nord` → `theme <slug> is not valid: bad name`, and no path is ever composed from the argument.
- [ ] An argument carrying a pasted newline, tab or ANSI escape is control-stripped in the echoed message, which stays a single line.
- [ ] Exit code is **1** for all four classes; no class returns a `*UsageError` (code 2) and none is a silent-exit sentinel (the message must actually print).
- [ ] stdout is empty on every refusal path — asserted, because a redirect would otherwise capture the error into the user's theme file.
- [ ] `reserved name` is unreachable (a built-in slug resolves to the built-in first) and a filename `bad name` is unreachable (the composed filename is always `<valid-slug>.theme`) — both asserted so the impossibility is pinned rather than assumed.
- [ ] No `theme` log events are emitted on any refusal path.

**Tests**:
- `"it refuses an unknown slug with the pinned frame"` — `TestThemeExport_UnknownSlugFrame`
- `"it refuses an invalid drop-in with its reason"` — `TestThemeExport_InvalidDropInFrame` (table over `bad syntax` / `bad colour` / `missing tokens`)
- `"it refuses a charset-failing slug as bad name"` — `TestThemeExport_BadNameFrame`
- `"it refuses an unreadable file or directory with the OS error"` — `TestThemeExport_UnreadableFrame`
- `"it distinguishes absent from unreadable"` — `TestThemeExport_AbsentIsNotUnreadable`
- `"it control-strips the echoed argument"` — `TestThemeExport_ArgumentIsControlStripped`
- `"it exits one for every failure class"` — `TestThemeExport_AllFailuresExitOne`
- `"it writes nothing to stdout on failure"` — `TestThemeExport_StdoutIsEmptyOnFailure`
- `"it can never report reserved name or a filename bad name"` — `TestThemeExport_ReservedAndFilenameReasonsAreUnreachable`

**Edge Cases**:
- Unknown slug, invalid drop-in, charset-failing slug and unreadable each take their pinned frame **verbatim**.
- `unreadable` is a separate frame because nothing was read, so "is not valid" would describe a judgement never made.
- An unreadable directory or file yields `unreadable` rather than "no theme named" — the misdirection §12.1 explicitly refuses.
- A charset failure is `bad name`, not `not found`: telling a user their file is missing when they typed an illegal name sends them looking in the wrong place.
- Exit 1 for every failure class since the reason string discriminates, not the code — export is a pipe-into-a-file tool, not a diagnostic like doctor.
- The argument is control-stripped where it is read, because §14A echoes it back and export never reads prefs (so it is not covered by the prefs-side rule).
- Every message goes to stderr so stdout stays clean and a redirect never captures an error.
- `not found` (§6.2's seventh reason) is the *concept* behind the unknown-slug case but its user-facing frame is `no theme named <slug>` — do not print the reason label here.
- An `ExactArgs(1)` violation is Cobra's usage error and is outside these four frames.

**Context**:
> §14A, `portal theme export`, stderr, exit 1: unknown slug → `no theme named <slug>`; invalid drop-in → `theme <slug> is not valid: <reason>`; slug fails the charset check → `theme <slug> is not valid: bad name`; unreadable → `theme <slug> could not be read: <OS error>` — "a separate frame, because the file is not *invalid*: nothing was read, so 'is not valid' would describe a judgement that was never made".
> §12.1: "**Unreachable because the directory or file could not be read** — Refused with reason **`unreadable`**, not `unknown slug`. Export is the fourth by-name resolver and inherits §5.5's discrimination like the other three: `not found` sends the user to check the filename, `unreadable` sends them to check permissions. Without it, `portal theme export nord-lee` against a `themes/` directory the user cannot read prints *'no theme named nord-lee'* about a file that plainly exists."
> §12.1: "**Failure exit code: 1 for every failure class.** Export is a pipe-into-a-file tool, not a diagnostic like doctor; the reason string on stderr is what discriminates, and distinguishing unknown-slug from invalid-file numerically buys nothing scriptable." And: "**A slug failing the charset check** — Refused with reason **`bad name`**, not `not found`."
> §9.5: "**A slug arriving as a CLI argument is control-stripped the same way**, at the point `portal theme export` reads its argument. Export never reads prefs, so it is not covered by the rule above — but §14A echoes the argument back on stderr, and an argument can carry a pasted escape exactly as a prefs value can."
> §12.1 also fixes the advisory boundary: "Doctor's advisory-vs-health distinction (§12.2) is doctor's own contract and does not extend here" — export refuses and exits non-zero, it does not warn and continue.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §14A, §12.1, §6.2, §9.5, §5.5
