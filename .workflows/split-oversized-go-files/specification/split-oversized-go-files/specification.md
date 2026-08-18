# Specification: Split Oversized Go Files

## Specification

### 1. The File-Organisation Standard

Portal sets its own file-organisation standard for Go source, recorded in `CLAUDE.md`. It is a **cohesion rule** — one concern per file within a package — carrying **two detectors** that work at different scales.

**Rationale.** The cost a large file imposes is context cost: a file too large to read in one pass forces chunked reads and burns the reader's context. Line count is the visible symptom, never the thing being measured.

#### 1.1 Detector one — the name-exclusion test

The operative form of the cohesion rule:

> A file's name must be specific enough to exclude things. If material that plausibly belongs in the package could be dropped into the file without contradicting its name, the name is a bucket label and the file will grow without bound.

- It bites at **any** size, including below the tripwire, and covers the whole range the tripwire is blind to.
- A name is judged on whether its concern has a **natural boundary**. Bounded by an external contract (an interface, a struct, a protocol) passes; an open-ended subject area — "everything about X" — does not, because it can accrete indefinitely.
- The test is evaluated against **what a file holds**, not against its name in the abstract: a name can only exclude things relative to a concern.
- **Mirror-naming is a correlate of failure, never the test itself.** A test file named after a production file is usually a bucket because a production filename is the broadest label available — but 217 of 595 test files are mirror-named and most are well-bounded.
- It is a **judgment call**, accepted as such: sharper than "one concern per file", not mechanical, and two readers may differ on a borderline name. The tripwire is the mechanical half; this is the half that needs a person.

#### 1.2 Detector two — the line tripwire

**One tripwire, 1,000 lines, every Go file. The rule names no categories** — no production/test classification, no exemption class.

- A file over 1,000 lines is **presumed** to hold more than one concern and must justify itself. Tripping is never itself a violation.
- The number is a **procedural attention device, not a quality metric**. Its job is to make someone stop and check, because cohesion is hard to judge and otherwise nobody does. A file under 1,000 lines is not thereby well-organised.
- Calibration: 1,000 sits just above the band Portal's own well-named files already occupy — production tops at 775, behaviour-named test files at 997 — so a properly-organised file essentially never trips, and a trip means something.
- **Raw line count.** No blank-or-comment refinement: subtlety buys nothing for an attention device.
- The read-cost asymmetry between file kinds lives in the **strength of the written claim**, not in a file category. A repetitive-by-construction file justifies itself in a sentence; a file holding several concerns cannot. Same detector, different burden of proof, adjudicated on the claim rather than on the `_test.go` suffix.

#### 1.3 Test-file organisation

Test files are organised **per behaviour area, one purpose-named file each** — finer-grained than a 1:1 production mirror (`cmd/open_targets.go` is 75 lines and carries two test files, `open_targets_test.go` and `open_targets_guard_test.go`). Naming a test file after the behaviour it covers is **how it passes the exclusion test**, not a separate prohibition.

#### 1.4 File count is cheap

Many small files beat one large one. There is **no lower bound**, and no "that's too many files" objection to a proposed split.

---

### 2. The `// portal:oversized` Marker

A file that trips the tripwire rebuts the presumption **in the file**, with a marker of this form:

```go
// portal:oversized 2026-08-16 @ f238265 — Fixture definitions only; each TUI
// screen contributes one. No logic lives here.
```

#### 2.1 Why in the file

A rebuttal stored anywhere else — an allow-list, a doc, a commit message — forces a second read to answer a question raised by the first. In the file, the answer rides the read already being made. This follows from the standard's own rationale and fits the surviving convention in the tree: 90 of 870 Go files still carry a leading doc comment.

The rebuttal must be **durable**, not re-derived. Without a recorded rebuttal a tripping file is re-adjudicated by every agent that opens it, which is precisely the context cost the standard exists to remove.

#### 2.2 What the claim must say

The claim **states the single concern the file owns**, so a later reader can falsify it by reading. A bare "large on purpose" rots silently — the file gains a second concern and the note keeps vouching for it.

#### 2.3 What the marker records, and what it must not

Only fields **true at tagging time that never need updating**: the **date** and the **commit verified against**.

- **No content hash** — self-invalidates on every edit, including a typo in the marker itself, and cannot include itself without excluding its own line.
- **No line count** — a poor proxy in both directions: a file can absorb a whole new concern at net-zero line change, or grow 200 lines of the same table and look drifted when nothing changed.
- **No mtime** — not committed, and reset by checkout.

All three must be maintained on every edit, and across a handful of files over years they will not be. **A stale marker that lies is worse than no marker.** A date and a commit simply age: they assert "this claim was checked against this state", so going stale is information rather than error. The commit reference is strictly better than a hash — it does not self-invalidate, and it yields the actual diff on demand (`git diff <sha>..HEAD -- <file>`). The date is nearly redundant against the commit and is kept because it reads at a glance with no tooling.

