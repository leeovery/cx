TASK: theming-system-10-4 — README's Four Appearance Sites and the Config-File Table

ACCEPTANCE CRITERIA:
- `grep -in appearance README.md` returns zero matches.
- The tmux-passthrough advice paragraph is gone, not reworded.
- A theme paragraph in Configuration states terminal-background detection, the dark no-answer fallback, the constant pin, drop-ins, and `NO_COLOR` (canvas suppressed, picker blocked), and links `docs/theming.md`.
- The feature bullet names the theme system, the three built-ins and `t`, and still names `NO_COLOR`.
- The TUI-views paragraph names `t` and `prefs.json` instead of `appearance`.
- The `prefs.json` table row lists `theme` / `theme_light` / `theme_dark` alongside the grouping mode.
- The table has a themes-directory row whose env override is `PORTAL_THEMES_DIR`, with the directory-not-file distinction stated.
- The TUI Keybindings table has a `t` row.
- The Logging section, every unrelated table row, and the `x`/`xctl` phrasing are byte-unchanged (`git diff` shows only the intended hunks).
- Every markdown link and in-page anchor added or touched resolves.
- `go test ./...` and `go test -tags integration -p 1 ./...` are unaffected (no test reads README).

STATUS: complete

SPEC CONTEXT: §12.5 enumerates the four README `appearance` sites and their dispositions — the pin-when-detection-misfires paragraph is **deleted** (§8.7 retires the "detection is unreliable inside tmux" premise as untrue), the feature bullet and TUI-views paragraph are **rewritten to the theme setting**, and the `prefs.json` config row now lists `theme` / `theme_light` / `theme_dark` with the table gaining a **themes-directory** row carrying `PORTAL_THEMES_DIR` (§5.5 fixed that name so the docs could print it). §12.5 also forbids documenting the retained `appearance` key as live (§10.4 keeps it on disk for downgrade only). §14 is the discoverability rationale for naming the built-ins and the `t` key; §9.10 supplies the `NO_COLOR` block on `t`. §15.3/§12.4 make `docs/theming.md` the contract's source of truth, so the README points rather than restates.

