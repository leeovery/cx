# Review Tracking: Theming System - Gap Analysis

## Findings

### 1. `bad name` carries two distinct causes but one definition and one message

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §5.6 (enumeration rules), §6.2 (reason vocabulary), §14A (doctor copy)

**Details**:
`bad name` is produced by two different failures:

1. §5.2 / §6.2 — the slug does not match `^[a-z0-9][a-z0-9-]*$`.
2. §5.6 — the *extension* casing is not exactly `.theme` (`Nord.THEME`, `nord.Theme`), so the file "never contributes a slug".

§6.2's reason table defines `bad name` as *"Slug does not match `[a-z0-9-]`"* only, and §14A pins the doctor line as `⚠ theme file <filename>: slug must be lowercase letters, digits and hyphens`. For a file named `mytheme.THEME` that message is actively wrong: the slug portion is already a legal slug, and the user is sent to fix the one thing that is fine while the real cause (extension casing) is never named. This is exactly the misdirection §9.4 and §12.1 elsewhere cite as the reason to discriminate `bad name` from `not found`.

Ordering is fine once §5.6's "never contributes a slug" clause is read carefully (a slug-less file cannot reach the `reserved name` check), but that clause is doing load-bearing work at a distance from the ladder that depends on it, and nothing in §6.2 says so.

Implementer consequence: either the reason vocabulary needs a second cause documented under `bad name` with a second pinned message, or the extension-casing failure needs its own reason (which would change the "seven reject classes" count, the panel row set, and §14A).

**Current**:
§6.2: "| `bad name` | Slug does not match `[a-z0-9-]` |"
§6.2 ladder: "1. `bad name` — the slug is checked before the file is opened, so a `bad name` file can never also report `unreadable` or anything about its contents."
§14A: "| Invalid theme file **with no slug** (`bad name`) | `⚠ theme file <filename>: slug must be lowercase letters, digits and hyphens` — the file is named, not a slug, because a `bad name` file has none (§5.2) and \"which file is it?\" is the whole diagnostic value here |"

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved by keeping seven reasons and discriminating in doctor: §6.2's `bad name` definition now names both causes (bad slug, non-exact extension casing) with the rationale for one class (same user-facing fact, no panel width to discriminate), the ladder's rung 1 reworded to check the *filename* and to note why rung 2 is then unreachable, and §14A split into two pinned messages so a `mytheme.THEME` user is told about the extension, not the slug.

---

### 2. Contradiction — what `tui.Build` takes: a theme or the nomination

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §8.4 (construction timing), §13.3 (harness changes)

**Details**:
§8.4 is explicit: *"The constructor therefore takes the loaded **nomination**, not a single theme"* — one theme under a constant, two under an adaptive pair, with the gate selecting between them after `Init`.

§13.3 is equally explicit in the opposite direction: *"`tui.Build` takes a **theme** where it takes a `prefs.Appearance` today."*

Both cannot be the signature. §8.4's later note that `capturetool --theme` *"passes the constant shape"* implies §13.3's wording is loose rather than a second decision — but §13.3 is the section an implementer building the harness reads, and the two readings produce different constructor signatures, different capturetool wiring, and a different injection point for the swap-and-diff guard's seam (§13.4). The `WithInitialMode`/`WithModePersister`/`WithAppearance` option shape already in `cmd/open.go` makes it likelier still that an implementer follows §13.3 literally.

**Current**:
§13.3: "**`tui.Build` takes a *theme* where it takes a `prefs.Appearance` today** — the exact injection mechanism this work removes. Without this the harness can only ever render the compiled-in default."

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.3's wording corrected to match §8.4: `tui.Build` takes the loaded *nomination*, with the note that capturetool always passes the constant shape.

---

### 3. The `appearance` translation's behaviour when theme keys are already set is undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §10.2, §10.3, §10.5, §8.2 (mutual exclusion)

**Details**:
§10.3 fixes the trigger as the `theme_migrated` marker *and nothing else* — deliberately not gated on the absence of theme keys. §10.2 then says `appearance: dark` ⇒ *"Write `\"theme\": \"tokyo-night\"`"*. §8.2 says committing a constant clears both slots.

