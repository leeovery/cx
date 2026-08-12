TASK: theming-system-14-2 — Carry The Token Rename Into The Render Layer's Own Comments

ACCEPTANCE CRITERIA:
- No retired token name appears in `internal/tui` or `internal/capture` except the exempted absence guards.
- Every rewritten comment names the token the adjacent code reads.
- The new guard fails when a retired name is reintroduced anywhere in the covered packages, and its exemption set is explicit and justified in-source.
- `internal/theme`, `docs/theming.md` and `testdata/vhs/` are untouched.
- `go test ./...` passes; `golangci-lint run` is clean.

STATUS: issues_found

SPEC CONTEXT:
The §2.4 token rename retired fifteen names in favour of the 19 named semantic roles that `internal/theme` now publishes. CLAUDE.md states the vocabulary is "a public contract every theme file is written against, so a rename breaks every drop-in", and the drop-in workflow in `docs/theming.md` is authored against it. The originating analysis finding (`.workflows/theming-system/implementation/theming-system/analysis-standards-c4.md:3-7`) was scoped to doc comments (~275 references across 25 files) and recommended extending the `colour_literal_guard_test.go` machinery. The plan task deliberately widened that scope: it counts "328 references … in production comments and a further ~446 in test comments and failure strings", Do step 1 names "(production and test, comments and `t.Errorf`/`t.Fatalf` message strings)", Do step 6 names the failure strings as the one sanctioned non-comment change, and the Tests section requires "A repository grep for each retired name over `internal/tui` and `internal/capture` returns only the exempted sites."

