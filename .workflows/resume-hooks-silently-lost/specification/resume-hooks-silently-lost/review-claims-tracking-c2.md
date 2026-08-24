# Review Tracking: Resume Hooks Silently Lost - Claims Verification

## Findings

### 1. The integration seeds carry their key literals at the call sites, not in the shared helper

**Source**: Tree measurement — `grep -rn 'SeedHooksJSON(' --include='*.go' . | grep -v 'transienttest/hooks.go'`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §9.3 Existing tests to re-point or retire

**Problem**:
§9.3 tells whoever builds this that the key-shape change reaches the destructive integration suites through one file — amend `internal/transienttest` and the consumers follow. The seeder does not work that way: `SeedHooksJSON` writes whatever `{key: command}` map its caller hands it and holds no key literal of its own, so the old positional keys are authored at four separate call sites, and one of them derives its "live" key from `tmux.StructuralKeyFormat` — the positional format this work unit abandons — rather than from a literal at all. Built to the section as written, the daemon-sweep and `doctor --fix` suites ship still seeding `alpha:0.0`-shaped keys and asserting against a key scheme the release no longer produces, which is exactly the coverage that should have caught the change going wrong.

**Proposal**:
Replace the bullet: the helper is key-shape agnostic and needs no amendment; name the four seeding call sites as the places the shapes are re-pointed, and flag that `cmd/state_daemon_hook_cleanup_integration_test.go` additionally has to stop reading its live key through `tmux.StructuralKeyFormat`.

**Evidence**:
Claim (§9.3): "**`internal/transienttest`** `SeedHooksJSON` / `HooksJSONBytes` (`internal/transienttest/hooks.go:38,57`) — the single-sourced hook seeder for the two destructive integration suites. The key-shape change routes through it, so it is amended once rather than at each consumer."

```
$ grep -n "func SeedHooksJSON" -A 3 internal/transienttest/hooks.go
38:func SeedHooksJSON(t *testing.T, env []string, entries map[string]string) {
39-	t.Helper()
40-	path := ResolveHooksFilePathFromEnv(t, env)
41-	t.Logf("transienttest.SeedHooksJSON: resolved hooks.json path = %s", path)

$ grep -rn 'SeedHooksJSON(' --include='*.go' . | grep -v 'transienttest/hooks.go'
cmd/state_daemon_hook_cleanup_integration_test.go:89:	transienttest.SeedHooksJSON(t, env, map[string]string{
cmd/cleanstale_transient_listpanes_shared_test.go:58:	transienttest.SeedHooksJSON(t, env, transientModeSeedEntries)
cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:95:		transienttest.SeedHooksJSON(t, env, seedEntries)
cmd/bootstrap/transient_listpanes_helpers_integration_test.go:107:		transienttest.SeedHooksJSON(t, env, entries)

$ grep -rn '0\.0"' cmd/cleanstale_transient_listpanes_shared_test.go cmd/state_daemon_hook_cleanup_integration_test.go cmd/bootstrap/transient_listpanes_helpers_integration_test.go
cmd/state_daemon_hook_cleanup_integration_test.go:43:	staleHookKey = "gone-XxXxXx:0.0"
cmd/cleanstale_transient_listpanes_shared_test.go:48:	"alpha:0.0": "echo a",
cmd/cleanstale_transient_listpanes_shared_test.go:49:	"beta:0.0":  "echo b",
cmd/cleanstale_transient_listpanes_shared_test.go:50:	"gamma:0.0": "echo c",
cmd/bootstrap/transient_listpanes_helpers_integration_test.go:104:			"smoke:0.0": "echo hello",

$ sed -n '91,94p' cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go
		seedEntries := map[string]string{
			"live:0.0": "echo live",
			"gone:0.0": "echo gone",
		}

$ sed -n '80,83p' cmd/state_daemon_hook_cleanup_integration_test.go
	liveHookKey := strings.TrimSpace(sock.Run(t, "list-panes",
		"-t", liveWorkSession, "-F", tmux.StructuralKeyFormat))
	if liveHookKey == "" {
		t.Fatalf("could not read live pane structural key for session %q", liveWorkSession)
```

