## Attempt 1

ISSUES:
- `cmd/run_hook_stale_cleanup_decline_error_test.go:38-50` ("it emits one clean-stale-skipped line
  naming the restore reason") is fully subsumed by `cmd/run_hook_stale_cleanup_test.go:396-411` ("it
  logs the stand-down at DEBUG and never WARN") — same fixture (`staleHookSeed`,
  `tokenRows(liveSeedA)`, `restoring: true`), same sink, same record assertions, and the existing one
  additionally scans for stray WARNs and asserts no `error` attr. Likewise `:52-69` ("… the
  empty-pane-read reason") is fully subsumed by `cmd/run_hook_stale_cleanup_test.go:547-574`, down to
  the identical `entries == 2` assertion, with the existing one also asserting the outcome and that
  `hooks.json` was not rewritten (`rows: nil` and `tokenRows()` are the same zero-row input). Neither
  new subtest can fail while its twin passes, so they add maintenance surface and the drift risk you
  yourself cited when declining to re-author the fifth prescribed test — the same reasoning applies
  here and was not applied consistently.
  FIX: delete both subtests from `cmd/run_hook_stale_cleanup_decline_error_test.go`, leaving the file
  focused on what is actually new — "it carries the decline reason inside the error the closure
  returns" plus the two outcome subtests. The prescribed coverage remains satisfied by the pre-existing
  tests, exactly as it is for the "reports DeclineReason for every decline path" case you already
  resolved this way.
  ALTERNATIVE: keep the new pair and delete the older two instead. Not recommended — the older
  subtests are supersets (outcome, file-unchanged and no-stray-WARN assertions) and pin the reason
  *values* as literals rather than through the constants, which is the assertion that would catch an
  accidental rename of a spec-governed `reason` value.
  CONFIDENCE: medium
  (`:95-114`, "it still returns nothing-persisted without a stand-down line", is a near-duplicate of
  `cmd/run_hook_stale_cleanup_test.go:576-595` but differs meaningfully — live rows present with an
  empty store, so the enumeration succeeds before `errNothingPersisted` — and is worth keeping.
  `:71-93` is likewise a genuine "ran and removed nothing with token-shaped live keys" case; keep it.)

COMMENT_CORRECTIONS:
- cmd/run_hook_stale_cleanup.go:149-151 — the second sentence asserts an impossibility the code does
  not enforce; `declinedError{}` and `declinedError{standDown{}}` both compile inside `cmd`, which is
  precisely why the source guard exists. A reader who believes the absolute will not look for the hole.
  OLD: // declinedError is a stand-down in transit — the reason its guard decided on,
// carried by the very error that aborts the clean. There is no decline without
// a reason to construct, so none can reach a caller unnamed.
  NEW: // declinedError is a stand-down in transit — the reason its guard decided on,
// carried by the very error that aborts the clean, so a decline names its
// reason at the site that returns it rather than in a variable beside it.

NOTES:
- What the guard actually closes, verified rather than assumed: its AST rule was replayed against
  synthetic sources. It flags `declinedError{}`; it passes `declinedError{view.Decline}` (correct), and
  it is blind to `declinedError{standDown{}}` and to `var e declinedError; return e`. Those two
  residual shapes are self-evidently wrong at the call site in a way the old bug was not — the old
  failure was an *omission* twenty lines from the return, whereas both survivors require writing an
  empty payload in the same expression as the return. The guard closes the one shape that still reads
  as innocent, and it is proportionate; hardening further would be guarding against deliberate misuse.
- The guard is not vacuous: it fatals at zero scanned literals and currently finds exactly the one at
  `:169`, so deleting the production literal fails the test rather than silently disarming it.
- Your decision not to re-author "it reports DeclineReason on the outcome for every decline path" is
  right — `TestHookSweepOutcomeNamesEveryDecline` pins all five post-corrigendum reasons plus a
  uniqueness check.
- `errors.As(err, &declined)` rather than `errors.AsType[declinedError](err)`: correct as written,
  because the recovery sits in a `switch` case where `AsType` cannot bind, and the codebase already
  uses `errors.As` in exactly this position elsewhere. No change wanted.
