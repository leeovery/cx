TASK: theming-system-15-7 — Derive Every Panel Fixture's Union From Its Rows (tick-1167fb, severity low, source: duplication)

ACCEPTANCE CRITERIA:
- No `theme.Union` literal in `internal/tui`'s tests carries a hand-written `Count` or `Rejected`.
- Every fixture's `Rejected` equals the count of non-selectable rows it declares.
- `arrowRejectedCount` has exactly one caller.
- Every existing panel assertion still holds, with any fixture divergence resolved in the rows rather than the count.
- `go test ./internal/tui` passes.

STATUS: complete

SPEC CONTEXT:
This is a phase-15 analysis-remediation task, not a spec-behaviour task: it hardens test fixtures behind the theme slide-over. The behaviour it protects is the spec's enumeration tally — `theme.Union.Count`/`Rejected` are what `internal/theme/union.go:121` reports as the `theme: enumerated` log line (`count`/`rejected` are spec-governed attr keys of the `theme` log component per CLAUDE.md). The production derivation is `Union{Rows: rows, DirUnusable: e.DirUnusable, Count: len(rows), Rejected: countRejected(rows)}` (`internal/theme/union.go:137`), with `countRejected` (`union.go:234-242`) counting `!row.Selectable()` and `Row.Selectable() == (r.Rejection == nil)` (`union.go:37-39`). The fixture helper must mirror exactly that, or a fixture states a tally its own rows contradict.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - Helper pair: `internal/tui/theme_panel_commit_recompute_test.go:291-297` — `themeRowsUnion(rows)` delegating to `themeRowsUnionDirUnusable(rows, dirUnusable)`, which builds `theme.Union{Rows: rows, DirUnusable: dirUnusable, Count: len(rows), Rejected: arrowRejectedCount(rows)}`.
  - Derivation: `arrowRejectedCount` at `internal/tui/theme_panel_arrow_test.go:89-97`.
  - The nine converted fixture sites (commit e436f8e1): `theme_panel_arrow_test.go:83`, `theme_panel_commit_test.go:61`, `theme_panel_cursor_test.go:244`(:331 pre-change), `:269`(:364 pre-change), `:300`(:412 pre-change), `theme_panel_entry_test.go:40`, `theme_panel_open_test.go:50`, `:403`, `theme_panel_test.go:46`.
- Notes:
  - The helper's expression is character-for-character the production one, so fixture tallies cannot diverge from what `Assembler.Reassemble` would produce for the same rows. Verified against `internal/theme/union.go:137` + `:234`.
  - The two `DirUnusable` consumers (`theme_panel_test.go:46`, `theme_panel_entry_test.go:40`) pass a *variable*, not a literal — which is why the explicit-flag form was the right pick of the two options the task allowed: a no-arg `themeRowsUnionUnusable` variant would have forced both call sites to branch. No options struct was added, as instructed.
  - Criterion 1 checked exhaustively. A full sweep of `Union{` in `internal/tui` leaves four remaining literals, all legitimate: the helper itself (`theme_panel_commit_recompute_test.go:296`), two map-of-`theme.Union` type literals (`:319`, `theme_seams_test.go:140`), and one empty `theme.Union{}` (`theme_panel_cursor_test.go:283`). The single hand-written pair that survives is `theme_seams_test.go:126-134` (`Count: 2`, `Rejected: 1`) — out of the helper's reach because that file is `package tui_test` while the helper is `package tui`, and it is the *subject* of a verbatim pass-through assertion (`:141` compares `got.Count`/`got.Rejected` back to `faked`), where deriving the values would weaken rather than strengthen the test. Its values agree with its rows (2 rows, 1 with a `Rejection`), so it is not the failure mode the task targets. Not a violation.
  - Criterion 3 checked: `arrowRejectedCount` is defined once and called from exactly one site, `theme_panel_commit_recompute_test.go:296`.
  - Criterion 4 checked by consumer analysis rather than by re-hardcoding: `Count` and `Rejected` are inert inside `internal/tui` — the panel reads only `union.Rows` and `union.DirUnusable` (`theme_panel.go:224,286-287`, `theme_panel_render.go:18-20`, `theme_panel_geometry.go:149-150`, `theme_panel_message.go:147-149`), and the only production consumer of the tallies anywhere is the `theme: enumerated` emission in `internal/theme`. So the four fixtures that previously omitted `Rejected` while declaring rejected rows (now correctly non-zero) cannot perturb any rendered frame. The two assertions that touch the tallies are relative, not absolute — `theme_panel_open_test.go:150-151` compares the panel's stored `Count` against the same union it was handed, and `theme_panel_close_test.go:211` asserts the union is zeroed on close. No fixture rows needed changing, and none were changed, which is consistent (no fixture's hand-written count actually disagreed with its rows; the drift was all omission).
  - Doc comments added by the task's own commit on the helper pair were later stripped by the repo-wide `chore(comments)` passes (e3fa1503 / 915e7fcb). That is the intentional later supersession the review context flags, not drift; the names carry the meaning unaided.

