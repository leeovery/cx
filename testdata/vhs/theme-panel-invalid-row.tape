# vhs tape: the §9.1 theme slide-over listing INVALID drop-in rows.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 8-16, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definition in internal/capture is
# PERMANENT and is NOT covered by that clearance — §13.4's swap-and-diff guard
# drives it.
#
# WHY THIS FRAME EXISTS — it is the ONLY place §9.5's row composition is visible.
#
# §9.5 fixes a four-element priority for a row's ~27–34 columns: the `⚠` glyph,
# then the right-aligned `●` badge, then the label, then the terse reason. Nothing
# else in the panel's specification is a LAYOUT rule, and no other frame renders a
# rejected row at all — so without this capture the whole priority ships unseen.
#
# WHAT TO LOOK AT — three rows, three different outcomes of that one priority:
#
#   * `aurora-glow` — the reason FITS: a text.subtle label (de-emphasised but
#     READABLE, because it is the file the user must identify) beside `⚠ bad
#     syntax` as one accent.attention run, all on ONE line. Every list row is
#     exactly one delegate line; an invalid row never wraps.
#   * `nord-lee` — invalid AND persisted, so the `● dark` badge takes the right
#     edge and the reason is DROPPED. That is the composition priority made
#     visible: the badge outranks the reason, because §9.4 exists so the marker
#     always has a home. `⚠` still says the row is invalid and doctor says why.
#     Note also that the badge sits on the PERSISTED slug and not on the theme
#     that actually loaded — a fallback never moves the `●` (§9.5's badge table).
#   * `My Gorgeous Midnight Palette.theme` — labelled by its FILENAME (a `bad
#     name` file has no slug at all) and long enough to TRUNCATE with `…`. It
#     drops its reason too, by the priority's other rule: the label outranks the
#     reason, so a name long enough to fill the row simply takes the columns.
#
# The cursor is on a VALID row, and must be: §9.5's arrow keys skip invalid rows,
# so a frame with the cursor parked on a rejection is a state production cannot
# reach.
#
# THE COHERENCE PAIRING: `--theme tokyo-night` names the theme UNDER THE CURSOR.
# The persisted dark slot names the broken `nord-lee`, so §8.5's fallback resolves
# it to the shipped dark default and §9.2 lands the cursor there. Changing the
# --theme below without changing the fixture's seeded cursor produces a frame that
# is INDISTINGUISHABLE from a correct one to a reviewer and that §13.4's guard
# passes either way.
#
# THE TAPE TYPES THE KEY ITSELF. The fixture declares `t` in its captureKeys and
# capturetool replays NO keys of its own — captureKeys is the offline mirror
# ModelAt drives, declared once so the two cannot drift. A panel tape that omitted
# the keypress would screenshot a panel-less Sessions frame: the whole subject of
# the capture missing, with nothing on the image to say so.
#
# Like every other fixture this NEVER opens a tmux server, spawns no daemon, runs
# no bootstrap and touches no real config: the union, the persisted keys, the slot
# records and the cursor are all declared fixture data — including every rejection
# on it, since a broken drop-in cannot be staged in a directory the harness is
# forbidden to read — and --theme takes an explicit input rather than a config
# lookup.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/theme-panel-invalid-row.tape
#
# VHS IS KNOWN TO FAIL SILENTLY ON WRITE (§13.3): it runs the tape, reports no
# error, and does not produce the PNG. Hash the target before and after and
# confirm the hash CHANGED before trusting or reviewing the capture; retry on
# failure. This is a FIRST-TIME write through a freshly-written tape and a theme
# change is visible ONLY in the image, so an absent capture reads as "the change
# didn't render" and a stale one reads as a false pass.
#
# To view the same screen live in a real terminal instead (press `t` by hand —
# capturetool replays no keys):
#
#   go run ./cmd/capturetool --fixture theme-panel-invalid-row --theme tokyo-night
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/theme-panel-invalid-row.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
# 1280x800 resolves to 113 columns x 31 rows, the same terminal the other
# preferred-width panel tapes capture at, so the frames are comparable.
Set Width 1280
Set Height 800
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the invalid-row fixture pinned to the
# theme its cursor lands on. `go run` compiles + runs in one step; no tmux server,
# no bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture theme-panel-invalid-row --theme tokyo-night"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the Sessions list. Give the compile + first render time to settle.
Sleep 4s

# The fixture's declared captureKeys, typed here so the tape and the offline
# driver reach the same frame: `t` opens the slide-over (§9.6).
Type "t"
Sleep 1s

Screenshot "testdata/vhs/theme-panel-invalid-row.png"
