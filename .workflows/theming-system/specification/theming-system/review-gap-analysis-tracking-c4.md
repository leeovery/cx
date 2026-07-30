# Review Tracking: Theming System - Gap Analysis

Cycle 4. Full fresh pass over the whole specification as a standalone document.

## Findings

### 1. The panel's row-set union is keyed on "has no file", which a built-in does not have

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §9.4 (the list), §9.5 (markers), §9.2 (opening cursor)

**Details**:

§9.4 defines the panel's row set as a three-way union — directory files ∪ built-ins ∪ persisted slugs — and the only dedup rule it gives is the phrase **"that has no file"**. Built-ins are embedded, not files in the themes directory (§7.1: "they are not Go structs" but they are also not directory entries), so a literal reading mints a **second** row for any persisted slug naming a built-in: one selectable built-in row, plus one unselectable `not found` row for the same slug.

This is not an exotic state. It is the state produced by the *most common action the panel supports*: pressing `Enter` on `tokyo-night` writes `"theme": "tokyo-night"`, and that slug names no file. The same applies to `d`/`l` on any built-in, and to a hand-edited `theme_dark = nord`. Under the misreading the user commits a theme and immediately sees it listed twice, once marked `⚠ not found`, with §9.2's opening cursor rule ("the theme that is actually rendering") now ambiguous about which of the two rows it means and §9.5's `●` placement equally so.

The correct rule is clearly *resolves to nothing* rather than *has no file* — but the spec never says "resolves", and its own doctor wording shows the difference is live: §12.2 says doctor "**Reports when a persisted theme name no longer resolves**", which is the right form, while §9.4's bullet says "A persisted slug with **no file** gets a row too". Two surfaces describing the same condition in two vocabularies, one of which is wrong for built-ins.

The adjacent case is also unstated, though it is inferable: a persisted slug naming a file that *exists but is invalid* must produce one row (the file's, carrying the reason and the badge), not two.

**Current**:

§9.4: "**Every `*.theme` file in the themes directory gets a row, plus every built-in, plus any slug named in `prefs.json` that has no file.**"

§9.4 bullet: "A persisted slug with **no file** gets a row too — marked, unselectable, reason `not found`. Same shape as an invalid file: the user sees what is set and why it is not applying. This covers a deleted file, a renamed file, and a typo in `prefs.json`."

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. Which prefs read the mandated RMW re-read uses, and when the translation's no-op condition is evaluated

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.9 (RMW), §10.3 (trigger vs no-op condition), §10.5 (compute vs persist, the two read variants)

**Details**:

Three separately-settled rules now meet on one code path and are not reconciled:

1. §10.5 defines **two** prefs reads — the migrating `loadPrefsStore` (used where a TUI is constructed) and a non-migrating variant (used by every bootstrap-exempt command).
2. §8.9 mandates that **every** writer "re-read `prefs.json` immediately before writing" — a **third** read site, mid-session, inside a TUI process, assigned to neither variant.
3. §10.3 adds a no-op condition ("if any theme key is already set, the translation writes no theme key"), and §10.5 keeps the translation's write **"best-effort and non-blocking"**, separated from the compute.

Two questions follow, both with user-visible consequences:

- **Which read does the RMW re-read use?** If it goes through `loadPrefsStore`, then every `s` keypress and every theme commit re-enters the translation path mid-session. That is harmless once the marker is on disk — but §10.5 explicitly allows the marker write to fail and retry next launch, so a process running with the condition still true is a specified state, not a hypothetical. If it goes through the non-migrating read, that needs saying, because §10.5 currently frames the non-migrating variant as existing for *bootstrap-exempt commands* and nothing else.

- **When is §10.3's no-op condition evaluated — at compute time or at the RMW re-read?** With a non-blocking write, the user can commit a theme in the window between compute and persist. Evaluated against the load-time snapshot, the pending translation writes `theme = tokyo-night` over the `nord` the user just committed and (per §8.2's write rule) clears the slots — the exact loss §10.3 was added to prevent, displaced from cross-launch to intra-process. Evaluated against the re-read bytes, the commit survives. §8.9 names only the marker half of this ("The RMW re-read also lets the migration observe that another instance already set `theme_migrated` and skip its own write"), not the theme-keys half.

