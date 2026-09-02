## Attempt 1

ISSUES:
- `internal/portaltest/teardown_guard_coverage_test.go:121-135` — `throughLocalArrange` folds `IsolateLine` and `GuardLine` through the package-local hop but never `ServerLine`, so a fixture whose server start sits in a same-package (even same-file) helper drops out of the rule entirely. The old file-level rule caught that shape; the new one does not. Verified with a synthetic staged in the scratchpad — a file holding `func startServer(t) { return tmuxtest.New(...) }` and a `TestX` that isolates, names `PORTAL_STATE_DIR`, calls `startServer(t)` and registers **no** guard: the pre-change rule reports it uncovered, the committed rule reports zero qualifying functions and (beside any other green fixture, i.e. in the real repo) passes it silently. That is the same "route it through a shared arrange and leave the guard's scope" escape the task set out to close, left open in its mirror direction.
  FIX: fold the server call through the same hop in `throughLocalArrange`, attributed to the call line like the other two:
  ```go
  if c.ServerLine == 0 && arrange.ServerLine != 0 {
      c.ServerLine = call.Line
  }
  ```
  The reviewer ran this over the whole tree: it stays green and lifts reach from 52 to 90 qualifying functions (`cmd/abridged_*`'s six tests, `cmd/reattach_*`'s seven, the `composition_e2e_*` suite, `internal/restore/exit_closes_pane_*`, etc. become individually judged instead of relying on their arrange). Order stays correct for the common shape — a helper that both starts the server and isolates yields equal lines, so no inversion is reported and the helper itself is still judged on its own body. Add a staged-source case (`"it fails a fixture whose server starts in a package-local helper"`) using the existing two-file `scanFixture` compose pattern, and drop the now-false `or one that starts the server itself` clause from the guard's doc comment (the COMMENT_CORRECTION below is written for the unfixed state — apply the fix first, then write the doc comment to match the fixed state, keeping the cross-scope limit and dropping the server-itself clause).
  ALTERNATIVE: keep the fold to isolate/guard only and restore a file-level backstop — any file holding a `tmuxtest.New` anywhere still gets the old presence check when no function in it qualifies. That preserves reach without changing per-function attribution, but reintroduces two coexisting rules over one subject and a second failure vocabulary. The reviewer recommends the fold: three lines, empirically green, one rule.
  CONFIDENCE: high (that the gap is real and new), medium (on the fold being the best of the two shapes).

COMMENT_CORRECTIONS:
- `internal/portaltest/teardown_guard_coverage_test.go:153-156` — the paragraph asserts the calls must come "both before it starts the server", then enumerates what stays out of reach and omits the cross-scope blind spot deliberately accepted; a reader concludes any inversion fails.
  OLD:
  ```
  // The pairing is judged per function, with one hop into a same-package arrange,
  // so routing a suite through a shared setup is judged rather than skipped. What
  // stays out of reach is an arrange in another package, or one that starts the
  // server itself.
  ```
  NEW (adjust for the fix: with the server folded through the hop, drop the "or one that starts the server itself" clause):
  ```
  // The pairing is judged per function, with one hop into a same-package arrange,
  // so routing a suite through a shared setup is judged rather than skipped. What
  // stays out of reach is an arrange in another package, and an inversion split
  // across two scopes: order is compared within a single scope, because sibling
  // closures run in whatever order their caller invokes them.
  ```

NOTES:
- The scope-local ordering call is right and was independently confirmed. `cmd/doctor_fix_transient_listpanes_integration_test.go:49-65` genuinely starts its server in a closure that runs after a sibling scope isolates, so a per-function line comparison would have been a false positive. The residual blind spot (server in the outer body, isolation inside a later closure) is a new capability's limit, not a regression — the old rule checked no order at all.
- No test pins the package/directory scoping of the local hop. Production composes the key as `filepath.Dir(rel) + ":" + pkg` inline in the guard body (line 186) while `scanFixture` uses the bare package name, so nothing would fail if the directory half were dropped and a same-named package in another directory started answering another's local calls. Cheap to cover: a second fixture declaring `package y` whose arrange is not followed.
- `firstDefect` recomputes `fn.aggregate().throughLocalArrange(...)` that `auditFixtureCoverage` has just computed and discarded (lines 319 and 335). Passing the already-computed `fixtureCalls` in would remove the duplicate walk and the chance of the two drifting.
- The defect strings are fully deterministic, but the assertions moved from exact equality to layered `strings.Contains`. The substring sets do pin every varying part, so this is not a coverage hole — but an exact `want` string on the two order cases would be stronger and matches the project's stated anti-pattern list.
- The report's "32 file-level records" figure is wrong; the pre-change rule scanned 25 files. The cross-scope limit is explained at `scannedFunc` and `orderDefect`, not in the guard's doc comment as the report claimed — hence the correction above.
