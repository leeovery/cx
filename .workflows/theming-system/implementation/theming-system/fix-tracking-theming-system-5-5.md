## Attempt 1

ISSUES:

- `internal/theme/events_test.go:776-812` — no test pins that a CONSTANT's `fallback applied` carries no `slot` attr. `TestEvents_AttrKeysAreInTheClosedSet` exercises three of the four distinct record shapes; the constant-fallback shape `{slug, reason}` is never key-set asserted, and `TestEvents_FallbackDifferentReasonEmitsTwice` (line 665), which does use a broken constant, asserts only `slug` and `reason`. Proven by a surviving mutation: making `FallbackApplied` append `"slot", "constant"` only when the slot is unnamed leaves the whole suite green. The task's edge-case list states "`slot` is carried only under a pair and is absent under a constant" for both events; today only `loaded` is pinned.

  FIX: add `if record.HasAttr("slot") { t.Errorf(...) }` to the loop in `TestEvents_FallbackDifferentReasonEmitsTwice` (`internal/theme/events_test.go:690-698`) — the fixture is already a constant that falls back twice, so it costs three lines and kills the mutation.

  ALTERNATIVE: extend `TestEvents_AttrKeysAreInTheClosedSet` with a third resolution of a broken constant and add `{"slug", "reason"}` + `{"slug"}` to `wantKeys`. Stronger (exact key set, in the test that owns the key-set contract) but reorders the existing expectation table; the first option is the smaller edit. The reviewer recommends the alternative if the key-set contract should stay exhaustive in one place.

  CONFIDENCE: high

- `internal/theme/events.go:44` — the catalogue now reads `theme: commit failed  WARN  Phase 9's theme persister`, changed from `Phase 6`. `planning.md:163` and task `theming-system-6-7` ("The `WithThemePersister` seam and the `cmd`-owned theme persister … is the **single emission site** for `theme: commit failed`") put both the persister and the emission in Phase 6; Phase 9 task 9-7 owns only the panel's outstanding-failure state, i.e. the consumer. The pre-existing `Phase 6` was correct, so this "correction" (self-reported in the executor's summary) regresses a load-bearing handoff comment. The task's own context line ("are Phase 6 and Phase 9's persister") is ambiguously worded; planning.md resolves it.

  **The orchestrator independently verified this against the plan: task 6-7 is in Phase 5's successor phase (Phase 6) and its text names it "the single emission site for `theme: commit failed`". Phase 9 task 9-7's text says "the error arrives from the persister, which is also the single emission site for `theme: commit failed` (Phase 6 task 6-7)". The reviewer is correct; restore `Phase 6`.**

  FIX: restore `theme: commit failed  WARN  Phase 6` (optionally `Phase 6's cmd-owned theme persister` to keep the site named without the wrong phase number).

  CONFIDENCE: medium — high on the plan's content, medium only because the task text is ambiguous.

- `internal/theme/events_test.go:23-24` — "Only four are reachable in this phase; `slot`, `count` and `rejected` belong to events Phases 5 and 8 add" is now false and is contradicted by a test 30 lines below it (`TestEvents_AttrKeysAreInTheClosedSet` asserts `slot` on three records). The task's "update the in-source event catalogue comment" was applied to `events.go` but this parallel statement in the file the executor edited was left behind.

  FIX: reword to match `events.go:46-48` — five keys are reachable; `count` and `rejected` arrive with Phase 8's `theme: enumerated`.

  CONFIDENCE: high

NOTES:

- SPEC_CONFORMANCE conformant on behaviour — §12.3's cadence (one `loaded` per nominated slot, never a combined line), the "loaded names the FALLBACK's slug" rule, WARN/INFO split, `slot` rendered `light`/`dark` and absent under a constant, dedup on `slug`+`reason` held on the injected instance, and unchanged emission control (`cmd/theme.go:120` and `cmd/capturetool/main.go:161` still `log.Discard()`, `cmd/open.go:735` still the real binding).
- ACCEPTANCE_CRITERIA: all 10 met, verified independently rather than from the report — constant → 1 `loaded` with `slug`, no `slot`; pair → 2 with `slot=light`/`slot=dark`; failed light slot → WARN `fallback applied` (failed slug + `slot=light` + reason) then INFO `loaded` naming `tokyo-night-day`; the different-slugs assertion is explicit (`events_test.go:618`); 5 resolutions → 1 WARN / 5 INFO; different reason → 2nd WARN; levels + `component=theme` asserted through a real `log.For` binding; `count`/`rejected` asserted absent by name; Discard/nil-logger/nil-seam all silent with a non-vacuity guard; per-instance dedup proven with two loggers writing to one sink.
- ARCHITECTURE sound: `reportSlot` (`resolution.go:218`) emits from the ASSEMBLED `SlotResolution` rather than from the producing branch, so `Loaded` can only reach `Resolved` — the "both events name the failed slug" state is structurally unreachable, not merely absent. A slot whose fallback itself fails never reaches the chokepoint and correctly emits nothing. `eventKey` carries the event discriminator, so `fallback applied` cannot collide with `rejected` on a shared slug. `append(themeAttrs(...), ...)` is allocation-safe (len==cap on both returned literals), so no aliasing hazard.
- CONVENTIONS followed: `package theme_test` black-box placement matches the file's existing Phase 1 tests; no `t.Parallel()`; `logtest.Sink` used through its typed accessors; `go test ./...` clean, `-race` clean on `internal/theme`, `go vet` clean, `gofmt` clean, `golangci-lint run ./internal/theme/...` → 0 issues. The two events use the shared `themeAttrs`/`slotAttr` helpers so key order and the constant's absent `slot` are single-sited, matching the `reportDirectoryUnusable` precedent. `wantExports` in `theme_test.go:224-228` updated for both new methods.
- Mutation results: 20 mutations injected via `go test -overlay` (no repo writes); 19 killed, including `Loaded` emitting `Requested`, the two events swapped in order, either level flipped, `FallbackApplied` naming the fallback, the `reason` attr dropped, `slot` values swapped, `slot` forced onto a constant, `Loaded` deduped, the dedup key losing `reason`, dedup state hoisted to package scope, both nil-seam guards removed, `log.OrDiscard` removed, and the success path left unreported. One survived — issue 1.
- `ResolveNomination` still has no production caller (only `cmd/theme.go`, `cmd/open.go` and capturetool construct loaders), so these events are exercised by tests alone until the construction path is wired (task 5-7). Expected at this point in the phase; noted so the absence of any cmd-side assertion isn't read as a gap.
- No `theme: rejected` fires for a failed nomination on the by-name construction read (`ResolveByName` doesn't emit it) — correct for this task, and the exact record counts asserted in the new tests would catch it if a later phase changed that silently.
- `resolution_test.go`'s revised `nominationLoader` comment is accurate: `NewLoader(nil)` yields a nil `*EventLogger`, and the nil-receiver early-returns are pinned by `TestEvents_DiscardSilencesResolution/a_nil_seam` (mutation-verified).
