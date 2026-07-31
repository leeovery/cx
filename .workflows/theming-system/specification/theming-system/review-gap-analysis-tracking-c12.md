# Review Tracking: Theming System - Gap Analysis

## Findings

### 1. The mandated abort-on-undecodable RMW cannot be implemented against the tolerant decode the spec elsewhere requires

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Critical
**Affects**: §8.9 (Concurrent instances and prefs writes), §8.1 (On-disk shape), §13.6 (Prefs + migration test)

**Details**:
§8.9 makes every writer read-modify-write and splits the re-read's outcome three ways: absent → create; present and usable → merge; **present but unusable — "a decode failure or an I/O failure" — abort**. §8.9 also states the abort is a *persistence semantic* living inside `prefs` beside "the decode they depend on".

But the spec pins prefs decode as tolerant in two places, and one of them is the abort bullet itself:

- §8.1: *"tolerant decode stays exactly as dumb as today: missing, empty or unrecognised falls to the shipped default per field."*
- §8.9's own abort bullet: *"Prefs is hand-editable and its decode is tolerant, so a stray comma degrades to a zero-value struct rather than erroring."*

A tolerant decode never produces "a decode failure", so the abort branch has no trigger. An implementer who reuses the existing prefs load for the RMW re-read — which §8.9 positively instructs (*"The RMW re-read uses the non-migrating read"*, and the non-migrating read is a variant of the ordinary load) — gets a zero-value struct, no error, and merges into it. That is exactly the outcome the bullet exists to forbid: `session_list_mode`, `theme_migrated`, every untouched theme key and the retained raw `appearance` erased in one `s` keypress or one theme commit.

What is missing is the discrimination the rule depends on: **the load path must stay tolerant (§8.1 requires it, and the existing `prefs` contract is tolerant), while the write-path re-read must use a decode that errors on malformed JSON.** Nothing in the spec says the two reads differ, or which of them the field-specific save methods (`SaveTheme`, `SaveThemeSlot`, `SaveMigrationMarker`, `SaveTranslation`) use internally. Two questions follow that also have no answer: whether a *syntactically valid* file carrying unrecognised values (which tolerant decode is specifically required to absorb) counts as usable for merge purposes — it must, or hand-editing prefs becomes fatal — and whether the load path gains any new behaviour at all, or stays exactly as today.

This is not a hypothetical branch. §13.6 names this path as *"the one part of the feature whose failure mode is silent, permanent destruction of a user's config"*, and its prefs-test row lists §10.2's mapping, §10.3's separation, §8.1's marker rules, §8.8's round-trip and §8.9's merge — but **not** the abort case, so no specified test would catch a tolerant-decode RMW either.

**Current**:
> - **`prefs.json` present but unusable** — a decode failure or an I/O failure — **aborts the write; it never becomes an overwrite.** Prefs is hand-editable and its decode is tolerant, so a stray comma degrades to a zero-value struct rather than erroring. Merging into that and committing it would erase `session_list_mode`, `theme_migrated`, every untouched theme key and the retained raw `appearance` in a single `s` keypress — the exact loss §8.8 calls out, on the path §13.6 names as the one whose failure is silent and permanent. Nothing is written, `theme: commit failed` is emitted, and the panel reports it (§9.13).

(and, in §8.1)

> Three flat keys match what `prefs.json` already is — a flat map of scalars — so **tolerant decode stays exactly as dumb as today**: missing, empty or unrecognised falls to the shipped default *per field*, with no type probing.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Real defect, latent since cycle 8. §8.9 now requires two decodes: the load path stays tolerant per §8.1, the write-path re-read is strict and errors on malformed JSON — without the split the abort branch had no trigger and the writer merged into a zero-value struct. Also pinned: unrecognised *values* in syntactically valid JSON are not "unusable" (tolerant decode absorbs them; treating them as fatal would make hand-editing prefs a lockout), and the save methods use the strict read internally.

---

### 2. Nothing owns assembly of the §9.4 row union, and the three statements that touch it cannot all hold

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.4 (The list), §12.3 (`theme: enumerated`, emission control), §8.9 (which packages emit the `theme` component), §13.3 (`ThemeEnumerator` seam and fixture inputs)

**Details**:
The panel's row set is defined as a union of three inputs — every `.theme` file, every built-in, and every slug named in `prefs.json` that resolves to neither (§9.4). The spec never says **which component computes that union**, and three pinned statements pull in incompatible directions:

1. **§12.3** defines `theme: enumerated`'s `count` as *"rows produced — the full §9.4 union, built-ins included"* and `rejected` as *"unselectable rows"* — a subset that includes the persisted-slug-only rows (`not found`, charset-rejected `bad name`). Both quantities are knowable only where the persisted keys and the directory walk meet.
2. **§12.3** also fixes emission: the loader takes a logger seam and `cmd` passes a real logger on the panel path, so `theme: enumerated` is emitted **by the loader**. **§8.9** closes the set of emitting packages explicitly — *"the loader (`internal/theme`), the translation (`cmd/config.go`), and this persister"*. `internal/tui` is not among them.
3. **§13.3** describes the seam as *"the real directory walk"*, and lists a panel fixture's four inputs as the `--theme` palette, **the raw persisted theme keys**, *"the faked `ThemeEnumerator`'s row set"*, and the cursor — i.e. the keys and the enumerator rows arrive at the panel **separately**, which reads as the panel doing the merge.

