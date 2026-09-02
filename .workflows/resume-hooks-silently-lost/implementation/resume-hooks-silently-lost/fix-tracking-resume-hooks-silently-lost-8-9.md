## Attempt 1

ISSUES:
- `/Users/leeovery/Code/portal/cmd/doctor_fix_hook_prune_report_test.go:44` — the only assertion of the new line reads its own phrase out of the production map (`"Skipped stale hook prune: "+skippedPrunePhrases[skipReasonSweepFailed]`), so the string the user actually sees is unpinned: a typo, a reword, or someone setting the map value to the raw `sweep-failed` slug all ship with the suite green. Every other reason's copy is pinned as a literal in `cmd/doctor_stand_down_copy_test.go:44-80`, and the new reason is deliberately excluded from that table (`notStandDownReasons`, line 36) and therefore also from its "renders no raw reason slug on either surface" check (line 218) — so nothing anywhere holds `"the sweep could not complete"` except the two production maps.
  FIX: assert the whole literal in the failure subtest — `assertSkippedPruneLine(t, outBuf.String(), "Skipped stale hook prune: the sweep could not complete")` — and, in the same subtest, add the raw-slug guard the excluded reason lost: fail if `phraseFor(skippedPrunePhrases, skipReasonSweepFailed)` equals or contains `skipReasonSweepFailed`.
  ALTERNATIVE: extend `standDownCopyCases` with a `sweep-failed` case and gate the table's exit-code and not-evaluable subtests on a per-case flag. That restores full table coverage but complicates a guard whose whole subject is stand-downs, for a reason that is not one — reviewer recommends the literal-plus-slug-guard fix above.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `/Users/leeovery/Code/portal/cmd/run_hook_stale_cleanup.go:13-14` — the diff added a member to this const block that is neither a decline nor carried by any logged `reason` attr, falsifying both sentences.
  OLD: `// The reasons a cycle declines to run. Both the logged reason attr and the`
  `// caller-facing rendering read them, so the two cannot drift.`
  NEW: `// The reasons a cycle removed nothing: the ones it declines to run under,`
  `// which the logged reason attr also names, and the failure only the`
  `// caller-facing rendering reports. Both read this set, so the two cannot drift.`

NOTES:
- `notEvaluableDetails[skipReasonSweepFailed]` (`cmd/run_hook_stale_cleanup.go:69`) is unreachable: the read-only diagnosis renders only stand-down reasons, and a failed sweep never becomes one. It exists solely because `TestStandDownPhraseCoverage` requires a phrase in both maps for every declared reason — correct under the task's step 2, but a maintainer will reasonably wonder when it prints.
- "Skipped stale hook prune: the sweep could not complete" is mildly self-contradictory (the sweep ran; it did not skip). The task's step 2 explicitly mandates routing through `skippedPrunePhrases`, so the shared prefix is the sanctioned trade, and the sentence still tells the user the truth that matters.
- `hookstest.Staging{WritesDenied: true}` is exactly the documented fixture for this condition ("takes its lock and reads cleanly but fails at the temp create") — a good fit rather than a synthesised error, so the test exercises the real error class rather than a stub.
