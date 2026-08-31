## Attempt 1

ISSUES:
- `cmd/hooks_read_lock_test.go:92-104` — `"it takes no tmux read and creates nothing on a fresh install"`
  now runs against a directory that already holds `hooks.json.lock`, because `hooksFileInTempDir` stages
  the sidecar. Its `before`/`after` `dirListing` comparison could previously catch `hook list` creating
  *either* `hooks.json` or its sidecar; it can now only catch the former, and the fixture contradicts the
  case's own name — a sidecar exists only after a mutation, so this is no longer a fresh install. Its
  doctor twin at `:51-66` keeps the honest fixture (bare dir, comment at `:52` spelling out "a read that
  created either it or the sidecar would show up"), so the CLI half of that pair silently lost its
  meaning. This is also the exact case your own new doc comment at `cmd/testhelpers_test.go:59-61` says
  must not reach for this route.
  FIX: stage the fixture through the shared stager's absence axis and point the env var at it, replacing
  `configDir, _ := hooksFileInTempDir(t)` with
  `_, hooksFile := hookstest.StageStore(t, hookstest.Staging{SidecarAbsent: true})`,
  `t.Setenv("PORTAL_HOOKS_FILE", hooksFile)`, `configDir := filepath.Dir(hooksFile)`. That restores an
  empty directory as the `before` baseline, states the precondition in the axis vocabulary (so the
  resulting degraded read is asked-for rather than incidental), and satisfies the helper's own directive.
  ALTERNATIVE: inline a bare `t.Setenv("PORTAL_HOOKS_FILE", filepath.Join(t.TempDir(), "hooks.json"))`
  mirroring the doctor twin at `:55-56`. Same coverage, two fewer moving parts, but it re-hand-rolls
  staging in a task whose point is that staging has one shape — prefer the first.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/testhelpers_test.go:70-71` — the stated reason is false at the only fixture that denies:
  `cmd/hooks_pane_token_test.go:114` chmods the directory to `0o000`, which denies traversal, so the
  pre-created sidecar cannot be opened either and the mutation fails at the acquire, not the write
  (verified on this filesystem).
  OLD:
	// Created before any fixture denies writes to the directory, so a denied
	// mutation fails at the write rather than earlier at the sidecar's own open.
  NEW:
	// Created while the directory still permits it: a fixture that strips the
	// directory's permissions afterwards could not stage it.

- `internal/hooks/cleanstale_snapshot_test.go:101` — `StageStore` now stages the sidecar, so the
  mutation no longer stages anything; what it contributes is a live entry beside the stale one.
  OLD:
		// A mutation stages the sidecar, which no read creates.
  NEW:
		// A live entry beside the stale one, so the delete set is narrower than
		// the file.

- `internal/hooks/cleanstale_snapshot_test.go:123-124` — same falsified premise; the surviving reason is
  the sink-installation order, not the sidecar.
  OLD:
		// A mutation stages the sidecar, so the clean's own pre-read has a lock
		// to take and nothing it says can be mistaken for the deletion's lines.
  NEW:
		// A live entry beside the stale one, registered before the sink is
		// installed so nothing it says can be mistaken for the deletion's lines.

- `internal/hookstest/doc.go:6-7` — the inserted sentence left an unwrapped 105-column line among
  78-column neighbours.
  OLD:
// rather than authoring the file, its sidecar and its permissions itself. It lives outside _test.go so any
// package's tests can import it; production code must not.
  NEW:
// rather than authoring the file, its sidecar and its permissions itself. It
// lives outside _test.go so any package's tests can import it; production code
// must not.

NOTES:
- The two `t.Fatalf("stage the sidecar: %v", err)` messages at
  `internal/hooks/cleanstale_snapshot_test.go:103` and `:126` carry the same falsified claim as the
  comments above them; reword them (e.g. `"register the live entry: %v"`) in the same edit.
- Your report states `cmd/hooks_pane_token_test.go:112` "is the denial case that depends on that
  ordering". It does not — that case passes with or without the pre-created sidecar. The ordering rule is
  load-bearing in exactly one place, `StageStore`, which is what the task asked for; the env route's copy
  is decorative.
- `cmd/hooks_read_lock_test.go:56` hand-builds a sidecar-free store inline; it could equally state
  `hookstest.Staging{SidecarAbsent: true}`. Not required by the criteria (it stages no sidecar), but it
  is the last inline path-stage in that file.
- The two mutual-exclusion `t.Fatalf` guards in `StageStore` are untested. Testing them needs a
  `testing.TB` fake; not worth it, and the guards are cheap insurance rather than behaviour.
- Verified and holding: the sidecar-before-denial ordering is genuinely load-bearing in `StageStore`
  (staging after the chmod fatals; staging nothing fails at the acquire rather than the write, so either
  break fails `staging_test.go:56`), the -3 cmd delta is exactly the re-homed test with both assertions
  surviving verbatim, and the re-baselined comparison is meaningful for the first time.

## Attempt 2

ISSUES:
- `cmd/testhelpers_test.go:72` — the env route's sidecar is load-bearing but nothing pins it. Deleting
  `hookstest.CreateHooksSidecar(t, hooksFile)` from `hooksFileInTempDir` under a build overlay left the
  **entire `cmd` suite green** (`ok github.com/leeovery/portal/cmd 11.064s`). The same experiment against
  `StageStore` fails loudly — 12 tests across three packages. So route 1 carries the pin the retired
  `TestStagedHooksStoreSidecar` provided ("a fixture's sidecar state cannot silently stop being the thing
  that decides whether its reads degrade") and route 2 carries none: dropping that line silently
  re-degrades ~78 fixtures and returns `cmd/hooks_read_lock_test.go:80`'s baseline to the degraded read
  this task's own acceptance criterion was written to fix. Your report calls the env route's copy
  "decorative" — true of the *ordering*, but not of the sidecar's presence, which makes that framing
  likely to get the line deleted later.
  FIX: pin it inside the test whose baseline the criterion names. In `cmd/hooks_read_lock_test.go`, move
  `sink := logtest.Install(t)` above `want := runHookList(t)` (currently line 80) and assert the baseline
  did not degrade before taking the hold:
  ```go
  sink := logtest.Install(t)
  want := runHookList(t)
  if degraded := hookstest.UnlockedRecords(t, sink); len(degraded) != 0 {
      t.Fatalf("the baseline read degraded: %+v", degraded)
  }

  hookstest.HoldHooksSidecar(t, hooksFile)
  got := runHookList(t)
  ```
  The single sink then accumulates 0 records from the baseline and 1 from the held read, so the existing
  `hookstest.AssertDegradedRead(t, sink, "cli")` at line 89 still holds unchanged. This exact edit was
  verified under an overlay: it passes against the current helper, and fails with
  `the baseline read degraded: [... op:load-unlocked via:cli ...]` when the env route's sidecar line is
  removed.
  ALTERNATIVE: assert structurally instead — `os.Stat(hooksFile + ".lock")` immediately after
  `hooksFileInTempDir` in one test. Cheaper, but it pins only that the file exists, not that a read
  through the route actually takes the lock, which is the property criterion 4 is about. The first is
  recommended.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/hooks/cleanstale_snapshot_test.go:124-125` — the rewritten comment justifies the ordering by
  reference to "the deletion's lines", but this subtest's clean aborts on the enumeration error and
  deletes nothing; the records the sink must not hold are the `Set`'s own breadcrumb.
  OLD:
		// A live entry beside the stale one, registered before the sink is
		// installed so nothing it says can be mistaken for the deletion's lines.
  NEW:
		// A live entry beside the stale one, registered before the sink is
		// installed so its own breadcrumb is not counted against the aborted
		// clean, which must emit nothing at all.

NOTES:
- The specific hazard was swept: every class where the new default could have shifted another test's
  meaning was checked — directory-listing assertions (2 sites), `os.Stat(path+".lock")` absence
  assertions (3 sites), `hooks.json`-absent assertions, sink-emptiness assertions
  (`cleanstale_snapshot_test.go:144`, `store_shape_test.go:113`), and the sidecar-creation suite in
  `lock_test.go`. **No second hollowing.** Two vestigial `store.Set` calls in
  `cleanstale_snapshot_test.go` lost their original purpose but you re-justified both, and
  `AssertSidecarFree` at `:112` still probes a sidecar that staging created, so it cannot pass
  vacuously.
- The two adjacent fresh-install fixtures now express the same precondition two ways:
  `cmd/hooks_read_lock_test.go:55` takes a bare `t.TempDir()`, while `:96` routes through
  `StageStore(t, Staging{SidecarAbsent: true})`. Both correct, the second more self-describing; worth
  knowing they diverge if either is edited.
- `StageStore`'s two mutual-exclusion guards are unexercised — testing a `t.Fatalf` guard needs a
  `testing.TB` fake the package does not have. Acceptable as-is.
- Both cross-scope duplication clusters are already banked and confirmed genuine; no new bank items.
