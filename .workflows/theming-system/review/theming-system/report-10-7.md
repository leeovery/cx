TASK: theming-system 10-7 — Amend the Modern Vivid Specification per §15.2 and §15.1's Keymap Revision (completed-unit correction protocol against `.workflows/spectrum-tui-design/specification/spectrum-tui-design/specification.md`)

ACCEPTANCE CRITERIA (from the plan task):
1. `manifest get spectrum-tui-design status` run and returned `completed` before any edit.
2. §2.1/§2.9 state 19 tokens with the §2.4 names, single values, contrast measured against the theme's own `canvas`; no Light/Dark variant framing, no `Token.ColorFor`/`theme.Mode`.
3. `border.footer` appears nowhere in the body; §8.1 describes a single `border` role.
4. §2.9's value table no longer reads as the source of truth for values, and points at the embedded `.theme` files.
5. §12.2 carries §14's revision including the two decided footer rows and the `m multi` label.
6. §15.4 is byte-unchanged.
7. The corrigendum block is extended with one entry per correction, each quoting the original claim and stating what is true, dated the day of the edit.
8. No "was X, now Y" residue survives in any edited section body.
9. The re-index command was run against the artifact path and succeeded.
10. The commit is scoped to `spectrum-tui-design` with the protocol's message form; the owning manifest's status is still `completed`.
11. Any correction beyond §15.1/§15.2's named sections was surfaced and agreed before being made.

STATUS: complete

SPEC CONTEXT:
theming-system spec §15.1/§15.2 name the minimum amendment set for the completed `spectrum-tui-design` unit (§2.1/§2.9 token renames + 20→19 + dropped `border.footer`; §2.9's removal of `Token.ColorFor`/`theme.Mode` and the two-hardcoded-canvas framing; §8.1's stale 2-tone border; §12.2's keymap/footer revision). §15.3 makes the embedded `.theme` files the source of truth for values; §15.4 declares MV's Paper frames historical, so a frame still showing `↑↓ navigate` is explicitly not a defect. The governing protocol is `.claude/skills/workflow-shared/references/correcting-historical-artifacts.md` (edit-in-place, corrigendum, re-index, scoped commit). The theming-system spec's own §11-equivalent (lines 1816–1825) supplies the notice-band precedence and the "Projects gets the flash contender alone" wording the corrigendum transcribes.

IMPLEMENTATION:
- Status: Implemented
- Location: `.workflows/spectrum-tui-design/specification/spectrum-tui-design/specification.md` (corrigendum block lines 18–41; body edits across §1, §2.1, §2.3, §2.6, §2.7, §2.8, §2.9, §3.4, §6.3, §8.1, §8.5, §11, §12.1/§12.2, §13.2, §14.5, §15.6, §16.1/§16.3). Commit `3ffa63ab` "specification(spectrum-tui-design): corrigendum from theming-system" — 3 files: the spec, `.workflows/.knowledge/store.msp`, `.workflows/.knowledge/metadata.json`.
- Verification against each criterion:
  - AC1: `.workflows/spectrum-tui-design/manifest.json` line 4 is `"status": "completed"`, and commit `3ffa63ab` does not touch the manifest — the ordering claim itself ("run before any edit") is not recoverable from artifacts.
  - AC2: §2.1 (line 101) — "A role holds one value. A token is a `{name, value}` pair resolving through a no-argument colour accessor, and a theme is a single palette that is itself light or dark". §2.9 (line 149) — "closed set of 19 named tokens", MV as two independently-tuned themes each carrying its own canvas. The three §2.9 tables enumerate exactly 19 tokens whose names match `internal/theme/theme.go`'s `fields()` table one-for-one and in order. No `ColorFor` or `theme.Mode` occurs anywhere in the file (the accessor described matches `Token.Color()`, `theme.go:22`).
  - AC3: `border.footer` occurs only inside the 2026-08-07 corrigendum (lines 29, 31, 38). §8.1 (line 345) reads "defined by its **single-tone `border` frame** … (the shared panel renderer takes one border token)"; §2.9's surfaces table has a single `border` row covering "title rule (2px), footer rule (1px), modal panel frames, edit-modal chips". Confirmed against code: `internal/tui/footer.go:137` renders the footer rule with `th.Border`.
  - AC4: §2.9's preamble (line 149) — "The values below are MV's record, not their source: the source is the embedded `.theme` files the loader parses (§2.8)"; the rules list repeats it (line 188). All 38 hexes in the tables match `internal/theme/builtins/tokyo-night.theme` and `tokyo-night-day.theme` verbatim, so the record is accurate as well as correctly subordinated.
  - AC5: §12.2 lines 521–523 carry the arrows-out-of-the-footer change, both decided footer rows, `t`/`m` promoted to core, the `m multi` label with its rationale, and the right-to-left degrade + lockstep filtering. Both rows match `internal/tui/keymap.go`'s `Core` entries exactly (Sessions: ⏎ / ␣ s x t m + right-aligned ?; Projects: ⏎ x e / t + right-aligned ?).
  - AC6: §15.4 ("Verification responsibilities in the task loop") carries no token name, no keymap and no canvas claim; §15.1's frame map and its `#0b0c14`/`#e1e2e7` frame note are untouched. §15.6 is the declared exception and took the renames, as the corrigendum's closing line states. Consistent with both readings of the criterion (MV §15.4 unedited; Paper frames not "fixed").
  - AC7: The block now holds three entries (2026-06-22, 2026-06-25, 2026-08-07) — extended, not replaced, and the earlier two are intact including their retired-token quotes, which the new entry explicitly ratifies as history (line 38). The new entry carries 18 per-correction bullets, each quoting the superseded claim and stating current truth. Date 2026-08-07 matches the commit date (Fri Aug 7 16:30 +0100).
  - AC8: Section bodies read as present-tense current truth throughout; two residual history clauses noted below (non-blocking).
  - AC9: `.workflows/.knowledge/store.msp` (+2196/-2196 lines of msgpack) and `metadata.json` moved in the same commit — the re-index ran. Chunk-level replacement and the post-index query result are not verifiable by reading.
  - AC10: Commit touches only the owning unit's artifact plus `.workflows/.knowledge`; message is the protocol form; manifest status is still `completed`. Not folded into 10-8/10-9.
  - AC11: The corrigendum enumerates every section it corrected beyond the named minimum (§1, §2.3, §2.6, §2.7, §2.8, §3.4, §6.3, §8.5, §11, §12, §13.2, §14.5, §16), so the extra set is visible in the artifact; whether it was surfaced at a gate before editing is not recoverable from artifacts.
- Notes on correctness of the corrected claims (spot-checked against the current codebase, not just internal consistency):
  - 19 token names/order: `internal/theme/theme.go:58-80` — exact match.
  - Dark ratio corrections (§2.9): recomputed WCAG against `#0b0c14` — `text.primary` 12.08 → table 12.1, `accent.primary` 8.43 → 8.4, `text.faint` 1.99 → 2.0. The corrected column is right; the old column was indeed pure-black-referenced.
  - Banded footnotes (`text.subtle` 3.0–4.49, `text.faint` 1.0–2.99): enforced by `internal/theme/contrast_test.go:96-118` (`assertAtLeast` + `assertBelow`), auto-enumerated over the embedded set — the claim "floor *and* ceiling, not floored-and-exempt" holds.
  - §2.6 "a single named theme skips the gate entirely": `internal/tui/appearance_gate.go:31-36` (`newNominationGate` pins on `IsConstant`), dark fallback at `resolveDark`, single-resolution at `resolve`.
  - §2.7's fourth degrade step / §3.4's right-to-left shed: `internal/tui/footer.go:97-124,191-207` (`fitClusterToWidth`, trailing `…`, anchor-last).
  - §3.4/§8.5 lockstep filtering: `renderSessionsFooter` takes entries as a parameter (footer.go:20-24) with the blocked-key filter at the call site — matches the corrected text.
  - §12.1 theme-picker keys (⏎ / d / l / esc, nested y/n confirm): `internal/tui/keymap.go:64-80`.
  - §11's precedence order matches the arbiter (`internal/tui/notice_band.go:180-255`) and CLAUDE.md; the theme-flash-above-filter-line tier exists as `themeFlashClaim`/`flashSlotClaim`.
  - §7.1's two filter footers ("type to filter · ↵/↓ browse results · esc clear" and "↵ attach · ↑↓ navigate · esc clear filter") match `internal/tui/filter_footer.go:21-50` — so the `↑↓ navigate` string surviving there is correct copy, not missed keymap residue.
- Corrigendum form: the new entry follows the file's two existing entries' house shape (`> **⚠ Corrigendum — {date} ({source}).**` + per-correction `Superseded:`/`Current:` bullets) rather than the protocol's literal one-line template. Judged correct: the task's instruction was to *extend* the existing block, and the entry carries every element the template requires (date, provenance, quoted original, corrected truth). Not raised as a finding.

TESTS:
- Status: Adequate for the work type
- Coverage: This is a documentation-correction task; its "tests" are grep-shaped artifact checks, all of which I re-ran by reading the file end-to-end:
  - owning unit completed — manifest.json `"status": "completed"` ✓
  - no dropped token name survives — `border.footer` / `accent.violet` / `accent.cyan` / `state.green` / `state.red` / `text.dim` / `text.detail` / `bg.track` / `bg.warning` / `border.separator` appear only inside corrigendum entries (historical quotes, explicitly ratified by the new entry's line 38) ✓
  - no removed API survives — no `ColorFor`, no `theme.Mode` anywhere ✓
  - keymap revision landed — `m multi` present in §3.4 and §12.2; `↑↓ navigate` survives in §12.2 only as the explicit negation "`↑↓ navigate` is **not** a footer entry", which the change requires, and in §7.1's filter-applied footer, which code confirms is accurate ✓
  - corrigendum grew per correction — 18 bullets, one per correction, plus the §15 carve-out line ✓
  - re-index — knowledge store + metadata moved in the same commit ✓ (chunk-level replacement/query not readable)
  - commit scoped — 3 files, owning unit only ✓
- Notes: No Go test change is expected or appropriate here, and none was made — correct. I did not execute any test; the code claims above were verified by reading source. The one criterion class that no artifact can evidence is the two process criteria (AC1 ordering, AC11 gate agreement).

CODE QUALITY:
- Project conventions: Followed. Edit-in-place with a single annotation block, house-style corrigendum, scoped commit message in the `specification(<unit>): …` form, manifest untouched (no reopen, no status change).
- SOLID principles: N/A (documentation artifact).
- Complexity: Low. The corrigendum is long but each bullet is a self-contained superseded/current pair; the rename bullet (line 38) consolidates the mechanical substitutions instead of repeating them per section, which is the right factoring.
- Modern idioms: N/A.
- Readability: Good. Bodies read as current truth with no diff-shaped prose; the corrigendum quotes originals verbatim so `git log -p` is not needed to know what changed. The §15 carve-out and the "retired names in earlier entries stand as written" ratification both pre-empt the obvious reader questions.
- Issues: Three small body-level accuracy/residue points and one out-of-scope staleness set, all below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `.workflows/spectrum-tui-design/specification/spectrum-tui-design/specification.md:195` — the last §2.9 rule still reads "All values are a **hypothesis until prototyped in a real terminal (§15)**; the table is the build target, validation is the lock", which re-asserts the table as the authority the preamble at line 149 just demoted (and is itself false now that the palette shipped). Replace with: "The values were prototyped and locked in a real terminal (§15); the theme files carry them, and this table records what that validation settled."
- [do-now] `…/specification.md:676` (and the same clause at `:73`) — "The user-overridable theme system **it was once bundled with** is delivered; transparency **did not come with it**" is the "was X, now Y" residue AC8 excludes from a section body; §2.8 already states delivery, so §16.3 needs no history. Replace the §16.3 bullet with: "**The \"use terminal background\" transparency opt-out** — a distinguished value meaning \"use the terminal default\", which the theme file format leaves the door open for as a purely additive loader change (§2.8). Deferred on its own terms." and trim §1's line 73 to "… is **deferred on its own terms** (§16.3): the theme file format leaves the door open for a distinguished value meaning \"use the terminal default\"."
- [do-now] `…/specification.md:486` — "Every other contender is Sessions-only with no Projects analogue" is contradicted by §11.4's command-pending banner, which is Projects-only and shares the same slot (`internal/tui/notice_band.go:196-204` ranks the flash above `commandPending`). Replace with: "Every other contender **in the order above** is Sessions-only; §11.4's command-pending banner is Projects' own persistent band and sits directly beneath the flash." (The imprecision was transcribed from the theming-system spec's own wording at specification.md:1825, so consider correcting there too.)
- [idea] `…/specification.md:436,455-464` and `:511` — pre-existing staleness this task did not (and was not asked to) cover: §10.2/§10.4 still describe an **11-step** bootstrap and map "9–11 marker/FIFO/stale cleanup" (the orchestrator has been ten steps since CleanStale moved onto the `_portal-saver` daemon), and §12.1's Preview keymap omits the `Home/End` top/bottom binding `previewKeymap()` ships (`internal/tui/keymap.go:97`). Neither was falsified by theming-system, so leaving them was within this task's scope; the decision is whether a further corrigendum against the same file is worth opening.
