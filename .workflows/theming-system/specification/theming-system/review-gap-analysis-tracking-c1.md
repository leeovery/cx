# Review Tracking: Theming System - Gap Analysis

## Findings

### 1. Panel opening state — initial cursor row and whether opening previews

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Critical
**Affects**: §9.2 (interaction model), §9.4 (the list), §9.5 (row rendering and markers)

**Details**:
§9.2 defines every key inside the panel but never says where the cursor starts when the panel opens, and §9.4/§9.5 define the list contents and markers without an initial-selection rule. Two readings are equally defensible and produce materially different behaviour:

- Cursor opens on the **active/persisted** theme's row — opening the panel changes nothing on screen; the `●` and the cursor coincide in the common case.
- Cursor opens on the **first row** (alphabetical by slug, §9.5) — opening the panel instantly re-themes the whole app to whatever sorts first, since arrowing applies live and there is no stated exception for the opening frame.

Compounding cases the rule has to answer: under an **adaptive pair** there are two marked rows (`● light` / `● dark`) — which one does the cursor land on? When the active theme is a **fallback** (§8.5) its row carries no marker at all (§9.4: "nothing ever implies the fallback was chosen") and the persisted row is unselectable — so the "start on the persisted row" reading lands on a row the arrows are specified to skip (§9.5). And it must be stated whether opening the panel applies the highlighted theme at all, or whether preview only starts on the first arrow press.

An implementer cannot build the panel without deciding this, and the wrong choice makes the mixed-mode flash (§9.2) fire on every panel open rather than only on deliberate navigation.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. `theme_migrated` is absent from the `prefs.json` schema, and the field counts contradict

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Critical
**Affects**: §8.1 (on-disk shape), §8.9 (concurrent instances), §10.3 (the trigger is an explicit marker)

**Details**:
§10.3 makes the appearance translation gate on "an explicit `theme_migrated` marker in `prefs.json`", but §8.1 — the section that defines the on-disk shape — says prefs "gains **three** keys" and shows a JSON block containing only `session_list_mode`, `theme`, `theme_light`, `theme_dark`. §8.9 then says prefs "now holds **four** independently-mutated fields". With the marker there are five keys and (arguably) five mutated fields. An implementer reading §8.1 as the schema writes the wrong file.

Beyond the count, the marker's contract is unspecified in every dimension §8.1 settles for the other keys:

