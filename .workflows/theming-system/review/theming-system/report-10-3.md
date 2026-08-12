TASK: theming-system-10-3 — Built-in Slugs, the Two-Slot Config and Palette Attribution (the third and final section set of `docs/theming.md`)

ACCEPTANCE CRITERIA:
1. The three built-in slugs are listed and documented as reserved, with `reserved name` named as the rejection reason.
2. The rename workaround is given concretely (`nord` → `nord-lee.theme`) alongside the export line that produces it.
3. The setting is documented as two states — constant or pair — with "nothing set" and "pair nominated" stated to be the same state.
4. Mutual exclusion is documented as Portal's write rule and `theme`-wins as the hand-edit tiebreak, with the both-present state noted as unreachable from the UI.
5. Partial pairs are documented as non-existent, with the `theme_dark = nord` example showing light still on `tokyo-night-day`.
6. Detection is documented as terminal-background via OSC 11 with a dark no-answer fallback, and the not-live-following cost is stated.
7. `prefs.json` is documented as the hand-editable home, and returning to the shipped pair is documented as a hand-edit.
8. Attribution carries source and link only; no licence line, no "(adapted)" convention, no contribution ceremony; attribution is stated to be absent from the UI.
9. Nord's two corrections are named with their reason and no derivation figures.
10. The word `appearance` appears nowhere in `docs/theming.md`.
11. Values quoted in this section match the shipped `.theme` files, not §7.3's tables, where the two differ.
12. Task 10-1's guard remains green.

STATUS: complete

SPEC CONTEXT:
- §5.4 (no shadowing): a colliding drop-in is rejected `reserved name`; the reserved set is deliberately invisible in the panel (§9.5), so `portal theme export` (§12.1) + `docs/theming.md` are the only discovery routes. Workaround is a rename (`nord` → `nord-lee.theme`).
- §8.2/§8.3: two states not three; mutual exclusion enforced on write; `theme` wins a hand-edited both-present file and the stale slots are neither read nor pruned; partial pairs do not exist because an unset slot holds the shipped default (`theme_light = tokyo-night-day`, `theme_dark = tokyo-night`).
- §8.7: detection is terminal background over OSC 11 (explicitly not DEC 2031 / the OS scheme), resolves once against a short timeout, no answer ⇒ dark; accepted cost is correct-at-startup rather than live-following.
- §9.9: "no unset" is accepted *because* `prefs.json` is hand-editable **and documented** — so the hand-edit route must be stated.
- §7.4: fidelity-versus-floors resolved in favour of the floors; Nord's `state.destructive` and `state.positive` ship corrected under Nord's own name, recorded in `docs/theming.md` alongside attribution.
- §12.4: doc contents (two-slot config, reserved slugs, attribution = source + link only, explicitly not in the UI); §12.5: the retained `appearance` key is never documented as live.

IMPLEMENTATION:
- Status: Implemented (documentation task; commit `3dccad31`, `docs/theming.md` +170 lines, no source changes).
- Location:
  - `docs/theming.md:326-362` — "The built-in themes": the three-slug table, `reserved name`, "before the file is opened", the §5.4 reason, the rename workaround with the export line, the `portal doctor` line, and the "the picker will not tell you which rows are built-in" discoverability note.
  - `docs/theming.md:364-453` — "Choosing a theme": `prefs.json` as the hand-editable home, the two states with JSON examples, "nothing set == a pair nominated", "Partial pairs do not exist" (`theme_dark = nord` ⇒ light still `tokyo-night-day`), "One form or the other" (mutual exclusion on write + `theme`-wins + slots not pruned), "How light and dark is decided" (OSC 11, once, dark no-answer, not live-following), "Going back to the shipped pair" (hand-edit; delete-vs-restate distinction).
  - `docs/theming.md:455-485` — "Attribution": the two-palette source/link table, credit in each file header, explicitly nowhere in the interface, and "Nord ships with two values corrected".
- Notes — every factual claim in the section was checked against the current code and the shipped files:
  - Reserved-before-open: `internal/theme/load.go:76-83` runs `isReserved` before `os.ReadFile`. `ReasonReservedName = "reserved name"` (`internal/theme/reason.go:14`).
  - Doctor line quoted verbatim matches `reservedNameAdvisoryFormat` (`cmd/doctor_theme.go:18`) — `⚠ theme file nord.theme: nord is a built-in — rename it (e.g. nord-mine.theme)`.
  - Export refusal wording matches `cmd/theme.go:41` and root's `SilenceErrors: true` (`cmd/root.go:163`), so the bare-line example is accurate.
  - The three slugs match the embedded set (`internal/theme/builtins/*.theme`); shipped defaults match `DefaultDarkSlug`/`DefaultLightSlug` (`internal/theme/builtins.go:12-14`).
  - Mutual exclusion on write matches `prefs.Store.SaveTheme`/`SaveThemeSlot` (`internal/prefs/store.go:220-245`); `theme`-wins + per-slot defaults + "stale slots left untouched" match `theme.ResolveSetting` (`internal/theme/setting.go:62-80`).
  - Detection matches `internal/tui/appearance_gate.go` (50ms timeout, single resolution, `resolveDark` fallback, constant nomination pinned so the gate never arms).
  - "Nothing clears" matches the panel's dispatch — `Enter`/`d`/`l`/`Esc` only (`internal/tui/theme_panel.go:318-334`); no unset key exists.
  - Nord's corrections match `internal/theme/builtins/nord.theme` (`state.destructive #DD8188` from nord11 3.05-on-canvas; `state.positive #A7C492` clearing both canvas and the nord2 selection tint) and §7.4's table; the doc quotes only `#2E3440`, which is the file's canvas. No derivation figures leaked into the doc — they remain as `#` comments in the `.theme` files.
  - `grep -c appearance docs/theming.md` = 0.
  - Attribution links match each file's header comment line-for-line (`tokyo-night.theme`, `tokyo-night-day.theme`, `nord.theme`).
  - Post-task drift check: `internal/theme/builtins/` has not changed since task 2-7, and the phases 11–17 remediations to `internal/theme/setting.go` / `internal/prefs/store.go` preserve exactly the semantics the doc states — no supersession affecting this section.

