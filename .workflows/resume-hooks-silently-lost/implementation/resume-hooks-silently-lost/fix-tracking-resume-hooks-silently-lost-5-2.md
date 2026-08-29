## Attempt 1

Thirteen of fourteen acceptance criteria genuinely met; one met in production but not pinned by a discriminating test. The production code is correct — do not change it.

ISSUES:
- `cmd/hooks_read_lock_test.go:80` — the baseline half of `"it keeps the stale-hooks check green under a degraded read"` is **itself a degraded read**. `seedHooksJSON` stages no sidecar, so `unlockedStore`'s `Load("doctor")` at `:81` degrades on ENOENT exactly as the held-lock run degrades on timeout. The test therefore compares degraded-against-degraded and pins nothing absolute (`:98-106` are all equality against `want`), so it cannot discriminate the failure the criterion exists to name — "a lock problem must not make `portal doctor` report a hook problem". Proven: with the degradation replaced by `return nil, err`, this test still **PASSES** (both sides collapse to `checkNotEvaluable` / "could not read hooks.json" and compare equal), while every other degradation test in both packages fails. This is the only test covering the doctor status/detail/exit-code half of that criterion.
  FIX: give the baseline store a real, free sidecar so `want` is a genuinely locked read. Change `:80` to capture the path and create it, mirroring the fixture pattern already used at `cmd/bootstrap_production_test.go:125`:
      unlockedStore, unlockedPath := seedHooksJSON(t, liveSeedA)
      if err := os.WriteFile(unlockedPath+".lock", nil, 0o600); err != nil {
          t.Fatalf("create sidecar: %v", err)
      }
  The reviewer applied exactly this in a scratch copy and confirmed both directions: green against the real implementation, and against the fail-instead-of-degrade mutant it fails with `status = 4 under a degraded read, want 1` and `detail = "could not read hooks.json" … want "no stale hooks"`.
  ALTERNATIVE: pin the expected values absolutely instead (`want.status == checkPass`, `want.detail == "no stale hooks"`, `wantUnhealthy == false`) and drop the baseline run. Shorter, but it hardcodes `checkStaleHooks`' healthy phrasing into a lock test, so a future wording change breaks it for an unrelated reason. The reviewer recommends the seeded-sidecar fix; doing both is also defensible.
  CONFIDENCE: high

NOTES (context — not work items):
- **Both of your beyond-the-task decisions were upheld.** The widened 5-1 guard still fires under the original regression (`store.go:122:12: Set calls s.Load`) and separately under `s.LoadSnapshot` / `s.loadShared` / `s.loadSharedBounded`; the `scanned == 0` fatal and the `calleeReceiverName(call) == "s"` discriminator are intact, so it is strictly wider, not looser.
- **The shared-fixture edit was justified and removed no coverage.** Reverting the seeding fails exactly three tests, each solely on the extra `load-unlocked` record. With the seeding in place all three still fire on their own subject: deleting the `len(persisted) == 0` early return fails one, emitting the clean-stale summary at zero removals fails another. The suppressed `load-unlocked via=internal` DEBUG is still asserted at `cmd/hooks_read_lock_test.go:189` and `internal/hooks/read_lock_test.go:365`.
- Every other load-bearing claim was mutation-verified: LOCK_SH→LOCK_EX fails the shared-lock and concurrent-reads tests; fail-instead-of-degrade fails eight tests; retaining the fd fails the release test; a `via`-driven bound fails the bound-from-parameter test. Live probe confirmed one DEBUG per read on a 42-entry file, and a fresh config dir left with no `hooks.json` and no `.lock` after both `hook list` and `doctor`.
- `Store.Get(key, via string)` is two adjacent same-typed strings, so `Get("internal", "k00")` compiles and silently looks up the wrong key. `Get` has no production caller, so the exposure is theoretical. **No change asked.**
- `internal/hooks/store.go:78`'s `load()` doc ("the non-locking read the mutations use from inside their own hold") is now narrower than the call graph — `loadSharedBounded` also calls it, on both branches. Nothing it says is false and the 5-2 diff did not touch it. **No correction filed**; noted in case a later reader trips on it.
- The live probe added `state/portal.log` files under the isolated config dir. That is `log.Init` in `main.go` running per invocation, not a hooks read side effect. Not a regression.

## Attempt 1

## Attempt 1

Thirteen of fourteen acceptance criteria genuinely met; one met in production but not pinned by a discriminating test. The production code is correct — do not change it.

ISSUES:
- `cmd/hooks_read_lock_test.go:80` — the baseline half of `"it keeps the stale-hooks check green under a degraded read"` is **itself a degraded read**. `seedHooksJSON` stages no sidecar, so `unlockedStore`'s `Load("doctor")` at `:81` degrades on ENOENT exactly as the held-lock run degrades on timeout. The test therefore compares degraded-against-degraded and pins nothing absolute (`:98-106` are all equality against `want`), so it cannot discriminate the failure the criterion exists to name — "a lock problem must not make `portal doctor` report a hook problem". Proven: with the degradation replaced by `return nil, err`, this test still **PASSES** (both sides collapse to `checkNotEvaluable` / "could not read hooks.json" and compare equal), while every other degradation test in both packages fails. This is the only test covering the doctor status/detail/exit-code half of that criterion.
  FIX: give the baseline store a real, free sidecar so `want` is a genuinely locked read. Change `:80` to capture the path and create it, mirroring the fixture pattern already used at `cmd/bootstrap_production_test.go:125`:
      unlockedStore, unlockedPath := seedHooksJSON(t, liveSeedA)
      if err := os.WriteFile(unlockedPath+".lock", nil, 0o600); err != nil {
          t.Fatalf("create sidecar: %v", err)
      }
  The reviewer applied exactly this in a scratch copy and confirmed both directions: green against the real implementation, and against the fail-instead-of-degrade mutant it fails with `status = 4 under a degraded read, want 1` and `detail = "could not read hooks.json" … want "no stale hooks"`.
  ALTERNATIVE: pin the expected values absolutely instead (`want.status == checkPass`, `want.detail == "no stale hooks"`, `wantUnhealthy == false`) and drop the baseline run. Shorter, but it hardcodes `checkStaleHooks`' healthy phrasing into a lock test, so a future wording change breaks it for an unrelated reason. The reviewer recommends the seeded-sidecar fix; doing both is also defensible.
  CONFIDENCE: high

NOTES (context — not work items):
- **Both of your beyond-the-task decisions were upheld.** The widened 5-1 guard still fires under the original regression (`store.go:122:12: Set calls s.Load`) and separately under `s.LoadSnapshot` / `s.loadShared` / `s.loadSharedBounded`; the `scanned == 0` fatal and the `calleeReceiverName(call) == "s"` discriminator are intact, so it is strictly wider, not looser.
- **The shared-fixture edit was justified and removed no coverage.** Reverting the seeding fails exactly three tests, each solely on the extra `load-unlocked` record. With the seeding in place all three still fire on their own subject: deleting the `len(persisted) == 0` early return fails one, emitting the clean-stale summary at zero removals fails another. The suppressed `load-unlocked via=internal` DEBUG is still asserted at `cmd/hooks_read_lock_test.go:189` and `internal/hooks/read_lock_test.go:365`.
- Every other load-bearing claim was mutation-verified: LOCK_SH→LOCK_EX fails the shared-lock and concurrent-reads tests; fail-instead-of-degrade fails eight tests; retaining the fd fails the release test; a `via`-driven bound fails the bound-from-parameter test. Live probe confirmed one DEBUG per read on a 42-entry file, and a fresh config dir left with no `hooks.json` and no `.lock` after both `hook list` and `doctor`.
- `Store.Get(key, via string)` is two adjacent same-typed strings, so `Get("internal", "k00")` compiles and silently looks up the wrong key. `Get` has no production caller, so the exposure is theoretical. **No change asked.**
- `internal/hooks/store.go:78`'s `load()` doc ("the non-locking read the mutations use from inside their own hold") is now narrower than the call graph — `loadSharedBounded` also calls it, on both branches. Nothing it says is false and the 5-2 diff did not touch it. **No correction filed**; noted in case a later reader trips on it.
- The live probe added `state/portal.log` files under the isolated config dir. That is `log.Init` in `main.go` running per invocation, not a hooks read side effect. Not a regression.