- **Type and value** — boolean `true`, a version string, a timestamp, the translated-from value?
- **Tolerant decode** — what an absent / empty / corrupt / unrecognised value degrades to. §8.1's whole argument is "three flat keys keep tolerant decode as dumb as today"; a marker whose corrupt-value behaviour is undefined re-opens exactly that. Getting it wrong either re-fires the one-shot translation (the failure §10.3 exists to prevent) or permanently suppresses it.
- **Who writes it** — §10.5 gives the translation to `loadPrefsStore`, but does the marker get written even when there is nothing to translate (`auto`/absent, where §10.2's action is "Nothing")? If not, the condition is re-evaluated on every launch forever; if so, "Nothing" is not accurate.
- **Whether it participates in mutual exclusion** (§8.2) — it must not, but that is currently only inferable.

**Current**:
> ### 8.1 On-disk shape — three flat string keys in `prefs.json`
>
> `prefs.json` gains three keys alongside the existing `session_list_mode`:
>
> ```json
> {
>   "session_list_mode": "flat",
>   "theme": "",
>   "theme_light": "",
>   "theme_dark": ""
> }
> ```

and

> Before this feature `prefs.json` had one field with a production writer. It now holds four independently-mutated fields written from two surfaces

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. `portal doctor` is specified as read-only but must read prefs through the migrating loader

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §12.2 (doctor theme line), §10.5 (ownership and write-path robustness)

**Details**:
§12.2 requires doctor to "report when a persisted theme name no longer resolves", which means doctor reads `prefs.json`. §10.5 puts the appearance translation in `cmd/config.go`'s `loadPrefsStore` — the shared prefs-load entry point — and that translation **writes** `prefs.json` (best-effort). So `portal doctor`, declared "**Read-only**, with no `--fix` action", would perform a one-shot config mutation as a side effect of running a diagnosis.

This also collides with doctor's bootstrap-exempt, non-healing read-only path (the existing `doctor` contract in CLAUDE.md: "heals nothing on the read-only path"), and with §12.1's separate insistence that `portal theme export` must not have side effects.

The spec needs to say either (a) doctor reads prefs through a non-migrating path, (b) the migration is gated on the TUI path rather than on prefs load, or (c) doctor triggering the translation is explicitly accepted and the "read-only" claim is qualified. The same question applies to any other bootstrap-exempt command that ends up reading prefs.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. The migration write is exempted from the mandated read-modify-write rule without saying so

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §8.9 (concurrent instances and prefs writes), §10.5 (write-path robustness)

**Details**:
§8.9 states the lost-update hazard and mandates: "**Both writers must read-modify-write**: re-read `prefs.json` immediately before writing, mutate only their own field(s), and write the merged result." §10.5 then introduces a **third** writer (the translation) and argues concurrency is a non-issue because all instances "compute the same value from the same input", so the write is idempotent.

That argument holds for simultaneous cold launches, but not for the case §8.9 was written to close: instance A is constructed against a pre-migration `prefs.json`, the user migrates and commits `theme = nord` in instance B, and A later flushes its stale in-memory prefs as part of the translation write — reverting B's commit *and* possibly re-writing the marker state. Whether the translation write is subject to the §8.9 RMW rule is left to the implementer, and the two sections read as if each is the complete story.

Also unstated: whether the translation write and a subsequent theme commit in the same process are one write or two, and whether the RMW re-read is expected to detect that another instance already migrated (marker now present) and skip.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 5. A charset-rejected persisted slug is assigned two different reasons

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §6.2 (reason vocabulary), §8.6 (persisted slug validation), §9.4 (the list)

**Details**:
§6.2 defines `bad name` as "Slug does not match `[a-z0-9-]`" and `not found` as "A slug named by `prefs.json` with no corresponding file". §9.4 then says the `not found` row "covers a deleted file, a renamed file, a typo in `prefs.json`, and **a persisted slug rejected by the charset check before any file is sought**" — which is precisely the `bad name` condition, surfaced under the `not found` label.

The two readings give the user different information for the same input (`"theme": "My Theme"` reads either "you typed a name that isn't legal" or "the file is missing"), and doctor's detail line (§6.3) would have to pick one. Since these labels are user-facing and enumerated as a closed set, the mapping needs to be single-valued.

**Current**:
> - A persisted slug with **no file** gets a row too — marked, unselectable, reason `not found`. Same shape as an invalid file: the user sees what is set and why it is not applying. This covers a deleted file, a renamed file, a typo in `prefs.json`, and a persisted slug rejected by the charset check before any file is sought.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 6. No precedence rule when a theme fails more than one validity check

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §6.1 (validity rule), §6.2 (reason vocabulary), §9.5 (row rendering)

**Details**:
The panel row carries **one** terse reason (§9.5), but multiple reasons routinely apply to the same file: a file named `My Theme.theme` that is also missing tokens (`bad name` + `missing tokens`); a file with a duplicate key *and* an invalid hex (`bad syntax` + `bad colour`); a `nord.theme` drop-in that is both reserved-named and malformed. The spec never states the evaluation order or that the first failure short-circuits.

This also determines observable behaviour beyond the label: if `bad name` short-circuits, the file is never opened, so a `bad name` row can never *also* report `unreadable`; if it does not, the implementation must decide which of two true reasons to render. Doctor's detail line (§6.3, "*which* token is missing, *which* line is malformed") has the same question — does doctor enumerate all failures or only the first?

Related, within §4.2/§4.3: it is unstated whether the "all 19 present" check runs before or after per-value validation (`missing tokens` vs `bad colour` for a file that is both).

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 7. "Reports inside the panel" — no reporting surface is defined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.13 (a failed commit write), §9.1 (shape and placement)

**Details**:
§9.13 requires a failed `Enter`/`d`/`l` write to "**Report inside the panel**" — but §9.1 defines the panel as a header, a list, and a vertical keymap footer, with no message region, and explicitly notes the notice band is *behind* the panel on the left (§9.7). So the one affordance the spec names has no specified home.

Unspecified: what the message says, where it renders (a row above the footer? replacing the footer? an inline row?), whether it is transient like Portal's existing flash or persistent until the next keypress, how it fits in ~24–30 columns, and whether it is glyph-backed (Portal's convention for state that must survive colourless — though §9.10 blocks the panel under `NO_COLOR`, `⚠` is the established marker).

This is a small surface but it is the *only* signal for the state §9.2's picker idiom was chosen to make non-silent, so leaving it undesigned undercuts the decision it serves.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 8. Invalid-row content does not fit the fixed panel width, and no truncation priority is given

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.5 (row rendering and markers), §9.8 (geometry)

