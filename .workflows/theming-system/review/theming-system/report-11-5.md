TASK: theming-system-11-5 — Split theme.Loader's Panel-Assembly Responsibility And Single-Source The Enumerator Adapter

ACCEPTANCE CRITERIA:
1. `Loader` no longer exposes `Open` / `Reassemble`; the panel row-model assembly lives on its own type constructed from a `Loader`.
2. Exactly one type in the repo implements the four-method `tui.ThemeEnumerator` delegation over a loader, and it is exported from `internal/theme`.
3. `cmd/theme_enumerator.go`'s hand-written adapter and both test re-implementations are deleted.
4. No consumer holds methods it does not call (doctor, export and capturetool each take the narrower type).
5. `LoadPath` has no unused receiver.

STATUS: complete

SPEC CONTEXT:
Spec §13.3 (line 1621-1625) requires the panel's theme enumeration to sit behind an injectable seam matching the `TmuxEnumerator` / `ScrollbackReader` idiom, with the seam returning the **finished §9.4 union** (files ∪ built-ins ∪ unresolved persisted slugs, deduped one-slug-one-row) rather than a directory listing, and with **`internal/theme` owning that assembly** — which is what keeps `theme: enumerated`'s `count`/`rejected` computable where they are emitted (§12.3) and keeps `internal/tui` from becoming a fourth package emitting the `theme` component (§8.9 closes the set). Spec §5.5/§3.2 (line 196) additionally pins that the loader takes the themes directory as an injected value and never resolves it — path resolution is `cmd/config.go`'s (`themesDirPath`). This task is a structural refactor that preserves all of those constraints while splitting the accreted `Loader` surface.

Note on later supersession: phases 13/15/17 renamed the seam (`tui.ThemeEnumerator` → `tui.ThemeSource`, 13-10), made it uniform in what it consumes (15-9: `Resolve(e, keys)` instead of `Resolve(e, Setting)`), and narrowed the slot method (17-4: `ResolveSlot` → `LoadSlot`), and the adapter file was renamed `internal/theme/enumerator.go` → `internal/theme/dir_theme_source.go` with `DirEnumerator` → `DirThemeSource`. Two comment-audit chores (25626754, 915e7fcb) stripped the commit's original doc blocks. The criteria are judged against that amended intent.

IMPLEMENTATION:
- Status: Implemented (verified against the current tree, not just commit f5fe41cb)
- Location:
  - `internal/theme/union.go:106-155` — `Assembler{Loader}` now owns `Open` / `Reassemble` and the `builtinRows` assembly; `Loader` has neither method.
  - `internal/theme/dir_theme_source.go:1-43` — the exported four-method adapter `DirThemeSource{Loader, Dir}` (`Open`, `Reassemble`, `Resolve`, `LoadSlot`), composing the loader and a per-call `Assembler`.
  - `internal/theme/load.go:94` — `LoadPath` is now a package function; the old "it stays a method so a caller reaches every entry point through the one Loader" comment is gone.
  - `cmd/theme_source.go:9-12` (was `cmd/theme_enumerator.go`) — `newThemeSource` returns `theme.DirThemeSource`; the hand-written `themeEnumerator` struct and its five methods are deleted.
  - `internal/tui/theme_seams.go:11-16` — the seam interface, one-to-one with `DirThemeSource`'s exported methods.
  - Consumers: `cmd/doctor_theme.go:51-56,91,136` holds a `theme.Loader` and calls only `OpenEnumeration` / `ResolveByNameFrom`; `cmd/theme.go:53-63` (export) constructs a silent loader and calls only `ResolveByName`; `cmd/capturetool/main.go:101-106,142` narrowed its local `themeLoader` interface to the single `LoadBuiltin` method and calls the free function `theme.LoadPath` for the path branch.
- Criterion-by-criterion:
  1. MET — `grep 'func (l Loader)'` across `internal/theme` returns no `Open`/`Reassemble`; both live on `Assembler`, constructed from a `Loader`, inside `internal/theme` (package boundary kept).
  2. MET — `theme.DirThemeSource` is the only loader-backed implementation. The two remaining fakes (`internal/capture/theme_fake.go:10`, `internal/tui/theme_source_fake_test.go:10`, `internal/tui/theme_seams_test.go:14`) hold no loader and answer from declared values — a different thing, and required by capture's no-real-config import guard. The two tests that *do* need production behaviour **embed** the adapter rather than restating it (`internal/tui/theme_panel_open_test.go:31-43` `countingThemeSource`, `internal/tui/theme_panel_behaviour_test.go:18-32` `behaviourEnumerator`), each with a comment stating why.
  3. MET — `realThemeEnumerator`, `loaderThemeEnumerator` and the `cmd` struct return zero grep hits repo-wide; `cmd/theme_enumerator.go` no longer exists.
  4. MET — no consumer can reach the assembly entry points any more (they left `Loader`). capturetool holds a 1-method interface; doctor and export hold `Loader` and use only parse/enumerate/resolve.
  5. MET — `LoadPath` is receiverless, and `internal/theme/theme_test.go:175` pins it as a bare function in the exported-surface list.
