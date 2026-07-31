# Phase 7: `portal doctor` theme advisories — 7 tasks

## theming-system-7-1

### Task 7.1: Doctor's closing summary — Portal-health checks counted apart

**Problem**: Doctor's contract is a scriptable exit code — 0 iff every check passes — but the report today is a header plus one line per check with **nothing trailing**, so the exit code's meaning has to be inferred by counting `✓`/`✗` glyphs. §12.2 is about to add a second class of line that deliberately does *not* drive the exit code, and once two classes coexist an unsummarised report cannot tell a reader which findings mattered. The summary is also a **new line on every run**, so every existing doctor output assertion changes with it — which is exactly why it ships on its own, before the advisory class, rather than arriving tangled with it.

**Solution**: A counting helper over the existing `[]checkResult` that counts **Portal-health checks only**, plus §14A's two pinned summary forms rendered as the last line of `renderDoctorReport`.

**Outcome**: Every doctor run — plain and `--fix` — ends with `<N> checks passed` or `<N> of <T> checks passed`, `N == T` is exactly equivalent to `doctorUnhealthy(results) == false`, and the informational host-terminal line and any not-evaluable check sit outside both counts so they can never make a healthy run read as partial.

**Do**:
- Add `func doctorCheckCounts(results []checkResult) (passed, total int)` to `cmd/doctor.go`, sited beside `doctorUnhealthy`. Membership is pinned per status and each arm carries its justification in-source:
  - `checkPass` → `passed++` **and** `total++`.
  - `checkFail` → `total++` only.
  - `checkUnknown` → `total++` only — the iota-0 sentinel already counts as unhealthy in `doctorUnhealthy`, so it must count identically here or the two disagree about the same run.
  - `checkInfo` and `checkNotEvaluable` → **neither count**. Both are documented as never driving the exit code; counting either in `<T>` alone would render `6 of 7 checks passed` beside exit 0, breaking the summary's stated job of making the exit code legible.
- Add `func doctorSummaryLine(results []checkResult) string` returning `fmt.Sprintf("%d checks passed", n)` when `n == t` and `fmt.Sprintf("%d of %d checks passed", n, t)` otherwise. **No singular carve-out** — §14A pins exactly one form for the checks count (unlike the advisory suffix in task 7-2, which pins both singular and plural), and the catalog never has a single member. Do not route it through `pluralCount`.
- Render the summary from `renderDoctorReport` as the **last** line it writes, after the per-check loop: two-space indent matching the body lines, no marker column, no name column, no blank line before it. Doc-comment that the indent-and-no-blank-line framing is a local choice — §14A pins the copy, not the surrounding whitespace — so a later copy change has one obvious home.
- Doc-comment both helpers with the boundary that defines this task: the summary **explains** the exit code and never computes it. `ErrDoctorUnhealthy`, `doctorUnhealthy`, `runDoctorDiagnosis`, the ordered catalog and every `checkResult` producer are untouched.
- Add a direct equivalence test asserting `passed == total` ⟺ `!doctorUnhealthy(results)` over a table covering every status and the mixed combinations (pass-only, one fail, one unknown, one info, one not-evaluable, and info+not-evaluable alongside all-pass). Assert it as a property, never assume it.
- Update every existing doctor output assertion for the new trailing line: `cmd/doctor_test.go`'s `TestDoctorAllStateChecksPassExitsZero`, `TestDoctorFreshInstallReportedHonestly`, the `--fix` tests that count `Portal doctor:` occurrences (`TestDoctorExecuteStaleEntryReturnsUnhealthy`, `TestDoctorFixPrunesStaleEntriesThenRediagnosesClean`, `TestDoctorFixDownServerPrunesProjectsButNotHooks`, `TestDoctorFixLogSweepNeverDrivesExit`), and any other assertion over the rendered report in `cmd`. The `--fix` path must now see **two** summary lines, one per `renderDoctorReport` call.

**Acceptance Criteria**:
- [ ] A run where all seven health checks pass and the host-terminal info line is present ends with `  7 checks passed` — the info line is in neither count.
- [ ] A run with one `checkFail` among seven health checks ends with `  6 of 7 checks passed` and still exits non-zero.
- [ ] A run with one `checkNotEvaluable` among seven ends with `  6 checks passed` (`N == T == 6`) and exits 0 — the not-evaluable check is in neither count.
- [ ] A `checkUnknown` result yields `N < T` and `doctorUnhealthy == true`; the two agree.
- [ ] `doctorCheckCounts` and `doctorUnhealthy` agree on every table row: `passed == total` iff the run is healthy.
- [ ] The summary is the final line of `renderDoctorReport`'s output, with no trailing blank line and no blank line between it and the last check line.
- [ ] `portal doctor --fix` prints exactly two summary lines — one per report render — and both reflect their own pass.
- [ ] `ErrDoctorUnhealthy`, `doctorUnhealthy`, `checkMarker`, `runDoctorDiagnosis` and the catalog order are byte-unchanged; the exit code for every existing scenario is unchanged.
- [ ] The summary copy is single-sourced in `doctorSummaryLine` — no format string for it exists at any other site.

**Tests**:
- `"it renders the all-passed summary"` — `TestDoctorSummary_AllChecksPassed`
- `"it renders the partial form when a check fails"` — `TestDoctorSummary_PartialForm`
- `"it leaves the informational host-terminal line outside both counts"` — `TestDoctorSummary_InfoLineOutsideCounts`
- `"it leaves a not-evaluable check outside both counts"` — `TestDoctorSummary_NotEvaluableOutsideCounts`
- `"it counts the unknown sentinel toward the total only"` — `TestDoctorSummary_UnknownCountsTowardTotalOnly`
- `"it keeps the summary and the exit code in agreement"` — `TestDoctorSummary_MatchesDoctorUnhealthy` (table over every status combination)
- `"it renders the summary as the last line"` — `TestDoctorSummary_IsTheLastLine`
- `"it renders one summary per report render"` — `TestDoctorSummary_FixPathRendersTwo`
- `"it uses no singular form for the checks count"` — `TestDoctorSummary_NoSingularCarveOut`

**Edge Cases**:
- The summary is a **new** line on every run — today's report is a header plus one line per check with nothing trailing, so every existing doctor output assertion gains it. §14A names this as the amendment, not a regression.
- Two forms only: `<N> checks passed` when all pass, `<N> of <T> checks passed` when any fails — the failing form is the one the summary exists for, since that is when the exit code needs explaining.
- `<N>` and `<T>` count **Portal-health checks only** — the class that drives the exit code — so the informational `host terminal` line (`checkInfo`, rendered without a marker) sits outside both and cannot make `N < T` on a healthy run.
- `checkNotEvaluable` never drives the exit code either, so its participation is pinned deliberately: counting it in `<T>` but not `<N>` would render `6 of 7 checks passed` beside exit 0.
- The `checkUnknown` zero-value sentinel counts as unhealthy in `doctorUnhealthy` and must count identically here, or the two disagree about the same run.
- `N == T` must be exactly equivalent to `doctorUnhealthy == false`, **asserted rather than assumed**.
- The summary renders once per report render, so `--fix` prints two — one per `renderDoctorReport` call.
- This task explains the exit code and never computes it: `ErrDoctorUnhealthy` and the pass/fail catalog are untouched.