Under reading (3) the union exists only inside `internal/tui`, so either the loader emits a count it cannot compute, or `internal/tui` becomes a fourth emitting package and §8.9's list is wrong. Under the alternative — the seam takes the persisted keys and returns the finished union — the seam's shape, its production implementation's inputs, and what a fixture's separately-declared raw keys are still *for* (badges need them too, per §8.4) all need stating, and §13.3's description of the seam as a directory walk is wrong.

This decides concrete build outputs an implementer cannot infer: the `ThemeEnumerator` interface signature, which package owns `not found` / charset-rejected row minting and the one-slug-one-row dedupe against built-ins (§9.4), where `theme: enumerated` is emitted, and what §13.6's panel-behaviour test drives through the seam. §9.2's post-commit *re-derivation* of the union (which must happen without re-enumerating the directory) inherits the same ambiguity — it needs whatever the union-builder is, callable with prefs state that changed but a directory read that did not.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.3 now states the seam returns the finished §9.4 union rather than a directory listing, with `internal/theme` owning assembly — which keeps `theme: enumerated` computable where it is emitted, keeps §8.9's emitter set closed at three, and gives §9.2's post-commit re-derivation one entry point. Fixture raw keys retained separately because badges read them directly (§8.4).

---

### 3. §14A says five of the six pinned flashes are reachable on Projects but gives no basis for identifying the sixth

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §14A (Projects gains a transient-flash slot), §13.3 (Projects-with-panel fixture)

**Details**:
§14A pins six flashes (`NO_COLOR` block, width floor at entry, height floor at entry, resize below width, resize below height, failed commit outstanding) and then states that *"five of these six flashes are reachable there [Projects]"*. Every one of the six appears reachable from Projects: §9.6 binds `t` there, so both entry-floor refusals and the `NO_COLOR` block fire there; the panel opens over Projects, so both resize-forced closes and a failed commit's close-flash fire there too. The spec offers no rule that excludes any one of them.

The count is not inert, because the sentence is the justification for the scope of the new Projects contender: an implementer reading "five of six" has to pick one to leave unwired, and the two candidates a reader would most plausibly exclude — the failed-commit flash, or the `NO_COLOR` block — are precisely the two the same section then argues must *not* be lost on Projects (*"makes §9.10's proactive block a silent no-op … and makes §9.13's report vanish outright"*). Either the count is six, or the excluded flash and its reason need naming.

**Current**:
> **Projects gains a transient-flash slot.** The existing arbiter is Sessions-only — every one of its six contenders is a Sessions element — yet §9.6 binds `t` on Projects and §14.2 puts `t theme` in its footer, so five of these six flashes are reachable there. **Projects gets the flash contender alone**, not the full arbiter: no other contender has a Projects analogue, and inventing them would be scope for nothing.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §14A's count corrected to six — all six pinned flashes are reachable from Projects, and the two a reader would most plausibly exclude are the two the same section argues must not be lost there.

---

### 4. §11.1's list of the restyle path's production callers omits the only path that actually reverts the canvas

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Minor
**Affects**: §11.1 (Speed is a non-issue), §9.2 (`Esc`), §9.8 (forced close)

**Details**:
§11.1 re-points `applyCanvasMode`'s production callers: *"from here its callers are the panel's arrow-preview and commit paths."* That enumeration is the spec's only statement of where the restyle is invoked, and it is wrong in both directions against the panel's own decided behaviour:

- **A commit never changes the rendered theme.** §9.2 is explicit — the panel keeps previewing whatever the cursor is on, and *"committing to a non-active slot changes nothing on screen"*; under `Enter` the previewed theme is already the one painting. A commit recomputes rows and badges (§9.2), not the canvas.
- **`Esc` does change it, and is absent from the list.** §9.2 makes `Esc` discard the preview and render *the resolved persisted state*, which is a live theme swap in the general case (and, per §9.2's edited-and-now-invalid case, may resolve to a fallback). §9.8's forced close *"takes the `Esc` path exactly"* and inherits the same swap. §9.2's open-time cases (an edited-but-valid active theme, or an active theme that has gone invalid) are a third swap site, on open rather than on arrow.

The consequence is not theoretical: the omitted path is the one where a missed style re-point leaves a *discarded preview* — a theme the user explicitly declined — rendered on the main screen after the panel closes, and §11.2 states the panel's `bubbles/list` instance and the library-owned styles are the class where exactly that goes unnoticed. §13.4's guard drives the arrow-preview entry point only, so it would not catch it.

**Current**:
> - **Restyle** — `applyCanvasMode` swaps the delegate and re-points the cached style structs `bubbles/list` holds. O(1), no I/O, no list content touched. It performs exactly the mid-session restyle a theme swap needs. **Its production caller changes with this feature**: today it runs when a late OSC 11 reply lands after first paint, which §8.8 retires (the gate resolves once); from here its callers are the panel's arrow-preview and commit paths. The mechanism is proven, not the caller — so §13.4's guard is driving an existing entry point with a new set of callers, not building one.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §11.1's caller list corrected: arrow-preview, open (mid-session edit changing or invalidating the active theme) and close (`Esc` plus the forced close). A commit is not a caller — it recomputes rows and badges, not the rendered theme. Flagged that close is the path where a missed re-point leaves a discarded preview painting the main screen, and §13.4's guard drives arrow-preview only.
