# Review Tracking: Resume Hooks Silently Lost - Gap Analysis

## Findings

### 1. The pane existence probe and the pane stamp have no home in the tmux client surface

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Important
**Affects**: §2.1 (The token and its tmux option), §3.3 (`internal/tmux` surface changes), §4.1 (Registration verifies the pane exists)

**Problem**:
§2.1 places "the probe, read and stamp" in `cmd`; §3.3 places the probe and the token read inside `internal/tmux`'s `ResolveHookKey`. The same two tmux calls are put in different packages by the two sections. Worse, §3.3's enumeration of `internal/tmux` surface changes names no method for either new pane-scoped tmux operation the design needs — `show-options -p` (the existence probe) and `set-option -p` (the stamp) — and the tmux client exposes no pane-option write at all today. A builder is left choosing between issuing tmux argv directly from `cmd` (crossing the boundary every other tmux call in the repo respects, and landing outside the `*Deps` seams `cmd` tests inject — §9.2's `hook set`/`hook rm` unit tests assume a seam exists) and inventing an unnamed client method. Registration is change B's entire deliverable, and its two reads currently have no owner.

**Proposal**:
§4.1 already fixes what each call is, and it settles the ownership: the probe deliberately names no option and the token read goes through `HookKeyFormat`, so neither composes the literal and both sit inside `ResolveHookKey` where §3.3 already puts them. The stamp is the one site in `cmd` that needs the option literal — which is what §2.1's clause is reaching for. Narrow §2.1 to the stamp, and add the pane-option write to §3.3's surface list as the new exported method `cmd` calls with `state.PortalPaneIDOption`.

**Current**:
> Every site that needs the literal composes it from that constant: `captureFormat` in the same package, `HookKeyFormat` and the all-pane enumeration format in `internal/tmux` (which already imports `internal/state`), the re-stamp in `internal/restore`, and the probe, read and stamp in `cmd` (which imports both).

**Proposed Text**:
§2.1, replacing the sentence above:

> Every site that needs the literal composes it from that constant: `captureFormat` in the same package, `HookKeyFormat` and the all-pane enumeration format in `internal/tmux` (which already imports `internal/state`), the re-stamp in `internal/restore`, and the stamp in `cmd` (which imports both). The existence probe names no option at all (§4.1) and the token read is made through `HookKeyFormat`, so neither composes the literal itself.

§3.3, appended to the `internal/tmux` surface-changes list:

> - A **pane-option write is added** — `internal/tmux` exposes no pane-scoped option setter today. It takes the option name from its caller, so `cmd` passes `state.PortalPaneIDOption` when `hook set` stamps at §4.1 step 4, and it is reached through the `hook` command's existing `*Deps` seam like every other tmux call the CLI makes. The existence probe adds no exported surface of its own: it is internal to `ResolveHookKey`, which owns both reads of §4.1 steps 2–3.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed, both edits. The ownership split follows from §4.1's own design — the probe names no option and the read goes through `HookKeyFormat`, so only the stamp needs the literal — and the new pane-option write is added to the tmux surface list so registration's two calls have an owner and a seam.

---

### 2. The `save.requested` touch failure has no named `op`, and the vocabulary tally leaves no room for one

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Important
**Affects**: §2.2 (Stamping is lazy, at `hook set`), §6.5 (What each of those emits)

**Problem**:
§2.2 requires one WARN when the dirty-flag touch fails, but names neither the message nor the `op` value it carries — and the project's rule is that log vocabulary is amended in the spec and never invented at a call site. §6.5 then declares the work unit's whole vocabulary amendment to be one `op` value and two `via` values, which leaves no room for this line. A builder either invents a value the rule forbids inventing, or files the WARN under `op=set`, where it reads in the log as a failed registration when the entry was in fact written — the exact misreport §2.2 refuses to make with the exit code.

**Proposal**:
Give the touch its own `op` value on the store's existing failure shape. The argument is §2.2's own: by the time the touch runs the entry is durably written, so naming the failure `set` would report a lost registration that was not lost — in the log for the same reason it must not in the exit status. Correct §6.5's tally to two `op` values so the closed-vocabulary amendment stays accurate.

**Current**:
§2.2:
> A failure at either step, resolving/creating the directory or the touch itself, logs one WARN under the `hooks` component carrying the existing `error` attr, and `hook set` still exits 0.

§6.5:
> That adds **one `op` value, two `via` values and no attr key** — the whole of this work unit's amendment to the closed logging vocabulary.

**Proposed Text**:
§2.2, replacing the sentence above:

> A failure at either step, resolving/creating the directory or the touch itself, logs one WARN under the `hooks` component on the store's existing failure shape — message and `op` both `touch-save-requested`, alongside `hook_key`, `via=cli` and the existing `error` attr — and `hook set` still exits 0. It is filed under its own `op` rather than under `set` for the same reason it does not fail the command: the registration succeeded, and a `set` WARN would name a loss that did not happen.

§6.5, replacing the sentence above:

> That adds **two `op` values** — `load-unlocked` here and `touch-save-requested` for the dirty-flag touch (§2.2) — **two `via` values and no attr key**: the whole of this work unit's amendment to the closed logging vocabulary.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed, both edits. `touch-save-requested` accepted as the op name; the argument that it must not file under `set` is the same one that keeps it out of the exit status. The shape is determined; the exact string `touch-save-requested` is a naming call and can be adjusted without disturbing the finding.

---

### 3. A pane re-registered after the upgrade carries two entries, and the conversion overwrites the newer one

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Important
**Affects**: §8.3 (What makes that safe rather than reckless)

**Problem**:
Between the upgrade and the conversion script's run, a `hook set` on a pane that already holds an old-format entry writes a *second* entry under a fresh token, and §5.2 retains the old-format one indefinitely. The conversion then resolves that stale entry to the same live pane and re-keys it — either onto the token the pane already carries, overwriting the fresher registration with the older command, or onto a newly minted token, orphaning the fresher entry for the reaper. Either way the user silently gets back a resume command they had already replaced, which for the Claude Code integration means a stale `--resume <id>` that fails and drops them at a bare shell. §8.3 exists to say what makes shipping without migration code safe, and it names ordering as the only thing code does not cover, so neither the script's author nor a reader of the spec is warned. `hook set` fires on every SessionStart, so the overlap is the likely case during the lag §8.3 already describes, not an exotic one.

**Proposal**:
Name the case in §8.3 beside the ordering hazard, with the rule the conversion has to honour — a pane already carrying a token has a current entry under it, and its old-format entry is superseded and dropped rather than re-keyed — and the mitigation §8.3 already offers, running the conversion promptly after the upgrading command.

**Proposed Text**:
§8.3, as a new paragraph after "Running the script after upgrading is the mitigation, not a code path.":

> **A pane can hold two entries by the time the script runs.** `hook set` fires on every Claude Code SessionStart, so during the lag above a pane that already has an old-format entry acquires a second, token-keyed one — and §5.2 retains the old one rather than reaping it. Re-keying that old entry would land it on the token the pane already carries and overwrite the newer command, or on a freshly minted token and orphan the newer entry for the reaper; either way the user gets back a resume command they had already replaced. The rule the conversion honours: **a pane that already carries a token has a current entry under it, and its old-format entry is superseded — dropped, not re-keyed.** Running the conversion promptly after the upgrading command keeps the overlap small; it does not remove it.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed, placed before the daemon-lag paragraph so the two upgrade-window hazards read in order. This is the sharpest finding of the cycle: a SessionStart during the lag leaves two entries on one pane, and re-keying the stale one hands back a superseded resume command.

---

### 4. §2.2 claims the state directory is the only path `hook` touches outside `hooks.json`; §6.2 adds two more

**Source**: Specification analysis
**Category**: Contradiction
**Move**: settled
**Priority**: Minor
**Affects**: §2.2 (Stamping is lazy, at `hook set`), §6.2 (A sidecar lock file)

**Problem**:
§2.2 rests its bootstrap-exemption argument on the state directory being "the one filesystem path outside `hooks.json` that `hook` touches". §6.2 has every `hook` write create the config directory and create-and-open `<hooks.json path>.lock`. A builder auditing what this bootstrap-exempt command is allowed to touch is handed two incompatible inventories, and the one in §2.2 is the one that reads as a constraint.

**Proposal**:
The exemption argument survives untouched — none of these paths starts a tmux server — but the count does not. Reword the claim so it names the state directory as the only path outside the config directory that holds `hooks.json` and its sidecar lock.

**Current**:
> This is the one filesystem path outside `hooks.json` that `hook` touches, and it starts no tmux server, so bootstrap-exemption is unaffected: `portal state notify` resolves the state directory exactly this way from an equally exempt command (`cmd/state_notify.go`).

**Proposed Text**:
> This is the only path `hook` touches outside the config directory holding `hooks.json` and its sidecar lock (§6.2), and like them it starts no tmux server, so bootstrap-exemption is unaffected: `portal state notify` resolves the state directory exactly this way from an equally exempt command (`cmd/state_notify.go`).

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The bootstrap-exemption argument is unaffected — none of the paths starts a tmux server — but the inventory was wrong and read as a constraint.

---

### 5. The sweep's advisory pre-read has no `via` value for its degraded read

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Move**: settled
**Priority**: Minor
**Affects**: §6.5 (What each of those emits), §6.3 (Readers take a shared lock, writers exclusive)

**Problem**:
§6.3 gives `runHookStaleCleanup` a locked pre-read of the store that it releases before calling `CleanStale`, so that read is governed by §6.5's read-degrades rule like any other. §6.5 then names the degraded read's `via` value for the hydrate helper, doctor and `hook list` only. The caller left out is the one that reads the file twice, and under the closed-vocabulary rule a builder cannot pick a value for it without the spec saying so.

**Proposal**:
Name the existing `via=internal` for it — the value the sweep's own emissions already carry (§5.3, §6.5's `op=clean-stale` WARN) — which adds nothing to the vocabulary and leaves §6.5's tally as it stands.

