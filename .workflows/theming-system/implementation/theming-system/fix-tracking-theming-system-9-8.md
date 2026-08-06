## Attempt 1

ISSUES:

- `/Users/leeovery/Code/portal/internal/tui/notice_band.go:530-535` — the origin discriminator inside `themeFlashClaim` is not asserted by any test. Deleting the guard clause (making the body just `return m.flashClaim()`) leaves the **entire repository suite green** — the reviewer ran `go test -count=1 ./...` under that mutation and every package passed. That mutation silently grants the theme tier to every flash in the model (the burst partial-failure report, the externally-killed-session bail, the bootstrap-warnings flash), which is exactly the "Scope it to the six" invariant this task exists to install and the reason the `flashOrigin` field was added at all. `TestThemeFlash_OriginDiscriminator` asserts the *field*, never the *claim*; the precedence tests and `TestThemeFlash_NonThemeFlashUnchanged` cannot distinguish the tiers because both resolve identically with no contender between them. As it stands, a later reader can delete the discrimination as apparently-dead code with full test-suite cover.

  FIX: Add a direct subtest asserting the tier predicate discriminates — three states in one body. Verified in an isolated copy that this fails under the mutation and passes against the current code:

      t.Run("the theme tier claims only a theme-origin flash", func(t *testing.T) {
          m := noticeBandModel("alpha-row")
          if _, _, ok := m.themeFlashClaim(); ok {
              t.Error("the theme tier claimed the slot with no flash live")
          }
          m.setFlash("__ORDINARY__")
          if _, _, ok := m.themeFlashClaim(); ok {
              t.Error("the theme tier claimed an ordinary flash; the tier is scoped to the theme signals")
          }
          (&m).setThemeFlash(themePanelNoColorFlash)
          role, message, ok := m.themeFlashClaim()
          if !ok || role != bandWarning || message != themePanelNoColorFlash {
              t.Errorf("the theme tier = (role %v, message %q, ok %v), want the theme flash claimed", role, message, ok)
          }
      })

  Place it in `TestThemeFlash_OriginDiscriminator` (`/Users/leeovery/Code/portal/internal/tui/theme_flash_precedence_test.go:35`), whose stated subject is the discriminator, or as a subtest of `TestThemeFlash_NonThemeFlashUnchanged` if the scoping framing is preferred.

  ALTERNATIVE: assert it through the arbiters instead — with a non-theme flash live, that `activeNoticeBand` / `activeProjectNoticeBand` resolve through the ordinary tier. That is more behavioural but not expressible today without a page-visible difference between the tiers, so it would need a returned-tier discriminator on the arbiters (new API for a test). The direct predicate assertion is the smaller change and pins the same invariant; reviewer recommends it.

  CONFIDENCE: high

COMMENT_CORRECTIONS:

- `/Users/leeovery/Code/portal/internal/tui/sessions_flash.go:54-57` — the claim that the existing setters keep today's order "without naming an origin" is falsified by the same diff: `setFlash` (model.go:2447) and `setSuccessFlash` (model.go:2477) both name it explicitly. The trailing "only setThemeFlash stamps" is also a cardinality claim.

  OLD:
  // flashOriginDefault is the zero value, so every existing setter and the
  // construction-time seed keep today's order without naming an origin; only
  // setThemeFlash stamps flashOriginTheme. Like flashKind it is irrelevant once
  // flashText is empty.

  NEW:
  // flashOriginDefault is the zero value, so the construction-time seed keeps
  // today's order without naming an origin, and setFlash / setSuccessFlash reset to
  // it explicitly. Like flashKind it is irrelevant once flashText is empty.

NOTES:

- VERDICT context: SPEC_CONFORMANCE conformant. §14A's "the theme flashes take precedence over [the filter line] … scoped to these flashes" is implemented as an origin carried on the flash plus explicit ordering in both arbiters, matching the task's conservative resolution of the spec/implementation ambiguity. The reviewer independently confirmed the premise: the filter line is a section-header claimant (`applySectionHeader` / `applyProjectsSectionHeader`), never a notice-band contender, so no suppression path exists to invert — explicit ordering + discriminator is the right shape. ACCEPTANCE_CRITERIA met with one criterion unguarded (the issue above). CONVENTIONS followed. ARCHITECTURE sound.
- Five reachable theme signals routed via `setThemeFlash` through exactly two chokepoints — `blockThemePanel` (`theme_panel.go:516`, covering the three entry blocks including `openThemePanel`'s post-read re-refusal) and `resizeThemePanel` (`:937`, covering both forced closes). No parallel setter was added.
- Eight mutations run in an isolated copy; seven caught: filter-state suppression at the top of both arbiters → 5 tests fail (the precedence tests ARE load-bearing, not vacuous); filter contender injected *between* the two tiers → only `TestThemeFlash_NonThemeFlashUnchanged` fails; `setThemeFlash` drops the origin stamp → 6 tests fail; origin resets removed from `setFlash`/`setSuccessFlash` → `TestThemeFlash_OriginDiscriminator` fails; `blockThemePanel` reverted to `setFlash` → 5 tests fail via the runtime table; `resizeThemePanel` reverted to `setFlash` → source guard fires with an exact file:line offender; tier order swapped → all green, inherent and acknowledged by the task.
- `setThemeFlash` delegating to `setFlash` (rather than restating the assignments) is the right composition — it structurally prevents the two lifecycles diverging, which is the failure the task warned about.
- Extracting `flashClaim` and narrowing it into `themeFlashClaim` gives both arbiters one shared contender definition and one shared tier definition; the two pages cannot drift.
- `flashOrigin` is a concrete named type with a zero value that means "today's behaviour", so every existing setter and the `WithInitialFlash` capture seed are correct by construction (verified: the one fixture seeding a flash, `sessions-inline-flash`, carries a non-theme message).
- `clearFlash` leaves `flashOrigin` stale, which is harmless because `themeFlashClaim` delegates to `flashClaim` and empty text short-circuits — consistent with `flashKind`'s existing treatment.
- **Source-guard heuristic — scrutinised, and it does cover task 9-9.** The vocabulary is derived structurally (31 identifiers today): any const/var whose value contains a string literal mentioning "theme", plus any string-returning func declared in a `theme*.go` file; `themeCopyReference` additionally flags inline literals mentioning "theme" at any depth of the argument. §14A pins 9-9's copy as `⚠ theme not saved — see portal.log`, which contains "theme", and 9-9's raise sites are the panel-close paths in `theme_panel.go` — so a const, a `theme*.go` selector, or an inline literal are all caught. Three residual misses, none likely for 9-9: (a) the copy assigned to a local var first, then passed; (b) a string-returning helper declared in a file *not* prefixed `theme`; (c) a future theme signal whose pinned copy does not contain the word "theme". Tightening rule 2 to "any const/var declared in a `theme*.go` file whose value is a string" would close (c) at no false-positive cost, since the vocabulary only fires at `setFlash`/`setSuccessFlash` call sites. Non-blocking.
- The same vocabulary currently absorbs generic names from `theme*.go` files (`FilterValue`, `paint`, `renderRow`, `cursorColumn`, `clampBlockHeight`). None reaches a flash setter today, but `m.setFlash(item.FilterValue())` would be a false positive. A latent, low-probability looseness worth remembering if the guard ever fires puzzlingly.
- Tier-order swap (`flashClaim` before `themeFlashClaim`) leaves the whole suite green. This is inherent — the theme tier is a strict narrowing of the flash tier, so with nothing between them the order has no observable effect. The task explicitly accepts this ("the guarantee currently holds by construction"). Not a defect; noted so a future reader does not mistake it for one.
- The pre-existing `TestProjectsFlash_FilterHeaderPrecedenceUnchanged` (`projects_flash_test.go:489`) still passes unchanged and remains accurate — it pins the non-change for an *ordinary* flash. Its comment "the §14A filter-line flip is Phase 9's" is now historical rather than false; the diff did not touch it, so it is out of this task's scope.
- The added production comments carry §-section citations throughout. `code-quality.md` forbids spec-section citations in comments, but this is the established house style across every file in `internal/tui` and every prior commit in this workflow; flagging them would be inconsistent and disproportionate. Recorded so the tension is on record.
- No production comment introduced by the diff makes a claim about tests, and none names a task or phase id (grepped).
- No existing test file was modified, moved or deleted — the diff touches only 4 production files plus one new test file. No assertion was weakened. `go test -count=1 ./...` green; `go vet ./...` clean; `golangci-lint run ./...` → 0 issues; `gofmt -l .` lists only the three pre-existing `internal/spawn` offenders.
