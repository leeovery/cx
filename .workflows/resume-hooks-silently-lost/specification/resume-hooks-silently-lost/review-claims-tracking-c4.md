# Review Tracking: Resume Hooks Silently Lost - Claims Verification

## Findings

### 1. Three `cmd` test files hold the pane enumeration's seam and are absent from the re-point list

**Source**: Tree measurement — `grep -rn "ListAllPaneHookKeys" --include="*.go" . | sort`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §9.3 Existing tests to re-point or retire

**Problem**:
The all-pane enumeration changes shape — from a flat list of hook-key strings to one row per live pane carrying a token and a location. The `cmd` package declares its own seam for that call (`AllPaneLister`), and three test files in `cmd` implement or drive it. None of the three appear in the list of existing tests to re-point, and the list is what the delivery plan is built from. Two of them also seed old-format key literals. As it stands, the change lands and the `cmd` test build does not compile — the same breakage the specification flags for the restore integration lane, on a package it does not name.

**Proposal**:
Add the three files to the re-point list: `cmd/run_hook_stale_cleanup_test.go` (`recordingHookKeyLister`, plus seeded `a:0.0` / `b:0.0` style keys and the subtest asserting the enumeration method), `cmd/doctor_test.go` (`fakeHookLister`, seeded `sessA:0.0` keys), and `cmd/bootstrap_production_test.go` (`stubAllPaneLister`). Measurement determined this: they are the only remaining implementors and drivers of `ListAllPaneHookKeys` in the tree that the list does not already name.

**Evidence**:

Claim (§9.3, the list of existing tests to re-point or retire) names, for the `cmd` package, only:

> **`cmd/hookkey_no_regression_upgrade_test.go`** — asserts an un-stamped session's name-keyed hook survives … Retired.
> **`cmd/rename_restore_cleanup_survival_integration_test.go`**, …
> **`internal/session/create_test.go`**, `quickstart_test.go`, `internal/state/*`, `internal/restore/session_test.go`, `cmd/hooks_test.go`, `cmd/state_hydrate_test.go` — updated in step with §7.2.
> **`cmd/state_daemon_run_test.go`** — its `oneSession()` fixture …

and §3.3 states the enumeration's shape change:

> `ListAllPaneHookKeys()` becomes an all-pane enumeration returning **one row per live pane**, each row carrying the pane's `@portal-pane-id` (empty for an unstamped pane) and its `<session>:<window>.<pane>` location.

Command:

```
$ grep -rn "ListAllPaneHookKeys" --include="*.go" . | sort
cmd/bootstrap_production_test.go:91:func (s *stubAllPaneLister) ListAllPaneHookKeys() ([]string, error) {
cmd/doctor_test.go:802:func (f fakeHookLister) ListAllPaneHookKeys() ([]string, error) { return f.keys, f.err }
cmd/doctor.go:289:	live, err := lister.ListAllPaneHookKeys()
cmd/hookkey_no_regression_upgrade_test.go:77:	live, err := lister.ListAllPaneHookKeys()
cmd/rename_restore_cleanup_survival_integration_test.go:96:	live, err := lister.ListAllPaneHookKeys()
cmd/run_hook_stale_cleanup_test.go:19:func (r *recordingHookKeyLister) ListAllPaneHookKeys() ([]string, error) {
cmd/run_hook_stale_cleanup_test.go:261:	t.Run("it enumerates live keys via ListAllPaneHookKeys not ListAllPanes", func(t *testing.T) {
cmd/run_hook_stale_cleanup_test.go:273:			t.Errorf("ListAllPaneHookKeys call count = %d, want 1 (the enumeration must switch to the hook-key method)", rec.hookKeyCalls)
cmd/run_hook_stale_cleanup.go:13:	ListAllPaneHookKeys() ([]string, error)
cmd/run_hook_stale_cleanup.go:26:	livePanes, err := lister.ListAllPaneHookKeys()
internal/tmux/hookkey_cross_site_realtmux_test.go:39,63,114
internal/tmux/list_all_pane_hookkeys_realtmux_test.go:13,32,41,57,66,88,101,125,139,142,144,147,150,154,167
internal/tmux/tmux.go:568,579,584
```

