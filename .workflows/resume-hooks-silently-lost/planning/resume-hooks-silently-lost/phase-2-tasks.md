---
phase: 2
phase_name: The pane token becomes the hook key
total: 5
---

## resume-hooks-silently-lost-2-1

### Task 2-1: The live enumeration answers with pane tokens

**Problem**: `Client.ListAllPaneHookKeys` (`internal/tmux/tmux.go:584`) renders `HookKeyFormat`'s positional composite — `<@portal-id or session_name>:window.pane` — once per live pane, and that flat `[]string` is the only thing the daemon sweep and `portal doctor` compare `hooks.json` against. The hook key is becoming a pane's `@portal-pane-id` token, and the enumeration has to answer in that vocabulary **before** anything writes one. It also has to stop being a bare key list: the mass-deletion guard's question is *"did the tmux read succeed?"*, which only a count of **pane rows** answers — under lazy stamping zero stamped panes is the ordinary steady state (every pane before its first `hook set`, and the whole install during the upgrade window), so a guard counting tokens would fire a mass-deletion WARN every 10s naming a hazard that does not exist. And each row has to carry the pane's location, because a token-only key costs the file all readability and `hook list`'s location column is what pays it back.

**Solution**: Declare `@portal-pane-id` once as `state.PortalPaneIDOption`, turn `ListAllPaneHookKeys` into a single two-field `list-panes -a -F` read returning one row per live pane — the pane's token (empty when unstamped) plus its `<session>:<window>.<pane>` location — and re-point the `cmd`-side `AllPaneLister` seam and both its consumers so the guard counts rows while the staleness comparison uses only the non-empty tokens.

**Outcome**: One tmux read serves the sweep, `portal doctor` and (later) `hook list`; a server with hooks present and zero stamped panes emits no guard WARN and deletes nothing it should not; a tmux failure still returns `(nil, err)`; and every positional key already on disk is carried through the flip untouched by Phase 1's retention rule.

