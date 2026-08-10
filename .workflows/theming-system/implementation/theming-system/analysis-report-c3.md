# Analysis Report: theming-system (Cycle 3)

## Stats

- Total findings: 18 (duplication 9, standards 4, architecture 5)
- Deduplicated findings: 17 (after grouping; 0 cross-agent duplicates)
- Proposed tasks: 14

## Summary

No high-severity findings and no cross-agent duplicates this cycle — the three agents
covered disjoint ground, which is itself a signal that the obvious problems are gone.
Production code is close to clean: the duplication pass found no repeated logic at all
across the 35 new production files, and the standards pass reports unusually close spec
conformance (the rejection ladder, reason vocabulary, per-slot fallback, prefs RMW, the
closed `theme` log catalogue, the panel state machines and the startup-canvas-hex
anchoring all match what was decided). Four medium findings drive the substantive work:
three test-layer single-sourcing tasks where separate task executors independently
authored the same scaffolding (capture panel-frame parsers and registration guards, the
prefs abort-on-undecodable case table which has already drifted apart, and repo-walking
guard machinery re-implemented beside a shared helper a sibling guard already uses), plus
one boundary-typing fix where `theme.ResolveSetting` takes three interchangeable strings
where the typed `RawKeys` value already exists — the same light/dark inversion hazard the
package went to real lengths to close one layer up.

The remaining ten tasks are low-severity but cluster into three coherent groups kept for
that reason: architecture refinements (a doctor type carrying one producer's dedup
identity, a log attr recovered by parsing rendered user-facing copy, a field maintained
for no reader, a seam named for one of its two responsibilities), spec/doc conformance
loose ends at feature close (the panel width ladder sitting outside the decided column
band, a missing bold on the panel cursor row, §13.2's un-performed start-of-feature
capture deletion, and `internal/themetest` missing from CLAUDE.md's inventory), and two
smaller test/harness single-sourcing items in `cmd` and `internal/capture`.

Two tasks (11, 12) change rendered output and carry a visual gate; task 13 deletes
committed artifacts and is scoped to the pre-feature set only, leaving this feature's own
frames for the separate sign-off act §13.2 defines.

## Discarded Findings

- The dark/light theme pair restated as a literal at 102 assertion sites (duplication,
  low) — a mechanical 102-site sweep across 31 files for zero behavioural risk. The
  finding itself states no site can silently diverge in meaning, only in membership, and
  its own recommendation is to do it opportunistically rather than as a sweeping edit. The
  churn-to-value ratio does not justify a task, and it does not cluster with any other
  finding.
