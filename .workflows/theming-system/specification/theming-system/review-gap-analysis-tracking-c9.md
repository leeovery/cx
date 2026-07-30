# Review Tracking: Theming System - Gap Analysis

## Findings

### 1. The commit RMW is owned by `cmd`, but `prefs` exposes no API that a `cmd`-side read-modify-write can be built on

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.9 (concurrent instances and prefs writes), §10.5 (ownership and write-path robustness), §8.8 (the retained raw `appearance` field), §13.6 (the new prefs + migration test)

**Details**:
§8.9 places the commit write in `cmd`: *"the persister behind the seam resolves the path, performs the RMW via the non-migrating read, and is the emission site for `theme: commit failed`"*. §10.5 places the migration write in `cmd/config.go`'s `loadPrefsStore` and states *"`prefs` stays dumb."* Both writers must therefore, from outside the `prefs` package: decode the whole file, mutate one or two fields, preserve every other field including the raw `appearance` (§8.8) and `theme_migrated`, and write atomically.

`prefs` as it stands cannot support that. Its file struct (`prefsFile`) is unexported, its `readFile` helper is unexported, and its only write entry points are field-specific (`SaveSessionListMode`, `SaveAppearance`) — each of which performs its *own* internal read-modify-write. There is no exported whole-record load, no exported record type, and no exported save.

So an implementer must invent the boundary, and the two obvious shapes have opposite consequences:

- **Export a record type plus `Load`/`Save` from `prefs`** — `cmd` genuinely performs the RMW as §8.9 says, but `prefs` now exposes a mutable whole-file API that any caller can clobber the file with, and the "stays dumb" claim starts to mean "dumb *and* wide open".
- **Add field-specific `SaveTheme` / `SaveThemeSlot` / `SaveMigrationMarker` methods to `prefs`** — the RMW stays inside `prefs` (matching how `SaveSessionListMode` already works and keeping the merge single-sited), but then `cmd`'s persister does *not* perform the RMW, and §8.9's abort-versus-create discrimination (§8.9's absent-vs-unusable rule) and its decode-failure abort have to be implemented inside the leaf that must not log — while `theme: commit failed` still has to be emitted from `cmd` on whatever error comes back.

The choice also decides where §8.9's two hard rules live (create on absent, abort on undecodable) and therefore what §13.6's new prefs test is actually testing. Nothing in the spec settles it, and §8.9/§10.5 are the sections whose failure mode the spec itself calls *"silent, permanent destruction of a user's config"* (§13.6) — which is precisely the wrong place for the package boundary to be left to inference.

Note also that §8.9's rule reaches the *existing* `s`-key mode persister ("Every writer must read-modify-write"), so whichever shape is chosen, `SaveSessionListMode` has to be brought onto it too — another consequence that follows from the boundary decision rather than being stated.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. What the panel does with the merged bytes its own commit RMW just read is undefined, and the two readings contradict each other

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.9 (RMW), §8.4 (the panel uses the construction-time prefs snapshot), §9.2 (a successful commit recomputes the panel's full row set), §9.5 (badge derivation)

**Details**:
Two decided rules meet at the commit keypress and the spec never says which governs.

§8.4: *"The panel uses the construction-time prefs snapshot; it does not re-read `prefs.json` on open… Re-reading it would let another instance's commit silently change what this panel shows and marks — the cross-instance sync §8.9 explicitly declines."*

§8.9: *"Every writer must read-modify-write: re-read `prefs.json` immediately before writing, mutate only its own field(s), and write the merged result."*

So on `Enter`/`d`/`l` the panel's own write path **does** re-read the file, and the merged result is in hand. If instance B committed `theme_light = nord` five minutes ago, that value is now sitting in the bytes this instance just read. §9.2 then requires a successful commit to *"re-derive the union (§9.4), re-sort it, and re-anchor the cursor"* — from state the spec does not identify:

- **Re-derive from the merged bytes** — badges and rows silently jump to another instance's choices at the moment the user presses a key, which is exactly the cross-instance sync §8.4 declines, arrived at by a different route.
- **Re-derive from the construction snapshot plus only this instance's mutation** — consistent with §8.4, but the panel's displayed badges now knowingly disagree with the file it just wrote (the `●` claims `theme_light` is one thing while disk says another), and §9.5's *"the `●` means 'what is set'"* becomes false for that slot.

Both are defensible and an implementer will pick one silently. The consequence is user-visible (badges moving, or badges lying) on the panel's most common action, and it is not recoverable from §8.4's rule because §8.4 scopes itself to *panel open*, not to the write path.

The same question applies to the migration write, whose RMW re-read (§8.9) can likewise observe another instance's theme keys — there §10.3/§8.9 do answer it (the no-op condition is evaluated at the re-read), which makes the panel's silence on the identical question more conspicuous rather than less.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. §13.6's test inventory names a new test for every risk area except the panel itself and the nomination-resolution path

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.6 (guard-test reshape), §13.3 (the `ThemeEnumerator` seam), §9.2 / §9.4 / §9.5 / §9.13 (panel behaviour), §8.4 / §8.5 / §8.6 (nomination resolution and fallback)

**Details**:
§13.6's table is the feature's test inventory — it is where a planner reads off the test work, and it is exhaustive enough to name the loader/parser test, the prefs + migration test, the embedded-set validity test, the swap-and-diff guard, the `RestoreTerminalBackground` anchor test and the docs token-table guard, each with a justification for why it exists. Two bodies of specified, determinate behaviour have no entry:

**The panel.** §13.3 says the `ThemeEnumerator` seam *"is also what makes the panel unit-testable (row composition, ordering, truncation, the invalid-row skip), none of which otherwise has a test home"* — and then no test is added to §13.6, so that sentence describes a possibility rather than a commitment. The panel carries a large amount of exactly-specified, purely-deterministic behaviour that nothing else covers: §9.5's sort key rules including the guaranteed `reserved name` / built-in tie and its "built-in sorts first" resolution; the four-way row-composition priority and the truncation floor; the three-row badge derivation table (including the shipped-default row that §9.5 calls out as the most common install); §9.4's union and its one-slug-one-row rule; §9.2's commit recomputation and identity-anchored cursor; the confirm's three-input resolution and swallow-everything-else rule; and §9.13's outstanding-failure state machine (persist-until-keypress, discharge-on-flash, cleared only by a subsequent successful commit). The only panel entry in §13.6 is `keymap_dispatch_guard_test`, which covers the descriptor, not any of this.

**Nomination resolution and fallback.** §13.6's *"Embedded-set validity + fallback-slug resolution"* entry is §7.6's build-time guarantee — it proves the fallback *slugs* resolve within the embedded set. It does not cover §8.5's per-slot mode-matched fallback selection, §8.4's embedded-set-before-directory ordering (the ordering that carries §5.4's no-shadowing safety property on the path that matters), or §8.6's charset validation of a persisted slug before it is used as a path component. The loader/parser test covers the charset rule as a *rule*; nothing covers it as applied to a persisted value, which is the path where `../something` would otherwise become a path component.

Since §13.6 is where the test work is enumerated, an omission there is an omission from the plan — and both bodies of behaviour are pure functions of injected state, so they are cheap to cover and expensive to leave to inspection.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. `portal doctor` can report the same slug twice, and nothing says whether it should or how the advisory count treats it

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §12.2 (doctor's theme lines), §14A (doctor copy, the `<M>` advisory count), §9.4 (one slug is one row, always)

**Details**:
§12.2 gives doctor two independent theme responsibilities: *"Scans the themes directory and reports any file failing validity"* and *"Reports when a persisted theme name no longer resolves."* For the single most likely failure — the user's selected theme is the broken one — both fire on the same slug, and §14A pins two different lines for it:

- `⚠ theme mytheme: missing tokens — missing text.primary`
- `⚠ theme mytheme (dark) does not resolve: missing tokens`

The spec does not say whether both are emitted, whether one subsumes the other, or how `<M>` counts them (two advisories for one broken file, or one). This matters more than it looks because the panel — the other surface rendering the same union — goes to explicit trouble to guarantee the opposite: §9.4's *"One slug is one row, always"*, argued from the concrete harm of a slug appearing twice. Doctor is specified with no equivalent rule, so the two surfaces would disagree about the same condition, and §14A's summary count would be inflated by user content that is one problem.

There is a defensible case for both lines (one says "this file is broken", the other says "and it is the one you selected"), which is why it needs deciding rather than inferring — an implementer reading §9.4's principle would suppress the duplicate, and an implementer reading §12.2's two bullets literally would not.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 5. `capturetool --theme <path>` is specified to warn on the filename reasons and, in the adjacent sentence, never to derive a filename from a path

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §13.3 (harness changes — the `--theme` flag), §3.2 (`Theme` carries no identity field), §6.2 (the reason ladder)

**Details**:
Two consecutive bullets in §13.3 cannot both hold. The first says only the content reasons apply to a path and closes with *"nothing derives a slug from it"*; the second requires `bad name` and `reserved name` — both of which are decided **from the slug alone, before the file is opened** (§6.2 rungs 1 and 2) — to be evaluated and warned about.

To warn `reserved name` on `--theme ./nord.theme`, the tool must derive a candidate slug from the basename and compare it against the built-in set. That is precisely what the preceding sentence says nothing does.

The ambiguity is not academic in either direction:

- Read as written, an implementer implements no filename check at all, and the warning silently does not exist — losing what §13.3 calls *"the one place a fatal filename is worth catching before the file reaches the themes directory"*, on the flag whose stated purpose is being the only visual-verification route for a drop-in author.
- Read the other way, it is also unstated whether the filename warnings apply to the **slug** form of the flag as well. They must not: `--theme nord` names a built-in, so a literal `reserved name` check on the slug form would warn on the normal, documented invocation.

The fix is a sentence, but which sentence it is changes what gets built.

**Current**:
> - **Only the content reasons apply to a path** — `bad syntax`, `bad colour`, `missing tokens`, `unreadable`. **Invalid input is a hard error** with the §6.2 reason and a non-zero exit, never a fallback: silently rendering the wrong theme at a visual gate is precisely the failure this tool exists to prevent. An explicit path may carry any extension, since nothing derives a slug from it (§3.2).
> - **The filename reasons — `bad name` and `reserved name` — warn on stderr but do not block.** Blocking would break the workflow §12.1 publishes, since an exported built-in is a reserved slug until the user renames it. Warning is what the flag's stated purpose demands: it is the only visual-verification route for someone authoring a drop-in, so it is the one place a fatal filename is worth catching before the file reaches the themes directory.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 6. The pinned `⚠ themes dir unreadable` row is the widest thing in the panel and is the one row with no degradation rule

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.5 (row composition, the pinned directory row), §9.8 (width degradation), §14A (pinned panel copy)

**Details**:
§14A pins the row's copy as `⚠ themes dir unreadable` — 23 columns of content. §9.8 sets the panel's *preferred* width at ~24–30 columns and requires it to **shrink to a minimum below that** before refusing, with the exact thresholds pinned at implementation. After the left border and padding, the content column at the preferred width is already around 21–27 columns; at the minimum width it is necessarily narrower.

§9.5's composition rules resolve this collision for every other row — a four-element priority order, a labelled truncation floor of three characters plus an ellipsis, and the explicit note that *"below the label's truncation floor the panel is already at §9.8's refuse threshold"*. The pinned row is outside all of it: it has no label, no badge and no reason, so none of the four priorities apply, and §9.5's floor argument does not transfer because the row's width is a fixed string rather than a function of user content.

It is also the row that must not degrade into nonsense: §9.5 justifies it as the thing standing between the user and the *"completely in the dark"* state, in the surface chosen to prevent it, and §13.3 mandates a fixture for it specifically because *"the latter has its own placement rule, token and pinned copy, and no other way to be checked"*. And §9.5's one-line-per-row invariant forbids the obvious escape of wrapping.

So an implementer at the minimum width must choose between truncating pinned copy (to what — `⚠ themes dir unre…`?), overflowing the frame, or letting the row set the panel's real minimum width. Any of the three is a decision the spec otherwise makes for every comparable case.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 7. The mandated adaptive-pair panel fixture has no defined relationship between its declared keys, its `--theme` palette, and its cursor

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §13.3 (new panel fixtures; a fixture declares its own raw persisted theme keys), §9.2 (opening state and the cursor invariant), §8.4 (`capturetool` passes the constant shape)

**Details**:
§13.3 establishes three separate inputs to a panel fixture and reconciles two of them: `--theme` pins the palette, *"a fixture declares its own raw persisted theme keys, independently of `--theme`"*, and the faked `ThemeEnumerator` supplies the rows. The third relationship — where the **cursor** goes — is left to §9.2, whose rule cannot be evaluated in the harness.

§9.2's rule for the adaptive case is *"the row for the slot currently in force — the light slot in a light terminal, the dark slot otherwise"*, resting on the gate. `capturetool` deliberately runs **no gate** (*"a pinned single theme, no gate, no wait, which is what makes captures byte-deterministic"*, §8.4). Reading "otherwise" as the default gives the dark slot, which is derivable — but nothing then ties the palette to it, and §9.2's invariant is stated in absolute terms: *"the cursor is always on a selectable row, and that row is always what is painted behind the panel."* A fixture declaring `theme_light = tokyo-night-day`, `theme_dark = nord` while `--theme` renders Tokyo Night Day produces a frame where the cursor sits on `nord` and the canvas is the day palette — internally contradictory, and indistinguishable from a correct frame to anyone reviewing it.

That matters disproportionately for this fixture: §9.14 identifies the slot half as the part of the panel with **no prior art anywhere**, and §13.1 makes the harness the only route to seeing it before release. The fixture whose whole job is to let a human judge the badge/cursor vocabulary is the one whose badge/cursor relationship is undefined, and §13.4's guard cannot report it (it enumerates fixtures and diffs colours; a coherent-looking wrong frame passes).

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 8. Quitting with a failed commit outstanding has no defined outcome, on the one path where the report cannot be deferred

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.13 (a failed commit write), §9.7 (`Ctrl-C` stays live inside the panel), §9.8 (forced close)

**Details**:
§9.13 builds a small state machine specifically so a failed commit cannot be silently lost: the message persists until the next keypress, *"outstanding" is a state, not a message*, arrowing away does not clear it, `Esc` raises a main-screen flash which discharges it, and §9.8's forced close is resolved explicitly (the commit flash wins over the geometry flash, and the state is discharged).

Every exit from the panel is enumerated except the one §9.7 keeps live inside it: **`Ctrl-C` quits Portal**. On that path the panel never "closes" in the §9.13 sense, no main-screen flash can be raised because the main screen is going away, and the process exits — so the outstanding state is destroyed undelivered. That is the exact outcome §9.13 exists to prevent, reached by the one key the panel is specified to always honour, and it is reachable in two keystrokes from a failed write.

It may well be acceptable (the `theme: commit failed` WARN is in the log either way, and §9.13's report is a UI affordance that has nowhere to render at exit) — but §9.13 resolves the analogous forced-close collision explicitly rather than leaving it to fall out, and this path is not mentioned at all. Either it is accepted with the log as the record, or the quit path carries the report some other way (Portal already has a post-TUI stderr warning channel via `warning.WriteLines`); an implementer has no basis to choose.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 9. Whether the slide-over animates is never stated, and the name implies it does

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.1 (shape and placement), §11.1 (speed is a non-issue)

**Details**:
The panel is called a "slide-over" throughout, and §9.1 justifies the left-border-only chrome so *"it reads as a slide-over rather than a floating dialog"* — a description of a motion idiom. Nothing says whether opening and closing are instantaneous or animated over some frames.

Portal has exactly one piece of animation today (the loading page, which §9.6 relies on being *"inert by design"*), so an instant appearance is the overwhelmingly likely reading — but it is a reading, and the spec is otherwise explicit about frame-level behaviour on this surface: §11.1 pins the swap cost at *"one ordinary keypress plus the style re-point"*, §11.3 pins OSC 11 emission to distinct-canvas changes only, and §9.1 pins that the main screen is *"deliberately not re-laid-out while the panel is open"*. An animated open would interact with all three (repeated OSC 11 emissions during a canvas-bearing slide, frames rendered at intermediate panel widths that no fixture covers, and a `t`-then-immediate-`Esc` race), which is a reason to state the answer rather than leave the naming to imply one.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:
