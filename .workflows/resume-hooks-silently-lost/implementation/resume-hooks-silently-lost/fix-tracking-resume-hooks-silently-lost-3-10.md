## Attempt 1

Verdict was **approved** with no blocking issues, but two mandatory comment corrections. The first is a false tmux claim — exactly the defect class this task exists to remove.

COMMENT_CORRECTIONS:
- `internal/tmux/tmux.go:408-409` — the final sentence makes a false tmux claim: for the three `-t` commands `PaneTarget`'s production callers actually issue, a stale `window.pane` coordinate **does** fail, with or without `=`. Measured on tmux 3.7c, scratch socket, session `bar` with live panes `0.0`/`0.1`: `set-option -p -t =bar:0.9` → exit 1 `no such pane`; `capture-pane -t =bar:9.0` → exit 1 `can't find window: 9`; `respawn-pane -k -t =bar:0.9` → exit 1 `can't find pane: 9` — and identically without the `=`. The only command that silently falls back on a nonexistent coordinate is `display-message` (exit 0, resolving to the session's current pane), which is **not** a `PaneTarget` production call site; that behaviour is already documented accurately and correctly scoped to `display-message` at `internal/tmuxtest/stamp.go:17-23`. The `display-message`-specific finding was generalised into a blanket claim about all `-t` targets. The one true residual the sentence gestures at is different: pane indices are positional and renumber on pane death, so a coordinate that still *exists* can address a different pane — verified, after `kill-pane -t baz:0.0`, `=baz:0.2` fails while `=baz:0.0` succeeds onto what was pane 1. This matters because the comment exists precisely to stop a reader over- or under-trusting `=`; as written it tells them `=` is weaker than it is, which invites adding an unnecessary exactness sweep across the call sites this task just declined to touch.
  OLD:
	// live "foo-2" once "foo" is gone. The "=" prefix pins the session name only —
	// it does not make a stale window.pane coordinate fail.
  NEW:
	// live "foo-2" once "foo" is gone. The "=" prefix pins the session name only;
	// it makes no difference to the window.pane half, so a coordinate that has
	// renumbered onto a different pane is caught by neither form.

- `internal/tmux/tmux.go:414-415` — the edit falsifies the sibling doc two lines below it. `PaneTargetExact`'s comment still calls `=` "the prefix every `-t` flag needs", which is the same blanket prohibition this task just removed from `PaneTarget`; the two now directly contradict each other on the same screen.
  OLD:
	// PaneTargetExact is the pane-level sibling of exactTarget: the "=" exact-match
	// prefix every `-t` flag needs.
  NEW:
	// PaneTargetExact is the pane-level sibling of exactTarget: the "=" exact-match
	// prefix a `-t` needs when its session may be gone. See PaneTarget.