Related and one line to settle in the same place: whether the translation's in-memory result remains the model's active theme after a mid-session commit supersedes it, and whether "skip its own write" means skipping the whole write or only the theme keys.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. The theme loader has no stated package home, and nothing owns themes-directory path resolution

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §5.5 (directory resolution), §7.1 (a built-in *is* a theme file), §8.4 (by-name resolution), §12.3 (the `theme` log component), §13.3 (harness / import guard)

**Details**:

The loader is the feature's central new component — parse (§4.2), validate (§6.1), the §6.2 ladder, by-name resolution embedded-set-first-then-directory (§8.4), enumeration (§5.6), and fallback (§8.5). It is referenced throughout ("the **same loader** as a user's", "the loader returns an ordinary error"), but **the spec never says where it lives**, and it has four consumers spanning two layers: TUI construction (`cmd/open.go`), the panel (`internal/tui`), `portal doctor`, and `portal theme export`.

This is conspicuous because the spec assigns ownership everywhere else it matters — §10.5 puts the translation in `cmd/config.go` and states "`prefs` stays dumb"; §3.2 fixes the `Theme` shape; §13.3 names the `ThemeEnumerator` seam. Three decisions are left to the implementer, and each has knock-on effects:

- **Package.** `internal/tui/theme` is today a pure data package (no I/O, no logging, and the one package the colour-literal guard excludes). Putting the loader there gives a *TUI subpackage* file I/O plus a `log.For("theme")` binding, and makes `cmd/doctor.go` and the export verb import a tui subpackage. A new leaf `internal/theme` avoids that but has to be named, because CLAUDE.md's rule is one component binding per package and §12.3 declares the component closed.
- **Path resolution.** §5.5 says the themes directory "resolves through Portal's existing per-file chain shape" — which lives in `cmd/config.go` (`configFilePath` / `prefsFilePath`). But `_DIR` is explicitly *not* that mechanism ("this resolves a *directory* where `configFilePath` resolves *files*"). So either `cmd/config.go` gains a `themesDirPath` and the loader takes an injected directory (which then has to be threaded into `internal/tui` for the panel's enumeration), or the loader resolves it itself via `internal/xdg` (the precedent `internal/state/paths.go` sets). Nothing says which, and it decides the shape of every consumer's call.
- **Separability of embed from discovery.** §7.1 requires the embedded set to stay reachable from `internal/capture` ("`go:embed` is not config discovery"), while §13.3 states the guard "forbids `internal/capture` reaching config at all". If both live in one package, whether that guard is still satisfiable depends on how it is written.

Consequence for planning: the task breakdown itself becomes a design decision, and the first task cannot be scoped without answering this.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. The `appearance` translation and the prefs write path have no test home

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.6 (guard-test reshape), §8.8 (retained raw `appearance`), §8.9 (RMW), §10.2/§10.3/§10.5

**Details**:

§13.6 is the specification's enumeration of new and changed tests — that framing is what earned the loader/parser test its row ("The single most branch-heavy component in the feature has no other test home"). The table lists nothing for the prefs and migration path, which is the one part of this feature whose failure mode is **silent destruction of a user's config**:

- §10.2's mapping (`dark` → `theme: tokyo-night`, `light` → `tokyo-night-day`, `auto`/absent → nothing).
- §10.3's separation of trigger from no-op condition, including the reachable loss-of-setting sequence it was written to close.
- §8.1's rules that the marker is written only when `prefs.json` already exists, and that empty values are omitted.
- §8.8's raw `appearance` round-trip, which the spec itself calls "load-bearing, not tidiness" and whose named failure is that "the first `s`-keypress or theme commit after upgrade **silently erases** the user's `appearance` pin".
- §8.9's RMW merge — writer A must not revert writer B's field.

Every one of these is a rule with a specific wrong outcome, none is observable to the user at the moment it goes wrong, and none is covered by any listed test. §8.8's failure in particular is invisible until a downgrade, which is precisely the scenario §10.4 exists to protect.

The gap is sharpened by contrast: the spec pins a named test for `RestoreTerminalBackground` (§11.4) on the reasoning that it is "the one path where a mistake re-sticks a colour in the user's terminal after Portal exits". The prefs path deletes a setting the user chose, permanently, and gets no equivalent.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 5. What the light/dark table changes in the floor test is never stated, and "not ratios" contradicts §13.6

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §13.5 (contrast checking), §13.6 (`TestLightTintFillsArePerceptible`)

**Details**:

§13.5 now states the canonical, theme-independent rule set, and separately says a light/dark table is "needed because the light surface tints are not numerically checkable" and that "the carve-out must apply to light themes only". But **the carve-out's content is never stated** — which rules are relaxed, replaced or added for a light theme. The only concrete statement is one line, and it conflicts with §13.6:

- §13.5: "*Light themes only:* the four eyeball-pinned tints (below) are exact-value pins, **not ratios**."
- §13.5's tint table nonetheless applies "fill vs canvas ≥ 1.10" to `bg.selection`, `bg.attention` and `bg.subtle` with no light/dark qualifier.
- §13.6: `TestLightTintFillsArePerceptible` "covers the same four tints ... its **≥1.1 fill floor** resolves its reference background from the theme rather than the hardcoded light canvas."

So for `tokyo-night-day` the fill legs are simultaneously "not ratios" and enforced at ≥1.1. An implementer reading §13.5 literally drops three legs from the auto-enumerating test for every light theme; one reading §13.6 keeps them. The safe answer is almost certainly "the pins are *additional*, nothing is relaxed, and the table exists solely to enrol light themes into the two pin tests" — but that is exactly what is not written, and §13.5's own framing ("the carve-out") implies something *is* relaxed.

One more thing the same sentence leaves open: `border` is one of the four pinned tokens (§13.5's count) but carries "no numeric floor" in the canonical table, so it participates in the pins and in nothing else — worth being explicit given the count is called load-bearing.

**Current**:

§13.5: "*Light themes only:* the four eyeball-pinned tints (below) are exact-value pins, not ratios."

§13.5: "**Plus a light/dark table**, needed because the light surface tints are not numerically checkable (light-tint-on-light-canvas is numeric-insufficient — hence `TestLightSurfaceTintsPinned`), so the carve-out must apply to light themes only."

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 6. The Loading page is absent from the panel's entry conditions, which are stated as exhaustive

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §9.6 (per-page `t` table), §9.7 (entry conditions)

**Details**:

§9.7 opens with an exhaustive claim — "**Nothing blocks `t` except a modal, a pending burst, and `NO_COLOR`**" — and §9.6's per-page table enumerates Sessions, Projects, Preview and Modals. Portal's page state machine is Loading → Sessions → Projects → Preview, and the **Loading page is missing from both**.

It is not a corner: on the cold + TUI path the loading page holds for at least `LoadingMinDuration` (1.2s) while the concurrent bootstrap runs, on every cold start, with the user sitting in front of it. Read literally, `t` opens a slide-over over the honest loading page mid-restore. The theme nomination is loaded at construction (§8.4), so the panel *could* enumerate and preview there — this is not blocked by construction order.

Two sub-questions follow if it is to be blocked: whether it is **silent** (the Preview/modal class) or **flashed** (the `NO_COLOR` class) — §9.7's rule is "flash where the key *is* bound and the user could reasonably expect it to work" — and whether the flash surface even exists on the loading page, which renders no notice band.

The natural implementation (the loading page is inert, keys handled only on Sessions/Projects) lands in the right place, which is why this is Minor — but §9.7's exhaustive phrasing is what an implementer checks against, and it currently says `t` works.

**Current**:

§9.7: "**Nothing blocks `t` except a modal, a pending burst, and `NO_COLOR`.**" … "**Sessions and Projects normal view** — always available."

§9.6 table rows: Sessions / Projects / Preview / Modals.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 7. A resize-forced panel close does not say what happens to the uncommitted preview

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.8 (geometry, resize while open), §9.2 (`Esc` semantics), §11.4 (exit-time restore)

**Details**:

§9.8 specifies "**Resize while open: degrade in place**, closing with a flash only if the terminal falls below the render floor" — but not what that close does with state the panel owns.

`Esc` is defined precisely ("Discards an uncommitted preview and renders the resolved persisted state"). A forced close is the only *other* way the panel goes away, and the spec does not say whether it takes the same path. If it does not, the user is left rendering a theme they never chose and never persisted, with the surface that could change it now gone — recoverable only by reopening the panel (which the terminal is by definition too narrow for) or quitting. That is also the state §11.4 singles out as the one where "a colour the user never chose can be left stuck in their terminal after Portal exits", so the two sections are describing the same hazard from opposite ends without meeting.

Second, smaller: if the slot-from-constant **confirm** is live when the forced close fires, it is resolved by neither `y` nor a cancelling key. §9.2 says "Nothing has been written yet, so there is no partial state to leave behind", which suggests silent cancellation is safe — but it wants stating, since the confirm is otherwise specified as resolvable only by a keypress.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 8. The message slot and the minimum-height floor are specified independently and collide

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.1 (message slot), §9.8 (minimum height)

**Details**:

Two geometry rules are each internally complete but do not compose:

- §9.1: the message slot is "**not reserved when empty** — it appears and the list shrinks by one", and "at the minimum panel width the slot may wrap to two rows".
- §9.8: the panel "refuse[s] with a flash only when header + footer + one row cannot fit".

At the floor the panel is exactly header (2 rows, §9.1) + vertical footer + **one** list row. Raising the slot-from-constant confirm there takes that row, leaving zero — so the user is asked "clear constant `<slug>`? y / n" about a theme whose row is no longer on screen, and a two-row wrap leaves the arithmetic negative.

An implementer must invent one of: the floor includes a message row (so the refuse threshold is one row higher than §9.8 states), the message overlays the list row rather than displacing it, or the list is genuinely allowed to reach zero rows while a message is live. The confirm cannot simply be suppressed — it gates a write that §9.2 says must not happen silently, and the same applies to §9.13's failed-commit line, which is specified to persist until the next keypress.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 9. `capturetool --theme <path>`: which reason classes apply to an input that has no slug

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §13.3 (`--theme` flag), §3.2 (`Theme` carries no identity), §6.2 (reason vocabulary)

**Details**:

§13.3 pins the default, the slug-vs-path discriminator and the hard-error behaviour ("**Invalid input is a hard error** with the §6.2 reason and a non-zero exit"), but three of §6.2's seven reasons are derived from the *filename*, not the contents — `bad name` (slug charset **and** extension casing, §5.6), `reserved name`, and `not found` — and §3.2 states that "a theme loaded from an explicit path has **no slug**". Which of those apply to a path input is undefined, and both answers have a cost:

- **If filename rules apply**, `--theme ./My-Theme.theme` fails `bad name` and `--theme ./nord.theme` fails `reserved name` — the second making the flag useless for the workflow §12.1 publishes (export a built-in to a file, look at it, then rename), since the exported file is a reserved slug until the user renames it.
- **If they do not apply**, the tool cannot warn the author that the filename they are about to drop in is fatal — and §13.3 calls this path form "the **only** visual-verification route for someone authoring a drop-in", which is the case where a fatal filename is most worth catching.

A third case falls out of the discriminator rule and is worth settling in the same place: because slug-vs-path is decided by the `.theme` suffix, `--theme ./mytheme.txt` (or any real file without the suffix) is classified as a *slug* and errors as an unknown built-in — an error naming the wrong problem for a file that plainly exists.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:
