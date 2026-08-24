# Review Tracking: Resume Hooks Silently Lost - Input Review

## Findings

### 1. Nothing exercises Portal's own existence probe against a real tmux server

**Source**: `investigation/resume-hooks-silently-lost.md` — Testing Recommendations (*"**`hook set` against an unresolvable `$TMUX_PANE`** must exit non-zero and write nothing. **Must be a real-tmux test**: a fake `Commander` cannot reproduce tmux's exit-0-with-`:.`, which is precisely why the behaviour was filed as a test obstacle rather than a bug."*); Why It Wasn't Caught (the `:.` behaviour was known in the repo and filed as a testing obstacle); Refinements, "B is stronger than first described, and nearly free" (`set-option -p` exits 1 where `display-message -p` exits 0)
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §9.2 (New tests) — the "existence probe separates the three cases" row; reasoning in §9.1

**Problem**:
The whole registration guard turns on one discrimination — a pane that is gone versus a live pane that has never been stamped — and §4.1 names the exact mistake that destroys it: naming the option in the `show-options` probe, which makes a live unstamped pane exit non-zero just like a gone one. Under the tests as listed, that mistake ships green. The real-tmux row asserts hand-written tmux commands, so it measures tmux rather than Portal's own argv; the `cmd`-level row runs against an injected fake that models the intended semantics by construction. The user's first `hook set` on any pane then fails with a "pane does not exist" error against a pane they are sitting in, and no listed test fails.

The source made this the one behaviour it insisted be pinned against real tmux, for exactly this reason: the discriminating signal lives in tmux's exit status, and a fake cannot tell you what tmux actually does with the argv Portal sends.

**Proposal**:
Drive the real-tmux row through `tmux.ResolveHookKey` itself — the two-call resolution §3.3 gives it — rather than through hand-written tmux commands, so the assertion measures Portal's composed argv against a live server across all three cases. The `cmd`-level test keeps its propagation-only role per §9.1, and the raw tmux facts B rests on stay asserted alongside.

**Current**:
```
| **The existence probe separates the three cases** | `show-options -p -t %999` (no option named) exits non-zero while `show-options -p -t <live pane>` exits 0 on a pane carrying no pane options; `set-option -p -t %999 @portal-pane-id X` likewise exits non-zero, unlike `display-message -p` against the same target, which exits 0. The behaviours B is built on (§4.1). | unit (real-tmux) |
```

**Proposed Text**:
```
| **The existence probe separates the three cases** | Driven through `tmux.ResolveHookKey` against a live server, so what is measured is Portal's own argv: a pane id no pane answers to fails; a live pane carrying no pane options at all resolves with an empty token; a stamped pane resolves to its token. This is what pins §4.1's rule that the `show-options` probe names no option — naming it makes a live unstamped pane indistinguishable from a gone one, which a fake `Commander` modelling the intended semantics cannot catch. The raw tmux facts B rests on are asserted alongside: `set-option -p -t %999 @portal-pane-id X` exits non-zero, unlike `display-message -p` against the same target, which exits 0. | unit (real-tmux) |
```

**Resolution**: Pending
**Notes**:

---

### 2. The pre-upgrade daemon deletes token-keyed entries within 10s of them being written

**Source**: `investigation/resume-hooks-silently-lost.md` — Migration ("Ordering at upgrade time (upgrade, then run the script) is the mitigation for entries registered in between, not code"); Refinements, C reduced / risk 1 ("the window in which one `doctor --fix` run would have destroyed all 41 entries"); Finding 3 (the sweep runs every 10s on the daemon's idle branch). The daemon-version dimension of that ordering is addressed by no source.
**Category**: Gap/Ambiguity
**Move**: settled
**Affects**: §8.3 (What makes that safe rather than reckless); touches §5.2, §2.2

**Problem**:
The protection that makes shipping without migration safe lives in the new binary's sweep — but the sweep does not run in the binary the user just installed. It runs in `_portal-saver`'s `portal state daemon`, which keeps executing the pre-upgrade binary until a bootstrapping command replaces it, and `hook` is bootstrap-exempt (§2.2), so no amount of `portal hook set` will do it.

While that lag lasts, the old sweep is not shape-aware and does not recognise a token key: every token-keyed entry — one written by a post-upgrade `hook set`, or the entire file the moment the conversion script finishes — is absent from the old positional live-key set and is deleted within 10s. The user registers a hook, gets exit 0, and loses it silently; or they run the conversion script and lose all 42 entries at once. That is the outcome §8.1 rejects "using `CleanStale` as the migration" to avoid, arriving by a different door, and §8.3's safety argument does not reach it because the retention rule it rests on is not the rule that is running.

The same lag has a second edge: a pre-upgrade daemon captures the pre-upgrade `captureFormat`, so a server death before it is replaced leaves restore with no saved token for any pane.

**Proposal**:
State the constraint alongside the ordering caveat §8.3 already owns: the upgrade is not complete for these purposes until the saver is running the new binary, so any bootstrapping command (`portal open` is the ordinary one) precedes both registration and conversion. Procedural for the same reason the existing ordering caveat is — there is no code path that can protect a file from a binary that shipped before the rule existed.

**Current**:
```
The one thing not covered by code is ordering: an entry registered *between* the upgrade and the script's run is already token-keyed and needs no conversion, while one registered before it is old-format and does. Running the script after upgrading is the mitigation, not a code path.
```

**Proposed Text**:
```
The one thing not covered by code is ordering: an entry registered *between* the upgrade and the script's run is already token-keyed and needs no conversion, while one registered before it is old-format and does. Running the script after upgrading is the mitigation, not a code path.

**Ordering covers the running daemon, not only the binary on disk.** The sweep runs inside `_portal-saver`'s `portal state daemon`, which keeps executing the pre-upgrade binary until a bootstrapping command replaces it (`EnsureSaver`) — and `hook` is bootstrap-exempt (§2.2), so registering a hook will never do it. A pre-upgrade sweep is not shape-aware and does not recognise a token key, so while that lag lasts every token-keyed entry — one written by a post-upgrade `hook set`, or the whole file the moment the conversion completes — is deleted within 10s, by the rule that was correct for the binary running it. Running any bootstrapping command (`portal open`) before registering or converting is what closes it. The same lag has a second edge: a pre-upgrade daemon captures the pre-upgrade `captureFormat`, so a server death before it is replaced leaves restore with no saved token for any pane.
```

**Resolution**: Pending
**Notes**:
