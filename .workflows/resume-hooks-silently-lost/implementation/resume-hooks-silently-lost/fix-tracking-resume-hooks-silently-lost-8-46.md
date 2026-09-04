## Attempt 1

ISSUES:
- `internal/tmux/target_composition_guard_test.go:449` — `takers[fn.Name.Name] = positions` overwrites rather than unions, so two same-named declarations collide and one function's target positions are silently dropped. There is a live collision in the tree today, confirmed by replaying the map build over the fifteen derived package directories:

  ```
  COLLISION stampPaneToken -> winner positions [2]
      [0] at cmd/hooks.go:109
      [2] at internal/restore/session.go:162
  ```

  `cmd.stampPaneToken(paneID string)` loses position 0 to `(*SessionRestorer).stampPaneToken(sessionName, liveKey, target, token)`, so its `paneID` parameter is exempted where it is spent (`bindFields` binds it) while its call site at `cmd/hooks.go:209` is never checked — precisely the exemption-wider-than-recognition asymmetry this task exists to close. Two comments the diff added or touched assert the opposite: `:443` ("a wider map only ever checks more call sites") and `:444-447` ("Reading both is what keeps the rule's exemption no wider than its recognition"), plus the `passThroughTargetParams` doc at `:117-119` ("Both halves read the same set over the same declarations, so a name it exempts is never a name it declines to check"). The winner is decided by the sorted directory order, so the current outcome is incidental: had `internal/restore` sorted before `cmd`, the restore method's position-2 check — the one covering the parameter renamed from `liveTarget` to `target` to satisfy this very allow-list — would be the one dropped.
  FIX: Union the positions instead of overwriting — `takers[fn.Name.Name] = union(takers[fn.Name.Name], positions)`, deduped and sorted so the finding order stays deterministic. That surfaces one new real-tree finding at `cmd/hooks.go:209`, where `stampPaneToken(paneID)` passes an identifier the scan cannot see as bound (the call sits in a `RunE` function literal inside a package-level `var`, so `enclosingFunc` returns nil and `bound` is empty). Close that by having `boundTargets` also bind identifiers declared by a `*ast.AssignStmt` whose LHS name is in `passThroughTargetParams` — the same treatment a signature-declared `paneID` already gets, and consistent with the existing local-binding arm `bindVocabularyAssignments`.
  ALTERNATIVE: Keep the overwrite and instead make the map key `<package>.<name>` while matching call sites within the declaring package only. That removes the false-collision class outright but narrows the guard's cross-package reach, which is the property the derived package set was built for — so the reviewer recommends the union.
  CONFIDENCE: medium — the union itself is unambiguous; how to absorb the `cmd/hooks.go:209` finding has more than one defensible answer.

- `internal/tmux/target_composition_guard_test.go:367,380,383` — the same edit renamed `targetTakingMethods` → `targetTakingFuncs` and its fatal text from "no method taking…" to "no function taking…", but left the test that asserts on that fatal named `TestBareTargetGuard_FatalsWhenItFindsNoTargetTakingMethods`, with diagnostics still reading "a package declaring no target-taking method" (`:380`) and "does not say the method set was empty" (`:383`). Criterion 4 puts naming-matches-subject inside this task's contract, and this is drift the diff itself created.
  FIX: Rename the test to `TestBareTargetGuard_FatalsWhenItFindsNoTargetTakingFuncs` and change "method" to "function" in both message strings.
  CONFIDENCE: high.

COMMENT_CORRECTIONS:
- internal/tmux/target_composition_guard_test.go:407 — the scan now reaches plain functions as well as methods, so "a method" is stale.
  OLD: `// "-t", or passed to a method that takes an already-composed target. The second`
  NEW: `// "-t", or passed to a function that takes an already-composed target. The second`
- internal/tmux/target_composition_guard_test.go:365 — the derived set is no longer a method set.
  OLD: `// method set it derives, so an empty set is a scan that has stopped looking`
  NEW: `// function set it derives, so an empty set is a scan that has stopped looking`

