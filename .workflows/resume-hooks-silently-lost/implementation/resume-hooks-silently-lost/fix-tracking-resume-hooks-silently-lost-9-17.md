## Attempt 1

ISSUES:
- `internal/tmux/target_composition_guard_test.go:396-411,522-534` — the guard reduction went one rule too far. Deleting `targetTakingFuncs` / `targetParamPositions` / `bareTargetArguments` / `unionPositions` was justified as "everything the compiler now covers", but the compiler does **not** cover two shapes the old rule did, and both reach a `Target` parameter unflagged:
  1. **An untyped string constant.** `tmux.PortalSaverName` and `tmux.PortalBootstrapName` are untyped consts (`internal/tmux/portal_saver.go:19,24`), so `c.RespawnPane(tmux.PortalSaverName, portalSaverDaemonCommand)` compiles and passes the reduced guard — an unpinned `_portal-saver` target reaching `respawn-pane`, the prefix-match class the whole rule exists to prevent. The old guard flagged it (param position 0 of `RespawnPane`, argument not a vocabulary call). Note the live call site is correctly pinned today (`CoordTargetExact(PortalSaverName)`), so this is a latent enforcement hole rather than a live defect.
  2. **An explicit conversion.** `c.SetPaneOption(tmux.Target(name+":"), …)` compiles and passes. The old guard flagged it. The executor's own report names this shape as "exactly the laundering the type exists to stop" — it was designed around at the one site it hit (`PaneIDTarget`), but nothing stops the next one.
  Net effect: the task's outcome ("a hand-composed tmux target no longer type-checks where a pinned one is required") is true only for computed non-constant strings, and where the type is silent the scan is now silent too. Two comments overclaim in the same direction and are part of the same defect: `internal/tmux/tmux.go:392` ("Only the exactness vocabulary below produces one") and `internal/tmux/target_composition_guard_test.go:262-264` ("The vocabulary's own package is the one place the type cannot speak") — a conversion or an untyped constant silences the type in *any* package.
  FIX: reinstate the argument-position rule, deriving the positions from the declared type instead of the deleted name allow-list — which keeps everything the task wanted gone (the `passThroughTargetParams` allow-list, the naming pressure on parameters) while restoring the reach. Concretely: bring back `targetParamPositions(fn)` with its body reading `isTargetType(field.Type)` (already written, at `:536`) rather than a name map; bring back `targetTakingFuncs` + `unionPositions` + `bareTargetArguments` unchanged (`bareTargetArguments` already routes through `targetIsExact`, which now unwraps `string(...)`, so it needs no edit); restore the `call` argument to `check` in `scanFileForBareTargets` (`:415-425`); and restore `TestBareTargetGuard_FatalsWhenItFindsNoTargetTakingFuncs`, since the derived set is load-bearing again. Add two fixture subtests beside the existing ones at `:196`: an untyped constant handed to a `Target` parameter is a finding, and a `tmux.Target(name+":")` conversion handed to one is a finding — with a `CoordTargetExact(name)` argument and a bound `Target` identifier passing. Then correct the two overclaiming comments to say what is actually true.
  ALTERNATIVE: close both shapes in the compiler instead, by making `Target` a one-unexported-field struct with a `String()` method — only the constructors could then build one, and the guard would shrink to the argv rule alone. Stronger boundary, but it departs from the `type Target string` the Do list prescribes, and it moves every error-message rendering onto `fmt`'s Stringer path, putting the byte-identity constraint at risk for no gain the guard arm does not also deliver. Take the guard arm.
  CONFIDENCE: high (that the gap is real and in scope); medium on the exact shape of the reinstated rule.

COMMENT_CORRECTIONS:
- `internal/tmux/tmux.go:392-395` — the first paragraph makes two claims the code falsifies: any conversion produces a `Target`, and the client itself spends one back to a string at every `c.cmd.Run`, so the exec'd argv is not "the one place".
  OLD: // Target is a composed tmux `-t` argument. Only the exactness vocabulary below
// produces one, so a target reaching tmux has been pinned by construction; a
// caller composing an argv the client does not run converts it back to a string
// at that argv, which is the one place the type is spent.
  NEW: // Target is a composed tmux `-t` argument. The exactness vocabulary below is
// the route that pins one, and a Target is spent back as a string at the argv
// that consumes it — inside the client on its way to the commander, or in a
// tmux command line the client never runs.

NOTES:
- Byte-identity was established independently of the new tests, by reading the complete diff of all nine production files: every `-t` site is a mechanical `string(X)` wrap of the identical inner expression, and the pre-existing literal assertions that predate the change still pass unedited.
- The fifth constructor (`PaneIDTarget`) is the right call — a pane ID is server-unique and never prefix-matched, and a named route keeps `$TMUX_PANE` out of the codebase as a bare conversion.
- The generic seam is right for the cycle it faces and does not weaken the contract: a `string`-taking capturer still satisfies it at `T=string`, so nothing that used to fit stopped fitting, and the pinning property never lived in that seam.
- The cross-package source pin edit was necessary, not convenient: that guard is a verbatim byte-match on the changed declaration, and its purpose survives the edit intact.
- The other deliberate narrowing (multi-valued call results no longer bound by name) is the safe direction — it can only produce a false positive, and no production site spends such an identifier on an argv today.
- Losing the emptiness tripwire does not let the guard pass by having stopped looking: a probe staged over the real `cmd` sources on every run is a stronger liveness proof than a count.
- `PaneIDTarget` accepts any string unvalidated. No wider than the name exemption it replaces, but a `%`-prefix check would make it non-launderable if it is ever spent on more than `$TMUX_PANE`.
- The integration lane was not re-run by the reviewer; the executor's stashed-tree comparison of the known composite-bootstrap flake was the one claim not independently reproduced.

