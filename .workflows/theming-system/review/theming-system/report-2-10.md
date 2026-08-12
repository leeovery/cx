TASK: theming-system-2-10 — Export Refusals — The Four Stderr Frames At Exit 1

ACCEPTANCE CRITERIA:
- Each of the four frames produced verbatim, asserted against §14A's literal strings.
- `export nope` with an absent themes directory → `no theme named nope`; with an unreadable themes directory → `theme nope could not be read: <OS error>`.
- `<themesDir>/mine.theme` duplicate key → `bad syntax`; bad hex → `bad colour`; missing token → `missing tokens`.
- `export ../evil`, `export -nord`, `export Nord` → `theme <slug> is not valid: bad name`, and no path is ever composed from the argument.
- An argument carrying a pasted newline, tab or ANSI escape is control-stripped in the echoed message, which stays one line.
- Exit code 1 for all four classes; no class is a `*UsageError` (code 2) and none is a silent-exit sentinel.
- stdout empty on every refusal path.
- `reserved name` unreachable and a filename `bad name` unreachable — both asserted.
- No `theme` log events on any refusal path.

STATUS: complete

SPEC CONTEXT:
§14A pins `portal theme export` (§12.1) refusals to stderr at exit 1 in four frames: `no theme named <slug>`, `theme <slug> is not valid: <reason>`, `theme <slug> is not valid: bad name`, and `theme <slug> could not be read: <OS error>` — the last deliberately a separate frame "because the file is not *invalid*: nothing was read". §12.1 fixes exit 1 for every failure class (the reason string, not the code, discriminates) and requires `unreadable` rather than `unknown slug` when the directory or file cannot be read, inheriting §5.5's absent-vs-unusable discrimination. §9.5/§9.6 require the CLI argument to be control-stripped at the point it is read, because export never reads prefs and §14A echoes the argument back. §6.2 supplies the terse reason labels rendered verbatim.

IMPLEMENTATION:
- Status: Implemented (mechanism later re-based onto the shared by-name resolver by task 5-3 — intended, not drift)
- Location:
  - `/Users/leeovery/Code/portal/cmd/theme.go:22` — `theme.StripControl(args[0])` applied at the read, before anything judges or composes.
  - `/Users/leeovery/Code/portal/cmd/theme.go:38-47` — `exportRefusal` maps `ReasonNotFound` → `no theme named %s`, `ReasonUnreadable` → `theme %s could not be read: %s`, default → `theme %s is not valid: %s` (the `Reason` constants' string values are §6.2's labels verbatim, so `bad name` / `bad syntax` / `bad colour` / `missing tokens` all render correctly through one arm).
  - `/Users/leeovery/Code/portal/cmd/theme.go:53-63` — `resolveThemeSource` drives `theme.NewSilentLoader().ResolveByName`, so no `theme` log record can be emitted from any path here.
  - `/Users/leeovery/Code/portal/cmd/theme.go:69-75` — `unlocatableAsUnreadable` folds the "themesDirPath could not answer" state into `unreadable` rather than letting it masquerade as `not found`; the fold is exact (a `themesDirPath` error always yields `dir == ""`, which is the only route to `not found` in that state).
  - `/Users/leeovery/Code/portal/internal/theme/name.go:47-59` — `StripControl` = `ansi.Strip` then a control-rune drop; the doc comment states the read-not-draw rule and why neither pass subsumes the other. Reuse is real, not theoretical: `/Users/leeovery/Code/portal/internal/theme/setting.go:25-27` applies it to the three persisted prefs slugs (the §9.5 site).
  - Absent-vs-unreadable discrimination lives in the shared resolver: `/Users/leeovery/Code/portal/internal/theme/resolve.go:44-84` (`statThemeDir` gate → `notFound`; `narrowReadFailure` uses `Lstat` + `fs.ErrNotExist`/`fs.ErrPermission` to keep a dangling symlink `unreadable`), with the OS error carried verbatim in `Rejection.Detail` (`/Users/leeovery/Code/portal/internal/theme/load.go:122-126`).