Reachable sequence, entirely within the specified rules: user upgrades; runs only bootstrap-exempt/CLI commands for a while, so the migration (which per §10.5 runs *only where a TUI is constructed*) never fires; reads the new `docs/theming.md` and hand-edits `theme_dark = nord`; then launches the picker. The marker is still `false`, the retained `appearance: dark` is still there, so the translation fires and pins `theme = tokyo-night` — silently discarding the pair the user just hand-authored (and, under §8.2's write rule, clearing the slots too).

The spec does not say whether the translation:
- writes unconditionally (overwriting/clearing hand-set theme keys),
- writes only when all three theme keys are empty (but §10.3 explicitly rejects absence-gating for the *trigger* — this would be a separate no-op condition, not a re-armable trigger),
- or writes and also applies §8.2's slot-clearing.

All three are defensible readings, and they differ in whether a user loses a setting. Note this is the mirror of the exact failure §10.1 exists to prevent (silently overriding a user who expressed a preference), so it is not a hypothetical class.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §10.3 by separating the trigger from the no-op condition: the marker decides whether the translation is pending, a theme-keys check decides whether there is anything to do. If any theme key is set, only the marker is written. The reachable loss-of-setting sequence is recorded, and mutual exclusion is noted as still applying when the translation does write (with nothing ever there to clear).

---

### 4. Whether the unconditional `theme_migrated` write creates `prefs.json` on a virgin install

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.1 (`theme_migrated` contract), §8.8 (retained raw `appearance` field), §10.5

**Details**:
§8.1 requires the marker to be *"written unconditionally on the first post-upgrade prefs load, including when there is nothing to translate."* On a genuinely fresh install there is no `prefs.json` at all — today the file only appears once the user changes something (an `s` keypress). Two unstated questions follow:

1. **Does the first TUI launch on a fresh install now create `prefs.json` purely to record `theme_migrated: true`?** The rule as written says yes. That is a new side effect on a path the spec is otherwise careful about (§5.5 pointedly refuses to create the themes directory; §12.3 celebrates that the feature adds nothing to the exec path). If it is intended, it should be stated; if not, the "unconditionally" rule needs an absent-file carve-out — but then the condition is re-evaluated on every launch for a user who never writes prefs, which is exactly what §8.1 says the marker exists to stop.

2. **Serialisation of the untouched fields.** §8.8 requires `prefsFile` to keep a raw `appearance string` so the on-disk value round-trips. With no `omitempty` decision stated, the first write emits `"appearance": ""` on installs that never had the key, and `"theme": ""` / `"theme_light": ""` / `"theme_dark": ""` (the §8.1 example shows them present-and-empty). Whether empty keys are written or omitted is a visible property of a hand-editable file and affects what a downgraded binary reads.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §8.1: the marker is written only when prefs.json already exists — a fresh install has nothing to translate, and creating the file purely for a marker would be a new side effect on a path this feature otherwise keeps free; re-evaluation costs an absent-field check on a read already happening. Also pinned: empty values are omitted on write (omitempty), with §8.1's example clarified as the schema rather than the on-disk shape.

---

### 5. Doctor's `<detail>` strings are unpinned, and the zero-advisory summary form is undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §14A (user-facing copy), §12.2 (doctor), §6.2/§6.3 (detail lives in doctor)

**Details**:
§14A opens by stating *"Every new user-facing string is pinned here"*, and §6.2/§6.3 push all the discriminating detail into doctor: *"Which token is missing, which line is malformed, and which key carries a bad colour stays in doctor."* But §14A pins only the frame, `⚠ theme <slug>: <reason> — <detail>`, and one example of `<detail>` (`missing text.primary, bg.subtle`). Undefined:

- `bad syntax` detail — line number? line content? both? What identifies the offending line for a duplicate key (two lines) versus a quoted value (one)?
- `bad colour` detail — the key, the bad value, or both? Enumerated across all bad keys ("doctor enumerates within the reason") in what separator/format?
- `unreadable` detail — the OS error verbatim, or a fixed phrase?
- `not found` detail for a persisted slug — §14A pins `⚠ theme <slug> (<slot>) does not resolve: <reason>` but does not say what `<slot>` renders as for a **constant** (there is no slot).

Also unpinned: the closing summary when there are **zero** advisories. §12.2 pins `<N> checks passed · <M> advisories`; whether the advisory clause is suppressed at M=0 (and whether it is singular at M=1) is a visible string on every doctor run, i.e. the most-seen new copy in the feature.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §14A extended with the five `<detail>` formats (missing tokens, bad colour, bad syntax with line number and second-occurrence rule, unreadable verbatim OS error, reserved name via its own line), the `<slot>` parenthetical omitted under a constant, and both closing-summary forms including M=0 suppression and M=1 singular.

---

### 6. The contrast floor/leg rule set the auto-enumerating test must implement is never stated as a definitive list

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.5 (contrast checking), §7.4 (the pairing legs table), §6.4 (bundled tier)

**Details**:
§13.5 makes floor-check enrolment automatic — *"The floor test **auto-enumerates the embedded set**, so a new built-in is checked by default"* — and §7.4 leans on that: *"The floor test auto-enumerating the embedded set (§13.5) means a missed leg surfaces at implementation rather than shipping."* For those claims to hold, the test must encode a complete, theme-independent rule set.

The spec never states what that rule set is or where it is authoritative. What it does provide:

- §7.4's 15-leg table, framed as *"the port's verification baseline"* — i.e. Nord-specific, and explicitly described as having been walked by hand for this port after the per-token pass was twice found incomplete.
- Two floors read off in prose after that table (accents ≥ 3.00, `accent.mode` ≥ 4.50, the three-leg warning-band rule).
- §13.5's amendment that the reference background now resolves from the theme rather than a constant.

An implementer therefore has to reconstruct the canonical rules from the existing `contrast_test.go` plus §7.4's table and decide, unaided, whether the legs §7.4 enumerates are already in the shipped test or are additions this feature must make — and whether the per-token bands (`text.subtle` 3.00–4.49, `text.faint` 1.00–2.99, fills ≥ 1.10) are the complete set for all 19 tokens or only the subset Nord happened to stress. Given §7.4 states the port was twice found incomplete on exactly this axis, leaving the rule set implicit is the highest-value thing to pin.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.5 now states the canonical, theme-independent rule set in full — the three floors, per-token foreground rules including text.subtle's band generalised from today's light-only ceiling, the three tint pair rules, the foreground-on-tint pairings, and state.positive's dual clearance — with §7.4's table explicitly reframed as the Nord port's verification record rather than the rules themselves.

---

### 7. The panel's theme enumeration needs an injectable seam for fixtures — only the swap seam is named

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.3 (new slide-over fixtures), §13.4 (swap-and-diff seam), §7.1 (`internal/capture` import guard), §5.6/§9.4 (enumeration)

**Details**:
§13.3 requires four new slide-over fixtures, including *"an invalid-theme row"*. Rendering that fixture requires the panel's row list — built-ins ∪ directory files ∪ persisted slugs, with reasons — to come from somewhere other than a real themes directory, because §7.1 preserves `internal/capture`'s no-real-config import guard (*"`go:embed` is not config discovery"*) and §13.3 restates the invariant for `--theme` paths (*"no XDG lookup, no prefs read"*).

So the panel cannot call the production enumerator directly under the harness: there must be an enumeration seam (a `ThemeEnumerator`-shaped interface, matching the `TmuxEnumerator` / `ScrollbackReader` idiom the preview page already uses) that fixtures fake. §13.4 thought the *swap* seam worth naming explicitly — *"`internal/capture` / `tui.Build` must expose a seam to drive that from a test"* — but the enumeration seam, which is both an architectural requirement and the thing the import guard constrains, is left to the blanket *"whatever the tool needs … is in scope"*.

This also has planning consequences: the seam is what makes the panel unit-testable at all (row composition, ordering, truncation, invalid-row skip), none of which otherwise has a stated test home in §13.6.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.3 gains a `ThemeEnumerator` seam matching the existing TmuxEnumerator/ScrollbackReader idiom, named as an architectural requirement (the import guard forbids internal/capture reaching config, so the invalid-theme-row fixture cannot use the production enumerator) and as what makes the panel unit-testable at all.

---

### 8. Footer behaviour below the reference width, now that two entries are promoted to core

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §14.2 / §14.3 (decided footers, width arithmetic)

**Details**:
§14.3 measures the Sessions footer at *"the reference mock's 86-column width"* and concludes it *"fits with ~5px spare and no headroom"*. The spec then addresses only one narrowing direction — blocked-key filtering, which *"only ever removes entries, so every blocked-state footer is strictly narrower"*.

Nothing states what the footer does at, say, 70 or 60 columns, where seven entries plus a right-aligned `? help` cannot fit. §9.8 cites MV §2.7's degradation ladder as *"drop right-side header hint → compact wordmark → truncate names"* — none of which is a footer step. So an implementer must invent a rule: wrap, truncate, drop entries right-to-left, or drop `? help`. That is a visible design decision, and the spec's own arithmetic (zero headroom at the reference width) is what makes it likely to be hit rather than theoretical.

Related and cheap to settle in the same place: §9.10 names only `sessionsHelpKeymap()` as the call-site filter for a blocked `t`, but §14.2 puts `t theme` in the **Projects** footer too, so a Projects-side filter is required and unnamed.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: New §14.4: the footer drops entries right-to-left, never wraps or truncates a label (a half-rendered hint advertises nothing), and never drops `? help` — which keeps every dropped entry recoverable via the help modal. Thresholds pinned at implementation per §2.7. The Projects-side blocked-`t` filter is also named in §14.3.

---

### 9. Several `theme` log events have no declared attrs, and the dedup key is undefined for slug-less rejections

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §12.3 (the `theme` log component)

**Details**:
The attr-key vocabulary is closed and spec-governed (*"components are never invented at a call site"*), so anything not declared cannot be added at implementation. The event catalogue declares attrs for `theme: loaded` (`slug`, `slot`), `theme: enumerated` (`count`, `rejected`), `theme: rejected` (`token`), and `theme: directory unusable` (`path`, `reason`) — but leaves `theme: fallback applied` (*"Per fallback"*) and `theme: commit failed` (*"Per failed write"*) with no attrs at all. A fallback line without `slug`/`slot`/`reason` is not greppable, which is the stated reason the log earns its place.

Two further holes in the same catalogue:

- **`theme: rejected` dedup key.** The rule is *"a given slug+reason logs once"*. A `bad name` file has **no slug** (§5.6, §9.5), so the dedup key is undefined for exactly the class most likely to recur across panel opens. `path` is declared but is described as belonging to the directory event.
- **Which attr identifies a slug-less file** in `theme: rejected` at all.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §12.3 now declares attrs for `theme: fallback applied` (slug/slot/reason) and `theme: commit failed` (slug/slot/reason), and pins the `theme: rejected` dedup key as slug+reason where a slug exists and path+reason where it does not — closing the hole for the one class most likely to recur.

---

### 10. Two theme-file lexical questions the branch table does not answer

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §4.2 (lexical rules and branch table), §4.3 (value domain)

**Details**:
§4.2's branch table is explicitly *"a user-visible reason label and a test case in the loader test"*, so an unlisted branch is a test the implementer has to invent along with the behaviour.

1. **Leading whitespace before a key.** The comment rule allows `#` *"after optional leading whitespace"*, and the whitespace rule is *"Trimmed around `=`"* — which does not obviously cover indentation at line start. So `····text.primary = #ECEFF4` is either valid (whole-line trim) or `bad syntax` (no leading trim). Both are plausible readings of the rules as written, and they differ in whether an indented hand-authored file loads.

2. **Hex case normalisation at parse.** §4.3 says *"Hex case (upper or lower) is not constrained"* but does not say whether the parsed value is canonicalised or retained verbatim. It matters at three comparison sites the feature introduces or re-points: §11.4's retained startup canvas hex vs the exit-time comparison, §11.3's Bubble Tea background diffing, and §13.4's scan of rendered output for theme-A values. (§12.1's export is unaffected — it emits file bytes.)

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Both resolved: §4.2's whitespace rule now trims each line at both ends first (so indentation before a key is fine, matching the tolerance the comment rule already grants), and §4.3 pins parser canonicalisation to uppercase, naming the three comparison sites (retained startup canvas, background diffing, swap-and-diff scan) that depend on it.

---

### 11. Two enumeration/ordering edges left open

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §5.6 (enumeration rules), §9.5 (sort key and label)

**Details**:

1. **A symlinked directory named `x.theme`.** §5.6 covers symlinked *files* (followed), dangling symlinks (`unreadable`), and *real* subdirectories named `x.theme` (skipped silently). A symlink whose target is a directory falls between: *"Symlinked directories are not followed"* says what not to do but not what the row does — skipped silently like a real subdirectory, or enumerated and rejected `unreadable`. The distinction is user-visible (a row appears or does not).

2. **Sort key for a persisted slug that fails the charset check.** §9.5 claims *"the sort key is fully determined"* and enumerates: slug where one exists; filename for a `bad name` row; slug for a `not found` persisted-slug row. But §9.4 creates a fourth case — a persisted string rejected by §8.6's charset check gets a row with reason `bad name`, and it has **neither** a slug (it is not one) **nor** a filename (no file was sought). Its position in the list is therefore undefined, in the one place the spec asserts determinism. §9.5's control-strip/truncate rule tells the implementer how to *render* it but not where to put it.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Both resolved: §5.6 treats a symlink resolving to a directory identically to a real subdirectory (skipped silently — what the entry resolves to decides, not whether a link is involved), and §9.5 sorts a charset-rejected persisted string by the string itself, control-stripped, keeping the ordering total.

