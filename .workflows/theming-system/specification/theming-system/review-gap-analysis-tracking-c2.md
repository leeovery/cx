# Review Tracking: Theming System - Gap Analysis

## Findings

### 1. Retaining `appearance` is incompatible with the mandated read-modify-write path

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Critical
**Affects**: §8.8 (what dies), §8.9 (RMW), §10.4 (`appearance` is retained), §12.6 (CLAUDE.md prefs row)

**Details**:
§10.4 makes retention of the legacy `appearance` key a load-bearing guarantee: "The translation adds the theme keys and leaves `appearance` in place", justified by downgrade protection ("an older binary reads nothing, falls to `auto`, and resumes detecting — precisely what the translation prevented").

Three other decisions make that guarantee unimplementable as written:

- §8.8 lists `prefs.Appearance` — "the `auto|light|dark` enum, its tolerant decode, `LoadAppearance`/`SaveAppearance`" — under **Dies**, and §12.6 says the prefs row is "Replaced by `theme` / `theme_light` / `theme_dark` / `theme_migrated`". Both read as removing the field from the prefs struct.
- §8.9 mandates that **every** writer read-modify-write `prefs.json` and "write the merged result".
- `prefs.json` decodes into a plain Go struct (`prefsFile`), so any key not declared as a field is **dropped on re-encode**. This is not hypothetical — it is the current shape of the store, and it is exactly why `Save` already RMWs to preserve `appearance` today.

Net effect if an implementer follows §8.8/§12.6 literally: the first `s`-keypress or theme commit after upgrade silently deletes `appearance` from the file, defeating §10.4 entirely and doing so at the moment the user is least likely to notice.

The spec also requires *reading* the legacy value (§10.5: "At prefs load, read `appearance`") after its decoder is deleted, without saying what reads it.

The missing statement is the retention mechanism: whether `prefsFile` keeps a raw `Appearance string` field (read-and-preserve, no enum, no parse), whether the store preserves unknown keys generically, or something else — and what §8.8's "Dies" row therefore means precisely (the enum + accessors + wiring, not the on-disk field).

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Confirmed against internal/prefs/store.go:125 — prefsFile is a plain struct so undeclared keys drop on re-encode. §8.8 now states prefsFile keeps a raw `appearance string` field, read-and-preserved and never parsed, and clarifies that the "Dies" row means the enum/API not the on-disk slot. §12.6's prefs row updated to match.

---

### 2. `tui.Build` taking a single theme contradicts §8.4's "load every nominated theme"

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §3.4 (model holds the active theme), §8.4 (construction timing), §13.3 (harness changes)

**Details**:
§8.4 is explicit that under the adaptive form — which is the **shipped default**, so the common path — construction happens *before* light/dark is known, so Portal "loads every *nominated* theme — at most two" and the gate "only **selects** between values already in hand."

§13.3 then specifies the constructor as "**`tui.Build` takes a *theme*** where it takes a `prefs.Appearance` today" — singular. Under adaptive, a single theme cannot be resolved at that call: the OSC 11 reply or the ~50ms timeout has not landed.

So the model must hold, at minimum: the light candidate, the dark candidate, which one is active, and (per §11.4) the retained startup canvas hex — while §3.4 describes it as holding "the active `Theme`". The spec never reconciles these, and the difference is a real design decision an implementer would have to invent:

- Does `Build` take a pair (or a small `themeSelection` value) and select on gate resolution?
- Does it take one theme plus a deferred alternate?
- What does `capturetool --theme <slug|path>` (§13.3) pass, given it pins a single theme and needs no gate?
- What does the constant case pass — the same shape with both slots equal, or a distinct shape?

This also affects §11.4's requirement to capture the startup canvas hex, since under adaptive the startup canvas is whichever theme the gate selected, not what `Build` was handed.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §8.4: the constructor takes the loaded *nomination* (one Theme under a constant, both under adaptive) plus which member is active; §3.4's "model holds the active Theme" describes what is threaded to renderers. capturetool passes the constant shape. §11.4's retained startup canvas is captured from the theme the gate selected.

