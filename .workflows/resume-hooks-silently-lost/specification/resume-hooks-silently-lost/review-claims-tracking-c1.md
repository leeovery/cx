# Review Tracking: Resume Hooks Silently Lost - Claims Verification

## Findings

### 1. `@portal-id` consumer grep spans 7 files, not 8

**Source**: Tree measurement — `grep -rn "PortalIDOption\|@portal-id\|PortalID" internal cmd --include="*.go" | grep -v _test.go`
**Category**: Enhancement to existing topic
**Affects**: §7.1 Why it goes, and why now

**Details**:
The specification claims, verbatim:

> Every non-test consumer of `@portal-id` exists to build the hook key and nothing else (`grep -rn "PortalIDOption\|@portal-id\|PortalID" internal cmd --include="*.go" | grep -v _test.go` → 21 lines across 8 files).

Running the spec's own recorded command:

```
$ grep -rn "PortalIDOption\|@portal-id\|PortalID" internal cmd --include="*.go" | grep -v _test.go | wc -l
      21
$ grep -rln "PortalIDOption\|@portal-id\|PortalID" internal cmd --include="*.go" | grep -v _test.go
internal/state/capture.go
internal/state/schema.go
internal/tmux/tmux.go
internal/restore/session.go
internal/session/create.go
internal/session/quickstart.go
cmd/run_hook_stale_cleanup.go
$ grep -rln "PortalIDOption\|@portal-id\|PortalID" internal cmd --include="*.go" | grep -v _test.go | wc -l
       7
```

The line count (21) holds; the file count is 7, not 8.

The two file sets in play differ rather than one simply being short by one, which is what makes the figure worth stating correctly:

- The grep's 7 files include `cmd/run_hook_stale_cleanup.go`, whose only hit is the `AllPaneLister` doc comment describing the `<@portal-id or session_name>:window.pane` form (`cmd/run_hook_stale_cleanup.go:10`) — a comment, not a consumer, and not a row in §7.2's removal table.
- §7.2's removal table has 7 rows, and its `cmd/state_migrate_rename.go` row is *not* a grep hit: that file carries no `@portal-id` or `PortalID` literal at all (it rewrites keys by `<oldName>:` prefix, `cmd/state_migrate_rename.go:57`).

So the grep is not a count of the removal table; the union of the two sets is 8 files, which is the most likely origin of the figure.

The claim is load-bearing: it is the measurement §7.1 offers for the removal footprint that §7.2's table then enumerates, and a reader reconciling the two against the tree finds that neither set has 8 members.

The source material does not carry this claim — the investigation names the individual sites (`.workflows/resume-hooks-silently-lost/investigation/resume-hooks-silently-lost.md:693-700`) but records no grep and no count (`grep -n "21 lines\|8 files\|7 files" …/investigation/resume-hooks-silently-lost.md` → no matches). Spec-only, so the fix belongs to the specification.

**Current**:
> Every non-test consumer of `@portal-id` exists to build the hook key and nothing else (`grep -rn "PortalIDOption\|@portal-id\|PortalID" internal cmd --include="*.go" | grep -v _test.go` → 21 lines across 8 files). A token-only pane key makes all of it dead at once.

**Proposed Change**:
> Every non-test consumer of `@portal-id` exists to build the hook key and nothing else (`grep -rn "PortalIDOption\|@portal-id\|PortalID" internal cmd --include="*.go" | grep -v _test.go` → 21 lines across 7 files, one of them a doc comment in `cmd/run_hook_stale_cleanup.go`). A token-only pane key makes all of it dead at once.

**Resolution**: Pending
**Notes**:

---