#### 2.4 The commit field's semantics

The commit recorded is the one the claim was **verified against** — the parent of the commit that introduces the marker, not the commit that introduces it. This must be stated wherever the form is written down, or the first author will think the field is wrong.

#### 2.5 Placement

**Decision required — the discussion left this open.** The marker must sit in the file's **leading comment block**, above the `package` clause.

**Derivation:** §2.1 places the rebuttal in the file so it rides the read already being made — which only holds if the reader meets it first. A marker buried at line 900 of a 1,200-line file is discovered by grep, not by reading, and grep is the property §2.6 needs, not the property §2.1 asked for. The leading position is also the one the tree's surviving doc-comment convention already occupies.

**Accepted cost:** a file whose leading comment block already carries a doc comment gains a second paragraph there.

#### 2.6 The prefix is a namespace

`portal:` is free — no comment pragma of that form exists in the tree today. The `// portal:oversized` prefix is greppable, and grep over it is the enumeration of every rebutted file: **the marker is its own allow-list**, so no separate exemption list exists to rot.

---

### 3. The Tripwire Source Guard

A repo-wide source guard mechanically enforces the tripwire.

#### 3.1 Behaviour

It walks every `.go` file in the repository and **fails on any file over 1,000 lines that does not carry a `// portal:oversized` marker**. Raw line count, per §1.2.

#### 3.2 Why a guard exists at all

A number whose stated job is to make someone stop only delivers that if something fires when the 1,001st line is written, rather than at an audit nobody schedules. The no-guard alternative — standard in `CLAUDE.md`, agents read `CLAUDE.md`, the sweep proves intent — relies on precisely the discipline that produced a 3,448-line `model.go` in a package already demonstrating the sibling pattern.

The sweep leaves **zero files tripping** at release, which makes this the cheapest moment a guard will ever be introduced: a guard born with a fifteen-entry exemption list protects nothing.

#### 3.3 Construction

It follows the repo's established source-guard idiom, ~25 guards deep:

- `internal/log/discard_guard_test.go` is the precedent for a repo-wide source guard — `portalbintest.ProjectRoot()`, walk the source, fail on a forbidden construction.
- `sourceguardtest.GoSourceFiles(root)` already performs the tree walk with its dot-dir, `vendor` and `node_modules` exclusions, and is the walk this guard uses.
- The root package already carries test files (`main_test.go`, `main_panic_test.go`, `main_theme_fatal_test.go`), so a repo-wide structural guard has a home without inventing a package for it.

**Unit lane.** It is a hermetic source walk — no tmux, no daemon, no built binary.

#### 3.4 No exemption list

Unlike the discard guard, which carries a hardcoded allow-list map naming the single file permitted to construct a discard logger — remote from the code it exempts, and exactly the kind of list that rots — this guard needs none. The marker in the file **is** the allow-list, so the exemption lives at the site it exempts and can only rot if the file itself does.

#### 3.5 What the guard checks in the marker

**Decision required — the discussion left this open.** The guard **greps the `// portal:oversized` prefix only**. It does not validate the date's or commit's shape, and it does not read the claim.

**Derivation:** every field the guard could validate is one the marker deliberately does not maintain (§2.3) — a shape check on a date or a SHA confirms only that characters were typed in the right pattern, never that the claim was verified or is still true. The thing that makes a marker honest is the claim, which is unvalidatable by construction and is deliberately left to a reviewer (§1.1, §3.6). A validator that checks the two fields it can and skips the one that matters converts a human obligation into a passed test.

**Accepted cost:** `// portal:oversized` with no fields and no claim silences the guard. That is the same escape hatch §3.6 already accepts, not a new one.

#### 3.6 Accepted risk — exemption inflation

An easy escape hatch can teach a marker-adding reflex and degrade the guard into ceremony. The only mitigation is the marker's own contract: silencing the guard requires naming the single concern the file owns, which is a claim a reviewer can read and falsify — the same deterrent a `//nolint` with a required reason relies on. Soft, and accepted as the residual.

#### 3.7 Concern-pinning guards are not a general obligation

A guard encoding "this file does not own that concern" — as `applyThemeCallSitesIn(…) == 0` already does in `internal/tui/theme_panel_commit_test.go` — remains an available tool where a boundary is load-bearing. It is **not** a requirement per split. The exclusion test is settled as a human check; mandating a mechanical shadow of it would contradict that and multiply guards across every split file.

---

### 4. Sweep Scope

#### 4.1 Posture

The release makes the mechanical detector **true on day one**. The alternative — ship the standard as forward-looking with a backlog — is closed off by the marker's own contract: a monster cannot claim to own one concern, and "this is a bucket and we have not got to it yet" is an admission rather than a rebuttal. There is no honest way to ship the standard while leaving known violators marked-and-standing.

