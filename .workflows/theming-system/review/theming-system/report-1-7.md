TASK: theming-system-1-7 — Enumerate The Themes Directory Into Classified Entries (`internal/theme/enumerate.go`)

ACCEPTANCE CRITERIA:
1. An absent directory yields zero entries, a nil rejection, and no log emission.
2. A regular file where a directory belongs, and a directory with mode `0000`, both yield `unreadable` carrying the OS error.
3. A themes directory reached through a symlink to a real directory enumerates normally.
4. `sub/inner.theme` is not enumerated — top level only, no recursion.
5. `link.theme` symlinked to a valid file elsewhere yields a valid entry whose slug derives from the link name.
6. `gone.theme`, a dangling symlink, yields an entry with reason `unreadable`.
7. A real subdirectory named `x.theme` and a symlink whose target is a directory named `y.theme` are both absent from the result with no rejection minted.
8. `Nord.THEME` yields an entry with reason `bad name`; `notes.txt`, `README` and `theme` yield no entry at all.
9. A directory holding one valid and two invalid files yields three entries — the valid one carrying a populated `Theme`, each invalid one carrying exactly one reason.

STATUS: complete

SPEC CONTEXT:
§5.5 fixes the directory-state table: **absent** is the common case and silent (no doctor line, no log, Portal never creates or seeds it), while **unreadable or a regular file where a directory belongs** is a genuine misconfiguration carrying a doctor advisory and a `theme: directory unusable` log entry, and a theme made unreachable that way carries reason `unreadable`, not `not found` ("permissions is the actual problem"). §5.6 is the rule set verbatim: top-level only; extension matched case-insensitively for *enumeration* then rejected `bad name` if not exactly lowercase (so a mis-cased file is visible but contributes no slug and cannot mint a duplicate); symlinked files followed with the slug from the link name; a dangling symlink enumerates then fails `unreadable`; the resolved root may itself be a symlink and **is** followed; directory-valued entries (real or via symlink) skipped silently — "what the entry resolves to is what decides". §5.7/§5.8 pin the cadence: no enumeration at construction, a fresh read on every panel open. §9.4 supplies the motive for entries-for-invalid-files: "there's my theme, it's registered, but it's invalid" beats being completely in the dark.