- Notes: no drift from the plan. Scope is exactly the split plus the call-site updates; nothing extra was added.

TESTS:
- Status: Adequate
- Coverage:
  - **Structural pin (the strongest guarantee here)**: `internal/theme/theme_test.go:136-243` (`wantExports` + `TestVocabulary_HasNoModeSurface`) enumerates the package's entire exported surface by AST walk and compares it to a literal list. It now carries `Assembler`, `Assembler.Open`, `Assembler.Reassemble`, `DirThemeSource` + its four methods, and `LoadPath` as a function, while `Loader.Open`, `Loader.Reassemble` and `Loader.LoadPath` are absent. Re-adding any of them, or re-homing the assembly, fails this test — criteria 1, 2 and 5 are enforced, not merely done.
  - **Compile-time seam assertion** (the plan's explicit third test bullet): `internal/tui/theme_seams_test.go:36-41` asserts both `tui.ThemeSource = fixtureThemeSource{}` and `tui.ThemeSource = theme.DirThemeSource{}`, so a signature drift stops compiling in the seam's own package rather than surfacing at the wiring site.
  - **Behaviour through the exported adapter**: `internal/tui/theme_seams_test.go:45-145` drives `DirThemeSource` for the finished-union shape (persisted-slug row, `not found` reason, counts), `Resolve` (constant wins over the pair), and `LoadSlot` (unset slot resolves to the shipped default). `cmd/theme_source_test.go:35-117` covers the read-only-on-open cadence, `Reassemble` doing no I/O, and the shared dedup scope (with an explicit control sub-test proving the duplicate a second loader would produce).
  - **The named regression test survives**: `TestThemePanelOpen_WiredThroughBuildTUIModel` (`cmd/theme_source_test.go:170`) still runs, now against `newThemeSource(loader)` → the exported adapter, asserting the `●` reaches the frame through the real wiring.
  - **Existing suites migrated, not weakened**: `internal/theme/union_test.go`, `union_order_test.go`, `badge_test.go`, `resolution_test.go` and `enumerate_test.go` now construct `theme.Assembler{Loader: …}`; `internal/theme/load_test.go:352-460,645` calls `theme.LoadPath` directly and its table comment states the reserved set is no longer even reachable from it.
- Notes: not over-tested — no redundant assertions were added for the refactor itself; the split rides the export pin plus one compile-time assertion, which is the minimum that actually holds it. `TestThemeSourceIsSatisfiedByAFixtureFakeAndByTheExportedAdapter` has an empty runtime body by design (its value is the two `var _` declarations); acceptable and clearly commented.

CODE QUALITY:
- Project conventions: Followed. Small seam interface declared in the consumer package (`internal/tui`), production adapter exported from the owning leaf, path resolution left in `cmd` (`themesDirPath`), `theme` component still emitted only from `internal/theme`'s loader/events and `cmd` — no new emitter package. `internal/theme` stays free of `internal/tui` (the seam assertion lives in the `tui_test` external package, so no import cycle).
- SOLID principles: Good — this is a textbook SRP split. `Loader` = parse/enumerate/resolve, `Assembler` = row model (loads nothing itself), `DirThemeSource` = directory-bound seam adapter that adds no policy. Interface segregation improved in both directions: the panel seam is exactly four methods, and capturetool's consumer interface shrank to one.
- DRY: Good — the four-method delegation went from three hand-written copies to one exported type; the two tests needing production behaviour embed it, so a drift is a compile error rather than a silently-diverging copy.
- Complexity: Low. `DirThemeSource`'s methods are one-liners; `Assembler.Open`/`Reassemble` carry the same step order as before with no added branching.
- Modern idioms: Yes — value receivers on immutable composites, a receiverless function where nothing injected bears on the call, `Assembler` built per call as a cheap value.
- Readability: Good. Comments explain *why* each piece exists (dedup scope on the single `Loader`, "must not consult Dir" on `Resolve`/`LoadSlot`, empty `Dir` == absent directory) and hold true against the code as it now stands — I checked the post-rename comments for stale `DirEnumerator`/`ThemeEnumerator` references and found none in `.go` or `.md` outside `.workflows` planning artifacts.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/union.go:106 — `Assembler` is the type this task exists to create, but after the comment-audit chores it carries only a field comment; nothing states its responsibility or why it is separate from `Loader`. Add a doc line above the type: `// Assembler builds the panel's row model — which rows exist, their order and the counts over them — from what a Loader parses. It loads nothing itself, so a consumer that merely reads a theme never holds the row model's entry points.`
- [quickfix] internal/tui/builtin_themes.go:13 — `loadBuiltinTheme(loader theme.Loader, slug string)` has exactly one caller (`defaultDarkTheme`, line 8) which always passes `theme.NewSilentLoader()`, so the loader parameter is an unexercised seam: nothing injects a different loader and no test drives it. Inline the silent loader into `defaultDarkTheme` and drop the parameter (keeping the "no fallback on a miss" comment on the surviving function). This is the one consumer named in the task's Do list that the commit did not touch.
