## Attempt 1

ISSUES:
- `cmd/doctor_stand_down_phrase_guard_test.go:198-202` — the new guard compares `skippedPrunePhrases`' *inline* literals against `notEvaluableDetails`' *inline* literals, so it enforces "neither map authors the same literal inline", which is weaker than the rule its own doc comment at `:160-165` states ("a phrase both vocabularies use must be written once and composed into each"). A half-lifted state passes it: lift a phrase to a const, compose it into one map, and re-author the same words inline in the sibling — the guard sees no shared inline literal, the rendered lines are unchanged so the copy suite stays green, and re-wording the const then moves only one surface. That is the exact silent drift this task removes, and it is a plausible partial version of this very task (lift one map, forget the other). It does not pass vacuously — the `found` check fatals if either map literal disappears — the reach is the problem, not the wiring.
  FIX: compare each vocabulary's inline literals against the *sibling's rendered values* rather than the sibling's inline literals — the map is in scope at test time:
  ```go
  runtime := map[string]map[string]string{
      "skippedPrunePhrases": skippedPrunePhrases,
      "notEvaluableDetails": notEvaluableDetails,
  }
  for i, name := range vocabularies {
      sibling := vocabularies[1-i]
      for _, phrase := range literals[name] {
          for _, rendered := range runtime[sibling] {
              if rendered == phrase {
                  t.Errorf("%s authors %q inline while %s renders the same phrase; lift it into a const both compose from",
                      name, phrase, sibling)
              }
          }
      }
  }
  ```
  This keeps the source-level half of the rule (an inline literal is what it forbids, so the shared-const arrangement is not itself flagged) while closing the half-lifted hole. The reviewer ran it against the current source under `-overlay`: green, with no false positive on `"hooks.json is locked"` vs `"hooks.json is locked (not evaluable)"` or on the `restoreStandDownPhrase + " (not evaluable)"` binary expression. The doc comment at `:160-165` then becomes true as written and needs no edit.
  ALTERNATIVE: resolve identifiers to their const values during the AST walk and compare fully-resolved value sets. Strictly source-level, but it re-implements constant resolution for a rule the runtime map already answers, and would flag the legitimate shared-const arrangement unless it also tracked *how* each value was written. Reviewer recommends the runtime-sibling comparison.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `cmd/doctor.go:292-294` — cardinality claim of the form code-quality.md names explicitly ("the only site that…"), falsified by any additive branch that builds a `checkResult` directly; the second clause ("no branch can author the copy") asserts an enforcement nothing performs.
  OLD:
  // staleHooksNotEvaluable reports a stand-down in the diagnosis's register. It is
  // the check's only route to a not-evaluable line, so no branch can author the
  // copy for a reason the vocabulary already words.
  NEW:
  // staleHooksNotEvaluable reports a stand-down in the diagnosis's register, so a
  // branch reporting one names its reason and the vocabulary words the copy.
- `cmd/doctor_stand_down_copy_test.go:295-298` — "the only way" is false as well as an exclusivity claim: a source guard over the check's own branches would separate the same two cases. The clause carrying the real content survives the trim.
  OLD:
  // rewordNotEvaluableDetail re-words one vocabulary entry for the test's
  // duration. Watching a branch follow the re-worded entry is the only way to
  // tell a branch that renders through the vocabulary from one that authors the
  // same words inline: an assertion on today's literal cannot separate them.
  NEW:
  // rewordNotEvaluableDetail re-words one vocabulary entry for the test's
  // duration. Watching a branch follow the re-worded entry tells a branch that
  // renders through the vocabulary from one that authors the same words inline:
  // an assertion on today's literal cannot separate them.

NOTES:
- All five verification asks came back clean. (a) The runtime re-wording test is genuine, proved by mutation: inlining each branch's literal under `-overlay` failed the matching subtest with the expected message. It varies the vocabulary rather than the branch, which is exactly the assertion the key-binding guards structurally cannot make, and the unreadable-store staging means the failed-`Load` branch is genuinely reached rather than short-circuited. (b) The new guard does not pass vacuously — its `found` check fatals if either map literal disappears; the reach is the problem. (c) Keeping one reason code for both conditions is correct and the reviewer would have made the same call: the reason enum is a rendering key that doubles as a log attr, not an error taxonomy, and a seventh const would need enumerating in `skipReasons`, a phrase in *both* vocabularies, and an exclusion from the copy table's count guard — three pieces of machinery for a distinction with no copy and no log line of its own. (d) Lifting the third phrase was required by criterion 1, not scope creep — both maps authored it inline. (e) Rendered output is byte-identical; every pre-existing pinned line is unchanged and green.
- Spec conformance verified against §5.4 and the 2026-09-01 corrigenda: all seven governed phrases preserved byte-for-byte, and the "sweep-failed declined nothing" distinction is untouched.
- `sweepFailedStandDownPhrase` is named for a condition the copy suite records as *not* a stand-down. The name is consistent with its four siblings in the same const block, which is the stronger pull; flagged so the inconsistency is known rather than discovered.
- The integration-lane report is consistent with the change's shape: the diff is confined to `cmd`, and `cmd/bootstrap` — where the two composite daemon tests fail — does not import it. The reviewer did not re-run those fixtures (the change cannot reach them) but compile-checked the integration configuration clean.