IMPLEMENTATION:
- Status: Implemented (with the plan-anticipated task 1-8 amendment)
- Location: `/Users/leeovery/Code/portal/internal/theme/enumerate.go`
  - `Entry` struct — `internal/theme/enumerate.go:14-20`, exactly the declared shape (`Path, Filename, Slug string; Theme Theme; Rejection *Rejection`).
  - `Loader.Enumerate(dir) ([]Entry, *Rejection)` — `enumerate.go:28-52`: directory read → verdict; candidate filter → directory-resolution skip → `classify` per candidate.
  - `isCandidate` — `enumerate.go:71-73`, `strings.EqualFold(filepath.Ext(name), FileExtension)`; everything else ignored with no entry and no reason.
  - `resolvesToDirectory` — `enumerate.go:78-81`, `os.Stat` (not `Lstat`), and a stat *failure* is deliberately not a skip, so a dangling symlink reaches the ladder.
  - `classify` — `enumerate.go:86-99` runs `LoadFile` (`load.go:74-89`), so all six rungs are inherited rather than re-implemented; a rejected entry keeps the slug its filename yields via `slugOrEmpty`, giving "Slug empty exactly when the reason is `bad name`" (`SlugFromFilename` is the only producer of an empty slug, and only on `bad name` — `name.go:65-75`).
  - `readThemeDir` / `statThemeDir` / `notADirectory` — `enumerate.go:109-145`: `os.IsNotExist` → `(nil, nil)` silently; not-a-directory → `unreadable` carrying a `fs.PathError{Op:"open", Err: syscall.ENOTDIR}`; any other stat error or `ReadDir` failure → `unreadable(err)` with the OS error verbatim in `Detail` and structurally matchable on `Err` (`load.go:125-127`).
  - No recursion; entries returned in `os.ReadDir` order (byte-wise filename sort in Go's `os.ReadDir`), no §9.5 display sort applied here — that lives in `Assembler.Reassemble` (`union.go:131-138`).
- Notes:
  - The plan said to leave the `Loader` **without** an event-logger seam, with task 1-8 threading it in and adding exactly three emission points. The current code carries that amendment: `l.events.DirectoryUnusable` at `enumerate.go:31` and `l.events.Rejected` at `enumerate.go:47`, with the absent-directory path emitting nothing. That is the planned successor state, not drift, and `*EventLogger` methods are nil-receiver safe (`events.go:56-124`), so `theme.Loader{}` stays usable and silent.
  - `OpenEnumeration` (`enumerate.go:58-65`) and `Enumeration` (`union.go:77-86`) are later-phase additions living in the same file; `Enumerate` has exactly one production consumer (`OpenEnumeration`), which is consumed by `Assembler.Open` (`union.go:117-124`) and `cmd/doctor_theme.go:53`. No orphan code.
  - Doctor reaches this through `theme.NewSilentLoader()` (`cmd/doctor_theme.go:52`), so the WARNs `Enumerate` now emits do not violate §12.3's "doctor itself emits nothing".
  - The synthetic not-a-directory error is coupled to the toolchain: Go 1.26's `os.ReadDir` opens with `O_DIRECTORY` (`os/file_unix.go:284-291`), so its own failure is `open <path>: not a directory` — exactly what `notADirectory` reconstructs, and `go.mod` requires `go 1.26.0`. The test asserts against the live `os.ReadDir` error, so any future toolchain change fails loudly rather than drifting silently.

TESTS:
- Status: Adequate
- Coverage: `/Users/leeovery/Code/portal/internal/theme/enumerate_test.go` carries all eleven planned tests under the planned names, one per acceptance criterion:
  - AC1 → `TestEnumerate_AbsentDirectoryIsSilent` (:17) for entries+rejection, and the "no log emission" half is pinned separately by `TestEnumerate_AbsentDirectoryEmitsNothing` (`events_test.go:363`), which loops five enumerations against a real capture sink.
  - AC2 → `TestEnumerate_RegularFileWhereDirectoryBelongs` (:30, asserts `errors.Is(..., syscall.ENOTDIR)` *and* that `Detail` equals the real `os.ReadDir` error verbatim) and `TestEnumerate_UnreadableDirectory` (:49, via `themetest.DenyDir`, which restores the mode on cleanup and skips when the mode bits deny nothing — the plan's root/chmod edge case is handled in the helper, `internal/themetest/deny.go:39-55`).
  - AC3 → `TestEnumerate_FollowsSymlinkedRoot` (:65), which also pins that the entry path is the enumerated path, not the link target.
  - AC4 → `TestEnumerate_TopLevelOnly` (:83). AC5 → `TestEnumerate_FollowsSymlinkedFiles` (:101). AC6 → `TestEnumerate_DanglingSymlinkIsUnreadable` (:115), which additionally asserts the slug survives ("only a bad name costs an entry its slug").
  - AC7 → `TestEnumerate_SkipsDirectoryValuedEntriesSilently` (:131), a two-case table (real subdir, symlink-to-dir) proving one rule rather than two.
  - AC8 → `TestEnumerate_CaseInsensitiveExtensionVisibleThenBadName` (:172, both `Nord.THEME` and `nord.Theme`, asserting filename visibility, reason, `BadNameCause` and the empty slug) and `TestEnumerate_IgnoresNonThemeFiles` (:242).
  - AC9 → `TestEnumerate_ValidAndInvalidFilesBothProduceEntries` (:258), a five-file superset that also pins `os.ReadDir`'s byte-wise order and asserts the slug-empty-iff-`bad name` invariant across every entry.
  - Extras that earn their place: `TestEnumerate_IllegalStemIsBadNameWithNoSlug` (:206, exact extension / illegal stem — a different `BadNameCause` arm), `TestEnumerate_AppliesTheInjectedReservedSlugs` (:225, proves the injected reservation reaches enumeration and that `reserved name` keeps its slug), `TestEnumerate_UsableDirectoryWithNoCandidatesIsEmptyNotNil` (:393, empty-slice-not-nil contract).
  - Emission behaviour at these call sites is covered downstream (`events_test.go:212-302`: dedup on slug+reason, path+reason for a slug-less file, path+reason for the directory verdict).
  - Would a break be caught? Yes — each assertion is behavioural (returned entries, reasons, slugs, order) and the helpers `requireValidEntry`/`requireRejectedEntry` cross-check that a rejected entry carries the zero `Theme` and a valid one carries the full token set.
- Notes:
  - One untested branch: `statThemeDir`'s non-`IsNotExist` stat failure (`enumerate.go:132-133`). The 0000-mode fixture chmods the *directory*, where `os.Stat` still succeeds and `os.ReadDir` fails, so the stat-error arm is never exercised. Non-blocking (defensive, and it lands on the same `unreadable` verdict), noted below.
  - One mirror-of-implementation assertion in `TestOpenEnumeration_ClassifiesEveryDirectoryState` (:455 / `handAssembledEnumeration` :468-475) — noted below.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays path-resolution-free (the directory is injected), the `Loader` is used by value with a shared `*EventLogger` pointer so dedup state is per-process not package-level, no raw hex or new log vocabulary is introduced, and no `t.Parallel()` appears in the tests.
- SOLID principles: Good. `Enumerate` composes four single-purpose helpers (`readThemeDir`, `isCandidate`, `resolvesToDirectory`, `classify`); the six-rung ladder is *not* duplicated here — `classify` delegates to `LoadFile`, so casing, reservation and read failures have exactly one implementation.
- Complexity: Low. `Enumerate` is one loop with two guard clauses; every helper is under ten lines with a single decision.
- Modern idioms: Yes — `strings.CutSuffix`-based slug derivation upstream, `strings.EqualFold` for the case-insensitive match, `os.DirEntry`, `fs.PathError` + `syscall` errno for structural matching, capacity-hinted slice.
- Readability: Good. Comments explain *why* each choice was made (Stat-not-Lstat, stat-failure-is-not-a-skip, deliberately-looser candidate match) rather than restating the code, and none references a task id, phase or spec section.
- Comment accuracy: Verified against the code. `resolvesToDirectory`, `classify`, `statThemeDir` and `notADirectory` all hold true; `notADirectory`'s "exact shape `os.ReadDir`'s own failure would take" is correct for the `go 1.26.0` toolchain the module requires. The only gap is that `Enumerate`'s doc comment describes returns but not its emissions (see note below).
- Security: No issue. Paths are `filepath.Join(dir, entryName)` over `os.ReadDir` output — no user-supplied traversal — and slug charset validation (`ValidSlug`, `name.go:34-45`) guards the by-name path elsewhere.
- Performance: Appropriate. One `ReadDir` plus one `Stat` and one read per `.theme` candidate, top-level only, deliberately uncached per §5.8; non-`.theme` names are filtered before any `Stat`.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `internal/theme/enumerate.go:22-27` — `Enumerate`'s doc comment documents both returns but is silent about its two side-effecting emissions, which is precisely why `cmd/doctor_theme.go:52` must construct `NewSilentLoader`. Append to the doc comment: "It emits one `theme: rejected` per rejected entry and one `theme: directory unusable` for an unusable directory; an absent directory emits nothing, and a silent loader emits nothing at all."
- [quickfix] `internal/theme/enumerate_test.go:49-63` — `statThemeDir`'s non-`IsNotExist` stat-error arm (`enumerate.go:132-133`) is unreached by the current fixtures. Add a case that stages the themes directory *inside* a parent passed to `themetest.DenyDir`, so `os.Stat(dir)` itself fails with EACCES, and assert the same `unreadable` + OS-error contract via `requireDirectoryUnusable`.
- [quickfix] `internal/theme/enumerate_test.go:455-475` — the `handAssembledEnumeration` `reflect.DeepEqual` assertion restates `OpenEnumeration`'s body (`enumerate.go:58-65`) line for line, so it can only fail if someone edits the copy; drop it and keep the adjacent `theme.Assembler{Loader: loader}.Open(...)` parity assertion (:458), which is the one that actually pins two production paths agreeing.
- [idea] `internal/theme/enumerate.go:109-145` — `statThemeDir` + `notADirectory` could collapse into classifying `os.ReadDir`'s own error (`os.IsNotExist(err)` → `(nil, nil)`, any other error → `unreadable(err)`), deleting the hand-built `fs.PathError`/`syscall.ENOTDIR` and both imports; on `go 1.26.0` `os.ReadDir` opens with `O_DIRECTORY`, so a regular file already yields the identical `open <path>: not a directory`, and a symlinked root is still followed. It departs from the plan's explicitly prescribed Stat-first shape and shifts the not-a-directory wording onto the toolchain, so it is a decision rather than a mechanical edit.
