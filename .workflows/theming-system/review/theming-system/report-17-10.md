TASK: theming-system-17-10 (tick-5354f7) — Collapse AdaptivePair's Tagged-Palette Machinery To A Named Constructor

ACCEPTANCE CRITERIA:
1. `MemberPalette` and `Member.Palette` no longer exist.
2. `AdaptivePair(light, dark)` is the single adaptive-nomination constructor and no second positional constructor sits beneath it.
3. `Nomination.Select(MemberLight)` and `Select(MemberDark)` return the palettes their arguments name for every existing test case.

STATUS: complete

SPEC CONTEXT:
This is an architecture-remediation task from analysis cycle 7 (`.workflows/theming-system/implementation/theming-system/analysis-architecture-c7.md:18-19`, `analysis-tasks-c7.md:270-284`), not a spec-governed behaviour. The specification does not govern the internal shape of the adaptive-nomination constructor — it governs the light/dark *resolution* semantics, which are unchanged here (`resolveNomination` still resolves SlotLight then SlotDark and reports both in `Resolution.Slots`).

The finding: `MemberPalette` / `Member.Palette` existed to make a light/dark transposition unrepresentable at the `AdaptivePair` call site, but (a) there is exactly one production call site, inside the same package as the type, and (b) the guard was one layer deep — `AdaptivePair` immediately delegated to a positional `pairFor(light, dark)`, so the inner boundary carried the same hazard unguarded. Three exported symbols plus a branch on a public-contract package for a hazard one line of doc can state.

Note on plan lineage: `MemberPalette`/`Member.Palette` were themselves introduced by earlier tasks in this same plan (11-7 / 12-12, commits `59c21223` / `c502058b`). This task is the plan's own deliberate supersession of that mechanism, not drift — the verifier-context amendment rule applies, and the prior reports (`report-11-7.md:26`, `report-12-12.md:24`) already record the supersession from the other side.

IMPLEMENTATION:
- Status: Implemented — all five "Do" items executed exactly.
- Location:
  - `internal/theme/nomination.go:32-36` — `func AdaptivePair(light, dark Theme) Nomination` returning the struct literal directly; `pairFor` deleted (not kept unexported — the "one of the two, not both" instruction is honoured).
  - `internal/theme/member.go` — file now ends at `Slot()` (line 24); `Member.Palette` and the `MemberPalette` struct are gone. `Member`, `Member.Opposite`, `Member.Slot` untouched, and `MemberDark` remains first at `member.go:8` with its zero-value comment intact.
  - `internal/theme/resolution.go:162` — `Nomination: AdaptivePair(light.Theme, dark.Theme)`, sitting directly beneath the `resolveSlot(SlotLight, …)` (`:153`) and `resolveSlot(SlotDark, …)` (`:157`) lines that name their slots, exactly as the task's rationale describes.
  - `internal/theme/theme_test.go:185-189` — exported-surface list drops `Member.Palette` and `MemberPalette`, retains `AdaptivePair` (`:137`).
  - `internal/tui/theme_panel_commit_slot_test.go:468-470` — `isThemeMemberValue` filter reduced to `HasPrefix(name, "Member") && name != "Member"`; the by-name `MemberPalette` exclusion is removed rather than left as dead residue.
- Notes:
  - AC1 verified repo-wide: `grep -rn "MemberPalette\|pairFor" --include="*.go"` returns nothing (exit 1). No stale references in docs either (the only `.md` hits are `.workflows` planning/analysis/review history, which is correct to keep).
  - AC2 verified structurally: `nominationAdaptive` is written at exactly one place (`nomination.go:35`), inside `AdaptivePair`. There is no second constructor above or beneath it.
  - The collapse is genuinely safe, and I checked the premise rather than taking it on trust: `AdaptivePair` has exactly one production call site in the whole repo (`resolution.go:162`), in-package. Every out-of-package production path takes its nomination from the loader's `Resolution` value, never by construction — `internal/tui/build.go:46`, `internal/tui/theme_panel_commit.go:126` (`m.themeState.nomination = resolution.Nomination`), `cmd/open.go:507,638`. The TUI's constant→adaptive conversion path in particular does not hand-build a pair, so the removed tag protected no external boundary. Widening `AdaptivePair`'s exported signature to positional therefore adds no reachable inversion site today.
  - The retained `Member` methods do still carry independent production load, as the task asserted: `Member.Slot` at `cmd/theme_persister.go:53` and `internal/tui/theme_panel.go:208`, `Member.Opposite` at `internal/tui/theme_panel_confirm.go:75-76`. Nothing was left as a dead export by the deletion.
  - Comment rewrite is accurate and minimal: "returns the nomination for an adaptive theme setting from its two palettes, light first" (`nomination.go:32-33`). It states the load-bearing fact (argument order) without reciting the call site or naming the tests — which also keeps it clear of the project's rule against comments pointing at process artifacts. The type-level `Nomination` doc (`nomination.go:5`) still says "Construct only via ConstantNomination or AdaptivePair" and remains true after the deletion.

