# Implementation Review: Theming System

**Plan**: theming-system
**QA Verdict**: Request Changes

## Summary

The feature is built, and built well. 175 in-scope plan tasks were independently verified; 173 came back complete, and the two that did not are both bounded, mechanical, and touch no runtime behaviour. The theme-file loader, the 19-token vocabulary, the three built-ins, the two-slot setting with its `appearance` upgrade, the slide-over panel with live preview and per-slot commits, `portal theme export`, doctor's advisory block, and the swap-and-diff completeness guard all match the specification section for section — and where a later task in the same plan deliberately superseded an earlier one's mechanism (phases 11–17 are seven analysis-remediation cycles), the verifiers traced the supersession to the task that made it rather than reporting drift. Several verifiers went further than confirming the code compiles against the criteria: the Nord and Tokyo Night Day contrast corrections were independently recomputed and land on their floors (4.502, 4.501, 4.500, 3.003), the 182-site token rename was diffed against the retired `theme.MV` struct to prove every surviving token kept its dark value byte-for-byte, and the one changed golden expectation in the whole tree is the footer rule `#20232E` → `#292E42` that §2.2 predicted. `go build ./...`, `go test ./...` and `golangci-lint run` are all clean.

The two blocking items are both incomplete sweeps rather than defects. Task 10-10's sign-off capture clear ran, and then six panel geometry/state tapes and their PNGs were re-created for the phase 13–15 visual gates and never re-cleared — so the tree ships scaffolding that three documents written by this same feature (`testdata/vhs/README.md`, `CLAUDE.md`, `internal/capture/fixtures.go:61`) each assert is absent. Task 14-2 swept retired token names out of comments but not out of the `t.Errorf`/`t.Fatalf` strings its own Do list placed in scope, leaving 151 dead token names across 37 test files — and files where the rewritten comment and the failure message two lines below it now name different tokens. Neither blocks a user; both leave the repo asserting something false about itself, which is the specific failure mode this feature spent two phases guarding against elsewhere.

Beneath that, 488 non-blocking notes — triaged rather than worked through, for a reason the distribution makes plain: 488 over 171 tasks, mean 2.85, median 3, and only 4 tasks producing none. Defects clump; that is flat, which makes the count a function of how many verifiers ran rather than of how much is wrong. 139 are applied, 75 declined as comment prose the current standard would not have asked for, 191 marked won't-fix as the test-hygiene tail, and 2 dropped with the CHANGELOG entry they maintained. 81 stay open: 5 spec corrections, 67 ideas needing a decision, and 9 bugs, none blocking.

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

- §9.1 still states the panel header "costs **two rows**", while task 8-17 shipped a two-shape header (compact 2 rows below the affordance, page-aligned above it) and 15-2 re-derived the floor from it. The code is the decided behaviour; §9.1's sentence was not amended.
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
- `go test -tags integration -p 1 ./...` — **flaky, and demonstrably not this feature's doing.** Three runs produced three different victims, all in the daemon-timing suites; the third was on the stashed pre-change tree. Every one passes on an isolated re-run, and no theming-system commit touches `internal/restore` or `cmd/bootstrap`. This is the contention flake CLAUDE.md documents as the reason `-p 1` is load-bearing.
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

**Applied 139** · **Declined 75** · **Won't fix 191** · **Dropped 2** · **Open 81**

488 notes over 171 tasks — mean 2.85, median 3, only 4 tasks producing none. Defects clump; that distribution is flat, which makes the count a property of how many verifiers ran rather than of how much is wrong. The notes were therefore triaged rather than worked through.

### Applied (139)

Three classes, all verified with `go build ./...`, `go test ./...` and `golangci-lint run`:

- **Comments the code falsifies** — corrected in place or deleted, never restored.
- **The do-now sweep** — documentation and data accuracy, identifier renames, failure-message corrections.
- **Defects hiding in the quick-fix bucket** — a slice-aliasing `append` writing into its own input; an empty `Requested` able to claim a badge under the empty identity; six assertions that could not fail; dead code with no caller; and six guards or probes that would not catch what they claimed — an AST matcher blind to `[]Theme{{…}}`, an exemption silently covering a whole file, a bold probe that read an RGB channel of 1 as bold, an enrolment table never checked for stale rows, an in-place slice filter mutating its caller, and a helper leaving the palette zero so its colour assertions ran against no colour.

Sites (124):

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

### Declined (75)

Each asks to add, restore or re-word a comment carrying no claim the code cannot. The plan predates the project's code-quality reference constraining comments, so tasks carried Do steps requiring them and the later strip sweeps removed what those steps wrote; the verifiers graded against the criteria as written. Not required — git history holds the text if a claim ever earns its place back.

