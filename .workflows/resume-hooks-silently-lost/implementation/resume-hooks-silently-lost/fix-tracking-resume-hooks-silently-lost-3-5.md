## Attempt 1

ISSUES:
- `cmd/noncontiguous_window_reboot_integration_test.go:106-118` (with the single-pane token read at :121) — nothing in the suite pins WHICH pane each hook fired in. The assertions are per-marker-file, not per-pane: `assertDivergentMarkerFiredOnce` proves each command ran somewhere, and `assertDivergentMarkerAbsent` proves the two texts did not land in the same file. A saved-to-live pairing that swaps the two stamped panes satisfies both. Proven, not inferred: the reviewer permuted `collectArmInfos` so the two stamped panes' arm info is swapped (each pane fires the other's hook, unstamped pane still last) and the whole suite passed green in 2.0s. The task's own edge case says "a hook firing on the wrong pane fails the test rather than passing it" — today it passes.
  FIX: in the "it fires each pane's own hook" subtest, assert the live-pane→token map across every live pane instead of reading only the last one at :121-123. Restore pairs saved to live panes by structural position (`internal/restore/session.go:112-141`) and uses the same `info.paneToken` for both the `set-option -p` re-stamp and the `--hook-key` baked into the hydrate argv, so the token a live pane carries is the hook that fired in it:
      for i, live := range fx.livePanes {
          want := ""
          if i < len(fx.stamped) { want = fx.stamped[i].token }
          if got := fx.readLivePaneToken(t, live); got != want {
              t.Errorf("live pane %s carries token %q; want %q — a hook fired on the wrong pane",
                  tmux.PaneTarget(divergentSessionName, live.Window, live.Pane), got, want)
          }
      }
  The `want == ""` iteration subsumes the existing last-pane check, so :121-123 goes.
  ALTERNATIVE: have each hook record its own fire site — seed the command at :323 as `echo <marker> $TMUX_PANE >> <file>` and compare the recorded id against `display-message -p -t <live coord> '#{pane_id}'`. The reviewer verified on a scratch socket that a `respawn-pane`d process inherits `TMUX_PANE` and that it equals `#{pane_id}`. This proves the fire site directly rather than through the stamp, so it would survive a regression that ever split the stamped token from the baked key; it costs an extra tmux read per pane and makes the marker file two fields. The reviewer recommends the first — it uses data the fixture already holds, and the one-value-serves-both invariant is documented on `savedPaneArmInfo`.
  CONFIDENCE: high

