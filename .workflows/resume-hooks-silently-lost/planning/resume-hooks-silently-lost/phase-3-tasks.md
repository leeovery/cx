---
phase: 3
phase_name: Carry the token across the reboot gap, and retire @portal-id
total: 5
---

## resume-hooks-silently-lost-3-1

### Task 3-1: Saved state carries each pane's token

**Problem**: A tmux pane user-option does not survive the server dying — the exact constraint that made the April spec choose positional keys over pane ids in the first place. After Phase 2 a hook key is the pane's `@portal-pane-id` token, stamped live at `hook set`, and nothing carries it across a reboot: `captureFormat` (`internal/state/capture.go:26`) still captures the session-scoped `#{@portal-id}` into `Session.PortalID`, and `collectArmInfos` (`internal/restore/session.go:62`) still bakes `tmux.HookKey(sess.PortalID, …)`. A restore therefore recreates panes carrying no token at all, the baked key is a positional composite no live pane answers to, and every token-keyed entry on disk is reaped by the ordinary rule on the first sweep after `@portal-restoring` clears. Without this task the durable identity Phase 2 built survives exactly up to the next reboot and no further.

**Solution**: Swap `captureFormat`'s trailing column from `#{@portal-id}` to `#{@portal-pane-id}`, lift it **per-pane** into a new `Pane.PortalPaneID` field, delete the session-scoped `Session.PortalID` field with its lift and its restore re-stamp, and re-point restore's saved-state bake at `p.PortalPaneID`.

**Outcome**: Every capture records each pane's own token in `sessions.json` (empty for an unstamped pane, which under lazy stamping is the ordinary pane), an older `sessions.json` still decodes, and restore bakes the saved token as the pane's `--hook-key` — so the key on disk and the key restore fires against are the same value regardless of how tmux renumbered the windows.

**Do**:
- `internal/state/schema.go`: add `PortalPaneID string \`json:"portal_pane_id"\`` to `Pane`, and delete `PortalID` from `Session` together with the doc comment above `Session` that describes it. Leave `SchemaVersion` at `1` and write no migration — the addition and the removal are both tolerant-decode changes on a struct that already ignores unknown fields.
- `internal/state/capture.go`: replace `captureFormat`'s trailing `#{@portal-id}` column with the pane token, composed by constant concatenation from `PortalPaneIDOption` (declared in this package by task 2-1) rather than restated — the format must hold no `@portal-pane-id` literal of its own. Keep `captureFieldCount` at `11`: one column swaps for another. Rename `paneRow.portalID` to `paneRow.portalPaneID`, keep it reading `parts[10]`, and carry it into `Pane.PortalPaneID` inside `buildPanes`. Delete the session-scoped lift at `:80-88` (the `rows[0].portalID` read, its comment, and the `PortalID:` field on the appended `Session`) and the `PortalID:` copy in `findOrAppendSession` (`:174`). Leave `mergePane`'s whole-struct pane assignment exactly as it is — narrowing it to selected fields would drop a skipped pane's previously-captured token.
- `internal/restore/session.go`: `collectArmInfos` bakes `p.PortalPaneID` where it called `tmux.HookKey(sess.PortalID, sess.Name, w.Index, p.Index)`; delete `createSkeleton`'s `if sess.PortalID != "" { _ = r.Client.SetSessionOption(…, session.PortalIDOption, …) }` block (`:78-80`) with its comment, and drop the now-unused `internal/session` import if nothing else in the file needs it. Rewrite `savedPaneArmInfo`'s doc comment — it names the saved (window, pane) indices and the live-`@portal-id` ordering trap, neither of which survives — to say the key is a token read from saved state. Leave `tmux.HookKey` itself in place; 3-4 deletes it with the rest of the set.
- **Keep `buildHydrateCommand` passing `--hook-key` even when the baked token is empty.** `cmd/state_hydrate.go:298` still marks the flag required, and cobra's required-flag check tests whether the flag was `Changed`, not what value it carries — so `--hook-key ''` satisfies it. Task 3-3 is what omits the flag *and* drops `MarkFlagRequired`, and those two edits must land together there; omitting the flag here would fail the hydrate helper on every restored unstamped pane.
- Delete `internal/state/portal_id_literal_guard_test.go` whole (it asserts `captureFormat` contains the `@portal-id` literal; the single home in `internal/state` leaves it nothing to bind) and re-point the blocked fixtures: `cmd/state_daemon_run_test.go`'s `oneSession()` (`:207-212`) keeps its 11-field row and its trailing empty column, with the comment renamed to the pane token; `internal/state/capture_test.go`'s `paneLineWithID` and its `aB3xY9kZ` assertions become per-pane token fixtures; `internal/state/capture_internal_test.go`'s `TestFindOrAppendSessionCopiesPortalID` retires whole with the `PortalID:` copy in `findOrAppendSession` that it covers; `internal/state/schema_test.go` re-points its four session-identity cases at the pane field — the JSON-tag round-trip (`:96-127`) and the legacy-payload decode (`:130-157`) become `Pane.PortalPaneID` / `portal_pane_id` cases, the schema-version case (`:160-163`) keeps its `1` and names the additive `portal_pane_id` field, and the `Session` fixture at `:21` drops its `PortalID` — so the criteria in this task about decoding a pre-upgrade and a post-upgrade `sessions.json` are the ones these cases assert; `internal/restore/session_test.go`'s `sess.PortalID` fixtures and the `tmux.HookKey`-derived expectations become `Pane.PortalPaneID` fixtures; `internal/restore/rename_reboot_hook_integration_test.go`, `rename_reboot_durability_integration_test.go` and `rename_reboot_shared_test.go`'s `renamePortalID` scaffolding are re-pointed at a per-pane token (stamped with `SetPaneOption`, seeded in `hooks.json` under the token, and asserted to survive the rename and the reboot — the user-visible rename guarantee stays under test for a new reason); `internal/restore/multipane_legacy_integration_test.go`'s multipane coverage is re-pointed at per-pane tokens and its un-stamped-name-fallback subtests are retired with the branch they cover.

