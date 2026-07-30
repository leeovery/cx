# Review Tracking: Theming System - Input Review

## Findings

### 1. A `bad name` row has no valid slug to render — the discussion says it shows the filename

**Source**: `discussion/theming-system.md` — "Every theme file gets a row — invalid ones shown, not selectable": *"a file with a **bad name** (`My Theme.theme`) was going to be silently skipped; now it is a row showing the filename with `bad name` as its reason."*
**Category**: Enhancement to existing topic
**Affects**: §9.4 (The list), §9.5 (Row rendering and markers), §9.2 (list order)

**Details**:
Every other row in the panel is labelled by its slug (§5.1 makes the slug the only name there is). A file rejected with reason `bad name` **has no valid slug by definition** — `My Theme.theme` cannot yield `my theme` because §5.2 rejects rather than normalises. The discussion answers this explicitly (the row shows the filename); the specification does not carry that answer, so the row's label is undefined for exactly the class of file the row exists to make visible.

Two consequences follow that the spec also does not state:
- The label for a `bad name` row is the **filename** (or filename-minus-extension), not a slug.
- §9.2 pins list order as "alphabetical by slug" — a `bad name` row has no slug to sort on, so ordering must fall back to the filename.

**Current**:
> §9.5: **Invalid rows** render in `text.faint` with `⚠` and a terse reason from §6.2 (`missing tokens`, `bad colour`, `bad syntax`, `bad name`, `reserved name`, `unreadable`, `not found`) — **glyph-backed** per MV spec §2.2 so it survives colourless. Full detail stays in doctor, where there is width to enumerate.
>
> §9.2: **List order is alphabetical by slug**; ordering same-mode themes first was proposed as a mitigation and **rejected** as unnecessary once the flash is accepted.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Added to §9.5: a `bad name` row is labelled by its filename, and list ordering falls back to the filename for a row with no slug.

---

### 2. The `theme` log catalogue is missing an attr key and an event the spec's own decisions require

**Source**: `discussion/theming-system.md` — "The `theme` log catalogue (F5)" (attr keys list), "Log cadence, corrected for lazy discovery (the review's F4)" (`theme: enumerated` carries `count` and `rejected`), and "Write-path robustness … The themes directory itself (F11)" (*"Unreadable, or a regular file where a directory belongs, gets a doctor advisory line **and a log entry**"*).
**Category**: Enhancement to existing topic
**Affects**: §12.3 (A new `theme` log component); cross-refs §5.5 (Directory states)

**Details**:
The spec states the log-component taxonomy is **closed and spec-governed** and that "New components/attrs require amending the spec — never invent at call-site" (CLAUDE.md, restated in §12.3). Two items escape the closed declaration:

1. **`rejected` is used but never declared.** The event table says `theme: enumerated` "Carries `count` and `rejected`", but the attr-key list is `slug`, `slot`, `reason`, `path`, `token`, `count` — no `rejected`. The discussion carries the same omission, so it is inherited rather than introduced. Implementation must either declare `rejected` or reuse `count` twice, and neither is sanctioned.
2. **No event covers the unreadable themes directory.** §5.5 requires "a **doctor advisory line** and a **log entry**" for an unreadable directory (or a regular file where a directory belongs). The six-event catalogue has nothing that fits: `theme: rejected` is per-*file* with a §6.2 reason, and `unreadable` in §6.2 is defined as "The file could not be read". A directory-level misconfiguration has no declared event, so the required log entry cannot be emitted without inventing one at the call site.

**Current**:
> | `theme: enumerated` | INFO | At panel open. Carries `count` and `rejected`. |
> …
> **Attr keys:** `slug`, `slot`, `reason`, `path`, `token`, `count`.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Added `theme: directory unusable` (WARN) to the §12.3 catalogue and declared `rejected` in the attr-key list, with a note that both close holes rather than extending the vocabulary by preference.

---

### 3. Init-time copies of *derived* styles were never swept — the discussion names this as a second, unaudited class

**Source**: `discussion/theming-system.md` — "The real risk is completeness — guarded behaviourally": *"Two specific offenders research already found: `pagepreview.go:35` copies a `Token` at **package init**, so it would never see a swap; and **init-time copies of *derived styles* were never swept for at all**."*
**Category**: Enhancement to existing topic
**Affects**: §11.2 (The real risk is completeness), §13.4 (The swap-and-diff completeness guard)

