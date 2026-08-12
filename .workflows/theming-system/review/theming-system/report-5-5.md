TASK: theming-system-5-5 — `theme: loaded` and `theme: fallback applied` on the injected logger

ACCEPTANCE CRITERIA:
- A constant nomination emits exactly one `loaded` record with a `slug` attr and no `slot` attr.
- A pair emits exactly two `loaded` records, one `slot=light` and one `slot=dark`, each with its own `slug`.
- A failed light slot emits one WARN `fallback applied` carrying the failed slug, `slot=light` and the reason, followed by one INFO `loaded` carrying `tokyo-night-day`.
- Both `loaded` and `fallback applied` name different slugs in that case — asserted explicitly.
- Five successive resolutions of the same broken slug emit one `fallback applied` and five `loaded` records.
- The same failed slug with a different reason emits a second `fallback applied`.
- `loaded` is INFO, `fallback applied` is WARN, both carry component `theme`.
- Every attr key is drawn from the closed seven; no `count`/`rejected` on either event.
- `NewEventLogger(log.Discard())` produces zero records for a full resolution including fallbacks; a nil logger does not panic.
- Two separately constructed `EventLogger`s each emit their own first `fallback applied` (per-instance dedup state).

STATUS: complete

SPEC CONTEXT:
§12.3's event catalogue (specification.md:1485, :1489) pins both events. `theme: loaded` is INFO, one line per nominated theme (one under a constant, two under a pair, `slot` only for the pair), no `count`, also fires at commit time for the newly-live opposite slot (§8.4), and **fires for the fallback too** carrying the fallback's slug — otherwise a `grep "theme:"` on a broken install cannot say which palette is rendering, which is the greppability the component is justified on (§6.3). `theme: fallback applied` is WARN carrying `slug` (the nomination that failed), `slot` where one applies, and `reason`, deduplicated per process on `slug`+`reason` because a persistently broken theme re-resolves at construction, on every panel open (§9.2) and on every `Esc` (§5.8). Emission is controlled by the injected logger (specification.md:1495) with the per-process dedup state living on that logger instance (specification.md:1497), and the component records where a theme is *used*, never where one is *diagnosed* (specification.md:1499).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `internal/theme/events.go:55-61` — `Loaded(slug string, slot Slot)`, INFO, message `loaded`, no dedup.
  - `internal/theme/events.go:77-86` — `FallbackApplied(slug string, slot Slot, r Reason)`, WARN, message `fallback applied`, deduped through the shared `firstSighting` on `{event, slug, reason}`.
  - `internal/theme/events.go:139-148` — `themeAttrs` emits `slug` always and `slot` only when `Slot.AttrName()` names one, so a constant carries no `slot` key at all (not empty, not `constant`).
  - `internal/theme/resolution.go:170-206` — emission is single-sited in `resolveSlot`'s reporting pass: `reportSlot` calls `reportFallback` (the failure) and then `Loaded(r.Resolved, r.Slot)` (the palette that actually loaded), so the ordering "failure first, then the fallback's slug" is structural rather than duplicated per call site.
  - `internal/theme/resolution.go:19-28` — `Slot.AttrName()` is the single `light`/`dark` vocabulary, reused by doctor (`cmd/doctor_theme.go:130`) and the persister (`cmd/theme_persister.go:53`).
  - Wiring: `cmd/open.go:27` holds the single `log.For("theme")` binding; `cmd/open.go:485-494` builds one loader per launch and `cmd/open.go:610,640` shares that one instance between construction (`themeResolution` → `ResolveNomination`) and the panel's `DirThemeSource`, so the dedup scope is genuinely per process. `theme.NewSilentLoader()` (`internal/theme/load.go:43-45`, `log.Discard()`) is what `portal doctor` (`cmd/doctor_theme.go:52`), `portal theme export` (`cmd/theme.go:54`) and `capturetool` (`cmd/capturetool/main.go:87`) construct.
- Notes:
  - Later phases refined the *cadence* seam rather than the events: `resolutionPass` (`resolution.go:89-110`) pairs each load route with its reporting rule so the panel's re-resolution (`ResolveNominationFrom`, used by `DirThemeSource.Resolve`) emits the fallback WARN only, while construction (`byNamePass`) and the Phase 9 commit load (`ResolveSlot`/`commitPass`) both emit `loaded`. That matches §12.3's cadence column exactly (construction + commit only) and is a correct superseding of this task's "emit from `ResolveNomination`" wording, not drift.
  - The task's "update the in-source event catalogue comment" bullet no longer applies: the Phase 11/comment-audit commits (`e30939b2`, `25626754`, `915e7fcb`) deliberately stripped phase/spec-citation comments from `internal/theme`. Absence of that catalogue comment is the intended end state, not an omission.
  - The by-name construction path emits no `theme: rejected` for a broken nomination (`internal/theme/resolve.go`), so a broken slot produces exactly `fallback applied` + `loaded` — consistent with §12.3 scoping `rejected` to enumerated files, and pinned by the exact-record-sequence assertion in `TestEvents_AttrKeysAreInTheClosedSet`.
  - `themeAttrs` returns a freshly allocated slice per call, so `FallbackApplied`'s `append(..., "reason", ...)` cannot alias or corrupt a caller's backing array.
  - Nil-safety holds on every method via the `e == nil` early return, and `NewEventLogger` funnels a nil logger through `log.OrDiscard`.

