## Attempt 1

ISSUES:

1. `internal/tmux/target_composition_guard_test.go:137-209` — the guard is blind to the shape the
   restore bug actually took, so it cannot catch a regression at the site it just fixed. Verified:
   reverting **only** `internal/restore/session.go:89` to its pre-fix
   `target := fmt.Sprintf("%s:", sess.Name)` made
   `TestSessionRestorer_SkeletonTargetsAreExactSessions` fail while
   `TestTmuxTargetsAreComposedThroughTheExactnessVocabulary` **passed**. The rule keys on a literal
   `"-t"` adjacent to the target, but restore composes the target and hands it to
   `SplitWindow`/`NewWindow`, where the `-t` lives in another package behind a `target` parameter — so
   for the restore half the guard only restates what that site's own test already covers. The same
   blind spot was confirmed on a staged
   `func (c *Client) CallerComposed(session string) error { return c.helper(session + ":") }`.
   (The `-t`-adjacent shapes *are* caught — verified on six staged shapes: bare parameter,
   `fmt.Sprintf`, non-exact `PaneTarget`, a local assigned bare, and the pre-fix quickstart argv, all
   reported with file:line.)
   FIX: extend the guard with a second rule covering targets passed as arguments. Parse
   `internal/tmux`'s production files (the guard already enumerates them) to build
   `map[methodName][]int` of `*Client` parameter positions named `target`/`paneID`, then in all three
   packages flag any call whose `SelectorExpr.Sel.Name` is in that map and whose argument at a
   recorded position is neither a vocabulary call nor an identifier in `boundTargets`. Coordination
   point: this also flags `internal/restore/session.go:140` (`liveTarget := tmux.PaneTarget(...)` →
   `SetPaneOption`/`RespawnPane`), which is already banked — so either pin it to `PaneTargetExact` in
   the same edit (a one-token change in a file this task already touches; it is protected upstream
   today because `ListPanesInSession` fails first, so this is hygiene not a bug) or record the
   exemption explicitly in the guard so the gap is visible rather than silent.
   ALTERNATIVE: a lower-reach rule needing no callee resolution — flag any expression composing a
   session-coordinate string (`fmt.Sprintf` whose format contains `%s:`, or a `+ ":"` concatenation)
   anywhere in the three packages outside the four vocabulary constructors. Blast radius checked: in
   these packages only `PaneTarget` (tmux.go:407), `windowTarget` (tmux.go:454) and `ExactCoordTarget`
   (tmux.go:446) compose that shape, so zero false positives today, and it catches both the reverted
   restore line and a `t := session + ":"` local. Heuristic where the first is precise; the first is
   recommended, with this as the fallback if callee resolution proves fiddly.
   CONFIDENCE: medium

2. `internal/tmux/target_composition_guard_test.go:163-168` — a composed argv at package level is
   skipped silently: `check` returns early when `enclosingFunc` finds no enclosing declaration, so
   `var stagedArgs = []string{"kill-session", "-t", "some-name"}` is not flagged (verified — staged in
   `internal/tmux` and absent from the findings). An exhaustiveness guard that quietly declines to look
   at a node class is the failure mode it exists to prevent.
   FIX: drop the early return and check with an empty bound set — replace the body of `check` with
   `bound := map[string]bool{}; if decl := enclosingFunc(file, pos); decl != nil { bound = boundTargets(decl) }; findings = append(findings, bareTargetsIn(fset, elems, bound)...)`.
   Applied in a scratch copy: the package-level literal is then flagged at file:line and the three real
   packages stay clean. Add a `var` line to the failing fixture in
   `TestBareTargetGuard_FlagsAPackageComposingABareTarget` and bump its expected count from 3 to 4.
   CONFIDENCE: high

3. `internal/session/quickstart_prefix_sibling_realtmux_test.go` — only the miss direction is pinned;
   nothing in the suite proves either composed step still *works*. `set-option -t =NAME:` is covered
   indirectly (`TestSessionTargets_LiveSessionStillResolves/SetSessionOption` exercises the same argv
   shape), but `attach-session -t =NAME` has no coverage anywhere — and because the tmux chain aborts
   at the first failure (measured), a wrong form on either step aborts the chain and leaves
   `portal open`'s quickstart path with no attach at all. The file's own house pattern
   (`exact_session_target_realtmux_test.go` carries a gone half *and* a live half for every site) is
   the standard to match.
   FIX: add a live half to the same real-tmux test — create the session under the generated name, run
   `chainStep(t, result.ExecArgs, "set-option")` and assert exit 0 plus `@portal-dir` present on that
   session and absent from the sibling; then run `chainStep(…, "attach-session")` with stdin
   `/dev/null` and assert the stderr is *not* a session-absence message (`no such session` /
   `can't find session` — the same two substrings `internal/tmux/errors.go:37` already classifies on,
   so the coupling is house practice rather than new).
   ALTERNATIVE: pin the attach form in `internal/tmux/exact_session_target_realtmux_test.go` instead,
   alongside the other per-command form checks, and leave the quickstart test to the miss direction.
   That keeps form-validity knowledge in one file but decouples it from the argv quickstart actually
   composes; the first is recommended.
   CONFIDENCE: medium

COMMENT_CORRECTIONS:
- `internal/tmux/target_composition_guard_test.go:14-17` — the history falsifies "reintroduced":
  neither site was ever pinned before this task, so neither can have regressed.
  `internal/restore/session.go`'s bare target dates from `131ffade` (skeleton-create) and
  `internal/session/quickstart.go`'s from `02e48403`, both bare from introduction; the
  rediscover-and-regress history belongs to `internal/tmux` alone.
  OLD:
// The packages that compose a tmux `-t` argument. internal/tmux holds the
// vocabulary; the other two compose argv the client does not run — an exec
// chain and a restore skeleton — and have each reintroduced a bare target
// before.
  NEW:
// The packages that compose a tmux `-t` argument. internal/tmux holds the
// vocabulary; the other two compose argv the client does not run — an exec
// chain and a restore skeleton — and each carried a bare target until now.

NOTES:
- Your correction of the task's premise is confirmed independently: on tmux 3.7c,
  `tmux new-session -d -s probe-2 … \; display-message -p 'REACHED'` against an existing `probe-2`
  printed nothing and exited 1 — the chain is abandoned at the first failure. Second-order consequence
  worth holding onto: because the chain aborts at the first failure, a *wrong* target form in the
  `set-option` step would abort before `attach-session` ever runs, so quickstart's happy path now
  depends on `=NAME:` being a valid `set-option` target. That is what makes issue 3 more than cosmetic.
- Both of your helper choices were re-measured and are correct: `set-option -t =probe-2` fails ("no
  such session") even for a live session, `set-option -t =probe-2:` succeeds, `attach-session
  -t =probe-2` resolves, and `attach-session -t =probe` / `set-option -t =probe:` correctly miss with
  `probe-2` live. The hazard was real: bare `set-option -t probe` wrote `@portal-dir` onto the live
  `probe-2` at exit 0.
- Your bite claims are true: the real-tmux quickstart test fails on the pre-fix argv, and the restore
  unit test fails on the pre-fix target.
