TASK: theming-system-17-7 — Single-Source Result And Enumeration Construction In internal/theme (tick-51e5b1, severity low, source: duplication)

ACCEPTANCE CRITERIA:
1. `Result{…Source: data}` is constructed in exactly one place in `internal/theme`.
2. `Enumeration{Entries: …, DirUnusable: …, DirPath: …}` is constructed in exactly one place, consumed by both `Assembler.Open` and doctor.
3. `Assembler.Open` still emits exactly one `theme: enumerated` per call, including for an absent or unusable directory; doctor still emits nothing.
4. No behaviour change for an absent, empty-path, unusable or ordinary themes directory on either caller.

STATUS: complete

SPEC CONTEXT:
- §14 (`portal theme export`, spec:1434): "Output is the file's bytes, comments included — not a re-serialisation of the parsed `Theme`." The verbatim-`Source` rule this task single-sources is exactly what makes byte-faithful export possible; re-serialisation would drop the `#` attribution header and the eyeball-pin derivation notes (§4.1, §7.1).
- §5.6 (Enumeration rules) / §5.8 (re-reads on every open): a directory read is classified once — absent is deliberately silent, unusable is `unreadable`, the parses are retained for the panel's lifetime. Two independent statements of "what a directory read means" is what would let the panel and doctor disagree about one directory, which §9.4 ("completely in the dark") and §12.2's advisory contract both depend on not happening.
- §12.2 / logging taxonomy: the `theme` component records where a theme is *used*, never where one is *diagnosed* — doctor's loader must stay silent through the refactor.

IMPLEMENTATION:
- Status: Implemented, matching the plan's seven `Do` steps exactly (commit `43e3d872`).
- Location:
  - `internal/theme/load.go:55-66` — new unexported `resultFromBytes(slug string, data []byte) (Result, *Rejection)`; the Source-is-verbatim rationale moved onto it (`:56-58`).
  - `internal/theme/load.go:88` (`LoadFile`), `:100` (`LoadPath`, `""` slug), `internal/theme/builtins.go:77-78` (`LoadBuiltin`, `found` third value unchanged) — all three tails delegate; each entry point's earlier rungs (filename `:75-81`, reserved `:79-81`, read `:83-85`/`:95-98`) stayed put, as instructed.
  - `internal/theme/enumerate.go:54-65` — new `Loader.OpenEnumeration(dir) Enumeration` beside `Enumerate`, with the empty-dir short-circuit.
  - `internal/theme/union.go:118` — `Assembler.Open` consumes it; the `events.Enumerated(union.Count, union.Rejected)` emission after `Reassemble` is untouched (`:121`).
  - `cmd/doctor_theme.go:53` — `themeAdvisoryUnion` delegates; the old `enumerateThemesDir` helper is deleted (no residue anywhere in the tree).
  - `internal/theme/theme_test.go:179` — exported-surface list updated with `Loader.OpenEnumeration`.
