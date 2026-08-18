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

## Working Notes
