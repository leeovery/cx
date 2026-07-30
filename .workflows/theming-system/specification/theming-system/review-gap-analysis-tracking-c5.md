# Review Tracking: Theming System - Gap Analysis

## Findings

### 1. §9.2's slot-from-constant confirm defines two mutually exclusive rules for the same keypress

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.2 (the interaction model — the confirm), §9.7 (key-exclusivity, `Ctrl-C` stays live), §14A (the confirm's pinned copy)

**Details**:

Two consecutive bullets in §9.2 give contradictory dispositions for the same input:

> **`y` confirms** — the constant is cleared and the slot written, in one atomic prefs write. **Any other key cancels**, including `Esc`, which cancels the confirm without closing the panel.

> While the confirm is live it is **key-exclusive within the panel**: arrows, `Enter` and the other slot key are swallowed until it resolves.

Press `↓` with the confirm live and the first rule says the confirm cancels; the second says the key is swallowed and the confirm stays live ("until it resolves"). The same collision applies to `Enter` and to the other slot key — the three inputs named as swallowed are precisely the ones "any other key cancels" would cancel. An implementer has to pick one of three behaviours that differ visibly at the moment a write is gated: `↓` cancels and moves the cursor, `↓` cancels without moving, or `↓` does nothing at all and the confirm persists.

Two smaller edges hang off the same sentence and are worth settling with it:

- **`Ctrl-C`.** §9.7 pins `Ctrl-C` as the one key that stays live inside the panel; §9.2 says any key other than `y` cancels the confirm. So `Ctrl-C` with a confirm live either quits Portal or merely cancels the confirm, and §11.4 names quit-with-the-panel-open as a specifically tested path.
- **Case.** §14A pins the prompt as `clear constant <slug>?  y / n`, which advertises `n` but says nothing about `Y` — under "any other key cancels", `Y` cancels, which is the opposite of what a user pressing shift-y intends.

The confirm is the one affordance in the feature that gates a write §9.2 says must not happen silently, so which keys resolve it is not a detail.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §9.2's confirm now resolves on exactly three inputs — `y`/`Y` confirms, `n`/`N`/`Esc` cancels (Esc resolving innermost-first per §9.7's nesting rule), `Ctrl-C` quits — and everything else is swallowed. Removes the any-other-key-cancels vs key-exclusive contradiction and closes both the `Ctrl-C` and shift-Y edges.

---