TESTS:
- Status: Adequate, with one redundant test (detail below).
- Coverage:
  - The inversion coverage — now the *whole* protection, since the type system no longer carries it — was not just retargeted but strengthened. `TestAdaptivePair_ArgumentOrderIsLightThenDark` (`internal/theme/nomination_test.go:49-68`) asserts both that `inOrder != swapped` (order is observable at all) and that the swapped construction answers each member with the *other* palette (`:62-67`). The pre-collapse test only proved the two tagged spellings agreed; the new one pins the positional contract in the direction that can actually regress. This is the correct guard for the mechanism that replaced the type-level one.
  - Task-requested zero-`Nomination` test exists and predates the change, unmodified: `TestNomination_ZeroValueIsNeitherState` (`:110-124`) asserts `IsConstant() == false` and `Select` returning the zero `Theme` for both members. `TestNomination_ZeroValueIsDistinguishableFromBothStates` (`:138-148`) additionally keeps the zero-sentinel distinguishable from `AdaptivePair(Theme{}, Theme{})`, retargeted at `:144`.
  - Every listed test user was updated and every assertion preserved: `internal/theme/nomination_test.go` (`:50,51,71,107,144`), `internal/tui/nomination_test.go:19`, `internal/tui/theme_testing_test.go:218`. Diffing `1ae04028` shows no assertion was dropped in the retarget — only the tag-specific ones that became unexpressible.
  - The two structural guards that would otherwise silently rot were both updated in the same commit (exported-surface list, `Member`-prefix AST filter), so the smaller surface is pinned rather than incidental.
- Notes:
  - One over-test: `TestAdaptivePair_FillsBothMembers` (`:70-78`) is now strictly weaker than `TestAdaptivePair_HoldsBothWithNoActiveMember` (`:32-47`) on an identical input — both build `AdaptivePair(nominationLight, nominationDark)`, but the latter already asserts each member equals its own non-zero palette, so the former's "not the zero Theme" check cannot fail first. It was meaningful under the tagged constructor (where naming one member twice could leave the other zero); with the positional shape that state is unreachable. Non-blocking; noted below. (Independently reached, and it matches the same call in `report-11-7.md:63`.)
  - Deleting it would not orphan `bothMembers` (`:14`), which is still used at `:25` and `:119`.

CODE QUALITY:
- Project conventions: Followed. `internal/theme` stays log-free at the constructor (the `theme` component is emitted only through the injected `EventLogger`, per CLAUDE.md). No raw hex, no `t.Parallel()`, tests are in the `theme_test` external package so the exported-surface guard tests the real contract. The change is unit-lane only — no daemon, no tmux, no built binary — so no isolation or build-tag obligations attach.
- SOLID principles: Good. This is an interface-segregation improvement in the small: the contract package sheds two exported types and a method that served one in-package caller, and `Nomination` retains a single reason to change.
- Complexity: Low, and lower than before — the `named.member == MemberLight` branch is gone, leaving a straight-line struct literal.
- Modern idioms: Yes. A named positional constructor with a doc comment stating the order is the idiomatic Go answer here; the tagged-argument pattern was the un-Go-like one.
- Readability: Good. `AdaptivePair(light.Theme, dark.Theme)` beneath two `resolveSlot(SlotLight…)` / `resolveSlot(SlotDark…)` lines reads better than the asymmetric `AdaptivePair(MemberLight.Palette(light.Theme), dark.Theme)` it replaces, which named one half and left the other implicit.
- Issues: None. One judgement call worth recording rather than flagging: this trades a compile-time guard for a doc-plus-test guard. That is the right trade *given* the single in-package call site verified above, and the strengthened inversion test is a real replacement rather than a nominal one. Should a future task ever construct an adaptive pair from outside `internal/theme`, that call deserves a fresh look — but nothing in the current tree does.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/nomination_test.go:70-78 — delete `TestAdaptivePair_FillsBothMembers`. It builds the same pair as `TestAdaptivePair_HoldsBothWithNoActiveMember` (:32-47) and asserts only that neither member is the zero `Theme`, which that test already implies by asserting each member equals its own non-zero palette; it is residue of the tagged constructor, where naming one member twice could leave the other unfilled. `bothMembers` (:14) stays in use at :25 and :119, so nothing is orphaned.
