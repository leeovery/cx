TASK: theming-system-10-1 — Docs/theming.md's Vocabulary Half and Its Token-Table Guard

ACCEPTANCE CRITERIA:
- [x] `docs/theming.md` exists and its token table carries exactly 19 rows whose names equal `Theme.All()`'s set.
- [x] The guard fails (not skips) when the doc file is absent.
- [x] The guard fails when the table parse yields zero rows, with a message distinguishing that from a name mismatch.
- [x] The guard fails when a token exists in `Theme.All()` but not in the table, naming the token.
- [x] The guard fails when the table names a token absent from `Theme.All()`, naming the row.
- [x] The guard parses the doc's example theme through the production loader and fails if it is not valid under §6.1.
- [x] Reordering the table's rows does not fail the guard.
- [x] The doc documents the ramp bright → faint in weight order in prose.
- [x] The doc states `text.faint` is decorative-only and never carries content a user must read.
- [x] Each `text.on-*` row names the tint it pairs against.
- [x] `border.footer` appears nowhere in `docs/theming.md`; `border` is documented as one role covering the title rule, footer rule, modal frames and chips.
- [x] The guard runs in the unit lane with no `t.Parallel()`.

STATUS: complete

SPEC CONTEXT:
§12.4 makes `docs/theming.md` the source of truth for the public 19-token contract and the sole record of the text ramp's weight ordering (§2.7 makes file ordering explicitly not a contract; §2.6 records that the ramp's `text.tertiary` → `text.muted` join rests on convention). §13.5 requires the doc to carry a guard — a test parsing the doc's token table against `Theme.All()`, with the copy-pasteable example theme covered by the same guard so it is not a fourth unguarded copy of the vocabulary (§15.3). §2.5 is the role table to author from, grouped text ramp / accents and states / surfaces, with `text.faint` decorative-only and both `text.on-*` roles named for the tint they pair against.

IMPLEMENTATION:
- Status: Implemented (with a deliberate later strengthening by task 11-12)
- Location:
  - `docs/theming.md:1-124` — the vocabulary half authored by this task (opener, "The 19 roles", three role tables, "Example theme"). Lines 126-486 were added by tasks 10-2 and 10-3 (`git log docs/theming.md`) and are out of this task's scope.
  - `internal/theme/docs_guard_test.go:1-459` — the guard. Commit `9a7d941e` landed this task's eight tests; commit `bd175e05` (task 11-12) added the four `…MatchesBuiltin` tests, and `25626754` stripped comments. The later additions are the sanctioned remediation, not drift.
- Notes:
  - Token table: three tables, all headed `| Token | Role |`, grouped exactly as §2.5 groups them — text ramp (7, incl. `text.on-selection`), accents and states (6), surfaces (6, incl. `text.on-attention`) = **19**, name-for-name equal to `theme.go:59-79`'s canonical field table. The guard aggregates across all three, so the §2.5 grouping and the single-set comparison coexist correctly.
  - The doc's other four tables (`| Rule | Detail |`, `| Slug | Palette |`, `| Theme | Source |`) are correctly invisible to `parseDocTokenRows` — the `inTokenTable` flag only arms on a `Token` header cell and resets on the first non-row line. The `| \`tokyo-night\` | … |` rows under `Slug` would otherwise be false positives; the Attribution table's multi-backtick first cell is additionally rejected by `backtickedValue`'s inner-backtick check (`docs_guard_test.go:268`).
  - Doc resolves relative to the package dir via `filepath.Join("..", "..", "docs", "theming.md")` (`docs_guard_test.go:12`) — exactly the prescribed `internal/tui/pagepreview_audit_test.go` idiom.
  - Example theme is parsed through the **production** loader: `auditDocExampleTheme` calls `parseThemeBytes` (`load.go:106`), the same content-half entry point `resultFromBytes` uses for disk and embedded bytes alike. No bespoke splitter.
  - Example theme body (`theming.md:100-123`) is byte-identical to the shipped `internal/theme/builtins/tokyo-night.theme:6-29`; only the header comment differs, which `bodyAfterHeaderComment` tolerates by design.
  - `border.footer` appears nowhere in the doc (grepped); `theming.md:87` documents `border` as "One role for every rule and frame: the title rule, the footer rule, modal panel frames, and edit-modal chips" — matching §2.2's consolidation. `border.footer` appears only in `docs_guard_test.go:76` as the synthetic retired-token fixture, which is correct.
  - Ramp ordering is stated as the doc's own prose (`theming.md:33-42`), including the explicit "That ordering is a property of the roles themselves … not of the order you happen to write the keys in" — the §2.7 point — and `text.on-selection` is correctly excluded from the ramp.
  - `text.faint` decorative-only is stated twice — the table row (`:54`) and the amplifying paragraph (`:58-61`). The "below the legibility threshold every other text role has to clear" claim is true against the shipped code: `contrast_test.go:115` asserts `text.faint` vs canvas is strictly *below* `floorLargeUI` for every embedded theme.
  - Both pairing rows name their tint: `text.on-selection` → "read against `bg.selection`, the tint it sits on" (`:55`); `text.on-attention` → "read against `bg.attention`, the tint it sits on" (`:88`).
  - Spot-checked the doc's illustrative use-sites against the renderers rather than taking them on trust (this task exists because of exactly that drift class): `bg.subtle` as the loading bar's empty track with fg==bg so nothing is drawn on it (`loading_view.go:216-217`), `accent.mode` on the Sessions header / preview chrome / in-progress tick (`section_header.go:115`, `pagepreview.go:118`, `loading_view.go:280`), `text.tertiary` on done-step labels and the selected row's path (`loading_view.go:278`, `project_item.go:67` under `selected`), `text.secondary` on selected-row meta (`session_item.go:240`). All hold.