- `cmd/noncontiguous_window_reboot_integration_test.go:524` — substring counting where the expected content is fully deterministic. The hook is `echo <marker> >> <file>` run once, so the file is exactly `<marker>\n`; `strings.Count(...) != 1` passes on a file that also holds unexpected extra content. The project's quality standard names this ("Substring assertions in tests when exact output is deterministic").
  FIX: compare exactly — `if got := string(data); got != p.markerText+"\n" { t.Errorf(...) }`. That subsumes the cross-fire check, so the loop at :110-117 and `assertDivergentMarkerAbsent` (:530-542) become dead and should go with it.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/noncontiguous_window_reboot_integration_test.go:27-29` — names the wrong mechanism for the fixture's central premise. Under `renumber-windows off` a later `new-window` would take the freed index 1; what actually leaves the gap is that the surviving windows are not renumbered down.
  OLD:
	// The window created between the two survivors and then killed: with
	// renumber-windows off its index is never reused, which is what leaves the
	// saved indices non-contiguous.
  NEW:
	// The window created between the two survivors and then killed: with
	// renumber-windows off the survivors keep their own indices rather than
	// closing the gap, which is what leaves the saved indices non-contiguous.

NOTES (context for the fix — not work items):
- Both claimed deviations check out and neither narrows a criterion into vacuity. The arm-time FIFO stat is forced (`cmd/state_hydrate.go:107` unlinks the FIFO) and the FIFO↔live-key pairing is still exercised end-to-end. The saved-key marker check narrowed to non-live keys is correct and also moot — the set equality at :142-144 already implies the loop at :148-156 in full, so that loop and :157-163 are dead weight.
- "it runs the sweep only after the restore marker clears" (:166-175) can essentially never fail: `restoreUnderMarker` returns the unset error and :360-362 fatals on it, so the marker is unset by construction. The substantive guard is the reap subtest. Criterion met as written; noted so nobody mistakes it for the load-bearing check.
- `readLivePaneToken` (:432-441) swallows every non-zero exit as "" — an unset option and a bad target both exit 1, so an empty result conflates them. Only matters for a pane expected to carry nothing; the recommended fix relies on it for exactly that case, so keep the failure message specific enough to tell them apart if it ever fires.
- Isolation verified end to end (per-test `-S` socket, correct helper ordering, binary through `restoretest`/`portalbintest`, no default-socket reach, no pgrep). Stable across 3 consecutive passes. Full `./cmd/...` integration lane showed only the two already-banked flakes.

## Attempt 1

## Attempt 1

ISSUES:
- `cmd/noncontiguous_window_reboot_integration_test.go:106-118` (with the single-pane token read at :121) — nothing in the suite pins WHICH pane each hook fired in. The assertions are per-marker-file, not per-pane: `assertDivergentMarkerFiredOnce` proves each command ran somewhere, and `assertDivergentMarkerAbsent` proves the two texts did not land in the same file. A saved-to-live pairing that swaps the two stamped panes satisfies both. Proven, not inferred: the reviewer permuted `collectArmInfos` so the two stamped panes' arm info is swapped (each pane fires the other's hook, unstamped pane still last) and the whole suite passed green in 2.0s. The task's own edge case says "a hook firing on the wrong pane fails the test rather than passing it" — today it passes.
  FIX: in the "it fires each pane's own hook" subtest, assert the live-pane→token map across every live pane instead of reading only the last one at :121-123. Restore pairs saved to live panes by structural position (`internal/restore/session.go:112-141`) and uses the same `info.paneToken` for both the `set-option -p` re-stamp and the `--hook-key` baked into the hydrate argv, so the token a live pane carries is the hook that fired in it:
      for i, live := range fx.livePanes {
          want := ""
          if i < len(fx.stamped) { want = fx.stamped[i].token }
          if got := fx.readLivePaneToken(t, live); got != want {
              t.Errorf("live pane %s carries token %q; want %q — a hook fired on the wrong pane",
                  tmux.PaneTarget(divergentSessionName, live.Window, live.Pane), got, want)
          }
      }
  The `want == ""` iteration subsumes the existing last-pane check, so :121-123 goes.
  ALTERNATIVE: have each hook record its own fire site — seed the command at :323 as `echo <marker> $TMUX_PANE >> <file>` and compare the recorded id against `display-message -p -t <live coord> '#{pane_id}'`. The reviewer verified on a scratch socket that a `respawn-pane`d process inherits `TMUX_PANE` and that it equals `#{pane_id}`. This proves the fire site directly rather than through the stamp, so it would survive a regression that ever split the stamped token from the baked key; it costs an extra tmux read per pane and makes the marker file two fields. The reviewer recommends the first — it uses data the fixture already holds, and the one-value-serves-both invariant is documented on `savedPaneArmInfo`.
  CONFIDENCE: high

- `cmd/noncontiguous_window_reboot_integration_test.go:524` — substring counting where the expected content is fully deterministic. The hook is `echo <marker> >> <file>` run once, so the file is exactly `<marker>\n`; `strings.Count(...) != 1` passes on a file that also holds unexpected extra content. The project's quality standard names this ("Substring assertions in tests when exact output is deterministic").
  FIX: compare exactly — `if got := string(data); got != p.markerText+"\n" { t.Errorf(...) }`. That subsumes the cross-fire check, so the loop at :110-117 and `assertDivergentMarkerAbsent` (:530-542) become dead and should go with it.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/noncontiguous_window_reboot_integration_test.go:27-29` — names the wrong mechanism for the fixture's central premise. Under `renumber-windows off` a later `new-window` would take the freed index 1; what actually leaves the gap is that the surviving windows are not renumbered down.
  OLD:
	// The window created between the two survivors and then killed: with
	// renumber-windows off its index is never reused, which is what leaves the
	// saved indices non-contiguous.
  NEW:
	// The window created between the two survivors and then killed: with
	// renumber-windows off the survivors keep their own indices rather than
	// closing the gap, which is what leaves the saved indices non-contiguous.

NOTES (context for the fix — not work items):
- Both claimed deviations check out and neither narrows a criterion into vacuity. The arm-time FIFO stat is forced (`cmd/state_hydrate.go:107` unlinks the FIFO) and the FIFO↔live-key pairing is still exercised end-to-end. The saved-key marker check narrowed to non-live keys is correct and also moot — the set equality at :142-144 already implies the loop at :148-156 in full, so that loop and :157-163 are dead weight.
- "it runs the sweep only after the restore marker clears" (:166-175) can essentially never fail: `restoreUnderMarker` returns the unset error and :360-362 fatals on it, so the marker is unset by construction. The substantive guard is the reap subtest. Criterion met as written; noted so nobody mistakes it for the load-bearing check.
- `readLivePaneToken` (:432-441) swallows every non-zero exit as "" — an unset option and a bad target both exit 1, so an empty result conflates them. Only matters for a pane expected to carry nothing; the recommended fix relies on it for exactly that case, so keep the failure message specific enough to tell them apart if it ever fires.
- Isolation verified end to end (per-test `-S` socket, correct helper ordering, binary through `restoretest`/`portalbintest`, no default-socket reach, no pgrep). Stable across 3 consecutive passes. Full `./cmd/...` integration lane showed only the two already-banked flakes.

## Attempt 2

ISSUES:
- `cmd/noncontiguous_window_reboot_integration_test.go:427-440` — `readLivePaneToken` is now the instrument for the suite's central per-pane identity claim (`:126`), and it silently substitutes **another pane's token** when its target does not resolve. `display-message -p -t <target> -F` does not error on a bad pane target: tmux falls back to the session's current pane and exits 0 (proved on 3.7c against an isolated scratch socket: `display-message -p -t alpha:9.9` → exit 0, returns pane `alpha:0.0`'s token; `show-options -p -v -t alpha:9.9` → exit 1, "no such pane"). The `t.Fatalf` at `:437` is therefore unreachable for target faults, and the comment at `:427-431` — "fails only when the target itself is bad … as it would be under `show-options -p -v`" — states the opposite of the truth. This is a regression against attempt 1, not a fix: the old read conflated a bad target with `""`, the new one conflates it with a plausible-looking wrong token, which is precisely the confusion the assertion exists to detect. Reachable in this file, not hypothetical: `fx.livePanes` is captured at `:361`, before hydration respawns every pane, so a pane that dies in that window leaves a stale coordinate that now reads back as a neighbour's identity instead of failing.
  FIX: drop the per-pane `-t` read entirely and take the token map from one server-side enumeration, so there is no target to resolve and no fallback path. `fx.client.ListAllPaneHookKeys()` already returns `PaneHookRow{Token, Location}` with `Location` in `<session>:<w>.<p>` form — build `map[location]token` once, and in the subtest look each live coordinate up in it, failing with "absent from the live pane enumeration" when the key is missing. That is a genuine "no pane answers to this coordinate" failure, it reads tokens through the same enumeration the sweep itself uses, and it lets `liveTokens` (`:411-425`) be derived from the same single read — the two helpers collapse into one. Delete `readLivePaneToken` and its comment with it.
  ALTERNATIVE (a): keep a per-pane read but route it through `fx.client.ResolveHookKey(tmux.PaneTargetExact(...))`, whose `show-options -p -t` existence probe is verified to exit 1 on a bad pane target. Costs two tmux reads per pane and couples the test to that resolver; the enumeration is one read with no target resolution at all, so the reviewer recommends the enumeration.
  NOTE: the false comment at `:427-431` is deliberately not filed as a separate COMMENT_CORRECTION — under the recommended fix the whole function goes, so a mechanical comment edit would conflict.
  CONFIDENCE: high on the diagnosis, medium on which shape to take.

- `cmd/noncontiguous_window_reboot_integration_test.go` — the acceptance criterion "each stamped pane's hook fires exactly once, into that pane's own marker file, **with no cross-firing between panes**" is still not pinned. The new assertion reads each pane's *stamp*, which is a proxy for the fire site, not the fire site. Proven: mutating `armPanes` to bake the other pane's token into the hydrate argv while stamping each pane correctly — actual cross-firing, pane 0 running pane 1's hook — passes the whole suite green. The coupling holds today only because `savedPaneArmInfo` carries a single `paneToken` field serving both the `set-option -p` re-stamp and the baked `--hook-key`, with a production comment (`internal/restore/session.go:30-34`) forbidding a second field. The criterion as written is nonetheless unmet, and this test is the only end-to-end coverage of the reboot boundary.
  FIX: prove the fire site directly — seed each hook at `:320` as `echo <marker> $TMUX_PANE >> <file>` and compare the recorded pane id against the live pane's own `#{pane_id}`. `TMUX_PANE` is inherited by a `respawn-pane`d process and equals `#{pane_id}` (verified in round 1 on a scratch socket). The exact-content compare at `:517` must then be built from the expected pane id rather than the bare marker text. Take this in the same edit as the enumeration fix above — `ListAllPanesWithFormat` can carry `#{pane_id}` alongside the location, so one enumeration serves both assertions.
  CONFIDENCE: high — the gap is demonstrated by mutation, not inferred.

