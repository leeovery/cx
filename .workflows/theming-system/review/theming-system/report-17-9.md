TASK: theming-system-17-9 — Keep Rejection.Tokens One Shape Across Both Reasons (tick-cd055c)

ACCEPTANCE CRITERIA:
- `Rejection.Tokens` holds token names for every reason that populates it.
- `Rejection.Detail` and the `theme: rejected` log line are byte-identical to before for both reasons, including multi-offender files.
- No caller re-splits a `Tokens` element.
- `internal/theme/theme_test.go`'s exported-surface list updated for the new field.

STATUS: complete

SPEC CONTEXT: The specification governs the `theme` log component's attr *keys* (`slug`/`slot`/`reason`/`path`/`token`/`count`/`rejected`) as a closed vocabulary (CLAUDE.md, "Logging & observability"), and §6.2 makes the named missing/offending token the thing that makes a bad theme file findable. It does not prescribe the internal shape of `Rejection`'s structured fields, so this task is an architecture-hygiene change: it must not alter the emitted line or the user-facing `Detail`, only the shape of the structured data behind them. The task's step 5 (keep the offending hex in the attr, since the log is the only place a rejected file's declared value is recorded) is the constraint that keeps this inside the spec-governed vocabulary rather than amending it.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `internal/theme/reason.go:27-39` — `Values []string` added beside `Tokens`, with the index-alignment and "populated only for ReasonBadColour" pairing documented on the fields; the struct-level doc comment updated to list `Values` among the structured sources behind `Detail`.
  - `internal/theme/validate.go:41-67` — `applyPairs` collects `(name, value)` into two parallel slices, sets `Tokens` to the names and `Values` to the values, and composes `Detail` via `renderedPairs`.
  - `internal/theme/validate.go:95-105` — new unexported `renderedPairs(names, values)` helper, the single owner of the `"%s = %s"` composition, used by both edges.
  - `internal/theme/validate.go:73-89` — `requireEveryToken` unchanged apart from the doc addition ("The absent names are the whole story, so the rejection carries no values"), as directed.
  - `internal/theme/events.go:150-163` — `tokenAttr` splits the previously-shared case: `ReasonMissingTokens` joins names, `ReasonBadColour` renders through `renderedPairs`. Comment updated to state it renders from the structured pair, still never parses `Detail`, and records the deliberate retention of the offending value (step 5's rejection).
- Notes:
  - **Detail byte-identity holds.** Before (`git show 9fd9cdc9`): `Detail: strings.Join(offenders, ", ")` where each offender was `fmt.Sprintf(detailBadColourPair, pair.Key, pair.Value)`. After: the same format constant, the same `", "` join, the same append order in the same loop — the only change is *when* the pair is rendered. Multi-offender ordering is preserved because the collection loop is untouched.
  - **Log-line byte-identity holds.** For `ReasonBadColour`, `strings.Join(r.Tokens, ", ")` over pre-rendered pairs is replaced by `strings.Join(renderedPairs(r.Tokens, r.Values), ", ")` over the same pairs — identical output for every rejection `validate.go` produces (the only production producer; `grep ReasonBadColour` outside tests finds only `internal/theme/validate.go:62` plus display-only sites at `cmd/doctor_theme.go:182` and `internal/capture/fixtures.go:540/547`, neither of which reads `Tokens`).
  - **No caller re-splits.** `Rejection.Tokens` is read in exactly two production places, both `internal/theme/events.go:157/159`, and neither splits an element; the rest of the hits are assertions inside `internal/theme`'s own tests. No consumer outside the package touches the field.
  - **Acceptance criterion 4 is vacuous, not skipped.** `theme_test.go`'s `wantExports` guard (`exportedDecls`/`exportedSpecs`, `internal/theme/theme_test.go:276-313`) enumerates top-level declarations only — types, funcs, methods, consts/vars. Struct fields are not in its domain (`Rejection` is listed; `Rejection.Tokens`, `Rejection.Detail` and the rest never were), so adding `Values` produces no new symbol and no update was possible or needed. The one field-level pin in that file (`theme_test.go:250-257`) is scoped to `Token`, deliberately. The list is still correct as written — `renderedPairs` is unexported, so it adds nothing either.

TESTS:
- Status: Adequate
- Coverage:
  - Two bad colours → names + values + unchanged Detail: `internal/theme/validate_test.go:154-166` (`TestValidate_BadColourCarriesTheOffendingNamesAndValuesAsData`). The fixture value was changed to `#gGgGgG` in this commit specifically so the test pins that `Values` echoes the user's casing rather than the upper-cased canonical form — exactly the "not canonicalised/upper-cased" requirement, and a good catch since the *valid* path does upper-case (`validate.go:55`).
  - Missing tokens → names in `Tokens`, empty `Values`: `internal/theme/validate_test.go:137-152`.
  - Multi-offender Detail (three offenders, shuffled + unknown-key noise): `internal/theme/validate_test.go:111-127` — pre-existing and unmodified, which is itself the byte-identity evidence for `Detail`.
  - `theme: rejected` `token` attr pinned for both reasons: `internal/theme/events_test.go:48-95` (table now carries `values`), plus `internal/theme/events_test.go:97-136`, which pins the attr against a deliberately *reworded* `Detail` and so proves the attr is rendered from the structured fields rather than parsed out of copy.
  - Loader-level invariant that only token-naming reasons carry `Tokens`: `internal/theme/load_test.go:275-301`, still valid unchanged.
- Notes: Proportionate — no redundant restatement of the same assertion across files, and the three task-mandated tests each land at the right layer (validate-level for the data shape, events-level for the emitted attr). Not over-tested. Two small gaps are noted below; neither undermines the acceptance criteria.

CODE QUALITY:
- Project conventions: Followed. The change stays inside the `theme` package's leaf constraints, invents no attr key, and honours the "rendered copy is not re-parsed downstream" rule the surrounding code is built on — it strengthens it, since the last remaining place where structured data was *pre-rendered* is now gone.
- SOLID principles: Good. `renderedPairs` gives the `"<name> = <value>"` composition a single owner shared by the two edges that need it, so `Detail` and the log attr cannot drift apart; the format constant `detailBadColourPair` remains the sole literal.
- Complexity: Low. One extra slice and one small pure helper; no new branching in the hot path.
- Modern idioms: Yes. Plain index-aligned slices with the pairing documented on the fields is the right call at this size — a `[]struct{Name, Value string}` would have been the alternative, but it would have changed `Tokens`' type and broken the "Tokens holds names" criterion.
- Readability: Good. The field doc, the `applyPairs` doc ("the offending value is deliberately not canonicalised, echoing back what the user wrote") and the `tokenAttr` doc all state the non-obvious decisions rather than restating code, and the step-5 rejection is recorded where a future reader would otherwise be tempted to "clean up" the attr to names-only.
- Comment accuracy: Verified. `validate.go:11-12` ("a consumer needing the tokens themselves reads `Rejection.Tokens`") is now *more* true than before the change. `events.go:150-153` accurately describes the new rendering path. `reason.go:32-35` correctly describes both the alignment and the empty-for-missing-tokens case. No comment references a task id, phase or spec section.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/validate.go:97-100 — the `i >= len(values)` degradation branch is unreachable from any production path (`applyPairs` always appends to both slices in lockstep) and no test exercises it, so the no-panic contract its comment claims is unverified. Add a case to `TestEventLogger_TokenAttrRendersFromTokensNotDetail` (`internal/theme/events_test.go:97`) with `Reason: theme.ReasonBadColour`, `Tokens: []string{"canvas", "text.primary"}`, `Values: []string{"blue"}` and `wantToken: "canvas = blue, text.primary"` — that pins the documented degradation from the external test package, where a hand-constructed `Rejection` is exactly the scenario the guard exists for.
- [quickfix] internal/theme/load_test.go:275-301 — `TestLoadFile_TokensCarriedOnlyByTheReasonsThatNameTokens` walks the whole rejection corpus pinning where `Tokens` may be populated, but has no counterpart for the new field, so nothing stops a future reason from populating `Values`. Add an arm to the same loop asserting `len(rejection.Values) != 0` only for `theme.ReasonBadColour` and `== 0` for every other reason in the corpus, mirroring the existing `Tokens` assertions.
