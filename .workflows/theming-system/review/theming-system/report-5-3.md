TASK: theming-system-5-3 — By-Name Theme Resolution — Charset Check, Embedded Set, Then The Themes Directory

ACCEPTANCE CRITERIA:
- `ResolveByName("tokyo-night", dir)` succeeds with `dir` at 0000, absent, and holding a broken `tokyo-night.theme` — always the embedded theme, never reading the directory.
- `("../evil")`, `("-nord")`, `("Nord")`, `("nord lee")`, `("")` each return `bad name`; no path composed, no file opened (proven by a readable file where a naive join lands).
- A valid drop-in at `<dir>/nord-lee.theme` resolves with its slug and theme.
- An absent `<dir>/nord-lee.theme` (directory present) returns `not found`, not `unreadable`, emitting nothing.
- An absent directory returns `not found` silently; an empty `themesDir` string does the same, composing no path.
- An unreadable directory and a regular file in the directory's place each return `unreadable` with the OS error plus exactly one `theme: directory unusable` record; five calls still emit one.
- Content reasons (`bad syntax` / `bad colour` / `missing tokens`) pass through from `LoadFile` unchanged.
- `reserved name` is never returned; a filename `bad name` is never returned.
- Exactly one file read per resolution; no `ReadDir` on any path.
- `portal theme export` routes through `ResolveByName`, its four §14A frames byte-unchanged, still emitting zero `theme` records.

STATUS: complete

SPEC CONTEXT:
§8.4 pins the by-name order (embedded set first, then the themes directory) as the mechanism that carries §5.4's no-shadowing guarantee on the non-enumerating construction path. §8.6 requires the persisted slug to be charset-validated before it becomes a path component. §5.5 requires `unreadable` (not `not found`) for a theme made unreachable by an unusable directory, with a `theme: directory unusable` WARN emitted from both the panel enumeration and the construction-time by-name read, deduplicated per process on path+reason (§12.3). §5.7 forbids a startup scan (one file read per nominated theme). §9.4 requires a charset-rejected slug to report `bad name`, not `not found`. §12.1/§14A pin export's four refusal frames and the same absent-versus-unreadable discrimination.

IMPLEMENTATION:
- Status: Implemented (mechanism partly superseded by later in-plan phases — see Notes; the outcome matches).
- Location:
  - `/Users/leeovery/Code/portal/internal/theme/resolve.go:16` `ResolveByName` → `:27` `resolveNamed` (charset → embedded → source) → `:44` `loadFromThemesDir` → `:71` `narrowReadFailure` → `:89` `reportDirectoryUnusable` → `:94` `notFound`.
  - `/Users/leeovery/Code/portal/internal/theme/enumerate.go:126` `statThemeDir` (shared absent/unusable directory verdict with `Enumerate`).
  - `/Users/leeovery/Code/portal/cmd/theme.go:53` `resolveThemeSource` calls `loader.ResolveByName(slug, dir)`; `:38` `exportRefusal` maps the reason to the four §14A frames; `:69` `unlocatableAsUnreadable` folds a themesDirPath resolution error into `unreadable`.
- Notes:
  - Ladder order is exactly as specified: `ValidSlug` → `badName(BadNameSlug)` before any join (resolve.go:28); `LoadBuiltin` returning `found` short-circuits (resolve.go:32), so a nominated built-in never touches the directory; only then the injected `themesDir`.
  - The plan's step 3 said "`os.Stat(themesDir)` → other stat error → `unreadable`". As implemented, a 0000 directory *stats fine*, so the denial surfaces one syscall later: `LoadFile` fails EACCES and `narrowReadFailure` (resolve.go:71) discriminates via `os.Lstat` on the composed path — `fs.ErrNotExist` → `not found`, `fs.ErrPermission` → directory-unusable + `unreadable`, anything else (dangling symlink, ENAMETOOLONG) → `unreadable` verbatim. This is stronger than the planned mapping, not drift: the planned stat alone would have missed the 0000 directory entirely. The comment at resolve.go:39-43 states this honestly.
  - `resolveNamed`/`slugLoader` (resolve.go:22-37) is a later-phase extraction so `ResolveByNameFrom` (resolution.go:119) reuses the identical charset + embedded-set ladder against a retained `Enumeration` — that is the Phase-8 amendment the shared-inputs note describes, and it strengthens the "cannot diverge" property rather than weakening it.
  - No `ReadDir` is reachable from `ResolveByName`; the only file read is `LoadFile`'s single `os.ReadFile`. The loader still resolves no paths — `themesDir` is injected, and `internal/theme/leaf_guard_test.go:23` pins that (no `internal/xdg` dep, no `PORTAL_THEMES_DIR` literal, no `os.UserHomeDir`).
  - Export's inline ordering is gone: `cmd/theme.go` contains no `ValidSlug`, `LoadBuiltin`, `LoadFile`, `filepath.Join` or `Lstat`, and `cmd/theme_test.go:940` enforces that by AST-banning those identifiers in the file. Export keeps `NewSilentLoader`, so the new `directory unusable` line is not emitted from the diagnose-shaped path.
  - `theme: directory unusable` carries `path`+`reason` and dedups on the injected `EventLogger` (events.go:115), shared with `Enumerate`'s sighting (enumerate.go:31) — the event is reported against the *directory*, never the composed file path, which is what makes the two collapse.

