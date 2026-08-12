TASK: theming-system-5-7 — Construction Loads The Nominated Themes And Hands tui.Build The Nomination

ACCEPTANCE CRITERIA (from plan):
1. `"theme": "nord"` yields a constant nomination; gate resolved and unarmable at construction, `Init` issues no timeout tick, first frame paints Nord's canvas.
2. `"theme_dark": "nord"` yields a pair; dark answer → Nord, light answer → `tokyo-night-day`, timeout → Nord.
3. The gate resolves exactly once; a late `tea.BackgroundColorMsg` still populates `originalBg` but does not re-theme.
4. A file carrying both `theme` and both slots renders the constant (slots never read).
5. `"theme": "no-such-theme"` renders `tokyo-night`, emits `fallback applied` + `loaded tokyo-night`, leaves `prefs.json` byte-identical.
6. Directory-read budget: built-in constant 0 reads, drop-in constant 1, drop-in pair 2, no `ReadDir`.
7. `prefs.Appearance`, `parseAppearance`, `LoadAppearance`, `SaveAppearance` gone; raw `appearance` field still round-trips.
8. A `themesDirPath()` failure and a nil prefs store both still open the TUI on the shipped pair.
9. Under `NO_COLOR` both members loaded, gate skipped, dark selected, `theme: loaded` twice.
10. Retained startup canvas hex captured from the theme the gate selected.
11. `portal open <target>` performs no prefs read, no themes-dir access, zero `theme` records.
12. `capturetool` still passes the constant shape and reads no prefs; fixtures byte-identical.
13. A fallback that itself cannot resolve is fatal here: `openTUI` returns `BrokenBuiltinError` and constructs no TUI.

STATUS: complete

