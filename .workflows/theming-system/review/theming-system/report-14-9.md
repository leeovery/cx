TASK: theming-system-14-9 — Collapse the per-built-in test scaffolding so a new built-in enrols itself

ACCEPTANCE CRITERIA:
- One built-in file reader exists; the two per-theme readers and the open-coded reads are gone.
- One enrolment test ranges over the embedded set; adding a built-in requires no new enrolment test.
- One record shape and one assert-value-then-comment loop back the three comment guards.
- Every value/figure/marker asserted before is still asserted.
- `go test ./internal/theme` passes.

STATUS: complete

SPEC CONTEXT:
The built-in derivation records are a spec-mandated artefact, not test decoration. §7.7 ("MV's erratum values — a re-derivation check") puts the seven re-derivation figures' "home in a `#` comment beside the value in `tokyo-night-day.theme`" because the file is exported byte-faithfully to users (§12.1) and the comment travels with the value. §7.1/§13.5 do the same for the four eyeball-pinned light surface tints, and spec:1719 makes the count of four explicitly load-bearing ("All four."). §7 (Nord port, spec:608) states the corrections and the chroma-retention figures the shipped reds must carry. So the three guards under review are the only mechanism keeping non-numerically-recoverable judgements from being silently deleted from shipped files — the acceptance criterion "every value/figure/marker asserted before is still asserted" is the load-bearing one, and I verified it record-by-record against the pre-task tree rather than by inspection of the result alone. CLAUDE.md's `theme` row states the package's self-claimed property, "adding a theme is adding a file", which is what this de-duplication protects.

