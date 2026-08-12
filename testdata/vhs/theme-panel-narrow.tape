# vhs tape: the §9.8 slide-over stepped DOWN to its minimum width.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 8-16, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definition in internal/capture is
# PERMANENT and is NOT covered by that clearance — §13.4's swap-and-diff guard
# drives it.
#
# WHY THIS FRAME EXISTS — it is the ONLY observable check on the width ladder.
#
# §9.8's doctrine for a space shortage is DEGRADE, NEVER REFUSE: the panel takes
# its preferred width while the content region is at least twice it, steps down to
# the minimum below that, and refuses only once even the minimum cannot render.
# The panel fixtures whose subject is the panel itself are captured wide enough to
# take the preferred width, so the step is what this frame is for. The fixture
# DECLARES the stepped-down terminal width itself, so the frame renders at the
# minimum wherever it is driven from.
#
# WHAT TO LOOK AT: a panel NARROWER than the one in theme-panel-adaptive-pair.png,
# with both `● light` and `● dark` badges still on their rows and every row still
# exactly ONE line. The fixture data is deliberately IDENTICAL to that frame's, so
# the pair is a controlled comparison: everything that differs between the two
# images is the width. Look too at the page beneath — §9.1's accepted cost is that
# the overlay CUTS WHEREVER ITS BORDER FALLS, mid-label included, and at this width
# it is cutting a good deal more of the session list than usual.
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
# capturetool replays NO keys of its own — captureKeys is the offline mirror
# ModelAt drives, declared once so the two cannot drift. A panel tape that omitted
# the keypress would screenshot a panel-less Sessions frame: the whole subject of
# the capture missing, with nothing on the image to say so.
#
# Like every other fixture this NEVER opens a tmux server, spawns no daemon, runs
# no bootstrap and touches no real config: the union, the persisted keys, the slot
# records and the cursor are all declared fixture data, and --theme takes an
# explicit input rather than a config lookup.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/theme-panel-narrow.tape
#
# VHS IS KNOWN TO FAIL SILENTLY ON WRITE (§13.3): it runs the tape, reports no
# error, and does not produce the PNG. Hash the target before and after and
# confirm the hash CHANGED before trusting or reviewing the capture; retry on
# failure. This is a FIRST-TIME write through a freshly-written tape and a theme
# change is visible ONLY in the image, so an absent capture reads as "the change
# didn't render" and a stale one reads as a false pass.
#
# To view the same screen live in a real terminal instead (press `t` by hand —
# capturetool replays no keys; no window resize is needed, because the fixture
# pins its own terminal width to the one that steps the ladder down):
#
#   go run ./cmd/capturetool --fixture theme-panel-narrow --theme nord
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/theme-panel-narrow.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
# VHS sizes the terminal in PIXELS, so the column count is recorded here rather
# than stated in the tape: 780x800 resolves to 63 COLUMNS x 31 rows (measured by
# running the tape, not by arithmetic — 10px per column at this font).
# 63 columns is the width the FIXTURE declares, so the tape has to give the vhs
# terminal at least that much or the pinned render is clipped. It matches it
# exactly: the frame then fills the capture with nothing round it. 63 columns
# leaves 59 content columns, below twice the 30-column preferred width, so the
# ladder steps down to its 24-column minimum.
Set Width 780
Set Height 800
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the narrow fixture pinned to the theme
# its cursor lands on. `go run` compiles + runs in one step; no tmux server, no
# bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture theme-panel-narrow --theme nord"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the Sessions list. Give the compile + first render time to settle.
Sleep 4s

# The fixture's declared captureKeys, typed here so the tape and the offline
# driver reach the same frame: `t` opens the slide-over (§9.6).
Type "t"
Sleep 1s

Screenshot "testdata/vhs/theme-panel-narrow.png"
