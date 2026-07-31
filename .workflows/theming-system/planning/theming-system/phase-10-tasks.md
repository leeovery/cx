# Phase 10: Documentation, spec amendments and capture cleanup — 10 tasks

## theming-system-10-1

### Task 10.1: `docs/theming.md`'s vocabulary half and its token-table guard

**Problem**: Nine phases in, the 19 token names are a **public contract** — a drop-in author writes those keys, and §1.3 records that renaming one is a mechanical repo-wide change for built-ins but *breaks* every file in a user's themes directory — yet the contract has no user-facing home at all. §12.4 and §15.3 make `docs/theming.md` **the source of truth** for the roles and their meanings, and the text ramp's weight ordering has **no other record anywhere**: §2.7 makes file ordering explicitly not a contract, and §2.6 admits the ramp's middle join (`text.tertiary` → `text.muted`) mixes an ordinal vocabulary with a qualitative one, so ordering at that join rests on convention rather than on the names. Nothing would keep such a doc honest either: this feature found MV §8.1's "2-tone border (`border.separator` + `border.footer`)" claim stale against the shipped implementation **purely by chance** (§13.5) — same drift class, same subsystem, and the doc is about to become the fourth place the vocabulary is written down.

**Solution**: Author the vocabulary half of `docs/theming.md` — the 19 roles with their meanings, the ramp documented bright → faint, and a complete copy-pasteable example theme — and land a guard test in `internal/theme` that parses the doc's token table against `Theme.All()` and parses the example theme through the real loader. TDD in that order: the guard goes red against the missing doc first, then the doc turns it green.

**Outcome**: `docs/theming.md` exists and documents all 19 roles as the public contract; `go test ./internal/theme` fails if a token is added, removed or renamed without the doc moving with it, and fails if the doc's example theme ever stops being a valid 19-key file.

**Do**:
- **Write `internal/theme/docs_guard_test.go` first** and confirm it fails against the absent doc before writing a word of prose.
  - Resolve the doc **relative to the package dir** — `filepath.Join("..", "..", "docs", "theming.md")`, the idiom `internal/tui/pagepreview_audit_test.go` already uses. (`portalbintest.ProjectRoot()`, used by `internal/log/discard_guard_test.go`, is the repo's alternative and is unit-lane safe if a walk-up is preferred.) **A missing doc must fail, never skip** — a skipping guard is indistinguishable from a passing one in the run output.
  - Parse the token table by matching markdown rows whose first cell is a backticked token name. **Assert the parse yielded exactly 19 rows before comparing anything.** A parse matching zero rows must fail loudly with its own message: a guard that silently parses nothing passes vacuously, which is the same negative-assertion trap §13.4 names for the swap-and-diff guard and the reason the row count is asserted rather than inferred.
  - Compare the **name set** against `Theme.All()` in **both directions**, naming the specific offender in each failure message: a token in `All()` with no table row, and a table row naming a token that no longer exists.
  - Extract the fenced example-theme block and parse it through the **real loader** (Phase 1's parse + validate entry point, never a bespoke splitter in the test), asserting validity under §6.1 — all 19 keys present, every value a well-formed `#RRGGBB`. This is what stops the example being an unguarded fourth copy of the vocabulary (§15.3).
  - **Do not assert row order.** The guard compares a *name set*; the ramp's weight ordering is prose the guard deliberately does not check, because §2.7 makes ordering explicitly not a contract.
  - Factor the parse-and-compare into an unexported helper taking the doc bytes, so the vacuous-parse case and both drift directions are table-driven against synthetic content while the real doc is the one live case.
  - Lane **unit** — no tmux, no fixture, no built binary, and no `t.Parallel()` per the project rule.
- **Write `docs/theming.md`** following the `docs/custom-terminals.md` precedent (a user-authored config file with its own doc): an H1 naming the subject, a short opener saying what a theme is and that built-ins and drop-ins are the same kind of file, then the sections below.
- **Author the token table from §2.5**, 19 rows, grouped exactly as §2.5 groups them — text ramp, accents and states, surfaces — with each role's meaning written out. Use-sites are illustrative of the role, not an inventory; the panel's own surface assignments live in §9.1 and are deliberately not repeated.
- **Document the ramp bright → faint in weight order**, explicitly, as the doc's own statement rather than an implied reading of table order — this doc is the ordering's sole record.
- State per §2.5 that **`text.faint` is decorative only and never carries content a user must read**, which is precisely why §13.5 floors it *below* the UI threshold rather than above it.
- **The two pairing names must say what they sit on** — `text.on-selection` against `bg.selection`, `text.on-attention` against `bg.attention`. §2.3 keeps the pairing kind specifically because an author choosing those values needs to know the tint underneath; a table row that omits the tint destroys the only justification for the name.
- **`border` is one role** after the §2.2 consolidation (title rule, footer rule, modal panel frames, edit-modal chips). **`border.footer` must not appear anywhere in the doc.**
- Include the **complete copy-pasteable example theme** (§12.1's no-terminal on-ramp) as a fenced block: all 19 keys with real `#RRGGBB` values — the shipped `tokyo-night.theme` bytes are the obvious source and already parse.
- Write the meanings **here**, not as a cross-reference to the MV spec: task 10-7 downgrades the MV spec to design rationale and contrast rules, so a doc that points at it for meanings would point at a document that no longer claims to carry them.

**Acceptance Criteria**:
- [ ] `docs/theming.md` exists and its token table carries exactly 19 rows whose names equal `Theme.All()`'s set.
- [ ] The guard fails (not skips) when the doc file is absent.
- [ ] The guard fails when the table parse yields zero rows, with a message distinguishing that from a name mismatch.
- [ ] The guard fails when a token exists in `Theme.All()` but not in the table, naming the token.
- [ ] The guard fails when the table names a token absent from `Theme.All()`, naming the row.
- [ ] The guard parses the doc's example theme through the production loader and fails if it is not valid under §6.1.
- [ ] Reordering the table's rows does not fail the guard.
- [ ] The doc documents the ramp bright → faint in weight order in prose.
- [ ] The doc states `text.faint` is decorative-only and never carries content a user must read.
- [ ] Each `text.on-*` row names the tint it pairs against.
- [ ] `border.footer` appears nowhere in `docs/theming.md`; `border` is documented as one role covering the title rule, footer rule, modal frames and chips.
- [ ] The guard runs in the unit lane with no `t.Parallel()`, and `go test ./internal/theme` is green.

**Tests**:
- `"it fails when the doc is missing"` — `TestThemingDocGuard_MissingDocFails`
- `"it fails when the table parses no rows"` — `TestThemingDocGuard_ZeroRowsFailsLoudly`
- `"it matches every token name in Theme.All()"` — `TestThemingDocTokenTableMatchesAllTokens`
- `"it fails on a token missing from the doc"` — `TestThemingDocGuard_TokenAbsentFromTableFails`
- `"it fails on a doc row naming an unknown token"` — `TestThemingDocGuard_UnknownTableRowFails`
- `"it ignores row order"` — `TestThemingDocGuard_RowOrderIsNotAsserted`
- `"the example theme parses as a valid 19-key theme"` — `TestThemingDocExampleThemeIsValid`
- `"it fails when the example theme drops a key"` — `TestThemingDocGuard_ExampleMissingTokenFails`

**Edge Cases**:
- A table parse matching **zero rows must fail loudly** — a guard that silently parses nothing passes vacuously, the same trap §13.4 names, and the reason 19 is asserted rather than inferred.
- Drift must fail in **both directions** — a token in `Theme.All()` absent from the table, and a table row naming a token that no longer exists.
- The guard compares a **name set, not a sequence**; the ramp ordering is prose the guard deliberately does not assert (§2.7).
- The example theme is parsed **through the real loader**, so it must be valid under §6.1 — that is what stops it being a fourth unguarded copy of the vocabulary (§15.3).
- The ramp is documented **bright → faint**, and this doc is its sole record because the ordinal/qualitative join rests on convention rather than the names (§2.6).
- `text.faint` is **decorative only** and never carries content a user must read — the reason §13.5 floors it *below* the UI threshold.
- Both `text.on-*` names must state their tint, which is the whole justification for keeping a pairing name (§2.3).
- `border` is **one** role after the consolidation, and `border.footer` does not appear.
- A **missing doc fails rather than skips**; the doc resolves relative to the package dir.
- Lane **unit** — no tmux, no fixture, no `t.Parallel()`.

**Context**:
> §12.4: "**The 19-token vocabulary with each role's meaning** — the substance of §2.5. `docs/theming.md` is **the source of truth for the public contract.**… **The text ramp's weight ordering** — the sole record of it, since file ordering carries nothing (§2.7)."
> §13.5: "**`docs/theming.md` gets a guard.** It is now the sole record of the ramp's weight ordering and the 19 roles' meanings, with nothing keeping it honest — and this feature found the MV spec's '2-tone border' claim stale against the implementation purely by chance. Same drift class, same subsystem. **A test parses the doc's token table and compares the name set against `Theme.All()`** — cheap, and matching the codebase's existing guard idiom. The doc's copy-pasteable example theme is covered by the same guard: it must parse and contain all 19 keys, so it is not a fourth unguarded copy of the vocabulary."
> §2.5 is the role table to author from — text ramp (`text.primary` → `text.faint` plus `text.on-selection`), accents and states (`accent.primary`, `accent.key`, `accent.mode`, `accent.attention`, `state.positive`, `state.destructive`), surfaces (`canvas`, `bg.selection`, `bg.attention`, `bg.subtle`, `border`, `text.on-attention`).
> §2.5: "`text.faint` | Decorative only — inactive dots, `+ add`, mode indicator, hints. **Never carries content a user must read**; §13.5 floors it below the UI threshold precisely so it cannot."
> §15.3 gives the four locations and their standing: `docs/theming.md` is the source of truth for the public contract; the MV spec keeps design rationale; the doc's example theme is covered by the same guard; the embedded `.theme` files hold the values.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.4, §13.5, §2.5, §2.6, §2.7, §15.3

## theming-system-10-2

### Task 10.2: The file format and discovery half — writing a theme file and where it goes

**Problem**: §12.4 names discovery as "the drop-in author's first three questions, and currently the only part of the contract with no documented home" — where the file goes, what it must be called, and how Portal finds it. Every failure mode there is **silent by design**: an absent themes directory produces no doctor line and no log entry (§5.5), Portal never creates or seeds it, and a shell redirect will not create it either — so without the published `mkdir -p` line the first thing a new user meets is a redirect error (§12.1). Get the extension casing or the slug charset wrong and the file is rejected `bad name`; get the comment rule wrong — and **every value in a theme file starts with `#`**, so this is the one lexical rule an author will get wrong — and the file is rejected `bad syntax`. §5.5 fixed `PORTAL_THEMES_DIR`'s name in the specification *precisely so this doc could print it*.

**Solution**: Author the second half of `docs/theming.md` — the file format's lexical rules and value domain, the two vocabulary levers, the filename rules, the enumeration rules, the directory resolution chain, and the two-line drop-in workflow — as hand-maintained prose that stays out of the guarded token table's territory.

**Outcome**: A user can write a valid theme file, name it correctly, put it somewhere Portal will find it, and know what happens when they get any of it wrong — all from this doc, without reading the spec or the source.

**Do**:
- Add a **file format** section: `#` starts a comment **only at the beginning of a line** (after optional leading whitespace), so a `#` after `=` is part of the colour and there are no trailing comments. Call this out as the rule to watch, because every value begins with `#`.
- Document the rest of §4.2 in user terms: values are **bare** (a quoted value is rejected), **duplicate keys reject the file** rather than resolving to one of them, lines are trimmed at both ends, blank lines are ignored, keys are lowercase and matched **case-sensitively**, and a line that is neither blank, a comment nor a well-formed `key = value` pair rejects the file.
- Document the **value domain**: `#RRGGBB` only — no `#RGB` shorthand, no ANSI indices, no named colours. Give §4.3's reason briefly (Portal imposes its own exact hues via truecolor; an ANSI index has no fixed RGB, so it could never be contrast-checked).
- Document **both vocabulary levers together** (§4.6): an **unknown key is ignored**, key and value alike; a **missing key rejects the whole theme**, which is not selectable and falls back per §8.5 with a message naming what is missing. Say plainly that this means there is **no merge, no `inherits`/`parent`/`base`, and no partial files** — every theme declares all 19.
- Add a **filename rules** section, placed **before** the "where it goes" section, because these rules must be read before the file is written: the **filename minus its extension is the identity** (there is no in-file `name` field), the slug must match `^[a-z0-9][a-z0-9-]*$`, and the extension must be **exactly lowercase `.theme`**. Violations produce `bad name`, which both the panel and `portal doctor` render.
- State the **reject-never-normalise** rule concretely: `Nord.theme` is rejected, never lowercased to `nord` — because normalising would let a user file shadow a built-in, and a built-in is what Portal falls back to. A non-exact extension casing (`nord.THEME`, `nord.Theme`) is **enumerated so it is visible and then rejected**, so the file appears in the panel with its reason rather than vanishing.
- Add a **where it goes** section printing the chain **verbatim**: `PORTAL_THEMES_DIR` → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/`. Note the `_DIR` suffix is the mechanical difference from `PORTAL_PREFS_FILE` and its siblings — this resolves a *directory* where the others resolve *files*.
- Document the **enumeration rules** that decide whether a file is seen at all (§5.6): **top-level only**, no subdirectory recursion; **symlinked files are followed** and the slug derives from the link name; the resolved **themes directory itself may be a symlink and is followed**; an entry that resolves to a **directory is skipped silently** (whether or not a symlink is involved); a **dangling symlink** is enumerated and then reported `unreadable`.
- State that an **absent directory is silent** — zero drop-ins is not an error, there is no doctor line and no log entry, and Portal never creates or seeds it — while an **unreadable** one (or a regular file where a directory belongs) is a genuine misconfiguration that surfaces in doctor and in the panel's pinned row.
- Publish the **two-line drop-in workflow verbatim**, `mkdir -p` included:
  ```
  mkdir -p ~/.config/portal/themes
  portal theme export nord > ~/.config/portal/themes/nord-lee.theme
  ```
  The `mkdir -p` is **part of the workflow, not an omission** — say so, or a reader will drop it.
- Name `portal theme export <slug>` as the documented route to a built-in's bytes (comments included), and **do not describe `portal theme list` or a `--theme` flag** — §1.4 rules both out and neither ships.
- Keep this half out of the guarded table's territory: do not restate role meanings here, and do not add a second token list.

**Acceptance Criteria**:
- [ ] The comment rule is documented as line-start-only, with the "every value starts with `#`" reason stated.
- [ ] Bare values, duplicate-key rejection, whitespace trimming, case-sensitive keys and malformed-line rejection are all documented.
- [ ] The value domain is documented as `#RRGGBB` only, explicitly excluding `#RGB`, ANSI indices and named colours.
- [ ] Both vocabulary levers appear together — unknown key ignored, missing key rejects the whole theme — with "no merge, no inherits, no partial files" stated.
- [ ] The filename section precedes the directory section and carries the identity rule, the `^[a-z0-9][a-z0-9-]*$` charset and the exactly-lowercase `.theme` extension.
- [ ] `Nord.theme` is documented as rejected rather than lowercased, and a wrong-cased extension as visible-then-rejected.
- [ ] `PORTAL_THEMES_DIR` is printed verbatim with the full three-step chain and the `_DIR`-versus-`_FILE` note.
- [ ] The five enumeration rules are documented: top-level only, symlinked files followed, symlinked root followed, directory-valued entries skipped silently, dangling symlink → `unreadable`.
- [ ] An absent directory is documented as silent and never created; an unreadable one as a reported misconfiguration.
- [ ] The two-line workflow appears with `mkdir -p` and an explicit note that it is part of the workflow.
- [ ] `portal theme list` and a `--theme` flag appear nowhere in the doc.
- [ ] Task 10-1's guard is still green — the token table's 19 rows and the example theme are untouched by this edit.

**Tests**:
- `"the token table and example theme are unchanged by this edit"` — `TestThemingDocTokenTableMatchesAllTokens` and `TestThemingDocExampleThemeIsValid` (both from task 10-1) stay green.
- `"the published workflow works as written"` — manual walkthrough: run the two published lines against a scratch `PORTAL_THEMES_DIR`, confirm the export lands and the theme appears in the panel.
- `"a wrong-cased extension is visible, not invisible"` — manual: drop `nord.THEME` in the scratch directory and confirm `portal doctor` reports `extension must be lowercase .theme`.
- `"an illegal slug is reported as a name problem"` — manual: drop `-bad.theme` and confirm doctor reports the slug rule, not a missing file.
- `"a trailing comment is a bad colour, as documented"` — manual: append ` # note` to a value and confirm the file is rejected, matching what the doc says will happen.
- `"an absent themes directory is silent"` — manual: unset the directory and confirm zero doctor lines and zero log entries.

**Edge Cases**:
- This half has **no automated check and is maintained by hand** (§12.4) — it must not drift into the guarded token table's territory nor restate it.
- The comment rule is the one lexical rule an author will otherwise get wrong, because **every value starts with `#`**.
- `PORTAL_THEMES_DIR` is printed **verbatim**; §5.5 fixed the name in the spec precisely so this doc could print it, and the `_DIR` suffix marks a *directory* resolver among `_FILE` siblings.
- The **`mkdir -p` line is part of the published workflow, not an omission** — Portal never creates or seeds the directory and a redirect will not either.
- **Unknown key ignored, missing key rejects the whole theme** — both levers, or an author cannot tell that a partial file will not work and that there is no base to inherit from.
- Filename rules belong **before** the file is written, not after it is rejected.
- `Nord.theme` is **rejected, never lowercased**; a non-exact extension casing is enumerated-then-rejected so it is visible rather than invisible.
- Enumeration is top-level only, follows symlinked files and a symlinked directory root, and silently skips directory-valued entries.
- An **absent** directory is deliberately silent — zero drop-ins is not an error.
- The doc must not describe `portal theme list` or a `--theme` flag (§1.4); `portal theme export` is the documented route to a built-in's bytes.

**Context**:
> §12.4: "**Discovery: where a theme file goes, what it must be called, and how Portal finds it.** The drop-in author's first three questions, and currently the only part of the contract with no documented home… **The two-line drop-in workflow** from §12.1, `mkdir -p` included — Portal never creates or seeds the directory, so without that line the first thing a new user meets is a redirect error."
> §12.4: "The guard covers the **vocabulary half only**… The discovery half above has no automated check and is maintained by hand."
> §4.2: "`#` starts a comment **only at the beginning of a line**, after optional leading whitespace. There are no trailing comments, so the ambiguity never arises — a `#` after `=` is always part of the colour."
> §4.6: "**Unknown key → ignored.** This makes *removing* a token survivable: old files keep working. **Missing key → the whole theme is rejected.**"
> §5.6: "**The extension is matched case-insensitively for *enumeration*, but only the exact lowercase `.theme` is *accepted*.**… Enumerating case-insensitively is what stops the file being *invisible*, the 'completely in the dark' state §9.4 exists to prevent."
> §5.5's directory-state table: absent → **silent**, no doctor line, no log entry, never created or seeded; unreadable or a regular file in its place → a doctor advisory and a `theme: directory unusable` log entry.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.4, §4.1–§4.3, §4.5, §4.6, §5.1–§5.6, §12.1

## theming-system-10-3

### Task 10.3: Built-in slugs, the two-slot config and palette attribution

**Problem**: Three parts of the contract are undiscoverable anywhere else. **The reserved built-in slug set is invisible from the UI by construction** — §9.5 makes built-in rows deliberately indistinguishable from drop-in rows — so a user learns a slug is reserved only by having their file rejected (§5.4 accepts that consequence explicitly *because* the docs make the set discoverable outside the panel). **The two-slot setting has no in-app documentation either**: the panel writes it, but §9.9 accepts "no unset" — returning to the shipped pair after pinning means a hand-edit — on the stated grounds that `prefs.json` is hand-editable *and documented*, and §8.2's `theme`-wins rule exists only for a hand-edited file. And **Nord ships corrected**: two values do not clear Portal's floors against Nord's own canvas, and §7.4 resolves fidelity-versus-floors by shipping corrected values under the palette's own name **with `docs/theming.md` recording the corrections alongside the attribution**.

**Solution**: Author the third and final section set of `docs/theming.md` — the reserved built-in slugs with the rename workaround, the two-slot config with its states and rules, and attribution for the ported palettes including Nord's two corrections.

**Outcome**: A user can discover which slugs are reserved and what to do about it, configure a constant or an adaptive pair by hand with correct expectations about what wins and what is defaulted, and see honestly which shipped values differ from their upstream palette and why.

**Do**:
- Add a **built-in themes** section listing the three shipped slugs — `tokyo-night`, `tokyo-night-day`, `nord` — stating that these slugs are **reserved**: a drop-in whose slug collides is rejected with `reserved name`, decided from the slug alone before the file is even opened.
- Give §5.4's reason in one line (an invalid theme falls back to a built-in, so a user file must never be able to shadow the built-in that *is* the fallback) and name the workaround concretely: copy `nord` to `nord-lee.theme`. Pair it with §12.1's export line so the workaround is executable, not just described.
- Add a **choosing a theme** section documenting **two states, not three**:
  - **Constant** — `"theme": "nord"`; detection is never consulted.
  - **Adaptive** — `"theme_light"` / `"theme_dark"`; detection chooses.
  - "Nothing set" and "pair nominated" are **the same state**, because the shipped default *is* an implicit pair (`theme_light = tokyo-night-day`, `theme_dark = tokyo-night`).
- Document **mutual exclusion as Portal's write rule** — committing a constant clears both slots, assigning a slot clears the constant — and **`theme` wins** as the deterministic rule for a **hand-edited** file carrying both. Say plainly that the both-present state is unreachable from the UI; it exists only because the file is hand-editable.
- Document that **partial pairs do not exist**: setting `theme_dark = nord` alone leaves light on `tokyo-night-day`, because an unset slot holds the shipped default. There is no incomplete-pair state to explain or work around.
- Document **detection**: it follows the **terminal's background via OSC 11**, not the OS colour scheme, and **no answer resolves to dark**. Note the one accepted cost — a terminal that changes background mid-session is not noticed until the next launch.
- Document `prefs.json` as the **hand-editable home** for the setting, and record that returning to the shipped pair after pinning is a hand-edit rather than a panel key (§9.9's accepted "no unset" rests on this being documented).
- Add an **attribution** section: source and link per ported palette, nothing further — no per-theme licence line, no "(adapted)" naming convention, no PR contribution ceremony. Ported palettes keep their own names. State that attribution lives in the repo and docs and is **explicitly not in the UI** — no credits screen, nothing in the slide-over.
- Record **Nord's two corrections** by name with their reason: `state.destructive` and `state.positive` did not clear Portal's contrast floors against Nord's own canvas `#2E3440`, so both ship corrected (the green must clear the selection tint as well as the canvas). Keep it to the fact and the reason — **MV's §7.7 derivation figures stay as `#` comments in `tokyo-night-day.theme`**; this doc documents roles, not derivations.
- **Do not document the retained `appearance` key anywhere in this doc** — §10.4 keeps it on disk as a frozen legacy value for downgrade, and documenting it would invite users to set it in a binary that no longer reads it.
- Read the final shipped values from the `.theme` files when writing this section: if task 2-5's re-derivation moved anything, the files are the source of truth and §7.3's tables are superseded (§7.7).

**Acceptance Criteria**:
- [ ] The three built-in slugs are listed and documented as reserved, with `reserved name` named as the rejection reason.
- [ ] The rename workaround is given concretely (`nord` → `nord-lee.theme`) alongside the export line that produces it.
- [ ] The setting is documented as two states — constant or pair — with "nothing set" and "pair nominated" stated to be the same state.
- [ ] Mutual exclusion is documented as Portal's write rule and `theme`-wins as the hand-edit tiebreak, with the both-present state noted as unreachable from the UI.
- [ ] Partial pairs are documented as non-existent, with the `theme_dark = nord` example showing light still on `tokyo-night-day`.
- [ ] Detection is documented as terminal-background via OSC 11 with a dark no-answer fallback, and the not-live-following cost is stated.
- [ ] `prefs.json` is documented as the hand-editable home, and returning to the shipped pair is documented as a hand-edit.
- [ ] Attribution carries source and link only; no licence line, no "(adapted)" convention, no contribution ceremony; attribution is stated to be absent from the UI.
- [ ] Nord's two corrections are named with their reason and no derivation figures.
- [ ] The word `appearance` appears nowhere in `docs/theming.md`.
- [ ] Values quoted in this section match the shipped `.theme` files, not §7.3's tables, where the two differ.
- [ ] Task 10-1's guard remains green.

**Tests**:
- `"the doc's guard still passes after the final section"` — `TestThemingDocTokenTableMatchesAllTokens`, `TestThemingDocExampleThemeIsValid`.
- `"every documented built-in slug exports"` — manual: `portal theme export <slug>` succeeds for each of the three documented slugs and fails for a name not in the list.
- `"the documented reserved-name workaround works"` — manual: drop `nord.theme` and confirm the panel/doctor report `reserved name`; rename to `nord-lee.theme` and confirm it becomes selectable.
- `"the documented hand-edit rules hold"` — manual: hand-edit `prefs.json` with both a constant and a slot and confirm the constant wins; set only `theme_dark` and confirm light stays `tokyo-night-day`.
- `"no appearance mention"` — `grep -c appearance docs/theming.md` returns 0.

**Edge Cases**:
- The **reserved set is not discoverable from the panel** (built-in rows are deliberately indistinguishable from drop-ins), so this doc plus `portal theme export` is where a user learns it — §5.4 accepts that consequence on exactly this basis.
- `theme` wins is a documented rule for a **hand-edited** file only; Portal's own writes hold mutual exclusion, so the state is unreachable from the UI.
- **Partial pairs do not exist** — an unset slot holds the shipped default, so the shipped default and a partially-overridden pair are the same mechanism.
- Detection follows the **terminal background**, not the OS scheme; no answer resolves to **dark**.
- §9.9's accepted "no unset" rests on `prefs.json` being hand-editable **and documented** — so the hand-edit route must be stated, not implied.
- Nord's corrections are named with their **reason** only; MV's derivation figures stay as `#` comments in `tokyo-night-day.theme`, because export is byte-faithful and that is their only durable home.
- Attribution is a source and a link, **nothing further**, and lives in the repo/docs — **explicitly not in the UI**.
- The retained `appearance` key is **not documented as live** anywhere.
- If task 2-5's re-derivation moved any light value, the `.theme` files supersede §7.3's tables (§7.7) and the doc quotes the files.

**Context**:
> §5.4: "**Accepted consequence:** because built-in rows are deliberately indistinguishable from drop-in rows in the panel (§9.5), the reserved-slug set is not discoverable from the UI — a user learns a slug is reserved by having their file rejected with a message naming the conflict. `portal theme export` (§12.1) and `docs/theming.md` make the set discoverable outside the panel."
> §12.4: "**The two-slot config** — `theme` / `theme_light` / `theme_dark`, constant vs adaptive, mutual exclusion, the `theme`-wins hand-edit rule. **The reserved built-in slugs.** **Attribution for ported palettes** — source and link, plus the Nord corrections. Attribution lives in the repo and README, **explicitly not in the UI** (no credits screen, nothing in the slide-over)."
> §8.3: "**Partial pairs do not exist.** The adaptive form always has two slots and the shipped values are their *defaults*, so `\"theme_dark\": \"nord\"` yields `{light: tokyo-night-day, dark: nord}`."
> §7.4: "**Fidelity versus floors — resolved.** The floors win, and the corrected values ship under the palette's own name… The corrections are minimal and perceptually close, judged **visually**… and `docs/theming.md` records them alongside the attribution."
> §12.5: "**The retained `appearance` key is not documented as live.** §10.4 keeps it on disk as a frozen legacy value for downgrade, not as a setting to advertise — documenting it would invite users to set it in a binary that no longer reads it."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.4, §5.4, §8.1–§8.3, §8.7, §9.9, §7.2, §7.4, §10.4

## theming-system-10-4

### Task 10.4: README's four `appearance` sites and the config-file table

**Problem**: The README is Portal's front door and it describes, at **four places**, a setting this feature deletes. Worse, one of those four is *advice built on a premise that does not survive testing*: it tells users to pin `appearance` "when auto-detection misfires (for example under tmux passthrough)", and §8.7 retires that premise outright — OSC 11 works reliably through tmux, and the "detection is unreliable inside tmux" claim was the main argument for deleting the appearance axis in the first place. Leaving any of the four standing sends a reader to set a key the binary no longer reads. The config-file table is also now incomplete in a second way: the themes directory is a **directory** in a table whose every other member is a file, and §5.5 fixed `PORTAL_THEMES_DIR`'s name in the spec **precisely so the docs could print it**.

**Solution**: Replace all four `appearance` sites per §12.5 in one pass — delete the advice paragraph outright, rewrite the feature bullet and the TUI-views paragraph to the theme setting, re-point the `prefs.json` table row at the three theme keys, and add a themes-directory row carrying `PORTAL_THEMES_DIR` — with everything pointing at `docs/theming.md` rather than restating the contract.

**Outcome**: `grep -i appearance README.md` returns nothing; a reader learns detection follows the terminal background, that a constant `theme` pins it, where drop-in themes go, and where the full contract lives.

**Do**:
- **Site 1 — delete the advice paragraph.** The Configuration section's `**Appearance.**` paragraph (currently the one beginning "Portal paints its own light or dark canvas…" and ending with the `NO_COLOR` sentence) comes out **entirely** — it is obsolete twice over, both for the deleted setting and for the tmux-passthrough premise §8.7 retires. Put a `**Theme.**` paragraph in its place carrying the replacement: detection follows the **terminal's background**, no answer falls back to dark, a constant `"theme": "<slug>"` pins it, three built-ins ship, drop-ins live in the themes directory, and **`NO_COLOR` still disables the canvas and renders on the terminal's native colours** — with the note that the theme picker is blocked under `NO_COLOR`. Link `docs/theming.md` for the rest; do not restate the token vocabulary, the file format or the slug rules.
- **Site 2 — the feature bullet.** Rewrite "a colourful, keyboard-driven picker that owns its own light or dark canvas (auto-detected, or pinned via `appearance`, and honours `NO_COLOR`)" to the theme setting: the picker owns its canvas, ships three themes, follows the terminal's background or a pinned constant, takes drop-in theme files, and still honours `NO_COLOR`. This bullet is the discoverability surface §14 exists to widen, so naming the three built-ins and the `t` key here is in scope.
- **Site 3 — the TUI-views paragraph.** Rewrite "It paints its own light/dark canvas (set `appearance` in `prefs.json`, or `NO_COLOR` for a colourless render; see Configuration)" to the same shape — the canvas comes from the active theme, pick one with `t` in the picker or set it in `prefs.json`, `NO_COLOR` for a colourless render — pointing at Configuration and `docs/theming.md`.
- **Site 4 — the config-file table.** Rewrite the `prefs.json` row's purpose so it lists **`theme` / `theme_light` / `theme_dark` alongside** the existing grouping mode, and add a **themes-directory row** — `themes/`, "drop-in theme files (`<slug>.theme`); see [docs/theming.md](docs/theming.md)", env override **`PORTAL_THEMES_DIR`**. Note in the row (or immediately under the table) that this one resolves a *directory*, which is what the `_DIR` suffix marks against the table's `_FILE` siblings.
- Add a `t` row to the **TUI Keybindings** table (it already lists every other page key, `?` included) — one row, "Open the theme picker", no more.
- **Do not document the retained `appearance` key** anywhere, as live or otherwise (§12.5 / §10.4): it survives on disk purely so a downgraded binary honours the old pin, and advertising it would invite users to set it in a binary that no longer reads it.
- **Leave everything else byte-unchanged**: every unrelated table row, the whole Logging section including its `subsystem:` prefix examples (task 10-6 owns the component-count correction, and it lands in CLAUDE.md, not here), the `x` / `xctl` phrasing, and the Screenshots section's light-mode sentence.
- Check that every link added resolves — `docs/theming.md` must already exist from tasks 10-1 to 10-3, and in-page anchors (`#configuration`) must match the heading they name.

**Acceptance Criteria**:
- [ ] `grep -in appearance README.md` returns zero matches.
- [ ] The tmux-passthrough advice paragraph is gone, not reworded.
- [ ] A theme paragraph in Configuration states terminal-background detection, the dark no-answer fallback, the constant pin, drop-ins, and `NO_COLOR` (canvas suppressed, picker blocked), and links `docs/theming.md`.
- [ ] The feature bullet names the theme system, the three built-ins and `t`, and still names `NO_COLOR`.
- [ ] The TUI-views paragraph names `t` and `prefs.json` instead of `appearance`.
- [ ] The `prefs.json` table row lists `theme` / `theme_light` / `theme_dark` alongside the grouping mode.
- [ ] The table has a themes-directory row whose env override is `PORTAL_THEMES_DIR`, with the directory-not-file distinction stated.
- [ ] The TUI Keybindings table has a `t` row.
- [ ] The Logging section, every unrelated table row, and the `x`/`xctl` phrasing are byte-unchanged (`git diff` shows only the intended hunks).
- [ ] Every markdown link and in-page anchor added or touched resolves.
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` are unaffected (no test reads README).

**Tests**:
- `"no appearance mention survives"` — `grep -in appearance README.md` returns nothing.
- `"the themes directory row is present with its env var"` — `grep -n PORTAL_THEMES_DIR README.md` returns the table row.
- `"docs/theming.md is linked and exists"` — `grep -n "docs/theming.md" README.md` plus `test -f docs/theming.md`.
- `"the diff touches only the intended hunks"` — `git diff README.md` reviewed hunk by hunk against the four sites plus the keybinding row.
- `"no banned promises"` — `grep -in "portal theme list\|--theme" README.md` returns nothing (neither ships).
- `"links resolve"` — every relative link in the touched hunks opens.

**Edge Cases**:
- All **four** sites change together; one of them is a **deletion**, not a rewrite — the tmux-passthrough premise is retired (§8.7), so rewording it would carry a false claim forward.
- The replacement paragraph must not restate the contract — it points at `docs/theming.md`, which §15.3 makes the source of truth.
- The themes-directory row is a **directory** row in a file table; the `_DIR` suffix is what discriminates it, and §5.5 fixed the name so the docs could print it.
- The retained `appearance` key is **not documented as live** — it exists on disk for downgrade only.
- `NO_COLOR` survives untouched and is still documented, with the added note that the theme picker is blocked under it.
- The README **Logging** section's component list is deliberately out of scope here — task 10-6 owns the count, in CLAUDE.md.
- The three built-ins and the `t` key are the discoverability mention §14 exists to widen; keep it to a bullet and a keybinding row rather than a new section.

**Context**:
> §12.5's four-site table: the pin-it-when-detection-misfires paragraph → **deleted** ("Obsolete twice over — the premise was probably never true in the first place (§8.7)"); the feature bullet → "Rewritten to the theme setting: detection follows the terminal background, pinned by a constant `theme`"; the TUI-views paragraph → "Same rewrite"; the `prefs.json` config-file row → "Now lists `theme` / `theme_light` / `theme_dark` alongside the grouping mode. The table also gains a **themes directory** row carrying `PORTAL_THEMES_DIR`."
> §12.5: "README gains the theme setting in their place, pointing at `docs/theming.md`."
> §8.7: "**The 'detection is unreliable inside tmux' premise is retired.** It was the main argument for deleting the appearance axis entirely, it appears in the README, and it does not survive testing — OSC 11 works reliably through tmux."
> §9.10: "`t` is blocked under `NO_COLOR`, with a flash… Under `NO_COLOR` Portal paints no canvas, imposes no hues, and renders glyph-backed on the terminal's native fg/bg."
> §14 exists because the feature "would otherwise be near-invisible — `--theme` and `portal theme list` ruled out, the themes directory silent and never seeded, built-in rows indistinguishable from drop-ins."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.5, §8.7, §9.10, §10.4, §5.5, §14

## theming-system-10-5

### Task 10.5: The CHANGELOG upgrade note

**Problem**: The translation from `appearance` to a constant theme is **deliberately silent at runtime** — §10.5 refuses a flash, a notice band and a banner, because it runs at prefs load before any surface exists, intent is preserved exactly, and §6.3 has already refused the single-slot notice band a permanent seventh contender. The CHANGELOG is named as **the compensating channel**, and two other decisions lean on the entry existing: §10.4 keeps `appearance` on disk *because Homebrew downgrades are routine*, and §9.9 accepts "no unset" on the grounds that `prefs.json` is hand-editable **and documented**. An entry that omits either element silently weakens a decision made elsewhere — and a user who deliberately pinned `dark` gets no explanation of what happened to their setting.

**Solution**: One CHANGELOG entry for the release carrying all three mandated elements — the new setting plus the three built-ins pointing at `docs/theming.md`, the automatic translation requiring no user action, and the old key left in place for downgrade and not kept in sync.

**Outcome**: A user upgrading reads what the setting became, that they need do nothing, and what a downgrade will honour — from the file the project already publishes changes in.

**Do**:
- Add a new version entry at the top of `CHANGELOG.md`, above the current top entry, following the file's existing `## [X.Y.Z] - YYYY-MM-DD` heading idiom and its emoji section idiom (`✨ Added` / `🔧 Changed` / `🐛 Fixed` / `🗑️ Removed`). **Do not invent an `Unreleased` heading** — the file has never carried one.
- Take the version and date from the **existing release process** (the tag drives goreleaser); a minor bump is the shape that fits an additive feature with a replaced-but-translated setting. Do not invent a versioning convention.
- **`✨ Added`** — the theme system as a user-visible feature: the in-picker theme selector on `t` with live preview; the three built-ins **`tokyo-night`, `tokyo-night-day`, `nord`**; drop-in `.theme` files auto-discovered from the themes directory (`PORTAL_THEMES_DIR`); `portal theme export <slug>` for copying a built-in as a starting point; `portal doctor`'s theme advisories. **Point at `docs/theming.md`** from this section — that link is one of the three mandated elements.
- **`🔧 Changed`** — the upgrade note proper, in plain user language:
  - The canvas setting is now `theme` / `theme_light` / `theme_dark` in `prefs.json`, replacing `appearance`.
  - **`appearance` is translated automatically** — a pinned `light`/`dark` becomes the equivalent constant theme (`tokyo-night-day` / `tokyo-night`), `auto` needs nothing — **so a user who set it does not have to act.** Say that explicitly; it is the element the silence is compensating for.
  - The **old key is left in place** so an older binary still honours it after a downgrade, and it **is not kept in sync** with later theme changes.
  - Light/dark detection now follows the **terminal's background**, not the OS colour scheme.
  - The footer keymap revision (§14): `↑↓ navigate` dropped, `t theme` and `m multi` promoted into the footer.
- **Do not promise what does not ship**: no panel key that unsets a theme back to the shipped default, no `--theme` flag, no `portal theme list` (§1.4 rules out all three).
- Keep the entry user-facing — no internal refactors, no package moves, no test-suite changes; `capturetool`'s `--theme` flag is developer tooling and stays out.
- Re-read the three mandated elements against §12.5 before finishing and tick them off one by one; this is the acceptance test.

**Acceptance Criteria**:
- [ ] The entry sits at the top of `CHANGELOG.md` above the previous release, under a `## [X.Y.Z] - YYYY-MM-DD` heading matching the file's existing form; no `Unreleased` heading is introduced.
- [ ] Element 1: the new setting (`theme` / `theme_light` / `theme_dark`) and the three built-ins are named, with `docs/theming.md` linked.
- [ ] Element 2: `appearance` is stated as replaced and **translated automatically**, with the exact mapping and an explicit "you do not need to do anything".
- [ ] Element 3: the old key is stated as **left in place** for downgrade and **not kept in sync** afterwards.
- [ ] Detection is described as following the terminal background.
- [ ] The footer keymap revision is noted.
- [ ] No mention of an unset key, a `--theme` flag or `portal theme list`.
- [ ] Sections use the file's existing emoji idiom and Keep a Changelog structure.
- [ ] The entry contains no internal-only changes.

**Tests**:
- `"all three mandated elements are present"` — checklist read of the entry against §12.5's three bullets.
- `"no unshipped promises"` — `grep -in "theme list\|--theme\|unset" CHANGELOG.md` shows nothing in the new entry.
- `"the mapping is stated exactly"` — the entry names `light` → `tokyo-night-day`, `dark` → `tokyo-night`, `auto` → nothing.
- `"the heading matches the file's convention"` — diff the new heading's shape against the previous entry's.
- `"docs/theming.md is linked and exists"` — `test -f docs/theming.md` plus the link in the entry.
- `"no internal-only lines"` — read-through: every line names something a user can see or do.

**Edge Cases**:
- This is a **user-visible upgrade note, not a feature line** — it is the compensating channel for a translation that is deliberately silent at runtime.
- All **three** elements are mandated; omitting either of the last two silently weakens §10.4's downgrade decision or §9.9's accepted "no unset".
- The translation is **exact, not approximate** — a pinned mode becomes a pinned theme and detection stays off, just as it was.
- The retained key is **frozen legacy**: honoured by an older binary, never re-synced by a newer one.
- It must not promise a panel unset key, a `--theme` flag or `portal theme list` — none of which ship (§1.4).
- Version and date follow the **existing release process**; no `Unreleased` section is invented.

**Context**:
> §12.5: "**CHANGELOG.** This release needs a user-visible upgrade note, not just a feature line — two other decisions lean on the user knowing the setting changed shape. §10.4 keeps `appearance` on disk precisely because Homebrew downgrades are routine, and §9.9 accepts 'no unset' on the grounds that `prefs.json` is hand-editable *and documented*. The entry must therefore cover: The new theme setting (`theme` / `theme_light` / `theme_dark`) and the three built-ins, pointing at `docs/theming.md`. That **`appearance` is replaced and translated automatically** — a pinned `light`/`dark` becomes the equivalent constant theme, `auto` needs nothing — so a user who set it does not have to act. That the old key is **left in place** for downgrade, and is not kept in sync afterwards."
> §10.5: "**The translation is silent to the user at runtime.** No flash, no notice band, no banner — the log line is a forensic trail with **no user-facing interruption**… The compensating channel is the CHANGELOG (§12.5), which is required to carry that `appearance` is translated automatically and the user need not act — the honest place for a one-time upgrade notice."
> §10.4: "**Accepted:** the retained `appearance` is a **frozen legacy value** and is **not** kept in sync with later panel commits. A downgraded binary honours the user's old pin rather than their current choice."
> §14.2's decided footers, for the keymap line: Sessions `⏎ attach · / filter · ␣ preview · s switch view · x projects · t theme · m multi` + `? help`; Projects `⏎ new session · x sessions · e edit · / filter · t theme` + `? help`.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.5, §10.5, §10.4, §10.2, §9.9, §1.4, §14.2

## theming-system-10-6

### Task 10.6: CLAUDE.md's five remaining stale entries

**Problem**: §12.6 opens with the reason this matters: "CLAUDE.md is what an implementing agent reads first", and seven of its entries describe the pre-feature world. Phase 3 corrected the two most dangerous ones as it changed the code they describe — the `tui/theme` row (task 3-1) and the `tui` row with its standing *"do not drop this guard"* warning (task 3-3). **Five remain**, and each is a specific hazard rather than untidiness: the config-path section still advertises `WithAppearance` as the TUI's canvas wiring and knows nothing of a directory-shaped config path; the bootstrap-exempt set is **quoted verbatim** and therefore goes stale silently; the `prefs` row documents an `Appearance` enum that no longer exists while omitting that the raw `appearance` field must survive on disk — omit that and the next implementer deletes the field and **erases every user's pin on the first `s` keypress**, invisible until a downgrade; the logging section pins the taxonomy at 17 components, the exact number this feature moves; and the capture-harness section describes committed PNGs as a durable visual-verification asset, which §13.2 says they were never meant to be.

**Solution**: Five scoped edits to `CLAUDE.md`, leaving Phase 3's two corrected rows untouched.

**Outcome**: An agent reading CLAUDE.md first finds the themes directory, the non-migrating prefs read, `WithThemePersister`, `theme` in the exempt set, the prefs schema including the preserved raw `appearance`, an 18-component taxonomy, and a capture harness described as scaffolding with a `--theme` flag.

**Do**:
- **Do not re-edit the `tui/theme` row or the `tui` row.** Phase 3 tasks 3-1 and 3-3 own them; the `tui` row now carries the standing "do not drop this guard" warning re-anchored to the retained **startup** canvas hex, and any edit here risks contradicting it. Read both before starting, then leave them alone.
- **Config path resolution section** (`### Config path resolution (cmd/config.go)`):
  - Add `themesDirPath` and state it resolves a **directory**, so it is explicitly **not** a `configFilePath` member and has **no** one-shot macOS Application Support migration (the directory is new; nothing exists there to move).
  - Add `PORTAL_THEMES_DIR` to the env-var set.
  - Record that `cmd/config.go` exposes a **non-migrating** prefs read for `portal doctor` (so a diagnosis never triggers the one-shot `appearance` translation), alongside the migrating `loadPrefsStore` that owns the translation.
  - Update the TUI wiring clause: `WithThemePersister` joins `WithInitialMode` / `WithModePersister`; **`WithAppearance` is gone**, replaced by the loaded nomination handed to `tui.Build`.
- **Bootstrap-exempt set** (the `skipTmuxCheck` list quoted in the Server bootstrap section): add **`theme`**. Verify the documented list against `cmd/root.go`'s `skipTmuxCheck` map rather than trusting the prose — it is quoted verbatim, which is exactly why it goes stale silently.
- **`prefs` row** (internal-packages table): replace the `Appearance` enum, its tolerant decode and the `cmd/open.go` `WithAppearance` wiring with `theme` / `theme_light` / `theme_dark` / `theme_migrated` per §8.1, and **state that `appearance` survives on disk as a preserved raw string that is read but never parsed** — with the reason, because that clause is the only thing standing between the next implementer and erasing every user's pin. Keep the row's existing leaf constraint (no `internal/log`) intact.
- **Logging section**: move the closed taxonomy from **17 to 18** component names, adding `theme` in exactly the form `spawn` and `resolve` are already recorded (a spec-governed amendment, never a call-site invention), with its attr keys `slug` / `slot` / `reason` / `path` / `token` / `count` / `rejected`, and the note that it is legally emitted from **three packages** (`internal/theme`'s loader, `cmd/config.go`'s translation, the `cmd`-owned theme persister) under CLAUDE.md's own bind-once-**per-package** rule.
- **Visual capture harness section**: state that `testdata/vhs/` PNGs and tapes are **scaffolding, not a durable asset** — there is no visual-regression obligation — while the Go fixture definitions in `internal/capture` and the harness itself are **permanent** (the swap-and-diff guard drives the fixture renderer and its coverage assertion needs the fixture set to exist). Change the documented flag to **`--theme <slug|path>` (default `tokyo-night`)** in place of `--appearance dark|light`.
- Cross-check each edited clause against the code it describes before committing: `themesDirPath` and the non-migrating read in `cmd/config.go`, `WithThemePersister` in `internal/tui`, `theme` in `cmd/root.go`'s `skipTmuxCheck`, the component constants in `internal/log`, and `capturetool`'s flag set.
- **CLAUDE.md is a repo file, not a `.workflows/` artifact**, so the completed-unit correction protocol does **not** apply — no corrigendum block, no knowledge re-index, no scoped commit against another work unit.

**Acceptance Criteria**:
- [ ] The `tui/theme` and `tui` rows are byte-unchanged by this task's diff.
- [ ] The config-path section names `themesDirPath` as a directory resolver outside `configFilePath`, with no Application Support migration, and lists `PORTAL_THEMES_DIR`.
- [ ] The config-path section records the non-migrating prefs read and its purpose.
- [ ] The TUI wiring clause lists `WithThemePersister` and no longer mentions `WithAppearance` anywhere in the file.
- [ ] `theme` appears in the documented `skipTmuxCheck` set, and that set matches `cmd/root.go`'s map exactly.
- [ ] The `prefs` row documents the four keys and states `appearance` survives as a preserved raw string that is read but never parsed, with the reason.
- [ ] The logging section reads **18** component names, includes `theme` with its seven attr keys, and notes the three emitting packages under bind-once-per-package.
- [ ] The capture-harness section describes PNGs and tapes as scaffolding, fixtures and harness as permanent, and documents `--theme <slug|path>` (default `tokyo-night`).
- [ ] `grep -n "WithAppearance\|17 component\|--appearance" CLAUDE.md` returns nothing.
- [ ] No corrigendum block is added and no knowledge re-index is run for this file.

**Tests**:
- `"no stale appearance wiring survives"` — `grep -n "WithAppearance\|--appearance" CLAUDE.md` returns nothing.
- `"the component count moved"` — `grep -n "18 component names" CLAUDE.md` matches; `17 component` does not.
- `"the exempt set matches the code"` — the documented `skipTmuxCheck` list is diffed against `cmd/root.go`'s map keys, `theme` included.
- `"the themes dir is documented as a directory resolver"` — `grep -n "themesDirPath\|PORTAL_THEMES_DIR" CLAUDE.md` matches the config-path section.
- `"the prefs row keeps the raw-appearance warning"` — `grep -n "preserved raw string" CLAUDE.md` matches the `prefs` row.
- `"Phase 3's rows are untouched"` — `git diff CLAUDE.md` shows no hunk inside the `tui/theme` or `tui` rows.
- `"documented symbols exist"` — `grep -rn "WithThemePersister" internal/tui`, `grep -rn "themesDirPath" cmd`, `capturetool --help` shows `--theme`.

**Edge Cases**:
- Phase 3 already corrected the **`tui/theme`** row (task 3-1) and the **`tui`** row (task 3-3) — this task must neither re-edit nor contradict either, and the `tui` row's "do not drop this guard" warning now points at the retained **startup** canvas hex.
- The `skipTmuxCheck` set is **quoted verbatim** in CLAUDE.md, which is exactly how it goes stale without a compiler signal — verify against `cmd/root.go`.
- `themesDirPath` resolves a **directory**, so it is explicitly not a `configFilePath` member and has **no** one-shot macOS migration.
- The `prefs` row **must** record that `appearance` survives as a preserved raw string read but never parsed — omitting it is what leads the next implementer to delete the field and erase every user's pin on the first `s` keypress, invisible until a downgrade.
- The component count is stated at all *because* it is the amendment marker — 17 → 18 in the same form `spawn` and `resolve` are recorded.
- `theme` is legally emitted from **three** packages under bind-once-**per-package**, exactly as `spawn` and `bootstrap` already span several files.
- The capture PNGs are **scaffolding, not a durable asset**, while the Go fixture definitions and the harness are **permanent** — conflating the two would license deleting fixtures and silently shrinking the swap-and-diff guard.
- CLAUDE.md is a **repo file**, so no corrigendum, no re-index, no scoped commit.

**Context**:
> §12.6's table, for the five entries this task owns: **Config path resolution** — "§3.2 adds `themesDirPath` (a *directory*, not a `configFilePath` member), §5.5 adds `PORTAL_THEMES_DIR`, §10.5 adds the non-migrating read variant, §8.9 adds `WithThemePersister`, and §8.8/§8.4/§13.3 delete `WithAppearance` in favour of the loaded nomination"; **the bootstrap-exempt set** — "Lists `skipTmuxCheck` verbatim. §12.1 adds `theme`"; **the `prefs` row** — "Replaced by `theme` / `theme_light` / `theme_dark` / `theme_migrated` per §8.1 and §8.8 — noting that `appearance` survives on disk as a preserved raw string (§8.8)"; **the logging section** — "Pins the taxonomy at '17 component names'. §12.3 adds an 18th (`theme`) with its own attr keys — the same shape of amendment `spawn` and `resolve` carried, which is why the count is stated at all"; **the visual capture harness section** — "Describes `testdata/vhs/` as committed reference PNGs forming a visual-verification harness, which reads as a durable asset. It is not (§13.2). The `capturetool` flag description also changes with §13.3."
> §8.8: "**`prefsFile` keeps a raw `appearance string` field, so the on-disk value round-trips.** This is load-bearing, not tidiness… Delete the field and the first `s`-keypress or theme commit after upgrade silently erases the user's `appearance` pin, defeating §10.4's downgrade guarantee at the moment the user is least likely to notice."
> §13.2: "**The deletion covers images and tapes, NOT fixtures.** The Go fixture *definitions* in `internal/capture` and the harness itself are **permanent** — the swap-and-diff guard drives the fixture renderer and its coverage assertion needs the fixture set to exist."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.6, §3.2, §5.5, §8.1, §8.8, §8.9, §10.5, §12.1, §12.3, §13.2, §13.3

## theming-system-10-7

### Task 10.7: Amend the Modern Vivid specification per §15.2 and §15.1's keymap revision

**Problem**: The `spectrum-tui-design` specification is a **completed** work unit whose knowledge-base chunks stay live at full confidence, so every wrong claim in it is re-served as validated context to future queries. This feature falsifies several: §2.1/§2.9 describe a closed set of **~20** tokens "each with a Light and Dark variant" measured against **two hardcoded canvases**, with `border.footer` as a distinct row and hue-named tokens (`accent.violet`, `state.green`) that no longer exist; §8.1 claims modals are framed by a **2-tone border (`border.separator` + `border.footer`)**, which the MV *implementation* already dropped before this feature started (§2.2 found it stale by chance); and §12.2's keymap revision predates §14's footer changes. Left standing, the file that governs Portal's design vocabulary describes a vocabulary the code no longer has.

**Solution**: Run the completed-unit correction protocol against `.workflows/spectrum-tui-design/specification/spectrum-tui-design/specification.md` — in-place edits to the affected sections, an extension of the file's existing corrigendum block, a knowledge re-index, and a commit scoped to the owning unit.

**Outcome**: The MV spec states the 19-token vocabulary, single-value tokens measured against each theme's own `canvas`, a single `border` role, and the revised keymap — with a corrigendum recording each correction and the knowledge store serving the corrected content.

**Do**:
- **Confirm status first**: `node .claude/skills/workflow-engine/scripts/engine.cjs manifest get spectrum-tui-design status` → expect `completed`. The confirmation prompt in the protocol is skipped **only because this approved task names the steps**; if the status is anything other than `completed`, stop and follow the protocol's branch for that status instead.
- **Edit in place** — the live file is current truth and git history is the historical record, so **no "was X, now Y" residue survives in the section bodies**:
  - **§2.1 / §2.9** — the §2.4 renames (hue and use-site names → weight-and-meaning names), the count **~20 → 19**, the dropped **`border.footer`**, the removal of `Token.ColorFor` and `theme.Mode`, and the retirement of the two-hardcoded-canvas framing in favour of **each theme carrying its own `canvas`** against which contrast is measured.
  - **§2.9's pinned hex table** must stop reading as the **source of truth for values** — those live in the embedded `.theme` files (§15.3). Keep the design rationale and the contrast rules; re-frame the table as the MV palette's record rather than the authority.
  - **§8.1** — the "2-tone border (`border.separator` + `border.footer`)" claim becomes the single `border` role. Note in the corrigendum that this was **already stale against the implementation before this feature** — a factual correction, not a change this feature introduces.
  - **§12.2** — §14's revision: `↑↓ navigate` **dropped** from the footer, `t` and `m` **promoted to core**, the two decided footer rows recorded, and the label **`m multi`, not `m multi-select`**.
- **§15.4 stands unchanged**: the MV Paper frames are historical, not specification, so a frame still showing `↑↓ navigate` is **not a defect** and is not updated.
- **Sweep the rest of the file for the same falsified facts before editing**, because the protocol's rule is to correct a specification wherever it is wrong and §15.2 names the **minimum** set, not a ceiling. Known further candidates: §2.6's `appearance: auto | light | dark` override (the pref this feature deletes), §2.8's "Deferred to its own initiative: a user-overridable theme system…" paragraph (delivered by this feature) including its `border.separator` + `border.footer` example, §14's role-layer description ("each with light + dark variants" plus the `appearance` pref), and §16's scope-boundary bullet naming the `appearance` pref. **Surface the final list at the gate before editing** so the corrigendum entries are agreed rather than assumed — the specification names three sections and these are additional.
- **Corrigendum block** — the file already carries **two** entries beneath its title, so **extend** the block rather than replacing it, one entry per correction, in the protocol's form:
  `> **Corrigendum {YYYY-MM-DD}** (from `theming-system`): {original claim, quoted} — corrected: {what is true}.`
  Use the date the edit is made.
- **Re-index** — part of the task, never an afterthought; without it the store serves the old content indefinitely:
  `node .claude/skills/workflow-knowledge/scripts/knowledge.cjs index .workflows/spectrum-tui-design/specification/spectrum-tui-design/specification.md`
- **Commit, scoped to the owning unit** (the knowledge store rides along):
  `node .claude/skills/workflow-engine/scripts/engine.cjs commit spectrum-tui-design -m "specification(spectrum-tui-design): corrigendum from theming-system"`
- **This is a specification stating facts**, so it is corrected rather than de-indexed — the protocol's discussion-chunk-removal branch does not apply. The owning unit's manifest is never touched: no reopen, no status change.

**Acceptance Criteria**:
- [ ] `manifest get spectrum-tui-design status` was run and returned `completed` before any edit.
- [ ] §2.1/§2.9 state 19 tokens with the §2.4 names, single values, and contrast measured against the theme's own `canvas`; no `Light`/`Dark` variant framing and no `Token.ColorFor`/`theme.Mode` survive.
- [ ] `border.footer` appears nowhere in the file's body; §8.1 describes a single `border` role.
- [ ] §2.9's value table no longer reads as the source of truth for values, and points at the embedded `.theme` files.
- [ ] §12.2 carries §14's revision including the two decided footer rows and the `m multi` label.
- [ ] §15.4 is byte-unchanged.
- [ ] The corrigendum block is **extended** with one entry per correction, each quoting the original claim and stating what is true, dated the day of the edit.
- [ ] No "was X, now Y" residue survives in any edited section body.
- [ ] The re-index command was run against the artifact path and succeeded.
- [ ] The commit is scoped to `spectrum-tui-design` with the protocol's message form, and the owning manifest's status is still `completed`.
- [ ] Any correction beyond §15.1/§15.2's named sections was surfaced and agreed before being made.

**Tests**:
- `"the owning unit is completed"` — `engine.cjs manifest get spectrum-tui-design status` prints `completed`.
- `"no dropped token name survives"` — `grep -n "border.footer\|accent.violet\|accent.cyan\|state.green\|state.red\|text.dim\|text.detail\|bg.track\|bg.warning" <spec>` returns only intentional historical quotes inside the corrigendum block.
- `"no removed API survives"` — `grep -n "ColorFor\|theme.Mode" <spec>` returns nothing outside the corrigendum.
- `"the keymap revision landed"` — `grep -n "m multi" <spec>` matches and `↑↓ navigate` is absent from §12.2's footer description.
- `"the corrigendum grew by one entry per correction"` — count entries before and after.
- `"the re-index replaced the chunks"` — `knowledge.cjs index <path>` reports the file's chunks replaced; a query for the corrected claim returns the new text.
- `"the commit is scoped"` — `git log -1 --stat` shows only the owning unit's artifact plus `.workflows/.knowledge`.

**Edge Cases**:
- The owning unit is **completed**, so the **full four-step protocol applies**; the confirmation prompt is skipped only because this approved task names the steps.
- The file **already carries two corrigendum entries** — extend the block, one entry per correction, never replace it.
- Wrong claims are **replaced in the body**; git history is the historical record, so no "was X, now Y" residue survives.
- §8.1's 2-tone-border claim was **already stale before this feature** — record it as a factual correction, not as a change this feature introduces.
- §15.4 **stands unchanged**: MV's Paper frames are historical, so a frame showing `↑↓ navigate` is not a defect.
- §2.9 keeps its design rationale and contrast rules but stops being the **source of truth for values** (§15.3).
- The **re-index is part of the task** — skipping it leaves the store serving the old content at full confidence indefinitely.
- The commit is **scoped to the owning unit** and must never be folded into tasks 10-8's or 10-9's.
- This is a specification, so it is **corrected, not de-indexed** — the discussion-chunk-removal branch does not apply.
- §15.2 names the minimum set; further falsified clauses in the same file (§2.6's `appearance` override, §2.8's deferred-theming paragraph, §14's role-layer description, §16's scope bullet) are the same class of error and must be raised explicitly rather than swept or ignored.

**Context**:
> §15.2's amendment table: **§2.1 / §2.9** — "The token renames (§2.4), 20 → 19, the dropped `border.footer`"; **§2.9** — "The removal of `Token.ColorFor` and `theme.Mode`; the two-hardcoded-canvas framing goes — each theme carries its own `canvas` token and contrast is measured against it"; **§8.1** — "The stale '2-tone border (`border.separator` + `border.footer`)' claim, which the implementation already dropped."
> §15.1: "**MV spec §12.2 — the keymap revision.** The footer changes of §14 above."
> §15.4: "**Modern Vivid is already implemented, so the code is the source of truth.** The MV Paper frames are historical reference from that feature's design phase; a footer in them that no longer matches (e.g. still showing `↑↓ navigate`) is **not a defect** and is not worth updating."
> Correction protocol, completed branch: "**Edit in place.** Replace the wrong claims in the affected sections with corrected content. The live file is current truth; git history is the historical record — never keep wrong content in the body for posterity." / "**Corrigendum block.** At the top of the file, directly beneath the title, add (or extend, one entry per correction)" / "**Re-index.** Replaces the file's existing chunks in one idempotent call" / "**Commit.** Scoped to the owning unit."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §15.1, §15.2, §15.3, §15.4, §2.2, §2.4, §14; protocol: `.claude/skills/workflow-shared/references/correcting-historical-artifacts.md`

## theming-system-10-8

### Task 10.8: Amend `portal doctor`'s contract — two line classes and the closing summary

**Problem**: §15.1 lists three amendments this feature carries, and the phase's acceptance bullet names only "the Modern Vivid specification" — but **doctor's contract has never lived in the MV spec**. It lives in the `cli-verb-surface-redesign` specification's "Exit-code contract" section, which states `portal doctor` "exits **0 iff every check passes; non-zero (1) if any check reports a problem**". An amendment aimed at MV would land nowhere and this decision would go unowned. Left unamended the claim is actively harmful: theme advisories are read-only with **deliberately no repair path**, so a stray junk file in `themes/` would hold the diagnostic red **permanently**, unlike every other check which is either `--fix`-repairable or indicates genuine runtime breakage — meaning an automated health check fires about the daemon because someone left a half-written palette lying around.

**Solution**: Run the completed-unit correction protocol against `.workflows/cli-verb-surface-redesign/specification/cli-verb-surface-redesign/specification.md`, amending the exit-code contract into two classes of line plus a closing summary — with **its own** corrigendum, **its own** re-index and **its own** scoped commit.

**Outcome**: The specification that owns doctor's contract states that Portal-health checks drive the exit code, `⚠`-marked user-content diagnostics do not, and a closing summary distinguishes the counts — so the exit code's meaning is legible without reading the contract.

**Do**:
- **Confirm status first**: `node .claude/skills/workflow-engine/scripts/engine.cjs manifest get cli-verb-surface-redesign status` → expect `completed`. Follow the protocol's other branch if it is not.
- **Correct the exit-code sentence in place.** The `### Exit-code contract` bullet reading "exits **0 iff every check passes; non-zero (1) if any check reports a problem**" becomes exit **0 iff every Portal-health check passes** — with the second class named as not participating. Do not leave the old sentence standing beside the new one.
- **State the two classes explicitly**, as a table or a pair of bullets: **Portal-health checks** carry the existing pass/fail markers and drive the exit code as today; **user-content diagnostics** carry **`⚠`** (Portal's established warning glyph, glyph-backed so it survives colourless) and **do not** drive it.
- **Give the reason in the spec's own terms**, because it is what makes the amendment defensible rather than a loosening: there is deliberately no repair path for a theme, so a failing theme line would go permanently non-zero until someone hand-edits a file; the exit code exists as a signal about the **resurrection machinery** — daemon alive, hooks registered, state sane — and a stray file in `themes/` is not that.
- **Record the closing summary as a new line on every run**: `<N> checks passed`, or `<N> of <T> checks passed` when any fails, plus ` · <M> advisory|advisories` when advisories are present and **suppressed entirely at M=0**. State that `<N>`/`<T>` count **Portal-health checks only** and `<M>` counts advisory **lines** — problems, not detections. Today's report is a header plus one line per check with nothing trailing, so **the amendment records an added line rather than a changed one** — that is what makes it an amendment and not a regression.
- **Do not re-litigate or merge the host-terminal line.** The spec already places it *outside* the pass/fail set as informational; the new class must sit **beside** it, not absorb it — they are different ideas (an environmental state versus user content) that happen to share the property of not driving the exit code.
- **Record `--fix`'s behaviour**: the theme scan runs on the `--fix` path too and its advisories and the ` · <M>` suffix appear in **both** renders (the initial diagnosis and the post-repair re-diagnosis); the exit code stays driven **solely** by the post-repair health checks; there is no theme repair, so `--fix` gains no theme step.
- **Write the class to admit later members.** Theme advisories are the **first** member of the user-content class, not its definition — a reader must be able to add the second without re-amending the contract.
- **Corrigendum block** — the file currently has none, so **create** it directly beneath the title in the protocol's form, one entry per correction:
  `> **Corrigendum {YYYY-MM-DD}** (from `theming-system`): {original claim, quoted} — corrected: {what is true}.`
- **Re-index**: `node .claude/skills/workflow-knowledge/scripts/knowledge.cjs index .workflows/cli-verb-surface-redesign/specification/cli-verb-surface-redesign/specification.md`
- **Commit, scoped to this owning unit and never folded into task 10-7's**: `node .claude/skills/workflow-engine/scripts/engine.cjs commit cli-verb-surface-redesign -m "specification(cli-verb-surface-redesign): corrigendum from theming-system"`

**Acceptance Criteria**:
- [ ] `manifest get cli-verb-surface-redesign status` was run and returned `completed` before any edit.
- [ ] The "0 iff every check passes" sentence is corrected in place to scope the exit code to Portal-health checks; the old wording does not survive in the body.
- [ ] Both classes are named with their markers, and the `⚠` class is stated as glyph-backed and non-exit-code-driving.
- [ ] The reason is recorded — no repair path, so a failing theme line would be permanently red, and the exit code signals the resurrection machinery.
- [ ] The closing summary is documented in both forms with the ` · <M> advisory|advisories` suffix and its M=0 suppression, and is described as a **new** line on every run.
- [ ] `<N>`/`<T>` are stated to count Portal-health checks only; `<M>` counts lines.
- [ ] The host-terminal informational line stays as it is, beside the new class rather than merged into it.
- [ ] `--fix` is documented as carrying advisories in both passes with the exit code driven solely by post-repair health checks.
- [ ] The class is written to admit later members rather than being defined as "theme".
- [ ] A corrigendum block is created beneath the title with one entry per correction, dated the day of the edit.
- [ ] The re-index ran against this artifact and the commit is scoped to `cli-verb-surface-redesign`, separate from tasks 10-7 and 10-9.

**Tests**:
- `"the owning unit is completed"` — `engine.cjs manifest get cli-verb-surface-redesign status` prints `completed`.
- `"the old exit-code claim is gone"` — `grep -n "0 iff every check passes" <spec>` matches only inside the corrigendum quote.
- `"both classes are documented"` — `grep -n "advisor" <spec>` returns the new class, the summary suffix and the `--fix` clause.
- `"the host-terminal line is untouched"` — `git diff` shows no hunk in the informational-line bullet.
- `"the corrigendum exists and is dated"` — the block sits directly beneath the title with today's date.
- `"the re-index replaced the chunks"` — `knowledge.cjs index <path>` succeeds and a query returns the corrected exit-code claim.
- `"the commit is scoped and separate"` — `git log` shows a distinct commit for this unit, not shared with `spectrum-tui-design` or `portal-observability-layer`.

**Edge Cases**:
- The owning artifact is the **`cli-verb-surface-redesign`** specification, **not** the MV spec — §15.1 groups all three amendments under one heading, but an amendment aimed at MV would land nowhere.
- Its unit is **completed**, so the same four-step protocol applies with **its own** corrigendum, re-index and scoped commit — never folded into task 10-7's.
- Left unamended, a stray junk file in `themes/` holds the diagnostic red **permanently**, because there is deliberately no repair path.
- The **host-terminal informational line is already outside the pass/fail set** — the new class sits beside it and the two ideas must not be merged.
- The **closing summary is a new line on every run** — an added line, not a changed one, which is what makes this an amendment rather than a regression.
- `<M>` counts **lines** — problems, not detections — which is what doctor's one-slug-one-line rule keeps true.
- `--fix` carries advisories in **both** passes; the exit code stays driven solely by post-repair health checks.
- Theme advisories are the **first** member of the second class, so the amendment is written to admit later members rather than defining the class as "theme".

**Context**:
> §12.2: "**Theme lines are advisory and do NOT drive the exit code — this amends doctor's contract.**… Because there is deliberately no repair path, a failing theme line would go **permanently** non-zero until someone hand-edits a file — unlike every other check, which is either `--fix`-repairable or indicates genuine runtime breakage. The exit code exists as a signal about the **resurrection machinery** — daemon alive, hooks registered, state sane. A stray junk file in `themes/` is not that."
> §12.2: "So doctor gains **two classes of line**… **Portal-health checks** | existing pass/fail markers | Yes, as today · **User-content diagnostics** | **`⚠`** — Portal's established warning glyph (MV §2.2, glyph-backed so it survives colourless) | **No**… **Doctor's closing summary distinguishes the two counts** — e.g. *'N checks passed · 2 advisories'* — so the exit code's meaning is legible without reading the contract."
> §12.2: "**The theme scan runs on the `--fix` path too**, and its advisories and the `· <M> advisories` suffix appear there. `--fix` re-diagnoses after repairs and the theme lines are read-only in both passes."
> §14A: "`<N>` and `<T>` count **Portal-health checks only** — the class that drives the exit code (§12.2). Advisories are counted separately by `<M>` and never fold into either… The summary line is **new**: today's report is a header plus one line per check with no trailing summary, so every run gains a line — that is the amendment §15.1 names, not a regression."
> §15.1: "**The `portal doctor` contract.** Two classes of line — Portal-health checks driving the exit code, user-content diagnostics carrying `⚠` and not driving it — plus a closing summary distinguishing the counts (§12.2)."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §15.1, §12.2, §14A; protocol: `.claude/skills/workflow-shared/references/correcting-historical-artifacts.md`

## theming-system-10-9

### Task 10.9: Amend the log-component vocabulary with the `theme` component

**Problem**: The `portal-observability-layer` specification's own extension policy states that **"New components require explicit amendment of THIS specification's closed component list"** and that "spec writers and code reviewers MAY NOT introduce new component or attr names ad hoc" — so adding `theme` to CLAUDE.md alone would be exactly the ad-hoc invention the policy forbids. Worse, the file is already behind reality: **its closed list still reads 15 components**, meaning `spawn` and `resolve` shipped under amendments that never landed here. Correcting it to name `theme` while leaving the count at a number that contradicts CLAUDE.md's 18 would replace one wrong fact with another.

**Solution**: Run the completed-unit correction protocol against `.workflows/portal-observability-layer/specification/portal-observability-layer/specification.md`, adding the `theme` component with its owners, attr keys and event catalogue, and reconciling the stale component count — with its own corrigendum, re-index and scoped commit.

**Outcome**: The specification that owns the closed taxonomy declares `theme` with its levels, cadences, attr keys and multi-package emission, and its stated counts agree with the code and with CLAUDE.md.

**Do**:
- **Confirm status first**: `node .claude/skills/workflow-engine/scripts/engine.cjs manifest get portal-observability-layer status` → expect `completed`.
- **Add `theme` to the closed component value space** — both the fenced name block and the "Component | Owns" table — with its owner description spanning the three emitting sites: `internal/theme`'s loader, `cmd/config.go`'s `appearance` translation, and the `cmd`-owned theme persister.
- **Reconcile the count.** The list reads **15** and CLAUDE.md reads **18**: `spawn` and `resolve` are absent. Add them alongside `theme` so the stated count is true — reconciling a stale factual count is squarely what the protocol is for, and a correction that silently claims a count contradicting CLAUDE.md would be a fresh wrong claim. Verify the resulting membership against `internal/log`'s component constants rather than against either document.
- **Carry the seven attr keys** — `slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected` — into the closed attr-key space. **Do not add them blind**: `reason` already exists in the lifecycle group and `path` in the contextual group, so only the genuinely-new keys are added and the stated key count must be computed from the **deduplicated** set. Verify each against the file; a mechanically-added total that double-counts an existing key is a new wrong claim of exactly the kind this task exists to remove.
- **Raise, do not sweep, the adjacent attr-key gap.** `spawn`'s and `resolve`'s attr keys are undeclared here for the same reason their components were — the same class of stale fact. Surface it at the gate with a recommendation and act on the agreed answer; do not fold it in silently and do not leave it unmentioned.
- **Carry the event catalogue** with its levels and cadences: `theme: loaded` (INFO, one line per nominated theme at construction, plus the commit-time load, plus the fallback), `theme: enumerated` (INFO, every panel open, `count` = rows produced, `rejected` = unselectable rows), `theme: rejected` (WARN, one per rejected file, deduplicated per process on `slug`+`reason` or `path`+`reason` where there is no slug), `theme: directory unusable` (WARN, deduped on `path`+`reason`), `theme: fallback applied` (WARN, deduped on `slug`+`reason`), `theme: appearance migrated` (INFO, one-shot, on successful persist only), `theme: commit failed` (WARN, per failed write).
- **State the binding rule correctly**: the component is emitted from **more than one package**, which is legal under **bind-once-*per-package*** exactly as `spawn` and `bootstrap` already are — the spec's "every package that logs binds its component name once at package init" must not be left reading as one-package-per-component.
- **Record why it earns a place**: `prefs` and `terminals` stay deliberately outside the vocabulary as dumb stores with no runtime behaviour, whereas the theme loader has parse/validate/fallback **outcomes**.
- **Record the two behavioural rules** that are part of the amendment rather than call-site detail: **rejections are WARN, not INFO** ("your config did not work" is a warning in a log, even though doctor treats it as advisory for exit-code purposes), and the component records where a theme is **used**, never where one is **diagnosed** — so `portal doctor` and `portal theme export` emit **nothing at all**. Note that `token`'s only consumer is `theme: rejected`, and that the per-process dedup state lives on the **injected logger** rather than in package state.
- **Corrigendum block** — create it beneath the title (the file has none), one entry per correction: the added component, and the reconciled count naming `spawn` and `resolve` as amendments that never landed.
- **Re-index**: `node .claude/skills/workflow-knowledge/scripts/knowledge.cjs index .workflows/portal-observability-layer/specification/portal-observability-layer/specification.md`
- **Commit, scoped and separate**: `node .claude/skills/workflow-engine/scripts/engine.cjs commit portal-observability-layer -m "specification(portal-observability-layer): corrigendum from theming-system"`

**Acceptance Criteria**:
- [ ] `manifest get portal-observability-layer status` was run and returned `completed` before any edit.
- [ ] `theme` appears in both the fenced component block and the "Component | Owns" table, with its three emitting sites named.
- [ ] The stated component count is true of the listed membership and agrees with CLAUDE.md's 18 and with `internal/log`'s constants.
- [ ] `spawn` and `resolve` are present, with the corrigendum recording that their amendments never landed.
- [ ] The seven theme attr keys are reflected without duplicating `reason` or `path`, and the stated key count matches the deduplicated set.
- [ ] The full seven-event catalogue is recorded with each event's level and cadence, including every dedup key.
- [ ] The multi-package emission is stated as legal under bind-once-per-package, and the binding rule no longer reads as one package per component.
- [ ] The spec records that rejections are WARN, that the component never records diagnosis (doctor and export emit nothing), that `token`'s only consumer is `theme: rejected`, and that dedup state lives on the injected logger.
- [ ] `prefs` and `terminals` remain documented as deliberately outside the vocabulary.
- [ ] A corrigendum block is created with one entry per correction, dated the day of the edit.
- [ ] The re-index ran against this artifact and the commit is scoped to `portal-observability-layer`, separate from tasks 10-7 and 10-8.
- [ ] The `spawn`/`resolve` attr-key gap was surfaced explicitly and resolved by decision, not by silence.

**Tests**:
- `"the owning unit is completed"` — `engine.cjs manifest get portal-observability-layer status` prints `completed`.
- `"the component is declared"` — `grep -n "theme" <spec>` returns both the fenced block and the owns-table row.
- `"the count is true of the list"` — count the names in the fenced block and compare with the stated total and with `internal/log`'s constants.
- `"no attr key is double-counted"` — `reason` and `path` appear once each; the stated key count equals the deduplicated membership.
- `"the catalogue is complete"` — all seven events present with level and cadence, dedup keys included.
- `"the corrigendum records both corrections"` — the added component and the reconciled count each have an entry.
- `"the re-index replaced the chunks"` — `knowledge.cjs index <path>` succeeds and a query returns the corrected list.
- `"the commit is scoped and separate"` — `git log` shows a third distinct commit, not shared with the other two units.

**Edge Cases**:
- The spec's **own text** requires this amendment — "new components require explicit amendment of THIS specification's closed component list" — so CLAUDE.md alone would be the ad-hoc invention the policy forbids.
- **Its list still reads 15**: `spawn` and `resolve` were added to CLAUDE.md without the spec amendment landing, so the correction must not claim a count that contradicts CLAUDE.md's **18**.
- Only the **genuinely-new** attr keys are added — `reason` and `path` already exist — and the stated count is computed from the deduplicated set, never by addition.
- The `spawn`/`resolve` **attr-key** gap is the same class of stale fact and must be raised explicitly rather than swept or ignored.
- The component is emitted from **three packages**, which is legal under **bind-once-per-package**, exactly as `spawn` and `bootstrap` already are.
- `prefs` and `terminals` stay **deliberately outside** the vocabulary — dumb stores with no runtime outcomes; what earns `theme` its place is that the loader has parse/validate/fallback outcomes.
- **Rejections are WARN, not INFO**, even though doctor treats them as advisory for exit-code purposes.
- The component records where a theme is **used**, never where one is **diagnosed** — `portal doctor` and `portal theme export` emit **nothing at all**, which is also what makes `theme: rejected`'s per-process dedup determinate.
- `token`'s only consumer is `theme: rejected`; the per-process dedup state lives on the **injected logger**, not in package state.
- One further stale fact is visible in the same file — the `clean` component, whose command `cli-verb-surface-redesign` retired — and it is out of this task's named scope: surface it, do not sweep it.

**Context**:
> The owning spec's extension policy: "New components require explicit amendment of THIS specification's closed component list. New attr keys require the same amendment process. Spec writers and code reviewers MAY NOT introduce new component or attr names ad hoc. The space is **genuinely closed**."
> §12.3: "Portal's log component taxonomy is **closed and spec-governed** — components are never invented at a call site. **This feature adds a `theme` component via spec amendment**, with direct precedent: `spawn` and `resolve` were both added by the features that needed them. What distinguishes it from `prefs` and `terminals` (both deliberately outside the vocabulary) is that those are **dumb stores with no runtime behaviour**, whereas the theme loader has parse/validate/fallback *outcomes*."
> §12.3: "**Attr keys:** `slug`, `slot`, `reason`, `path`, `token`, `count`, `rejected`." / "Rejections are **WARN**, not INFO." / "**The component records where a theme is *used*, never where one is *diagnosed*.** `portal doctor` and `portal theme export` both enumerate or parse and both can hit every §6.2 reason — and **neither emits any `theme` event**."
> §8.9: "This means the `theme` component is emitted from more than one package — the loader (`internal/theme`), the translation (`cmd/config.go`), and this persister. That is legal and normal: CLAUDE.md's rule is *bind once per package*, and `spawn` and `bootstrap` already emit from several files."
> §15.1: "**The log-component vocabulary.** A new `theme` component with its own attr keys and event catalogue (§12.3)."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §15.1, §12.3, §8.9, §10.5; protocol: `.claude/skills/workflow-shared/references/correcting-historical-artifacts.md`

## theming-system-10-10

### Task 10.10: Clear this feature's captures and tapes at sign-off

**Problem**: §13.2 draws a retention rule this feature is the first to live under: captures and the tapes that produce them are **scaffolding**, created as work proceeds, committed while they are being collaborated on, and **cleared out after sign-off** so they do not live in the repository forever. There is **no visual-regression obligation**, so the alternative is a permanently growing image set that could never survive a token rename or a theme split anyway. Two failure modes bracket this task. Clear too eagerly — before the last visual gate — and the gate has no artifact. Clear too broadly — sweeping the Go fixture definitions along with the images — and the swap-and-diff guard silently shrinks, because §13.4 enumerates whatever fixtures exist and **absence reads as coverage**. And CLAUDE.md now says these PNGs are scaffolding (task 10-6), so an unemptied directory contradicts the document an implementing agent reads first.

**Solution**: The second and final bounded clearing act — delete this feature's capture images and tapes from `testdata/vhs/`, decide explicitly on the files that are neither, and prove nothing in the suite read them.

**Outcome**: `testdata/vhs/` holds no capture PNGs or tapes from this feature; `internal/capture`'s fixture definitions and the harness are untouched; the full suite is green; and the directory's own README no longer contradicts CLAUDE.md.

**Do**:
- **Confirm this is sign-off.** Every visual gate in Phases 3, 8 and 9 must already have been taken and signed off — Nord's grouped `text.subtle` gate (3-5), the panel fixtures (8-15, 8-16) and the message-slot fixtures (9-12). Confirm no earlier task's acceptance still depends on a committed image. This is a **sign-off act** and must not run before the last gate.
- **Inventory before deleting.** List `testdata/vhs/` and classify every entry — Phase 3 task 3-1 already performed the **first** bounded act (deleting the pre-rename images and tapes), so what remains should be this feature's own plus the non-image, non-tape files below. Do not delete by glob without reading the listing first.
- **Delete this feature's `*.png` and `*.tape`** from `testdata/vhs/`. §13.2 explicitly refuses both a rolling clear-as-you-go and a general repo-wide capture cleanup: this is one bounded act on this feature's artifacts.
- **Keep the fixtures and the harness — they are permanent.** The Go fixture *definitions* in `internal/capture`, `cmd/capturetool`, and the VHS route all stay. §13.2 retires the committed artifacts, not the mechanism, and deleting a fixture would silently shrink §13.4's coverage assertion rather than the repository. Assert `capture.FixtureNames()` is unchanged across this task.
- **Decide explicitly, do not sweep, on the files that are neither image nor tape:**
  - **`testdata/vhs/README.md`** — the workflow README survives, because tapes are written per fixture from here on. But its current text calls the directory a "permanent visual-test rig" with PNGs "committed, overwritten in place", and lists specific files that will no longer exist — which **contradicts task 10-6's CLAUDE.md correction**. Reconcile it to §13.2's retention rule and to `capturetool`'s `--theme <slug|path>` flag, or the two documents disagree.
  - **`testdata/vhs/LOCK-IN.md`** — the MV light-tint lock-in record: a historical artifact of a completed feature, neither image nor tape. Its fate is a **deliberate decision**, taken and stated, not a sweep.
  - **`testdata/vhs/reference/`** — the committed Paper exports. **Flag this one rather than deciding it silently**: §13.2's deletion is worded as "everything that exists today as an image or tape", which reads onto these, while §9.14 calls the new theme-panel frames "the only reference that exists" for the slot half and §15.4 keeps the MV frames as historical reference. The spec does not resolve it, so raise it at the gate with a recommendation and act on the answer.
- **Prove nothing in the suite reads a deleted path.** Check rather than assume: grep the Go sources for `testdata/vhs`. Today every hit is a **comment** naming a reference frame (`internal/capture/fixtures.go`, `internal/capture/capture_test.go`, `cmd/capturetool/main.go`, several `internal/tui/*_test.go`) — no test opens one — but a comment that now names a deleted file is a dangling reference, so decide whether to update or keep each, and re-run the check after deleting.
- **Run both lanes after the deletion**: `go test ./...` and `go test -tags integration -p 1 ./...`. A cleanup that reds the suite means something read an image, which is the case this step exists to catch.
- Note that `.gifcache/` is a git-ignored VHS byproduct and needs no action.

**Acceptance Criteria**:
- [ ] Every Phase 3 / 8 / 9 visual gate is confirmed taken and signed off before any deletion.
- [ ] The directory listing was inventoried and classified before deleting.
- [ ] No `*.png` or `*.tape` from this feature remains in `testdata/vhs/`.
- [ ] `internal/capture`'s fixture definitions, `cmd/capturetool` and the VHS route are untouched; `capture.FixtureNames()` returns the same set before and after.
- [ ] `testdata/vhs/README.md` survives and no longer contradicts task 10-6's CLAUDE.md correction or `capturetool`'s current flag.
- [ ] `LOCK-IN.md`'s fate was decided explicitly and the decision is stated.
- [ ] `testdata/vhs/reference/`'s fate was raised at the gate rather than decided silently, and the agreed action was taken.
- [ ] `grep -rn "testdata/vhs" --include="*.go" .` was run after deletion and every remaining hit is a comment whose target was deliberately kept or updated.
- [ ] `go test ./...` and `go test -tags integration -p 1 ./...` are both green after the deletion.
- [ ] `git status` shows deletions of images and tapes only — no fixture, harness or Go source deleted.

**Tests**:
- `"the unit lane is green after the clear"` — `go test ./...`
- `"the integration lane is green after the clear"` — `go test -tags integration -p 1 ./...`
- `"the fixture set is unchanged"` — `capture.FixtureNames()` compared before and after (the swap-and-diff guard's coverage assertion depends on it).
- `"no Go source reads a deleted path"` — `grep -rn "testdata/vhs" --include="*.go" .` returns comments only.
- `"only images and tapes were deleted"` — `git status --short` reviewed entry by entry.
- `"the harness still runs"` — `go run ./cmd/capturetool --fixture <name> --theme nord` renders, proving the mechanism survived the artifact deletion.
- `"the vhs README and CLAUDE.md agree"` — read both retention statements side by side.

**Edge Cases**:
- The deletion covers **images and tapes only** — the Go fixture *definitions* and the harness are **permanent**, because §13.4's guard drives the fixture renderer and its coverage assertion needs the fixture set to exist; deleting a fixture silently shrinks the guard rather than the repository.
- `capturetool` and the VHS route both **stay** — §13.2 retires the committed artifacts, not the mechanism, and VHS remains the route if a gif is ever wanted.
- Phase 3 task 3-1 already performed the **first** bounded act; this is the **second and last**. §13.2 explicitly refuses both a rolling clear-as-you-go and a general repo-wide cleanup.
- `README.md` and `LOCK-IN.md` are **neither image nor tape** — each needs a deliberate decision rather than being swept; the workflow README survives but must be reconciled with task 10-6's CLAUDE.md correction or the two contradict.
- `testdata/vhs/reference/`'s Paper exports sit in a genuine gap between §13.2's "images" wording and §9.14/§15.4's treatment of frames as reference — **raise it, do not decide it silently**.
- **No test may reference a `testdata/vhs/` path** — that must be *checked*, not assumed; today's hits are comments, and a comment naming a deleted file is a dangling reference to resolve deliberately.
- This is a **sign-off act**: it must land after every visual gate in Phases 3, 8 and 9, and no earlier task's acceptance may depend on a committed image.
- `.gifcache/` is git-ignored and needs no action.

**Context**:
> §13.2: "**Retention rule, drawn now:** **Everything that exists today as an image or tape is deleted** — the committed reference PNGs and the VHS tapes that produce them… **From this feature forward, captures and the tapes that produce them are created as work proceeds, committed while they are being collaborated on, and cleared out after sign-off** so they do not live in the repository forever. A tape is scaffolding on the same terms as the image it renders. **This feature does not take on a general repo-wide capture cleanup**, and does not clear captures continuously as it goes. Both of the above are in scope and both are single, bounded acts: delete today's images and tapes once at the start, clear this feature's own once at sign-off."
> §13.2: "**The deletion covers images and tapes, NOT fixtures.** The Go fixture *definitions* in `internal/capture` and the harness itself are **permanent** — the swap-and-diff guard drives the fixture renderer and its coverage assertion needs the fixture set to exist. 'Cleared out after sign-off' likewise means the images, not the fixtures."
> §13.4: "**The guard enumerates the harness's fixture set; it never names fixtures.**… the fixture list *is* the coverage list, and it grows automatically as screens are added."
> §13.3: "A missing fixture is a blind spot the guard structurally cannot report: §13.4 enumerates whatever fixtures exist, so absence reads as coverage."
> §12.6: the capture-harness CLAUDE.md row — "Describes `testdata/vhs/` as committed reference PNGs forming a visual-verification harness, which reads as a durable asset. It is not (§13.2)."

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §13.2, §13.1, §13.3, §13.4, §12.6
