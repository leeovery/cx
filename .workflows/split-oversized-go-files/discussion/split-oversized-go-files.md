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

## Summary

### Key Insights

*(to be captured as the discussion develops)*

### Open Threads

*(to be captured as the discussion develops)*

### Current State

*(to be captured as the discussion develops)*