**Context**:
> §14A pins the copy: *"Closing summary, all checks passed → `<N> checks passed`. Closing summary, some checks failed → `<N> of <T> checks passed` — the failing case is the one the summary exists for, since it is when the exit code needs explaining."* And: *"`<N>` and `<T>` count **Portal-health checks only** — the class that drives the exit code (§12.2). Advisories are counted separately by `<M>` and never fold into either… The summary line is **new**: today's report is a header plus one line per check with no trailing summary, so every run gains a line — that is the amendment §15.1 names, not a regression."*
> §12.2: *"**Doctor's closing summary distinguishes the two counts** — e.g. 'N checks passed · 2 advisories' — so the exit code's meaning is legible without reading the contract."*
> §15.1 lists this as one of the three named amendments this feature carries: *"**The `portal doctor` contract.** Two classes of line — Portal-health checks driving the exit code, user-content diagnostics carrying `⚠` and not driving it — plus a closing summary distinguishing the counts."*
> **Ambiguity flagged**: the spec pins the summary's *copy* but not its indentation, nor whether a blank line separates it from the catalog. Two-space indent (matching the existing body lines) with no separating blank line is chosen for visual continuity with the report's existing shape; record the choice in a source comment so a later phase can revisit it deliberately.
> Phase boundary: the ` · <M> advisory|advisories` suffix and the advisory block itself are task 7-2, which appends to whichever form this task produces.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.2, §14A, §15.1

## theming-system-7-2

### Task 7.2: The advisory line class — a trailing `⚠` block outside the exit code

**Problem**: Theme findings are **user-content diagnostics**, not Portal-health checks: there is deliberately no repair path, so a failing theme line would hold doctor permanently non-zero until someone hand-edits a file, and an automated health check would fire about the resurrection machinery because a user left a half-written palette in `themes/`. Modelling one as a `checkResult` carrying a new status would put it inside the struct whose every consumer — `checkMarker`, `doctorUnhealthy`, task 7-1's counts — treats it as a check, so the exclusion would have to be re-argued at three sites. The block's *position* matters just as much: the catalog is a fixed-order, fixed-length report, while the theme class is 0..N lines whose cardinality depends on the contents of a directory.

**Solution**: A second, distinct line class in `cmd/doctor.go` — no name column, no pass/fail marker, no participation in `<N>`/`<T>` — rendered by `renderDoctorReport` as a trailing block between the catalog and task 7-1's summary, with the ` · <M> advisory|advisories` suffix appended to whichever summary form applies.

**Outcome**: `renderDoctorReport` emits three regions in a fixed order — catalog, advisory block, summary — an all-passing catalog plus any number of advisories still exits 0, and a clean install's report gains no stray punctuation because the suffix is suppressed entirely at M=0.

**Do**:
- Declare the class in `cmd/doctor.go`:
  ```go
  // advisory is a user-content diagnostic line: the §15.1 contract amendment's
  // second class. It carries NO name column, NO pass/fail marker and NO
  // participation in <N>/<T>, and it can never make doctorUnhealthy true.
  // line is the full §14A copy INCLUDING its leading "⚠ " (the copy table pins
  // each line with its glyph, so keeping them whole lets a copy test compare
  // against the table verbatim). slug and fromPrefs are the identity task 7-6's
  // one-slug-one-line union dedups on; the renderer reads only line.
  type advisory struct {
      line      string
      slug      string
      fromPrefs bool
  }
  ```
- Change the renderer to `func renderDoctorReport(w io.Writer, results []checkResult, advisories []advisory)` and emit, in this order and never interleaved:
  1. the `Portal doctor:` header;
  2. the ordered check catalog, one line per `checkResult` — **including** the informational host-terminal line, which stays at the end of the catalog (the report has three regions, not two);
  3. the advisory block: one line per advisory, two-space indented like the body lines, the advisory's `line` written verbatim with nothing prepended or appended. Zero advisories render **nothing at all** — no blank line, no heading.
  4. task 7-1's summary, with the suffix below.
- Extend `doctorSummaryLine` (or wrap it) to take the advisory count and append ` · ` + `pluralCount(m, "advisory", "advisories")` when `m > 0`, appended to **whichever** summary form applies (`7 checks passed · 1 advisory`, `6 of 7 checks passed · 3 advisories`). At `m == 0` the suffix is **suppressed entirely** — not rendered as ` · 0 advisories`.
- Update `doctorCmd.RunE` to pass an advisory slice to both `renderDoctorReport` calls. In this task production supplies an **empty** slice; task 7-3 introduces the first producer.
- Do **not** touch `doctorUnhealthy`'s signature or behaviour: advisories are never passed to it, which is the structural form of "an advisory can never drive the exit code". Assert it directly — an all-passing catalog plus N advisories must Execute to exit 0.
- Doc-comment the interleaving rule where the renderer builds region 2 → region 3: the catalog is one line per check in a fixed order, the theme class is 0..N lines depending on user content, and interleaving would make a fixed-order report vary in length and position with the contents of a directory.
- `⚠` is Portal's established warning glyph (MV §2.2) and is **glyph-backed**, so the class survives a colourless terminal with no colour applied anywhere in the doctor renderer.

**Acceptance Criteria**:
- [ ] With advisories present the rendered output is exactly: header, every check line in catalog order, every advisory line, then the summary — verified by index, not by substring presence.
- [ ] No advisory line ever appears between two check lines, and the host-terminal info line is the last line of the catalog region.
- [ ] `7 checks passed · 1 advisory` at M=1 and `7 checks passed · 3 advisories` at M≥2; the same suffix appends to the `6 of 7 checks passed` form.
- [ ] At M=0 the summary is byte-identical to task 7-1's output — no ` · `, no `0 advisories`.
- [ ] An all-passing catalog plus 5 advisories Executes with exit 0 and a nil error; `doctorUnhealthy` is not consulted with advisory input.
- [ ] A failing check plus advisories renders **one** summary line carrying both counts, and exits non-zero.
- [ ] An advisory line renders with no pass/fail marker and no name column — its text begins with `⚠` after the indent.
- [ ] Zero advisories render zero bytes between the last check line and the summary.
- [ ] `<M>` is the length of the slice the renderer was handed — it is never recomputed from a producer's raw counts.

**Tests**:
- `"it renders advisories after the catalog and before the summary"` — `TestAdvisories_BlockPositionIsFixed`
- `"it never interleaves an advisory with a check line"` — `TestAdvisories_NeverInterleave`
- `"it keeps the host-terminal line at the end of the catalog"` — `TestAdvisories_HostTerminalStaysInCatalog`
- `"it suffixes the summary with the advisory count"` — `TestAdvisories_SummarySuffix` (table: M=1 singular, M=2 plural, against both summary forms)
- `"it suppresses the suffix at zero advisories"` — `TestAdvisories_SuffixSuppressedAtZero`
- `"it never drives the exit code"` — `TestAdvisories_NeverDriveExitCode` (all-passing catalog + N advisories → exit 0)
- `"it renders both counts when a check also failed"` — `TestAdvisories_FailingCheckAndAdvisoriesShareOneSummary`
- `"it renders the warning glyph with no pass/fail marker"` — `TestAdvisories_GlyphBackedNoMarker`
- `"it renders nothing for an empty advisory block"` — `TestAdvisories_EmptyBlockRendersNothing`

**Edge Cases**:
- Advisories are a **second class of line**, not a `checkResult` carrying a new status — no name column, no marker, no `<N>`/`<T>` participation. That is precisely the §15.1 contract amendment.
- The block renders **after** the ordered catalog and **before** the summary and never interleaves: the catalog is fixed-order and fixed-length, the theme class is 0..N lines whose cardinality depends on user content.
- The informational host-terminal line stays at the end of the **catalog**, so the report has three regions rather than two.
- `⚠` is Portal's established warning glyph (MV §2.2) and is **glyph-backed**, so the class survives a colourless terminal.
- The suffix is ` · <M> advisory` at M=1 and ` · <M> advisories` above, appended to whichever summary form applies, and **suppressed entirely at M=0** so a clean install's report gains no stray punctuation.
- `<M>` counts **lines** — problems, not detections — which is what task 7-6's one-slug-one-line rule exists to keep true.
- An advisory can **never** make `doctorUnhealthy` true, asserted directly with an all-passing catalog plus N advisories exiting 0.
- A failing check plus advisories renders both counts in the one summary line.
- The rejected alternative was failing the exit code on a broken drop-in — the user already gets the panel row and the doctor line without conscripting a signal that means the resurrection machinery is broken.