TESTS:
- Status: Adequate
- Coverage: All eleven planned tests exist in `internal/theme/events_test.go` under the planned names — `TestEvents_LoadedOncePerNomination` (:384, constant→1 / pair→2), `TestEvents_SlotAttrOnlyUnderAPair` (:424, asserts `HasAttr("slot")` is false under a constant and light-then-dark under a pair), `TestEvents_LoadedNamesTheFallbackSlug` (:469, `loaded` names `tokyo-night-day` for the broken light slot), `TestEvents_FallbackAppliedNamesTheFailedSlug` (:497, failed slug + `slot=light` + `reason=missing tokens`, and an explicit assertion that the two events name *different* slugs), `TestEvents_FallbackAppliedDedupsOnSlugAndReason` (:535, five resolutions → 1 WARN / 5 INFO), `TestEvents_FallbackDifferentReasonEmitsTwice` (:558, `not found` then `missing tokens` for the same slug → 2), `TestEvents_LoadedIsNotDeduplicated` (:587), `TestEvents_LevelsAreLoadedInfoFallbackWarn` (:609, levels + component + the WARN-before-INFO ordering), `TestEvents_AttrKeysAreInTheClosedSet` (:638, exact per-record key sequence across six records plus a closed-set membership check and an explicit "no `count`/`rejected`" check), `TestEvents_DiscardSilencesResolution` (:690, four silent-seam variants incl. `NewSilentLoader`, `log.Discard()`, a nil logger and a zero-value `Loader`), `TestEvents_FreshInstanceHasFreshDedupState` (:717, two instances × two resolutions → 2 WARNs against one shared sink).
- Notes:
  - Each acceptance criterion has a test that would fail if the behaviour broke: dropping `slot` under a pair, naming the failed slug on `loaded`, deduping `loaded`, or losing the dedup on `fallback applied` are each caught by a distinct assertion, and the failure messages state the reason rather than the mechanic.
  - Not over-tested. There is mild overlap (the "constant carries no slot" fact appears in both `SlotAttrOnlyUnderAPair` and the key-sequence table in `AttrKeysAreInTheClosedSet`), but each test pins a different plan-mandated criterion and the overlap costs no fixture setup.
  - Fixtures use the shared `internal/themetest` authoring helpers (`Lines`/`WithoutKey`/`Write`) rather than hand-rolled theme bytes, matching the Phase 11 single-sourcing; the "different reason" test drives `not found` → `missing tokens` by writing the file between resolutions, which exercises the real ladder rather than injecting a `Reason`.
  - Component/level assertions correctly use the `log.SetTestHandler` + `log.For("theme")` route (which carries the `component` attr) rather than the bare `logtest.NewCaptureLogger` used where only shape matters.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays a leaf that binds no component itself (the single `log.For("theme")` lives in `cmd`, guarded by `cmd/open_theme_nomination_test.go:112`); attrs stay inside the closed seven; no `t.Parallel()`; the new tests are unit-lane and hermetic (no tmux, no daemon, no built binary).
- SOLID principles: Good. `EventLogger` owns emission + dedup and nothing else; the `resolutionPass` value keeps "where a slug loads from" and "how the slot is reported" paired, which is what stops a future call site emitting `loaded` on the per-keypress path.
- Complexity: Low. Both new methods are a guard, an optional dedup check and one emit; the dedup check reuses the existing `firstSighting` critical section, so there is no second locking scheme.
- Modern idioms: Yes — `for open := range 5` range-over-int in tests, `slices.Equal`/`slices.Contains` for the key assertions, table subtests with named cases.
- Readability: Good. Attr assembly is single-sited in `themeAttrs`, so `loaded` and `fallback applied` cannot drift on the `slug`/`slot` half, and `resolution.go:192` states the ordering rule at the one place ordering is decided.
- Issues: None blocking. Two small notes below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/events.go:17-18 — the comment "Each constant is used twice: as the message and as the dedup key's discriminator." is false for two of the five constants it heads: `eventLoaded` (:60) and `eventEnumerated` (:71) are each used once, since neither event is deduplicated. Restore the original scoping: "Each deduplicated event's constant is used twice: as the message and as the dedup key's discriminator."
- [quickfix] internal/theme/events.go:146-148 — `slotAttr` is a one-line pass-through to `Slot.AttrName()` with a single caller (:140); every other consumer (`cmd/theme_persister.go:53`, `cmd/doctor_theme.go:130`) calls the method directly. Delete the wrapper and call `slot.AttrName()` at :140.