NOTES (context — not work items):
- All four of the executor's load-bearing claims were verified and hold. The `liveTarget` reasoning is sound: `armPanes` calls `ListPanesInSession(sess.Name)` at `internal/restore/session.go:113` and aborts the session on error, so the session is live under its exact name for the loop that follows. Confirmed on a scratch socket that an exact session name beats a prefix match even when the prefix-sibling was created first — a pre-existing `foo-2` cannot capture a `-t foo:…` while `foo` lives.
- `PaneTargetExact` is used exactly where claimed (`SelectPane` `:795`, `ResizePaneZoom` `:809`, its only two callers).
- The prefix-match hazard is real for the `-t` forms the callers use, not just `display-message`: with `foo` killed and `foo-2` live, `set-option -p -t foo:0.0 @probe PREFIX_HIT` exited 0 and read back off **`foo-2:0.0`**. The `=` form correctly fails.
- Comment-only was proven rather than eyeballed: with all `//` lines stripped, HEAD's file and the worktree file are byte-identical.
- The reviewer's minimal alternative to correction 1 is to delete the final sentence outright, ending at `…once "foo" is gone.` It recommends the replacement instead, to preserve the warning that `=` is not a cure-all. **Take the replacement.**
- The prefix-match explanation now appears three times in twenty lines (`:407-408`, `:420-422`, and `exactTarget`'s own). The task directed taking the substance from `exactTarget`, so this is sanctioned. Not a work item.

## Attempt 1

## Attempt 1

Verdict was **approved** with no blocking issues, but two mandatory comment corrections. The first is a false tmux claim — exactly the defect class this task exists to remove.

COMMENT_CORRECTIONS:
- `internal/tmux/tmux.go:408-409` — the final sentence makes a false tmux claim: for the three `-t` commands `PaneTarget`'s production callers actually issue, a stale `window.pane` coordinate **does** fail, with or without `=`. Measured on tmux 3.7c, scratch socket, session `bar` with live panes `0.0`/`0.1`: `set-option -p -t =bar:0.9` → exit 1 `no such pane`; `capture-pane -t =bar:9.0` → exit 1 `can't find window: 9`; `respawn-pane -k -t =bar:0.9` → exit 1 `can't find pane: 9` — and identically without the `=`. The only command that silently falls back on a nonexistent coordinate is `display-message` (exit 0, resolving to the session's current pane), which is **not** a `PaneTarget` production call site; that behaviour is already documented accurately and correctly scoped to `display-message` at `internal/tmuxtest/stamp.go:17-23`. The `display-message`-specific finding was generalised into a blanket claim about all `-t` targets. The one true residual the sentence gestures at is different: pane indices are positional and renumber on pane death, so a coordinate that still *exists* can address a different pane — verified, after `kill-pane -t baz:0.0`, `=baz:0.2` fails while `=baz:0.0` succeeds onto what was pane 1. This matters because the comment exists precisely to stop a reader over- or under-trusting `=`; as written it tells them `=` is weaker than it is, which invites adding an unnecessary exactness sweep across the call sites this task just declined to touch.
  OLD:
	// live "foo-2" once "foo" is gone. The "=" prefix pins the session name only —
	// it does not make a stale window.pane coordinate fail.
  NEW:
	// live "foo-2" once "foo" is gone. The "=" prefix pins the session name only;
	// it makes no difference to the window.pane half, so a coordinate that has
	// renumbered onto a different pane is caught by neither form.

- `internal/tmux/tmux.go:414-415` — the edit falsifies the sibling doc two lines below it. `PaneTargetExact`'s comment still calls `=` "the prefix every `-t` flag needs", which is the same blanket prohibition this task just removed from `PaneTarget`; the two now directly contradict each other on the same screen.
  OLD:
	// PaneTargetExact is the pane-level sibling of exactTarget: the "=" exact-match
	// prefix every `-t` flag needs.
  NEW:
	// PaneTargetExact is the pane-level sibling of exactTarget: the "=" exact-match
	// prefix a `-t` needs when its session may be gone. See PaneTarget.

NOTES (context — not work items):
- All four of the executor's load-bearing claims were verified and hold. The `liveTarget` reasoning is sound: `armPanes` calls `ListPanesInSession(sess.Name)` at `internal/restore/session.go:113` and aborts the session on error, so the session is live under its exact name for the loop that follows. Confirmed on a scratch socket that an exact session name beats a prefix match even when the prefix-sibling was created first — a pre-existing `foo-2` cannot capture a `-t foo:…` while `foo` lives.
- `PaneTargetExact` is used exactly where claimed (`SelectPane` `:795`, `ResizePaneZoom` `:809`, its only two callers).
- The prefix-match hazard is real for the `-t` forms the callers use, not just `display-message`: with `foo` killed and `foo-2` live, `set-option -p -t foo:0.0 @probe PREFIX_HIT` exited 0 and read back off **`foo-2:0.0`**. The `=` form correctly fails.
- Comment-only was proven rather than eyeballed: with all `//` lines stripped, HEAD's file and the worktree file are byte-identical.
- The reviewer's minimal alternative to correction 1 is to delete the final sentence outright, ending at `…once "foo" is gone.` It recommends the replacement instead, to preserve the warning that `=` is not a cure-all. **Take the replacement.**
- The prefix-match explanation now appears three times in twenty lines (`:407-408`, `:420-422`, and `exactTarget`'s own). The task directed taking the substance from `exactTarget`, so this is sanctioned. Not a work item.
