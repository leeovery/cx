# vhs tape: the §9.8 slide-over ON ITS HEIGHT FLOOR with a message live.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 9-12, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definition in internal/capture is
# PERMANENT and is NOT covered by that clearance — §13.4's swap-and-diff guard
# drives it.
#
# WHY THIS FRAME EXISTS — the floor arithmetic is observable NOWHERE ELSE.
#
# §9.8 defines the height floor as HEADER + FOOTER + ONE LIST ROW + ONE MESSAGE ROW,
# and §13.3 requires a frame of it because "that arithmetic is only observable on a
# frame that renders it". The message row is counted although §9.1 leaves the slot
# unreserved when empty, because neither of its contenders can be suppressed — a
# floor computed without it puts the panel exactly one row short at the moment a
# message appears, asking a question about a row that has just been pushed off
# screen.
#
# AND IT IS THE ONLY CHECK ON THE PER-DIMENSION RULE. §9.1's two dimensions degrade
# DIFFERENTLY on purpose: at the minimum WIDTH the slot may wrap to two rows (see
# theme-panel-confirm.png, which captures exactly that), but "it does cost a row of
# vertical budget, so at the minimum HEIGHT the message is TRUNCATED to one line
# rather than wrapped". A two-row message here would leave zero list rows or
# overflow the frame.
#
# WHAT TO LOOK AT: count the rows. Above the rule the panel draws nothing; beneath
# it, the `Themes` label, then EXACTLY ONE theme row (`nord`, the cursor's, with its
# `● dark`), then `⚠ couldn't save theme` on EXACTLY ONE line, then the standing
# four-row footer ending in `esc close`. That is the floor, rendered. Judge whether
# a one-row list still reads as a picker — this is the smallest panel that opens at
# all, and one row shorter it refuses with a flash rather than opening a broken
# frame. Note the width too: it is the same 27-column minimum the confirm frame uses,
# so the two are directly comparable and the only difference is the height.
#
# WHY THE FAILED-COMMIT LINE RATHER THAN THE CONFIRM: §9.2's confirm substitutes a
# SHORTER footer, and the floor is deliberately computed from the STANDING scope —
# it has to hold for the footer that is live longest, not for the transient one. A
# frame built on the confirm would therefore render the floor's arithmetic for a
# scope the floor is not the arithmetic of.
#
# THE COHERENCE PAIRING: `--theme nord` names the theme UNDER THE CURSOR. §9.2's
# invariant is that the cursor's row is always what is painted behind the panel, and
# at this height that row is the ONE row the body renders. Changing the --theme below
# without changing the fixture's seeded cursor produces a frame that is
# INDISTINGUISHABLE from a correct one to a reviewer and that §13.4's guard passes
# either way.
#
# THE TAPE TYPES THE KEY ITSELF. The fixture declares `t` in its captureKeys and
# capturetool replays NO keys of its own — captureKeys is the offline mirror ModelAt
# drives, declared once so the two cannot drift. The failure report is a declared
# fixture SEED carrying STATE and never text (a fixture wires no theme persister, so
# a capture could not reach the state by committing), and the copy is composed by
# the panel from §14A's pinned constants.
#
# Like every other fixture this NEVER opens a tmux server, spawns no daemon, runs
# no bootstrap and touches no real config: every input is declared fixture data,
# nothing is written anywhere, and --theme takes an explicit input rather than a
# config lookup.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/theme-panel-min-height-message.tape
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
# capturetool replays no keys; the size is the whole subject here, so shrink the
# window to about 54 columns x 13 rows — one row taller and the body gains a second
# theme row, one row shorter and `t` refuses with `terminal too short for the theme
# picker`):
#
#   go run ./cmd/capturetool --fixture theme-panel-min-height-message --theme nord
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/theme-panel-min-height-message.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
# VHS sizes the terminal in PIXELS, so the column/row counts are recorded here
# rather than stated in the tape: 690x420 resolves to 54 COLUMNS x 13 ROWS.
# BOTH halves are load-bearing, and this is the one tape where the height is the
# subject. 13 rows leaves 11 content rows, which is exactly §9.8's floor: the panel
# spends 5 on its header, 4 on the standing footer and 1 on the message slot,
# leaving the ONE list row the floor guarantees. 14 rows gives the body a second row
# and the arithmetic stops being visible; 12 refuses to open at all. The width is
# the same minimum theme-panel-confirm.tape uses — 54 columns leaves 50 content
# columns and §9.8's ladder clamps up to the 27-column minimum — so the two frames
# differ in nothing but height.
Set Width 690
Set Height 420
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the floor fixture pinned to the theme its
# cursor lands on. `go run` compiles + runs in one step; no tmux server, no
# bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture theme-panel-min-height-message --theme nord"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the Sessions list. Give the compile + first render time to settle.
Sleep 4s

# The fixture's declared captureKeys, typed here so the tape and the offline
# driver reach the same frame: `t` opens the slide-over (§9.6). The failure report
# is raised by the fixture's seed as the panel arms, so there is no second key.
Type "t"
Sleep 1s

Screenshot "testdata/vhs/theme-panel-min-height-message.png"
