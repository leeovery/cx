TASK: theming-system-7-3 — Scan The Themes Directory Into Per-File Advisories

ACCEPTANCE CRITERIA:
- A file missing two tokens produces exactly `⚠ theme <slug>: missing tokens — missing text.primary, bg.subtle`.
- A file with two bad hexes produces `⚠ theme <slug>: bad colour — text.primary = #GGGGGG, canvas = blue`.
- A duplicate-keyed file produces `⚠ theme <slug>: bad syntax — line 12: duplicate key text.primary`, naming the second occurrence's line.
- A mode-`0000` file and a dangling symlink each produce `⚠ theme <slug>: unreadable — <OS error verbatim>`, the OS error appearing exactly once.
- A valid `.theme` file produces no line; a non-`.theme` file produces no line.
- Every advisory carries `slug == Entry.Slug` and `fromPrefs == false`.
- An absent themes directory produces zero advisories, no error, and no log record of any kind.
- An unreadable directory and a regular file where the directory belongs each produce exactly one line, `⚠ themes directory unreadable: <path>`, and no per-file lines.
- A `themesDirPath()` failure yields zero theme-file advisories, and a diagnosis that still renders every check and its summary; the skip is scoped to the scan.
- A directory holding a full reject set produces zero records through a `logtest.Sink` installed for the whole run.
- Doctor never reports one file under two reasons: each entry contributes at most one line.
- The themes directory and every file in it are byte-identical after the run, and `prefs.json` is untouched by this scan.

STATUS: complete

SPEC CONTEXT:
§12.2 gives doctor three theme duties — scan the themes directory and report any file failing validity with reason and specific token/line/key, report an unresolvable persisted theme, report an unreadable/not-a-directory themes dir — with an *absent* directory silent (§5.5). §6.3 splits by surface: the panel carries the terse reason, doctor carries the enumerated detail at full width, the log is the passive trail. §6.2 fixes the ladder so a file has exactly one reason, and doctor enumerates *within* the reason, never across. §12.3 pins that the `theme` component records where a theme is *used*, never where one is *diagnosed* — doctor emits nothing. §14A pins the frames: `⚠ theme <slug>: <reason> — <detail>` (line 1844) and `⚠ themes directory unreadable: <path>` (line 1849), with the `unreadable` detail being the OS error verbatim. §12.2 also makes theme lines advisory only — they never drive the exit code.

IMPLEMENTATION:
- Status: Implemented (mechanism superseded in two places by later in-plan tasks, both intentional)
- Location:
  - `cmd/doctor.go:59` — `ThemesDir string` on `DoctorDeps`, undocumented per-field exactly as `StateDir` is (the struct-level comment at :56 covers optionality for all fields).
  - `cmd/doctor.go:101-103` — best-effort `themesDirPath()` resolution; on error the field is left empty and nothing is scanned. `cmd/doctor.go:110-112` — the test override.
  - `cmd/doctor_theme.go:38-46` — `collectThemeAdvisories(deps *DoctorDeps) []advisory`, the single entry point; converts the identity-bearing `themeAdvisory` to the render-only `advisory`.
  - `cmd/doctor_theme.go:158-170` — `scanThemesDirectory`: `DirUnusable` yields the one pinned directory line using `DirPath` (which `OpenEnumeration` sets to `deps.ThemesDir` verbatim); absent/unresolved yields an empty (non-nil) slice.
  - `cmd/doctor_theme.go:176-205` — `themeFileAdvisory`: the four content reasons render the generic frame with `slug: Entry.Slug`, `fromPrefs: false`; the two filename reasons are task 7-4's own frames; an explicit `default:` arm drops `not found`.
  - `cmd/doctor_theme.go:218-223` — `rejectionDetail`: `Rejection.Detail` verbatim, `Err.Error()` only when `Detail` is empty.
  - `cmd/doctor.go:155` and `cmd/doctor.go:176` — wired into `doctorCmd.RunE` on both the plain and the `--fix` report renders, re-collected per render.