IMPLEMENTATION:
- Status: Implemented (with one deliberate, correct deviation from the plan's literal wording — see Notes)
- Location:
  - `internal/theme/builtins_tokyo_night_day_test.go:135` `builtinPath(slug)`, `:139` `readBuiltinFile(t, slug)` — the single reader
  - `internal/theme/builtins_tokyo_night_day_test.go:150` `derivationRecord` — the unified record shape
  - `internal/theme/builtins_tokyo_night_day_test.go:158` `assertDerivationRecords` — the lifted loop + marker sweep
  - `internal/theme/builtins_tokyo_night_day_test.go:202` `recordTokens`
  - `internal/theme/builtins_test.go:96` `TestBuiltins_AreEnrolledInFloorChecks` — the single enrolment test
  - Callers re-pointed: `builtins_test.go:23,43,177`; `builtins_nord_test.go:112,117`; `builtins_tokyo_night_day_test.go:69,84,88,109`
  - Commit `d2f49e3a`; later comment-only sweeps `25626754` (package-wide comment strip) and `f1e2e95f` (spec-citation strip) touched these files afterwards and changed no assertion.

- AC1 (one reader): Met. Grepped every `os.ReadFile` in `internal/theme/*_test.go`: the only read of a built-in source is `builtins_tokyo_night_day_test.go:143`, inside `readBuiltinFile`. `readNord` / `readTokyoNightDay` are deleted and the three open-coded reads against `tokyoNightPath` in `builtins_test.go` are gone. `tokyoNightPath` / `nordPath` survive only as message text and are now both derived from `builtinPath(slug)`.

- AC2 (one enrolment test): Met. `TestNord_IsEnrolledInFloorChecks` and `TestTokyoNightDay_IsEnrolledInFloorChecks` are deleted; `TestBuiltins_AreEnrolledInFloorChecks` names no theme, so a fourth built-in enrols with no test edit.
  Deviation, and it is the right call: the plan said "ranging over `theme.BuiltinSlugs()`". The implementation ranges over `committedBuiltinSlugs(t)` (`builtins_test.go:106`, an on-disk directory read) instead. Ranging over `BuiltinSlugs()` would have been a tautology — `embeddedThemes` (`contrast_test.go:223`) builds its map from exactly `BuiltinSlugs()` and fatals on any load failure, so membership is guaranteed by construction and the `t.Errorf` would be unreachable. The task's own Tests bullet ("confirm the enrolment test fails naming its slug") is unsatisfiable against that shape. The implementer's first attempt did write the tautology; it was caught and corrected in-cycle (`.workflows/theming-system/implementation/theming-system/fix-tracking-theming-system-14-9.md`, Attempt 1) before the commit. Comparing an independently-read committed directory against the loaded floor set is genuinely falsifiable (a `.theme` file committed but outside the `//go:embed builtins/*.theme` pattern fails naming its slug).

- AC3 (one record shape, one loop): Met. `derivationRecord{kind, token, shipped, figures, marker}` replaces `nordJudgement`, the anonymous `{token, shipped, figures}` struct and the anonymous `{token, value, anchor}` struct. All three guards now delegate: `builtins_nord_test.go:112`, `builtins_tokyo_night_day_test.go:69` and `:84`. The `nordJudgementGroups` group-of-groups wrapper is gone, replaced by `slices.Concat` of the three per-kind slices with `kind`/`marker` carried per record.

- AC4 (no claim weakened): Met. Diffed the record literals against the pre-task tree: all 13 records carry byte-identical figure lists, including the lower-case anchor `#28243a` that a generic loop cannot re-derive. Per guard:
  - Nord: value + non-empty-block + marker + figures per record, then the sweep over `theme.TokenNames()` for a falsely-claimed `Correction`/`Invention`. Preserved. The `13 + 2 + 3 + 1` arithmetic still appears in the sweep message, now via `fmt.Sprintf` over `len(nordCorrections)`/`len(nordInventions)`.
  - Tokyo-night-day eyeball pins: value + anchor-figure + `eyeball-confirmed` marker + the false-claim sweep. Preserved, and it gained a `block == ""` fatal it did not have (better diagnostics, not new detection — an absent block already failed both `strings.Contains` legs).
  - Tokyo-night-day seven re-derivations: value + non-empty-block + figures. Preserved; they carry no marker, so the sweep is a no-op for them as before.
  One documented trade: the swept marker list is now derived from the records (`assertDerivationRecords:163-169`) rather than a literal `[]string{correctionMarker, inventionMarker}`. Today's coverage is identical, but deleting every record carrying a given marker would silently retire that marker's sweep. This is a small loss of resistance to test-side hollowing, not a loss of any current claim, and it was a conscious call.

- AC5 (`go test ./internal/theme` passes): Not executed (verifier does not run tests). Read-level check is clean: imports match usage in all three files (`fmt` added to both per-theme files, `maps` moved into `builtins_test.go`, `path/filepath`/`os` retained where still used), no symbol is declared twice across the package's test files, and every referenced symbol resolves (`theme.FileExtension` at `name.go:13`, `embeddedThemes` at `contrast_test.go:223`, `committedBuiltinSlugs` at `builtins_test.go:106`).

- Notes: `internal/theme/light_pins_test.go:11` `themeIsLight` still needs a row per built-in, and `TestRowOrder_TotalAndDeterministic` needs an entry — but those are deliberate enrolment gates that fail loudly naming the slug, not silent scaffolding, and are outside this task's scope.

TESTS:
- Status: Adequate
- Coverage: This task's deliverable IS test code; the question is whether the collapsed form still falsifies everything the expanded form did. It does — see AC4. The eight falsifiable claim classes (shipped-value drift, missing marker, false marker via the sweep, missing figure, missing anchor, entirely deleted comment block, unenrolled built-in, non-embedded committed file) all still have a failing path, and the first six fail under a subtest name that states the kind and the token (`correction/state.destructive`, `eyeball pin/bg.selection`, `re-derivation/text.muted`).
- Notes:
  - Not over-tested: the collapse removed 227 lines against 220 added, most of the addition being the shared helper plus the record literals that were previously anonymous-struct rows.
  - Minor diagnostic narrowing: the eyeball-pin failure was `"%s's comment omits its dark anchor %s"` and is now the generic `"%s's comment omits %q"`. The `kind` still reaches the reader through the subtest name, so which derivation is at fault remains visible; acceptable given the task explicitly authorised the loop to move.
  - `assertDerivationRecords`'s final sweep runs `commentBlockAbove` for all 19 tokens even when `markers` is empty (the seven-values call). Harmless, and it incidentally asserts every token in the closed vocabulary is declared in the file.
  - The task's three hand-mutation checks (fourth built-in absent from floor checks, changed shipped value, falsely-claimed marker) are recorded as executed against a scratch copy in the fix-tracking file, including the empirical proof of the Attempt-1 tautology. I did not re-run them.

CODE QUALITY:
- Project conventions: Followed. Unit-lane test only (no daemon, no built binary, no tmux), no `t.Parallel()`, `t.Helper()` on every helper, package `theme_test` external-test convention held. Comment state matches the post-`25626754` standard for `internal/theme` (all rationale prose stripped package-wide); no workflow vocabulary or spec-section citation survives in these files, satisfying the task's Do-6.
- SOLID principles: Good. `assertDerivationRecords` has one responsibility (assert records, then sweep for unclaimed markers) and is parameterised by data, not by theme; each guard keeps ownership of its own records and its own test name, which is exactly the split the task asked for.
- Complexity: Low. `assertDerivationRecords` is two sequential loops with no nesting beyond the subtest closure.
- Modern idioms: Yes — `slices.Concat`, `slices.Sorted(maps.Keys(...))`, `strings.SplitSeq`, `strings.CutSuffix`, `slices.IndexFunc`. Consistent with the repo's `modernize` linter setting.
- Readability: Good, with one structural concern: the whole shared scaffolding block lives in `builtins_tokyo_night_day_test.go`, a file named for one specific built-in (see non-blocking notes).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/builtins_tokyo_night_day_test.go:135-252 — move the shared scaffolding (`builtinPath`, `readBuiltinFile`, `derivationRecord`, `assertDerivationRecords`, `recordTokens`, `declaresKey`, `valueFor`, `commentBlockAbove`) out of the theme-named file into `builtins_test.go` or a new `builtins_shared_test.go`. Today a Nord guard failure is reported at `builtins_tokyo_night_day_test.go`, and retiring that one built-in would take the whole package's scaffolding with it — the inverse of the "adding a theme is adding a file" property this task exists to protect. The task text directed the placement and the implementer's own fix-tracking flagged it as "worth moving in a later sweep"; it was not done.
- [quickfix] internal/theme/contrast_test.go:62 — `TestFloorsEnumerateTheEmbeddedSet` compares `slices.Sorted(maps.Keys(embeddedThemes(t)))` against `theme.BuiltinSlugs()`, but `embeddedThemes` (contrast_test.go:223) builds that map from `BuiltinSlugs()` itself, so the `slices.Equal` leg cannot fail. Point `want` at `committedBuiltinSlugs(t)` (builtins_test.go:106), or drop the leg and keep only the non-empty `t.Fatal`. This is the identical tautology this task's first attempt was corrected for, still standing in the sibling test — and the new `TestBuiltins_AreEnrolledInFloorChecks` now makes the corrected form free.
- [quickfix] internal/theme/builtins_tokyo_night_day_test.go:69 — replace the bare `""` fourth argument with a named constant (e.g. `const noMarkedSet = ""`) passed at the call site. Now that the explanatory doc comment has been stripped package-wide, an empty string in the `markedSet` slot reads as an oversight rather than as "these records claim no marker, so the sweep has nothing to report".
