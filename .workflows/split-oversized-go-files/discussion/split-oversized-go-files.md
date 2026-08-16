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

**The test rule was put as a fork and resolved toward the tree.** Two formulations were available: *a test file mirrors its production file* (simple, predictable, no judgment call — but it would force `open_targets_test.go` to absorb `open_targets_guard_test.go`, undoing a split the tree deliberately made), or *one purpose-named test file per behaviour area* (what the tree actually practises, finer-grained, at the cost of "behaviour area" being a judgment call the mirror rule avoids). The user took the behaviour-area form, on the principle that **file count is cheap** — ten small test files are preferable to one large one. That principle is worth carrying beyond the test rule: it means no lower bound is wanted, and a proposed split is never rejected on grounds of "that's too many files".

**The two-number split was then dropped, and the reason is the strongest empirical finding in the topic.** A background review flagged that the read-cost argument (tests looser) and the convention finding (the tree's test practice is finer-grained than production) point in opposite directions and were never reconciled *(resolves review-001 F5)*. Checking which one the tree supports produced a sharper result than the finding: **every test file over 1,000 lines is named after a production file — all fourteen of them** — and the production files they mirror are tiny. `cmd/theme_test.go` is 1,035 lines against an 87-line `theme.go` (12:1); `portal_saver_test.go` 3,772 against 386 (10:1); `state_commit_now_test.go` 1,163 against 172 (7:1); `resolution_test.go` 1,016 against 220 (5:1); `capture_test.go` 1,626 against 369 (4:1). Meanwhile every behaviour-named test file in the tree — `theme_panel_geometry_test.go`, `open_burst_run_test.go`, `edit_modal_state_machine_test.go`, `theme_panel_commit_load_test.go`, `hooks_register_test.go` — sits in the 300–997 band. Not one has run away.

So the mirror name is not merely a style we prefer against: **it is the mechanism that produces monsters.** A file-mirroring name is unbounded by construction — `theme_test.go` means "tests for theme.go", so everything about themes goes in and the name never signals that the file is full. A behaviour name is self-limiting: new behaviour, new file. `theme.go` stayed at 87 lines precisely because the production side already had a naming discipline the test side lacked.

That kills 2,000 as a standard: it would never trip a single behaviour-named file, because the practised convention peaks at 997. It only selects the four monsters, which makes it a sweep selector wearing a standard's clothes. The split collapses to **one tripwire at 1,000 for both file kinds** — just above the practised band on both sides (production tops at 775, behaviour-named tests at 997), so a well-named file essentially never trips and a trip means something. The read-cost insight is not discarded; it moves out of the number and into the **burden of rebuttal**: a 1,400-line table-driven test file trips and justifies itself in a sentence, a 1,400-line production file trips and probably cannot. Same detector, different burden of proof. Accepted cost: production loosens from 800 to 1,000, so a future 900-line production file will not trip where it would have — affordable given that cohesion is the actual rule and file count is cheap.

**Whether to keep a number at all was reopened and closed.** The user tested dropping the tripwire entirely in favour of naming and behaviour-bucketing rules alone, then argued back to keeping it: cohesion is hard to judge, and without a trigger nobody stops to check. That fixes what the number *is* — a procedural attention device, not a quality metric. It is there to make someone stop; it is never the thing being satisfied.

**A second detector was added because the line tripwire has a false-negative floor** *(resolves review-001 F1)*. The review's remaining objection to the numbers — that a threshold read off the current distribution ratifies the status quo by construction — survived the collapse to a single number, because 1,000 was derived the same descriptive way. The derivation itself holds up: it was calibrated against the *behaviour-named subset* of the tree, the part already following the rule being adopted, and used to catch the part that is not — that is calibration against a validated sample, not circularity. What does not hold up is relying on it alone. The reviewer's worked example: `internal/tmux/tmux.go` is 775 lines, 225 under the line and permanently compliant, and it wraps sessions, windows and panes, hook-key derivation, environment, server lifecycle, options and global hooks — comfortably more than five concerns, invisible to the detector forever. If the tripwire is the only thing that makes anyone stop, the cohesion rule only applies above 1,000 and C slides back toward A with a different number.

The second detector is name-shaped rather than line-shaped, and it generalises the finding that produced the test rule. The same pattern is visible on the production side within one package: every file in `internal/tui` is named for a behaviour or a component — `notice_band.go`, `burst_progress.go`, `loading_view.go`, `session_item.go`, `edit_modal.go`, `pagepreview.go`, all 284–433 lines — except `model.go`, which is named after a *type* and is eight times larger than the next biggest. `internal/tmux/tmux.go` is named after its package. Two production data points, so suggestive rather than proven, but pointing the same way the test-side evidence did.

This also supplies the operative content the cohesion half was missing — the objection that sank option B *(resolves review-001 F2)*. The first formulation was "the file can be named after the one behaviour it owns", and it was **too loose, in the over-indicting direction**: measured against the tree, **217 of 595 test files are mirror-named**, and 173 of those are under 500 lines. `internal/project/pathkey_test.go` beside a small `pathkey.go` is a perfectly good file. Read literally, a rule against mirror-naming indicts 217 files to fix about a dozen.

Mirror-naming is therefore a **correlate, not the property** — it correlates because a production filename is usually the broadest label available for that material. The property actually doing the work is whether the name *excludes* anything:

> A file's name must be specific enough to exclude things. If material that plausibly belongs in the package could be dropped into the file without contradicting its name, the name is a bucket label and the file will grow without bound.

It separates every case examined so far, in both directions:

| File | Could plausible new material go in without contradicting the name? | Verdict |
|---|---|---|
| `cmd/theme_test.go` (1,035) | yes — any theme-command test | bucket |
| `internal/tui/theme_panel_geometry_test.go` (997) | no — commit tests contradict it | bounded |
| `internal/tui/model.go` (3,448) | yes — any TUI code | bucket |
| `internal/tui/notice_band.go` (284) | no | bounded |
| `internal/tmux/tmux.go` (775) | yes — any client method | bucket |
| `internal/project/pathkey_test.go` | no | bounded, and mirror-named |

It also explains the mirror correlation rather than merely observing it: `theme` is a poor name not because it matches a filename but because nothing theme-related contradicts it. Accepted cost: "could this plausibly go in?" remains a judgment call — sharper than "one concern", but not mechanical, and two readers could differ on a borderline name. That is not avoidable; the tripwire is the mechanical half, and this is the half that needs a person.

An implication for `sweep-scope`, not decided here: the sweep's candidate set is no longer "everything over the line" — a file can fail the exclusion test while sitting under 1,000.

**The rebuttal was given a durable home, because without one the shape is not C at all** *(resolves review-001 F3)*. Presumption-and-rebuttal is the entire practical difference between this shape and a plain ceiling: remove the ability to record a rebuttal and the tripwire behaves as a hard limit; leave the rebuttal implicit and it behaves as the pure cohesion rule already rejected for decaying. The document had defined the tripwire to the line and left "has to justify itself" undefined.

The worked case: `internal/capture/fixtures.go` is 762 lines and grows by construction — every new TUI screen contributes a fixture. Say it reaches 1,150. It trips, someone establishes it genuinely is one concern (table-shaped fixture definitions, the "repetitive by construction" case), and moves on. Six weeks later an agent opens it, sees 1,150 lines, and redoes the whole analysis. Then the next one does. **That re-litigation is precisely the context cost the standard exists to remove**, so the rebuttal must be durable.

The standard's own rationale picks the location: **in the file**. Any other home — an allow-list, a doc, a commit message — forces a second read to answer a question raised by the first; in the file, the answer rides the read already being made. This fits the surviving convention rather than reversing the comment-strip sweep: 90 of 870 Go files still carry a leading doc comment.

The justification **states the concern the file claims to own**, so a later reader can falsify it by reading rather than having to trust it. A bare "large on purpose" rots silently — the file gains a second concern and the old note keeps vouching for it.

**On drift detection, the governing principle is: record nothing that must be updated when the file changes.** Candidates weighed — a content hash self-invalidates on every edit including a typo in the marker itself (and cannot include itself without excluding its own line), a line count is a poor proxy in both directions (a file can absorb a whole new concern at net-zero line change, or grow 200 lines of the same table and look drifted when nothing changed), and mtime is not committed and is reset by checkout. All three share the fatal property that they must be maintained, and across a set of perhaps three to eight marked files over years they will not be — an unmaintained staleness marker is worse than none, because it lies.

What survives are fields that are true at tagging time and never need touching: the **date** and the **commit verified against**. Both stay honest forever — they assert "this claim was checked against this state", and going stale is information rather than an error. The commit reference is strictly better than a hash: it does not self-invalidate, and it yields the actual diff (`git diff <sha>..HEAD -- <file>`) rather than a boolean. The date is nearly redundant against the commit but is kept because it reads at a glance with no tooling.

Adopted form — a greppable prefix (no `portal:` comment pragma exists in the tree today, so the namespace is free):

```go
// portal:oversized 2026-08-16 @ f238265 — Fixture definitions only; each TUI
// screen contributes one. No logic lives here.
```

**The production/test classification was then removed from the rule altogether** *(resolves review-001 F4)*. The review identified a third category the split missed: code that is test-only in purpose but production-suffixed in filename — `internal/portaltest`, `themetest`, `spawntest`, `restoretest`, `tmuxtest`, `portalbintest`, `transienttest`, `logtest`, `sourceguardtest`, plus `internal/capture` and `cmd/capturetool`. Go can only distinguish the kinds by the `_test.go` suffix, which none of these carry.

Collapsing to one number had already removed most of the classification's work; only the burden of rebuttal still keyed to it. That keying does not survive scrutiny either: what makes a rebuttal easy is the *claim*, not the suffix. "Each TUI screen contributes one fixture, no logic lives here" either holds or it does not, and the filename has no bearing on whether it does. The `_test.go` distinction was only ever a proxy for "probably repetitive by construction", and once a trip is adjudicated on a written claim the proxy has nothing left to do.

The practical stake confirmed the dissolution rather than a new rule: across all eleven of those packages only `internal/capture/fixtures.go` (762) is anywhere near the line — next largest `portaltest/fingerprint.go` at 300, then `capture/swatch.go` at 212 — so a third file class would have existed to govern a single file whose claim already stands on its own. The standard therefore names no categories: one tripwire, one exclusion test, one rebuttal mechanism. Accepted cost: nothing in the rule tells a contributor that a table-driven test file gets an easy ride, so someone may argue harder for one than needed; if that reassurance is wanted it belongs as a line of guidance in `CLAUDE.md`, never as a category in the rule.

A hook for `claude-md-and-enforcement`, not decided here: the marker can serve as its own allow-list — a guard walking `.go` files and flagging anything over 1,000 without the marker needs no separate list, and grep is the enumeration. That also answers the objection that in-file justifications are not enumerable.

### Decision

*(converging — shape, the single tripwire, the two detectors, and the test-side rule agreed; what "justify itself" concretely means remains open)*

- **Shape: C.** The standard is a cohesion rule — one concern per file within a package — with a line count acting as a tripwire that presumes a violation and demands justification, never as a violation in itself.
- **Rationale is context cost, not line count.** A file too large to read in one pass forces chunked reads and burns context; a production file usually must be understood before editing, so it pays that cost in full, while a test file is usually appended to and reachable surgically.
- **Two detectors, not one.** A line tripwire and a name-exclusion test, working at different scales. The tripwire catches files grown past being readable in one pass; the exclusion test catches files holding more than one concern at *any* size — the majority of the tree, and the whole range the tripwire is blind to.
- **The exclusion test is the operative form of the cohesion rule.** *A file's name must be specific enough to exclude things. If material that plausibly belongs in the package could be dropped into the file without contradicting its name, the name is a bucket label and the file will grow without bound.* This is what makes C more than a ceiling: it bites below the line, where the tripwire is silent (`internal/tmux/tmux.go`, 775 lines, seven-odd concerns, permanently compliant with the number). Mirror-naming is a correlate of failure, never the test itself — 217 of 595 test files are mirror-named and most are perfectly well-bounded.
- **One tripwire, 1,000 lines, every Go file.** No production/test classification exists in the rule at all. Set just above the band Portal's own well-named files already occupy (production tops at 775; behaviour-named test files at 997), so a properly-organised file essentially never trips. The earlier two-number split (production 800 / tests 2,000) is dropped: 2,000 would never fire on anything the tree does, and 200 lines of separation did not earn a second number.
- **The read-cost asymmetry lives in the strength of the written claim, not in a file category.** A repetitive-by-construction file justifies itself in a sentence; a file holding several concerns cannot. That difference is adjudicated on the claim the marker states — it is not keyed to the `_test.go` suffix, or to any classification, and the rule names none.
- **The number is a procedural attention device, not a quality metric.** Its job is to make someone stop and check, because cohesion is hard to judge and otherwise nobody does. It is never the thing being satisfied — a file under 1,000 is not thereby well-organised.
- **The test-side rule is behaviour-area, not mirror.** A test file is named for the behaviour it covers, not derived from a production filename — one purpose-named test file per behaviour area, which is what the tree already practises and is finer-grained than the production file layout. This is load-bearing rather than cosmetic: a mirror name is unbounded by construction (nothing about "tests for X.go" ever says the file is full), while a behaviour name is self-limiting. Every test file in the tree over 1,000 lines is mirror-named; none of the behaviour-named ones has run away.
- **A trip is rebutted in the file, with a `// portal:oversized {date} @ {commit} — {claim}` marker.** The claim names the single concern the file owns, so a later reader can falsify it by reading. The location follows from the standard's own rationale: a rebuttal stored anywhere else forces a second read to answer a question raised by the first.
- **The marker records only fields that never need updating** — date and verified-at commit. No content hash, line count, or mtime: each must be maintained on every edit, and across a handful of files over years it will not be. A stale marker that lies is worse than no marker; a date and a commit that simply age are honest, and yield the diff on demand.
- **File count is cheap.** Many small files beat one large one; there is no lower bound and no "too many files" objection to a proposed split.

---

## Summary

### Key Insights

*(to be captured as the discussion develops)*

### Open Threads

*(to be captured as the discussion develops)*

### Current State

*(to be captured as the discussion develops)*