**Context**:
> §12.2: *"**Theme lines are advisory and do NOT drive the exit code — this amends doctor's contract.** … Because there is deliberately no repair path, a failing theme line would go **permanently** non-zero until someone hand-edits a file… The exit code exists as a signal about the **resurrection machinery** — daemon alive, hooks registered, state sane. A stray junk file in `themes/` is not that."*
> §12.2's two-class table: Portal-health checks keep the existing pass/fail markers and drive the exit code; user-content diagnostics carry **`⚠`** — *"Portal's established warning glyph (MV §2.2, glyph-backed so it survives colourless)"* — and do not.
> §12.2: *"**Advisories render as a trailing block, after the ordered check catalog and before the summary.** They do not interleave: the catalog is one line per check in a fixed order, whereas the theme class is 0..N lines whose cardinality depends on user content and which do not participate in `<N>`/`<T>`. Interleaving would make a fixed-order report vary in length and position with the contents of a directory."*
> §14A: *"Closing summary, advisories present → Either form above plus ` · <M> advisory` at M=1, ` · <M> advisories` above. Suppressed entirely at M=0."* And: *"`<M>` counts lines, so it counts problems rather than detections."*
> **Decision recorded**: §14A gives each advisory's copy *with* its leading `⚠`, so the producers own the whole string and the renderer only indents. Prefixing the glyph in the renderer instead would split one pinned string across two sites.
> Phase boundary: this task ships the class and the renderer with an empty production slice. The four producers are tasks 7-3 (content reasons + the directory line), 7-4 (the filename reasons), 7-5 (the persisted-slug line) and 7-6 (the deduped, ordered union).

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §12.2, §14A, §15.1

## theming-system-7-3

### Task 7.3: Scan the themes directory into per-file advisories

**Problem**: A drop-in theme that fails validity is invisible everywhere except the panel row, which has width for a terse reason and nothing more — so a user with `missing tokens` cannot learn *which* token, and a user with `bad syntax` cannot learn *which* line. Doctor is the only surface with terminal width to enumerate, and until it scans the themes directory a broken palette has no diagnosis route at all. The scan must also stay honest about doctor's two standing contracts: it heals nothing, and it is *the user looking*, so it must not write the largest possible WARN volume into the log on the run that needs it least.

**Solution**: A theme scan in a new `cmd/doctor_theme.go` driving Phase 1's `Loader.Enumerate` over the `themesDirPath`-resolved directory with a **`log.Discard`**-backed event logger, mapping each rejected entry carrying a slug to §14A's generic frame and an unusable directory to its own pinned line.

**Outcome**: `portal doctor` on a themes directory holding one valid and three broken files prints three `⚠` advisories carrying the loader's own detail verbatim, an absent directory prints nothing at all, an unusable one prints exactly `⚠ themes directory unreadable: <path>`, and the whole run leaves zero `theme` log records.

**Do**:
- Add a `ThemesDir string` field to `DoctorDeps` in `cmd/doctor.go`, documented exactly like `StateDir` (empty means "resolved by `resolveDoctorDeps`"; tests set a hermetic temp dir). In `resolveDoctorDeps`, resolve it **best-effort** via `themesDirPath()` alongside the existing best-effort store construction: on error leave it empty and scan nothing. The advisory class has no not-evaluable form, so degrading to zero lines is the only shape available — a resolution failure must never abort the diagnosis.
- Create `cmd/doctor_theme.go` with `func collectThemeAdvisories(deps *DoctorDeps) []advisory`, the single entry point tasks 7-4/7-5/7-6 extend. Return early with `nil` when `deps.ThemesDir == ""`.
- Construct the loader as `theme.NewLoader(theme.NewEventLogger(log.Discard()))` — **`log.Discard`, always**, on every doctor path. Doc-comment the reason: the `theme` component records where a theme is *used*, never where one is *diagnosed*; doctor is the user looking, its whole output is already the diagnostic, and it is the run most likely to hit a full reject set, so emitting would put the largest WARN volume on the surface that needs it least. It also keeps the read-only claim literal.
- Call `entries, dirRej := loader.Enumerate(deps.ThemesDir)`:
  - `dirRej != nil` (unreadable directory, or a regular file where a directory belongs) → one advisory `⚠ themes directory unreadable: <path>` using `deps.ThemesDir` verbatim. `Enumerate` returns no entries in that state, so it is the only theme-file line.
  - `entries == nil && dirRej == nil` (absent directory) → **no line, no error, no log**. Zero drop-ins is not an error and Portal never creates or seeds the directory.