- Notes:
  - Exit code: `RunE` returns a plain error; `cmd.Execute` (`cmd/root.go:193-203`) passes non-fatal errors through untouched and `main.classify` (`main.go:62-80`) prints once to stderr and returns 1. `rootCmd` sets `SilenceErrors`/`SilenceUsage` (`cmd/root.go:159-160`), so Cobra neither double-prints the frame nor dumps usage onto stdout — load-bearing for the "stdout is empty / a redirect never captures an error" guarantee.
  - stdout is written exactly once, after validation succeeds (`cmd/theme.go:29`); every refusal path returns before it.
  - `reserved name` has no arm by construction (the embedded set resolves first inside `ResolveByName`), and the omission is documented at `cmd/theme.go:34-37`.
  - Behavioural nuance worth recording (spec-conformant, not a defect): stripping at the read means the stripped value is also what gets resolved, so `export $'no\npe'` resolves the theme `nope` rather than refusing the raw argument. §9.5's "property of the value" rule mandates exactly this, and the in-code comment states it.

TESTS:
- Status: Adequate (thorough; a little duplication — see notes)
- Coverage (`/Users/leeovery/Code/portal/cmd/theme_test.go`):
  - `TestThemeExport_UnknownSlugFrame` (648) — empty and never-created themes directories both give `no theme named nope`.
  - `TestThemeExport_InvalidDropInFrame` (662) — table over duplicate key / bad hex / missing token asserting the three literal `is not valid: …` frames.
  - `TestThemeExport_BadNameFrame` (696) — `../evil`, `-nord`, `Nord` after `--`; plus a subtest that seeds a valid `evil.theme` in the themes directory's **parent** and asserts its bytes never reach stdout (the "no path is composed" criterion, tested by consequence rather than by inspection); plus a subtest pinning that the charset check precedes even *locating* the directory; plus the honest carve-out that a bare `-nord` is pflag's shorthand-cluster error.
  - `TestThemeExport_UnreadableFrame` (766) — unreadable file, unreadable directory, and unlocatable directory, each with the expected OS error derived by performing the same read rather than hard-coding platform wording.
  - `TestThemeExport_AbsentIsNotUnreadable` (799) — includes a dangling symlink whose read reports ENOENT yet must still be `could not be read`; the sharpest case in the file, and exactly §5.5's distinction.
  - `TestThemeExport_ArgumentIsControlStripped` (831) — newline/tab/ANSI/trailing-newline, each asserting the exact frame *and* scanning the rendered message for any surviving control rune.
  - `TestThemeExport_AllFailuresExitOne` (859) + `requireOrdinaryError` (595) — asserts the refusal is not a `*bootstrap.FatalError` (message suppressed), not a silent-exit sentinel, not a `*UsageError`. That is a structural proxy for exit 1 rather than a process-level assertion; it composes with `TestRun` (`/Users/leeovery/Code/portal/main_test.go:46-60`), which pins "ordinary error → code 1 + printed once to stderr". Adequate — an integration exec would add nothing this pair does not already fix.
  - `TestThemeExport_StdoutIsEmptyOnFailure` (869) — the four classes plus a vacuity guard proving the success path does write.
  - `TestThemeExport_ReservedAndFilenameReasonsAreUnreachable` (979) — a shadowing drop-in per built-in slug (with a guard that the shadow differs from the embedded bytes), the composed-filename round-trip over seven slug shapes including a 200-char one, and `nord-` proving a valid-but-absent slug is `no theme named`, never `bad name`.
  - `TestThemeExport_EmitsNoThemeEvents` (322) — six seeds including invalid, charset-failing and unreadable, with a harness-vacuity check that the sink *does* capture a deliberately emitted theme event; `TestThemeExport_UsesSharedByNameResolver/it still emits no theme records` (966) covers the remaining unreadable-directory path, which is the one that reaches `reportDirectoryUnusable`.