## Attempt 2

ISSUES:
- `internal/tmux/target_composition_guard_test.go:551-585,822-838` — `targetReturningFuncs` / `targetResultPositions` / `bindMultiValuedTargets` are new machinery authored this round and no fixture exercises them. Only the real-tree scan touches the positive direction; the discriminating property is untested in both of its parts, confirmed by mutation in a copied tree:
  (a) replacing `bindMultiValuedTargets`'s body with "bind every identifier on the left of any multi-valued assignment" — which reinstates exactly the blanket exemption the type change was meant to delete — leaves the entire guard suite green;
  (b) changing `width := max(len(field.Names), 1)` to `width := len(field.Names)`, dropping the unnamed-result position accounting, also leaves it green.
  Nothing pins that a non-Target sibling at another result position stays unbound, and nothing pins the unnamed-result width at all. That is the regression class this guard exists to be tamper-evident against, and it is the one arm of the scan with no fixture behind it.
  FIX: add one subtest beside the others in `TestBareTargetGuard_FlagsAPackageComposingABareTarget`, in the style of the neighbouring fixtures — a producer with unnamed results, `func resolveCurrentPaneKey(name string) (string, Target, error) { return "", CoordTargetExact(name), nil }`, a consumer `key, target, err := resolveCurrentPaneKey(name)` that spends BOTH identifiers after a literal `-t`, and an assertion of exactly one finding whose `detail`/`pos` name the `key` spend. That single fixture kills both mutants: the sibling must stay unbound (kills (a)), and the Target must be recognised at result position 1 behind an unnamed field (kills (b)).
  ALTERNATIVE: two subtests, one per property. Cheaper to read in isolation but it re-authors the same fixture twice; the combined one is the better fit for this file's existing style. Take the single fixture.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/tmux/tmux.go:442-445` — the paragraph claims the type system holds the whole composition rule, which the file's own companion guard exists because it does not: an untyped constant converts implicitly and an explicit conversion converts anything. Same class of overclaim the previous round corrected four lines up.
  OLD: // The exact forms return Target, and every `-t` parameter takes one, so the
// composition rule holds in the type system: a hand-composed string does not
// type-check where a pinned target is required, and neither renaming a local nor
// passing one through a helper launders it into place.
  NEW: // The exact forms return Target, and every `-t` parameter takes one, so a
// computed string does not type-check where a pinned target is required and
// renaming a local cannot launder one into place. Two shapes still reach a
// Target without the vocabulary — an untyped constant, which converts
// implicitly, and an explicit conversion, which converts anything — so the type
// carries the rule without closing it.
- `internal/tmux/target_composition_guard_test.go:545-550` — "the compiler already vouched for it" overstates what a declared result position establishes: the compiler vouches the value is a Target, not that the vocabulary produced it.
  OLD: // targetReturningFuncs records, per function name, the result positions declared
// with the target type — the mirror of targetTakingFuncs, and how a target
// arriving through a multi-valued call is recognised: there the right-hand side
// is one call for several identifiers, so the position is what says which of
// them holds the target. The declaring signature is where the compiler already
// vouched for it.
  NEW: // targetReturningFuncs records, per function name, the result positions declared
// with the target type — the mirror of targetTakingFuncs, and how a target
// arriving through a multi-valued call is recognised: there the right-hand side
// is one call for several identifiers, so the position is what says which of
// them holds the target. The declared type is what the position is read from,
// which is narrower than provenance: a declaring body composing its result by
// conversion is a shape no rule here reaches.

NOTES:
- The result-side binding is the right resolution and is strictly narrower than the name allow-list it replaces (that rule trusted any local named target/paneID from any multi-valued source, including a plain string; this one requires a compiler-declared Target at that position). It does not re-open the deleted hole under a different name.
- Residual gap, measured rather than reasoned: a multi-valued producer that hand-composes its Target result yields 0 findings at every spend, while the single-valued equivalent yields 2. Strictly narrower than the name rule it replaces, costs a deliberately-declared Target-returning function to exploit, and closing it symmetrically would need exemptions for the vocabulary's own constructors and for empty literals on error paths — more machinery than a residue backstop warrants.
- Both new fixtures from the previous round verified to bite; each guard arm mutation-probed individually; fixture provenance checked per fixture, with the 5→4 finding-count drop traced to the one removed line whose coverage the new fixtures carry.
- The two deleted "non-method helper" subtests are not lost coverage: under the type rule a `target string` parameter is no longer exempt, so that shape is flagged at the helper itself rather than at its caller.
- `unionPositions`'s pooling survives and now serves both takers and producers, but the subtest that pinned it went with the name rule; the behaviour is claimed in prose and verified nowhere. No same-named collision exists in the scanned set today.
- `isTargetType` matches the qualified spelling as literal text, so a file importing internal/tmux under an alias would silently drop out of both rules. No such import exists; same fragility class as the name rule it replaces.
- The integration lane was not run by either reviewer; the executor's stashed-tree flake comparison remains unreproduced independently.