IMPLEMENTATION:
- Status: Partial
- Location: commit `0d318752` (74 code files); guard at `internal/tui/retired_token_guard_test.go:1-97`
- Notes:
  - The comment sweep is complete and correct. Zero retired names survive in any comment under `internal/tui` or `internal/capture` (verified by grepping comment-prefixed lines across both packages — no matches). `internal/capture` is clean in *every* line, comments and strings alike (0 occurrences of any retired name).
  - The rewrites were verified against the adjacent code, not applied blind, per Do step 2. Spot-checked `section_header.go` (`accent.mode`↔`th.AccentMode`, `state.positive`↔`th.StatePositive`, `text.muted`↔`th.TextMuted`, `state.destructive`↔`th.StateDestructive`), `notice_band.go` (band role → `barToken`/`tintToken` mapping at lines 34-53 matches the rewritten role table exactly, including `bandSuccess` → `state.positive` on the shared `bg.attention` tint), `edit_modal.go` (`inputBoxIdle/Focused/Editing` → `border`/`accent.primary`/`accent.attention` matches `inputBoxBorderToken`), `header.go`, `build.go`, `capture/fixtures.go`.
  - Do step 3 (deliberate references) was honoured at authoring time: `active_theme_test.go` and `help_modal_frame_test.go` kept their retired names with the prose rewritten to read as statements about a removed name (`git show 0d318752:internal/tui/active_theme_test.go:64,80`).
  - Do step 6 was honoured exactly: diffing the commit's non-comment changed lines yields only three trailing line comments in `edit_modal.go` plus the new guard file. No behaviour, assertion or test-name change.
  - Do step 4 (exclusions) honoured: the commit touches nothing under `internal/theme/`, `docs/` or `testdata/`.
  - **The gap**: `t.Errorf`/`t.Fatalf` message strings were not swept. 150 retired-name occurrences remain across 37 `internal/tui` test files (168 total minus the 18 in the guard's own declared table). All are in string literals — failure messages and role-label table data. Only ~2 of them are the deliberate absence-guard sites the task sanctioned. No later plan task re-scoped this: phases 15-17 contain no task touching the token rename, so this is genuine incompleteness rather than a superseded mechanism.

TESTS:
- Status: Under-tested (relative to the acceptance criteria)
- Coverage: `TestNoRetiredTokenNameInComments` enumerates both package directories via `sourceguardtest.PackageGoFiles(dir, true)` (tests included; errors on an empty enumeration so it cannot pass by having stopped looking), parses each file with `parser.ParseComments|SkipObjectResolution`, and reports file:line, the dead name and the current role per hit. A subtest per file gives precise failure attribution. Reading the logic, it would fail on a reintroduced retired name in a comment.
- Notes:
  - Scope is comments only (`for _, group := range file.Comments`) — the guard is structurally blind to the string literals that hold the 150 surviving occurrences, so AC3's "anywhere in the covered packages" holds only for prose.
  - Not over-tested: one guard, one declared table, no redundant assertions, no mocking.
  - The guard is correctly homed in `package tui_test` alongside `colour_literal_guard_test.go` as Do step 5 asked, with no symbol collisions in that package.
  - The task's manual verification steps (temporarily reintroduce a name; confirm the exempted sites do not fire) cannot be confirmed from the tree; the logic reads correctly for both.

CODE QUALITY:
- Project conventions: Followed — mirrors the established source-guard shape, routes file enumeration through the shared `sourceguardtest` primitive (per CLAUDE.md's "single canonical declaration" rule for guard scaffolding), stdlib-only, untagged so it runs in the unit lane.
- SOLID principles: Good — the retired list is one declared table (single source), the exemption predicate is isolated, the covered directories are one declared slice.
- Complexity: Low.
- Modern idioms: Yes — `parser.SkipObjectResolution`, `filepath.Join`, table-driven exemptions.
- Readability: Good. The guard's own doc comments were later removed by the out-of-plan `e3fa1503 chore(comments): strip internal/tui to the code-quality standard` sweep; the `reason` field still carries the per-exemption justification AC3 requires.
- Comment accuracy: The comments this task rewrote are accurate against the code they sit on. The contradiction is comment-vs-string within the same files (see blocking issue).
- Issues: None in the guard's own construction beyond the scope narrowing and the now-vacuous exemption entries noted below.

BLOCKING ISSUES:
- Acceptance criterion 1 is unmet: 150 retired token names survive in `internal/tui`, all in test failure-message strings and role-label table data, across 37 files. Do step 1 explicitly placed "`t.Errorf`/`t.Fatalf` message strings" in scope, Do step 6 named them the one sanctioned non-comment change, and the Tests section required a repo grep to return only the exempted sites. The implementation swept comments only and named the guard `TestNoRetiredTokenNameInComments`, narrowing delivery to the task's title rather than its criteria. The result is worse than uniform staleness — it is a file-local contradiction:
  * `internal/tui/footer_test.go:58-65` — the rewritten comments read `accent.key`, `text.muted`, `accent.primary`, while the three `t.Errorf` strings immediately beneath still print `accent.blue key-glyph`, `text.detail label`, `accent.violet ?`.
  * `internal/tui/header_test.go:34-35` — the role labels `"accent.violet caret"` and `"text.detail subtitle"` are table *data* feeding the failure message, two lines under a comment rewritten to `accent.primary` / `text.muted`; line 42's message still says `border.separator` for an assertion reading `th.Border`.
  * Highest-density files: `edit_modal_test.go` (16), `loading_view_test.go` (8), `rename_modal_test.go` (7), `section_header_test.go` / `session_row_anatomy_test.go` / `sessions_grouped_reskin_test.go` / `filtering_reskin_test.go` (6 each).
  A maintainer debugging a red test reads the dead vocabulary directly off the failure output — the exact rot the task exists to stop — and AC3's guard does not catch a reintroduction there.

NON-BLOCKING NOTES:
- [bug] internal/tui/retired_token_guard_test.go:48-51 — the `help_modal_frame_test.go` / `border.separator` exemption no longer covers a deliberate reference. The comment it was written for (line 41 pre-strip, recording the two consolidated frame roles) was removed by `e3fa1503`; the sole surviving occurrence is `help_modal_frame_test.go:20`, whose message reads "must be border.separator SGR core %q (not white)" on an assertion that reads `th.Border` — stale prose, not an absence guard. Once the guard covers strings this entry would bless it. Rewrite that message to name `border` and drop the exemption entry.
- [quickfix] internal/tui/retired_token_guard_test.go:37-57 — all four exemption entries are currently vacuous: the guard scans only comments, and none of the three exempted files hold a retired name in a comment any more (`e3fa1503` stripped them). Re-point the table at the string occurrences that genuinely are deliberate (`active_theme_test.go:49,52`) as part of extending the guard, or delete the entries that cover nothing.
- [quickfix] internal/tui/retired_token_guard_test.go:38-41 — the guard's self-exemption uses an empty `name`, which blanket-exempts the whole file for every retired name. Since `retiredTokenNames` lives in a declaration and the guard reads only comments, the entry protects nothing and leaves a hole for any comment later added to that file. Scope it to a specific name or remove it.