- Notes:
  - Vacuity discipline throughout is unusually good — nearly every fixture is checked for being the thing it claims to be before the assertion runs (`osReadError`, `requireCommentedSource`, the "this subtest would be vacuous" guards, `themetest.DenyRead` skipping when running as root).
  - Not over-mocked: no fakes, real temp dirs and real permission bits; fixtures derive from the shipped built-in so no hex is restated in Go.
  - Mild duplication: `TestThemeExport_UsesSharedByNameResolver` (897, added by task 5-3) restates all four frames already covered by the four dedicated frame tests, and `TestThemeExport_AbsentIsNotUnreadable/an absent file is no theme named` (800) is byte-identical to `TestThemeExport_UnknownSlugFrame/with an empty themes directory` (649). Coverage is unaffected; the cost is that a copy change touches four sites.
  - Not covered: an argument that is empty after stripping (see NON-BLOCKING NOTES).

CODE QUALITY:
- Project conventions: Followed. Bootstrap-exempt via `skipTmuxCheck["theme"]` and asserted; no `os.Exit` outside `main`; the `theme` log component is bound in `cmd` only and deliberately silenced here (`NewSilentLoader`), matching CLAUDE.md's "records where a theme is used, never where one is diagnosed"; `%s` at a user-facing boundary rather than `%w` is the documented boundary rule in `.claude/skills/golang-error-handling`.
- SOLID principles: Good. `cmd/theme.go` holds copy and classification only; charset check, embedded-set precedence, path composition and the absent/unreadable line all live in `internal/theme`, and an AST guard (`theme_test.go:940-964`) bans `ValidSlug`/`LoadBuiltin`/`LoadFile`/`Join`/`Lstat` from the file so the two by-name resolvers cannot drift.
- Complexity: Low. `exportRefusal` is a three-arm switch; `unlocatableAsUnreadable` is a single guarded fold.
- Modern idioms: Yes. `strings.Map` over `ansi.Strip`, `errors.Is` with `fs.ErrNotExist`/`fs.ErrPermission` in the resolver rather than string matching.
- Readability: Good. Every non-obvious choice carries a why-comment (why no `reserved name` arm, why a plain error rather than a `*UsageError`, why the fold is exact).
- Comment accuracy: Verified line by line against the code — the `reserved name`, plain-error, strip-at-the-read, silent-loader and `unlocatableAsUnreadable` comments are all true as written, and none cites a spec section, phase or task id.
- Security: The charset check runs before any path is composed, and the traversal case is proven by consequence (a real theme file planted in the parent directory is never emitted). No injection surface; the OS error is echoed verbatim by design (§14A) and the argument is control-stripped before it is echoed.
- Performance: N/A — one stat plus one read on the cold path; the resolver deliberately avoids a `ReadDir`.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] `/Users/leeovery/Code/portal/cmd/theme.go:45` — an argument that is empty, or that strips to empty (`portal theme export ""`, `portal theme export -- $'\n'`), renders `theme  is not valid: bad name` with a doubled space and no visible subject. §14A pins the frame but says nothing about an empty `<slug>`, so choosing a rendering (quote the value, or a distinct "no theme name given" line) is a copy decision, not a mechanical edit. Add the chosen case to `TestThemeExport_BadNameFrame` once decided.
- [quickfix] `/Users/leeovery/Code/portal/cmd/theme_test.go:615-646, 897-936` — add a `want string` field to `themeExportFailure` and drive `TestThemeExport_UsesSharedByNameResolver`'s four frame subtests from `themeExportFailures()` instead of restating them, so the pinned copy lives at one site rather than four.
- [quickfix] `/Users/leeovery/Code/portal/cmd/theme_test.go:800-804` — drop the `an absent file is no theme named` subtest; it is byte-identical to `TestThemeExport_UnknownSlugFrame/with an empty themes directory` (649-653), and the surrounding test earns its keep from the unreadable-directory and dangling-symlink cases.