- Notes:
  - Two intentional in-plan supersessions, neither drift. (1) The loader is `theme.NewSilentLoader()` (`cmd/doctor_theme.go:52`) rather than the task's literal `theme.NewLoader(theme.NewEventLogger(log.Discard()))`; `internal/theme/load.go:43` shows `NewSilentLoader` is exactly that expression, and it additionally keeps built-in slug reservation, which a hand-assembled `Loader` would lose (`internal/theme/loader_construction_guard_test.go` enforces no production composite literal). Outcome — zero `theme` records on every doctor path — is unchanged and directly asserted.
  - (2) `Loader.Enumerate` is reached through `Loader.OpenEnumeration` (`internal/theme/enumerate.go:58`), which folds the task's `(entries, dirRej)` pair plus the empty-path skip into a single `Enumeration{Entries, DirUnusable, DirPath}` shared with the panel. `cmd/doctor_theme_enumeration_test.go:11` pins that doctor's and the panel's reads of one directory are `DeepEqual`, which is a stronger guarantee than the task asked for. The empty-`ThemesDir` skip now lives in `OpenEnumeration` (returns the zero `Enumeration`) rather than in doctor, so `collectThemeAdvisories` still runs the persisted producer — exactly the behaviour the task's "do not return early" instruction demanded.
  - The scan is genuinely read-only: no write, no `MkdirAll`, no `prefs.json` access on the directory path, and `resolveDoctorDeps` uses `loadPrefsStoreNoMigrate` (`cmd/doctor.go:98`), so the read-only claim stays literal.
  - `cmd/testmain_isolation_test.go:19` poisons `PORTAL_THEMES_DIR` to a nonexistent path package-wide, so no whole-command doctor test can read the developer's real themes directory — the CLAUDE.md filesystem-isolation invariant holds for this feature's new env seam.

TESTS:
- Status: Adequate
- Coverage: All eleven named tests from the plan exist in `cmd/doctor_theme_test.go` and each maps to an acceptance criterion:
  - `TestThemeAdvisories_InvalidFileFrame:69` — the three-row table pins the exact expected strings for missing tokens / bad colour (in file order) / bad syntax (naming the second occurrence at line 12).
  - `TestThemeAdvisories_UnreadableFileKeepsOSError:110` — mode-`0000` file and dangling symlink, and `assertUnreadableAdvisory:129` counts the OS error occurrences to prove it appears exactly once (the double-prefix hazard).
  - `TestThemeAdvisories_ValidFileIsSilent:142` — valid `.theme` files plus `notes.txt`/`README.md`.
  - `TestThemeAdvisories_AbsentDirectoryIsSilent:155` — zero advisories, zero log records under a `logtest.Sink`, and a `Stat` proving the directory was never created.
  - `TestThemeAdvisories_UnusableDirectoryLine:172` — both unusable shapes, `requireOneAdvisory` proving no per-file line escapes even though the mode-`0000` case seeds a broken file inside, plus `slug == ""` / `fromPrefs == false`.
  - `TestThemeAdvisories_UnresolvedDirDegrades:215` — the empty-path scan, and a full `doctor` Execute under `unresolvableThemesDir` asserting no `⚠` and the intact `7 checks passed` summary. The complementary half of that criterion (the persisted producer still runs from an unresolved path) is pinned by `cmd/doctor_persisted_theme_test.go:334`.
  - `TestThemeAdvisories_DetailIsVerbatim:243` — derives the expectation from the loader's own `Reason`/`Detail` rather than a literal, so a doctor-side re-derivation of any detail format fails here; guards against vacuity by failing when a fixture loads cleanly or yields an empty detail.
  - `TestThemeAdvisories_OneReasonPerFile:278` — a doubly-broken file reports only the ladder's first reason and says nothing about presence; a mixed directory yields exactly one line per broken file.
  - `TestThemeAdvisories_FileLinesCarryTheirSlug:312` — `slug`/`fromPrefs` per advisory, in order.
  - `TestThemeAdvisories_EmitsNoThemeRecords:338` — full reject set (including a denied file) and an unusable directory, both asserting zero records; `assertNoThemeRecords` (`cmd/theme_test.go:384`) proves the capture harness is live by emitting a real `theme` event afterwards, so the zero-record assertion cannot pass vacuously.
  - `TestThemeAdvisories_ScanIsReadOnly:376` — `portaltest` fingerprint diff over the whole config root, with a pre-check that the snapshot is non-empty.
  - Plus `TestThemeAdvisories_ReachTheDoctorReport:415` covering the `RunE` wiring end-to-end, including the no-override path where `ThemesDir` can only arrive through production resolution.
  - Fixture hygiene is strong: `sourceMissingTokens`/`sourceBadColours`/`sourceDuplicateKeyAt` (`cmd/theme_test.go:502-565`) each fail loudly if the built-in stopped declaring the key they mutate, so a fixture cannot silently degrade into a valid file. `themetest.DenyRead`/`DenyDir` (`internal/themetest/deny.go`) restore the mode on cleanup and `t.Skip` when the mode bits deny nothing (root / permissive fs) — the plan's chmod-cleanup and root-skip edge case is handled at the helper, not per test.