---

### 3. Panel open is undefined when the fresh enumeration disagrees with the in-memory active theme

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §5.8 (enumeration supersedes), §9.2 (opening state), §9.5 (invalid rows unselectable)

**Details**:
Three rules collide when the active theme's file has been edited since construction — which is the exact loop §5.8 exists to serve ("copy a built-in, edit it, see it, without relaunching"):

1. §5.8: "The panel's parse supersedes the construction-time parse for the same slug. After a mid-session edit the panel holds the fresher truth."
2. §9.2: "the cursor lands on the theme that is actually rendering, and opening previews nothing… **opening the panel never changes the screen**."
3. §9.2: "When the resolved theme is a **fallback**, the cursor lands on the fallback's row" — because the persisted row is unselectable and "parking the cursor there would put it somewhere navigation cannot return to."

Two concrete states are unresolved:

- **Active theme's file edited and still valid.** The panel holds different values for the slug that is currently painted. Does opening re-render with the fresher values (violating "opening never changes the screen"), or keep the stale ones until the user arrows away and back (at which point the "preview" of the row already under the cursor changes the screen anyway)? Either answer is defensible; the spec asserts both.
- **Active theme's file edited and now invalid.** The screen is rendering a theme whose row is now unselectable. §9.2 says the cursor lands on "the theme that is actually rendering" (an unselectable row — forbidden by its own reasoning) and §5.8 says `Esc` "lands on the §8.5 fallback". Does the panel re-theme to the fallback on **open** and park the cursor there, or defer the flip to `Esc`? Where does `●` sit meanwhile?

An implementer must pick, and the two choices produce visibly different products.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §9.2: opening never changes *which* theme is shown, but does apply the fresher parse of that theme's values. Edited-and-valid re-renders on open; edited-and-now-invalid resolves the fallback on open (not deferred to Esc) with the cursor on the fallback's row and the ● still on the persisted slug. Invariant stated: the cursor is always on a selectable row and that row is always what is painted.

---

### 4. The parser's reject branches have no test home

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §4.2 (lexical rules branch table), §7.6 (embedded-set validity test), §13.6 (guard-test reshape)

**Details**:
§4.2 introduces its branch-by-branch table by asserting each row "is a user-visible reason label **and a test case in the embedded-set validity test (§7.6)**". That pointer cannot be right: §7.6's test parses and validates the **embedded built-ins**, all of which are valid by construction — it has no malformed input to feed. A duplicate key, a quoted value, a `#FFF`, a mid-file BOM and a keyless line can never appear in that test.

§13.6's reshape table is the spec's enumeration of new and changed tests, and it lists no loader/parser unit test at all. So the single most branch-heavy new component in the feature — the hand-rolled parser, the §4.3 hex validator, the §5.2 slug charset check, the §5.4 reserved-name check and especially §6.2's **fixed-order short-circuit ladder** (six reasons, first failure wins, "a file always has exactly one reason") — is specified in detail but assigned to no test.

The ladder in particular is only meaningful if tested: a file that is simultaneously duplicate-keyed and missing tokens must report `bad syntax`, and nothing pins that ordering.

**Current**:
> **Branch-by-branch, because each one is a user-visible reason label and a test case in the embedded-set validity test (§7.6):**

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §4.2's mis-pointer corrected (§7.6 → §13.6), and a new Loader/parser test row added to §13.6 — table-driven over the branch table, hex domain, slug charset, reserved-name check and especially §6.2's short-circuit ladder, which nothing else pins.

---

### 5. The panel's own token assignments are unspecified beyond body, border and header

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §9.1 (shape), §9.5 (row rendering and markers), §9.13 (failed commit)

**Details**:
§9.1 is deliberately precise where it settles tokens — body is `canvas`, left border is `border`, header label is `accent.mode`, header rule is `border`, cursor row is "the shipped selection treatment". §9.11 then makes it a hard rule that every panel surface re-themes with no exceptions, and §9.1 argues that "every panel surface resolving to an existing token" is what keeps the colour-literal guard and the swap-and-diff guard satisfied "without a carve-out".

