TASK: theming-system-13-7 — Carry The Missing-Token List As Structured Data On Rejection Instead Of Parsing Rendered Copy (tick-702a97, phase 13 / analysis cycle 3)

ACCEPTANCE CRITERIA:
- No code derives a log attr or any structured value by parsing `Rejection.Detail`.
- `Rejection.Tokens` is populated for `missing tokens` and `bad colour` and empty for every other reason.
- The emitted `token` attr value is byte-identical to today for both reasons.
- The rendered `Detail` for both reasons is byte-identical to today.
- Editing the `missing ` lead-in changes only rendered copy, never the attr.
- `go test ./internal/theme ./cmd` passes.

STATUS: complete

SPEC CONTEXT:
Specification §12.3 ("A new `theme` log component") defines `theme: rejected` as WARN, one per rejected file, deduped per process, and states it "Carries `token` where the reason names one (`missing tokens`, `bad colour`) — this is the `token` attr's only consumer", with the closed attr-key set `slug, slot, reason, path, token, count, rejected`. The spec fixes *which* reasons carry the attr, not the attr's rendered form, so the task's byte-identical requirement is the correct conformance bar — the change is a pure internal re-plumbing with no spec-visible surface. §12.2 pins doctor's advisory lines (which render `Rejection.Detail` verbatim), so Detail's rendered form is separately load-bearing and must not move.

IMPLEMENTATION:
- Status: Implemented (as amended by a later plan task — see Notes)
- Location:
  - `internal/theme/reason.go:22-39` — `Rejection` gains `Tokens []string` and `Values []string`, documented alongside `Line`/`BadNameCause`/`Err` as the structured sources behind `Detail`, each populated only for the reason it belongs to.
  - `internal/theme/validate.go:11-17` — `detailMissingTokensLeadIn` / `detailMissingTokens` / `detailBadColourPair` reduced to rendering constants; the comment now reads "User-facing copy only: a consumer needing the tokens themselves reads Rejection.Tokens" (the old read-back justification is gone).
  - `internal/theme/validate.go:41-67` — `applyPairs` collects `names`/`values` and populates both fields; `Detail` renders through the shared `renderedPairs` helper, unchanged in form.
  - `internal/theme/validate.go:73-89` — `requireEveryToken` populates `Tokens: missing` and renders `Detail` from the same slice; no value list (an absent token has none).
  - `internal/theme/validate.go:95-105` — `renderedPairs`, the single composer of the `"<name> = <value>"` form, shared by `Detail` and the attr.
  - `internal/theme/events.go:150-163` — `tokenAttr` rewritten: `strings.Join(r.Tokens, ", ")` for `missing tokens`, `renderedPairs(r.Tokens, r.Values)` joined for `bad colour`. No `TrimPrefix`, no reference to `detailMissingTokensLeadIn`. Comment updated — the prohibition on parsing rendered copy stays, the self-exception is gone.
- Notes:
  - **Deliberate supersession, not drift.** The task's Do step 3 asked for `Tokens` to hold rendered `key = value` pairs for `bad colour`. The shipped shape instead holds bare names in `Tokens` for *both* reasons with an index-aligned `Values`. That is phase 17's `tick-cd055c` "Keep Rejection.Tokens One Shape Across Both Reasons" amending this task, and it is strictly better: `Tokens` means the same thing under both reasons, and the rendered pair form stays in one place.
  - Attr byte-identity holds: `missing tokens` previously `TrimPrefix(Detail, "missing ")` == the comma-joined names, now the joined `Tokens` directly; `bad colour` previously `Detail` verbatim, now `renderedPairs` — the exact function that composes that `Detail`.
  - `Detail` byte-identity holds: both renderers still use the same format constants and the same joins, and the doctor frames are pinned byte-exactly (below).
  - Emptiness for the other reasons is structural, not conventional: `badName` (`name.go:94-96`), the reserved arm and `unreadable` (`load.go:80, 125-127`), `badSyntax` (`lex.go:106-112`) and `unresolvedRejection`/`notFound` (`union.go:205-210`) construct `Rejection` with no token fields at all.
  - No remaining consumer parses `Detail`: `cmd/doctor_theme.go:184,218-223` renders it verbatim (with the documented `Err` fallback for `unreadable`), `cmd/theme.go:38-47` interpolates it into the export refusal, `internal/tui/theme_row.go:132` renders `Reason` only, and `internal/theme/{enumerate,union,resolution}.go` pass the pointer through untouched.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/theme/validate_test.go:137-152` — missing-tokens rejection carries the absent names as data, carries no `Values`, and `Detail == "missing " + Join(Tokens, ", ")`, i.e. the detail is the structured list rendered.
  - `internal/theme/validate_test.go:154-166` — bad-colour rejection carries names and offending values, index-aligned and echoed back as written.
  - `internal/theme/events_test.go:48-95` — the `token` attr per reason: exact value for both token-bearing reasons, attr *absent* for the other five.
  - `internal/theme/events_test.go:97-136` — the direct proof of the task's central property: a rejection whose `Detail` has been reworded away from the shipped copy ("absent: …", "canvas (blue) and …") still emits the correct attr. This is a stronger form of the plan's "changing the lead-in constant does not change the attr" test — it holds for *any* copy edit, to either reason's wording, not just the lead-in.
  - `internal/theme/load_test.go:275-301` — `Tokens` carried only by the two reasons that name tokens, driven over the whole `rejectionCorpus()` (valid file, bad name, reserved name, absent/unreadable/dangling, duplicate key, bad colour, missing token, empty file). Broader than the plan's enumerated four reasons and self-extending as the corpus grows.
  - `cmd/doctor_theme_test.go:69-108` — doctor's rendered detail pinned byte-exactly for both reasons (`⚠ theme mine: missing tokens — missing text.primary, bg.subtle` and `⚠ theme nord-lee: bad colour — text.primary = #GGGGGG, canvas = blue`), which is the `Detail`-unchanged criterion.
