TASK: theming-system-1-5 — The Reason Ladder: One File Yields Exactly One Reason (`internal/theme.Loader.LoadFile`)

ACCEPTANCE CRITERIA:
- A file that is duplicate-keyed **and** missing tokens reports `bad syntax` only, with no missing-token content in its detail.
- A file that has a bad colour **and** is missing tokens reports `bad colour` only.
- A `bad name` file that does not exist on disk reports `bad name`, not `unreadable`.
- A file whose slug collides with an injected reserved slug reports `reserved name` even when its contents are valid, and even when the file is unreadable or absent.
- With the **empty** reserved set this phase wires, no input can produce `reserved name`.
- An unreadable file (mode 0000, and separately a dangling symlink) reports `unreadable` with the OS error preserved verbatim and retrievable from the rejection.
- A valid file returns its slug and a `Theme` whose 19 values are uppercase-canonical.
- `LoadFile` never returns both a populated `Result` and a rejection; `Rejection.Detail` is always scoped to its single reason.
- `not found` is never returned by `LoadFile` for any input.

STATUS: complete

SPEC CONTEXT:
§6.2 fixes seven reject classes and pins their evaluation order — `bad name` (filename judged before the file is opened) → `reserved name` (slug alone, before any read) → `unreadable` → `bad syntax` (lexical failure aborts the parse) → `bad colour` (known keys only) → `missing tokens` (last) — with the first failure short-circuiting so "a file always has exactly one reason and the panel's single-reason row is never a choice". `not found` is explicitly outside the ladder (a persisted slug with no file, §9.4). §6.3 splits surfaces: panel = terse reason, doctor = detail, log = forensic trail. §14A pins the details: `unreadable` = "the OS error verbatim"; `reserved name` has no generic detail (it has its own pinned doctor line); doctor "enumerates within the reason, not across reasons". §13.6 names this ladder as the thing "only meaningful if pinned … nothing else asserts that".

IMPLEMENTATION:
- Status: Implemented (with later-phase amendments that are intentional, not drift)
- Location:
  - `internal/theme/load.go:74-89` — `Loader.LoadFile`: `SlugFromFilename(filepath.Base(path))` (rung 1) → `l.isReserved(slug)` (rung 2) → `os.ReadFile` → `unreadable(err)` (rung 3) → `resultFromBytes` → `lexPairs` (rung 4) → `themeFromPairs` (rungs 5–6). Every rung returns the zero `Result` alongside its rejection.
  - `internal/theme/load.go:106-112` — `parseThemeBytes` is the shared content half (disk + embedded bytes), so a built-in cannot bypass the format rungs.
  - `internal/theme/load.go:117-127` — `isReserved` (exact equality, documented as deliberately un-normalised) and `unreadable` (Detail = `err.Error()` verbatim, plus structured `Err`).
  - `internal/theme/reason.go:12-20` — the seven-reason vocabulary in ladder order; `ReasonNotFound` declared with a comment recording it sits outside the ladder.
  - `internal/theme/reason.go:27-39` — `Rejection` with the per-reason structured fields (`Line`, `BadNameCause`, `Tokens`, `Values`, `Err`), each documented as zero on every other reason.
  - Rung sources: `internal/theme/name.go:65-96` (rung 1), `internal/theme/lex.go:29-56` (rung 4), `internal/theme/validate.go:22-33` (rungs 5 then 6, in that order).
- Notes:
  - Later tasks in this plan deliberately extended the shape and none contradicts the task's intent: `Result.Source` (exact parsed bytes), `LoadPath` (content rungs only, no slug — the capturetool explicit-path input, `cmd/capturetool/main.go:142`), `LoadBuiltin` + `BuiltinSource` seam, the `events *EventLogger` seam (task 1.8), and `NewLoader`/`NewSilentLoader` which wire the real built-in slug set from the embedded directory (Phase 2). The "reserved set is empty this phase" criterion is now expressed as "an empty *injected* set never rejects" and is still pinned by test; Phase 2's `reserved_test.go` pins the populated case against every real built-in slug.
  - `not found` is produced only outside `LoadFile`, in `internal/theme/resolve.go:44-96` (`loadFromThemesDir` / `narrowReadFailure`) — exactly the §9.4 by-name path the task carved out, so the "never from `LoadFile`" invariant still holds structurally.
  - `internal/theme/loader_construction_guard_test.go` enforces that no production file assembles a `theme.Loader` literal (only `NewLoader`), so the injected reserved set can never be silently empty in production; all production call sites verified as `NewLoader`/`NewSilentLoader` (`cmd/open.go:486`, `cmd/theme.go:54`, `cmd/doctor_theme.go:52`, `cmd/capturetool/main.go:87`, `internal/tui/builtin_themes.go:8`, `internal/capture/fixtures.go:422`).

