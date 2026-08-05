# vhs tape: the §9.5 pinned `⚠ dir unreadable` chrome row — captured ON PAGE 2.
#
# SCAFFOLDING, NOT AN ASSET (spec §13.2). This tape and the PNG it renders are
# created for the collaboration on task 8-16, committed while that gate is being
# worked, and CLEARED OUT at sign-off in Phase 10 — on exactly the same terms as
# the image. Nothing is expected to keep them; do not treat either as a
# visual-regression baseline. The Go fixture definition in internal/capture is
# PERMANENT and is NOT covered by that clearance — §13.4's swap-and-diff guard
# drives it.
#
# WHY THIS FRAME EXISTS — §13.3: the pinned directory row "has its own placement
# rule, token and pinned copy, and NO OTHER WAY TO BE CHECKED".
#
# Without it every drop-in silently vanishes and the user sees only built-ins:
# the exact "completely in the dark" state §9.4 exists to prevent, in the surface
# it was chosen to prevent it, at the moment the user is standing there to pick a
# theme. The copy is fixed at 16 columns precisely so it fits the panel's minimum
# width WITHOUT truncation — it is the one row §9.5's composition rules cannot
# degrade, having no label, no badge and no reason.
#
# WHY PAGE 2 — it is the whole point. §9.5 makes the row "chrome pinned to the
# viewport directly beneath the header, NOT a list row… a list row participates in
# pagination, so the warning would vanish the moment the user paged down." A
# page-1 capture is INDISTINGUISHABLE from one of a warning implemented as a list
# delegate, because both show the row. The claim is only observable once the user
# has paged past it — hence the `Ctrl+↓` below, and hence this tape's reduced
# HEIGHT: the union must overflow the panel body or there is no second page to
# reach.
#
# WHAT TO LOOK AT: `⚠ dir unreadable` in accent.attention, directly beneath the
# `Themes` header, with theme rows STILL RENDERING BENEATH IT on page 2 —
# `solarized-lee` keeping its `● light` and the built-in `tokyo-night` under the
# cursor. §9.5: "built-in rows and persisted-slug rows still render beneath it —
# the PERSISTED ROWS ESPECIALLY, or a user with an unreadable directory loses the
# `●` entirely." Both persisted rows are `unreadable` rather than `not found`: the
# theme may be sitting right there in a directory nothing can read, and `not found`
# would send the user past the actual problem, which is permissions.
#
# THE COHERENCE PAIRING: `--theme tokyo-night` names the theme UNDER THE CURSOR
# AFTER THE PAGING KEY. The fixture seeds the cursor onto a page-1 row (a `Ctrl+↓`
# from the last page moves nothing), and the key then lands it on `tokyo-night` —
# which is also what the broken dark slot falls back to, so the frame is painted in
# the theme actually rendering. Changing the --theme below without changing the
# fixture's seeded cursor produces a frame that is INDISTINGUISHABLE from a correct
# one to a reviewer and that §13.4's guard passes either way.
#
# THE TAPE TYPES THE KEYS ITSELF. The fixture declares `t` then `Ctrl+↓` in its
# captureKeys and capturetool replays NO keys of its own — captureKeys is the
# offline mirror ModelAt drives, declared once so the two cannot drift. A tape that
# omitted the paging key would screenshot page 1, where this frame's entire claim
# is unobservable, with nothing on the image to say so.
#
# Like every other fixture this NEVER opens a tmux server, spawns no daemon, runs
# no bootstrap and touches no real config: the unreadable directory is a DECLARED
# FLAG on a faked enumeration, not a staged directory — §7.1's import guard
# forbids internal/capture reaching config at all — and --theme takes an explicit
# input rather than a config lookup.
#
# Run it (sandbox MUST be disabled — vhs needs loopback networking for ttyd;
# see testdata/vhs/README.md):
#
#   vhs testdata/vhs/theme-panel-dir-unreadable.tape
#
# VHS IS KNOWN TO FAIL SILENTLY ON WRITE (§13.3): it runs the tape, reports no
# error, and does not produce the PNG. Hash the target before and after and
# confirm the hash CHANGED before trusting or reviewing the capture; retry on
# failure. This is a FIRST-TIME write through a freshly-written tape and a theme
# change is visible ONLY in the image, so an absent capture reads as "the change
# didn't render" and a stale one reads as a false pass.
#
# To view the same screen live in a real terminal instead (press `t` then
# `Ctrl+↓` by hand — capturetool replays no keys; a full-size terminal will not
# paginate, so shrink it to about 16 rows to reproduce this frame):
#
#   go run ./cmd/capturetool --fixture theme-panel-dir-unreadable --theme tokyo-night
#
# NOTE: vhs tape paths containing a slash MUST be quoted (the GIF Output and the
# Screenshot path below) — an unquoted slashed path is a parser error.

# Output is required by vhs even though we only care about the PNG screenshot.
Output "testdata/vhs/.gifcache/theme-panel-dir-unreadable.gif"

Set FontFamily "JetBrains Mono"
Set FontSize 16
# VHS sizes the terminal in PIXELS, so the column/row counts are recorded here
# rather than stated in the tape: 1280x476 resolves to 113 columns x 16 ROWS.
# The height is the load-bearing half. 16 rows leaves 14 content rows; the panel
# spends 5 on its header, 1 on the pinned directory row and 4 on its footer, so
# the list body is 4 rows and `bubbles/list` pages the five-row union two at a
# time — which is what gives the `Ctrl+↓` below a second page to reach. A taller
# terminal fits the whole union on one page and the paging key does nothing.
Set Width 1280
Set Height 476
# A fixed shell prompt keeps the captured frame deterministic across machines.
Set Shell "bash"

# Build then launch the capture tool with the unreadable-directory fixture pinned
# to the theme its cursor lands on. `go run` compiles + runs in one step; no tmux
# server, no bootstrap, no daemon.
Type "go run ./cmd/capturetool --fixture theme-panel-dir-unreadable --theme tokyo-night"
Enter

# First paint: the model fetches the fixture sessions + projects on Init and
# renders the Sessions list. Give the compile + first render time to settle.
Sleep 4s

# The fixture's declared captureKeys, typed here so the tape and the offline
# driver reach the same frame: `t` opens the slide-over (§9.6), then `Ctrl+↓`
# pages the panel's list forward (§9.2's paging key, §12.2's arrows-only revision).
Type "t"
Sleep 1s
Ctrl+Down
Sleep 1s

Screenshot "testdata/vhs/theme-panel-dir-unreadable.png"