- `internal/theme/enumerate.go:22-27` — 27` — `Enumerate`'s doc comment documents both returns but is silent about its two side-effecting emissions, which is precisely why `cmd/doc (Report 1-7)
- `internal/theme/contrast_test.go:20` — the const block's doc comment describes three floors but the block declares four names; add a line above `ratioIdentity`: `// ratioIdentity  (Report 2-3)
- `internal/theme/light_pins_test.go:11` — the scope rule is now undocumented, and "light-only" is the one thing here a reader can misread as a discount. The comment sweep was right t (Report 2-6)
- `internal/theme/builtins/nord.theme:41-45` — the `text.subtle` invention comment records only the interpolation arithmetic, while the file header claims an invention "is settled at a vi (Report 3-5)
- `internal/capture/grouped_subtle_locus_test.go:55` — `styledRunOpening` drops the trailing reset with no stated reason since the comment strip; restore the one-line trap above the function: `// (Report 3-5)
- `internal/capture/swap_harness_test.go:19-22` — `harnessWidth`/`harnessHeight` carry no rationale, and the task required one ("a comment saying why the value was chosen"); the comment swee (Report 4-3)
- `internal/tui/restore_divergence_test.go:36` — `internal/tui/restore_divergence_test.go:36` — add one line above `withCapturedOriginal`: `// Set directly: Update renders every reply as lo (Report 4-5)
- `internal/prefs/theme_keys_test.go:141` — restore the deleted why above `TestLoadThemeKeys_NoValidationOrNormalisation`, which is now the only thing stopping a future reader adding a (Report 5-1)
- `internal/prefs/store.go:218-219` — extend `SaveTheme`'s doc comment to name how the clear reaches disk, e.g. `// SaveTheme persists slug verbatim as the constant theme, cleari (Report 6-2)
- `internal/prefs/store.go:89` — `internal/prefs/store.go:89` — `MigrationState` is exported with no doc comment, and the one property a future r (Report 6-3)
- `internal/prefs/store.go:262-265` — 265` — the godoc's second clause ("a failure between separate writes would persist the key with the marker unset, and the marker must stay u (Report 6-4)
- `cmd/config.go:108` — add a comment recording that `TranslationPending` has no production reader (grep confirms: written at `:135`, read only from tests): the per (Report 6-5)
- `cmd/doctor.go:458` — `cmd/doctor.go:458` — the record of the summary's whitespace framing is gone. The spec pins the copy but not the indentation or the blank-li (Report 7-1)
- `cmd/doctor_theme.go:58-60` — the doc comment on `assembleThemeAdvisories` pins the order and the win rule but no longer states the reserved-name non-collision, so a read (Report 7-6)
- `cmd/doctor_theme.go:58` — the comment says "Region order is pinned (directory, files, persisted)" while the assembly receives two slices, because `scanThemesDirectory (Report 7-6)
- `internal/theme/union.go:106` — exported type `Assembler` has no doc comment (its `Loader` field does), leaving the package's panel-facing entry point undocumented while ev (Report 8-1)
- `internal/theme/union.go:48-49` — `SortKey`'s doc no longer carries §9.5's totality rationale (it moved to `Identity`, which frames it as findability, not ordering). Extend t (Report 8-2)
- `internal/theme/union_order_test.go:158` — `themetest.WriteWithCanvas(t, dir, "aa-early.theme", "blue")` reads as "a valid theme with a blue canvas" when `blue` is in fact unparseable (Report 8-2)
- `internal/tui/theme_row.go:30` — `FilterValue` carries no comment, so nothing at the declaration explains why an item in a list with filtering disabled implements it. Add ab (Report 8-4)
- `internal/tui/theme_panel_render.go:14` — `renderThemePanel(p, height, th, colourless)` takes the theme and colourless flag beside a `themePanel` whose list delegate already carries  (Report 8-6)
- `internal/tui/theme_panel.go:150-151` — the two consecutive `anchorThemePanelCursor` calls read as a duplicated line; add a comment above the second, e.g. `// The capture-only curs (Report 8-6)
- `internal/tui/theme_seams.go:10` — the last doc line reads "Resolve's error is the broken-builtin fatal." The task asked the degrade policy be pinned once for all three panel  (Report 8-8)
- `internal/tui/theme_panel.go:357` — the comment states the two deviations without naming what they deviate *from*. Replace the first line with: `// model.go's skipHeaderRow app (Report 8-9)
- `internal/tui/theme_panel.go:238-240` — the plan asked for the "no re-layout on close" negative to be stated at the close itself so a reader cannot "complete" the close with a recl (Report 8-10)
- `internal/tui/notice_band.go:193-195` — 195` — `activeProjectNoticeBand`'s doc comment no longer records *why* Projects carries only the flash contender, so the asymmetry with `act (Report 8-12)
- `internal/tui/theme_panel.go:121-122` — the post-read refusal silently emits a `theme: enumerated` line for a panel that never opens, and nothing in source records that this is acc (Report 8-13)
- `internal/capture/swap_harness_test.go:19-22` — the guard's single pinned render size carries no comment recording why the value is what it is; this task requires the panel's entry floor t (Report 8-15)
- `internal/tui/theme_panel_geometry.go:61` — `themePanelHeaderShapeFor` passes `themePanelKeymap()` (the standing scope) while `themePanelListSize` reserves against the live `themePanel (Report 8-17)
- `internal/tui/theme_panel_commit.go:75` — `internal/tui/theme_panel_commit.go:75` — the `_ =` discard of `commitSelected`'s error carries no explanation here, while the sibling `Ente (Report 9-3)
- `internal/tui/keymap.go:73-74` — 74` — the comment records only the uppercase-dispatch half of what the task asked to document. Replace the two comment lines with: `// A sec (Report 9-4)
- `internal/tui/theme_panel.go:238-240` — `closeThemePanel`'s comment explains the discard but not that the discard is also what cancels a live confirm (the one non-keypress exit §9. (Report 9-5)
- `internal/tui/theme_panel_confirm.go:72` — the plan required the code to state in-source that the *assigned* slot is deliberately not re-loaded ("the half a reader is most likely to a (Report 9-6)
- `internal/tui/theme_panel.go:318` — Add a one-line comment above the `keyIsCtrlC` branch recording the deliberate non-action, which is currently undocumented and reads as an om (Report 9-9)
- `internal/tui/theme_panel_behaviour_test.go:1` — `internal/tui/theme_panel_behaviour_test.go:1` — the comment strip removed the one fact about this suite that is not recoverable from the co (Report 9-11)
- `testdata/vhs/README.md:93` — the live-view example pairs `--fixture theme-panel-confirm` with `--theme nord`, which breaks the coherence rule the fixture's own doc comme (Report 9-12)
- `internal/tui/theme_panel.go:155-166` — `seedThemePanelMessage` hard-codes `theme.MemberLight` with no stated reason (the original commit explained it; the comment trim dropped it) (Report 9-12)
- `internal/capture/fixtures.go:468-475` — `themePanelConfirmFixture` asks in its comment to be captured at the panel's minimum width but declares no width, so `capturetool --fixture  (Report 9-12)
- `docs/theming.md:471-485` — 485` — the section documents Nord's two corrections (as scoped by §7.4 and the task) but not the three *invented* values (`text.muted`, `tex (Report 10-3)
- `CLAUDE.md:41` — CLAUDE.md:41 and after :45 — the feature's user-facing verb is never introduced: `theme` appears in the bootstrap-exempt set (:126) and `por (Report 10-6)
- `internal/theme/union.go:106` — `Assembler` is the type this task exists to create, but after the comment-audit chores it carries only a field comment; nothing states its r (Report 11-5)
- `internal/theme/member.go:3` — add a type doc comment to `Member`, whose sibling enums `Slot` (resolution.go:3-4) and `Badge` (badge.go:3-5) both carry one and whose disti (Report 11-7)
- `internal/tui/theme_source_fake_test.go:20` — the `err` field comment reads as a garden-path sentence ("Returned by both Resolve, alongside the declared resolution, and LoadSlot."). Repl (Report 11-9)
- `internal/tui/theme_state.go:51-53` — the `canvasMode` field doc no longer states the deliberate divergence from `gate.appearance`, which AC 2 asked for; the original implementat (Report 11-15)
- `internal/theme/name.go:10-13` — the doc comment on the newly exported `FileExtension` does not open with the identifier, while every other exported symbol in the same file  (Report 12-11)
- `internal/capture/panel_frame_test.go:21` — `fields()` is designated by AC2 as the package's single statement of the row-shape rule but carries no comment, so the non-obvious part (why (Report 13-1)
- `internal/tui/theme_state.go:35-36` — the later comment sweep dropped the clause that records this task's finding, so the comment no longer warns a future reader off treating the (Report 13-9)
- `internal/capture/theme_panel_message_fixtures_test.go:27` — `messagePanelTermWidth = 54` is now a bare literal (its ladder derivation was pruned with the surrounding comments), while the sibling narro (Report 13-11)
- `internal/tui/theme_panel_geometry.go:59-61` — the predicate hardcodes `themePanelKeymap()` and the reason (the confirm's shorter footer would flip the header shape mid-session and reflow (Report 15-2)
- `internal/tui/theme_state.go:51-53` — the trimmed field comment reads as if the *zero value* is what the adopt calls establish, and it dropped the name of the accessor the whole  (Report 15-3)
- `internal/sourceguardtest/foreachfunccall.go:9` — restore the whole-declaration half of the traversal decision that acceptance criterion 2 required be stated once. Append to the doc comment, (Report 15-6)
- `internal/tui/theme_testing_test.go:202-203` — the comment's conclusion ("so a transposed shape still resolves") states the failure mode too obliquely to act as the warning it is meant to (Report 16-1)
- `internal/tui/theme_testing_test.go:146-149` — `panelReadOnlyPath` carries no comment saying why `absentSubtest` is a field rather than a composed string, so the next maintainer is likely (Report 16-2)
- `cmd/doctor_test.go:897` — The comment reads `... indistinguishable from "every pane is gone" - so the mass-deletion hazard guard ...`, using a spaced hyphen. It is th (Report 16-3)
- `internal/capture/fixtures.go:468-469` — 469 (`theme-panel-confirm`) and :556-559 (`theme-panel-dir-unreadable`) — decide whether to enrol these size-dependent frames in the new mec (Report 17-11)
- `internal/tui/model.go:902-904` — the `applyCanvasMode` doc names `startupCanvasHex` as the deliberate exclusion, but the path also leaves `bubbles/list`'s `StatusBar`/`Statu (Report 4-1)
- `internal/tui/model.go:902-904` — beyond restoring the one-sentence exclusion list above, decide where the *full* residue record (per-entry reasons, `internal/capture/swatch. (Report 4-1)
- `internal/tui/theme_panel.go:82` — internal/tui/theme_panel.go:82, internal/tui/theme_panel.go:43, internal/tui/theme_state.go:18, cmd/doctor_theme.go:207 — each doc comment o (Report 12-7)
- `internal/theme/resolve_test.go:571` — restore the coverage note above `themeCallGraph` that the task's Do step 2 asked be stated at the call site: "A function that calls nothing  (Report 15-6)
- `cmd/prefs_translation_test.go:147` — add a comment on `TestLoadPrefsStore_ComputesWithoutWriting` stating that `TestMain` neutralises `persistTranslation` package-wide, so the b (Report 6-5)
- `internal/theme/load.go:10` — the `Loader` type doc still describes only this task's surface ("Loader turns one theme file into a Theme through the fixed rejection ladder (Report 1-5)
- `internal/capture/theme_swap_guard_test.go:64-77` — `carriesRun`'s terminator check is the guard's defence against SGR prefix ambiguity, but nothing says so, and with the fixed-width generator (Report 4-3)
- `internal/capture/theme_swap_guard_test.go:422-436` — `foregroundOnlyForms`/`backgroundOnlyForms` copy one run into BOTH the `fg` and `bg` fields, which reads like a bug until you notice `observ (Report 4-3)
- `internal/theme/builtins.go:9-11` — the "changing these values silently breaks the property" warning this task explicitly required (§8.5's trap) was present through task 11-3 a (Report 5-4)
- `internal/prefs/store.go:277` — `internal/prefs/store.go:277` — the `slug == ""` disjunct sits unexplained beside three "user already chose a theme" conditions, which reads (Report 6-4)
- `cmd/theme_persister.go:52-55` — `themeSlotAttr` silently discards `AttrName`'s `ok`, the only unexplained decision in an otherwise fully-reasoned file. Add above the functi (Report 6-7)
- `cmd/doctor_theme_test.go:403` — the `"it creates no prefs.json when there is none"` subtest runs through `themeAdvisoriesFor`, whose deps carry no `PrefsStore`, so it const (Report 7-3)
- `internal/tui/theme_panel.go:150-151` — the two consecutive `anchorThemePanelCursor` calls read as a duplicate. Add above them: "The capture seed anchors last so a fixture can park (Report 8-7)
- `internal/tui/theme_panel.go:150-151` — two back-to-back `anchorThemePanelCursor` calls read as a duplicated line; the second is the capture-only seed. Add above line 151: "// The  (Report 8-8)
- `internal/capture/theme_panel_remaining_fixtures_test.go:32` — `minimumPanelTermWidth = 28` is an unexplained literal. Add above it: `// The narrowest terminal whose content region (termW − 2*tui.Hinset) (Report 8-16)
- `internal/tui/theme_panel_commit_test.go:175` — the subtest name "the frame is byte-identical across the keypress" overstates production behaviour: a landed commit does move the `●`, and t (Report 9-1)
- `internal/tui/model_test.go:21` — add the note step 3 required, above `editFieldFocused`: "// The one inlined copy of the SGR probe: this is package tui_test, which cannot re (Report 12-4)
- `cmd/theme_persister.go:52` — `themeSlotAttr` discards `AttrName`'s presence bool with no stated reason; add the one-line why above the func: `// A Member is one half of  (Report 12-9)
- `internal/capture/theme_panel_remaining_fixtures_test.go:32` — same for `minimumPanelTermWidth = 28`, which is the panel's unexported 24-column minimum plus `2*tui.Hinset` with nothing on the line saying (Report 13-11)
- `internal/themetest/theme_file.go:68-72` — the `WithDuplicateKeyAt` doc does not state the precondition every current caller enforces, so a future direct caller can stage a fixture wh (Report 14-8)
- `internal/theme/lex.go:90–97` — `wellFormedKey`'s `strings.Contains(key, "=")` clause is unreachable via `lexPairs` (the left half of a first-`=` `Cut` cannot contain `=`), (Report 1-2)

### Won't fix (191)

The quick-fix tail after every note naming a concrete defect was applied. Shared-helper extractions, redundant subtests, fixture consolidation, parameter-order and naming preferences — ~85% in `_test.go` files, none touching behaviour or what the suite proves. Applying them would rotate roughly 28,000 lines of test code for no change in coverage, against a real risk of breaking working tests. If the duplication is ever worth addressing it is worth one deliberate consolidation pass over the theme test suite, not 191 separate edits.

Recorded by target so nothing is lost; the per-task reports carry each note in full.

- `internal/tui/theme_testing_test.go` — 8
- `internal/capture/theme_swap_guard_test.go` — 6
- `internal/theme/builtins_tokyo_night_day_test.go` — 5
- `cmd/theme_test.go` — 5
- `internal/prefs/store_write_path_test.go` — 5
- `_test.go` — 4
- `internal/theme/embedded_test.go` — 4
- `internal/capture/fixtures.go` — 4
- `internal/tui/theme_panel_geometry.go` — 4
- `internal/capture/theme_panel_fixture_render_test.go` — 4
- `internal/capture/theme_panel_fixture_test.go` — 4
- `internal/tui/keymap_dispatch_guard_theme_test.go` — 4
- `internal/theme/name_test.go` — 3
- `internal/theme/enumerate_test.go` — 3
- `cmd/capturetool/swatch_test.go` — 3
- `internal/tui/theme_panel_cursor_test.go` — 3
- `internal/tui/theme_panel_arrow_test.go` — 3
- `internal/tui/model.go` — 3
- `internal/tui/theme_panel_behaviour_test.go` — 3
- `internal/theme/nomination_test.go` — 3
- `internal/theme/events_test.go` — 2
- `internal/theme/reserved_test.go` — 2
- `internal/theme/load_test.go` — 2
- `cmd/open_theme_construction_test.go` — 2
- `cmd/doctor_theme_test.go` — 2
- `cmd/doctor_theme.go` — 2
- `cmd/doctor_fix_theme_test.go` — 2
- `internal/tui/theme_panel_geometry_test.go` — 2
- `internal/capture/theme_panel_remaining_fixtures_test.go` — 2
- `internal/tui/theme_panel_confirm.go` — 2
- `internal/tui/flash_slot_claim_test.go` — 2
- `internal/capture/theme_panel_message_fixtures_test.go` — 2
- `internal/themetest/theme_file_test.go` — 2
- `cmd/open_theme_commit_test.go` — 2
- `internal/tui/theme_panel_open_test.go` — 2
- `internal/theme/docs_guard_test.go` — 2
- `internal/capture/swap_harness_test.go` — 2
- `internal/theme/validate.go` — 2
- `internal/capture/swatch_test.go` — 2
- `internal/tui/builtin_theme_table_test.go` — 2
- `cmd/doctor_test.go` — 2
- `internal/tui/theme_seams_test.go` — 2
- `internal/theme/slot_default_test.go` — 2
- `internal/tui/theme_flash_precedence_test.go` — 2
- `internal/theme/theme_test.go` — 1
- `internal/theme/validate_test.go` — 1
- `internal/theme/name.go` — 1
- `internal/theme/leaf_guard_test.go` — 1
- `internal/theme/builtins_test.go` — 1
- `internal/theme/contrast_test.go` — 1
- `internal/theme/builtins/tokyo-night-day.theme` — 1
- `internal/theme/light_pins_test.go` — 1
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
- `cmd/config.go` — 1
- `internal/prefs/store_test.go` — 1
- `internal/prefs/theme_savers_test.go` — 1
- `internal/prefs/store.go` — 1
- `cmd/theme_persister_test.go` — 1
- `cmd/doctor_advisory_test.go` — 1
- `internal/theme/union_order_test.go` — 1
- `internal/theme/badge_test.go` — 1
- `internal/tui/theme_row_test.go` — 1
- `internal/tui/notice_band.go` — 1
- `internal/tui/theme_panel_footer.go` — 1
- `internal/tui/footer_test.go` — 1
- `internal/tui/footer_revision_test.go` — 1
- `internal/tui/theme_panel_chrome_test.go` — 1
- `internal/tui/theme_panel_confirm_test.go` — 1
- `internal/tui/key_column_row_test.go` — 1
- `internal/tui/modal_footer.go` — 1
- `internal/prefs/read_shared_test.go` — 1
- `cmd/capturetool/main_test.go` — 1
- `internal/tui/canvas_cell_background_test.go` — 1
- `internal/tui/theme_panel_message_test.go` — 1
- `internal/tui/theme_panel_entry_test.go` — 1
- `internal/sourceguardtest/gosourcefiles.go` — 1
- `internal/log/discard_guard_test.go` — 1
- `cmd/open.go` — 1
- `internal/capture/panel_frame_test.go` — 1
- `cmd/open_theme_nomination_test.go` — 1
- `internal/sourceguardtest/packagegofiles_test.go` — 1
- `internal/resolver/log_free_test.go` — 1
- `cmd/doctor_theme_union_test.go` — 1
- `internal/theme/loader_construction_guard_test.go` — 1
- `internal/tui/burst_preflight_abort_test.go` — 1
- `theme_panel_identity_guard_test.go` — 1
- `internal/tui/theme_panel_commit_test.go` — 1
- `internal/tui/theme_panel_commit_protocol_test.go` — 1
- `internal/theme/union_test.go` — 1
- `cmd/doctor_theme_enumeration_test.go` — 1
- `internal/capture/fixture_render_size_test.go` — 1
- `internal/capture/capture_test.go` — 1
- `internal/tui/build.go` — 1
- `cmd/doctor_persisted_theme_test.go` — 1
- `internal/tui/retired_token_guard_test.go` — 1

### Dropped (2)

`CHANGELOG.md` is owned by the release process. Task 10-5's entry was reverted (`ad49b54f`), so the notes maintaining it describe something that no longer exists.


## Recommendations

### Do now

*5 notes across 4 targets. All five amend a completed specification — four in other work units, which the historical-artifact correction protocol gates on explicit confirmation.*

1. `specification.md` — 2 notes
   - `…/specification.md:676` (and the same clause at `:73`) — "The user-overridable theme system **it was once bundled with** is delivered; transparency **did not come with it**" is the "was X, now Y" residue AC8 excludes from a section body; §2.8 already states delivery, so §16.3 needs no history. Replace the §16.3 bullet with: "**The \"use terminal background\" transparency opt-out** — a distinguished value meaning \"use the terminal default\", which the theme file format leaves the door open for as a purely additive loader change (§2.8). Deferred on its own terms." and trim §1's line 73 to "… is **deferred on its own terms** (§16.3): the theme file format leaves the door open for a distinguished value meaning \"use the terminal default\"." (Report 10-7)
   - `…/specification.md:486` — "Every other contender is Sessions-only with no Projects analogue" is contradicted by §11.4's command-pending banner, which is Projects-only and shares the same slot (`internal/tui/notice_band.go:196-204` ranks the flash above `commandPending`). Replace with: "Every other contender **in the order above** is Sessions-only; §11.4's command-pending banner is Projects' own persistent band and sits directly beneath the flash." (The imprecision was transcribed from the theming-system spec's own wording at specification.md:1825, so consider correcting there too.) (Report 10-7)
2. `workflows/cli-verb-surface-redesign/specification/cli-verb-surface-redesign/specification.md`:340 — `.workflows/cli-verb-surface-redesign/specification/cli-verb-surface-redesign/specification.md:340` — the bullet retains the pre-amendment absolutes "exits **0 iff everything is healthy post-repair, non-zero if anything remains unhealthy or unfixable**" and scopes them only by the trailing "— driven **solely** by the post-repair Portal-health checks". Corrigendum entry 2 (`:5`) quotes that exact phrase as the claim being corrected, so the wording it refutes still stands in the body. Replace with "exits **0 iff every Portal-health check passes post-repair, non-zero if any Portal-health check remains unhealthy or unfixable**" and drop the now-redundant "— driven **solely** by the post-repair Portal-health checks" clause opener to "The user-content scan is read-only, so it runs on the `--fix` path too: …". (Report 10-8)
3. `workflows/spectrum-tui-design/specification/spectrum-tui-design/specification.md`:195 — `.workflows/spectrum-tui-design/specification/spectrum-tui-design/specification.md:195` — the last §2.9 rule still reads "All values are a **hypothesis until prototyped in a real terminal (§15)**; the table is the build target, validation is the lock", which re-asserts the table as the authority the preamble at line 149 just demoted (and is itself false now that the palette shipped). Replace with: "The values were prototyped and locked in a real terminal (§15); the theme files carry them, and this table records what that validation settled." (Report 10-7)
4. `workflows/theming-system/specification/theming-system/specification.md`:954 — .workflows/theming-system/specification/theming-system/specification.md:954 — §9.1 still states "The header therefore costs **two rows**, which is what §9.8's minimum-height rule (header + footer + one row) resolves against", which the shipped page-aligned header (page header block + section header) contradicts for the render, and the spec nowhere records the inner gutter or the below-the-rule border start. This task's own edge-case note deferred the amendment to Phase 10, and no Phase 10 task took it (10-7/10-8/10-9 amend the MV spec's keymap, doctor and log vocabulary only). Replace the sentence with: "The panel's header region is measured off the page's own header and section-header blocks, so `Themes` lands on the `Sessions N` row and the panel's first row on the page's first session row; below the height that affords those alignment rows the header degrades to a compact two-row form (rule then label), and it is that compact cost — never the page-aligned one — that §9.8's minimum-height rule resolves against." Prior analysis cycles discarded the *width* half of this divergence as spec-process work after 13-11 restored the 24–30 band; the header half is still outstanding and is now the only place the two artefacts disagree about the panel's anatomy. (Report 8-17)

### Ideas

*67 notes across 52 targets. Each needs a decision rather than an edit.*

5. `cmd/capturetool/main.go` — 3 notes
   - decide whether the path branch should expand a leading `~` (e.g. via `resolver.ExpandTilde`) before `theme.LoadPath`: the documented invocation relies on the shell expanding it, so a quoted argument or a tape line fails as `unreadable: open ~/themes/x.theme: no such file or directory`. The error names the right class of problem, so this is a convenience call, not a correctness one — and it would add an `internal/resolver` import to a binary deliberately kept thin. (Report 3-4)
   - `captureKeys` is replayed only by `ModelAt`; the live `capturetool --fixture <name>` path builds the model and hands it to `tea.NewProgram` without them, so the fixtures that need a key to reach their captured state (`projects`, `sessions-empty`, `preview-screen`, and every `theme-panel-*`) open on a different screen than the harness renders, and the user has to know which key to press. Decide whether capturetool should feed the fixture's declared script at start-up (which needs an exported accessor on `Fixture`, since `captureKeys` is unexported) or whether the manual keypress is the intended interactive behaviour. (Report 4-2)
   - decide how to tell a human that the live terminal is smaller than a fixture's declared size, where the pinned frame now silently wraps or clips (the tapes' "resize your window" instruction used to cover this by hand). Placement is the open question: `main.go:46-47` deliberately writes warnings before the program starts so they land on the primary screen, while the size is only known once the first resize reaches the filter. (Report 17-11)
6. `internal/theme/loader_construction_guard_test.go` — 3 notes
   - `NewLoader` now rejects a literal nil seam, but a future production call site could still reach unnamed silence through `theme.NewEventLogger(nil)` or `theme.NewEventLogger(log.Discard())`, which the grep in criterion 1 would not surface. Decide whether to extend this production-only scan to reject those two argument shapes outside `NewSilentLoader`; that would make the criterion's outcome ("one grep answers where Portal writes no `theme` records") structurally enforced rather than currently-true. (Report 14-12)
   - the guard covers composite literals only, so the zero value stays reachable from production by other routes: `var l theme.Loader`, `new(theme.Loader)`, an elided-type element (`[]theme.Loader{{}}`, `map[string]theme.Loader{"a": {}}`), and — most realistically — a sibling struct written without its Loader field, since both `theme.Assembler` (union.go:106-111) and `theme.DirThemeSource` (dir_theme_source.go:6-12) hold an exported `Loader` field, so `theme.DirThemeSource{Dir: d}` yields a reserve-nothing loader with no `Loader` literal anywhere. Decide how far the guarantee should extend — e.g. also flag `var`/`new` of the type and require the `Loader` field to be present in `Assembler`/`DirThemeSource` literals, versus accepting composite-literal coverage as the pragmatic line. (Report 15-4)
   - the production/test split is the `_test.go` suffix, so a non-`_test.go` helper in a test-only package (`internal/themetest`, `internal/capture`, `internal/spawntest`) is treated as production and would fail the guard even though it never links into the shipped binary. Decide whether those packages should be exempt by path, or whether the current strictness is the intended contract. (Report 15-4)
7. `internal/tui/model.go` — 3 notes
   - the Projects `t` arm carries no `m.commandPending` guard, unlike the `x`/`d`/`e` arms above it (:1735, :1745, :1750), so the panel opens from the command-pending mode whose footer advertises only `⏎ / n / esc`. Nothing breaks (Esc resolves innermost-first, the pending command survives) and §9.7's blocker list does not name command-pending, so this needs a decision: either add the guard for symmetry with the other suppressed page keys, or leave it live and record command-pending as deliberately non-blocking. (Report 8-13)
   - AC 1 ("exactly one function") is currently held by convention only; a fourth list can re-inline the sequence without any test failing. Consider a unit-lane source guard (the repo already has ~20 driven by `sourceguardtest`) asserting that no production file outside `model.go` assigns `Styles.NoItems`, `Styles.ActivePaginationDot`, `Styles.HelpStyle` or `Styles.TitleBar` — decide whether the guard is worth its maintenance against the swap-and-diff net that already covers the rendered consequence. (Report 11-1)
   - `WithCanvasMode` is exported but has no caller outside `internal/tui`'s own tests (`internal/capture` and `cmd` use only `WithServerStarted`/`WithProgressReceiver`/`WithThemeNomination`). Pre-existing, and this task only retyped its parameter; consider unexporting it to `withCanvasMode` so the exported Option set matches the seams external packages actually wire. - None. (Report 14-10)
8. `workflows/theming-system/specification/theming-system/specification.md` — 3 notes
   - .workflows/theming-system/specification/theming-system/specification.md:1766 (§14.3) — the "fits at the reference mock's 86 columns with ~5px spare" claim does not survive conversion to cells: the pinned row is 80 + 1 spacer + 6 anchor = 87 content cells, so at 86 the `m multi` entry degrades to `· …`. The copy is spec-pinned and §14.4 covers the degrade, so no code changes; decide whether to correct the arithmetic (or restate the reference width) so the record matches what renders. (Report 8-14)
   - .workflows/theming-system/specification/theming-system/specification.md:~1596 (§13.2 "Retention rule, drawn now") — the spec's rule still reads as unconditional ("Everything that exists today as an image or tape is deleted — the committed reference PNGs and the VHS tapes that produce them"), with no `testdata/vhs/reference/` carve-out; CLAUDE.md and `testdata/vhs/README.md` now both document one. The carve-out was adopted in flight (reference-first: export and commit the design frame before implementing) and covers design exports rather than renders of the code, so the spec is not wrong about what it decided — but a future reader reconciling the three documents finds two-of-three agreement. Concrete change: add one sentence to §13.2 recording that design exports committed under `reference/` are kept, distinct from the captures the rule sweeps. Tagged idea rather than do-now because whether a signed-off spec is amended post-hoc (versus leaving the deviation recorded only in the review trail) is a decision, not a transcription. (Report 16-9)
   - .workflows/theming-system/specification/theming-system/specification.md:1816-1823 — §14A still states the band's order begins with the filter line and that the theme flashes "take precedence over it… a change to the band's precedence". The code proves the filter line renders on the section-header row and never contends for the band. Decide whether to issue a spec erratum recording that the required behaviour holds structurally (with the tier retained as a forward guard), so a future implementer reading §14A does not re-derive the wrong premise the way this task's code originally did. (Report 17-15)
9. `cmd/open.go` — 2 notes
   - the typed-nil wiring guard has no test observing production: `themeRoundTripConfig` (cmd/open_theme_commit_test.go:89-91) re-implements the same `if store != nil` check test-side, so a regression that dropped the production guard would not fail anything. Decide whether to extract the wiring into a small assertable helper (e.g. `themePersisterFor(*prefs.Store) tui.ThemePersister` returning a nil interface for a nil store, which also makes the guard unforgettable at the call site) or to accept the gap as the equally-unasserted `modePersister` precedent does. (Report 6-7)
   - the production `if prefsStore != nil` guard (which stops a typed-nil `*prefs.Store` being boxed into a non-nil persister) now has no assertion anywhere: the AST guard is gone and the behavioural carve-out sets `cfg.themePersister = nil` directly (`cmd/open_theme_commit_test.go:336`) rather than exercising openTUI's wiring, which `themeRoundTripConfig:89-91` merely re-states test-side. Same residual `report-6-7.md` recorded for the `modePersister` precedent. Closing it needs a design call — e.g. extract the wiring into a pure helper (`func themeSeamsFor(load prefsLoad) (tui.ModePersister, tui.ThemePersister)`) called by `openTUI` and assert it returns nil seams for a zero `prefsLoad`. (Report 11-4)
10. `internal/capture/theme_panel_fixture_test.go` — 2 notes
   - `backgroundSGR` is a fourth verbatim copy of the SGR-probe body (identical semantics to `bgSeq`), but it lives in the internal test package `capture`, not `capture_test`, so it cannot call `sgrParameterRun` and is outside this task's stated criterion. Consolidating it means deciding where the probe should live — e.g. hoisting it into `internal/themetest` (or a `logtest`-style shared test package) and having both test packages call it — which is a placement decision rather than a mechanical edit, and is the same cross-package class this task's item 7 deliberately scoped out. (Report 11-13)
   - 34 and internal/capture/panel_frame_test.go:14 — the `"theme-panel-"` literal and its registry-filter loop now exist once per test package: `panelFixturePrefix` + `panelFixtureNames()` in `package capture`, `panelFixtureNamePrefix` + `registeredPanelFixtureNames()` in `capture_test`. The two packages cannot share an unexported identifier, so closing this means deciding whether to export the constant from the `package capture` test file (e.g. `PanelFixturePrefix`, declared in a `_test.go` so nothing reaches the production build) and have the external test package read it through `capture.` — or to accept the split as the cost of the two-package test layout. Note the divergence is self-reporting today: the two derivations feed `TestPanelFixture_RegistryHoldsTheSpecifiedPanelSet`, so a prefix change that reached only one of them fails loudly rather than silently shrinking a guard. (Report 12-5)
11. `internal/capture/theme_swap_guard_test.go` — 2 notes
   - the full fixture set is built and rendered twice over in each of ~8 tests (`guardedFixtures` + `RenderSwapRender`/`swappedFrames` per test), so the unit lane pays roughly 8 passes over 27 fixtures for one set of frames. If this measures as a meaningful slice of `go test ./internal/capture`, memoise the `(A-frame, B-frame)` set behind a package-level `sync.OnceValue` and share it across the guard and the four `TestTokenCoverage_*` tests. Decide against it if the cost is negligible — shared state across tests is a real trade in a suite that forbids `t.Parallel()` precisely to keep tests independent, and the current shape rebuilds fixtures per test which is the safer default. (Report 4-3)
   - 466,482-484 — the `exclusivelyBackground: true` rows and `border`'s "found by no background run" check assert a *negative over the whole fixture set*: a future fixture that legitimately paints `bg.selection` as a foreground, or `border` as a background, fails the suite for a non-bug (the message does say "re-measure and re-record", so the cost is bounded). Decide whether the recorded-measurement tripwire is worth that maintenance edge or whether the exclusivity half should degrade to a `t.Log` of the measured split, keeping only the positive "found by this form" assertions. (Report 4-4)
12. `internal/sourceguardtest/foreachfunccall.go` — 2 notes
   - decide whether the iterator should yield the enclosing `*ast.FuncDecl` alongside (or instead of) its name. It would let gate-then-scan guards route through it — `cmd/theme_fatal_test.go:59-68` is the live example, whose `touchesTheme(fn)` gate needs the declaration and would be weakened by any call-only reformulation (it matches signature references). Trade-off: yielding the declaration widens the seam and invites call sites to re-walk it, which is the divergence this task removed. Not obviously worth it for one site. (Report 15-6)
   - decide whether to express the iterator as `iter.Seq2[string, *ast.CallExpr]` now that the module targets Go 1.26. Call sites would become `for funcName, call := range sourceguardtest.FuncCalls(file)` with `break` replacing `return false`, and the `stopped` bookkeeping would disappear. Counter-argument: the callback form matches `ast.Inspect`, which every surrounding guard is written in, and the repo uses no range-over-func anywhere yet — so this is a repo-wide idiom decision, not a local one. (Report 15-6)
13. `internal/theme/enumerate.go` — 2 notes
   - 145` — `statThemeDir` + `notADirectory` could collapse into classifying `os.ReadDir`'s own error (`os.IsNotExist(err)` → `(nil, nil)`, any other error → `unreadable(err)`), deleting the hand-built `fs.PathError`/`syscall.ENOTDIR` and both imports; on `go 1.26.0` `os.ReadDir` opens with `O_DIRECTORY`, so a regular file already yields the identical `open <path>: not a directory`, and a symlinked root is still followed. It departs from the plan's explicitly prescribed Stat-first shape and shifts the not-a-directory wording onto the toolchain, so it is a decision rather than a mechanical edit. (Report 1-7)
   - `Enumerate` now has exactly one production caller (`OpenEnumeration`) yet remains exported, so a future consumer can still hand-assemble an `Enumeration` and re-fork the invariant this task closed; only `cmd/doctor_theme.go` is guarded against that, by an AST call-count test. Worth deciding whether to unexport it (≈24 call sites across five external `theme_test` files would move to the internal test package) or to add a repo guard that `Enumeration{…}` literals appear only in `OpenEnumeration`, with `internal/capture`'s hermetic fixtures excepted. (Report 17-7)
14. `internal/tui/theme_panel_close_report_test.go` — 2 notes
   - `collectFlashTicks` blocks on `flashAutoClearDuration` (3s) per evaluated tick, six times in this file (~18s of unit-lane wall time, plus siblings elsewhere in the package). Decide whether to make the duration test-settable (const → package var) or otherwise avoid evaluating the timer command, and apply it package-wide rather than in this one file; the mechanism is a judgement call about touching production state for test speed, which is why this is not a mechanical fix. (Report 9-9)
   - 78` — `collectFlashTicks` evaluates real `tea.Tick` commands, so each `requireSingleFlashTick` call blocks for `flashAutoClearDuration` (3s). There are now six such call sites in the package (`TestCloseReport_RaisesTheFlash`, `…ForcedCloseCommitFlashWins` ×2, `…ForcedCloseGeometryFlashSelfClears` ×2, `…ProjectsFlashSlot`), ≈18s of pure sleep in a lane CLAUDE.md describes as "fast, hermetic"; this task added two of them. Evaluation is load-bearing for the generation assertion (a synthesized `flashTickMsg` proves nothing about what was *scheduled*), so the fix is a seam decision, not a rewrite: e.g. make `flashAutoClearDuration` a package-level `var` that tests shrink under `t.Cleanup` (safe here — `t.Parallel()` is banned repo-wide), or route `flashTickCmd` through a swappable tick constructor. Decide which before touching it. (Report 17-1)
15. `internal/tui/theme_state.go` — 2 notes
   - `themeState` mixes its three capture-only seeds (`initialCursor`, `initialConfirm`, `initialCommitFailed`) with production state, separated only by a bare `// Capture-only seeds:` line, while `Deps` got a nested `CaptureSeeds` type for exactly this separation. Mirroring it (a `capture themeCaptureSeeds` field, updating the three reads in `armThemePanel`/`seedThemePanelMessage` and the three options) would make the model side as visibly split as the wiring side; whether three fields justify the extra type is the open question. (Report 11-15)
   - the divergence's safety is still structural-by-convention: `adoptGateAnswer` would silently clobber a converted session's answer back to the pinned gate's dark fallback if it ever ran after `adoptRetainedReply`. Today it cannot (a pinned gate is permanently resolved, so `arm`/`resolve` no-op and `syncResolvedMode` is unreachable), and `TestCommitSlotLoad_ConversionIssuesNoQuery` pins that, but the task's stated outcome was for the coupling to stop being load-bearing. Consider making it structural — e.g. record that the answer was adopted from the reply and have `adoptGateAnswer` refuse to overwrite it — which needs a decision on whether the extra state is worth more than the current test pin. (Report 15-3)
16. `(unattributed)` — internal/tui — decide whether to add a `sourceguardtest`-driven guard rejecting `theme.Union{…}` composite literals that carry a `Count:` or `Rejected:` key in `package tui` test files. The task's stated outcome is that "a panel fixture *cannot* state a rejected count its own rows contradict", but nothing structurally stops the next fixture from writing the literal inline again; the repo already runs ~20 such guards, so the precedent exists — the call is whether a test-fixture invariant earns one. (Report 15-7)
17. `CLAUDE.md`:126 — the `skipTmuxCheck` set is quoted verbatim with nothing binding it to `cmd/root.go:23`; this task had to re-derive it by hand and the same silent-staleness hazard remains for the next verb. Decide whether to add a unit-lane guard that parses the documented list out of CLAUDE.md and diffs it against the map (the repo already has ~20 source guards via `sourceguardtest`, so the precedent exists), or to accept the prose as unguarded — it is a design call about how much of CLAUDE.md becomes machine-checked, not a mechanical edit. (Report 10-6)
18. `README.md`:116-244 — 244 (Commands section) — the section documents every other user-facing verb (`xctl list`, `xctl kill`, `xctl alias`, `xctl hook`, `xctl doctor`, `xctl version`, `portal uninstall`, `portal init`) but has no entry for `portal theme export`, which ships (cmd/theme.go:15, :78) and is the command docs/theming.md:297-307 tells users to run for the whole drop-in workflow. §12.5 scoped this task to four sites only, so adding a Commands entry is a genuine scope decision (widen §14's discoverability surface vs. keep the README a pointer) rather than an omission from this task. (Report 10-4)
19. `cmd/config.go`:103-110 — `prefsLoad.TranslationPending` is written by production and read only inside `loadPrefsStore` itself and by tests (`TranslatedSlug` likewise: consumed at :141/:149 and otherwise test-only). Decide whether both stay as the load's observable contract for the migration, or drop `TranslationPending` from the returned struct and let the tests assert through the recorded dispatch instead. (Report 6-6)
20. `cmd/doctor_theme.go`:110-123 — `persistedThemeNomination` is now a 1:1 shadow of `theme.InForceKey` plus a rendered label, produced by a loop that does nothing else. Consider dropping the struct and passing `theme.InForceKey` straight to `persistedThemeAdvisory`, labelling at the format call. Whether doctor keeps its own nomination vocabulary is a design call, so this is a decision rather than a mechanical edit. (Report 12-1)
21. `cmd/doctor_theme_enumeration_test.go`:14 — `cmd/doctor_theme_enumeration_test.go:14` — a later task (17-7) added a fourth missing-token fixture in `cmd` built ad hoc from `themetest.WithoutKey(themetest.Lines(), "bg.subtle")` instead of the shared `sourceMissingTokens`, so `cmd` now has two origins for invalid-theme fixtures (built-in-derived vs themetest-authored). Decide whether that test should route through the shared builder (it asserts no rejection reason and needs `themetest.Write`'s lines-shaped input, so the ad-hoc form is defensible) or whether the single-builder-per-class invariant this task established should be restated to cover it. (Report 11-11)
22. `cmd/prefs_translation_persist_test.go`:174 — `TestPersistTranslation_NeverEmitsCommitFailed`'s two cases are covered by `TestPersistTranslation_FailureIsSilentAndRetryable` (both make the write fail and both end in `assertThemeEvents(t, sink)` with zero wants, which already proves no `commit failed` record; the explicit `rec.Msg == "commit failed"` loop is subsumed by it). Decide whether to fold the two cases in as subtests of the failure test, or keep the separate function as a named witness for §8.9's single-siting invariant. (Report 6-6)
23. `cmd/theme.go`:45 — `cmd/theme.go:45` — an argument that is empty, or that strips to empty (`portal theme export ""`, `portal theme export -- $'\n'`), renders `theme is not valid: bad name` with a doubled space and no visible subject. §14A pins the frame but says nothing about an empty `<slug>`, so choosing a rendering (quote the value, or a distinct "no theme name given" line) is a copy decision, not a mechanical edit. Add the chosen case to `TestThemeExport_BadNameFrame` once decided. (Report 2-10)
24. `cmd/theme_persister_test.go` — AC2 ("exactly one domain→persistence conversion") is pinned structurally on the `internal/tui` side only; consider whether the `cmd` package warrants a matching AST guard asserting `prefs.Slot*` / `prefs.ThemeSlot` is named only in `theme_persister.go`, or whether the repo's existing guard budget makes that redundant given `SaveThemeSlot` has a single caller. (Report 12-9)
25. `docs/theming.md`:366-367 — 367` — the `prefs.json` path chain names only `$XDG_CONFIG_HOME/portal/prefs.json` and `~/.config/portal/prefs.json`, omitting the `PORTAL_PREFS_FILE` rung that `configFilePath` actually honours (`cmd/config.go:187-188`), while the themes-dir chain three sections earlier (:249-253) is spelled out in full including its env var. Decide whether this doc should advertise `PORTAL_PREFS_FILE` — §5.5 fixed `PORTAL_THEMES_DIR`'s name in the spec *precisely so this doc could print it*, so the omission may be deliberate scoping rather than an oversight. (Report 10-3)
26. `internal/capture/panel_frame_test.go`:14 — internal/capture/panel_frame_test.go:14 and internal/capture/theme_panel_fixture_test.go:34 — the `"theme-panel-"` literal exists twice, as `panelFixtureNamePrefix` (`package capture_test`) and `panelFixturePrefix` (`package capture`). They are in different Go packages sharing a directory, so the unexported constant cannot cross, and unifying them means deciding between exporting a test-only prefix from production `capture` (pollutes the production surface for a test concern) and accepting the split (a `theme-panel-` rename would need both edited). Both constants are single-sourced *within* their own package, so the AC's intent holds; the decision is whether the cross-package duplication is worth a production export. Recommend accepting the split and, if anything, cross-referencing the two in a comment. (Report 13-1)
27. `internal/capture/swatch.go`:113 — `internal/capture/swatch.go:113/121/128/135` — the band labels are string literals, justified at `:100-101` on the grounds that a hand-built `Theme` carries values without names. A *parsed* theme does carry them (`internal/theme/validate.go:55` sets `Token{Name: ref.Name, …}`), so `tintLabel` could render `tok.Name` and be immune to a token rename, at the cost of populating `Name` in `swatchTestPalette` (`swatch_test.go:13`). Only a partial win — the pairing captions at `:118`/`:125` stay literal either way — so this is a judgement call about whether half-covering the drift is worth losing the "the label is exactly what a user types" property the comment claims. (Report 2-4)
28. `internal/log/discard_guard_test.go`:24 — internal/log/discard_guard_test.go:24 and internal/log/migration_guard_test.go:27 — these two guards still hand-roll their own `filepath.WalkDir` over the repo root (skipping only `.git`, `vendor`, `node_modules`) while importing `portalbintest` for `ProjectRoot`, so they are the last repo-wide guards outside `sourceguardtest.GoSourceFiles`. Routing them through it would finish the single-source story, but it is not a mechanical swap: `GoSourceFiles` excludes *all* dot-directories and returns test sources too, so the log guards' own `_test.go` filter and exclusion set would have to be re-decided — a scope call this task explicitly forbade ("do not change which directories any guard walks"), hence an idea for a follow-up rather than an edit here. (Report 16-5)
29. `internal/prefs/store.go`:212 — internal/prefs/store.go:212,221,237,253,267 vs the four abort tests — all five current `mutate` callers are enrolled in the table, but nothing structurally enrolls a *sixth*: a new saver added to `store.go` can ship with no abort test and no failing test to say so, leaving the "no saver can silently fall out of coverage" outcome true only for today's set. The repo already uses source guards for exactly this class of drift (`colour_literal_guard_test.go`, `keymap_dispatch_guard_test.go`, and `sourceguardtest`'s shared AST primitives, which `PackageGoFiles` + `ForEachFuncCall` would serve directly). Whether the enumeration is worth its own maintenance cost, and whether it keys on `mutate` call sites or on an exported-`Save*` naming rule, are open design calls — hence an idea rather than a concrete edit. (Report 13-2)
30. `internal/sourceguardtest/packagegofiles.go`:15 — the positional `includeTests bool` reads opaquely at the ten call sites (`PackageGoFiles(".", false)` gives the reader nothing without opening the signature), and exactly one site passes `true`. Decide between exported named constants (e.g. `sourceguardtest.WithTests` / `ProductionOnly`) and a second `PackageGoFilesWithTests` entry point; the task mandated an explicit parameter, so this is a shape question rather than a defect. (Report 14-4)
31. `internal/theme/badge_test.go`:221-234 — the purity subtest pins "references no `Theme`" via the AST selector scan but leaves the acceptance criterion's "performs no I/O" to inspection. A structural pin is possible (badge.go currently has an empty import block), but the shape needs deciding: asserting zero imports is brittle against a legitimate future `strings`/`cmp` import, while a deny-list of I/O packages needs its own maintenance rule. Worth deciding rather than adding blind. (Report 8-3)
32. `internal/theme/builtins_nord_test.go`:16-36 — 36` — `wantNordTokens` pins all 19 shipped hexes in Go, duplicating the `.theme` file that is their source of truth; `TestEveryEmbeddedThemeIsValid` (`embedded_test.go:23`) already proves 19 populated upper-case-canonical tokens for every built-in, and the derivation records already pin the 7 judged values. Decide whether to reduce the pin to the judged values plus a name-order assertion (and, if so, apply the same call to `wantTokyoNightTokens`/`wantTokyoNightDayTokens`) or to keep the full pin as a deliberate edit-detector for a shipped palette — §13.6 deleted `TestMVDarkVariantsPinned` on the former reasoning. (Report 2-7)
33. `internal/theme/builtins_tokyo_night_day_test.go` — consider making the recorded figures self-checking: parse each of the seven's comment for the original/re-derived hexes and the stated ΔE and chroma %, recompute both with go-colorful (already a dependency, and `contrast_test.go` already does colour math in this package) and assert agreement to the recorded precision. Upside: the exported record could never rot into a false claim, and the metric statement in the header would be executable rather than documentary. Downside: it pins a comment-text grammar in test code and adds a parser for prose; the shipped values are already pinned by `wantTokyoNightDayTokens` and `sevenCheckedValues[].shipped`, so a value change fails loudly today and the author is pointed at the comment. Worth a decision, not obviously worth doing. (Report 2-5)
34. `internal/theme/contrast_test.go` — the suite auto-enumerates *themes* but not *tokens*: a 20th token added to `Theme.fields()` would carry no floor and nothing would fail. Consider a coverage assertion that every name in `theme.TokenNames()` is either exercised by at least one rule or listed in an explicit no-floor exemption set (`border`, `canvas`, and the two `text.on-*` pairings which are only measured on their tints). Needs a design decision on how rules declare which token they cover, which is why this is not a mechanical edit. (Report 2-3)
35. `internal/theme/docs_guard_test.go`:330-334 — no guard ties the doc's built-in table (`docs/theming.md:330-334`) to `theme.BuiltinSlugs()`, yet §5.4 makes this table one of only two discovery routes for the reserved set, and "adding a theme is adding a file" means a fourth built-in would silently rot it. A cheap addition would parse the `| Slug |`-headed table and compare against `BuiltinSlugs()`. §12.4 deliberately scopes the guard to the vocabulary half and declares the discovery half hand-maintained, so this is a decision to widen that scope, not a defect. (Report 10-3)
36. `internal/theme/lex.go`:85–87 — `trimLine`'s `strings.TrimSuffix(raw, "\r")` is redundant: the `strings.TrimSpace` that wraps it already removes a trailing `\r` (and any run of them), so removing the `TrimSuffix` is behaviour-identical for every input. Decide whether the explicit CRLF step is worth keeping as intent (the comment above it already carries that intent) or should be dropped as dead work. (Report 1-2)
37. `internal/theme/light_pins_test.go`:51 — `TestThemeAppearanceTableHasNoStaleEntries` is fully subsumed by the `slices.Equal(enrolled, embedded)` assertion at :46, which already fails on a stale row. It survives only for a sharper message. The plan named both tests explicitly, so this is deliberate; worth deciding whether the extra test earns its place or whether the equality assertion's failure output should absorb the diagnostic instead. (Report 2-6)
38. `internal/theme/slot_default_test.go`:40-64 — two of the table's three assertions duplicate existing coverage: the unset-slot `Slug` answer is already pinned by `setting_test.go:329-340`, and the light/dark fallback by `resolution_test.go:82-97`, which additionally pins `Reason` and `Theme`. Decide whether the cross-path table's value (stating the one-rule invariant in a single place) justifies the triplication, or whether the older single-path rows should be trimmed toward it — noting the `resolution_test.go` rows assert strictly more and should not simply be deleted. (Report 17-3)
39. `internal/theme/validate.go`:55 — the offending value is echoed verbatim into `Rejection.Detail`, which doctor prints to a terminal; a `.theme` file value containing an ANSI escape or control byte therefore reaches the terminal unstripped. §9.4/§12.1 control-strip slugs from `prefs.json` and CLI arguments for exactly this reason but say nothing about token values. Decide whether to extend the same strip to the echoed value (and, if so, whether it belongs here or in the lexer alongside `wellFormedKey`). (Report 1-3)
40. `internal/themetest/synthetic.go`:30 — internal/themetest/synthetic.go:30,77 — the red-floor and equal-reds fatals are structurally untestable while the helpers take a concrete `*testing.T`. Decide whether to widen `themetest`'s signatures to `testing.TB` and add a recorder fake so acceptance criterion 3 has permanent coverage; the trade is the `*testing.T`-first parameter that structurally keeps these helpers out of production code (the convention CLAUDE.md records for `portaltest`). (Report 14-1)
41. `internal/themetest/theme_file.go`:104 — decide whether the monochrome fixture deserves the write-level convenience its canvas sibling has. `themetest.Write(t, dir, base, themetest.MonochromeLines(v))` is spelled out at ~60 call sites (all of `internal/tui/theme_panel_*_test.go`, `internal/tui/builtin_themes_test.go`, `internal/tui/theme_testing_test.go`), whereas the canvas form got `WriteWithCanvas` for far fewer. A `WriteMonochrome(t, dir, base, value)` would restore the symmetry, at the cost of a ~60-site sweep for no behaviour change. (Report 17-5)
42. `internal/tui/build.go`:130-137 — two gating idioms still coexist in `Build`: eight seams use `if deps.X != nil`, while `Detector`, `Resolve`, `SessionExists`, `AckChannel`, `SpawnExe`, `SpawnGetenv` and `SpawnLogger` are applied unconditionally through nil-tolerant options. They are behaviourally identical (every setter is a plain assignment to a nil-defaulted field), but AC 1's "all optional `Deps` seams use the same plain nil gate" is not literally true, so the ambiguity the task set out to remove is reduced rather than eliminated. Either add the seven guards or drop the eight — the direction is a judgment call about whether seven no-op guards are worth the churn. (Report 11-15)
43. `internal/tui/builtin_theme_table_test.go`:60 — the guard matches a single source line, so a row split across lines (`{\n\tname: "dark",\n\tth: testDarkTheme(t),\n}`) or written value-first reintroduces the table undetected. Decide whether to swap the regex for an AST walk of composite literals (the package already parses Go source in `retired_token_guard_test.go`), or to accept line-shape matching as sufficient for a discipline aid. (Report 15-5)
44. `internal/tui/filter_footer.go`:83-96 — `filterFooterRow` renders its cluster unfitted, so under the inverted assembler the contextual filter footers go from full cluster to anchor-alone in one step (for `filterAppliedFooterEntries`, every width below roughly 48 cells); decide whether to give it a `fitFilterCluster`-style fitter so they degrade one entry at a time like the standard footer, or accept the coarser step as the documented trade. (Report 8-14)
45. `internal/tui/footer.go`:22-28 — `renderSessionsFooter` and `renderProjectsFooter` are byte-identical one-line delegations with identical signatures, so nothing prevents passing the Projects entries to the Sessions renderer; decide whether to collapse them into `renderCondensedFooter` at the call sites (churns ~10 test files) or keep the page-named seams for call-site readability. (Report 8-14)
46. `internal/tui/modal_footer.go`:32 — `th` and `colourless` are used for nothing but `headerCanvasBg(th, colourless)`; passing a prepared canvas `lipgloss.Style` instead would cut the signature from 8 parameters to 7 and make the builder theme-agnostic, matching how the key and label styles are already resolved at the call site. Decide whether the builder should keep ownership of canvas painting (it currently paints pad and gap) before making the change. (Report 11-8)
47. `internal/tui/theme_flash_precedence_test.go`:100-188 — 188` — the §14A conformance table raises all six theme signals and asserts copy + `flashOriginTheme`, but discards the returned command, which is exactly why this defect survived to a remediation cycle. Extending that table to also pin "every theme signal schedules an auto-clear tick" would make the six-signal transience structural rather than per-site. It needs a design call first because the raise funcs currently return only the model, and asserting the tick by evaluation compounds the sleep cost in the note above. (Report 17-1)
48. `internal/tui/theme_panel_arrow_test.go`:532-553 — a third copy of the "the themes directory holds %d entries … want only the seeded drop-in" assertion (with its own `themetest.Write` staging) survives on the arrow-preview path, i.e. the same duplication class this task retired for open/close. Decide whether the arrow path should route through `requireNoPrefsOrThemesWrite` (it would gain the present-prefs byte-comparison it currently lacks, at the cost of renaming its "the themes directory is untouched" subtest to the helper's two fixed names) or stay separate because its fixture guard and preview-specific control do not fit the helper's `act` contract. (Report 16-2)
49. `internal/tui/theme_panel_close_test.go`:505-511 — internal/sourceguardtest — decide whether a `ForEachFuncDecl`-style sibling should exist for the non-call half. Roughly six guards still re-author `Decls → FuncDecl → nil-body decision → Inspect` for other node types, and they still disagree on the nil-body handling exactly as the call walks used to: `internal/tui/theme_panel_close_test.go:505-511` (AssignStmt, guards nil body), `cmd/doctor_theme_union_test.go:329-335` (MapType, does not), `internal/theme/slot_default_test.go:71-76` (Ident, guards), `internal/theme/loader_construction_guard_test.go:48-52` (CompositeLit, inspects `decl` not `fn.Body`), `internal/capture/theme_swap_guard_test.go` (CaseClause). Explicitly outside this task's scope (call walks only) — raise as its own item if the divergence is judged worth closing. (Report 15-6)
50. `internal/tui/theme_panel_commit_slot_test.go`:416 — `TestPanelSlotCommit_TypedSlotOnly` would not catch a reintroduced TUI-local light/dark enum: such a type's `member()` bridge names `MemberDark`/`MemberLight` (so `members` still matches) and branches rather than converting (so `conversions` stays empty). The repo already keeps deleted-helper guards for exactly this shape (CLAUDE.md records one keeping `canvasHexFor` gone), and this task's own check for it was a one-shot grep. Decide whether to extend this guard — e.g. assert `internal/tui` declares no unexported two-valued type whose method returns `theme.Member` — or accept the grep as sufficient. (Report 14-10)
51. `internal/tui/theme_row.go`:162 — decide whether the panel's UNSELECTED labels should also carry `nameBase`. The Sessions delegate bolds its name on every row (`session_item.go:251,256`; the goldens at `row_style_helpers_test.go:136` show the unselected `bravo` bold), so the panel's non-cursor rows still read a step lighter than Sessions' non-cursor rows even though §9.1's stated reason for the treatment is that "the panel's list reads as the same kind of list as Sessions". This task explicitly scoped itself to the cursor row ("Unselected rows are unchanged"), so this is not a deviation — it is the residual parity question that scope leaves open, and answering it needs a design call, not an edit. (Report 13-12)
52. `internal/tui/theme_row_test.go`:159-161 — the `len(reasons) != 7` fatal guards the test's own literal table, so an eighth `theme.Reason` constant would silently go unrendered-and-unasserted here (and in `internal/theme/reason_test.go:12`, which restates the same list). Consider exporting the vocabulary from `internal/theme` (e.g. an ordered `AllReasons()` slice, the way `theme.TokenNames()` already backs the token table) and driving both tables off it; this is a new exported API on a closed vocabulary, so it needs a decision rather than a mechanical edit. (Report 8-4)
53. `internal/tui/theme_testing_test.go`:87 — the helper hardcodes the seeded drop-in as `sunset.theme` while accepting `keys` as a parameter, so the two are only correlated by convention. A third caller passing keys that name a different slug would resolve through the rejection fallback to a built-in and still satisfy all five assertions, i.e. pass vacuously. Decide between deriving the seeded filename from `keys` (needs a rule for which slot to read when both `Light` and `Dark` are set) and adding a fixture guard after the open that the panel's rows carry the seeded slug. (Report 14-5)
54. `model_test.go` — internal/themetest — decide whether to host a single exported `SGRParams(t *testing.T, style lipgloss.Style) string` there, so `package tui`, `package tui_test`, `package capture` and `package capture_test` all share one derivation: it would remove the last inline copy (`model_test.go`) and the capture two-copy split without exporting anything from production code (`themetest` is test-only). The task deliberately scoped to per-package single-sourcing, so this is a decision to widen `themetest`'s remit (currently theme-file/palette authoring, no lipgloss dependency), not a defect in the delivered work. (Report 12-4)
55. `specification.md`:436 — `…/specification.md:436,455-464` and `:511` — pre-existing staleness this task did not (and was not asked to) cover: §10.2/§10.4 still describe an **11-step** bootstrap and map "9–11 marker/FIFO/stale cleanup" (the orchestrator has been ten steps since CleanStale moved onto the `_portal-saver` daemon), and §12.1's Preview keymap omits the `Home/End` top/bottom binding `previewKeymap()` ships (`internal/tui/keymap.go:97`). Neither was falsified by theming-system, so leaving them was within this task's scope; the decision is whether a further corrigendum against the same file is worth opening. (Report 10-7)
56. `workflows/portal-observability-layer/specification/portal-observability-layer/specification.md`:183 — `.workflows/portal-observability-layer/specification/portal-observability-layer/specification.md:183` — the `clean` owns-row reads "`portal clean` command path", naming a command `cli-verb-surface-redesign` retired (the component itself is still live, bound at `cmd/bootstrap/bootstrap.go:16` and `internal/state/fifo_sweep.go:13`, so the count of 18 is unaffected). The task deliberately scoped this out ("surface it, do not sweep it"), so leaving it is compliance, not drift. Decide whether a follow-up correction cycle re-invokes the protocol on this file to fix this row together with the `11-step` note above, rather than paying the gate twice. (Report 10-9)

### Bugs

*9 notes across 8 targets. None blocking.*

57. `internal/theme/resolve.go` — 2 notes
   - 63` — on a case-insensitive filesystem (macOS APFS default) the by-name path can accept a file the panel rejects: `loadFromThemesDir` composes `<themesDir>/<slug>.theme` and `LoadFile` derives the slug from that *composed* base, so a `Mine.theme` on disk is opened and accepted as slug `mine`, while `Enumerate` lists the same file as `bad name`. The no-shadowing property itself is unaffected (a built-in slug never reaches the directory — `resolve.go:32-34` consults the embedded set first), and spec §5.6 line 391 asserts the by-name path "looks for `<slug>.theme` and nothing else", which is what the code implements — so this is a spec-level edge rather than a coding mistake, and it is outside this task's criteria. Concrete change: after a successful `LoadFile` in `loadFromThemesDir`, confirm the on-disk entry name equals `slug+FileExtension` (one `os.ReadDir`/`filepath.Glob` name comparison) and return `notFound()` when it does not, so panel and launch cannot disagree about the same file. (Report 2-2)
   - a *directory* named `<slug>.theme` inside the themes dir makes `ResolveByName` return `unreadable` (ReadFile gives EISDIR; Lstat succeeds so nothing narrows), while `Enumerate` skips directories (`enumerate.go:78 resolvesToDirectory`) and the enumeration-backed path answers `not found` (`resolution.go:138` → `union.go:209`). One on-disk state, two different reasons across construction and the panel/doctor. Narrow the same way the other cases are narrowed: in `narrowReadFailure`, when the composed path stats as a directory, return `notFound()` so by-name agrees with enumeration's skip; add the case to `TestResolveByName_AbsentFileIsNotFound`. (Report 5-3)
58. `cmd/capturetool/main.go`:63-66 — 66` — the swatch branch never restores the terminal background. `tui.RestoreTerminalBackground` runs only when the final model is a `tui.Model`, but `swatchModel.View()` sets `BackgroundColor` (`internal/capture/swatch.go:58`), so quitting `--fixture contrast-validation` leaves the theme's canvas set on any terminal that ignores Bubble Tea's OSC 111 reset — the exact failure `internal/tui/restore.go` exists to prevent. This predates the task, but `--theme` widens the blast radius from two known MV canvases to any drop-in's canvas, and the swatch is precisely the surface a human runs in their real terminal at a visual gate. Concrete change: have `swatchModel` capture the original background (`tea.RequestBackgroundColor` in `Init`) and set it back on exit, mirroring `internal/tui/restore.go` including its canvas-echo guard. (Report 2-4)
59. `cmd/config.go`:123-124 — `LoadThemeKeys()` and `LoadMigrationState()` each perform their own `os.ReadFile` (`internal/prefs/store.go:191,201`), so §10.5's "load-time snapshot" is in fact two snapshots. Between the two reads another instance's `SaveTranslation` can land, yielding `keys={}` with `Migrated=true`; the marker gate then returns zero keys and that launch renders the shipped adaptive pair instead of the pin just translated — §10.1's silent flip, narrow (two simultaneous cold launches) and self-correcting on the next launch, but reachable. Fix: add one combined prefs read (e.g. `LoadThemeState() (ThemeKeys, MigrationState, error)` off a single `readFile`) and call it once in `loadPrefsStore`; it also removes one of the three prefs file reads the launch path currently performs. (Report 6-5)
60. `cmd/theme.go`:15 — `themeExportCmd` sets neither `SilenceErrors` nor `SilenceUsage`, so a refusal reaches the user as cobra's `Error: no theme named nope`, then the **full usage + flags block**, then `main.classify` (main.go:71) printing the same frame a second time, bare. Both the spec's single-line refusal frame (§14A) and `.claude/skills/golang-cli`'s "Printing usage on every error" mistake row are missed, and `doctorCmd`, `uninstallCmd` and `stateCommitNowCmd` already carry the pair. Add `SilenceErrors: true, SilenceUsage: true` to `themeExportCmd`, and extend `requireExportRefusal` (cmd/theme_test.go:580) to assert `run.stderr` is empty (main, not cobra, owns the printing) so the shape is pinned. The frame *copy* is task 2-10's; this is the surrounding noise, so the fix belongs with whichever of the two is touched first. (Report 2-9)
61. `internal/tui/model.go`:917-933 — `Styles.ArabicPagination` keeps `bubbles/list`'s hardcoded `#9B9B9B`/`#5C5C5C` (`bubbles/v2@v2.1.0/list/style.go:83`), and `paginationView` escalates to it whenever the rendered dot row is wider than the list (`list/list.go:1188-1192`) — a colour belonging to no theme reaching a real frame, on a narrow terminal with many pages. It is the same class as the `Styles.NoItems` offender this task's review caught, it is invisible to task 4-3's swap-and-diff guard (no fixture paginates that far and the grey is neither theme), and since the residue block was stripped it is no longer recorded either. Fix: set it alongside the dots in `applyListCanvasMode` — `l.Styles.ArabicPagination = lipgloss.NewStyle().Foreground(th.TextFaint.Color()).Background(th.Canvas.Color())` on the canvas arm and a bare `lipgloss.NewStyle()` on the colourless arm — with a matching assertion in the `probedLists` loop. (Report 4-1)
62. `internal/tui/model_test.go`:26-28 — `editFieldFocused` returns `false` when the probe carries no SGR sequence, so a colourless render (or a profile downgrade) reports "field not focused" instead of "probe broken", sending the reader after the wrong defect. Replace the `return false` with `t.Fatalf("could not derive an SGR parameter run from %q", probe)`, matching the shared helper. (Report 12-4)
63. `internal/tui/retired_token_guard_test.go`:48-51 — the `help_modal_frame_test.go` / `border.separator` exemption no longer covers a deliberate reference. The comment it was written for (line 41 pre-strip, recording the two consolidated frame roles) was removed by `e3fa1503`; the sole surviving occurrence is `help_modal_frame_test.go:20`, whose message reads "must be border.separator SGR core %q (not white)" on an assertion that reads `th.Border` — stale prose, not an absence guard. Once the guard covers strings this entry would bless it. Rewrite that message to name `border` and drop the exemption entry. (Report 14-2)
64. `workflows/portal-observability-layer/specification/portal-observability-layer/specification.md`:173 — `.workflows/portal-observability-layer/specification/portal-observability-layer/specification.md:173` — the `bootstrap` owns-row reads "The 11-step bootstrap orchestrator"; as-built the orchestrator is ten steps (CLAUDE.md:126; former step 11 `CleanStale` was re-homed onto the `_portal-saver` daemon). Line 869 repeats "the 11-step bootstrap orchestrator". Change both to "ten-step". This stale number sits in the very table this amendment edited and, unlike the `clean` row, was not surfaced anywhere — so a file that now carries a corrigendum block asserting its counts are reconciled still serves one wrong count to the knowledge base. (Report 10-9)
