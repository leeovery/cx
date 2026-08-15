# Discussion: Split Oversized Go Files

## Context

Portal is to set its own file-organisation standard, record it in `CLAUDE.md`, and split the oversized Go files along their concern seams — standard and sweep landing in the same release.

**The inherited position from discovery.** The seed's case is that file size here is an active cost, not an aesthetic complaint, and the evidence is agent-read cost: during the theming-system feature `internal/tui/model.go` was the single most-read artifact of the whole implementation — 327 reads across 85 distinct implementation subagents, each full pass roughly three chunked reads at agent tooling's 2,000-line window. Hub-file gravity also concentrates merge risk: any future parallelism in implementation is dead on arrival while one file is the junction for half the feature surface. Go makes the mechanical half free — files inside a package split with no import, caller or test changes — so the work is choosing the seams. The existing `theme_panel.go` / `theme_panel_commit.go` / `theme_row.go` siblings are the working pattern already in the tree.

**The user's framing extended the seed twice.** First, the outcome should align with a recognised convention rather than an arbitrary threshold, because the current sizes serve neither a human nor an agent reader. Second, it must not stop at a refactor: the standard goes into `CLAUDE.md` so the discipline holds for future work, with trimming and refactoring done as far as is practical, all in the same release. The user was explicitly unsure about a size discipline as such while judging it worth having.

**A premise correction was accepted into the framing rather than resolved.** Go has no file-size convention to align with — the standard library ships multi-thousand-line files without apology (`net/http/server.go` ~3,900 lines, `go/types/expr.go` comparable). What Go does conventionalise is *one concern per file within a package*: a cohesion rule, not a line count. The standard this work sets is therefore Portal's own, argued from the measured agent-read cost rather than borrowed from the language, and the honest target is files a reader can hold rather than conformance to an external number. Where exactly that lands — a hard ceiling, a soft guideline, or a cohesion rule with size as a symptom — was left open for this phase.

**Shape was settled in discovery and is not reopened here.** The splits are the same seam-choosing exercise repeated over a handful of files rather than independent concerns needing their own cycles, so no topic multiplication. The work-as-standard reading was considered and rejected: the standard alone would terminate at specification, and the refactor must land in the same release, which needs the work to reach implementation.

### Current state of the tree (measured 2026-08-15)

The seed's headline numbers have moved — `model.go` was 5,467 when the idea was logged on 2026-08-09 and is 3,448 now, the drop coming from the comment-strip sweep rather than any split. `model_test.go` is 7,116 against the seed's 7,766. No concern split has happened.

Largest files:

| File | Lines |
|---|---|
| `internal/tui/model_test.go` | 7,116 |
| `internal/tmux/portal_saver_test.go` | 3,772 |
| `cmd/open_test.go` | 3,581 |
| `internal/tui/model.go` | 3,448 |
| `internal/tmux/tmux_test.go` | 3,211 |
| `cmd/state_hydrate_test.go` | 1,849 |
| `internal/state/capture_test.go` | 1,626 |
| `cmd/doctor_test.go` | 1,495 |
| `internal/hooks/store_test.go` | 1,425 |

Distribution matters more than the top of the list. **Production: 275 files, 5 over 500 lines, 1 over 800, 1 over 1,000** — `model.go` at 3,448 is a lone outlier, the next largest being `internal/tmux/tmux.go` at 775 and `internal/capture/fixtures.go` at 762. **Tests: 595 files, 68 over 500, 25 over 800, 14 over 1,000.** The production side is already close to whatever standard we set; the test side is where the mass sits.

### References

- `.workflows/split-oversized-go-files/seeds/2026-08-09-split-oversized-go-files.md` — the originating inbox idea
- `.workflows/split-oversized-go-files/discovery/sessions/session-001.md` — discovery session log (the carrier)
- `CLAUDE.md` — where the standard is to be recorded
- `internal/tui/theme_panel*.go` — the existing in-tree pattern for panel-scoped concern files

---

## Standard Shape

### Context

Discovery left open what form the standard takes — a hard ceiling, a soft guideline, or a cohesion rule with size as a symptom — having established that Go itself offers nothing to borrow. Everything downstream hangs off this: which files the sweep touches, what goes into `CLAUDE.md`, and whether anything can mechanically enforce it.

### Options Considered

**A — Line ceiling.** "No Go file over N lines."
- Pros: checkable, guard-testable, unambiguous.
- Cons: blind to *why* size hurts. Flags a table-driven test file (plausibly one coherent concern) identically to `model.go` (nine concerns in one file). Invites the worst fix — splitting mid-concern to satisfy a number.

**B — Pure cohesion rule.** "One concern per file within a package."
- Pros: the actual Go convention; names the real cost driver.
- Cons: judgment-only and un-auditable, so it decays. The evidence is in the tree — `model.go` reached 3,448 lines in a package that already had `theme_panel.go` / `theme_panel_commit.go` / `theme_row.go` demonstrating the pattern.

**C — Cohesion rule, line count as a tripwire.** The rule is cohesion; the number is a review trigger, not a violation. A file past N is presumed to hold more than one concern and has to justify itself.
- Pros: survives the objection discovery accepted (`net/http/server.go` at 3,900 lines is fine because it *would* justify itself) while still giving a reviewer or an agent something mechanical to act on. The only one of the three with an obvious enforcement story in this repo's idiom, given the ~25 source-guard tests already in the tree.
- Cons: two moving parts to state rather than one.

