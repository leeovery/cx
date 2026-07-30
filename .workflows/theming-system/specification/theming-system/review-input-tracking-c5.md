# Review Tracking: Theming System - Input Review

Cycle 5. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. The panel's commit write has no named owner, so the one `theme` event that fires outside the loader and `loadPrefsStore` has nowhere to be emitted from

**Source**: `discussion/theming-system.md` — "Prefs writes, ownership and the log catalogue" (lines 1995–2000: *"Three decided constraints met here unreconciled: `prefs` is a deliberate leaf that must not import `internal/log`; the translation happens 'at prefs load'; the `theme` log component records it. **`cmd/config.go`'s `loadPrefsStore` owns it** … `prefs` stays dumb."*), "A failed commit write (F13)" (lines 1953–1959), the `theme` catalogue (line 2023: `theme: commit failed`), and "Concurrent Portal instances" (lines 1795–1798: the `ModePersister` seam a theme persister follows)
**Category**: Gap/Ambiguity
**Affects**: §8.9 (Concurrent instances and prefs writes), §9.13 (A failed commit write), §12.3 (log catalogue), §10.5 (Ownership); cross-ref §3.2

**Details**:

The source resolved *who owns a prefs write* once, explicitly, and only for the migration — because three constraints collided there: `prefs` is a leaf that must not import `internal/log`, the write happens at prefs load, and the `theme` component records it. The specification carries that resolution intact in §10.5.

**The identical collision exists for the panel's commit write, and nothing resolves it.** This feature adds a second prefs writer:

- It writes `theme` / `theme_light` / `theme_dark` into `prefs.json` (§8.2's mutual exclusion, §9.2's three commit keys).
- §8.9 requires it to read-modify-write, using the **non-migrating** read.
- §12.3 declares `theme: commit failed` (WARN, carrying `slug` / `slot` / `reason`) for when it fails.

So the commit path must (a) resolve the prefs path, (b) RMW, and (c) emit under the `theme` component — none of which `prefs` may do, since §10.5 keeps it dumb for exactly this reason. The spec names the *seam* the persister follows (§8.9: "the existing `ModePersister` seam that a theme persister follows") but never names the owner behind the seam, so `theme: commit failed` has no declared emission site.

This also leaves §3.2's justification for the new `internal/theme` package overstated: it argues the package gives "one component binding, as CLAUDE.md's rule requires", while §10.5 already puts a `theme:`-component emission in `cmd/config.go` and the commit failure would put a third somewhere. The rule CLAUDE.md actually states is *bind once per package* (`spawn`, `bootstrap` and others are emitted from more than one file), so multi-package emission is legal — but the spec should say where, rather than implying a single site it does not have.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. A failed commit is reported, then silently discarded by the only key that closes the panel

**Source**: `discussion/theming-system.md` — "A failed commit write (F13)" (lines 1953–1957: *"reports inside the panel, **keeps the theme applied in memory**, and does not move the `●` … This does recreate 'applied but not persisted', but as a *reported* state rather than a silent one"*) against "Two undefined transitions" (lines 1457–1461: *"**`Esc` discards the preview and renders the resolved persisted state**"*)
**Category**: Gap/Ambiguity
**Affects**: §9.13 (A failed commit write), §9.2 (the `Esc` row and its sharpening)

**Details**:

Two source decisions land on the same state and the specification carries both without reconciling them.

§9.13: a failed write "**Keeps the theme applied in memory**", reports `⚠ couldn't save theme` in the message slot, and the message "**persists until the next keypress** rather than timing out like a transient flash: it reports a state the user must act on".

