# Review Tracking: Resume Hooks Silently Lost - Gap Analysis

## Findings

### 1. Registration cannot distinguish "pane gone" from "live pane, no token"

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: choice
**Priority**: Critical
**Affects**: §4.1 (Registration verifies the pane exists), §4.2 (Removal verifies the same way), §3.3 (`ResolveHookKey`)

**Details**:
§4.1 establishes three outcomes of the `show-options -p` read and states that Portal reads only the exit status:

| `tmux show-options -p -t <target> @portal-pane-id` | Result |
| pane does not exist | exit 1, `no such pane: %999` |
| live pane, no token | exit 1, `invalid option: @portal-pane-id` |
| live pane, stamped | exit 0, `@portal-pane-id TOKEN1` |

followed by "Portal never parses that message text: a non-zero exit is the whole signal".

Two of the three cases exit 1, and the sequence requires them to diverge: step 2 says "A gone pane fails here, before anything is written", while step 3 says "A live pane with no token is minted one and stamped". With the exit status as the whole signal, step 2 cannot separate them — it either fails both (in which case a first-ever `hook set` on a legitimate live pane is impossible) or fails neither (in which case a gone pane is caught at step 3, and step 3's description of itself as "a second, redundant guard" is wrong, since nothing caught it first).

An implementer has to invent the discriminator: parse tmux's stderr (which §4.1 forbids), add a separate existence probe (e.g. a `list-panes` membership test), or reorder so that `set-option -p` is the only gate. Each gives different behaviour and different user-visible messages, and §4.2 inherits the same read — `hook rm` on a live pane with no token is supposed to "report that", which the "no such pane" wording would misdescribe.

**Proposed Change**:
Split existence from value. `show-options -p -t <target>` with no option named is the existence gate; `display-message -p '#{@portal-pane-id}'` reads the token.

**Resolution**: Approved
**Notes**: Option 1 chosen. Measured on tmux 3.7c in an isolated `-L` socket: `show-options -p -t %999` → exit 1 `no such pane`; `show-options -p -t %0` (no option named, pane carrying no pane options) → exit 0, empty; `show-options -p -t %0 @portal-pane-id` → exit 1 `invalid option` (the collapse); `display-message -p -t %999 '#{@portal-pane-id}'` → exit 0, empty. §4.1's three-case table replaced by the measured call/case matrix plus the note on why the option name is omitted; sequence renumbered to six steps with the ordering rule now on steps 4–5. §4.2 re-pointed at "steps 2–3" and its no-token report stated as Portal's own wording. §3.3's `ResolveHookKey` row describes the pair. §9.2's `set-option -p` test row widened to cover the probe, which is now the primary gate.

---

### 2. The stale sweep is unconstrained during the restore window

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Critical
**Affects**: §2.3 (Restore re-stamp), §5.2 (Deletion becomes shape-aware), §5.4 (The mass-deletion guard)

**Details**:
Under the token key, a restored pane exists before it carries its token: §2.3 places the re-stamp "After the skeleton exists and before each pane is armed", so for the interval between skeleton construction and the re-stamp, live panes answer to no token. §5.2 deletes any token-shaped key whose token is absent from the live set, and §5.4's guard deliberately keys off the pane row count rather than the token set — so with a skeleton up and no tokens stamped yet, the guard does not defer and the sweep is authorised to delete every token-keyed entry on the machine.

The spec does not say whether the sweep is suppressed for the duration of the restore window (the existing `@portal-restoring` marker is the obvious candidate, and §1.1/§5.1 place the sweep on the daemon's idle branch, which is reachable while the capture loop is suppressed). Nor does it bound the window: a large working set can take longer to reconstruct than one sweep interval.

This is a new hazard created by the change rather than an existing one: a positional key resolved the moment a pane existed, so the old scheme had no equivalent gap between "pane exists" and "pane is identifiable". Without a stated rule an implementer will either add suppression on their own judgement or ship the race.

**Proposed Change**:
Three paragraphs appended to §5.4 naming the restore-window gap, the daemon's existing immunity as load-bearing, and the move of the `@portal-restoring` check into `runHookStaleCleanup`.

**Resolution**: Approved
**Notes**: Tree measurement changed the answer: `tick` already reads `@portal-restoring` and returns before `maybeRunHookCleanup` (`cmd/state_daemon.go:174-181`), so the daemon path was never exposed — but that early return previously protected only capture, and the spec must now name it load-bearing. `portal doctor --fix` (`cmd/doctor.go` `pruneDoctorStaleHooks`) had no gate, so the check moves into `runHookStaleCleanup`, travelling with the rule the way §5.2's shape-awareness does. Failed read presumes set, matching `cmd/state_commit_now.go:111`. Applied under `auto`.

---

### 3. `hook list`'s location column has no specified tmux surface to read from

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §4.4 (`portal hook list` renders the resolved location), §3.3 (`internal/tmux` surface changes)

**Details**:
§4.4 states "Resolution is one `list-panes -a` read over the token format (§3.3), reused across all rows", pointing at §3.3 for the mechanism. But §3.3 defines the only all-pane enumeration, `ListAllPaneHookKeys()`, as returning "one entry per live pane, empty string for an unstamped pane" — tokens alone. A token set carries no `<session>:<window>.<pane>` location, so it cannot answer the question §4.4 asks of it.

The column therefore needs either a second format field on that enumeration (changing its stated return shape and the row-count property §5.4 depends on) or a new client method returning token → location pairs. §3.3's surface-change list is otherwise exhaustive for `internal/tmux`, so the omission reads as an oversight rather than an implementation choice, and the two options have different consequences for §5.4's guard.

**Proposed Change**:
`ListAllPaneHookKeys()` returns one row per live pane carrying both the token and its `<session>:<window>.<pane>` location, from a single two-field `list-panes -a -F` read.

**Resolution**: Approved
**Notes**: The second-format-field reading does not in fact break §5.4's row-count property — one row per live pane is preserved whatever each row carries — so the objection to it is weaker than the finding states, and the alternative costs a second all-pane tmux read on a separate path. One enumeration serving the sweep, `portal doctor` and `hook list` also keeps §3.3's "no derivation, only a read of one value" invariant intact. The location field is marked display-only so it implies no coupling with the positional siblings. §3.3's stale `(§5.3)` cross-reference corrected to `(§5.4)` in the same edit. Applied under `auto`.

---

### 4. The `@portal-pane-id` option constant is never given a name or a home

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: choice
**Priority**: Important
**Affects**: §2.1 (The token and its tmux option), §7.2 (What is removed), §9.4 (Guards)

**Details**:
§9.4 requires the existing literal-binding guards to be re-pointed and states that "the `@portal-pane-id` literal remains duplicated between the option constant and `captureFormat`, and the guard remains the only thing binding them" — so an option constant exists. The spec never names it or says which package declares it.

The obvious inheritance is broken by this work unit's own changes: the old constant was `session.PortalIDOption` in `internal/session/create.go`, and §7.2 deletes both that constant and the stamp that justified it living there. Under §2.2 the stamping site is now `cmd/hooks.go`, the reading site is `internal/tmux`, and the capture literal is in `internal/state`. §9.4 further constrains the choice — the re-pointed `cmd` guard compares the constant against `tmux.HookKeyFormat`, which requires `cmd` to be able to import the constant's package cycle-free.

The identifier is referenced by three sections and declared by none, so the first implementer to reach it makes a placement decision that §9.4's guard is written against.

**Proposed Change**:
`state.PortalPaneIDOption`, declared in `internal/state` beside the other tmux option-name constants; every site composes it; §9.4's two guards deleted rather than re-pointed.

**Resolution**: Approved
**Notes**: Option 1 chosen. Measured: `internal/state` already declares `SkeletonMarkerPrefix`, `RestoringMarkerName` and `BootstrappedMarkerName` (`internal/state/markers.go`) and imports only `tmuxerr`/`tmuxout`/`log`/`fileutil`/`xdg`, so it is importable by `internal/tmux` (which already imports it, `internal/tmux/portal_saver.go:11`) and by `cmd`. That collapses the two-copies condition §9.4 assumed permanent — `captureFormat` composes the constant in-package, `HookKeyFormat` composes it across the existing import, and the guards have nothing to bind. §2.1 gains the declaration paragraph; §9.4 rewritten to delete both guards with the by-construction argument.

---

### 5. The `save.requested` touch has no state-dir resolution and no failure behaviour

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §2.2 (Stamping is lazy, at `hook set`), §4.1 (step 5)

**Details**:
§2.2 says `hook set` "calls `state.TouchSaveRequested(<state dir>)`" and §4.1 step 5 says "Touch `save.requested` (§2.2)". `<state dir>` is a placeholder: the spec does not say how a bootstrap-exempt, config-file-only command obtains the state directory, and `hook` is characterised in §2.2 as touching nothing beyond the hooks file and (on the `$TMUX_PANE` path) tmux.

Nor is the failure path stated. The touch can fail for ordinary reasons — no state directory yet on a fresh install, a permissions problem — and the spec gives no rule for whether that is best-effort-and-swallowed, a WARN, or a non-zero exit. Given §4.1 makes the surrounding steps a strict ordered sequence with a non-zero exit on failure at steps 2 and 3, an implementer cannot infer which shape step 5 takes, and the two readings differ in whether a registration that has already written `hooks.json` can still report failure.

**Proposed Change**:
Resolve via `state.EnsureDir()`; the touch is best-effort — one WARN under the `hooks` component with the existing `error` attr, exit status unaffected.

**Resolution**: Approved
**Notes**: Precedent measured: `cmd/state_notify.go` resolves with `state.EnsureDir()` then `state.TouchSaveRequested(dir)` from an equally bootstrap-exempt command, and `state.TouchSaveRequested` already documents its own mtime bump as best-effort (`internal/state/paths.go:64`). The exit-status question resolves on ordering — the write precedes the touch, so a failure there cannot mean a lost registration. §2.2's dirty-flag bullet rewritten with the resolution, the best-effort rule and the bootstrap-exemption note. Applied under `auto`.

---

### 6. The restore re-stamp WARN has no message or attr keys

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §2.4 (Restore re-stamp: failures are surfaced)

**Details**:
§2.4 requires that "A failed pane re-stamp logs at WARN under the `restore` component, naming the session and pane", and §9.2 asserts against it ("produces a WARN naming the session and pane"). The project's logging contract is a closed vocabulary — a fixed message catalog per component and a fixed attr-key set — so "naming the session and pane" is not implementable without the message string and the attr keys, and an implementer inventing them at the call site violates the standing rule that new attrs are spec-governed.

§5.3 does exactly this work for the other new emission in this spec ("The existing `hooks` component attr vocabulary (`op`, `hook_key`, `via`, `entries`) is sufficient — no new component and no new attr key"), which makes the silence for the `restore` component a gap rather than a deliberate omission.

**Proposed Change**:
Message `set pane token failed` under the `restore` component, attrs `session` / `pane_key` / `error` — the existing vocabulary, no additions.

**Resolution**: Approved
**Notes**: Derived from the `restore` component's measured emission style: `internal/restore/session.go:274` already emits `set skeleton marker failed` with exactly `session`, `pane_key` and `error`, from the same loop over the same panes. `pane_key` is specified as the live structural key rather than the token, since a stamp that failed has no token on the pane to name. Applied under `auto`.

---

### 7. The lock-timeout emissions have no message, component or attr keys

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §6.5 (Acquisition is bounded, and a timeout degrades rather than wedges)

**Details**:
§6.5 introduces three new emissions and specifies only their level: the sweep "skips this cycle with a WARN", a degraded read "logs at DEBUG", and `hook set` / `hook rm` "exit non-zero with the reason". None of the three has a message, a component, or attr keys, and the same closed-vocabulary constraint applies as in finding 6 — a lock timeout is not obviously expressible in the `hooks` component's current attr set (`op`, `hook_key`, `via`, `entries`), which §5.3 declares sufficient for the *deletion* line only.

"Exit non-zero with the reason" is also unspecified as to surface: whether the reason reaches the user on stderr, and whether it also produces a log line. §4.2 makes `hook rm`'s exit status a contract an external caller reads, so the difference between a silent non-zero and a reasoned one is load-bearing for the integration described in §8.4.

**Proposed Change**:
Sweep skip and CLI failure reuse the component's existing failed-operation WARN shape; the degraded read adds one `op` value, `load-unlocked`, and no attr key. CLI reason goes to stderr via cobra *and* the log.

**Resolution**: Approved
**Notes**: Measured the `hooks` component's actual vocabulary (`internal/hooks/store.go:73,93,103,144,220` plus `internal/storelog`): messages are op names, attrs are `op` / `hook_key` / `value` / `via` / `entries` / `took` / `error` / `error_class`. A lock timeout is an operation failing, which the existing `logger.Warn(<op>, "op", <op>, …, "error", err)` shape already expresses — so only the read-side degradation needs anything, and `Store.Load` currently emits nothing at all. Amendment scoped explicitly to one op value so the closed-vocabulary rule is satisfied rather than bypassed at the call site. Applied under `auto`.

---

### 8. Where the sweep's stale determination is made relative to the exclusive lock

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §5.1 (What the reaper does now), §6.3 (Readers take a shared lock, writers exclusive)

**Details**:
§6.3 names "the sweep's own pre-read" among the callers of `Store.Load`, and separately requires that `CleanStale` "acquire once and hold across their internal load and save". That describes two reads of `hooks.json` per sweep: an unlocked-or-shared pre-read at the call site, and `CleanStale`'s own load under the exclusive lock.

The spec does not say which read the deletion decision is computed from. If `runHookStaleCleanup` derives the delete set from the pre-read and hands that list to `CleanStale`, the read-modify-write window §6.1 exists to close is reopened at exactly the interleaving §6.1 diagrams — an entry written by `hook set` between the pre-read and the exclusive load is deleted on the strength of a stale snapshot. If instead `CleanStale` receives the live token set and re-derives the delete set under its own lock, the window closes. The two shapes are indistinguishable from the text, and the §9.2 lost-update test as described exercises interleaved *writers*, so it would not catch the wrong one.

**Proposed Change**:
§6.3 gains a paragraph pinning the delete set to `CleanStale`'s own locked derivation from the live token set; the call-site read is advisory only. §9.2's lost-update row gains the matching case.

**Resolution**: Approved
**Notes**: Only one shape is defensible — the other reopens the window D exists to close, which is the whole justification for the lock. The current code already has the right shape (`cmd/run_hook_stale_cleanup.go` loads for the guard, then calls `store.CleanStale(livePanes)` passing the live set, not a delete list), so this pins an existing property rather than changing behaviour — which is exactly why it needed stating: nothing in the spec stopped an implementer regressing it. Test row extended because the finding correctly notes the described lost-update test exercises interleaved writers and would not catch the wrong derivation. Applied under `auto`.

---

### 9. `hook rm` on a stamped pane with no matching entry has no defined exit status

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: choice
**Priority**: Important
**Affects**: §4.2 (Removal verifies the same way)

**Details**:
§4.2 defines removal's outcome for two of three cases: the pane is gone (non-zero) and the pane is live but carries no token (non-zero, "reports that"). The third — a live pane carrying a token for which `hooks.json` holds no entry — is not covered, and it is the routine case the moment a caller deregisters twice, or deregisters from a pane that never registered.

The section makes this exit status a contract rather than an incidental: it argues that "a caller can no longer read `rc == 0` as proof anything happened" and warns the external integration (§8.4) that non-zero becomes routine. An implementer choosing exit 0 (nothing to do) versus non-zero (nothing removed) changes what that caller must tolerate, and §9.2's `hook rm` test asserts only the unresolvable-`$TMUX_PANE` case, so either choice passes.

**Proposed Change**:
`hook rm` exits 0 iff it removed an entry. The third case — live stamped pane, no matching entry — is non-zero with its own message.

**Resolution**: Approved
**Notes**: Option 1 chosen. §4.2 gains the rule as a single stated invariant covering all three ways of removing nothing, with the idempotent reading named and rejected on the grounds that it reinstates the uninformative exit code this work unit exists to remove.

---

### 10. Lock acquisition versus config-directory creation on a first write

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §6.2 (A sidecar lock file), §6.5 (Acquisition is bounded)

**Details**:
The sidecar is "`<hooks.json path>.lock`", opened `O_CREAT`, and §6.5 rules that a write which cannot take the lock does not write, extending that explicitly to "the sidecar lock file failing to open or be created at all".

On an install where the hooks file has never been written, the directory holding it may not exist yet, and creating it is part of the write path. The spec does not order lock acquisition against that creation. Taken literally — acquire, then write — the very first `portal hook set` on a clean install fails to create the lock file and, by §6.5's rule, must not write, so registration is impossible until something else creates the directory. Taken the other way, the directory is created outside the lock and acquisition follows.

This is a first-run path with no other coverage in the spec (§9.2's lock test presumes an existing file), so the ordering needs to be stated rather than inferred.

**Proposed Change**:
§6.2 gains a paragraph: the config directory is created before the lock is acquired, with the reason the ordering is not a hole in the exclusion.

**Resolution**: Approved
**Notes**: Only one ordering is viable — the other bricks first registration on a clean install — so this states the answer rather than choosing between live options. The justification is that directory creation is idempotent, races benignly, and touches nothing the lock protects; `hooks.json` stays untouched until the lock is held. Applied under `auto`.

---

### 11. The window between an out-of-band conversion and the next capture is not covered by §8.3's safety argument

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §8.3 (What makes that safe rather than reckless), §2.2 (the `save.requested` touch), §5.2

**Details**:
§8.3's safety argument covers the *unconverted* entry: it is not token-shaped, so the reaper retains it. It does not cover the freshly *converted* one. After the conversion described in §8.2 — stamp the live pane, rewrite the entry under the token — the token exists only as a tmux pane option until the daemon's next capture lifts it into `sessions.json`. If the server dies in that window, restore has an empty `PortalPaneID` for the pane, §2.3 skips it, no live pane carries the token, and §5.2 deletes the entry as a token-shaped key with no live match.

The conversion therefore moves an entry from the protected class (retained forever) into the reapable class before the durability that justifies reaping it exists. §2.2 recognises exactly this hazard for `hook set` and narrows it with the `save.requested` touch; the out-of-band route has no equivalent, and §8.3 states the ordering caveat only for entries registered between upgrade and conversion, not for this one.

The script itself is out of scope (§8.2), but the property that makes it safe is asserted by §8.3 and the deletion rule that acts on its output is §5.2, both of which are in scope.

**Proposed Change**:
§8.3 gains a paragraph naming the converted-entry window, the mitigation available to the script (touch `save.requested`, or run `portal state commit-now`), and the residual as identical in kind to §2.2's accepted one.

**Resolution**: Approved
**Notes**: The finding is right that §8.3's argument covers only the unconverted entry. The fix stays inside scope — the script remains out of scope; what lands is the honest statement of the window plus the mitigation Portal already provides (`portal state commit-now` exists as a command, `cmd/state_commit_now.go:86`). Applied under `auto`.

---

### 12. "Not only at DEBUG" leaves the per-key line's DEBUG fate ambiguous

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §5.3 (The reaper names what it deleted)

**Details**:
§5.3 opens "Each removed key is logged at **INFO** under the `hooks` component, not only at DEBUG." That reads two ways: the existing DEBUG line is promoted to INFO (one line per removed key), or an INFO line is added alongside the retained DEBUG line (two lines per removed key at debug level). §9.2's assertion — "the deletion names the key at INFO rather than only counting it" — passes under either.

The choice is visible in `portal.log` at DEBUG, which is the level an operator raises to precisely when investigating a loss, so duplicate per-key lines are worth deciding rather than discovering.

**Proposed Change**:
Promoted, not duplicated — one line per removed key, at INFO. The existing DEBUG line goes.

**Resolution**: Approved
**Notes**: The DEBUG line (`internal/hooks/store.go:220`) carries `op` / `hook_key` / `via` and nothing the INFO line would not, so retaining it buys duplicate lines at the one level where the reader is already hunting. Applied under `auto`.

---

### 13. The shape predicate's anti-drift requirement has no mechanism

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §3.2 (Key shape), §9.4 (Guards)

**Details**:
§3.2 states "The predicate has **one home** and the shape it tests must not be able to drift from the width and alphabet the generator uses", then leaves the home to implementation. The requirement is real — a predicate hardcoding `{6}` and `[A-Za-z0-9]` while `suffixLen` or `NanoIDAlphabet` moves would silently reclassify every key — but no mechanism is named, and there are two very different ones: derive the predicate from `session`'s exported constants (making drift impossible), or duplicate the values and bind them with a guard test.

§9.4 enumerates the guards this work unit needs and does not include one for the predicate, which points at the derive-from-constants reading — but §3.2's own evidence paragraph frames the import as merely possible rather than required, so the reading is not safe to assume.

**Proposed Change**:

**Resolution**: Pending
**Notes**:

---

### 14. A successful stamp followed by a failed write leaves an unreferenced token

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §4.1 (Registration verifies the pane exists), §2.2

**Details**:
§4.1 fixes the order — stamp at step 3, write at step 4, "Steps 3 and 4 must not be reordered" — so any failure of the write (§6.5's lock timeout, a disk error) leaves the pane carrying a token that no `hooks.json` entry references.

§2.2's reuse rule ("A pane already carrying a token keeps it") makes this benign in practice, but the spec never says so, and the state is otherwise novel: nothing else in the spec produces a stamped pane with no entry, and §4.2 treats "a pane with no token has no entry" as the only no-entry shape. Stating the outcome (leave it, the next registration reuses it) closes the question before an implementer adds an unstamp-on-failure rollback that would race a concurrent registration.

**Proposed Change**:

**Resolution**: Pending
**Notes**:

---

### 15. §8.4 restates §1.3's account of the external hook script

**Source**: Specification analysis
**Category**: Duplication
**Priority**: Minor
**Affects**: §8.4 (The user's own integration), §1.3 (Out of scope)

**Details**:
The facts that the script re-implements `HookKeyFormat` verbatim, that it matches against `portal hook list` output, that it cites `:87-95`, and that it needs updating in step with this change are all stated in §1.3's first out-of-scope bullet. §8.4's opening restates all four before pointing back at §1.3 for the scope call. Only the second paragraph (how it degrades) and the "same person's job on the same day" rationale are §8.4's own.

**Current**:
```
`~/.claude/hooks/portal-resume-hook.sh` re-implements `HookKeyFormat` verbatim (`:87-95`) and matches it against `portal hook list` output. It needs updating in step with this change. It is out of scope (§1.3) and named here only because the conversion script and the hook script are the same person's job on the same day.
```

**Proposed Change**:
```
`~/.claude/hooks/portal-resume-hook.sh` (§1.3) is named here only because the conversion script and the hook script are the same person's job on the same day.
```

**Resolution**: Pending
**Notes**:

---

### 16. §9.5 restates §1.3's positional-siblings enumeration and rationale

**Source**: Specification analysis
**Category**: Duplication
**Priority**: Minor
**Affects**: §9.5 (The positional siblings are checked, not assumed separate), §1.3 (Out of scope)

**Details**:
§1.3's last out-of-scope bullet is the home for the sibling list (`state.SanitizePaneKey`, `StructuralKeyFormat`, `ResolveStructuralKey`, `ListAllPanes`), for what they key, and for the fact that they are checked against the change rather than changed by it. §9.5 re-enumerates all four names and re-states both facts before adding its own content, which is the specific thing the testing phase asserts.

**Current**:
```
`state.SanitizePaneKey` and `internal/tmux`'s `StructuralKeyFormat` / `ResolveStructuralKey` / `ListAllPanes` key the hydrate FIFO paths and `@portal-skeleton-*` markers off the same positional addressing (§1.3). They are not changed, but the addressing assumption is identical, so their existing coverage is run against the change rather than assumed unaffected — specifically that a restore whose window indices are renumbered still pairs FIFOs and markers correctly, which is the §9.2 non-contiguous-index test observing a second surface.
```

**Proposed Change**:
```
The positional siblings (§1.3) are not changed, but the addressing assumption is identical, so their existing coverage is run against the change rather than assumed unaffected — specifically that a restore whose window indices are renumbered still pairs FIFOs and markers correctly, which is the §9.2 non-contiguous-index test observing a second surface.
```

**Resolution**: Pending
**Notes**:

---

### 17. §5.1 restates the sweep cadence that §1.1 sources

**Source**: Specification analysis
**Category**: Duplication
**Priority**: Minor
**Affects**: §5.1 (What the reaper does now), §1.1 (The defect)

**Details**:
§1.1 is the home for the cadence: "The sweep runs on the daemon's idle branch at `hookCleanupInterval = 10 * time.Second` (`cmd/state_daemon.go:105`)". §5.1 restates the value and re-cites the same file for the same call site. (§6.1 and §6.5 also carry "10s", but there it is argument the reader needs in place — the race frequency and the relation between the lock bound and the tick — rather than a second statement of the constant.)

**Current**:
```
It runs from two call sites over the same code path: the daemon's idle branch every 10s (`cmd/state_daemon.go` `maybeRunHookCleanup`), and `portal doctor --fix` (`cmd/doctor.go` `pruneDoctorStaleHooks`), which supplies an `onRemoved` callback printing `Pruned stale hook: <key>`.
```

**Proposed Change**:
```
It runs from two call sites over the same code path: the daemon's idle branch (§1.1, `cmd/state_daemon.go` `maybeRunHookCleanup`), and `portal doctor --fix` (`cmd/doctor.go` `pruneDoctorStaleHooks`), which supplies an `onRemoved` callback printing `Pruned stale hook: <key>`.
```

**Resolution**: Pending
**Notes**:

---

### 18. §2.2 restates the stamp-before-write ordering rule

**Source**: Specification analysis
**Category**: Duplication
**Priority**: Minor
**Affects**: §2.2 (Stamping is lazy, at `hook set`), §4.1 (Registration verifies the pane exists)

**Details**:
The ordering rule and its justification live in §4.1, which owns the numbered sequence and closes with "Steps 3 and 4 must not be reordered: a write that precedes the stamp would persist an entry keyed to a token no pane carries", having already established that "the stamp *is* the check". §2.2 states the placement in its own lead sentence ("applied by `portal hook set`, immediately before the `hooks.json` write") and then states it a second time as a bullet with the same justification.

**Current**:
```
- **The stamp precedes the write.** Under B (§4) the stamp doubles as the existence check, so ordering is not optional.
```

**Proposed Change**:
Delete the bullet.

**Resolution**: Pending
**Notes**:

---

### 19. §8.3's third bullet restates §5.2's first consequence

**Source**: Specification analysis
**Category**: Duplication
**Priority**: Minor
**Affects**: §8.3 (What makes that safe rather than reckless), §5.2 (Deletion becomes shape-aware)

**Details**:
§5.2's first consequence is the home for shape-carried protection reaching `portal doctor --fix`: "The protection travels with the rule, so it holds wherever the rule lives. There is no 'guard at the daemon call site versus inside `CleanStale`' split". §8.3's third bullet repeats the claim in near-identical words, in a section that already opens by pointing at §5.2. The two remaining bullets (inert entry, partial conversion) are migration-specific consequences and belong where they are.

**Current**:
```
- the protection is by **key shape**, so it holds wherever the rule lives, including `portal doctor --fix`, which shares the same code path.
```

**Proposed Change**:
Delete the bullet.

**Resolution**: Pending
**Notes**:
