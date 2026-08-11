# Analysis Report: theming-system (Cycle 7)

## Stats

- Total findings: 19
- Deduplicated findings: 19
- Proposed tasks: 15

## Summary

Three agents ran a full fresh pass over the theming implementation and reported 19 findings — 10 duplication, 3 standards, 6 architecture — with no two agents reporting the same issue, so nothing collapsed on dedup. Six are medium: one real conformance gap (the resize-forced-close flash is raised without a `flashTickCmd`, so two of §14A's six "transient" theme signals never auto-clear), two genuine parallel implementations in production code (`commitConstant`/`commitSlot`, and the slot→shipped-default pairing written out three times), a public seam whose only production effect is a log emission, a hand-authored duplicate of the union assembly inside the capture fixtures, and the largest copy-paste surface the topic introduced — the "make this path unreadable" test fixture re-authored ~14 times across four packages, already drifted into three incompatible root policies where several sites degrade to vacuous assertions.

Three pairs of findings were grouped into single tasks: the `ThemeSource` slot-collapse duplication and the `ResolveSlot` seam-honesty finding (same seam, same files, conflicting edits if split); the panel-rule-row duplication and the §9.1 header-order documentation finding (same function); and the deny-read fixture consolidation and the canvas-valued theme-file fixture (same pattern — `internal/themetest` is the declared owner and both were hand-rolled at call sites). Every remaining low-severity finding is either a multi-site cluster (5+ occurrences of one idiom), a stated project-convention violation, or a claim in-source that the code does not support, so none met the discard bar.

## Discarded Findings

- None — no finding was low-severity *and* isolated *and* free of a stated correctness or convention argument. The three findings not given their own task (`ThemeSource` collapse duplication, the §9.1 header-order note, the canvas-valued fixture) were folded into grouped tasks rather than dropped.
