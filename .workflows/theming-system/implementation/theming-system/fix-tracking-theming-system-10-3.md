## Attempt 1

ISSUES:

- `docs/theming.md:227` — the inserted fourth bullet (`:222-225`) changed what the pre-existing "Break either" sentence scopes over. That sentence now sits directly beneath a rule whose breach is `reserved name`, while asserting the rejection is `bad name`. A user reading the naming section top-to-bottom — the section the doc positions as "worth reading before you write the file" — is told the wrong rejection reason for a reserved slug, in a doc that elsewhere goes to lengths to keep the six ladder rungs distinct.

  FIX: name the two rules the sentence actually covers, so the claim is independent of how many bullets precede it. Replace

      Break either and the file is rejected as `bad name` — an unselectable row in the

  with

      Break the slug rule or the extension rule and the file is rejected as `bad name` — an
      unselectable row in the

  and, to name the reason at the site the user is reading, add it to the new bullet:

      built-in themes* below. `nord.theme` is rejected as `reserved name` however good the
      palette inside it is, and the copy in the workflow below is named `nord-lee` for

  ALTERNATIVE: move the reserved-slug bullet above the slug-charset bullet so "either" again binds to the two adjacent `bad name` rules. Cheaper, but it leaves "either" load-bearing on list order — the same fragility that produced this defect — and it reads oddly to state which names are taken before stating what a name may contain. The reviewer recommends the explicit naming.

  CONFIDENCE: high

NOTES:

- VERDICT context: SPEC_CONFORMANCE conformant. Every claim in the three new sections traces to a spec decision and to the shipped code: §5.4 (reserved slugs + the rename workaround + not-discoverable-from-the-UI), §8.2 (two states, mutual exclusion on write, theme-wins for a hand-edit), §8.3 (shipped pair, partial pairs do not exist), §8.7/§8.8 (OSC 11 terminal background, not the OS scheme; single resolution; dark no-answer fallback; constant skips the wait; the correct-at-launch cost), §9.9 (no unset — hand-edit route, and deleting ≠ writing the shipped slug back), §7.4 (Nord's two corrections, reason only), §12.4 (attribution = source + link, in repo/docs, explicitly not in the UI), §10.4/§12.5 (`appearance` documented nowhere). ACCEPTANCE_CRITERIA all met. CONVENTIONS followed. ARCHITECTURE sound.
- Verified `BuiltinSlugs()` returns exactly `[nord tokyo-night tokyo-night-day]`; no fourth built-in. `ResolveSetting("","","")` → `{tokyo-night-day, tokyo-night}`. `SaveTheme` clears both slots and `SaveThemeSlot` clears the constant (`internal/prefs/store.go`). `ResolveSetting("nord","a","b")` → constant, slots unread. `ResolveSetting("","","nord")` → light=tokyo-night-day.
- Detection claims match `internal/tui/appearance_gate.go` (50ms race, single resolution, `resolveDark` → dark, pinned gate for a constant).
- **Nord's two corrections verified as the only two**: `state.destructive` (#DD8188 vs nord11 #BF616A) and `state.positive` (#A7C492 vs nord14 #A3BE8C) are the only shipped values that diverge from an upstream Nord colour. All 19 values in each of the three built-ins cross-checked against §7.3/§7.4 — nothing moved, so no superseded values to quote.
- Quoted doctor line is byte-accurate: `⚠ theme file nord.theme: nord is a built-in — rename it (e.g. nord-mine.theme)` matches `reservedNameAdvisoryFormat` (`cmd/doctor_theme.go:59`).
- Each `theme export` is byte-identical to its `builtins/*.theme`, so the attribution header travels with a copy.
- Live hand-edit checks under isolation: `{theme: ghost-constant, theme_dark: ghost-dark}` → doctor reports only the constant, the slot is not read (`theme` wins confirmed live). `{theme_dark: ghost-dark}` → doctor reports only `(dark)`; light is silent because it took the shipped default (partial pairs confirmed non-existent).
- 10-1's guard green — the new `Slug|Palette` and `Theme|Source` tables are invisible to the `Token`-headed parser. `grep -c appearance docs/theming.md` → 0. The diff touches no `.go` file.
- `:349` "Any name outside the three works" — true of the reservation rule, but the slug charset and the lowercase extension still apply (`Nord-Lee.theme` does not work). The rules are two sections above and the sentence is scoped to the collision, so it reads fine in place; worth a glance only while the fix above is being made.
- `:460` the attribution table credits `tokyo-night-day` as "Tokyo Night", while that file's own header reads "Tokyo Night Day — <same link>". Same source, same URL, so `:463` "Each file carries the same credit in its header" holds substantively.
- The Nord section documents the two corrections, which is what the task and §7.4 ask for. The port also invents three values (`text.muted`, `text.subtle`, `bg.attention`) and takes one functional maximum (`text.on-selection`) — a reader could infer the other 17 values are Nord's verbatim. The file's own header records all of it and export prints it, so the durable record exists; noted only because the section's purpose is honesty about divergence.
- Out of scope: `tokyo-night-day.theme` ships seven contrast corrections against its source values, and the doc records corrections for Nord only. §12.4 scopes the doc's corrections record to Nord, so the executor followed the spec — flagged in case the attribution-honesty intent should reach the light built-in in a spec amendment.
- `:367-368` "A change lands at the next launch" — verified correct for a hand-edit (theme keys are read once in `loadPrefsStore`, and the panel's enumeration takes the in-memory keys rather than re-reading prefs). A panel commit does apply live in-session; the antecedent is the hand-edit, but the sentence sits two clauses after "The theme picker writes it for you".
- Full unit lane `go test -count=1 ./...` green; `go vet ./...` clean; `golangci-lint run ./...` 0 issues; `gofmt -l .` reports only the three pre-existing `internal/spawn` offenders.
