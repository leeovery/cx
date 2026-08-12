TASK: theming-system-10-5 — The CHANGELOG Upgrade Note

ACCEPTANCE CRITERIA:
- Entry at the top of `CHANGELOG.md` above the previous release, under a `## [X.Y.Z] - YYYY-MM-DD` heading matching the file's existing form; no `Unreleased` heading.
- Element 1: the new setting (`theme` / `theme_light` / `theme_dark`) and the three built-ins named, with `docs/theming.md` linked.
- Element 2: `appearance` stated as replaced and translated automatically, with the exact mapping and an explicit "you do not need to do anything".
- Element 3: the old key stated as left in place for downgrade and not kept in sync afterwards.
- Detection described as following the terminal background.
- The footer keymap revision noted.
- No mention of an unset key, a `--theme` flag or `portal theme list`.
- Sections use the file's existing emoji idiom and Keep a Changelog structure.
- No internal-only changes in the entry.

STATUS: complete

SPEC CONTEXT:
§12.5 (spec:1553-1557) mandates a user-visible upgrade note, not just a feature line, carrying three elements: the new setting + three built-ins pointing at `docs/theming.md`; `appearance` replaced and translated automatically (pinned `light`/`dark` → equivalent constant theme, `auto` needs nothing) so the user need not act; the old key left in place for downgrade and not re-synced. §10.5 (spec:1347) makes the CHANGELOG the *compensating channel* for a runtime-silent translation — no flash, no notice band, no banner. §10.4 keeps `appearance` on disk as a frozen legacy value because Homebrew downgrades are routine; §9.9 accepts "no unset" on the grounds that prefs.json is hand-editable *and documented*. §1.4 rules out an unset key, a `--theme` flag and `portal theme list` — none may be promised. Notably, spec:858-860 shows OSC 11 was already the detection signal pre-feature and DEC 2031 (OS colour scheme) is deliberately declined — so the CHANGELOG must not imply the OSC 11 mechanism is new, only that it now chooses between two *themes*.

IMPLEMENTATION:
- Status: Implemented
- Location: `/Users/leeovery/Code/portal/CHANGELOG.md:8-22` (entry `## [0.11.0] - 2026-08-07`, above `## [0.10.5] - 2026-07-27` at :24). Authored in commit 55631723.
- Notes (criterion-by-criterion, each verified against the shipped code, not just the text):
  - **Placement/heading** (:8): `## [0.11.0] - 2026-08-07` is byte-shape-identical to the previous heading form; sits directly above 0.10.5; no `Unreleased` heading anywhere (grep clean). Version shape is right: latest tag is `v0.10.5`, so a minor bump for an additive feature follows the existing release process; no versioning convention invented. No link-reference block at the file's tail, matching the file (it has none).
  - **Element 1** (:11-15, `✨ Added`): names the `t` selector with live preview, `Enter` set, `d`/`l` slot assignment — matches `themePanelKeymap` (`internal/tui/keymap.go:64-70`); the three built-ins `tokyo-night` (dark), `tokyo-night-day` (light), `nord` (dark) — matches the three files in `internal/theme/builtins/` and their canvases (`#0b0c14`, `#e1e2e7`, `#2E3440`); drop-in `.theme` auto-discovery with `PORTAL_THEMES_DIR` (`cmd/config.go:194-197`); the "re-read every time the selector opens" claim is exactly what `openThemePanel` does (`internal/tui/theme_panel.go:121-134`, "Re-read per open"); `portal theme export <slug>` "comments and all" is accurate — the command writes the source bytes verbatim (`cmd/theme.go:29`) and the built-in files are comment-headed; `portal doctor` theme advisories match all three shipped advisory shapes (unreadable dir / rejected file + reason / persisted theme that no longer resolves — `cmd/doctor_theme.go:143,160,184`). `docs/theming.md` is pointed at (:11) and exists (485 lines, a full guide).
  - **Element 2** (:18-19): setting named as `theme` or `theme_light`/`theme_dark` replacing `appearance`; translation stated as automatic "the first time you open the picker" — accurate, `loadPrefsStore` is called from exactly one site, `open.go:openTUI` (pinned by `cmd/prefs_translation_test.go:292-303`). The mapping is stated **exactly** and matches `translateAppearance` (`cmd/config.go:176-185`) with `DefaultDarkSlug = "tokyo-night"` / `DefaultLightSlug = "tokyo-night-day"` (`internal/theme/builtins.go:13-14`): `dark` → `theme: tokyo-night`, `light` → `theme: tokyo-night-day`, `auto` → nothing. The "auto needs nothing — the shipped default is already the light/dark pair it meant" claim is verified against `ResolveSetting` (`internal/theme/setting.go:67-80`): no keys ⇒ adaptive pair of the shipped defaults. The explicit "so there is nothing you need to do" is present — the element the runtime silence is compensating for.
  - **Element 3** (:20): both halves present — "left in `prefs.json` untouched, so an older Portal still honours it if you downgrade" and "theme changes made from here on are not written back to it". Matches the preserved raw field (`internal/prefs/store.go:73`) and §10.4's frozen-legacy decision.
  - **Detection** (:21): carefully worded and *correct* — the "now" attaches to "chooses between your two themes rather than between two fixed canvases" (the real change), while the OSC 11 sentence is descriptive and does not claim the mechanism is new. This avoids the trap the task's own bullet wording ("detection now follows the terminal's background") could have walked into, since OSC 11 was already the pre-feature signal (`internal/tui/appearance_gate.go` at the prior revision). The OS-colour-scheme contrast matches spec:858-860.
  - **Keymap** (:22): "Sessions footer drops `↑↓ navigate` … and gains `t theme` and `m multi`; Projects gains `t theme`" matches the descriptors exactly — the nav entry carries no `Core` flag (`internal/tui/keymap.go:22`), while `t`/`m` are `Core: true` on Sessions (:40-41) and `t` is `Core: true` on Projects (:53). The "still listed in `?` help" aside is true (the descriptor retains it for the help modal).
  - **No unshipped promises**: `grep -in "theme list\|--theme\|unset\|Unreleased" CHANGELOG.md` returns only lines 409 and 418 (unrelated `@portal-skeleton-*` marker text in the 0.6.x-era entries). Nothing in the new entry.
  - **No internal-only lines**: read-through of :11-22 — every line names something a user can see or do. No package moves, no test-suite changes, no `capturetool --theme`.
  - **Emoji idiom**: `✨ Added` / `🔧 Changed`, no blank line between heading and first bullet, matching the majority form in the file (e.g. :10-11, :17-18, :31-32). No `🗑️ Removed` section, correctly — nothing user-visible is removed (the footer nav entry survives in help, and `appearance` is retained).