**Do**:
- Add `PortalPaneIDOption = "@portal-pane-id"` to `internal/state/markers.go`, beside `RestoringMarkerName` / `SkeletonMarkerPrefix` / `BootstrappedMarkerName`. This is the **only** place the literal is written; every other site composes the tmux format or the option argument from it (`internal/tmux` already imports `internal/state` via `portal_saver.go`, so no cycle).
- In `internal/tmux/tmux.go`: add `type PaneHookRow struct { Token string; Location string }` (doc: `Token` is the pane's `@portal-pane-id`, empty for an unstamped pane; `Location` is display-only and is never a key). Add an unexported format const built by constant concatenation — `"#{" + state.PortalPaneIDOption + "}|#{session_name}:#{window_index}.#{pane_index}"` — and change `ListAllPaneHookKeys` to `([]PaneHookRow, error)`, reading through the existing `ListAllPanesWithFormat` and parsing with a new unexported `parsePaneHookRows`.
- `parsePaneHookRows` splits the raw output on `"\n"`, skips a wholly-blank line, and splits each remaining line at the **first** `|` only (`strings.Cut`), taking everything after it verbatim as `Location` — the location half carries `:` and `.`, and a session name may carry `|`. A line with no separator returns a wrapped error naming the offending line. Empty output returns a non-nil empty slice; a tmux failure returns `(nil, err)`. Leave `parsePaneOutput` and the name-based `StructuralKeyFormat` / `ResolveStructuralKey` / `ListAllPanes` siblings untouched — they serve non-hook structural use.
- In `cmd/run_hook_stale_cleanup.go`: widen `AllPaneLister`'s method to `ListAllPaneHookKeys() ([]tmux.PaneHookRow, error)` (keeping the `state.RestoringChecker` embed) and rewrite its doc comment, which currently names the `<@portal-id or session_name>:window.pane` form, to describe the two-field rows. In `runHookStaleCleanup` keep the empty-set guard on `len(rows) == 0` (rows, never tokens), keep the `panes=` count on the existing counts DEBUG line as the row count, and pass `store.CleanStale` a token slice built by a single shared helper (e.g. `liveTokensFrom(rows []tmux.PaneHookRow) []string`) that keeps only non-empty `Token` values.
- Re-point `cmd/doctor.go`'s `checkStaleHooks` through the same helper — `len(rows) == 0` still drives its existing not-evaluable branch, and its `hooks.StaleKeys` call takes `liveTokensFrom(rows)` — then re-point the blocked test surfaces: the three seam fakes (`recordingHookKeyLister` `cmd/run_hook_stale_cleanup_test.go:19`, `fakeHookLister` `cmd/doctor_test.go:802`, `stubAllPaneLister` `cmd/bootstrap_production_test.go:91`); retire `cmd/hookkey_no_regression_upgrade_test.go` whole (its subject is the name-fallback branch this work unit deletes) and move its `assertLiveHookKeyPresent` helper's job into `cmd/rename_restore_cleanup_survival_integration_test.go`, which is re-pointed to stamp a pane token with `set-option -p` and assert the token-keyed entry survives a rename plus the sweep while a stale token-shaped key is still reaped; `internal/tmux/list_all_pane_hookkeys_realtmux_test.go` and `hookkey_cross_site_realtmux_test.go` (see Edge Cases for the cross-site narrowing); the live key in `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:91-94` (`live:0.0`); and the `tmux.StructuralKeyFormat` live-key read at `cmd/state_daemon_hook_cleanup_integration_test.go:80-81`, which stamps a token on the live pane and seeds under it instead.

**Acceptance Criteria**:
- [ ] `@portal-pane-id` appears as a literal in exactly one place in non-test source — `state.PortalPaneIDOption` — and the enumeration format is composed from it
- [ ] `ListAllPaneHookKeys` returns one `PaneHookRow` per live pane, including panes carrying no token, whose `Token` is `""` and whose `Location` is still populated
- [ ] A row's `Location` survives an embedded `|` in the session name (first-separator split, unbounded remainder)
- [ ] Empty tmux output returns a non-nil empty slice; a `list-panes -a` failure returns `(nil, err)` and never an empty slice
- [ ] `runHookStaleCleanup`'s empty-set guard counts rows: with rows present and every token empty it does **not** take the guard branch, emits no guard WARN, and calls `CleanStale` with an empty live-token slice
- [ ] `CleanStale` and `checkStaleHooks` receive only non-empty tokens, through one shared projection helper used by both call sites
- [ ] With a server holding hooks and zero stamped panes, the sweep emits no `reason=empty-pane-read` WARN and deletes no non-token-shaped entry
- [ ] A token-shaped key absent from the live token set is still deleted when rows are present
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` both pass

**Tests**:
- `"it returns one row per live pane"` — fake `Commander` output of three rows, two stamped and one not; assert three rows and the empty token on the unstamped one
- `"it keeps an unstamped pane's row rather than dropping it"` — a leading-separator row (`|sess:0.0`) parses to `{Token: "", Location: "sess:0.0"}`
- `"it splits on the first separator only"` — a location containing `|` and one containing `:` and `.` both survive verbatim
- `"it errors on a row with no separator"` — malformed output is a wrapped error, not a silently half-parsed row
- `"it returns a non-nil empty slice for empty output"` — the existing contract, re-pointed to the new type
- `"it returns (nil, err) when list-panes fails"` — real-tmux with the server killed; assert `errors.As(*tmux.CommandError)` and a nil slice
- `"it enumerates a stamped and an unstamped pane in one read"` — real-tmux: stamp one pane with `set-option -p @portal-pane-id`, leave a sibling unstamped; assert both rows, correct tokens and correct locations
- `"it does not fire the mass-deletion guard when no pane is stamped"` — `runHookStaleCleanup` with rows present, all tokens empty, entries present; assert no WARN, no `onSkipped` call, `hooks.json` bytes unchanged (the entries are non-token-shaped and retained)
- `"it still deletes a token-shaped key absent from the live token set"` — rows present carrying a different token
- `"it takes the empty-pane-read branch only on zero rows"` — zero rows with entries present still WARNs and stands down
- `"it reports not evaluable from doctor on zero rows"` — `checkStaleHooks` unchanged in behaviour under the new type
- `"it keeps a restored token-keyed hook across a rename and the sweep"` — the re-pointed `cmd/rename_restore_cleanup_survival_integration_test.go`

**Edge Cases**:
- An unstamped pane must still produce a row: token-only output would shrink the enumeration to the stamped panes, and at zero stamped panes the row-counting guard would fire a WARN every 10s
- The guard counts rows and the stale comparison uses non-empty tokens; the two questions must not be conflated in either consumer
- The separator must be a non-whitespace character: `Commander.Run` trims the whole output, so a tab or space in the leading field position would be eaten off the first row when that pane is unstamped
- A tmux failure returns `(nil, err)`, never an empty slice — a caller reading a failure as "no live panes" orphans every entry at once
- The `Location` field is display-only. It renders the same shape as the positional structural siblings but is never a key, so it couples nothing to them
- The three `cmd` seam fakes are compile-time blockers; so is the integration-lane `cmd/rename_restore_cleanup_survival_integration_test.go`, which consumes the seam **and** the helper defined in the retired no-regression file
- `internal/tmux/hookkey_cross_site_realtmux_test.go` asserts that `ResolveHookKey`'s output appears byte-identically in the enumeration. That agreement does not hold between this task and the next one — registration is still positional here — so narrow its assertions to the enumeration side in this task (stamp with `set-option -p`, read the row back, assert an unstamped sibling still appears with an empty token). Task 2-2 restores the cross-site comparison in its end-state form
- Fixtures asserting an entry is *preserved because its pane is live* must be re-pointed to a stamped token, or they silently start passing on Phase 1's retention rule instead of on liveness

**Context**:
> **Ordering is load-bearing and must not be reversed.** This task lands before registration writes tokens (2-2). If registration flipped first, every freshly written token key would be absent from a still-positional live set, token-shaped by the Phase 1 rule, and reaped by the daemon's next idle sweep within ~10s — a hook vanishing seconds after the command reported success. This direction is safe because with no pane stamped the live token set is empty and every on-disk positional key is retained by shape, untouched.
>
> A single `list-panes -a -F` read over a two-field format serves the sweep, `portal doctor` and `hook list` alike — there is no second enumeration and no second tmux read.
>
> The option name is declared once because that is what retires the two `@portal-id` literal-binding guard tests rather than re-pointing them: a guard binding two copies of a literal has nothing left to bind when there is one copy, and a drift the compiler makes impossible cannot be introduced by someone who never ran the guard.
>
> CLAUDE.md is deliberately **not** edited here. Half of the hook-key description would contradict the other half mid-phase; the whole passage is rewritten in one coherent pass in task 2-2.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §2.1 (the option's single home), §3.3 (`ListAllPaneHookKeys` becomes one row per live pane with token and location; the `AllPaneLister` doc comment), §5.4 (the guard keys off live panes, not live tokens), §9.2/§9.3 (no spurious mass-deletion WARN; the enumeration fakes and fixtures to re-point).

## resume-hooks-silently-lost-2-2

### Task 2-2: Registration writes the pane's own token

**Problem**: A hook key is still half a coordinate. `HookKeyFormat` (`internal/tmux/tmux.go:565`) resolves to `<@portal-id or session_name>:#{window_index}.#{pane_index}`, and `window_index.pane_index` is a value tmux recomputes from layout: closing a sibling pane, `break-pane`, `move-pane`, or closing a window under `renumber-windows on` changes it, nothing re-keys the entry, and the sweep removes it within ~10s of the move. No key format built on a coordinate can survive that. A durable per-pane identity exists in tmux — a pane user-option survives every operation that moves a pane's coordinates, and survives `respawn-pane -k`, which is what restore does to every pane — but Portal does not create panes, so there is no creation point to stamp at and a pane with no hook needs no token.

**Solution**: Make the hook key the pane's `@portal-pane-id` token and nothing else, minted and stamped lazily by `portal hook set` immediately before the `hooks.json` write: read the pane's token, mint and stamp only when it reads back empty, then write the entry under that token.

**Outcome**: `portal hook set` in a live pane writes one entry keyed by a six-character token that no tmux operation can recompute; a second registration on the same pane reuses the token and issues no `set-option`; a failed stamp ends the command non-zero with nothing written; and the key carries no session component, so no rearrangement — within a window, across windows, or across sessions — can change it.

**Do**:
- In `internal/session`, beside `IsTokenShaped`, add an exported minter (e.g. `func NewPaneToken() (string, error)`) that returns `NewNanoIDGenerator()()`. It names no width and no alphabet of its own; callers name neither.
- In `internal/tmux/tmux.go`: change `HookKeyFormat` to the pane-token format, composed by constant concatenation from the constant — `"#{" + state.PortalPaneIDOption + "}"` — so `ResolveHookKey` now returns the pane's token, empty for a live pane that has never been stamped. Rewrite `ResolveHookKey`'s doc comment: its warning against synthesising a name-based key has nothing left to warn about, because there is no name-based key. Add a pane-scoped option writer `SetPaneOption(target, name, value string) error` running `set-option -p -t <target> <name> <value>`, taking the option name from its caller.
- In `cmd/hooks.go`: add a pane-option seam to `HooksDeps` (e.g. `PaneStamper` over a one-method `PaneOptionSetter` interface, satisfied by `*tmux.Client`), defaulting to `buildHooksTmuxClient()` exactly as `KeyResolver` does. Change `resolveCurrentPaneKey` to return the pane id alongside the resolved key so `hook set` can stamp the pane it read.
- Rewrite `hooksSetCmd`'s `RunE` to the fixed order: resolve the pane's token → if it is empty, mint one and stamp it with `SetPaneOption(paneID, state.PortalPaneIDOption, token)`, returning that error unchanged on failure → only then `store.Set(token, "on-resume", command, "cli")`. A comment on the stamp-before-write ordering is warranted, in one line, wording left to the executor. Do not add a rollback or an unstamp on a failed write.
- Rewrite CLAUDE.md's hook-key passages in one pass: the `tmux` row's `HookKeyFormat` / `ResolveHookKey` / `ListAllPaneHookKeys` clauses (including the enumeration's new two-field rows from 2-1), the `hooks` row's key description, and the "Resume hooks" section's key-scheme paragraph. Delete the sentence claiming the hook identity rides `sessions.json` and is re-stamped at restore — that becomes true of the pane token in Phase 3 and must not be written before it is. Leave every `@portal-id`-as-session-stamp passage (the `session` row, the `state` row's capture column and `Session.PortalID`, and `tmux.HookKey`) exactly as it is.
- Re-point the blocked tests: `cmd/hooks_test.go`'s `mockKeyResolver` keys and `hook set` assertions move to tokens (and the new stamper seam); `internal/tmux/hookkey_format_realtmux_test.go` reads `@portal-pane-id` off panes stamped with `set-option -p`; `internal/tmux/hookkey_test.go`'s `TestHookKeyFormatContainsPortalIDLiteral` is deleted (the format holds no `@portal-id` literal to bind, and `TestHookKey` stays until Phase 3 deletes `tmux.HookKey`); `cmd/portal_id_binding_guard_test.go` is deleted whole rather than re-pointed; and `internal/tmux/hookkey_cross_site_realtmux_test.go` is restored to its end-state form — `ResolveHookKey(pane)` returns exactly the `Token` the enumeration's row for that pane carries.

**Acceptance Criteria**:
- [ ] `hook set` in a live unstamped pane mints a token, issues exactly one `set-option -p` carrying `state.PortalPaneIDOption`, and writes exactly one `hooks.json` entry keyed by that same token
- [ ] The written key satisfies `session.IsTokenShaped` and equals the value handed to the stamper
- [ ] `hook set` on a pane already carrying a token writes under that token and issues **no** `set-option` — no second token is minted
- [ ] A failed stamp ends the command non-zero and leaves `hooks.json` byte-identical (absent stays absent)
- [ ] A stamp that succeeds followed by a failed write leaves the pane's token in place — no unstamp, no rollback — and a retry reuses it
- [ ] `hook set` never writes an empty key: a mint failure is a non-zero exit with nothing written
- [ ] `@portal-pane-id` is composed from `state.PortalPaneIDOption` at both the format and the stamp; the literal is restated nowhere
- [ ] `hook` remains bootstrap-exempt — no tmux server is started by any of these calls
- [ ] `hook rm --pane-key <key>` still issues no tmux call at all and is behaviourally unchanged
- [ ] CLAUDE.md describes the hook key as the pane token and makes no claim about the token being captured or re-stamped

**Tests**:
- `"it stamps and writes under a freshly minted token"` — unstamped seam; assert one `set-option` recorded with `@portal-pane-id`, one entry, key equal to the stamped value and token-shaped
- `"it reuses an existing token and issues no set-option"` — seam returns a token; assert the entry lands under it and the stamper recorded zero calls
- `"it writes nothing when the stamp fails"` — stamper returns an error; assert non-zero, the error text propagates, and `hooks.json` is unchanged/absent
- `"it stamps before it writes"` — assert ordering by recording both seams; a write that precedes the stamp is a failure
- `"it leaves the token in place when the write fails"` — deny the hooks path; assert the stamp stands and no unstamp is issued
- `"it never writes an empty key"` — resolver returns `""` and the mint fails; assert non-zero with nothing written
- `"it resolves a stamped pane to its token"` — real-tmux: `set-option -p` then `ResolveHookKey` returns exactly that value
- `"it resolves a live unstamped pane to an empty token"` — real-tmux
- `"it agrees with the enumeration for the same pane"` — real-tmux cross-site: `ResolveHookKey(pane)` equals that pane's row `Token`
- `"it takes no tmux call on the --pane-key path"` — `hook rm --pane-key <seeded key>` with a seam that fails on any call

**Edge Cases**:
- A pane already carrying a token keeps it: re-registering must not mint a second token, which would orphan the first entry
- Steps must not be reordered and a failed stamp ends the command: a write that precedes the stamp, or follows one that failed, persists an entry keyed to a token no pane carries
- The mirror state — stamp succeeded, write failed — is left exactly as it is. There is no rollback: unstamping races a concurrent registration that may already have read the token, and turns a benign no-op into a lost identity
- `hook rm`'s `$TMUX_PANE` path shares `resolveCurrentPaneKey` and now resolves a token, so a live unstamped pane resolves to an empty key. Keep today's behaviour for it here — its fixed words and its exit-0-iff-removed rule are Phase 4's, and pre-empting them splits one rule across two phases
- `hook` stays bootstrap-exempt: a set `$TMUX_PANE` already implies a live server, and the stamp is a pane write on that server, not a server start
- Restore's saved-state bake keeps the positional `tmux.HookKey` until Phase 3; do not touch `collectArmInfos`, the capture format or `Session.PortalID` here
- `cmd/portal_id_binding_guard_test.go` asserts `tmux.HookKeyFormat` contains `session.PortalIDOption` and fails the moment the format flips. It is deleted, not re-pointed — the `internal/state` capture-format guard goes with the capture column in Phase 3

**Context**:
> A hook key is the pane's `@portal-pane-id` token and nothing else — no session component, no coordinates. The composite `<portal-id>:<pane-token>` was considered and rejected: `move-pane -t <other-session>` changes the session half, so drift returns for exactly the operation a user performs deliberately.
>
> Stamping is lazy because Portal does not create panes — the user splits them — and a pane with no hook needs no token. There is no inheritance: a pane created by `split-window` or `new-window` from a stamped pane reads back empty, so a split cannot duplicate an id.
>
> tmux's own `%N` pane id is not the identity. It is stable only within a server lifetime and tmux is free to recycle it, so it needs identical carry-and-re-stamp machinery while being less trustworthy: a recycled id can name a pane the entry was not written for, which fires a hook on the wrong pane rather than losing it.
>
> Registration's refusal of a `$TMUX_PANE` that names no live pane is task 2-3. After this task a gone pane still fails — the `set-option -p` at step 4 errors on a bogus target in its own right — but it fails after minting and with no way to tell a gone pane from an unstamped one. That discrimination, and the tests that pin it against a real server, are 2-3's.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §2.1 (the token, its option and its generator), §2.2 (lazy stamping at `hook set`; reuse; bootstrap-exemption), §3.1 (the key is the token alone), §3.3 (`HookKeyFormat`, `ResolveHookKey`, the added pane-option write, the CLAUDE.md rewrite), §4.1 steps 3–5 (the fixed order and the no-rollback rule), §9.2/§9.3/§9.4 (token-reuse test; tests to re-point; guards deleted rather than re-pointed).

## resume-hooks-silently-lost-2-3

### Task 2-3: Registration refuses a pane that is not there

**Problem**: `portal hook set` will register a hook for a pane that does not exist and exit 0. `tmux display-message -p -t %999 '<format>'` exits **0** and prints an empty result for a target no pane answers to (tmux 3.7c), `ResolveHookKey` returns that verbatim, `resolveCurrentPaneKey` passes it through, and nothing between the read and the file validates that the key identifies any pane at all — there is no such validation anywhere in the repo. This is the write-time moment of the defect, and no key format closes it: 61 of the 63 degenerate `:.` lines found in a month of `portal.log` came from this class of read. After task 2-2 a gone pane fails late, at the stamp, having already minted a token and with no way to tell "pane gone" from "pane live but never stamped" — which is exactly the distinction `hook rm` needs in Phase 4.

**Solution**: Give `ResolveHookKey` a tmux-native existence probe ahead of its token read — `show-options -p -t <target>` naming **no option** — so a target no pane answers to fails on exit status before anything is minted, stamped or written, with tmux's own words passed through unaltered.

**Outcome**: `portal hook set` against a `$TMUX_PANE` that names no live pane exits non-zero with tmux's message and leaves `hooks.json` byte-identical; a live pane carrying no pane options at all still resolves, with an empty token; and a stamped pane still resolves to its token. The discrimination is pinned against a real tmux server, where Portal's own argv is what is measured.

**Do**:
- In `internal/tmux`'s `ResolveHookKey(paneID string)`, run `show-options -p -t <paneID>` first with **no option name**. A non-zero exit returns `("", <wrapped error>)` before the token read; a zero exit falls through to the existing `display-message -p -t <paneID> HookKeyFormat` read. Both calls stay internal to `ResolveHookKey` — the probe adds no exported surface.
- Wrap the probe's error so the `*tmux.CommandError` stays recoverable by `errors.As` and tmux's stderr survives in the rendered message. Portal must branch on exit status only: no `strings.Contains` on tmux's message text anywhere on this path.
- Carry one line of comment on the probe explaining that it must name no option, wording left to the executor — the code cannot express why the option name is omitted, and re-adding it is the tempting simplification that silently breaks the discrimination.
- Rewrite `ResolveHookKey`'s doc comment to describe the two reads and what each answers.
- Re-point `internal/tmux/resolve_hookkey_realtmux_test.go`: its read-failure case kills the server to force an error, on the stated premise that a bogus `-t` target is tolerated. That premise inverts — a bogus target is now a real failure through the probe — so drive the failure with a bogus pane id against a **live** server, and keep a server-down case only if it still asserts something distinct.
- Add the CLI-level propagation test in `cmd` with the `HooksDeps` seam injected (never a real server — `cmd`'s `TestMain` poisons `TMUX` package-wide): a `KeyResolver` returning a `*tmux.CommandError` carrying a known stderr string makes `hook set` exit non-zero with that string in the returned error and `hooks.json` untouched. Extend CLAUDE.md's `ResolveHookKey` clause to name the two-call resolution.

**Acceptance Criteria**:
- [ ] `ResolveHookKey` against a pane id no pane answers to returns a non-nil error and an empty key, from the probe, before the token read runs
- [ ] `ResolveHookKey` against a live pane carrying no pane options at all returns `("", nil)` — resolved, unstamped, not an error
- [ ] `ResolveHookKey` against a stamped pane returns its token
- [ ] The probe names no option; the token's value comes from the `display-message` read
- [ ] `hook set` on an unresolvable `$TMUX_PANE` exits non-zero, writes nothing to `hooks.json` (an absent file stays absent), mints nothing and stamps nothing
- [ ] The error surfaced to the user carries tmux's own words, and no production code inspects that text
- [ ] The returned error keeps `*tmux.CommandError` recoverable via `errors.As`
- [ ] Real-tmux coverage pins the raw facts the probe rests on: `set-option -p -t %999 @portal-pane-id X` exits non-zero, while `display-message -p -t %999 '<format>'` exits 0

**Tests**:
- `"it fails for a pane id no pane answers to"` — real-tmux, live server, bogus `%999`; non-zero, empty key, `errors.As(*tmux.CommandError)`
- `"it resolves a live pane carrying no pane options at all"` — real-tmux; empty token, nil error
- `"it resolves a stamped pane to its token"` — real-tmux; the token set by `set-option -p`
- `"it probes without naming the option"` — real-tmux: `show-options -p -t <live pane> @portal-pane-id` exits non-zero with `invalid option`, which is why the option must be omitted; assert the live pane still resolves through Portal's own call
- `"it pins the raw tmux facts the probe rests on"` — `set-option -p` against a bogus target exits non-zero; `display-message -p` against the same target exits 0
- `"it probes before reading the token"` — a fake `Commander` recording the argv sequence; the `display-message` read must not be issued once the probe fails
- `"it exits non-zero from hook set on an unresolvable pane"` — `cmd`, injected seam; assert non-zero, the error text, and `hooks.json` untouched
- `"it mints and stamps nothing when the probe fails"` — `cmd`; assert the stamper recorded zero calls

**Edge Cases**:
- The probe must name no option: `show-options -p -t <live pane> @portal-pane-id` exits 1 with `invalid option`, indistinguishable on exit status from `no such pane`, which collapses the very discrimination the probe exists to make
- A live pane with no pane options at all exits 0 with empty output and must resolve rather than fail
- Exit status is the whole signal — tmux's message text is never parsed, and the wrap must not swallow tmux's words on the way to stderr
- A fake `Commander` modelling the intended semantics cannot catch a probe that names the option; the discrimination is measured against a real server in `internal/tmux`'s real-tmux client lane
- `hook rm`'s `$TMUX_PANE` path inherits the probe here, because it shares `resolveCurrentPaneKey`. Its wording and its exit-0-iff-removed rule stay in Phase 4
- `internal/tmux/resolve_hookkey_realtmux_test.go` files the bogus-target tolerance as a testing obstacle in a comment; the comment goes with the premise
- `cmd`'s `TestMain` poisons `TMUX` package-wide, so the CLI-level propagation test drives the injected `*Deps` seam rather than a real server

**Context**:
> This is the only change that closes the write-time moment, because tmux offers no error to detect on the read path. Verification is tmux-native — an exit status from a call that genuinely refuses a bogus target — rather than a shape heuristic on a returned string.
>
> The three cases and the two calls that separate them (tmux 3.7c):
>
> | Call | pane gone | live, no token | live, stamped |
> |---|---|---|---|
> | `show-options -p -t <target>` (no option named) | exit 1 | exit 0 | exit 0 |
> | `display-message -p -t <target> '#{@portal-pane-id}'` | exit 0, empty | exit 0, empty | exit 0, `TOKEN1` |
> | `set-option -p -t <target> @portal-pane-id X` | exit 1 | exit 0 | exit 0 |
>
> The full registration order is: `requireTmuxPane` → probe → read token → mint and stamp if empty → write → touch `save.requested`. The stamp at step 4 errors on a bogus target in its own right — a second, redundant guard that costs nothing — but it is not the guard, because by then a token has been minted and a gone pane is indistinguishable from an unstamped one.
>
> **This failure will fire routinely on the removal side, and that is expected** — a deregistration against an already-closed pane is the ordinary case, since a SessionEnd commonly fires *because* the pane was closed. The specification accepts that and treats it as the point: a caller can no longer read `rc == 0` as proof anything happened. The removal half of that rule is Phase 4's.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §4.1 (registration verifies the pane exists; why the probe names no option; the fixed step order), §3.3 (`ResolveHookKey` becomes the two-call live resolution and its doc comment is rewritten), §9.1/§9.2/§9.3 (lane placement; the existence-probe test; the re-pointed real-tmux read-failure test).

## resume-hooks-silently-lost-2-4

### Task 2-4: A registered hook asks for a capture

**Problem**: A pane token lives in two places, and only one of them survives a reboot. `hook set` stamps `@portal-pane-id` on the live pane and writes the entry, but the token only becomes durable when the daemon captures it into `sessions.json`. Left to itself the daemon reaches that state on its own schedule: with no dirty flag set, the capture waits for the gap branch, up to `MaxGap` (30s, `cmd/state_daemon.go:425`). A crash inside that window leaves an entry keyed to a token no saved pane carries — the pane comes back unstamped, no live pane answers to the key, and the entry is reaped under the ordinary rule. Every other event that changes structure already narrows this window by touching `save.requested` (`portal state notify`, driven from tmux hooks); registration is the one structural change that does not.

**Solution**: After a successful `hooks.json` write, `hook set` resolves the state directory with `state.EnsureDir()` and calls `state.TouchSaveRequested(dir)`, best-effort — a failure at either step logs one WARN under its own `op` and the command still exits 0.

**Outcome**: The next daemon tick captures the new token instead of waiting for the gap branch; a machine with no reachable state directory still registers hooks and exits 0, with one WARN naming what did not happen; and no failure on this path can report a registration as lost when the entry is already durably on disk.

**Do**:
- In `hooksSetCmd`'s `RunE` (`cmd/hooks.go`), after `store.Set` returns nil, call `state.EnsureDir()` then `state.TouchSaveRequested(dir)`, and return nil regardless of what either returns. `state.EnsureDir` is the same resolution `portal state notify` performs from an equally bootstrap-exempt command (`cmd/state_notify.go`), so no server is started and the exemption is unaffected.
- Emit exactly one WARN on failure at either step, through the `hooksLogger` binding `cmd` already holds (`cmd/state_common.go:11`): message and `op` both `touch-save-requested`, plus `hook_key` (the token just written), `via` `cli`, and the existing `error` attr carrying the failure. Add no new component and no new attr key, and never file this under `op=set`.
- Factor the two steps into one small unexported helper so there is exactly one emission site and both failure modes cannot each emit.
- Do not touch `hooksRmCmd` — removal does not touch the dirty flag.
- Cover the happy path and both failure modes in `cmd/hooks_test.go`.

**Acceptance Criteria**:
- [ ] A successful `hook set` creates `save.requested` in the resolved state directory
- [ ] With the state directory unresolvable or uncreatable, `hook set` exits 0 with the entry written and emits exactly one WARN under `op=touch-save-requested`
- [ ] With the state directory present but the touch failing, the outcome is identical — one WARN, exit 0, entry written
- [ ] The WARN is under the `hooks` component and carries `op`, `hook_key`, `via=cli` and `error`; no `op=set` WARN is emitted alongside it
- [ ] Exactly one WARN is emitted per failing `hook set`, never two
- [ ] The touch runs only after the write returns without error: a failed `hook set` touches nothing
- [ ] `hook rm` touches nothing on either of its paths
- [ ] `hook` starts no tmux server on this path — bootstrap-exemption is unchanged

**Tests**:
- `"it touches save.requested after a successful registration"` — `PORTAL_STATE_DIR` at a temp dir; assert the file exists after the command
- `"it exits 0 when the state directory cannot be resolved"` — point `PORTAL_STATE_DIR` at a path under a regular file so `MkdirAll` fails; assert exit 0, the entry present, one WARN with `op=touch-save-requested` and a non-empty `error`
- `"it exits 0 when the touch itself fails"` — create the state dir and its `scrollback` subdir, then `chmod 0500` the state dir; assert exit 0, the entry present, one WARN
- `"it emits no set WARN when only the touch fails"` — assert the sink holds no WARN under `op=set`
- `"it emits exactly one warn per failing hook set"` — assert the record count, not just presence
- `"it does not touch when the write fails"` — deny the hooks path; assert no `save.requested` and no `touch-save-requested` record
- `"it touches on a no-op re-registration"` — register the same command twice; the second returns nil at the call site and still touches
- `"it does not touch from hook rm"` — assert no `save.requested` after a removal

**Edge Cases**:
- A failure at either step — resolving/creating the directory, or the touch — logs exactly one WARN and `hook set` still exits 0. The registration succeeded; failing the command over a latency optimisation would report a loss that did not happen
- The WARN is filed under its own `op`, never under `set`, for the same reason: a `set` WARN names a loss that did not occur
- A no-op re-registration (`classifySet` → `set-noop`) returns nil indistinguishably at the call site and touches too, costing one spare capture tick. Do not add a second read to tell them apart
- `cmd`'s `TestMain` poisons `PORTAL_STATE_DIR` package-wide with `/nonexistent/...`, so every existing `hook set` test already exercises the unresolvable-directory branch. They must keep exiting 0
- This is the only path `hook` takes outside the config directory holding `hooks.json`, and it starts no tmux server
- The residual window between registration and capture is narrowed, not closed; a crash inside it still loses the stamp, and that is accepted

**Context**:
> The touch is a latency optimisation and never affects the exit status: by the time it runs the entry is already durably written.
>
> It is emitted from `cmd`, where the touch happens — the store has no path to the state directory — through the `hooks` binding `cmd` already holds. The store's own emissions are unaffected and no new component binding is introduced.
>
> `op=touch-save-requested` is one of the three new `op` values this work unit adds to the closed `hooks` vocabulary, and it is spec-governed rather than a call-site invention. It adds no attr key: `hook_key`, `via` and `error` are all already in the component's vocabulary.
>
> The same window, and the same mitigation, apply to the out-of-band conversion script that re-keys the existing `hooks.json` — it can touch the flag after its last conversion, or run `portal state commit-now`. That script is outside this work unit and no task here covers it.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §2.2 (`hook set` touches the dirty flag; the WARN's shape, its own `op`, and why it never fails the command), §6.5 (the three new `op` values and the unchanged attr vocabulary), §9.2 (a failed dirty-flag touch does not fail `hook set`).

## resume-hooks-silently-lost-2-5

### Task 2-5: A moved pane keeps its hook

**Problem**: The gap that let this bug live for months is that **no test ever moved a pane**. The existing cross-site tests prove every site derives the same key for a pane *at rest* — the case that works — while the failing case is a pane whose `window_index.pane_index` tmux recomputed after a rearrangement. Tasks 2-1 through 2-3 make the key a durable token on the strength of tmux behaviours verified once by hand in a scratch socket. Nothing in the repository holds that verification, so a later change to the format, the stamp target or the enumeration could silently reintroduce a positional component and no test would notice.

**Solution**: Add a real-tmux client suite in `internal/tmux` that registers a token on a pane and asserts the hook still resolves — through Portal's own resolve and enumeration — after each of the four rearrangements the defect names, after `respawn-pane -k`, and that a split inherits nothing.

**Outcome**: The durability the whole work unit rests on is pinned by executable coverage: `break-pane`, `kill-window` under `renumber-windows on`, `move-pane` back and `move-pane` into another session each leave the pane's token and its hook key unchanged while its coordinates change, and a pane created from a stamped one carries no token.

**Do**:
- Add a real-tmux client test file in `internal/tmux` (unit lane, `tmuxtest.SkipIfNoTmux` + a per-test `-L` socket via `tmuxtest.New`; no daemon, no built binary). Set `renumber-windows on` globally on the fixture server — it is not tmux's default and it is what makes the `kill-window` case renumber.
- Seed a session with several windows and panes, capture the target pane's `%N` id, stamp it through `client.SetPaneOption(paneID, state.PortalPaneIDOption, token)`, and record its starting `Location` from `client.ListAllPaneHookKeys()`.
- Drive the moves in sequence — `break-pane`, `kill-window` on an earlier window, `move-pane` back, `move-pane` into a second session — and after **each** one assert three things through Portal's own calls: `client.ResolveHookKey(paneID)` still returns the token; the enumeration holds exactly one row carrying that token; and that row's `Location` has changed from the value recorded before the move (a move that did not move proves nothing).
- Assert the stamp survives `client.RespawnPane(target, <a trivial command>)` — restore arms every pane that way, and Phase 3 rests on the stamp outliving it.
- Assert non-inheritance: `SplitWindow` and `NewWindow` from the stamped pane produce rows whose `Token` is empty, so no rearrangement can duplicate a token.

**Acceptance Criteria**:
- [ ] The token resolves unchanged after `break-pane`, after a `kill-window` under `renumber-windows on`, after `move-pane` back, and after `move-pane` into another session — each asserted immediately after its own move
- [ ] Each move is proven to have moved: the pane's enumerated `Location` differs from the value recorded before it
- [ ] Exactly one enumerated row carries the token after every move
- [ ] The token survives `respawn-pane -k`
- [ ] A pane created by `split-window` or `new-window` from the stamped pane enumerates with an empty token
- [ ] Every assertion runs through `ResolveHookKey` / `ListAllPaneHookKeys` / `SetPaneOption` rather than raw tmux format reads, so what is measured is Portal's own argv
- [ ] `renumber-windows on` is set explicitly on the fixture server, not assumed
- [ ] The suite runs in the unit lane and spawns no daemon and no built binary

**Tests**:
- `"it keeps the hook key when the pane is broken out to its own window"` — `break-pane`; token unchanged, location changed
- `"it keeps the hook key when an earlier window is closed under renumber-windows on"` — `kill-window`; token unchanged, window index changed
- `"it keeps the hook key when the pane is moved back"` — `move-pane`; token unchanged, location changed
- `"it keeps the hook key when the pane is moved to another session"` — `move-pane -t <other session>`; token unchanged, the session half of the location changed
- `"it keeps the hook key across respawn-pane -k"` — the operation restore performs on every pane
- `"it does not inherit the token into a split"` — `split-window` from the stamped pane enumerates an empty token
- `"it does not inherit the token into a new window"` — `new-window` from the stamped session enumerates an empty token
- `"it enumerates exactly one row carrying the token after every move"` — guards against a duplicated id

**Edge Cases**:
- Each of the four moves is asserted after its own move, not only at the end of the chain — a single end-state assertion cannot say which operation broke the key
- `renumber-windows on` has to be set on the fixture server; without it the `kill-window` case does not renumber and the test passes for the wrong reason
- The cross-session move is the case a composite `<portal-id>:<pane-token>` key would have failed, which is why it is a named case rather than a variation
- `respawn-pane -k` survival belongs in this suite because restore arms every pane that way and Phase 3 rests on the stamp outliving it
- A split or a new window from a stamped pane inherits nothing; without that, a rearrangement could duplicate a token and fire one hook on two panes
- The pane's `%N` id is stable within a server lifetime, which is what lets the same target be re-read after each move; it is the test's handle, never the identity under test
- Scope: CLAUDE.md's remaining `@portal-id` passages — the capture column, `Session.PortalID`, `tmux.HookKey` — belong to Phase 3 and must not be pre-empted here

**Context**:
> A tmux pane user-option was verified in an isolated `-L` socket (tmux 3.7c) to survive every mutation that moves a pane's coordinates, and the one Portal itself performs:
>
> | Operation | Coordinates before → after | Stamp |
> |---|---|---|
> | initial | `t:1.2` | `STAMP1` |
> | `break-pane` | `t:1.2` → `t:3.1` | survives |
> | `kill-window` under `renumber-windows on` | `t:3.1` → `t:2.1` | survives |
> | `move-pane` back | `t:2.1` → `t:1.2` | survives |
> | `respawn-pane -k` | `t:1.2` (unchanged) | survives |
> | `rename-session` | `t:1.2` → `newname:1.2` | survives |
>
> This task is what moves those facts out of a scratch socket and into the repository.
>
> The positional siblings — `state.SanitizePaneKey`'s hydrate FIFO paths and `@portal-skeleton-*` markers, and `internal/tmux`'s `StructuralKeyFormat` / `ResolveStructuralKey` / `ListAllPanes` — share the addressing assumption but are not changed by this work: they live for the duration of one bootstrap and are rebuilt from live coordinates each time, so drift has no window in which to occur. Their coverage is run against the change rather than assumed unaffected; the surface that observes them is Phase 3's non-contiguous-window-index restore test, not this one.

**Spec Reference**: `.workflows/resume-hooks-silently-lost/specification/resume-hooks-silently-lost/specification.md` — §2.1 (the verified survival table and the no-inheritance property), §3.1 (why a token-only key closes lifetime drift, including the cross-session move), §9.1/§9.2 (lane placement; "a pane that moves keeps its hook" is the test the gap in coverage calls for), §9.5 (the positional siblings are checked, not assumed separate).
