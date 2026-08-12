TASK: theming-system-13-4 — Take The Typed RawKeys Value In ResolveSetting Instead Of Three Positional Strings (tick-457f75, medium, architecture)

ACCEPTANCE CRITERIA:
- No call site passes three positional strings; a transposed light/dark is not expressible at any `ResolveSetting` call.
- Control-stripping is performed in exactly one place.
- The field-by-field `theme.RawKeys{...}` rebuilds at the prefs boundary are gone; both `cmd` sites go through the constructor.
- Resolution behaviour is unchanged: the `theme`-wins tiebreak, per-slot shipped defaults, and empty-after-strip counting as unset all hold.
- `internal/theme` gains no imports and stays a leaf.
- `go build ./... && go test ./...` pass; `golangci-lint run` is clean.

STATUS: complete

SPEC CONTEXT:
Spec §9.5 (specification.md:1094) is the governing rule: "A slug that came from `prefs.json` is control-stripped at the point it is read, not at the point it is drawn — it is a property of the value, so every consumer inherits it." It also fixes the two consuming surfaces (panel row, doctor advisory line at §14A), keeps truncation panel-local, and (line 1096) keeps the separate CLI-argument strip at `portal theme export` (`cmd/theme.go:22`) outside this path. Spec §11 (line 818) requires the panel constructor to receive the raw persisted keys "control-stripped per §9.5". The implementation's placement of the strip in the `RawKeys` constructor is a faithful reading of "a property of the value" — it moves the strip from the collapse function to the point of construction at the prefs boundary.

IMPLEMENTATION:
- Status: Implemented (commit 0ae29e82), and intact after the later phase-15/16/17 refactors and the phase-11/12 comment audits.
- Location:
  - internal/theme/setting.go:19-21 — `NewRawKeys(theme, light, dark string) RawKeys` delegating to `stripped()`.
  - internal/theme/setting.go:23-29 — `(RawKeys).stripped()`, the single control-strip implementation.
  - internal/theme/setting.go:67-80 — `ResolveSetting(keys RawKeys) (Setting, RawKeys)`; tiebreak, per-slot defaults and the returned stripped raw all preserved verbatim.
  - internal/theme/setting.go:102 (`SlugForSlot`) and :127 (`InForceKeys`) — both pass their `keys` value straight through; the destructure at the old `InForceKeys` line is gone.
  - cmd/open.go:501 — `theme.ResolveSetting(theme.NewRawKeys(keys.Theme, keys.Light, keys.Dark))` at the prefs boundary.
  - cmd/doctor_theme.go:97 — `theme.NewRawKeys(keys.Theme, keys.Light, keys.Dark)`; the field-by-field `theme.RawKeys{Theme: …, Light: …, Dark: …}` literal is gone.
  - internal/tui/theme_panel.go:201 — `theme.ResolveSetting(m.themeState.keys)` passes the whole value.
- Notes:
  - Zero remaining three-arg `ResolveSetting` calls anywhere (prod, test, capture fixtures) — verified by repo-wide grep; every hit is single-value.
  - `internal/theme/setting.go` still imports only `cmp`; the package gained no import from this task, and the purity deny-list guard (`impureSettingImports`, setting_test.go:583) still covers it.
  - `ResolveSetting` deliberately re-runs `stripped()` (setting.go:68-70) rather than trusting its input. That is correct rather than redundant: `RawKeys` has exported fields and is legitimately built as a literal in `internal/capture/fixtures.go` and across the tui/cmd tests, so the collapse cannot assume construction went through `NewRawKeys`. The strip *rule* still exists in exactly one function; only its invocation is belt-and-braces, and it is idempotent so behaviour is unaffected. The inline comment states exactly this.
  - Consequence worth knowing (not a defect): because `ResolveSetting` re-strips and returns the stripped form, both `cmd` boundary sites would still yield stripped values even without `NewRawKeys`. The constructor is the contract-carrying entry point (and what §9.5 asks for), not a behavioural load-bearer today.
  - The residual transposition hazard is reduced, not zero: `NewRawKeys(theme, light, dark)` is itself positional, so light/dark can be swapped at its two call sites. That is inherent to the task's own constraint #8 — `internal/theme` must not import `internal/prefs`, so the constructor cannot take `prefs.ThemeKeys`. Four transposable sites became two, and the acceptance criterion (no transposition expressible at a `ResolveSetting` call) is met exactly.