NOTES (context — not work items):
- Attempt 1's two findings are genuinely closed. Mutation table from the verification pass: swap `infos[0]/infos[1]` → FAIL with both halves named; disable the re-stamp in `stampPaneToken` → FAIL in three subtests; bake the other pane's token while stamping correctly → PASS (the blind spot above); clean tree → PASS, 3 consecutive runs at ~1.3s.
- The attempt-1 NOTES removals check out. The set equality at `:151` implies the removed saved-key marker loop in full, and `WaitForSkeletonMarkersCleared` at `:405` fatals unless every marker cleared, so "all markers clear after hydration" still fails the run — just earlier. `savedPaneKeys` went with them and nothing else broke.
- The comment correction was applied verbatim and is now accurate.
- `fx.savedWindows` (`:304`) is built in capture order and never sorted while `fx.restoredWindows` (`:371`) is sorted before the divergence comparison. `CaptureStructure` groups from `list-panes -a`, which tmux emits in window-index order, so they are comparable in practice, and the saved indices are independently pinned by `divergentSavedToken`'s w0/w2 lookups. No change needed; noted so the asymmetry is not read as a bug.
- Isolation re-verified end to end: the developer's default socket held 49 sessions before and after the full `./cmd/...` integration lane, real daemon pid 6075 alive and unchanged, no pgrep/pkill, no default-socket reach. Full lane green in 411s.

## Attempt 2

## Attempt 1

ISSUES:
- `cmd/noncontiguous_window_reboot_integration_test.go:106-118` (with the single-pane token read at :121) — nothing in the suite pins WHICH pane each hook fired in. The assertions are per-marker-file, not per-pane: `assertDivergentMarkerFiredOnce` proves each command ran somewhere, and `assertDivergentMarkerAbsent` proves the two texts did not land in the same file. A saved-to-live pairing that swaps the two stamped panes satisfies both. Proven, not inferred: the reviewer permuted `collectArmInfos` so the two stamped panes' arm info is swapped (each pane fires the other's hook, unstamped pane still last) and the whole suite passed green in 2.0s. The task's own edge case says "a hook firing on the wrong pane fails the test rather than passing it" — today it passes.
  FIX: in the "it fires each pane's own hook" subtest, assert the live-pane→token map across every live pane instead of reading only the last one at :121-123. Restore pairs saved to live panes by structural position (`internal/restore/session.go:112-141`) and uses the same `info.paneToken` for both the `set-option -p` re-stamp and the `--hook-key` baked into the hydrate argv, so the token a live pane carries is the hook that fired in it:
      for i, live := range fx.livePanes {
          want := ""
          if i < len(fx.stamped) { want = fx.stamped[i].token }
          if got := fx.readLivePaneToken(t, live); got != want {
              t.Errorf("live pane %s carries token %q; want %q — a hook fired on the wrong pane",
                  tmux.PaneTarget(divergentSessionName, live.Window, live.Pane), got, want)
          }
      }
  The `want == ""` iteration subsumes the existing last-pane check, so :121-123 goes.
  ALTERNATIVE: have each hook record its own fire site — seed the command at :323 as `echo <marker> $TMUX_PANE >> <file>` and compare the recorded id against `display-message -p -t <live coord> '#{pane_id}'`. The reviewer verified on a scratch socket that a `respawn-pane`d process inherits `TMUX_PANE` and that it equals `#{pane_id}`. This proves the fire site directly rather than through the stamp, so it would survive a regression that ever split the stamped token from the baked key; it costs an extra tmux read per pane and makes the marker file two fields. The reviewer recommends the first — it uses data the fixture already holds, and the one-value-serves-both invariant is documented on `savedPaneArmInfo`.
  CONFIDENCE: high