- AC1: verified by inspection — `Source:` appears on exactly one composite literal in the whole package (`load.go:65`). `resolution.go:136` builds a `Result` from a retained `Entry` with no `Source` by design, documented at `resolution.go:118`; it is not a bytes-carrying construction, so it does not violate the criterion.
- AC2: verified — the only `Enumeration{Entries…DirUnusable…DirPath}` literal in the tree is `enumerate.go:64`. `internal/capture/fixtures.go:516,566` fabricate `theme.Enumeration` values, but those are the hermetic offline capture fixtures that deliberately never resolve, open or stat a directory (`fixtures.go:497`); they are not a second statement of the directory-read invariant.
- AC3: `Assembler.Open`'s emission is structurally unconditional (one call site after `Reassemble`, no branch). The absent-directory case is pinned by `union_test.go:51-54` (`sink.OnlyRecord`); the unusable case is exercised but not asserted for emission (see notes).
- AC4 (the plan's step-4 confirmation): behaviour-preserving. `statThemeDir("")` → `os.Stat("")` → ENOENT → `os.IsNotExist` → `(false, nil)`, so `Enumerate("")` returned `(nil, nil)` with no `DirectoryUnusable` event, and the old inline literal produced `{Entries: nil, DirUnusable: false, DirPath: ""}` — byte-identical to the new `Enumeration{}` short-circuit. The plan's fallback branch ("keep `Assembler.Open` on the non-short-circuiting path") was correctly not taken. This is a live production path, not a hypothetical: `cmd/theme_source.go:10` discards the path-resolution error and hands `""` to `DirThemeSource.Dir`, whose doc (`dir_theme_source.go:9-11`) still holds.
- Drift: none. No later plan task supersedes this mechanism — both constructors are still the sole construction sites in the current tree.
- Notes: the consolidation also removed the only real hazard here — `ResolveByName` (the export path, `cmd/theme.go:62`) reaches `Source` through both `LoadBuiltin` and `LoadFile`, which were previously two independent constructions of the same rule and are now one.

TESTS:
- Status: Adequate.
- Coverage:
  - Task test 1 — `internal/theme/load_test.go:589` `TestLoadEntryPoints_CarryTheExactSourceBytes` and `:609` `TestLoadEntryPoints_RejectionReturnsTheZeroResult`, both driven from the single `loadEntryPoints()` table (`:635-660`) covering `LoadFile` / `LoadPath` / `LoadBuiltin`, asserting exact input bytes plus the expected slug on success and `reflect.DeepEqual(got, theme.Result{})` on rejection. Correctly stages `LoadBuiltin` through the `BuiltinSource` seam rather than a real embedded file, so all three entry points parse the *same* bytes — which is what makes the parity claim meaningful.
  - Task test 2 — `internal/theme/enumerate_test.go:409` `TestOpenEnumeration_ClassifiesEveryDirectoryState`: one table over absent / empty-path / regular-file-where-a-directory-belongs / populated (valid + missing-token file), asserting `DirUnusable`, `DirPath`, entry count, equality with a hand-assembled reference stating the rule independently (`:468-475` — the exact code deleted from doctor, so this is the "no behaviour change" pin), and equality with `Assembler.Open`'s enumeration for every state.
  - Task test 3 — `cmd/doctor_theme_enumeration_test.go:11` `TestDoctorAndPanelReadOneThemesDirectoryIdentically`: cross-package equality over a staged dir holding one valid and one broken file.
  - Doctor's one-read + silence contract: `cmd/doctor_persisted_theme_test.go:660` `TestThemeAdvisories_DirectoryIsReadOnce` (AST guard, updated to count `OpenEnumeration == 1` and to forbid `Enumerate`/`LoadFile`/`ReadDir`/`ReadFile`/`Open` in `doctor_theme.go`), and `:673` `TestPersistedThemeAdvisory_EmitsNoThemeRecords` (full `doctor` run over an unreadable dir with both advisory lines reached, asserting zero `theme` records — with a non-vacuity sub-test proving the same condition *does* emit through a loud logger).
  - Emission parity: `union_test.go:17` (absent dir → exactly one record, and it is `enumerated`), `:162` (one per open, undeduped), `:459` (`Reassemble` adds none), `cmd/theme_source_test.go:42-74` (adapter construction emits none; one per open).
- Not over-tested: the two new load tests overlap slightly with pre-existing per-entry-point assertions (`load_test.go:406-410` for `LoadPath`, `builtins_test.go:200-204` for `LoadBuiltin`/`LoadFile` against the committed file), but those assert different things (a file on disk, a committed built-in) — the new table is the cross-entry-point parity pin the shared helper warrants, and it is one table rather than three tests. Fixtures are minimal and there is no mocking beyond the existing `BuiltinSource` seam.
- Notes: the tests correctly avoid asserting through the `found` third return of `LoadBuiltin`, which the task said to leave unchanged.

CODE QUALITY:
- Project conventions: Followed. Both constructors live in `internal/theme`, which stays a leaf with no logging decisions of its own; `OpenEnumeration` emits nothing beyond what `Enumerate` already emits, so the closed `theme` component vocabulary is untouched and doctor's silent loader still writes nothing. The exported-surface list guard (`theme_test.go`) was updated in the same commit rather than left to drift. Comments explain *why* (the two-answers-to-one-question rationale at `enumerate.go:54-57`, the re-serialisation hazard at `load.go:56-58`) in the codebase's established voice; no process-artifact references, no restated code.
- SOLID: Good. One reason to change per constructor; `Loader` gains a method that composes its own primitive rather than a new dependency; the panel/doctor split now shares one rule with no shared mutable state.
- Complexity: Low. `resultFromBytes` is one branch, `OpenEnumeration` two; net −42/+240 with the deletion of one duplicated helper.
- Modern idioms: Yes. `LoadBuiltin`'s two-step `result, rejection := …; return result, rejection, true` is required (Go cannot spread a 2-value call into a 3-value return), not an oversight.
- Readability: Good. Naming is plan-prescribed and consistent with the package's `Open`/`Reassemble` panel vocabulary.
- Security / performance: No change — same syscalls, same order, one fewer `os.Stat` on the empty-path panel launch.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/load.go:51-52 — the `Source` field comment, rewritten by this task, now reads "Source is the exact bytes that were parsed. Nil on rejection.", which a caller can read as "non-nil on success"; `entryResult` (`internal/theme/resolution.go:136`) returns an accepted `Result` with a nil `Source`. Replace with: `// Source is the exact bytes that were parsed. Nil on rejection, and nil for a` / `// Result derived from a retained Enumeration (ResolveByNameFrom), which holds` / `// parses rather than files.` (The imprecision predates the task; the rewrite is the moment to fix it, and the field is the public contract `portal theme export` writes to stdout.)
- [quickfix] internal/theme/union_test.go:402-418 — `TestUnion_DirUnusableIsAFlagNotAMember` builds the assembler with `theme.NewSilentLoader()`, so nothing pins the *unusable*-directory half of acceptance criterion 3 (the absent half is pinned at `:51-54`). Build it with `logtest.NewCaptureLogger` instead and assert the open emitted exactly two records — one `directory unusable` and one `enumerated`.
- [quickfix] cmd/doctor_theme_enumeration_test.go:17 — the test is named for doctor but calls `theme.Loader.OpenEnumeration` directly, so it asserts the constructor against itself and would stay green if `doctor_theme.go` stopped using it (only the AST guard `TestThemeAdvisories_DirectoryIsReadOnce` would catch that). Anchor it in doctor's real output: compare `themeAdvisoryUnion(&DoctorDeps{ThemesDir: dir})`'s lines against `scanThemesDirectory(panelEnumeration)`'s for the same staged directory, so panel/doctor agreement is asserted through the code path doctor actually runs.
- [idea] internal/theme/enumerate.go:28 — `Enumerate` now has exactly one production caller (`OpenEnumeration`) yet remains exported, so a future consumer can still hand-assemble an `Enumeration` and re-fork the invariant this task closed; only `cmd/doctor_theme.go` is guarded against that, by an AST call-count test. Worth deciding whether to unexport it (≈24 call sites across five external `theme_test` files would move to the internal test package) or to add a repo guard that `Enumeration{…}` literals appear only in `OpenEnumeration`, with `internal/capture`'s hermetic fixtures excepted.
