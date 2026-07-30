# Review Tracking: Theming System - Input Review

Cycle 6. Full fresh pass over the whole specification against the whole discussion source.

## Findings

### 1. The capture harness is recorded in-source as flaky-on-write, and the spec has since made VHS the *sole* PNG mechanism

**Source**: `discussion/theming-system.md` — "The premise correction — committed PNGs were scaffolding, not an asset" (line 2338: *"43 committed reference PNGs × 4 built-ins = 172 images on a harness **already recorded as flaky-on-write**"*), against "Two mechanisms, two audiences" (lines 2391–2401: *"**Without a producible PNG the agent cannot see what it built**"*)
**Category**: Enhancement to existing topic
**Affects**: §13.3 (Harness changes required — the PNG-production bullet); cross-ref §13.1 (the agent/human table)

**Details**:

The source names flakiness as a **property of the capture harness**, in passing, while making a different point (the matrix cost). It was a minor detail at the time because the harness was being analysed as a cost centre. It is not minor now, because a later decision changed what rests on it:

- §13.1 makes a producible PNG a **hard requirement** — *"Without a producible PNG the agent cannot see what it built, and every task ends up hand-corrected — the exact failure mode this tooling exists to prevent."*
- §13.3 then resolves the mechanism question by **committing to VHS as the only route** — *"PNG production stays on VHS. No direct writer, no new dependency."*
- §13.2 deletes every existing tape and requires **new tapes written per fixture as work proceeds**, so every capture in this feature's implementation loop is a fresh first-time write through that mechanism.

So the single mechanism the agentic self-review loop depends on is the one the source flags as unreliable at exactly the step that matters (writing the image), and the spec carries no acknowledgement of it. The failure mode is silent and directional: the tape runs, no error surfaces, and the agent pixel-checks a **stale or absent** PNG — which reads as "the change didn't render" or, worse, as "the previous capture is my new one", i.e. a false pass at a visual gate. This feature is unusually exposed to it because a theme change is *only* visible in the image; there is no functional assertion that would catch a capture that never landed.

This is not a request to reverse the VHS decision (settled explicitly by the user in cycle 2). It is that the decision was taken on "VHS already satisfies the requirement", and the source's own record of *how well* it satisfies it did not travel with it. The mitigation is procedural and cheap — verify a fresh write (the file's hash changed) before trusting or reviewing a capture, and retry — and it belongs beside the mechanism it qualifies, since §13.1's whole argument is that the agent cannot see its work without it.

Adjacent, same paragraph: the spec's §13.2 correctly rescales the matrix figure (43 → 129, not 172), so the numeric half of that source sentence was carried and the qualitative half was not.

**Current**:

> §13.3:
> - **PNG production stays on VHS. No direct writer, no new dependency.** The hard requirement is that **every fixture can produce a PNG** (§13.1) — that is what the mechanism must satisfy, and VHS already satisfies it. Rasterising styled ANSI needs a terminal-cell renderer with an embedded font and fixed cell metrics, which would mean a real module dependency plus a font asset in a repo that has deliberately avoided both, to replace a mechanism that works.
>
>   §13.2 deletes the *current* tapes along with the images, because both are scaffolding tied to the pre-rename, pre-split screens. **New tapes are written per fixture as work proceeds and cleared out after sign-off**, under exactly the same retention rule as the images (§13.2) — a tape is scaffolding, not an asset. VHS also remains the route if a gif is ever wanted for motion.

> §13.1:
> | **A producible PNG per fixture**, via VHS (§13.3) | The **agent** | … **Without a producible PNG the agent cannot see what it built**, and every task ends up hand-corrected — the exact failure mode this tooling exists to prevent. |

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.3 gains a bullet recording that the harness fails silently on write and that this feature is unusually exposed (a theme change is visible only in the image; no functional assertion catches a capture that never landed), with the mandatory mitigation: verify the file's hash changed before trusting or reviewing a capture, and retry. Qualifies the cycle-2 VHS decision rather than reopening it. Corroborated by prior recorded experience with this harness.

---

### 2. The §13.6 reshape table names three of the tests split invalidates; ten more in the same file are rewritten by the same decisions and are never named

**Source**: `discussion/theming-system.md` — "The unwalked legs" (line 903: *"measured against roughly half of `contrast_test.go`'s **fourteen tests**"*), "Verification — four resolutions" (lines 2489–2491: *"`contrast_test.go` currently measures against **two hardcoded canvases**; under split each theme carries its own `canvas` token, so the test resolves its reference background **from the theme**"*), and "Floor-check enrolment is automatic (F6)" (lines 2527–2534: *"the floor test **auto-enumerates the embedded set**"*). Checked against `internal/tui/theme/contrast_test.go`.
**Category**: Enhancement to existing topic
**Affects**: §13.6 (Guard-test reshape table); cross-ref §13.5 (Floor-check enrolment, contrast_test.go's reference background)

**Details**:

The source states the reshape at **file** granularity — "`contrast_test.go`'s fourteen tests", "the test resolves its reference background from the theme", "the floor test auto-enumerates". The specification renders it at **test** granularity: §13.6 is a per-test table that names every affected test individually, and cycle 1 established that the table's completeness is load-bearing (three rows were added then for exactly this reason).

Measured against the file, the table names **three** of the fourteen — `TestEveryTokenHasLightVariant`, `TestLightSurfaceTintsPinned`, `TestLightTintFillsArePerceptible`. One more (`TestContrastMath`) is pure ratio math and genuinely untouched. The remaining **ten are all rewritten by this feature and appear nowhere in the specification**:

`TestForegroundFloorAgainstOwnCanvas`, `TestTextDimHeldToThreeToOneFloor`, `TestTextFaintDecorativeBand`, `TestBgSelectionPairRule`, `TestBgWarningPairRule`, `TestInlineFlashWarningPairClearsFloor`, `TestPreviewPeekChromeClearsFloorAgainstCanvas`, `TestBgTrackPairRule`, `TestForegroundOnTintPairings`, `TestStateGreenClearsCanvasAndSelection`.

The rewrite is not incidental. Each one currently: reads `theme.MV.<Field>` directly (a package-level value §3.2 **deletes**), addresses `.Dark`/`.Light` on a `Token` (a shape §3.2 **collapses** to `{Name, Value}`), runs a `/dark` + `/light` subtest pair per token (a mode axis that no longer exists), and measures against the package constants `canvasDark`/`canvasLight` (the two hardcoded canvases §13.5 and §15.2 explicitly **retire**). Every one of the four things they are built on is removed by this feature, and each must additionally gain the auto-enumeration over the embedded set that §13.5 requires — which is a structural change (a loop over themes wrapping a loop over tokens), not a find-and-replace.

Three consequences of leaving them unnamed:

- **§13.5's "the floor test" reads as one test.** It is ten, and the enrolment assertion ("every embedded theme appears in the light/dark table") has to compose with all of them. Scale matters to whoever plans this: the floor suite is the single largest mechanical surface in the test reshape and currently reads as a one-liner.
- **Two rules §13.5 states canonically have named carriers the spec never mentions.** The three-leg warning-band rule is `TestBgWarningPairRule`; the single-token dual clearance that caught the Nord green is `TestStateGreenClearsCanvasAndSelection`. The source names both, at the point where each rule is derived. §13.6 names test carriers for every other rule it discusses.
- **The table reads as complete.** It is the enumeration a reader checks against, and it lists five deletions/survivals plus six new tests — an implementer would reasonably conclude the existing floor suite needs nothing, when in fact it does not compile after §3.2.

**Current**:

> §13.6 (the table's existing membership, abbreviated): `TestMVTokenCount` (20 → 19) · `TestMVDarkVariantsPinned` (deleted) · `TestLightSurfaceTintsPinned` (survives, per-light-theme) · `TestEachTokenCarriesLightVariant` (deleted) · `TestEveryTokenHasLightVariant` (deleted) · `TestLightTintFillsArePerceptible` (survives, per-light-theme) · Loader/parser test (new) · Prefs + migration test (new) · Embedded-set validity (new) · Swap-and-diff guard (new) · `RestoreTerminalBackground` anchor (new) · `docs/theming.md` guard (new) · `keymap_dispatch_guard_test` (extended) · Colour-literal guard (unchanged).

> §13.5: **Floor-check enrolment is automatic.** The floor test **auto-enumerates the embedded set**, so a new built-in is checked by default.
>
> §13.5: **`contrast_test.go` resolves its reference background from the theme.** It currently measures against two hardcoded canvases; under split each theme carries its own `canvas` token, so the reference comes from the theme rather than from a constant.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: §13.6 gains a row naming all ten rewritten floor tests, with the four removed foundations they rest on (theme.MV, .Dark/.Light, the /dark+/light subtest axis, the canvasDark/canvasLight constants), the structural nature of the auto-enumeration change, the fact that they do not compile after §3.2, and the two that are named carriers of rules §13.5 states canonically. §13.5's "the floor test" corrected to plural with a pointer.

---

### 3. Whether a `portal doctor` run emits `theme` log events is undefined, and the catalogue's cadences say two different things about it

**Source**: `discussion/theming-system.md` — "Write-path robustness … The themes directory itself (F11)" (lines 1948–1951: *"Unreadable, or a regular file where a directory belongs, gets a **doctor advisory line and a log entry**"*), against "Log cadence, corrected for lazy discovery" (lines 2030–2040: `theme: enumerated` *"at panel open"*) and "Correction — the exec-path justification was wrong" (lines 1898–1922: the log earns its place because *"a **TUI launch** that rejects a theme should leave a passive record"*)
**Category**: Gap/Ambiguity
**Affects**: §12.3 (event catalogue — the cadence column), §12.2 (`portal doctor`); cross-ref §5.5

**Details**:

`portal doctor` is the **second** surface that enumerates the themes directory and evaluates every file against the §6.2 ladder. Whether that enumeration emits under the `theme` component is not stated, and the specification's own text points both ways:

- **Points at "yes":** §5.5 pairs them directly — an unreadable directory gets *"a **doctor advisory line** and a **log entry**"*, one sentence, one condition — and §12.3's `theme: directory unusable` cadence is *"Per enumeration where the themes directory is unreadable"*, which is a condition doctor meets by definition. Read literally, doctor emits at least that one event.
- **Points at "no":** every other cadence in the same table is TUI-scoped — `theme: loaded` *"At TUI construction"*, `theme: enumerated` *"At panel open"*. Doctor constructs no TUI and opens no panel, so a doctor run that finds three broken files would emit `theme: directory unusable` and **nothing else** — no `enumerated`, no per-file `rejected` — which is an odd shape for a catalogue whose stated job is the forensic trail of loader outcomes.

The ambiguity is not cosmetic. §12.3 declares the vocabulary **closed and spec-governed** and `theme: rejected` is deduplicated *per process* — a rule that only has a determinate meaning once it is known which processes emit it. And the volume differs materially: doctor is the surface a user runs *because* something is wrong, so it is the run most likely to produce a full WARN set, into a log the source specifically wants as the record that exists "without the user going looking".

There is a reasonable case either way, which is why it needs deciding rather than inferring. Emitting makes the trail complete and matches §5.5's own pairing. Not emitting keeps the component's cadence story simple (the loader logs where the *product* uses a theme; doctor's whole output is already the diagnostic, printed to the user's screen) and avoids a diagnosis command writing WARNs about a state it just reported — which is adjacent to the reasoning §10.5 and §12.2 use to keep doctor's read-only claim literal.

The same question applies, more narrowly, to `portal theme export`: it parses and validates a theme and can refuse it with a §6.2 reason, which is a `theme: rejected`-shaped outcome on a third non-TUI surface.

**Proposed Addition**:

**Resolution**: Approved
**Notes**: Resolved: the `theme` component records where a theme is *used*, never where one is *diagnosed* — neither doctor nor export emits any event. Reasons: the log's job is the record that exists without the user going looking (doctor is the user looking), doctor would produce the largest WARN volume on the surface needing it least, and it keeps doctor's read-only claim literal per §10.5/§12.2. §5.5's directory-states row clarified so its "log entry" is explicitly the TUI-path one. Also makes `theme: rejected`'s per-process dedup determinate.

---