- `cmd/noncontiguous_window_reboot_integration_test.go:524` — substring counting where the expected content is fully deterministic. The hook is `echo <marker> >> <file>` run once, so the file is exactly `<marker>\n`; `strings.Count(...) != 1` passes on a file that also holds unexpected extra content. The project's quality standard names this ("Substring assertions in tests when exact output is deterministic").
  FIX: compare exactly — `if got := string(data); got != p.markerText+"\n" { t.Errorf(...) }`. That subsumes the cross-fire check, so the loop at :110-117 and `assertDivergentMarkerAbsent` (:530-542) become dead and should go with it.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/noncontiguous_window_reboot_integration_test.go:27-29` — names the wrong mechanism for the fixture's central premise. Under `renumber-windows off` a later `new-window` would take the freed index 1; what actually leaves the gap is that the surviving windows are not renumbered down.
  OLD:
	// The window created between the two survivors and then killed: with
	// renumber-windows off its index is never reused, which is what leaves the
	// saved indices non-contiguous.
  NEW:
	// The window created between the two survivors and then killed: with
	// renumber-windows off the survivors keep their own indices rather than
	// closing the gap, which is what leaves the saved indices non-contiguous.

NOTES (context for the fix — not work items):
- Both claimed deviations check out and neither narrows a criterion into vacuity. The arm-time FIFO stat is forced (`cmd/state_hydrate.go:107` unlinks the FIFO) and the FIFO↔live-key pairing is still exercised end-to-end. The saved-key marker check narrowed to non-live keys is correct and also moot — the set equality at :142-144 already implies the loop at :148-156 in full, so that loop and :157-163 are dead weight.
- "it runs the sweep only after the restore marker clears" (:166-175) can essentially never fail: `restoreUnderMarker` returns the unset error and :360-362 fatals on it, so the marker is unset by construction. The substantive guard is the reap subtest. Criterion met as written; noted so nobody mistakes it for the load-bearing check.
- `readLivePaneToken` (:432-441) swallows every non-zero exit as "" — an unset option and a bad target both exit 1, so an empty result conflates them. Only matters for a pane expected to carry nothing; the recommended fix relies on it for exactly that case, so keep the failure message specific enough to tell them apart if it ever fires.
- Isolation verified end to end (per-test `-S` socket, correct helper ordering, binary through `restoretest`/`portalbintest`, no default-socket reach, no pgrep). Stable across 3 consecutive passes. Full `./cmd/...` integration lane showed only the two already-banked flakes.

## Attempt 1

## Attempt 1

ISSUES:
- `cmd/noncontiguous_window_reboot_integration_test.go:106-118` (with the single-pane token read at :121) — nothing in the suite pins WHICH pane each hook fired in. The assertions are per-marker-file, not per-pane: `assertDivergentMarkerFiredOnce` proves each command ran somewhere, and `assertDivergentMarkerAbsent` proves the two texts did not land in the same file. A saved-to-live pairing that swaps the two stamped panes satisfies both. Proven, not inferred: the reviewer permuted `collectArmInfos` so the two stamped panes' arm info is swapped (each pane fires the other's hook, unstamped pane still last) and the whole suite passed green in 2.0s. The task's own edge case says "a hook firing on the wrong pane fails the test rather than passing it" — today it passes.
  FIX: in the "it fires each pane's own hook" subtest, assert the live-pane→token map across every live pane instead of reading only the last one at :121-123. Restore pairs saved to live panes by structural position (`internal/restore/session.go:112-141`) and uses the same `info.paneToken` for both the `set-option -p` re-stamp and the `--hook-key` baked into the hydrate argv, so the token a live pane carries is the hook that fired in it:
      for i, live := range fx.livePanes {
          want := ""
          if i < len(fx.stamped) { want = fx.stamped[i].token }
          if got := fx.readLivePaneToken(t, live); got != want {
              t.Errorf("live pane %s carries token %q; want %q — a hook fired on the wrong pane",
                  tmux.PaneTarget(divergentSessionName, live.Window, live.Pane), got, want)
          }
      }
  The `want == ""` iteration subsumes the existing last-pane check, so :121-123 goes.
  ALTERNATIVE: have each hook record its own fire site — seed the command at :323 as `echo <marker> $TMUX_PANE >> <file>` and compare the recorded id against `display-message -p -t <live coord> '#{pane_id}'`. The reviewer verified on a scratch socket that a `respawn-pane`d process inherits `TMUX_PANE` and that it equals `#{pane_id}`. This proves the fire site directly rather than through the stamp, so it would survive a regression that ever split the stamped token from the baked key; it costs an extra tmux read per pane and makes the marker file two fields. The reviewer recommends the first — it uses data the fixture already holds, and the one-value-serves-both invariant is documented on `savedPaneArmInfo`.
  CONFIDENCE: high

