## Attempt 1

ISSUES:

- `internal/tui/theme_panel_commit_slot_test.go:630-666` — **`TestPanelSlotCommit_FailedWriteLeavesKeysAlone/the keypress mutates nothing` cannot detect a mirror-before-write ordering error**, so the "a failed commit mutates nothing" criterion is verified **vacuously** for the keys.

  `newSlotSplitPanelModel` leaves the cursor on `opened[1]` (the dark slot's own row) and the fixture has no constant, so `mirrorThemeSlot` is the **identity** on those keys — a `commitSlot` that mirrors first and writes second leaves `themeKeys` (and therefore the badges) byte-identical. Confirmed by mutation: with the mirror moved ahead of the persister call, **the whole `internal/tui` package still passes**. This was the one survivor of the reviewer's four-mutation battery.

  The test's own comment at line 626 claims the opposite ("an implementation that mirrored first and wrote afterwards would get wrong in both halves"), so a later task reading it will trust a guarantee that is not there — and **task 9-5 explicitly depends on this path** ("On failure the constant is **not** cleared in memory, so the badges still show it").

  FIX: make the fixture's mirror non-identity — arrow to a row that is *not* already in the target slot before the keypress. At line 635-636 insert `m = arrowToThemeRow(t, m, opened[2].Slug)` before `keys := m.themeKeys`, and update the expectation at line 643 to `slotCommit{slug: opened[2].Slug, slot: prefs.SlotDark}`. Verified in a scratch copy: this leaves the test passing against the current (correct) code and fails the mirror-first mutation with `left keys {Theme: Light:theme-a0 Dark:theme-a2}, want the untouched {… Dark:theme-a1}`.

  Worth also tightening the "the helper returns the error" subtest (line 668-675) to drive `commitSelectedSlot` over a keys shape carrying a **constant** and assert the constant survives the failure — that is the half 9-5 leans on and it is currently untested in either direction. The comment at line 626 then becomes true as written.

  CONFIDENCE: high

NOTES:

MUTATION EVIDENCE (reviewer-run against a scratch copy; the working tree was never touched):

| Mutation | Result |
|---|---|
| transpose the slots in `mirrorThemeSlot` | **killed** — 13 failures |
| drop the constant gate | **killed** — `InertOverAConstant` |
| empty the carried-through pair | **killed** — 13 failures |
| add a second seam call to clear the constant | **killed** — 11 failures |
| move the mirror **ahead** of the write | **SURVIVED the entire package** (see ISSUE) |

- **(a) The gate deviation is correct — keep it.** Routing through `m.themeSetting().IsConstant` is genuinely equivalent to the specified `m.themeKeys.Theme != ""` for every reachable state: `ResolveSetting` sets `IsConstant` iff `StripControl(Theme) != ""`, and production keys arrive already stripped (`cmd/open.go:809` feeds `WithThemeKeys` the `raw` output of the same call). The only divergent shape is a control-only `Theme`, and there the helper is **more** correct — the union assembly (`internal/theme/union.go:365`) and the badge resolution both classify that state as adaptive, so the literal condition would have raised a confirm naming an empty constant while the panel listed and marked a pair. Single-tiebreak-site is the right call.
- **(b) The unused `raiseSlotConfirm` parameter** is neither harmful nor quite the forethought claimed: task 9-5's own text specifies `raiseSlotConfirm(slug string, slot prefs.ThemeSlot)` — it needs the cursor's slug to commit on `y` — so 9-5 will still edit the signature. Harmless as a documented placeholder; not worth changing now.
- **(b) One-atomic-write claim verified.** Exactly one `CommitThemeSlot` call in `commitSlot` (`theme_panel_commit.go:220`); `prefs.Store.SaveThemeSlot` (`internal/prefs/store.go:441`) sets the slot and `f.Theme = ""` inside a single `mutate`/`AtomicWrite`. `requireSlotCommits` additionally asserts `len(p.constants) == 0`, so a second clearing call cannot hide.
- **(c) Untouched-other-slot verified structurally** (`mirrorThemeSlot` copies both fields then overwrites one), including for an empty value — though **no test drives the empty-other-slot case**, which is the *fresh-install* path (absent prefs → `d` → `{Light:"", Dark:slug}`, with `● light` landing on the shipped default via `cmp.Or`). Behaviour is correct because `ResolveSetting` substitutes the default on read; only the coverage is absent. Non-blocking.
- **(d) The hole is genuinely inert.** `handleSlotCommitKey` returns the caller's own copy unmutated; `InertOverAConstant` asserts no seam call, unchanged keys, empty message slot, unchanged `activeTheme`, nil cmd and a **byte-identical frame**, with a positive control on the adaptive path — and dropping the gate fails it. `raiseSlotConfirm`'s doc opens with "it is a NO-OP until task 9-5 fills it in", so it cannot read as complete.
- **(e) The re-pointed `commitSlotForTest` did not weaken task 9-2.** All four call sites pass `keys.Dark`/`prefs.SlotDark` where the fixtures have `Theme == Dark == "sunset"`, so `mirrorThemeSlot` yields exactly the `RawKeys` the old helper assigned by hand; `newRecomputePanelModel` wires a non-nil, non-failing persister so the production path really runs; and the helper now asserts the constant was cleared **by production code** rather than by the fixture. **Strictly stronger.** The mirror-transposition mutation now takes four of 9-2's tests down with it, which is the intended coupling.
- **SPEC_CONFORMANCE conformant**: §9.2's `d`/`l` table row, §8.2's write-side mutual exclusion via one `CommitThemeSlot`, §8.3's empty-string clear, §9.5's `● both`, §9.13's "a failed commit does not move the `●`". The confirm hole matches §9.2 and is left to 9-5 as planned.
- **ARCHITECTURE sound**: `mirrorThemeSlot` is a pure `RawKeys → RawKeys` function that expresses the clear **structurally** (constructs a pair-only value) rather than by an assignment an edit can forget, mirroring `commitConstant`'s shape; the panel re-implements no merge; the recompute stays the single badge/row derivation site.
- **CONVENTIONS followed**: no `t.Parallel()`, seam-injected fakes with positive controls, the arm sits ahead of the swallow-all default, `isRuneKey` matches the page-key idiom, lint/vet clean, `.tick/tasks.jsonl` touched only for this task's own record.
- `TestPanelSlotCommit_NoOtherIO` drives only `d`; `l` reaches the identical `handleSlotCommitKey` → `commitSelectedSlot` → `commitSlot` chain, so the risk is nil, but the criterion says "either key". Not worth a change on its own.
- The §9.12 descriptor↔dispatch parity guard for the panel's six keys is **task 9-10's**, not a gap here; `themePanelKeymap()` already lists `d`/`l` from Phase 8, and Phase 8's `TestPanelRouting_PanelOwnedKeysNeverReachThePage` was written as an absence-of-page-effect assertion and survives the keys becoming writers.