TESTS:
- Status: Adequate (documentation task — no automated coverage expected or appropriate)
- Coverage: The task's "tests" are checklist reads and greps, all of which I re-ran independently: three-element checklist against §12.5 ✓; `grep -in "theme list\|--theme\|unset"` shows nothing in the new entry ✓; mapping stated exactly and matching `translateAppearance` ✓; heading shape diffed against the previous entry ✓; `docs/theming.md` linked and present ✓; no internal-only lines ✓.
- Notes: No Go test references `CHANGELOG.md` (grep across `*.go` is empty), which is correct — a guard test over prose would be brittle with no drift it could catch. Beyond the mandated checklist I verified every factual claim in the entry against the shipped implementation rather than against the task text, which is where a CHANGELOG can rot silently; all claims hold.

CODE QUALITY:
- Project conventions: Followed. The entry adopts the file's `## [X.Y.Z] - YYYY-MM-DD` heading, emoji sections, plain second-person user language, and code-span doc references (`docs/theming.md`) rather than markdown links — matching the file's own precedent at :32, even though README uses markdown links.
- SOLID principles: N/A (documentation).
- Complexity: N/A.
- Modern idioms: N/A.
- Readability: Good. Each `🔧 Changed` bullet states the change *and* why it is safe to ignore, which is what makes it work as the compensating channel rather than a feature list. The detection bullet is the standout — it distinguishes what changed (two themes vs two fixed canvases) from what did not (OSC 11), which prevents a future reader from mis-dating the mechanism.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] CHANGELOG.md:8 — change the heading date `2026-08-07` to the actual tag date when v0.11.0 is cut. The entry was authored on 2026-08-07, but the feature's final implementation commit lands 2026-08-12 and the release is still untagged (latest tag `v0.10.5`), so the published entry would claim a release date preceding the code it describes. The file's convention is that the entry is dated on release day (release commit 75654201 added the 0.10.5 heading dated 2026-07-27 on 2026-07-27).
- [idea] CHANGELOG.md:11-15 — decide whether to add an `✨ Added` line noting that `t` is refused under `NO_COLOR` ("theme picker needs colour — NO_COLOR is set", `internal/tui/theme_panel.go:34`). It is user-visible behaviour introduced by this feature and the file has precedent for documenting a blocked-key refusal (the 0.10.2 entry documents the analogous `m`-on-unsupported-terminal refusal at :49); against that, §12.5 does not mandate it, README already covers it, and NO_COLOR is a niche audience — hence a scope call rather than an omission.