TESTS:
- Status: Adequate
- Coverage (`internal/theme/load_test.go`, plus `load_internal_test.go`, `reserved_test.go`):
  - Valid file → slug + all 19 tokens uppercase-canonical: `TestLoadFile_ValidThemeReturnsSlugAndTheme:16` (fixture values are lower case by construction in `themetest.Lines`, so the assertion proves canonicalisation rather than echo; the 19 count itself is pinned by `theme_test.go:15`).
  - Ladder short-circuit table: `TestLoadFile_LadderShortCircuits:32` — bad name beats unreadable, reserved beats bad colour, bad syntax beats bad colour, bad syntax beats missing tokens, bad colour beats missing tokens. Each asserts the exact detail string, so a rung that "wins" while leaking the other reason's copy fails.
  - Filename decided before open: `TestLoadFile_BadNameDecidedBeforeOpen:95` uses paths under a non-existent directory across four bad-name shapes and asserts `Err == nil` ("the file was never opened") — a genuinely load-bearing assertion, not a restatement of the reason.
  - Reserved from the slug alone: `TestLoadFile_ReservedNameDecidedFromSlugAlone:124` across valid / absent / mode-0000 file, plus an unreserved neighbour that still loads (guards over-matching).
  - Empty injected set: `TestLoadFile_EmptyInjectedReservedSetNeverRejects:177` across nil and allocated-empty sets × built-in-named valid and invalid files.
  - Unreadable verbatim: `TestLoadFile_UnreadableKeepsOSErrorVerbatim:211` for mode 0000 and a dangling symlink; it first captures the real `os.ReadFile` error and asserts both `Detail` and `Err` equal it, so "verbatim" is actually pinned rather than pattern-matched. `themetest.DenyRead` restores the mode in `t.Cleanup` and skips where the mode bits deny nothing (root / permissive FS) — the edge case the task called for.
  - `not found` never produced: `TestLoadFile_NotFoundIsOutsideTheLadder:249` over a 10-case corpus spanning every rung plus a valid file and an empty file.
  - Detail scoping: `TestLoadFile_DetailNeverSpansTwoReasons:303` walks a repair sequence (duplicate+bad colour+missing → bad colour+missing → missing) and asserts the surviving detail mentions no foreign reason's tokens — this is the §14A "enumerates within the reason, not across reasons" property, pinned properly.
  - Zero-Result-on-rejection is asserted by the shared `requireLoadRejection:569` helper on every rejection path (plus `TestLoadEntryPoints_RejectionReturnsTheZeroResult:609` across all three entry points), and the helper also asserts `Line == 0` for every non-`bad syntax` reason.
  - Rung-1-before-rung-2 against *real* reserved slugs is covered by the later `reserved_test.go:63` (`TestLoadFile_MixedCaseFilenameIsBadNameNotReserved`), so the one ordering pair the ladder table omits is not actually unpinned.
- Notes: Not over-tested. `rejectionCorpus` is shared by two tests with genuinely different assertions (reason identity vs. token-carrying), and the overlap between `LadderShortCircuits`'s "bad syntax beats missing tokens" row and `DetailNeverSpansTwoReasons`'s first case is justified — one pins the verdict, the other pins the detail's isolation, and both were separately named in the plan. Setup is real files in `t.TempDir()` with no mocking.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf that emits only through the injected `EventLogger` (nil-safe, `events.go:55-124`), matching CLAUDE.md's "the loader decides nothing about logging"; no `*slog.Logger` is constructed here; no `t.Parallel()`; tests live in the external `theme_test` package with a small internal test for `parseThemeBytes`; fixtures route through `internal/themetest`.
- SOLID principles: Good. The ladder is a single flat function whose ordering is the whole contract; each rung's logic lives in its own file (`name.go`, `lex.go`, `validate.go`) and `LoadFile` only sequences them. `resultFromBytes`/`parseThemeBytes` single-source the content half so disk, explicit path and embedded bytes cannot diverge. Reserved slugs and the builtin byte source are injected, not hardcoded.
- Complexity: Low. `LoadFile` is 4 branches, no nesting.
- Modern idioms: Yes — `strings.CutSuffix`, `slices`/`maps`-era helpers in the fixtures, `os.ReadFile`, structured rejection instead of string parsing downstream (`tokenAttr` explicitly renders from `Tokens`/`Values` rather than re-parsing `Detail`).
- Readability: Good. Comments explain *why* (why exact equality is safe for reserved matching, why `Err` is kept alongside `Detail`, why `not found` is absent, why the loader takes the reserved set rather than deriving it) without restating the code, and carry no task ids or spec-section references.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/load.go:10 — the `Loader` type doc still describes only this task's surface ("Loader turns one theme file into a Theme through the fixed rejection ladder"), while the type now also owns `Enumerate`/`OpenEnumeration` (enumerate.go), `ResolveByName`/`ResolveNomination`/`ResolveSlot` (resolve.go, resolution.go) and `LoadBuiltin` (builtins.go). Replace the first sentence with: "Loader is the package's entry point for turning theme files into Themes: one file through the fixed rejection ladder, a directory of candidates, the embedded built-ins, and a nominated slug or pair resolved to a palette." Keep the following sentence ("It resolves no paths and decides nothing about logging.") unchanged.
