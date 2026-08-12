TASK: theming-system-2-5 — Ship tokyo-night-day.theme And Run The Oklab Re-Derivation Check

ACCEPTANCE CRITERIA (from plan):
1. `internal/theme/builtins/tokyo-night-day.theme` parses through `LoadBuiltin` with no rejection and 19 uppercase-canonical tokens.
2. It auto-enrols in task 2-3's floor tests and passes every rule against `#e1e2e7` (with the planning-measured ratios).
3. Each of the seven checked values carries a `#` comment with all four figures — original, chroma retained, re-derivation, ΔE — and its verdict.
4. `accent.primary` carries an out-of-scope comment and is unchanged at `#8A3FD1`.
5. The four pinned tints carry their derivation + `eyeball-confirmed` comments; no other value carries a derivation comment.
6. The metric is stated once in the file header (Oklab Euclidean ΔE, OkLch chroma).
7. Any value at or over ΔE 0.05 was replaced and visually gated, with the gate outcome recorded; any value under threshold is unchanged with its figure still recorded.
8. `portal theme export tokyo-night-day` reproduces the comments verbatim — they are in the file, not in Go.
9. `internal/tui/theme/theme.go`'s inline erratum comments still present and unmodified (their deletion is Phase 3).

STATUS: complete

SPEC CONTEXT:
§7.7 ("MV's erratum values — a re-derivation check") owns this task. It names the input set self-containedly (six `§2.9 erratum` corrections plus the seventh, `text.tertiary`, and `accent.primary` explicitly out of scope) precisely because §7.1 deletes the inline erratum comments that would otherwise be the only record — the ordering trap the task calls out. It fixes three steps (re-derive in Oklab; measure chroma loss of shipped vs original; gate anything that moved), the threshold (ΔE >= 0.05, anchored against the Nord port's 0.018), the home for the figures (a `#` comment beside the value in the theme file, because export is byte-faithful, the comment travels with the value, and it survives a re-derivation that supersedes §7.3's tables), and a determinate outcome either way — "a passing check is a finding, not a non-event". §7.3 supplies the 19-value light table; §7.1 gives the eyeball pins the same home. The metric implementation was left open by the spec and the task resolved it (go-colorful `OkLab()` Euclidean, `OkLch()` chroma), requiring it be stated in the header.

IMPLEMENTATION:
- Status: Implemented, and the numeric work is real (verified independently — see below).
- Location:
  - `internal/theme/builtins/tokyo-night-day.theme` (114 lines) — the shipped file.
  - `internal/theme/builtins_tokyo_night_day_test.go` — the file's guards.
  - Enrolment is structural: `internal/theme/builtins.go:23` (`go:embed builtins/*.theme`) + `BuiltinSlugs()` derived from the embedded filenames, so the floor suite in `internal/theme/contrast_test.go:223` (`embeddedThemes`) picks the file up by existing.
- Values: all 19 match §7.3's light table exactly (spec lines 548-561 vs file lines 31-114). Token order and section headings mirror `tokyo-night.theme`, so the two built-ins read as one family.

- **The recorded figures are correct.** I re-derived several by hand from the shipped hexes rather than trusting the comments:
  - `state.positive` — the flagged case. Oklab ΔE(`#3B5E18`, `#406000`) computes to 0.01621 (file records 0.0162); chroma of `#3B5E18` / chroma of the true original `#4C7A1F` = 0.10558 / 0.13003 = 81.2% (file records 81.2%, measured against the original and NOT the intermediate `#456E1C`, exactly as the edge case demanded); the re-derivation retains 90.8% (file records 90.7%, within my hand-precision). WCAG `#3B5E18` vs `#e1e2e7` = 5.798 (file 5.80, criteria 5.797); `#406000` vs canvas = 5.62 as recorded.
  - `text.tertiary` — the re-derivation `#4C557B` measures **4.500** against `#D0C6F0`, i.e. it lands exactly on the floor it was constrained by. `text.subtle`'s re-derivation `#7A7FA5` measures **3.003** against the canvas, exactly on its band floor. Those two are strong evidence the "minimal-distance colour that clears the same floor" step was actually executed rather than asserted.
  - The correction ratios quoted in the comments reconcile with the acceptance criteria's planning measurements: `text.tertiary` 5.703 canvas / 4.573 selection (file 5.70 / 4.57), original 5.196 / 4.167 (file 5.20 / 4.17), `bg.attention` fill 1.1107 (file 1.11) — the value the edge case warned not to disturb, and it is untouched.
- Criterion 1: met — `TestLoadBuiltin_TokyoNightDayIsValid` asserts no rejection, found, slug, and all 19 tokens in canonical uppercase.
- Criterion 2: met — enrolment is automatic via `go:embed`/`BuiltinSlugs`, and two guards keep it honest (`TestFloorsEnumerateTheEmbeddedSet` in contrast_test.go:56, `TestBuiltins_AreEnrolledInFloorChecks` in builtins_test.go:96). See TESTS for the naming supersession.
- Criterion 3: met — all seven (`text.tertiary`, `text.muted`, `text.subtle`, `accent.key`, `accent.mode`, `state.positive`, `state.destructive`) carry original + chroma-retained + re-derived hex + ΔE + verdict. The originals match §7.7's table one for one, including `state.positive`'s true original `#4C7A1F` with the double-darkening chain written out (file:77).
- Criterion 4: met — file:56-60 records the out-of-scope reason (large/UI floor, cleared at 4.37 unremedied, never darkened, "no chroma for one to have cost"), value unchanged.
- Criterion 5: met — the four pins carry dark-anchor derivation + `eyeball-confirmed` (file:95-113). Carry-across from the deleted `internal/tui/theme/theme.go` is faithful: `#28243a`/`#241B10`/`#26283A`/`#292E42` anchors, fills 1.25/1.11/1.14/1.23, and the on-band legs all match the old in-source notes, with `text.strong` correctly re-labelled `text.secondary` and `text.tertiary` (4.57) added now that its own correction is recorded. The old "at the 1-9 gate" process reference was dropped — correct.
- Criterion 6: met in the file (header:17-20 states Oklab Euclidean ΔE, OkLch chroma, go-colorful, the 0.05 threshold and its anchor). Not test-guarded — see NON-BLOCKING.
- Criterion 7: met vacuously-but-recorded — all seven are under threshold (max 0.0162), so nothing moved, no fresh visual gate was owed, no §7.3-superseded line was owed, and no pin moved to task 2-6. The header states the outcome explicitly ("All seven came in under the threshold... these are the values that were always shipped"), which is the "a passing check is a finding" requirement.
- Criterion 8: met structurally — the record is entirely in the `.theme` file and `BuiltinBytes` returns verbatim bytes, so export reproduces it.
- Criterion 9: met at the time of the commit — `git show --stat e40ab859` touches only the theme file, its test, and workflow bookkeeping; `internal/tui/theme/theme.go` was untouched and was deleted later by Phase 3 (7581cd4b), as planned. Not drift.
- Notes: no throwaway derivation program leaked into the repo (commit is 4 files). Nothing outside the task's scope was touched.

TESTS:
- Status: Adequate, with one gap against criterion 6.
- Coverage:
  - `TestLoadBuiltin_TokyoNightDayIsValid` — parses, 19 tokens, canonical case (task's "it ships a valid light built-in").
  - Floor enrolment — the task named `TestTokyoNightDay_IsEnrolledInFloorChecks`; it existed in commit e40ab859 and was later generalised by task 14-9 into `TestBuiltins_AreEnrolledInFloorChecks` (builtins_test.go:96), which enumerates the committed `builtins/` directory, plus `TestFloorsEnumerateTheEmbeddedSet` asserting the floor set equals `BuiltinSlugs()`. Intentional later supersession; coverage is strictly stronger (it cannot go stale per built-in).
  - `TestTokyoNightDayFile_SevenValuesCarryDerivationComments` — parses the file and asserts each of the seven's adjacent comment block carries the original hex, chroma %, re-derived hex, ΔE and the verdict phrase. It would fail if a value were edited without updating its record (the shipped hex is asserted alongside).
  - `TestTokyoNightDayFile_PinnedTintsCarryDerivationComments` — the four pins, their anchors and the `eyeball-confirmed` marker, plus an exclusivity sweep over `theme.TokenNames()` proving no other token claims the marker. That is the "no other value carries a derivation comment" half of criterion 5, and it is the right shape (a marker-claim sweep, not a hand-listed negative).
  - `TestTokyoNightDayFile_AccentPrimaryUnchangedAndMarkedOutOfScope` — value plus both halves of the justification ("out of scope", "never darkened").
  - `TestLoadBuiltin_CommentsDoNotAffectParse` — strips comments to a temp file and asserts an identical parse, and that stripping leaves exactly `len(theme.TokenNames())` lines. Covers the task's sixth named test and doubles as a 19-key guard for this file.
  - `internal/theme/light_pins_test.go` enrols the slug in `themeIsLight` and pins the four tints' values, so the eyeball pins are guarded on both sides (file comment + Go table).
- Notes:
  - **Gap:** nothing asserts the *header* states the metric (criterion 6). Nord has exactly this guard (`TestNordFile_HeaderAttributesThePalette`, builtins_nord_test.go:116) and the reusable `leadingCommentBlock` helper is already in the package, so the day theme is the odd one out — the header could be deleted or its metric statement reworded and the suite stays green.
  - No count guard on the seven, though the four pins have one (`TestLightPins_AreExactlyFourTokens`). Dropping a row from `sevenCheckedValues` would silently unguard that value's record.
  - Not over-tested: no redundant assertions, no mocking, and the two file-parsing helpers (`valueFor`, `commentBlockAbove`) are shared with the Nord suite rather than duplicated. The tests assert the durable record's content, which is the behaviour under review, not implementation detail.
  - Correctly no test of the numeric derivation itself — a comment record is not machine-checkable without re-implementing the metric. See the NON-BLOCKING idea.

CODE QUALITY:
- Project conventions: Followed. `.theme` data file only — no Go added beyond tests. Ordering, section headings and header shape match `tokyo-night.theme` and (subsequently) `nord.theme`, so the three built-ins are a consistent family and a drop-in author copying one gets the house style. `go:embed`-derived enrolment means "adding a theme is adding a file" holds, per CLAUDE.md and §7.6.
- SOLID principles: N/A for a data file; the test file's helper split (record table + shared `assertDerivationRecords`) is single-purpose and reused by the Nord suite.
- Complexity: Low. `assertDerivationRecords`'s marker-exclusivity loop is the only non-trivial control flow and it is small and clearly named.
- Modern idioms: Yes — `strings.SplitSeq`, `slices.IndexFunc`/`Equal`/`Reverse`, `maps.Keys` with `slices.Sorted`, table-driven subtests, `t.Helper()` throughout.
- Readability: Good. The comments are prose that explains *why* rather than restating the value, and the header's two-kinds framing makes the file navigable.
- Comment accuracy: Verified numerically rather than taken on trust — see IMPLEMENTATION. No process artifacts: no task ids, no phase references, no `§` spec citations anywhere in the file (the old MV in-source notes' `§2.9` and "1-9 gate" references were deliberately dropped in the carry-across). One small framing inaccuracy noted below.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] `internal/theme/builtins_tokyo_night_day_test.go` — add a `TestTokyoNightDayFile_HeaderStatesTheMetric` mirroring `TestNordFile_HeaderAttributesThePalette`: pull the leading block with the existing `leadingCommentBlock` and assert it contains `"Oklab"`, `"ΔE"`, `"OkLch"`, `"0.05"` and `"#e1e2e7"`. Criterion 6 ("the metric is stated once in the file header so a future re-derivation is comparable") is the one acceptance criterion with no guard, and the header is the only place the metric is written down. Requires generalising `leadingCommentBlock` (builtins_nord_test.go:135), whose `t.Fatalf` hardcodes `nordPath` — take the path as a parameter.
- [quickfix] `internal/theme/builtins_tokyo_night_day_test.go:58` — add a count/vocabulary guard over `sevenCheckedValues` mirroring `TestLightPins_AreExactlyFourTokens` (light_pins_test.go:100): assert `len(sevenCheckedValues) == 7`, that the tokens are distinct, and that each is in `theme.TokenNames()`. §7.7 fixes the set at exactly seven; without the guard, deleting a row silently unguards that value's record.
- [do-now] `internal/theme/builtins/tokyo-night-day.theme:9` — the header says "Two kinds of note appear below, each beside the value it belongs to", but three kinds appear: the seven re-derivation records, the four eyeball pins, and `accent.primary`'s out-of-scope note (file:56-59). Append to the seven-values paragraph (after "...these are the values that were always shipped."): "One further value, accent.primary, carries a note recording why it is outside that set of seven rather than a check of its own."
- [quickfix] `internal/theme/builtins/tokyo-night-day.theme:94` — `canvas = #e1e2e7` is the one lower-case value among eighteen upper-case ones (the casing was carried verbatim from §7.3's table). The loader canonicalises to upper case, `wantTokyoNightDayTokens` already expects `#E1E2E7`, and `nord.theme` is uniformly upper-case, so change it to `#E1E2E7`. The file is exported byte-faithfully and is the template drop-in authors copy, so its internal casing is user-visible. (`tokyo-night.theme` carries the same wart at its `canvas` and `bg.selection` lines if the sweep is widened.)
- [idea] `internal/theme/builtins_tokyo_night_day_test.go` — consider making the recorded figures self-checking: parse each of the seven's comment for the original/re-derived hexes and the stated ΔE and chroma %, recompute both with go-colorful (already a dependency, and `contrast_test.go` already does colour math in this package) and assert agreement to the recorded precision. Upside: the exported record could never rot into a false claim, and the metric statement in the header would be executable rather than documentary. Downside: it pins a comment-text grammar in test code and adds a parser for prose; the shipped values are already pinned by `wantTokyoNightDayTokens` and `sevenCheckedValues[].shipped`, so a value change fails loudly today and the author is pointed at the comment. Worth a decision, not obviously worth doing.
