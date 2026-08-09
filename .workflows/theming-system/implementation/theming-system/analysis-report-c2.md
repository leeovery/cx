# Analysis Report: theming-system (Cycle 2)

## Stats

- Total findings: 16 (duplication 7, standards 4, architecture 5)
- Deduplicated findings: 15
- Proposed tasks: 12

## Summary

Cycle 1's extractions hold: all three agents report the production theme code as tightly consolidated, the loader/assembler/resolution split clean, the leaf and logging constraints intact, and the implementation tracking the specification closely enough that the standards agent found no conformance gaps at all. One overlap was merged — duplication and architecture independently found the same finding from opposite directions (the "which persisted theme keys are in force" rule authored once in `internal/theme` and again in `cmd/doctor`, unexported so doctor structurally could not compose on it, with no cross-surface parity test); that convergence plus its correctness stake (the panel and doctor able to disagree about which slug is live) makes it the cycle's one high-severity task. Two further merges collapsed the two production-comment standards findings into one sweep over the same fifteen files, and the two `Nomination` findings (a constructor that accepts a half-empty pair, a panel field that goes write-only after the gate) into one contract task.

The remaining volume is test scaffolding that regenerates because the natural home sits across an in-directory `package X` / `package X_test` split: a built-in-loading helper written seven times across five packages (two copies already drifted into collapsing the two failure classes the build-time guarantee exists to discriminate), an SGR-probe derivation written eight times with six in one package and three different return shapes, three near-identical no-config-access fixture tests that also re-implement the theme-file body cycle 1 single-sourced into `internal/themetest`, and two repeated panel-model construction sequences across nine files. `internal/themetest` already exists from cycle 1 with `Lines`/`WithValue`/`WithoutKey`/`Write`, so it is the ready destination for the cross-package pieces.

## Discarded Findings

- The panel's column band is wider than the specification's stated ~24–30 columns (standards, low) — discarded for the second cycle running, on the same grounds cycle 1 recorded: the widening to `themePanelPreferredWidth = 34` was a visual-gate outcome, the in-source note already records the reason (badges crowded the right edge and `set as light` ran to the border at 30), and the finding's own recommendation is to raise a spec amendment to §9.1/§9.5/§9.8. That is a design-artifact edit owned by the spec process, not an implementation improvement — the finding proposes no code change, and nothing in the implementation depends on the old number.