- Notes: No under-testing found; every acceptance criterion has a test that would fail if the behaviour broke. Mild fixture-shape overlap between `InvalidFileFrame`, `DetailIsVerbatim`, `OneReasonPerFile` and `FileLinesCarryTheirSlug`, but each pins a distinct property (literal frame / no re-derivation / cardinality / identity), so this is not over-testing. No mocking beyond the log sink and temp dirs.

CODE QUALITY:
- Project conventions: Followed. Component logging stays out of `internal/theme` behind the injected seam; `cmd` performs the path resolution the loader is forbidden to do; the CLAUDE.md rule that tests must not touch real state is upheld by the `TestMain` env poison; no `t.Parallel()`; no hex or copy duplicated across files (`TestThemeAdvisories_FilenameFramesAreSingleSourced:702` even guards the frames to one declaration site).
- SOLID principles: Good. One producer per region, a single assembly point, and the render type (`advisory`) kept separate from the identity-bearing type (`themeAdvisory`) so dedup identity cannot leak into rendering.
- Complexity: Low. The deepest function is a seven-arm switch with early returns; no nesting beyond two levels; no map iteration anywhere in the assembly, which is what makes the output order deterministic by construction.
- Modern idioms: Yes — `slices.Contains`, preallocated slices, composite literals with field names, `fmt.Sprintf` against named format constants.
- Readability: Good. Comments explain *why* (silence rationale, region order, stated-not-copied slug, fresh collection per render) and none references a task id, phase or spec section.
- Issues: One dead branch, noted below. Nothing security- or performance-relevant: the directory is read exactly once per report render, deliberately (`cmd/doctor_persisted_theme_test.go:660` pins it), and no unbounded work is done per entry.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/doctor_theme.go:218-223 — `rejectionDetail`'s `Err` fallback is unreachable. `theme.unreadable` (internal/theme/load.go:126) is the only site that ever sets `Rejection.Err`, and it always sets `Detail: err.Error()` alongside it, so `Detail == "" && Err != nil` cannot hold for any rejection the enumeration produces; the branch is untestable dead code. Either collapse the helper to `return rejection.Detail` and inline it at cmd/doctor_theme.go:184, or keep the branch and replace the comment with one stating it is defensive cover for a future rejection that carries a structured error with no rendered detail (the current wording describes a double-render risk that the `unreadable` constructor no longer creates).
- [do-now] cmd/doctor_theme_test.go:403 — the `"it creates no prefs.json when there is none"` subtest runs through `themeAdvisoriesFor`, whose deps carry no `PrefsStore`, so it constrains the *directory scan* only and not the persisted producer (which `TestPersistedThemeAdvisory_UsesNonMigratingRead` covers). Add a line above it so the scope is not mistaken for prefs-producer coverage: `// The deps carry no PrefsStore: this pins that the directory scan alone never reaches prefs.json — the persisted producer's read-only property is TestPersistedThemeAdvisory_UsesNonMigratingRead's.`