IMPLEMENTATION:
- Status: Implemented
- Location: single commit `1933f60f` touching README.md only (plus tick/manifest bookkeeping); no later commit re-touches README.md.
  - README.md:83 — feature bullet rewritten (three built-ins by display name, `t`, terminal-background match, drop-in `.theme` files, `NO_COLOR`).
  - README.md:260 — `| `t` | Open the theme picker |` keybinding row.
  - README.md:267 — TUI-views paragraph rewritten (active theme's colours, `t`, `prefs.json`, links `#configuration` + `docs/theming.md`, `NO_COLOR`).
  - README.md:380 — `prefs.json` row now lists grouping mode + `theme` / `theme_light` / `theme_dark`.
  - README.md:382 — new `themes/` row with `PORTAL_THEMES_DIR`.
  - README.md:385 — the `_DIR` vs `_FILE` distinction sentence under the table.
  - README.md:389 — `**Theme.**` paragraph replacing the deleted `**Appearance.**` paragraph.
- Notes: every factual claim in the new copy was checked against the code, not just against the plan text:
  - "picks a light or dark one by asking your terminal … falling back to dark" — `theme.ResolveSetting` (internal/theme/setting.go:67-80) defaults to the **adaptive pair** when no theme key is set, with `DefaultLightSlug`/`DefaultDarkSlug` = `tokyo-night-day`/`tokyo-night` (internal/theme/builtins.go:13-14) and the dark no-answer fallback per the appearance gate. Accurate.
  - "`t` is unavailable there" under `NO_COLOR` — `Model.themePanelEntry` returns `themePanelNoColorFlash` and refuses entry when `m.colourless` (internal/tui/theme_panel.go:96-104, flash constant at :34). Accurate, and §9.10-conformant.
  - `t` carries **no** "(sessions list only)" qualifier, correctly: `keymapEntry{Key: "t", Action: "theme"}` is present in both the sessions and projects descriptors (internal/tui/keymap.go:40, :53), unlike `Space`/`s`/`m` which are qualified in the table.
  - "Three themes ship — Tokyo Night, Tokyo Night Day, and Nord" / the slug spelling in the Configuration paragraph both match `internal/theme/builtins/` (`nord.theme`, `tokyo-night.theme`, `tokyo-night-day.theme`) and docs/theming.md's built-in table (slug ↔ palette-name column), so the display names are not invented.
  - "Portal never creates it — no directory simply means no drop-ins" matches `themesDirPath` (cmd/config.go:191-205), which resolves and returns a path and never creates/seeds/stats.
  - "Two of those overrides end in `_DIR` … `themes/` and `state/`" is correct for the table as it now stands.
  - Retained `appearance` key: not mentioned anywhere in README (`grep -in appearance README.md` → no matches), satisfying the §12.5/§10.4 prohibition.
  - Scope discipline held: `git show 1933f60f -- README.md` is exactly four hunks covering the feature bullet, the keybindings table + TUI-views paragraph, and the config table + note + Theme paragraph. Logging section, `x`/`xctl` phrasing, Screenshots light-mode sentence and every unrelated table row are byte-unchanged.

TESTS:
- Status: Adequate (documentation task — verified by the plan's grep/diff checks, correctly no Go test added)
- Coverage: ran the task's stated checks directly —
  - `grep -in appearance README.md` → zero matches (exit 1).
  - `grep -n PORTAL_THEMES_DIR README.md` → README.md:382, the table row.
  - `grep -n "docs/theming.md" README.md` → :267, :382, :389; `test -f docs/theming.md` passes.
  - Banned promises: `grep -in "portal theme list\|--theme" README.md` → zero matches (exit 1). Neither ships, and neither is promised.
  - Link resolution: every relative link in the file resolves (`docs/theming.md`, `docs/custom-terminals.md`, `LICENSE`); the only non-resolving targets are external `https://` URLs. In-page anchors resolve: `#configuration` → `## Configuration` (README.md:371), `#multi-select-mode` → `### Multi-Select Mode` (:289), `#logging` → `## Logging` (:393).
  - Suite impact: no Go source reads README.md. The only `README` occurrences under `*.go` are unrelated fixture filenames used as negative cases (cmd/doctor_theme_test.go:147, internal/theme/enumerate_test.go:245, internal/sourceguardtest/{gosourcefiles,packagegofiles}_test.go). Both lanes are structurally unaffected.
- Notes: a Go guard asserting README stays `appearance`-free would be over-testing here — the doc's honesty is maintained by hand for the discovery half by spec (§12.4 puts the automated guard on docs/theming.md's *vocabulary* table only). No guard added is the right call.

CODE QUALITY:
- Project conventions: Followed. The doc points at `docs/theming.md` rather than restating the token vocabulary, file format or slug rules, exactly as §15.3 makes that doc the source of truth. No `--theme` / `portal theme list` promises. Copy avoids tool-specific references (memory: `project_portal_no_claude_messaging`) and stays in the README's existing register.
- SOLID principles: N/A (documentation).
- Complexity: Low.
- Modern idioms: N/A.
- Readability: Good. The Theme paragraph is one dense paragraph but tracks the reader's actual question order (what decides the colour → how to pin → where drop-ins go → `NO_COLOR` → where the full contract lives). The feature bullet uses display names while the Configuration paragraph uses slugs — deliberate and correct (a bullet is prose, a config paragraph must print the literal value), and both match docs/theming.md's built-in table.
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] README.md:373 — the table's lead-in still reads "Each file also has a per-file env var override that takes full precedence", but the table now holds two directory rows (`themes/`, `state/`) and its first column header is "File". Replace the sentence with "Each entry also has an env var override that takes full precedence." and the header cell `| File | Purpose | Env override |` with `| File / directory | Purpose | Env override |`. (Pre-existing residual — `state/` already made the table mixed before this task — but this task is what added the second directory row and the :385 distinction sentence, so the lead-in is the last place the file-only framing survives.)
- [idea] README.md:116-244 (Commands section) — the section documents every other user-facing verb (`xctl list`, `xctl kill`, `xctl alias`, `xctl hook`, `xctl doctor`, `xctl version`, `portal uninstall`, `portal init`) but has no entry for `portal theme export`, which ships (cmd/theme.go:15, :78) and is the command docs/theming.md:297-307 tells users to run for the whole drop-in workflow. §12.5 scoped this task to four sites only, so adding a Commands entry is a genuine scope decision (widen §14's discoverability surface vs. keep the README a pointer) rather than an omission from this task.
