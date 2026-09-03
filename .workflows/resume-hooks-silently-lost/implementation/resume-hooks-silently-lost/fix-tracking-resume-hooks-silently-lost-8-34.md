## Attempt 1

# Fix attempt 1 — resume-hooks-silently-lost-8-34

## ISSUES

### `cmd/hooks_write_lock_test.go:227-229` — the seed literal this task extracted is not used at the one other site that wants it

The third subtest still carries a byte-identical copy of the `oneEntry` seed literal extracted 38 lines above it at `:187-189`. This is duplication **the task itself introduced**: the shared name was created in this edit and then not used at the one other site in the same function that wants it.

It is orthogonal to the measurement-interval argument for keeping that subtest hand-rolled — using `oneEntry` for the seed changes nothing about what `assertReturnsAtLockBound` times.

FIX: replace the inline map literal at `cmd/hooks_write_lock_test.go:227-229` with `oneEntry`:

```go
_, hooksFile := hooksFileInTempDir(t, oneEntry)
```

If the `oneEntry` comment at `:184-186` then reads awkwardly against three consumers, reword its opening clause to "The seed every held-lock row is driven against" — do **not** add a claim about which subtests use it (a cardinality claim about call sites is exactly what the comment discipline forbids).

Do NOT take the reviewer's listed ALTERNATIVE (splitting `runRmCase` into a `stageRmCase` returning the drive closure). It is the more complete answer to the task's Outcome wording, but it adds a second helper for a single call site, against a sibling suite (`TestHookSetLockTimeout`) that hand-rolls five subtests and has no runner at all. The one-line reuse is what is wanted here; the fuller split is a consolidation-pass question.

## NOTES

Everything else was verified and needs no action. Do not revisit:

- **The unjudgeable-sibling claim holds, and was verified rather than inspected.** On a scratch copy the reviewer mutated `internal/hooks/store.go`'s `Store.Remove` to `clear(h)` after the delete; exactly two assertions fired — the token-shaped sibling on the resolved-token path and the unjudgeable sibling at `cmd/hooks_test.go:595`. Both factors bite; nothing was lost in the collapse.
- Every assertion from the four deleted subtests was independently re-derived against `git show HEAD:cmd/hooks_test.go` and accounted for. No surviving assertion is new outside `TestRmCaseRows`.
- **The `%3` → `%42` change is right and does not weaken Home A.** `mockKeyResolver.ResolveHookKey` discards its pane argument; with `%3` the `data["%42"]` assertion would have been vacuous. The change is what makes the migrated assertion meaningful.
- **Leaving the third lock-timeout subtest hand-rolled is accepted.** `assertReturnsAtLockBound` asserts `elapsed >= lockBound` as a *lower* bound, so folding staging into the interval would inflate it in the direction that makes the assertion easier to pass — weakening the one thing that subtest exists for. The task's own problem statement scopes its count to the two that were migrated.
- **The Do-item 3 `t.Fatalf` design call is correct.** A structural fix would make the task's required test name unwriteable; the check is on the sole path into `runRmCase` and routes through `harnesstest.TestingT` so the refusal is itself under test. Both guard rails were mutation-verified to bite.
- Extending `rmOutcome` with `err` as well as `out` was forced by `assertLockFailureReachesStderr`'s error-value assertions. Correct.

Three minor observations were recorded and explicitly need no action: the nil-vs-`{}` seed substitution in the migrated no-entry home (both variants still covered), the resolved-token × unjudgeable-sibling cross product no longer being one case (each factor pinned separately, both fail-fast), and `TestRmCaseRows`' second case not asserting the stamper is the *unpoisoned* one (marginal).

A bank entry deposited by this task carried a factual error about which two subtests are duplicates; it has already been amended in the manifest. No code action.

## VERIFICATION

- `go build ./...`, `go vet ./...`, `go vet -tags integration ./cmd/...`, `gofmt -l .`
- `go test -count=1 ./cmd/...` then the full unit lane `go test ./...`
- `golangci-lint run`
- Confirm `TestHookRmLockTimeout` still passes and that the third subtest's timing assertion is unaffected by the seed reuse.

MACHINE LOAD: the machine is loaded by unrelated system processes (load ~9-22 on 10 cores). Check load before attributing any slow or flaky result to this change; never spawn synthetic load.
