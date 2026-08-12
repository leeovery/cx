TASK: theming-system-16-7 — Declare The Shared Nav/Page Keymap Entries Once (tick-7826bd, severity low, source: duplication)

ACCEPTANCE CRITERIA:
1. The nav and page entries are written once; no descriptor restates either literal.
2. All three descriptors still return their entries in their current order.
3. `previewKeymap` is untouched.
4. Footer and help-modal rendering is byte-identical on Sessions, Projects and the theme panel.
5. `go test ./internal/tui ./internal/capture` passes, including the descriptor↔dispatch parity guard.

STATUS: complete

SPEC CONTEXT:
The task is a duplication-remediation item from analysis cycle 16, not a spec-driven behaviour change, so the spec constrains it only as a "do not break" boundary. Two clauses are load-bearing:
- §9.12 (specification.md:1222-1227) — "The panel's keys live in the keymap descriptor as a panel scope — **all six**: ↑/↓, Ctrl+↑/Ctrl+↓, Enter, d, l, Esc. The descriptor must be complete or the dispatch guard's descriptor↔dispatch parity is what breaks." Arrows and paging are required present-but-non-Core, exactly as §14.1 treats them on the main footer. Refactoring the arrows out to a shared helper is legal only if `themePanelKeymap()` still *returns* all six; it does (internal/tui/keymap.go:64-71 → 2 shared + 4 own).
- specification.md:1200 — the `t` block is a call-site filter so "the static descriptor is unchanged, so the keymap dispatch guard stays green". The task's Do item 4 restates this; the change preserves it (the descriptors take no model and do no filtering).
The spec says nothing that constrains *how* the nav rows are declared, so the DRY refactor is spec-neutral.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/tui/keymap.go:19-25 — new `navKeymapEntries() []keymapEntry` returning the two non-Core rows in their prior order with unchanged `Key`/`HelpKey`/`Action`/`HelpAction` values.
  - internal/tui/keymap.go:29-44 (`sessionsKeymap`), :47-60 (`projectsKeymap`), :64-71 (`themePanelKeymap`) — each now `return append(navKeymapEntries(), []keymapEntry{…}...)`.
  - internal/tui/keymap.go:93-103 (`previewKeymap`) — unchanged.
  - Commit 53d26e0a; diff confirms the only production edit is the extraction plus the three `append(…)` rewrites, with every surviving entry byte-identical and in its prior position.
- Notes:
  - AC1 verified by search: the only production-code occurrences of the two literals are keymap.go:22-23. keymap.go:96 carries `{Key: "^↑/↓", Action: "page", HelpAction: "Page up / down"}` — a *different* row (distinct `HelpAction`), which Do item 3 explicitly required be left alone. Correct call: folding it in would silently relabel the preview help.
  - AC2 verified against the diff — no entry moved; nav-first ordering was already the pre-existing order in all three, so leading with the shared pair is a no-op on order.
  - AC4 follows structurally: the footer (`splitFooterEntries` → Core only) and help body (`helpModalBody` → all entries) are pure functions of the descriptor slice, and the slice is element-wise and order-wise identical. `internal/capture` holds no keymap reference of its own (grep clean), so its fixtures render through the same unchanged descriptors.
  - Aliasing checked and safe: `navKeymapEntries` returns a fresh composite literal (cap == len == 2) on every call, so each `append` reallocates and the three descriptors share no backing array. No caller mutates a returned descriptor in place either — `dropKeymapKey` (internal/tui/model.go:3303-3312) builds a new slice.
  - Purity preserved (Do item 4): `sessionsHelpKeymap` / `projectsHelpKeymap` (internal/tui/model.go:3270-3288) still apply the `t`/`m` blocks at the call site; the descriptors take no receiver and read no model state.
  - No drift from the plan's intent. The one later change to this file is the repo-wide comment strip/audit (e3fa1503, 915e7fcb), which trimmed the helper's doc comment; per the verifier-context note that is a deliberate later supersede, not drift.