**Current**:
> The degraded read is the one genuinely new emission: DEBUG, `op=load-unlocked`, the lock error in `error`, and `via` naming the caller — `hydrate` for `LookupOnResume`, `doctor` for `checkStaleHooks`, and the existing `cli` for `hook list`.

**Proposed Text**:
> The degraded read is the one genuinely new emission: DEBUG, `op=load-unlocked`, the lock error in `error`, and `via` naming the caller — `hydrate` for `LookupOnResume`, `doctor` for `checkStaleHooks`, the existing `cli` for `hook list`, and the existing `internal` for the sweep's advisory pre-read (§6.3), which degrades by the same rule and adds no value of its own.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. `internal` is the value the sweep's own emissions already carry, so the vocabulary tally is undisturbed. If finding 2 is applied, its rewrite of the following sentence composes with this one; the two edits touch adjacent sentences, not the same one.

---

### 6. A restored pane with no saved token — is `--hook-key` omitted or passed empty?

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §3.4 (An empty key is rejected at every boundary), §3.3 (`buildHydrateCommand`)

**Problem**:
§3.4 requires `collectArmInfos` not to bake an empty key and, separately, requires `LookupOnResume` to reject an empty argument — which reads as though the empty key both does and does not reach the helper. The concrete question a builder has to answer is what the `respawn-pane -k` command line looks like for an unstamped saved pane: `--hook-key ''` or no flag at all. Under lazy stamping that pane is the ordinary case rather than the exception — it is every pane that has never carried a hook — so if `--hook-key` is a required flag, the wrong choice fails the arm phase for most panes on the machine on the first reboot after upgrade.

