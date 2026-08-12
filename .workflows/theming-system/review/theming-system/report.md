# Implementation Review: Theming System

**Plan**: theming-system
**QA Verdict**: Request Changes

## Summary

The feature is built, and built well. 175 in-scope plan tasks were independently verified; 173 came back complete, and the two that did not are both bounded, mechanical, and touch no runtime behaviour. The theme-file loader, the 19-token vocabulary, the three built-ins, the two-slot setting with its `appearance` upgrade, the slide-over panel with live preview and per-slot commits, `portal theme export`, doctor's advisory block, and the swap-and-diff completeness guard all match the specification section for section — and where a later task in the same plan deliberately superseded an earlier one's mechanism (phases 11–17 are seven analysis-remediation cycles), the verifiers traced the supersession to the task that made it rather than reporting drift. Several verifiers went further than confirming the code compiles against the criteria: the Nord and Tokyo Night Day contrast corrections were independently recomputed and land on their floors (4.502, 4.501, 4.500, 3.003), the 182-site token rename was diffed against the retired `theme.MV` struct to prove every surviving token kept its dark value byte-for-byte, and the one changed golden expectation in the whole tree is the footer rule `#20232E` → `#292E42` that §2.2 predicted. `go build ./...`, `go test ./...` and `golangci-lint run` are all clean.

The two blocking items are both incomplete sweeps rather than defects. Task 10-10's sign-off capture clear ran, and then six panel geometry/state tapes and their PNGs were re-created for the phase 13–15 visual gates and never re-cleared — so the tree ships scaffolding that three documents written by this same feature (`testdata/vhs/README.md`, `CLAUDE.md`, `internal/capture/fixtures.go:61`) each assert is absent. Task 14-2 swept retired token names out of comments but not out of the `t.Errorf`/`t.Fatalf` strings its own Do list placed in scope, leaving 151 dead token names across 37 test files — and files where the rewritten comment and the failure message two lines below it now name different tokens. Neither blocks a user; both leave the repo asserting something false about itself, which is the specific failure mode this feature spent two phases guarding against elsewhere.

Beneath that, 488 non-blocking notes — triaged rather than worked through, for a reason the distribution makes plain: 488 over 171 tasks, mean 2.85, median 3, and only 4 tasks producing none. Defects clump; that is flat, which makes the count a function of how many verifiers ran rather than of how much is wrong. 151 are applied, 75 declined as comment prose the current standard would not have asked for, 254 marked won't-fix, 2 dropped with the CHANGELOG entry they maintained. **6 stay open**: 4 ideas and 2 bugs, and the two bugs are one condition needing a specification decision rather than a fix.

## QA Verification

### Specification Compliance

Implementation aligns with the specification. Verified against it at section granularity across all 175 tasks; the deviations found were each traced to a later task in this plan that made the change deliberately:

- **§2 vocabulary** — 19 tokens, §2.4 names in §2.4 order, one canonical `fields()` table backing `All()`/`TokenNames()`/the parser's key→field assignment. No `border.footer`, no `Light`/`Dark`, no `Mode`, no package-level `MV`.
- **§4 file format** — the lexical rules, the six-rung rejection ladder and the seven reason classes are implemented branch for branch, including the cases §4.2 enumerates individually (`bad syntax` for any leading quote matched or not, `bad colour` for a stray second `=`, duplicate-key rejection before classification, `missing tokens` for a wrong-case key).
- **§5 identity/discovery** — filename-as-slug with `^[a-z0-9][a-z0-9-]*$`, reject-never-normalise, reserved built-in slugs decided from the slug alone before any read, case-insensitive enumeration with exact-lowercase acceptance, `PORTAL_THEMES_DIR` resolving a directory with no Application Support migration and no directory creation.
- **§7 built-ins** — three shipped, each a `.theme` file parsed by the same loader as a drop-in, with the contrast floors auto-enumerated over the embedded set against each theme's own canvas.
- **§8–§10 setting, fallback, upgrade** — two-state collapse with `theme`-winning, per-slot mode-matched fallback, marker-gated one-shot `appearance` translation with the raw field preserved on disk, and a build-time guarantee replacing the rejected runtime hardcoded palette.
- **§9 panel** — union assembly, badge derivation, row ordering, geometry ladder, commit protocol, confirm, failure reporting and keymap all present and behaviourally tested through the seam.
- **§11–§13 live swap and guards** — the swap-and-diff guard enumerates the fixture registry, asserts per-fixture and union token coverage, and the exit-time canvas restore is anchored to the retained startup hex with a source guard keeping `canvasHexFor` deleted.

Two spec-vs-code reconciliations surfaced that are the spec's side of the ledger, not the code's:

- §9.1's "the header therefore costs two rows" is **corrected** in this pass: the shipped header is two-shaped, and the compact form is what §9.8's floor resolves against.
- §13.2's retention rule reads as unconditional and carries no `reference/` carve-out, though CLAUDE.md and `testdata/vhs/README.md` both now state one (task 16-9). The spec is the one document of the three that does not.

### Plan Completion

- [x] Phase acceptance criteria met — all 17 phases
- [x] All tasks completed or deliberately discarded — 175 of 176 leaf tasks done; **`theming-system-13-13`** ("Perform §13.2's Start-Of-Feature Capture Deletion And Drop The Artifact Citations") is **cancelled** and was excluded from verification by design. Its cancellation is materially connected to blocking issue 1: it was the plan's only other capture-clearing task.
- [x] Task **10-5** ("The CHANGELOG Upgrade Note") is done in the plan but its output is **reverted** (`ad49b54f`): `CHANGELOG.md` is owned by the release process, not by feature work.
- [x] No scope creep — one in-plan scope deviation recorded: task 14-14 amended a CLAUDE.md sentence its own criteria forbade touching. The new text is correct and reverting it would restore a false claim, so it is noted as scope discipline rather than a defect.

Verified independently of the verifiers:

- `go build ./...` — clean
- `go test ./...` — all packages pass
- `golangci-lint run` — 0 issues
- `go test -tags integration -p 1 ./...` — **flaky, and demonstrably not this feature's doing.** Three runs produced three different victims, all in the daemon-timing suites; the third was on the stashed pre-change tree. Every one passes on an isolated re-run, and no theming-system commit touches `internal/restore` or `cmd/bootstrap`.
- `internal/sourceguard` is gone (renamed to `internal/sourceguardtest` by 17-14); `internal/portalbintest` holds only `build.go`/`build_test.go` as 16-5 intended
- working tree is clean apart from this review's own artifacts

### Code Quality

No systemic issues. The package boundaries hold: `internal/theme` is a leaf that resolves no paths and decides nothing about logging, `internal/prefs` stayed a no-logging leaf under a guard, and `internal/themetest`/`internal/sourceguardtest` are test-only with no production importer (54 importing files, all `_test.go`).

The one recurring quality theme across the notes was **comment and doc-comment accuracy**, and it is resolved rather than carried forward. Two causes, both structural rather than careless:

1. The out-of-plan comment-strip sweeps removed rationale that individual tasks had been explicitly instructed to write. Several load-bearing warnings survive only inside a test's failure message now (`internal/theme/builtins.go`'s shipped-pair degradation trap is the clearest case), and a handful survive nowhere.
2. Comments written against a mechanism a later remediation cycle replaced. `internal/tui/theme_seams.go:8`'s "No method reads the filesystem" is falsified by `Open`, which re-reads the themes directory on every panel open by design; three separate verifiers independently flagged it.

Neither pattern touches behaviour. The first produced comments merely absent — declined. The second produced comments actively false — corrected. See **Findings disposition** below.

### Test Quality

Tests adequately verify requirements. 173 of 175 reports rate coverage adequate; only 14-2 is under-tested, and only relative to its own unmet criterion (its guard scans comments, so it is structurally blind to the strings that carry the surviving occurrences).

Strengths worth recording, because they are unusual:

- **Anti-vacuity is systematic.** Negative assertions across the topic carry positive controls — a test asserting the `theme` component emits nothing first proves the sink can see a `theme` record; a test asserting `prefs.json` is byte-identical first proves the writer is reachable; the swap-and-diff guard's coverage assertion carries a narrowed-set control proving it can fail.
- **Guards are structural, not conventional.** Single-sourcing claims are pinned by AST source guards (`defaultSlugFor` reachable from one function, `applyCommitResult` callers `== [commit]`, no `prefs.Slot*` in `internal/tui`, no hex literal in `internal/theme`), so re-duplication fails the suite rather than passing review.
- **Byte-identity was proved, not asserted.** Refactors claiming an unchanged render were verified by sorted line-level diffs of the commit (14-11's 490-line file split has zero unmatched deletions) or by hand-written pre-refactor oracles (17-13's swatch fill).

Ten notes flag assertions that cannot fail as written. The sharpest is `internal/capture/theme_panel_message_fixtures_test.go:119-125`, whose "wrapped rows are charged to the list body" check reduces algebraically to `got == got`; the behaviour it claims is genuinely covered elsewhere, so it is dead weight rather than a coverage hole.

### Required Changes

1. **Clear the residual capture scaffolding from `testdata/vhs/`** (Report 10-10, acceptance criterion 3). Six tape/PNG pairs are still tracked: `theme-panel-adaptive-pair`, `theme-panel-confirm`, `theme-panel-constant-previewing`, `theme-panel-min-height-message`, `theme-panel-narrow`, `theme-panel-paginated` — twelve files, confirmed present via `git ls-files testdata/vhs`. They are the panel geometry/state frames re-captured for the phase 13–15 gates after 10-10 ran; `theme-panel-confirm.tape:4-10` declares of itself that it is "CLEARED OUT at sign-off in Phase 10". Delete the twelve files. Keep `README.md`, `LOCK-IN.md` and `reference/`, and touch nothing in `internal/capture` or `cmd/capturetool` — deleting a fixture silently shrinks §13.4's guard rather than failing it. While there, add the sentence 10-10's own note proposes to `README.md`: sign-off means the *feature's* sign-off, and work that re-captures a screen in a later phase owns clearing what it re-captured. That is the rule this feature fell through.

2. **Finish task 14-2's sweep into test failure strings** (Report 14-2, acceptance criterion 1). 151 retired token names survive across 37 files in `internal/tui` (169 occurrences, less the 18 in the guard's own declared table); `internal/capture` is already clean in every line. The task's Do step 1 named "`t.Errorf`/`t.Fatalf` message strings" in scope and Do step 6 named them the one sanctioned non-comment change, so this is unfinished delivery, not a re-scope. The result is a file-local contradiction — `internal/tui/footer_test.go:58-65` has comments reading `accent.key`/`text.muted`/`accent.primary` directly above three `t.Errorf` strings printing `accent.blue`/`text.detail`/`accent.violet`; `internal/tui/header_test.go:34-35` carries `"accent.violet caret"` and `"text.detail subtitle"` as table data feeding failure output. Sweep the strings, then extend `TestNoRetiredTokenNameInComments` to cover string literals and rename it accordingly. Its four exemption entries are currently vacuous (the comments they were written for were stripped by `e3fa1503`) and one would wrongly bless a genuinely stale message at `help_modal_frame_test.go:20` once the guard covers strings — re-point or delete them as part of the same change.

## Findings disposition

**Applied 151** · **Declined 75** · **Won't fix 254** · **Dropped 2** · **Open 6**

488 notes over 171 tasks — mean 2.85, median 3, only 4 tasks producing none. Defects clump; that distribution is flat, which makes the count a property of how many verifiers ran rather than of how much is wrong. The notes were triaged on that basis: everything naming a concrete defect or a real decision was taken, the rest closed. Every pass verified with `go build ./...`, `go test ./...` and `golangci-lint run`.

### Applied (151)