- For each entry with a non-nil `Rejection` whose reason is one of the four this task owns — `missing tokens`, `bad colour`, `bad syntax`, `unreadable` — emit the generic frame `⚠ theme <slug>: <reason> — <detail>`, where `<slug>` is `Entry.Slug` and `<reason>` is the `Reason`'s string value (the terse §6.2 label verbatim). The filename reasons (`bad name`, `reserved name`) are task 7-4 and are skipped here — leave an explicit `default:` arm so the compiler-level exhaustiveness is visible rather than silently dropping them.
- `<detail>` is the loader's own: `Rejection.Detail` verbatim when non-empty, else `Rejection.Err.Error()` (Phase 1 carries `unreadable`'s OS error on the dedicated `Err` field, not in `Detail`). Nothing is re-derived, re-ordered, re-wrapped or double-prefixed here — Phase 1 already renders `missing text.primary, bg.subtle`, `text.primary = #GGGGGG, canvas = blue` and `line 12: duplicate key text.primary`. Assert the `unreadable` line carries the OS error exactly once.
- A valid entry (nil `Rejection`) produces no line.
- Wire it into `doctorCmd.RunE`: `renderDoctorReport(w, results, collectThemeAdvisories(deps))`, replacing task 7-2's empty slice on the plain path (the `--fix` path is task 7-7).
- Keep the scan strictly read-only: no write, no repair, no directory creation, and no `prefs.json` read (that is task 7-5's, through a different seam).

**Acceptance Criteria**:
- [ ] A file missing two tokens produces exactly `⚠ theme <slug>: missing tokens — missing text.primary, bg.subtle`.
- [ ] A file with two bad hexes produces `⚠ theme <slug>: bad colour — text.primary = #GGGGGG, canvas = blue`.
- [ ] A duplicate-keyed file produces `⚠ theme <slug>: bad syntax — line 12: duplicate key text.primary`, naming the **second** occurrence's line.
- [ ] A mode-`0000` file and a dangling symlink each produce `⚠ theme <slug>: unreadable — <OS error verbatim>`, the OS error appearing exactly once.
- [ ] A valid `.theme` file produces no line; a non-`.theme` file produces no line.
- [ ] An **absent** themes directory produces zero advisories, no error, and no log record of any kind.
- [ ] An unreadable directory and a regular file where the directory belongs each produce exactly one line, `⚠ themes directory unreadable: <path>`, and no per-file lines.
- [ ] A `themesDirPath()` failure yields zero advisories and a diagnosis that still renders every check and its summary.
- [ ] A directory holding a full reject set produces **zero** records through a `logtest.Sink` installed for the whole run.
- [ ] Doctor never reports one file under two reasons: each entry contributes at most one line.
- [ ] The themes directory and every file in it are byte-identical after the run, and `prefs.json` is untouched by this scan.

**Tests**:
- `"it reports an invalid theme file with its reason and detail"` — `TestThemeAdvisories_InvalidFileFrame` (table: missing tokens, bad colour, bad syntax)
- `"it reports an unreadable file with the OS error verbatim"` — `TestThemeAdvisories_UnreadableFileKeepsOSError` (0000-mode file, dangling symlink)
- `"it produces no line for a valid file"` — `TestThemeAdvisories_ValidFileIsSilent`
- `"it is silent for an absent directory"` — `TestThemeAdvisories_AbsentDirectoryIsSilent`
- `"it reports an unusable directory as the only theme-file line"` — `TestThemeAdvisories_UnusableDirectoryLine` (table: 0000 directory, regular file in its place)
- `"it degrades when the themes directory cannot be resolved"` — `TestThemeAdvisories_UnresolvedDirDegrades`
- `"it reuses the loader's detail verbatim"` — `TestThemeAdvisories_DetailIsVerbatim`
- `"it reports exactly one reason per file"` — `TestThemeAdvisories_OneReasonPerFile`
- `"it emits zero theme log records"` — `TestThemeAdvisories_EmitsNoThemeRecords` (`logtest.Sink`, full reject set)
- `"it writes nothing and reads no prefs"` — `TestThemeAdvisories_ScanIsReadOnly`

**Edge Cases**:
- The scan runs through Phase 1's `Loader.Enumerate` over the `themesDirPath`-resolved directory, so doctor re-derives no enumeration rule, no §6.2 ladder and no detail format of its own.
- The loader is handed **`log.Discard`**, so a full reject set produces **zero** `theme` records.
- An **absent** directory produces **no line at all**, no error and no log entry — zero drop-ins is not an error and Portal never creates or seeds it.
- An **unusable** directory produces `⚠ themes directory unreadable: <path>` and, since `Enumerate` returns no entries in that state, it is the only theme-file line.
- A `themesDirPath` resolution failure must degrade rather than abort the diagnosis, matching doctor's existing best-effort store construction where a nil store yields a not-evaluable check.
- The generic frame is `⚠ theme <slug>: <reason> — <detail>` for every reason that has a slug in hand (`missing tokens`, `bad colour`, `bad syntax`, `unreadable`).
- `<detail>` is `Rejection.Detail` verbatim — nothing is re-derived, re-ordered or double-prefixed — with `unreadable`'s OS error read off the rejection's `Err` field.
- A duplicate names the **second** occurrence's line, which is the one to delete.
- `unreadable` carries the **OS error verbatim**, the only thing distinguishing a permission denial from a dangling symlink.
- Doctor enumerates **within** the reason and never across, so a file is never reported as both `bad colour` and `missing tokens`.
- A valid file produces no line; the scan performs no write, no repair and reads no `prefs.json`.
- The `0000`-mode file and directory tests need a `chmod` cleanup and should skip when the suite runs as root, where mode bits do not deny.

**Context**:
> §12.2: doctor *"**Scans the themes directory** and reports any file failing validity, with the reason and the specific token/line/key… **Reports an unreadable themes directory** (or a regular file where a directory belongs). An *absent* directory is silent (§5.5)."*
> §6.3's split by surface: the panel carries the terse reason (*"sufficient to tell the user their file did not work and it is not their imagination"*), **doctor** carries *"the detail — full terminal width, per-file, enumerating exactly which tokens are missing or which key is bad"*, and the log is the passive forensic trail.
> §6.2: *"Doctor enumerates within the reason, not across reasons — all missing tokens, or all bad-coloured keys, for the one reason that applies. It does not report a file as both `bad colour` and `missing tokens`."*
> §12.3: *"**The component records where a theme is *used*, never where one is *diagnosed*.** `portal doctor` and `portal theme export` both enumerate or parse and both can hit every §6.2 reason — and **neither emits any `theme` event**"* — the log's job is to be the record that exists without the user going looking; doctor is the user looking; and doctor is the run most likely to hit a full reject set.
> §14A's `<detail>` formats: `missing tokens` → the token names comma-separated; `bad colour` → every offending `key = value` pair comma-separated; `bad syntax` → line number and offending content; `unreadable` → *"The OS error verbatim — it is the only thing that distinguishes a permission denial from a dangling symlink, and doctor is where a verbatim system message belongs."*
> Phase boundary: the two filename reasons and their distinct frames are task 7-4; the persisted-slug line is task 7-5; the one-slug-one-line union and the block's pinned order are task 7-6; the `--fix` path is task 7-7.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §5.5, §6.2, §6.3, §12.2, §12.3, §14A

## theming-system-7-4

### Task 7.4: The filename-reason frames — two `bad name` messages and the reserved-name line

**Problem**: Two of §6.2's seven reasons are decided from the **filename**, before the file is ever opened — and a file rejected that way has **no slug**, so task 7-3's `⚠ theme <slug>: …` frame has nothing to put in its subject. Worse, the generic `<reason> — <detail>` shape actively misdirects for both: a `bad name` file with a legal stem and a wrong-case extension would be told its *name* is wrong when the slug portion is fine, and a `reserved name` file would be told "reserved name" with no hint that the fix is a two-second rename. §6.2 deliberately keeps one reason class because the panel row has no width to discriminate — doctor's width is exactly what these frames spend.

**Solution**: Three additional frames in `collectThemeAdvisories`, all labelled by **filename** rather than slug, selected off Phase 1's `Rejection.BadNameCause` for the two `bad name` variants and off the reason for `reserved name`.

**Outcome**: `Nord.theme` reports a slug problem, `nord.THEME` reports an extension problem, and a drop-in colliding with a built-in reports both the conflict and the concrete rename that fixes it — each in one line, none of them following the generic detail frame.

**Do**:
- Extend `cmd/doctor_theme.go`'s per-entry mapping with the two filename reasons, using `Entry.Filename` (the base name as enumerated, never the full path — §14A's placeholder is `<filename>`):
  - `bad name` with `BadNameCause == BadNameSlug` → `⚠ theme file <filename>: slug must be lowercase letters, digits and hyphens`
  - `bad name` with `BadNameCause == BadNameExtension` → `⚠ theme file <filename>: extension must be lowercase .theme`
  - `reserved name` → `⚠ theme file <filename>: <slug> is a built-in — rename it (e.g. <slug>-mine.theme)`, where `<slug>` is `Entry.Slug` (a `reserved name` entry has a valid slug — that is what collided).
- Doc-comment why the frame differs from task 7-3's: the differing line frame (`⚠ theme file <filename>: …` versus `⚠ theme <slug> …`) is what carries the **input class** — a file versus a slug — which is why §6.2 keeps one reason class while doctor names which cause.
- Doc-comment why the two `bad name` causes take distinct messages: with a bad extension the slug portion is **already legal**, so a single message telling the user to fix their slug sends them to fix the one thing that is fine — the misdirection the spec discriminates against at three other sites.
- Doc-comment why `reserved name` is labelled by filename despite having a valid slug: that slug is *identical* to the built-in's, so labelling by slug would print the same name twice with no way to tell which row is theirs.
- The `reserved name` line deliberately does **not** follow the generic `<reason> — <detail>` frame: it names the conflict **and** the fix, which is what makes §5.4's workaround self-documenting rather than merely short. Assert its exact text, including the `(e.g. <slug>-mine.theme)` suffix.
- Assert — do not merely rely on — that doctor names no built-in slug: the reserved set is derived inside `internal/theme` from `BuiltinSlugs()` (Phase 2 task 2-2), so a future built-in is covered with no edit here. A test loops `theme.BuiltinSlugs()` and seeds one colliding file per slug.
- Add a negative assertion for the ladder's first rung: a `bad name` file whose contents are also broken (bad hex, duplicate key) and whose mode is `0000` still produces **only** the filename line — the filename is decided before the file is opened, so a `bad name` file can never also report `unreadable` or any content reason.

**Acceptance Criteria**:
- [ ] `Nord.theme` produces exactly `⚠ theme file Nord.theme: slug must be lowercase letters, digits and hyphens`.
- [ ] `nord_lee.theme`, `nord lee.theme`, `-nord.theme` and `.theme` each produce the same slug-cause frame, labelled by their own filename.
- [ ] `nord.THEME` and `nord.Theme` each produce exactly `⚠ theme file nord.THEME: extension must be lowercase .theme` (with the respective filename) — the **extension** cause, never the slug cause.
- [ ] A `themes/nord.theme` alongside the built-in `nord` produces exactly `⚠ theme file nord.theme: nord is a built-in — rename it (e.g. nord-mine.theme)`.
- [ ] Every member of `theme.BuiltinSlugs()` produces the reserved-name line when a colliding file is dropped in; the test names no theme.
- [ ] A `bad name` file that is also unreadable and also has a bad hex produces exactly one line, the filename one.
- [ ] A `reserved name` file whose contents are perfectly valid still produces its line — the reason is decided from the slug alone, before any read.
- [ ] No filename-reason line uses the `⚠ theme <slug>:` frame, and no content-reason line uses the `⚠ theme file <filename>:` frame.
- [ ] The three frames are single-sourced in `cmd/doctor_theme.go`; no format string for them exists elsewhere.