TESTS:
- Status: Adequate.
- Coverage:
  - Signature regression guard: `TestResolveSetting_IsPureAndDeterministic` / "it deals in slugs only" (internal/theme/setting_test.go:290-295) reflects over `ResolveSetting`'s inputs and fails if it ever takes anything but a single `theme.RawKeys` — this is what structurally prevents a relapse to positional strings, and it is the right assertion for an architecture task.
  - Constructor strip: `TestNewRawKeys_StripsControlFromEveryKey` (:534) covers ANSI/tab/newline across all three fields.
  - Control-only ⇒ unset (the task's named edge case): `TestNewRawKeys_ControlOnlyValueStripsToEmpty` (:542) asserts the constructor yields the zero value AND that the resolution treats it as unset (no constant, both shipped defaults, empty raw) — i.e. it pins "unset, not an illegal slug" end to end as the task asked, so the paired assertions are load-bearing rather than duplicated.
  - Idempotence: `TestNewRawKeys_IsIdempotent` (:562).
  - Stripped return value: `TestResolveSetting_StripsKeysItIsHandedUnstripped` (:570) plus the pre-existing `TestResolveSetting_ReturnsRawKeysForTheSameEvaluation` (:220) and `TestInForceKeys_AcceptsAlreadyResolvedKeys` (:434).
  - Behaviour-unchanged criterion: the pre-existing suite (`ConstantWins`, `ConstantIgnoresSlots`, `UnsetSlotsTakeShippedDefaults`, `DefaultsAreTheSharedConstants`, `ControlStripsAllThree`, `ControlOnlyValueIsUnset`, `NoTrimOrLowercase`, the `InForceKeys` table) was adapted to the new signature with assertions unchanged — the correct shape for a refactor task: the diff is call-shape only, so a behaviour change would have surfaced.
  - Prefs-boundary round trip: `cmd/doctor_persisted_theme_test.go:360` (an escape-laden persisted value renders stripped, asserted via `strings.ContainsAny(got.line, "\x1b\n\t")`) and :381 (a control-only persisted value produces no advisory) still exercise the doctor site through the new constructor unchanged in outcome — exactly the task's last test bullet.
- Notes:
  - No over-testing found: no redundant table rows, no new mocks, no assertions on internals (`stripped` is unexported and is only reached through the two exported entry points).
  - Would the tests fail if the feature broke? Yes on both axes — reverting the signature fails the reflect guard, and dropping the strip fails four separate tests plus the doctor round trip.
  - The `cmd/open` side has no control-char test of its own, but the task did not ask for one and the doctor test covers the same constructor; the behaviour is additionally guaranteed by `ResolveSetting`'s re-strip.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays free of `internal/prefs` (constraint #8 honoured — the `prefs.ThemeKeys` → `theme.RawKeys` mapping lives in `cmd`, which owns the prefs boundary per CLAUDE.md). No new imports; setting.go remains stdlib-`cmp`-only, consistent with CLAUDE.md's description of the package.
- SOLID principles: Good. Single, named collapse point; the strip is one method with one reason to change; the value type carries its own normalisation rather than each consumer re-deriving it.
- Complexity: Low. `NewRawKeys` and `stripped` are single-expression; `ResolveSetting` lost a four-line literal and gained a one-line call.
- Modern idioms: Yes. Value receivers returning new values (consistent with the existing `WithConstant`/`WithMember` pair, which have their own no-mutation test at :520), `cmp.Or` retained for the defaults, and the reflect guard uses the modern `signature.Ins()`/`Outs()` iterators with `reflect.TypeFor`.
- Readability: Good. The three exported doc comments (`RawKeys`, `NewRawKeys`, `ResolveSetting`) each state one rule with no duplication, which is Do-step 3 satisfied — the stripping paragraph moved out of `RawKeys`/`ResolveSetting` and into `NewRawKeys`.
- Comment accuracy: Verified against the code. `NewRawKeys`'s doc ("control-stripped … a value that is only control characters strips to empty and so counts as unset … nothing else is normalised") is true of `stripped()` + `StripControl` and is pinned by tests. The inline "Re-stripped, since a caller may have built its keys as a plain literal rather than through NewRawKeys" is true and explains a non-obvious choice rather than restating the line. No process-artifact references (task ids, phases, spec sections) in any of the changed comments.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/open.go:501 and cmd/doctor_theme.go:97 — the `prefs.ThemeKeys` → `theme.RawKeys` field destructure is still written twice, so the task's stated Outcome ("the prefs→theme key mapping is written once") is only partly delivered and two lines can still transpose light/dark. Add a single `cmd`-local helper beside the other prefs-boundary code, e.g. `func themeRawKeys(k prefs.ThemeKeys) theme.RawKeys { return theme.NewRawKeys(k.Theme, k.Light, k.Dark) }`, and call it from both sites — leaving exactly one transposable line in the codebase.

VERIFICATION LIMITS:
- Per verifier rules no commands were executed beyond the output-file rename, so the "`go build ./... && go test ./...` pass; `golangci-lint run` clean" criterion was judged by reading: every `ResolveSetting`/`NewRawKeys`/`RawKeys` reference across cmd, internal/theme, internal/tui and internal/capture (prod and test) was enumerated by grep and is consistent with the new signature, and the exported-API guard list in internal/theme/theme_test.go carries `NewRawKeys`, so no stale call site or unlisted export remains.
