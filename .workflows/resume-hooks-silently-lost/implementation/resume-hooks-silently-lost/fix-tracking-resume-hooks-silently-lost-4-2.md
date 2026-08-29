## Attempt 1

ISSUES:
- `cmd/hooks_rm_exit_test.go:93-114` — the subtest `"it consults the store for nothing when the pane carries no token"` does not verify what its name and its acceptance criterion ("No empty key reaches `Store.Remove` from either path") claim. Both of its assertions are satisfied by an empty key that *does* reach the store: `Store.Remove` on a key it cannot find returns `(false, nil)` and writes nothing (`internal/hooks/store.go:139-145`), so neither the `os.Stat` absent-file check at `:103` nor `assertHooksFileUnchanged` at `:113` discriminates. Proven: moving the guard from `cmd/hooks.go:225` to after `store.Remove` — so an empty key genuinely reaches the mutation — leaves the entire `./cmd` package green. The regression this fails to catch is real and observable: `Remove("")` against a `hooks.json` carrying a `""` entry deletes it, which is exactly the hazard the guard's position exists to prevent.
  FIX: seed the empty key into the second-half fixture and assert it survives. At `cmd/hooks_rm_exit_test.go:107-109`, change the seed map to include a `""` entry, e.g. `map[string]map[string]string{"": {"on-resume": "an empty-key entry"}, "tok123": {"on-resume": "claude --resume abc"}}`, and leave the existing `assertHooksFileUnchanged(t, hooksFile, before)` at `:113` as the assertion — it fails the moment the guard moves below the store, because `Remove("")` would empty the file. The reviewer ran this exact probe: green against the current code, red against the moved-guard mutation with `hooks.json changed on a failing route … after {}`. Two lines, no production change, no new seam.
  ALTERNATIVE: point `PORTAL_HOOKS_FILE` at an unreadable path so `Store.Load` errors — the returned message would become the load failure rather than `no resume hook registered for this pane`, which also discriminates. It costs a chmod fixture and couples the assertion to the store's error text; the reviewer recommends the empty-key seed.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/hooks_rm_exit_test.go:143-144` — the second clause narrates what the assertions below it prove, which the comment discipline lists under "never in a comment" (claims about tests); removing or moving the two call-count checks turns it into a confident lie. The first clause is legitimate rationale for configuring both seams with errors.
  OLD:
		// Both seams fail loudly if consulted, so a tmux call on this path cannot
		// pass silently; their call counts prove none was made.
  NEW:
		// Both seams return an error if consulted, so an accidental tmux call on
		// this path cannot pass silently.

NOTES (context — not work items):
- All fifteen acceptance criteria are met in code; the gap is coverage, not behaviour. The production change is correct as written — do not move the guard.
- The three first-write passes were mutation-checked and are **not** vacuous: forcing `removed = false` reds both success subtests; `%w`→`%v` in `resolveCurrentPaneKey`'s wrap, and dropping the wrapped error entirely, each red the gone-pane subtest. The executor's argument that they pin preserved behaviour holds.
- The README token `k3Xp7Q` was verified through `session.IsTokenShaped` — 6 bytes, every byte in `NanoIDAlphabet` → `true`; the retired `sess:0.1` → `false`. Code-fence comment alignment preserved.
- Documentation bounds hold exactly: CLAUDE.md 1 line, README.md 2 lines, the README rename guarantee (`:195`) and "When hooks fire" paragraph (`:203-207`) byte-identical, no CHANGELOG edit.
- `it exits 0 and removes on the --pane-key path` (`:190`) asserts `resolver.calls == 0`, but with `TMUX_PANE` set to `""` that is weakly satisfied — `requireTmuxPane` short-circuits before the resolver even under a hoisted-resolve mutation. The discrimination is carried by the failing-route sibling at `:157`. Collectively adequate; no change needed.
- The unidentified `cmd` integration failure did not reproduce: two clean standalone runs (411.9s each). The full lane produced one banked teardown flake in `internal/restore` (`TestMultiPaneLegacy_UnstampedNoHookLandsOnBareShell`, `TempDir RemoveAll … directory not empty` — test body passed, only cleanup failed).
- The two `errorsastype` lint hits in the new file were left as `errors.As` deliberately, matching ~8 pre-existing sites including the adjacent `cmd/hooks_test.go:318`. Correct call; banked as a repo-wide sweep.