- Notes:
  - Not over-tested: the copy/structure split is asserted once per property, and the "which reasons carry tokens" claim is table-driven off an existing corpus rather than restated per reason.
  - One genuinely untested branch: `renderedPairs`' unpaired-lists degradation (`validate.go:98-100`) is unreachable from any producer and never exercised by a test — see the non-blocking note.
  - Test execution not attempted (read-only verification); the acceptance criterion `go test ./internal/theme ./cmd` is judged by reading — the package compiles coherently (both `strings` and `fmt` remain used in their files, `renderedPairs` is package-internal to both call sites) and no existing assertion in the read suites is contradicted by the new shape.

CODE QUALITY:
- Project conventions: Followed. The `theme` component's attr vocabulary is unchanged (no call-site invention), `internal/theme` stays a leaf, the comment style (why-not-what, no spec-section or task citations) matches the topic's standard, and the new fields are documented on the struct rather than at the call sites.
- SOLID principles: Good. The producer now owns the structured datum and the renderer owns the copy; `renderedPairs` gives the pair form one home, so the log attr and the user-facing detail cannot drift apart.
- Complexity: Low. `tokenAttr` is a three-arm switch with no string surgery; the producers each gained one field assignment.
- Modern idioms: Yes — `strings.Join`, `slices` in the tests, no reflection or ad-hoc parsing.
- Readability: Good. `Rejection`'s doc comment now names all five structured narrowings in one sentence and states the `Tokens`/`Values` alignment rule explicitly, so a reader hitting `Values` on a `missing tokens` rejection knows it is empty by design.
- Issues: None material. Two small documentation/test-fidelity points below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/theme/validate.go:11-12 — the const block's comment claims "User-facing copy only", but `detailBadColourPair` is also the log attr's format: `renderedPairs` (validate.go:95-105) is called from `tokenAttr` (events.go:159), so editing that constant changes the `token` attr as well as the rendered detail — the residual of exactly the coupling this task removed for the lead-in. Replace the comment with: `// Rendering constants. detailMissingTokensLeadIn and detailMissingTokens are\n// user-facing copy alone; detailBadColourPair also composes the token attr\n// through renderedPairs, so editing it changes both. A consumer needing the\n// tokens themselves reads Rejection.Tokens.`
- [do-now] internal/theme/events_test.go:142 — the bad-colour fixture in `TestEventLogger_AttrKeysAreInTheClosedSet` carries `Detail: "canvas = blue"` with no `Tokens`/`Values`, a rejection no producer can create post-change; the record it asserts on now emits `token=""`. Add `Tokens: []string{"canvas"}, Values: []string{"blue"}` to the literal so the key-set assertion runs over a producible rejection (the asserted key set is unchanged).
- [quickfix] internal/theme/validate.go:98-100 — `renderedPairs`' "a name with no matching value renders bare" branch is unreachable from every producer and exercised by no test, yet it exists precisely for hand-constructed rejections reaching `tokenAttr`. Add a case to `TestEventLogger_TokenAttrRendersFromTokensNotDetail` (events_test.go:97-136) with `Reason: ReasonBadColour`, `Tokens: []string{"canvas", "text.primary"}` and `Values: []string{"blue"}`, asserting `token == "canvas = blue, text.primary"` — or delete the branch and let the lists be required to pair.