NOTES:
- `internal/tmux/target_composition_guard_test.go:276-278` asserts on the finding with `strings.Contains(findings[0].detail, "addressSession")` while its sibling fixture two blocks down pins its detail string exactly. The full detail here is deterministic (`the target passed to addressSession is composed by hand — ` + `routeItThrough`) and is pinned exactly nowhere, so an exact comparison would cost nothing and close that gap.
- The three parameter renames the tightening forced (`emitResolveDecision`'s and `singleMissError`'s `target` → `query`; `swingPortalLogSymlink`'s `target` → `dayName`) each read as an improvement in their own right rather than a concession. The `resolve` component's `target` log attr key is unchanged, so the spec-governed vocabulary is intact. The executor's flag that the task's "zero new findings" premise held only after these renames is accurate.
- `boundTargets` is never consulted for a call inside a function literal held by a package-level `var` (`enclosingFunc` walks `file.Decls` for `*ast.FuncDecl` only), which is how every Cobra `RunE` body in `cmd` is shaped. Today that only under-binds; a `target := tmux.SessionTargetExact(x)` local spent inside a `RunE` would be a loud false positive rather than a silent miss, so it is not urgent — but it is the same flow-insensitivity family the deferred `type Target string` note at `internal/tmux/tmux.go:422-431` is meant to subsume, and that note does not mention it.
- CLAUDE.md's `tmux` row was updated in the same edit as instructed, including the added "the four share one `<kind>TargetExact` shape" clause; no README or `docs/` reference to these helpers exists.

## Attempt 2

ISSUES:
- `internal/tmux/target_composition_guard_test.go:694` — the new name arm of `bindAssignedTargets` binds **any** identifier named `target`/`paneID` on the LHS of **any** assignment, whatever it was assigned from. That reopens the laundering hole the guard exists to close, in a new place: a hand-composed target assigned to a local of that name is now silently exempt. Verified empirically against a copy of the tree (repo untouched) — the fixture

  ```go
  func addressSession(name string) {
      target := name + ":"
      run("has-session", "-t", target)
  }
  ```

  yields **1 finding at HEAD** (`the target after "-t" is composed by hand — route it through …`) and **0 findings under the delivered code**. It is a strict weakening, it is not covered by any fixture, and it falsifies two claims the same diff asserts: the `passThroughTargetParams` doc at `:119-121` ("Both halves read the same set over the same declarations, so a name it exempts is never a name it declines to check" — a local has no call sites for the recognition half to check) and `targetTakingFuncs`' "keeps the rule's exemption no wider than its recognition" at `:497-501`. The real-tree finding the arm was added to absorb (`cmd/hooks.go:209`, `hookKey, paneID, err := resolveCurrentPaneKey()`) is a *multi-valued* assignment and does not need the paired case.
  FIX: Restrict the name arm to the unpaired (multi-valued) shape, which is the only one the paired vocabulary arm cannot express — `if (!paired && passThroughTargetParams[ident.Name]) || (paired && isExactTargetCall(assign.Rhs[i]))`. The reviewer applied exactly this to a full copy of the tree and re-ran the guard: the real tree still produces **zero** findings, every existing fixture keeps its verdict (including the new `"it passes a target held by a local of the vocabulary's own name"`, whose `key, target, err := resolveCurrentPane()` is unpaired), and the laundered-local fixture above is flagged again. Add a fixture pinning it — `"it flags a hand-composed target assigned to a local of a pass-through name"`, asserting 1 finding with the exact `the target after "-t" is composed by hand — ` + `routeItThrough` detail — and adjust `bindAssignedTargets`' doc at `:682-686` to say the name arm reads a multi-valued result, not any assignment.
  ALTERNATIVE: Bind the name only when the RHS is a call expression (single or multi-valued). That also lets `target := someHelper(name)` through and still admits `fmt.Sprintf`-composed targets, so it is weaker than the unpaired restriction for no extra reach — the reviewer recommends the unpaired form.
  CONFIDENCE: high — the narrower rule is measured green on the real tree and on the whole existing fixture set.

COMMENT_CORRECTIONS: none (the two false claims above are remedied by the code change, so they belong to the issue).

NOTES:
- The prior review round prescribed this arm ("bind identifiers declared by a `*ast.AssignStmt` whose LHS name is in `passThroughTargetParams`") at `medium` confidence and flagged that the absorption of `cmd/hooks.go:209` had more than one defensible answer. The executor followed it faithfully; the finding above is the narrower answer, not a reversal of the collision fix, which is correct and well-covered by `"it flags every position two same-named target takers declare"`.
- The name-based binding is type-blind: `internal/state/scrollback_tail.go:56` (`target := n + 1`, an int) is bound as a target today. Harmless where it sits, and the unpaired restriction removes it too. `internal/log/sink.go:225` (`target := portalLogName + "." + today`) is the exact laundering shape sitting in a package that merely happens not to import `internal/tmux`.
- The three collateral parameter renames (`emitResolveDecision`/`singleMissError` `target` → `query`, `swingPortalLogSymlink` `target` → `dayName`) each read correctly against their callers, and the `resolve` component's spec-governed `target` log attr key is unchanged. `restampPaneToken` matches its own doc ("re-establishes").
