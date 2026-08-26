# Review Tracking: Resume Hooks Silently Lost - Traceability

## Findings

### 1. The first hooks registered after the upgrade are eaten within ten seconds, and nothing in the plan says so

**Type**: Missing from plan
**Spec Reference**: §8.3 ("Ordering covers the running daemon, not only the binary on disk" — a pre-upgrade sweep is not shape-aware, so every token-keyed entry is deleted within 10s until a bootstrapping command replaces the daemon; running `portal open` before registering is what closes it)
**Plan Reference**: Phase 2, task `resume-hooks-silently-lost-2-2` — **Edge Cases** and **Context**
**Move**: settled
**Change Type**: add-to-task

**Problem**:
Installing the new binary does not replace the daemon that is already running. `_portal-saver` keeps executing the old `portal state daemon` until some command bootstraps the server, and the hook command is bootstrap-exempt — so it never triggers that replacement. The old daemon's sweep does not recognise a token key, so on the day this ships, a user who upgrades and then registers a hook watches it disappear within ten seconds, repeatedly, with the command reporting success every time: the exact symptom this whole work unit exists to remove, reproduced by the release that fixes it. The specification names this and names the one-line mitigation — run any bootstrapping command after upgrading and before registering. The plan carries the harmless half of that same upgrade lag (task 3-1 records that a pre-upgrade daemon captures the old format, so a crash in the window loses the saved token) and drops the damaging half entirely, so nothing in the plan tells whoever ships this that the release needs an ordering step, and no reviewer reading the plan would know to ask.

**Proposal**:
Record it where the exposure is created — the task that first makes `hook set` write a token-keyed entry — as an accepted residual with no code, matching how task 3-1 already records its sibling. The specification states both the hazard and the mitigation and assigns no work item to either, so nothing is being added to the build; what is being added is the note that makes the hazard visible at the point a reader would otherwise assume Phase 1's retention rule covers it. It does not: retention protects the *old-format* keys already on disk, judged by the *new* binary, and this is a *token* key judged by the *old* one.

**Current**:

*Edge Cases — the bullet the new one follows, and the one it precedes:*
```markdown
- `hook` stays bootstrap-exempt: a set `$TMUX_PANE` already implies a live server, and the stamp is a pane write on that server, not a server start
- Restore's saved-state bake keeps the positional `tmux.HookKey` until Phase 3; do not touch `collectArmInfos`, the capture format or `Session.PortalID` here
```

*Context — the closing paragraph:*
```markdown
> Registration's refusal of a `$TMUX_PANE` that names no live pane is task 2-3. After this task a gone pane still fails — the `set-option -p` at step 4 errors on a bogus target in its own right — but it fails after minting and with no way to tell a gone pane from an unstamped one. That discrimination, and the tests that pin it against a real server, are 2-3's.
```

**Proposed Text**:

*Edge Cases:*
```markdown
- `hook` stays bootstrap-exempt: a set `$TMUX_PANE` already implies a live server, and the stamp is a pane write on that server, not a server start
- Installing the binary does not replace the daemon that is running: `_portal-saver` goes on executing the pre-upgrade `portal state daemon` until a bootstrapping command replaces it (`EnsurePortalSaverVersion`, bootstrap step 5), and `hook` is bootstrap-exempt, so registering a hook will never do it. A pre-upgrade sweep is not shape-aware and does not recognise a token key, so for as long as that lag lasts every entry this task writes is deleted within 10s by the rule that was correct for the binary running it. Running any bootstrapping command (`portal open`) after the upgrade and before registering is what closes it — accepted by the specification, an ordering step rather than code
- Restore's saved-state bake keeps the positional `tmux.HookKey` until Phase 3; do not touch `collectArmInfos`, the capture format or `Session.PortalID` here
```

*Context:*
```markdown
> Registration's refusal of a `$TMUX_PANE` that names no live pane is task 2-3. After this task a gone pane still fails — the `set-option -p` at step 4 errors on a bogus target in its own right — but it fails after minting and with no way to tell a gone pane from an unstamped one. That discrimination, and the tests that pin it against a real server, are 2-3's.
>
> Phase 1's retention rule does **not** cover the upgrade lag above, and must not be read as covering it. Retention protects an old-format key on disk from the *new* binary's sweep; the lag is a *token* key judged by the *old* binary's sweep, which knows no shape rule at all. That is the "let `CleanStale` absorb the old entries" outcome the specification rejects for the migration, arriving through a different door — and the door is closed operationally, by ordering, not by anything built here. Its sibling edge, a pre-upgrade daemon capturing the pre-upgrade format, is recorded in task 3-1.
```

**Resolution**: Fixed
**Notes**:

---
