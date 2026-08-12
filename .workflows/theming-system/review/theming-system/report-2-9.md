TASK: theming-system-2-9 — `portal theme export <slug>` — By-Name Resolution And Byte-Verbatim Stdout

ACCEPTANCE CRITERIA:
1. `portal theme export tokyo-night` writes bytes byte-identical to `BuiltinBytes("tokyo-night")` — comments, blank lines, trailing newline included.
2. `portal theme export nord-lee` with a valid `<themesDir>/nord-lee.theme` writes that file's bytes verbatim.
3. The output is not a re-serialisation: scrambled key order with interleaved comments round-trips unchanged.
4. A built-in slug succeeds even when `PORTAL_THEMES_DIR` points at a mode-0000 directory.
5. `skipTmuxCheck["theme"]` is true, and a recording `bootstrapDeps.Orchestrator` proves `Run` is never called.
6. Zero and two arguments are rejected by `ExactArgs(1)`.
7. No prefs read occurs, and no file is written anywhere.
8. Exporting produces no `theme` log records through a `logtest.Sink`.
9. Output goes through `cmd.OutOrStdout()` so a test can capture it.

STATUS: complete

SPEC CONTEXT:
§12.1 pins the command surface: bootstrap-exempt, exactly one slug via `ExactArgs(1)`, slug domain is built-ins *and* drop-ins ("show me what Portal parsed"), the `theme` group has only `export` ("a one-member group, noted deliberately"), and — the load-bearing clause — "**Output is the file's bytes, comments included** … not a re-serialisation of the parsed `Theme`", because re-serialising drops the attribution header (§4.1) and the eyeball-pin derivation notes (§7.1). §10.5 requires `prefs.json` never be read: the argument is a slug, so side-effect-freedom is by construction. §12.3 requires silence — the `theme` component "records where a theme is *used*, never where one is *diagnosed*". §8.4/§5.4 fix the resolution ordering (charset check → embedded set → `<themes dir>/<slug>.theme`), which is what carries the no-shadowing guarantee on a path that never enumerates. §14A's refusal frames and control-stripping belong to task 2-10.

IMPLEMENTATION:
- Status: Implemented (mechanism intentionally superseded by a later task in the same plan — see Notes)
- Location:
  - `cmd/theme.go:10-13` — `themeCmd` (`Use: "theme"`, `Short: "Manage themes"`), one child registered at `cmd/theme.go:77-80`.
  - `cmd/theme.go:15-32` — `themeExportCmd` (`Use: "export <slug>"`, `Args: cobra.ExactArgs(1)`); `RunE` control-strips the argument, resolves, and writes `source` to `cmd.OutOrStdout()` with no re-encoding.
  - `cmd/theme.go:53-63` — `resolveThemeSource`: `theme.NewSilentLoader()` + `themesDirPath()` → `loader.ResolveByName(slug, dir)` → returns `result.Source` (the exact bytes parsed).
  - `cmd/theme.go:69-75` — `unlocatableAsUnreadable` folds a themes dir that cannot be *located* into `unreadable` rather than the resolver's `not found`.
  - `cmd/root.go:31` — `"theme": true` in `skipTmuxCheck`; the `PersistentPreRunE` parent-chain walk (`cmd/root.go:89-93`) exempts `theme export` via its parent.
  - `internal/theme/resolve.go:16-37` — `ResolveByName` owns the ordering: `ValidSlug` → `LoadBuiltin` → themes dir. `internal/theme/load.go:43-45,59-66` — `NewSilentLoader` (still reserves every built-in slug) and `Result.Source` = bytes handed in verbatim.