- **Comments the code falsifies** — corrected in place or deleted, never restored.
- **Documentation and data accuracy** — CLAUDE.md, `docs/theming.md`, README (including the missing `theme export` entry), the three built-ins' group headers, the vhs retention notes.
- **Defects in the quick-fix bucket** — a slice-aliasing `append` writing into its own input; an empty `Requested` able to claim a badge; six assertions that could not fail; dead code with no caller; six guards or probes that did not catch what they claimed.
- **Specification corrections** — five false claims across four specs, each edited in place with a corrigendum, re-indexed, and committed scoped to its owning work unit.
- **Six of the eight bugs** — the swatch leaving the terminal background changed on exit; two prefs reads where the spec requires one snapshot; the export refusal printing itself twice around a usage block; the pagination counter rendering in `bubbles/list`'s own grey; a probe reporting the wrong defect; a stale guard exemption.

Sites (136):

- `internal/theme/theme.go:27` (Report 1-1)
- `internal/theme/name.go:10-12` (Report 1-4)
- `internal/theme/name.go:77` (Report 1-4)
- `internal/theme/events.go:17-18` (Report 1-8, 5-5)
- `CLAUDE.md:67` (Report 3-3)
- `internal/capture/harness.go:70-74` (Report 4-2)
- `internal/prefs/store.go:56-57` (Report 6-1, 6-3)
- `cmd/doctor.go:55-57` (Report 7-5)
- `internal/tui/theme_seams.go:5-10` (Report 8-1)
- `internal/tui/theme_seams.go:7-8` (Report 15-9, 8-7)
- `internal/tui/theme_panel_geometry.go:20` (Report 8-11, 8-17)
- `internal/tui/theme_seams.go:8` (Report 9-6)
- `cmd/open_theme_commit_test.go:95-96` (Report 11-4)
- `internal/tui/help_modal.go:15` (Report 11-8)
- `internal/theme/events.go:11-15` (Report 11-10)
- `internal/theme/union.go:173` (Report 12-1)
- `internal/tui/theme_panel.go:12-13` (Report 12-7)
- `internal/tui/theme_panel_footer.go:8` (Report 12-7)
- `internal/tui/theme_state.go:35-36` (Report 12-12)
- `internal/sourceguardtest/gosourcefiles.go:11-12` (Report 13-3, 17-14)
- `internal/theme/validate.go:11-12` (Report 13-7)
- `internal/themetest/theme_file.go:1-7` (Report 13-14)
- `internal/theme/load.go:23-25` (Report 14-12)
- `internal/theme/name.go:25-27` (Report 14-13)
- `internal/theme/union.go:107-109` (Report 15-4)
- `internal/tui/theme_panel.go:205` (Report 16-4)
- `internal/sourceguardtest/doc.go:10-12` (Report 16-5, 17-14)
- `internal/tui/theme_seams.go:7` (Report 17-4)
- `internal/themetest/theme_file.go:1-4` (Report 17-5)
- `internal/theme/load.go:51-52` (Report 17-7)
- `internal/tui/theme_panel_geometry.go:75` (Report 17-11)
- `cmd/doctor_persisted_theme_test.go:356-357` (Report 7-5)
- `CLAUDE.md:98` (Report 10-6)
- `internal/theme/name.go:78` (Report 1-4, 14-13)
- `internal/theme/name_test.go:270` (Report 1-4)
- `internal/theme/builtins/tokyo-night.theme:23` (Report 2-1)
- `internal/theme/builtins/tokyo-night-day.theme:9` (Report 2-5)
- `internal/theme/builtins/nord.theme:78-79` (Report 2-7)
- `internal/tui/model.go:991` (Report 3-1)
- `internal/tui/project_row_anatomy_test.go:56` (Report 3-1)
- `internal/capture/theme_swap_guard_test.go:373-377` (Report 4-4)
- `cmd/config.go:124` (Report 6-5)
- `CLAUDE.md` (Report 11-2, 8-1)
- `internal/tui/theme_row_test.go:106-108` (Report 8-4)
- `internal/tui/theme_panel_open_test.go:388-396` (Report 8-7)
- `internal/tui/help_modal_test.go:178-190` (Report 8-14)
- `internal/tui/theme_panel_commit_slot_test.go:452-453` (Report 9-3)
- `internal/tui/theme_panel_behaviour_test.go:18` (Report 13-10, 9-11)
- `docs/theming.md:161` (Report 10-2)
- `docs/theming.md:171-174` (Report 10-2)
- `docs/theming.md:251` (Report 10-2)
- `docs/theming.md:370` (Report 10-3)
- `docs/theming.md:364-398` (Report 10-3)
- `README.md:373` (Report 10-4)
- `testdata/vhs/LOCK-IN.md:20-22` (Report 10-10)
- `testdata/vhs/README.md:39-40` (Report 10-10)
- `internal/capture/theme_panel_fixture_render_test.go:256` (Report 11-3, 12-8)
- `cmd/doctor_theme.go:24-26` (Report 11-6)
- `cmd/open_theme_commit_test.go:307-328` (Report 12-2)
- `internal/theme/setting.go:100` (Report 12-7)
- `internal/tui/theme_panel_commit_slot_test.go:452-454` (Report 12-9)
- `internal/prefs/store_write_path_test.go:121-135` (Report 13-2)
- `internal/theme/events_test.go:142` (Report 13-7)
- `internal/tui/theme_panel_entry_test.go:35` (Report 13-10)
- `cmd/theme_source_test.go:131` (Report 13-10)
- `internal/tui/keymap_dispatch_guard_test.go:275` (Report 13-10)
- `internal/tui/theme_row_test.go:606` (Report 13-12)
- `CLAUDE.md:84` (Report 13-14, 14-8)
- `internal/tui/theme_panel_commit_load_test.go:857-859` (Report 14-3)
- `internal/tui/theme_testing_test.go:120` (Report 14-5)
- `internal/theme/name_test.go:141-147` (Report 14-13)
- `CLAUDE.md:177` (Report 14-14)
- `CLAUDE.md:176` (Report 14-14)
- `internal/theme/resolution.go:118` (Report 15-1)
- `internal/tui/theme_panel_render.go:28-34` (Report 15-2)
- `internal/theme/loader_construction_guard_test.go:61` (Report 15-4)
- `internal/theme/union_order_test.go:303-309` (Report 15-8)
- `internal/tui/theme_panel_confirm_test.go:760` (Report 15-9)
- `internal/tui/theme_testing_test.go:153` (Report 16-2)
- `internal/capture/theme_panel_fixture_test.go:48` (Report 17-6)
- `internal/tui/header_rule_test.go:60-61` (Report 17-8)
- `internal/tui/theme_flash_precedence_test.go:210-211` (Report 17-15)
- `internal/tui/theme_panel_close_report_test.go:39` (Report 17-15)
- `internal/tui/theme_flash_precedence_test.go:470` (Report 17-15)
- `internal/prefs/store.go:296` (Report 12-7)
- `internal/theme/validate.go:15` (Report 1-3)
- `internal/theme/events.go:146-148` (Report 1-8, 11-6, 5-5)
- `internal/capture/swatch.go:33-35` (Report 2-4)
- `internal/theme/embedded_test.go:30` (Report 2-8)
- `cmd/theme_test.go:800-804` (Report 2-10)
- `internal/tui/appearance_gate.go:32` (Report 3-2)
- `cmd/capturetool/swatch_test.go:44-52` (Report 3-4)
- `internal/tui/restyle_repoint_test.go:20-28` (Report 14-1, 4-1)
- `cmd/theme_test.go:1014` (Report 5-3)
- `internal/prefs/theme_savers_test.go:227-232` (Report 6-2)
- `internal/prefs/migration_marker_test.go:110-125` (Report 6-3)
- `cmd/config.go:141` (Report 6-5)
- `cmd/doctor.go:187` (Report 7-7)
- `internal/theme/union.go:214-215` (Report 8-1)
- `internal/theme/union.go:214-216` (Report 8-2)
- `internal/theme/badge.go:23` (Report 8-3)
- `internal/tui/theme_row.go:127-133` (Report 8-4)
- `internal/tui/theme_panel_footer_test.go:50-52` (Report 8-5)
- `internal/tui/theme_panel_keymap_test.go:30-39` (Report 8-5)
- `internal/tui/theme_panel_message.go:132-134` (Report 9-4)
- `internal/tui/keymap_dispatch_guard_theme_test.go:297-341` (Report 9-10)
- `internal/theme/docs_guard_test.go:111-115` (Report 10-1)
- `cmd/capturetool/main_test.go:324` (Report 12-3)
- `cmd/theme_source_test.go:136-151` (Report 13-10)
- `internal/themetest/synthetic.go:60-62` (Report 14-1)
- `internal/sourceguardtest/gosourcefiles.go:33` (Report 14-4)
- `internal/theme/contrast_test.go:62` (Report 14-9)
- `internal/tui/keymap.go:30` (Report 16-7)
- `cmd/doctor_theme.go:93` (Report 16-8)
- `cmd/prefs_translation_test.go:200-203` (Report 17-5)
- `internal/capture/harness.go:48` (Report 17-11)
- `internal/tui/builtin_themes.go:13` (Report 11-5)
- `internal/theme/light_pins_test.go:24` (Report 2-6)
- `internal/tui/pagepreview_helpers_test.go:22` (Report 3-1)
- `internal/capture/theme_swap_guard_test.go:91-93` (Report 4-3)
- `internal/theme/broken_builtin_test.go:218-221` (Report 5-6)
- `internal/prefs/appearance_api_guard_test.go:37` (Report 13-3)
- `internal/tui/theme_row_test.go:581` (Report 13-12)
- `internal/tui/retired_token_guard_test.go:38-41` (Report 14-2)
- `README.md:116-244` (Report 10-4)
- `workflows/theming-system/specification/theming-system/specification.md:954` (Report 8-17)
- `workflows/spectrum-tui-design/specification/spectrum-tui-design/specification.md:195` (Report 10-7)
- `specification.md:486` (Report 10-7)
- `workflows/cli-verb-surface-redesign/specification/cli-verb-surface-redesign/specification.md:340` (Report 10-8)
- `workflows/portal-observability-layer/specification/portal-observability-layer/specification.md:173` (Report 10-9)
- `cmd/capturetool/main.go:63-66` (Report 2-4)
- `cmd/theme.go:15` (Report 2-9)
- `internal/tui/model.go:917-933` (Report 4-1)
- `cmd/config.go:123-124` (Report 6-5)
- `internal/tui/model_test.go:26-28` (Report 12-4)
- `internal/tui/retired_token_guard_test.go:48-51` (Report 14-2)