But several panel surfaces have no token named anywhere:

- The **`●` / `● dark` / `● light` badge** — `accent.primary`? `state.positive`? `text.muted`?
- The **`⚠` glyph on an invalid row** — the row is `text.faint`, but §2.5 assigns the warning `⚠` to `accent.attention`; §9.5 does not say which wins.
- The **terse reason text** on an invalid row.
- The **vertical keymap footer** — key glyphs vs labels (the horizontal footer splits `accent.key` / `text.muted`; the panel footer is a different renderer).
- The **message slot** — the confirm and the §9.13 failure line, including whether the failure line uses `bg.attention` / `text.on-attention` like the warning band or plain text.
- The **pinned `⚠ themes dir unreadable` row**.

§9.14 explicitly forbids reading these off the Paper frames ("per-frame literal hexes… reference, never truth"), so the implementer has no fallback source and must make design decisions. This also feeds §13.4 assertion 3 (every token exercised by at least one fixture) — the panel fixtures are candidates for covering the at-risk transient tokens, which only works if the assignments are known.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Every panel surface's token tabled in §9.1 — body, borders, header, cursor row, valid/invalid row labels, the ● badge (accent.primary, not state.positive, which would imply liveness), the ⚠ and reason (accent.attention, keeping its own token so it stays legible on a text.faint row), the vertical footer split, and both message-slot states (no bg.attention band inside a narrow panel).

---

### 6. "Direct PNG output from `capturetool`" has no named mechanism, and the tapes that produced PNGs are deleted

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.1 (why capturetool is load-bearing), §13.2 (retention rule), §13.3 (harness changes)

**Details**:
§13.1 makes a producible PNG per fixture a hard requirement for the agentic loop ("**Without a producible PNG the agent cannot see what it built**"). §13.2 deletes "the committed reference PNGs **and the VHS tapes that produce them**". §13.3 then states: "**Direct PNG output from `capturetool` is required, not an optimisation.** The retention decision deletes the tapes that made PNG production work."

The requirement is stated; the mechanism is not. Rendering styled ANSI output to a PNG needs a terminal-cell rasteriser (font, cell metrics, palette handling, truecolor SGR parsing) — which almost certainly means a new dependency or an invoked external tool. That collides with a constraint the spec itself leans on hard in §4.1 ("**zero new dependencies** — every config today is stdlib") and with `cmd/capturetool` being an offline program.

Unresolved as a result: what produces the PNG, whether a new module dependency is acceptable for a test-only/offline tool, what the flag/output-path surface is, whether the harness still has any VHS dependency ("VHS is retained only if a gif is ever wanted for motion" — retained how, if the tapes are deleted?), and what the deterministic font/size baseline is so two agents' captures are comparable.

Without this, the harness leg the whole implementation loop rests on is not plannable.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: User decision (asked, not baked): keep VHS, drop the direct-writer requirement. §13.3 rewritten — the hard requirement is a producible PNG per fixture, which VHS already satisfies; a rasteriser would mean a module dependency plus an embedded font to replace something that works. New tapes are written per fixture and cleared after sign-off under §13.2's retention rule.

---

### 7. The swap-and-diff guard's swap must be a live in-model swap through the production path — unstated

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §13.4 (swap-and-diff completeness guard), §11.2 (the real risk is completeness)

**Details**:
§13.4 describes the guard as "render a screen under theme A, **switch to theme B**, render again". The guard's entire purpose (§11.2) is catching **cached styles assigned once** — `bubbles/list` help styles, pagination dots, TitleBar, both filter inputs, and any init-time copy of a derived style.

That purpose is only served if the switch happens **on the same model instance, through the same code path the panel's arrow-preview uses** (`applyCanvasMode` + the style re-point). If the test instead builds two models — one per theme — every cached style is assigned correctly at each construction and the guard passes green while live swap is broken. That is the failure mode the guard exists to prevent, and the fixture harness today builds a fresh model per fixture, which is precisely the shape that would produce a vacuous pass.