**Details**:
§11.2 lists the cached styles Portal does not own (`bubbles/list`'s help styles, pagination dots, TitleBar, both filter inputs) and names the two known offenders that are fixed outright. It drops the discussion's separate observation that **init-time copies of derived styles were never audited at all** — i.e. `pagepreview.go` is the one that was *found*, not the boundary of the class.

This matters to implementation sequencing: the spec currently reads as "two known offenders, both fixed, the guard catches the rest", when the source says a whole category was never swept. The swap-and-diff guard is the safety net for it (which is the resolution), but the residual is worth stating so the implementer runs the sweep rather than assuming the two named fixes close the class.

**Current**:
> §11.2: Threading the theme (§3.4) fixes most of this: anything taking the theme as a parameter re-derives per frame. What remains is the **cached styles Portal does not own** — `bubbles/list`'s help styles, pagination dots, TitleBar, and both filter inputs — which are assigned once. That list is hand-maintained with no guard test, unlike the colour-literal rule which has an AST glob guard. Miss a site and the element silently keeps the previous theme's colours until something else re-renders it.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Added to §11.2: the two named offenders are what was found, not the class boundary; implementation must sweep init-time copies of derived styles.

---

### 4. "Three eyeball-pinned light surface tints" undercounts — `TestLightSurfaceTintsPinned` pins five entries across four tokens

**Source**: `discussion/theming-system.md` — repeated as "three light surface tints" in "Theme model — split, not paired" (*"three light surface tints were eyeball-pinned at a validation gate"*), "Verification — four resolutions (F4)", and "Guard tests reshape". Checked against `internal/tui/theme/contrast_test.go:351` and `theme.go:132-185`.
**Category**: Gap/Ambiguity
**Affects**: §3.1, §4.7, §7.1, §13.5, §13.6

**Details**:
The count "three" is carried verbatim from the discussion into five places in the spec, and it is wrong against the code. `TestLightSurfaceTintsPinned` pins **five** entries — `bg.selection`, `bg.warning`, `bg.track`, `border.separator`, `border.footer` — which is **four distinct tokens** after the §2.2 border consolidation (`bg.selection`, `bg.attention`, `bg.subtle`, `border`). `theme.go` carries a matching `pinned — derivation … eyeball-confirmed at the 1-9 gate` comment on each of the four. (`TestLightTintFillsArePerceptible` covers the same five.)

The count is load-bearing in three places, so an undercount loses real content:
- §7.1: the erratum comments are deleted, and "the three eyeball-pinned light surface tints" move into the theme file as `#` comments — at three, `border`'s pin note is dropped and the judgement behind it is lost with no numeric guard to recover it.
- §13.5 / §4.7: the light/dark test table exists *because* those tints are not numerically checkable; the carve-out must cover all four.
- §3.1 / §13.6: the "pairing MV implies isn't real" argument and `TestLightSurfaceTintsPinned`'s per-light-theme reshape both quote the number.

(The companion figure — "six corrected light values" in §6.4 / §7.7 — **is** correct: six `§2.9 erratum` corrections in `theme.go`.)

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Verified against contrast_test.go:351 and theme.go — five pinned entries, four distinct tokens post-consolidation. Corrected in §3.1, §3.3, §4.7, §7.1, §13.5 (with the enumerated token list and why the count is load-bearing) and §13.6.

---

### 5. The guard-test reshape table omits the light-variant tests that split structurally invalidates

**Source**: `discussion/theming-system.md` — "Guard tests reshape" (names only `TestMVTokenCount`, `TestMVDarkVariantsPinned`, `TestLightSurfaceTintsPinned`); checked against `internal/tui/theme/theme_test.go:72` and `contrast_test.go:332, 388`.
**Category**: Gap/Ambiguity
**Affects**: §13.6 (Guard-test reshape), §13.5 (Contrast checking)

**Details**:
§13.6 presents itself as the enumeration of the reshape ("Test | Change"), and the discussion's list behind it is equally short. Three existing tests are structurally invalidated or materially changed by split and appear nowhere:

- **`TestEachTokenCarriesLightVariant`** (`theme_test.go`) — asserts every token carries a Light variant. Cannot compile once `Token` is `{Name, Value}`.
- **`TestEveryTokenHasLightVariant`** (`contrast_test.go`) — same shape, same fate.
- **`TestLightTintFillsArePerceptible`** — the ≥1.1 fill floor for light tints against the light canvas. Under split it needs the same per-light-theme treatment §13.6 gives `TestLightSurfaceTintsPinned`, and the same light/dark table membership from §13.5; the spec is silent on it.

Without naming them, the reshape table reads as complete while leaving two tests that cannot survive the `Token` collapse and one whose per-theme scoping is undecided.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Added three rows to the §13.6 table: TestEachTokenCarriesLightVariant and TestEveryTokenHasLightVariant deleted (cannot compile once Token is {Name, Value}); TestLightTintFillsArePerceptible survives per-light-theme with theme-resolved reference background.

---

### 6. `portal theme export`'s "canonical form" is undefined — and comment preservation is load-bearing

**Source**: `discussion/theming-system.md` — "`portal theme export` — command surface (the review's F5)": *"**Output** is the theme in canonical form."* Interacts with "Built-ins are theme files" (*"the *why* moves into the theme file as a `#` comment"*) and "On-ramp — `portal theme export`".
**Category**: Gap/Ambiguity
**Affects**: §12.1 (`portal theme export <slug>`), §7.1, §13.6

**Details**:
Both the discussion and §12.1 say export writes the theme "in canonical form" without ever defining it, and the two plausible readings diverge on something the spec depends on elsewhere:

- **Re-serialise from the parsed `Theme`** — deterministic, and matches "show me what Portal parsed" (§12.1's diagnosis-tool framing). But it **drops every `#` comment**, including the attribution header §4.1 justified the whole file format for, and the eyeball-pin derivation notes §7.1 moves into the theme file as the only surviving record of a non-numeric judgement. A user who runs `portal theme export tokyo-night-day > …` to start a light theme gets a file stripped of exactly the notes that explain its pinned tints.
- **Byte-copy the embedded/on-disk file** — preserves comments and makes the on-ramp faithful, but then "canonical" means nothing, and an *invalid* drop-in cannot be dumped at all (which is fine, since §12.1 refuses those anyway).

Also unstated: whether export emits a trailing newline / stable key order, which matters because the output is redirected straight into a file that must re-parse.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved to byte-faithful output: the theme is parsed and validated first, but the file bytes are written, preserving comments. Recorded in §12.1 with the reasoning (attribution header + eyeball-pin notes) and the note that it also disposes of key-order/trailing-newline questions.

---

### 7. The Nord port's contrast-leg verification table and the "unwalked leg" consequence are not carried forward

**Source**: `discussion/theming-system.md` — "The unwalked legs — a second correction (the review's F8)" (the ten-leg table and the lesson), and "The Nord port, analysed against the real spec" (*"`accent.mode` ← nord8 `#88C0D0` (6.24 — **chosen over nord7**, being Nord's own primary UI accent)"*).
**Category**: Enhancement to existing topic
**Affects**: §7.4 (The Nord port), §13.5 (Contrast checking)

**Details**:
Two items from the source do not appear in the spec:

1. **The pairing legs the port was verified against.** §7.4's table gives each token's ratio vs canvas, but the port's second correction was found by walking the *pairing* legs — `state.positive` on `bg.selection` (4.23, the failure), `text.secondary` on `bg.selection` (7.09), `text.tertiary` on `bg.selection` (6.39), `bg.subtle` fill (1.24), `accent.mode` as peek chrome (6.24), the `text.subtle` 3.00–4.49 band, the `text.faint` 1.00–2.99 band. The spec carries the two corrections but not the record of which legs were measured, so a re-derivation (see §7.7, which may move MV's light values) has no baseline to re-check against.

2. **The consequence the discussion drew and the spec drops:** *"a failure on an unwalked leg can force re-deriving an **invented** value, which by this port's own precedent then needs a fresh visual gate."* This is an implementation-time constraint on §7.4's three invented values, and it composes with §7.7's conditional built-in-set decision — but §7.4 only records the already-outstanding `text.subtle` gate, not the rule that produces new ones.

Minor, same source: §7.4's table gives `accent.mode` ← nord8 without the recorded reason it was chosen over nord7 (`#8FBCBB`, 5.99) — Nord's own primary UI accent.

**Current**:
> §7.4: **Outstanding visual gate:** `text.subtle` has no locus on any captured Nord frame — it renders group `··· N` counts and pending loading steps, neither of which appears on the flat Sessions frame. **It needs a visual gate at implementation, on a grouped Nord capture.** (`text.muted` has already been seen — it is the "N window(s)" text on `Sessions — Nord (port)`.)

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Added the ten-leg verification table to §7.4 as the port baseline, the unwalked-leg rule (a failed leg can force re-deriving an invention, which then needs a fresh visual gate), and the nord8-over-nord7 reason in the mapping table.

---

### 8. The themes-directory env var is never named

**Source**: `discussion/theming-system.md` — "Discovery — lazy, not startup-scanned": *"**Directory resolution** follows Portal's existing per-file chain shape: dedicated env var → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/`."*
**Category**: Gap/Ambiguity
**Affects**: §5.5 (Directory resolution)

**Details**:
Both the discussion and §5.5 say "dedicated env var" without naming it. Every other member of Portal's config chain has a spec-fixed name (`PORTAL_TERMINALS_FILE`, `PORTAL_STATE_DIR`, etc.) precisely because it is a user-facing, documented contract — `docs/theming.md` (§12.4) will have to print it. Left unnamed, the name is invented at implementation, and it is the one item in §5.5 that a user types.

Adjacent, same sentence: §5.5 notes the mechanical difference (a *directory*, where `configFilePath` resolves *files*) but does not say whether the override is expected to be a directory path (`PORTAL_THEMES_DIR`) — the naming convention should follow that distinction.

**Current**:
> The themes directory resolves through Portal's existing per-file chain shape:
>
> **dedicated env var → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/`**

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Named as `PORTAL_THEMES_DIR` in §5.5, with the `_DIR` suffix marking the directory-vs-file distinction and a note on why the name is spec-fixed rather than left to implementation.

---

### 9. `capturetool`'s existing `--appearance` flag and the contrast-validation swatch fixture are unaddressed

**Source**: `discussion/theming-system.md` — "Decisions" under *Capture harness & tests*: *"`tui.Build` takes a *theme* where it took a `prefs.Appearance` (the exact injection mechanism this work removes), and `capturetool` gains a `--theme` flag."* Checked against `cmd/capturetool/main.go:35` and `swatch_test.go`.
**Category**: Enhancement to existing topic
**Affects**: §13.3 (Harness changes required), §7.5, §13.5

**Details**:
§13.3 says `capturetool` **gains** `--theme` but never says what happens to `--appearance`, which exists today (`main.go:35`, `dark|light`, resolving to a pinned `prefs.Appearance`). Its entire mechanism — `prefs.Appearance` and the `WithAppearance` option — is deleted by §8.8, so the flag cannot survive; the spec should say so rather than leave a dead flag whose backing type is gone.

Related and more substantive: `capturetool` carries a **contrast-validation swatch** branch (`main.go:79-82`, `swatch_test.go`) — the standalone labelled-tint surface built for the MV §16.5 lock-in/bail gate, currently driven by `--appearance`. That surface is the mechanism behind §7.5's and §13.5's requirement that a new light theme's `TestLightSurfaceTintsPinned` values are "established by human eyeball at a visual gate through `capturetool`". The spec names the requirement but not the surface that satisfies it, and the swatch needs the same `--theme` re-pointing as the fixture path.

**Current**:
> §13.3:
> - **`tui.Build` takes a *theme* where it takes a `prefs.Appearance` today** — the exact injection mechanism this work removes. Without this the harness can only ever render the compiled-in default.
> - **`capturetool` gains a `--theme` flag.** It accepts a built-in slug **and an explicit path to a real theme file**. An explicit path from a flag is an **input, not config discovery**, so the `internal/capture` no-real-config import guard's invariant is preserved (no XDG lookup, no prefs read). This matters disproportionately: it is the only visual-verification route for someone authoring a drop-in.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Added to §13.3: `--appearance` is removed (its backing type is deleted by §8.8), and the contrast-validation swatch branch is re-pointed to `--theme` since it is the surface that satisfies the §7.5/§13.5 light-theme eyeball gate.
