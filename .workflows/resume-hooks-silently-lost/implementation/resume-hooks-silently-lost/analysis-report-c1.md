# Analysis Report: resume-hooks-silently-lost (Cycle 1)

## Stats

- Total findings: 61 (22 from analysis agents, 39 banked residue)
- Deduplicated findings: 56
- Banked residue: 35 verified in, 0 resolved, 4 discarded
- Proposed tasks: 25

## Summary

The production surface of the hook-key rewrite lands as specified — one home for the `@portal-pane-id` literal, one implementation of the staleness rule, mutations serialised behind the sidecar lock, and strong end-to-end reboot coverage. What the three agents converge on is the *boundaries*: a sweep whose "nothing happened" report covers three of its five decline paths (so `portal doctor --fix` can print nothing at all), a `CleanStale` contract whose central ordering invariant is held only by a comment while its two same-typed arguments are silently transposable, and a persistence store that stopped being a leaf to reach a ten-line predicate. The banked residue adds two reproduced test flakes riding the work unit's headline guarantee, one production version-pinning hazard (`portal state hydrate` baked as a bare PATH lookup), and a large, coherent body of test-helper duplication that the phase boundaries could not reach.

Every bank entry was re-read against the current tree. All 39 still describe live conditions; none had been resolved by later phases, though three had narrowed (one of the four concern-named test files no longer exists, two of the three `AllPaneLister` fakes have since merged into `hookkey_vocabulary_test.go`, and the `seedScrollback`/`seedPaneScrollback` pair now both route through `restoretest.SeedScrollback` — only the assertion half of that entry survives). Those narrowings are reflected in the task text rather than treated as discards.

## Discarded Findings

- **`readHookKey` in `internal/tmux` must NOT be consolidated** (bank, task 3-9) — verified still present at `internal/tmux/hookkey_format_realtmux_test.go:11-15` and correctly so: the `HookKeyFormat` read *is* its subject under test. The entry is a negative record warning a future sweep off it, not a unit of work. Nothing to propose; recorded here so the warning survives the bank's retirement.

- **`project.Store.Remove` rewrites the file for an absent path and emits an INFO naming a removal that did not happen** (bank, task 4-1) — verified at `internal/project/store.go:211-213`, doc comment intact ("It rewrites the file even when the path is absent, so the breadcrumb is emitted either way"). This is the same class of falsehood the work unit removed from the hooks store, in a different store. **Discarded as out of remit, not as invalid** — `internal/project` is outside this work unit's surface entirely and needs a scoping decision (its own work unit, or an explicit widening of this one) rather than a fix folded into a consolidation pass.

- **`internal/project` carries the identical unlocked read-modify-write window this work unit just closed for hooks** (bank, task 5-1) — verified: `go list`-level check finds no `flock` and no lock file anywhere in `internal/project`; `Upsert` (`store.go:74`), `CleanStale` (`:159`), `Rename` (`:189`), `Remove` (`:213`) and `tags.go:38,65` are all `Load()` → mutate → `AtomicWrite`. The concurrent writers are real and already wired (the daemon's throttled prune, `doctor --fix`, the TUI edit modal's immediate-persist path), and `internal/hooks/lock.go` is now a drop-in pattern. Same lost-update class, same 10s cadence. **Discarded as out of remit** for the same reason as above — one store, two distinct defects, both needing a scoping decision.

- **The daemon capture loop issues a bare `-t` target exposed to the rename class** (bank, task 3-10) — verified at `cmd/state_daemon.go:278`: `tmux.PaneTarget(sess.Name, win.Index, pane.Index)` flows to `capture-pane -e -p -S - -t <bare target>`, and session names come from a live enumeration earlier in the same tick, so the target is known-live only as of that read. Measured on tmux 3.7c: with `foo` killed and `foo-2` live, the bare form silently resolves to the wrong session; the `=` form fails correctly. **Discarded as out of remit** — the fix is a call-site change in `cmd/state_daemon.go`, which the prompt scopes outside this work unit. Note that Task 8 below closes the same class *inside* `internal/tmux` (which is in remit, since the work unit rewrote the hook-key primitives there); it does not reach this call site, which passes an already-composed target.