**Details**:
§9.8 fixes the panel at "~24–30 columns (name, markers, slot indicators, border, padding)" and says long slugs truncate with `…`. §9.5 requires an invalid row to render `⚠` plus a terse reason. The longest reasons are `missing tokens` (14 cols) and `reserved name` (13 cols); adding `⚠ ` and a border/padding leaves roughly 6–12 columns for the slug at the preferred width, and fewer at the minimum. A real drop-in name (`tokyo-night-lee`) would be truncated to near-nothing.

The spec does not say which element yields: does the reason truncate, the slug truncate, or does an invalid row wrap to two lines? Two lines would break the one-row-per-delegate invariant that MV's grouping work established as load-bearing for `bubbles/list` pagination (and §9.8 relies on `bubbles/list` paging plus the invalid-row skip). It also does not say what happens to the badge column on an invalid row that is *also* the persisted slot (`● dark` + `⚠ not found` + slug in ≤30 columns is not achievable).

Because §9.8 defers only the *thresholds* ("exact thresholds are pinned at implementation"), not the layout rules, an implementer has to invent the composition rule.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 9. A `reserved name` rejection produces two rows with the same visible label

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §5.4 (no shadowing), §9.4 (the list), §9.5 (row rendering)

**Details**:
§9.4 gives a row to every `*.theme` file **plus** every built-in. A drop-in `nord.theme` is rejected with `reserved name` (§5.4) but still gets a row — and its slug *is* `nord`, identical to the built-in's. The panel therefore shows two rows labelled `nord`: one valid and selectable (the built-in), one `⚠ reserved name` (the user's file). §9.5 additionally makes built-in rows "deliberately indistinguishable from drop-in rows", so nothing on screen tells the user which is which.