**Acceptance Criteria**:
- [ ] `state.Pane` carries `portal_pane_id`; `state.Session` carries no `PortalID`; `SchemaVersion` is unchanged at `1` and no migration code exists
- [ ] `captureFormat`'s trailing column is the pane token composed from `state.PortalPaneIDOption`, with no `@portal-pane-id` literal restated in `internal/state/capture.go`
- [ ] `captureFieldCount` is still `11`, and a pane row with 10 or 12 fields still errors
- [ ] The token is lifted per-pane: two panes of one session carrying different tokens capture as two different `Pane.PortalPaneID` values
- [ ] An unstamped pane captures `""` with no error and no log line
- [ ] A pane in `skipSet` keeps its previously-captured `PortalPaneID` from `prev` rather than the freshly-read empty one
- [ ] A `sessions.json` written before the upgrade (carrying `portal_id`, no `portal_pane_id`) decodes without error, at the same schema version, with every `Pane.PortalPaneID` empty
- [ ] `collectArmInfos` bakes `p.PortalPaneID` and nothing else; no call to `tmux.HookKey` remains in `internal/restore`
- [ ] `createSkeleton` issues no session-scoped `@portal-id` stamp
- [ ] `buildHydrateCommand` still emits `--hook-key ''` for an empty baked token, so `portal state hydrate`'s required-flag check is still satisfied
- [ ] `internal/state/portal_id_literal_guard_test.go` is deleted, not re-pointed, and no replacement guard is added
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it captures a stamped pane's token into that pane's record"` — fake `Commander` returning an 11-field row with a token in the trailing column
- `"it captures different tokens for two panes of one session"` — the property the session-scoped lift could not express
- `"it captures an empty token for an unstamped pane"` — trailing column empty; no error, no anomaly
- `"it rejects a pane row with the wrong field count"` — 10 and 12 fields both error, unchanged
- `"it keeps a skipped pane's previously captured token"` — `skipSet` plus a `prev` index carrying a token; the merged pane keeps it
- `"it decodes a sessions.json written before the upgrade"` — `portal_id` present, `portal_pane_id` absent; no error, empty tokens, version unchanged
- `"it decodes a sessions.json written after the upgrade"` — `portal_pane_id` round-trips through encode/decode
- `"it bakes the saved pane token as the hydrate hook key"` — `Pane.PortalPaneID` set; assert the rendered `--hook-key`
- `"it bakes an empty hook key for a saved pane with no token"` — assert `--hook-key ''` is still rendered in this task
- `"it issues no session option stamp during skeleton creation"` — assert no `set-option -t <session> @portal-id` in the recorded argv
- `"it keeps the daemon capture fixture parsing at eleven fields"` — the re-pointed `oneSession()` still drives the daemon run tests

**Edge Cases**:
- `captureFieldCount` stays `11` — a fixture fabricating a pane row keeps its arity while the trailing column's meaning changes; a fixture that "helpfully" drops or adds a column silently changes what is under test
- The lift moves from session-scoped (`rows[0].portalID`, repeated on every pane row of a session) to genuinely per-pane in `buildPanes`; `findOrAppendSession`'s `PortalID` copy goes with it
- `mergeSkippedPanes` copies the whole `Pane` struct, so a skipped pane keeps its previously-captured token rather than re-reading an empty one — the merge must not be narrowed to selected fields
- An unstamped pane captures `""` and that is the ordinary case under lazy stamping, not an anomaly to warn about
- `SchemaVersion` is unchanged in both directions: an existing `sessions.json` carrying `portal_id` decodes fine and the value is ignored; one written before the upgrade decodes `portal_pane_id` empty
- Deleting `Session.PortalID` compile-breaks `internal/restore/session.go` at both sites, which is why `collectArmInfos` and `createSkeleton` are edited in this task rather than a later one
- `tmux.HookKey` becomes production-dead here but stays until 3-4 deletes it with the rest of the `@portal-id` set
- A pre-upgrade daemon keeps capturing the pre-upgrade format until a bootstrapping command replaces it, so a server death in that lag leaves no saved token for any pane — accepted by the specification, no code
- `@portal-dir` is untouched: a separate stamp serving TUI session grouping, unrelated to hook identity

**Context**:
> The schema change is additive on a tolerant-decode struct, and the removal is its mirror image: no `SchemaVersion` bump, no migration, in either direction.
>
> The column swaps rather than appends because the field count is what parsing validates: `#{@portal-id}` was session-scoped and repeated on every pane row of a session, while `#{@portal-pane-id}` is genuinely per-pane. Same arity, different meaning.
>
> The `@portal-id` re-stamp in `createSkeleton` existed so the next capture would not write `""` and let stale-cleanup re-key the session by name. That whole failure mode belongs to a key scheme that no longer exists — the pane token's own re-stamp, which is a different call against a different target, is task 3-2.
>
> The firing path must never read a live tmux value: the baked key is a pure function of saved state, so firing stays correct independent of the re-stamp's timing.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §2.3 (schema, capture and restore across the reboot gap), §3.3 (the saved-state bake is a struct field read, not a formatting call), §7.2 (what is removed and why the field removal needs no migration), §9.3/§9.4 (fixtures to re-point; both literal-binding guards deleted rather than re-pointed).