**Proposal**:
Settle it at the command line, which is where "bake" means something: the flag is omitted for such a pane. Make `portal state hydrate` tolerate an absent flag as well as an empty value, since §3.4's second guard exists precisely for an empty value arriving by some route other than restore.

**Current**:
> - **`collectArmInfos` must not bake an empty key.** A saved pane with an empty `PortalPaneID` is armed with no hook — the pane restores and hydrates as normal, it simply has nothing to resume.

**Proposed Text**:
> - **`collectArmInfos` must not bake an empty key.** A saved pane with an empty `PortalPaneID` is armed with no hook — the pane restores and hydrates as normal, it simply has nothing to resume. Concretely, `buildHydrateCommand` omits the `--hook-key` flag entirely for that pane rather than passing an empty value, and `portal state hydrate` treats an absent flag and an empty one alike as "no hook". Under lazy stamping this is the ordinary pane, not an anomaly, so the flag cannot be required.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. Under lazy stamping the unstamped pane is the ordinary case, so a required flag would fail the arm phase for most panes on the machine on the first reboot.

---

### 7. Whether `hook rm` clears the pane's token is unstated

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Move**: settled
**Priority**: Minor
**Affects**: §4.2 (Removal verifies the same way)

**Problem**:
§4.2 says removal does not mint. It does not say whether removal *unstamps*. A builder tidying up after a removal would add a `set-option -up`, and that turns a benign reusable identity into a lost one: the next `hook set` on the pane mints a fresh token, the token saved in `sessions.json` goes stale until the next capture, and removal acquires a tmux write that can fail *after* the entry is already gone — an exit-code case the "exits 0 iff it removed an entry" rule does not cover.

**Proposal**:
State that removal leaves the stamp in place, on the reasoning §4.1 already gives for not rolling a stamp back: an orphan token costs nothing and is reused by the next registration on that pane.

**Current**:
> Removal does **not** mint. A pane with no token has no entry to remove; `hook rm` reports that — in Portal's own words, since the existence probe has already separated it from a gone pane — and exits non-zero rather than silently succeeding, which is the same silent-success shape as the `:.` bug on the write side.

**Proposed Text**:
> Removal does **not** mint, and it does **not** unstamp. A pane with no token has no entry to remove; `hook rm` reports that — in Portal's own words, since the existence probe has already separated it from a gone pane — and exits non-zero rather than silently succeeding, which is the same silent-success shape as the `:.` bug on the write side. A pane whose entry is removed keeps its token, for the reason §4.1 gives for not rolling back a stamp: the orphan costs nothing, the next registration on that pane reads it back and reuses it, and clearing it would add a tmux write that can fail after the entry is already gone.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. Symmetric with the no-rollback rule settled in cycle 1: an unstamp would add a tmux write that can fail after the entry is already gone, which the exit-status rule does not cover.

---

### 8. §3.3 restates the row-count / token-set split that §5.4 owns

**Source**: Specification analysis
**Category**: Duplication
**Move**: settled
**Priority**: Minor
**Affects**: §3.3 (`internal/tmux` surface changes)

**Problem**:
The rule that the mass-deletion guard counts pane rows while the stale comparison uses only the non-empty subset is stated in full in both §3.3 and §5.4. §5.4 is where it is decided and argued; the copy in §3.3 is what an edit to the guard has to remember to chase, and a copy that quietly stops agreeing comes back as a contradiction.

**Current**:
> Both properties are load-bearing: the stale comparison uses the rows with a non-empty token, while the mass-deletion guard counts rows (§5.4).

**Proposed Text**:
> Both properties are load-bearing — §5.4 has the rule that consumes them.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The surrounding sentence in §3.3 already states the return shape (one row per live pane, token possibly empty, plus location), which is what §3.3 owns; only the consuming rule moves out.

---

### 9. §5.2 restates §5.4's `checkStaleHooks` amendment

**Source**: Specification analysis
**Category**: Duplication
**Move**: settled
**Priority**: Minor
**Affects**: §5.2 (Deletion becomes shape-aware)

**Problem**:
How `checkStaleHooks` treats retained old-format entries is specified twice — as a consequence bullet in §5.2 and as the amendment itself in §5.4. Two statements of one rule means two places to edit if the check's treatment ever changes, and the second copy is the one nobody remembers.

**Current**:
> - **`portal doctor` stays green and keeps its "exit 0 iff all pass" contract.** A closed pane's entry is still deleted, so retained old-format entries are the only thing that persists — and `checkStaleHooks` (§5.4) is amended not to count them as failures.

**Proposed Text**:
> - **`portal doctor` stays green and keeps its "exit 0 iff all pass" contract.** A closed pane's entry is still deleted, so retained old-format entries are the only thing that persists — and §5.4 settles how the check treats them.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed.

---
