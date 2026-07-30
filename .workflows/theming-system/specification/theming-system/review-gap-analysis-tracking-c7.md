# Review Tracking: Theming System - Gap Analysis

Cycle 7. Full fresh pass over the whole specification as a standalone document. Prior cycles' findings (gap-analysis c1–c6, input review c1–c7) were read first; nothing already resolved is re-raised.

## Findings

### 1. Badge derivation for a slot holding a shipped default — §8.4 and §9.5 state opposite rules, and they disagree about the most common install of all

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.5 (row rendering and markers), §8.4 (construction timing — the raw persisted keys), §9.4 (the list — files ∪ whatever prefs names), §8.1 (`omitempty`), §8.3 (the shipped default), §9.2 (commit recomputation), §9.9

**Details**:

Two sections give the `●` badge two different sources, and they produce different screens on a virgin install.

- **§8.4** is explicit that the badge reads the raw persisted key: *"A badge needs the **persisted** slug, not the nomination's. Under a fallback these differ by design — 'the `●` still marks the persisted slug' (§9.2) while the nomination holds the fallback's palette."* The constructor is specified to take `theme` / `theme_light` / `theme_dark` "exactly as read" precisely so the panel can do this.
- **§9.5** closes with the opposite: *"**Because the panel shows both slots' badges at all times**, a user can see what light is set to without having to remember whether they set it — **including slots never touched, which hold shipped defaults (§8.3)**."* That reads the badge off the *effective* slot value.

The two are not reconcilable at the case that matters most. §8.1 omits empty values on write and refuses to create `prefs.json` at all on a fresh install, and §8.3's shipped default is "unset slot holds the shipped default" — so a brand-new user's prefs contain **no theme key whatsoever**:

- Under §8.4's rule there is no persisted slug, therefore **no `●` anywhere in the panel** on the default install.
- Under §9.5's rule `tokyo-night-day` carries `● light` and `tokyo-night` carries `● dark`.

Three further statements depend on which is right:

- **§9.4's justification for the whole union** — *"This is what makes the `●` marker always have something to sit on"* — is true only under the effective reading. Under the persisted reading the marker has nothing to sit on in the default case, which is the one case the union cannot help with.
- **§9.2's commit recomputation.** `Enter` on a virgin install "clears both slots" that were never set: under the effective reading two badges visibly collapse to one bare `●`; under the persisted reading nothing visibly changes except a badge appearing. The spec pins that a commit "recomputes the panel's full row set" without pinning what the badges were showing beforehand.
- **§9.9's accepted "no unset"** turns on the difference between an inherited default and a pin. A badge on an untouched slot is the one place that difference could be visible to a user, so whether it is shown is a decision, not a detail.

The badge rule also has a third case neither section names: a slot that is **set but unloadable** (§8.5 fallback). §8.4 covers it — the badge stays on the persisted-but-broken slug — but only under the persisted reading; the effective reading would have to say explicitly that a fallback does not move the badge.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved with one rule covering all three cases: the badge marks the slug a slot resolves to *before* fallback — persisted slug when set (loadable or not, so a fallback never moves the badge), shipped default's slug when unset. That makes §9.4's "the marker always has something to sit on" true on a virgin install (where §8.1 leaves prefs.json absent entirely), and makes §9.9's inherited-default-vs-pin distinction visible. §8.4's bullet cross-referenced to the full rule; the virgin-install commit's visible badge collapse recorded as correct.

---

### 2. `portal doctor`'s closing summary is pinned only for the all-checks-pass case — the case it exists for is unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §12.2 (`portal doctor` — a read-only theme health line), §14A (doctor copy), §15.1 (named amendment 2)

**Details**:

§12.2 amends doctor's contract with two classes of line and states: *"**Doctor's closing summary distinguishes the two counts** — e.g. *'N checks passed · 2 advisories'* — so the exit code's meaning is legible without reading the contract."* §15.1 carries that forward as one of the three named spec amendments. §14A then pins exactly two forms:

| Case | Pinned |
|---|---|
| M ≥ 1 | `<N> checks passed · <M> advisory` / `· <M> advisories` |
| M = 0 | `<N> checks passed` |

Both forms describe a run in which **every health check passed**. That is the one run whose exit code needs no explanation. The summary's stated purpose is to make a *non-zero* exit legible — and there is no pinned form for a run with one or more failed checks, which is precisely when a user reads the summary. An implementer must invent that copy, in the section that opens by stating every new user-facing string is pinned here.

Two further things are left undetermined by the same table:

- **What `<N>` counts.** The report is not a two-state pass/fail list — it also carries lines that are informational and lines that are not evaluable, neither of which is a passed check nor a failed one. `<N> checks passed` needs a stated rule for whether those are counted, excluded, or reported separately, and whether `<N>` is a numerator against a stated total.
- **The M = 0 claim.** §14A justifies suppressing the advisory clause on the grounds that *"an unchanged doctor run reads exactly as it does today"*. But the summary line is itself new — today's report is a header plus one line per check, with no trailing summary — so at M = 0 the run does not read as it does today; it gains a line. Either the claim needs correcting or the M = 0 form needs to be "no summary line at all", which would then leave §12.2's "distinguishes the two counts" satisfied only when advisories exist.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §14A now pins three summary forms including the failing case the summary exists for (`<N> of <T> checks passed`), defines N and T as Portal-health checks only with advisories counted separately by M, and corrects the M=0 justification — the summary line is new on every run, which is the §15.1 amendment rather than a regression.

---

### 3. Two new panel states with pinned copy and a stated wrap risk have no fixture, so neither can be seen before release

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.3 (harness changes — new fixtures), §9.1 (message slot), §9.2 (slot-from-constant confirm), §9.13 (failed commit write), §9.5 / §5.5 (the pinned directory row), §9.8 (minimum height), §14A

**Details**:

§13.1 is unambiguous that the capture harness is *"the **only viable route** to seeing a visual change before release"*, and §13.3 accordingly enumerates the new panel fixtures: **the adaptive-pair state, the constant-while-previewing state, an invalid-theme row, and the narrow degraded panel**. §11.2 adds a fifth requirement on top (one panel fixture must carry enough rows to paginate, so the pagination dots are covered).

Two specified panel surfaces are outside that set:

1. **The message slot, in either of its two states.** §9.1 introduces it as a single-row region with two contenders, and warns in the same paragraph that *"At the minimum panel width the slot may wrap to two rows."* §14A pins both strings (`clear constant <slug>?  y / n`, `⚠ couldn't save theme`) and states that in the panel *"the wording is a **layout constraint** as much as a copy choice — it has to fit 24–30 columns"*. The single state whose copy the spec says may not fit is the one state with no capture to check it against.
2. **The pinned `⚠ themes dir unreadable` row.** It has its own placement rule (§9.5: outside the ordering, always first), its own token assignment (§9.1), its own pinned copy (§14A), and a stated reason for existing (§9.5's "completely in the dark" argument) — and no fixture.

The consequence is not only cosmetic. §9.8's minimum-height floor is defined as **header + footer + one row + one message row**, and its whole justification is that a floor computed without the message row would put the panel one row short *"at exactly the moment a message appears, asking 'clear constant `<slug>`?' about a row no longer on screen"*. That failure is observable only on a frame that renders the message at the floor — which no fixture produces. The swap-and-diff guard does not close the gap either: §13.4 has it enumerate whatever fixtures exist, so a missing fixture is a blind spot the guard cannot report.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.3's fixture list expanded from four to seven, adding the pinned directory row, both message-slot states (the one copy the spec says may not fit), and the minimum-height-with-message frame (§9.8's floor arithmetic is only observable there), plus §11.2's paginating panel. Recorded that a missing fixture is a blind spot §13.4 structurally cannot report, since absence reads as coverage.

---

### 4. A failed *migration* write has no defined event, emission site, or user-facing outcome

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §8.9 (concurrent instances and prefs writes), §10.5 (ownership and write-path robustness), §12.3 (`theme` event catalogue), §9.13

**Details**:

§8.9 closes its read-modify-write rule with: *"So a decode failure **or** an I/O failure on the re-read is treated as a failed write: nothing is written, `theme: commit failed` is emitted, and the panel reports it (§9.13). **The same applies to the migration write**, which this rule already covers."*

The migration write has no panel. §10.5 puts it in `loadPrefsStore` at prefs load, and §10.5 states the migration *"runs only where a TUI is constructed"* — i.e. at or before construction, ahead of any panel existing. So "the same applies" cannot mean the whole sentence, and which part it does mean is unstated. Three specific unknowns follow, and the first is not resolvable at a call site because §12.3's catalogue is closed and spec-governed:

- **Which event fires.** §12.3 describes `theme: commit failed` as WARN, cadence *"per failed write"*, carrying `slug` / `slot` / `reason` — which would admit the migration write. But §10.5 designs the migration's failure signal to be the **absence** of `theme: appearance migrated` (*"its absence after a translation is itself the signal that the write failed"*), which reads as no event at all. Both cannot be the specification.
- **From where.** §8.9 names the injected theme persister as *"**the emission site for `theme: commit failed`**"*. The migration does not go through that persister — it is owned by `loadPrefsStore` — so if the event fires for it, it fires from a second site the catalogue does not account for.
- **Whether anything is user-facing.** §10.5's write is best-effort and non-blocking with a retry next launch, which implies silence; §8.9's sentence implies a report. Nothing else in the spec gives the migration a reporting surface.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §8.9: the migration write inherits only the abort-don't-overwrite half. It runs before any panel exists, is best-effort with a next-launch retry, and emits no `theme: commit failed` — its failure signal is the absence of `theme: appearance migrated` per §10.5, which keeps the commit-failed event single-sited on the theme persister.

---

### 5. Two branches of §4.2's lexical table are under-determined, and one row contradicts itself

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §4.2 (lexical rules and the branch table), §6.2 (the reason ladder), §13.6 (the new loader / parser test), §14A (`bad syntax` detail formats)

**Details**:

§13.6 makes the loader test *"table-driven over §4.2's branch table"*, so each row of that table is directly a test case. Two of them cannot be encoded as written, and a third case the table does not carry is reachable.

- **(a) "Quoted value" has no definition.** §4.2's rule table rejects a quoted value and §14A pins the doctor detail `line 4: quoted value`, but nothing says what counts as quoted. Single quotes as well as double? A matched outer pair only, or any leading quote? And how does `#FFFFFF"` — one trailing quote, unmatched — classify? §6.2 puts the two candidate outcomes on **different rungs of the short-circuit ladder** (`bad syntax` at 4, `bad colour` at 5), so the answer is user-visible in both the panel row and doctor's detail, and the ladder test §13.6 calls out as "critically" needing pinning is exactly the test that would encode it. (Portal already carries a matched-outer-quotes helper in `internal/tmuxout`, so a near option exists — the spec simply does not pick one.)

- **(b) The whitespace row contradicts itself.** The row reads:

  > | Trailing or internal whitespace in a value | `bad colour` | Trimming is defined around `=` only; a value with interior whitespace is not a valid hex. Trailing whitespace after the value **is** trimmed, since it is whitespace around the pair rather than inside the value. |

  The Input column names trailing whitespace as a `bad colour` case; the Why column says trailing whitespace is trimmed, i.e. is *not* a `bad colour` case. Only interior whitespace is. As a test row it is unencodable, and as a rule it says both things.

- **(c) What the parser accepts as a *key* is not stated.** §4.2 says keys are *"Lowercase by definition (per the vocabulary charset), matched **case-sensitively**"* — which describes the 19 known keys, not the parser's admissible key syntax. So `text primary = #FFFFFF` (interior whitespace in the key) has no determined outcome: a well-formed pair with an unknown key (→ ignored → file fails `missing tokens`) or a malformed line (→ `bad syntax`). §4.2's malformed-line rule turns entirely on what "a well-formed `key = value` pair" means, and that is the one term left undefined.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: All three settled in §4.2's branch table: the contradictory whitespace row split into interior-whitespace (`bad colour`) and trailing-whitespace (trimmed, not an error); "quoted" defined as any leading quote matched or not, with the reasoning that a matched-pair definition would send the unmatched case to `bad colour` and blame the wrong thing; and a well-formed key defined as non-empty with no whitespace and no `=`, so a key typo reports `bad syntax` rather than being ignored into a misleading `missing tokens`.

---

### 6. The "failed commit outstanding" state has no defined discharge, so it can outlive the flash designed to discharge it

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.13 (a failed commit write), §9.8 (forced close)

**Details**:

§9.13 makes outstanding-ness a state rather than a message, and defines its lifetime one-sidedly: *"A commit failure is outstanding from the moment a write fails until a **subsequent commit succeeds** — nothing else clears it."* It then specifies the discharge event: *"closing the panel with a failed commit outstanding raises a main-screen flash: `⚠ theme not saved — see portal.log`."*

Nothing says raising that flash clears the state. Read literally — and §9.13's own "nothing else clears it" invites a literal reading, since it was written to stop arrowing from clearing it — the state survives the flash, with these consequences:

- Reopen the panel, press `Esc` without attempting any commit, and the same flash fires again about a failure the user has already been told about. It repeats on every subsequent close for the life of the process unless a commit eventually succeeds.
- §9.8's forced close *"takes the `Esc` path exactly"*, so a narrow-terminal resize while the state is outstanding also raises it — potentially alongside `terminal too narrow — theme picker closed`, which the notice band's single slot then has to arbitrate between.

The opposite reading (the flash discharges the state) is equally consistent with the text and needs stating just as much, because §9.13 goes out of its way to enumerate what does *not* clear the state without saying anything besides a successful commit that does.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §9.13: raising the flash discharges the state — it is the report the state exists to produce. Without it, reopening and pressing Esc would re-fire the flash on every close for the life of the process, and §9.8's forced close would stack it against the too-narrow flash in a single-slot notice band.

---

### 7. The adaptive nomination's "active member" at construction is specified two ways

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §13.3 (`tui.Build` takes the loaded nomination), §8.4 (construction timing), §8.8 (the gate resolves exactly once), §11.4 (the retained startup canvas hex)

**Details**:

§8.4 defines the value the constructor takes, and under adaptive it explicitly does **not** include an active member:

> **Adaptive** — both loaded `Theme`s, light and dark; the gate selects between them when the OSC 11 reply or the timeout lands.

§13.3 describes the same value as including one:

> `tui.Build` takes the loaded *nomination* … (one theme under a constant, both under an adaptive pair, **plus which member is active**).

Under adaptive the caller cannot supply it — the gate has not run at construction. So the implementer has to decide whether the value carries a *provisional* active member (dark, later confirmed or replaced by the gate) or no active member until the gate resolves, and the decision reaches two other statements:

- **§8.8's** *"The gate resolves exactly once. A reply that arrives after the timeout has resolved it does not re-resolve it"* and its never-paint-then-flip guarantee. A provisional active member that the gate later overrides is a second resolution unless nothing painted in between — which is true today but is a property of the first-paint gate, not of the nomination, and is not stated as the reason.
- **§11.4's** retained startup canvas hex, *"captured from the theme the gate **selected**"*. If no member is active before the gate resolves, the retained hex has no value at all for the window between construction and resolution, and `RestoreTerminalBackground`'s anchor is undefined if Portal dies in it.

It also fixes the shape of a type three surfaces consume — `cmd/open.go`, `capturetool` (which §13.3 says always passes the constant shape), and the model — so it is worth pinning rather than leaving to whichever site is written first.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §8.4: the nomination carries no provisional active member under adaptive. Nothing needs one — the gate resolves before first paint, so there is no frame in the interval and no second resolution to reconcile with §8.8's resolve-once rule. §11.4's retained hex is captured when the gate resolves, defined for every frame that exists. §13.3's wording corrected to match.

---

### 8. An unreadable themes directory has no defined reject reason for the drop-in slugs it makes unreachable

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §5.5 (directory states), §6.2 (the reason vocabulary), §8.4 (the by-name construction path), §8.5 (fallback), §9.5 (rows beneath the pinned directory row), §12.3 (`theme: fallback applied`, `theme: directory unusable`), §14A (doctor copy)

**Details**:

§5.5 gives an unreadable themes directory two surfaces — a doctor advisory and `theme: directory unusable` *"from the TUI path when the panel's enumeration hits it"* — and §9.5 gives it a pinned row. What none of them gives is the reason carried by the **themes** the condition makes unreachable, and two places require one:

- **Construction, on §8.4's by-name path.** A nominated non-built-in slug cannot be read because the directory itself is unreadable, so §8.5's fallback applies and §12.3's `theme: fallback applied` requires a `reason` attr. §6.2 offers `unreadable` (*"the file could not be read"*) and `not found` (*"a slug named by `prefs.json` with no corresponding file"*); a directory-level failure fits neither definition cleanly, and the two send a user to different places — one to check permissions, one to check the filename. §5.6's *"`unreadable` covers every read failure"* does not settle it: that rule is scoped to entries *inside* the directory.
- **The panel row.** §9.5 requires persisted-slug rows to render beneath the pinned directory row — *"the persisted rows especially, or a user with an unreadable directory loses the `●`"* — but enumeration produced no entry for those slugs, so the terse reason on those rows is undetermined by the same question, and doctor's line inherits it through §14A's `⚠ theme <slug> (<slot>) does not resolve: <reason>`.

Lower-stakes, from the same condition: §5.5 scopes `theme: directory unusable` to *"when the panel's enumeration hits it"*, which leaves open whether the construction-time by-name read emitting the same condition emits the event too, or stays silent and lets `theme: fallback applied` carry it alone. §12.3 dedups the event per process on `path`+`reason`, so either answer is workable — but it decides whether a user who never opens the panel gets any log record of a broken directory.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §5.5: a theme made unreachable by an unusable directory carries `unreadable`, not `not found` — the same discrimination §9.4 draws, and permissions is the actual problem. Applied uniformly to the fallback event's reason attr, the persisted-slug rows beneath the pinned directory row, and doctor's line. Also settled that the construction-time by-name read emits `theme: directory unusable` too (dedup makes it safe), so a user who never opens the panel still gets a record.