**Tests**:
- `"it renders the slug-cause bad-name frame"` — `TestThemeAdvisories_BadNameSlugFrame` (table: `Nord.theme`, `nord_lee.theme`, `-nord.theme`, `.theme`)
- `"it renders the extension-cause bad-name frame"` — `TestThemeAdvisories_BadNameExtensionFrame` (table: `nord.THEME`, `nord.Theme`)
- `"it labels a filename-reason row by filename"` — `TestThemeAdvisories_FilenameReasonsLabelledByFilename`
- `"it renders the reserved-name line naming the conflict and the fix"` — `TestThemeAdvisories_ReservedNameFrame`
- `"it derives the reserved set from the embedded built-ins"` — `TestThemeAdvisories_ReservedSetIsTheEmbeddedSet` (loops `theme.BuiltinSlugs()`, names no theme)
- `"it never reports a bad-name file's contents"` — `TestThemeAdvisories_BadNameNeverReportsContent`
- `"it reports a reserved-name file whose contents are valid"` — `TestThemeAdvisories_ReservedNameDecidedBeforeRead`

**Edge Cases**:
- A `bad name` file has **no slug**, so it takes the `⚠ theme file <filename>: …` frame — the differing frame is what carries the input class (a file versus a slug), which is why §6.2 keeps one reason class while doctor names which cause.
- The two causes take **distinct** messages off Phase 1's `BadNameCause` — `slug must be lowercase letters, digits and hyphens` versus `extension must be lowercase .theme` — because with a bad extension the slug portion is already legal.
- `Nord.theme` fails on the **slug** cause while `nord.THEME` / `nord.Theme` fail on the **extension** cause, and the latter are visible at all only because §5.6 enumerates the extension case-insensitively before rejecting it.
- `reserved name` is labelled by **filename** too even though its slug is valid, because that slug is *identical* to the built-in's and labelling by slug would print the same name twice.
- Its line names the **conflict and the fix** — `<slug> is a built-in — rename it (e.g. <slug>-mine.theme)` — and deliberately does **not** follow the generic `<reason> — <detail>` frame.
- The reserved set is the embedded set itself (Phase 2 task 2-2), never a hand-maintained list, so a future built-in is covered without editing doctor.
- A `bad name` file can never also report `unreadable` or any content reason, the filename being decided before the file is opened.
- The terse §6.2 labels stay the panel's business — doctor's width is what these frames spend.

**Context**:
> §14A pins all three: *"`bad name`, bad **slug** → `⚠ theme file <filename>: slug must be lowercase letters, digits and hyphens`"*; *"`bad name`, bad **extension casing** → `⚠ theme file <filename>: extension must be lowercase .theme` — a distinct message because the slug portion is already legal, and sending the user to fix the one thing that is fine is exactly the misdirection §9.4 and §12.1 discriminate against elsewhere"*; *"`reserved name` conflict → `⚠ theme file <filename>: <slug> is a built-in — rename it (e.g. <slug>-mine.theme)` — the message names the conflict *and* the fix, which is what makes §5.4's workaround self-documenting rather than merely short."*
> §6.2's `bad name` row: *"Three causes across two input classes… One reason class because the user-facing fact is the same in all three, and the panel row has no width to discriminate; doctor and export name which (§14A), and their differing line frames (`⚠ theme file <filename>: …` versus `⚠ theme <slug> …`) are what carry the input class."*
> §6.2's ladder: rung 1 `bad name` — *"the **filename** is checked before the file is opened, so a `bad name` file can never also report `unreadable` or anything about its contents"*; rung 2 `reserved name` — *"likewise decided from the slug alone, before any read."*
> §5.4: *"an invalid theme falls back to a built-in, so **if a user file could shadow the built-in that is the fallback, the fallback itself could be broken** … The workaround is a two-second file rename and is self-documenting: copy `nord` to `nord-lee.theme`."*
> §14A's `<detail>` table records that `reserved name` is *"Covered by its own pinned line above, which names the conflict and the fix rather than following the generic frame."*

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §5.2, §5.4, §5.6, §6.2, §12.2, §14A

## theming-system-7-5

### Task 7.5: The unresolvable-persisted-theme line off the non-migrating prefs read

**Problem**: The failure a user is most likely to hit is not a stray file in `themes/` — it is *the theme they chose no longer applying*: a deleted file, a renamed file, a typo in `prefs.json`. Portal falls back silently by design and never overwrites the persisted name, so without a doctor line the only signal is "my colours changed". Reading `prefs.json` to report it collides head-on with doctor's read-only contract, because `loadPrefsStore` now carries the one-shot `appearance` translation — a config mutation as a side effect of running a diagnosis. And the resolution has to be the *diagnostic* one: `ResolveNomination` would substitute fallbacks (hiding the failure) and can raise task 5-6's fatal (aborting the diagnosis), neither of which belongs here.

**Solution**: A second producer in `cmd/doctor_theme.go` reading through **`loadPrefsStoreNoMigrate`**, deriving the keys in force via `theme.ResolveSetting`, and resolving each **persisted** key through `theme.ResolveByName` on a `log.Discard`-backed loader — emitting §14A's `does not resolve` frame with its slot parenthetical.

**Outcome**: `"theme": "nord-lee"` with the file deleted prints `⚠ theme nord-lee does not resolve: not found`; a broken light slot prints `⚠ theme <slug> (light) does not resolve: <reason>`; two slots naming one broken slug print a single `(both)` line; a virgin install prints nothing; and `prefs.json` is byte-identical afterwards with `theme_migrated` still unset.

**Do**:
- Add a `PrefsStore *prefs.Store` field to `DoctorDeps`, documented and constructed exactly like `HookStore` / `ProjectStore`: in `resolveDoctorDeps`, `if s, err := loadPrefsStoreNoMigrate(); err == nil { deps.PrefsStore = s }`. A nil store produces no lines rather than an error. Doc-comment the **non-migrating** requirement at the assignment: doctor heals nothing on the read-only path, and `loadPrefsStore` (the migrating variant) must never appear anywhere in doctor's call graph — Phase 6 task 6-5's `TestLoadPrefsStore_SingleProductionCaller` source guard is what keeps that true, and this call site must not break it.
- Add `func persistedThemeAdvisories(deps *DoctorDeps, loader theme.Loader) []advisory` in `cmd/doctor_theme.go`, called from `collectThemeAdvisories` with the same `log.Discard`-backed loader task 7-3 constructs (one loader per doctor run, so its per-process dedup state is owned and, being `Discard`, emits nothing regardless).
- Read tolerantly and derive: `keys, _ := deps.PrefsStore.LoadThemeKeys()` (every degenerate file — absent, empty, corrupt JSON, missing fields — yields zero keys with no error), then `setting, raw := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)`. The **raw** keys are what say which values are *persisted*; the setting is what says which are *in force*.
- Select the keys to check, per §8.2's `theme`-wins rule — reporting an ignored key as unresolvable would send the user to fix something Portal is not reading:
  - `setting.IsConstant` → check `setting.Constant` alone, with **no slot parenthetical**. The slots are not read at all, even when persisted and broken.
  - otherwise → check only the slots whose **raw** value is non-empty. An unset slot holds the shipped default, which is a built-in and always resolves, so a virgin install produces no line. When both raw slots are non-empty **and equal**, check once and render the slot as `both`; otherwise check each independently as `light` and `dark`.