SPEC CONTEXT:
§8.4 — "At construction Portal loads every *nominated* theme — at most two"; the gate only *selects* between values already in hand. Cold-path cost is one file read for a constant, two for a pair, no read on the critical path, no flip. Resolution order on the by-name path is embedded set first, then the themes directory, so a built-in slug never touches the directory (this is what makes §5.4's no-shadowing implementable at construction). §8.4 also states the constructor takes the raw persisted keys alongside the nomination — the plan text's "do not thread rawKeys yet" deferral was owned by Phase 8, which has since landed it (spec-conformant, not drift). §8.8 — a constant user "needs no detection, so their first paint is immediate"; the gate "resolves exactly once", a late reply is still *consumed* for `restore.go` but never flips the active theme; the `prefs.Appearance` enum and its API die, the on-disk field does not. §9.10 — under `NO_COLOR` both nominated themes are still loaded, the gate is skipped, the canvas hex is captured as normal from the selected member. §10.5 — the migration runs only where a TUI is constructed, which is also the only place its result is used. §12.3 — "on the path Portal is most careful to keep free of cost, this feature adds nothing at all."

IMPLEMENTATION:
- Status: Implemented (with two Phase 6 / Phase 8 supersessions, both spec-conformant)
- Location:
  - `cmd/open.go:496-512` — `themeResolution(keys, loader)`: `theme.ResolveSetting(theme.NewRawKeys(...))` → `themesDirPath()` (error deliberately discarded to `""`) → `loader.ResolveNomination(setting, themesDir)`; the loader error is returned unwrapped as the fatal.
  - `cmd/open.go:483-494` — `newThemeLoader` builds `theme.NewLoader(theme.NewEventLogger(themeLogger))`; the single `log.For("theme")` binding is the package-level `themeLogger` (`cmd/open.go:27`). `buildThemeLoader` adds the test-only `OpenDeps.ThemeLoader` seam (production never sets it — verified by grep across non-test sources).
  - `cmd/open.go:601-614` — one `loadPrefsStore()` per launch; its keys feed `themeResolution`; a load failure degrades to a zero `prefsLoad` (nil store, zero keys → shipped pair) rather than blocking.
  - `cmd/open.go:456-457, 532-534, 638-639` — `tuiConfig.appearance` is gone, replaced by `theme`/`themeKeys`, mapped onto `tui.Deps.Theme`/`ThemeKeys` → `WithThemeNomination`/`WithThemeKeys` (`internal/tui/build.go:118,126`).
  - `internal/prefs/store.go` — `Appearance` enum, `parseAppearance`, `String()`, the three constants, `LoadAppearance`, `SaveAppearance` all deleted; the raw `Appearance string` field survives at line 73 with a comment explaining why it must stay declared.
  - `internal/prefs/appearance_api_guard_test.go` — tree-wide source guard banning the eight deleted identifiers, self-exempting, correctly *not* banning the on-disk field.
- Notes:
  - Two deliberate supersessions by later phases, both matching the spec rather than the plan text: (a) keys now arrive via Phase 6's `prefsLoad.Keys` (post-translation in-memory value) instead of a direct `prefsStore.LoadThemeKeys()` call — §10.5's requirement, and `cmd/open.go:496-499` documents it; (b) `rawKeys` *is* threaded to the model now (Phase 8), which §8.4 line 818 explicitly requires.
  - The two failure shapes are correctly separated at `cmd/open.go:503-510`: `themesDirPath()` error → empty directory (degrade; `theme.Loader.loadFromThemesDir` turns `""` into `not found` before any join, so built-ins still resolve), `ResolveNomination` error → returned fatal.
  - Ordering holds for criterion 13: `themeResolution` at line 611 returns before `buildSessionConnector` (618), the `CurrentSessionName()` read (666) and `buildTUIModel` (673).
  - Exec-path isolation is structurally guaranteed: `loadPrefsStore` has exactly one production caller (`openTUI`), and doctor uses the non-migrating `loadPrefsStoreNoMigrate`.

TESTS:
- Status: Adequate (unusually precise)
- Coverage: All 13 named tests exist and each maps to a criterion.
  - `cmd/open_theme_construction_test.go:60` constant-from-prefs paints Nord from frame one and a late light reply does not flip it (crit 1).
  - `:76` pair by gate — dark/light/timeout table, plus an explicit "no member active before resolution" assertion (`assertPaintedCanvas(..., nil)`) that pins never-paint-then-flip (crit 2).
  - `:118` late reply consumed for `OriginalBackground()` but not re-themed (crit 3).
  - `:136` constant wins over two *unusable* slot values, proven from the log: exactly one `loaded` line and no `fallback applied` is what "the slots were never read" looks like (crit 4) — a genuinely discriminating fixture rather than an equality check.
  - `:151` fallback events asserted exactly and ordered, plus a byte-compare of `prefs.json` (crit 5).
  - `:199` read budget: 50-file themes dir chmod'd 0111 (searchable, unlistable) with a self-vacuity tripwire that fails if the dir is still listable; the shadow `nord.theme` carries a different canvas so the *palette* identifies which file was parsed (crit 6). This is the strongest test in the set — it proves the embedded-first ordering behaviourally, not by assertion.
  - `:248` prefs-load failure and unresolvable themes dir both degrade to the shipped pair, with an `unresolvableThemesDir` vacuity tripwire (crit 8).
  - `:290` NO_COLOR both-loaded + two `loaded` lines + real content on the first frame (gate skipped, not pending) (crit 9).
  - `:317` startup canvas hex read through `restore.go`'s echo guard in both directions — each member's canvas replayed as the reply must suppress the set-back, a third colour must not (crit 10). Discriminating both ways rather than one.
  - `:348` fatal path: asserts the error, that a `recordingCommander` saw zero calls (the pre-construction tripwire), and that staged bootstrap warnings are still pending (the post-construction tripwire) — construction is bracketed on both sides (crit 13). This was the one gap the implementation's own fix-tracking recorded, and the fix is present including the explicit `t.Setenv("TMUX", ...)` so the tripwire does not depend on `TestMain`'s poison.
  - `cmd/open_theme_nomination_test.go:22` exec path: an AST guard restricting `theme.*` call sites to `openTUI`/`themeResolution`/`buildThemeLoader`/`newThemeLoader`, plus a real `portal open <session>` under a denied themes dir asserting zero `theme` records — loud-before-silent (it first proves records *would* be seen) so the zero assertion cannot be vacuous (crit 11).
  - `internal/prefs/appearance_api_guard_test.go:28` API gone (crit 7); raw-field round-trip still covered by 5-1's tests (`store_write_path_test.go:247-257`, `theme_savers_test.go:177-190`, `migration_marker_test.go`).
  - Two criteria are pinned one layer down and composed: "Init issues no timeout tick" by `internal/tui/nomination_test.go:68` (`assertNoTimeoutTick`, against `appearance_detection_test.go:154`) and "dark selected under NO_COLOR" by `internal/tui/nomination_test.go:161`. The cmd tests supply the other half (that these prefs values yield a constant / a pair). Sound composition, not a gap.
  - Crit 12 (capturetool) is verified structurally: `cmd/capturetool/main.go:87` builds `theme.NewSilentLoader()` and passes a pinned constant; nothing in the capture graph reads prefs.
  - The exec-path clause "performs no prefs read" — not directly assertable by the zero-`theme`-records test — is closed by Phase 6's call-site guard at `cmd/prefs_translation_test.go:292-303`, which pins `loadPrefsStore` to `open.go:openTUI` alone.
- Notes: mild, purposeful overlap only — `INFO loaded slug=nord` is asserted in three tests, but each uses it as the discriminator for a different property (slot-reading, read budget, event cadence). No redundant setup, no implementation-detail assertions (the one internal type touched, `appearanceTimeoutMsg`, is deliberately kept in the `tui` package and reached from `cmd` via `Init`-drain, with a comment saying so). No `t.Parallel()`; package-level seams restored via `t.Cleanup`; chmod cleanups registered after `t.TempDir` so LIFO restores the mode before removal.

CODE QUALITY:
- Project conventions: Followed. Single `log.For("theme")` binding in `cmd` (guarded by `TestThemeComponent_BoundOnceInCmd`); `OpenDeps.ThemeLoader`/`buildThemeLoader` mirror the established `*Deps` seam pattern; `prefs` stays a leaf (no `internal/log` import — the translation that logs lives in `cmd/config.go`); real logger at the use site, `NewSilentLoader` for doctor/export/capturetool exactly as CLAUDE.md's theme row prescribes.
- SOLID principles: Good. `themeResolution` takes both collaborators as parameters, so tests drive the production function without globals; only the `openDeps` lookup hides behind `buildThemeLoader`. Injecting the concrete `*theme.Loader` rather than inventing an interface is right — `Loader.BuiltinSource` is the seam `internal/theme` already declares for the broken-built-in state.
- Complexity: Low. `themeResolution` is three statements plus an error return; the fallback ladder lives in `internal/theme`.
- Modern idioms: Yes (`cmp.Or` for slot defaults, value-type `Loader` with a pointer `events` field documented as the shared dedup scope).
- Readability: Good. Every non-obvious choice carries a why-comment (per-call loader = one dedup scope; discarded `themesDirPath` error; typed-nil prefs store defeating the interface nil check). No process-artifact references (task ids, phases, spec section numbers) in either production or test comments — checked by grep.
- Issues: None material.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] `cmd/config.go:123-124` (with `cmd/open.go:608`) — `prefs.json` is opened three times on the TUI launch path (`LoadThemeKeys`, `LoadMigrationState`, then `Store.Load` for the grouping mode), so the mode, the theme keys and the migration marker can come from three different snapshots. Collapse to one read: add a single `Store` accessor returning the decoded record's mode + keys + migration state, and have `loadPrefsStore` carry the mode on `prefsLoad` so `openTUI` stops re-reading. The multi-read shape arrived with Phase 6's `loadPrefsStore`, not with this task's diff.
- [quickfix] `cmd/open_theme_construction_test.go:164` — the byte-compare re-derives the file path from `os.Getenv("PORTAL_PREFS_FILE")` although `setPrefsFile` returns it; change line 153 to `path := setPrefsFile(t, content)` and read `path`, so the assertion is anchored to the file the fixture wrote rather than to the env var it happens to set.