- Notes:
  - **Intentional supersession, not drift.** The task's "Do" list specified an in-`cmd` ordering (`ValidSlug`, then `LoadBuiltin`, then compose `filepath.Join(themesDir, slug+".theme")` and `LoadFile`). Task 5-3 (`By-Name Theme Resolution — Charset Check, Embedded Set, Then The Themes Directory`, commit b5021fdd) lifted that ordering into `theme.Loader.ResolveByName` so export and TUI construction share one implementation; `cmd/theme.go` now names none of those symbols, and `cmd/theme_test.go:940-964` is an AST guard that keeps it that way. The observable ordering, including the never-reads-the-directory property for a built-in, is unchanged and directly tested.
  - Likewise `theme.NewEventLogger(log.Discard())` became `theme.NewSilentLoader()` (task 11-10, single-sourcing the silent loader). Semantics identical — `internal/theme/load.go:43-45` is exactly that constructor.
  - The "the `theme` group has exactly one member, deliberately" comment the task asked for was written in 06b185c2 and later deleted by the repo-wide comment-standard passes (a4bc7bd5, 915e7fcb) under the "delete this, what concretely gets worse?" rule. That is a deliberate later decision; re-adding it would fight the standard, so it is recorded here rather than raised as a finding.
  - No prefs symbol, no `internal/prefs` import, no path composition and no filesystem write on any path — verified by reading `cmd/theme.go` and by the AST guard at `cmd/theme_test.go:434-459`.
  - Failure paths return plain errors (`cmd/theme.go:38-47`), which is what `main.classify` (`main.go:62-79`) prints and exits 1 on — matching the task's contract. See the non-blocking `[bug]` about what else lands on stderr alongside it.

TESTS:
- Status: Adequate (one contained over-test, noted below)
- Coverage (`cmd/theme_test.go`):
  - AC1 — `TestThemeExport_BuiltinBytesAreVerbatim` (:113) iterates **every** slug from `theme.BuiltinSlugs()` and compares against `theme.BuiltinBytes`; `requireCommentedSource` (:57) fails the fixture if it carries no `#` or no trailing newline, so the verbatim assertion cannot go vacuous.
  - AC2 — `TestThemeExport_DropInBytesAreVerbatim` (:139) in both the with- and without-trailing-newline shapes, the latter proving no newline is added.
  - AC3 — `TestThemeExport_IsNotAReserialisation` (:217) reverses the built-in's key lines and interleaves comments, and first asserts the fixture differs from the built-in so a re-serialiser could not accidentally pass.
  - AC4 — `TestThemeExport_BuiltinNeverReadsThemesDirectory` (:234): mode-0000 dir + built-in slug succeeds; a companion subtest proves the dir really is unreadable (kills the vacuous case); a third subtest strips `PORTAL_THEMES_DIR`/`XDG_CONFIG_HOME`/`HOME` so the directory cannot even be *located* — the sharp discriminator against any implementation that touches the directory first.
  - AC5 — `TestThemeExport_IsBootstrapExempt` (:98) checks the map entry *and* the recording orchestrator's call count; `execThemeExport` (:35) injects the recorder unconditionally, so no test in the file can reach real tmux.
  - AC6 — `TestThemeExport_ExactArgsOne` (:283) exercises the declared validator directly and the end-to-end arity failures, asserting empty stdout on both.
  - AC7 — `TestThemeExport_ReadsNoPrefs` (:406): unreadable `prefs.json` + byte-comparison that it is untouched, an AST scan banning the `internal/prefs` import and `prefsFilePath`/`loadPrefsStore`, and a `SnapshotStateDir`/`DiffFingerprints` sweep of the whole config root across three slugs (hit, drop-in, miss) proving nothing is written.
  - AC8 — `TestThemeExport_EmitsNoThemeEvents` (:322) across six cases (built-in, valid/invalid/unreadable drop-in, unknown slug, charset failure); `assertNoThemeRecords` (:384) re-arms a live sink and emits a probe event, failing if the harness records 0 — the anti-vacuity check that makes the silence assertion mean something.
  - AC9 — every case captures through `rootCmd.SetOut`, which only works because the command writes to `cmd.OutOrStdout()`.
  - Ordering/no-shadowing beyond the ACs: `TestThemeExport_ReservedAndFilenameReasonsAreUnreachable` (:979) seeds a shadowing file at every built-in slug and asserts the embedded bytes win.
- Notes:
  - Would fail if the feature broke: a re-serialising implementation fails :217 and :113; a directory-first implementation fails :262; a chatty loader fails :322; a stray prefs read fails :406.
  - The remaining tests in the file (`*Frame`, `AllFailuresExitOne`, `ArgumentIsControlStripped`) belong to task 2-10 and are outside this task's scope.
  - One over-test: `TestThemeExport_UsesSharedByNameResolver > "the four refusal frames are unchanged"` (:898-936) restates five assertions that already exist verbatim elsewhere in the same file. See NON-BLOCKING NOTES.
  - Coverage gap that is not a coverage gap for *this* task: nothing asserts what actually reaches **stderr** on a refusal (`execThemeExport` captures it at :45 but only quotes it inside failure messages). That surfaced the `[bug]` below.