Unspecified: how the two rows sort relative to each other under "alphabetical by slug"; whether the rejected row is labelled by filename instead (the §9.5 escape used for `bad name` rows); and whether the reason text is expected to name the conflict (§5.4 says "a message naming the conflict", but §6.2's terse label is just `reserved name`, and §5.4 also says the reserved set is *not* discoverable from the UI).

This is the single case where the panel row is the user's only feedback for a rejection class the spec chose to make invisible elsewhere, so ambiguous labelling has more weight here than elsewhere.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 10. The panel is silent when the themes directory is unreadable

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §5.5 (directory resolution), §9.4 (the list), §6.3 (where rejection surfaces)

**Details**:
§5.5 routes an unreadable themes directory (or a regular file where the directory belongs) to "a **doctor advisory line** and a **log entry**" — deliberately not to the panel. But §9.4's per-file rows exist precisely so the user "sees 'there's my theme, it's registered, but it's invalid' rather than being completely in the dark", and §6.3 assigns the panel the job of telling the user "their file did not work and it is not their imagination".

With an unreadable directory, every drop-in silently disappears from the panel and the user sees only built-ins — the exact "completely in the dark" state, in the surface the spec chose to prevent it. The panel is also where the user is standing when they notice (they opened it to pick a theme).

The spec should state what the panel does here: nothing (accepted, with the reasoning), a single row/line, or the same `⚠` treatment. It should also state whether the persisted-slug rows (§9.4) still render when the directory read fails — they must, or a user with an unreadable directory loses the `●` entirely.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 11. Coherence between the construction-loaded active theme and the panel's re-enumeration

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §5.7 / §5.8 (lazy discovery, re-read on open), §8.4 (construction timing), §9.2 (`Esc` semantics)

**Details**:
Three rules meet without being reconciled:

- §8.4 loads the nominated theme(s) **once at construction**.
- §5.8 re-enumerates and re-parses the directory **on every panel open** (explicitly to support "copy a built-in, edit it, see it, without relaunching").
- §9.2 makes `Esc` "discard an uncommitted preview and render **the resolved persisted state**".

So after the user edits their active theme's file mid-session, the panel holds a *fresher* parse than the model does, and `Esc` has two possible sources of truth. Concretely: the persisted theme is `nord`, the user edits `nord.theme` and introduces a bad hex, opens the panel (row now `⚠ bad colour`, unselectable), and presses `Esc`. Does Portal keep rendering the construction-loaded `nord` (stale but valid), or apply the §8.5 fallback (`tokyo-night`) because the persisted theme is now unloadable? The mirror case is benign but equally undefined: the user fixes a previously-invalid theme and expects the edit to take effect.

Related and also unstated: whether the panel's enumeration parse is retained so arrowing previews from values already in hand, or whether each preview re-reads the file (which decides whether the swap is the O(1) restyle §11.1 promises).

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 12. Footer visibility of `t` (and `m`) when the key is blocked

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §14.1 / §14.3 (footer keymap revision), §9.10 (`NO_COLOR`), §9.7 (blocked-`t` feedback)

**Details**:
§9.10 filters the `t` row out of `?` **help** while blocked under `NO_COLOR`, via the existing call-site filter that already drops the `m` row. §14.3 repeats only the help half. But §14.1 **promotes both `t` and `m` to core**, which puts them in the *footer* for the first time — a surface neither key occupied when the "filter it out of help while blocked" precedent was set.

So under `NO_COLOR` (or, for `m`, an unsupported terminal) the footer would advertise `t theme` / `m multi` while pressing them only produces a blocked flash. The spec does not say whether the footer is filtered in lockstep with help.

This matters twice over: it is a live-inconsistency question (help says one thing, footer another), and §14.3's width arithmetic is measured with both entries present — if the footer is filtered, the blocked-state footer is a different width and the "~5px spare, no headroom" budget is only pinned for the unfiltered case.

**Current**:
> The `t` row is filtered out of `?` help while blocked under `NO_COLOR` (§9.10); `m`'s existing filter under an unsupported terminal is unchanged.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 13. Parser lexical rules leave several branches undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §4.2 (lexical rules), §4.3 (value domain), §4.4 (what a theme file may contain)

**Details**:
§4.2/§4.3 are close to a parser spec but stop short on branches an implementer must decide, each of which changes which reason (§6.2) the user is shown:

- **Empty value** (`text.primary =`) — `bad syntax` (not a well-formed pair) or `bad colour` (a value that is not `#RRGGBB`)?
- **Duplicate unknown key** — §4.2 rejects duplicate keys, §4.4 ignores unknown keys. Two `foo = x` lines: rejected or ignored?
- **A known key duplicated with the same value** — still rejected? (§4.2's reasoning is about *conflicting* values.)
- **Wrong-case key** (`Text.Primary`) — §4.2 matches case-sensitively, so it is an unknown key, silently ignored, and the file then fails as `missing tokens`. That is defensible but the surfaced reason actively misdirects; worth stating as intended.
- **Trailing whitespace after a value**, and internal whitespace inside a value — §4.2 trims "around `=`" only.
- **Key with no `=`**, and **`=` with no key** — presumably `bad syntax`; unstated.
- **Wrong-length hex** (`#FFF`, `#FFFFFFFF`) — §4.3 excludes `#RGB` shorthand but does not name the reason (`bad colour`, presumably).
- **A file that is empty, or contains only comments** — `missing tokens`, presumably; unstated.
- **Line-ending/BOM interaction**: §4.2 tolerates CRLF and strips a BOM — whether the BOM strip applies only at file start (so a BOM mid-file is `bad syntax`) is unstated.

These are cheap to settle and expensive to guess, because each one is a user-visible reason label and each is a test case in the embedded-set validity test (§7.6).

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 14. §7.7's re-derivation check has no threshold for "moved materially"

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §7.7 (MV's erratum values), §7.3 (Tokyo Night values), §13.5 (contrast checking)

**Details**:
§7.7 makes the built-in-set decision **conditional** on a re-derivation check the implementation owns: re-derive MV's six corrected light values in Oklab, "measure chroma loss, and give a **fresh visual gate** to any that moved materially". No threshold defines "materially", and no acceptance criterion says what a *pass* looks like.

This is the one place where the spec hands implementation an open decision that can change shipped colours, move `TestLightSurfaceTintsPinned`'s eyeball-established pins, and falsify §7.3's "the existing MV values move across unchanged". Without a threshold the check has no determinate outcome — two implementers can reach opposite conclusions on the same measurement, and there is nothing to record as "checked, nothing moved".

Also unstated: whether a value that *does* move requires re-running the two light-tint tests' pins, and what happens to §7.3's value tables in this spec if it does (are they re-written, or does the theme file become the sole record?).

The Nord port sets a usable precedent for the *derivation* method (§7.4: Oklab ΔE 0.018 cited as "essentially imperceptible"), which suggests a ΔE band is available as the threshold.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 15. Slug rule is under-specified at the edges, and a persisted slug is rendered unsanitised

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §5.2 (slug charset), §8.6 (persisted slug validation), §9.4 (the list)

**Details**:
"A slug must match `[a-z0-9-]`" is a character class, not an anchored pattern, leaving three edges open: is the empty slug legal (a file named exactly `.theme`, or `"theme": ""` — which §8.1 uses as the *unset* sentinel, so the two must not collide)? Are leading/trailing hyphens legal? Is there any length bound (relevant because §9.8 truncates at ~24–30 columns and §5.1 makes the slug the durable identity)?

Separately, §9.4 renders "any slug named in `prefs.json` that has no file" as a panel row. That string comes from a hand-editable file and is *not* charset-validated before display (§8.6 validates it before **use** as a path component, then treats it as unresolvable — but the unresolvable row still shows it). A pasted value containing a newline, a tab, or an ANSI escape would be drawn into the panel's fixed-width frame. Stating that the displayed value is truncated and control-stripped closes it.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 16. Built-in vs. user-directory precedence on the by-name construction path

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §5.4 (no shadowing), §5.7 (discovery is lazy), §8.4 (construction timing)

**Details**:
§5.4's no-shadowing guarantee is stated as a property of *enumeration* ("a user file whose slug collides with a built-in is rejected"), but §5.7 says construction "loads **only the nominated themes by name** — one file read... **No enumeration**". On that path there is nothing to enumerate and therefore no collision to detect unless the loader resolves the embedded set **first** and never looks in the user directory for a reserved slug.

The guarantee §5.4 exists to protect (the fallback cannot itself be broken) depends entirely on that ordering, and it is the construction path — not the panel — where the fallback is resolved. Stating the resolution order explicitly ("a nominated slug resolves against the embedded set first; a reserved slug never reads the themes directory") makes the safety property implementable rather than inferable.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 17. Whether a parsed `Theme` carries its own slug

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §3.2 (Go-side data shape), §5.1 (the filename is the identity), §12.3 (log events)

**Details**:
§3.2 defines `Theme` as "a struct of 19 named `Token` fields with a stable-order `All()` accessor" — no identity field — while §5.1 makes the slug the durable identity Portal persists, displays and logs. Several consumers need slug and palette together: `theme: loaded` logs "resolved slug(s)" at construction (§12.3), the panel needs the active theme's slug to place the `●` (§9.5), and `capturetool --theme` accepts a slug *or a path* (§13.3) — the latter having no slug at all.

Whether the slug rides on `Theme`, is carried alongside it, or is held only by the model is a data-shape decision left to the implementer; it also decides what `--theme <path>` produces for a value the panel/log would otherwise expect.

Related and equally small: `All()`'s "stable order" is asserted but never defined (the §2.4 table's 1–19 numbering is the obvious candidate), and the swap-and-diff guard (§13.4) constructs synthetic themes in-test — which needs some construction route for a type §3.2 otherwise defines as "the parse result of a theme file".

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 18. What actually happens if an embedded built-in fails to parse at runtime

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §7.6 (the build-time guarantee)

**Details**:
§7.6 removes the runtime last-resort palette and says a binary somehow shipped with a broken default "fails **loudly at startup**", noting `main.go` owns a panic-recovering exit with a `process: panic` marker. That describes the *outcome* but not the mechanism: does the loader panic on an embedded parse failure, or return an error that some caller escalates? Which caller? Is it startup-eager (validate the embedded set at init) or only at the moment a fallback is needed?

It also does not say what the user sees. "Fails loudly" via the panic path means Portal dies with a Go panic trace, which is a different user experience from a one-line fatal message. Given this path is by design unreachable, a sentence pinning the mechanism is enough — but the implementer currently picks between panic and error-return with no steer.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 19. The panel header's content is implied but never specified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.1 (shape and placement), §9.8 (geometry)

**Details**:
§9.1 says "**No theme count in the panel header** — noise at this list size", which establishes that a header exists but says only what it does *not* contain. Its label text, styling (which of the 19 tokens), and whether it is separated by a rule like the Sessions title are unspecified. §9.8's minimum-height rule ("refuse with a flash only when header + footer + one row cannot fit") makes the header's row cost load-bearing for the degradation thresholds.

The Paper frames show a header, but §9.14/§15.4 are explicit that the frames are "reference, never truth", so they cannot settle it.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 20. Committing a slot from a constant silently discards the constant

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.2 (interaction model), §8.2 (two states), §8.3 (partial pairs do not exist)

**Details**:
Three rules compose into an outcome the spec never names. A user on `"theme": "nord"` in a dark terminal presses `l` to set a light theme for later. Per §9.2 the constant is cleared; per §8.3 the untouched `theme_dark` slot holds the *shipped* default. On `Esc` the resolved theme is `tokyo-night`, not `nord` — the user's constant is gone and their visible theme changed, from a keypress §9.2 describes as "changes nothing on screen".

Each rule is individually decided and defensible, and §9.9 already accepts a related sharp edge ("no unset"), so this may well be accepted too — but it is currently only derivable by composing three sections, it contradicts §9.2's own reassurance that committing to a non-active slot is inert, and it is the most likely way a user loses a setting they chose. Naming it (as §9.9 names its own residue) would make the behaviour deliberate rather than emergent, and it is a candidate line for `docs/theming.md` (§12.4).

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