`internal/transienttest/hooks.go` contains no hook-key literal anywhere in the file (68 lines; the only key-shaped strings are the caller's map values passed through to `store.Set`). The investigation carries the weaker "a key-shape change must route through it" (`investigation/resume-hooks-silently-lost.md:578-579`); the "amended once rather than at each consumer" inference is the specification's own.

**Current**:
- **`internal/transienttest`** `SeedHooksJSON` / `HooksJSONBytes` (`internal/transienttest/hooks.go:38,57`) — the single-sourced hook seeder for the two destructive integration suites. The key-shape change routes through it, so it is amended once rather than at each consumer.

**Proposed Text**:
- **The seeded keys in the destructive integration suites** — `internal/transienttest.SeedHooksJSON` (`internal/transienttest/hooks.go:38`) writes whatever `{key: command}` map its caller hands it and carries no key literal of its own, so the helper is unchanged and the shapes are re-pointed at each of the four seeding call sites: `cmd/cleanstale_transient_listpanes_shared_test.go:48-50` (`alpha:0.0` / `beta:0.0` / `gamma:0.0`), `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:91-94` (`live:0.0` / `gone:0.0`), `cmd/bootstrap/transient_listpanes_helpers_integration_test.go:104-105` (`smoke:0.0` / `smoke:1.0`), and `cmd/state_daemon_hook_cleanup_integration_test.go:43,89-92`. The last one also reads its *live* key with `tmux.StructuralKeyFormat` (`:80-81`), which is a structural key rather than a hook key after this change: it reads the pane's token instead.

**Resolution**: Pending
**Notes**:

---

### 2. Three existing tests build on the deleted `@portal-id` machinery and are not listed

**Source**: Tree measurement — `grep -rln 'HookKeyFormat\|tmux.HookKey(\|PortalIDOption\|PortalID\|@portal-id' --include='*_test.go' .`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §9.3 Existing tests to re-point or retire

**Problem**:
§9.3 and §9.4 enumerate the existing tests that stand on the machinery §3.3 and §7.2 delete, and three files that stand on it are missing from the enumeration. One of them, `internal/restore/multipane_legacy_integration_test.go`, calls the deleted `tmux.HookKey` four times and drives `session.PortalIDOption` and `Session.PortalID` directly — the `restore` package's integration lane will not compile once the deletions land, and half that file tests the un-stamped-name-fallback branch that no longer exists. A reader planning delivery from this section budgets for a compile break they were not told about, in the lane that is slowest to discover it.

**Proposal**:
Add the three measured files to the enumeration, applying §9.3's own established rule — coverage of a user-visible guarantee is re-pointed, coverage of a deleted branch is retired: `internal/restore/multipane_legacy_integration_test.go` (multipane restore re-pointed at the pane token, its un-stamped-name-fallback subtests retired alongside `cmd/hookkey_no_regression_upgrade_test.go`), `internal/restore/rename_reboot_shared_test.go` (the scaffolding the re-pointed rename tests share), and `cmd/state_daemon_run_test.go` (the fabricated `captureFormat` row whose trailing column §2.3 swaps).

**Evidence**:
```
$ grep -rln 'HookKeyFormat\|tmux.HookKey(\|PortalIDOption\|PortalID\|@portal-id' --include='*_test.go' . | sort
cmd/hookkey_no_regression_upgrade_test.go
cmd/portal_id_binding_guard_test.go
cmd/rename_restore_cleanup_survival_integration_test.go
cmd/state_daemon_run_test.go
internal/restore/multipane_legacy_integration_test.go
internal/restore/rename_reboot_durability_integration_test.go
internal/restore/rename_reboot_hook_integration_test.go
internal/restore/rename_reboot_shared_test.go
internal/restore/session_test.go
internal/session/create_test.go
internal/session/quickstart_test.go
internal/state/capture_internal_test.go
internal/state/capture_test.go
internal/state/portal_id_literal_guard_test.go
internal/state/schema_test.go
internal/tmux/hookkey_cross_site_realtmux_test.go
internal/tmux/hookkey_format_realtmux_test.go
internal/tmux/hookkey_test.go
internal/tmux/list_all_pane_hookkeys_realtmux_test.go
internal/tmux/resolve_hookkey_realtmux_test.go
```
17 of the 20 are named by §9.3 or §9.4 (`internal/state/*` covers the four state files). Unnamed: `cmd/state_daemon_run_test.go`, `internal/restore/multipane_legacy_integration_test.go`, `internal/restore/rename_reboot_shared_test.go`.

```
$ grep -n 'tmux.HookKey(\|PortalIDOption\|sess.PortalID' internal/restore/multipane_legacy_integration_test.go
54:	pane0Key := tmux.HookKey(renamePortalID, renameOldName, 0, 0)
55:	pane1Key := tmux.HookKey(renamePortalID, renameOldName, 0, 1)
89:		if err := client.SetSessionOption(renameOldName, session.PortalIDOption, renamePortalID); err != nil {
90:			t.Fatalf("SetSessionOption %s=%s: %v", session.PortalIDOption, renamePortalID, err)
103:	if sess.PortalID != renamePortalID {
171:	legacyKey := tmux.HookKey("", legacyName, 0, 0)
200:		if sess.PortalID != "" {
204:	bakedKey := tmux.HookKey(sess.PortalID, sess.Name, 0, 0)

$ sed -n '15,18p' internal/restore/rename_reboot_shared_test.go
const (
	renamePortalID = "tok123"
	renameOldName  = "renamesrc"
	renameNewName  = "renamedst"

$ sed -n '207,212p' cmd/state_daemon_run_test.go
func oneSession() (sessionsOut, panesOut string) {
	sessionsOut = "work|1|0|"
	// Fields match captureFormat; the trailing empty one is the un-stamped
	// @portal-id column a legacy session resolves to "".
	panesOut = "work|||0|||main|||layout|||0|||1|||0|||/tmp|||1|||zsh|||"
	return
}
```

**Current**:
- **`internal/session/create_test.go`**, `quickstart_test.go`, `internal/state/*`, `internal/restore/session_test.go`, `cmd/hooks_test.go`, `cmd/state_hydrate_test.go` — updated in step with §7.2.

**Proposed Text**:
- **`internal/session/create_test.go`**, `quickstart_test.go`, `internal/state/*`, `internal/restore/session_test.go`, `cmd/hooks_test.go`, `cmd/state_hydrate_test.go` — updated in step with §7.2.
- **`internal/restore/multipane_legacy_integration_test.go`** — builds its expected keys with the deleted `tmux.HookKey` (`:54,55,171,204`) and drives `session.PortalIDOption` / `Session.PortalID` directly (`:89-90,103,200`), so the `restore` integration lane does not compile until it is dealt with. Its multipane restore coverage is re-pointed at the pane token; its un-stamped-name-fallback subtests cover the branch §7.2 deletes and retire with `cmd/hookkey_no_regression_upgrade_test.go`.
- **`internal/restore/rename_reboot_shared_test.go`** — the `renamePortalID` scaffolding (`:16`) shared by the two re-pointed rename-reboot tests; amended with them.
- **`cmd/state_daemon_run_test.go`** — its `oneSession()` fixture (`:207-212`) fabricates a `captureFormat` pane row whose trailing column is the `@portal-id` field §2.3 swaps for `@portal-pane-id`; the field count is unchanged at 11, the column's meaning is not.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The compile break in the `restore` integration lane is the load-bearing part — nothing in the section warned of it. Applied under `auto`.

---

### 3. The daemon's tick loop is 1s; 10s is the sweep's throttle

**Source**: Tree measurement — `grep -n 'TickerPeriod:\|hookCleanupInterval =' cmd/state_daemon.go`
**Category**: Source defect
**Move**: route
**Affects**: §6.5 Acquisition is bounded, and a timeout degrades rather than wedges

**Problem**:
§6.5 justifies the bounded lock by saying an unbounded acquire would park "the daemon's 10s tick loop". The daemon's loop runs at 1s and does the capture work on it; 10s is only the throttle the hook sweep sits behind on that loop's idle branch. As written the hazard reads as a slow background prune being delayed, when what actually stalls behind a held lock is the 1s capture cycle — a reader weighing the 2s bound against an unbounded acquire is weighing it against an understated cost.

**Evidence**:
Claim (§6.5): "an unbounded acquire would park the daemon's 10s tick loop behind it — the blocking path the investigation named as the single riskiest part of this work unit."

```
$ grep -n 'TickerPeriod:\|hookCleanupInterval =' cmd/state_daemon.go
105:const hookCleanupInterval = 10 * time.Second
424:			TickerPeriod:       1 * time.Second,
```
`tick` (`cmd/state_daemon.go:173`) runs on that 1s ticker and reaches `maybeRunHookCleanup` only on the not-dirty/not-gap branch, where `hookCleanupInterval` throttles the sweep to 10s.

Carried from the source: `investigation/resume-hooks-silently-lost.md:749` — "unbounded `LOCK_EX` would park the daemon's 10s tick loop behind it — the regression risk D…".

**Resolution**: Routed
**Notes**: Source defect, routed per resolve-source-incoherence. Re-measured: `cmd/state_daemon.go:424` `TickerPeriod: 1 * time.Second`, `:105` `hookCleanupInterval = 10 * time.Second`. The corrected value strengthens the decision it supports rather than undermining it — what stalls behind a held lock is the 1s capture cycle, not a background prune — so it landed without a gate. Correction written into `investigation/resume-hooks-silently-lost.md:749`, reindexed, committed as `6c9002ea`; the specification's §6.5 sentence re-aligned to it.

---

### 4. The `set skeleton marker failed` emission is not in the loop the re-stamp lands in

**Source**: Tree measurement — `grep -n 'func (r \*SessionRestorer) armPanes\|func (r \*SessionRestorer) ApplySkeletonMarkers\|setSkeletonMarker' internal/restore/session.go`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §2.4 Restore re-stamp: failures are surfaced, mispairings are not stamped

**Problem**:
§2.4 points at `set skeleton marker failed` as the neighbouring emission "in the same loop" as the pane re-stamp. It is in a different loop, in a later restore phase: it fires from `ApplySkeletonMarkers`, which runs after `armPanes` has returned and iterates **every** live pane, while §2.3 puts the re-stamp inside `armPanes` before each pane is armed and §2.4 confines it to the `pairCount` prefix. Following the pointer puts the token write in the one loop that has no `pairCount` bound — the mispairing case §2.4 exists to prevent, and the one whose consequence it describes as permanent.

**Proposal**:
Keep the emission as the shape and attr-set precedent (it matches exactly: `restore` component, `session` / `pane_key` / `error`) and drop the "same loop" locator, which is what makes the reference misread as a placement instruction.

**Evidence**:
Claim (§2.4): "That is the shape and the exact attr set of the neighbouring `set skeleton marker failed` emission in the same loop (`internal/restore/session.go:274`), so **no new component and no new attr key** is introduced."

```
$ grep -n 'func (r \*SessionRestorer) armPanes\|pairCount :=\|func (r \*SessionRestorer) ApplySkeletonMarkers\|setSkeletonMarker\|set skeleton marker failed' internal/restore/session.go
119:func (r *SessionRestorer) armPanes(sess state.Session, armInfos []savedPaneArmInfo) ([]tmux.PaneCoord, error) {
129:	pairCount := min(len(livePanes), len(armInfos))
247:func (r *SessionRestorer) ApplySkeletonMarkers(sess state.Session, livePanes []tmux.PaneCoord) {
252:		r.setSkeletonMarker(sess.Name, liveKey)
272:func (r *SessionRestorer) setSkeletonMarker(sessionName, liveKey string) {
274:		r.logger().Warn("set skeleton marker failed", "session", sessionName, "pane_key", liveKey, "error", err)

$ sed -n '247,254p' internal/restore/session.go
func (r *SessionRestorer) ApplySkeletonMarkers(sess state.Session, livePanes []tmux.PaneCoord) {
	savedCount := countSavedPanes(sess)
	r.warnOnPaneCountMismatch(sess.Name, len(livePanes), savedCount)

	for _, live := range livePanes {
		liveKey := state.SanitizePaneKey(sess.Name, live.Window, live.Pane)
		r.setSkeletonMarker(sess.Name, liveKey)
	}
}
```
The attr set the claim rests on does hold: `restore` component, `session` / `pane_key` / `error`, no new vocabulary. Only the locator is wrong.

**Current**:
That is the shape and the exact attr set of the neighbouring `set skeleton marker failed` emission in the same loop (`internal/restore/session.go:274`), so **no new component and no new attr key** is introduced.

**Proposed Text**:
That is the shape and the exact attr set of restore's own `set skeleton marker failed` emission (`internal/restore/session.go:274`), so **no new component and no new attr key** is introduced.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. The locator was introduced by this session's own finding-6 edit in cycle 1 and was wrong: `set skeleton marker failed` fires from `ApplySkeletonMarkers` (`internal/restore/session.go:247-254`), a later phase iterating every live pane, not from `armPanes`' `pairCount`-bounded loop. The attr-set precedent it was cited for holds; only the locator is dropped. Applied under `auto`.

---