## Attempt 1

## Attempt 1

ISSUES:
- `cmd/hooks_rm_exit_test.go:93-114` — the subtest `"it consults the store for nothing when the pane carries no token"` does not verify what its name and its acceptance criterion ("No empty key reaches `Store.Remove` from either path") claim. Both of its assertions are satisfied by an empty key that *does* reach the store: `Store.Remove` on a key it cannot find returns `(false, nil)` and writes nothing (`internal/hooks/store.go:139-145`), so neither the `os.Stat` absent-file check at `:103` nor `assertHooksFileUnchanged` at `:113` discriminates. Proven: moving the guard from `cmd/hooks.go:225` to after `store.Remove` — so an empty key genuinely reaches the mutation — leaves the entire `./cmd` package green. The regression this fails to catch is real and observable: `Remove("")` against a `hooks.json` carrying a `""` entry deletes it, which is exactly the hazard the guard's position exists to prevent.
  FIX: seed the empty key into the second-half fixture and assert it survives. At `cmd/hooks_rm_exit_test.go:107-109`, change the seed map to include a `""` entry, e.g. `map[string]map[string]string{"": {"on-resume": "an empty-key entry"}, "tok123": {"on-resume": "claude --resume abc"}}`, and leave the existing `assertHooksFileUnchanged(t, hooksFile, before)` at `:113` as the assertion — it fails the moment the guard moves below the store, because `Remove("")` would empty the file. The reviewer ran this exact probe: green against the current code, red against the moved-guard mutation with `hooks.json changed on a failing route … after {}`. Two lines, no production change, no new seam.
  ALTERNATIVE: point `PORTAL_HOOKS_FILE` at an unreadable path so `Store.Load` errors — the returned message would become the load failure rather than `no resume hook registered for this pane`, which also discriminates. It costs a chmod fixture and couples the assertion to the store's error text; the reviewer recommends the empty-key seed.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/hooks_rm_exit_test.go:143-144` — the second clause narrates what the assertions below it prove, which the comment discipline lists under "never in a comment" (claims about tests); removing or moving the two call-count checks turns it into a confident lie. The first clause is legitimate rationale for configuring both seams with errors.
  OLD:
		// Both seams fail loudly if consulted, so a tmux call on this path cannot
		// pass silently; their call counts prove none was made.
  NEW:
		// Both seams return an error if consulted, so an accidental tmux call on
		// this path cannot pass silently.

NOTES (context — not work items):
- All fifteen acceptance criteria are met in code; the gap is coverage, not behaviour. The production change is correct as written — do not move the guard.
- The three first-write passes were mutation-checked and are **not** vacuous: forcing `removed = false` reds both success subtests; `%w`→`%v` in `resolveCurrentPaneKey`'s wrap, and dropping the wrapped error entirely, each red the gone-pane subtest. The executor's argument that they pin preserved behaviour holds.
- The README token `k3Xp7Q` was verified through `session.IsTokenShaped` — 6 bytes, every byte in `NanoIDAlphabet` → `true`; the retired `sess:0.1` → `false`. Code-fence comment alignment preserved.
- Documentation bounds hold exactly: CLAUDE.md 1 line, README.md 2 lines, the README rename guarantee (`:195`) and "When hooks fire" paragraph (`:203-207`) byte-identical, no CHANGELOG edit.
- `it exits 0 and removes on the --pane-key path` (`:190`) asserts `resolver.calls == 0`, but with `TMUX_PANE` set to `""` that is weakly satisfied — `requireTmuxPane` short-circuits before the resolver even under a hoisted-resolve mutation. The discrimination is carried by the failing-route sibling at `:157`. Collectively adequate; no change needed.
- The unidentified `cmd` integration failure did not reproduce: two clean standalone runs (411.9s each). The full lane produced one banked teardown flake in `internal/restore` (`TestMultiPaneLegacy_UnstampedNoHookLandsOnBareShell`, `TempDir RemoveAll … directory not empty` — test body passed, only cleanup failed).
- The two `errorsastype` lint hits in the new file were left as `errors.As` deliberately, matching ~8 pre-existing sites including the adjacent `cmd/hooks_test.go:318`. Correct call; banked as a repo-wide sweep.