Unstated and needed:

- The swap must be a live mutation of one already-rendered model, via the production swap entry point (not a test-only setter, not a rebuild).
- What seam `internal/capture` / `tui.Build` exposes to drive that from a test, given fixtures are currently one-shot renders.
- Whether the guard renders **before** the swap as well (assertion 1 needs an A-render to have happened for the caches to be populated; a fixture rendered only after the swap would trivially pass).
- That the render must be forced to a truecolor profile — under `go test` stdout is not a TTY, so lipgloss would otherwise strip colour. (Assertion 2 would catch this, but only if it is understood as a colour-profile problem rather than a missing-site one.)

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §13.4: the swap must be a live mutation of one already-rendered model through the production swap path, with the A-render as load-bearing cache population, a new capture/tui.Build seam to drive it, and a forced truecolor profile (stdout is not a TTY under go test). The two-model shape that would pass vacuously is named explicitly.

---

### 8. Unknown keys: "ignored" (§4.4) vs "per-value validation across the whole file" (§6.2)

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Important
**Affects**: §4.4 (what a theme file may contain), §4.6 (vocabulary evolution), §6.2 (reason ladder)

**Details**:
§4.4 and §4.6 state flatly that **unknown keys are ignored** — and §4.6 makes that a load-bearing forward-compatibility lever ("This makes *removing* a token survivable: old files keep working").

§6.2's ladder defines step 5 as "`bad colour` — **per-value validation across the whole file**". Read literally, that validates unknown keys' values too, so `some.legacy.key = maroon` rejects the whole file — directly defeating §4.6's lever and contradicting §4.4.

The two readings differ in observable behaviour and in test cases:

- Validate known keys only → a removed token's old line survives with any value.
- Validate every parsed line → the ignore rule only covers *well-formed-hex* unknowns, which is a much weaker guarantee and should be stated as such.

Related and equally unstated: §4.2 covers duplicate unknown keys explicitly (`bad syntax`), which shows the spec has considered unknown-key edge cases — making the silence on unknown-key **values** look like an omission rather than an intentional read.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §6.2 step 5: value validation covers *known* keys only. An unknown key is ignored entirely, key and value both — otherwise §4.6's forward-compatibility lever only holds for values that happen to still be well-formed hex.

---

### 9. Panel key-exclusivity swallows the global quit, and quit-while-previewing is untested

**Source**: Specification analysis
**Category**: Enhancement to existing topic
**Priority**: Important
**Affects**: §9.7 (entry conditions and input routing), §11.4 (exit-time canvas restore), §13.6

**Details**:
§9.7 states: "**The panel is key-exclusive.** It owns arrows, `Enter`, `d`, `l` and `Esc`; **everything else is swallowed.**" The same section shows the spec is aware of `Ctrl-C` as a survivor elsewhere — "The burst input-locks the model (only `Ctrl-C`/`Esc` live)".

Read literally, opening the theme panel takes away the user's quit key. The rationale given for exclusivity is about *destructive pass-through* (`k` killing a session, `x` swapping page, `m` starting a multi-select) — none of which applies to `Ctrl-C`. So the intent is almost certainly that `Ctrl-C` stays live, but the rule as written says otherwise and an implementer following it produces a trap.