- `cmd/noncontiguous_window_reboot_integration_test.go:524` — substring counting where the expected content is fully deterministic. The hook is `echo <marker> >> <file>` run once, so the file is exactly `<marker>\n`; `strings.Count(...) != 1` passes on a file that also holds unexpected extra content. The project's quality standard names this ("Substring assertions in tests when exact output is deterministic").
  FIX: compare exactly — `if got := string(data); got != p.markerText+"\n" { t.Errorf(...) }`. That subsumes the cross-fire check, so the loop at :110-117 and `assertDivergentMarkerAbsent` (:530-542) become dead and should go with it.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/noncontiguous_window_reboot_integration_test.go:27-29` — names the wrong mechanism for the fixture's central premise. Under `renumber-windows off` a later `new-window` would take the freed index 1; what actually leaves the gap is that the surviving windows are not renumbered down.
  OLD:
	// The window created between the two survivors and then killed: with
	// renumber-windows off its index is never reused, which is what leaves the
	// saved indices non-contiguous.
  NEW:
	// The window created between the two survivors and then killed: with
	// renumber-windows off the survivors keep their own indices rather than
	// closing the gap, which is what leaves the saved indices non-contiguous.

NOTES (context for the fix — not work items):
- Both claimed deviations check out and neither narrows a criterion into vacuity. The arm-time FIFO stat is forced (`cmd/state_hydrate.go:107` unlinks the FIFO) and the FIFO↔live-key pairing is still exercised end-to-end. The saved-key marker check narrowed to non-live keys is correct and also moot — the set equality at :142-144 already implies the loop at :148-156 in full, so that loop and :157-163 are dead weight.
- "it runs the sweep only after the restore marker clears" (:166-175) can essentially never fail: `restoreUnderMarker` returns the unset error and :360-362 fatals on it, so the marker is unset by construction. The substantive guard is the reap subtest. Criterion met as written; noted so nobody mistakes it for the load-bearing check.
- `readLivePaneToken` (:432-441) swallows every non-zero exit as "" — an unset option and a bad target both exit 1, so an empty result conflates them. Only matters for a pane expected to carry nothing; the recommended fix relies on it for exactly that case, so keep the failure message specific enough to tell them apart if it ever fires.
- Isolation verified end to end (per-test `-S` socket, correct helper ordering, binary through `restoretest`/`portalbintest`, no default-socket reach, no pgrep). Stable across 3 consecutive passes. Full `./cmd/...` integration lane showed only the two already-banked flakes.

## Attempt 2

ISSUES:
- `cmd/noncontiguous_window_reboot_integration_test.go:427-440` — `readLivePaneToken` is now the instrument for the suite's central per-pane identity claim (`:126`), and it silently substitutes **another pane's token** when its target does not resolve. `display-message -p -t <target> -F` does not error on a bad pane target: tmux falls back to the session's current pane and exits 0 (proved on 3.7c against an isolated scratch socket: `display-message -p -t alpha:9.9` → exit 0, returns pane `alpha:0.0`'s token; `show-options -p -v -t alpha:9.9` → exit 1, "no such pane"). The `t.Fatalf` at `:437` is therefore unreachable for target faults, and the comment at `:427-431` — "fails only when the target itself is bad … as it would be under `show-options -p -v`" — states the opposite of the truth. This is a regression against attempt 1, not a fix: the old read conflated a bad target with `""`, the new one conflates it with a plausible-looking wrong token, which is precisely the confusion the assertion exists to detect. Reachable in this file, not hypothetical: `fx.livePanes` is captured at `:361`, before hydration respawns every pane, so a pane that dies in that window leaves a stale coordinate that now reads back as a neighbour's identity instead of failing.
  FIX: drop the per-pane `-t` read entirely and take the token map from one server-side enumeration, so there is no target to resolve and no fallback path. `fx.client.ListAllPaneHookKeys()` already returns `PaneHookRow{Token, Location}` with `Location` in `<session>:<w>.<p>` form — build `map[location]token` once, and in the subtest look each live coordinate up in it, failing with "absent from the live pane enumeration" when the key is missing. That is a genuine "no pane answers to this coordinate" failure, it reads tokens through the same enumeration the sweep itself uses, and it lets `liveTokens` (`:411-425`) be derived from the same single read — the two helpers collapse into one. Delete `readLivePaneToken` and its comment with it.
  ALTERNATIVE (a): keep a per-pane read but route it through `fx.client.ResolveHookKey(tmux.PaneTargetExact(...))`, whose `show-options -p -t` existence probe is verified to exit 1 on a bad pane target. Costs two tmux reads per pane and couples the test to that resolver; the enumeration is one read with no target resolution at all, so the reviewer recommends the enumeration.
  NOTE: the false comment at `:427-431` is deliberately not filed as a separate COMMENT_CORRECTION — under the recommended fix the whole function goes, so a mechanical comment edit would conflict.
  CONFIDENCE: high on the diagnosis, medium on which shape to take.

- `cmd/noncontiguous_window_reboot_integration_test.go` — the acceptance criterion "each stamped pane's hook fires exactly once, into that pane's own marker file, **with no cross-firing between panes**" is still not pinned. The new assertion reads each pane's *stamp*, which is a proxy for the fire site, not the fire site. Proven: mutating `armPanes` to bake the other pane's token into the hydrate argv while stamping each pane correctly — actual cross-firing, pane 0 running pane 1's hook — passes the whole suite green. The coupling holds today only because `savedPaneArmInfo` carries a single `paneToken` field serving both the `set-option -p` re-stamp and the baked `--hook-key`, with a production comment (`internal/restore/session.go:30-34`) forbidding a second field. The criterion as written is nonetheless unmet, and this test is the only end-to-end coverage of the reboot boundary.
  FIX: prove the fire site directly — seed each hook at `:320` as `echo <marker> $TMUX_PANE >> <file>` and compare the recorded pane id against the live pane's own `#{pane_id}`. `TMUX_PANE` is inherited by a `respawn-pane`d process and equals `#{pane_id}` (verified in round 1 on a scratch socket). The exact-content compare at `:517` must then be built from the expected pane id rather than the bare marker text. Take this in the same edit as the enumeration fix above — `ListAllPanesWithFormat` can carry `#{pane_id}` alongside the location, so one enumeration serves both assertions.
  CONFIDENCE: high — the gap is demonstrated by mutation, not inferred.

NOTES (context — not work items):
- Attempt 1's two findings are genuinely closed. Mutation table from the verification pass: swap `infos[0]/infos[1]` → FAIL with both halves named; disable the re-stamp in `stampPaneToken` → FAIL in three subtests; bake the other pane's token while stamping correctly → PASS (the blind spot above); clean tree → PASS, 3 consecutive runs at ~1.3s.
- The attempt-1 NOTES removals check out. The set equality at `:151` implies the removed saved-key marker loop in full, and `WaitForSkeletonMarkersCleared` at `:405` fatals unless every marker cleared, so "all markers clear after hydration" still fails the run — just earlier. `savedPaneKeys` went with them and nothing else broke.
- The comment correction was applied verbatim and is now accurate.
- `fx.savedWindows` (`:304`) is built in capture order and never sorted while `fx.restoredWindows` (`:371`) is sorted before the divergence comparison. `CaptureStructure` groups from `list-panes -a`, which tmux emits in window-index order, so they are comparable in practice, and the saved indices are independently pinned by `divergentSavedToken`'s w0/w2 lookups. No change needed; noted so the asymmetry is not read as a bug.
- Isolation re-verified end to end: the developer's default socket held 49 sessions before and after the full `./cmd/...` integration lane, real daemon pid 6075 alive and unchanged, no pgrep/pkill, no default-socket reach. Full lane green in 411s.
