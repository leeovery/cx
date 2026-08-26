# Review Tracking: Resume Hooks Silently Lost - Traceability

## Findings

### 1. Shared real-tmux scaffolding keeps stamping the retired session identity

**Type**: Missing from plan
**Spec Reference**: §9.3 (existing tests to re-point — `internal/tmux/hookkey_realtmux_shared_test.go` is named individually alongside its four siblings)
**Plan Reference**: Phase 2, task `resume-hooks-silently-lost-2-2` — the final **Do** bullet ("Re-point the blocked tests: …")
**Move**: settled
**Change Type**: add-to-task

**Problem**:
The real-tmux suite that proves a hook key resolves the same way at every site is driven by one shared fixture builder, and that builder is the thing that stamps the session-level identity onto every session it creates. The plan re-points all four tests that call into it and never names the builder itself. Left as written, those tests go on seeding sessions with the identity this work is deleting, so the format test would read pane tokens off panes nothing ever stamped — coverage that passes while measuring nothing — and the retired `@portal-id` stamp survives in the repository past the release meant to remove it, discovered only by a repo-wide grep in the final task, three tasks later, in a different phase.

**Proposal**:
Name the shared builder in the same **Do** bullet that re-points its callers, and say what it becomes: it stamps the pane it hands back rather than the session, through the pane-option writer the same task introduces. The specification enumerates this file by name in the list of tests to re-point at the token key; carrying that name into the task is the specification's own instruction, not a new call. The `portalIDLiteral` constant it shares is left to travel with its last reader (task 2-3's re-point of the read-failure test) rather than being stranded here while that file still reads it.

**Current**:
```markdown
- Re-point the blocked tests: `cmd/hooks_test.go`'s `mockKeyResolver` keys and `hook set` assertions move to tokens (and the new stamper seam); `internal/tmux/hookkey_format_realtmux_test.go` reads `@portal-pane-id` off panes stamped with `set-option -p`; `internal/tmux/hookkey_test.go`'s `TestHookKeyFormatContainsPortalIDLiteral` is deleted (the format holds no `@portal-id` literal to bind, and `TestHookKey` stays until Phase 3 deletes `tmux.HookKey`); `cmd/portal_id_binding_guard_test.go` is deleted whole rather than re-pointed; and `internal/tmux/hookkey_cross_site_realtmux_test.go` is restored to its end-state form — `ResolveHookKey(pane)` returns exactly the `Token` the enumeration's row for that pane carries.
```

**Proposed Text**:
```markdown
- Re-point the blocked tests: `cmd/hooks_test.go`'s `mockKeyResolver` keys and `hook set` assertions move to tokens (and the new stamper seam); `internal/tmux/hookkey_realtmux_shared_test.go`'s `seedThreePaneStampedSession` stops stamping the session option and stamps a pane instead — it takes a pane token rather than a `portalID` and writes it with `client.SetPaneOption(paneID, state.PortalPaneIDOption, token)` onto the pane it returns, so both callers seed a stamped pane rather than a stamped session; `internal/tmux/hookkey_format_realtmux_test.go` reads `@portal-pane-id` off panes stamped that way; `internal/tmux/hookkey_test.go`'s `TestHookKeyFormatContainsPortalIDLiteral` is deleted (the format holds no `@portal-id` literal to bind, and `TestHookKey` stays until Phase 3 deletes `tmux.HookKey`), while its `portalIDLiteral` const stays only as long as task 2-3's `resolve_hookkey_realtmux_test.go` still reads it and goes with that re-point; `cmd/portal_id_binding_guard_test.go` is deleted whole rather than re-pointed; and `internal/tmux/hookkey_cross_site_realtmux_test.go` is restored to its end-state form — `ResolveHookKey(pane)` returns exactly the `Token` the enumeration's row for that pane carries.
```

**Resolution**: Pending
**Notes**:

---

### 2. The saved-state tests that prove an upgrade can still read old saved sessions are not converted

**Type**: Incomplete coverage
**Spec Reference**: §9.3 (`internal/state/*` updated in step with the removals), §2.3 / §7.2 (the field addition and the field removal are both tolerant-decode changes — no schema-version bump, no migration, in either direction)
**Plan Reference**: Phase 3, task `resume-hooks-silently-lost-3-1` — the final **Do** bullet ("Delete `internal/state/portal_id_literal_guard_test.go` whole … and re-point the blocked fixtures: …")
**Move**: settled
**Change Type**: add-to-task

**Problem**:
The promise a user actually depends on across this upgrade is that Portal can still open the sessions file it saved before the upgrade, and still opens the one it saves after — with no version bump and no conversion step. That promise is currently held by four tests in the state package plus one that pins the session-record copy being deleted, and every one of them is written against the identity field this task removes. The task's re-point list names only one of the state package's test files, so those tests are neither retired nor rewritten to the pane-level version: the round-trip and legacy-decode guarantees lose their coverage instead of gaining the replacement, and the build breaks partway through the task on a fixture nobody planned to touch.

**Proposal**:
Add both files to the same re-point list, with what each becomes. `capture_internal_test.go`'s single test covers the session-record copy this task deletes, so it retires with it. `schema_test.go`'s four cases are exactly the round-trip, legacy-decode and unchanged-schema-version assertions the task already commits to in its acceptance criteria and its named tests — they are re-pointed at the pane field rather than dropped, which is what keeps those criteria backed by something. The specification names the state package's tests as updated in step with these removals; this spells out which ones and how.