TESTS:
- Status: Adequate
- Coverage:
  - internal/tui/keymap_test.go:110-145 — `TestNavKeymapEntries`, added by this commit. Subtest 1 pins the shared pair's exact contents and order; subtest 2 ("every list descriptor leads with the shared pair") asserts `sessionsKeymap`, `projectsKeymap` and `themePanelKeymap` all open with it. This is precisely the test the task asked for.
  - Pre-existing full-descriptor golden order tests all survive unchanged and independently restate the two nav rows in their `want` literals, which is what makes AC2/AC4 enforceable rather than assumed: keymap_test.go:8-33 (Sessions, 14 entries), projects_keymap_test.go:8-31 (Projects, 12 entries), theme_panel_keymap_test.go:8-28 (panel, all six per §9.12).
  - Descriptor↔dispatch parity still probes the full descriptors: keymap_dispatch_guard_test.go:129 / :192 and keymap_dispatch_guard_theme_test.go:153 — including the `↑↓` probe at :134-139, so the extracted rows remain covered by the guard rather than only by equality assertions.
  - Rendering byte-identity is held by the existing footer/help suites that consume the descriptors directly (footer_test.go, footer_revision_test.go:290-307, projects_footer_test.go, help_modal_test.go, theme_panel_footer_test.go, theme_panel_keymap_test.go:71-74 and :134-138).
  - The `previewKeymap`-stays-distinct invariant is covered behaviourally: pagepreview_help_test.go:51-62 asserts the rendered preview help contains "Page up / down", so a future attempt to fold `previewKeymap` into the shared helper fails a test rather than silently changing copy. Worth recording because the commit's original comment explaining that carve-out was later removed by the comment audit — the guard, not the comment, is what holds it.
- Notes: Not over-tested. Subtest 2 overlaps the three full-order goldens, but it is the direct statement of the new invariant (descriptors lead with the *shared* pair, not merely with equal values) and catches a descriptor that stops using the helper while its golden still passes. That is a distinct failure mode, so the overlap is justified rather than redundant. No new test was warranted beyond this — the change is behaviour-preserving by construction and the byte-identity claim is already carried by the untouched suites.

CODE QUALITY:
- Project conventions: Followed. Pure static descriptor functions per the file's own contract; no model access, no filtering, no rendering in keymap.go. Comments on the changed code are accurate — keymap.go:19 ("Shared so the arrow glyphs and help labels cannot diverge between descriptors") is true; keymap.go:27-28 and :46 ("the `t`/`m` blocked-key filters are applied at the call site") check out against model.go:3270-3288; keymap.go:62-63 ("Complete scope — every key the panel dispatches, arrows and paging included") remains true post-extraction. No process-artifact references, no restated code.
- SOLID principles: Good. Single source for a shared display row; the three descriptors keep sole ownership of their own entries.
- Complexity: Low. Three one-line call-shape changes; no branches added.
- Modern idioms: Yes, with one small opportunity (see the note below on `slices.Concat`, available on go 1.26.0 and already used at internal/theme/builtins_nord_test.go:113).
- Readability: Good. `navKeymapEntries` is well named and the `append(nav, own...)` shape reads as "nav first, then the surface's own keys", which mirrors the ordering contract.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/keymap.go:30, :48, :65 — replace `append(navKeymapEntries(), []keymapEntry{…}...)` with `slices.Concat(navKeymapEntries(), []keymapEntry{…})` in all three descriptors. Correct today only because `navKeymapEntries` returns a cap == len composite literal, so the append reallocates and the three descriptors cannot share a backing array; if that helper ever became a package-level var grown by a later `append`, three call sites would start writing into one array. `slices.Concat` makes the independence unconditional instead of an invisible invariant of the callee, is behaviour-identical, and the repo is on go 1.26.0 with the idiom already in use. Low priority — no current defect.