CODE QUALITY:
- Project conventions: Followed. `cmd.OutOrStdout()` rather than `os.Stdout`; error returned rather than `os.Exit` in `RunE`; `init()`-registered subcommand; the log seam is the silent loader rather than a bare nil; `internal/theme` stays the single owner of resolution ordering. One deviation from `.claude/skills/golang-cli` — see the `[bug]`.
- SOLID principles: Good. `cmd/theme.go` is a thin adapter — argument hygiene, one resolver call, one write, one error renderer. Resolution, validation and the reason vocabulary all live in `internal/theme`, so there is exactly one by-name resolver in the binary.
- Complexity: Low. Three small functions, no nesting beyond one level, one switch with three arms.
- Modern idioms: Yes. Typed `*theme.Rejection` discrimination rather than string matching on errors; `strings.CutSuffix`-era helpers in the collaborators; no reflection or stringly-typed control flow.
- Readability: Good. Each comment states a decision the code cannot state itself (why `reserved name` has no arm, why the error is plain rather than a `*UsageError`, why the unlocatable fold is exact).
- Comment accuracy: All four surviving comment blocks in `cmd/theme.go` hold against the code. Spot-checked the strongest claim — "`not found` alongside a resolution error can only be this state, so the fold is exact" (:67-68): when `themesDirPath` errors, `dir` is `""`, and `ResolveByName` can then only return `bad name`, a built-in hit, or `not found`, so the fold cannot swallow any other state. No process-artifact references (task ids, phase numbers, spec sections) remain in this file.
- Security: Good, and this is the file where it matters. The argument is control-stripped at the read (`cmd/theme.go:22`) before it is judged or echoed, and `ValidSlug` runs inside `ResolveByName` *before* any path is composed, so `../evil` never becomes a path component — pinned by `TestThemeExport_BadNameFrame > "no path is composed from the argument"` (:720), which plants a real theme file in the themes directory's parent and asserts its bytes never reach stdout.
- Performance: Fine. One env/`$HOME` resolution, at most one `stat` + one `ReadFile`; a built-in reads no filesystem at all. No enumeration on this path, as §5.7 requires.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [bug] cmd/theme.go:15 — `themeExportCmd` sets neither `SilenceErrors` nor `SilenceUsage`, so a refusal reaches the user as cobra's `Error: no theme named nope`, then the **full usage + flags block**, then `main.classify` (main.go:71) printing the same frame a second time, bare. Both the spec's single-line refusal frame (§14A) and `.claude/skills/golang-cli`'s "Printing usage on every error" mistake row are missed, and `doctorCmd`, `uninstallCmd` and `stateCommitNowCmd` already carry the pair. Add `SilenceErrors: true, SilenceUsage: true` to `themeExportCmd`, and extend `requireExportRefusal` (cmd/theme_test.go:580) to assert `run.stderr` is empty (main, not cobra, owns the printing) so the shape is pinned. The frame *copy* is task 2-10's; this is the surrounding noise, so the fix belongs with whichever of the two is touched first.
- [quickfix] cmd/theme_test.go:898-936 — `TestThemeExport_UsesSharedByNameResolver > "the four refusal frames are unchanged"` duplicates, assertion for assertion, `TestThemeExport_UnknownSlugFrame` (:649), `TestThemeExport_InvalidDropInFrame`'s missing-token case (:678), `TestThemeExport_BadNameFrame`'s wrong-case case (:705), `TestThemeExport_UnreadableFrame`'s unreadable-file case (:767) and `TestThemeExport_DropInBytesAreVerbatim` (:139). It was a parity snapshot for the task-5-3 resolver move and has outlived that migration. Delete the five duplicated subtests and keep only the two resolver-specific pins in that test — "the export command re-implements no step of the ordering" (:940) and "it still emits no theme records" (:966).