## resume-hooks-silently-lost-3-2

### Task 3-2: Restore re-stamps the token before it arms a pane

**Problem**: Task 3-1 gets the token into `sessions.json` and back out again as the baked `--hook-key`, but the live pane restore recreates carries no `@portal-pane-id` at all — a tmux pane option dies with the server. The entry fires once on that boot and is then reaped: the post-restore sweep enumerates live panes, finds no pane answering to the token, and deletes the entry under the ordinary rule. That is the reboot boundary the whole work unit exists to close, and only re-establishing the option on the live pane closes it. The re-stamp cannot be modelled on the session stamp it replaces, either: that one discarded its error (`_ = r.Client.SetSessionOption(...)`) because a missed `@portal-id` cost only rename-immunity, whereas for the pane token the stamp **is** the identity — a swallowed failure permanently orphans that pane's hook with no trace anywhere.

**Solution**: In `armPanes`, immediately before each paired pane's `respawn-pane -k`, write the saved token onto that same live pane with `SetPaneOption`, skipping a saved pane whose token is empty and surfacing a failure as one WARN under the `restore` component without aborting the restore.

**Outcome**: After a restore every pane that carried a hook answers to the same token its `hooks.json` entry is keyed by, so the entry survives the post-restore sweep and the hook keeps firing on every subsequent reboot; a pane with no saved token is left untouched and silent; and a genuine tmux failure produces one actionable WARN naming the session and the pane's live location.

**Do**:
- In `internal/restore/session.go`, have `savedPaneArmInfo` carry the saved token in the single field the `--hook-key` bake already reads — the hook key and the token are the same value by construction, and a second field would let them drift. Rename the field if `hookKey` no longer describes it, and keep the doc comment 3-1 rewrote accurate.
- Inside `armPanes`'s existing `for i := range pairCount` loop, after `state.CreateFIFO` and **before** `r.Client.RespawnPane(liveTarget, hydrateCmd)`, when the token is non-empty call `r.Client.SetPaneOption(liveTarget, state.PortalPaneIDOption, token)` against the **same** `liveTarget` that iteration respawns.
- On a non-nil error from the stamp, emit `r.logger().Warn("set pane token failed", "session", sess.Name, "pane_key", liveKey, "error", err)` — the exact shape and attr set of the `set skeleton marker failed` emission at `:274` — and then carry on with the respawn. Do not return, do not wrap into the abort paths the FIFO and respawn failures take.
- When the saved token is empty, issue no tmux call and log nothing at all. Writing `@portal-pane-id ""` would stamp a value indistinguishable on read-back from absence onto most panes on the machine.
- Add no second loop and no separate stamping pass: the existing `pairCount` bound is what leaves the unpaired remainder untouched when live and saved pane counts differ.
- Cover the new behaviour in `internal/restore/session_test.go` against the existing recording `Commander` mock, asserting the recorded argv sequence rather than only the end state.

**Acceptance Criteria**:
- [ ] Each paired pane with a non-empty saved token receives exactly one `set-option -p` carrying `state.PortalPaneIDOption` and that token
- [ ] The stamp is issued against the same live pane target as that iteration's `respawn-pane`, and precedes it in the recorded argv sequence
- [ ] A saved pane whose token is empty produces no `set-option` call and no log record of any kind
- [ ] A failed stamp produces exactly one WARN under the `restore` component with message `set pane token failed` and the attrs `session`, `pane_key`, `error` — no new component, no new attr key
- [ ] `pane_key` carries the live structural key (`state.SanitizePaneKey(sess.Name, live.Window, live.Pane)`), never the token
- [ ] A failed stamp does not abort the restore: that pane is still armed, later panes are still processed, and `Restore` returns the live pane coords with no error
- [ ] With live and saved pane counts differing in either direction, only panes within `pairCount` are stamped and the remainder carries no token
- [ ] A restore in which every saved pane is unstamped emits no WARN — the unstamped majority is silent by construction
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it stamps each saved token onto its paired live pane"` — two saved panes with distinct tokens; assert two `set-option -p` calls with the right targets and values
- `"it stamps before it arms the pane"` — assert the `set-option -p` for a pane precedes that pane's `respawn-pane` in the recorded sequence
- `"it stamps nothing for a saved pane with an empty token"` — assert zero `set-option -p` calls and an empty log capture
- `"it warns and continues when the stamp fails"` — mock fails the `set-option -p`; assert one WARN with `session`/`pane_key`/`error`, that the respawn still ran, and that `Restore` returned no error
- `"it names the live structural key in pane_key"` — restore under renumbered live coords; assert `pane_key` matches the live key, not the saved one and not the token
- `"it keeps stamping the remaining panes after one fails"` — first stamp fails, second still issued
- `"it stamps only the paired prefix when more live panes than saved"` — 3 live / 2 saved; the third pane gets no `set-option`
- `"it stamps only the paired prefix when more saved panes than live"` — 2 live / 3 saved; exactly two stamps
- `"it emits no warn on a boot where every saved pane is unstamped"` — the ordinary upgrade-window restore