TESTS:
- Status: Adequate
- Coverage: All eight prescribed tests are present and each verifies its criterion by construction, not by proxy:
  - `TestThemingDocGuard_MissingDocFails:18` — asserts an error *and* that it names the unreadable path.
  - `TestThemingDocGuard_ZeroRowsFailsLoudly:30` — asserts exactly one problem, that it says "no token rows", and that it names **no** token, which is precisely the "distinguishable from a name mismatch" criterion rather than a weaker substring check.
  - `TestThemingDocTokenTableMatchesAllTokens:48` — the one live case, against the real doc, with `t.Fatalf` (never `t.Skip`) on a missing file.
  - `TestThemingDocGuard_TokenAbsentFromTableFails:65` / `…UnknownTableRowFails:75` — both drift directions, each via `requireProblemNaming`, which insists on *exactly one* problem naming the offender. That is stronger than a contains-check: it would catch a shotgun message that names every token.
  - `TestThemingDocGuard_RowOrderIsNotAsserted:85` — reversed table, expects zero problems. Correctly pins that ordering is prose, per §2.7.
  - `TestThemingDocExampleThemeIsValid:97` and `…ExampleMissingTokenFails:118` — the real loader on the real doc, and the negative case.
  - The vacuous-parse trap §13.4 names is closed: `auditDocTokenTable:187-192` short-circuits on zero rows with its own message, and `auditDocTokenTable:196-198` asserts the row **count** as well as the name set — so a duplicated row (20 rows, 19 distinct names) fails, which a set comparison alone would pass.
  - The parse-and-compare is factored into `auditDocTokenTable(doc []byte, want []string)` exactly as the task directs, so every drift case is driven against synthetic content built by `docWithTokenTable:439` while the real doc is the single live case.
  - Lane: no build tag, no `t.Parallel()`, no tmux, no fixture, no built binary. Correct for the unit lane and for the project's no-parallel rule.
- Notes:
  - One dead assertion — see the non-blocking note. Otherwise not over-tested: no redundant assertions, no mocking, and the synthetic-doc builders are the minimum needed to stage each case.
  - `TestThemingDocGuard_MissingDocFails` exercises the test's own `readThemingDoc` helper rather than the live guard path, so it is close to tautological (it is `os.ReadFile` erroring on a missing file). It is not valueless — it pins that the error names the path, which is what a developer who deleted the doc will read — and the "fails not skips" property is structurally guaranteed by the `t.Fatalf` at `:51` and `:100`. No change warranted.

CODE QUALITY:
- Project conventions: Followed. Guard-test naming (`docs_guard_test.go`) matches the repo's established ~20-guard idiom (`leaf_guard_test.go`, `slug_collapse_guard_test.go`, `colour_literal_guard_test.go`) rather than the community skill's file-per-source-file default, which is the right precedence call for a guard with no source file under test. No `t.Parallel()` per CLAUDE.md. In-package test (`package theme`), which is what lets it reach the unexported `parseThemeBytes` — the correct choice, since routing through an exported entry point would have meant a bespoke path or a slug the doc example does not have.
- SOLID principles: Good. Each helper does one thing: `parseDocTokenRows` (extract), `auditDocTokenTable` (compare), `extractDocExampleTheme` (locate), `auditDocExampleTheme` (validate). The audit helpers return `[]string` problems rather than taking `*testing.T`, so they are pure and table-drivable — that separation is what makes both drift directions and the vacuous case testable at all.
- Complexity: Low. The deepest function is `parseDocTokenRows` at one loop with a state flag; `markdownTableCells` / `backtickedValue` / `indexOfHeading` / `indexOfFence` are each a handful of lines.
- Modern idioms: Yes — `strings.SplitSeq` (`:226`), range-over-int with `max()` (`:342`), `slices.Clone` / `slices.Reverse`. All valid under `go 1.26.0` and consistent with the repo's `modernize` linter.
- Readability: Good. `t.Helper()` on both helpers taking `*testing.T`. Failure messages state the offending value and what was wanted, and the vacuous-parse message additionally states the doc convention it needs (`"the role tables must open with a %q header cell and each row with a backticked token name"`), so a failure is self-servicing.
- Comment accuracy: N/A — the file carries no comments after the `25626754` comment-stripping sweep, and the identifiers carry the intent. No stale claims. `docs/theming.md`'s own prose was checked against the renderers (see Implementation notes) and holds.
- Security / performance: N/A — a single file read of a repo-local doc.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/docs_guard_test.go:111-115 — delete the `for _, token := range built.All()` empty-value loop in `TestThemingDocExampleThemeIsValid`. It can never fire: `parseThemeBytes` only returns a `Theme` when `requireEveryToken` (internal/theme/validate.go:73-89) found no field with an empty `Value`, so a non-nil-rejection parse already guarantees all 19 are populated. An assertion that cannot fail is the same class of noise the task's own §13.4 vacuous-assertion reasoning warns about. The preceding `problems` check is the load-bearing half and stays.