- Resolve each via `loader.ResolveByName(slug, deps.ThemesDir)` — **never** `ResolveNomination`, which resolves fallbacks and can raise the fatal. A nil rejection produces no line. Otherwise emit `⚠ theme <slug> (<slot>) does not resolve: <reason>`, or `⚠ theme <slug> does not resolve: <reason>` under a constant. `<reason>` is the terse §6.2 label alone — this frame carries **no** ` — <detail>` tail, unlike task 7-3's file frame.
- Mark each of these advisories `fromPrefs: true` and carry its `slug`, for task 7-6's union.
- Rely on `ResolveByName`'s own discrimination rather than re-deriving it: a charset failure returns `bad name` **before any path is composed** (so `../evil` never becomes a path component), an absent directory or absent file returns `not found`, and an unusable directory returns `unreadable` — permissions being the actual problem. An empty `deps.ThemesDir` (unresolved path) still resolves built-ins from the embedded set and returns `not found` for a drop-in slug, composing no path.
- The slug renders **control-stripped but untruncated**: stripping already happened at the point the value was read (Phase 5 task 5-2 returns stripped `RawKeys`), and truncation stays panel-local because doctor has full width and wants the whole value. A raw value that strips to empty is *unset* and produces no line.
- Assert the whole producer is inert on disk: `prefs.json` byte-identical before and after, `theme_migrated` absent afterwards on a file that never had it, and zero `theme` records through a `logtest.Sink` — including for the `unreadable`-directory condition, which would otherwise emit `theme: directory unusable`.

**Acceptance Criteria**:
- [ ] `{"theme":"nord-lee"}` with no such file yields exactly `⚠ theme nord-lee does not resolve: not found` — no parenthetical.
- [ ] `{"theme_light":"solar"}` with no such file yields `⚠ theme solar (light) does not resolve: not found`; the same in `theme_dark` yields `(dark)`.
- [ ] `{"theme_light":"x","theme_dark":"x"}` with no such file yields exactly **one** line carrying `(both)`.
- [ ] `{"theme_light":"a","theme_dark":"b"}`, both missing, yields two lines — `(light)` then `(dark)`.
- [ ] `{"theme":"nord","theme_dark":"broken"}` yields **no** line for `broken` — the slots are not in force — and none for `nord`, which resolves as a built-in.
- [ ] An absent `prefs.json`, an empty one, and one with no theme keys each yield zero lines; a corrupt one yields zero lines and no error that aborts the diagnosis.
- [ ] `{"theme":"../evil"}` yields `⚠ theme ../evil does not resolve: bad name`, and no file is opened at any composed path (proven by placing a readable file where a naive join would land).
- [ ] `{"theme":"nord-lee"}` with the themes directory at mode `0000` yields `unreadable`, not `not found`.
- [ ] A persisted value carrying a newline, tab or ANSI escape renders on one line with the escape stripped and **not** truncated, however long it is.
- [ ] A persisted value that is only control characters yields no line — it strips to empty and is therefore unset.
- [ ] After a full doctor run `prefs.json` is byte-identical and `theme_migrated` is still absent; `loadPrefsStore` (migrating) appears nowhere in doctor's call graph.
- [ ] A persisted slug naming a valid built-in or a valid drop-in yields no line.

**Tests**:
- `"it reports an unresolvable constant with no slot parenthetical"` — `TestPersistedThemeAdvisory_ConstantOmitsSlot`
- `"it reports an unresolvable slot by name"` — `TestPersistedThemeAdvisory_SlotRendersLightOrDark`
- `"it collapses two slots naming one slug to a single both line"` — `TestPersistedThemeAdvisory_BothSlots`
- `"it reports only the keys in force"` — `TestPersistedThemeAdvisory_ConstantWinsOverSlots`
- `"it produces no line for an unset slot"` — `TestPersistedThemeAdvisory_VirginInstallIsSilent`
- `"it reports a charset-failing value as bad name"` — `TestPersistedThemeAdvisory_CharsetFailureIsBadName` (table incl. `../evil`, asserting no path composed)
- `"it distinguishes not found from unreadable"` — `TestPersistedThemeAdvisory_NotFoundVersusUnreadable`
- `"it renders the slug control-stripped and untruncated"` — `TestPersistedThemeAdvisory_ControlStrippedUntruncated`
- `"it treats a control-only value as unset"` — `TestPersistedThemeAdvisory_ControlOnlyValueIsUnset`
- `"it tolerates an absent or corrupt prefs file"` — `TestPersistedThemeAdvisory_TolerantOnDegeneratePrefs`
- `"it reads prefs through the non-migrating variant"` — `TestPersistedThemeAdvisory_UsesNonMigratingRead` (byte-compare `prefs.json`; `theme_migrated` still absent)
- `"it resolves without fallbacks and never raises the fatal"` — `TestPersistedThemeAdvisory_NoFallbackAndNoFatal`
- `"it emits zero theme log records"` — `TestPersistedThemeAdvisory_EmitsNoThemeRecords` (including the unusable-directory condition)