The mechanical half can be made true and then held by a guard. The exclusion test is the human half by construction, so it applies to work as it happens.

#### 4.2 The in-scope set — 17 files, 35,880 lines, 8 packages

Everything over the 1,000-line tripwire, plus the two below-the-line exclusion-test failures this work has already adjudicated.

| File | Lines | Package clause | In scope because |
|---|---|---|---|
| `internal/tui/model_test.go` | 7,116 | `tui_test` | over the line |
| `internal/tmux/portal_saver_test.go` | 3,772 | `tmux_test` | over the line |
| `cmd/open_test.go` | 3,581 | `cmd` | over the line |
| `internal/tui/model.go` | 3,448 | `tui` | over the line |
| `internal/tmux/tmux_test.go` | 3,211 | `tmux_test` | over the line |
| `cmd/state_hydrate_test.go` | 1,849 | `cmd` | over the line |
| `internal/state/capture_test.go` | 1,626 | `state_test` | over the line |
| `cmd/doctor_test.go` | 1,495 | `cmd` | over the line |
| `internal/hooks/store_test.go` | 1,425 | `hooks_test` | over the line |
| `cmd/bootstrap/bootstrap_test.go` | 1,287 | `bootstrap` | over the line |
| `cmd/state_daemon_run_test.go` | 1,217 | `cmd` | over the line |
| `cmd/state_commit_now_test.go` | 1,163 | `cmd` | over the line |
| `internal/resolver/query_test.go` | 1,129 | `resolver_test` | over the line |
| `cmd/theme_test.go` | 1,035 | `cmd` | over the line |
| `internal/theme/resolution_test.go` | 1,016 | `theme_test` | over the line |
| `internal/tmux/tmux.go` | 775 | `tmux` | adjudicated exclusion-test failure |
| `cmd/open.go` | 735 | `cmd` | adjudicated exclusion-test failure |

Over-the-line subtotal: 15 files, 34,370 lines. The two adjudicated failures add 1,510.

Packages touched: `internal/tui`, `internal/tmux`, `cmd`, `cmd/bootstrap`, `internal/state`, `internal/hooks`, `internal/resolver`, `internal/theme`.

`cmd` alone holds seven of the seventeen.

#### 4.3 Why the two below-the-line files are in

`internal/tmux/tmux.go` (775) wraps sessions, windows and panes, hook-key derivation, environment, server lifecycle, options and global hooks — comfortably more than five concerns, and permanently invisible to any line detector. `cmd/open.go` (735) fails the same way.

Both were **already adjudicated** in this work. Excluding them would discard work already done and would ship a standard true on the mechanical detector and knowingly false on the human one, for files someone has actually examined. They cost 1,510 lines against a 34,370-line sweep.

Neither can carry a `// portal:oversized` marker to defer — they are under the line, and the marker's contract rules out "not got to it yet" regardless.

#### 4.4 Unadjudicated exclusion-test failures stay out, explicitly

The human detector applies to work as it happens. Hand-auditing 275 production files before release is not practical, and the two above are in *because* they were already adjudicated — a set bounded by construction at two, not the start of an audit.

**No second marker.** A `// portal:split-pending` (or similar) declaring a known-but-deferred failure was rejected on two grounds. First, the two detectors are not symmetric: the tripwire presumes guilt and demands an answer, so 1,150 lines obliges a decision; the exclusion test carries no presumption — it is the question asked when deciding where to put code. Nobody is required to adjudicate `tmux.go`, so there is no standing verdict to record, and the derivation happens only when someone is about to act on it, which is when it is cheapest because they are already reading. Second, having declined the 275-file audit, only the failures someone happened to notice would ever be marked, which is arbitrary.

#### 4.5 `cmd/open.go` is the sweep's highest-risk file

It is the product's main entry path — resolution grammar, domain pinning, TUI construction, burst dispatch — and **the only file in scope where a bad seam changes behaviour rather than merely relocating it**. Everything else in scope is a test file, `model.go` (clean panel-shaped seams), or `tmux.go` (a method-bag wrapper that splits by method group). It is taken anyway, understood as such.

#### 4.6 `cmd` is not deferred

Dropping `cmd` from the sweep would cut the scope from seventeen files to ten, but `cmd` holds six of the fourteen over-the-line test files plus `open.go`. Leaving them standing breaks the day-one posture, and the marker cannot honestly paper over it — so deferring `cmd` reinstates the forward-looking posture by the back door. The deciding factor: dropping `cmd` costs the posture that makes a guard possible at all, whereas the order-dependency leak it carries (§7.1) is a real bug that will bite regardless of this work.

---

## Working Notes