TESTS:
- Status: Adequate (documentation task — the automated obligation is task 10-1's guard staying green, plus the manual checks the task lists).
- Coverage: `internal/theme/docs_guard_test.go` — `TestThemingDocTokenTableMatchesAllTokens` parses only tables whose first header cell is `Token` (`parseDocTokenRows`, lines 222-248) and `inTokenTable` resets on any non-table line. Both new tables (`| Slug | Palette |` at :330, `| Theme | Source |` at :460) are preceded by a blank line and carry a non-`Token` header cell, so their backticked slug cells are never harvested as token rows — the guard is unaffected. `TestThemingDocExampleThemeIsValid` / `TestThemingDocExampleThemeIsTheDarkBuiltin` extract the fenced block under the "Example theme" heading (:90-124), which sits above everything this task added, so the new ```json / ```sh fences cannot be picked up. Verified by reading; not executed (test execution is out of scope for this review).
- Notes: no new automated coverage was added, which is right for a prose task — the doc's discovery half is spec-declared hand-maintained (§12.4). The residual rot risk is called out as a non-blocking note below.

CODE QUALITY:
- Project conventions: Followed. Matches the `docs/custom-terminals.md` precedent and the established voice of the 10-1/10-2 sections; cross-references are by section name ("the reason is the one given under *Naming the file*") rather than spec section numbers, and no workflow/process artefacts (task ids, phase numbers, spec §§) leak into user-facing prose.
- SOLID principles: N/A (documentation).
- Complexity: Low — three sections, each with one job; the built-in table is the single list, and the reserved-name reason is stated once and referred back to rather than restated.
- Modern idioms: N/A.
- Readability: Good. The "two states, not three" framing, the "partial pairs do not exist" subsection and the delete-vs-restate distinction under "Going back to the shipped pair" each land the non-obvious point in one paragraph. The doc consistently states the rule *and* why it exists, which is what makes the hand-edit contract usable.
- Comment accuracy: N/A for source; the doc's claims were individually verified against the code (see IMPLEMENTATION notes) and none is falsified by the implementation.
- Issues: none blocking.

BLOCKING ISSUES:
- None. All twelve acceptance criteria are met.

NON-BLOCKING NOTES:
- [do-now] `docs/theming.md:370` — "A change lands at the next launch." reads as covering both routes, but a picker commit repaints in-session immediately (`internal/tui/theme_panel_commit.go` commits into the live `themeState.active`); only a hand-edit waits for the next launch. Replace the sentence with: "A hand-edited change lands at the next launch."
- [do-now] `docs/theming.md:364-398` — the built-in table labels each palette "dark"/"light" (:332-334) but nothing says Portal never checks that a slot's theme actually matches its slot, which invites the reading that Portal knows. Add one sentence after the adaptive example (:393): "Nothing checks that a slot's theme is actually light or dark — the slot says *when* a theme is used, not what is in it." (spec §8.2 states exactly this; `theme.Theme` carries no mode field.)
- [idea] `docs/theming.md:366-367` — the `prefs.json` path chain names only `$XDG_CONFIG_HOME/portal/prefs.json` and `~/.config/portal/prefs.json`, omitting the `PORTAL_PREFS_FILE` rung that `configFilePath` actually honours (`cmd/config.go:187-188`), while the themes-dir chain three sections earlier (:249-253) is spelled out in full including its env var. Decide whether this doc should advertise `PORTAL_PREFS_FILE` — §5.5 fixed `PORTAL_THEMES_DIR`'s name in the spec *precisely so this doc could print it*, so the omission may be deliberate scoping rather than an oversight.
- [idea] `internal/theme/docs_guard_test.go` — no guard ties the doc's built-in table (`docs/theming.md:330-334`) to `theme.BuiltinSlugs()`, yet §5.4 makes this table one of only two discovery routes for the reserved set, and "adding a theme is adding a file" means a fourth built-in would silently rot it. A cheap addition would parse the `| Slug |`-headed table and compare against `BuiltinSlugs()`. §12.4 deliberately scopes the guard to the vocabulary half and declares the discovery half hand-maintained, so this is a decision to widen that scope, not a defect.
- [idea] `docs/theming.md:471-485` — the section documents Nord's two corrections (as scoped by §7.4 and the task) but not the three *invented* values (`text.muted`, `text.subtle`, `bg.attention`) that the port also carries, so a reader taking the section as the honest record of "what differs from upstream Nord" gets 2 of 5. One sentence would close it, e.g. "Three further values are inventions — Nord's dark end holds no slot for them — and each carries its derivation as a `#` comment in the file." Decide whether that belongs here or stays exclusively in `nord.theme`'s comments.