TESTS:
- Status: Adequate
- Coverage: `TestThemeRowsUnion_DerivesTalliesFromRows` (`theme_panel_commit_recompute_test.go:299-338`) is a table over four cases — zero rows, all-selectable rows, one rejected row, several rejected rows — matching the task's required shape exactly ("including a rows slice with zero and with several rejected rows"). It asserts `Count`, `Rejected` and that `DirUnusable` is not set unasked, for both helpers, plus that `themeRowsUnionDirUnusable(rows, true)` does set the flag. Local `valid`/`rejected` row constructors keep each case's intent legible.
- Notes:
  - Testing a *test helper* is unusual but warranted here: this helper is the single load-bearing derivation for eleven fixtures across seven files, so a silent bug in it would weaken every panel test at once — the same reasoning behind the package's existing `fixture:`-prefixed `t.Fatalf` guards (e.g. `theme_panel_arrow_test.go:116,124`).
  - Mild redundancy: the table asserts the same tallies for `themeRowsUnion` and `themeRowsUnionDirUnusable` when the former is a one-line delegation. It pins the delegation contract, so it is defensible rather than bloat — no change recommended.
  - The task's third listed test ("temporarily add a rejected row to one fixture and confirm `Rejected` follows without a second edit") is a one-off manual verification, not a persisted artefact; the derivation makes it structurally true.
  - Lane/convention rules hold: the new test is hermetic, spawns no daemon and execs no binary (unit lane, correctly untagged), and uses no `t.Parallel()`.

CODE QUALITY:
- Project conventions: Followed. Unit-lane placement, no `t.Parallel()`, error messages in the codebase's explanatory `got/want + why` voice (`"reports Rejected %d, want %d — the count of its own unselectable rows"`), and the `map[string]theme.Union` range-over-named-cases idiom already used at `theme_seams_test.go:140`.
- SOLID principles: Good. One derivation, one owner; the `DirUnusable` variant extends without an options struct, as directed.
- Complexity: Low. Two one-line helpers plus a flat table test.
- Modern idioms: Yes.
- Readability: Good. `themeRowsUnionDirUnusable(rows, false)` is a slightly odd read at the literal-`false` call sites, but the variable-passing consumers justify the flag form and the function name states what the bool means.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_panel_arrow_test.go:89 — rename `arrowRejectedCount` to `rejectedRowCount` and move it beside `themeRowsUnionDirUnusable` in `theme_panel_commit_recompute_test.go`; the `arrow` prefix now names a file it has no caller in (its sole caller is the union helper). The task explicitly said "Leave `arrowRejectedCount` where it is", so this is a deliberate deferral, not a miss — raising it only as the residual follow-up.
- [idea] internal/tui — decide whether to add a `sourceguardtest`-driven guard rejecting `theme.Union{…}` composite literals that carry a `Count:` or `Rejected:` key in `package tui` test files. The task's stated outcome is that "a panel fixture *cannot* state a rejected count its own rows contradict", but nothing structurally stops the next fixture from writing the literal inline again; the repo already runs ~20 such guards, so the precedent exists — the call is whether a test-fixture invariant earns one.