### Journey

**The distribution reframed the problem before the options were weighed.** Measured at session time: production is 275 files with five over 500 lines and exactly one over 800 — `model.go` at 3,448. The next largest are 775 (`internal/tmux/tmux.go`), 762 (`internal/capture/fixtures.go`), 735 (`cmd/open.go`), 518 (`cmd/doctor.go`), then nothing until 447. **There is no production file between 775 and 3,448.** Tests are 595 files with 68 over 500, 25 over 800, 14 over 1,000 — a smooth decay with no cliff at that scale, but a stark one much higher: 7,116 · 3,772 · 3,581 · 3,211, then a gap down to 1,849 and a smooth tail. Four monsters, then twenty-one merely-large files.

So a line rule binds today on `model.go` plus a dozen-odd test files. The seed's justification — measured agent-read cost on a production hub file — does not transfer unexamined to a 3,200-line test file.

C was taken directly, not as a compromise between A and B: A alone produces bad splits, B alone produces no splits, and the tree is the evidence for the second.

**On the number, the position moved once.** The opening position was a single tripwire at 800 for everything, read off the production cliff. It moved to a split — production 800, tests 2,000 — on the argument that a line of test code carries less information about concern-count than a line of production code, because table-driven cases and fixture setup are repetitive by construction. If the number's job under C is to *predict* "this file probably holds more than one concern", it should be calibrated to how well a line predicts that, and it predicts worse in tests. 2,000 is independently anchored to the agent read window (a file an agent sees whole in one pass) and sits in the test data's own gap; what it selects is exactly the four files the seed named on intuition, before anyone counted.

**The user's framing replaced the rationale with a better one.** The cost of a big file is not line count as such — it is that reading it forces chunked reads and burns context. And the two file kinds are read differently: a production file must usually be understood before it can be edited, so its full cost lands every time; a test file is usually appended to, and can be reached surgically by grep or offset read, so its full cost rarely lands at all. That is a stronger justification for the prod-strict / test-loose split than the line-prediction argument, and it supersedes it — the split now rests on how each file kind is *read*, not on how well a line predicts concern-count.

**The self-healing hypothesis was checked and holds for one file in four.** The user's observation was that splitting a production file drags its test file along, so the test side may self-heal wherever a split is warranted. Against the tree: only `model_test.go` (7,116) has an oversized production counterpart (`model.go`, 3,448). The other three monsters do not — `internal/tmux/portal_saver_test.go` (3,772) pairs with a 386-line production file, `cmd/open_test.go` (3,581) with 735 lines, `internal/tmux/tmux_test.go` (3,211) with 775. Ratios of 10:1, 5:1, 4:1. Self-healing covers `model_test.go` and nothing else, so a test-side rule is not redundant.

**The larger finding: the convention already exists in the tree and is already practised.** `cmd/open_test.go` (3,581 lines, 68 test funcs) sits alongside fifteen-plus purpose-named siblings — `open_burst_run_test.go`, `open_theme_construction_test.go`, `open_surfaces_test.go`, `open_multitarget_test.go`, `open_targets_test.go`, `open_nocolor_test.go`, `open_fatal_test.go`, `open_domain_routing_test.go`, and more. `internal/tmux` is the same: `portal_saver_test.go` (3,772) sits beside `portal_saver_lifecycle_events_test.go`, `portal_saver_integration_test.go`, `portal_saver_endstate_integration_test.go`, and the `hooks_*` family splits six ways. Every recent feature dropped its tests into a purpose-named sibling. **The monsters are pre-convention residue** — the core that predates the habit and that nobody goes back to drain, because nobody edits old tests.

This means the standard is not being invented. It is being written down, and then back-filled onto the files that predate it — which is exactly the "Portal's own standard, derived not borrowed" framing discovery asked for.

It also shows the tree's test convention is *not* a mirror of production files: it is one purpose-named file per behaviour area, and it is finer-grained than production. `cmd/open_targets.go` is 75 lines and carries two test files (`open_targets_test.go`, `open_targets_guard_test.go`). The monsters violate that convention by being named for a *file* rather than for a behaviour.

### Decision

*(converging — shape and prod/test split agreed; the exact formulation of the test-side rule is open)*

- **Shape: C.** The standard is a cohesion rule — one concern per file within a package — with a line count acting as a tripwire that presumes a violation and demands justification, never as a violation in itself.
- **Rationale is context cost, not line count.** A file too large to read in one pass forces chunked reads and burns context; a production file usually must be understood before editing, so it pays that cost in full, while a test file is usually appended to and reachable surgically.
- **The tripwire differs by file kind** for that reason: stricter for production, looser for tests. Working values: production 800 (the observed cliff in Portal's own tree), tests 2,000 (the agent read window; selects exactly the four seed-named monsters).

---

## Summary

### Key Insights

*(to be captured as the discussion develops)*

### Open Threads

*(to be captured as the discussion develops)*

### Current State

*(to be captured as the discussion develops)*
