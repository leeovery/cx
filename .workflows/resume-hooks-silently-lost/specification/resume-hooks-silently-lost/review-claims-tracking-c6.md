# Review Tracking: Resume Hooks Silently Lost - Claims Verification

## Findings

### 1. The `hooks` log component is already bound in `cmd`

**Source**: Tree measurement — `grep -rn 'log.For("hooks")\|hooksLogger' --include='*.go' cmd internal | grep -v _test.go`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §2.2 (Stamping is lazy, at `hook set`), §6.5 (Acquisition is bounded, and a timeout degrades rather than wedges)

**Problem**:
The specification tells the reader that emitting the dirty-flag-touch WARN from `cmd` adds a new `hooks` component binding to that package, and its closing ledger of everything this work unit adds to the closed logging vocabulary counts that binding as one of the additions. `cmd` already binds the `hooks` component — `hooksLogger = log.For("hooks")` sits in `cmd/state_common.go:11` alongside eight other component bindings in the same package. The new WARN reuses an existing binding; nothing is gained. A reader auditing the vocabulary amendment against the tree finds one of the four listed items is not an amendment at all, which puts the whole ledger in doubt — and the ledger is the artefact CLAUDE.md's "new components/attrs require amending the spec" rule is checked against.

There is a second, real consequence the current wording hides. `hooksLogger`'s only consumer today is `cmd/state_migrate_rename.go:24` — the file §7.3 deletes outright. So the touch WARN is what keeps an existing binding alive across that deletion, rather than what introduces a new one.

**Proposal**:
State that the binding already exists and is reused. `cmd/state_common.go:11` declares `hooksLogger = log.For("hooks")` today, and its sole non-test consumer is the file §7.3 removes; correct §2.2 to say the WARN reuses that binding, and drop the binding from §6.5's list of vocabulary additions so the ledger reads two `op` values, two `via` values, no new component binding and no attr key.

**Evidence**:
Claim (§2.2): "The line is emitted from `cmd`, where the touch happens — the store has no path to the state directory — so the `hooks` component gains a binding in that package."

Claim (§6.5): "That adds **two `op` values** — `load-unlocked` here and `touch-save-requested` for the dirty-flag touch (§2.2) — **two `via` values, one component binding in `cmd` (§2.2) and no attr key**: the whole of this work unit's amendment to the closed logging vocabulary."

```
$ grep -rn 'log.For("hooks")\|hooksLogger' --include='*.go' cmd internal | grep -v _test.go
cmd/state_migrate_rename.go:24:		return runMigrateRename(store, args[0], args[1], hooksLogger)
cmd/state_common.go:11:	hooksLogger     = log.For("hooks")
internal/hooks/store.go:19:var logger = log.For("hooks")
```

```
$ grep -rn 'log.For(' --include='*.go' cmd/state_common.go
cmd/state_common.go:8:	daemonLogger    = log.For("daemon")
cmd/state_common.go:9:	hydrateLogger   = log.For("hydrate")
cmd/state_common.go:10:	notifyLogger    = log.For("notify")
cmd/state_common.go:11:	hooksLogger     = log.For("hooks")
cmd/state_common.go:12:	bootstrapLogger = log.For("bootstrap")
cmd/state_common.go:13:	restoreLogger   = log.For("restore")
cmd/state_common.go:14:	previewLogger   = log.For("preview")
cmd/state_common.go:18:	signalLogger  = log.For("signal")
cmd/state_common.go:19:	captureLogger = log.For("capture")
```

The claim appears in no source document:

```
$ grep -n 'hooksLogger\|component binding\|log.For\|binding in' .workflows/resume-hooks-silently-lost/investigation/resume-hooks-silently-lost.md
(no output)
```

**Current**:
§2.2, final sentence of the touch bullet:

> The line is emitted from `cmd`, where the touch happens — the store has no path to the state directory — so the `hooks` component gains a binding in that package. The store's own emissions are unaffected.

§6.5, final sentence of the degraded-read paragraph:

> That adds **two `op` values** — `load-unlocked` here and `touch-save-requested` for the dirty-flag touch (§2.2) — **two `via` values, one component binding in `cmd` (§2.2) and no attr key**: the whole of this work unit's amendment to the closed logging vocabulary.

**Proposed Text**:
§2.2:

> The line is emitted from `cmd`, where the touch happens — the store has no path to the state directory. The `hooks` component is already bound in that package (`hooksLogger`, `cmd/state_common.go:11`), so the WARN reuses that binding rather than adding one; its only consumer today is the file §7.3 deletes, which is what makes this line the binding's remaining reader. The store's own emissions are unaffected.

§6.5:

> That adds **two `op` values** — `load-unlocked` here and `touch-save-requested` for the dirty-flag touch (§2.2) — **two `via` values, no new component binding and no attr key**: the whole of this work unit's amendment to the closed logging vocabulary.

**Resolution**: Pending
**Notes**:
