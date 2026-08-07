# vhs tape: §9.2's SLOT-FROM-CONSTANT CONFIRM in the §9.1 message slot, captured
# at the panel's MINIMUM WIDTH.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 9-12, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definition in internal/capture is
# PERMANENT and is NOT covered by that clearance — §13.4's swap-and-diff guard
# drives it.
#
# WHY THIS FRAME EXISTS — it is the ONLY visual proof of the footer substitution.
#
# While the confirm is live it is KEY-EXCLUSIVE within the panel and resolves on
# `y`/`Y`, `n`/`N` or `Esc` alone. The standing footer would therefore advertise
# `⏎ set theme` / `d set as dark` / `l set as light` / `esc close` — FOUR KEYS OF
# WHICH NONE WOULD ACT, which §14.3 names as exactly the dead end a proactive block
# exists to prevent. So the scope is SUBSTITUTED, not extended: two rows, `y
# confirm` / `n cancel`. Nothing else in the harness shows that happening.
#
# AND WHY AT THE MINIMUM WIDTH — §9.1 warns the slot "may wrap to two rows" there,
# and §14A calls panel wording "a layout constraint as much as a copy choice". The
# one copy the spec says might not fit has to be capturable AT the width it might
# not fit at; every other panel frame but theme-panel-narrow is captured wide enough
# to take the preferred width, where this line fits comfortably on one row and the
# warning is about nothing observable.
#
# WHAT TO LOOK AT: `clear constant nord?  y / n` in text.secondary directly above
# the footer, on TWO rows, with NO background band behind it — the confirm is text
# on the panel's own canvas. Then the footer beneath it: two rows, `y confirm` and
# `n cancel`, and none of the standing four. Judge the wrap: the break falls after
# `y /`, leaving a lone `n` on the second row, and the question is whether that
# still reads as an answerable prompt at 27 columns. The list body is two rows
# shorter than it would be with the slot empty — the slot is NOT reserved when
# empty, so raising it costs the list exactly the rows it renders.
#
# THE COHERENCE PAIRING: `--theme tokyo-night-day` names the theme UNDER THE CURSOR
# — the PREVIEWED row, deliberately NOT the persisted constant `nord` the question
# names. §9.2's invariant is that the cursor's row is always what is painted behind
# the panel, and on this frame those are two different rows by design (which is also
# what makes the confirm the state a real `l` would produce here: it asks about
# clearing the constant in order to assign the row you are looking at). Changing the
# --theme below without changing the fixture's seeded cursor produces a frame that
# is INDISTINGUISHABLE from a correct one to a reviewer and that §13.4's guard
# passes either way.
#
# THE TAPE TYPES THE KEY ITSELF. The fixture declares `t` in its captureKeys and
# capturetool replays NO keys of its own — captureKeys is the offline mirror ModelAt
# drives, declared once so the two cannot drift. The CONFIRM is not typed: it is a
# declared fixture SEED (a fixture is a one-shot render, so a state reached by
# pressing `l` has to be declarable directly), and the seed carries STATE and never
# text — the copy is composed by the panel from §14A's pinned constants, so this
# capture cannot ship a paraphrase of it.
#
# Like every other fixture this NEVER opens a tmux server, spawns no daemon, runs
# no bootstrap and touches no real config: the union, the persisted keys, the slot
# records, the cursor and the raised question are all declared fixture data, no
# theme persister is wired at all, and --theme takes an explicit input rather than a
# config lookup.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/theme-panel-confirm.tape
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
# capturetool replays no keys; narrow the window to about 54 columns, or the panel
# takes the preferred width and the wrap this frame is about disappears):
#
#   go run ./cmd/capturetool --fixture theme-panel-confirm --theme tokyo-night-day
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/theme-panel-confirm.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
# VHS sizes the terminal in PIXELS, so the column/row counts are recorded here
# rather than stated in the tape: 690x800 resolves to 54 COLUMNS x 31 rows.
# The width is the load-bearing half. 54 columns leaves 50 content columns, and
# §9.8's ladder takes half of that — 25, below the 27-column minimum, so it clamps
# UP to the minimum. That is the narrowest panel that renders at all, and the width
# at which the confirm's copy wraps. Any terminal from 60 columns up walks back into
# the degraded band and the line fits on one row again.
# The height is deliberately generous: §9.1's slot wraps only ABOVE the height
# floor (at it, the message is truncated to one line instead — see
# theme-panel-min-height-message.tape, which captures that opposite case).
Set Width 690
Set Height 800
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the confirm fixture pinned to the theme
# its cursor lands on. `go run` compiles + runs in one step; no tmux server, no
# bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture theme-panel-confirm --theme tokyo-night-day"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the Sessions list. Give the compile + first render time to settle.
Sleep 4s

# The fixture's declared captureKeys, typed here so the tape and the offline
# driver reach the same frame: `t` opens the slide-over (§9.6). The confirm itself
# is raised by the fixture's seed as the panel arms, so there is no second key.
Type "t"
Sleep 1s

Screenshot "testdata/vhs/theme-panel-confirm.png"