### Declined (75)

Comment prose the current code-quality standard would not have asked for. The plan predates that standard, so tasks carried Do steps requiring comments and the later strip sweeps removed them; verifiers graded against the criteria as written.

- `internal/theme/enumerate.go:22-27` — 27` — `Enumerate`'s doc comment documents both returns but is silent about its two side-effecting emissions, which is precisely wh (Report 1-7)
- `internal/theme/contrast_test.go:20` — the const block's doc comment describes three floors but the block declares four names; add a line above `ratioIdentity`: `// rati (Report 2-3)
- `internal/theme/light_pins_test.go:11` — the scope rule is now undocumented, and "light-only" is the one thing here a reader can misread as a discount. The comment sweep w (Report 2-6)
- `internal/theme/builtins/nord.theme:41-45` — the `text.subtle` invention comment records only the interpolation arithmetic, while the file header claims an invention "is settl (Report 3-5)
- `internal/capture/grouped_subtle_locus_test.go:55` — `styledRunOpening` drops the trailing reset with no stated reason since the comment strip; restore the one-line trap above the fun (Report 3-5)
- `internal/capture/swap_harness_test.go:19-22` — `harnessWidth`/`harnessHeight` carry no rationale, and the task required one ("a comment saying why the value was chosen"); the co (Report 4-3)
- `internal/tui/restore_divergence_test.go:36` — `internal/tui/restore_divergence_test.go:36` — add one line above `withCapturedOriginal`: `// Set directly: Update renders every r (Report 4-5)
- `internal/prefs/theme_keys_test.go:141` — restore the deleted why above `TestLoadThemeKeys_NoValidationOrNormalisation`, which is now the only thing stopping a future reade (Report 5-1)
- `internal/prefs/store.go:218-219` — extend `SaveTheme`'s doc comment to name how the clear reaches disk, e.g. `// SaveTheme persists slug verbatim as the constant the (Report 6-2)
- `internal/prefs/store.go:89` — `internal/prefs/store.go:89` — `MigrationState` is exported with no doc comment, and the one property  (Report 6-3)
- `internal/prefs/store.go:262-265` — 265` — the godoc's second clause ("a failure between separate writes would persist the key with the marker unset, and the marker m (Report 6-4)
- `cmd/config.go:108` — add a comment recording that `TranslationPending` has no production reader (grep confirms: written at `:135`, read only from tests (Report 6-5)
- `cmd/doctor.go:458` — `cmd/doctor.go:458` — the record of the summary's whitespace framing is gone. The spec pins the copy but not the indentation or th (Report 7-1)
- `cmd/doctor_theme.go:58-60` — the doc comment on `assembleThemeAdvisories` pins the order and the win rule but no longer states the reserved-name non-collision, (Report 7-6)
- `cmd/doctor_theme.go:58` — the comment says "Region order is pinned (directory, files, persisted)" while the assembly receives two slices, because `scanTheme (Report 7-6)
- `internal/theme/union.go:106` — exported type `Assembler` has no doc comment (its `Loader` field does), leaving the package's panel-facing entry point undocumente (Report 8-1)
- `internal/theme/union.go:48-49` — `SortKey`'s doc no longer carries §9.5's totality rationale (it moved to `Identity`, which frames it as findability, not ordering) (Report 8-2)
- `internal/theme/union_order_test.go:158` — `themetest.WriteWithCanvas(t, dir, "aa-early.theme", "blue")` reads as "a valid theme with a blue canvas" when `blue` is in fact u (Report 8-2)
- `internal/tui/theme_row.go:30` — `FilterValue` carries no comment, so nothing at the declaration explains why an item in a list with filtering disabled implements  (Report 8-4)
- `internal/tui/theme_panel_render.go:14` — `renderThemePanel(p, height, th, colourless)` takes the theme and colourless flag beside a `themePanel` whose list delegate alread (Report 8-6)
- `internal/tui/theme_panel.go:150-151` — the two consecutive `anchorThemePanelCursor` calls read as a duplicated line; add a comment above the second, e.g. `// The capture (Report 8-6)
- `internal/tui/theme_seams.go:10` — the last doc line reads "Resolve's error is the broken-builtin fatal." The task asked the degrade policy be pinned once for all th (Report 8-8)
- `internal/tui/theme_panel.go:357` — the comment states the two deviations without naming what they deviate *from*. Replace the first line with: `// model.go's skipHea (Report 8-9)
- `internal/tui/theme_panel.go:238-240` — the plan asked for the "no re-layout on close" negative to be stated at the close itself so a reader cannot "complete" the close w (Report 8-10)
- `internal/tui/notice_band.go:193-195` — 195` — `activeProjectNoticeBand`'s doc comment no longer records *why* Projects carries only the flash contender, so the asymmetry (Report 8-12)
- `internal/tui/theme_panel.go:121-122` — the post-read refusal silently emits a `theme: enumerated` line for a panel that never opens, and nothing in source records that t (Report 8-13)
- `internal/capture/swap_harness_test.go:19-22` — the guard's single pinned render size carries no comment recording why the value is what it is; this task requires the panel's ent (Report 8-15)
- `internal/tui/theme_panel_geometry.go:61` — `themePanelHeaderShapeFor` passes `themePanelKeymap()` (the standing scope) while `themePanelListSize` reserves against the live ` (Report 8-17)
- `internal/tui/theme_panel_commit.go:75` — `internal/tui/theme_panel_commit.go:75` — the `_ =` discard of `commitSelected`'s error carries no explanation here, while the sib (Report 9-3)
- `internal/tui/keymap.go:73-74` — 74` — the comment records only the uppercase-dispatch half of what the task asked to document. Replace the two comment lines with: (Report 9-4)
- `internal/tui/theme_panel.go:238-240` — `closeThemePanel`'s comment explains the discard but not that the discard is also what cancels a live confirm (the one non-keypres (Report 9-5)
- `internal/tui/theme_panel_confirm.go:72` — the plan required the code to state in-source that the *assigned* slot is deliberately not re-loaded ("the half a reader is most l (Report 9-6)
- `internal/tui/theme_panel.go:318` — Add a one-line comment above the `keyIsCtrlC` branch recording the deliberate non-action, which is currently undocumented and read (Report 9-9)
- `internal/tui/theme_panel_behaviour_test.go:1` — `internal/tui/theme_panel_behaviour_test.go:1` — the comment strip removed the one fact about this suite that is not recoverable f (Report 9-11)
- `testdata/vhs/README.md:93` — the live-view example pairs `--fixture theme-panel-confirm` with `--theme nord`, which breaks the coherence rule the fixture's own (Report 9-12)
- `internal/tui/theme_panel.go:155-166` — `seedThemePanelMessage` hard-codes `theme.MemberLight` with no stated reason (the original commit explained it; the comment trim d (Report 9-12)
- `internal/capture/fixtures.go:468-475` — `themePanelConfirmFixture` asks in its comment to be captured at the panel's minimum width but declares no width, so `capturetool  (Report 9-12)
- `docs/theming.md:471-485` — 485` — the section documents Nord's two corrections (as scoped by §7.4 and the task) but not the three *invented* values (`text.mu (Report 10-3)
- `CLAUDE.md:41` — CLAUDE.md:41 and after :45 — the feature's user-facing verb is never introduced: `theme` appears in the bootstrap-exempt set (:126 (Report 10-6)
- `internal/theme/union.go:106` — `Assembler` is the type this task exists to create, but after the comment-audit chores it carries only a field comment; nothing st (Report 11-5)
- `internal/theme/member.go:3` — add a type doc comment to `Member`, whose sibling enums `Slot` (resolution.go:3-4) and `Badge` (badge.go:3-5) both carry one and w (Report 11-7)
- `internal/tui/theme_source_fake_test.go:20` — the `err` field comment reads as a garden-path sentence ("Returned by both Resolve, alongside the declared resolution, and LoadSlo (Report 11-9)
- `internal/tui/theme_state.go:51-53` — the `canvasMode` field doc no longer states the deliberate divergence from `gate.appearance`, which AC 2 asked for; the original i (Report 11-15)
- `internal/theme/name.go:10-13` — the doc comment on the newly exported `FileExtension` does not open with the identifier, while every other exported symbol in the  (Report 12-11)
- `internal/capture/panel_frame_test.go:21` — `fields()` is designated by AC2 as the package's single statement of the row-shape rule but carries no comment, so the non-obvious (Report 13-1)
- `internal/tui/theme_state.go:35-36` — the later comment sweep dropped the clause that records this task's finding, so the comment no longer warns a future reader off tr (Report 13-9)
- `internal/capture/theme_panel_message_fixtures_test.go:27` — `messagePanelTermWidth = 54` is now a bare literal (its ladder derivation was pruned with the surrounding comments), while the sib (Report 13-11)
- `internal/tui/theme_panel_geometry.go:59-61` — the predicate hardcodes `themePanelKeymap()` and the reason (the confirm's shorter footer would flip the header shape mid-session  (Report 15-2)
- `internal/tui/theme_state.go:51-53` — the trimmed field comment reads as if the *zero value* is what the adopt calls establish, and it dropped the name of the accessor  (Report 15-3)
- `internal/sourceguardtest/foreachfunccall.go:9` — restore the whole-declaration half of the traversal decision that acceptance criterion 2 required be stated once. Append to the do (Report 15-6)
- `internal/tui/theme_testing_test.go:202-203` — the comment's conclusion ("so a transposed shape still resolves") states the failure mode too obliquely to act as the warning it i (Report 16-1)
- `internal/tui/theme_testing_test.go:146-149` — `panelReadOnlyPath` carries no comment saying why `absentSubtest` is a field rather than a composed string, so the next maintainer (Report 16-2)
- `cmd/doctor_test.go:897` — The comment reads `... indistinguishable from "every pane is gone" - so the mass-deletion hazard guard ...`, using a spaced hyphen (Report 16-3)
- `internal/capture/fixtures.go:468-469` — 469 (`theme-panel-confirm`) and :556-559 (`theme-panel-dir-unreadable`) — decide whether to enrol these size-dependent frames in t (Report 17-11)
- `internal/tui/model.go:902-904` — the `applyCanvasMode` doc names `startupCanvasHex` as the deliberate exclusion, but the path also leaves `bubbles/list`'s `StatusB (Report 4-1)
- `internal/tui/model.go:902-904` — beyond restoring the one-sentence exclusion list above, decide where the *full* residue record (per-entry reasons, `internal/captu (Report 4-1)
- `internal/tui/theme_panel.go:82` — internal/tui/theme_panel.go:82, internal/tui/theme_panel.go:43, internal/tui/theme_state.go:18, cmd/doctor_theme.go:207 — each doc (Report 12-7)
- `internal/theme/resolve_test.go:571` — restore the coverage note above `themeCallGraph` that the task's Do step 2 asked be stated at the call site: "A function that call (Report 15-6)
- `cmd/prefs_translation_test.go:147` — add a comment on `TestLoadPrefsStore_ComputesWithoutWriting` stating that `TestMain` neutralises `persistTranslation` package-wide (Report 6-5)
- `internal/theme/load.go:10` — the `Loader` type doc still describes only this task's surface ("Loader turns one theme file into a Theme through the fixed reject (Report 1-5)
- `internal/capture/theme_swap_guard_test.go:64-77` — `carriesRun`'s terminator check is the guard's defence against SGR prefix ambiguity, but nothing says so, and with the fixed-width (Report 4-3)
- `internal/capture/theme_swap_guard_test.go:422-436` — `foregroundOnlyForms`/`backgroundOnlyForms` copy one run into BOTH the `fg` and `bg` fields, which reads like a bug until you noti (Report 4-3)
- `internal/theme/builtins.go:9-11` — the "changing these values silently breaks the property" warning this task explicitly required (§8.5's trap) was present through t (Report 5-4)
- `internal/prefs/store.go:277` — `internal/prefs/store.go:277` — the `slug == ""` disjunct sits unexplained beside three "user already chose a theme" conditions, w (Report 6-4)
- `cmd/theme_persister.go:52-55` — `themeSlotAttr` silently discards `AttrName`'s `ok`, the only unexplained decision in an otherwise fully-reasoned file. Add above  (Report 6-7)
- `cmd/doctor_theme_test.go:403` — the `"it creates no prefs.json when there is none"` subtest runs through `themeAdvisoriesFor`, whose deps carry no `PrefsStore`, s (Report 7-3)
- `internal/tui/theme_panel.go:150-151` — the two consecutive `anchorThemePanelCursor` calls read as a duplicate. Add above them: "The capture seed anchors last so a fixtur (Report 8-7)
- `internal/tui/theme_panel.go:150-151` — two back-to-back `anchorThemePanelCursor` calls read as a duplicated line; the second is the capture-only seed. Add above line 151 (Report 8-8)
- `internal/capture/theme_panel_remaining_fixtures_test.go:32` — `minimumPanelTermWidth = 28` is an unexplained literal. Add above it: `// The narrowest terminal whose content region (termW − 2*t (Report 8-16)
- `internal/tui/theme_panel_commit_test.go:175` — the subtest name "the frame is byte-identical across the keypress" overstates production behaviour: a landed commit does move the  (Report 9-1)
- `internal/tui/model_test.go:21` — add the note step 3 required, above `editFieldFocused`: "// The one inlined copy of the SGR probe: this is package tui_test, which (Report 12-4)
- `cmd/theme_persister.go:52` — `themeSlotAttr` discards `AttrName`'s presence bool with no stated reason; add the one-line why above the func: `// A Member is on (Report 12-9)
- `internal/capture/theme_panel_remaining_fixtures_test.go:32` — same for `minimumPanelTermWidth = 28`, which is the panel's unexported 24-column minimum plus `2*tui.Hinset` with nothing on the l (Report 13-11)
- `internal/themetest/theme_file.go:68-72` — the `WithDuplicateKeyAt` doc does not state the precondition every current caller enforces, so a future direct caller can stage a  (Report 14-8)
- `internal/theme/lex.go:90–97` — `wellFormedKey`'s `strings.Contains(key, "=")` clause is unreachable via `lexPairs` (the left half of a first-`=` `Cut` cannot con (Report 1-2)

### Won't fix (254)

- **Test hygiene (191)** — shared-helper extractions, redundant subtests, fixture consolidation, parameter order, naming. ~85% in `_test.go`, none touching behaviour or what the suite proves.
- **Speculative ideas (62)** — "consider whether…" / "decide whether…", proposing no change anyone is owed.
- **One spec-prose preference** — §16.3's "was once bundled with…" phrasing; not a false claim.

- `internal/tui/theme_testing_test.go` — 9
- `internal/capture/theme_swap_guard_test.go` — 8
- `internal/theme/builtins_tokyo_night_day_test.go` — 6
- `internal/capture/theme_panel_fixture_test.go` — 6
- `cmd/theme_test.go` — 5
- `internal/prefs/store_write_path_test.go` — 5
- `internal/tui/model.go` — 5
- `_test.go` — 4
- `internal/theme/embedded_test.go` — 4
- `internal/capture/fixtures.go` — 4
- `internal/tui/theme_panel_arrow_test.go` — 4
- `internal/tui/theme_panel_geometry.go` — 4
- `internal/capture/theme_panel_fixture_render_test.go` — 4
- `internal/tui/keymap_dispatch_guard_theme_test.go` — 4
- `internal/theme/loader_construction_guard_test.go` — 4
- `internal/theme/name_test.go` — 3
- `internal/theme/enumerate_test.go` — 3
- `cmd/capturetool/swatch_test.go` — 3
- `cmd/doctor_theme.go` — 3
- `internal/tui/theme_panel_cursor_test.go` — 3
- `internal/tui/theme_panel_behaviour_test.go` — 3
- `internal/theme/nomination_test.go` — 3
- `internal/theme/docs_guard_test.go` — 3
- `cmd/open.go` — 3
- `internal/theme/validate.go` — 3
- `internal/tui/builtin_theme_table_test.go` — 3
- `internal/theme/slot_default_test.go` — 3
- `internal/tui/theme_flash_precedence_test.go` — 3
- `cmd/capturetool/main.go` — 3
- `workflows/theming-system/specification/theming-system/specification.md` — 3
- `internal/theme/events_test.go` — 2
- `internal/theme/reserved_test.go` — 2
- `internal/theme/load_test.go` — 2
- `internal/theme/light_pins_test.go` — 2
- `cmd/config.go` — 2
- `cmd/open_theme_construction_test.go` — 2
- `internal/prefs/store.go` — 2
- `cmd/theme_persister_test.go` — 2
- `cmd/doctor_theme_test.go` — 2
- `cmd/doctor_fix_theme_test.go` — 2
- `internal/theme/badge_test.go` — 2
- `internal/tui/theme_row_test.go` — 2
- `internal/tui/theme_panel_geometry_test.go` — 2
- `internal/capture/theme_panel_remaining_fixtures_test.go` — 2
- `internal/tui/theme_panel_confirm.go` — 2
- `internal/tui/flash_slot_claim_test.go` — 2
- `internal/capture/theme_panel_message_fixtures_test.go` — 2
- `internal/themetest/theme_file_test.go` — 2
- `cmd/open_theme_commit_test.go` — 2
- `internal/tui/modal_footer.go` — 2
- `internal/tui/theme_panel_open_test.go` — 2
- `internal/capture/swap_harness_test.go` — 2
- `internal/log/discard_guard_test.go` — 2
- `internal/capture/panel_frame_test.go` — 2
- `internal/capture/swatch_test.go` — 2
- `cmd/doctor_test.go` — 2
- `internal/tui/theme_seams_test.go` — 2
- `cmd/doctor_theme_enumeration_test.go` — 2
- `internal/tui/build.go` — 2
- `internal/theme/enumerate.go` — 2
- `internal/tui/theme_panel_close_report_test.go` — 2
- `specification.md` — 2
- `internal/sourceguardtest/foreachfunccall.go` — 2
- `internal/theme/theme_test.go` — 1
- `internal/theme/validate_test.go` — 1
- `internal/theme/name.go` — 1
- `internal/theme/leaf_guard_test.go` — 1
- `internal/theme/builtins_test.go` — 1
- `internal/theme/contrast_test.go` — 1
- `internal/theme/builtins/tokyo-night-day.theme` — 1
- `internal/tui/delete_modal_test.go` — 1
- `internal/tui/panel.go` — 1
- `internal/tui/theme_answer_test.go` — 1
- `internal/tui/restore_source_guard_test.go` — 1
- `internal/capture/grouped_subtle_locus_test.go` — 1
- `internal/tui/restyle_repoint_test.go` — 1
- `internal/tui/restore_divergence_test.go` — 1
- `internal/prefs/theme_keys_test.go` — 1
- `internal/theme/setting_test.go` — 1
- `internal/theme/resolve_test.go` — 1
- `internal/theme/resolution_test.go` — 1
- `internal/theme/broken_builtin_test.go` — 1
- `internal/prefs/store_test.go` — 1
- `internal/prefs/theme_savers_test.go` — 1
- `cmd/doctor_advisory_test.go` — 1
- `internal/theme/union_order_test.go` — 1
- `internal/tui/notice_band.go` — 1
- `internal/tui/theme_panel_footer.go` — 1
- `internal/tui/footer_test.go` — 1
- `internal/tui/footer_revision_test.go` — 1
- `internal/tui/theme_panel_chrome_test.go` — 1
- `internal/tui/theme_panel_confirm_test.go` — 1
- `internal/tui/key_column_row_test.go` — 1
- `internal/prefs/read_shared_test.go` — 1
- `cmd/capturetool/main_test.go` — 1
- `internal/tui/canvas_cell_background_test.go` — 1
- `internal/tui/theme_panel_message_test.go` — 1
- `internal/tui/theme_panel_entry_test.go` — 1
- `internal/sourceguardtest/gosourcefiles.go` — 1
- `cmd/open_theme_nomination_test.go` — 1
- `internal/sourceguardtest/packagegofiles_test.go` — 1
- `internal/resolver/log_free_test.go` — 1
- `cmd/doctor_theme_union_test.go` — 1
- `internal/tui/burst_preflight_abort_test.go` — 1
- `theme_panel_identity_guard_test.go` — 1
- `internal/tui/theme_panel_commit_test.go` — 1
- `internal/tui/theme_panel_commit_protocol_test.go` — 1
- `internal/theme/union_test.go` — 1
- `internal/capture/fixture_render_size_test.go` — 1
- `internal/capture/capture_test.go` — 1
- `cmd/doctor_persisted_theme_test.go` — 1
- `internal/tui/retired_token_guard_test.go` — 1
- `internal/theme/builtins_nord_test.go` — 1
- `cmd/prefs_translation_persist_test.go` — 1
- `internal/tui/footer.go` — 1
- `internal/tui/filter_footer.go` — 1
- `docs/theming.md` — 1
- `CLAUDE.md` — 1
- `workflows/portal-observability-layer/specification/portal-observability-layer/specification.md` — 1
- `internal/tui/theme_state.go` — 1
- `model_test.go` — 1
- `internal/tui/theme_row.go` — 1
- `internal/themetest/synthetic.go` — 1
- `internal/sourceguardtest/packagegofiles.go` — 1
- `internal/tui/theme_panel_commit_slot_test.go` — 1
- `internal/tui/theme_panel_close_test.go` — 1
- `(unattributed)` — 1
- `internal/themetest/theme_file.go` — 1
- `internal/capture/swatch.go` — 1
- `internal/theme/lex.go` — 1

### Dropped (2)

`CHANGELOG.md` is owned by the release process. Task 10-5's entry was reverted (`ad49b54f`).


## Recommendations

### Ideas

1. `cmd/theme.go`:45 — `cmd/theme.go:45` — an argument that is empty, or that strips to empty (`portal theme export ""`, `portal theme export -- $'\n'`), renders `theme is not valid: bad name` with a doubled space and no visible subject. §14A pins the frame but says nothing about an empty `<slug>`, so choosing a rendering (quote the value, or a distinct "no theme name given" line) is a copy decision, not a mechanical edit. Add the chosen case to `TestThemeExport_BadNameFrame` once decided. (Report 2-10)
2. `internal/theme/contrast_test.go` — the suite auto-enumerates *themes* but not *tokens*: a 20th token added to `Theme.fields()` would carry no floor and nothing would fail. Consider a coverage assertion that every name in `theme.TokenNames()` is either exercised by at least one rule or listed in an explicit no-floor exemption set (`border`, `canvas`, and the two `text.on-*` pairings which are only measured on their tints). Needs a design decision on how rules declare which token they cover, which is why this is not a mechanical edit. (Report 2-3)
3. `internal/tui/model.go`:1756 — the Projects `t` arm carries no `m.commandPending` guard, unlike the `x`/`d`/`e` arms above it (:1735, :1745, :1750), so the panel opens from the command-pending mode whose footer advertises only `⏎ / n / esc`. Nothing breaks (Esc resolves innermost-first, the pending command survives) and §9.7's blocker list does not name command-pending, so this needs a decision: either add the guard for symmetry with the other suppressed page keys, or leave it live and record command-pending as deliberately non-blocking. (Report 8-13)
4. `internal/tui/theme_state.go`:89-97 — the divergence's safety is still structural-by-convention: `adoptGateAnswer` would silently clobber a converted session's answer back to the pinned gate's dark fallback if it ever ran after `adoptRetainedReply`. Today it cannot (a pinned gate is permanently resolved, so `arm`/`resolve` no-op and `syncResolvedMode` is unreachable), and `TestCommitSlotLoad_ConversionIssuesNoQuery` pins that, but the task's stated outcome was for the coupling to stop being load-bearing. Consider making it structural — e.g. record that the answer was adopted from the reply and have `adoptGateAnswer` refuse to overwrite it — which needs a decision on whether the extra state is worth more than the current test pin. (Report 15-3)

### Bugs

5. `internal/theme/resolve.go` — 2 notes
   - 63` — on a case-insensitive filesystem (macOS APFS default) the by-name path can accept a file the panel rejects: `loadFromThemesDir` composes `<themesDir>/<slug>.theme` and `LoadFile` derives the slug from that *composed* base, so a `Mine.theme` on disk is opened and accepted as slug `mine`, while `Enumerate` lists the same file as `bad name`. The no-shadowing property itself is unaffected (a built-in slug never reaches the directory — `resolve.go:32-34` consults the embedded set first), and spec §5.6 line 391 asserts the by-name path "looks for `<slug>.theme` and nothing else", which is what the code implements — so this is a spec-level edge rather than a coding mistake, and it is outside this task's criteria. Concrete change: after a successful `LoadFile` in `loadFromThemesDir`, confirm the on-disk entry name equals `slug+FileExtension` (one `os.ReadDir`/`filepath.Glob` name comparison) and return `notFound()` when it does not, so panel and launch cannot disagree about the same file. (Report 2-2)
   - a *directory* named `<slug>.theme` inside the themes dir makes `ResolveByName` return `unreadable` (ReadFile gives EISDIR; Lstat succeeds so nothing narrows), while `Enumerate` skips directories (`enumerate.go:78 resolvesToDirectory`) and the enumeration-backed path answers `not found` (`resolution.go:138` → `union.go:209`). One on-disk state, two different reasons across construction and the panel/doctor. Narrow the same way the other cases are narrowed: in `narrowReadFailure`, when the composed path stats as a directory, return `notFound()` so by-name agrees with enumeration's skip; add the case to `TestResolveByName_AbsentFileIsNotFound`. (Report 5-3)