**Edge Cases**:
- A saved pane whose `PortalPaneID` is empty is skipped entirely — no `set-option` and nothing logged; writing an empty value would be indistinguishable on read-back from absence, on most panes on the machine
- Because an empty token never reaches the stamp, the WARN can only fire for a genuine tmux failure — it is never a per-boot report on the unstamped majority
- Only panes within `pairCount` are stamped: when counts differ `armPanes` pairs up to the shorter list, and a durable token written onto the wrong pane does not self-correct the way a misplaced FIFO does — it fires the hook on the wrong pane on every later reboot
- The failure must not be swallowed the way the session stamp was: for the pane token the stamp *is* the identity, so a discarded error permanently orphans that pane's hook with no trace
- A stamp failure does not abort the restore — an unstamped pane is a lost hook, not a lost session — unlike the FIFO and respawn failures in the same loop, which do abort
- The stamp must address the same live target as that iteration's respawn, or the token lands on a different pane than the one being armed
- `respawn-pane -k` preserves the pane option (pinned by task 2-5), so the ordering is about the hydrate helper firing against a stamped pane, not about the stamp surviving
- The daemon's capture is suppressed by `@portal-restoring` across this window, so no capture observes the pre-stamp panes

**Context**:
> The re-stamp is what makes the token durable across repeated reboots: `sessions.json` is a snapshot the daemon regenerates from live tmux, so a token that is not re-established on the live pane is captured as `""` on the next tick and the entry is orphaned all over again.
>
> The WARN reuses restore's existing `set skeleton marker failed` shape exactly, so no new component and no new attr key enter the closed logging vocabulary. `pane_key` carries the live structural key because the token that failed to land is not a location — an operator needs to know *which pane* lost its identity.
>
> This is the only re-stamp: the session-scoped `@portal-id` re-stamp was deleted in 3-1 and is not replaced.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §2.3 (restore re-stamps the saved token before arming; an empty token is skipped), §2.4 (failures are surfaced, mispairings are not stamped), §9.2 (restore re-stamp failures are surfaced; `armPanes` short-list stamps nothing).

## resume-hooks-silently-lost-3-3

### Task 3-3: An empty key fires on nothing

**Problem**: Under lazy stamping most panes carry no token, so most saved panes restore with an empty `PortalPaneID` — and after 3-1 that empty value is baked straight into `--hook-key ''`. `hooks.LookupOnResume` (`internal/hooks/lookup.go`) does a bare map index on whatever key it is handed, so a single `""` entry in `hooks.json` would fire its command on **every unstamped restored pane on the machine**. Registration cannot write one after Phase 2, but a hand-edit or a bug in the out-of-band conversion script still can, and neither the firing path nor the bake currently refuses an empty key. `cmd/state_hydrate.go:298` also marks `--hook-key` **required**, which is why 3-1 had to keep passing an empty value rather than omitting the flag.

**Solution**: Two independent guards — `buildHydrateCommand` omits the `--hook-key` flag entirely for a pane with no saved token (with `MarkFlagRequired("hook-key")` dropped in the same task so an absent flag is legal), and `LookupOnResume` returns "no hook" for an empty key before `hooks.json` is consulted at all.

**Outcome**: An unstamped pane restores and hydrates exactly as it does today — it simply has nothing to resume — and a `""` entry in `hooks.json`, however it got there, fires on nothing.

**Do**:
- In `internal/restore/session.go`, change `buildHydrateCommand` so an empty `hookKey` renders `portal state hydrate --fifo <q> --file <q>` with no `--hook-key` segment at all, and a non-empty one renders exactly as it does today. Retain `shellQuoteSingle` on all three values on the non-empty path; the token charset carries no shell metacharacters, so the boundary becomes strictly safer rather than needing new care.
- In `cmd/state_hydrate.go`, delete `_ = stateHydrateCmd.MarkFlagRequired("hook-key")` (`:298`), leaving `--fifo` and `--file` required, and correct the flag's help text from `Saved structural identifier (<session>:<window>.<pane>)` to name the pane token it now carries. **These two edits land together with the flag omission above** — 3-1 deliberately kept the flag passed with an empty value because cobra's required check tests `Changed` rather than the value, so omitting the flag while `MarkFlagRequired` still stands fails the hydrate helper on every restored unstamped pane.
- In `internal/hooks/lookup.go`, return `("", false, nil)` for an empty `hookKey` before `store.Load` is called, so the file is neither read nor indexed. Rewrite the doc comment: its claim that `hookKey` is a raw saved identifier whose colons round-trip verbatim describes a key form that no longer exists.
- Introduce no trimming and no whitespace special case: a whitespace-only key is not empty, is looked up literally, and matches nothing unless something seeded it.
- Re-point the blocked assertions: `internal/restore/session_build_hydrate_test.go:19` (an empty key now omits the flag rather than rendering `--hook-key ''`), `internal/restore/session_test.go`'s `extractHookKey` helper (which currently fails on a hydrate command with no `--hook-key` marker and must answer "no flag" instead), and `cmd/state_test.go:145`'s hydrate invocation. Add absent-flag / empty-flag equivalence coverage in `cmd/state_hydrate_test.go`.