**Current**:
```markdown
- Delete `internal/state/portal_id_literal_guard_test.go` whole (it asserts `captureFormat` contains the `@portal-id` literal; the single home in `internal/state` leaves it nothing to bind) and re-point the blocked fixtures: `cmd/state_daemon_run_test.go`'s `oneSession()` (`:207-212`) keeps its 11-field row and its trailing empty column, with the comment renamed to the pane token; `internal/state/capture_test.go`'s `paneLineWithID` and its `aB3xY9kZ` assertions become per-pane token fixtures; `internal/restore/session_test.go`'s `sess.PortalID` fixtures and the `tmux.HookKey`-derived expectations become `Pane.PortalPaneID` fixtures; `internal/restore/rename_reboot_hook_integration_test.go`, `rename_reboot_durability_integration_test.go` and `rename_reboot_shared_test.go`'s `renamePortalID` scaffolding are re-pointed at a per-pane token (stamped with `SetPaneOption`, seeded in `hooks.json` under the token, and asserted to survive the rename and the reboot — the user-visible rename guarantee stays under test for a new reason); `internal/restore/multipane_legacy_integration_test.go`'s multipane coverage is re-pointed at per-pane tokens and its un-stamped-name-fallback subtests are retired with the branch they cover.
```

**Proposed Text**:
```markdown
- Delete `internal/state/portal_id_literal_guard_test.go` whole (it asserts `captureFormat` contains the `@portal-id` literal; the single home in `internal/state` leaves it nothing to bind) and re-point the blocked fixtures: `cmd/state_daemon_run_test.go`'s `oneSession()` (`:207-212`) keeps its 11-field row and its trailing empty column, with the comment renamed to the pane token; `internal/state/capture_test.go`'s `paneLineWithID` and its `aB3xY9kZ` assertions become per-pane token fixtures; `internal/state/capture_internal_test.go`'s `TestFindOrAppendSessionCopiesPortalID` retires whole with the `PortalID:` copy in `findOrAppendSession` that it covers; `internal/state/schema_test.go` re-points its four session-identity cases at the pane field — the JSON-tag round-trip (`:96-127`) and the legacy-payload decode (`:130-157`) become `Pane.PortalPaneID` / `portal_pane_id` cases, the schema-version case (`:160-163`) keeps its `1` and names the additive `portal_pane_id` field, and the `Session` fixture at `:21` drops its `PortalID` — so the criteria in this task about decoding a pre-upgrade and a post-upgrade `sessions.json` are the ones these cases assert; `internal/restore/session_test.go`'s `sess.PortalID` fixtures and the `tmux.HookKey`-derived expectations become `Pane.PortalPaneID` fixtures; `internal/restore/rename_reboot_hook_integration_test.go`, `rename_reboot_durability_integration_test.go` and `rename_reboot_shared_test.go`'s `renamePortalID` scaffolding are re-pointed at a per-pane token (stamped with `SetPaneOption`, seeded in `hooks.json` under the token, and asserted to survive the rename and the reboot — the user-visible rename guarantee stays under test for a new reason); `internal/restore/multipane_legacy_integration_test.go`'s multipane coverage is re-pointed at per-pane tokens and its un-stamped-name-fallback subtests are retired with the branch they cover.
```

**Resolution**: Pending
**Notes**:

---

### 3. Nothing stops deregistration from changing the pane's identity

**Type**: Incomplete coverage
**Spec Reference**: §4.2 (removal does not mint and does not unstamp; a pane whose entry is removed keeps its token, and clearing it would add a tmux write that can fail after the entry is already gone)
**Plan Reference**: Phase 4, task `resume-hooks-silently-lost-4-2` — **Acceptance Criteria** and **Tests**
**Move**: settled
**Change Type**: add-to-task

**Problem**:
Removing a hook must leave the pane's identity exactly as it found it. Registration mints a token and writes it onto the pane; removal shares the very same pane-resolving code path and must do neither. The task states that in its background prose and nowhere else — no criterion and no test would fail if an implementation folded the mint into the shared path. If that happens, every `hook rm` re-stamps the pane with a fresh identity, and any entry still keyed to the old one is orphaned and reaped by the sweep within ten seconds: the precise loss this work exists to remove, reintroduced through the deregistration door and invisible until a user's hook goes missing after they deregistered a different one. The mirror mistake — clearing the token on the way out — costs the same thing, and adds a tmux write that can fail after the entry is already gone.

**Proposal**:
Give the rule a criterion and a test, on both of removal's paths. The specification states it as a rule with its reason attached, and the task already carries the reason; what it lacks is anything that checks it. The stamper seam registration uses is already injected in these tests, so asserting it recorded nothing costs one line per case.

**Current**:

*Acceptance Criteria — the two lines the new criterion sits between:*
```markdown
- [ ] No empty key reaches `Store.Remove` from either path
- [ ] `hook rm` writes no `save.requested` on any path
```

*Tests — the line the new test sits above:*
```markdown
- `"it touches no dirty flag on either path"` — assert no `save.requested` after both a success and a failure
```

**Proposed Text**:

*Acceptance Criteria:*
```markdown
- [ ] No empty key reaches `Store.Remove` from either path
- [ ] `hook rm` mints no token and issues no pane-option write on either path — not on a successful removal, not for a live pane carrying no token, and not on `--pane-key`: removal neither mints nor unstamps, so a pane whose entry is removed keeps the identity it had
- [ ] `hook rm` writes no `save.requested` on any path
```

*Tests:*
```markdown
- `"it mints and stamps nothing on either path"` — table over a successful resolved-token removal, a live pane carrying no token, and the `--pane-key` path; assert the stamper seam recorded zero calls in every case
- `"it touches no dirty flag on either path"` — assert no `save.requested` after both a success and a failure
```

**Resolution**: Pending
**Notes**:

---