**Edge Cases**:
- Doctor reads prefs through **`loadPrefsStoreNoMigrate`** (Phase 6 task 6-5), so running a diagnosis never triggers the one-shot `appearance` translation — a config mutation as a side effect of a diagnosis is exactly what breaks the read-only claim.
- It reports the keys **in force** under §8.2's `theme`-wins rule — the constant alone when one is set, both slots otherwise — because reporting an ignored key as unresolvable would send the user to fix something Portal is not reading.
- Only a **persisted** key produces a line: an unset slot holds the shipped default, which is a built-in and always resolves, so a virgin install produces none, and an unresolvable built-in is Phase 5 task 5-6's fatal rather than an advisory.
- Resolution goes per key through `ResolveByName`, **not** `ResolveNomination`, which would resolve fallbacks and can raise that fatal — neither belongs in a diagnosis.
- `<slot>` renders `light` or `dark` under a pair, **`both`** when the two slots name the same slug (§9.5's `● both`, reachable in two keypresses), and the parenthetical is **omitted entirely** under a constant.
- A charset-failing persisted value yields **`bad name`**, never `not found`, and no path is ever composed from it — `../something` must not become a path component.
- An absent themes directory yields `not found` for a drop-in slug while an unusable one yields `unreadable`, since permissions is the actual problem.
- The slug renders **control-stripped but untruncated** — stripping is a property of the value (Phase 5 task 5-2) while truncation stays panel-local because doctor has full width and wants the whole value.
- A value that strips to empty is unset and produces no line.
- An absent or corrupt `prefs.json` yields zero keys tolerantly and no line, never an error that aborts the diagnosis.
- The resolver is on `log.Discard`, so even a `theme: directory unusable` condition emits nothing.

**Context**:
> §12.2: doctor *"**Reports when a persisted theme name no longer resolves.**… Reading `prefs.json` to report an unresolvable theme goes through the **non-migrating** prefs read (§10.5), so running doctor never triggers the one-shot `appearance` translation — the read-only claim holds literally."*
> §8.4: *"**`portal doctor` reports the keys *in force*** — under §8.2's `theme`-wins rule that is the constant alone when one is set, and both slots otherwise. Reporting an ignored key as unresolvable would send the user to fix something Portal is not reading."*
> §14A: *"Persisted theme unresolvable → `⚠ theme <slug> (<slot>) does not resolve: <reason>`. `<slot>` renders `light` or `dark` under an adaptive pair, **`both` when the two slots name the same slug** (§9.5's `● both` state, reachable in two keypresses), and the parenthetical is omitted entirely under a **constant**… One line in every case, per §12.2's one-slug-one-line rule, which two lines for one slug would break along with `<M>`'s problems-not-detections property."*
> §8.6: *"The persisted value comes from a hand-editable file and is used to **locate a file by name** on a path that deliberately does not enumerate — so `../something` would be used as a path component. **Validate the persisted slug against the same `[a-z0-9-]` charset before use**, and treat an invalid one as unresolvable."*
> §9.5: *"A slug that came from `prefs.json` is control-stripped at the point it is read, not at the point it is drawn… **Truncation is separate and stays panel-local** — doctor has full width and wants the whole value."*
> §5.5: *"A theme made unreachable by an unusable directory carries the reason `unreadable`, not `not found`… `not found` sends the user to check the filename, `unreadable` sends them to check permissions — and permissions is the actual problem."*
> Phase boundary: this task emits its lines independently; the dedup against task 7-3/7-4's file lines and the block's pinned order are task 7-6.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §5.5, §8.2, §8.4, §8.6, §9.5, §10.5, §12.2, §14A

## theming-system-7-6

### Task 7.6: One slug, one advisory line — the persisted line wins

**Problem**: The single most likely failure in the whole feature — the user's persisted theme *is* the invalid file — currently produces **two** lines from two independent producers, so `<M>` counts detections rather than problems and doctor disagrees with the panel about how many things are wrong. §9.4 pins "one slug is one row, always" for the panel; the two surfaces render the same union and must not diverge. Separately, the block's line order is currently whatever the producers happened to append, and a report whose line order can shift between runs is not testable and reads as noise.

**Solution**: An assembly step inside `collectThemeAdvisories` that drops any **file** line whose slug is also carried by a **persisted** line, then emits the surviving lines in a pinned order — directory line, file lines, persisted lines — with `<M>` computed from the final set.

**Outcome**: A user whose `theme_dark = nord-lee` names a file with a bad hex gets exactly one advisory — the persisted one, which carries the reason *and* the slot — while a `bad name` file, a `reserved name` file and the directory line all keep their own lines, and two runs over an unchanged directory produce byte-identical output.

**Do**:
- Restructure `collectThemeAdvisories` into an explicit assembly rather than a concatenation:
  1. **Directory line** (task 7-3), when present. It is directory-level and is **never** deduplicated against a slug.
  2. **File lines** (tasks 7-3 and 7-4), in `Enumerate`'s own order — Phase 1 task 1-7 returns entries in `os.ReadDir` filename order, which is already deterministic; do not re-sort and do not iterate a map.
  3. **Persisted lines** (task 7-5), in a fixed key order: the constant alone, otherwise light then dark, with the `both` case occupying the single position the pair would otherwise fill twice.
  4. Build the set of slugs carried by the persisted lines (`fromPrefs == true`, non-empty `slug`) and **drop** every file line whose `slug` is non-empty and a member of it.
  5. Return `directory ++ survivingFiles ++ persisted`.
- Doc-comment the win rule at the drop: when a persisted slug **is** the invalid file, the persisted line carries strictly more — the reason *and* which slot is affected — so the file line would add nothing but a second entry in `<M>`.
- Doc-comment the two structural non-collisions so they are pinned rather than incidental:
  - a `bad name` file has **no slug** (`Entry.Slug` is empty exactly when the rejection is `bad name`), so it can never match a persisted slug and both lines legitimately stand;
  - a persisted slug naming a `reserved name` file resolves to the **built-in** at `ResolveByName`'s step 2, so task 7-5 produces no line for it at all — the file keeps its own line, and that collision is the entire content of the reason.
- `<M>` is `len(finalSet)` — computed from the assembled slice and handed to the renderer, never from the producers' raw counts. Task 7-2's renderer already derives the count from the slice it is given; assert the two cannot diverge.
- Add a determinism test that runs the assembly twice over an unchanged directory + prefs and compares the rendered block byte-for-byte, and a second that seeds several files whose enumeration order is non-alphabetical on disk and pins the resulting order.

**Acceptance Criteria**:
- [ ] `theme_dark = nord-lee` plus a `nord-lee.theme` with a bad hex yields exactly **one** advisory — `⚠ theme nord-lee (dark) does not resolve: bad colour` — and no `⚠ theme nord-lee: bad colour — …` line.
- [ ] The same collision under a **constant** yields one line with no parenthetical.
- [ ] A `Nord.theme` (`bad name`, no slug) plus any persisted key yields both lines — the filename line and, if unresolvable, the persisted line.
- [ ] `theme = nord` plus a drop-in `nord.theme` yields exactly one line — the file's `reserved name` line — and **no** persisted line, because the slug resolved to the built-in.
- [ ] A persisted slug naming a **valid** drop-in yields neither line.
- [ ] Two slots naming the same broken slug yield one `(both)` line, and any file line for that slug is dropped — one line total.
- [ ] An unusable directory line is never dropped, whatever any persisted slug names.
- [ ] Running the assembly twice over an unchanged directory and prefs yields byte-identical output; no map is iterated anywhere in the assembly.
- [ ] The block's regions appear in the pinned order: directory line, then file lines, then persisted lines.
- [ ] `<M>` equals the number of lines actually rendered, on every scenario above.

**Tests**:
- `"it drops the file line when a persisted line covers the same slug"` — `TestThemeAdvisoryUnion_PersistedLineWins` (constant and slot variants)
- `"it keeps both lines for a bad-name file"` — `TestThemeAdvisoryUnion_BadNameFileNeverCollides`
- `"it keeps the reserved-name file line and produces no persisted line"` — `TestThemeAdvisoryUnion_ReservedNameResolvesToBuiltin`
- `"it produces neither line for a persisted valid file"` — `TestThemeAdvisoryUnion_ValidPersistedFileIsSilent`
- `"it renders one both line rather than two"` — `TestThemeAdvisoryUnion_BothSlotsStayOneLine`
- `"it never dedups the directory line against a slug"` — `TestThemeAdvisoryUnion_DirectoryLineIsNeverDeduped`
- `"it renders the block in a pinned order"` — `TestThemeAdvisoryUnion_OrderIsDeterministic` (repeat runs byte-identical; non-alphabetical seeding)
- `"it counts M from the final line set"` — `TestThemeAdvisoryUnion_CountMatchesRenderedLines`

**Edge Cases**:
- This mirrors §9.4's "one slug is one row, always" so the panel and doctor render the same union and cannot disagree about how many problems exist.
- When a persisted slug **is** the invalid file — the most likely failure of all — the **unresolvable-persisted line wins**, because it carries strictly more (the reason *and* which slot is affected) while the file line would add nothing but a second entry in `<M>`.
- `<M>` therefore counts **problems, not detections**.
- A `bad name` file has **no slug**, so it can never collide with a persisted slug and both lines legitimately stand.
- A persisted slug naming a **`reserved name`** file resolves to the *built-in* and produces no persisted line at all, while the file keeps its own line — that collision is the whole content of the reason.
- A persisted slug naming a **valid** file produces neither line.
- Both slots naming the same broken slug collapse to one `(both)` line rather than two, and the log is already asymmetry-free here since `theme: fallback applied` dedups on `slug`+`reason`.
- The `⚠ themes directory unreadable` line is directory-level and is never deduplicated against a slug.
- The advisory block's order must be **deterministic** so the report is testable and stable between runs — the directory line, the file lines and the persisted lines need a pinned order rather than map-iteration order.
- The union is assembled before rendering, so `<M>` is computed from the final line set and never from the two producers' raw counts.

**Context**:
> §12.2: *"**One slug produces one advisory line**, mirroring §9.4's *'one slug is one row, always'* — the two surfaces render the same union and must not disagree about how many problems exist. When a persisted theme is *also* the invalid file (the most likely failure of all), the **unresolvable-persisted line wins**: it carries strictly more — the reason *and* which slot is affected — so the file-validity line would add nothing but a second entry in `<M>`. `<M>` counts lines, so it counts problems rather than detections."*
> §9.4: *"**'Resolves', not 'has a file'** — the distinction is load-bearing… **One slug is one row**, always: a persisted slug that names a built-in *is* that built-in's row, and a persisted slug that names an existing-but-invalid file *is* that file's row, carrying both the reason and the badge."*
> §14A on the `both` case: *"One line in every case, per §12.2's one-slug-one-line rule, which two lines for one slug would break along with `<M>`'s problems-not-detections property. The log is already asymmetric-free here: `theme: fallback applied` dedups on `slug`+`reason`, so it emits once for the two failed slots regardless."*
> **Decision recorded**: the spec requires the block to be a stable, testable report but does not pin the *sequence* of its three regions. Directory → files → persisted is chosen because it reads outermost-to-innermost (the container, then its contents, then the setting that points into it) and because the directory line, when present, is the condition that explains the absence of every file line beneath it.
> Phase boundary: `--fix` carrying this same assembled block across both renders is task 7-7.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §9.4, §12.2, §14A

## theming-system-7-7

### Task 7.7: `--fix` carries the advisories and repairs nothing

**Problem**: `portal doctor --fix` renders **two** reports — an initial diagnosis and a post-repair re-diagnosis — and the theme scan is wired into only the first. Left there, `--fix` becomes a *less* informative diagnosis than the plain run: the user who reaches for the repairing form is the one most likely to have something wrong, and they would be the one who cannot see it. The opposite risk is worse: doctor has a repair surface, so it is a natural place to "helpfully" delete a junk `.theme` file or rewrite a broken `prefs.json` key — and doctor can prune a stale hook entry, but it cannot repair someone's colours.

**Solution**: Run the same `collectThemeAdvisories` on both renders of the `--fix` path — freshly, not by reusing the first pass's slice — and add **no** theme step to `runDoctorFix`.

**Outcome**: `portal doctor --fix` prints the advisory block and the ` · <M>` suffix in both reports, exits 0 when the only findings are theme advisories, prints no "Pruned …" line for them, leaves every `.theme` file and `prefs.json` byte-identical, and emits zero `theme` log records across the whole run.

**Do**:
- In `doctorCmd.RunE`'s `--fix` branch, call `collectThemeAdvisories(deps)` a **second** time after `runDoctorFix` and pass its result to the second `renderDoctorReport`. Do **not** hoist the first pass's slice — the second report is a genuine re-diagnosis of the same read-only condition, and reusing the first result would make it a replay.
- Add **nothing** to `runDoctorFix`: no theme step, no `.theme` rewrite, no prefs write, no directory creation, no seeding, and no pruning of an invalid theme file. Doc-comment the reason beside the existing repair list — doctor can prune a stale hook entry, but it cannot repair someone's colours, so the theme lines are read-only in both passes.
- Leave the exit code driven **solely** by the post-repair health checks, exactly as today: `doctorUnhealthy(postResults)`. An advisory in either pass cannot move it (task 7-2 already proves the class cannot; this task proves it on the `--fix` path too).
- Assert the whole theme surface is inert across `--fix`: snapshot the themes directory tree (every filename, mode and byte) and `prefs.json` before the run and compare after; assert no `Pruned ` line appears when the only findings are theme advisories.
- Assert zero `theme` records across both passes with a `logtest.Sink` installed for the whole Execute — the loader is `log.Discard`-backed on every doctor path.
- Assert doctor remains bootstrap-exempt: `doctor` stays in `skipTmuxCheck`, so the theme surface starts no server, ensures no saver and runs no restore. Reuse the existing `TestDoctorRegisteredInSkipTmuxCheck` shape rather than inventing a second guard.
- Assert the existing repairs are untouched: the stale-hook prune, the stale-project prune and the log-retention sweep behave exactly as before, including the down-server hazard-guard deferral.
- Prove the re-run behaviourally at the function level — call `collectThemeAdvisories(deps)`, delete the broken file, call it again, and assert the second result reflects current disk state — and structurally with a source guard over `cmd/doctor.go` asserting the `--fix` branch invokes `collectThemeAdvisories` for its second render rather than reusing a variable, in the same source-guard idiom as `TestLoadPrefsStore_SingleProductionCaller`.

**Acceptance Criteria**:
- [ ] A `--fix` run over a directory holding two broken themes prints both advisories in **both** reports, with ` · 2 advisories` on both summaries.
- [ ] A `--fix` run whose only findings are theme advisories exits **0** and prints no `Pruned ` line.
- [ ] A `--fix` run with a failing health check that `--fix` repairs still exits 0, and both reports carry their own advisory block and suffix.
- [ ] `runDoctorFix` contains no theme step: after the run every `.theme` file, the directory's entry set and its modes, and `prefs.json` are byte-identical.
- [ ] `theme_migrated` is not written by any `--fix` run.
- [ ] The whole `--fix` Execute produces zero `theme` records through a `logtest.Sink`.
- [ ] `doctor` is still in `skipTmuxCheck`; the theme surface starts no tmux server and touches no saver or restore path.
- [ ] The second render's advisories are recomputed — a themes-directory change between the two calls is reflected in the second result.
- [ ] The existing stale-hook / stale-project prunes and the log sweep are behaviourally unchanged, hazard guard included.

**Tests**:
- `"it renders advisories in both --fix passes"` — `TestDoctorFix_AdvisoriesInBothPasses`
- `"it suffixes both summaries with the advisory count"` — `TestDoctorFix_SuffixInBothSummaries`
- `"it repairs no theme state"` — `TestDoctorFix_ThemeStateUntouched` (tree + `prefs.json` byte-compare)
- `"it exits zero when the only findings are advisories"` — `TestDoctorFix_AdvisoryOnlyExitsZero` (asserts no `Pruned ` line)
- `"it re-runs the scan for the second render"` — `TestDoctorFix_ScanReRunForSecondPass` (behavioural re-run + source guard)
- `"it emits zero theme records across both passes"` — `TestDoctorFix_EmitsNoThemeRecords`
- `"it stays bootstrap-exempt"` — `TestDoctorFix_RemainsBootstrapExempt`
- `"it leaves the existing repairs unchanged"` — `TestDoctorFix_ExistingRepairsUnchanged`

**Edge Cases**:
- The theme scan runs on the `--fix` path too and its advisories and the ` · <M>` suffix appear in **both** renders — the initial diagnosis and the post-repair re-diagnosis — because suppressing them would make `--fix` a *less* informative diagnosis than the plain run.
- The theme lines are **read-only in both passes**: there is deliberately no repair to perform, so `runDoctorFix` gains no theme step — no file rewrite, no prefs write, no directory creation, no seeding and no pruning of an invalid `.theme` file, since doctor can prune a stale hook entry but cannot repair someone's colours.
- The exit code stays driven **solely** by the post-repair health checks exactly as today, and an advisory in either pass cannot move it.
- A `--fix` run whose only findings are theme advisories exits **0** and prints no "Pruned …" line.
- The scan is **re-run** for the second render rather than the first result being reused, so the second report is a genuine re-diagnosis of the same read-only condition.
- The advisory set may legitimately differ between passes only if the user's directory changed under the run, and nothing `--fix` does causes that.
- The whole command emits **zero** `theme` records across both passes, the loader being on `log.Discard` on every path.
- Doctor remains bootstrap-exempt, so the theme surface starts no server, ensures no saver and runs no restore.
- The log-retention sweep and the two existing prunes are untouched.

**Context**:
> §12.2: *"**The theme scan runs on the `--fix` path too**, and its advisories and the `· <M> advisories` suffix appear there. `--fix` re-diagnoses after repairs and the theme lines are read-only in both passes — there is no repair to perform, and suppressing them would make `--fix` a *less* informative diagnosis than the plain run."*
> §12.2: *"**Read-only, with no `--fix` action.** Doctor can prune a stale hook entry; it cannot repair someone's colours."*
> `cmd/doctor.go`'s existing contract, unchanged by this task: *"the exit is driven SOLELY by the post-repair results — the repairs never touch it directly"*, and every repair is best-effort with its failure logged under the bootstrap component and swallowed.
> §5.5: Portal *"never creates or seeds"* the themes directory — so `--fix` must not create it either, even though `--fix` is the one doctor path that writes.
> Phase boundary: this is the last task of the phase. The panel's own rendering of the same union (`ThemeEnumerator`, one slug one row) is Phase 8, and the two surfaces must agree about the count — which task 7-6's rule is what guarantees.

**Spec Reference**: `.workflows/theming-system/specification/theming-system/specification.md` §5.5, §12.2, §12.3, §14A