### 2. §13.5's canonical floor set and §7.4's leg table disagree on the accent floor and on the `bg.selection` pairings

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.5 (contrast checking — the canonical rule set), §7.4 (the Nord port's verification baseline)

**Details**:

§13.5 states the rule set the auto-enumerating floor test must implement, and explicitly demotes §7.4's table to a record rather than a source: *"§7.4's table is the *Nord port's* verification record — a walk of these rules for one palette — not the rules themselves."* But the two do not describe the same rules.

**The accent floor.** §13.5's foreground-vs-canvas table floors `accent.primary` at ≥ 3.00 and puts `accent.key`, `accent.mode`, `accent.attention`, `state.positive`, `state.destructive` at ≥ 4.50. §7.4's leg table floors `accent.key` at **≥ 3.00** and the `accent.attention` bar at **≥ 3.00**, and then reads the general rule off explicitly:

> **accents carry a ≥ 3.00 UI floor**, not the 4.50 normal-text floor (`accent.mode` is the exception, floored at 4.50 because it renders preview peek chrome)

That sentence and §13.5's table cannot both be the rule. It is not an academic difference: it decides whether a future ported palette whose key-hint blue sits at 3.5 is a bundled-tier failure or a pass, and §7.4 states the port was twice found incomplete on exactly this axis. Nord happens to clear both readings (`accent.key` 4.64, `accent.attention` 8.00), so the built-in set cannot arbitrate — the test's author has to choose, and the two choices produce a different guarantee for the tier §6.4 says carries Portal's name.

**The `bg.selection` pairings.** §7.4's leg table measures `text.primary` on `bg.selection` (7.49 ≥ 4.50). §13.5's *"Foreground-on-tint pairings, all ≥ 4.50"* lists `text.on-selection`, `text.secondary`, `text.tertiary` and `state.positive` on `bg.selection` — `text.primary` is absent, and §2.5 gives it a role ("Names, wordmark, active labels, modal titles, chip text") that renders on a selected row. So the canonical list is either missing a leg the port walked, or the port walked a leg the rules do not require, and only one of those is right.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Verified against contrast_test.go: the shipped floors are per-token (only accent.violet/primary is 3.0; accent.blue/cyan and the state tokens are 4.5), so §13.5 was right and §7.4's read-off sentence was wrong. §7.4's table corrected (`accent.key` floor 4.50) and the sentence rewritten to state that accent.primary alone carries 3.00 and that the bar leg's 3.00 is a property of the pair rule not of the token. `text.primary` on `bg.selection` re-marked as walked-but-not-required with a footnote — the selected row renders `text.on-selection`, which is what that token is for.

---

### 3. "A failed commit outstanding" has no defined lifetime, so the flash that rescues it can be cancelled by any keypress

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.13 (a failed commit write), §9.2 (`Esc`), §9.8 (forced close), §14A (the flash copy)

**Details**:

§9.13 defines two things about a failed write and does not connect them:

> It **persists until the next keypress** rather than timing out like a transient flash

> **So closing the panel with a failed commit outstanding raises a main-screen flash**: `⚠ theme not saved — see portal.log`.

"Outstanding" is never defined, and the only lifetime rule in the section belongs to the *message*, not to the state. Two readings both follow from the text and diverge in user-visible outcome:

- **Outstanding == the message is showing.** Then any keypress that is not the close clears it — arrow to another theme, then `Esc`, and there is no flash. That reinstates precisely the silent revert the flash was added to prevent (the user's chosen theme drops away with no signal anywhere), which is the failure this section exists to close.
- **Outstanding == a commit has failed since the panel opened.** Then the flash fires on close even after the user retried successfully — §9.13 states commits are re-attemptable, so `d` fails, `l` succeeds, `Esc`, and the user is told the theme was not saved when it was.

Neither is obviously intended and the spec supplies no third rule (e.g. "cleared by any subsequent successful commit"). Because the whole point of §9.13 is that the state is *reported* rather than silent, the condition that drives the report is load-bearing and is currently left to the implementer.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §9.13 now defines "outstanding" as a state cleared only by a subsequent successful commit — explicitly not by arrowing (which dismisses the message but not the state, stopping the next Esc reinstating the silent revert) and explicitly cleared by a successful retry (so a failed `d` followed by a successful `l` raises no flash).

---

### 4. The mandated read-modify-write has no defined behaviour when the re-read fails to decode

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.9 (concurrent instances and prefs writes), §8.8 (`appearance` round-trip), §10.5 (the migration write), §13.6 (the new prefs + migration test)

**Details**:

§8.9 makes read-modify-write mandatory for every prefs writer — *"re-read `prefs.json` immediately before writing, mutate only its own field(s), and write the merged result"* — and §8.8 explains why the merge target matters: a field that is not carried through the decode/re-encode round trip is silently erased.

What the spec never says is what the merge target is when the re-read yields nothing usable. `prefs.json` is hand-editable by design (§8.1) and its decode is tolerant by design, so a syntactically broken file (a stray comma after a hand-edit, a truncated write) does not error — it degrades to the zero-value struct. Under the mandated rule the writer then "mutates only its own field(s)" of a struct in which every other field is already empty, and `AtomicWrite` lands that as the new file. In one `s` keypress or one theme commit the user loses `session_list_mode`, `theme_migrated`, the theme keys they did not touch, and the retained raw `appearance` — the last of which is the exact loss §8.8 calls out as *"defeating §10.4's downgrade guarantee at the moment the user is least likely to notice"*, and §13.6 describes this path as *"the one part of the feature whose failure mode is silent, permanent destruction of a user's config"*.

The choice an implementer is left to make is real and not cosmetic: treat a decode failure as a failed commit (report `⚠ couldn't save theme`, emit `theme: commit failed`, write nothing), or treat it as an empty file and overwrite. The same question applies to the migration write, which §8.9 explicitly brings under the same rule, and to a re-read that fails on I/O rather than syntax.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §8.9 now aborts the write on a re-read that fails to decode or fails on I/O — never an overwrite — treating it as a failed commit with `theme: commit failed` and the panel report, since merging into a tolerantly-zeroed struct would erase session_list_mode, the marker, untouched theme keys and the retained appearance in one keypress.

---

### 5. The panel's own `bubbles/list` is a new instance of exactly the class §11.2 inventories, and nothing says how it restyles

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §11.2 (the real risk is completeness), §11.1 (restyle vs rebuild), §9.11 (everything re-themes, panel included), §13.4 (the swap-and-diff guard)

**Details**:

§9.8 puts the panel's list on `bubbles/list` (*"Overflow: scroll, through the `bubbles/list` machinery, so `Ctrl+↑/↓` paging applies"*), and §9.11 requires the panel's own chrome to re-theme with the previewed theme, *"No exceptions"*. §11.2 then inventories the completeness risk as *"the cached styles Portal does not own — `bubbles/list`'s help styles, pagination dots, TitleBar, and both filter inputs — which are assigned once"*, and names the swap path as `applyCanvasMode`'s restyle and style re-point.

That inventory describes the pre-feature TUI. The panel introduces a **third** `bubbles/list` instance whose styles are also assigned once — at panel open — and it is the one surface where the theme changes on *every arrow keypress*. Nothing in §9 or §11 says how it is kept current: whether `applyCanvasMode` is extended to re-point the panel list's cached styles, whether the panel's list is rebuilt per keypress (which §11.1 rules out for the main list as the expensive path), or whether the panel's delegate re-derives per frame from the previewed theme and the `bubbles/list`-owned styles are simply never used by it.

The safety net does not obviously catch this either. §13.4's guard renders fixtures, swaps, and re-renders — but the panel's pagination dots only render when the panel's list paginates, so covering them needs a panel fixture with enough theme rows to overflow, which §13.3's fixture list (adaptive-pair, constant-while-previewing, invalid-row, narrow degraded) does not obviously include. §11.2's own hedge — *"These two are what was *found*, not the boundary of the class"* — points at a sweep of existing code, not at a new list the implementer is about to write.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §11.2 now names the panel's `bubbles/list` as a third instance and the worst case of the class: its delegate re-derives per frame, its bubbles/list-owned styles are re-pointed by the same restyle path (not rebuilt — §11.1 rules that out, and it would be worse on a per-keypress surface). Added the coverage consequence that one panel fixture must carry enough rows to paginate, or §13.4 is blind at the new site.

---

### 6. §13.2's retention rule contradicts itself on whether deleting and clearing out captures is this feature's work

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §13.2 (committed reference PNGs were scaffolding), §12.6 (the CLAUDE.md capture-harness correction)

**Details**:

§13.2's three-bullet retention rule ends with a bullet that negates the two above it:

> - **Everything that exists today as an image or tape is deleted** …
> - **From this feature forward, captures and the tapes that produce them are created as work proceeds, committed while they are being collaborated on, and cleared out after sign-off** …
> - **Cleaning up is not this feature's job** and is not done as we go.

Bullets 1 and 2 are both cleanup instructions addressed to this feature; bullet 3 says cleanup is not this feature's job. Read against bullet 1 it means the existing PNGs and tapes stay in the repo; read against bullet 2 it means this feature's own captures are not cleared at sign-off; read narrowly ("not done *as we go*") it means only that the clear-out is a single end-of-work step rather than continuous — which is what bullet 2 already says, making the bullet redundant rather than contradictory.

A planner cannot tell from this whether a deletion task exists at all. It also has a documentation consequence: §12.6 requires `CLAUDE.md`'s capture-harness section to be corrected on the grounds that committed reference PNGs *"read as a durable asset. It is not (§13.2)"* — a correction that is wrong if the PNGs are in fact still sitting in `testdata/vhs/`.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.2's third bullet rewritten: this feature does not take on a general repo-wide cleanup and does not clear continuously, but both stated deletions are in scope as single bounded acts — today's images and tapes at the start, this feature's own at sign-off. Removes the reading under which the PNGs stay and §12.6's CLAUDE.md correction would be wrong.

---

### 7. One pinned flash string covers two different render floors, and it describes only one of them

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.8 (geometry — width and height floors), §14A (flashes)

**Details**:

§9.8 defines two independent refuse conditions. Width: *"It **refuses only when even the minimum panel cannot render** … and then it flashes rather than opening a broken frame."* Height: *"refuse with a flash only when **header + footer + one row + one message row** cannot fit."*

§14A pins one string per trigger, and both name width:

| `t` below the render floor (§9.8) | `terminal too narrow for the theme picker` |
| Resize below the floor with the panel open (§9.8) | `terminal too narrow — theme picker closed` |

On a wide, short terminal — a two-line-tall split pane, which is a common tmux shape and exactly where Portal runs — the user is told their terminal is too narrow while it is demonstrably wide. §14A opens by stating that *"Every new user-facing string is pinned here"* and that panel copy is *"a **layout constraint** as much as a copy choice"*, so an implementer meeting the height case has no sanctioned string and either reuses a wrong one or invents copy the section says is not invented.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §14A now pins four flash strings instead of two, splitting width from height — a wide, short split pane is a common tmux shape and was previously told its terminal was too narrow.

---

### 8. §7.7 requires a chroma figure to be recorded for seven values but names no home for it

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §7.7 (the re-derivation check), §7.4 (Nord's chroma figures), §15.3 (where the vocabulary lives)

**Details**:

§7.7 makes recording an obligation, not a by-product:

> **The chroma figure is recorded for all seven values regardless of outcome**, which is also what closes a gap §7.4 opens: Nord's two corrections carry their chroma figures precisely so they are checkable if ever re-derived, and without this step MV's would be the only corrections in the built-in set without one.

The comparison it draws is to Nord's figures, which live in **this specification** (§7.4's two correction paragraphs). But §7.7 also says that if anything moves, *"§7.3's value tables in this specification are superseded by the theme files rather than being re-written here"* — so the spec is explicitly not where the moved values live, and §15.3 lists four homes for the vocabulary without a slot for derivation records.

The candidates are materially different in durability: a `#` comment in `tokyo-night-day.theme` (which is where §7.1 sends the one other non-numerically-recoverable judgement, the eyeball pins, and which `portal theme export` then ships to users byte-faithfully per §12.1), a line in `docs/theming.md`, an amendment back into §7.4, or a commit message that is gone in a year. The check is stated to be *"a finding, not a non-event"* even when it passes, so leaving the artefact homeless is what makes the passing case silently evaporate.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §7.7 now homes the chroma record in a `#` comment beside the value in `tokyo-night-day.theme` — the same home §7.1 gives the eyeball pins, and the only durable one: it is exported byte-faithfully to users, travels with the value, and survives a re-derivation that supersedes §7.3's tables.

---

### 9. The control-strip/truncate rule for a hand-edited persisted slug is scoped to the panel, while doctor renders the same string

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.5 (row rendering), §8.6 (the persisted slug is validated before use), §12.2 / §14A (doctor's advisory lines)

**Details**:

§9.5 identifies the hazard and fixes one surface:

> **A displayed slug that came from `prefs.json` is truncated and control-stripped before rendering.** It is hand-editable text drawn into a fixed-width frame, and §8.6 validates it before *use* as a path component but the unresolvable row still shows it — so a pasted newline, tab or ANSI escape would otherwise reach the panel.

The same string reaches a second surface by design. §12.2 requires doctor to *"report when a persisted theme name no longer resolves"*, and §14A pins the line as `⚠ theme <slug> (<slot>) does not resolve: <reason>` — where `<slug>` is that same unvalidated, charset-rejected persisted value (§9.4 and §12.1 both establish that a charset-rejected persisted string is reported rather than suppressed). Doctor writes it straight to a terminal, so an embedded escape sequence or newline corrupts the diagnostic output the user is reading to find the problem.

Doctor has full width so truncation is not the issue — control-stripping is. As stated, the rule is a property of the panel's renderer rather than of the value, which leaves the second consumer uncovered; making it a property of the value would cover doctor, and `portal theme export`'s `theme <slug> is not valid: bad name` line, without a second rule.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §9.5 re-scoped control-stripping from the panel's renderer to the value itself, at read time, so doctor's advisory line and export's stderr inherit it — both render the same unvalidated string. Truncation stays panel-local, since doctor and export have full width and want the whole value.

---

### 10. Whether the themes directory itself may be a symlink is left open by §5.6's "symlinked directories are not followed"

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §5.6 (enumeration rules), §5.5 (directory resolution and directory states)

**Details**:

§5.6 states a flat rule with no scope qualifier:

> **Symlinked directories are not followed.** … A **symlink whose target is a directory** is treated identically: skipped silently. What the entry resolves to is what decides, not whether a link is involved, so there is one rule rather than two.

In context it is about *entries inside* the themes directory, but nothing says so, and §5.5's directory-states table enumerates only **Absent** and **Unreadable / a regular file where a directory belongs** — a symlink-to-directory at the resolved themes path is none of those. Dotfiles users symlink `~/.config/portal` (or individual entries under it) as a matter of course, and §5.6 itself argues the symlink rules exist because *"dotfiles users are exactly who hand-authors a theme"*.

Applied to the root the rule produces the worst available outcome: enumeration yields nothing, `PORTAL_THEMES_DIR`/`XDG` resolution reports no error, so every drop-in silently vanishes and the panel shows only built-ins — the "completely in the dark" state §9.4 and §9.5 exist to prevent, with no row and no doctor line, because §5.5 makes an absent directory deliberately silent. The natural implementation (`os.ReadDir` on the resolved path, which follows the root symlink) does the right thing, so this is one clarifying clause rather than a design question — but as written the spec's only stated rule points the other way.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §5.6 now scopes its symlink rules to entries *inside* the directory and states that the resolved themes directory itself may be a symlink and is followed — not following it would make every drop-in vanish with no row and no doctor line, since §5.5 makes an absent directory deliberately silent.

---

### 11. The colour-literal guard's exclusion is specified as unchanged, but §3.2 moves the excluded package out of the guard's scope

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §13.6 (guard-test reshape — the colour-literal guard), §3.2 (the token layer moves to `internal/theme`), §2.1

**Details**:

Three statements do not compose. §2.1: *"the existing glob-based colour-literal guard in `internal/tui` continues to enforce this, excluding the `theme` subpackage"*. §3.2: *"**The token layer moves to a new leaf package, `internal/theme`** … The colour-literal guard's exclusion re-points to it."* §13.6: *"**Colour-literal guard** | Unchanged in mechanism; continues to exclude the `theme` subpackage."*

After the move there is no `theme` subpackage under `internal/tui` for the guard to exclude, so "re-points to it" implies the guard's *scan scope* widens to cover a sibling package purely in order to exempt it — which is a mechanism change, not the unchanged mechanism §13.6 asserts. The alternative reading is that the exclusion simply becomes dead configuration, which is fine but is not what either section says.

Worth resolving in one line because the answer is not neutral: under this feature `internal/theme` holds a parser and a vocabulary and, by §7.1, **no hex values at all** (they live in the embedded `.theme` files), so the exemption that existed to let `theme.MV` declare hexes has lost its reason to exist. Leaving a stale exemption in place, or widening the guard's globs to reinstate one, are opposite outcomes and an implementer currently has no basis to choose.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §13.6: the guard's scope is unchanged (`internal/tui`) and its `theme`-subpackage exemption is **deleted**, not re-pointed — §3.2 moves that package out, and widening the globs to reach a sibling in order to exempt it would be a mechanism change. The exemption has also lost its reason, since after §7.1 the new package holds no hex values at all. §2.1 and §3.2 reworded to match.