**Acceptance Criteria**:
- [ ] `buildHydrateCommand` emits no `--hook-key` token anywhere in its output for an empty key, and the rest of the command line is unchanged
- [ ] `buildHydrateCommand` emits a single-quoted `--hook-key '<token>'` for a non-empty key, unchanged from today
- [ ] `portal state hydrate --fifo <f> --file <s>` with no `--hook-key` parses and runs; `--fifo` and `--file` remain required
- [ ] An absent `--hook-key` and an empty `--hook-key ''` produce identical behaviour on every hydrate path
- [ ] `LookupOnResume` with an empty key returns `("", false, nil)` even when `hooks.json` holds a `""` entry whose `on-resume` is set
- [ ] `LookupOnResume` with an empty key reads no file: an unreadable or absent `hooks.json` still returns a clean miss with no error
- [ ] A non-empty key behaves exactly as it does today, including the load-error and miss paths
- [ ] A whitespace-only key is not special-cased — it is looked up literally
- [ ] The `--hook-key` help text describes the pane token
- [ ] Restoring a saved pane with no token still creates its FIFO, arms it and hydrates it; only the hook is absent

**Tests**:
- `"it omits the hook-key flag for a pane with no saved token"` — assert the rendered command contains no `--hook-key`
- `"it renders a quoted hook-key flag for a non-empty token"` — unchanged rendering
- `"it accepts a hydrate invocation with no hook-key flag"` — cobra-level: the command parses and `--fifo` / `--file` are still enforced
- `"it treats an absent hook-key and an empty one identically"` — drive both, assert the same exec target and the same `hook lookup` result
- `"it returns no hook for an empty key even when hooks.json holds an empty-key entry"` — seed `{"": {"on-resume": "rm -rf /"}}`; assert a miss
- `"it reads no file for an empty key"` — store pointed at an unreadable path; assert `("", false, nil)`, not an error
- `"it still returns the hook for a non-empty key"` — the unchanged happy path
- `"it does not trim a whitespace-only key"` — `" "` misses unless seeded, and hits when seeded under `" "`
- `"it hydrates an unstamped restored pane to a bare shell"` — arm a saved pane with no token; the pane restores, replays and execs `$SHELL` with `hook_present=false`
- `"it renders an empty hook_key in the timeout warn"` — the existing timeout WARN carries an empty `hook_key` with no added branch

**Edge Cases**:
- Both guards are independently required: `hook set` cannot write an empty key after Phase 2, but a hand-edit or a bug in the out-of-band conversion script still can, and the bare map index would then fire that one command on every unstamped restored pane on the machine
- `--hook-key` stops being a required flag while `--fifo` and `--file` stay required; the flag omission and the `MarkFlagRequired` drop are one edit and must not be split across tasks
- An absent flag and an empty one must behave identically — the helper is also driven by hand from tests, which pass an explicit empty value
- An unstamped pane still restores and hydrates normally; it simply has nothing to resume, which under lazy stamping is the ordinary pane and is exactly why the flag cannot be required
- A whitespace-only key is not empty and is not special-cased: it matches nothing, and no trimming is introduced anywhere
- `shellQuoteSingle` is retained on the non-empty path — the token charset carries no shell metacharacters, so this boundary becomes strictly safer rather than needing new care
- `LookupOnResume`'s doc comment about colons in a session name round-tripping verbatim describes a key form that no longer exists
- The hydrate timeout WARN names the hook key (`cmd/state_hydrate_test.go:1028`) and simply renders it empty — no branch is added for it
- Returning before `store.Load` means an empty key with an unreadable `hooks.json` now reports a miss rather than a load error; the end state (bare shell) is identical, and any existing test driving a load error must use a non-empty key

**Context**:
> `LookupOnResume` does a bare map index on the key, so one `""` entry is a machine-wide command execution. §4 stops the CLI writing one; these two guards stop anything else that manages to.
>
> Concretely, `buildHydrateCommand` omits the flag rather than passing an empty value, and `portal state hydrate` treats an absent flag and an empty one alike as "no hook".
>
> The out-of-band conversion script that re-keys the existing `hooks.json` is outside this work unit and no task covers it — these guards exist precisely because that script is not code this plan controls.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §3.4 (an empty key is rejected at every boundary; both guards required), §3.3 (`portal state hydrate --hook-key` keeps its flag and its meaning, with corrected help text; `shellQuoteSingle` retained), §9.2 (an empty key fires on nothing).

## resume-hooks-silently-lost-3-4

### Task 3-4: The `@portal-id` machinery goes

**Problem**: `@portal-id` exists for exactly one reason — so a session rename cannot orphan a resume hook — and a token-only key carries no session identity at all, so renames are now irrelevant by construction. Every non-test consumer of the option exists to build the hook key and nothing else, and after 3-1 every one of them is dead: the session stamp in `CreateFromDir`, the `set-option` link in `QuickStart`'s chained `ExecArgs`, `tmux.HookKey`, and `portal state migrate-rename`, whose whole job is rewriting hook keys by `<oldName>:` prefix — a shape no key can carry any more, so it can match nothing. Retaining it would leave two identity systems, one inert, with source comments cross-referencing a key format that no longer exists.

**Solution**: Delete the `@portal-id` option constant, both stamps, `tmux.HookKey` and the `migrate-rename` subcommand, keep `migrateRenameSubstring` (which reaps hooks older binaries installed), and rewrite CLAUDE.md's remaining `@portal-id` passages to describe what 3-1 and 3-2 built instead.

**Outcome**: No `PortalIDOption`, no `@portal-id` stamp, no `Session.PortalID`, no `tmux.HookKey` and no `portal state migrate-rename` remain anywhere in the repository; `portal uninstall` still reaps the inert `session-renamed` hooks on the user's tmux server; and CLAUDE.md describes the pane-token scheme end to end with zero `@portal-id` occurrences.

**Do**:
- `internal/session/create.go`: delete the `PortalIDOption` const with its doc comment and the `if token, genErr := sc.gen(); genErr == nil { … }` stamp in `CreateFromDir` (`:90-93`). Amend the "Both stamps are best-effort and swallowed" comment above them to describe the one remaining `@portal-dir` stamp. Keep the `IDGenerator` seam, `sc.gen` and `PortalDirOption` — the generator still produces session names.
- `internal/session/quickstart.go`: delete the `idToken, idGenErr := qs.gen()` line with its comment and the `";", "set-option", … PortalIDOption, idToken` link from `ExecArgs`. Rewrite `Run`'s ordering comment to name `@portal-dir` alone; creating detached is still what gives the in-server stamp point that `new-session -A` lacked, so the load-bearing claim stays, only its subject narrows.
- `internal/tmux/tmux.go`: delete `HookKey` (`:398`) and its doc comment — its only production caller was replaced in 3-1 — and delete `TestHookKey` from `internal/tmux/hookkey_test.go`, removing the file if nothing else remains in it.
- Delete `cmd/state_migrate_rename.go` and `cmd/state_migrate_rename_test.go` whole. Drop `stateMigrateRenameCmd` from `stateChildCommands` (`cmd/state_test.go:243`) and `migrate-rename` from the enumerations at `:87`, `:112`, `:145`, `:278` and `:294`, including the "each of the six child command vars" subtest name, which becomes five. Leave `migrateRenameSubstring` (`internal/tmux/hooks_register.go:45`) and its comment exactly as they are, along with its `teardownFingerprints` consumption at `:64-67`.
- Rewrite CLAUDE.md's remaining `@portal-id` passages in one pass: the `session` row's two-stamp description and its QuickStart chain (both now `@portal-dir` only); the `state` row, which loses `Session.PortalID` and the `#{@portal-id}` capture column and gains what 3-1 and 3-2 built — `Pane.PortalPaneID` (`json:"portal_pane_id"`, additive, tolerant-decode, no `SchemaVersion` bump), the trailing `#{@portal-pane-id}` `captureFormat` column lifted per-pane at unchanged arity, and restore's per-pane re-stamp; the `tmux` row's `HookKey` clause; and the "Resume hooks" section's remaining re-stamp and "must never read the live `@portal-id`" claims. End with zero `@portal-id` occurrences while leaving `@portal-dir`, `@portal-pane-id`, `@portal-skeleton-`, `@portal-restoring` and `@portal-spawn-` intact — a blanket find-and-replace would corrupt all five.
- Re-point `internal/session/create_test.go` and `quickstart_test.go`: the `ExecArgs` ordering subtests keep their `@portal-dir`-before-`attach-session` assertion, and the `@portal-id` ones retire with the stamp.

**Acceptance Criteria**:
- [ ] `grep -rn "PortalIDOption\|@portal-id\|PortalID" internal cmd --include="*.go"` returns nothing, in test and non-test source alike
- [ ] `CreateFromDir` issues exactly one `SetSessionOption` call, for `@portal-dir`, and still returns a generated session name
- [ ] `QuickStart`'s `ExecArgs` chain is `new-session -d … ; set-option … @portal-dir … ; attach-session …` with no `@portal-id` link, and `@portal-dir` still precedes `attach-session`
- [ ] `tmux.HookKey` does not exist; nothing in the repository calls it
- [ ] `portal state migrate-rename` does not resolve, is absent from `stateChildCommands`, and the state-surface tests enumerate five hidden children
- [ ] `migrateRenameSubstring` and its comment are unchanged, and `portal uninstall` still reaps a legacy `session-renamed` hook through `teardownFingerprints`
- [ ] No replacement literal-binding guard is added: both `@portal-id` guards are gone (`cmd/portal_id_binding_guard_test.go` in 2-2, `internal/state/portal_id_literal_guard_test.go` in 3-1) and neither is re-pointed
- [ ] CLAUDE.md contains zero occurrences of `@portal-id`, `PortalID` and `portal_id`, still contains `@portal-dir`, `@portal-pane-id`, `@portal-skeleton-`, `@portal-restoring` and `@portal-spawn-`, and its `state` row describes `Pane.PortalPaneID`, the per-pane capture column and the restore re-stamp
- [ ] `go build ./...`, `go test ./...` and `go test -tags integration -p 1 ./...` all pass

**Tests**:
- `"it stamps only @portal-dir at session creation"` — assert exactly one `SetSessionOption` call and its option name
- `"it still generates a session name from the injected generator"` — the `IDGenerator` seam survives the stamp's removal
- `"it emits no @portal-id link in the QuickStart ExecArgs"` — assert the full argv chain
- `"it keeps @portal-dir before attach-session in the ExecArgs chain"` — the retained ordering assertion
- `"it omits the stamp link when the directory is unavailable"` — the existing degradation subtests still hold for the one remaining stamp
- `"it does not resolve portal state migrate-rename"` — `rootCmd.Find([]string{"state", "migrate-rename"})` does not return that command
- `"it registers exactly five hidden state children"` — the re-pointed surface enumeration
- `"it keeps the legacy session-renamed teardown fingerprint"` — `portal uninstall` still matches a hook body carrying `migrateRenameSubstring`

**Edge Cases**:
- `migrateRenameSubstring` is **not** removable and its comment stays — it exists so `portal uninstall` reaps the inert `session-renamed` hooks that older binaries installed on the user's tmux server, a job that outlives the command
- `managedEvents` binds `session-renamed` to `notifyCommand`, so deleting the subcommand changes no registration and needs no hook-table migration
- `cmd/state_test.go` enumerates the `state` subcommand set at `:87`, `:112`, `:145`, `:243`, `:278` and `:294`; every one must drop `migrate-rename` or the command-surface tests fail on a phantom, and the "six child command vars" subtest name must follow
- The `IDGenerator` seam and `sc.gen` are still used for session-name generation, so only the stamp call is deleted, never the generator
- `quickstart.go`'s detached-create comment is load-bearing and is rewritten rather than deleted: creating detached is still what gives the stamp point `new-session -A` lacked, now for `@portal-dir` alone
- `@portal-dir` is untouched — a separate stamp serving TUI session grouping, unrelated to hook identity
- A blanket `@portal-id` find-and-replace across CLAUDE.md corrupts `@portal-dir`, `@portal-pane-id`, `@portal-skeleton-`, `@portal-restoring` and `@portal-spawn-`; the passages are rewritten by hand
- The `state` row in CLAUDE.md gains what 3-1 and 3-2 built rather than being merely deleted — otherwise the architecture description loses the durable-identity mechanism entirely

**Context**:
> The deciding argument is supersession, not dead weight: a token-only key carries no session identity, so renames become irrelevant by construction — A subsumes the July fix's purpose, not merely its machinery. Retaining an inert second identity system with comments cross-referencing a dead key format is the "ship it and remember to delete it later" pattern the specification rejects.
>
> Replacement and removal ship in one release. Accepted cost: a misbehaving release cannot be bisected between "the new key works" and "the old machinery is gone". This is deliberate — it is a single-install tool that can be rolled back wholesale.
>
> The hazard the deleted guards encoded is answered by construction instead of by assertion: `@portal-pane-id` is written as a literal in exactly one place, so a drift the compiler makes impossible cannot be introduced by someone who never ran the guard. Do not add a replacement guard.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §7.1 (why it goes and why now), §7.2 (the removal table), §7.3 (`migrate-rename` goes; `migrateRenameSubstring` stays), §7.4 (replacement and removal ship together), §3.3 (the CLAUDE.md rewrite), §9.4 (both guards deleted rather than re-pointed).

## resume-hooks-silently-lost-3-5

### Task 3-5: A reboot that renumbers windows keeps its hooks

**Problem**: The reboot boundary is the one moment of the defect no unit test can reach, and it is the moment that was latent on the reporting install: restore recreates each extra window with `NewWindow(target, …)` and passes no index, so tmux assigns the next free one — and where the saved window indices are non-contiguous, the restored ones differ. Under the old key the hook fired correctly exactly once and then died, because the entry named a coordinate no restored pane answered to. Nothing in the repository exercises that path: the existing reboot round-trips restore contiguous indices, so a regression that reintroduced any positional component would pass every test in both lanes. Tasks 3-1 and 3-2 claim the boundary is closed; only an end-to-end reboot with divergent indices can show it.

**Solution**: Add an integration-lane test that saves a session whose window indices are non-contiguous, kills the server, restores it under `renumber-windows off`, proves the restored indices diverge from the saved ones, and asserts each pane's own hook fires and its entry survives the post-restore sweep alongside a genuinely stale entry that is reaped.

**Outcome**: The reboot boundary is pinned by executable coverage — divergent window indices, hooks firing on the right panes, entries surviving the sweep that used to eat them — and the positional siblings (FIFO paths and `@portal-skeleton-*` markers) are observed under the same renumbering rather than assumed unaffected.

**Do**:
- Add an integration-tagged test file in **package `cmd`** (`//go:build integration`), which is where both a real restore and the unexported `runHookStaleCleanup` are reachable — `cmd/rename_restore_cleanup_survival_integration_test.go` is the precedent. Set it up with `tmuxtest.SkipIfNoTmux`, `portaltest.IsolateStateForTest(t)`, `portaltest.RegisterStateDirTeardownGuard`, a per-test `-L` socket via `tmuxtest.New`, `PORTAL_STATE_DIR` / `PORTAL_HOOKS_FILE` overrides, and a binary staged through `restoretest.BuildPortalBinaryDir` + `PrependPATH` (never a hand-rolled `go build` — the daemon-pgrep sandbox tag must not be dropped).
- Build the divergent fixture: set `renumber-windows off` explicitly on the fixture server, create a session with three windows, add a second pane to the last window, then `kill-window` the middle one so the live window indices are non-contiguous. Stamp `@portal-pane-id` on one pane in the first window and one pane in the last (`client.SetPaneOption`), leave the sibling pane in the last window unstamped, then capture through `state.CaptureStructure` so the saved tokens come from the real stamps.
- Seed `hooks.json` with one entry per stamped token whose command appends a distinct marker line to that pane's **own** file, plus one genuinely stale token-shaped key naming no live pane. Seed a scrollback file per pane and persist the index.
- Kill the server, `EnsureServer`, set `renumber-windows off` again on the new server (a killed server loses its options, and the assertion must not rest on tmux's default), restore under `@portal-restoring` set-then-cleared, then `restoretest.DriveSignalHydrate` and `restoretest.WaitForSkeletonMarkersCleared`. Poll for each expected marker file under a bounded wait — never a fixed sleep.
- Assert, in order: the restored window indices **differ** from the saved ones (fail the test outright if they match — a restore that happened not to renumber proves nothing); each stamped pane's own marker file exists with exactly its own payload and no other; the unstamped pane is live, hydrated and fired nothing; `@portal-restoring` is unset before the sweep runs; and after `runHookStaleCleanup(client, store, nil, nil)` both token-keyed entries survive while the seeded stale key is gone.
- Assert the positional siblings under the same renumbering: every restored pane's `@portal-skeleton-*` marker was keyed by its **live** structural key and all of them cleared, so FIFO and marker pairing is observed under divergent indices rather than assumed.

**Acceptance Criteria**:
- [ ] The fixture's saved window indices are non-contiguous and the test fails if they are not
- [ ] `renumber-windows off` is set explicitly on both server lifetimes rather than inherited
- [ ] The restored window indices are asserted to differ from the saved ones
- [ ] Each stamped pane's hook fires exactly once, into that pane's own marker file, with no cross-firing between panes
- [ ] The unstamped pane restores, hydrates and fires nothing
- [ ] `@portal-restoring` is confirmed unset before the sweep runs, so the sweep is not standing down under `reason=restoring`
- [ ] After the sweep both token-keyed entries are still in `hooks.json` and the seeded stale token-shaped key is gone
- [ ] Every restored pane's skeleton marker is keyed by its live structural key and all markers clear after hydration
- [ ] The test carries `//go:build integration`, builds its binary through `restoretest` / `portalbintest`, runs under `portaltest.IsolateStateForTest` and a per-test `-L` socket, and touches no default tmux server
- [ ] Hydration is awaited by bounded polling, with no fixed sleep anywhere in the test

**Tests**:
- `"it restores a session whose saved window indices are non-contiguous"` — the session and every pane come back
- `"it renumbers the restored windows away from the saved indices"` — the divergence guard; the rest of the suite is meaningless without it
- `"it fires each pane's own hook after the reboot"` — per-pane marker files, one line each
- `"it fires no hook on the pane that carries no token"` — the unstamped sibling hydrates to a bare shell
- `"it keeps both token-keyed entries across the post-restore sweep"` — survival comes from liveness, not from shape retention
- `"it still reaps a genuinely stale token-shaped key in the same sweep"` — the reaper's correctness is not weakened by the fix
- `"it runs the sweep only after the restore marker clears"` — assert the marker is unset before the sweep call
- `"it pairs skeleton markers with the renumbered live coordinates"` — the positional siblings observed under the renumbering

**Edge Cases**:
- `renumber-windows off` must be set explicitly rather than inherited — it is tmux's default but not the reporting user's setting, and it is what makes the restored indices diverge; a killed server loses the option, so it is set again on the second lifetime
- The saved-vs-restored divergence is asserted, not assumed: a restore that happened not to renumber proves nothing, and this is the case that fires correctly exactly once today and then dies
- Survival must be proven to come from liveness rather than from Phase 1's shape retention, which is why a genuinely stale token-shaped key is seeded alongside and asserted reaped in the same sweep
- Each pane's hook writes its own marker file, so a hook firing on the wrong pane fails the test rather than passing it
- The sweep must run after `@portal-restoring` clears, or it stands down under `reason=restoring` and the test passes for the wrong reason
- Hydration is awaited by polling for the marker files under a bounded wait, never a fixed sleep
- The lane is integration because arming execs `portal state hydrate` through `respawn-pane -k`; the binary is built through `restoretest` / `portalbintest` so the daemon-pgrep sandbox tag is never dropped, under `portaltest.IsolateStateForTest` and a per-test `-L` socket
- The same restore is the second surface for the positional siblings — `state.SanitizePaneKey`'s FIFO paths and `@portal-skeleton-*` markers — which are checked under the renumbering rather than assumed unaffected
- A session whose panes carry no token must still restore and hydrate in the same run, so the fixture holds both a stamped and an unstamped pane

**Context**:
> Restore recreates each extra window with `NewWindow(target, …)` where `target` is `"<session>:"` and no index is passed, so tmux assigns the next free index. Under the old key the baked key still fired — `collectArmInfos` derives it purely from saved state — but no live pane answered to it afterwards and the sweep reaped it. The hook survived exactly one reboot. This was latent on the reporting install, whose `renumber-windows on` kept saved and restored indices contiguous; it is a general defect on tmux's default settings.
>
> The reboot boundary closes because restore re-establishes the token itself: the key on disk and the key the live pane answers to are the same value regardless of how tmux renumbered the windows, so nothing needs re-keying and the post-restore sweep finds the entry live.
>
> The positional siblings are not changed by this work — they live for the duration of one bootstrap and are rebuilt from live coordinates each time — but the addressing assumption is identical, so their existing coverage is run against the change rather than assumed unaffected. This test is the surface that observes them.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §1.1 (the reboot boundary and the non-contiguous-index case), §3.1 (why the reboot boundary closes under a token key), §9.1 (lane placement), §9.2 (non-contiguous saved window indices), §9.5 (the positional siblings are checked, not assumed separate).