---

### 12. The panel's descriptor scope versus the pinned four-row footer

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.12 (descriptor-governed keymap), §9.2 (panel key table), §14A (pinned footer copy)

**Details**:
§9.12 requires the panel's keys to live in the keymap descriptor as a panel scope, with *"its vertical footer renders from the descriptor"* and `keymap_dispatch_guard_test` extended to cover them. §9.2's panel key table has six entries (`↑`/`↓`, `Ctrl+↑`/`Ctrl+↓`, `Enter`, `d`, `l`, `Esc`), while §14A pins the footer at exactly four (`⏎ set theme` / `d set as dark` / `l set as light` / `esc close`).

If the footer renders from the descriptor, the descriptor must carry arrows and paging as non-rendered entries — presumably via the existing `Core` flag, but that mapping is not stated, and the dispatch guard's contract (descriptor ↔ dispatch parity) is what makes it matter: an implementer who omits arrows/paging from the descriptor to make the footer come out right may put the guard and the descriptor out of step, while one who includes them without a flag renders a six-row footer that contradicts pinned copy.

Also unstated: whether `?` inside the panel does anything. §9.7 says everything except `Ctrl-C` is swallowed, which implies no panel help modal — worth being explicit, since §9.12's "panel scope" in the descriptor otherwise reads as though a help body exists for it.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §9.12 now requires all six keys in the descriptor with the footer rendering only the `Core` subset — arrows and paging are non-core, exactly the distinction §14.1 applies on the main footer — so the six-entry descriptor and §14A's four-row footer are both satisfied without a special case. `?` inside the panel is explicitly a no-op with the reasoning.