This has a second-order consequence the spec half-covers: quitting **while a preview is uncommitted** is the one path where §11.4's retained startup canvas hex is doing its work — Portal exits with a canvas the user never persisted. §11.4's named test requirement covers "the case where a theme was committed mid-session" but not "quit with an uncommitted preview active", which is the more likely mistake (the model's active theme is the previewed one, so a naive implementation compares against it).

**Current**:
> **The panel is key-exclusive.** It owns arrows, `Enter`, `d`, `l` and `Esc`; everything else is swallowed. Pass-through is genuinely bad — `k` would kill the highlighted session while you pick a theme, `x` would swap to Projects with the panel open, `m` would start a multi-select behind it.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §9.7: `Ctrl-C` stays live, matching the burst input-lock. The exclusivity rationale is about destructive pass-through and does not reach the global quit. §11.4's named test extended to cover quit-with-uncommitted-preview, flagged as the likelier mistake than the committed-mid-session case.

---

### 10. Hand-edited `theme` + slots both present: only the resolution rule is defined, not the rendering or the next write

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §8.2 (two states, not three), §9.5 (markers)

**Details**:
§8.2 handles the hand-edited both-present file with a resolution rule: "**If a hand-edit leaves both present, `theme` wins** — a documented deterministic rule."

§9.5 then asserts a property that state breaks: "The two setting states never coexist on screen, so a row never carries both forms" — and §9.5's closing argument is that the panel "shows **both slots' badges at all times**". With `theme` **and** both slots non-empty on disk, an implementer must decide whether the panel renders a bare `●` on the constant only (consistent with "theme wins", but hides state that is in the file) or also the two `● light`/`● dark` badges (three markers, contradicting §9.5).

Also unstated: after `d`/`l` clears the constant on such a file, the *other* stale hand-edited slot silently becomes live. Whether the §9.2 confirm text should account for that (it "names the constant that will be cleared") is undefined.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §8.2: `theme` winning means the slots are not read at all, so the panel shows a single bare ● and no slot badges — §9.5's no-coexistence property holds via the resolution rule, not via the file. Stale slots are left on disk; the one visible consequence (a stale slot going live when d/l clears the constant) is named.

---

### 11. Enumeration edge cases: extension case, directory entries, broken symlinks

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §5.3 (extension), §5.6 (enumeration rules), §6.2 (reason vocabulary)

**Details**:
§5.6 defines enumeration as "files matching `.theme` in the directory itself". Three cases are undefined, and each has a user-visible outcome:

- **Extension case.** Is `Nord.THEME` matched? If matching is case-insensitive, its slug (`Nord`) is `bad name` and it gets a row; if case-sensitive, the file is invisible — which is the "completely in the dark" state §9.4 exists to prevent, on a case-insensitive macOS filesystem where the user may reasonably expect it to work. §5.2's "**Reject, never normalise**" reasoning suggests case-sensitive, but the extension is a separate decision from the slug charset and is not covered by it.
- **A directory (or symlinked directory) named `x.theme`.** §5.6 says symlinked directories are not followed, but not what a *real* subdirectory with a `.theme` name does: skipped silently, or opened and reported `unreadable` (EISDIR)?
- **A broken symlink.** §5.6 says symlinked files are followed. A dangling link presumably lands on `unreadable`, but `unreadable` is defined in §6.2 as "the file could not be read", which reads as a permissions case; nothing states the mapping.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §5.6: extension matched case-insensitively so `Nord.THEME` enumerates and gets a visible `bad name` row rather than vanishing (the slug is still matched exactly, so §5.2's reject-never-normalise is untouched); a real subdirectory named x.theme is skipped silently as a non-candidate; a dangling symlink is `unreadable`, and `unreadable` is widened to cover every read failure not just permissions.

---

### 12. Panel row ordering is under-determined for mixed slug/filename labels

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.5 (row rendering and markers)

**Details**:
§9.5 gives two ordering statements that need one more rule to be deterministic:

- "**ordering is alphabetical by slug, falling back to the filename for a row that has no slug**" — which sorts `bad name` rows by filename.
- A `reserved name` row is *labelled* by filename but has a valid slug, and the spec relies on it sorting "adjacent to the built-in it collides with" — i.e. sorted by **slug**, displayed by **filename**.

So sort key and display label diverge for one row class and coincide for another. Unstated: whether the comparison is byte-wise or case-insensitive (filenames can be mixed-case, slugs cannot, so `Zed.theme` sorts before every slug under byte order), and where the `not found` persisted-slug rows sit (presumably by slug, but they have no file).

The pinned `⚠ themes dir unreadable` row is correctly excluded from ordering, which shows the spec intends the ordering to be complete.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §9.5: sort key is the slug wherever one exists (so a reserved-name row sorts by slug while displaying its filename), filename only for `bad name` rows, comparison case-insensitive with a byte-wise tie-break, and the pinned directory row excluded from ordering entirely.

---

### 13. User-facing copy is unfixed for every new panel and CLI message

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §9.2, §9.8, §9.10, §9.13, §12.1, §12.2

**Details**:
The spec pins exact copy in some places (`⚠ themes dir unreadable`, the seven terse reason labels, the vertical footer's `⏎ set theme` / `d set as dark` / `l set as light` / `esc close`, the footer strings in §14.2, doctor's "N checks passed · 2 advisories" as an example) but leaves the rest as descriptions:

- The §9.2 slot-from-constant confirm ("naming the constant that will be cleared") — wording, and how `y` is advertised.
- The §9.13 failed-commit line ("`⚠` plus a terse statement that the theme could not be saved").
- The §9.10 `NO_COLOR` block flash.
- The §9.8 narrow-terminal refuse flash and the resize-close flash.
- §12.1 `export` stderr messages for an invalid drop-in and an unknown slug.
- §12.2's per-file doctor advisory line format.

Given Portal's convention of single-sourcing exact strings (e.g. `spawn.UnsupportedNoopMessage`) and that these strings are the whole feedback surface for states the user cannot otherwise diagnose, leaving them to the implementer is a real design decision — and one that tends to get re-litigated at the visual gate.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: User decision (asked, not baked): pin all of it. New §14A tables the exact copy for the panel message slot, rows, header and footer; the three flashes; export's three stderr cases; and doctor's four line formats — with the note that panel wording is a layout constraint at 24–30 columns.

---

### 14. `portal theme export`: argument, exit-code and on-ramp details

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §12.1 (`portal theme export <slug>`), §5.5 (directory states)

**Details**:
§12.1 settles the important calls (byte-faithful output, both slug domains, bootstrap-exempt, refuse-with-stderr) but leaves gaps that surface immediately in implementation:

- **Zero or multiple args** — a Cobra `Args` rule is needed; unstated.
- **The non-zero exit value** — "non-zero exit" twice, no value. Portal's doctor drives a scriptable code; export should state whether it uses 1 for all failure classes or distinguishes unknown-slug from invalid-file.
- **A slug failing the charset check** (`portal theme export Nord`) — `bad name` or `not found`? §9.4 makes exactly this distinction for the panel ("Telling a user their file is missing when they typed an illegal name sends them looking in the wrong place"), so the same discrimination presumably applies here.
- **The documented workflow fails on a virgin install.** §12.1's example is `portal theme export nord > ~/.config/portal/themes/nord-lee.theme`, but §5.5 says Portal "never creates or seeds" the directory and a shell redirect will not `mkdir`. The published on-ramp needs the `mkdir -p` step (or `docs/theming.md` must carry it), otherwise the first thing a user meets is a redirect error.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §12.1: ExactArgs(1); exit 1 for every failure class (export is a pipe-into-a-file tool, not a diagnostic — the stderr reason discriminates); a charset-failing slug refused as `bad name` not `not found`, matching §9.4's discrimination. The published workflow gains the `mkdir -p` line, since Portal never seeds the directory and a redirect will not create it.

---

### 15. `capturetool --theme`: default value and invalid-input behaviour

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §13.3 (harness changes)

**Details**:
§13.3 replaces `--appearance` with `--theme`, accepting "a built-in slug **and an explicit path to a real theme file**". Three things an implementer must decide:

- **The default when the flag is omitted.** `--appearance` had one (fixtures render in a defined mode today); with three built-ins and no prefs read allowed, the default must be a named slug. Every capture an agent takes without passing the flag depends on this.
- **Slug-vs-path disambiguation.** `nord` vs `./nord.theme` vs `nord.theme` in cwd — is the discriminator a path separator, the `.theme` suffix, or an explicit `--theme-file` split?
- **Invalid input behaviour.** A path that does not parse, or a slug that is not a built-in: hard error with the §6.2 reason and non-zero exit, or fall back? Falling back would silently render the wrong theme at a visual gate — the failure mode this tool exists to prevent — so it wants stating.

The path form is called out as "the only visual-verification route for someone authoring a drop-in", which raises the cost of getting these wrong.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §13.3: default `tokyo-night` when the flag is omitted; slug-vs-path discriminated by the `.theme` suffix rather than a path separator; invalid input is a hard error with the §6.2 reason and non-zero exit, never a fallback — silently rendering the wrong theme at a visual gate is the failure the tool exists to prevent.

---

### 16. `theme` log events: cardinality and attr usage undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §12.3 (the `theme` log component)

**Details**:
The event catalogue is complete but three details are left open, and this is a **closed, spec-governed** taxonomy where call-site invention is explicitly prohibited:

- **`theme: loaded` cardinality.** "At TUI construction. Resolved slug(s) only" — one line carrying both slugs, or one line per slot? The declared attrs (`slug`, `slot`) support either. Under adaptive there are two nominated themes; under a constant, one.
- **The `token` attr has no declared consumer.** §6.2 puts "*which* token is missing" in doctor, not the log, and no catalogued event mentions it. Either an event carries it (presumably `theme: rejected`) or it should not be in the closed list.
- **`theme: appearance migrated` is INFO / "one-shot"**, but §10.5 makes the migration write best-effort with retry on the next launch ("A failed write means Portal renders the correct theme this launch and retries next launch"). So the line can legitimately fire on several consecutive launches. Whether it is emitted on *compute* or on *successful persist* determines whether "one-shot" is accurate.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §12.3: `theme: loaded` is one line per nominated theme (keeping slug/slot single-valued and greppable); `token` declared as carried by `theme: rejected` where the reason names one, which is its only consumer; `theme: appearance migrated` tied to successful persist rather than compute, which is what makes "one-shot" true and makes its absence the signal that the write failed.

---

### 17. §7.7's re-derivation check does not identify the six values it operates on

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §7.1 (a built-in *is* a theme file), §7.7 (MV's erratum values)

**Details**:
§7.7 is a gating check — "**The built-in-set decision is conditional on this check**" — with a threshold, acceptance criteria and a supersession rule. What it never states is **which six values** are in scope. They are identified only indirectly: "MV's six corrected light values are described in-source as *'darkened, hue-preserved'*".

That creates an ordering dependency the spec does not name: §7.1 **deletes** MV's inline erratum comments, which are the only record identifying the six. If a task deletes them before §7.7 runs (a natural order — the comments go when the values move into `.theme` files), the check's input set is gone.

Also undefined: what happens if a re-derived value is produced but **rejected at its fresh visual gate** (§7.7 assumes replacement is accepted) — keep the shipped value, iterate, or escalate.

Listing the six tokens in §7.7 would make the check self-contained and remove the ordering trap.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §7.7 now names all six §2.9 erratum corrections with original → shipped values under their new token names, plus text.tertiary as a seventh darkening carrying the same risk, and excludes accent.primary explicitly (never darkened). The ordering trap is named: §7.1 deletes the comments that are otherwise the only record. Also added: a re-derived value rejected at its visual gate leaves the shipped value standing.

---

### 18. `key = value` splitting when a line contains more than one `=`

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Priority**: Minor
**Affects**: §4.2 (lexical rules)

**Details**:
§4.2's branch table is presented as exhaustive over the reason labels, and covers no-key, no-`=`, empty-value, duplicates, quoting, whitespace and BOM placement. It does not cover a line with two or more `=` — e.g. `text.primary = #ECEFF4 = x`, or a stray `=` pasted into a value.

Both plausible parsers reject, but with **different user-visible reasons**: split-on-first-`=` yields `bad colour` (the value is `#ECEFF4 = x`), while "a well-formed pair has exactly one `=`" yields `bad syntax`. §6.2 makes each reason map to exactly one condition, and §4.2 frames each branch as a test case, so the choice is contract-level rather than incidental.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved in §4.2's branch table: split on the first `=`, so a stray second `=` lands in the value and fails as `bad colour`. Consistent with the comment rule — the format never re-interprets anything right of the first separator.

---