`cmd/hookkey_no_regression_upgrade_test.go`, `cmd/rename_restore_cleanup_survival_integration_test.go` and the two `internal/tmux` files are already in the list; `cmd/run_hook_stale_cleanup_test.go`, `cmd/doctor_test.go` and `cmd/bootstrap_production_test.go` are not.

The `cmd`-side seam whose shape they mirror:

```
$ sed -n '9,14p' cmd/run_hook_stale_cleanup.go
// AllPaneLister returns every live pane's hook key, in the same
// <@portal-id or session_name>:window.pane form registration writes — a divergent
// form reaps freshly-registered entries as stale.
type AllPaneLister interface {
	ListAllPaneHookKeys() ([]string, error)
}
```

Old-format key literals seeded by two of the three:

```
$ grep -n "0\.0" cmd/run_hook_stale_cleanup_test.go | head -6
35:  "a:0.0": {"on-resume": "cmd-a"},
36:  "b:0.0": {"on-resume": "cmd-b"}
98:		seed := `{"a:0.0": {"on-resume": "cmd-a"}}`
140:		lister := &stubAllPaneLister{panes: []string{"a:0.0"}, err: nil}
158:  "a:0.0": {"on-resume": "cmd-a"},
159:  "b:0.0": {"on-resume": "cmd-b"},

$ grep -n "0\.0" cmd/doctor_test.go | head -4
854:	hookStore, hooksPath := seedHooksJSON(t, "sessA:0.0")
859:	// A live-pane set excluding sessA:0.0 makes it stale, and its non-emptiness
861:	lister := fakeHookLister{keys: []string{"sessB:0.0"}}
873:	if strings.Contains(string(hooksAfter), "sessA:0.0") {
```

**Current**:

> - **The seeded keys in the destructive integration suites** — `internal/transienttest.SeedHooksJSON` (`internal/transienttest/hooks.go:38`) writes whatever `{key: command}` map its caller hands it and carries no key literal of its own, so the helper is unchanged and the shapes are re-pointed at each of the four seeding call sites: `cmd/cleanstale_transient_listpanes_shared_test.go:48-50` (`alpha:0.0` / `beta:0.0` / `gamma:0.0`), `cmd/cleanstale_transient_listpanes_doctorfix_integration_test.go:91-94` (`live:0.0` / `gone:0.0`), `cmd/bootstrap/transient_listpanes_helpers_integration_test.go:104-105` (`smoke:0.0` / `smoke:1.0`), and `cmd/state_daemon_hook_cleanup_integration_test.go:43,89-92`. The last one also reads its *live* key with `tmux.StructuralKeyFormat` (`:80-81`), which is a structural key rather than a hook key after this change: it reads the pane's token instead.

**Proposed Text**:

Insert the following bullet immediately after the "seeded keys in the destructive integration suites" bullet, leaving that bullet unchanged:

> - **The `cmd`-side enumeration seam and its three fakes** — `AllPaneLister` (`cmd/run_hook_stale_cleanup.go:13`) mirrors `ListAllPaneHookKeys`, so its method signature and its doc comment (which names the `<@portal-id or session_name>:window.pane` form) move to the two-field rows of §3.3 with it. Three unit-lane files in `cmd` implement or drive that seam and do not compile until they do: `cmd/bootstrap_production_test.go:91` (`stubAllPaneLister`), `cmd/doctor_test.go:802` (`fakeHookLister`), and `cmd/run_hook_stale_cleanup_test.go:19` (`recordingHookKeyLister`, whose `:261-273` subtest asserts the sweep enumerates through this method). The first two also carry old-format key literals — `a:0.0` / `b:0.0` (`cmd/run_hook_stale_cleanup_test.go:35-36,98,140,158-161`) and `sessA:0.0` / `sessB:0.0` (`cmd/doctor_test.go:854,861,903,946`) — re-pointed at tokens alongside the seam, and `cmd/run_hook_stale_cleanup_test.go` additionally covers the row-counting guard of §5.4.

**Resolution**: Approved
**Notes**: Applied verbatim as proposed. Same class as the cycle-2 finding about the restore integration lane — a compile break on a package the enumeration's shape change reaches through a `cmd`-local seam the spec never named. The `AllPaneLister` doc comment still describes the old key form, so it moves with the signature. Applied under `auto`.

---
