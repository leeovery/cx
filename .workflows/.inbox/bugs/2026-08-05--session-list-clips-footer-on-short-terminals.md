# Session list clips the keymap footer on short terminals

On a short terminal the Sessions page loses its keymap footer entirely. At content heights of ten rows or fewer, a direct resize leaves the session list one row taller than the frame can hold, and the composed view clips the bottom — taking the whole footer with it. `? help` is absent at content heights 7, 8, 9 and 10, and reappears from 11 upward.

This was found during the review of theming-system task 8-17, but it is not a theme-panel defect. It reproduces on committed code with a standalone test that never opens the panel — two models sized down to a short content height, one of them additionally squeezed by a notice band and handed the row back. The panel work only surfaced it: task 8-17 raised the panel's minimum height, which moved an existing test from a content height where the two paths happened to agree into one where they don't.

There is a second, related symptom in the same measurements: the list's page size is **path-dependent**. A list that has been squeezed by a notice band and then had the row returned settles on a different `PerPage` than one that was never squeezed. Measured at content height 10, a direct resize gives `PerPage=2` while the squeezed-and-returned path gives `PerPage=1`. The divergence appears from content height 10 through 12 and closes again at 13. The path-dependence lives in `applySessionListSize` alone, with no panel in the picture.

The impact is that on a short terminal — a split pane, a small window, a tmux pane sharing vertical space — the user loses every keybinding hint at once, including `? help`, which is the escape hatch that makes the rest discoverable. They are left with a list and no visible way to learn what any key does. The page still functions; nothing is unreachable by keyboard. But the affordance that tells you so is gone, and there is no indication anything has been dropped.

This is the same failure class CLAUDE.md already records as the original cursor-invisible incident, where uncounted rows overflowed the frame and scrolled the title and cursor off the top. There the fix was to make every list row exactly one delegate line so pagination arithmetic stayed exact.

Relevant code: `applySessionListSize` in `internal/tui/model.go` is where the sizing happens. `applyThemePanelListStyles` in `internal/tui/theme_panel.go` already carries an in-source note about `bubbles/list` settling its page derivation on a second pass, which may be the same underlying behaviour seen from the other side.
