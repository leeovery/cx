# vhs tape: §9.13's FAILED-COMMIT LINE in the §9.1 message slot.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 9-12, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definition in internal/capture is
# PERMANENT and is NOT covered by that clearance — §13.4's swap-and-diff guard
# drives it.
#
# WHY THIS FRAME EXISTS — it is the ONLY place the ABSENCE of a band is checkable.
#
# §9.1's token table gives this line `accent.attention` for BOTH the `⚠` and the
# text, and NO `bg.attention` band: "the warning band is a full-width main-screen
# flash treatment and would read as heavy inside a 24–30 column panel". That is a
# judgement about how something LOOKS at a particular width, so an assertion cannot
# settle it and only an image can. The frame exists to be looked at.
#
# WHAT TO LOOK AT: `⚠ couldn't save theme` directly above the footer, glyph and text
# in one orange run, sitting on the panel's plain canvas with NO tinted band behind
# it. Compare it against the main screen's own warning flash (sessions-inline-flash)
# — the point of the decision is that the band which reads as right across the full
# width would read as heavy in a 34-column column. Then look at the BADGES:
# `● light` on tokyo-night-day and `● dark` on nord, exactly where
# theme-panel-adaptive-pair.png has them. §9.13 forbids a failed write moving the
# `●` — the marker means "what is PERSISTED" and would be lying if it moved — and
# the standing four-row footer is still in force beneath the line, because a failure
# raises no question and substitutes no scope.
#
# THE COHERENCE PAIRING: `--theme nord` names the theme UNDER THE CURSOR. §9.2's
# invariant is that the cursor's row is always what is painted behind the panel, so
# the palette follows the CURSOR, never the persisted setting. At open that resolves
# to the DARK slot's theme, because capturetool runs no light/dark gate and the
# standing no-answer fallback selects dark (§8.8). Changing the --theme below
# without changing the fixture's seeded cursor produces a frame that is
# INDISTINGUISHABLE from a correct one to a reviewer and that §13.4's guard passes
# either way.
#
# THE TAPE TYPES THE KEY ITSELF. The fixture declares `t` in its captureKeys and
# capturetool replays NO keys of its own — captureKeys is the offline mirror ModelAt
# drives, declared once so the two cannot drift. The FAILURE is not typed and could
# not be: a fixture wires no theme persister at all (task 6-7), so a capture cannot
# reach the reported state by committing however many keys it pressed. It is a
# declared fixture SEED carrying STATE and never text — the copy is composed by the
# panel from §14A's pinned constants, so this capture cannot ship a paraphrase of it
# — and it seeds BOTH halves §9.13 gives different lifetimes: the slot's line, and
# the outstanding-failure state behind it.
#
# Like every other fixture this NEVER opens a tmux server, spawns no daemon, runs
# no bootstrap and touches no real config: the union, the persisted keys, the slot
# records, the cursor and the reported failure are all declared fixture data,
# NOTHING IS WRITTEN ANYWHERE (there is no persister to write with), and --theme
# takes an explicit input rather than a config lookup.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/theme-panel-commit-failed.tape
#
# VHS IS KNOWN TO FAIL SILENTLY ON WRITE (§13.3): it runs the tape, reports no
# error, and does not produce the PNG. Hash the target before and after and confirm
# the hash CHANGED before trusting or reviewing the capture; retry on failure. This
# is a FIRST-TIME write through a freshly-written tape and a message state is
# visible ONLY in the image — there is no functional assertion that would catch a
# capture that never landed — so an absent capture reads as "the change didn't
# render" and a stale one reads as a false pass.
#
# To view the same screen live in a real terminal instead (press `t` by hand —
# capturetool replays no keys):
#
#   go run ./cmd/capturetool --fixture theme-panel-commit-failed --theme nord
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/theme-panel-commit-failed.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
# VHS sizes the terminal in PIXELS, so the column/row counts are recorded here
# rather than stated in the tape: 1280x800 resolves to 113 columns x 31 rows, the
# same terminal the other preferred-width panel tapes use. At 113 columns the
# content region is 109 wide and §9.8's ladder clamps to the PREFERRED 34-column
# panel, which is where this frame belongs: the band question is about the widest
# panel that ships, so a narrower capture would understate the width the decision
# was taken against.
Set Width 1280
Set Height 800
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the failed-commit fixture pinned to the
# theme its cursor lands on. `go run` compiles + runs in one step; no tmux server,
# no bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture theme-panel-commit-failed --theme nord"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the Sessions list. Give the compile + first render time to settle.
Sleep 4s

# The fixture's declared captureKeys, typed here so the tape and the offline
# driver reach the same frame: `t` opens the slide-over (§9.6). The failure report
# is raised by the fixture's seed as the panel arms, so there is no second key.
Type "t"
Sleep 1s

Screenshot "testdata/vhs/theme-panel-commit-failed.png"