§9.2: `Esc` "**Closes.** Discards an uncommitted preview and renders the resolved persisted state" — and `Esc` is the *only* way out of the panel (§9.2 pins that deliberately; §9.8's forced close takes the same path).

Compose them and the failed commit resolves to nothing: the write did not land, so the theme is uncommitted; the next keypress is very often `Esc`, which simultaneously clears the message and re-resolves from persisted state — dropping the theme the user just chose, with no trace on the main screen and no `●` movement to signal it either (§9.13 correctly forbids that). The "reported rather than silent" property the picker idiom was said to buy therefore holds only until the panel closes, which is the very next keypress.

What is undecided is not the resolution rule (§9.2 determines it) but whether anything survives the close. Three shapes are available and the spec picks none: state the revert explicitly as accepted; carry a main-screen flash on close after a failed commit (§14A already pins flash copy for the other panel-blocked states); or let a failed commit keep the theme applied for the session. Left unstated, an implementer will pick one silently, and the user-visible outcomes differ materially.

Adjacent and unstated in the same section: whether a *retry* is available at all — pressing the same commit key again is the obvious recovery, but §9.2's key table describes commits as unconditional writes and §9.13 does not say the failed commit is re-attemptable.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. §12.6's `CLAUDE.md` correction list claims completeness at four entries; at least three further surfaces are invalidated

**Source**: `discussion/theming-system.md` — "The premise correction — committed PNGs were scaffolding, not an asset" (lines 2349–2351: *"Note this contradicts how `CLAUDE.md` currently describes `testdata/vhs/` … **Worth correcting when the docs are updated**"*), applied against this feature's own decisions in §3.2, §5.5, §8.8, §10.5, §11.4 and §12.1
**Category**: Enhancement to existing topic
**Affects**: §12.6 (`CLAUDE.md`)

**Details**:

§12.6 states its scope as closed — *"four of its entries describe the pre-feature world. **All four** are corrected by this feature"* — and the reasoning it gives (an implementing agent reads `CLAUDE.md` first, and a stale entry actively misdescribes the subsystem under construction) applies with equal force to entries the table omits. Checked against the current `CLAUDE.md`:

- **The `tui` row.** It describes `restore.go`'s restore-on-exit as painting "the **mode-matched** canvas" and the canvas-echo guard as "`sameHexColour` **against the canvas hex**", carrying the standing *"Do not drop the guard"* warning. §11.4 re-anchors that comparison to a **retained startup canvas hex** and makes `canvasHexFor` theme-agnostic, and §3.2 deletes the mode concept the row's wording rests on. This is the one entry whose staleness is actively dangerous — it is the warning an implementer reads before touching the exact code §11.4 changes.
- **The "Config path resolution (cmd/config.go)" section.** It enumerates the config surface (`projects.json`, `aliases`, `hooks.json`, `prefs.json`, `terminals.json`) as resolving via `configFilePath`, and describes the TUI wiring as `WithInitialMode` / `WithModePersister` / **`WithAppearance`** with "the appearance pref … read once at construction and feeds the owned-canvas mode gate". §3.2 adds `themesDirPath` (a *directory*, not a `configFilePath` member), §5.5 adds `PORTAL_THEMES_DIR`, §10.5 adds the non-migrating read variant, and §8.8/§8.4/§13.3 delete `WithAppearance` in favour of the loaded nomination.
- **The "Server bootstrap" section's bootstrap-exempt set.** It lists `skipTmuxCheck` verbatim (`version`, `init`, `help`, `alias`, `hook`, `doctor`, `uninstall`, `state`, `__complete`). §12.1 adds `theme`.

Also incomplete rather than absent: the table's **`tui/theme` row** entry says "every clause is deleted", but §3.2 *relocates* the package to `internal/theme` — so the row does not merely lose its clauses, it moves out of the TUI's subtree in the internal-packages inventory (which is where an agent looks up package ownership), and the `internal/theme` leaf is a new inventory member.

**Current**:

> ### 12.6 `CLAUDE.md`
>
> `CLAUDE.md` is what an implementing agent reads first, and four of its entries describe the pre-feature world. All four are corrected by this feature — leaving them stale would have three of them actively misdescribing the subsystem under construction while the work is under way.
>
> | Entry | Correction |
> |---|---|
> | **The `tui/theme` row** | Describes ~20 tokens "each with a **Light and Dark** variant", `Token.ColorFor(mode)`, `theme.MV` as the single built-in, `Mode`'s zero value as the no-answer fallback, and `contrast_test.go` measuring against two hardcoded canvases. Every clause is deleted by §2.1, §3.2 and §13.5. |
> | **The `prefs` row** | Documents the `appearance` override, the `Appearance` enum and its tolerant decode, and `cmd/open.go`'s `WithAppearance` wiring. Replaced by `theme` / `theme_light` / `theme_dark` / `theme_migrated` per §8.1 and §8.8 — noting that `appearance` survives on disk as a preserved raw string (§8.8). |
> | **The logging section** | Pins the taxonomy at "17 component names". §12.3 adds an 18th (`theme`) with its own attr keys — the same shape of amendment `spawn` and `resolve` carried, which is why the count is stated at all. |
> | **The visual capture harness section** | Describes `testdata/vhs/` as committed reference PNGs forming a visual-verification harness, which reads as a durable asset. It is not (§13.2). The `capturetool` flag description also changes with §13.3. |

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. The source names four README places; §12.5 disposes of one

**Source**: `discussion/theming-system.md` — "Docs / README-CHANGELOG consequences" (lines 2221–2225: *"`appearance` is described in `README.md` at **four places**, including a paragraph recommending users pin it … That comes out with the setting"*)
**Category**: Enhancement to existing topic
**Affects**: §12.5 (README and CHANGELOG); cross-ref §5.5

**Details**:

The source counts four README sites deliberately — the count is the point of the sentence — and names one of them (the tmux-passthrough paragraph) as the *example* carrying obsolete advice. §12.5 repeats the count but then disposes only of that one paragraph ("**That paragraph** comes out with the setting"), leaving "README gains the theme setting in its place" to imply the rest. The other three each name a setting the new binary no longer honours:

- The feature-bullet line — *"owns its own light or dark canvas (auto-detected, or **pinned via `appearance`**, and honours `NO_COLOR`)"*.
- The TUI-views paragraph — *"It paints its own light/dark canvas (**set `appearance` in `prefs.json`**, or `NO_COLOR` …)"*.
- The config-file table row for `prefs.json` — *"UI preferences: last-used session-list grouping mode and the owned-canvas **`appearance` (`auto`/`light`/`dark`)**"*.

The third is the sharpest, because that same table is the README's inventory of Portal's config surface and per §5.5 now needs a **themes directory** row carrying `PORTAL_THEMES_DIR` — a user-facing documented contract the spec fixed the name of precisely so the docs could print it. Leaving the section as "gains the theme setting in its place" also risks the retained-on-disk `appearance` (§10.4) staying documented as live, which is the opposite of what §10.4 decided: it is a frozen legacy value for downgrade, not a setting to advertise.

**Current**:

> `appearance` is described in `README.md` at four places, including a paragraph recommending users pin it *"when auto-detection misfires (for example under tmux passthrough)"*.
>
> **That paragraph comes out with the setting** — and the advice is obsolete twice over, since the premise was probably never true in the first place (§8.7).
>
> README gains the theme setting in its place, pointing at `docs/theming.md`.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