TESTS:
- Status: Adequate (one narrow over-test in `cmd`, noted below).
- Coverage: `/Users/leeovery/Code/portal/internal/theme/resolve_test.go` carries all eleven named tests from the task, one per acceptance criterion:
  - `TestResolveByName_CharsetCheckedBeforePathComposition:53` — table over `../evil`, `../../etc/passwd`, `-nord`, `Nord`, `nord lee`, `""`, run against `escapeTargetDir:37`, which stages a *readable, valid* `evil.theme` one level above the themes dir plus `Nord.theme` / `-nord.theme` / `nord lee.theme` / `.theme` inside it. Any naive join would resolve successfully, so `bad name` + zero records is a real proof that no path was composed, and `BadNameCause == BadNameSlug` is asserted (no extension involved).
  - `TestResolveByName_BuiltinNeverReadsDirectory:104` — all four hostile directory states (0000, absent, shadowing broken file under the built-in's own name, regular file in the directory's place) × every embedded slug, asserting the *embedded bytes* come back and nothing is logged.
  - `:157` drop-in resolves (slug, token set, verbatim source bytes); `:183` absent file → `not found`, with the neighbouring negative cases (unreadable file, dangling symlink, OS-refused name) pinning that only a genuine ENOENT narrows; `:250` absent dir and empty `themesDir` (the latter with `t.Chdir` into a directory holding a matching file, so a relative join would be caught); `:274` unusable directory table asserting the OS error verbatim and `Err` non-nil; `:345` dedup (5 calls → 1 record; enumeration + by-name → 1 record; two event loggers → 2 records); `:394` content reasons compared against `LoadFile`'s own rejection including `Line`; `:429` unreachable `reserved name` / filename `bad name`; `:476` a search-only (0111) directory still resolves, plus the AST reachability guard.
  - The one-read / no-ReadDir criterion is proven statically by `osCallsReachableFrom:536` (a package-wide call-graph walk asserting exactly one `os.ReadFile` and zero `os.ReadDir` reachable from `ResolveByName`). The `== 1` half is what keeps the `== 0` half non-vacuous, and `resolution_test.go:1012` re-asserts the same anchor from the other side.
  - Export parity: `cmd/theme_test.go:897` re-runs the four §14A frames end-to-end through the re-pointed path (`no theme named …`, `theme … is not valid: missing tokens`, `theme … is not valid: bad name`, `theme … could not be read: <OS error>`) plus verbatim stdout, the banned-identifier AST guard at `:940`, and `:966` asserting zero log records with the themes directory denied.
  - Fixture hygiene is good throughout: nearly every fixture is asserted to be in the state it claims before the assertion runs (`osReadError:615`, `requireUnnameableThemeRead:234`, the `os.ReadDir` vacuity checks at `:310`), and root-skip + chmod restore is handled by `themetest.DenyDir` and `searchOnlyDir:508`.
- Notes: Unit lane, no build tags, no `t.Parallel`, log capture through the sanctioned `log.SetTestHandler` seam, `execThemeExport` injects `bootstrapDeps` so no test body can reach real tmux — all conventions respected.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a path-free leaf with an injected `EventLogger`; the component binding stays in `cmd`; export uses `NewSilentLoader` per §12.3's "records where a theme is used, never where one is diagnosed".
- SOLID principles: Good. `slugLoader` is a one-function seam that lets the directory-backed and enumeration-backed resolvers share one ladder; `statThemeDir` is single-sourced with `Enumerate` so the two cannot disagree about a directory.
- Complexity: Low. Four small functions, each with one decision; the only branching of note is `narrowReadFailure`'s three-way errno discrimination, which is exactly the distinction §5.5/§12.1 require.
- Modern idioms: Yes — `errors.Is` against `fs.ErrNotExist`/`fs.ErrPermission` rather than string or errno matching; `os.Lstat` deliberately chosen so a dangling symlink is not mistaken for an absent file.
- Readability: Good. Comments explain the non-obvious choices (why no `ReadDir`, why the empty directory short-circuits before the join, why the event names the directory rather than the file) and each holds true against the code.
- Issues: None blocking. Security-relevant path handling is correct — the slug is charset-validated before `filepath.Join`, so no traversal component can survive, and `embed` cannot be escaped either.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [bug] internal/theme/resolve.go:71 — a *directory* named `<slug>.theme` inside the themes dir makes `ResolveByName` return `unreadable` (ReadFile gives EISDIR; Lstat succeeds so nothing narrows), while `Enumerate` skips directories (`enumerate.go:78 resolvesToDirectory`) and the enumeration-backed path answers `not found` (`resolution.go:138` → `union.go:209`). One on-disk state, two different reasons across construction and the panel/doctor. Narrow the same way the other cases are narrowed: in `narrowReadFailure`, when the composed path stats as a directory, return `notFound()` so by-name agrees with enumeration's skip; add the case to `TestResolveByName_AbsentFileIsNotFound`.
- [quickfix] cmd/theme_test.go:1014 — the `"a composed filename always clears the filename rules"` subtest is now a byte-for-byte duplicate of `internal/theme/resolve_test.go:448` (same slug list, same `SlugFromFilename` assertion) and asserts a property of `internal/theme` from a package that, by the guard at `cmd/theme_test.go:940`, composes no filename at all. Delete that subtest (and the `themeFileExtension` const it needs) from `TestThemeExport_ReservedAndFilenameReasonsAreUnreachable`, keeping the `"a colliding drop-in never reports reserved name"` half, which is a genuine end-to-end assertion.
- [quickfix] internal/theme/resolve_test.go:536 — `osCallsReachableFrom`, `themeCallGraph` and `importedPackageNames` are shared source-guard machinery consumed by `resolution_test.go:1007` as well, but live in a file named for one test subject. Move the three helpers into a dedicated `internal/theme/callgraph_test.go` so a reader of `resolution_test.go` finds them where they are declared.
