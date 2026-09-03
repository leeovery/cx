## Attempt 1

# Fix attempt 1 — resume-hooks-silently-lost-8-31

## ISSUES

### `cmd/doctor_stand_down_copy_test.go:139-146` — two deterministic error attrs regressed from exact equality to `strings.Contains`

`standDownErrorAttrCarrying` uses `strings.Contains` for all four error-bearing rows. Two of them were pinned by **exact equality** in the suites this task deleted:

- `run_hook_stale_cleanup_test.go`'s marker-read case asserted `rec.AttrString(t, "error") != sentinel.Error()` for `"tmux dead"`.
- `TestHookSweepReportsStandDown`'s enumeration case did the same for the pane-read error.

Rows `restore marker unreadable` (`:79`, `"no server running"`) and `pane enumeration failed` (`:101`, `"tmux transient"`) carry a **fully deterministic** attr, so the substring form now passes over any extra text the attr grows. That is a precision regression, and the project code-quality standard names substring assertions on deterministic output as an anti-pattern.

The reviewer confirmed on a scratch copy that both rows pass under exact equality. Only the two path-bearing sentinel rows — `hooks.json unreadable` and `hooks.json locked` — genuinely require containment, because their attr embeds a temp path.

FIX: add a sibling beside `standDownErrorAttrCarrying`, e.g.

```go
standDownErrorAttrExactly(want string) func(*testing.T, logtest.Record)
```

asserting `got != want`, and use it for the `restore marker unreadable` and `pane enumeration failed` rows. Leave `standDownErrorAttrCarrying` on the two rows whose attr embeds a temp path. Document on the pair which case each is for, so a later row picks the right one.

Do NOT take the rejected alternative of one helper with an `exact bool` column — a boolean parameter on a table column reads worse than two named helpers, and the code-quality standard discourages boolean parameters.

## COMMENT_CORRECTIONS

- `cmd/doctor_fix_hook_prune_report_test.go:30-32` — the comment opens with a cardinality claim about another file's contents ("pinned once"), which the comment discipline forbids and which ordinary additive change falsifies silently. The load-bearing half is the second clause.
  - OLD:
    ```
    // The words themselves are pinned once, in the stand-down copy home; this
    // case is about the path reaching a line at all, so it renders the one it
    // expects rather than writing a second copy of it.
    ```
  - NEW:
    ```
    // This case is about the path reaching a line at all rather than about its
    // words, so it renders the line it expects through the same vocabulary the
    // report does instead of restating it.
    ```

## NOTES

Reviewed clean — do not revisit:

- Every other deleted assertion was independently accounted for against `HEAD`. The lock-timeout daemon case folded into `run_hook_stale_cleanup_single_report_test.go:44` is **stronger** than what it replaced, and `assertSkippedPruneLine`'s new default arm generalises the deleted loop to all 8 call sites.
- The retained `TestDoctorFixReportsLockedHookPrune` case and the copy home's lock row are complementary rather than duplicates — the seeds differ deliberately (the live seed is forced on the lock row by its exit-code duty). Keeping both is right.
- `assertStalePrunesApplied`'s projects precondition was mutation-verified: unlinking the file now reports the absent-file cause at four call sites instead of misattributing it. Correct, not noise.
- `doctor_fix_hook_prune_report_test.go:47` keeping a literal for `the sweep could not complete` is correct — `skipReasonSweepFailed` is excluded from the copy table, so that line is its sole home.
- The table is at the edge of readable but not over-parameterised: every row runs under `t.Run(tc.name)` so a failure names its reason, and the suite runs in 0.65s.
- CLAUDE.md needs no amendment — no production behaviour, package role, or shared test-helper surface changed.
- One correction to the prior report, for accuracy only (no action): with the table *also* updated to match, the OLD table-value `borrows` form fires too. The two forms are equivalent in that direction, so the win from the fix is diagnosis quality, not detection power. The fix is still right and stays.

Two further findings were banked as cross-scope and must NOT be acted on here: the not-evaluable copy still literal-pinned in `cmd/doctor_test.go` (six sites, written by sibling tasks), and the two coexisting "hooks.json unchanged" vocabularies (whose single-home fix reaches `internal/hookstest`).

## VERIFICATION

- `go build ./...`, `go vet ./...`, `go vet -tags integration ./cmd/...`, `gofmt -l .`
- `go test -count=1 ./cmd/...` then the full unit lane `go test ./...`
- `golangci-lint run`
- Prove the two rows moved to exact equality still pass, and that the exactness bites: append text to each of those two error attrs in production on a **scratch copy** and confirm the rows now fail. Never `-overlay`, never edit the real repo.