---

### 13. `NO_COLOR` under the adaptive pair — is detection run, and which member is active?

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §8.7/§8.8 (detection and the surviving gate), §9.10 (`NO_COLOR` blocks the panel), §11.4 (retained startup canvas)

**Details**:
The `NO_COLOR` carve-out *"survives unchanged"* (§8.8) — no canvas, no detection, colourless render. The shipped default is the adaptive pair (§8.3). The spec does not say what the theme machinery does in that intersection:

- Are both nominated themes still loaded at construction (two file reads for a render that uses no colour)?
- With detection skipped, which member of the pair is "active" — the dark slot by the standing no-answer fallback, or is the question simply moot?
- Is `theme: loaded` still emitted, and with which `slot`?
- §11.4 requires the startup canvas hex to be captured *"from the theme the gate selected"* — under `NO_COLOR` no gate runs and no canvas is painted, so what is retained, and does `RestoreTerminalBackground` still have a defined comparison value?

Nothing here is visually consequential, but each is a branch an implementer must decide and a test the named `RestoreTerminalBackground` anchor test (§11.4, §13.6) may or may not need to cover.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §9.10 now states that the theme machinery runs unchanged under NO_COLOR: both nominated themes still load (so a commit has something to persist against), the gate is skipped so the dark no-answer fallback selects, `theme: loaded` emits as normal, and the startup canvas hex is captured but unused — with the explicit note that the §11.4 anchor test needs no NO_COLOR case.

---

### 14. §12.5 names the CHANGELOG in its heading but specifies nothing for it

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §12.5 (README and CHANGELOG)

**Details**:
The section is titled *"README and CHANGELOG"* and its body covers only the README (the four `appearance` mentions, the obsolete tmux-passthrough paragraph, the replacement pointing at `docs/theming.md`). The CHANGELOG gets no content, no entry shape, and no statement of whether the `appearance` removal needs a user-visible upgrade note.

That is a live question rather than boilerplate: §10.4 keeps `appearance` on disk precisely because Homebrew downgrades are routine, and §9.9 accepts "no unset" on the grounds that `prefs.json` is hand-editable and documented — both of which lean on the user knowing the setting changed shape. Either the heading should lose the CHANGELOG, or the entry (and whether it flags the `appearance` → `theme`/`theme_light`/`theme_dark` translation) should be specified alongside the README change.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §12.5 gains a CHANGELOG subsection: the entry must cover the new setting and built-ins, that `appearance` is translated automatically so a user who set it need not act, and that the old key is left in place for downgrade and not kept in sync — all three because §10.4 and §9.9 lean on the user knowing the setting changed shape.
